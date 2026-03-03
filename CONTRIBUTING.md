# Contributing to GOTH

Thank you for your interest in contributing to GOTH! This document provides guidelines and instructions for contributing to this monolithic architecture built with Go, SQLite, Templ, and HTMX.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Initial Setup](#initial-setup)
- [Development Workflow](#development-workflow)
  - [Git Hooks](#git-hooks)
  - [Code Generation](#code-generation)
- [Code Style & Conventions](#code-style--conventions)
  - [Go Code](#go-code)
  - [Frontend Code](#frontend-code)
- [Testing Guidelines](#testing-guidelines)
  - [Unit Tests](#unit-tests)
  - [Integration Tests](#integration-tests)
  - [E2E Tests](#e2e-tests)
  - [Benchmarks](#benchmarks)
- [Pull Request Process](#pull-request-process)
  - [PR Checklist](#pr-checklist)
- [Architecture Overview](#architecture-overview)
- [Common Tasks](#common-tasks)

## Code of Conduct

Please be respectful and constructive in your interactions. We welcome contributions from developers of all experience levels.

## Getting Started

### Prerequisites

Before contributing, ensure you have the following installed:

- **Go 1.25+** - [Download](https://go.dev/doc/install)
- **Node.js 20+** (with npm) - [Download](https://nodejs.org/)
- **Git** - Version control system

### Initial Setup

1. **Fork and clone the repository:**
   ```bash
   git clone https://github.com/YOUR_USERNAME/goth.git
   cd goth
   ```

2. **Install dependencies:**
   ```bash
   go mod download
   npm install
   ```

3. **Set up Git hooks:**
   ```bash
   go tool lefthook install
   ```

4. **Configure environment variables:**
   ```bash
   cp .env.example .env
   # Edit .env with your local configuration
   ```

5. **Initialize the development environment:**
   ```bash
   make dev
   ```

   This command:
   - Generates code (Templ, SQLC, Swagger)
   - Compiles Tailwind CSS assets
   - Resets the local database with seed data
   - Starts the server with hot-reload

## Development Workflow

### Git Hooks

This project uses **Lefthook** to enforce code quality before commits and pushes. Hooks are configured in `lefthook.yml` and run automatically:

- **Pre-commit:** Code formatting, linting, code generation
- **Pre-push:** Tests, vulnerability checks, build verification

To manually run hooks:
```bash
go tool lefthook run pre-commit
go tool lefthook run pre-push
```

### Code Generation

GOTH uses code generation for type-safe components. Always run before committing:

```bash
make generate
```

This executes:
- `templ generate` - Server-side rendered components
- `sqlc generate` - Type-safe database queries
- `swag init` - OpenAPI documentation
- Tailwind CSS compilation

**Important:** Generated files must be committed to the repository.

## Code Style & Conventions

### Go Code

**Formatting:**
- Use `gofmt` for all Go files (enforced by CI)
- Run: `gofmt -s -w .`

**Linting:**
- Project uses `golangci-lint` with custom configuration (`.golangci.yml`)
- Run: `make lint` or `golangci-lint run --timeout=5m`

**Enabled linters:**
- `errcheck` - Unchecked errors
- `unused` - Unused constants, variables, functions
- `ineffassign` - Ineffective assignments
- `staticcheck` - Go's more advanced `vet`
- `govet` - Standard Go vet
- `typecheck` - Type checking

**Code organization:**
- Follow standard Go project layout
- Keep business logic in `internal/`
- Handlers in `internal/web/handlers/`
- Database operations in `internal/db/`
- Middleware in `internal/middleware/`

**Error handling:**
- Always check errors explicitly
- Wrap errors with context using `fmt.Errorf("%w", err)`
- Avoid generic error messages

**Example:**
```go
user, err := db.GetUserByID(ctx, id)
if err != nil {
    return fmt.Errorf("get user %d: %w", id, err)
}
```

### Frontend Code

**Templ Components:**
- Keep components small and focused
- Use type-safe parameters
- Avoid inline JavaScript

**HTMX:**
- Prefer `hx-get`, `hx-post` for interactions
- Use `hx-swap` for partial updates
- Keep HTML semantic

**Tailwind CSS:**
- Use utility classes directly in templates
- Follow mobile-first approach
- Avoid custom CSS when possible

**Alpine.js:**
- Use for client-side state only when necessary
- Keep logic minimal (GOTH philosophy: Go as source of truth)

## Testing Guidelines

GOTH maintains a rigorous testing structure. All contributions must include appropriate tests.

### Security Testing

**Run security scans:**
```bash
make sec
```

This runs `gosec` with project-specific configuration (`.gosec.json`).

**Security requirements:**
- No high-severity issues in new code
- Path traversal prevention for file operations
- Request body size limits for form handlers
- Secure file permissions (0600 for files, 0750 for directories)

**Example security fixes:**
```go
// Limit request body size to prevent memory exhaustion
r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB

// Prevent path traversal
cleanPath := filepath.Clean(userPath)
if !filepath.IsLocal(cleanPath) {
    return fmt.Errorf("invalid path")
}

// Use secure file permissions
os.WriteFile(path, data, 0600) // Owner read/write only
```

### Unit Tests

**Location:** Alongside source code (`*_test.go`)

**Purpose:** Test isolated logic within packages

**Run:**
```bash
make test
```

**Example:**
```go
func TestCalculateDiscount(t *testing.T) {
    result := CalculateDiscount(100, 0.2)
    if result != 80 {
        t.Errorf("expected 80, got %d", result)
    }
}
```

### Integration Tests

**Location:** `test/integration/`

**Purpose:** Test component interactions (Web + DB, Worker + Queue)

**Run:**
```bash
make test-integration
```

**Guidelines:**
- Use test databases (in-memory or temporary files)
- Clean up after tests
- Test real component interactions

### E2E Tests

**Location:** `test/e2e/`

**Purpose:** Full end-to-end user flow testing with Playwright

**Run:**
```bash
make test-e2e
```

**Server Management:**
E2E tests now automatically manage the test server lifecycle:
- Server is built and started before tests run
- Server is stopped after tests complete
- Test database is cleaned up automatically

**Requirements:**
- Playwright browsers installed: `npx playwright install --with-deps chromium`
- No need to start server manually (done automatically)

**Example:**
```go
func TestUserLogin(t *testing.T) {
    page := playwright.Browser.NewPage()
    page.Goto("http://localhost:8080/login")
    page.Fill("#email", "user@example.com")
    page.Fill("#password", "password123")
    page.Click("button[type=submit]")
    page.WaitForURL("/dashboard")
}
```

### Benchmarks

**Location:** `test/benchmarks/`

**Purpose:** Performance testing and regression detection

**Run:**
```bash
make bench
```

**Guidelines:**
- Use `-benchmem` for memory allocation stats
- Run with `-race` for race condition detection
- Compare results before and after changes

**Example:**
```go
func BenchmarkGetUser(b *testing.B) {
    for i := 0; i < b.N; i++ {
        _, err := db.GetUserByID(ctx, 1)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

### Test Coverage

**Generate coverage report:**
```bash
make test-cover
```

**Minimum coverage:** Aim for 80%+ on critical paths

## Pull Request Process

### Before Submitting

1. **Create a feature branch:**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes** following the code style guidelines

3. **Run all checks locally:**
   ```bash
   make generate
   make lint
   make sec        # Security scan
   make test
   make test-integration
   make bench
   ```

4. **Ensure build passes:**
   ```bash
   make build
   ```

5. **Commit with clear messages:**
   - Use present tense ("Add feature" not "Added feature")
   - Reference issues when applicable

### PR Checklist

Before submitting a pull request, ensure:

- [ ] Code follows project style guidelines (`make lint`)
- [ ] All generated files are up to date (`make generate`)
- [ ] Security scan passes (`make sec`)
- [ ] Unit tests pass (`make test`)
- [ ] Integration tests pass (`make test-integration`)
- [ ] E2E tests pass (if applicable) (`make test-e2e`)
- [ ] Benchmarks show no regression (if applicable) (`make bench`)
- [ ] Build succeeds (`make build`)
- [ ] No new vulnerabilities (`go tool govulncheck ./...`)
- [ ] Documentation updated (README, code comments)
- [ ] Changes are atomic and focused on a single concern

### PR Description Template

```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
Describe tests performed and results

## Checklist
- [ ] I have read the CONTRIBUTING.md document
- [ ] My code follows the project's style guidelines
- [ ] I have added/updated tests
- [ ] All tests pass locally
- [ ] I have updated documentation as needed
```

## Architecture Overview

### Project Structure

```
goth/
├── cmd/api/                 # Application entry point
├── docs/                    # OpenAPI specification
├── internal/
│   ├── cmd/                 # CLI commands
│   ├── contextkeys/         # Context keys type-safe
│   ├── db/                  # Database layer (SQLC)
│   ├── features/            # Feature modules
│   │   ├── admin/           # Admin dashboard
│   │   ├── auth/            # Authentication
│   │   ├── billing/         # Billing (Asaas)
│   │   ├── jobs/            # Worker queue + DLQ
│   │   ├── sse/             # Real-time SSE
│   │   └── user/            # User management
│   ├── platform/            # Shared infrastructure
│   │   ├── config/          # Configuration
│   │   ├── featureflags/    # Feature flags
│   │   ├── http/            # HTTP + middleware
│   │   ├── logging/         # Structured logging
│   │   ├── metrics/         # Prometheus metrics
│   │   └── secrets/         # Secret management
│   └── view/                # UI layer (Templ)
├── migrations/              # Database migrations
├── test/
│   ├── benchmarks/          # Performance benchmarks
│   ├── integration/         # Integration tests
│   └── e2e/                 # End-to-end tests
├── web/static/              # Static assets (CSS, JS, images)
└── storage/                 # File persistence (local)
```

### GOTH Philosophy

All contributions should align with the five pillars:

1. **Go as Source of Truth** - Business logic resides in backend
2. **SSR with Templ + HTMX** - Minimal client-side JavaScript
3. **SQLite Hardened** - WAL mode, optimized for vertical scaling
4. **Single Binary** - All assets embedded via `go:embed`
5. **Operational Resilience** - Idempotent jobs, zombie recovery

## Common Tasks

| Task | Command |
|------|---------|
| Start development server | `make dev` |
| Run all tests | `make test` |
| Run integration tests | `make test-integration` |
| Run E2E tests | `make test-e2e` |
| Run benchmarks | `make bench` |
| Generate code | `make generate` |
| Build binary | `make build` |
| Run linter | `make lint` |
| Run migrations | `make migrate` |
| Reset database | `make dev-reset` |
| Check coverage | `make test-cover` |

## Questions?

If you have questions, please open an issue for discussion before starting work on major features.

---

Thank you for contributing to GOTH!
