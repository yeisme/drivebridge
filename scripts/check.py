#!/usr/bin/env python3
"""Run offline checks and preserve redacted owner evidence on success or failure."""
import datetime
import json
import os
import pathlib
import re
import subprocess
import sys
import uuid

root = pathlib.Path(__file__).resolve().parents[1]
run = root / "temp" / "integration-test-runs" / (datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%dT%H%M%SZ") + "-" + uuid.uuid4().hex[:8])
(run / "artifacts").mkdir(parents=True)
started=datetime.datetime.now(datetime.timezone.utc)
command = sys.argv[1:] or ["go", "test", "-race", "./..."]
if command and command[0] == "--":
    command = command[1:]

def redact(value):
    value = re.sub(r"https?://[^\s\"<>]+", "[redacted-url]", value)
    value = re.sub(r"(?i)(authorization|access_token|refresh_token|client_secret|token)\s*[:=]\s*\S+", r"\1=[redacted]", value)
    value = re.sub(r"db_[a-f0-9]{48,}", "[redacted-agent-token]", value)
    return value

(run / "command.txt").write_text(redact(" ".join(command)) + "\n")
(run / "env.json").write_text(json.dumps({"project": root.parent.name + "/" + root.name, "environment": "offline-fixtures", "platform": sys.platform, "credentials": "not-collected"}))
try:
    child_env=os.environ.copy()
    child_env["DRIVEBRIDGE_EVIDENCE_ARTIFACTS"]=str(run / "artifacts")
    completed = subprocess.run(command, cwd=root, env=child_env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, timeout=900)
    code, stdout, stderr = completed.returncode, completed.stdout, completed.stderr
except subprocess.TimeoutExpired:
    code, stdout, stderr = 124, "", "Check timed out.\n"
except OSError:
    code, stdout, stderr = 127, "", "Check command unavailable.\n"
(run / "stdout.log").write_text(redact(stdout))
(run / "stderr.log").write_text(redact(stderr))
# Playwright may write textual error context outside the run directory.
for artifact in (root / "test-results").glob("**/*"):
    if artifact.is_file() and artifact.suffix in {".md", ".txt", ".json", ".log"}:
        artifact.write_text(redact(artifact.read_text(errors="replace")))
(run / "summary.json").write_text(json.dumps({"schema_version":"yeisme.integration_test_evidence.v1", "project":root.parent.name+"/"+root.name, "run_id":run.name, "command":command, "started_at":started.isoformat(), "finished_at":datetime.datetime.now(datetime.timezone.utc).isoformat(), "duration_ms":int((datetime.datetime.now(datetime.timezone.utc)-started).total_seconds()*1000), "redaction":{"enabled":True,"policy":"credentials-and-bearer-urls"}, "evidence":{"stdout":"stdout.log","stderr":"stderr.log","artifacts":"artifacts/"}, "status": "passed" if code == 0 else "failed", "exit_code": code, "layer": "e2e" if any("playwright" in arg or "remote_system.py" in arg for arg in command) else "component", "real_onedrive": False, "paid_provider": False, "deployment": False}))
print(redact(stdout), end="")
print(redact(stderr), end="", file=sys.stderr)
print("Evidence: " + str(run.relative_to(root)))
sys.exit(code)
