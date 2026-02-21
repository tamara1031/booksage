import asyncio
import logging
import os
import tempfile
from collections.abc import AsyncIterable
from concurrent.futures import ProcessPoolExecutor

import grpc

from booksage.config import load
from booksage.pb.booksage.v1 import booksage_pb2, booksage_pb2_grpc

# ============================================================================
# Dummy Interfaces for Dependency Injection
# These represent the heavy CPU/GPU-bound tasks implemented in outer modules
# (e.g., etl/ and generation/) and will be injected into the server.
# ============================================================================


class DocumentParser:
    def parse(self, file_path: str, file_type: str, document_id: str) -> dict:
        """
        Mock implementation of the CPU-heavy PDF parsing (e.g., using Docling/PyMuPDF).
        """
        import time

        logging.info(f"Starting heavy ETL parsing for {file_path} (type: {file_type})")
        time.sleep(2)  # Simulate CPU-bound work

        return {
            "document_id": document_id,
            "extracted_metadata": {"status": "mock_success"},
            "documents": [
                {
                    "content": f"# Mock Parsed Content for {file_type}",
                    "type": "text",
                    "page_number": 1,
                }
            ],
        }


class EmbeddingGenerator:
    def generate(self, texts: list[str], embedding_type: str, task_type: str) -> dict:
        """
        Mock implementation of the GPU/CPU-heavy tensor calculations (PyTorch).
        """
        import time

        logging.info(f"Starting heavy embedding generation for {len(texts)} chunks")
        time.sleep(1)  # Simulate GPU/CPU-bound work

        results = [{"text": text, "dense": [0.1] * 768} for text in texts]
        return {"results": results, "total_tokens": len(texts) * 10}


# ============================================================================
# gRPC Servicer Implementation
# ============================================================================


class BookSageWorker(
    booksage_pb2_grpc.DocumentParserServiceServicer, booksage_pb2_grpc.EmbeddingServiceServicer
):
    def __init__(
        self, parser: DocumentParser, embedder: EmbeddingGenerator, executor: ProcessPoolExecutor
    ):
        self.parser = parser
        self.embedder = embedder
        self.executor = executor

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
                self.executor,
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

    async def GenerateEmbeddings(  # noqa: N802
        self, request: booksage_pb2.EmbeddingRequest, context: grpc.aio.ServicerContext
    ) -> booksage_pb2.EmbeddingResponse:
        logging.info(f"Received GenerateEmbeddings request for {len(request.texts)} texts")

        if not request.texts:
            await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "No texts provided for embedding")

        try:
            # 【最重要】 offloading CPU/GPU-bound operations
            loop = asyncio.get_running_loop()
            response_dict = await loop.run_in_executor(
                self.executor,
                self.embedder.generate,
                list(request.texts),
                request.embedding_type,
                request.task_type,
            )

            results = [
                booksage_pb2.EmbeddingResult(
                    text=res["text"], dense=booksage_pb2.DenseVector(values=res["dense"])
                )
                for res in response_dict["results"]
            ]

            return booksage_pb2.EmbeddingResponse(
                results=results, total_tokens=response_dict["total_tokens"]
            )
        except grpc.aio.AioRpcError:
            raise
        except Exception as e:
            logging.error(f"Error generating embeddings: {e}", exc_info=True)
            await context.abort(
                grpc.StatusCode.INTERNAL, f"Internal error during embedding generation: {e}"
            )


# ============================================================================
# Server Configuration & Startup
# ============================================================================


async def serve():
    config = load()
    server = grpc.aio.server()

    # Initialize Dependencies (DI)
    # In a real app, these would come from container.py or similar setup
    parser = DocumentParser()
    embedder = EmbeddingGenerator()

    # Initialize the ProcessPoolExecutor for heavy lifting tasks
    # Defaults to os.cpu_count() workers
    executor = ProcessPoolExecutor(max_workers=config.max_workers)

    worker = BookSageWorker(parser=parser, embedder=embedder, executor=executor)

    booksage_pb2_grpc.add_DocumentParserServiceServicer_to_server(worker, server)
    booksage_pb2_grpc.add_EmbeddingServiceServicer_to_server(worker, server)

    server.add_insecure_port(config.worker_listen_addr)

    logging.info(
        f"Starting BookSage worker gRPC server on {config.worker_listen_addr} "
        f"with {executor._max_workers} processes"
    )
    await server.start()

    try:
        await server.wait_for_termination()
    except asyncio.CancelledError:
        logging.info("Shutting down worker server...")
    finally:
        # Graceful shutdown of the process pool
        executor.shutdown(wait=True)


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO, format="%(asctime)s - [%(levelname)s] - %(message)s")

    try:
        asyncio.run(serve())
    except KeyboardInterrupt:
        logging.info("Server stopped by user")
