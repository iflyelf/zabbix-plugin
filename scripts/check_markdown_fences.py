#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""检查 Markdown 文件的代码围栏（``` / ~~~）是否成对闭合。

用法：
    python3 scripts/check_markdown_fences.py
    python3 scripts/check_markdown_fences.py address/README.md ports/README.md
    python3 scripts/check_markdown_fences.py --root /path/to/zabbix_plugin

退出码：
    0  所有 Markdown 文件的代码围栏均成对闭合
    1  存在未闭合的代码围栏或文件读取失败
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

DEFAULT_ROOT = Path(__file__).resolve().parent.parent

# CommonMark 围栏规则：最多 3 个前导空格，至少 3 个 ` 或 ~。
FENCE_RE = re.compile(r"^( {0,3})(`{3,}|~{3,})(.*)$")


def check_file(path: Path) -> str | None:
    """返回错误描述；围栏成对闭合时返回 None。"""
    try:
        text = path.read_text(encoding="utf-8-sig")
    except OSError as exc:
        return f"无法读取文件: {exc}"

    open_char: str | None = None
    open_len = 0
    open_line = 0
    fence_count = 0

    for line_no, line in enumerate(text.splitlines(), start=1):
        match = FENCE_RE.match(line)
        if match is None:
            continue

        _, fence, rest = match.groups()
        char = fence[0]

        if open_char is None:
            # 开启新的代码块
            open_char = char
            open_len = len(fence)
            open_line = line_no
            fence_count += 1
            continue

        # 只有相同字符、长度不小于开启围栏、且后面没有内容时才是闭合围栏
        if char == open_char and len(fence) >= open_len and rest.strip() == "":
            open_char = None

    if open_char is not None:
        return f"未闭合的代码围栏（开始于第 {open_line} 行，共 {fence_count} 个开启围栏）"

    return None


def collect_files(root: Path, args: list[str]) -> list[Path]:
    if args:
        files = []
        for item in args:
            path = Path(item)
            if not path.is_absolute():
                path = root / path
            files.append(path)
        return files

    return sorted(
        path
        for path in root.rglob("*.md")
        if ".git" not in path.parts and "vendor" not in path.parts
    )


def main() -> int:
    parser = argparse.ArgumentParser(
        description="检查 Markdown 文件中的代码围栏是否成对闭合"
    )
    parser.add_argument(
        "--root",
        type=Path,
        default=DEFAULT_ROOT,
        help="Markdown 文件搜索根目录（默认：脚本所在目录的上一级）",
    )
    parser.add_argument(
        "files",
        nargs="*",
        help="指定要检查的 Markdown 文件（相对 --root 或绝对路径），默认检查根目录下全部 .md",
    )
    args = parser.parse_args()

    root = args.root.resolve()
    files = collect_files(root, args.files)

    if not files:
        print(f"[ERROR] 未找到 Markdown 文件: {root}")
        return 1

    failed = False
    for path in files:
        error = check_file(path)
        try:
            display = path.relative_to(root)
        except ValueError:
            display = path

        if error:
            print(f"[FAIL]  {display}: {error}")
            failed = True
        else:
            print(f"[OK]    {display}")

    print()
    if failed:
        print("检查失败：请补充或删除未闭合的代码围栏。")
        return 1

    print(f"检查通过：{len(files)} 个 Markdown 文件的代码围栏均成对闭合。")
    return 0


if __name__ == "__main__":
    sys.exit(main())
