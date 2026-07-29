#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ADDR="${PHONYC_ADDR:-0.0.0.0:23342}"
DATA_DIR="${PHONYC_DATA_DIR:-$ROOT/data}"
BIN="$ROOT/bin/phonyc"
SERVICE_NAME="phonyc"

cd "$ROOT"

echo "==> building frontend"
cd web
if [[ ! -d node_modules ]]; then
  npm install
fi
npx vite build
cd "$ROOT"

echo "==> building backend"
mkdir -p bin
go build -o "$BIN" ./cmd/phonyc

echo "==> ensuring data dir"
mkdir -p "$DATA_DIR"

if command -v systemctl >/dev/null 2>&1 && systemctl list-unit-files | grep -q "^${SERVICE_NAME}.service"; then
  echo "==> restarting systemd service ${SERVICE_NAME}"
  systemctl restart "$SERVICE_NAME"
  systemctl --no-pager --full status "$SERVICE_NAME" | head -20
else
  echo "==> systemd unit missing; starting binary directly"
  fuser -k 23342/tcp 2>/dev/null || true
  pkill -x phonyc 2>/dev/null || true
  sleep 0.5
  nohup env PHONYC_ADDR="$ADDR" PHONYC_DATA_DIR="$DATA_DIR" "$BIN" \
    >>/var/log/phonyc/phonyc.log 2>&1 &
  echo $! >/var/run/phonyc.pid
fi

sleep 1
echo "==> health checks"
curl -fsS "http://127.0.0.1:23342/api/health" || true
echo
ss -tlnp | rg '23342' || true
