#!/usr/bin/env python3
"""Real stdio JSON-RPC smoke test; no configured cloud account is accessed."""
import json
import os
import pathlib
import select
import subprocess
import tempfile

root = pathlib.Path(__file__).resolve().parents[1]
with tempfile.TemporaryDirectory(prefix="drivebridge-stdio-") as temp:
    binary = str(pathlib.Path(temp) / "drivebridge")
    subprocess.run(["go", "build", "-o", binary, "./cmd/drivebridge"], cwd=root, check=True)
    proc = subprocess.Popen([binary, "mcp", "--state", str(pathlib.Path(temp) / "state")], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    def rpc(i, method, params):
        proc.stdin.write(json.dumps({"jsonrpc": "2.0", "id": i, "method": method, "params": params}) + "\n")
        proc.stdin.flush()
        assert select.select([proc.stdout], [], [], 30)[0], "MCP response timed out"
        response = json.loads(proc.stdout.readline())
        assert response.get("id") == i and "error" not in response, "MCP request failed"
        return response["result"]
    try:
        rpc(1, "initialize", {"protocolVersion": "2024-11-05", "capabilities": {}, "clientInfo": {"name": "fixture", "version": "1"}})
        proc.stdin.write(json.dumps({"jsonrpc": "2.0", "method": "notifications/initialized"}) + "\n")
        proc.stdin.flush()
        listing = rpc(2, "tools/list", {})
        names = {tool["name"] for tool in listing["tools"]}
        assert {"upload", "prepare_reference", "save_output", "resume"}.issubset(names)
        result = rpc(3, "tools/call", {"name": "capabilities", "arguments": {}})
        caps = json.loads(result["content"][0]["text"])
        assert caps["enabled"] is False and caps["secure_handoff"] is False
        assert {t["name"]: t for t in caps["tools"]} == {t["name"]: t for t in listing["tools"]}, "registry/resource drift"
        resource = rpc(4, "resources/read", {"uri": "drivebridge://capabilities"})
        assert {t["name"]: t for t in json.loads(resource["contents"][0]["text"])["tools"]} == {t["name"]: t for t in listing["tools"]}
        disabled = rpc(5, "tools/call", {"name": "upload", "arguments": {"space_id": "test", "local_path": "file", "name": "file", "idempotency_key": "one"}})
        assert disabled["isError"]
        assert "disabled" in disabled["content"][0]["text"]
        print("PASS: real stdio discovery, schemas, resources, and disabled mutation")
    finally:
        proc.stdin.close()
        try:
            proc.wait(timeout=10)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait()
