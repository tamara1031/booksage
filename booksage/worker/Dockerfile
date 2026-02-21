# syntax=docker/dockerfile:1
FROM python:3.12-slim

WORKDIR /app

# Install uv for fast dependency resolution
RUN pip install uv

# Copy uv project files
COPY worker/pyproject.toml worker/uv.lock worker/README.md ./worker/

WORKDIR /app/worker
# Install dependencies into a new isolated environment (.venv)
RUN uv sync
ENV PATH="/app/worker/.venv/bin:$PATH"

# Copy shared proto
COPY proto /app/proto/

# Copy python source
COPY worker/src ./src

# Generate Python stubs via grpc_tools
RUN python -m grpc_tools.protoc -I/app/proto \
    --python_out=src/booksage/pb \
    --grpc_python_out=src/booksage/pb \
    /app/proto/booksage/v1/booksage.proto \
    && sed -i 's/from booksage.v1 import/from . import/g' src/booksage/pb/booksage/v1/*.py

# Set PYTHONPATH so absolute imports work
ENV PYTHONPATH=/app/worker/src

# Expose gRPC port
EXPOSE 50051

# Command to run the worker package
CMD ["python", "-m", "booksage.server"]
