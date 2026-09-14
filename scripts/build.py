#!/usr/bin/env python3
"""Build standalone, un-published local candidate archives with SHA-256 manifests."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import zipfile

parser = argparse.ArgumentParser()
parser.add_argument("--target", action="append", choices=["linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64", "windows/amd64", "windows/arm64"])
parser.add_argument("--output", default="temp/distribution")
args = parser.parse_args()
root = Path(__file__).resolve().parents[1]
out = Path(args.output).resolve(); out.mkdir(parents=True, exist_ok=True)
web = root.parents[1] / "client" / "drivebridge-client"
subprocess.run(["pnpm", "build"], cwd=web, check=True)
manifest = []
for target in args.target or ["linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64"]:
    system, arch = target.split("/")
    windows = system == "windows"
    with tempfile.TemporaryDirectory(prefix="drivebridge-build-") as temporary:
        binary = Path(temporary) / ("drivebridge.exe" if windows else "drivebridge")
        environment = {**os.environ, "GOOS": system, "GOARCH": arch, "CGO_ENABLED": "0"}
        subprocess.run(["go", "build", "-trimpath", "-o", str(binary), "./cmd/drivebridge"], cwd=root, env=environment, check=True)
        archive = out / (f"drivebridge-{system}-{arch}.zip" if windows else f"drivebridge-{system}-{arch}.tar.gz")
        if windows:
            # ZipFile has no recursive add; mirror the tar layout manually.
            with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as bundle:
                bundle.write(binary, arcname="drivebridge.exe")
                bundle.write(root / "docs" / "personal-agent-loop.md", arcname="README.md")
                for asset in (web / "dist").rglob("*"):
                    if asset.is_file():
                        bundle.write(asset, arcname=str(Path("web") / asset.relative_to(web / "dist")))
                bundle.write(root / "docs" / "insights-audit.md", arcname="insights-audit.md")
        else:
            with tarfile.open(archive, "w:gz") as bundle:
                bundle.add(binary, arcname="drivebridge")
                bundle.add(root / "docs" / "personal-agent-loop.md", arcname="README.md")
                bundle.add(web / "dist", arcname="web")
                bundle.add(root / "docs" / "insights-audit.md", arcname="insights-audit.md")
        status = "local-candidate" if not windows else "local-candidate-windows-unverified-on-machine"
        manifest.append({"target": target, "archive": archive.name, "sha256": hashlib.sha256(archive.read_bytes()).hexdigest(), "status": status})
        print("Built", archive.name)
(out / "manifest.json").write_text(json.dumps({"schema_version": "1.0", "published": False, "builds": manifest}, indent=2) + "\n")
(out / "SHA256SUMS").write_text("".join(f"{entry['sha256']}  {entry['archive']}\n" for entry in manifest))
