import asyncio
import logging
import os
import tempfile
from collections.abc import AsyncIterable
from concurrent.futures import ProcessPoolExecutor, ThreadPoolExecutor

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


class EmbeddingGenerator:
    def __init__(self, model_name: str = "all-MiniLM-L6-v2"):
        # Load model lazily or at init depending on memory needs
        # In a real heavy environment, maybe we load at __init__ to warm up
        self.model_name = model_name
        self._model = None

    def _get_model(self):
        if self._model is None:
            from sentence_transformers import SentenceTransformer

            self._model = SentenceTransformer(self.model_name)
        return self._model

    def generate(self, texts: list[str], embedding_type: str, task_type: str) -> dict:
        """
        Implementation of the GPU/CPU-heavy tensor calculations using sentence-transformers.
        """
        if not texts:
            return {"results": [], "total_tokens": 0}

        model = self._get_model()
        logging.info(
            f"Starting actual embedding generation for {len(texts)} chunks using {self.model_name}"
        )

        # Calculate embeddings
        embeddings = model.encode(texts, convert_to_numpy=True)

        # Optional: handling sparse vs dense if embedding_type is different, but for now we do dense
        results = [
            {"text": text, "dense": vector.tolist()}
            for text, vector in zip(texts, embeddings, strict=False)
        ]

        # Approximate token count (simplified)
        total_tokens = sum(len(text.split()) for text in texts) * 2

        return {"results": results, "total_tokens": total_tokens}


# ============================================================================
# gRPC Servicer Implementation
# ============================================================================


class BookSageWorker(
    booksage_pb2_grpc.DocumentParserServiceServicer, booksage_pb2_grpc.EmbeddingServiceServicer
):
    def __init__(
        self,
        parser: DocumentParser,
        embedder: EmbeddingGenerator,
        cpu_executor: ProcessPoolExecutor,
        gpu_executor: ThreadPoolExecutor,
    ):
        self.parser = parser
        self.embedder = embedder
        self.cpu_executor = cpu_executor
        self.gpu_executor = gpu_executor

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

    async def GenerateEmbeddings(  # noqa: N802
        self, request: booksage_pb2.EmbeddingRequest, context: grpc.aio.ServicerContext
    ) -> booksage_pb2.EmbeddingResponse:
        logging.info(f"Received GenerateEmbeddings request for {len(request.texts)} texts")

        if not request.texts:
            await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "No texts provided for embedding")

        try:
            # 【最重要】 offloading GPU-bound operations to ThreadPoolExecutor for CUDA safety
            loop = asyncio.get_running_loop()
            response_dict = await loop.run_in_executor(
                self.gpu_executor,
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

    # Initialize the ProcessPoolExecutor for heavy CPU lifting tasks
    # Defaults to os.cpu_count() workers
    cpu_executor = ProcessPoolExecutor(max_workers=config.max_workers)

    # Initialize Context-safe ThreadPoolExecutor for PyTorch/GPU tasks
    # Defaults to max 4 to avoid OOM on GPUs
    gpu_executor = ThreadPoolExecutor(max_workers=min(4, config.max_workers))

    worker = BookSageWorker(
        parser=parser,
        embedder=embedder,
        cpu_executor=cpu_executor,
        gpu_executor=gpu_executor,
    )

    booksage_pb2_grpc.add_DocumentParserServiceServicer_to_server(worker, server)
    booksage_pb2_grpc.add_EmbeddingServiceServicer_to_server(worker, server)

    server.add_insecure_port(config.worker_listen_addr)

    logging.info(
        f"Starting BookSage worker gRPC server on {config.worker_listen_addr} "
        f"with {cpu_executor._max_workers} CPU procs and {gpu_executor._max_workers} GPU threads"
    )
    await server.start()

    try:
        await server.wait_for_termination()
    except asyncio.CancelledError:
        logging.info("Shutting down worker server...")
    finally:
        # Graceful shutdown of the executors
        cpu_executor.shutdown(wait=True)
        gpu_executor.shutdown(wait=True)


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO, format="%(asctime)s - [%(levelname)s] - %(message)s")

    try:
        asyncio.run(serve())
    except KeyboardInterrupt:
        logging.info("Server stopped by user")
