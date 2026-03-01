#!/bin/bash
# ===========================================
# Backup/Restore Script for Litestream
# ===========================================
#
# Usage:
#   ./scripts/backup.sh restore  # Restore from S3
#   ./scripts/backup.sh snapshot # Create local snapshot
#   ./scripts/backup.sh list     # List available backups

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

show_usage() {
    echo "Usage: $0 <command>"
    echo ""
    echo "Commands:"
    echo "  restore     Restore database from S3"
    echo "  snapshot    Create local snapshot"
    echo "  list        List available backups"
    echo "  status      Show replication status"
    echo ""
    echo "Examples:"
    echo "  $0 restore"
    echo "  $0 snapshot"
    echo "  $0 list"
}

cmd_restore() {
    if [ -z "$REPLICA_URL" ]; then
        echo "ERROR: REPLICA_URL not set"
        echo ""
        echo "Set in .env:"
        echo "  REPLICA_URL=s3://bucket-name/goth.db"
        exit 1
    fi

    echo "==> Restoring database from: $REPLICA_URL"
    echo "    Target: $DATA_DIR/$DB_NAME"

    # Stop app if running
    docker-compose stop app 2>/dev/null || true

    # Restore
    docker-compose run --rm -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_REGION \
        litestream restore -v "$DATA_DIR/$DB_NAME" "$REPLICA_URL"

    # Start app
    docker-compose start app 2>/dev/null || true

    echo "==> Restore complete!"
}

cmd_snapshot() {
    echo "==> Creating local snapshot..."

    docker-compose run --rm litestream snapshot "$DATA_DIR/$DB_NAME"

    echo "==> Snapshot created!"
}

cmd_list() {
    if [ -z "$REPLICA_URL" ]; then
        echo "ERROR: REPLICA_URL not set"
        exit 1
    fi

    echo "==> Listing snapshots from: $REPLICA_URL"

    docker-compose run --rm -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_REGION \
        litestream shadows "$DATA_DIR/$DB_NAME" "$REPLICA_URL"
}

cmd_status() {
    echo "==> Checking replication status..."

    docker-compose logs litestream | tail -20
}

# Main
case "${1:-}" in
    restore)
        cmd_restore
        ;;
    snapshot)
        cmd_snapshot
        ;;
    list)
        cmd_list
        ;;
    status)
        cmd_status
        ;;
    *)
        show_usage
        exit 1
        ;;
esac
