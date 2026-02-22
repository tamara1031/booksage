import asyncio
import logging
from concurrent.futures import ProcessPoolExecutor

import grpc

from booksage.config import load
from booksage.application.service import DocumentParser
from booksage.adapters.grpc.handler import BookSageWorker
from booksage.pb.booksage.v1 import booksage_pb2_grpc


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
