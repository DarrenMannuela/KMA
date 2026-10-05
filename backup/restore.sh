#!/bin/sh
# Puts a backup back:
#   ./backup/restore.sh backups/kma-<date>.sqlite.gz
#
# It checks the backup first (SQLite's integrity check), asks you to type
# yes, stops the backend, moves the current database aside (it can be put
# back the same way), puts the backup in its place and starts the backend
# again. Run it from anywhere; it works in this stack's folder.
#
# Photos are separate: to put them back too, unpack the uploads backup
# from the same date over the uploads folder:
#   tar -xzf backups/kma-uploads-<date>.tar.gz -C uploads
set -eu
cd "$(dirname "$0")/.."

DB="db_data/kma.sqlite"
SERVICE="backend"

file="${1:-}"
if [ -z "$file" ] || [ ! -f "$file" ]; then
  echo "Usage: ./backup/restore.sh backups/kma-<date>.sqlite.gz"
  echo "Newest backups:"
  ls -1t backups/kma-[0-9]*.sqlite.gz 2>/dev/null | head -n 10 | sed 's/^/  /'
  exit 1
fi

gzip -t "$file"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
gunzip -c "$file" > "$tmp"
check="$(sqlite3 "$tmp" 'PRAGMA integrity_check;' 2>&1 || true)"
if [ "$check" != "ok" ]; then
  echo "That backup fails SQLite's integrity check, so nothing was changed:"
  echo "$check"
  exit 1
fi

printf 'This replaces the live database (%s) with %s.\nType yes to go on: ' "$DB" "$file"
read -r answer
if [ "$answer" != "yes" ]; then
  echo "Nothing changed."
  exit 1
fi

docker compose stop "$SERVICE"
aside="db_data/before-restore-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$aside"
# The -wal and -shm files go too: left next to the restored file, SQLite
# would replay the old database's last changes onto it.
for f in "$DB" "$DB-wal" "$DB-shm"; do
  if [ -f "$f" ]; then
    mv "$f" "$aside/"
  fi
done
cp "$tmp" "$DB"
docker compose start "$SERVICE"
echo "Restored $file. The database as it was is in $aside."
