import importlib.util
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('workspace_visual_baseline', Path(__file__).with_name('workspace_visual_baseline.py'))
assert spec is not None and spec.loader is not None
baseline = importlib.util.module_from_spec(spec)
spec.loader.exec_module(baseline)


class VisualBaselineTests(unittest.TestCase):
    def test_invalid_observations_fail_explicitly(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            with self.assertRaises(ValueError):
                baseline.load_observations(root, 'missing.json', 'observations')
            for value in ('broken', '{}', '{"observations": {}}'):
                (root / 'input.json').write_text(value)
                with self.assertRaises(ValueError):
                    baseline.load_observations(root, 'input.json', 'observations')
            (root / 'input.json').write_text(json.dumps({'observations': []}))
            self.assertEqual(baseline.load_observations(root, 'input.json', 'observations'), [])

    def test_report_records_missing_coverage_without_claiming_completion(self):
        text = baseline.render('reference', 'product', [], [])
        self.assertIn('不是功能验收', text)
        self.assertIn('尚未单独捕获，保留补验', text)
        self.assertIn('凭据显示页面禁止截图', text)
        self.assertIn('Pinax真实接入', text)
        self.assertEqual(len(baseline.SURFACES), 17)

    def test_comparison_requires_every_view_and_explicit_clean_observation(self):
        rows = [{'route': route, 'width': width, 'theme': theme, 'overflow': False, 'errors': [], 'stable': True, 'expectedState': 'Ready', 'text': 'Ready'}
                for _, route, _, _ in baseline.SURFACES
                for width in (320, 390, 768, 1280, 1440, 1920) for theme in ('light', 'dark')]
        text = baseline.render_comparison('reference', 'current', rows, [])
        self.assertIn('204观察', text)
        self.assertIn('不是业务验收', text)
        self.assertEqual(set(baseline.COMPARISON_SCOPE), {ref for ref, _, _, _ in baseline.SURFACES})
        for invalid in (rows[:-1], rows + [rows[0]], [{**rows[0], 'overflow': True}, *rows[1:]],
                        [{**rows[0], 'overflow': 0}, *rows[1:]], [{**rows[0], 'errors': ['failed']}, *rows[1:]],
                        [{**rows[0], 'stable': False}, *rows[1:]], [{**rows[0], 'stable': 1}, *rows[1:]],
                        [{**rows[0], 'expectedState': ''}, *rows[1:]], [{**rows[0], 'text': '读取工作台摘要… Ready'}, *rows[1:]],
                        [{**rows[0], 'text': 'Different page'}, *rows[1:]]):
            with self.assertRaises(ValueError):
                baseline.render_comparison('reference', 'current', invalid, [])

    def test_details_require_actual_images_and_do_not_reuse_desktop_reference(self):
        row = ('Fixed preview', 'details', 'reference-detail-file-preview.png', 'preview-{width}.png', 'Preserve original', 'files/content', 'file-preview-layout')
        with tempfile.TemporaryDirectory() as directory, patch.object(baseline, 'DETAILS', (row,)):
            root = Path(directory)
            for run, names in [('details', ['reference-detail-file-preview.png']), ('passed-run', ['preview-1440.png', 'preview-390.png'])]:
                artifacts = root / 'temp/integration-test-runs' / run / 'artifacts'
                artifacts.mkdir(parents=True)
                for name in names:
                    (artifacts / name).write_bytes(b'fixture-image')
            report, gallery = baseline.render_details(root, 'reference', 'details', None, ['passed-run'])
            self.assertIn('390：未观察到独立对应图', report)
            self.assertEqual(gallery.count('reference-detail-file-preview.png'), 1)
            self.assertIn('preview-390.png', gallery)
            (root / 'temp/integration-test-runs/passed-run/artifacts/preview-390.png').unlink()
            with self.assertRaisesRegex(ValueError, 'Missing implementation screenshot'):
                baseline.render_details(root, 'reference', 'details', None, ['passed-run'])

    def test_supplement_rejects_transient_reference_details(self):
        valid = {'stable': True, 'text': '文件活动 暂无活动记录。', 'errors': [], 'blocked': []}
        baseline.validate_supplement([valid])
        for invalid in ([], [{**valid, 'stable': False}], [{**valid, 'stable': 1}],
                        [{**valid, 'text': '正在读取活动 · 本地演示'}],
                        [{**valid, 'text': '正在读取版本 · 本地演示'}],
                        [{**valid, 'text': '正在读取任务详情 · 本地演示'}],
                        [{**valid, 'errors': ['failed']}], [{**valid, 'blocked': ['unknown.js']}]):
            with self.assertRaises(ValueError):
                baseline.validate_supplement(invalid)

    def test_gallery_does_not_invent_missing_reference(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(baseline, 'ROOT', Path(directory)):
            page = baseline.comparison_gallery('reference', 'current', None)
            self.assertIn('No independent frozen screenshot', page)
            self.assertIn('product-dark-390-agents.png', page)
            self.assertNotIn('src="http', page)
            old = baseline.comparison_gallery('reference', 'current', 'desktop-only')
            self.assertNotIn('reference-detail-search.png', old)
            with self.assertRaises(ValueError):
                baseline.comparison_gallery('reference', 'current', None, 'missing-supplement')


if __name__ == '__main__':
    unittest.main()
