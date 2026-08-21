#!/bin/sh
set -eu

# Dokyr uninstaller
# Usage: curl -fsSL https://sh.dokyr.com/uninstall.sh | sudo sh

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

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

require_command() {
  if ! command_exists "$1"; then
    error "'$1' is required but not installed. Aborting."
  fi
}

require_command docker

COMPOSE_CMD="docker compose"
if ! docker compose version >/dev/null 2>&1; then
  COMPOSE_CMD="docker-compose"
fi

if [ "$(id -u)" -ne 0 ]; then
  warn "This uninstaller should be run as root because it manages Docker containers."
  if [ "${DOKYR_ALLOW_NONROOT:-}" != "true" ]; then
    error "Run as root or set DOKYR_ALLOW_NONROOT=true to skip this check."
  fi
fi

if [ ! -d "$INSTALL_DIR" ]; then
  error "Install directory not found: ${INSTALL_DIR}"
fi

log "This will remove the Dokyr containers, volumes, and configuration in ${INSTALL_DIR}."
log "Managed project data and images will NOT be removed."

CONFIRM="${DOKYR_CONFIRM_UNINSTALL:-$(read_input "Type 'yes' to continue" "no")}"
if [ "$CONFIRM" != "yes" ]; then
  log "Uninstall cancelled."
  exit 0
fi

cd "$INSTALL_DIR"

if [ -f compose.yaml ]; then
  log "Stopping and removing Dokyr containers and volumes..."
  $COMPOSE_CMD down -v || warn "docker compose down failed; continuing with manual cleanup."
else
  warn "compose.yaml not found in ${INSTALL_DIR}; skipping docker compose down."
fi

log "Removing install directory ${INSTALL_DIR}..."
rm -rf "$INSTALL_DIR"

log "Dokyr has been uninstalled."
log "Docker images, networks, and project containers created by Dokyr were not removed."
log "To remove them manually, run: docker system prune -a"
