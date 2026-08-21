#!/bin/sh
set -eu

# Dokyr Compose topology updater
# Usage: curl -fsSL https://raw.githubusercontent.com/azayr/dokyr/main/scripts/update.sh | sudo sh
#
# This replaces and reconciles Dokyr's platform Compose stack. Project
# containers and their volumes are managed separately and are not stopped or
# removed by this script.

REPO="azayr/dokyr"
BRANCH="${DOKYR_BRANCH:-main}"
INSTALL_DIR="${DOKYR_INSTALL_DIR:-/opt/dokyr}"
GITHUB_RAW="https://raw.githubusercontent.com/${REPO}/${BRANCH}"

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

require_command docker
require_command curl
require_command grep
require_command awk
require_command mktemp
require_command date
require_command cp
require_command mv
require_command chmod

if ! docker compose version >/dev/null 2>&1 && ! docker-compose version >/dev/null 2>&1; then
  error "Docker Compose is required but not found. Install the Docker Compose plugin and retry."
fi

COMPOSE_CMD="docker compose"
if ! docker compose version >/dev/null 2>&1; then
  COMPOSE_CMD="docker-compose"
fi

if [ "$(id -u)" -ne 0 ]; then
  warn "This updater should be run as root because it manages Docker containers."
  if [ "${DOKYR_ALLOW_NONROOT:-}" != "true" ]; then
    error "Run as root or set DOKYR_ALLOW_NONROOT=true to skip this check."
  fi
fi

if [ ! -d "$INSTALL_DIR" ]; then
  error "Install directory not found: ${INSTALL_DIR}"
fi

if [ ! -f "${INSTALL_DIR}/compose.yaml" ]; then
  error "Installed compose.yaml not found in ${INSTALL_DIR}"
fi

if [ ! -f "${INSTALL_DIR}/.env" ]; then
  error "Installed .env not found in ${INSTALL_DIR}; refusing to update without the existing secrets"
fi

UPDATE_DIR="$(mktemp -d "${INSTALL_DIR}/.update.XXXXXX")"
cleanup() {
  rm -rf "$UPDATE_DIR"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

download() {
  _source="$1"
  _destination="$2"
  log "  - ${_destination}"
  curl -fsSL "${GITHUB_RAW}/${_source}" -o "${UPDATE_DIR}/${_destination}" || error "Failed to download ${_source}"
}

log "Downloading the latest Dokyr platform topology..."
download compose.production.yaml compose.yaml
download Caddyfile Caddyfile
download buildkitd.toml buildkitd.toml

if ! grep -q '^  dokyr:$' "${UPDATE_DIR}/compose.yaml"; then
  error "Downloaded compose.production.yaml does not contain the Dokyr service"
fi

# Preserve all existing configuration and secrets. Migrate former SELFHOST_*
# names without overriding a DOKYR_* value the operator has already set.
awk -v install_dir="$INSTALL_DIR" '
  BEGIN { install_dir_seen = 0; legacy_count = 0 }
  /^SELFHOST_[A-Z0-9_]*=/ {
    migrated = $0
    sub(/^SELFHOST_/, "DOKYR_", migrated)
    key = migrated
    sub(/=.*/, "", key)
    if (!(key in legacy_values)) legacy_keys[++legacy_count] = key
    legacy_values[key] = migrated
    next
  }
  /^DOKYR_INSTALL_DIR=/ {
    print "DOKYR_INSTALL_DIR=" install_dir
    install_dir_seen = 1
    current["DOKYR_INSTALL_DIR"] = 1
    next
  }
  /^DOKYR_[A-Z0-9_]*=/ {
    key = $0
    sub(/=.*/, "", key)
    current[key] = 1
    print
    next
  }
  /^BUILDKIT_IMAGE=moby\/buildkit:rootless$/ {
    print "BUILDKIT_IMAGE=moby/buildkit:buildx-stable-1"
    next
  }
  { print }
  END {
    for (i = 1; i <= legacy_count; i++) {
      key = legacy_keys[i]
      if (!(key in current)) print legacy_values[key]
    }
    if (!install_dir_seen) print "DOKYR_INSTALL_DIR=" install_dir
  }
' "${INSTALL_DIR}/.env" > "${UPDATE_DIR}/.env"

log "Validating the downloaded Compose topology..."
if ! $COMPOSE_CMD --env-file "${UPDATE_DIR}/.env" -f "${UPDATE_DIR}/compose.yaml" config --quiet; then
  error "The downloaded Compose topology is invalid; the installed stack was not changed"
fi

if [ "${DOKYR_UPDATE_DRY_RUN:-false}" = "true" ]; then
  log "Dry run passed. The installed stack was not changed."
  exit 0
fi

# Pull before replacing any installed file. A registry failure therefore leaves
# the current stack and configuration untouched.
log "Pulling images required by the new topology..."
if ! $COMPOSE_CMD --env-file "${UPDATE_DIR}/.env" -f "${UPDATE_DIR}/compose.yaml" pull; then
  error "Could not pull the required images; the installed stack was not changed"
fi

BACKUP_ID="$(date -u +%Y%m%dT%H%M%SZ)-$$"
BACKUP_DIR="${INSTALL_DIR}/backups/${BACKUP_ID}"
mkdir -p "$BACKUP_DIR"
chmod 700 "$BACKUP_DIR"

log "Backing up the current platform files to ${BACKUP_DIR}..."
for file in compose.yaml Caddyfile buildkitd.toml .env; do
  if [ -f "${INSTALL_DIR}/${file}" ]; then
    cp -p "${INSTALL_DIR}/${file}" "${BACKUP_DIR}/${file}"
  fi
done
chmod 600 "${BACKUP_DIR}/.env"

for file in compose.yaml Caddyfile buildkitd.toml .env; do
  mv "${UPDATE_DIR}/${file}" "${INSTALL_DIR}/${file}"
done
chmod 600 "${INSTALL_DIR}/.env"

cd "$INSTALL_DIR"

log "Reconciling Dokyr platform services..."
if ! $COMPOSE_CMD up -d --remove-orphans; then
  warn "The new topology failed to start. Restoring ${BACKUP_DIR}..."
  for file in compose.yaml Caddyfile buildkitd.toml .env; do
    if [ -f "${BACKUP_DIR}/${file}" ]; then
      cp -p "${BACKUP_DIR}/${file}" "${INSTALL_DIR}/${file}"
    fi
  done
  chmod 600 "${INSTALL_DIR}/.env"
  $COMPOSE_CMD up -d --remove-orphans || warn "Automatic rollback could not fully restore the platform; inspect Docker Compose logs."
  error "Dokyr platform update failed and the previous files were restored"
fi

printf '\n'
log "Dokyr platform topology updated successfully."
log "Backup: ${BACKUP_DIR}"
log "Project containers and project volumes were left running."
log "Use 'cd ${INSTALL_DIR} && ${COMPOSE_CMD} ps' to inspect platform services."
