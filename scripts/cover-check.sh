#!/usr/bin/env bash
# 覆盖率门禁（P1-6）：跑一次 `go test -race -cover ./...`，既当测试步骤，又按包卡覆盖率下限。
#
#   bash scripts/cover-check.sh           # 全量
#   bash scripts/cover-check.sh --no-race # 不跑竞态检测（更快，仅本地临时用）
#
# 阈值表在下面 THRESHOLDS 里，取「当前值向下取整再留 1~3 个点」：
# 目的是防止覆盖率悄悄下滑，不是逼着一次补到某个数字。
# 新增包如果属于核心链路，请把它加进表里；表外包不卡。
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RACE="-race"
if [[ "${1:-}" == "--no-race" ]]; then
    RACE=""
fi

LOG="$(mktemp)"
trap 'rm -f "$LOG"' EXIT

echo "== go test $RACE -cover ./... =="
(cd "$ROOT/server" && go test $RACE -cover ./... 2>&1) | tee "$LOG"
TEST_STATUS=${PIPESTATUS[0]}

if [[ "$TEST_STATUS" != "0" ]]; then
    echo
    echo "❌ 单测失败（exit=$TEST_STATUS），覆盖率门禁不再评估"
    exit "$TEST_STATUS"
fi

# 包名（去掉 module 前缀） → 覆盖率下限（百分数整数）
THRESHOLDS=$(cat <<'EOF'
internal/service 50
internal/handler 3
internal/router 90
internal/repository 7
internal/middleware 45
internal/database 28
internal/pkg/crypto 70
internal/pkg/fileutil 80
internal/pkg/jwt 75
internal/pkg/password 90
internal/pkg/errcode 90
internal/pkg/page 90
internal/pkg/response 90
EOF
)

# 从 go test 的汇总行里抽「包 覆盖率」：ok/FAIL 行里带 coverage: NN.N% 的都算。
# go test 的汇总行是并发写的，多个包会挤在同一行（前面还带着上一个包的残留），
# 所以先按 "ok/FAIL" 把流切开再匹配；逐行解析会漏包。
PARSED="$(awk '{gsub(/(ok|FAIL)[ \t]+/, "\nOK "); print}' "$LOG" \
    | sed -n 's/^OK \([^ \t]*\).*coverage: \([0-9.][0-9.]*\)% of statements.*/\1 \2/p' \
    | sed 's|^apidoc/||')"

fail=0
echo
echo "== 覆盖率门禁 =="
while read -r pkg min; do
    [[ -z "${pkg:-}" ]] && continue
    got="$(printf '%s\n' "$PARSED" | awk -v p="$pkg" '$1 == p {print $2; found=1} END {if (!found) print "missing"}')"
    if [[ "$got" == "missing" ]]; then
        echo "  ⚠️  ${pkg}: 本次未产出覆盖率（包名改了？）"
        fail=1
        continue
    fi
    # 小数比较交给 awk
    if awk -v g="$got" -v m="$min" 'BEGIN { exit (g + 0 < m + 0) ? 1 : 0 }'; then
        printf '  ✅ %-28s %s%% (下限 %s%%)\n' "$pkg" "$got" "$min"
    else
        printf '  ❌ %-28s %s%% < 下限 %s%%\n' "$pkg" "$got" "$min"
        fail=1
    fi
done <<< "$THRESHOLDS"

if [[ "$fail" != "0" ]]; then
    echo
    echo "覆盖率低于下限。要么补测试，要么在本脚本 THRESHOLDS 里给出下调理由（并写进提交信息）。"
    exit 1
fi
echo
echo "覆盖率门禁通过 ✓"
