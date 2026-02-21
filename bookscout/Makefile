REGISTRY ?= tamara1031
TAG ?= latest

.PHONY: test-small
test-small:
	@echo "==> Running BookScout Small Tests"
	@go test ./...

.PHONY: test
test: test-small

.PHONY: fmt
fmt:
	@go fmt ./...

.PHONY: lint
lint:
	@golangci-lint run ./...

.PHONY: tidy
tidy:
	@go mod tidy

.PHONY: image-build
image-build:
	@echo "==> Building BookScout Image: $(REGISTRY)/bookscout:$(TAG)"
	docker build -t $(REGISTRY)/bookscout:$(TAG) -f Dockerfile .

.PHONY: image-push
image-push:
	@echo "==> Pushing BookScout Image: $(REGISTRY)/bookscout:$(TAG)"
	docker push $(REGISTRY)/bookscout:$(TAG)
