import importlib.util
import tempfile
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location('prototype_snapshot', Path(__file__).with_name('prototype_snapshot.py'))
assert spec is not None and spec.loader is not None
snapshot = importlib.util.module_from_spec(spec)
spec.loader.exec_module(snapshot)
capture, references = snapshot.capture, snapshot.references

ENTRY = 'https://example.test/api/projects/drivebridge/raw/index.html'


class SnapshotTests(unittest.TestCase):
    def test_unique_direct_assets(self):
        html = '<script src="./assets/app.js?v=1"></script><link rel="stylesheet" href="./a.css"><script src="./assets/app.js?v=1"></script>'
        self.assertEqual(references(ENTRY, html), [ENTRY, ENTRY.replace('index.html', 'assets/app.js?v=1'), ENTRY.replace('index.html', 'a.css')])

    def test_rejects_external_or_parent_assets(self):
        for path in ['https://other.test/app.js', '../app.js', 'file:///app.js', '//other.test/a.js', './app.js#fragment']:
            with self.subTest(path=path), self.assertRaises(ValueError):
                references(ENTRY, f'<script src="{path}"></script>')

    def test_capture_generates_manifest_after_double_read(self):
        calls = []
        def fetch(url):
            calls.append(url)
            return b'<script src="./a.js"></script>' if url == ENTRY else b'export default 1;'
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'snapshot'
            manifest = capture(ENTRY, output, fetch)
            self.assertEqual(len(calls), 4)
            self.assertTrue(manifest['stable_double_read'])
            self.assertFalse(manifest['business_verified'])
            self.assertEqual(len(manifest['assets']), 2)
            self.assertTrue((output / 'snapshot-manifest.json').exists())

    def test_drift_does_not_write_accepted_snapshot(self):
        calls = 0
        def fetch(_):
            nonlocal calls
            calls += 1
            return b'first' if calls == 1 else b'changed'
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'snapshot'
            with self.assertRaises(ValueError):
                capture(ENTRY, output, fetch)
            self.assertFalse(output.exists())


if __name__ == '__main__':
    unittest.main()
