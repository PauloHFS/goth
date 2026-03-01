# GOTH Stack Boilerplate

Arquitetura monolítica de alta performance baseada em Go, SQLite, Templ e HTMX. Este projeto é focado em escalabilidade vertical, deploy simplificado através de binário único e eliminação de dependências de infraestrutura externa.

## O GOTH Way

A filosofia GOTH orienta o desenvolvimento deste sistema através de cinco pilares fundamentais:

1. Go como Fonte da Verdade: Toda a lógica de negócio, roteamento e validação residem exclusivamente no backend. O frontend atua como uma projeção do estado do servidor.
2. SSR Interativo (Templ + HTMX): Utilização de Templ para componentes type-safe e HTMX para interatividade, minimizando a necessidade de JavaScript no lado do cliente.
3. SQLite Hardened: O banco de dados é um arquivo local otimizado com modo WAL (Write-Ahead Logging), Busy Timeout e Sincronização Normal para garantir performance de nível de produção.
4. Single Binary: Assets, migrações de banco de dados, documentação API e o executável são consolidados em um único arquivo binário via go:embed.
5. Resiliência Operacional: Sistema de background jobs com rastreio de idempotência e recuperação automática de processos interrompidos (zombie recovery).

## Stack Tecnológico

| Componente | Tecnologia | Detalhe |
| --- | --- | --- |
| Linguagem | Go 1.25+ | Biblioteca padrão e arquitetura modular |
| Banco de Dados | SQLite | WAL Mode, FTS5, Foreign Keys |
| Interface | Templ | Server-Side Rendering type-safe |
| Interatividade | HTMX | Comunicação assíncrona servidor-cliente |
| Estilos | Tailwind CSS v4 | Compilação JIT nativa |
| Tempo Real | SSE Broker | Server-Sent Events direcionados por usuário |
| Fila de Jobs | Custom Engine | Persistência transacional em SQLite + Dead Letter Queue |
| Documentação | Swagger | Especificação OpenAPI 3.0 |

## Estrutura do Projeto

```
├── cmd/api/                # Ponto de entrada da aplicação
├── docs/                   # Especificação OpenAPI e Documentação
├── internal/
│   ├── cmd/                # Implementação dos comandos CLI
│   ├── contextkeys/        # Context keys type-safe
│   ├── db/                 # Camada de persistência (SQLC + migrations)
│   ├── features/           # Feature modules (coesão alta)
│   │   ├── admin/          # Admin dashboard
│   │   ├── auth/           # Auth (OAuth, email, password)
│   │   ├── billing/        # Asaas integration
│   │   ├── jobs/           # Worker queue + DLQ
│   │   │   └── worker/     # Worker processor
│   │   ├── sse/            # Real-time SSE broker
│   │   ├── user/           # User management
│   │   └── webhook/        # Webhook handling
│   ├── platform/           # Shared infrastructure
│   │   ├── config/         # Multi-environment config
│   │   ├── featureflags/   # Feature flags management
│   │   ├── http/           # Handlers + middleware chain
│   │   ├── httpclient/     # HTTP client com circuit breaker
│   │   ├── i18n/           # Internacionalização
│   │   ├── logging/        # Logging estruturado
│   │   ├── mailer/         # SMTP com circuit breaker
│   │   ├── metrics/        # Prometheus collectors
│   │   ├── observability/  # Audit + tracing
│   │   └── secrets/        # Secret management
│   ├── policies/           # Business policies
│   ├── routes/             # Route definitions
│   ├── validator/          # Validation wrapper
│   └── view/               # UI layer (Templ templates)
├── migrations/             # Esquema SQL idempotente
├── docker/                 # Infraestrutura Docker
│   ├── app/                # Dockerfile da aplicação
│   ├── traefik/            # Reverse Proxy & SSL
│   ├── otel/               # Observabilidade (Prometheus, Grafana, Loki, Tempo)
│   └── litestream/         # Backup SQLite
├── test/                   # Benchmarks, E2E e testes de integração
├── web/
│   └── static/             # Assets estáticos (CSS, JS, Imagens)
└── storage/                # Diretório de persistência de arquivos (local)
```

## Desenvolvimento

### Pré-requisitos

Para contribuir com este projeto, é necessário instalar as seguintes dependências em seu ambiente:

1. **Linguagens e Ambientes:**
   - [Go 1.25+](https://go.dev/doc/install)
   - [Node.js 20+](https://nodejs.org/) (incluindo NPM)

2. **Gerenciamento de Ferramentas:**
   Este projeto utiliza o sistema de `tool` do Go 1.24+. As ferramentas necessárias (`templ`, `sqlc`, `swag`, `govulncheck`, `air`, `lefthook`) são gerenciadas automaticamente através do arquivo `go.mod`.

   Para executar qualquer ferramenta manualmente, utilize:
   ```bash
   go tool <ferramenta> [argumentos]
   ```

### Procedimento Inicial

1. Instalação de dependências do Go e NPM:
```bash
go mod download
npm install
```

2. Ativação dos Git Hooks:
```bash
go tool lefthook install
```

3. Inicialização do ambiente de desenvolvimento:
```bash
make dev
```
Este comando executa a geração de código (`templ`, `sqlc`, `swag`), compilação de assets Tailwind, reset do banco de dados local com sementes (`seed`) e inicia o servidor com hot-reload.

## CLI Console

O binário gerado atua como uma ferramenta de linha de comando para operações administrativas:

- `server`: Inicia o servidor web (operação padrão)
- `migrate`: Executa migrações pendentes no banco de dados
- `seed`: Popula o banco com dados de teste
- `create-user`: Registra manualmente um usuário (args: `<email> <password>`)
- `help`: Exibe a lista de comandos disponíveis

## Funcionalidades Production-Ready

- HTTP Server Timeouts (Slowloris protection)
- Request ID + Log Correlation
- Centralized Input Validation
- RFC 7807 Error Responses
- Hybrid Rate Limiting (Traefik + App)
- Circuit Breaker (Google OAuth, SMTP)
- Dead Letter Queue (failed jobs)
- Backup Verification Script
- Migration Rollback Tests (CI)
- OpenTelemetry Tracing
- Prometheus Metrics + Grafana Dashboards
- Structured Logging (slog)
- Security Scanning (gosec + govulncheck)
- Alertmanager (Slack, PagerDuty, Email)

## Observabilidade

### Métricas (Prometheus)
- HTTP requests/latency
- Job processing
- Circuit breaker state
- Database connections
- Business metrics (auth, billing)

### Dashboards (Grafana)
- http://localhost:3001 (dev)
- Dashboards provisionados automaticamente
- Logs (Loki), Traces (Tempo), Metrics (Prometheus)

### Alertas (Alertmanager)
- 18 alertas configurados (error rate, latency, queue, disk, etc.)
- Notificações via Slack, PagerDuty ou Email
- Runbook: `docs/RUNBOOK.md`

### Tracing (Tempo)
- http://localhost:3200
- Trace ID correlation com logs

### Comandos
```bash
# Iniciar stack de observabilidade
make telemetry-up

# Abrir Grafana
make grafana-open

# Abrir Tempo (traces)
make tempo-open

# Ver logs
docker-compose logs -f otelcol loki tempo prometheus grafana
```

## Infraestrutura e Observabilidade

- **Health Check:** `GET /health` - Monitoramento de conectividade de banco, latência de jobs e espaço em disco.
- **Métricas:** `GET /metrics` - Exposição de coletores nativos para Prometheus.
- **API Docs:** `GET /swagger/index.html` - Documentação interativa das rotas do sistema.

## Configuração

O GOTH utiliza uma separação clara entre **segredos** (`.env`) e **configuração estrutural** (`config.yaml`):

### Arquitetura de Configuração

```
┌─────────────────────────────────────────────────────────────┐
│                      .env (Segredos)                        │
│  SESSION_SECRET, SMTP_USER/PASS, API Keys, OAuth Secrets   │
│  NUNCA commitar no git - copie de .env.example             │
└─────────────────────────────────────────────────────────────┘
                            ↓ (injeta no container/processo)
┌─────────────────────────────────────────────────────────────┐
│                   config.yaml (Estrutura)                    │
│  - environments: { dev, staging, prod }                     │
│  - Cada ambiente: server, database, logging, smtp, etc.     │
│  - Interpola variáveis: ${SESSION_SECRET}, ${SMTP_HOST}     │
└─────────────────────────────────────────────────────────────┘
```

### `.env` - Apenas Segredos

Copie o arquivo base e preencha **apenas os segredos**:

```bash
cp .env.example .env
```

| Variável | Obrigatório | Descrição |
|----------|-------------|-----------|
| `SESSION_SECRET` | Producao | Chave para assinatura de sessão. Gere com: `openssl rand -hex 32` |
| `SMTP_USER` | Producao | Usuário do servidor SMTP |
| `SMTP_PASS` | Producao | Senha do servidor SMTP |
| `GOOGLE_CLIENT_ID` | Opcional | Client ID do OAuth Google |
| `GOOGLE_CLIENT_SECRET` | Opcional | Client Secret do OAuth Google |
| `ASAAS_API_KEY` | Producao | API Key do Asaas (pagamentos) |
| `ASAAS_WEBHOOK_TOKEN` | Producao | Token para validar webhooks do Asaas |
| `ASAAS_HMAC_SECRET` | Recomendado | Secret para assinatura HMAC |

### `config.yaml` - Configuração Estrutural

Este arquivo contém **toda configuração por ambiente**:

- **`defaults`**: Configurações globais (URLs, intervals, integrações)
- **`environments.dev`**: Desenvolvimento local (debug, swagger, rate limit alto)
- **`environments.staging`**: Homologação (espelha produção)
- **`environments.prod`**: Produção (otimizado, seguro, sem swagger)

**Interpolação de variáveis**: Use `${VAR}` para injetar segredos do `.env`:
```yaml
session:
  secret: "${SESSION_SECRET}"  # injetado via env
smtp:
  host: "${SMTP_HOST:-localhost}"  # com default
```

### Seleção de Ambiente

O ambiente é selecionado via `APP_ENV` (default: `dev`):

```bash
# Desenvolvimento (default)
./goth server

# Produção
APP_ENV=prod ./goth server

# Docker
docker run -e APP_ENV=prod -e SESSION_SECRET=xxx goth:latest
```

### Configuração Obrigatória em Produção

| Variável | Obrigatório | Descrição |
|----------|-------------|-----------|
| `SESSION_SECRET` | Produção | Chave para assinatura de sessão. Gere com: `openssl rand -hex 32` |
| `SMTP_USER` | Produção | Usuário do servidor SMTP |
| `SMTP_PASS` | Produção | Senha do servidor SMTP |
| `ASAAS_API_KEY` | Produção | API Key do Asaas (pagamentos) |
| `ASAAS_WEBHOOK_TOKEN` | Produção | Token para validar webhooks do Asaas |
| `GOOGLE_CLIENT_ID/SECRET` | Opcional | OAuth Google (se utilizado) |

## Qualidade e Automação

### Guia de Testes

O projeto segue uma estrutura de testes rigorosa para garantir estabilidade e performance:

1. **Testes Unitários:** Localizados junto ao código fonte (`*_test.go`). Focam em lógica isolada de pacotes.
   ```bash
   make test
   ```
2. **Testes de Integração:** Testam a comunicação entre múltiplos componentes (ex: Web + DB).
3. **Benchmarks de Performance:** Localizados em `test/benchmarks/`. Medem latência, alocação de memória e concorrência.
   ```bash
   make bench
   ```
4. **Cobertura de Código:**
   ```bash
   make test-cover
   ```

O fluxo de trabalho é protegido por:

- **CI (GitHub Actions):** Execução automatizada de testes unitários, benchmarks de performance, análise de vulnerabilidades, linting e verificação de integridade de código gerado (Drift Check).
- **Git Hooks (Lefthook):** Executa formatação, geração de código e testes locais antes de permitir a subida de código ao repositório remoto.
