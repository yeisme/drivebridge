#!/usr/bin/env python3
"""Run offline checks and preserve redacted owner evidence on success or failure."""
import datetime
import hashlib
import json
import os
import pathlib
import re
import signal
import subprocess
import sys
import uuid

root = pathlib.Path(__file__).resolve().parents[1]
run = root / "temp" / "integration-test-runs" / (datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%dT%H%M%SZ") + "-" + uuid.uuid4().hex[:8])
(run / "artifacts").mkdir(parents=True)
started=datetime.datetime.now(datetime.timezone.utc)
command = sys.argv[1:] or ["go", "test", "./..."]
if command and command[0] == "--":
    command = command[1:]
# Explicit long-run budget: --timeout SECONDS (or --timeout=SECONDS) before the
# wrapped command. Scale fixtures exceed the default 900s guard; the flag keeps
# the same redaction and six-piece evidence, and records the budget used.
timeout_seconds = 900
while command and (command[0] == "--timeout" or command[0].startswith("--timeout=")):
    raw = command[0][10:] if command[0].startswith("--timeout=") else (command[1] if len(command) > 1 else "")
    command = command[2:] if not command[0].startswith("--timeout=") else command[1:]
    try:
        timeout_seconds = max(1, int(raw))
    except ValueError:
        timeout_seconds = -1
if timeout_seconds <= 0:
    sys.exit("usage: check.py [--timeout SECONDS] -- command...")
if command and command[0] == "--":
    command = command[1:]

def redact(value):
    value = re.sub(r"https?://[^\s\"<>]+", "[redacted-url]", value)
    value = re.sub(r"(?i)(authorization|access_token|refresh_token|client_secret|token)\s*[:=]\s*\S+", r"\1=[redacted]", value)
    value = re.sub(r"(?:db_|dbu_)[a-f0-9]{48,}", "[redacted-agent-token]", value)
    return value

(run / "command.txt").write_text(redact(" ".join(command)) + "\n")
fixture_flags = {name: os.environ.get("DRIVEBRIDGE_TEST_" + name) == "1" for name in (
    "PROPERTY_EDITING", "SHARE_RECEIVERS", "LABEL_MANAGEMENT", "HOLD_MANAGEMENT",
    "AUTOMATION_MANAGEMENT", "TRANSFER_PLANS", "ARCHIVE_FAILURE", "TEAM_DEMO", "DEMO", "VISUAL_FULL")}
web_dir = pathlib.Path(os.environ.get("DRIVEBRIDGE_WEB_DIR", str(root / "web/dist")))
web_index = web_dir / "index.html"
try:
    index_sha256 = hashlib.sha256(web_index.read_bytes()).hexdigest()
except OSError:
    index_sha256 = None
web_artifact = {"source": "explicit" if "DRIVEBRIDGE_WEB_DIR" in os.environ else "workspace-default",
                "index_sha256": index_sha256}
(run / "env.json").write_text(json.dumps({"project": root.parent.name + "/" + root.name, "environment": "offline-fixtures", "platform": sys.platform, "credentials": "not-collected", "database": os.environ.get("DRIVEBRIDGE_TEST_DATABASE", "default") if os.environ.get("DRIVEBRIDGE_TEST_DATABASE", "default") in {"sqlite", "postgres", "default"} else "unknown", "fixture_flags": fixture_flags, "web_artifact": web_artifact}))
try:
    child_env=os.environ.copy()
    child_env["DRIVEBRIDGE_EVIDENCE_ARTIFACTS"]=str(run / "artifacts")
    process = subprocess.Popen(command, cwd=root, env=child_env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, start_new_session=os.name == 'posix')
    stdout, stderr = process.communicate(timeout=timeout_seconds)
    code = process.returncode
except subprocess.TimeoutExpired:
    # Only our process group is stopped. Preserve partial output, and give
    # fixture runners time to release their own disposable services first.
    if os.name == 'posix':
        try:
            os.killpg(process.pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
    else:
        process.terminate()
    try:
        stdout, stderr = process.communicate(timeout=10)
    except subprocess.TimeoutExpired:
        if os.name == 'posix':
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
        else:
            process.kill()
        stdout, stderr = process.communicate()
    code, stderr = 124, stderr + "\nCheck timed out.\n"
except OSError:
    code, stdout, stderr = 127, "", "Check command unavailable.\n"
(run / "stdout.log").write_text(redact(stdout))
(run / "stderr.log").write_text(redact(stderr))
# Playwright may write textual error context outside the run directory.
for artifact in (run / "artifacts").glob("**/*"):
    if artifact.is_file() and artifact.suffix in {".md", ".txt", ".json", ".log"}:
        artifact.write_text(redact(artifact.read_text(errors="replace")))
(run / "summary.json").write_text(json.dumps({"schema_version":"yeisme.integration_test_evidence.v1", "project":root.parent.name+"/"+root.name, "run_id":run.name, "command":command, "started_at":started.isoformat(), "finished_at":datetime.datetime.now(datetime.timezone.utc).isoformat(), "duration_ms":int((datetime.datetime.now(datetime.timezone.utc)-started).total_seconds()*1000), "redaction":{"enabled":True,"policy":"credentials-and-bearer-urls"}, "evidence":{"stdout":"stdout.log","stderr":"stderr.log","artifacts":"artifacts/"}, "status": "passed" if code == 0 else "failed", "exit_code": code, "layer": "e2e" if any("playwright" in arg or "remote_system.py" in arg for arg in command) else "component", "real_onedrive": False, "paid_provider": False, "deployment": False}))
summary = json.loads((run / "summary.json").read_text())
summary["timeout_seconds"] = timeout_seconds
(run / "summary.json").write_text(json.dumps(summary))
print(redact(stdout), end="")
print(redact(stderr), end="", file=sys.stderr)
print("Evidence: " + str(run.relative_to(root)))
sys.exit(code)
