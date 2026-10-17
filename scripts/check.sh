#!/usr/bin/env bash
# 一键校验（本地与 CI 共用）：Go 格式/构建/静态检查/单测 + 前端类型检查/Lint/构建。
# 用法：
#   bash scripts/check.sh              # 全量（单测带 -race）
#   bash scripts/check.sh --skip-build # 跳过前端构建（本地快速自检）
#   bash scripts/check.sh --no-race    # 单测不开 -race（更快，仅本地临时用）
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKIP_BUILD=0
# 默认开 -race：并发问题（watcher、sync engine、批量还原）只在竞态检测下暴露，
# 本地默认不跑就等于永远不跑。
RACE="-race"
for arg in "$@"; do
    case "$arg" in
    --skip-build) SKIP_BUILD=1 ;;
    --no-race) RACE="" ;;
    esac
done

echo "== gofmt（列出未格式化文件，有输出即失败） =="
unformatted="$(cd "$ROOT/server" && gofmt -l cmd internal)"
if [[ -n "$unformatted" ]]; then
    echo "$unformatted"
    echo "-> 请运行：cd server && gofmt -w cmd internal"
    exit 1
fi

echo "== 三仓 share 版本一致性（P2-2） =="
bash "$ROOT/scripts/share-version.sh"

echo "== go build =="
(cd "$ROOT/server" && go build ./...)

echo "== go vet =="
(cd "$ROOT/server" && go vet ./...)

echo "== go test $RACE + 覆盖率门禁 =="
RACE_ARG=""
if [[ -z "$RACE" ]]; then
    RACE_ARG="--no-race"
fi
bash "$ROOT/scripts/cover-check.sh" $RACE_ARG

echo "== 前端类型检查 + ESLint =="
(cd "$ROOT/web" && npm run --silent type-check && npm run --silent lint)

echo "== 前端单测（vitest） =="
(cd "$ROOT/web" && npm run --silent test)

if [[ "$SKIP_BUILD" == "0" ]]; then
    echo "== 前端构建 =="
    (cd "$ROOT/web" && npm run --silent build)
else
    echo "== 已跳过前端构建（--skip-build） =="
fi

echo "全部校验通过 ✓"
