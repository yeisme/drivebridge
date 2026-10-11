/// <reference lib="es2024.promise" />
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { ShareReceiversDialog } from './ShareReceiversDialog';
const json = (data: unknown, status = 200) => new Response(JSON.stringify(status === 200 ? { data } : { error: { code: data } }), { status });
const device = { id: 'device', fingerprint: 'public-fingerprint', label: 'Fixture browser', created_at: '2026-10-09', last_seen: '2026-10-09', opens: 1, revoked: false };
const listing = { devices: [device], opens_used: 2, opens_max: 3, downloads_used: 0, downloads_max: null };
afterEach(() => { cleanup(); vi.restoreAllMocks(); });
it('loads exact owner counters and revokes only the confirmed device', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(listing)).mockResolvedValueOnce(json({ revoked: true })).mockResolvedValueOnce(json({ ...listing, devices: [{ ...device, revoked: true }] }));
  render(<ShareReceiversDialog id="share" close={() => {}}/>);
  await screen.findByText(/打开次数：2/);
  fireEvent.click(screen.getByRole('button', { name: '撤销设备 Fixture browser' }));
  expect(fetch).toHaveBeenCalledTimes(1);
  fireEvent.click(await screen.findByRole('button', { name: '确认撤销设备' }));
  await screen.findByText('已撤销');
  expect(fetch.mock.calls[1][0]).toBe('/api/v1/shares/share/receivers/device');
  expect(fetch.mock.calls[1][1]?.method).toBe('DELETE');
});
it('reports feature-disabled instead of empty success', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(json('unsupported_action', 400));
  render(<ShareReceiversDialog id="share" close={() => {}}/>);
  await screen.findByText(/服务器未启用接收设备功能/);
  expect(screen.queryByText('尚未登记接收设备。')).toBeNull();
});
it.each(['drivebridge-context', 'unmount'])('discards owner data after %s', async action => {
  const pending = Promise.withResolvers<Response>();
  const fetch = vi.spyOn(globalThis, 'fetch').mockReturnValue(pending.promise);
  const view = render(<ShareReceiversDialog id="share" close={() => {}}/>);
  if (action === 'unmount') view.unmount(); else fireEvent(window, new Event(action));
  pending.resolve(json(listing));
  await waitFor(() => expect(fetch.mock.calls[0][1]?.signal?.aborted).toBe(true));
  expect(screen.queryByText(device.fingerprint)).toBeNull();
});
