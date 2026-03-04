# GOTH Stack - Full-Stack Boilerplate

**Go + Templ + HTMX + Tailwind CSS**

A modern, minimalist, production-ready full-stack boilerplate for rapid MVP development with immediate scalability.

[![CI/CD Pipeline](https://github.com/PauloHFS/goth/actions/workflows/ci-cd.yml/badge.svg)](https://github.com/PauloHFS/goth/actions/workflows/ci-cd.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/PauloHFS/goth)](https://goreportcard.com/report/github.com/PauloHFS/goth)
[![Go Version](https://img.shields.io/badge/go-1.25-blue.svg)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

---

## Table of Contents

- [Features](#features)
- [Technology Stack](#technology-stack)
- [Quick Start](#quick-start)
- [Project Structure](#project-structure)
- [Development](#development)
- [Production](#production)
- [Testing](#testing)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

---

## Features

### Core
- **Go 1.25+** - Performant and type-safe backend
- **Templ** - Type-safe SSR components
- **HTMX** - Interactivity without complex JavaScript
- **Alpine.js** - Lightweight reactive JavaScript
- **Tailwind CSS v4** - Utility-first styling

### Security
- **OAuth2 Authentication** (Google, GitHub)
- **2FA/TOTP** - Two-factor authentication
- **RBAC** - Role-based access control (Casbin)
- **CSRF Protection** - CSRF attack prevention
- **Session Timeout** - Inactivity timeout
- **CSP Headers** - Content Security Policy

### Observability
- **OpenTelemetry** - Distributed tracing
- **Grafana** - Dashboards and metrics
- **Loki** - Log aggregation
- **Tempo** - Trace storage
- **Sentry** - Error tracking (frontend + backend)
- **Web Vitals** - Performance monitoring

### Developer Experience
- **Taskfile** - Unified task runner
- **Hot Reload** (Air)
- **golangci-lint** - Automated linting
- **gosec** - Security scanning
- **govulncheck** - Vulnerability checking

### Production Ready
- **Docker** - Containerization
- **SQLite WAL** - Optimized performance
- **Litestream** - Continuous S3 backup
- **Health Checks** - Health monitoring
- **Graceful Shutdown** - Elegant shutdown

### API
- **REST API v1** - Versioned API at /api/v1/
- **WebSockets** - Real-time bidirectional communication
- **SSE** - Server-Sent Events

---

## Technology Stack

| Category | Technology | Version |
|----------|------------|---------|
| **Backend** | Go | 1.25+ |
| **Database** | SQLite (WAL, FTS5, FK) | 3.x |
| **SSR** | Templ | 0.3.x |
| **Interactivity** | HTMX | 2.x |
| **JavaScript** | Alpine.js | 3.x |
| **Styling** | Tailwind CSS | 4.x |
| **WebSockets** | gorilla/websocket | 1.5.x |
| **Auth** | OAuth2, 2FA/TOTP | - |
| **RBAC** | Casbin | 2.x |
| **Tracing** | OpenTelemetry | 1.x |
| **Metrics** | Prometheus + Grafana | - |
| **Logs** | Loki | - |
| **Error Tracking** | Sentry | - |
| **Backup** | Litestream | - |

---

## Quick Start

### Pré-requisitos

- Go 1.25+
- Node.js 20+
- Docker (opcional)
- Task (opcional)

### 1. Clone o repositório

```bash
git clone https://github.com/PauloHFS/goth.git
cd goth
```

### 2. Instale dependências

```bash
# Go dependencies
go mod download

# Node dependencies
npm install

# Dev tools (opcional)
task install:tools
```

### 3. Configure variáveis de ambiente

```bash
cp .env.example .env
# Edite .env com suas credenciais
# Opcional: configure CORS_ALLOWED_ORIGINS paraAllow origins
```

### 4. Rode migrações

```bash
go run github.com/pressly/goose/v3/cmd/goose -dir migrations up
```

### 5. Inicie o servidor de desenvolvimento

```bash
# Com Task
task dev

# Ou manualmente
air -c .air.toml
```

### 6. Acesse a aplicação

```
http://localhost:8080
```

---

## Estrutura do Projeto

```
goth/
├── cmd/                    # Application entry point
├── internal/
│   ├── cmd/               # Server setup
│   ├── db/                # Database layer
│   ├── features/          # Feature modules
│   │   ├── auth/         # Authentication
│   │   ├── billing/      # Billing (Asaas)
│   │   ├── jobs/         # Background jobs
│   │   └── user/         # User management
│   ├── platform/          # Shared platform
│   │   ├── config/       # Configuration
│   │   ├── featureflags/ # Feature flags (weak.Pointer cache)
│   │   ├── http/         # HTTP handlers + middleware
│   │   │   └── middleware/
│   │   │       ├── cors.go      # CORS (configurable via env)
│   │   │       ├── security.go  # Security headers
│   │   │       └── ...
│   │   ├── logging/      # Logging
│   │   ├── metrics/      # Metrics
│   │   ├── observability/# OTel, Grafana
│   │   ├── secrets/     # Secret management
│   │   ├── security/    # RBAC, 2FA
│   │   ├── seo/         # Sitemap, Robots.txt
│   │   └── websocket/    # WebSocket hub
│   └── view/             # Templ templates
├── migrations/            # Database migrations
├── web/
│   ├── components/       # UI components
│   ├── static/           # Static assets
│   └── lib/              # JavaScript libraries
├── test/                 # Test files
├── docker/               # Docker configs
├── docs/                 # Documentation
├── storage/              # File storage
└── LICENSE               # MIT License
```

---

## 🚢 Produção

### Docker Build

```bash
# Build image
docker build -t goth:latest .

# Run container
docker run -p 8080:8080 \
  -e ENV=prod \
  -e DATABASE_URL=/data/goth.db \
  -v $(pwd)/data:/data \
  goth:latest
```

### Docker Compose

```bash
# Development
docker-compose -f docker-compose.dev.yml up -d

# Production
docker-compose up -d
```

### Backup (Litestream)

```bash
# Configure .env
LITESTREAM_ACCESS_KEY_ID=your-key
LITESTREAM_SECRET_ACCESS_KEY=your-secret
LITESTREAM_BUCKET=your-bucket

# Run Litestream
litestream replicate goth.db s3://your-bucket/goth.db
```

---

## Testes

```bash
# Unit tests
go test ./... -short -v

# Integration tests
go test ./... -run Integration -v

# Coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# E2E (Playwright)
npx playwright test
```