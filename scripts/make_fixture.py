#!/usr/bin/env python3
"""Create disposable metadata workloads without copying user files."""

import argparse
import json
import os
from pathlib import Path


def main():
    parser = argparse.ArgumentParser(description="生成 PanBridge 扫描测试目录")
    parser.add_argument("--output", type=Path, required=True, help="新的或空的测试目录")
    parser.add_argument("--files", type=int, default=10000, help="每侧普通文件数量")
    parser.add_argument("--layout", choices=("flat", "nested"), default="nested")
    parser.add_argument("--changes", type=int, default=10, help="左侧不同的文件数量")
    parser.add_argument("--large-gib", type=int, default=0, help="额外创建稀疏文件，仅测试元数据")
    args = parser.parse_args()
    if not 1 <= args.files <= 1000000:
        parser.error("files 必须在 1 到 1000000 之间")
    if not 0 <= args.changes <= args.files or not 0 <= args.large_gib <= 100:
        parser.error("changes 不得超过 files；large-gib 必须在 0 到 100 之间")
    if args.output.exists() and any(args.output.iterdir()):
        parser.error("测试目录必须为空，避免覆盖已有文件")
    args.output.mkdir(parents=True, exist_ok=True)
    left, right = args.output / "left", args.output / "right"
    left.mkdir()
    right.mkdir()
    stamp = 1700000000000000000
    for number in range(args.files):
        relative = Path(f"file-{number:07d}.txt")
        if args.layout == "nested":
            relative = Path(f"group-{number // 1000:04d}") / relative
        for side in (left, right):
            target = side / relative
            target.parent.mkdir(exist_ok=True)
            with target.open("xb"):
                pass
            os.utime(target, ns=(stamp, stamp))
        if number < args.changes:
            (left / relative).write_bytes(b"changed")
        if (number + 1) % 25000 == 0:
            print(f"已生成每侧 {number + 1} 个文件", flush=True)
    if args.large_gib:
        for side in (left, right):
            path = side / "large.sparse"
            with path.open("xb") as file:
                file.truncate(args.large_gib * (1 << 30))
            os.utime(path, ns=(stamp, stamp))
    manifest = {
        "files_per_side": args.files,
        "layout": args.layout,
        "changed_left_files": args.changes,
        "large_sparse_gib": args.large_gib,
        "content": "empty files plus small changes; sparse file is metadata-only",
    }
    (args.output / "fixture.json").write_text(json.dumps(manifest, indent=2) + "\n")
    print(f"完成：{args.output.resolve()}")


if __name__ == "__main__":
    main()
