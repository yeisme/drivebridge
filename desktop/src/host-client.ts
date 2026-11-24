// Main-only transport. Neither bundle paths nor token/address are public fields.
import { spawn, type ChildProcessWithoutNullStreams } from 'node:child_process';
import { createHash, randomBytes } from 'node:crypto';
import { readFile, lstat, realpath } from 'node:fs/promises';
import { join } from 'node:path';
import { request as httpRequest } from 'node:http';
import Ajv2020 from 'ajv/dist/2020';
import { HostFrames, HostExchange, encodeHostRequest, type HostEvent } from './host-protocol';
import publicContract from '../../contracts/desktop/openapi.json';
import type { components } from '../../web/src/generated/desktop';

type Preview = components['schemas']['BindingPreview'];
const version = 'drivebridge.desktop.host.v1alpha1';
const unavailable = () => new Error('desktop_host_unavailable');
// The loopback facade answers with the closed DesktopError envelope. Carry that
// value so main can return the owner's real code instead of a generic failure.
function coded(value: unknown): Error {
  const error = new Error('desktop_request_failed');
  (error as Error & { desktopError?: unknown }).desktopError = value;
  return error;
}
export function desktopErrorOf(error: unknown): unknown {
  const value = (error as { desktopError?: unknown })?.desktopError;
  if (!value || typeof value !== 'object') return undefined;
  const candidate = value as { origin?: unknown; code?: unknown; message?: unknown; recovery?: unknown };
  if (candidate.origin !== 'drivebridge' && candidate.origin !== 'pinax') return undefined;
  if (typeof candidate.code !== 'string' || !/^[a-z][a-z0-9_]{0,127}$/.test(candidate.code)) return undefined;
  return value;
}
const validators = new Ajv2020({ strict: false, validateFormats: false });
const previewValid = validators.compile<Preview>(publicContract.components.schemas.BindingPreview);
const id = () => randomBytes(32).toString('hex');
type Bundle = { schema_version: 'drivebridge.desktop.bundle.v1'; drivebridge: string; pinax: string; manifest: string };

export async function verifyBundle(directory: string): Promise<Bundle> {
  try {
    if (await realpath(directory) !== directory) throw unavailable();
    const configPath = join(directory, 'bundle.json');
    const configStat = await lstat(configPath);
    if (!configStat.isFile() || configStat.size > 4096) throw unavailable();
    const value = JSON.parse(await readFile(configPath, 'utf8')) as Bundle;
    if (Object.keys(value).sort().join() !== 'drivebridge,manifest,pinax,schema_version' || value.schema_version !== 'drivebridge.desktop.bundle.v1') throw unavailable();
    for (const name of ['drivebridge', 'pinax', 'manifest'] as const) if (!/^sha256:[a-f0-9]{64}$/.test(value[name])) throw unavailable();
    for (const name of ['drivebridge', 'pinax'] as const) {
      const path = join(directory, name + (process.platform === 'win32' ? '.exe' : ''));
      const stat = await lstat(path);
      if (!stat.isFile() || stat.size > 512 * 1024 * 1024 || process.platform !== 'win32' && (stat.mode & 0o022)) throw unavailable();
      const digest = 'sha256:' + createHash('sha256').update(await readFile(path)).digest('hex');
      if (digest !== value[name]) throw unavailable();
    }
    // The installation directory must itself be trusted. Hash checking is
    // not a filesystem lease or a replacement for signed package verification.
    return value;
  } catch { throw unavailable(); }
}

export class HostClient {
  #child: ChildProcessWithoutNullStreams;
  #token = id();
  #nonce = id();
  #address = '';
  #stateRef = '';
  #exchange = new HostExchange(this.#nonce);
  #frames = new HostFrames();
  #pending = new Map<string, { resolve: (event: HostEvent) => void; reject: (error: Error) => void; timer: ReturnType<typeof setTimeout> }>();
  #readyResolve!: () => void;
  #readyReject!: (error: Error) => void;
  #ready: Promise<void>;
  #readyTimer: ReturnType<typeof setTimeout>;
  #closed = false;
  #exited = false;
  #exit: Promise<void>;
  #closing?: Promise<void>;
  #http = new Set<ReturnType<typeof httpRequest>>();

  private constructor(directory: string, bundle: Bundle, statePath?: string, stateAction?: 'create' | 'migrate') {
    // The host receives only OS-provided user identity locations, never an
    // inherited environment bag and never a renderer value. Without these it
    // cannot resolve the configured owner state, so guard admission would be
    // unreachable; the host still requires an existing state and never creates
    // or migrates one.
    const environment: NodeJS.ProcessEnv = {};
    for (const name of ['HOME', 'USERPROFILE', 'XDG_CONFIG_HOME', 'XDG_CACHE_HOME', 'SystemRoot'] as const) {
      const value = process.env[name];
      if (value) environment[name] = value;
    }
    this.#child = spawn(join(directory, 'drivebridge' + (process.platform === 'win32' ? '.exe' : '')), ['desktop', 'serve'], { cwd: directory, env: environment, shell: false, stdio: 'pipe', windowsHide: true });
    this.#ready = new Promise((resolve, reject) => { this.#readyResolve = resolve; this.#readyReject = reject; });
    this.#readyTimer = setTimeout(() => this.#fail(), 15000);
    this.#exit = new Promise(resolve => {
      this.#child.once('close', () => { this.#exited = true; this.#fail(); resolve(); });
    });
    this.#child.on('error', () => this.#fail());
    this.#child.stdin.on('error', () => this.#fail());
    this.#child.stdout.on('error', () => this.#fail());
    this.#child.stderr.on('error', () => this.#fail());
    this.#child.stdout.on('data', (chunk: Buffer) => {
      if (this.#closed) return;
      try { this.#frames.push(chunk, event => this.#receive(event)); } catch { this.#fail(); }
    });
    this.#child.stdout.on('end', () => { try { this.#frames.finish(); } catch { /* Same safe failure. */ } this.#fail(); });
    // Consume without persisting upstream diagnostics; cap abusive output.
    let stderrBytes = 0;
    this.#child.stderr.on('data', (chunk: Buffer) => { stderrBytes += chunk.length; if (stderrBytes > 64 * 1024) this.#fail(); });
    this.#child.stdin.write(encodeHostRequest({ schema_version: version, session_nonce: this.#nonce, bearer: this.#token, binary_digest: bundle.pinax, manifest_digest: bundle.manifest, ...(statePath ? { state_path: statePath } : {}), ...(stateAction ? { state_action: stateAction } : {}) }), error => { if (error) this.#fail(); });
  }
  static async start(directory: string, statePath?: string, stateAction?: 'create' | 'migrate'): Promise<HostClient> {
    const bundle = await verifyBundle(directory);
    const client = new HostClient(directory, bundle, statePath, stateAction);
    try { await client.#ready; return client; } catch { await client.close(); throw unavailable(); }
  }
  get stateRef(): string { return this.#stateRef; }
  get alive(): boolean { return !this.#closed && !!this.#address; }
  #receive(event: HostEvent): void {
    this.#exchange.accept(event);
    if (event.type === 'ready') { this.#stateRef = event.state_ref ?? ''; this.#address = event.address; clearTimeout(this.#readyTimer); this.#readyResolve(); return; }
    if (event.type === 'shutdown' || event.type === 'error' && !event.request_id) { this.#fail(); return; }
    if (!('request_id' in event) || !event.request_id) throw unavailable();
    const pending = this.#pending.get(event.request_id);
    if (!pending) throw unavailable();
    clearTimeout(pending.timer); this.#pending.delete(event.request_id);
    if (event.type === 'error') pending.reject(unavailable()); else pending.resolve(event);
  }
  #fail(): void {
    if (this.#closed) return;
    this.#closed = true;
    clearTimeout(this.#readyTimer); this.#readyReject(unavailable()); this.#exchange.close();
    for (const pending of this.#pending.values()) { clearTimeout(pending.timer); pending.reject(unavailable()); }
    this.#pending.clear();
    for (const request of this.#http) request.destroy();
    this.#child.stdin.end();
    queueMicrotask(() => { void this.close().catch(() => { /* Explicit close still reports cleanup failure. */ }); });
  }
  #send(action: 'select' | 'register' | 'cancel', value: string): Promise<HostEvent> {
    if (!this.alive) return Promise.reject(unavailable());
    const request = action !== 'cancel'
      ? { schema_version: version, action, request_id: id(), native_path: value } as const
      : { schema_version: version, action, request_id: id(), selection_id: value } as const;
    try { this.#exchange.expect(request); } catch { this.#fail(); return Promise.reject(unavailable()); }
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => this.#fail(), 5000);
      this.#pending.set(request.request_id, { resolve, reject, timer });
      try { this.#child.stdin.write(encodeHostRequest(request), error => { if (error) this.#fail(); }); }
      catch { this.#fail(); }
    });
  }
  async preview(nativePath: string): Promise<Preview> {
    const selected = await this.#send('select', nativePath);
    if (selected.type !== 'selection') throw unavailable();
    let preview: Preview | undefined;
    try {
      const response = await this.#httpRequest('POST', '/v1/desktop/bindings/preview', { native_selection_id: selected.selection_id });
      if (!response || typeof response !== 'object' || Object.keys(response).join() !== 'data') throw unavailable();
      const data = (response as { data: unknown }).data;
      if (!previewValid(data) || data.reason === 'none' && data.decision !== 'ready' || data.scopes.join() !== 'reader.metadata,reader.body,reader.relations,reader.assets' || !Number.isFinite(Date.parse(data.expires_at)) || Date.parse(data.expires_at) <= Date.now()) throw unavailable();
      if (data.decision === 'ready' && (data.pending_mutations !== 0 || data.reason !== 'none')) throw unavailable();
      if (data.decision === 'blocked' && data.reason === 'none') throw unavailable();
      // No future server ready response may silently enable binding here.
      preview = data;
      return data;
    } finally {
      // If the response was invalid/lost, its preview identity is unknown.
      // Closing the session releases all pending previews, never guesses one.
      if (!preview) await this.close();
      else if (preview.decision === 'blocked') await this.#send('cancel', preview.preview_id).catch(() => this.close());
    }
  }
  // bindSource keeps the preview open so main can confirm it. A ready preview is
  // the only one that may be confirmed; the host still owns confirmation and CAS.
  async bindSource(nativePath: string, register = false): Promise<Preview> {
    const selected = await this.#send(register ? 'register' : 'select', nativePath);
    if (selected.type !== 'selection') throw unavailable();
    const response = await this.#httpRequest('POST', '/v1/desktop/bindings/preview', { native_selection_id: selected.selection_id });
    if (!response || typeof response !== 'object' || Object.keys(response).join() !== 'data') throw unavailable();
    const data = (response as { data: unknown }).data;
    if (!previewValid(data)) throw unavailable();
    return data;
  }
  // The loopback facade answers with a single `data` envelope; main consumes the
  // payload only and never forwards an unknown shape to the renderer.
  #unwrap(response: unknown): unknown {
    if (!response || typeof response !== 'object' || Object.keys(response).join() !== 'data') throw unavailable();
    return (response as { data: unknown }).data;
  }
  async confirmBinding(preview: Preview, idempotencyKey: string): Promise<unknown> {
    if (!previewValid(preview) || preview.decision !== 'ready' || !/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(idempotencyKey)) throw unavailable();
    return this.#unwrap(await this.#httpRequest('POST', '/v1/desktop/bindings', { preview_id: preview.preview_id, preview_digest: preview.preview_digest, expected_guard_revision: preview.expected_guard_revision, confirmed: true }, idempotencyKey));
  }
  async readReadingState(input: components['schemas']['ReadReadingState']): Promise<unknown> {
    return this.#unwrap(await this.#httpRequest('POST', '/v1/desktop/reading-state/read', input));
  }
  async updateReadingState(input: components['schemas']['UpdateReadingState'], key: string): Promise<unknown> {
    return this.#unwrap(await this.#httpRequest('PUT', '/v1/desktop/reading-state', input, key));
  }
  async lookupReadingMutation(input: components['schemas']['LookupReadingMutation']): Promise<unknown> {
    return this.#unwrap(await this.#httpRequest('POST', '/v1/desktop/reading-state/lookup', input));
  }
  async listSources(input: { limit?: number; cursor?: string }): Promise<unknown> {
    const query = new URLSearchParams();
    if (input.limit !== undefined) query.set('limit', String(input.limit));
    if (input.cursor) query.set('cursor', input.cursor);
    return this.#unwrap(await this.#httpRequest('GET', '/v1/desktop/sources' + (query.size ? '?' + query : '')));
  }
  async listNotes(sourceID: string, input: { limit?: number; cursor?: string; sort?: string } = {}): Promise<unknown> {
    const query = new URLSearchParams();
    if (input.limit !== undefined) query.set('limit', String(input.limit));
    if (input.cursor) query.set('cursor', input.cursor);
    if (input.sort) query.set('sort', input.sort);
    return this.#unwrap(await this.#httpRequest('GET', `/v1/desktop/sources/${encodeURIComponent(sourceID)}/notes${query.size ? '?' + query.toString() : ''}`));
  }
  async capabilities(): Promise<{ schema_version: string; available: string[] }> {
    const data = this.#unwrap(await this.#httpRequest('GET', '/v1/desktop/capabilities')) as { schema_version?: unknown; available?: unknown };
    if (data?.schema_version !== 'drivebridge.desktop.v1alpha1' || !Array.isArray(data.available) || data.available.some(v => typeof v !== 'string')) throw unavailable();
    return data as { schema_version: string; available: string[] };
  }
  async searchNotes(sourceID: string, input: { q: string; limit?: number; cursor?: string }): Promise<unknown> {
    const query = new URLSearchParams({ q: input.q });
    if (input.limit !== undefined) query.set('limit', String(input.limit));
    if (input.cursor) query.set('cursor', input.cursor);
    return this.#unwrap(await this.#httpRequest('GET', `/v1/desktop/sources/${encodeURIComponent(sourceID)}/search?${query.toString()}`));
  }
  async readDocument(sourceID: string, input: { object_id: string; revision: string; cursor?: string; anchor?: string }): Promise<unknown> {
    const query = new URLSearchParams({ revision: input.revision });
    if (input.cursor) query.set('cursor', input.cursor);
    if (input.anchor) query.set('anchor', input.anchor);
    return this.#unwrap(await this.#httpRequest('GET', `/v1/desktop/sources/${encodeURIComponent(sourceID)}/notes/${encodeURIComponent(input.object_id)}/document?${query.toString()}`));
  }
  async readLinks(sourceID: string, input: { object_id: string; revision: string; direction?: string; limit?: number; cursor?: string }): Promise<unknown> {
    const query = new URLSearchParams({ revision: input.revision });
    if (input.direction) query.set('direction', input.direction);
    if (input.limit !== undefined) query.set('limit', String(input.limit));
    if (input.cursor) query.set('cursor', input.cursor);
    return this.#unwrap(await this.#httpRequest('GET', `/v1/desktop/sources/${encodeURIComponent(sourceID)}/notes/${encodeURIComponent(input.object_id)}/links?${query.toString()}`));
  }
  async readEditorSource(sourceID: string, objectID: string, revision: string): Promise<unknown> {
    const query = new URLSearchParams({ object_id: objectID, revision });
    return this.#unwrap(await this.#httpRequest('GET', `/v1/desktop/sources/${encodeURIComponent(sourceID)}/writer/editor-source?${query.toString()}`));
  }
  async proposeEditorDraft(sourceID: string, input: { object_id: string; revision: string; body: string }): Promise<unknown> {
    return this.#unwrap(await this.#httpRequest('POST', `/v1/desktop/sources/${encodeURIComponent(sourceID)}/writer/editor:propose`, input));
  }
  async releaseProtection(bindingID: string, expectedRevision: number, idempotencyKey: string): Promise<unknown> {
    return this.#unwrap(await this.#httpRequest('POST', `/v1/desktop/bindings/${encodeURIComponent(bindingID)}/release`, { expected_revision: expectedRevision, confirmed: true }, idempotencyKey));
  }
  // Binary data is available only to the native session protocol, never IPC.
  asset(handle: string, range?: string): Promise<Response> {
    if (!/^[a-f0-9]{64}$/.test(handle) || !this.alive) return Promise.reject(unavailable());
    return new Promise((resolve, reject) => {
      const headers: Record<string, string> = { Authorization: `Bearer ${this.#token}` };
      if (range) headers.Range = range;
      const request = httpRequest(new URL('/v1/desktop/assets/' + handle, this.#address), { method: 'GET', agent: false, headers }, response => {
        if (response.statusCode !== 200 && response.statusCode !== 206) { response.resume(); reject(unavailable()); return; }
        const media = response.headers['content-type'] ?? '';
        if (!['image/png', 'image/jpeg', 'image/webp', 'image/gif', 'image/avif'].includes(media)) { response.resume(); reject(unavailable()); return; }
        const chunks: Buffer[] = []; let size = 0;
        response.on('data', (chunk: Buffer) => { size += chunk.length; if (size > 8 * 1024 * 1024) request.destroy(unavailable()); else chunks.push(chunk); });
        response.on('error', () => reject(unavailable()));
        response.on('end', () => {
          if (!this.alive) { reject(unavailable()); return; }
          const output: Record<string, string> = { 'Content-Type': media, 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff', 'Content-Security-Policy': "default-src 'none'" };
          const contentRange = response.headers['content-range'];
          if (typeof contentRange === 'string') output['Content-Range'] = contentRange;
          resolve(new Response(new Uint8Array(Buffer.concat(chunks)), { status: response.statusCode, headers: output }));
        });
      });
      this.#http.add(request); request.once('close', () => this.#http.delete(request));
      request.setTimeout(15000, () => request.destroy(unavailable())); request.on('error', () => reject(unavailable())); request.end();
    });
  }
  #httpRequest(method: 'GET' | 'POST' | 'PUT', path: string, body?: unknown, idempotencyKey?: string): Promise<unknown> {
    return new Promise((resolve, reject) => {
      if (!this.alive) { reject(unavailable()); return; }
      const payload = body === undefined ? '' : JSON.stringify(body);
      const headers: Record<string, string> = { Authorization: `Bearer ${this.#token}` };
      if (payload) { headers['Content-Type'] = 'application/json'; headers['Content-Length'] = String(Buffer.byteLength(payload)); }
      if (idempotencyKey) headers['Idempotency-Key'] = idempotencyKey;
      const request = httpRequest(new URL(path, this.#address), { method, agent: false, headers }, response => {
        const chunks: Buffer[] = []; let size = 0;
        response.on('data', (chunk: Buffer) => { size += chunk.length; if (size > 128 * 1024) request.destroy(); else chunks.push(chunk); });
        response.on('error', () => reject(unavailable()));
        response.on('end', () => {
          if (this.#closed || !response.headers['content-type']?.startsWith('application/json')) { reject(unavailable()); return; }
          let parsed: unknown;
          try { parsed = JSON.parse(Buffer.concat(chunks).toString('utf8')); } catch { reject(unavailable()); return; }
          if (response.statusCode === 200) { resolve(parsed); return; }
          const failure = (parsed as { error?: unknown })?.error;
          reject(desktopErrorOf({ desktopError: failure }) ? coded(failure) : unavailable());
        });
      });
      const deadline = setTimeout(() => request.destroy(), 25000);
      this.#http.add(request);
      request.on('close', () => { clearTimeout(deadline); this.#http.delete(request); });
      request.on('error', () => reject(unavailable()));
      if (payload) request.write(payload);
      request.end();
    });
  }
  close(): Promise<void> {
    if (this.#closing) return this.#closing;
    this.#closing = (async () => {
      this.#fail();
      if (this.#exited) return;
      let kill: ReturnType<typeof setTimeout> | undefined;
      let timeout: ReturnType<typeof setTimeout> | undefined;
      const bounded = new Promise<void>((resolve, reject) => {
        kill = setTimeout(() => this.#child.kill('SIGKILL'), 7000);
        timeout = setTimeout(() => reject(new Error('desktop_host_cleanup_timeout')), 9000);
        void this.#exit.then(resolve);
      });
      try { await bounded; } finally { clearTimeout(kill); clearTimeout(timeout); this.#token = ''; this.#address = ''; this.#stateRef = ''; }
    })();
    return this.#closing;
  }
}
