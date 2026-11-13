#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""检查各插件 README.md 是否覆盖了 plugin/metrics.go 中注册的全部监控项。

用法：
    python3 scripts/check_readme_metrics.py
    python3 scripts/check_readme_metrics.py address ports
    python3 scripts/check_readme_metrics.py --root /path/to/zabbix_plugin

退出码：
    0  所有监控项均已在 README 中出现
    1  存在遗漏的监控项或检查失败
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

DEFAULT_PLUGINS = [
    "address",
    "conntrack",
    "hardware",
    "login",
    "ports",
    "tcpconnect",
]

# 匹配 const 块中的字符串常量，例如：keyAddressDiscovery = "address.discovery"
CONST_RE = re.compile(
    r'^\s*(?:const\s+)?([A-Za-z_]\w*)\s*=\s*"([^"]+)"',
    re.MULTILINE,
)

# 匹配 MetricSet / metricHandlers 中的键，例如：keyAddressDiscovery: ... 或 "address.discovery": ...
ENTRY_RE = re.compile(
    r'^\s*(?:([A-Za-z_]\w*)|"([^"]+)")\s*:',
    re.MULTILINE,
)


def extract_metric_keys(source: str) -> list[str]:
    """从 metrics.go 源码中提取监控项键值。"""
    constants = dict(CONST_RE.findall(source))

    # 优先解析 var metrics = metric.MetricSet{...}，失败时回退到 metricHandlers。
    block = re.search(r"metric\.MetricSet\s*\{(.*?)\n\}", source, re.DOTALL)
    if block is None:
        block = re.search(
            r"metricHandlers\s*=\s*map\[[^\]]+\][^{]+\{(.*?)\n\}",
            source,
            re.DOTALL,
        )
    if block is None:
        return []

    keys: list[str] = []
    for ident, literal in ENTRY_RE.findall(block.group(1)):
        key = literal or constants.get(ident, "")
        if key and key not in keys:
            keys.append(key)

    return keys


def check_plugin(root: Path, name: str) -> tuple[list[str], str | None]:
    """返回 (遗漏的监控项, 错误信息)。"""
    readme = root / name / "README.md"
    metrics = root / name / "plugin" / "metrics.go"

    if not readme.is_file():
        return [], f"README.md 不存在: {readme}"
    if not metrics.is_file():
        return [], f"plugin/metrics.go 不存在: {metrics}"

    keys = extract_metric_keys(metrics.read_text(encoding="utf-8"))
    if not keys:
        return [], "未从 metrics.go 中解析到任何监控项"

    readme_text = readme.read_text(encoding="utf-8")
    missing = [key for key in keys if f"`{key}`" not in readme_text]

    return missing, None


def main() -> int:
    parser = argparse.ArgumentParser(
        description="检查插件 README.md 是否覆盖 plugin/metrics.go 中的全部监控项"
    )
    parser.add_argument(
        "--root",
        type=Path,
        default=Path(__file__).resolve().parent.parent,
        help="插件根目录（默认：脚本所在目录的上一级）",
    )
    parser.add_argument(
        "plugins",
        nargs="*",
        help=f"要检查的插件目录名，默认检查：{', '.join(DEFAULT_PLUGINS)}",
    )
    args = parser.parse_args()

    root: Path = args.root.resolve()
    plugins = args.plugins or DEFAULT_PLUGINS

    failed = False
    for name in plugins:
        missing, error = check_plugin(root, name)

        if error:
            print(f"[ERROR] {name}: {error}")
            failed = True
            continue

        if missing:
            print(f"[FAIL]  {name}: 缺少 {len(missing)} 个监控项")
            for key in missing:
                print(f"        - {key}")
            failed = True
        else:
            print(f"[OK]    {name}: README 已覆盖全部监控项")

    print()
    if failed:
        print("检查失败：请补充 README.md 的“监控项”章节。")
        return 1

    print(f"检查通过：{len(plugins)} 个插件的 README 均覆盖全部监控项。")
    return 0


if __name__ == "__main__":
    sys.exit(main())
