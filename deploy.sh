#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# OpenStory Production Deployment Script
# ============================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

COMPOSE_FILE="docker-compose.prod.yml"
ENV_FILE=".env"

# ─── Colors ────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC}  $*"; }
err()  { echo -e "${RED}[ERROR]${NC} $*"; }

# ─── Pre-flight checks ─────────────────────────────────
preflight() {
    log "Running pre-flight checks..."

    if ! command -v docker &>/dev/null; then
        err "Docker is not installed"
        exit 1
    fi

    if ! docker compose version &>/dev/null; then
        err "docker compose (v2) is required"
        exit 1
    fi

    if [ ! -f "$ENV_FILE" ]; then
        warn "$ENV_FILE not found, copying from .env.production"
        cp .env.production "$ENV_FILE"
        warn "Please edit $ENV_FILE and set MINIO_PUBLIC_ENDPOINT to your server IP, then re-run"
        exit 1
    fi

    local minio_public_endpoint
    minio_public_endpoint="$(
        awk -F= '/^[[:space:]]*MINIO_PUBLIC_ENDPOINT[[:space:]]*=/ {
            value=$0
            sub(/^[[:space:]]*MINIO_PUBLIC_ENDPOINT[[:space:]]*=/, "", value)
            print value
        }' "$ENV_FILE" | tail -n 1
    )"

    if [ -z "$minio_public_endpoint" ] ||
       [[ "$minio_public_endpoint" == *"YOUR_SERVER_IP"* ]] ||
       [[ "$minio_public_endpoint" == *"YOUR_DOMAIN"* ]]; then
        err "MINIO_PUBLIC_ENDPOINT in $ENV_FILE is not set. Edit $ENV_FILE and set it to https://incipe.top"
        exit 1
    fi

    log "Pre-flight checks passed"
}

# ─── Build images ──────────────────────────────────────
build() {
    log "Building Docker images..."
    docker compose -f "$COMPOSE_FILE" build --parallel
    log "Build complete"
}

# ─── Start services ────────────────────────────────────
start() {
    # Remove stale one-shot containers to prevent dependency deadlock
    docker compose -f "$COMPOSE_FILE" rm -f migrate minio-init 2>/dev/null || true
    log "Starting services..."
    docker compose -f "$COMPOSE_FILE" up -d
    log "All services started"
}

# ─── Stop services ─────────────────────────────────────
stop() {
    log "Stopping services..."
    docker compose -f "$COMPOSE_FILE" down
    log "All services stopped"
}

# ─── Restart a specific service ────────────────────────
restart() {
    local svc="${1:-}"
    if [ -z "$svc" ]; then
        err "Usage: $0 restart <service-name>"
        exit 1
    fi
    log "Restarting $svc..."
    docker compose -f "$COMPOSE_FILE" restart "$svc"
    log "$svc restarted"
}

# ─── View logs ─────────────────────────────────────────
logs() {
    local svc="${1:-}"
    if [ -z "$svc" ]; then
        docker compose -f "$COMPOSE_FILE" logs -f --tail=100
    else
        docker compose -f "$COMPOSE_FILE" logs -f --tail=100 "$svc"
    fi
}

# ─── Show status ───────────────────────────────────────
status() {
    log "Service status:"
    docker compose -f "$COMPOSE_FILE" ps
    echo ""
    log "Resource usage:"
    docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}" \
        $(docker compose -f "$COMPOSE_FILE" ps -q) 2>/dev/null || true
}

# ─── Run database migrations manually ──────────────────
migrate() {
    log "Running database migrations..."
    docker compose -f "$COMPOSE_FILE" run --rm migrate
    log "Migrations complete"
}

# ─── Full deploy (build + start) ───────────────────────
deploy() {
    preflight
    build
    start
    echo ""
    log "Deployment complete!"
    log "Check status with: $0 status"
    log "View logs with:   $0 logs"
}

# ─── Main ──────────────────────────────────────────────
case "${1:-deploy}" in
    deploy)  deploy ;;
    build)   preflight && build ;;
    start)   preflight && start ;;
    stop)    stop ;;
    restart) restart "${2:-}" ;;
    logs)    logs "${2:-}" ;;
    status)  status ;;
    migrate) migrate ;;
    *)
        echo "Usage: $0 {deploy|build|start|stop|restart <svc>|logs [svc]|status|migrate}"
        exit 1
        ;;
esac
