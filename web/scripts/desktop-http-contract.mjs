// Derive planned main-to-facade HTTP contracts from the fixed Go IPC catalog.
// HTTP does not accept the renderer wrapper: route IDs are path parameters,
// mutation identities are headers, and native selection is main-only input.
export function httpContract(methods, desktop, owner) {
  const schemas = { ...desktop.components.schemas, ...owner.components.schemas };
  const paths = {};
  const ref = name => ({ $ref: `#/components/schemas/${name}` });
  const envelope = name => ({ type: 'object', additionalProperties: false, properties: { data: ref(name) }, required: ['data'] });
  const error = { type: 'object', additionalProperties: false, properties: { error: ref('DesktopError') }, required: ['error'] };
  const json = schema => ({ 'application/json': { schema } });
  const opaque = { type: 'string', minLength: 1, maxLength: 128, pattern: '^[A-Za-z0-9][A-Za-z0-9._:-]*$' };
  const common = () => ({
    'x-implementation-status': 'planned', security: [{ desktopSession: [] }],
    'x-cache-control': 'private, no-store', 'x-reject-unknown-query': true,
    'x-reject-duplicate-scalar-query': true,
  });
  const errors = () => Object.fromEntries([400, 401, 403, 404, 409, 410, 413, 416, 422, 503].map(code => [code, { description: 'Safe owner error; no private paths or upstream payload.', content: json(error) }]));
  for (const m of methods) {
    if (m.status !== 'planned' || !schemas[m.request_schema] || !schemas[m.response_schema]) throw new Error('Invalid HTTP contract catalog');
    const parameters = [];
    const request = schemas[m.request_schema];
    const pathFields = [...m.path.matchAll(/\{([^}]+)\}/g)].map(match => match[1]);
    for (const name of pathFields) parameters.push({ name, in: 'path', required: true, schema: request.properties?.[name] || opaque });
    if (m.idempotent) parameters.push({ name: 'Idempotency-Key', in: 'header', required: true, schema: opaque });
    if (m.http_method === 'GET') {
      for (const [name, schema] of Object.entries(request.properties || {})) {
        if (pathFields.includes(name)) continue;
        const querySchema = ['readDocument', 'readLinks'].includes(m.name) && name === 'revision' ? { ...schema, pattern: '^sha256:[a-f0-9]{64}$', minLength: 71, maxLength: 71 } : schema;
        parameters.push({ name, in: 'query', required: (request.required || []).includes(name) || m.name === 'readDocument' && name === 'revision', schema: querySchema,
          ...(schema.type === 'array' ? { style: 'form', explode: true } : {}) });
      }
    }
    const status = m.name === 'confirmBinding' ? '201' : m.name === 'exportSource' ? '202' : '200';
    const operation = { ...common(), operationId: m.name, parameters,
      'x-allow-repeated-query': parameters.filter(p => p.in === 'query' && p.schema.type === 'array').map(p => p.name),
      responses: { [status]: { description: 'Owner result; not a runtime availability claim.', content: json(envelope(m.response_schema)) }, ...errors() } };
    if (m.http_method !== 'GET') {
      const bodySchema = m.name === 'chooseVault' ? 'NativeSelection' : m.name === 'exportSource' ? 'ExportRequest' : m.request_schema;
      operation.requestBody = { required: true, content: json(ref(bodySchema)) };
      operation['x-request-max-bytes'] = 131072;
      if (m.native_selection) operation['x-main-only-native-selection'] = true;
    }
    const entry = paths[m.path] ||= {};
    const method = m.http_method.toLowerCase();
    if (entry[method]) throw new Error('Duplicate HTTP operation');
    entry[method] = operation;
  }
  // A separate controlled app-protocol byte channel, never a general IPC fetch.
  paths['/v1/desktop/assets/{handle}'] = { get: {
    ...common(), operationId: 'readAssetBytes', 'x-main-asset-channel-only': true, 'x-max-range-bytes': 8388608,
    parameters: [
      { name: 'handle', in: 'path', required: true, schema: opaque },
      { name: 'Range', in: 'header', required: false, schema: { type: 'string', pattern: '^bytes=[0-9]+-[0-9]+$' } },
    ], responses: {
      '200': { description: 'Complete resource within 8 MiB; includes zero-byte resources.', content: { 'application/octet-stream': { schema: { type: 'string', format: 'binary' } } }, headers: { ETag: { required: true, schema: { type: 'string' } } } },
      '206': { description: 'Single bounded authorized byte range.', content: { 'application/octet-stream': { schema: { type: 'string', format: 'binary' } } }, headers: { 'Content-Range': { required: true, schema: { type: 'string' } }, ETag: { required: true, schema: { type: 'string' } } } },
      ...errors(),
    },
  } };
  paths['/v1/desktop/assets/{handle}'].get.responses['416'].headers = { 'Content-Range': { required: true, schema: { type: 'string', pattern: '^bytes \\*/[0-9]+$' } } };
  return { openapi: '3.1.0', info: { title: 'DriveBridge planned desktop facade', version: 'v1alpha1' },
    jsonSchemaDialect: 'https://json-schema.org/draft/2020-12/schema',
    'x-implementation-status': 'planned', 'x-owner-reader-digest': owner['x-reader-schema-digest'], paths,
    components: { schemas, securitySchemes: { desktopSession: { type: 'http', scheme: 'bearer', description: 'Private main-to-facade session; never exposed to renderer.' } } } };
}
