#!/bin/bash
# ===========================================
# Backup Verification Script for Litestream
# ===========================================
#
# Este script verifica a integridade dos backups
# fazendo restore em um database temporário e
# rodando queries de validação.
#
# Usage:
#   ./scripts/backup-verify.sh           # Verify latest backup
#   ./scripts/backup-verify.sh --all     # Verify all backups
#   ./scripts/backup-verify.sh --clean   # Clean temp files

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Load environment variables
if [ -f "$PROJECT_DIR/.env" ]; then
    export $(cat "$PROJECT_DIR/.env" | grep -v '^#' | xargs)
fi

# Default values
REPLICA_URL="${REPLICA_URL:-}"
DATA_DIR="${DATA_DIR:-/data}"
DB_NAME="${DB_NAME:-goth.db}"
TEMP_DIR="${PROJECT_DIR}/tmp/backup-verify"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

show_usage() {
    echo "Usage: $0 [options]"
    echo ""
    echo "Options:"
    echo "  --all       Verify all available backups"
    echo "  --clean     Clean temporary files"
    echo "  --help      Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                  # Verify latest backup"
    echo "  $0 --all            # Verify all backups"
    echo "  $0 --clean          # Clean temp files"
}

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

setup_temp_dir() {
    log_info "Setting up temporary directory: $TEMP_DIR"
    mkdir -p "$TEMP_DIR"
}

cleanup_temp_dir() {
    log_info "Cleaning temporary directory"
    rm -rf "$TEMP_DIR"
}

verify_database() {
    local db_path="$1"
    local errors=0

    log_info "Verifying database integrity: $db_path"

    # Check if file exists
    if [ ! -f "$db_path" ]; then
        log_error "Database file not found: $db_path"
        return 1
    fi

    # SQLite integrity check
    log_info "Running PRAGMA integrity_check..."
    integrity_result=$(sqlite3 "$db_path" "PRAGMA integrity_check;" 2>&1)
    if [ "$integrity_result" != "ok" ]; then
        log_error "Database integrity check failed: $integrity_result"
        return 1
    fi
    log_info "✓ Database integrity check passed"

    # Check required tables exist
    log_info "Checking required tables..."
    required_tables=("users" "audit_log" "jobs" "sessions" "feature_flags")
    for table in "${required_tables[@]}"; do
        table_exists=$(sqlite3 "$db_path" "SELECT name FROM sqlite_master WHERE type='table' AND name='$table';" 2>&1)
        if [ -z "$table_exists" ]; then
            log_warn "Table not found: $table (may not exist yet)"
        else
            row_count=$(sqlite3 "$db_path" "SELECT COUNT(*) FROM $table;" 2>&1)
            log_info "✓ Table exists: $table ($row_count rows)"
        fi
    done

    # Check for corrupted rows (basic check)
    log_info "Checking for corrupted data..."
    corrupted_count=$(sqlite3 "$db_path" "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND sql IS NULL AND name NOT LIKE 'sqlite_%';" 2>&1 || echo "0")
    if [ "$corrupted_count" -gt 0 ]; then
        log_warn "Found $corrupted_count tables with NULL sql (may indicate corruption)"
    else
        log_info "✓ No obvious corruption detected"
    fi

    # Database size check
    db_size=$(stat -c%s "$db_path" 2>/dev/null || stat -f%z "$db_path" 2>/dev/null || echo "0")
    log_info "Database size: $db_size bytes"

    if [ "$db_size" -eq 0 ]; then
        log_warn "Database file is empty"
    fi

    return 0
}

verify_backup_standalone() {
    local backup_url="$1"
    local temp_db="$TEMP_DIR/verify_$$.db"
    local exit_code=0

    log_info "Verifying backup: $backup_url"

    # Download backup using litestream (standalone)
    log_info "Downloading backup with litestream..."
    if ! litestream restore -v "$backup_url" "$temp_db" 2>&1; then
        log_error "Failed to download backup"
        return 1
    fi

    # Verify database
    if ! verify_database "$temp_db"; then
        log_error "Database verification failed"
        exit_code=1
    else
        log_info "✓ Backup verification passed"
    fi

    # Cleanup temp database
    rm -f "$temp_db" "$temp_db-wal" "$temp_db-shm"

    return $exit_code
}

verify_backup_docker() {
    local backup_url="$1"
    local temp_db="$TEMP_DIR/verify_$$.db"
    local exit_code=0

    log_info "Verifying backup: $backup_url"

    # Download backup using litestream (Docker)
    log_info "Downloading backup with Docker..."
    if ! docker-compose run --rm \
        -e AWS_ACCESS_KEY_ID \
        -e AWS_SECRET_ACCESS_KEY \
        -e AWS_REGION \
        litestream restore -v "$backup_url" "$temp_db" 2>&1; then
        log_error "Failed to download backup"
        return 1
    fi

    # Verify database
    if ! verify_database "$temp_db"; then
        log_error "Database verification failed"
        exit_code=1
    else
        log_info "✓ Backup verification passed"
    fi

    # Cleanup temp database
    rm -f "$temp_db" "$temp_db-wal" "$temp_db-shm"

    return $exit_code
}

cmd_verify() {
    local verify_all=false
    local use_docker=false

    while [[ $# -gt 0 ]]; do
        case $1 in
            --all)
                verify_all=true
                shift
                ;;
            --clean)
                cleanup_temp_dir
                exit 0
                ;;
            --docker)
                use_docker=true
                shift
                ;;
            --help)
                show_usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done

    if [ -z "$REPLICA_URL" ]; then
        log_error "REPLICA_URL not set in .env"
        echo ""
        echo "Set in .env:"
        echo "  REPLICA_URL=s3://bucket-name/goth.db"
        exit 1
    fi

    setup_temp_dir

    log_info "Starting backup verification..."
    log_info "Replica URL: $REPLICA_URL"

    # Try standalone first, fallback to Docker
    if command -v litestream &> /dev/null; then
        log_info "Using standalone litestream"
        if verify_backup_standalone "$REPLICA_URL"; then
            log_info "✓ Backup verification completed successfully"
            cleanup_temp_dir
            exit 0
        fi
    elif command -v docker-compose &> /dev/null; then
        log_info "Using Docker litestream"
        if verify_backup_docker "$REPLICA_URL"; then
            log_info "✓ Backup verification completed successfully"
            cleanup_temp_dir
            exit 0
        fi
    else
        log_error "Neither litestream nor docker-compose found!"
        echo "Install litestream: curl -L https://litestream.io/install | sudo bash"
        exit 1
    fi

    log_error "✗ Backup verification failed"
    cleanup_temp_dir
    exit 1
}

# Main
case "${1:-verify}" in
    verify|"")
        cmd_verify "$@"
        ;;
    --all|--clean|--docker|--help)
        cmd_verify "$@"
        ;;
    *)
        log_error "Unknown command: $1"
        show_usage
        exit 1
        ;;
esac
