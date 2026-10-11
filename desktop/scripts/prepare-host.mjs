// Development resources only. Release signing/trusted package assembly is D07.
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { mkdir, chmod, readFile, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { join } from 'node:path';
const root = fileURLToPath(new URL('../../', import.meta.url));
const target = join(root, 'desktop', 'host-bin');
if (/(?:^|\s)-race(?:\s|=|$)/.test(process.env.GOFLAGS || '')) throw new Error('Race instrumentation is not authorized.');
await mkdir(target, { recursive: true, mode: 0o700 });
const env = { ...process.env, CGO_ENABLED: '0', GOWORK: 'off' };
const suffix = process.platform === 'win32' ? '.exe' : '';
const digests = {};
for (const [name, cwd] of [['drivebridge', root], ['pinax', join(root, '../pinax')]]) {
  const file = join(target, name + suffix);
  execFileSync('go', ['build', '-trimpath', '-o', file, `./cmd/${name}`], { cwd, env, timeout: 120000, stdio: ['ignore', 'ignore', 'pipe'] });
  await chmod(file, 0o700);
  digests[name] = 'sha256:' + createHash('sha256').update(await readFile(file)).digest('hex');
}
const projection = JSON.parse(execFileSync(join(target, 'pinax' + suffix), ['api', 'manifest', '--json'], { cwd: target, env: {}, timeout: 15000, maxBuffer: 8 * 1024 * 1024, encoding: 'utf8' }));
if (projection.status !== 'success' || !/^sha256:[a-f0-9]{64}$/.test(projection.facts?.digest)) throw new Error('Owner manifest export failed.');
await writeFile(join(target, 'bundle.json'), JSON.stringify({ schema_version: 'drivebridge.desktop.bundle.v1', drivebridge: digests.drivebridge, pinax: digests.pinax, manifest: projection.facts.digest }, null, 2) + '\n', { mode: 0o600 });
console.log('Development host resources prepared; not signed or released.');
