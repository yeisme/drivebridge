// Test-only preparation for the desktop end-to-end suite: rebuild the owner CLI
// into the host bundle directory and refresh bundle.json digests so the packaged
// host verifies the binaries it actually ships.
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { readFile, writeFile, chmod } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const desktop = join(here, '..');
const repo = join(desktop, '..');
const bundleDir = join(desktop, 'host-bin');

const drivebridge = join(bundleDir, 'drivebridge');
execFileSync('go', ['build', '-trimpath', '-o', drivebridge, './cmd/drivebridge'], {
  cwd: repo,
  env: { ...process.env, CGO_ENABLED: '0', GOFLAGS: '', GOWORK: 'off' },
  stdio: 'inherit',
});
await chmod(drivebridge, 0o700);

const digest = async path => 'sha256:' + createHash('sha256').update(await readFile(path)).digest('hex');
const manifestRaw = execFileSync(join(bundleDir, 'pinax'), ['api', 'manifest', '--json'], { cwd: repo, encoding: 'utf8' });
const manifest = JSON.parse(manifestRaw);
if (manifest.status !== 'success' || !/^sha256:[a-f0-9]{64}$/.test(manifest.facts.digest)) throw new Error('owner manifest invalid');

const bundle = {
  schema_version: 'drivebridge.desktop.bundle.v1',
  drivebridge: await digest(drivebridge),
  pinax: await digest(join(bundleDir, 'pinax')),
  manifest: manifest.facts.digest,
};
await writeFile(join(bundleDir, 'bundle.json'), JSON.stringify(bundle, null, 2) + '\n');
console.log(JSON.stringify({ bundle: bundleDir, drivebridge: bundle.drivebridge, manifest: bundle.manifest }));
