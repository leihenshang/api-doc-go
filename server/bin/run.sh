#!/usr/bin/env bash
# API-DOC 后端服务控制脚本。
# 用法：./bin/run.sh {start|stop|restart|status|build}
# 环境：默认加载 bin/env.test（测试配置）；生产用 ENV_FILE=config/prod.env 等方式覆盖：ENV_FILE=xxx ./bin/run.sh start
set -u
cd "$(dirname "$0")/.."
ROOT="$PWD"
BIN=bin/apidoc-server
PIDFILE=bin/apidoc-server.pid
LOGFILE=bin/logs/server.log
ENV_FILE="${ENV_FILE:-bin/env.test}"

load_env() {
  if [ -f "$ENV_FILE" ]; then
    set -a
    # shellcheck disable=SC1090
    . "./$ENV_FILE"
    set +a
    echo "[env] loaded $ENV_FILE (PORT=${PORT:-8080}, DB_DRIVER=${DB_DRIVER:-sqlite})"
  else
    echo "[env] $ENV_FILE not found, using defaults"
  fi
}

is_running() {
  [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null
}

build() {
  echo "[build] go build -o $BIN ./cmd/server"
  go build -o "$BIN" ./cmd/server || { echo "[build] FAILED"; exit 1; }
}

start() {
  if is_running; then
    echo "already running (pid $(cat "$PIDFILE"))"
    return 0
  fi
  mkdir -p bin/logs "${UPLOAD_DIR:-data/uploads}" "$(dirname "${DB_DSN:-data/apidoc.db}")"
  build
  nohup "$BIN" >> "$LOGFILE" 2>&1 &
  echo $! > "$PIDFILE"
  sleep 1
  if is_running; then
    echo "started: pid=$(cat "$PIDFILE") port=${PORT:-8080} log=$LOGFILE"
  else
    echo "failed to start, last log lines:"
    tail -n 10 "$LOGFILE"
    exit 1
  fi
}

stop() {
  if ! is_running; then
    echo "not running"
    rm -f "$PIDFILE"
    return 0
  fi
  pid=$(cat "$PIDFILE")
  kill "$pid" 2>/dev/null
  sleep 1
  is_running && kill -9 "$pid" 2>/dev/null
  rm -f "$PIDFILE"
  echo "stopped (pid $pid)"
}

status() {
  if is_running; then
    echo "running: pid=$(cat "$PIDFILE")"
    curl -s "http://127.0.0.1:${PORT:-8080}/api/health" 2>/dev/null || true
    echo
  else
    echo "not running"
  fi
}

case "${1:-}" in
  start)   load_env; start ;;
  stop)    stop ;;
  restart) load_env; stop; start ;;
  status)  load_env; status ;;
  build)   load_env; build ;;
  *) echo "usage: $0 {start|stop|restart|status|build}"; exit 2 ;;
esac
