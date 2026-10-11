#!/usr/bin/env python3
"""Develop and verify the complete DriveBridge repository from one directory."""
import argparse
import os
from pathlib import Path
import signal
import shlex
import socket
import subprocess
import sys
import tempfile
import time
import urllib.request

ROOT = Path(__file__).resolve().parents[1]


def environment():
    if any(flag == '-race' or flag.startswith('-race=') for flag in shlex.split(os.environ.get('GOFLAGS', ''))):
        raise SystemExit('Remove race detection from GOFLAGS before using this workflow.')
    return {**os.environ, 'GOWORK': 'off', 'CGO_ENABLED': '0',
            'NODE_OPTIONS': (os.environ.get('NODE_OPTIONS', '')+' --no-experimental-webstorage').strip()}


def run(command, *, evidence=True, allow_failure=False):
    if any(arg == '-race' or arg.startswith('-race=') for arg in command):
        raise SystemExit('Race detection is excluded from this workflow.')
    call = [sys.executable, str(ROOT/'scripts/check.py'), '--', *command] if evidence else command
    result = subprocess.run(call, cwd=ROOT, env=environment())
    if result.returncode and not allow_failure:
        raise SystemExit(result.returncode)
    return result.returncode


def build():
    (ROOT/'temp/build').mkdir(parents=True, exist_ok=True)
    run(['go','build','-o','temp/build/drivebridge','./cmd/drivebridge'])
    run(['go','-C','server','build','-o','../temp/build/drivebridge-server','./cmd/drivebridge-server'])
    run(['pnpm','--dir','web','build'])


def available(port):
    with socket.socket() as probe:
        probe.bind(('127.0.0.1', port))


def stop(process):
    if os.name == 'posix':
        try:
            os.killpg(process.pid, signal.SIGINT)
        except ProcessLookupError:
            return
    else:
        if process.poll() is not None:
            return
        process.terminate()
    try:
        process.wait(timeout=5)
    except subprocess.TimeoutExpired:
        if os.name == 'posix':
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
        else:
            process.kill()
        process.wait()


def dev(args):
    if args.port == args.api_port:
        raise SystemExit('Web and API ports must be different.')
    try:
        available(args.port); available(args.api_port)
    except OSError:
        raise SystemExit('A requested port is in use; choose other ports. Existing processes were not stopped.')
    processes=[]
    def interrupted(*_):
        raise KeyboardInterrupt
    signal.signal(signal.SIGTERM,interrupted)
    with tempfile.TemporaryDirectory(prefix='drivebridge-dev-') as folder:
        binary=Path(folder)/('fixture.exe' if os.name=='nt' else 'fixture')
        run(['go','-C','server','build','-o',str(binary),'./cmd/drivebridge-fixture'], evidence=False)
        env=environment()
        env['DRIVEBRIDGE_DEV_API_ORIGIN']=f'http://127.0.0.1:{args.api_port}'
        try:
            processes.append(subprocess.Popen([str(binary),'--listen',f'127.0.0.1:{args.api_port}',
                '--public-origin',f'http://127.0.0.1:{args.port}','--demo'],cwd=ROOT/'server',env=env,start_new_session=os.name=='posix'))
            deadline=time.monotonic()+30
            while time.monotonic()<deadline:
                if processes[0].poll() is not None:
                    raise RuntimeError('Fixture exited before readiness.')
                try:
                    with urllib.request.urlopen(f'http://127.0.0.1:{args.api_port}/healthz',timeout=1) as response:
                        if response.status==200:
                            break
                except OSError:
                    time.sleep(.1)
            else:
                raise RuntimeError('Fixture readiness timed out.')
            processes.append(subprocess.Popen(['pnpm','--dir','web','exec','vite','--host','127.0.0.1','--port',str(args.port),'--strictPort'],cwd=ROOT,env=env,start_new_session=os.name=='posix'))
            print(f'DriveBridge development: http://127.0.0.1:{args.port} (disposable local fixtures)',flush=True)
            while all(p.poll() is None for p in processes):
                time.sleep(.3)
            if any(p.returncode not in (None,0) for p in processes):
                raise RuntimeError('A development process exited unexpectedly.')
        except KeyboardInterrupt:
            pass
        finally:
            for process in reversed(processes):
                stop(process)


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    sub=parser.add_subparsers(dest='command',required=True)
    dev_parser=sub.add_parser('dev',help='Start Vite and disposable Server fixtures; Ctrl+C stops both')
    dev_parser.add_argument('--port',type=int,default=5173)
    dev_parser.add_argument('--api-port',type=int,default=8787)
    for name,help_text in [('build','Build CLI, Server and Web'),('test','Run ordinary CLI, Server and Web tests'),('e2e','Run local MCP/HTTP and browser user flows'),('check','Run tests, static checks and builds')]:
        sub.add_parser(name,help=help_text)
    args=parser.parse_args()
    if args.command=='dev':
        if not all(1 <= port <= 65535 for port in (args.port,args.api_port)):
            parser.error('Ports must be between 1 and 65535.')
        dev(args); return
    if args.command in ('test','check'):
        # Collect evidence from every module even if an earlier module fails.
        codes=[run(command,allow_failure=True) for command in [
            ['go','test','./...'],
            ['go','-C','server','test','./...'],
            ['pnpm','--dir','web','exec','vitest','run']]]
        if any(codes):
            raise SystemExit(next(code for code in codes if code))
    if args.command=='check':
        run(['go','vet','./...'])
        run(['go','-C','server','vet','./...'])
        run(['pnpm','--dir','web','exec','tsc','--noEmit'])
    if args.command in ('build','check'):
        build()
    if args.command=='e2e':
        run(['pnpm','--dir','web','build'])
        run([sys.executable,'scripts/remote_system.py'])
        run(['pnpm','--dir','web','exec','playwright','test'])


if __name__=='__main__':
    main()
