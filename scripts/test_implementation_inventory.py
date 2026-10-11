import importlib.util
import json
import tempfile
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location('implementation_inventory', Path(__file__).with_name('implementation_inventory.py'))
assert spec is not None and spec.loader is not None
inventory = importlib.util.module_from_spec(spec)
spec.loader.exec_module(inventory)


class ImplementationInventoryTests(unittest.TestCase):
    def fixture(self, root):
        files = {
            'web/src/WorkspaceApp.tsx': 'const navigation: unknown = [["files", "Files"], ["agents", "Agents"]]; function App() {}',
            'web/src/features/agents/LegacyAgentPage.tsx': 'export const old = "/agents";',
            'web/src/generated/personal-workspace.ts': '"/agent-grants"',
            'web/src/client.test.ts': '"/agent-grants"',
            'server/internal/contract/routes.go': 'Method: "get", Path: "/api/v1/agent-grants"\nMethod: "get", Path: "/api/v1/agent-invocations"',
            'server/internal/service/http.go': 'mux.HandleFunc("GET /api/v1/agent-grants", handler)',
            'server/internal/service/example_test.go': 'func TestWorkspaceFixture(t *testing.T) {}',
            'server/sdk/example_test.go': 'func TestOther(t *testing.T) {}',
        }
        for change in inventory.CHANGES:
            files[f'openspec/changes/{change}/tasks.md'] = '- [x] one done\n- [ ] two todo\n'
        for relative, body in files.items():
            path = root / relative
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(body, encoding='utf-8')

    def test_report_is_deterministic_and_never_upgrades_presence(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            self.fixture(root)
            report = inventory.render(root)
            self.assertEqual(report, inventory.render(root))
            self.assertIn('不是运行时 capability', report)
            self.assertIn('目录尚不存在', report)
            self.assertIn('未在字面注册中发现', report)
            grant = next(line for line in report.splitlines() if '`GET /api/v1/agent-grants`' in line)
            self.assertIn('server/internal/service/http.go', grant)
            self.assertIn('未发现（不含生成类型', grant)
            self.assertNotIn('generated/personal-workspace.ts', grant)
            self.assertIn('1/2', report)
            self.assertIn('静态发现 1 个', report)

    def test_consumer_is_only_a_literal_clue(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            self.fixture(root)
            (root / 'web/src/client.ts').write_text('const path = "/agent-grants";', encoding='utf-8')
            report = inventory.render(root)
            self.assertIn('`web/src/client.ts`', report)
            self.assertIn('不证明可达', report)

    def test_evidence_rejects_traversal_incomplete_and_wrong_owner(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            with self.assertRaises(ValueError):
                inventory.evidence_facts(root, '../outside')
            run_id = '20261005T081914Z-fixture'
            directory = root / 'temp/integration-test-runs' / run_id
            directory.mkdir(parents=True)
            with self.assertRaises(ValueError):
                inventory.evidence_facts(root, run_id)
            for name in ('command.txt', 'stdout.log', 'stderr.log', 'env.json'):
                (directory / name).write_text('', encoding='utf-8')
            (directory / 'artifacts').mkdir()
            summary = {'project': 'cli/other', 'run_id': run_id, 'status': 'passed', 'exit_code': 0}
            (directory / 'summary.json').write_text(json.dumps(summary), encoding='utf-8')
            with self.assertRaises(ValueError):
                inventory.evidence_facts(root, run_id)
            summary['project'] = 'cli/drivebridge'
            summary['exit_code'] = 1
            (directory / 'summary.json').write_text(json.dumps(summary), encoding='utf-8')
            with self.assertRaises(ValueError):
                inventory.evidence_facts(root, run_id)
            summary['exit_code'] = 0
            (directory / 'summary.json').write_text(json.dumps(summary), encoding='utf-8')
            (directory / 'stdout.log').write_text('--- PASS: TestWorkspaceFixture (0.01s)\n    --- PASS: TestWorkspaceFixture/child (0.01s)\n--- PASS: TestDocumentFixture (0.01s)\n', encoding='utf-8')
            self.assertEqual(inventory.evidence_facts(root, run_id), ['TestDocumentFixture', 'TestWorkspaceFixture'])


if __name__ == '__main__':
    unittest.main()
