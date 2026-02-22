REGISTRY ?= tamara1031
TAG ?= latest

# --- Protocol Buffers ---
PROTO_DIR = proto/booksage/v1
GO_PROTO_OUT = api/internal/pb
PYTHON_PROTO_OUT = worker/src/booksage/pb

# --- API (Go) ---

.PHONY: test-api-small
test-api-small:
	@echo "==> Running API Small Tests"
	@cd api && go test ./internal/...

.PHONY: test-api-medium
test-api-medium:
	@echo "==> Running API Medium Tests"
	@cd api && go test -v ./tests/sut/...

.PHONY: fmt-api
fmt-api:
	@cd api && go fmt ./...

.PHONY: lint-api
lint-api:
	@cd api && golangci-lint run ./...

.PHONY: tidy-api
tidy-api:
	@cd api && go mod tidy

.PHONY: image-api-build
image-api-build:
	@echo "==> Building API Image: $(REGISTRY)/booksage-api:$(TAG)"
	docker build -t $(REGISTRY)/booksage-api:$(TAG) -f api/Dockerfile .

.PHONY: image-api-push
image-api-push:
	@echo "==> Pushing API Image: $(REGISTRY)/booksage-api:$(TAG)"
	docker push $(REGISTRY)/booksage-api:$(TAG)

# --- Worker (Python) ---

.PHONY: test-worker-small
test-worker-small:
	@echo "==> Running Worker Small Tests"
	@cd worker && uv run pytest tests/unit/

.PHONY: test-worker-medium
test-worker-medium:
	@echo "==> Running Worker Medium Tests"
	@cd worker && uv run pytest tests/sut/

.PHONY: test-worker-large
test-worker-large:
	@echo "==> Running Worker Large Tests"
	@cd worker && uv run pytest tests/integration/

.PHONY: fmt-worker
fmt-worker:
	@cd worker && uvx ruff format . && uvx ruff check . --fix

.PHONY: lint-worker
lint-worker:
	@cd worker && uvx ruff check .

.PHONY: tidy-worker
tidy-worker:
	@cd worker && uv lock

.PHONY: image-worker-build
image-worker-build:
	@echo "==> Building Worker Image: $(REGISTRY)/booksage-worker:$(TAG)"
	docker build -t $(REGISTRY)/booksage-worker:$(TAG) -f worker/Dockerfile .

.PHONY: image-worker-push
image-worker-push:
	@echo "==> Pushing Worker Image: $(REGISTRY)/booksage-worker:$(TAG)"
	docker push $(REGISTRY)/booksage-worker:$(TAG)

# --- Aggregates ---

.PHONY: test-small
test-small: test-api-small test-worker-small

.PHONY: test-medium
test-medium: test-api-medium test-worker-medium

.PHONY: test-large
test-large: test-worker-large
	@./tests/large/e2e_test.sh

.PHONY: test-all
test-all: test-small test-medium test-large

.PHONY: image-build
image-build: image-api-build image-worker-build

.PHONY: image-push
image-push: image-api-push image-worker-push

.PHONY: fmt
fmt: fmt-api fmt-worker

.PHONY: lint
lint: proto-lint lint-api lint-worker

.PHONY: tidy
tidy: tidy-api tidy-worker

# --- Proto Targets ---

.PHONY: proto-go
proto-go:
	@mkdir -p $(GO_PROTO_OUT)
	@uv run --project worker python -m grpc_tools.protoc -Iproto --plugin=protoc-gen-go=$(shell go env GOPATH)/bin/protoc-gen-go --plugin=protoc-gen-go-grpc=$(shell go env GOPATH)/bin/protoc-gen-go-grpc --go_out=$(GO_PROTO_OUT) --go_opt=paths=source_relative --go-grpc_out=$(GO_PROTO_OUT) --go-grpc_opt=paths=source_relative $(PROTO_DIR)/*.proto

.PHONY: proto-python
proto-python:
	@mkdir -p $(PYTHON_PROTO_OUT)
	@uv run --no-project --with grpcio-tools python -m grpc_tools.protoc -Iproto --python_out=worker/src/booksage/pb --grpc_python_out=worker/src/booksage/pb $(PROTO_DIR)/*.proto
	@sed -i 's/from booksage.v1 import/from . import/g' $(PYTHON_PROTO_OUT)/booksage/v1/*.py

.PHONY: proto-gen
proto-gen: proto-go proto-python

.PHONY: proto-lint
proto-lint:
	@protoc --lint_out=proto -Iproto $(PROTO_DIR)/*.proto

.PHONY: proto-install-deps
proto-install-deps:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.1
	go install github.com/ckaznocha/protoc-gen-lint@v0.3.0
