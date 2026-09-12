#!/usr/bin/env python3
"""Run three complete file workflows against the real CLI and an offline remote."""
import base64
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import zipfile

root = Path(__file__).resolve().parents[1]
with tempfile.TemporaryDirectory(prefix="drivebridge-personal-loop-") as directory:
    temp = Path(directory)
    local, remote = temp / "local", temp / "remote"
    local.mkdir(); remote.mkdir()
    binary = temp / "drivebridge"
    subprocess.run(["go", "build", "-o", str(binary), "./cmd/drivebridge"], cwd=root, check=True)
    config = temp / "rclone.conf"
    config.write_text(f"[fixture]\ntype = alias\nremote = {remote}\n")
    config.chmod(0o600)
    environment = {k: v for k, v in os.environ.items() if not k.startswith("DRIVEBRIDGE_")}
    environment["XDG_CONFIG_HOME"] = str(temp / "config")
    def cli(*args, ok=True):
        result = subprocess.run([str(binary), "--state", str(temp / "state"), "--rclone-config", str(config), *args, "--json"], cwd=temp, env=environment, capture_output=True, text=True, timeout=45)
        if not ok:
            assert result.returncode != 0, "Rejected operation unexpectedly succeeded"
            return result.stderr
        assert result.returncode == 0, "CLI operation failed: " + result.stderr
        return json.loads(result.stdout)
    cli("setup", "--remote", "fixture", "--remote-path", "", "--local-root", str(local))
    (local / "report.md").write_text("# Research report\nVerified fixture observations.\n")
    with zipfile.ZipFile(local / "build.zip", "w") as archive:
        archive.writestr("build/result.txt", "tested build artifact")
    (local / "reference.png").write_bytes(base64.b64decode("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII="))
    for scenario, filename in [("report", "report.md"), ("build", "build.zip"), ("creative", "reference.png")]:
        dest = "outputs/" + scenario
        cli("mkdir", "--space", "personal", "--path", dest)
        args = ("upload", "--space", "personal", "--file", filename, "--dir", dest, "--caller", scenario + "-agent", "--task-ref", "task:" + scenario)
        receipt = cli(*args)["data"]
        assert receipt["status"] == "completed" and receipt["caller"] == scenario + "-agent"
        assert cli("resume", receipt["id"])["data"]["id"] == receipt["id"]
        assert cli("recent", "--space", "personal", "--pending")["data"]["count"] == 0
        assert cli(*args)["data"]["id"] == receipt["id"], "Replay changed the operation"
        file = cli("stat", "--ref", receipt["file_ref"])["data"]
        assert file["task_ref"] == "task:" + scenario
        assert file["sha256"] == hashlib.sha256((local / filename).read_bytes()).hexdigest()
        assert cli("find", "--space", "personal", "--query", filename, "--path", dest)["data"]["count"] == 1
        target = "copy-" + filename
        cli("download", "--ref", receipt["file_ref"], "--out", target)
        assert (local / target).read_bytes() == (local / filename).read_bytes()
        cli("trash", "--ref", receipt["file_ref"])
        cli("download", "--ref", receipt["file_ref"], "--out", "trashed-" + filename, ok=False)
        cli("restore", "--ref", receipt["file_ref"])
        cli("download", "--ref", receipt["file_ref"], "--out", target)
        print("PASS:", scenario, "input -> upload -> pinned ref -> provenance -> replay -> download -> trash/restore")
    assert "path_not_allowed" in cli("upload", "--space", "personal", "--file", "../rclone.conf", ok=False)
    print("PASS: three scenarios share one space, transfer model and receipt contract; no live accounts.")
