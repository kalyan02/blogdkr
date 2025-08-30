# BlogSync Hybrid Setup Makefile
# Caddy runs in Docker, BlogSync runs natively on host

# Variables
COMPOSE_FILE := docker-compose.yml
SERVICE_CADDY := caddy
ODDITY_DIR := oddity

# Environment configuration (debug by default)
ENV ?= debug

.PHONY: help
help: ## Show this help message
	@echo "Current build mode: $(ENV) (set ENV=release for release mode)"
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Setup targets
.PHONY: setup
setup: ## Initial setup - create directories
	@mkdir -p caddy/data caddy/config caddy/logs
	@echo "Created Caddy directories"
	@echo ""
	@echo "🎉 Setup complete! Next steps:"
	@echo "1. Edit $(ODDITY_DIR)/config.toml with your blog configuration"
	@echo "2. Run 'make start' to start both Caddy and BlogSync"
	@echo "3. Get auth URL: curl http://localhost:3001/admin/auth"
	@echo "4. Open the returned URL in browser to authenticate with Dropbox"
	@echo "5. Use curl -X POST http://localhost:3001/admin/sync to trigger manual sync"

# Docker services management
.PHONY: up
up: ## Start all Docker services (PHP + Caddy)
	@mkdir -p caddy/data caddy/config caddy/logs public_html
	sudo docker-compose up -d

.PHONY: down
down: ## Stop all Docker services
	sudo docker-compose down

.PHONY: docker-restart
docker-restart: down up ## Restart all Docker services

# Individual service management
.PHONY: caddy-up
caddy-up: ## Start Caddy reverse proxy (requires sudo for port 80)
	@mkdir -p caddy/data caddy/config caddy/logs
	sudo docker-compose up -d $(SERVICE_CADDY)

.PHONY: caddy-down
caddy-down: ## Stop Caddy reverse proxy
	sudo docker-compose stop $(SERVICE_CADDY)

.PHONY: caddy-restart
caddy-restart: caddy-down caddy-up ## Restart Caddy

.PHONY: php-up
php-up: ## Start PHP-FPM service
	sudo docker-compose up -d php

.PHONY: php-down
php-down: ## Stop PHP-FPM service
	sudo docker-compose stop php

.PHONY: php-restart
php-restart: php-down php-up ## Restart PHP-FPM

.PHONY: php-build
php-build: ## Build PHP container
	sudo docker-compose build php

.PHONY: caddy-logs
caddy-logs: ## Show Caddy logs
	sudo docker-compose logs -f $(SERVICE_CADDY)

.PHONY: caddy-shell
caddy-shell: ## Open shell in Caddy container
	sudo docker-compose exec $(SERVICE_CADDY) /bin/sh

# Combined operations
.PHONY: start
start: caddy-up  ## Start both Caddy and BlogSync
	@echo ""
	@echo "✅ Services started:"
	@echo "  - Caddy reverse proxy: http://localhost (port 80)"
	@echo "  - BlogSync service: localhost:3000 (proxied through Caddy)"
	@echo ""
	@echo "Check status with: make status"

.PHONY: stop
stop: oddity-stop caddy-down ## Stop both services
	@echo "✅ All services stopped"

.PHONY: restart
restart: stop start ## Restart both services

.PHONY: status
status: ## Check status of all services
	@echo "=== Caddy Status ==="
	@sudo docker-compose ps
	@echo ""
	@echo "=== BlogSync Status ==="
	@cd $(ODDITY_DIR) && make status || echo "BlogSync not running"
	@echo ""
	@echo "=== Service Health ==="
	@curl -s -o /dev/null -w "Port 80 (Caddy): %{http_code}\n" http://localhost/health 2>/dev/null || echo "Port 80 (Caddy): FAILED"
	@curl -s -o /dev/null -w "Port 3000 (BlogSync): %{http_code}\n" http://localhost:3000/health 2>/dev/null || echo "Port 3000 (BlogSync): FAILED"

.PHONY: logs
logs: ## Show logs for both services
	@echo "Starting log tail for both services..."
	@echo "Press Ctrl+C to stop"
	@(sudo docker-compose logs -f $(SERVICE_CADDY) &)
	@cd $(ODDITY_DIR) && make logs


# Troubleshooting
.PHONY: debug
debug: ## Show debug information
	@echo "=== System Information ==="
	@echo "Docker: $(shell docker --version 2>/dev/null || echo 'Not installed')"
	@echo "Docker Compose: $(shell docker-compose --version 2>/dev/null || echo 'Not installed')"
	@echo "Rust: $(shell rustc --version 2>/dev/null || echo 'Not installed')"
	@echo ""
	@echo "=== Network Information ==="
	@netstat -tlnp | grep -E ':80|:3000' || echo "No services listening on ports 80 or 3000"
	@echo ""
	@echo "=== Directory Structure ==="
	@ls -la
	@echo ""
	@echo "=== BlogSync Directory ==="
	@ls -la $(ODDITY_DIR)/