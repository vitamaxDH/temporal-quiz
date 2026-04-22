#!/usr/bin/env python3
"""Regenerate runs.json in a quizzes directory.

Walks <quizzes_dir>/runs/<YYYY-MM-DD>/ and produces <quizzes_dir>/runs.json:

  {
    "runs": [
      {
        "date": "2026-04-19",
        "generated_at": "2026-04-19T17:14:21Z",
        "total_count": 247
      },
      ...
    ]
  }

Sorted newest-first. Only directories whose names parse as YYYY-MM-DD are
included. The script is idempotent and safe to re-run.

Usage:
  python3 scripts/build_runs_index.py <quizzes_dir>
"""

from __future__ import annotations

import json
import re
import sys
from datetime import date
from pathlib import Path

DATE_RE = re.compile(r"^\d{4}-\d{2}-\d{2}$")


def parse_date(name: str) -> date | None:
    if not DATE_RE.match(name):
        return None
    try:
        return date.fromisoformat(name)
    except ValueError:
        return None


def build_index(quizzes_dir: Path) -> dict:
    runs_dir = quizzes_dir / "runs"
    if not runs_dir.is_dir():
        return {"runs": []}

    entries = []
    for child in runs_dir.iterdir():
        if not child.is_dir():
            continue
        d = parse_date(child.name)
        if d is None:
            continue

        manifest_path = child / "manifest.json"
        generated_at = ""
        total_count = 0
        if manifest_path.is_file():
            try:
                m = json.loads(manifest_path.read_text(encoding="utf-8"))
                generated_at = m.get("generated_at", "") or ""
                total_count = int(m.get("total_count", 0) or 0)
            except (json.JSONDecodeError, ValueError):
                pass

        entries.append(
            {
                "date": child.name,
                "generated_at": generated_at,
                "total_count": total_count,
            }
        )

    entries.sort(key=lambda e: e["date"], reverse=True)
    return {"runs": entries}


def main(argv: list[str]) -> int:
    if len(argv) != 2:
        print(f"usage: {argv[0]} <quizzes_dir>", file=sys.stderr)
        return 2

    quizzes_dir = Path(argv[1])
    if not quizzes_dir.is_dir():
        print(f"error: {quizzes_dir} is not a directory", file=sys.stderr)
        return 1

    index = build_index(quizzes_dir)
    out = quizzes_dir / "runs.json"
    out.write_text(json.dumps(index, indent=2) + "\n", encoding="utf-8")
    print(f"wrote {out} with {len(index['runs'])} run(s)")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
