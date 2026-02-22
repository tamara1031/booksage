import asyncio
import logging
import os
import tempfile
from collections.abc import AsyncIterable
from concurrent.futures import ProcessPoolExecutor

import grpc

from booksage.config import load

# ============================================================================
# Dummy Interfaces for Dependency Injection
# These represent the heavy CPU/GPU-bound tasks implemented in outer modules
# (e.g., etl/ and generation/) and will be injected into the server.
# ============================================================================
from booksage.domain.models import DocumentMetadata
from booksage.etl.epub_adapter import EpubParser
from booksage.etl.ports import IDocumentParser
from booksage.etl.pymupdf_adapter import PyMuPDFParser
from booksage.pb.booksage.v1 import booksage_pb2, booksage_pb2_grpc


class DocumentParser:
    def __init__(self):
        # Setup the router/registry of parsers based on extension
        self.parsers: dict[str, IDocumentParser] = {
            ".pdf": PyMuPDFParser(),
            ".epub": EpubParser(),
        }

    def parse(self, file_path: str, file_type: str, document_id: str) -> dict:
        """
        Routes the file to the appropriate ETL parser based on file extension.
        """
        import os

        _, ext = os.path.splitext(file_path)
        ext = ext.lower()

        parser = self.parsers.get(ext)
        if not parser:
            logger = logging.getLogger(__name__)
            logger.warning(f"No specific parser found for extension {ext}, using PyMuPDF fallback.")
            parser = PyMuPDFParser()

        # Build basic domain metadata object
        metadata = DocumentMetadata(
            book_id=document_id,
            title=os.path.basename(file_path),
            extra_attributes={"file_type": file_type},
        )

        logging.info(
            f"Starting actual ETL parsing for {file_path} using {parser.__class__.__name__}"
        )

        # Execute the parse
        raw_doc = parser.parse_file(file_path, metadata)

        # Convert the RawDocument Pydantic model into the dictionary format
        # expected by the gRPC handler
        return {
            "document_id": document_id,
            "extracted_metadata": raw_doc.metadata,
            "documents": [
                {
                    "content": el.content,
                    "type": el.type,
                    "page_number": el.page_number,
                }
                for el in raw_doc.elements
            ],
        }



# ============================================================================
# gRPC Servicer Implementation
# ============================================================================


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
    ) -> booksage_pb2.ParseResponse:
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

            # Reconstruct protobuf from the pickleable dict
            documents = [
                booksage_pb2.RawDocument(
                    content=doc["content"],
                    type=doc["type"],
                    page_number=doc["page_number"],
                )
                for doc in response_dict["documents"]
            ]
            response = booksage_pb2.ParseResponse(
                document_id=response_dict["document_id"],
                extracted_metadata=response_dict["extracted_metadata"],
                documents=documents,
            )
            logging.info(f"Successfully finished parsing for document {response.document_id}")
            return response
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



# ============================================================================
# Server Configuration & Startup
# ============================================================================


async def serve():
    config = load()
    server = grpc.aio.server()

    # Initialize Dependencies (DI)
    # In a real app, these would come from container.py or similar setup
    parser = DocumentParser()

    # Initialize the ProcessPoolExecutor for heavy CPU lifting tasks
    cpu_executor = ProcessPoolExecutor(max_workers=config.max_workers)

    worker = BookSageWorker(
        parser=parser,
        cpu_executor=cpu_executor,
    )

    booksage_pb2_grpc.add_DocumentParserServiceServicer_to_server(worker, server)

    server.add_insecure_port(config.worker_listen_addr)

    logging.info(
        f"Starting BookSage worker gRPC server on {config.worker_listen_addr} "
        f"with {cpu_executor._max_workers} CPU procs"
    )
    await server.start()

    try:
        await server.wait_for_termination()
    except asyncio.CancelledError:
        logging.info("Shutting down worker server...")
    finally:
        # Graceful shutdown of the executors
        cpu_executor.shutdown(wait=True)


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO, format="%(asctime)s - [%(levelname)s] - %(message)s")

    try:
        asyncio.run(serve())
    except KeyboardInterrupt:
        logging.info("Server stopped by user")
