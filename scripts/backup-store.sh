#!/usr/bin/env bash
set -euo pipefail
# Backup WhatsApp session DB. Safe to run while wabot is running (SQLite on same host;
# copy is crash-consistent enough for disaster recovery).
WDIR="${WABOT_DIR:-$(dirname "$0")/..}"
WDIR="$(cd "$WDIR" && pwd)"
DB="${WABOT_STORE_DB:-$WDIR/store.db}"
DEST="${WABOT_BACKUP_DIR:-$WDIR/backups}"
RETAIN="${WABOT_BACKUP_RETAIN:-14}"

if [[ ! -f "$DB" ]]; then
  echo "backup-store: database not found at $DB" >&2
  exit 1
fi

mkdir -p "$DEST"
stamp="$(date -u +%Y%m%d_%H%M%S)"
cp -a -- "$DB" "$DEST/store-${stamp}.db"
echo "backup-store: wrote $DEST/store-${stamp}.db"

# Keep the newest RETAIN files, delete older ones.
if [[ "$RETAIN" =~ ^[0-9]+$ ]] && [[ "$RETAIN" -gt 0 ]]; then
  mapfile -t files < <(ls -1t "$DEST"/store-*.db 2>/dev/null || true)
  if ((${#files[@]} > RETAIN)); then
    for ((i = RETAIN; i < ${#files[@]}; i++)); do
      rm -f -- "${files[$i]}"
    done
  fi
fi
