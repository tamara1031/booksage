# syntax=docker/dockerfile:1
FROM golang:1.25.7 AS builder

WORKDIR /app

# Install protoc dependencies (not strictly needed if compiled out of band, 
# but good practice if you want to compile stubs in Docker)
RUN apt-get update && apt-get install -y protobuf-compiler \
    && rm -rf /var/lib/apt/lists/*
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest \
    && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Pre-copy/cache go.mod for pre-downloading dependencies
COPY api/go.mod api/go.sum ./api/
RUN cd api && go mod download

# Map in the shared proto definitions
COPY proto ./proto
# Map in Go source
COPY api ./api

WORKDIR /app/api
# Build the Go API Orchestrator
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/booksage-api

FROM alpine:latest
WORKDIR /
COPY --from=builder /server /server
ENTRYPOINT ["/server"]
