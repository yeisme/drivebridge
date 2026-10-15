// Build-time owner export only. Never used for runtime note operations.
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { access, mkdtemp, readFile, readdir, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

const canonical = value => Array.isArray(value) ? value.map(canonical) : value && typeof value === 'object' ? Object.fromEntries(Object.keys(value).sort().map(key => [key, canonical(value[key])])) : value;
// Match Go encoding/json's HTML and line-separator escaping for digest parity.
export function ownerDigest(document) {
  const encoded = JSON.stringify(canonical(document)).replace(/[<>&\u2028\u2029]/g, c => `\\u${c.charCodeAt(0).toString(16).padStart(4, '0')}`);
  return `sha256:${createHash('sha256').update(encoded).digest('hex')}`;
}

export function readerContract(projection) {
  const source = projection?.data?.schema;
  if (projection?.command !== 'api.schema.export' || projection?.status !== 'success' || source?.openapi !== '3.1.0' || ownerDigest(source) !== projection?.facts?.schema_digest) throw new Error('Invalid Pinax owner schema projection or digest');
  const all = source.components?.schemas;
  if (!all || typeof all !== 'object' || Array.isArray(all)) throw new Error('Missing Pinax schema registry');
  const names = Object.keys(all).filter(name => name.startsWith('pinax.reader.')).sort();
  if (!names.includes('pinax.reader.document.v1alpha1') || !names.includes('pinax.reader.notes.request.v1alpha1')) throw new Error('Required reader contracts are missing');
  const selected = {};
  function include(name) {
    if (Object.hasOwn(selected, name)) return;
    if ((!name.startsWith('pinax.reader.') && name !== 'pinax.projection_warning.v1') || !Object.hasOwn(all, name)) throw new Error('Unsupported reader schema dependency');
    selected[name] = structuredClone(all[name]);
    visit(selected[name]);
  }
  function visit(value) {
    if (!value || typeof value !== 'object') return;
    if (Object.hasOwn(value, '$ref')) {
      const ref = value.$ref;
      if (typeof ref !== 'string' || !ref.startsWith('#/components/schemas/') || /[~%]/.test(ref)) throw new Error('Reader schema contains unsupported reference');
      include(ref.slice('#/components/schemas/'.length));
    }
    for (const child of Object.values(value)) visit(child);
  }
  names.forEach(include);
  const schemas = Object.fromEntries(Object.keys(selected).sort().map(name => [name, selected[name]]));
  // These are projections, not routes through the DriveBridge facade.
  return {
    openapi: '3.1.0', info: { title: 'Pinax reader owner contracts', version: 'v1alpha1' },
    'jsonSchemaDialect': source.jsonSchemaDialect,
    'x-owner': 'pinax', 'x-owner-openapi-digest': projection.facts.schema_digest,
    'x-reader-schema-digest': ownerDigest(schemas),
    'x-drivebridge-integration-status': 'pending', paths: {}, components: { schemas },
  };
}

export async function exportReaderContract(root) {
  if (/(?:^|[\s"'])-race(?:=|[\s"']|$)/.test(process.env.GOFLAGS || '')) throw new Error('Race detection is excluded from contract export');
  if (process.env.DRIVEBRIDGE_PINAX_READER_CONTRACT_SOURCE === 'fixture') {
    const fixture = JSON.parse(await readFile(join(root, 'contracts', 'desktop', 'pinax-reader.openapi.json'), 'utf8'));
    if (fixture.openapi !== '3.1.0' || fixture['x-owner'] !== 'pinax' || fixture['x-drivebridge-integration-status'] !== 'pending') throw new Error('Invalid checked-in Pinax reader contract fixture');
    return fixture;
  }
  const vault = await mkdtemp(join(tmpdir(), 'drivebridge-pinax-contract-'));
  try {
    // Explicit empty disposable directory prevents default personal-vault selection.
    const packagedPinax = process.env.DRIVEBRIDGE_PINAX_BIN || join(root, 'desktop', 'host-bin', process.platform === 'win32' ? 'pinax.exe' : 'pinax');
    const usePackagedPinax = await access(packagedPinax).then(() => true, () => false);
    const command = usePackagedPinax ? packagedPinax : 'go';
    const args = usePackagedPinax
      ? ['api', 'schema', 'export', '--format', 'openapi', '--vault', vault, '--json']
      : ['run', './cmd/pinax', 'api', 'schema', 'export', '--format', 'openapi', '--vault', vault, '--json'];
    const raw = execFileSync(command, args, {
      cwd: usePackagedPinax ? root : resolve(root, '../pinax'), encoding: 'utf8', maxBuffer: 16 * 1024 * 1024, timeout: 120000,
      env: { ...process.env, CGO_ENABLED: '0', GOWORK: 'off' }, stdio: ['ignore', 'pipe', 'pipe'],
    });
    if ((await readdir(vault)).length) throw new Error('Schema export mutated its disposable vault');
    return readerContract(JSON.parse(raw));
  } catch {
    // Do not echo subprocess arguments, local paths, or owner payloads.
    throw new Error('Pinax reader contract export failed; verify the sibling owner checkout and public schema command');
  } finally {
    await rm(vault, { recursive: true, force: true });
  }
}
