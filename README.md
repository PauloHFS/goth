# GOTH Stack Boilerplate

Arquitetura monolítica de alta performance baseada em Go, SQLite, Templ e HTMX.

> 📚 **Documentação completa:** [Wiki](https://github.com/PauloHFS/goth.wiki)

---

## Quick Start

```bash
# Clone
git clone https://github.com/PauloHFS/goth.git
cd goth

# Setup
go mod download && npm install
go tool lefthook install

# Run
make dev
```

Acesse: http://localhost:8080

---

## O GOTH Way

Cinco pilares fundamentais:

1. **Go como Fonte da Verdade** - Backend é a fonte da verdade
2. **SSR Interativo (Templ + HTMX)** - Mínimo JavaScript no client
3. **SQLite Hardened** - WAL mode, otimizado para produção
4. **Single Binary** - Tudo embedado via `go:embed`
5. **Resiliência Operacional** - Jobs idempotentes + Dead Letter Queue

---

## Stack

| Componente | Tecnologia |
|------------|------------|
| Linguagem | Go 1.25+ |
| Banco de Dados | SQLite (WAL, FTS5, FK) |
| Frontend | Templ + HTMX |
| Estilos | Tailwind CSS v4 |
| Tempo Real | SSE |
| Jobs | Custom Worker + DLQ |
| Observabilidade | Prometheus + Grafana + Loki + Tempo |

---

## Estrutura

```
├── cmd/api/                # Entry point
├── internal/
│   ├── features/           # Feature modules (auth, billing, jobs...)
│   ├── platform/           # Shared infra (config, http, logging...)
│   └── view/               # Templ templates
├── migrations/             # DB migrations
├── test/                   # Tests (unit, integration, e2e, bench)
└── web/static/             # Static assets
```

---

## Comandos Principais

```bash
make dev              # Dev server com hot-reload
make build            # Build production
make test             # Run tests
make lint             # Run linter
make migrate          # Run migrations
make db-seed          # Seed database
make bench            # Run benchmarks
```

---

## Configuração

```bash
cp .env.example .env
# Edite .env com seus segredos
```

**Variáveis obrigatórias (produção):**
- `SESSION_SECRET` - `openssl rand -hex 32`
- `SMTP_USER/SMTP_PASS` - SMTP credentials
- `ASAAS_API_KEY` - Asaas payment API key

**Ambientes:** `APP_ENV=dev` (default), `staging`, `prod`

---

## CLI

```bash
./goth server         # Start server
./goth migrate        # Run migrations
./goth seed           # Seed database
./goth create-user <email> <password>
```

---

## Production-Ready

✅ HTTP Server Timeouts (Slowloris protection)
✅ Request ID + Log Correlation
✅ Circuit Breaker (OAuth, SMTP)
✅ Dead Letter Queue
✅ OpenTelemetry Tracing
✅ Prometheus Metrics + Grafana
✅ Structured Logging (slog)
✅ Backup (Litestream + S3)
✅ Health Checks

---

## Links Úteis

| Recurso | URL |
|---------|-----|
| **Wiki** | https://github.com/PauloHFS/goth.wiki |
| Swagger | http://localhost:8080/swagger |
| Health | http://localhost:8080/health |
| Metrics | http://localhost:8080/metrics |
| Grafana | http://localhost:3001 |
| Tempo | http://localhost:3200 |

---

## Licença

MIT License
