// Main process only. Never import this module in preload or renderer code.
import Ajv2020 from 'ajv/dist/2020';
import schemas from './generated/host-schemas.json';
import type { components } from './generated/host';

export type HostEvent = components['schemas']['HostEvent'];
export type HostBootstrap = components['schemas']['HostBootstrap'];
export type HostSelection = components['schemas']['HostSelection'];
export type HostCancel = components['schemas']['HostCancel'];
const ajv = new Ajv2020({ strict: true, allErrors: false });
const validators = {
  HostEvent: ajv.compile<HostEvent>(schemas.HostEvent),
  HostBootstrap: ajv.compile<HostBootstrap>(schemas.HostBootstrap),
  HostSelection: ajv.compile<HostSelection>(schemas.HostSelection),
  HostCancel: ajv.compile<HostCancel>(schemas.HostCancel),
};
const fail = (): never => { throw new Error('desktop_host_protocol_invalid'); };
const MAX_FRAME = 64 * 1024;
const utf8 = new TextDecoder('utf-8', { fatal: true });

// Events contain only string fields. Parse this narrow flat grammar before
// JSON.parse to reject duplicate decoded keys (including escaped aliases).
// No JSON.parse reviver can detect overwritten duplicate fields afterwards.
function strictEventObject(text: string): Record<string, string> {
  const source = text.trim();
  if (!source.startsWith('{') || !source.endsWith('}')) return fail();
  const inner = source.slice(1, -1);
  const pair = /\s*("(?:[^"\\\u0000-\u001f]|\\(?:["\\/bfnrt]|u[0-9a-fA-F]{4}))*")\s*:\s*("(?:[^"\\\u0000-\u001f]|\\(?:["\\/bfnrt]|u[0-9a-fA-F]{4}))*")\s*/y;
  let offset = 0;
  const seen = new Set<string>();
  while (offset < inner.length) {
    pair.lastIndex = offset;
    const match = pair.exec(inner);
    if (!match) return fail();
    const key: unknown = JSON.parse(match[1]);
    if (typeof key !== 'string' || seen.has(key)) return fail();
    seen.add(key);
    offset = pair.lastIndex;
    if (offset === inner.length) break;
    if (inner[offset] !== ',' || offset + 1 === inner.length) return fail();
    offset++;
  }
  return JSON.parse(source) as Record<string, string>;
}

export function decodeHostEvent(frame: Uint8Array): HostEvent {
  try {
    if (!frame.byteLength || frame.byteLength >= MAX_FRAME) return fail();
    const value = strictEventObject(utf8.decode(frame));
    if (!validators.HostEvent(value)) return fail();
    if (value.type === 'ready') {
      const address = new URL(value.address);
      const port = Number(address.port);
      if (!Number.isInteger(port) || port < 1 || port > 65535 || value.address !== `http://127.0.0.1:${port}`) return fail();
    }
    return value;
  } catch { return fail(); }
}

export function encodeHostRequest(value: HostBootstrap | HostSelection | HostCancel): Buffer {
  const valid = 'action' in value ? value.action === 'select' ? validators.HostSelection(value) : validators.HostCancel(value) : validators.HostBootstrap(value);
  if (!valid) return fail();
  const result = Buffer.from(JSON.stringify(value) + '\n');
  if (result.length > MAX_FRAME) return fail();
  return result;
}

// A chunk may contain several frames or a split multibyte sequence. No
// unbounded string concatenation and no partial frame accepted at EOF.
export class HostFrames {
  #pending = Buffer.alloc(0);
  #failed = false;
  push(chunk: Uint8Array, receive: (event: HostEvent) => void): void {
    if (this.#failed) return fail();
    try {
      let start = 0;
      for (let i = 0; i < chunk.length; i++) {
        if (chunk[i] !== 10) continue;
        if (this.#pending.length + i - start >= MAX_FRAME) return fail();
        const frame = Buffer.concat([this.#pending, chunk.subarray(start, i)]);
        this.#pending = Buffer.alloc(0);
        receive(decodeHostEvent(frame));
        start = i + 1;
      }
      if (this.#pending.length + chunk.length - start >= MAX_FRAME) return fail();
      this.#pending = Buffer.concat([this.#pending, chunk.subarray(start)]);
    } catch { this.#failed = true; this.#pending = Buffer.alloc(0); return fail(); }
  }
  finish(): void { if (this.#failed || this.#pending.length) { this.#failed = true; this.#pending = Buffer.alloc(0); return fail(); } }
}

// Correlation is a state machine, not proof that an unsolicited event is safe.
// Register before writing stdin; callers cancel/close the child on any failure.
export class HostExchange {
  #state: 'starting' | 'ready' | 'closed' = 'starting';
  #pending = new Map<string, 'selection' | 'cancelled'>();
  constructor(private readonly nonce: string) {}
  expect(request: HostSelection | HostCancel): void {
    if (this.#state !== 'ready' || this.#pending.size >= 32 || this.#pending.has(request.request_id)) return fail();
    encodeHostRequest(request);
    this.#pending.set(request.request_id, request.action === 'select' ? 'selection' : 'cancelled');
  }
  accept(event: HostEvent): void {
    try {
      if (this.#state === 'closed') return fail();
      if (event.type === 'ready') {
        if (this.#state !== 'starting' || event.session_nonce !== this.nonce) return fail();
        this.#state = 'ready'; return;
      }
      if (event.type === 'error' && !event.request_id) { this.close(); return; }
      if (this.#state !== 'ready') return fail();
      if (event.type === 'shutdown') {
        if (event.session_nonce !== this.nonce) return fail();
        this.close(); return;
      }
      if (!('request_id' in event) || !event.request_id) return fail();
      const expected = this.#pending.get(event.request_id);
      if (!expected || event.type !== 'error' && event.type !== expected) return fail();
      this.#pending.delete(event.request_id);
    } catch { this.close(); return fail(); }
  }
  close(): void { this.#state = 'closed'; this.#pending.clear(); }
}
