import asyncio
import logging

import grpc
from grpc.aio import ServicerContext

import booksage.pb.booksage.v1.booksage_pb2 as pb
import booksage.pb.booksage.v1.booksage_pb2_grpc as pb_grpc
from booksage.config import load

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


class DocumentParserServiceServicer(pb_grpc.DocumentParserServiceServicer):
    async def Parse(self, request_iterator, context: ServicerContext):  # noqa: N802
        """
        Stream parser. Receives chunks, responds with parsed documents.
        """
        metadata = None
        chunks = []

        try:
            async for req in request_iterator:
                if req.HasField("metadata"):
                    metadata = req.metadata
                    logger.info(f"Received metadata for: {metadata.filename}")
                elif req.HasField("chunk_data"):
                    if metadata is None:
                        await context.abort(
                            grpc.StatusCode.INVALID_ARGUMENT, "Metadata must be sent first"
                        )
                    chunks.append(req.chunk_data)

            if not chunks:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "No file data received")

            # Note: docling ETL logic goes here (mocked for initial setup)
            # Reconstruct complete file: data = b"".join(chunks)
            total_size = sum(len(c) for c in chunks)
            logger.info(f"Received all chunks. Total size: {total_size} bytes")

            # Simulated parsing result
            doc1 = pb.RawDocument(
                content="# Parsed Header\nThis is a dummy response.",
                type="text",
                page_number=1,
                metadata={"parsed_by": "docling_dummy"},
            )

            res = pb.ParseResponse(
                document_id=metadata.document_id if metadata else "unknown",
                extracted_metadata={
                    "author": "unknown",
                    "title": metadata.filename if metadata else "",
                },
                documents=[doc1],
            )
            return res

        except Exception as e:
            logger.error(f"Error during Parse streaming: {e}")
            await context.abort(grpc.StatusCode.INTERNAL, str(e))


class EmbeddingServiceServicer(pb_grpc.EmbeddingServiceServicer):
    async def GenerateEmbeddings(self, request: pb.EmbeddingRequest, context: ServicerContext):  # noqa: N802
        """
        Batch embedding generation.
        """
        try:
            texts = request.texts
            emb_type = request.embedding_type
            logger.info(f"Received request to embed {len(texts)} chunks using {emb_type}")

            if not texts:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "Texts array cannot be empty")

            results = []
            # Note: actual torch model inference goes here (mocked for setup)
            for _idx, text in enumerate(texts):
                # Mock dummy dense vector
                dense = pb.DenseVector(values=[0.1, 0.2, 0.3])
                res = pb.EmbeddingResult(text=text, dense=dense)
                results.append(res)

            return pb.EmbeddingResponse(results=results, total_tokens=len(texts) * 10)

        except Exception as e:
            logger.error(f"Error generating embeddings: {e}")
            await context.abort(grpc.StatusCode.INTERNAL, str(e))


async def serve():
    config = load()
    server = grpc.aio.server()
    pb_grpc.add_DocumentParserServiceServicer_to_server(DocumentParserServiceServicer(), server)
    pb_grpc.add_EmbeddingServiceServicer_to_server(EmbeddingServiceServicer(), server)

    server.add_insecure_port(config.worker_listen_addr)
    logger.info(f"Starting Booksage Python gRPC worker on {config.worker_listen_addr}")

    await server.start()

    # Wait for termination
    try:
        await server.wait_for_termination()
    except asyncio.exceptions.CancelledError:
        logger.info("Gracefully shutting down...")
        await server.stop(grace=5)


if __name__ == "__main__":
    asyncio.run(serve())
