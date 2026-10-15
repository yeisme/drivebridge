#!/usr/bin/env python3
"""Check the release version across user-facing and generated metadata."""
from pathlib import Path
import json
import re
import sys

root = Path(__file__).resolve().parents[1]
version = (root / "VERSION").read_text().strip()
if not re.fullmatch(r"0\.\d+\.\d+", version):
    raise SystemExit(f"invalid VERSION: {version!r}")

checks = {
    "web/package.json": json.loads((root / "web/package.json").read_text())["version"],
    "desktop/package.json": json.loads((root / "desktop/package.json").read_text())["version"],
}
for path in ("web/scripts/configure.mjs", "desktop/scripts/configure.mjs", "internal/bridge/mcp.go", "internal/remote/remote.go", "server/internal/contract/openapi.go", "server/internal/service/mcp.go", "contracts/personal-workspace/openapi.json"):
    text = (root / path).read_text()
    if version not in text:
        raise SystemExit(f"{path}: missing release version {version}")
for path, actual in checks.items():
    if actual != version:
        raise SystemExit(f"{path}: expected {version}, found {actual}")
print(f"version: {version}")
