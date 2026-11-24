// Development resources only. Release signing/trusted package assembly is D07.
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { chmod, copyFile, mkdir, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
const root = fileURLToPath(new URL('../../', import.meta.url));
const target = join(root, 'desktop', 'host-bin');
if (/(?:^|\s)-race(?:\s|=|$)/.test(process.env.GOFLAGS || '')) throw new Error('Race instrumentation is not authorized.');
await mkdir(target, { recursive: true, mode: 0o700 });
const env = { ...process.env, CGO_ENABLED: '0', GOWORK: 'off' };
const suffix = process.platform === 'win32' ? '.exe' : '';
const digests = {};
const pinaxCheckout = process.env.DRIVEBRIDGE_PINAX_DIR;
if (!process.env.DRIVEBRIDGE_PINAX_BIN && !pinaxCheckout) throw new Error('Supply DRIVEBRIDGE_PINAX_BIN or an explicit DRIVEBRIDGE_PINAX_DIR candidate.');
const sdkVersion = execFileSync('go', ['list', '-m', '-f', '{{.Version}}', 'github.com/yeisme/pinax'], { cwd: root, encoding: 'utf8', env, timeout: 30000 }).trim();
const builds = [
  ['drivebridge', root, './cmd/drivebridge'],
  ['pinax', pinaxCheckout, './cmd/pinax'],
];
for (const [name, cwd, packagePath] of builds) {
  const file = join(target, name + suffix);
  const supplied = name === 'pinax' && process.env.DRIVEBRIDGE_PINAX_BIN;
  if (supplied) await copyFile(process.env.DRIVEBRIDGE_PINAX_BIN, file);
  else {
    const buildCwd = cwd;
    const buildPackage = packagePath;
    execFileSync('go', ['build', '-trimpath', '-o', file, buildPackage], { cwd: buildCwd, env, timeout: 120000, stdio: ['ignore', 'ignore', 'pipe'] });
  }
  await chmod(file, 0o700);
  digests[name] = 'sha256:' + createHash('sha256').update(await readFile(file)).digest('hex');
}
const binaryInfo = execFileSync('go', ['version', '-m', join(target, 'pinax' + suffix)], { encoding: 'utf8', timeout: 15000 });
const ownerVersion = /^\s*mod\s+github\.com\/yeisme\/pinax\s+(\S+)/m.exec(binaryInfo)?.[1];
if (process.env.DRIVEBRIDGE_PINAX_BIN && ownerVersion !== sdkVersion) throw new Error('Pinax binary version does not match the SDK pin.');
const vault = await mkdtemp(join(tmpdir(), 'drivebridge-package-schema-'));
let schemaDigest;
try {
  const schema = JSON.parse(execFileSync(join(target, 'pinax' + suffix), ['api', 'schema', 'export', '--format', 'openapi', '--vault', vault, '--json'], { env: {}, encoding: 'utf8', timeout: 15000, maxBuffer: 16 * 1024 * 1024 }));
  schemaDigest = schema.facts?.schema_digest;
  for (const name of ['pinax-reader.openapi.json', 'pinax-writer-sync.openapi.json']) {
    const contract = JSON.parse(await readFile(join(root, 'contracts', 'desktop', name), 'utf8'));
    if (schema.status !== 'success' || schemaDigest !== contract['x-owner-openapi-digest']) throw new Error('Packaged Pinax schema does not match the generated desktop contracts.');
  }
} finally { await rm(vault, { recursive: true, force: true }); }
const projection = JSON.parse(execFileSync(join(target, 'pinax' + suffix), ['api', 'manifest', '--json'], { cwd: target, env: {}, timeout: 15000, maxBuffer: 8 * 1024 * 1024, encoding: 'utf8' }));
if (projection.status !== 'success' || !/^sha256:[a-f0-9]{64}$/.test(projection.facts?.digest)) throw new Error('Owner manifest export failed.');
await writeFile(join(target, 'bundle.json'), JSON.stringify({ schema_version: 'drivebridge.desktop.bundle.v1', drivebridge: digests.drivebridge, pinax: digests.pinax, manifest: projection.facts.digest }, null, 2) + '\n', { mode: 0o600 });
const evidence = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS || join(root, 'temp', 'desktop-host-build');
await mkdir(evidence, { recursive: true, mode: 0o700 });
await writeFile(join(evidence, 'host-build-provenance.json'), JSON.stringify({ schema_version: 'drivebridge.desktop.build.v1', sdk_version: sdkVersion, binary_module_version: ownerVersion, source: process.env.DRIVEBRIDGE_PINAX_BIN ? 'version-matched-supplied-binary' : 'explicit-local-source-candidate', digests, manifest_digest: projection.facts.digest, owner_schema_digest: schemaDigest, signed: false, released: false }, null, 2) + '\n', { mode: 0o600 });
console.log('Development host resources prepared; SDK, owner schema and binary provenance verified; not signed or released.');
