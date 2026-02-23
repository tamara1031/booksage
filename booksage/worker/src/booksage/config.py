import multiprocessing

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class WorkerSettings(BaseSettings):
    """Configuration for the BookSage Worker."""

    model_config = SettingsConfigDict(env_prefix="SAGE_WORKER_", case_sensitive=False)

    port: str = Field(default="[::]:50051", description="gRPC server listen address")
    max_concurrency: int = Field(
        default_factory=lambda: multiprocessing.cpu_count(),
        description="Maximum Number of CPU worker processes",
    )


def load() -> WorkerSettings:
    """Load settings from environment variables."""
    return WorkerSettings()
