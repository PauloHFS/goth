#!/bin/bash
# ===========================================
# GOTH Stack - Rebranding Script
# ===========================================
# Script para atualizar imports e nomes ao fazer fork do boilerplate.
#
# Uso:
#   ./scripts/rebrand.sh <owner> <project> [domain]
#
# Exemplos:
#   ./scripts/rebrand.sh mycompany myapp
#   ./scripts/rebrand.sh john myproject myproject.com
#
# O script:
# 1. Cria backup automático (stash git)
# 2. Atualiza todos os imports Go
# 3. Atualiza go.mod e go.sum
# 4. Atualiza config.yaml, README, docs
# 5. Atualiza arquivos Docker e workflow
# 6. Roda go mod tidy
# 7. Regenera código (make generate)
#
# Rollback:
#   git stash pop
# ===========================================

set -e

# Cores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Função de log
log() {
    echo -e "${BLUE}>>>${NC} $1"
}

success() {
    echo -e "${GREEN}✓${NC} $1"
}

warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

error() {
    echo -e "${RED}✗${NC} $1"
}

# Validação de argumentos
if [ $# -lt 2 ]; then
    echo ""
    echo "Uso: $0 <owner> <project> [domain]"
    echo ""
    echo "Argumentos:"
    echo "  owner    - Seu usuário/organização no GitHub"
    echo "  project  - Nome do novo projeto"
    echo "  domain   - Domínio opcional (default: project.com)"
    echo ""
    echo "Exemplos:"
    echo "  $0 mycompany myapp"
    echo "  $0 john myproject myproject.com"
    echo ""
    exit 1
fi

OWNER=$1
PROJECT=$2
DOMAIN=${3:-${PROJECT}.com}

# Valores originais
OLD_OWNER="PauloHFS"
OLD_PROJECT="goth"
OLD_MODULE="github.com/${OLD_OWNER}/${OLD_PROJECT}"
NEW_MODULE="github.com/${OWNER}/${PROJECT}"

# Converter para formato legível (ex: my-app -> MyApp)
PROJECT_TITLE=$(echo "$PROJECT" | sed -r 's/(^|-)([a-z])/\U\2/g')

echo ""
echo "============================================"
echo "  🎨 GOTH Stack Rebranding"
echo "============================================"
echo ""
log "Owner:   ${OLD_OWNER} → ${OWNER}"
log "Project: ${OLD_PROJECT} → ${PROJECT}"
log "Domain:  goth.com → ${DOMAIN}"
log "Module:  ${OLD_MODULE}"
log "         → ${NEW_MODULE}"
echo ""

# Confirmação
read -p "Deseja continuar? (isso criará um backup automático) [y/N] " -n 1 -r
echo ""
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    warn "Operação cancelada."
    exit 0
fi

# ===========================================
# 1. Backup
# ===========================================
log "Criando backup..."

# Verifica se há mudanças não commitadas
if ! git diff --quiet 2>/dev/null || ! git diff --cached --quiet 2>/dev/null; then
    warn "Há mudanças não commitadas. Por favor, commit ou stash antes de continuar."
    exit 1
fi

# Criar stash backup
git add -A 2>/dev/null || true
git stash push -m "pre-rebrand-backup" 2>/dev/null || true
success "Backup criado (git stash)"

# ===========================================
# 2. Atualizar imports Go
# ===========================================
log "Atualizando imports Go..."

# Encontrar e substituir em arquivos .go
GO_FILES=$(find . -type f -name "*.go" ! -path "./vendor/*" ! -path "./.git/*" 2>/dev/null | wc -l)
find . -type f -name "*.go" ! -path "./vendor/*" ! -path "./.git/*" -exec sed -i.bak \
    "s|github.com/${OLD_OWNER}/${OLD_PROJECT}|github.com/${OWNER}/${PROJECT}|g" {} +

# Remover backups .bak
find . -type f -name "*.bak" ! -path "./vendor/*" ! -path "./.git/*" -delete 2>/dev/null || true
success "Imports Go atualizados (${GO_FILES} arquivos)"

# ===========================================
# 3. Atualizar go.mod e go.sum
# ===========================================
log "Atualizando go.mod..."

if [ -f "go.mod" ]; then
    sed -i.bak "s|module ${OLD_MODULE}|module ${NEW_MODULE}|" go.mod
    rm -f go.mod.bak
    success "go.mod atualizado"
else
    warn "go.mod não encontrado"
fi

# ===========================================
# 4. Atualizar config.yaml
# ===========================================
log "Atualizando config.yaml..."

if [ -f "config.yaml" ]; then
    cp config.yaml config.yaml.bak
    
    # Atualizar nomes do projeto
    sed -i.bak "s/GOTH Stack/${PROJECT_TITLE} Stack/g" config.yaml
    sed -i.bak "s/GOTH/${PROJECT_TITLE}/g" config.yaml
    
    # Atualizar nomes de banco de dados
    sed -i.bak "s/goth\.db/${PROJECT}.db/g" config.yaml
    
    # Atualizar emails
    sed -i.bak "s/@goth\.com/@${DOMAIN}/g" config.yaml
    sed -i.bak "s/noreply@staging\.goth\.com/noreply@staging.${DOMAIN}/g" config.yaml
    sed -i.bak "s/noreply@goth\.com/noreply@${DOMAIN}/g" config.yaml
    
    rm -f config.yaml.bak
    success "config.yaml atualizado"
fi

# ===========================================
# 5. Atualizar README.md
# ===========================================
log "Atualizando README.md..."

if [ -f "README.md" ]; then
    cp README.md README.md.bak
    
    sed -i.bak "s/GOTH Stack/${PROJECT_TITLE} Stack/g" README.md
    sed -i.bak "s/GOTH/${PROJECT_TITLE}/g" README.md
    sed -i.bak "s|github.com/${OLD_OWNER}/${OLD_PROJECT}|github.com/${OWNER}/${PROJECT}|g" README.md
    sed -i.bak "s|${OLD_OWNER}/${OLD_PROJECT}.wiki|${OWNER}/${PROJECT}.wiki|g" README.md
    
    rm -f README.md.bak
    success "README.md atualizado"
fi

# ===========================================
# 6. Atualizar CONTRIBUTING.md
# ===========================================
log "Atualizando CONTRIBUTING.md..."

if [ -f "CONTRIBUTING.md" ]; then
    cp CONTRIBUTING.md CONTRIBUTING.md.bak
    
    sed -i.bak "s|github.com/${OLD_OWNER}/${OLD_PROJECT}|github.com/${OWNER}/${PROJECT}|g" CONTRIBUTING.md
    sed -i.bak "s/GOTH/${PROJECT_TITLE}/g" CONTRIBUTING.md
    
    rm -f CONTRIBUTING.md.bak
    success "CONTRIBUTING.md atualizado"
fi

# ===========================================
# 7. Atualizar arquivos Docker
# ===========================================
log "Atualizando arquivos Docker..."

# docker-compose.yml
if [ -f "docker-compose.yml" ]; then
    cp docker-compose.yml docker-compose.yml.bak
    sed -i.bak "s/goth/${PROJECT}/g" docker-compose.yml
    rm -f docker-compose.yml.bak
    success "docker-compose.yml atualizado"
fi

# docker-compose.dev.yml
if [ -f "docker-compose.dev.yml" ]; then
    cp docker-compose.dev.yml docker-compose.dev.yml.bak
    sed -i.bak "s/goth/${PROJECT}/g" docker-compose.dev.yml
    rm -f docker-compose.dev.yml.bak
    success "docker-compose.dev.yml atualizado"
fi

# Dockerfiles
find docker -type f -name "Dockerfile*" 2>/dev/null | while read -r file; do
    cp "$file" "${file}.bak"
    sed -i.bak "s/goth/${PROJECT}/g" "$file"
    rm -f "${file}.bak"
done
success "Dockerfiles atualizados"

# ===========================================
# 8. Atualizar Prometheus alerts
# ===========================================
log "Atualizando Prometheus alerts..."

if [ -f "docker/otel/prometheus-alerts.yml" ]; then
    cp docker/otel/prometheus-alerts.yml docker/otel/prometheus-alerts.yml.bak
    sed -i.bak "s|github.com/${OLD_OWNER}/${OLD_PROJECT}|github.com/${OWNER}/${PROJECT}|g" docker/otel/prometheus-alerts.yml
    rm -f docker/otel/prometheus-alerts.yml.bak
    success "Prometheus alerts atualizados"
fi

# ===========================================
# 9. Atualizar GitHub Actions
# ===========================================
log "Atualizando GitHub Actions..."

if [ -d ".github/workflows" ]; then
    find .github/workflows -type f -name "*.yml" -o -name "*.yaml" | while read -r file; do
        cp "$file" "${file}.bak"
        sed -i.bak "s|github.com/${OLD_OWNER}/${OLD_PROJECT}|github.com/${OWNER}/${PROJECT}|g" "$file"
        sed -i.bak "s|${OLD_OWNER}/${OLD_PROJECT}|${OWNER}/${PROJECT}|g" "$file"
        rm -f "${file}.bak"
    done
    success "GitHub Actions atualizados"
fi

# ===========================================
# 10. Atualizar arquivos diversos
# ===========================================
log "Atualizando arquivos diversos..."

# .air.toml
if [ -f ".air.toml" ]; then
    cp .air.toml .air.toml.bak
    sed -i.bak "s/goth/${PROJECT}/g" .air.toml
    rm -f .air.toml.bak
    success ".air.toml atualizado"
fi

# litestream.yml
if [ -f "litestream.yml" ]; then
    cp litestream.yml litestream.yml.bak
    sed -i.bak "s/goth/${PROJECT}/g" litestream.yml
    rm -f litestream.yml.bak
    success "litestream.yml atualizado"
fi

# sqlc.yaml
if [ -f "sqlc.yaml" ]; then
    cp sqlc.yaml sqlc.yaml.bak
    sed -i.bak "s/goth/${PROJECT}/g" sqlc.yaml
    rm -f sqlc.yaml.bak
    success "sqlc.yaml atualizado"
fi

# ===========================================
# 11. Atualizar templates Templ
# ===========================================
log "Atualizando templates Templ..."

TEMPL_FILES=$(find . -type f -name "*.templ" ! -path "./vendor/*" ! -path "./.git/*" 2>/dev/null | wc -l)
find . -type f -name "*.templ" ! -path "./vendor/*" ! -path "./.git/*" -exec sed -i.bak \
    "s/GOTH/${PROJECT_TITLE}/g" {} +
find . -type f -name "*.templ" ! -path "./vendor/*" ! -path "./.git/*" -exec sed -i.bak \
    "s/goth/${PROJECT}/g" {} +
find . -type f -name "*.templ" ! -path "./vendor/*" ! -path "./.git/*" -delete 2>/dev/null || true
success "Templates Templ atualizados (${TEMPL_FILES} arquivos)"

# ===========================================
# 12. Go Mod Tidy
# ===========================================
log "Rodando go mod tidy..."

if command -v go &> /dev/null; then
    go mod tidy
    success "go mod tidy completado"
else
    warn "Go não encontrado. Execute 'go mod tidy' manualmente."
fi

# ===========================================
# 13. Regenerar código
# ===========================================
log "Regenerando código..."

if command -v make &> /dev/null && [ -f "Makefile" ]; then
    make generate 2>/dev/null || warn "make generate falhou. Execute manualmente."
    success "Código regenerado"
else
    warn "Makefile não encontrado. Execute 'make generate' manualmente."
fi

# ===========================================
# 14. Limpeza
# ===========================================
log "Limpando arquivos temporários..."

# Remover todos os arquivos .bak residuais
find . -type f -name "*.bak" ! -path "./vendor/*" ! -path "./.git/*" -delete 2>/dev/null || true
success "Limpeza completada"

# ===========================================
# 15. Summary
# ===========================================
echo ""
echo "============================================"
echo "  ✅ Rebranding Completado!"
echo "============================================"
echo ""
log "Novo module: ${NEW_MODULE}"
log "Projeto:     ${PROJECT_TITLE} Stack"
echo ""
warn "Próximos passos:"
echo ""
echo "  1. Review as mudanças:"
echo "     git diff"
echo ""
echo "  2. Se estiver satisfeito, commit:"
echo "     git add -A"
echo "     git commit -m 'chore: rebrand from ${OLD_PROJECT} to ${PROJECT}'"
echo ""
echo "  3. Para rollback (se necessário):"
echo "     git stash pop"
echo ""
echo "  4. Atualize a wiki (opcional):"
echo "     git clone https://github.com/${OWNER}/${PROJECT}.wiki.git"
echo ""
echo "============================================"
echo ""
