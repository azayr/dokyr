#!/bin/sh
set -eu

# Dokyr one-line installer
# Usage: curl -fsSL https://raw.githubusercontent.com/azayr/dokyr/main/scripts/install.sh | sh
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

read_input() {
  _prompt="$1"
  _default="$2"
  if [ -n "$_default" ]; then
    printf "%s [%s]: " "$_prompt" "$_default" > /dev/tty
  else
    printf "%s: " "$_prompt" > /dev/tty
  fi
  read -r _value < /dev/tty
  if [ -z "$_value" ]; then
    _value="$_default"
  fi
  printf '%s' "$_value"
}

read_secret() {
  _prompt="$1"
  printf "%s: " "$_prompt" > /dev/tty
  stty -echo 2>/dev/null < /dev/tty || true
  read -r _value < /dev/tty
  stty echo 2>/dev/null < /dev/tty || true
  printf '\n' > /dev/tty
  printf '%s' "$_value"
}

random_secret() {
  if command_exists openssl; then
    openssl rand -base64 32 | tr -d '=+/\n' | cut -c1-48
  else
    # Fallback that works on most Unix-like systems
    LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom 2>/dev/null | head -c 48
  fi
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
require_command stty
require_command tr
require_command head
require_command grep
require_command sed

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

printf '\n'
HTTP_PORT="${HTTP_PORT:-80}"
HTTPS_PORT="${HTTPS_PORT:-443}"
DOKYR_IMAGE="${DOKYR_IMAGE:-ghcr.io/azayr/dokyr:latest}"
DOKYR_REGISTRY_IMAGE="${DOKYR_REGISTRY_IMAGE:-ghcr.io/azayr/dokyr}"
DOKYR_UPDATE_CHANNEL="${DOKYR_UPDATE_CHANNEL:-latest}"
REGISTRY_HOSTS="${REGISTRY_HOSTS:-registry.invalid}"
REGISTRY_HTTP_RELATIVEURLS="${REGISTRY_HTTP_RELATIVEURLS:-true}"
PUBLIC_URL="${PUBLIC_URL:-$(read_input "Public URL (e.g. http://panel.example.com)" "http://localhost:${HTTP_PORT}")}"
CONTROL_HOSTS="${CONTROL_HOSTS:-$(read_input "Control panel hostnames (space-separated, e.g. panel.example.com)" "localhost")}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-$(read_input "PostgreSQL password (leave blank to generate)" "")}"
JWT_SECRET="${JWT_SECRET:-$(read_input "JWT secret (leave blank to generate)" "")}"
ENCRYPTION_KEY="${ENCRYPTION_KEY:-$(read_input "Encryption key (leave blank to generate)" "")}"
REGISTRY_INTERNAL_SECRET="${REGISTRY_INTERNAL_SECRET:-}"
STALWART_RECOVERY_PASSWORD="${STALWART_RECOVERY_PASSWORD:-}"
STALWART_RELAY_PASSWORD="${STALWART_RELAY_PASSWORD:-}"

if [ -z "$POSTGRES_PASSWORD" ]; then POSTGRES_PASSWORD="$(random_secret)"; fi
if [ -z "$JWT_SECRET" ]; then JWT_SECRET="$(random_secret)"; fi
if [ -z "$ENCRYPTION_KEY" ]; then ENCRYPTION_KEY="$(random_secret)"; fi
if [ -z "$REGISTRY_INTERNAL_SECRET" ]; then REGISTRY_INTERNAL_SECRET="$(random_secret)"; fi
if [ -z "$STALWART_RECOVERY_PASSWORD" ]; then STALWART_RECOVERY_PASSWORD="$(random_secret)"; fi
if [ -z "$STALWART_RELAY_PASSWORD" ]; then STALWART_RELAY_PASSWORD="$(random_secret)"; fi

printf '\n'
log "Downloading Compose files into ${INSTALL_DIR}..."
mkdir -p "$INSTALL_DIR"
cd "$INSTALL_DIR"

# Download files from the selected branch on GitHub.
for file in compose.yaml Caddyfile .env.example; do
  log "  - ${file}"
  curl -fsSL "${GITHUB_RAW}/${file}" -o "${file}.tmp" || error "Failed to download ${file}"
  mv "${file}.tmp" "$file"
done

if ! grep -q '^  dokyr:$' compose.yaml 2>/dev/null; then
  error "Downloaded compose.yaml does not contain the Dokyr service"
fi

# The repository compose.yaml includes `build: .` for local development. The
# published image is used for a VPS install, so remove the build directive so the
# installer does not need a local Dockerfile.
if grep -q '^    build: \.$' compose.yaml 2>/dev/null; then
  sed -i.bak '/^    build: \.$/d' compose.yaml && rm -f compose.yaml.bak
fi

# Build .env from the example. Keep the example as a reference.
log "Writing .env configuration..."
cat > .env <<EOF
HTTP_PORT=${HTTP_PORT}
HTTPS_PORT=${HTTPS_PORT}
PUBLIC_URL=${PUBLIC_URL}
CONTROL_HOSTS=${CONTROL_HOSTS}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
JWT_SECRET=${JWT_SECRET}
ENCRYPTION_KEY=${ENCRYPTION_KEY}
COOKIE_SECURE=false
HOST_DISK_PATH=${INSTALL_DIR}/host-disk

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
log "  2. Open Infrastructure -> Mail and choose the server's public mail hostname."
log "  3. Point that hostname to this server, set matching reverse DNS, and allow TCP port 25."
log "  4. Add sending domains from Infrastructure -> Mail."
log "  5. Point a registry hostname to this server, then attach it from Infrastructure -> Registry -> Registry domain."
log "  6. (Optional) Link GitHub from Settings -> Security; Dokyr creates an identity-only GitHub App for this server."
log ""
log "To manage the stack later:"
log "  cd ${INSTALL_DIR} && ${COMPOSE_CMD} up -d"
log ""
log "Your secrets are saved in ${INSTALL_DIR}/.env. Keep that file safe."
