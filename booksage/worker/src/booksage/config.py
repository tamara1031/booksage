import multiprocessing
import os
from dataclasses import dataclass


@dataclass
class Config:
    """Config holds all environmentally dependent settings for the BookSage Worker."""

    worker_listen_addr: str
    max_workers: int


def load() -> Config:
    """Load reads settings from environment variables with sensible defaults."""
    workers_str = os.getenv("SAGE_WORKER_MAX_WORKERS", "")
    max_workers = int(workers_str) if workers_str.isdigit() else multiprocessing.cpu_count()

    return Config(
        worker_listen_addr=os.getenv("SAGE_WORKER_LISTEN_ADDR", "[::]:50051"),
        max_workers=max_workers,
    )
