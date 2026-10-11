import { webcrypto, createHmac } from 'node:crypto';
import { afterEach, expect, it, vi } from 'vitest';
import { openShareProof, openShareDevice, registerShareDevice, receiverError, type StoredShareDevice } from './share-receiver';

const json = (data: unknown, status = 200) => new Response(JSON.stringify(status === 200 ? { data } : { error: { code: data } }), { status });
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });
it('matches SDK HMAC format and never transmits the link secret', async () => {
  vi.stubGlobal('crypto', webcrypto);
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({ nonce: 'challenge' })).mockResolvedValueOnce(json({ opened: true, session_kind: 'proof', receiver_kind: 'anonymous' }));
  await openShareProof('share', 'local-only-secret', 'passcode', new AbortController().signal);
  const body = JSON.parse(fetch.mock.calls[1][1]!.body as string);
  expect(body).toEqual({ nonce: 'challenge', proof: createHmac('sha256', 'local-only-secret').update('share:challenge').digest('hex'), passcode: 'passcode' });
  expect(JSON.stringify(fetch.mock.calls)).not.toContain('local-only-secret');
  expect(fetch.mock.calls[0][1]?.headers).not.toHaveProperty('Origin');
});
it.each(['disabled', 'share_unavailable', 'throttled', 'forbidden', 'request_failed'])('does not fall back on challenge failure %s', async code => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(code, 403));
  await expect(openShareProof('share', 'secret', '', new AbortController().signal)).rejects.toThrow(code);
  expect(fetch).toHaveBeenCalledTimes(1);
  expect(JSON.stringify(fetch.mock.calls)).not.toContain('secret');
});
it('uses legacy only after explicit unsupported_action challenge rejection', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json('unsupported_action', 400)).mockResolvedValueOnce(json({ csrf_token: 'csrf' })).mockResolvedValueOnce(json({ opened: true }));
  const result = await openShareProof('share', 'secret', '', new AbortController().signal);
  expect(result.session_kind).toBe('grant');
  expect(fetch.mock.calls[2][0]).toBe('/api/v1/shared/share/open');
});
it.each(['share_unavailable', 'unsupported_action', 'disabled'])('never retries proof failure %s through legacy', async code => {
  vi.stubGlobal('crypto', webcrypto);
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({ nonce: 'nonce' })).mockResolvedValueOnce(json(code, 403));
  await expect(openShareProof('share', 'secret', '', new AbortController().signal)).rejects.toThrow(code);
  expect(fetch).toHaveBeenCalledTimes(2);
});
it('signs exact SDK device message with a non-exportable private key', async () => {
  vi.stubGlobal('crypto', webcrypto);
  const pair = await webcrypto.subtle.generateKey('Ed25519', false, ['sign', 'verify']) as webcrypto.CryptoKeyPair;
  const publicKey = await webcrypto.subtle.exportKey('raw', pair.publicKey);
  const device = { id: 'device', privateKey: pair.privateKey, publicKey: Array.from(new Uint8Array(publicKey), b => b.toString(16).padStart(2, '0')).join('') } as StoredShareDevice;
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({ nonce: 'nonce' })).mockResolvedValueOnce(json({ opened: true, session_kind: 'device', receiver_kind: 'device' }));
  await openShareDevice('share', device, new AbortController().signal);
  const body = JSON.parse(fetch.mock.calls[1][1]!.body as string);
  expect(body.device_id).toBe('device');
  expect(await webcrypto.subtle.verify('Ed25519', pair.publicKey, Buffer.from(body.signature, 'hex'), new TextEncoder().encode('share:nonce'))).toBe(true);
  await expect(webcrypto.subtle.exportKey('pkcs8', pair.privateKey)).rejects.toThrow();
});
it('does not send proof after cancellation during challenge', async () => {
  const controller = new AbortController();
  vi.stubGlobal('crypto', webcrypto);
  const fetch = vi.spyOn(globalThis, 'fetch').mockImplementation(async () => { controller.abort(); return json({ nonce: 'nonce' }); });
  await expect(openShareProof('share', 'secret', '', controller.signal)).rejects.toThrow();
  expect(fetch).toHaveBeenCalledTimes(1);
});
it('reports unavailable without inventing the hidden revocation or quota reason', () => {
  expect(receiverError(new Error('share_unavailable'))).toContain('额度');
  expect(receiverError(new Error('share_unavailable'))).toContain('撤销');
});
it('does not register when secure device storage is unavailable', async () => {
  vi.stubGlobal('indexedDB', undefined);
  const fetch = vi.spyOn(globalThis, 'fetch');
  await expect(registerShareDevice('share', new AbortController().signal)).rejects.toThrow('device_storage_unavailable');
  expect(fetch).not.toHaveBeenCalled();
});
it('does not downgrade a missing HMAC WebCrypto implementation to legacy', async () => {
  vi.stubGlobal('crypto', {});
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({ nonce: 'nonce' })).mockResolvedValueOnce(json({ opened: true, session_kind: 'proof', receiver_kind: 'anonymous' }));
  await openShareProof('share', 'secret', '', new AbortController().signal);
  expect(fetch.mock.calls[1][0]).toBe('/api/v1/shared/share/proof-open');
});
