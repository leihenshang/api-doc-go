#!/usr/bin/env bash
# 服务端 e2e 一键跑：重置独立库 → 起真实服务 → 跑 HTTP 端到端 → 停服务。
# 独立端口/独立 SQLite/独立上传目录，不碰开发库 data/apidoc.db。
set -uo pipefail
cd "$(dirname "$0")/.."
export PATH="$PATH:$HOME/.vfox/sdks/golang/bin"

PORT="${PORT:-8093}"

stop_server() {
    local pid
    pid="$(ss -ltnp 2>/dev/null | grep ":$PORT " | grep -o 'pid=[0-9]*' | head -1 | cut -d= -f2)"
    if [[ -n "$pid" ]]; then
        kill "$pid" 2>/dev/null || true
        sleep 1
    fi
}

stop_server
# 产物统一放 data/tmp/（已 gitignore），避免污染工作区
rm -f data/tmp/e2e.db data/tmp/e2e.db-*
rm -rf data/tmp/e2e-uploads
mkdir -p data/tmp/e2e-uploads

PORT="$PORT" GIN_MODE=release DB_DRIVER=sqlite DB_DSN=data/tmp/e2e.db \
    JWT_SECRET=e2e-secret SECRET_ENCRYPT_KEY=e2e-key \
    ADMIN_NAME=admin ADMIN_PASSWORD=admin123 \
    UPLOAD_DIR=data/tmp/e2e-uploads BACKUP_ENABLED=false \
    go run ./cmd/server > data/tmp/e2e-server.log 2>&1 &
SRV=$!

ready=0
for _ in $(seq 1 30); do
    if curl -s -m 1 "http://127.0.0.1:$PORT/api/health" | grep -q '"status":"ok"'; then
        ready=1
        break
    fi
    sleep 2
done
if [[ "$ready" != "1" ]]; then
    echo "服务端未能就绪，日志尾部：" >&2
    tail -20 data/tmp/e2e-server.log >&2
    kill "$SRV" 2>/dev/null || true
    exit 1
fi

E2E_BASE="http://127.0.0.1:$PORT" python3 data/e2e_full.py
RC=$?

kill "$SRV" 2>/dev/null || true
stop_server
exit "$RC"
