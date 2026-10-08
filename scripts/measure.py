#!/usr/bin/env python3
"""Measure the compiled CLI so compiler and fixture setup costs are excluded."""

import argparse
import json
import platform
import subprocess
from datetime import datetime, timezone
from pathlib import Path


def main():
    parser = argparse.ArgumentParser(description="测量 PanBridge 扫描与只读预览")
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--fixture", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--scan-pause", default="5ms")
    parser.add_argument("--cpu-budget", default="25")
    args = parser.parse_args()
    binary = str(args.binary.resolve())
    left, right = args.fixture / "left", args.fixture / "right"
    if not left.is_dir() or not right.is_dir():
        parser.error("fixture 需要包含 left 和 right 目录")
    manifest = json.loads((args.fixture / "fixture.json").read_text())

    def run(arguments):
        result = subprocess.run(
            [binary, *arguments, "--scan-pause", args.scan_pause, "--cpu-budget", args.cpu_budget, "--json"],
            check=True, capture_output=True, text=True,
        )
        return json.loads(result.stdout.splitlines()[-1])

    report = {
        "kind": "one_shot_metadata_scan_and_preview",
        "measured_at": datetime.now(timezone.utc).isoformat(),
        "system": platform.platform(),
        "machine": platform.machine(),
        "processor": platform.processor(),
        "logical_cpus": __import__("os").cpu_count(),
        "version": subprocess.check_output([binary, "version"], text=True).strip(),
        "scan_pause": args.scan_pause,
        "cpu_budget_single_core_percent": float(args.cpu_budget),
        "fixture": manifest,
        "limitations": [
            "No file contents are hashed or transferred.",
            "No background idle or watcher workload is measured.",
            "Single samples depend on filesystem caches and machine load.",
            "CPU percentage uses one logical core; RSS is the process lifetime peak.",
        ],
        "scan": run(["scan", "--root", str(left)]),
        "dry_run": run(["dry-run", "--left", str(left), "--right", str(right), "--limit", "0"]),
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps(report, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
