#!/bin/bash
# ===========================================
# Traefik Setup Script
# ===========================================
# Usage: ./scripts/setup-traefik.sh
#
# This script prepares the necessary files and permissions
# for Traefik to run with Let's Encrypt SSL

set -e

echo "==> Setting up Traefik..."

# Create necessary directories
echo "Creating directories..."
mkdir -p traefik/dynamic
mkdir -p traefik/logs

# Set correct permissions for acme.json (required by Let's Encrypt)
echo "Setting permissions on acme.json..."
if [ -f traefik/acme.json ]; then
    chmod 600 traefik/acme.json
else
    echo "Creating empty acme.json..."
    echo '{}' > traefik/acme.json
    chmod 600 traefik/acme.json
fi

# Create dynamic config if it doesn't exist
if [ ! -f traefik/dynamic/config.yml ]; then
    echo "Creating dynamic config..."
    cat > traefik/dynamic/config.yml << 'EOF'
# ===========================================
# Traefik Dynamic Configuration
# ===========================================
http:
  middlewares:
    goth-rate-limit:
      rateLimit:
        average: 100
        burst: 50
        period: 1s
    goth-security:
      headers:
        frameDeny: true
        browserXssFilter: true
        contentTypeNosniff: true
        forceSTSHeader: true
        stsSeconds: 31536000
    goth-compress:
      compress: {}
    goth-redirecthttps:
      redirectScheme:
        scheme: https
        permanent: true
        port: "443"
EOF
fi

# Create main traefik config if it doesn't exist
if [ ! -f traefik/traefik.yml ]; then
    echo "Creating main traefik config..."
    cat > traefik/traefik.yml << 'EOF'
# ===========================================
# Traefik Configuration
# ===========================================
api:
  dashboard:
    enabled: true
    insecure: true

entryPoints:
  web:
    address: ":80"
  websecure:
    address: ":443"

certificatesResolvers:
  letsencrypt:
    acme:
      email: paulohernane10@gmail.com
      storage: /acme.json
      httpChallenge:
        entryPoint: web

providers:
  docker:
    endpoint: "unix:///var/run/docker.sock"
    exposedByDefault: false
  file:
    directory: /etc/traefik/dynamic
    watch: true
EOF
fi

echo "==> Traefik setup complete!"
echo ""
echo "To start the stack:"
echo "  docker-compose up -d"
echo ""
echo "To view Traefik dashboard (development only):"
echo "  Open http://localhost:8080/dashboard/"
echo ""
echo "To enable SSL in production:"
echo "  1. Set APP_DOMAIN in .env to your domain"
echo "  2. Point your domain DNS to this server"
echo "  3. Run: docker-compose up -d"
echo ""
