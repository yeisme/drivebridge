import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { SharedView } from './Sharing';
import * as receiver from './share-receiver';
const json = (data: unknown, status = 200) => new Response(JSON.stringify(status === 200 ? { data } : { error: { code: data } }), { status });
const listing = { name: 'Receiver fixture', mode: 'fixed', expires_at: '2099-01-01', files: [], allow_download: false };
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.unstubAllGlobals(); history.replaceState(null, '', '/'); });
it('opens a full link with proof, strips fragment secret and registers only with consent', async () => {
  history.replaceState(null, '', '/#share/fixture/local-secret');
  const register = vi.spyOn(receiver, 'registerShareDevice').mockResolvedValue({ id: 'device' } as receiver.StoredShareDevice);
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({ nonce: 'nonce' })).mockResolvedValueOnce(json({ opened: true, session_kind: 'proof', receiver_kind: 'anonymous' })).mockResolvedValueOnce(json(listing));
  render(<SharedView/>);
  await screen.findByText('Receiver fixture');
  expect(location.hash).toBe('#share/fixture');
  expect(JSON.stringify(fetch.mock.calls)).not.toContain('local-secret');
  expect(register).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: '记住此设备' }));
  await screen.findByText(/此设备已登记/);
  expect(register).toHaveBeenCalledTimes(1);
});
it('keeps the secret only in memory for a passcode retry with a fresh challenge', async () => {
  history.replaceState(null, '', '/#share/fixture/local-secret');
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({ nonce: 'first' })).mockResolvedValueOnce(json('share_passcode_required', 403)).mockResolvedValueOnce(json({ nonce: 'second' })).mockResolvedValueOnce(json({ opened: true, session_kind: 'proof', receiver_kind: 'anonymous' })).mockResolvedValueOnce(json(listing));
  render(<SharedView/>); await screen.findByRole('button', { name: '打开分享' });
  fireEvent.change(screen.getByLabelText('提取码'), { target: { value: 'correct-code' } });
  fireEvent.click(screen.getByRole('button', { name: '打开分享' }));
  await screen.findByText('Receiver fixture');
  expect(fetch.mock.calls.filter(([url]) => String(url).endsWith('/challenge'))).toHaveLength(2);
  expect(JSON.stringify(fetch.mock.calls)).not.toContain('local-secret');
});
it.each(['unmount', 'drivebridge-context', 'hashchange'])('discards late receiver reads on %s', async event => {
  history.replaceState(null, '', '/#share/fixture');
  const pending = Promise.withResolvers<Response>();
  const fetch = vi.spyOn(globalThis, 'fetch').mockReturnValueOnce(pending.promise);
  const view = render(<SharedView/>);
  if (event === 'unmount') view.unmount();
  else if (event === 'hashchange') { history.replaceState(null, '', '/#/files'); fireEvent(window, new Event(event)); }
  else fireEvent(window, new Event(event));
  pending.resolve(json(listing));
  await waitFor(() => expect(fetch.mock.calls[0][1]?.signal?.aborted).toBe(true));
  expect(screen.queryByText('Receiver fixture')).toBeNull();
});
it.each(['unauthorized', 'share_unavailable'])('uses a stored device after %s without a passcode or legacy fallback', async code => {
  history.replaceState(null, '', '/#share/fixture');
  vi.spyOn(receiver, 'loadShareDevice').mockResolvedValue({ id: 'device' } as receiver.StoredShareDevice);
  const open = vi.spyOn(receiver, 'openShareDevice').mockResolvedValue({ opened: true, session_kind: 'device', receiver_kind: 'device' });
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(code, 403)).mockResolvedValueOnce(json(listing));
  render(<SharedView/>); await screen.findByText('Receiver fixture');
  expect(open).toHaveBeenCalledTimes(1);
  expect(screen.queryByLabelText('提取码')).toBeNull();
  expect(screen.queryByRole('button', { name: '记住此设备' })).toBeNull();
  expect(fetch.mock.calls.some(([url]) => String(url).endsWith('/open'))).toBe(false);
});
it('reports unavailable device storage honestly without registering or claiming remembered state', async () => {
  history.replaceState(null, '', '/#share/fixture');
  vi.spyOn(receiver, 'loadShareDevice').mockRejectedValue(new Error('device_storage_unavailable'));
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json('share_unavailable', 403));
  render(<SharedView/>); await screen.findByText(receiver.deviceStorageNotice);
  expect(screen.queryByText(/此设备已登记/)).toBeNull();
  expect(fetch).toHaveBeenCalledTimes(1);
});
it('preserves public listing compatibility without challenge or device registration', async () => {
  history.replaceState(null, '', '/#share/fixture');
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(listing));
  render(<SharedView/>); await screen.findByText('Receiver fixture');
  expect(fetch).toHaveBeenCalledTimes(1);
  expect(screen.queryByRole('button', { name: '记住此设备' })).toBeNull();
});
it('does not claim remembered state when registration finishes after context loss', async () => {
  history.replaceState(null, '', '/#share/fixture/local-secret');
  const pending = Promise.withResolvers<receiver.StoredShareDevice>();
  vi.spyOn(receiver, 'registerShareDevice').mockReturnValue(pending.promise);
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({ nonce: 'nonce' })).mockResolvedValueOnce(json({ opened: true, session_kind: 'proof', receiver_kind: 'anonymous' })).mockResolvedValueOnce(json(listing));
  render(<SharedView/>);
  fireEvent.click(await screen.findByRole('button', { name: '记住此设备' }));
  fireEvent(window, new Event('drivebridge-context'));
  pending.resolve({ id: 'device' } as receiver.StoredShareDevice);
  await waitFor(() => expect(screen.queryByText('Receiver fixture')).toBeNull());
  expect(screen.queryByText(/此设备已登记/)).toBeNull();
});
