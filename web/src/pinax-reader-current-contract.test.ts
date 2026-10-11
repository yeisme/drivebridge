import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import Ajv2020 from 'ajv/dist/2020.js';
import addFormats from 'ajv-formats';
import { expect, it } from 'vitest';
import type { components } from './generated/pinax-reader';

async function load(name: string) {
  return JSON.parse(await readFile(resolve(process.cwd(), '../contracts/desktop', name), 'utf8'));
}

it('consumes current owner union schemas without adding runtime IPC or private authority', async () => {
  const owner = await load('pinax-reader.openapi.json');
  const ipc = await load('ipc.json');
  expect(owner['x-owner-openapi-digest']).toBe('sha256:4c85fe7909c8575546deee3ae2d5f0d85c20f30e5d3ab1e6e1617d1824f595a8');
  expect(owner.paths).toEqual({});
  expect(owner['x-drivebridge-integration-status']).toBe('pending');
  expect(ipc['x-methods']).toHaveLength(23);
  expect(ipc['x-methods'].some((m: { name: string }) => /rich|composed|recovery|writer/i.test(m.name))).toBe(false);
  for (const name of Object.keys(owner.components.schemas)) {
    expect(name.startsWith('pinax.reader.') || name === 'pinax.projection_warning.v1').toBe(true);
  }
  // The recovery query shape is public; its private grant and full operation
  // payload are NOT an approved renderer surface and must stay outside closure.
  expect(owner.components.schemas['pinax.reader.recovery_lookup.request.v1alpha1']).toBeDefined();
  expect(owner.components.schemas['pinax.operation.v1']).toBeUndefined();
  expect(owner.components.schemas['pinax.local_session.bootstrap.v1alpha1']).toBeUndefined();
  const copy = JSON.parse(JSON.stringify(owner.components.schemas, (key, value) => key === '$id' ? undefined : value).replaceAll('#/components/schemas/', '#/$defs/'));
  const ajv = new Ajv2020({ strict: true, strictRequired: false });
  addFormats(ajv);
  const validate = (name: string) => ajv.compile({ $ref: `#/$defs/${name}`, $defs: copy });
  const embed = validate('pinax.reader.embed.v1alpha1');
  const target = { object_id: 'note-target', revision: `sha256:${'a'.repeat(64)}` };
  const base = { embed_id: 'embed-1', parent_embed_id: '', source_anchor: 'block-1', page_anchors: ['embed-1-block-1'] };
  const expanded = { ...base, status: 'expanded', depth: 1, target } satisfies components['schemas']['pinax.reader.embed.v1alpha1'];
  expect(embed(expanded), JSON.stringify(embed.errors)).toBe(true);
  expect(embed({ ...expanded, depth: 3 })).toBe(false);
  expect(embed({ ...expanded, target: null })).toBe(false);
  for (const status of ['unavailable', 'unsupported', 'depth_limit']) {
    const stopped = { ...base, status, depth: status === 'depth_limit' ? 3 : 1 };
    expect(embed(stopped), JSON.stringify(embed.errors)).toBe(true);
    expect(embed({ ...stopped, target })).toBe(false);
  }
  expect(embed({ ...expanded, page_anchors: ['x', 'x'] })).toBe(false);
  expect(embed({ ...expanded, execute: true })).toBe(false);
  const request = validate('pinax.reader.composed_document.request.v1alpha1');
  expect(request(target)).toBe(true);
  expect(request({ ...target, max_depth: 4 })).toBe(false);
  const rich = validate('pinax.reader.rich_annotation.v1alpha1');
  expect(rich(expanded)).toBe(false);
  const recovery = validate('pinax.reader.recovery_lookup.request.v1alpha1');
  expect(recovery({ operation_id: 'op-original' })).toBe(true);
  expect(recovery({ operation_id: 'op-original', principal_digest: 'private' })).toBe(false);
});
