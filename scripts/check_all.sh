#!/usr/bin/env bash
# 统一执行 scripts 下的文档检查脚本。
#
# 用法：
#   ./scripts/check_all.sh
#   bash scripts/check_all.sh
#
# 退出码：
#   0  全部检查通过
#   1  存在检查失败项

set -uo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd)"

usage() {
    cat <<'EOF'
用法：
    ./scripts/check_all.sh
    ./scripts/check_all.sh -h|--help

说明：
    统一执行 scripts 下的文档检查脚本，任一检查失败时退出码为 1。

新增检查：
    在下方“检查项”区域追加一行 run_check 即可，例如：

        run_check "插件配置检查" \
            python3 "$SCRIPT_DIR/check_plugin_configs.py" --root "$ROOT_DIR"
EOF
}

case "${1:-}" in
    -h|--help)
        usage
        exit 0
        ;;
    "")
        ;;
    *)
        echo "[ERROR] 未知参数: $1" >&2
        usage >&2
        exit 2
        ;;
esac

if ! command -v python3 >/dev/null 2>&1; then
    echo "[ERROR] 未找到 python3，请先安装 Python 3。" >&2
    exit 1
fi

status=0

run_check() {
    local title="$1"
    shift

    echo "===== ${title} ====="
    if "$@"; then
        echo "[PASS] ${title}"
    else
        echo "[FAIL] ${title}"
        status=1
    fi
    echo
}

# ===== 检查项 =====
# 新增文档类检查时，只需在下面追加一项 run_check。

run_check "README 监控项检查" \
    python3 "$SCRIPT_DIR/check_readme_metrics.py" --root "$ROOT_DIR"

run_check "Markdown 代码围栏检查" \
    python3 "$SCRIPT_DIR/check_markdown_fences.py" --root "$ROOT_DIR"

if [ "$status" -eq 0 ]; then
    echo "全部检查通过。"
else
    echo "存在检查失败项，请根据上方输出修复后重试。"
fi

exit "$status"
