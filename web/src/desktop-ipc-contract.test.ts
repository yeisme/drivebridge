import { expect, it } from 'vitest';
import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import Ajv2020 from 'ajv/dist/2020.js';
import addFormats from 'ajv-formats';
// @ts-expect-error Build-only JavaScript generator.
import { ipcArtifacts } from '../scripts/desktop-ipc-contract.mjs';

async function contract() {
  return JSON.parse(await readFile(resolve(process.cwd(), '../contracts/desktop/ipc.json'), 'utf8'));
}

it('validates every fixed IPC wrapper and rejects arbitrary transport/private fields', async () => {
  const doc = await contract();
  const fixtures = JSON.parse(await readFile(resolve(process.cwd(), '../contracts/desktop/fixtures/responses.json'), 'utf8'));
  // OpenAPI component IDs are provenance, not nested JSON Schema resource
  // bases. For Ajv, rebase refs and remove those IDs on a validation-only copy.
  const jsonSchema = (value: unknown) => JSON.parse(JSON.stringify(value, (key, child) => key === '$id' ? undefined : child).replaceAll('#/components/schemas/', '#/$defs/'));
  for (const method of doc['x-methods']) {
    const ajv = new Ajv2020({ strict: true, strictRequired: false }); addFormats(ajv);
    const validate = ajv.compile(jsonSchema({ ...doc['x-ipc-requests'][method.name], $defs: doc.components.schemas }));
    // Rewrite only local reference prefixes; owner schemas remain unchanged.
    const input = fixtures[method.request_schema] ?? (method.name === 'readGraph' ? { mode: 'source' } : method.name === 'findDaily' ? { date: '2026-01-01', timezone: 'UTC' } : method.name === 'searchNotes' ? { q: 'fixture' } : method.name === 'readDocument' ? { object_id: 'note_fixture', revision: 'sha256:' + 'a'.repeat(64) } : method.name === 'readLinks' ? { object_id: 'note_fixture', revision: 'sha256:' + 'a'.repeat(64) } : {});
    const request: Record<string, unknown> = { input };
    if (method.target_field) request[method.target_field] = 'fixture-id';
    if (method.idempotent) request.idempotency_key = 'original-key';
    expect(validate(request), `${method.name}: ${JSON.stringify(validate.errors)}`).toBe(true);
    for (const extra of [{ path: '/private' }, { url: 'https://example.invalid' }, { method: 'DELETE' }, { token: 'fixture' }, { native_selection_id: 'spoofed' }]) expect(validate({ ...request, ...extra })).toBe(false);
    if (method.name === 'readDocument') { const missing = { ...input }; delete missing.revision; expect(validate({ ...request, input: missing })).toBe(false); }
    if (method.idempotent) { const bad = { ...request }; delete bad.idempotency_key; expect(validate(bad)).toBe(false); }
    if (method.native_selection) expect(validate({ ...request, input: { ...input, native_selection_id: 'spoofed' } })).toBe(false);
  }
});

it('fails generation for duplicate methods, missing schemas, or private request types', async () => {
  const doc = await contract();
  const methods = doc['x-methods'];
  const owner = { components: doc.components, 'x-reader-schema-digest': doc['x-owner-reader-digest'] };
  expect(() => ipcArtifacts([...methods, methods[0]], doc, owner)).toThrow();
  expect(() => ipcArtifacts([{ ...methods[0], request_schema: 'missing' }], doc, owner)).toThrow();
  expect(() => ipcArtifacts([{ ...methods[0], request_schema: 'ExportRequest' }], doc, owner)).toThrow();
  expect(() => ipcArtifacts([{ ...methods[0], target_field: 'path' }], doc, owner)).toThrow();
});
