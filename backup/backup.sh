#!/bin/sh
set -eu

DB_PATH="/db/kma.sqlite"
BACKUP_DIR="/backups"
# How many days of daily backups to keep before pruning. Weekly retention
# doesn't need a separate schedule — a Sunday backup just happens to be
# the one that survives once older dailies age out, if you keep this at
# 7+; bump to e.g. 90 for ~13 weeks of history at low disk cost (SQLite
# backups compress well and this DB is small).
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-30}"

mkdir -p "$BACKUP_DIR"

timestamp=$(date +%Y%m%d-%H%M%S)
dest="$BACKUP_DIR/kma-$timestamp.sqlite"

# sqlite3's .backup command is safe to run against a live database (it
# takes the appropriate read lock and correctly folds in anything still
# sitting in the WAL file) — unlike `cp`, which can silently miss
# recently-written data while WAL mode is on.
sqlite3 "$DB_PATH" ".backup '$dest'"

# Compress after the fact so the backup step itself stays as fast/safe as
# possible; gzip -9 on a small SQLite file is cheap.
gzip -9 "$dest"

echo "$(date -Iseconds) backed up to ${dest}.gz"

# Prune anything older than RETENTION_DAYS.
find "$BACKUP_DIR" -name 'kma-*.sqlite.gz' -mtime "+${RETENTION_DAYS}" -delete

echo "$(date -Iseconds) pruned backups older than ${RETENTION_DAYS} days"