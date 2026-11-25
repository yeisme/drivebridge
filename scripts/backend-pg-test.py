#!/usr/bin/env python3
"""Run the service suite against a disposable PostgreSQL container."""
import argparse
import os
import signal
import socket
import subprocess
import sys
import time


def free_port():
    sock = socket.socket()
    sock.bind(("127.0.0.1", 0))
    port = sock.getsockname()[1]
    sock.close()
    return port


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--run', default='.', help='Go test selection; default runs the entire service suite')
    parser.add_argument('--test-timeout', default='30m', help='Go test timeout, including opt-in scale runs')
    parser.add_argument('--skip', default='', help='Diagnostic exclusion regex; never a full-suite acceptance result')
    args = parser.parse_args()
    def interrupted(*_):
        raise KeyboardInterrupt
    signal.signal(signal.SIGTERM, interrupted)
    root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    port = free_port()
    container = subprocess.check_output([
        "docker", "run", "--rm", "-d", "-e", "POSTGRES_PASSWORD=drivebridge-test",
        "-e", "POSTGRES_DB=drivebridge", "-p", f"127.0.0.1:{port}:5432", "postgres:18-bookworm",
    ], text=True).strip()
    try:
        deadline = time.time() + 60
        while time.time() < deadline:
            probe = subprocess.run(["docker", "exec", container, "pg_isready", "-U", "postgres", "-d", "drivebridge"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            if probe.returncode == 0:
                break
            time.sleep(1)
        else:
            raise SystemExit("postgres fixture did not become ready")
        env = dict(os.environ)
        env["DRIVEBRIDGE_TEST_DATABASE"] = "postgres"
        env["DRIVEBRIDGE_TEST_PG_DSN"] = f"postgres://postgres:drivebridge-test@127.0.0.1:{port}/drivebridge?sslmode=disable"
        command = ["go", "-C", "server", "test", "./internal/service", "-run", args.run, "-count=1", "-timeout=" + args.test_timeout]
        if args.skip:
            command += ['-skip', args.skip]
            print('diagnostic_exclusions=' + args.skip, flush=True)
        print('backend_database=postgres suite=service selection=' + args.run, flush=True)
        return subprocess.run(command, cwd=root, env=env).returncode
    finally:
        subprocess.run(["docker", "rm", "-f", container], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


if __name__ == "__main__":
    sys.exit(main())
