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

  private constructor(directory: string, bundle: Bundle) {
    const environment: NodeJS.ProcessEnv = {};
    if (process.platform === 'win32' && process.env.SystemRoot) environment.SystemRoot = process.env.SystemRoot;
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
    this.#child.stdin.write(encodeHostRequest({ schema_version: version, session_nonce: this.#nonce, bearer: this.#token, binary_digest: bundle.pinax, manifest_digest: bundle.manifest }), error => { if (error) this.#fail(); });
  }
  static async start(directory: string): Promise<HostClient> {
    const bundle = await verifyBundle(directory);
    const client = new HostClient(directory, bundle);
    try { await client.#ready; return client; } catch { await client.close(); throw unavailable(); }
  }
  get alive(): boolean { return !this.#closed && !!this.#address; }
  #receive(event: HostEvent): void {
    this.#exchange.accept(event);
    if (event.type === 'ready') { this.#address = event.address; clearTimeout(this.#readyTimer); this.#readyResolve(); return; }
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
  #send(action: 'select' | 'cancel', value: string): Promise<HostEvent> {
    if (!this.alive) return Promise.reject(unavailable());
    const request = action === 'select'
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
      const response = await this.#previewRequest(selected.selection_id);
      if (!response || typeof response !== 'object' || Object.keys(response).join() !== 'data') throw unavailable();
      const data = (response as { data: unknown }).data;
      if (!previewValid(data) || data.decision !== 'blocked' || data.reason === 'none' || data.scopes.length !== 1 || data.scopes[0] !== 'reader.metadata' || !Number.isFinite(Date.parse(data.expires_at)) || Date.parse(data.expires_at) <= Date.now()) throw unavailable();
      // No future server ready response may silently enable binding here.
      preview = data;
      return data;
    } finally {
      // If the response was invalid/lost, its preview identity is unknown.
      // Closing the session releases all pending previews, never guesses one.
      if (!preview) await this.close();
      else await this.#send('cancel', preview.preview_id).catch(() => this.close());
    }
  }
  #previewRequest(ticket: string): Promise<{ data?: unknown }> {
    return new Promise((resolve, reject) => {
      const url = new URL('/v1/desktop/bindings/preview', this.#address);
      const body = JSON.stringify({ native_selection_id: ticket });
      const request = httpRequest(url, { method: 'POST', agent: false, headers: { Authorization: `Bearer ${this.#token}`, 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(body) } }, response => {
        const chunks: Buffer[] = []; let size = 0;
        response.on('data', (chunk: Buffer) => { size += chunk.length; if (size > 128 * 1024) request.destroy(); else chunks.push(chunk); });
        response.on('error', () => reject(unavailable()));
        response.on('end', () => {
          if (this.#closed || response.statusCode !== 200 || !response.headers['content-type']?.startsWith('application/json')) { reject(unavailable()); return; }
          try { resolve(JSON.parse(Buffer.concat(chunks).toString('utf8'))); } catch { reject(unavailable()); }
        });
      });
      const deadline = setTimeout(() => request.destroy(), 25000);
      this.#http.add(request);
      request.on('close', () => { clearTimeout(deadline); this.#http.delete(request); });
      request.on('error', () => reject(unavailable()));
      request.end(body);
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
      try { await bounded; } finally { clearTimeout(kill); clearTimeout(timeout); this.#token = ''; this.#address = ''; }
    })();
    return this.#closing;
  }
}
