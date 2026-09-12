#!/usr/bin/env python3
"""Verify local candidate archives and execute only the native binary's help."""
import hashlib
import json
from pathlib import Path
import platform
import subprocess
import tarfile
import tempfile

root = Path(__file__).resolve().parents[1]
out = root / "temp/distribution"
manifest = json.loads((out / "manifest.json").read_text())
assert manifest["published"] is False
machine = {"x86_64": "amd64", "aarch64": "arm64", "arm64": "arm64"}.get(platform.machine())
native = platform.system().lower() + "/" + str(machine)
executed = False
for build in manifest["builds"]:
    assert Path(build["archive"]).name == build["archive"]
    archive = out / build["archive"]
    assert hashlib.sha256(archive.read_bytes()).hexdigest() == build["sha256"], "Archive checksum mismatch"
    with tarfile.open(archive) as bundle:
        assert {"drivebridge", "README.md", "web/insights.html"}.issubset(bundle.getnames())
        assert all(".." not in Path(name).parts and not Path(name).is_absolute() for name in bundle.getnames())
        if build["target"] == native:
            with tempfile.TemporaryDirectory(prefix="drivebridge-distribution-") as directory:
                binary = Path(directory) / "drivebridge"
                binary.write_bytes(bundle.extractfile("drivebridge").read())
                binary.chmod(0o700)
                result = subprocess.run([str(binary), "--help"], capture_output=True, text=True, timeout=10, check=True)
                assert all(command in result.stdout for command in ["setup", "recent", "upload", "resume", "usage", "audit", "view"])
                executed = True
    print("PASS: checksum and contents", build["target"])
assert executed, "No native candidate to smoke test"
print("PASS: standalone native binary exposes setup/recent/upload/resume; other targets are compile-only.")
