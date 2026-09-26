#!/usr/bin/env bash
# 一键校验（本地与 CI 共用）：Go 格式/构建/静态检查/单测 + 前端类型检查/Lint/构建。
# 用法：
#   bash scripts/check.sh              # 全量
#   bash scripts/check.sh --skip-build # 跳过前端构建（本地快速自检）
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKIP_BUILD=0
if [[ "${1:-}" == "--skip-build" ]]; then
    SKIP_BUILD=1
fi

echo "== 1/6 gofmt（列出未格式化文件，有输出即失败） =="
unformatted="$(cd "$ROOT/server" && gofmt -l cmd internal)"
if [[ -n "$unformatted" ]]; then
    echo "$unformatted"
    echo "-> 请运行：cd server && gofmt -w cmd internal"
    exit 1
fi

echo "== 2/6 go build =="
(cd "$ROOT/server" && go build ./...)

echo "== 3/6 go vet =="
(cd "$ROOT/server" && go vet ./...)

echo "== 4/6 go test =="
(cd "$ROOT/server" && go test ./...)

echo "== 5/6 前端类型检查 + ESLint =="
(cd "$ROOT/web" && npm run --silent type-check && npm run --silent lint)

if [[ "$SKIP_BUILD" == "0" ]]; then
    echo "== 6/6 前端构建 =="
    (cd "$ROOT/web" && npm run --silent build)
else
    echo "== 6/6 已跳过前端构建（--skip-build） =="
fi

echo "全部校验通过 ✓"
