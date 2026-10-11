// Generate TypeScript from Go contracts, including explicitly planned routes.
// This is not a claim that HTTP handlers are implemented.
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { applyArtifacts } from './contract-artifacts.mjs';
import { exportReaderContract } from './pinax-reader-contract.mjs';
import { ipcArtifacts } from './desktop-ipc-contract.mjs';
import { httpContract } from './desktop-http-contract.mjs';
import { mkdtemp, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import openapiTS, { astToString } from 'openapi-typescript';

const root = fileURLToPath(new URL('../../', import.meta.url));
const args = process.argv.slice(2);
const check = args.includes('--check');
const profileArgs = args.filter(arg => arg !== '--check');
const profile = profileArgs.length === 0 ? 'personal-workspace' : profileArgs[1];
if (args.filter(arg => arg === '--check').length > 1 ||
    (profileArgs.length !== 0 && (profileArgs.length !== 2 || profileArgs[0] !== '--profile')) ||
    !['personal-workspace', 'desktop', 'all'].includes(profile)) {
  throw new Error('Usage: node web/scripts/generate-contract-types.mjs [--check] [--profile personal-workspace|desktop|all]');
}
const artifacts = {};
for (const selected of profile === 'all' ? ['personal-workspace', 'desktop'] : [profile]) {
  const flags = selected === 'personal-workspace' ? [] : ['--profile', selected];
  const document = JSON.parse(execFileSync('go', ['-C', 'server', 'run', './cmd/drivebridge-contracts', 'openapi', ...flags], { cwd: root, encoding: 'utf8' }));
  const text = '// Generated from Go DTOs. Do not edit. Planned contracts; endpoints are not implemented.\n' + astToString(await openapiTS(document));
  const fixtures = execFileSync('go', ['-C', 'server', 'run', './cmd/drivebridge-contracts', 'fixtures', ...flags], { cwd: root, encoding: 'utf8' });
  artifacts[`web/src/generated/${selected}.ts`] = text;
  artifacts[`contracts/${selected}/openapi.json`] = JSON.stringify(document, null, 2) + '\n';
  artifacts[`contracts/${selected}/fixtures/responses.json`] = fixtures;
}
if (profile === 'desktop' || profile === 'all') {
  const reader = await exportReaderContract(root);
  artifacts['contracts/desktop/pinax-reader.openapi.json'] = JSON.stringify(reader, null, 2) + '\n';
  artifacts['web/src/generated/pinax-reader.ts'] = '// Generated from the Pinax public schema export. Do not edit. DriveBridge integration pending.\n' + astToString(await openapiTS(reader));
  const methods = JSON.parse(execFileSync('go', ['-C', 'server', 'run', './cmd/drivebridge-contracts', 'ipc', '--profile', 'desktop'], { cwd: root, encoding: 'utf8' }));
  const ipc = ipcArtifacts(methods, JSON.parse(artifacts['contracts/desktop/openapi.json']), reader);
  artifacts['contracts/desktop/ipc.json'] = JSON.stringify(ipc.document, null, 2) + '\n';
  artifacts['web/src/generated/desktop-ipc.ts'] = ipc.ts;
  const http = httpContract(methods, JSON.parse(artifacts['contracts/desktop/openapi.json']), reader);
  artifacts['contracts/desktop/facade.openapi.json'] = JSON.stringify(http, null, 2) + '\n';
  artifacts['web/src/generated/desktop-http.ts'] = '// Generated planned facade contract. No runtime handlers.\n' + astToString(await openapiTS(http));
}
if (check) {
  const temporary = await mkdtemp(join(tmpdir(), 'drivebridge-generated-'));
  try {
    await applyArtifacts(temporary, artifacts);
    const regenerated = {};
    for (const name of Object.keys(artifacts)) regenerated[name] = await readFile(join(temporary, name), 'utf8');
    await applyArtifacts(root, regenerated, true);
  } finally {
    await rm(temporary, { recursive: true, force: true });
  }
} else {
  await applyArtifacts(root, artifacts);
}
console.log(check ? 'Contract artifacts match Go DTOs.' : 'Generated OpenAPI, response fixtures and TypeScript.');
