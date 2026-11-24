import argparse
import ast
import json
import os
import socket
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import workspace


class WorkspaceSafetyTests(unittest.TestCase):
    def test_browser_variants_cover_opt_in_and_disabled_scenarios(self):
        self.assertEqual(workspace.browser_variants('governance.spec.ts'), [(['--config', 'playwright.governance.config.ts'], {})])
        for name in ('teams.spec.ts', 'users-management.spec.ts', 'workspace-access.spec.ts'):
            self.assertEqual(workspace.browser_variants(name), [([], {'DRIVEBRIDGE_TEST_TEAM_DEMO': '1'})])
        self.assertEqual(workspace.browser_variants('compact-workspace.spec.ts'), [([], {})])
        self.assertEqual(workspace.browser_variants('key-copy.spec.ts'), [
            (['--grep', 'own key supports'], {}),
            (['--grep', 'team demo keeps'], {'DRIVEBRIDGE_TEST_TEAM_DEMO': '1'})])
        for name, flag in [('label-dictionary.spec.ts', 'LABEL_MANAGEMENT'),
                           ('trash-holds.spec.ts', 'HOLD_MANAGEMENT'),
                           ('transfer-tasks.spec.ts', 'TRANSFER_PLANS'),
                           ('file-properties.spec.ts', 'PROPERTY_EDITING'),
                           ('property-migration.spec.ts', 'PROPERTY_EDITING'),
                           ('submission-inbox.spec.ts', 'ARCHIVE_FAILURE')]:
            self.assertEqual(workspace.browser_variants(name), [([], {'DRIVEBRIDGE_TEST_' + flag: value}) for value in ('0', '1')])

    def test_busy_port_never_starts_or_stops_processes(self):
        with socket.socket() as occupied:
            occupied.bind(('127.0.0.1',0))
            port=occupied.getsockname()[1]
            with patch.object(workspace.subprocess,'Popen') as start:
                with self.assertRaisesRegex(SystemExit,'port is in use'):
                    workspace.dev(argparse.Namespace(port=port,api_port=port+1 if port<65535 else port-1))
                start.assert_not_called()

    def test_race_inherited_through_goflags_is_refused(self):
        with patch.dict(os.environ,{'GOFLAGS':'-race'}):
            with self.assertRaisesRegex(SystemExit,'GOFLAGS'):
                workspace.environment()

    def test_failed_check_preserves_exit_status(self):
        with patch.object(workspace.subprocess,'run') as execute:
            execute.return_value.returncode=23
            with self.assertRaises(SystemExit) as failure:
                workspace.run(['go','test','./...'])
            self.assertEqual(failure.exception.code,23)

    def test_module_failure_does_not_skip_web_tests(self):
        with patch.object(workspace,'run',side_effect=[0,23,0]) as execute:
            with patch.object(workspace.sys,'argv',['workspace.py','test']):
                with self.assertRaises(SystemExit) as failure:
                    workspace.main()
            self.assertEqual(failure.exception.code,23)
            self.assertEqual(execute.call_count,3)
            self.assertEqual(execute.call_args_list[-1].args[0],['pnpm','--dir','web','exec','vitest','run'])

    def test_web_runner_default_is_vitest(self):
        tree = ast.parse((workspace.ROOT / 'web/scripts/check.py').read_text())
        assignments = [node for node in tree.body if isinstance(node, ast.Assign)
                       and any(isinstance(target, ast.Name) and target.id == 'command' for target in node.targets)]
        self.assertEqual(len(assignments), 1)
        default = assignments[0].value
        assert isinstance(default, ast.BoolOp)
        self.assertIsInstance(default.op, ast.Or)
        self.assertEqual(ast.literal_eval(default.values[-1]), ['pnpm', 'exec', 'vitest', 'run'])
        scripts = json.loads((workspace.ROOT / 'web/package.json').read_text())['scripts']
        self.assertEqual(scripts['test'], 'python3 scripts/check.py -- pnpm exec vitest run --no-file-parallelism')
        self.assertEqual(scripts['test:e2e'], 'python3 scripts/check.py -- pnpm exec playwright test')

    def test_aggregate_commands_exclude_race_and_nested_runners(self):
        for action in ('test', 'check', 'build', 'e2e'):
            with self.subTest(action=action):
                with patch.object(workspace, 'run', return_value=0) as execute, patch.object(workspace.shutil, 'copytree'), patch('builtins.print'), patch.object(workspace.sys, 'argv', ['workspace.py', action]):
                    workspace.main()
                self.assertGreater(execute.call_count, 0)
                for call in execute.call_args_list:
                    command = call.args[0]
                    self.assertFalse(any(arg == '-race' or arg.startswith('-race=') for arg in command))
                    self.assertFalse(any(arg.endswith('check.py') for arg in command))
                    if command[0] == 'pnpm':
                        self.assertNotIn('test', command[:4])
                        self.assertNotIn('test:e2e', command)
                        if 'build' in command:
                            scripts = json.loads((workspace.ROOT / 'web/package.json').read_text())['scripts']
                            self.assertEqual(scripts['build'], 'tsc --noEmit && vite build')

    def test_e2e_serves_a_snapshot_even_if_another_build_changes_dist(self):
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder)
            dist=root/'web/dist'
            dist.mkdir(parents=True)
            (dist/'index.html').write_text('tested build')
            (root/'web/tests').mkdir()
            (root/'web/tests/fixture.spec.ts').write_text('')
            observed=[]
            def execute(command, **options):
                if 'playwright' in command:
                    snapshot=Path(options['env_updates']['DRIVEBRIDGE_WEB_DIR'])
                    (dist/'index.html').write_text('other build')
                    self.assertEqual((snapshot/'index.html').read_text(), 'tested build')
                    observed.append(snapshot)
                return 0
            with patch.object(workspace, 'ROOT', root), patch.object(workspace, 'run', side_effect=execute), patch.object(workspace.sys, 'argv', ['workspace.py','e2e']):
                workspace.main()
            self.assertEqual(len(observed),1)
            self.assertFalse(observed[0].exists())

    def test_e2e_keeps_all_specs_and_returns_the_original_failure(self):
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder)
            (root/'web/dist').mkdir(parents=True)
            (root/'web/tests').mkdir()
            for name in ('a.spec.ts','b.spec.ts'):
                (root/'web/tests'/name).write_text('')
            observed=[]
            def execute(command, **options):
                if 'playwright' in command:
                    observed.append(command[-1])
                    return 23 if command[-1]=='tests/a.spec.ts' else 0
                return 0
            with patch.object(workspace, 'ROOT', root), patch.object(workspace, 'run', side_effect=execute), patch.object(workspace.sys, 'argv', ['workspace.py','e2e']), patch('builtins.print'):
                with self.assertRaises(SystemExit) as failure:
                    workspace.main()
            self.assertEqual(observed,['tests/a.spec.ts','tests/b.spec.ts'])
            self.assertEqual(failure.exception.code,23)

    @unittest.skipUnless(os.name=='posix','POSIX process groups')
    def test_cleanup_targets_owned_group_even_after_leader_exits(self):
        from unittest.mock import Mock
        process=Mock(pid=98765)
        process.poll.return_value=0
        with patch.object(workspace.os,'killpg') as kill:
            workspace.stop(process)
            kill.assert_called_once_with(98765,workspace.signal.SIGINT)


if __name__=='__main__':
    unittest.main()
