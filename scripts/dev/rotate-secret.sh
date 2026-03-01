#!/bin/bash
# ===========================================
# Secret Rotation Script
# ===========================================
#
# Rotaciona segredos de forma segura
#
# Usage:
#   ./scripts/rotate-secret.sh session_secret
#   ./scripts/rotate-secret.sh password_pepper
#   ./scripts/rotate-secret.sh all

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
ENV_FILE="$PROJECT_DIR/.env"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

generate_hex() {
    openssl rand -hex "$1"
}

generate_base64() {
    openssl rand -base64 "$1"
}

rotate_secret() {
    local secret_type="$1"
    local new_value=""

    case "$secret_type" in
        session_secret|SESSION_SECRET)
            new_value=$(generate_hex 32)
            env_var="SESSION_SECRET"
            ;;
        password_pepper|PASSWORD_PEPPER)
            new_value=$(generate_hex 16)
            env_var="PASSWORD_PEPPER"
            ;;
        asaas_hmac_secret|ASAAS_HMAC_SECRET)
            new_value=$(generate_hex 32)
            env_var="ASAAS_HMAC_SECRET"
            ;;
        google_client_secret|GOOGLE_CLIENT_SECRET)
            new_value=$(generate_base64 32)
            env_var="GOOGLE_CLIENT_SECRET"
            ;;
        smtp_pass|SMTP_PASS)
            new_value=$(generate_base64 24)
            env_var="SMTP_PASS"
            ;;
        *)
            log_error "Unknown secret type: $secret_type"
            return 1
            ;;
    esac

    # Backup .env
    cp "$ENV_FILE" "${ENV_FILE}.backup.$(date +%Y%m%d_%H%M%S)"

    # Update .env
    if grep -q "^${env_var}=" "$ENV_FILE"; then
        sed -i "s|^${env_var}=.*|${env_var}=${new_value}|" "$ENV_FILE"
    else
        echo "${env_var}=${new_value}" >> "$ENV_FILE"
    fi

    log_info "Secret '$env_var' rotated successfully"
    echo "New value: ${new_value:0:4}...${new_value: -4}"
}

rotate_all() {
    log_info "Rotating all rotatable secrets..."
    
    rotate_secret "session_secret"
    rotate_secret "password_pepper"
    rotate_secret "asaas_hmac_secret"
    
    log_info "All secrets rotated!"
    echo ""
    log_warn "IMPORTANT: Restart the service to apply changes:"
    echo "  systemctl restart goth"
    echo "  or"
    echo "  docker-compose restart"
}

show_usage() {
    echo "Usage: $0 <secret_type>"
    echo ""
    echo "Secret types:"
    echo "  session_secret       - Generate new session secret (32 bytes hex)"
    echo "  password_pepper      - Generate new password pepper (16 bytes hex)"
    echo "  asaas_hmac_secret    - Generate new Asaas HMAC secret"
    echo "  google_client_secret - Generate new Google OAuth secret"
    echo "  smtp_pass            - Generate new SMTP password"
    echo "  all                  - Rotate all auto-generatable secrets"
    echo ""
    echo "Examples:"
    echo "  $0 session_secret"
    echo "  $0 all"
}

# Main
case "${1:-}" in
    session_secret|password_pepper|asaas_hmac_secret|google_client_secret|smtp_pass|all)
        if [ "$1" = "all" ]; then
            rotate_all
        else
            rotate_secret "$1"
            echo ""
            log_warn "IMPORTANT: Restart the service to apply changes:"
            echo "  systemctl restart goth"
        fi
        ;;
    --help|-h)
        show_usage
        ;;
    *)
        log_error "Invalid secret type: ${1:-}"
        show_usage
        exit 1
        ;;
esac
