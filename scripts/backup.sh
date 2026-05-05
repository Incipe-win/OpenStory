#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# OpenStory Backup Script
# ============================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BACKUP_DIR="${BACKUP_DIR:-$PROJECT_DIR/backups}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_PATH="$BACKUP_DIR/$TIMESTAMP"

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

log()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
err()  { echo -e "${RED}[ERROR]${NC} $*"; }

mkdir -p "$BACKUP_PATH"

# ─── Backup PostgreSQL ─────────────────────────────────
backup_postgres() {
    log "Backing up PostgreSQL..."
    local container="openstory-postgres"
    if ! docker ps -q -f name="$container" | grep -q .; then
        err "PostgreSQL container is not running"
        return 1
    fi
    docker exec "$container" pg_dump -U openstory openstory > "$BACKUP_PATH/database.sql"
    log "Database backup saved to $BACKUP_PATH/database.sql"
}

# ─── Backup volumes (optional, can be large) ──────────
backup_volumes() {
    log "Backing up MinIO data..."
    if docker ps -q -f name="openstory-minio" | grep -q .; then
        docker run --rm \
            -v openstory_miniodata:/data:ro \
            -v "$BACKUP_PATH:/backup" \
            alpine:3.21 tar czf /backup/miniodata.tar.gz -C /data .
        log "MinIO backup saved to $BACKUP_PATH/miniodata.tar.gz"
    fi
}

# ─── Cleanup old backups ───────────────────────────────
cleanup() {
    local keep="${BACKUP_KEEP_DAYS:-7}"
    log "Removing backups older than $keep days..."
    find "$BACKUP_DIR" -maxdepth 1 -mindepth 1 -type d -mtime "+$keep" -exec rm -rf {} \; 2>/dev/null || true
}

# ─── Main ──────────────────────────────────────────────
case "${1:-full}" in
    full)
        backup_postgres
        log "Full backup complete at $BACKUP_PATH"
        ;;
    db)
        backup_postgres
        ;;
    volumes)
        backup_volumes
        ;;
    cleanup)
        cleanup
        ;;
    *)
        echo "Usage: $0 {full|db|volumes|cleanup}"
        exit 1
        ;;
esac
