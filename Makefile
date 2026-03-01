.PHONY: help generate css dev build test test-cover lint migrate migrate-create migrate-down migrate-up migrate-status migrate-reset

# Database URL default
DATABASE_URL ?= ./goth.db?_journal_mode=WAL

# ===========================================
# Database
# ===========================================
migrate-up:
	@go run -tags fts5 ./scripts/ops/migrate.go up sqlite3 $(DATABASE_URL)

migrate-down:
	@go run -tags fts5 ./scripts/ops/migrate.go down sqlite3 $(DATABASE_URL)

migrate-status:
	@go run -tags fts5 ./scripts/ops/migrate.go status sqlite3 $(DATABASE_URL)

migrate-reset:
	@go run -tags fts5 ./scripts/ops/migrate.go reset sqlite3 $(DATABASE_URL)

migrate-create:
	@go run github.com/pressly/goose/v3 -dir migrations create $(name) sql

migrate: migrate-up

db-seed:
	@go run -tags fts5 ./cmd/api seed

db-reset:
	@rm -f goth.db goth.db-wal goth.db-shm
	@go run -tags fts5 ./cmd/api seed

# ===========================================
# Development
# ===========================================
generate: update-js css
	@go tool templ generate
	@go tool sqlc generate
	@go tool swag init -g internal/cmd/server.go

update-js:
	@mkdir -p web/static/assets/js
	@cp node_modules/htmx.org/dist/htmx.min.js web/static/assets/js/
	@cp node_modules/alpinejs/dist/cdn.min.js web/static/assets/js/alpine.min.js

css:
	@./node_modules/.bin/tailwindcss -i ./web/static/assets/css/input.css -o ./web/static/assets/styles.css --minify

dev: generate css
	@rm -f goth.db goth.db-wal goth.db-shm
	@go run -tags fts5 ./cmd/api seed
	@go tool air

dev-reset: generate css
	@rm -f goth.db goth.db-wal goth.db-shm
	@go run -tags fts5 ./cmd/api seed

build: generate css
	@go build -tags fts5 -ldflags="-s -w" -o bin/goth ./cmd/api

# ===========================================
# Tests
# ===========================================
test:
	@go test -tags fts5 -v -race ./internal/... ./test/integration/...

test-integration:
	@go test -tags fts5 -v ./test/integration/...

test-e2e:
	@go test -tags fts5 -v ./test/e2e/...

test-all: test-integration test-e2e

test-cover:
	@go test -tags fts5 -coverprofile=coverage.out ./internal/... ./test/integration/...
	@go tool cover -html=coverage.out

# ===========================================
# Benchmarks
# ===========================================
bench:
	@go test -tags fts5 -bench=. -benchmem ./test/benchmarks/...

bench-json:
	@mkdir -p test/benchmarks/golden
	@go test -tags fts5 -bench=. -benchmem -json ./test/benchmarks/... 2>&1 | \
		go run ./scripts/bench/benchmark-parse/main.go > test/benchmarks/golden/bench_$(shell date +%Y%m%d_%H%M%S).json

bench-save:
	@mkdir -p test/benchmarks/golden
	@echo "Running benchmarks and saving as golden baseline..."
	@go test -tags fts5 -bench=. -benchmem ./test/benchmarks/... 2>&1 | \
		tee /tmp/bench_output.txt
	@go run ./scripts/bench/benchmark-parse/main.go < /tmp/bench_output.txt > test/benchmarks/golden/golden_$(shell date +%Y%m%d_%H%M%S).json
	@echo "Golden file saved!"

bench-compare:
	@echo "Running benchmarks..."
	@go test -tags fts5 -bench=. -benchmem ./test/benchmarks/... 2>&1 | \
		tee /tmp/bench_current.txt
	@go run ./scripts/bench/benchmark-parse/main.go < /tmp/bench_current.txt > /tmp/bench_current.json
	@echo ""
	@go run ./scripts/bench/benchmark-compare/main.go \
		--baseline test/benchmarks/golden/golden_baseline.json \
		--current /tmp/bench_current.json \
		--threshold $(or $(THRESHOLD),10) \
		--all

bench-check:
	@echo "Checking for regressions..."
	@go test -tags fts5 -bench=. -benchmem ./test/benchmarks/... 2>&1 | \
		tee /tmp/bench_current.txt
	@go run ./scripts/bench/benchmark-parse/main.go < /tmp/bench_current.txt > /tmp/bench_current.json
	@go run ./scripts/bench/benchmark-compare/main.go \
		--baseline test/benchmarks/golden/golden_baseline.json \
		--current /tmp/bench_current.json \
		--threshold $(or $(THRESHOLD),10)

bench-run:
	@go test -tags fts5 -bench=$(name) -benchmem ./test/benchmarks/...

bench-profile:
	@mkdir -p profiles
	@go test -tags fts5 -bench=$(or $(name),.) -benchmem -cpuprofile=profiles/cpu.prof -memprofile=profiles/mem.prof ./test/benchmarks/...
	@echo "Profiles saved to profiles/"

# ===========================================
# Load Testing (k6)
# ===========================================
loadtest:
	@echo "Running load test with k6..."
	@mkdir -p test-results
	@go tool k6 run --duration 5m --vus 100 test/load/loadtest.js

loadtest-short:
	@echo "Running short load test (1m)..."
	@mkdir -p test-results
	@go tool k6 run --duration 1m --vus 50 test/load/loadtest.js

loadtest-stress:
	@echo "Running stress test..."
	@mkdir -p test-results
	@go tool k6 run --duration 10m --vus 500 test/load/loadtest.js


bench-list:
	@echo "Available golden files:"
	@ls -lh test/benchmarks/golden/*.json 2>/dev/null || echo "No golden files found"

bench-clean:
	@rm -f test/benchmarks/test_perf*.db*
	@rm -f profiles/*.prof
	@echo "Benchmark artifacts cleaned"

# ===========================================
# Docker
# ===========================================
docker-build:
	docker-compose build

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-restart:
	docker-compose restart

docker-logs:
	docker-compose logs -f

docker-logs-app:
	docker-compose logs -f app

docker-logs-traefik:
	docker-compose logs -f traefik

docker-logs-otel:
	docker-compose logs -f otelcol

# Development stack
docker-dev: docker-up
	@echo "Development stack started!"
	@echo "App: http://localhost"
	@echo "Traefik: http://localhost:8080/dashboard/"

# Production stack (with telemetry + backup)
docker-prod:
	docker-compose --profile prod up -d

# Stop all
docker-stop:
	docker-compose down

# ===========================================
# Telemetry (Grafana Stack)
# ===========================================
telemetry-up:
	docker-compose -f docker-compose.yml -f docker-compose.dev.yml up -d otelcol loki tempo prometheus grafana

telemetry-down:
	docker-compose -f docker-compose.yml -f docker-compose.dev.yml down

telemetry-logs:
	docker-compose logs -f otelcol loki tempo prometheus grafana

grafana-open:
	@echo "Opening Grafana..."
	@if command -v xdg-open > /dev/null; then \
		xdg-open http://localhost:3001; \
	elif command -v open > /dev/null; then \
		open http://localhost:3001; \
	else \
		echo "Open http://localhost:3001 in your browser"; \
	fi

tempo-open:
	@echo "Opening Tempo..."
	@if command -v xdg-open > /dev/null; then \
		xdg-open http://localhost:3200; \
	elif command -v open > /dev/null; then \
		open http://localhost:3200; \
	else \
		echo "Open http://localhost:3200 in your browser"; \
	fi

# ===========================================
# Traefik
# ===========================================
traefik-dashboard:
	@echo "Opening Traefik Dashboard..."
	@if command -v xdg-open > /dev/null; then \
		xdg-open http://localhost:8080/dashboard/; \
	elif command -v open > /dev/null; then \
		open http://localhost:8080/dashboard/; \
	else \
		echo "Open http://localhost:8080/dashboard/ in your browser"; \
	fi

traefik-certs:
	@docker-compose exec traefik traefik certificates list 2>/dev/null || echo "Run 'docker-compose up -d' first"

# ===========================================
# Backup (Litestream)
# ===========================================
backup-restore:
	@./scripts/ops/backup.sh restore

backup-list:
	@./scripts/ops/backup.sh list

backup-status:
	@./scripts/ops/backup.sh status

backup-snapshot:
	@./scripts/ops/backup.sh snapshot

# ===========================================
# Setup
# ===========================================
setup: setup-traefik setup-tools
	@echo "Setup complete!"

setup-traefik:
	@./scripts/dev/setup-traefik.sh

setup-tools:
	@echo "Installing development tools..."
	@go tool golangci-lint
	@go tool gosec
	@go tool k6 version
	@echo "Tools installed successfully!"

# ===========================================
# Lint
# ===========================================
lint:
	@go tool golangci-lint run --timeout=5m

lint-install:
	@echo "Installing golangci-lint..."
	@go tool golangci-lint
	@echo "golangci-lint installed!"

# ===========================================
# Security
# ===========================================
sec:
	@echo "Running security scan with gosec..."
	@go tool gosec -fmt=text ./...

sec-json:
	@echo "Running security scan with gosec (JSON output)..."
	@go tool gosec -fmt=json -out=security-results.json ./...
	@echo "Results saved to security-results.json"

sec-install:
	@echo "Installing gosec..."
	@go tool gosec
	@echo "gosec installed!"

# ===========================================
# Secrets
# ===========================================
secret-rotate:
	@echo "Rotating secrets..."
	@./scripts/dev/rotate-secret.sh $(or $(SECRET),session_secret)

secret-status:
	@echo "Checking secrets status..."
	@curl -s http://localhost:8080/api/secrets/status | jq .

secret-reload:
	@echo "Reloading secrets..."
	@curl -s -X POST http://localhost:8080/api/secrets/reload | jq .

# ===========================================
# Help
# ===========================================
help:
	@echo "Goth Stack - Makefile Commands"
	@echo ""
	@echo "Setup:"
	@echo "  make setup           - Install all tools and setup Traefik"
	@echo "  make setup-tools     - Install development tools (golangci-lint, gosec, k6)"
	@echo "  make setup-traefik   - Setup Traefik for development"
	@echo ""
	@echo "Database:"
	@echo "  make migrate          - Run database migrations"
	@echo "  make db-seed          - Seed database with test data"
	@echo "  make db-reset         - Reset database"
	@echo ""
	@echo "Development:"
	@echo "  make dev              - Start development server with hot reload"
	@echo "  make build            - Build production binary"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-dev       - Start development stack"
	@echo "  make docker-prod      - Start production stack (with telemetry + backup)"
	@echo "  make docker-down      - Stop containers"
	@echo "  make docker-logs      - View logs"
	@echo ""
	@echo "Telemetry:"
	@echo "  make telemetry-up     - Start Grafana stack"
	@echo "  make grafana-open     - Open Grafana"
	@echo ""
	@echo "Lint & Security:"
	@echo "  make lint             - Run linter (golangci-lint)"
	@echo "  make lint-install     - Install golangci-lint"
	@echo "  make sec              - Run security scan (gosec)"
	@echo "  make sec-json         - Run security scan with JSON output"
	@echo "  make sec-install      - Install gosec"
	@echo ""
	@echo "Secrets:"
	@echo "  make secret-rotate    - Rotate a secret (SECRET=session_secret)"
	@echo "  make secret-status    - Check secrets status"
	@echo "  make secret-reload    - Reload secrets from .env file"
	@echo ""
	@echo "Backup:"
	@echo "  make backup-restore   - Restore from S3"
	@echo "  make backup-list      - List backups"
	@echo ""
	@echo "Tests:"
	@echo "  make test             - Run tests"
	@echo "  make bench            - Run benchmarks"
	@echo ""
	@echo "Load Testing (k6):"
	@echo "  make loadtest         - Run standard load test (5m, 100 VUs)"
	@echo "  make loadtest-short   - Run short load test (1m, 50 VUs)"
	@echo "  make loadtest-stress  - Run stress test (10m, 500 VUs)"
	@echo ""
	@echo "Full list: make help"
