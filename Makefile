.PHONY: build run test clean generate watch db-migrate db-rollback seed

# Build configuration
BINARY_NAME=goth
CMD_PATH=./cmd/api
MAIN_GO=$(CMD_PATH)/main.go

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOVET=$(GOCMD) vet
GOFMT=$(GOCMD) fmt
GOGENERATE=$(GOCMD) generate

# Tailwind CSS
TAILWIND=./tailwindcss-linux-x64
TAILWIND_INPUT=./web/static/assets/css/tailwind.css
TAILWIND_OUTPUT=./web/static/assets/styles.css

# Templ
TEMPL=templ

# Build flags
LDFLAGS=-ldflags "-s -w"
BUILD_FLAGS=-tags fts5

# Database
DB_PATH=./storage/goth.db

# Internal commands
INTERNAL_CMD_PATH=./internal/cmd

# Build the application
build: generate css
	$(GOBUILD) $(BUILD_FLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME) ./internal/cmd/api
	@echo "✓ Build complete: bin/$(BINARY_NAME)"

# Build for production
build-prod: generate css
	CGO_ENABLED=0 $(GOBUILD) $(BUILD_FLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME) ./internal/cmd/api
	@echo "✓ Production build complete: bin/$(BINARY_NAME)"

# Run the application
run: generate css
	$(GORUN) $(BUILD_FLAGS) $(CMD_PATH) server

# Run with hot reload (air)
watch:
	air

# Generate code (templ, sqlc, swagger)
generate:
	@echo "Generating templates..."
	$(TEMPL) generate
	@echo "✓ Templates generated"

# Build CSS with Tailwind
css:
	@echo "Building CSS..."
	$(TAILWIND) -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT) --minify
	@echo "✓ CSS built: $(TAILWIND_OUTPUT)"

# Watch CSS for changes
watch-css:
	$(TAILWIND) -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT) --watch

# Run tests
test:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	@echo "✓ Tests complete"

# Run tests with coverage report
test-coverage: test
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

# Run lint
lint:
	golangci-lint run

# Security scan with gosec
sec:
	@command -v gosec >/dev/null 2>&1 || { \
		echo "Installing gosec..."; \
		go install github.com/securego/gosec/v2/cmd/gosec@latest; \
	}
	gosec -conf .gosec.json ./...
	@echo "✓ Security scan complete"

# Format code
fmt:
	$(GOFMT) ./...
	$(TEMPL) fmt

# Vet code
vet:
	$(GOVET) ./...

# Install dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy
	npm install

# Download Tailwind CLI standalone
download-tailwind:
	curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/download/v4.1.18/tailwindcss-linux-x64
	chmod +x tailwindcss-linux-x64
	@echo "✓ Tailwind CLI downloaded"

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -f $(TAILWIND_OUTPUT)
	@echo "✓ Clean complete"

# Database migrations (example)
db-migrate:
	@echo "Running database migrations..."
	$(GORUN) $(BUILD_FLAGS) ./scripts/ops/migrate.go up sqlite3 ./storage/goth.db

db-rollback:
	@echo "Rolling back database migrations..."
	$(GORUN) $(BUILD_FLAGS) ./scripts/ops/migrate.go down sqlite3 ./storage/goth.db

# Seed database with default data
seed:
	@echo "Seeding database..."
	$(GORUN) $(BUILD_FLAGS) $(CMD_PATH) seed
	@echo "✓ Database seeded with admin@admin.com / admin123"

# Create a new user
# Usage: make create-user email=seu@email.com password=suaSenha
create-user:
ifndef email
	@echo "Error: email is required"
	@echo "Usage: make create-user email=seu@email.com password=suaSenha"
	@exit 1
endif
ifndef password
	@echo "Error: password is required"
	@echo "Usage: make create-user email=seu@email.com password=suaSenha"
	@exit 1
endif
	@echo "Creating user $(email)..."
	$(GORUN) $(BUILD_FLAGS) $(CMD_PATH) create-user $(email) $(password)
	@echo "✓ User $(email) created"

# Reset database (clean and seed)
reset-db: clean-db seed
	@echo "✓ Database reset complete"

# Clean database
clean-db:
	@echo "Cleaning database..."
	rm -f $(DB_PATH)
	@echo "✓ Database cleaned"

# Dev: run server with watch
dev:
	@echo "Starting development server..."
	$(MAKE) watch

# Full dev: run server + watch css
dev-all:
	@echo "Starting full development environment..."
	$(MAKE) watch &
	$(MAKE) watch-css

# Docker: start all services
docker-up:
	docker-compose up -d
	@echo "✓ Docker containers started"

# Docker: stop all services
docker-down:
	docker-compose down
	@echo "✓ Docker containers stopped"

# Docker: view logs
docker-logs:
	docker-compose logs -f

# Docker: rebuild and restart
docker-restart:
	docker-compose down
	docker-compose build --no-cache
	docker-compose up -d
	@echo "✓ Docker containers rebuilt and restarted"

# Open coverage report
coverage: test
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report generated: coverage.html"

# Help
help:
	@echo "GOTH Stack - Available commands:"
	@echo ""
	@echo "  build           - Build the application"
	@echo "  build-prod      - Build for production (static binary)"
	@echo "  run             - Run the application"
	@echo "  dev             - Run server with hot reload"
	@echo "  dev-all         - Run server + watch CSS"
	@echo "  generate        - Generate templates and code"
	@echo "  css             - Build CSS with Tailwind"
	@echo "  watch-css       - Watch CSS for changes"
	@echo "  test            - Run tests"
	@echo "  coverage        - Run tests + generate HTML report"
	@echo "  lint            - Run linter"
	@echo "  sec             - Run security scan with gosec"
	@echo "  fmt             - Format code"
	@echo "  vet             - Vet code"
	@echo "  deps            - Install dependencies"
	@echo "  download-tailwind - Download Tailwind CLI standalone"
	@echo "  clean           - Clean build artifacts"
	@echo "  seed            - Seed database with default data"
	@echo "  create-user     - Create new user (email=x password=y)"
	@echo "  reset-db        - Clean and seed database"
	@echo "  db-migrate      - Run database migrations"
	@echo "  db-rollback     - Rollback database migrations"
	@echo "  docker-up       - Start Docker containers"
	@echo "  docker-down     - Stop Docker containers"
	@echo "  docker-logs     - View Docker logs"
	@echo "  docker-restart  - Rebuild and restart Docker"
	@echo ""
