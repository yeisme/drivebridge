/// <reference lib="es2024.promise" />
import { api, errorText, setCSRF } from './api';
import { hmac } from '@noble/hashes/hmac.js';
import { sha256 } from '@noble/hashes/sha2.js';

export interface ShareOpenResult {
  opened: boolean;
  session_kind: 'grant' | 'proof' | 'device';
  receiver_kind: 'anonymous' | 'device' | 'user';
}
export interface ShareReceiverDevice {
  id: string;
  fingerprint: string;
  label: string;
  created_at: string;
  last_seen: string;
  opens: number;
  revoked: boolean;
}
export interface ShareReceivers {
  devices: ShareReceiverDevice[];
  opens_used: number;
  opens_max: number | null;
  downloads_used: number;
  downloads_max: number | null;
}
export interface StoredShareDevice {
  id?: string;
  privateKey: CryptoKey;
  publicKey: string;
}
export const deviceStorageNotice = '此浏览器无法安全保存 Ed25519 设备密钥。请保留完整分享链接，复访时仍需完整链接与提取码。';
export function receiverError(error: unknown): string {
  const code = error instanceof Error ? error.message : '';
  if (code === 'share_unavailable') return '分享无法打开：可能已过期、撤销、设备失效或额度耗尽。请联系分享者；不会改用旧方式绕过验证。';
  if (code === 'device_storage_unavailable') return deviceStorageNotice;
  if (code === 'full_link_required') return '请重新打开完整分享链接；此浏览器没有可用的已登记设备。';
  if (code === 'unsupported_action') return '服务器未启用接收设备功能。请使用完整分享链接与提取码。';
  if (code === 'disabled') return '分享服务已关闭，无法打开分享。请联系分享者。';
  return errorText(error);
}
const path = (id: string) => `/shared/${encodeURIComponent(id)}`;
const hex = (bytes: Uint8Array) => Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('');
const message = (id: string, nonce: string) => new TextEncoder().encode(`${id}:${nonce}`);
async function challenge(id: string, signal: AbortSignal) {
  const result = await api<{ nonce: string }>(`${path(id)}/challenge`, 'GET', undefined, {}, signal);
  if (!result || typeof result.nonce !== 'string' || !result.nonce) throw new Error('invalid_contract');
  return result.nonce;
}
function opened(result: ShareOpenResult): ShareOpenResult {
  if (!result?.opened) throw new Error('invalid_contract');
  return result;
}
export async function openShareProof(id: string, secret: string, passcode: string, signal: AbortSignal): Promise<ShareOpenResult> {
  let nonce: string;
  try { nonce = await challenge(id, signal); }
  catch (error) {
    // Only unsupported_action identifies an unavailable receiver capability.
    // disabled shuts down the whole service; never transmit a grant after it.
    // Proof, credential, quota, HTTP 404 and transport failures never downgrade.
    if (!(error instanceof Error) || error.message !== 'unsupported_action') throw error;
    try { const me = await api<{ csrf_token: string }>('/me', 'GET', undefined, {}, signal); setCSRF(me.csrf_token); }
    catch (authError) { if (signal.aborted) throw authError; }
    const result = await api<ShareOpenResult>(`${path(id)}/open`, 'POST', { grant: secret, passcode }, {}, signal);
    return { ...opened(result), session_kind: 'grant', receiver_kind: result.receiver_kind || 'anonymous' };
  }
  const proof = hex(hmac(sha256, new TextEncoder().encode(secret), message(id, nonce)));
  return opened(await api<ShareOpenResult>(`${path(id)}/proof-open`, 'POST', { nonce, proof, passcode }, {}, signal));
}
export async function openShareDevice(id: string, device: StoredShareDevice, signal: AbortSignal): Promise<ShareOpenResult> {
  if (!device.id) throw new Error('full_link_required');
  const nonce = await challenge(id, signal);
  let signature: ArrayBuffer;
  try { signature = await crypto.subtle.sign('Ed25519', device.privateKey, message(id, nonce)); }
  catch { throw new Error('device_storage_unavailable'); }
  signal.throwIfAborted();
  return opened(await api<ShareOpenResult>(`${path(id)}/device-open`, 'POST', { device_id: device.id, nonce, signature: hex(new Uint8Array(signature)) }, {}, signal));
}

// IndexedDB belongs to this browser profile and origin, not the logged-in actor.
// The explicit origin/share key prevents cross-share reuse during migrations;
// changing login does not delete the anonymous receiver device credential.
const database = 'drivebridge-share-receivers';
const store = 'devices';
async function deviceRecord(id: string, value?: StoredShareDevice): Promise<StoredShareDevice | undefined> {
  if (!globalThis.indexedDB || !globalThis.crypto?.subtle || typeof Promise.withResolvers !== 'function') throw new Error('device_storage_unavailable');
  const { promise, resolve, reject } = Promise.withResolvers<StoredShareDevice | undefined>();
  let settled = false;
  const fail = () => { settled = true; reject(new Error('device_storage_unavailable')); };
  let request: IDBOpenDBRequest;
  try { request = indexedDB.open(database, 1); }
  catch { throw new Error('device_storage_unavailable'); }
  request.onupgradeneeded = () => { request.result.createObjectStore(store); };
  request.onerror = fail;
  request.onblocked = fail;
  request.onsuccess = () => {
    const db = request.result;
    if (settled) { db.close(); return; }
    db.onversionchange = () => db.close();
    try {
      const tx = db.transaction(store, value ? 'readwrite' : 'readonly');
      const records = tx.objectStore(store);
      const key = `${location.origin}/share/${encodeURIComponent(id)}`;
      const operation = value ? records.put(value, key) : records.get(key);
      tx.oncomplete = () => { db.close(); resolve(value || operation.result as StoredShareDevice | undefined); };
      tx.onabort = tx.onerror = () => { db.close(); fail(); };
    } catch { db.close(); fail(); }
  };
  return promise;
}
export async function loadShareDevice(id: string): Promise<StoredShareDevice | undefined> {
  const device = await deviceRecord(id);
  if (device && (!(device.privateKey instanceof CryptoKey) || device.privateKey.type !== 'private' || device.privateKey.extractable || device.privateKey.algorithm.name !== 'Ed25519' || !/^[a-f0-9]{64}$/.test(device.publicKey) || (device.id !== undefined && (typeof device.id !== 'string' || !device.id)))) throw new Error('device_storage_unavailable');
  return device;
}
export async function registerShareDevice(id: string, signal: AbortSignal): Promise<StoredShareDevice> {
  signal.throwIfAborted();
  let device = await loadShareDevice(id);
  signal.throwIfAborted();
  if (!device) {
    try {
      const keys = await crypto.subtle.generateKey('Ed25519', false, ['sign', 'verify']) as CryptoKeyPair;
      device = { privateKey: keys.privateKey, publicKey: hex(new Uint8Array(await crypto.subtle.exportKey('raw', keys.publicKey))) };
      signal.throwIfAborted();
      await deviceRecord(id, device);
      // Test the actual persisted CryptoKey, not just in-memory crypto support.
      device = await loadShareDevice(id);
      if (!device) throw new Error('device_storage_unavailable');
      await crypto.subtle.sign('Ed25519', device.privateKey, message(id, 'storage-check'));
    } catch (error) { if (signal.aborted) throw error; throw new Error('device_storage_unavailable'); }
  }
  signal.throwIfAborted();
  const registered = await api<ShareReceiverDevice>(`${path(id)}/devices`, 'POST', { public_key: device.publicKey, label: '此浏览器' }, {}, signal);
  if (!registered?.id || registered.revoked) throw new Error('share_unavailable');
  device = { ...device, id: registered.id };
  await deviceRecord(id, device);
  signal.throwIfAborted();
  return device;
}
