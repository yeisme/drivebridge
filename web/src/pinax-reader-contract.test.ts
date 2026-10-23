import { afterEach, expect, it, vi } from 'vitest';
// @ts-expect-error Build-only JavaScript owner schema adapter.
import { readerContract, writerSyncContract, ownerDigest, exportReaderContract } from '../scripts/pinax-reader-contract.mjs';

afterEach(() => vi.unstubAllEnvs());

it('rejects inherited race flags before executing the owner command', async () => {
  vi.stubEnv('GOFLAGS', '-race');
  await expect(exportReaderContract('/unused')).rejects.toThrow('Race detection is excluded');
});

function projection() {
  const schema = {
    openapi: '3.1.0', jsonSchemaDialect: 'https://json-schema.org/draft/2020-12/schema',
    components: { schemas: {
      'pinax.reader.document.v1alpha1': { type: 'object', properties: { node: { $ref: '#/components/schemas/pinax.reader.document_node.v1alpha1' } } },
      'pinax.reader.document_node.v1alpha1': { type: 'object', properties: { child: { $ref: '#/components/schemas/pinax.reader.document_node.v1alpha1' }, warning: { $ref: '#/components/schemas/pinax.projection_warning.v1' } } },
      'pinax.reader.notes.request.v1alpha1': { type: 'object', properties: {} },
      'pinax.projection_warning.v1': { type: 'string', description: '<fixture>&\u2028' },
      'pinax.local_session.bootstrap.v1alpha1': { type: 'object', properties: { bearer: { type: 'string' } } },
    } },
  };
  return { command: 'api.schema.export', status: 'success', facts: { schema_digest: ownerDigest(schema) }, data: { schema } };
}

it('copies the reader closure without bootstrap, credentials, or runtime route claims', () => {
  const input = projection();
  const output = readerContract(input);
  expect(Object.keys(output.components.schemas)).toHaveLength(4);
  expect(output.components.schemas['pinax.local_session.bootstrap.v1alpha1']).toBeUndefined();
  expect(output.components.schemas['pinax.reader.document_node.v1alpha1']).toEqual(input.data.schema.components.schemas['pinax.reader.document_node.v1alpha1']);
  expect(output.paths).toEqual({});
  expect(output['x-drivebridge-integration-status']).toBe('pending');
  expect(readerContract(input)).toEqual(output);
  expect(output.components.schemas['pinax.reader.document_node.v1alpha1']).not.toBe(input.data.schema.components.schemas['pinax.reader.document_node.v1alpha1']);
});

it('fails closed on digest drift and wrong export identity', () => {
  const input = projection();
  input.data.schema.openapi = '3.0.0';
  expect(() => readerContract(input)).toThrow();
  const wrong = projection(); wrong.command = 'note.read';
  expect(() => readerContract(wrong)).toThrow();
  const digest = projection(); digest.facts.schema_digest = 'sha256:' + '0'.repeat(64);
  expect(() => readerContract(digest)).toThrow();
});

it.each(['https://external.invalid/schema', '#/components/schemas/missing', '#/components/schemas/pinax.local_session.bootstrap.v1alpha1', '#/components/schemas/pinax.reader.%2Fsecret'])('rejects reference %s', ref => {
  const input = projection();
  input.data.schema.components.schemas['pinax.reader.document.v1alpha1'].properties.node.$ref = ref;
  input.facts.schema_digest = ownerDigest(input.data.schema);
  expect(() => readerContract(input)).toThrow();
});

it('matches Go JSON escaping and is insensitive to object key ordering', () => {
  expect(ownerDigest({ b: 1, a: '<>&\u2028\u2029' })).toEqual(ownerDigest({ a: '<>&\u2028\u2029', b: 1 }));
  expect(ownerDigest({ a: '<' })).not.toEqual(ownerDigest({ a: '\\u003c' }));
});

it('exports writer and sync schemas without private session contracts', () => {
  const schema = { openapi: '3.1.0', components: { schemas: {
    'pinax.writer.editor_source.request.v1alpha1': { type: 'object' },
    'pinax.writer.propose.request.v1alpha1': { type: 'object', properties: { change: { $ref: '#/components/schemas/pinax.writer.change.v1alpha1' } } },
    'pinax.writer.change.v1alpha1': { type: 'object' },
    'pinax.writer.apply.request.v1alpha1': { type: 'object' },
    'pinax.sync.status.request.v1': { type: 'object' }, 'pinax.sync.diff.request.v1': { type: 'object' },
    'pinax.sync.push.request.v1': { type: 'object' }, 'pinax.sync.pull.request.v1': { type: 'object' },
    'pinax.projection.v1': { type: 'object', properties: { error: { $ref: '#/components/schemas/pinax.error.v1' } } },
    'pinax.error.v1': { type: 'object' }, 'pinax.local_session.bootstrap.v1alpha1': { type: 'object' },
  } } };
  const output = writerSyncContract({ command: 'api.schema.export', status: 'success', facts: { schema_digest: ownerDigest(schema) }, data: { schema } });
  expect(output.components.schemas['pinax.writer.change.v1alpha1']).toBeDefined();
  expect(output.components.schemas['pinax.error.v1']).toBeDefined();
  expect(output.components.schemas['pinax.local_session.bootstrap.v1alpha1']).toBeUndefined();
  expect(output.paths).toEqual({});
  expect(output['x-drivebridge-integration-status']).toBe('pending');
});
