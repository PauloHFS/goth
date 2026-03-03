# 🚀 GOTH Stack - Full-Stack Boilerplate

**Go + Templ + HTMX + Tailwind CSS**

Um boilerplate full-stack moderno, minimalista e production-ready para desenvolvimento rápido de MVPs com escalabilidade imediata.

[![CI/CD Pipeline](https://github.com/PauloHFS/goth/actions/workflows/ci-cd.yml/badge.svg)](https://github.com/PauloHFS/goth/actions/workflows/ci-cd.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/PauloHFS/goth)](https://goreportcard.com/report/github.com/PauloHFS/goth)
[![Go Version](https://img.shields.io/badge/go-1.25-blue.svg)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

---

## 📋 Índice

- [Features](#-features)
- [Stack Tecnológico](#-stack-tecnológico)
- [Quick Start](#-quick-start)
- [Estrutura do Projeto](#-estrutura-do-projeto)
- [Desenvolvimento](#-desenvolvimento)
- [Produção](#-produção)
- [Testes](#-testes)
- [Documentação](#-documentação)
- [Contributing](#-contributing)
- [License](#-license)

---

## ✨ Features

### Core
- ✅ **Go 1.25+** - Backend performático e type-safe
- ✅ **Templ** - SSR components type-safe
- ✅ **HTMX** - Interatividade sem JavaScript complexo
- ✅ **Alpine.js** - JavaScript reativo leve
- ✅ **Tailwind CSS v4** - Estilização utilitária

### Segurança
- ✅ **Autenticação OAuth2** (Google, GitHub)
- ✅ **2FA/TOTP** - Autenticação de dois fatores
- ✅ **RBAC** - Controle de acesso baseado em roles (Casbin)
- ✅ **CSRF Protection** - Proteção contra CSRF attacks
- ✅ **Session Timeout** - Timeout por inatividade
- ✅ **CSP Headers** - Content Security Policy

### Observabilidade
- ✅ **OpenTelemetry** - Tracing distribuído
- ✅ **Grafana** - Dashboards e métricas
- ✅ **Loki** - Agregação de logs
- ✅ **Tempo** - Armazenamento de traces
- ✅ **Sentry** - Error tracking (frontend + backend)
- ✅ **Web Vitals** - Performance monitoring

### DX (Developer Experience)
- ✅ **Taskfile** - Task runner unificado
- ✅ **Hot Reload** (Air)
- ✅ **golangci-lint** - Linting automatizado
- ✅ **gosec** - Security scanning
- ✅ **govulncheck** - Vulnerability checking

### Production Ready
- ✅ **Docker** - Containerização
- ✅ **SQLite WAL** - Performance otimizada
- ✅ **Litestream** - Backup contínuo S3
- ✅ **Health Checks** - Monitoramento de saúde
- ✅ **Graceful Shutdown** - Shutdown elegante

---

## 🛠️ Stack Tecnológico

| Categoria | Tecnologia | Versão |
|-----------|------------|--------|
| **Backend** | Go | 1.25+ |
| **Database** | SQLite (WAL, FTS5, FK) | 3.x |
| **SSR** | Templ | 0.3.x |
| **Interatividade** | HTMX | 2.x |
| **JavaScript** | Alpine.js | 3.x |
| **Estilização** | Tailwind CSS | 4.x |
| **Auth** | OAuth2, 2FA/TOTP | - |
| **RBAC** | Casbin | 2.x |
| **Tracing** | OpenTelemetry | 1.x |
| **Metrics** | Prometheus + Grafana | - |
| **Logs** | Loki | - |
| **Error Tracking** | Sentry | - |
| **Backup** | Litestream | - |

---

## 🚀 Quick Start

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

## 📁 Estrutura do Projeto

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
│   │   ├── http/         # HTTP handlers
│   │   ├── logging/      # Logging
│   │   ├── metrics/      # Metrics
│   │   ├── observability/# OTel, Grafana
│   │   ├── security/     # RBAC, 2FA
│   │   └── seo/          # Sitemap, Robots.txt
│   └── view/             # Templ templates
├── migrations/            # Database migrations
├── web/
│   ├── components/       # UI components
│   ├── static/           # Static assets
│   └── lib/              # JavaScript libraries
├── test/                 # Test files
├── docker/               # Docker configs
├── docs/                 # Documentation
└── storage/              # File storage
```

---

## 💻 Desenvolvimento

### Comandos Úteis

```bash
# Build
task build

# Run dev server
task dev

# Run tests
task test

# Run linters
task lint

# Generate code (Templ, SQLC, Swagger)
task gen

# Security scan
task security:check

# Database
task db:migrate
task db:backup
```

### Hot Reload

O projeto usa [Air](https://github.com/air-verse/air) para hot reload:

```bash
air -c .air.toml
```

### Generate Code

```bash
# Templ components
templ generate

# SQLC types
sqlc generate

# Swagger docs
swag init -g internal/cmd/server.go -o docs
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

## 🧪 Testes

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

---

## 📚 Documentação

- [ANALISE_ARQUITETURAL.md](ANALISE_ARQUITETURAL.md) - Análise arquitetural completa
- [IMPLEMENTATION_PROGRESS.md](IMPLEMENTATION_PROGRESS.md) - Progresso de implementação
- [QUICK_START.md](QUICK_START.md) - Guia rápido
- [THEME_SYSTEM.md](THEME_SYSTEM.md) - Sistema de temas
- [ICON_SYSTEM.md](ICON_SYSTEM.md) - Sistema de ícones
- [DESIGN_ANALYSIS.md](DESIGN_ANALYSIS.md) - Análise de design

---

## 🤝 Contributing

1. Fork o projeto
2. Crie uma branch (`git checkout -b feature/AmazingFeature`)
3. Commit (`git commit -m 'Add AmazingFeature'`)
4. Push (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

### Setup do Ambiente

```bash
# Clone
git clone https://github.com/PauloHFS/goth.git
cd goth

# Install tools
task install:tools

# Setup
task setup
```

---

## 📄 License

Distribuído sob a licença MIT. Veja [LICENSE](LICENSE) para mais informações.

---

## 👥 Autores

- **Paulo Hernane** - [@PauloHFS](https://github.com/PauloHFS)

---

## 🙏 Agradecimentos

- [Go](https://golang.org/)
- [Templ](https://templ.guide/)
- [HTMX](https://htmx.org/)
- [Tailwind CSS](https://tailwindcss.com/)
- [Alpine.js](https://alpinejs.dev/)

---

## 📊 Stats

![Alt](https://repobeats.axiom.co/api/embed/xxx.svg "Repobeats analytics image")

---

**⭐ Se você gosta deste projeto, considere dar uma estrela!**
