# --- Orchestration ---

.PHONY: up
up:
	docker compose up -d

.PHONY: up-build
up-build:
	docker compose up -d --build --force-recreate

.PHONY: down
down:
	docker compose down

.PHONY: logs
logs:
	docker compose logs -f

COMPONENTS = booksage bookscout

# --- Quality Assurance (Collective) ---

.PHONY: test-small
test-small:
	@echo ">> Running Small Tests (Unit)"
	@$(foreach comp,$(COMPONENTS),$(MAKE) -C $(comp) test-small;)

.PHONY: test-medium
test-medium:
	@echo ">> Running Medium Tests (SUT)"
	@$(MAKE) -C booksage test-medium

.PHONY: test-large
test-large:
	@echo ">> Running Large Tests (E2E)"
	@$(MAKE) -C booksage test-large

.PHONY: fmt
fmt:
	@$(foreach comp,$(COMPONENTS),$(MAKE) -C $(comp) fmt;)

.PHONY: lint
lint:
	@$(foreach comp,$(COMPONENTS),$(MAKE) -C $(comp) lint;)

.PHONY: tidy
tidy:
	@$(foreach comp,$(COMPONENTS),$(MAKE) -C $(comp) tidy;)

# --- Release ---

.PHONY: build
build:
	@echo ">> Building all images"
	@$(foreach comp,$(COMPONENTS),$(MAKE) -C $(comp) image-build;)

.PHONY: push
push:
	@echo ">> Pushing all images"
	@$(foreach comp,$(COMPONENTS),$(MAKE) -C $(comp) image-push;)

.PHONY: release
release: build push

# --- Protocol Buffers ---

.PHONY: proto-gen
proto-gen:
	@$(MAKE) -C booksage proto-gen

.PHONY: proto-install-deps
proto-install-deps:
	@$(MAKE) -C booksage proto-install-deps

# --- Helper ---

.PHONY: help
help:
	@echo "BookSage Monorepo Makefile"
	@echo ""
	@echo "Test Targets:"
	@echo "  test-small   - Run fast unit tests (PR scope)"
	@echo "  test-medium  - Run SUT tests (Main merge scope)"
	@echo "  test-large   - Run E2E tests (Release scope)"
	@echo ""
	@echo "Release Targets:"
	@echo "  release      - Build and push all images"
	@echo "  build        - Build all images locally"
	@echo "  push         - Push all images to registry"
	@echo ""
	@echo "Maintenance Targets:"
	@echo "  fmt          - Format all code"
	@echo "  lint         - Lint all code"
	@echo "  tidy         - Tidy all modules"
	@echo "  proto-gen    - Generate all protobuf stubs"
	@echo ""
	@echo "Orchestration Targets:"
	@echo "  up           - Start all services (local)"
	@echo "  up-build     - Rebuild and start all services"
	@echo "  down         - Stop all services"
