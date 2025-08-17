# Dropbox Sync Makefile

# Variables
BINARY_NAME := dropbox-sync
BUILD_DIR := target
CONFIG_FILE := config.toml
DOCKER_IMAGE := dropbox-sync
DOCKER_TAG := latest

# Environment configuration (debug by default)
ENV ?= debug
ifeq ($(ENV),release)
    CARGO_BUILD := cargo build --release
    CARGO_RUN := cargo run --release --
    BINARY_PATH := $(BUILD_DIR)/release/$(BINARY_NAME)
    BUILD_FLAG := --release
    BUILD_MODE := release
else
    CARGO_BUILD := cargo build
    CARGO_RUN := cargo run --
    BINARY_PATH := $(BUILD_DIR)/debug/$(BINARY_NAME)
    BUILD_FLAG := 
    BUILD_MODE := debug
endif

# Default target
.PHONY: help
help: ## Show this help message
	@echo "Current build mode: $(BUILD_MODE) (set ENV=release for release mode)"
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Build targets
.PHONY: build
build: ## Build in current mode (ENV=debug|release)
	$(CARGO_BUILD)

.PHONY: check
check: ## Check code without building
	cargo check

.PHONY: test
test: ## Run all tests
	cargo test

.PHONY: fmt
fmt: ## Format code using rustfmt
	cargo fmt

.PHONY: lint
lint: ## Run clippy linter
	cargo clippy -- -D warnings

.PHONY: clean
clean: ## Clean build artifacts
	cargo clean

# Configuration targets
.PHONY: init-config
init-config: build ## Generate default configuration file
	$(BINARY_PATH) init-config

.PHONY: check-config
check-config: ## Validate configuration file exists
	@if [ ! -f $(CONFIG_FILE) ]; then \
		echo "Configuration file $(CONFIG_FILE) not found. Run 'make init-config' first."; \
		exit 1; \
	fi
	@echo "Configuration file $(CONFIG_FILE) found ✓"



# Server targets
.PHONY: start
start: build check-config ## Start the webhook server and sync daemon
	$(BINARY_PATH) start

.PHONY: start-bg
start-bg: build check-config ## Start server in background
	nohup $(BINARY_PATH) start > dropbox-sync.log 2>&1 &
	@echo "Server started in background. Check dropbox-sync.log for output."

.PHONY: stop
stop: ## Stop background server
	@pkill -f "$(BINARY_NAME)" || echo "No running instances found"

.PHONY: status
status: ## Check if server is running
	@pgrep -f "$(BINARY_NAME)" > /dev/null && echo "Server is running ✓" || echo "Server is not running ✗"

.PHONY: logs
logs: ## Show server logs (if running in background)
	tail -f dropbox-sync.log

# Development workflow targets
.PHONY: dev
dev: ## Run directly with cargo (good for development)
	$(CARGO_RUN) --help

.PHONY: dev-start
dev-start: check-config ## Start server using cargo run
	$(CARGO_RUN) start

.PHONY: quick-check
quick-check: fmt lint check ## Quick development checks

.PHONY: full-check
full-check: clean fmt lint check test build ## Full development checks

.PHONY: watch
watch: ## Watch for changes and rebuild (requires cargo-watch)
	@which cargo-watch > /dev/null || (echo "cargo-watch not installed. Run: cargo install cargo-watch" && exit 1)
	cargo watch -x check -x test

.PHONY: install-deps
install-deps: ## Install required development dependencies
	rustup component add rustfmt clippy
	cargo install cargo-watch

# Docker targets
.PHONY: docker-build
docker-build: ## Build Docker image
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

.PHONY: docker-run
docker-run: docker-build ## Run in Docker container
	docker run -d \
		--name $(BINARY_NAME) \
		-p 3000:3000 \
		-v $(PWD)/$(CONFIG_FILE):/app/config.toml:ro \
		-v $(PWD)/sync:/app/sync \
		-v $(PWD)/output:/app/output \
		-v $(HOME)/.dropbox_sync:/root/.dropbox_sync \
		$(DOCKER_IMAGE):$(DOCKER_TAG) start

.PHONY: docker-stop
docker-stop: ## Stop Docker container
	docker stop $(BINARY_NAME) || true
	docker rm $(BINARY_NAME) || true

.PHONY: docker-logs
docker-logs: ## Show Docker container logs
	docker logs -f $(BINARY_NAME)

.PHONY: docker-shell
docker-shell: ## Open shell in Docker container
	docker exec -it $(BINARY_NAME) /bin/bash

# Installation targets
.PHONY: install
install: ## Install binary to system PATH
	$(CARGO_BUILD)
	sudo cp $(BINARY_PATH) /usr/local/bin/
	@echo "$(BINARY_NAME) installed to /usr/local/bin/"

.PHONY: uninstall
uninstall: ## Remove binary from system PATH
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "$(BINARY_NAME) removed from /usr/local/bin/"

# Utility targets
.PHONY: size
size: build ## Show binary size
	@ls -lh $(BINARY_PATH) | awk '{print "Binary size: " $$5}'

.PHONY: deps
deps: ## Show dependency tree
	cargo tree

.PHONY: update
update: ## Update dependencies
	cargo update

.PHONY: security-audit
security-audit: ## Run security audit on dependencies
	@which cargo-audit > /dev/null || (echo "cargo-audit not installed. Run: cargo install cargo-audit" && exit 1)
	cargo audit

.PHONY: backup-config
backup-config: ## Backup configuration and tokens
	@mkdir -p backups
	@cp $(CONFIG_FILE) backups/$(CONFIG_FILE).$(shell date +%Y%m%d_%H%M%S) 2>/dev/null || true
	@cp -r $(HOME)/.dropbox_sync backups/dropbox_sync_tokens.$(shell date +%Y%m%d_%H%M%S) 2>/dev/null || true
	@echo "Configuration and tokens backed up to backups/ directory"

.PHONY: version
version: ## Show version information
	@echo "Build mode: $(BUILD_MODE)"
	@echo "Rust version: $(shell rustc --version)"
	@echo "Cargo version: $(shell cargo --version)"
	@if [ -f $(BINARY_PATH) ]; then \
		echo "Binary: $(shell ls -la $(BINARY_PATH) | awk '{print $$9 " (built " $$6 " " $$7 " " $$8 ")"}')"; \
	else \
		echo "Binary: not built yet"; \
	fi

# Setup target for new users
.PHONY: setup
setup: install-deps init-config ## Complete setup for new users
	@echo ""
	@echo "🎉 Setup complete! Next steps:"
	@echo "1. Edit $(CONFIG_FILE) with your Dropbox app credentials"
	@echo "2. Run 'make start' to start the sync service"
	@echo "3. Get auth URL: curl http://localhost:3001/admin/auth"
	@echo "4. Open the returned URL in browser to authenticate"
	@echo "5. Use curl -X POST http://localhost:3001/admin/sync to trigger manual sync"
	@echo ""
	@echo "💡 Use ENV=release for production builds"
	@echo "🔒 Admin port 3001 should be firewalled off from public access"