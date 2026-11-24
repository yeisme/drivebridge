#!/usr/bin/env python3
"""Develop and verify the complete DriveBridge repository from one directory."""
import argparse
import os
from pathlib import Path
import signal
import shlex
import shutil
import socket
import subprocess
import sys
import tempfile
import time
import urllib.request

ROOT = Path(__file__).resolve().parents[1]


def browser_variants(name):
    """Run every conditional scenario with its required disposable fixture."""
    team_specs = {
        'activity-folder-polish.spec.ts', 'admin-takeover.spec.ts',
        'compact-governance.spec.ts', 'creation-wizard.spec.ts',
        'direct-actions.spec.ts', 'interaction-polish.spec.ts',
        'lazy-user-directory.spec.ts', 'shared-form-layout.spec.ts',
        'team-quota.spec.ts', 'teams-layout.spec.ts', 'teams.spec.ts',
        'users-management.spec.ts', 'workspace-access.spec.ts',
    }
    if name in team_specs:
        return [([], {'DRIVEBRIDGE_TEST_TEAM_DEMO': '1'})]
    if name == 'key-copy.spec.ts':
        return [(['--grep', 'own key supports'], {}),
                (['--grep', 'team demo keeps'], {'DRIVEBRIDGE_TEST_TEAM_DEMO': '1'})]
    if name == 'governance.spec.ts':
        return [(['--config', 'playwright.governance.config.ts'], {})]
    if name == 'demo.spec.ts':
        return [([], {'DRIVEBRIDGE_TEST_DEMO': '1'})]
    if name == 'automation-management.spec.ts':
        return [([], {'DRIVEBRIDGE_TEST_AUTOMATION_MANAGEMENT': '1'})]
    modes = {
        'label-dictionary.spec.ts': 'LABEL_MANAGEMENT',
        'trash-holds.spec.ts': 'HOLD_MANAGEMENT',
        'transfer-tasks.spec.ts': 'TRANSFER_PLANS',
        'file-properties.spec.ts': 'PROPERTY_EDITING',
        'property-migration.spec.ts': 'PROPERTY_EDITING',
        'submission-inbox.spec.ts': 'ARCHIVE_FAILURE',
    }
    if name in modes:
        return [([], {'DRIVEBRIDGE_TEST_' + modes[name]: enabled}) for enabled in ('0', '1')]
    return [([], {})]


def environment():
    if any(flag == '-race' or flag.startswith('-race=') for flag in shlex.split(os.environ.get('GOFLAGS', ''))):
        raise SystemExit('Remove race detection from GOFLAGS before using this workflow.')
    return {**os.environ, 'GOWORK': 'off', 'CGO_ENABLED': '0',
            'NODE_OPTIONS': (os.environ.get('NODE_OPTIONS', '')+' --no-experimental-webstorage').strip()}


def run(command, *, evidence=True, allow_failure=False, env_updates=None):
    if any(arg == '-race' or arg.startswith('-race=') for arg in command):
        raise SystemExit('Race detection is excluded from this workflow.')
    call = [sys.executable, str(ROOT/'scripts/check.py'), '--', *command] if evidence else command
    result = subprocess.run(call, cwd=ROOT, env={**environment(), **(env_updates or {})})
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
        env['DRIVEBRIDGE_DEV_API_ORIGIN']=f'http://{args.host}:{args.api_port}'
        try:
            processes.append(subprocess.Popen([str(binary),'--listen',f'{args.host}:{args.api_port}',
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
            processes.append(subprocess.Popen(['pnpm','--dir','web','exec','vite','--host',args.host,'--port',str(args.port),'--strictPort'],cwd=ROOT,env=env,start_new_session=os.name=='posix'))
            print(f'DriveBridge development: http://{args.host}:{args.port} (disposable local fixtures)',flush=True)
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
    dev_parser.add_argument('--host',default='127.0.0.1')
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
        # Other local quality lanes may rebuild dist while browser tests run.
        # Serve one immutable build for this run, never a changing asset tree.
        with tempfile.TemporaryDirectory(prefix='drivebridge-web-e2e-') as folder:
            snapshot=Path(folder)/'web'
            shutil.copytree(ROOT/'web/dist', snapshot, symlinks=True)
            specs=sorted((ROOT/'web/tests').glob('*.spec.ts'))
            if not specs:
                raise SystemExit('No browser specifications were found.')
            codes=[]
            # Each spec owns its backend data. Shared seeded identities must
            # not inherit another spec's rotations, quotas or account lifecycle.
            for spec in specs:
                for flags, fixture in browser_variants(spec.name):
                    print(f'Browser specification: {spec.name}; fixture: {fixture or "personal"}', flush=True)
                    env = {'DRIVEBRIDGE_TEST_' + key: '0' for key in (
                        'TEAM_DEMO', 'DEMO', 'AUTOMATION_MANAGEMENT', 'LABEL_MANAGEMENT',
                        'HOLD_MANAGEMENT', 'TRANSFER_PLANS', 'ARCHIVE_FAILURE')}
                    env.update({'DRIVEBRIDGE_WEB_DIR':str(snapshot),
                        'DRIVEBRIDGE_TEST_PROPERTY_EDITING':'1',
                        'DRIVEBRIDGE_TEST_SHARE_RECEIVERS':'1', **fixture})
                    codes.append(run(['pnpm','--dir','web','exec','playwright','test','tests/'+spec.name, *flags],
                        allow_failure=True, env_updates=env))
            print(f'Browser specifications: {len(specs)}; fixture runs: {len(codes)}; failed: {sum(code != 0 for code in codes)}', flush=True)
            if any(codes):
                raise SystemExit(next(code for code in codes if code))


if __name__=='__main__':
    main()
