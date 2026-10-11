import { expect, it } from 'vitest';
import { readFile } from 'node:fs/promises';
import { decodeHostEvent, encodeHostRequest, HostFrames, HostExchange, type HostEvent, type HostSelection } from './host-protocol';
const version = 'drivebridge.desktop.host.v1alpha1' as const;
const nonce = 'fixture-nonce-123456789';
const ready = { schema_version: version, type: 'ready', session_nonce: nonce, address: 'http://127.0.0.1:12345' } as const;
const select: HostSelection = { schema_version: version, action: 'select', request_id: 'request-one', native_path: '/fixture/private' };
const wire = (value: object) => Buffer.from(JSON.stringify(value));
const selected: HostEvent = { schema_version: version, type: 'selection', request_id: select.request_id, selection_id: 'ticket-one' };

it('validates all Go event branches with exact fields', () => {
  for (const event of [ready, selected, { schema_version: version, type: 'cancelled', request_id: 'r' }, { schema_version: version, type: 'shutdown', session_nonce: nonce }, { schema_version: version, type: 'error', code: 'desktop_host_invalid' }]) expect(decodeHostEvent(wire(event))).toEqual(event);
  for (const invalid of [ { ...ready, bearer: 'private' }, { ...ready, type: 'future' }, { ...ready, schema_version: 'future' }, { ...ready, address: 'http://127.0.0.1:65536' }, { ...ready, address: 'http://localhost:12345' }, { ...ready, request_id: 'r' }, { ...selected, selection_id: null }, { schema_version: version, type: 'selection', request_id: 'r' } ]) expect(() => decodeHostEvent(wire(invalid))).toThrow('desktop_host_protocol_invalid');
});
it('rejects duplicate escaped keys, invalid UTF8, trailing JSON and null', () => {
  for (const text of [JSON.stringify(ready).replace('{', '{"type":"ready",'), JSON.stringify(ready).replace('{', '{"\\u0074ype":"ready",'), JSON.stringify(ready) + '{}', JSON.stringify(ready).replace('"ready"', 'null'), JSON.stringify(ready).replace('{', '{"Type":"ready",')]) expect(() => decodeHostEvent(Buffer.from(text))).toThrow('desktop_host_protocol_invalid');
  expect(() => decodeHostEvent(Buffer.from([0xff]))).toThrow();
});
it('decodes fragmented/batched lines and rejects oversized or unfinished frames', () => {
  const frames = new HostFrames(); const events: HostEvent[] = [];
  const data = Buffer.from(JSON.stringify(ready) + '\n' + JSON.stringify(selected) + '\n');
  for (const byte of data) frames.push(Buffer.from([byte]), event => events.push(event));
  frames.finish(); expect(events).toEqual([ready, selected]);
  const pending = new HostFrames(); pending.push(Buffer.from('{'), () => {}); expect(() => pending.finish()).toThrow();
  const huge = new HostFrames(); expect(() => huge.push(Buffer.alloc(65536, 120), () => {})).toThrow();
  expect(() => huge.push(Buffer.from('{}\n'), () => {})).toThrow();
});
it('validates outgoing frames and bounds encoded UTF8 bytes', () => {
  expect(encodeHostRequest(select).at(-1)).toBe(10);
  expect(() => encodeHostRequest({ ...select, native_path: '中'.repeat(30000) })).toThrow();
  expect(() => encodeHostRequest({ ...select, extra: 'no' } as HostSelection)).toThrow();
});
it('binds replies to the current handshake and pending action', () => {
  const flow = new HostExchange(nonce);
  expect(() => flow.expect(select)).toThrow();
  flow.accept(ready); flow.expect(select); flow.accept(selected);
  expect(() => flow.accept(selected)).toThrow(); // duplicate/unsolicited
  expect(() => flow.expect(select)).toThrow(); // failure is terminal
  const wrong = new HostExchange('other-nonce'); expect(() => wrong.accept(ready)).toThrow();
  const duplicate = new HostExchange(nonce); duplicate.accept(ready); expect(() => duplicate.accept(ready)).toThrow();
  const shutdown = new HostExchange(nonce); shutdown.accept(ready); shutdown.accept({ schema_version: version, type: 'shutdown', session_nonce: nonce }); expect(() => shutdown.expect(select)).toThrow();
});
it('limits pending operations and rejects mismatched action results', () => {
  const flow = new HostExchange(nonce); flow.accept(ready);
  for (let i = 0; i < 32; i++) flow.expect({ ...select, request_id: `r${i}` });
  expect(() => flow.expect({ ...select, request_id: 'overflow' })).toThrow();
  expect(() => flow.accept({ schema_version: version, type: 'cancelled', request_id: 'r0' })).toThrow();
});
it('does not permit private host models in the renderer or preload bundle', async () => {
  const preload = await readFile(new URL('./preload.ts', import.meta.url), 'utf8');
  expect(preload).not.toContain('host-protocol'); expect(preload).not.toContain('generated/host');
});
