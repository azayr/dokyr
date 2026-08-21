#!/bin/sh
set -eu

# Dokyr one-line installer
# Usage: curl -fsSL https://sh.dokyr.com | sudo sh
# The script installs the Docker Compose stack into /opt/dokyr by default.

REPO="azayr/dokyr"
BRANCH="${DOKYR_BRANCH:-main}"
INSTALL_DIR="${DOKYR_INSTALL_DIR:-/opt/dokyr}"
GITHUB_RAW="https://raw.githubusercontent.com/${REPO}/${BRANCH}"

# ---- helpers ----

log() {
  printf '\033[1;34m==>\033[0m %s\n' "$*"
}

warn() {
  printf '\033[1;33m==>\033[0m %s\n' "$*" >&2
}

error() {
  printf '\033[1;31m==>\033[0m %s\n' "$*" >&2
  exit 1
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

require_command() {
  if ! command_exists "$1"; then
    error "'$1' is required but not installed. Aborting."
  fi
}

random_secret() {
  if command_exists openssl; then
    openssl rand -base64 32 | tr -d '=+/\n' | cut -c1-48
  else
    # Fallback that works on most Unix-like systems
    LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom 2>/dev/null | head -c 48
  fi
}

detect_server_ip() {
  if [ -n "${SERVER_IP:-}" ]; then
    printf '%s' "$SERVER_IP"
    return
  fi
  if _public_ip="$(curl -4fsS --max-time 5 https://api.ipify.org 2>/dev/null)" &&
    printf '%s' "$_public_ip" | grep -Eq '^[0-9]+(\.[0-9]+){3}$'; then
    printf '%s' "$_public_ip"
    return
  fi
  if command_exists hostname; then
    _local_ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
    if [ -n "$_local_ip" ]; then
      printf '%s' "$_local_ip"
      return
    fi
  fi
  printf '127.0.0.1'
}

# ---- preflight ----

require_command docker

if ! docker compose version >/dev/null 2>&1 && ! docker-compose version >/dev/null 2>&1; then
  error "Docker Compose is required but not found. Install the Docker Compose plugin and retry."
fi

COMPOSE_CMD="docker compose"
if ! docker compose version >/dev/null 2>&1; then
  COMPOSE_CMD="docker-compose"
fi

require_command curl
require_command tr
require_command head
require_command grep
require_command chmod
require_command awk


# Docker socket is required for Dokyr to manage the host engine.
if [ ! -S /var/run/docker.sock ]; then
  warn "Docker socket not found at /var/run/docker.sock. Dokyr needs it to orchestrate containers."
fi

# Root is recommended because the container needs the host Docker socket.
if [ "$(id -u)" -ne 0 ]; then
  warn "This installer is usually run as root because Dokyr mounts /var/run/docker.sock."
  if [ "${DOKYR_ALLOW_NONROOT:-}" != "true" ]; then
    error "Run as root or set DOKYR_ALLOW_NONROOT=true to skip this check."
  fi
fi

# ---- gather configuration ----

log "Welcome to the Dokyr installer."
log "Install directory: ${INSTALL_DIR}"

HTTP_PORT="${HTTP_PORT:-80}"
RECOVERY_HTTP_PORT="${RECOVERY_HTTP_PORT:-3030}"
HTTPS_PORT="${HTTPS_PORT:-443}"
DOKYR_IMAGE="${DOKYR_IMAGE:-ghcr.io/azayr/dokyr:latest}"
DOKYR_REGISTRY_IMAGE="${DOKYR_REGISTRY_IMAGE:-ghcr.io/azayr/dokyr}"
DOKYR_UPDATE_CHANNEL="${DOKYR_UPDATE_CHANNEL:-latest}"
REGISTRY_HOSTS="${REGISTRY_HOSTS:-registry.invalid}"
REGISTRY_HTTP_RELATIVEURLS="${REGISTRY_HTTP_RELATIVEURLS:-true}"
SERVER_IP="$(detect_server_ip)"
PUBLIC_URL="${PUBLIC_URL:-http://${SERVER_IP}:${RECOVERY_HTTP_PORT}}"
CONTROL_HOSTS="${CONTROL_HOSTS:-${SERVER_IP}}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-}"
JWT_SECRET="${JWT_SECRET:-}"
ENCRYPTION_KEY="${ENCRYPTION_KEY:-}"
REGISTRY_INTERNAL_SECRET="${REGISTRY_INTERNAL_SECRET:-}"
STALWART_RECOVERY_PASSWORD="${STALWART_RECOVERY_PASSWORD:-}"
STALWART_RELAY_PASSWORD="${STALWART_RELAY_PASSWORD:-}"

if [ -z "$POSTGRES_PASSWORD" ]; then POSTGRES_PASSWORD="$(random_secret)"; fi
if [ -z "$JWT_SECRET" ]; then JWT_SECRET="$(random_secret)"; fi
if [ -z "$ENCRYPTION_KEY" ]; then ENCRYPTION_KEY="$(random_secret)"; fi
if [ -z "$REGISTRY_INTERNAL_SECRET" ]; then REGISTRY_INTERNAL_SECRET="$(random_secret)"; fi
if [ -z "$STALWART_RECOVERY_PASSWORD" ]; then STALWART_RECOVERY_PASSWORD="$(random_secret)"; fi
if [ -z "$STALWART_RELAY_PASSWORD" ]; then STALWART_RELAY_PASSWORD="$(random_secret)"; fi

if [ "$HTTP_PORT" = "$RECOVERY_HTTP_PORT" ]; then
  error "HTTP_PORT and RECOVERY_HTTP_PORT must use different host ports"
fi

printf '\n'
log "Downloading Compose files into ${INSTALL_DIR}..."
mkdir -p "$INSTALL_DIR"
cd "$INSTALL_DIR"

# Install the VPS topology under Docker Compose's conventional filename.
log "  - compose.yaml"
curl -fsSL "${GITHUB_RAW}/compose.production.yaml" -o compose.yaml.tmp || error "Failed to download compose.production.yaml"
mv compose.yaml.tmp compose.yaml

for file in Caddyfile buildkitd.toml .env.example; do
  log "  - ${file}"
  curl -fsSL "${GITHUB_RAW}/${file}" -o "${file}.tmp" || error "Failed to download ${file}"
  mv "${file}.tmp" "$file"
done

if ! grep -q '^  dokyr:$' compose.yaml 2>/dev/null; then
  error "Downloaded compose.yaml does not contain the Dokyr service"
fi

# Build .env from the example. Keep the example as a reference.
log "Writing .env configuration..."
cat > .env <<EOF
HTTP_PORT=${HTTP_PORT}
RECOVERY_HTTP_PORT=${RECOVERY_HTTP_PORT}
HTTPS_PORT=${HTTPS_PORT}
PUBLIC_URL=${PUBLIC_URL}
CONTROL_HOSTS=${CONTROL_HOSTS}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
JWT_SECRET=${JWT_SECRET}
ENCRYPTION_KEY=${ENCRYPTION_KEY}
COOKIE_SECURE=false
DOKYR_INSTALL_DIR=${INSTALL_DIR}
HOST_DISK_PATH=${INSTALL_DIR}/host-disk

# Auto builds analyze with Railpack and execute the generated plan on BuildKit.
DOKYR_BUILDKIT_HOST=tcp://buildkit:1234
BUILDKIT_IMAGE=moby/buildkit:buildx-stable-1
DOKYR_BUILDKIT_CACHE_REF=
DOKYR_RAILPACK_FRONTEND=ghcr.io/railwayapp/railpack-frontend:latest

DOKYR_IMAGE=${DOKYR_IMAGE}
DOKYR_REGISTRY_IMAGE=${DOKYR_REGISTRY_IMAGE}
DOKYR_UPDATE_CHANNEL=${DOKYR_UPDATE_CHANNEL}

# Attach the public registry domain from Infrastructure -> Registry after
# installation. This value is only a first-start and compatibility fallback.
REGISTRY_HOSTS=${REGISTRY_HOSTS}
REGISTRY_TOKEN_ISSUER=dokyr-registry
REGISTRY_TOKEN_SERVICE=dokyr-registry
REGISTRY_INTERNAL_SECRET=${REGISTRY_INTERNAL_SECRET}
REGISTRY_STORAGE=filesystem
REGISTRY_HTTP_RELATIVEURLS=${REGISTRY_HTTP_RELATIVEURLS}

# Optional S3-compatible storage for the registry.
REGISTRY_S3_REGION=
REGISTRY_S3_BUCKET=
REGISTRY_S3_ACCESSKEY=
REGISTRY_S3_SECRETKEY=
REGISTRY_S3_ENDPOINT=
REGISTRY_S3_FORCEPATHSTYLE=false
REGISTRY_S3_SECURE=true

GITLAB_CLIENT_ID=
GITLAB_CLIENT_SECRET=
GITLAB_BASE_URL=https://gitlab.com
GITEA_CLIENT_ID=
GITEA_CLIENT_SECRET=
GITEA_BASE_URL=https://gitea.com

SMTP_ENABLED=true
SMTP_HOST=
SMTP_PORT=587
SMTP_ENCRYPTION=starttls
SMTP_USERNAME=
SMTP_PASSWORD=
SMTP_FROM_NAME=Dokyr
SMTP_FROM_EMAIL=
SMTP_NOTIFY_DEPLOYMENT_FAILURES=true
SMTP_NOTIFY_DEPLOYMENT_SUCCESSES=false

# Built-in Stalwart mail service. Its public hostname is configured once from
# Infrastructure -> Mail after the owner account is created.
STALWART_IMAGE=stalwartlabs/stalwart:v0.16
STALWART_HOSTNAME=
STALWART_PUBLIC_URL=
STALWART_RECOVERY_PASSWORD=${STALWART_RECOVERY_PASSWORD}
STALWART_RELAY_PASSWORD=${STALWART_RELAY_PASSWORD}
STALWART_SMTP_PORT=25
STALWART_SUBMISSIONS_PORT=465
STALWART_IMAPS_PORT=993
STALWART_POP3S_PORT=995
STALWART_SIEVE_PORT=4190

MAIL_STALWART_URL=http://stalwart:8080
MAIL_STALWART_API_KEY=
MAIL_STALWART_USER=admin
MAIL_STALWART_PASSWORD=${STALWART_RECOVERY_PASSWORD}
MAIL_STALWART_HOSTNAME=
MAIL_STALWART_DEFAULT_DOMAIN=
MAIL_STALWART_RELAY_HOST=stalwart
MAIL_STALWART_RELAY_PORT=465
MAIL_STALWART_RELAY_PASSWORD=${STALWART_RELAY_PASSWORD}
EOF
chmod 600 .env

# Create the host disk mount target so the path is stable.
mkdir -p "${INSTALL_DIR}/host-disk"

# For the host disk path, the compose file uses ${HOST_DISK_PATH:-.}.
# Export it so the relative path resolves to the install directory.
export HOST_DISK_PATH="${INSTALL_DIR}/host-disk"

# ---- pull and start ----

log "Pulling images..."
$COMPOSE_CMD pull

log "Starting Dokyr..."
$COMPOSE_CMD up -d --remove-orphans

printf '\n'
log "Dokyr is starting up."
log "Control panel: ${PUBLIC_URL}"
log "Install directory: ${INSTALL_DIR}"
log ""
log "Next steps:"
log "  1. Visit ${PUBLIC_URL} and create the owner account."
log "  2. Add a control-panel domain from the dashboard when DNS is ready."
log "  3. Configure optional mail, registry, and source integrations from the panel."
log ""
log "To manage the stack later:"
log "  cd ${INSTALL_DIR} && ${COMPOSE_CMD} up -d"
log ""
log "Your secrets are saved in ${INSTALL_DIR}/.env. Keep that file safe."
