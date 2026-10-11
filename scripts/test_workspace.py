import argparse
import ast
import json
import os
import socket
import unittest
from unittest.mock import patch

import workspace


class WorkspaceSafetyTests(unittest.TestCase):
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
        self.assertEqual(scripts['test'], 'python3 scripts/check.py -- pnpm exec vitest run')
        self.assertEqual(scripts['test:e2e'], 'python3 scripts/check.py -- pnpm exec playwright test')

    def test_aggregate_commands_exclude_race_and_nested_runners(self):
        for action in ('test', 'check', 'build', 'e2e'):
            with self.subTest(action=action):
                with patch.object(workspace, 'run', return_value=0) as execute, patch.object(workspace.sys, 'argv', ['workspace.py', action]):
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
