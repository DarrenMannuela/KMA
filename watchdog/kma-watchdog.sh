#!/bin/bash
# Checks that KMA's frontend, backend and auth service are reachable; if any
# isn't, brings that stack's docker compose services back up and logs it.
#
# Why this exists: on 2026-10-14 the kma_backend and kma_auth_backend
# containers stopped cleanly (not a crash) while the Mac stayed up the whole
# time — Docker Desktop itself likely restarted (e.g. an auto-update) without
# bringing the containers back, despite `restart: unless-stopped` being set.
# Nobody noticed for ~41 hours. This is the safety net for that: it runs
# every 5 minutes via the com.kma.watchdog LaunchAgent (launchctl list |
# grep kma to check it's loaded) and self-heals within that window instead.
#
# This file is the source of truth, version-controlled here so the watchdog
# itself isn't just loose, unbacked-up state on one Mac (see README.md's
# Watchdog section for the setup — it has to run from outside this repo,
# at ~/Library/Scripts/, for macOS's folder-access protection to allow a
# background process to execute it at all).

set -uo pipefail
export PATH="/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"

LOG="$HOME/Library/Logs/kma-watchdog.log"
log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" >> "$LOG"; }

notify() {
  osascript -e "display notification \"$1\" with title \"KMA Watchdog\"" >/dev/null 2>&1 || true
}

check() {
  curl -fsS --max-time 10 "$1" >/dev/null 2>&1
}

BASE="https://kma.corvus-grouper.ts.net"
VSCODE="$HOME/Documents/vscode"

if ! check "$BASE/api/v1/healthz"; then
  log "Backend unreachable — running docker compose up -d in KMA"
  (cd "$VSCODE/KMA" && docker compose up -d) >> "$LOG" 2>&1
  notify "KMA backend was down — restarted it."
fi

if ! check "$BASE/auth/healthz"; then
  log "Auth service unreachable — running docker compose up -d in KMA-Auth"
  (cd "$VSCODE/KMA-Auth" && docker compose up -d) >> "$LOG" 2>&1
  notify "KMA auth service was down — restarted it."
fi

if ! check "$BASE/"; then
  log "Frontend unreachable — running docker compose up -d in KMA-Frontend"
  (cd "$VSCODE/KMA-Frontend" && docker compose up -d) >> "$LOG" 2>&1
  notify "KMA frontend was down — restarted it."
fi
