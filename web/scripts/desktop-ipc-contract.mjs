// Build fixed IPC request schemas and signatures from the Go method catalog.
// No runtime dispatch, arbitrary URL, fs or Electron API is exposed here.
export const syncIdentityHeaders = ['X-Pinax-Operation-ID', 'Idempotency-Key'];
export function syncTransferInputSchema(schema) {
  const body = structuredClone(schema);
  delete body.$id; // A narrowed host DTO must not reuse the owner's schema identity.
  for (const field of syncIdentityHeaders) delete body.properties?.[field];
  if (body.required) body.required = body.required.filter(field => !syncIdentityHeaders.includes(field));
  return body;
}

export function ipcArtifacts(methods, desktop, owner, writerSync = { components: { schemas: {} } }) {
  const schemas = { ...desktop.components.schemas, ...owner.components.schemas, ...writerSync.components.schemas };
  const requests = {};
  const signatures = [];
  const seen = new Set();
  const unsafe = new Set(['path', 'url', 'method', 'bearer', 'token', 'native_selection_id', 'expected_destination_revision', 'merged_path']);
  const writerSyncName = name => name.startsWith('pinax.writer.') || name.startsWith('pinax.sync.') || ['pinax.projection.v1', 'pinax.projection_warning.v1', 'pinax.action.v1', 'pinax.operation.v1', 'pinax.error.v1'].includes(name);
  const type = name => `${writerSyncName(name) ? 'PinaxWriterSync' : name.startsWith('pinax.') ? 'Pinax' : 'Desktop'}['schemas'][${JSON.stringify(name)}]`;
  for (const method of methods) {
    if (!/^[a-z][A-Za-z0-9]*$/.test(method.name) || seen.has(method.name) || method.status !== 'planned' ||
        !['GET', 'POST', 'PUT', 'PATCH'].includes(method.http_method) || !method.path.startsWith('/v1/desktop/') ||
        !schemas[method.request_schema] || !schemas[method.response_schema]) throw new Error('Invalid desktop IPC catalog');
    seen.add(method.name);
    const syncTransfer = ['syncPush', 'syncPull'].includes(method.name);
    // The host derives owner identity from the one outer recovery slot. Keep
    // the owner export intact, but reject a second, conflicting renderer key.
    const body = syncTransfer ? syncTransferInputSchema(schemas[method.request_schema]) : schemas[method.request_schema];
    if (Object.keys(body.properties || {}).some(field => unsafe.has(field))) throw new Error('Private input cannot enter renderer IPC');
    // Desktop reads remain pinned even though owner document API also permits
    // an initial unpinned read. Strengthen the wrapper, not the owner schema.
    const pinnedRead = ['readDocument', 'readLinks'].includes(method.name);
    const input = pinnedRead ? { allOf: [body, { type: 'object', required: ['revision'], properties: { revision: { type: 'string', pattern: '^sha256:[a-f0-9]{64}$' } } }] } : body;
    const properties = { input };
    const required = ['input'];
    const pinned = method.name === 'readDocument' ? ' & { revision: string }' : '';
    const inputType = syncTransfer ? `Omit<${type(method.request_schema)}, ${syncIdentityHeaders.map(field => JSON.stringify(field)).join(' | ')}>` : type(method.request_schema);
    const fields = [`input: ${inputType}${pinned}`];
    if (method.target_field) {
      if (!['source_id', 'binding_id', 'operation_id'].includes(method.target_field)) throw new Error('Unsafe IPC target');
      properties[method.target_field] = { type: 'string', minLength: 1, maxLength: 128, pattern: '^[A-Za-z0-9][A-Za-z0-9._:-]*$' };
      required.push(method.target_field);
      fields.push(`${method.target_field}: string`);
    }
    if (method.idempotent) {
      properties.idempotency_key = { type: 'string', minLength: 1, maxLength: 128, pattern: '^[A-Za-z0-9][A-Za-z0-9._:-]*$' };
      required.push('idempotency_key'); fields.push('idempotency_key: string');
    }
    requests[method.name] = { type: 'object', additionalProperties: false, properties, required };
    signatures.push(`  ${method.name}(request: { ${fields.join('; ')} }): Promise<DesktopResult<${type(method.response_schema)}>>;`);
  }
  const document = { openapi: '3.1.0', info: { title: 'DriveBridge fixed IPC requests', version: 'v1alpha1' }, paths: {},
    'x-implementation-status': 'planned', 'x-owner-reader-digest': owner['x-reader-schema-digest'], 'x-owner-writer-sync-digest': writerSync['x-writer-sync-schema-digest'],
    components: { schemas }, 'x-ipc-requests': requests, 'x-methods': methods };
  const ts = `// Generated from Go IPC catalog and owner schemas. No runtime implementation.\nimport type { components as Desktop } from './desktop';\nimport type { components as Pinax } from './pinax-reader';\nimport type { components as PinaxWriterSync } from './pinax-writer-sync';\nexport type DesktopResult<T> = { data: T } | { error: Desktop['schemas']['DesktopError'] };\nexport interface DesktopBridge {\n${signatures.join('\n')}\n}\n`;
  return { document, ts };
}
