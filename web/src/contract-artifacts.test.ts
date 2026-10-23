import { afterEach, expect, it } from 'vitest';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
// @ts-expect-error Existing Node generator is a JavaScript build tool.
import { applyArtifacts } from '../scripts/contract-artifacts.mjs';

const roots: string[] = [];
afterEach(async () => { await Promise.all(roots.splice(0).map(root => rm(root, { recursive: true, force: true }))); });
async function directory() { const root = await mkdtemp(join(tmpdir(), 'drivebridge-contract-')); roots.push(root); return root; }

it('detects missing and altered outputs without overwriting them', async () => {
 const root = await directory();
 const artifacts = { 'generated/types.ts': 'export type State = "active";\n', 'schema.json': '{"required":["expires_at"]}\n' };
 await expect(applyArtifacts(root, artifacts, true)).rejects.toThrow('Contract drift');
 await applyArtifacts(root, artifacts);
 await expect(applyArtifacts(root, artifacts, true)).resolves.toBeUndefined();
 for (const name of Object.keys(artifacts)) {
  await writeFile(join(root, name), 'altered\n');
  await expect(applyArtifacts(root, artifacts, true)).rejects.toThrow(name);
  expect(await readFile(join(root, name), 'utf8')).toBe('altered\n');
  await applyArtifacts(root, artifacts);
 }
});

it('detects real generated OpenAPI, fixture and TypeScript drift', async () => {
 const root = await directory();
 const artifacts: Record<string, string> = {};
 for (const name of ['src/generated/personal-workspace.ts', '../contracts/personal-workspace/openapi.json', '../contracts/personal-workspace/fixtures/responses.json', 'src/generated/desktop.ts', '../contracts/desktop/openapi.json', '../contracts/desktop/fixtures/responses.json', 'src/generated/pinax-reader.ts', '../contracts/desktop/pinax-reader.openapi.json', 'src/generated/pinax-writer-sync.ts', '../contracts/desktop/pinax-writer-sync.openapi.json', 'src/generated/desktop-ipc.ts', '../contracts/desktop/ipc.json', 'src/generated/desktop-http.ts', '../contracts/desktop/facade.openapi.json']) {
  artifacts[name.replaceAll('../', '').replaceAll('/', '_')] = await readFile(join(process.cwd(), name), 'utf8');
 }
 await applyArtifacts(root, artifacts);
 for (const [name, original] of Object.entries(artifacts)) {
  await writeFile(join(root, name), original + '\n');
  await expect(applyArtifacts(root, artifacts, true)).rejects.toThrow(name);
  expect(await readFile(join(root, name), 'utf8')).toBe(original + '\n');
  await applyArtifacts(root, artifacts);
 }
 await expect(applyArtifacts(root, artifacts, true)).resolves.toBeUndefined();
});

it('rejects outputs outside the generated root', async () => {
 const root = await directory();
 await expect(applyArtifacts(root, { '../escape': 'forbidden' })).rejects.toThrow('Invalid artifact path');
});
