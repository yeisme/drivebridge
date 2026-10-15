#!/usr/bin/env python3
"""Verify that a source archive builds without sibling repositories."""
from pathlib import Path
import os
import subprocess
import tempfile

root = Path(__file__).resolve().parents[2]
with tempfile.TemporaryDirectory(prefix="drivebridge-fresh-clone-") as target:
    archive = Path(target) / "source.tar"
    subprocess.run(["git", "archive", "--format=tar", "HEAD", "-o", str(archive)], cwd=root, check=True)
    checkout = Path(target) / "checkout"
    checkout.mkdir()
    subprocess.run(["tar", "-xf", str(archive), "-C", str(checkout)], check=True)
    subprocess.run(["go", "list", "./..."], cwd=checkout, env={**os.environ, "GOWORK": "off", "CGO_ENABLED": "0"}, check=True)
print("fresh-clone: go list ./... passed without sibling repositories")
