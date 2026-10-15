#!/usr/bin/env python3
"""Install the explicitly configured DriveBridge LAN service, without reading secrets."""
import argparse
import os
from pathlib import Path
import subprocess

parser = argparse.ArgumentParser(description=__doc__)
for name in ("binary", "state", "config", "web-dir"):
    parser.add_argument("--" + name, required=True)
parser.add_argument("--origin", required=True)
parser.add_argument("--listen", default="127.0.0.1:48091")
parser.add_argument("--allow-http", action="store_true")
parser.add_argument("--apply", action="store_true")
args = parser.parse_args()
if args.origin.startswith("http://") and not args.allow_http:
    parser.error("HTTP cookie transport requires explicit --allow-http")
paths = {key: Path(getattr(args, key)).absolute() for key in ("binary", "state", "config", "web_dir")}
for value in paths.values():
    if not value.exists():
        parser.error("Required deployment path does not exist")
features = ["admin-direct-write", "document-editing", "property-editing", "automation-management", "share-receivers", "hold-management", "label-management", "cloud-connectors", "transfer-plans"]
command = [str(paths["binary"]), "--config", str(paths["config"]), "--state", str(paths["state"]), "serve", "--enabled", "--listen", args.listen, "--origin", args.origin, "--web-dir", str(paths["web_dir"])]
if args.allow_http:
    command.append("--development")
command.extend("--" + feature for feature in features)

def quote(value):
    if any(ch in value for ch in "\n\r\0"):
        raise ValueError("Invalid unit argument")
    return '"' + value.replace('\\', '\\\\').replace('"', '\\"').replace('%', '%%') + '"'

unit = "\n".join([
    "[Unit]", "Description=DriveBridge LAN file workspace", "After=network-online.target", "",
    "[Service]", "Type=simple", "ExecStart=" + " ".join(map(quote, command)),
    "Restart=on-failure", "RestartSec=3", "TimeoutStopSec=45", "UMask=0077", "",
    "[Install]", "WantedBy=default.target", "",
])
destination = Path.home() / ".config/systemd/user/drivebridge-server-lan.service"
if not args.apply:
    print("Validated service configuration. Add --apply after stopping the existing server and backing up state.")
else:
    destination.parent.mkdir(parents=True, exist_ok=True)
    temporary = destination.with_suffix(".tmp")
    temporary.write_text(unit)
    temporary.chmod(0o600)
    temporary.replace(destination)
    env = dict(os.environ, XDG_RUNTIME_DIR=f"/run/user/{os.getuid()}", DBUS_SESSION_BUS_ADDRESS=f"unix:path=/run/user/{os.getuid()}/bus")
    subprocess.run(["systemctl", "--user", "daemon-reload"], env=env, check=True)
    subprocess.run(["systemctl", "--user", "enable", "--now", destination.name], env=env, check=True)
    print("Installed drivebridge-server-lan.service with explicit human management features; no demo login enabled.")
