import asyncio
import logging
import os
import tempfile
from collections.abc import AsyncIterable
from concurrent.futures import ProcessPoolExecutor

import grpc

from booksage.application.service import DocumentParser
from booksage.pb.booksage.v1 import booksage_pb2, booksage_pb2_grpc


class BookSageWorker(booksage_pb2_grpc.DocumentParserServiceServicer):
    def __init__(
        self,
        parser: DocumentParser,
        cpu_executor: ProcessPoolExecutor,
    ):
        self.parser = parser
        self.cpu_executor = cpu_executor

    async def Parse(  # noqa: N802
        self,
        request_iterator: AsyncIterable[booksage_pb2.ParseRequest],
        context: grpc.aio.ServicerContext,
    ) -> AsyncIterable[booksage_pb2.ParseResponse]:
        logging.info("Received streaming Parse request")

        metadata, file_chunks = await self._collect_parse_stream(request_iterator, context)

        # Combine the chunks into a complete binary payload
        file_data = b"".join(file_chunks)
        logging.info(f"Received {len(file_data)} bytes for document {metadata.document_id}")

        # Extract the file extension from the filename, default to .txt if none
        _, ext = os.path.splitext(metadata.filename)
        if not ext:
            ext = ".txt"

        with tempfile.NamedTemporaryFile(delete=False, suffix=ext) as tmp_file:
            tmp_file.write(file_data)
            tmp_file_path = tmp_file.name

        try:
            # Offloading CPU-bound operations
            loop = asyncio.get_running_loop()
            response_dict = await loop.run_in_executor(
                self.cpu_executor,
                self.parser.parse,
                tmp_file_path,
                metadata.file_type,
                metadata.document_id,
            )

            all_docs = response_dict["documents"]
            chunk_size = 50  # Number of documents (pages/paragraphs) per gRPC message

            total_docs = len(all_docs)
            logging.info(
                f"Parsing complete. Yielding {total_docs} chunks in batches of {chunk_size}"
            )

            for i in range(0, total_docs, chunk_size):
                chunk_docs = all_docs[i : i + chunk_size]

                documents_pb = [
                    booksage_pb2.RawDocument(
                        content=doc["content"],
                        type=doc["type"],
                        page_number=doc["page_number"],
                        metadata={
                            "level": str(doc.get("level", 0)),
                            **doc.get("extra_metadata", {}),
                        },
                    )
                    for doc in chunk_docs
                ]

                # Only include metadata in the first message to save bandwidth
                extracted_meta = response_dict["extracted_metadata"] if i == 0 else {}

                yield booksage_pb2.ParseResponse(
                    document_id=response_dict["document_id"],
                    extracted_metadata=extracted_meta,
                    documents=documents_pb,
                )

            logging.info(
                f"Successfully finished streaming parsing for document {metadata.document_id}"
            )

        except grpc.aio.AioRpcError:
            # Re-raise gRPC abortions
            raise
        except Exception as e:
            logging.error(f"Error during parsing: {e}", exc_info=True)
            await context.abort(grpc.StatusCode.INTERNAL, f"Internal error during parsing: {e}")
        finally:
            # Always clean up the temporary file after Parsing is complete
            if os.path.exists(tmp_file_path):
                os.remove(tmp_file_path)

    async def _collect_parse_stream(self, request_iterator, context):
        metadata = None
        file_chunks = []
        try:
            async for request in request_iterator:
                if request.HasField("metadata"):
                    if metadata is not None:
                        await context.abort(
                            grpc.StatusCode.INVALID_ARGUMENT, "Metadata already received"
                        )
                    metadata = request.metadata
                    logging.info(f"Receiving chunks for document: {metadata.document_id}")
                elif request.HasField("chunk_data"):
                    if metadata is None:
                        await context.abort(
                            grpc.StatusCode.INVALID_ARGUMENT,
                            "Metadata must be the first message in the stream",
                        )
                    file_chunks.append(request.chunk_data)
                else:
                    await context.abort(
                        grpc.StatusCode.INVALID_ARGUMENT, "Unknown payload type in stream"
                    )

            if not metadata:
                await context.abort(
                    grpc.StatusCode.INVALID_ARGUMENT, "No metadata provided in the stream"
                )

            if not file_chunks:
                await context.abort(
                    grpc.StatusCode.INVALID_ARGUMENT, "No file data chunks received"
                )
        except Exception as e:
            logging.error(f"Error reading stream: {e}", exc_info=True)
            await context.abort(grpc.StatusCode.INTERNAL, f"Error reading stream: {e}")

        return metadata, file_chunks
