#!/usr/bin/env python3
"""Rehearse the four-step key-only self-hosted activation with real binaries.

Runs offline: platform-connect -> users import -> serve --enabled -> remote
status, then a browser-equivalent smoke (auth mode, key session, space,
upload/download round trip, quota fields) and a read-only backup check.
Plaintext keys stay inside the 0600 distribution CSV under the evidence
artifacts directory; they are never printed to stdout/stderr.
"""
import csv
import hashlib
import http.cookiejar
import json
import os
import pathlib
import select
import signal
import socket
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request

root = pathlib.Path(__file__).resolve().parents[1]
client_dist = root.parent / "web" / "dist"
artifacts = pathlib.Path(os.environ.get("DRIVEBRIDGE_EVIDENCE_ARTIFACTS", tempfile.mkdtemp(prefix="drivebridge-rehearsal-artifacts-")))
artifacts.mkdir(parents=True, exist_ok=True)
processes = []


def step(message):
    print("rehearsal: " + message, flush=True)


def run(args, stdin=None, env=None, timeout=120):
    completed = subprocess.run(args, cwd=root, input=stdin, env=env, capture_output=True, text=True, timeout=timeout)
    if completed.returncode != 0:
        raise SystemExit("command failed: " + " ".join(args[:2]) + "\n" + completed.stderr.strip()[:400])
    return completed


def free_port():
    probe = socket.socket()
    probe.bind(("127.0.0.1", 0))
    port = probe.getsockname()[1]
    probe.close()
    return port


try:
    if not (client_dist / "index.html").is_file():
        step("building web client (dist missing)")
        run(["pnpm", "--dir", str(client_dist.parent), "build"], timeout=600)
    assert (client_dist / "index.html").is_file(), "web dist unavailable"

    temporary = pathlib.Path(tempfile.mkdtemp(prefix="drivebridge-rehearsal-"))
    build_env = dict(os.environ, CGO_ENABLED="0")
    step("building drivebridge-server and fixture binaries")
    run(["go", "build", "-o", str(temporary / "server"), "./cmd/drivebridge-server"], env=build_env, timeout=600)
    run(["go", "build", "-o", str(temporary / "fixture"), "./cmd/drivebridge-fixture"], env=build_env, timeout=600)

    fixture = subprocess.Popen([str(temporary / "fixture"), "--listen", "127.0.0.1:0", "--s3-listen", "127.0.0.1:0"], stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, text=True)
    processes.append(fixture)
    assert select.select([fixture.stderr], [], [], 60)[0], "fixture S3 startup timed out"
    line = fixture.stderr.readline().strip()
    s3 = line.removeprefix("Fixture S3 endpoint at ")
    assert s3.startswith("http://127.0.0.1:"), "fixture S3 endpoint unavailable: " + line
    step("fixture S3 listening")

    state = temporary / "state"
    state.mkdir(0o700)
    step("step 1/4: platform-connect --development")
    run([str(temporary / "server"), "--state", str(state), "platform-connect", "--development", "--json"],
        stdin=json.dumps({"endpoint": s3, "region": "us-east-1", "bucket": "fixture", "prefix": "rehearsal", "access_key": "fixture-key", "secret_key": "fixture-secret"}))

    step("step 2/4: users import (admin + user)")
    dist_csv = artifacts / "dist.csv"
    run([str(temporary / "server"), "--state", str(state), "users", "import", "--csv", "--out", str(dist_csv)],
        stdin="name,days,admin\nrehearsal-admin,0,true\nrehearsal-user,0,false\n")
    assert oct(dist_csv.stat().st_mode & 0o777) == "0o600", "distribution CSV must be 0600"
    keys = {}
    with dist_csv.open() as stream:
        for row in csv.DictReader(stream):
            keys[row["name"]] = row["key"]
    assert set(keys) == {"rehearsal-admin", "rehearsal-user"}, "unexpected distribution rows"
    admin_token = temporary / "admin.key"
    descriptor = os.open(admin_token, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, "w") as stream:
        stream.write(keys["rehearsal-admin"])

    step("preflight doctor")
    run([str(temporary / "server"), "doctor", "--state", str(state), "--web-dir", str(client_dist), "--json"])

    port = free_port()
    origin = "http://127.0.0.1:" + str(port)
    serve_log = (temporary / "serve.log").open("w")
    serve = subprocess.Popen([str(temporary / "server"), "serve", "--state", str(state), "--listen", "127.0.0.1:" + str(port), "--origin", origin, "--web-dir", str(client_dist), "--development", "--enabled"], stdout=serve_log, stderr=serve_log)
    processes.append(serve)
    step("step 3/4: serve --enabled on " + origin)
    for _ in range(150):
        try:
            if urllib.request.urlopen(origin + "/healthz", timeout=1).status == 200:
                break
        except Exception:
            time.sleep(0.2)
    else:
        raise SystemExit("serve did not become healthy")

    step("step 4/4: remote status check")
    status = json.loads(run([str(temporary / "server"), "status", "--url", origin, "--token-file", str(admin_token), "--json"]).stdout)
    assert status.get("status") == "success", "remote status failed"

    jar = http.cookiejar.CookieJar()
    browser = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))

    def call(method, path, body=None, csrf="", raw=False, idem=""):
        headers = {"Origin": origin}
        if csrf:
            headers["X-CSRF-Token"] = csrf
        if idem:
            headers["Idempotency-Key"] = idem
        data = None
        if body is not None:
            data = body if isinstance(body, bytes) else json.dumps(body).encode()
            headers["Content-Type"] = "application/octet-stream" if isinstance(body, bytes) else "application/json"
        request = urllib.request.Request(origin + path, data=data, headers=headers, method=method)
        with browser.open(request, timeout=15) as response:
            payload = response.read()
            return payload if raw else json.loads(payload)["data"]

    step("browser smoke: key-only login")
    mode = call("GET", "/auth/mode")
    assert mode.get("oidc") is False and mode.get("key") is True, "auth mode must report key-only honestly"
    call("POST", "/auth/key", {"key": keys["rehearsal-user"]})
    me = call("GET", "/api/v1/me")
    csrf = me["csrf_token"]

    step("browser smoke: space + upload/download round trip")
    space = call("POST", "/api/v1/spaces", {"name": "rehearsal", "connection_id": "platform"}, csrf)
    payload = b"self-hosted rehearsal payload\n"
    digest = hashlib.sha256(payload).hexdigest()
    operation = call("POST", "/api/v1/uploads", {"space_id": space["id"], "path": "docs/hello.txt", "size_bytes": len(payload), "sha256": digest, "caller": "selfhosted-rehearsal"}, csrf, idem="rehearsal-upload-1")
    call("PUT", "/api/v1/operations/" + operation["id"] + "/parts/0", payload, csrf)
    done = call("POST", "/api/v1/operations/" + operation["id"] + "/complete", {}, csrf)
    assert done.get("status") == "completed" and done.get("file_id"), "upload did not complete"
    replay = call("POST", "/api/v1/uploads", {"space_id": space["id"], "path": "docs/hello.txt", "size_bytes": len(payload), "sha256": digest, "caller": "selfhosted-rehearsal"}, csrf, idem="rehearsal-upload-1")
    assert replay.get("id") == operation["id"], "idempotent replay changed the operation"
    files = call("GET", "/api/v1/files?space_id=" + space["id"])
    entry = next(f for f in files if f["id"] == done["file_id"])
    assert entry.get("caller") == "selfhosted-rehearsal", "caller provenance missing"
    assert call("GET", "/api/v1/files/" + entry["id"] + "/content", raw=True) == payload, "download bytes differ"
    quotas = call("GET", "/api/v1/spaces")
    row = next(s for s in quotas if s["id"] == space["id"])
    assert row.get("used_bytes", 0) >= len(payload) and "max_bytes" in row and "reserved_bytes" in row, "quota fields missing"

    step("stopping serve for read-only backup check")
    serve.terminate()
    serve.wait(timeout=30)
    processes.remove(serve)
    backup = temporary / "backup"
    run([str(temporary / "server"), "backup", "--state", str(state), "--output", str(backup), "--json"])
    run([str(temporary / "server"), "doctor", "--state", str(state), "--backup", str(backup), "--json"])

    step("PASS: key-only activation rehearsal completed (dist.csv in evidence artifacts)")
except Exception as failure:
    print("rehearsal FAILED: " + str(failure), file=sys.stderr, flush=True)
    raise
finally:
    for proc in processes:
        try:
            proc.send_signal(signal.SIGINT)
            proc.wait(timeout=10)
        except Exception:
            proc.kill()
