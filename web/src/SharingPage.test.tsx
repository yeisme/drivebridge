import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { SharesPage } from './Sharing';
const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status });
const row = { id: 'private-share', name: 'Share fixture', mode: 'fixed', audience: 'users', path: '', expires_at: '2099-01-01', revoked: false, allow_download: false };
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.unstubAllGlobals(); });
it('does not show empty success while loading or failed, and retries explicitly', async () => {
  let finish!: (value: Response) => void;
  const fetch = vi.spyOn(globalThis, 'fetch').mockImplementationOnce(() => new Promise(resolve => { finish = resolve; })).mockResolvedValueOnce(json({ data: [] }));
  render(<SharesPage/>);
  expect(screen.queryByText('还没有分享')).toBeNull();
  finish(json({}, 503));
  await screen.findByRole('alert');
  expect(screen.queryByText('还没有分享')).toBeNull();
  fireEvent.click(screen.getByRole('button', { name: '刷新分享' }));
  await screen.findByText('还没有分享');
  expect(screen.queryByText('暂无记录')).toBeNull();
  expect(screen.queryByRole('alert')).toBeNull();
  expect(fetch).toHaveBeenCalledTimes(2);
});
it.each(['drivebridge-context', 'unmount'])('does not copy a link arriving after %s', async action => {
  let finish!: (value: Response) => void;
  const copy = vi.fn().mockResolvedValue(undefined);
  vi.stubGlobal('navigator', Object.assign(Object.create(navigator), { clipboard: { writeText: copy } }));
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({ data: [row] })).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const view = render(<SharesPage/>);
  fireEvent.click(await screen.findByRole('button', { name: '复制链接' }));
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
  if (action === 'unmount') view.unmount(); else fireEvent(window, new Event(action));
  finish(json({ data: { link: 'https://example.invalid/synthetic-private-link' } }));
  await waitFor(() => expect(fetch.mock.calls[1][1]?.signal?.aborted).toBe(true));
  expect(copy).not.toHaveBeenCalled();
});
it.each([401, 403])('clears shares on a denied link read (%s)', async status => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({ data: [row] })).mockResolvedValueOnce(json({}, status));
  render(<SharesPage/>); fireEvent.click(await screen.findByRole('button', { name: '复制链接' }));
  await screen.findByText(/身份或范围已变化/); expect(screen.queryByText(row.name)).toBeNull();
});
it('stops a confirmed batch immediately after authorization loss', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({ data: [row, { ...row, id: 'second' }] })).mockResolvedValueOnce(json({}, 403));
  render(<SharesPage/>);
  fireEvent.click(await screen.findByRole('checkbox', { name: '选择分享交付当前页' }));
  fireEvent.click(screen.getByRole('button', { name: '撤销所选' }));
  fireEvent.click(await screen.findByRole('button', { name: '确认撤销所选' }));
  await screen.findByText(/身份或范围已变化/);
  expect(fetch.mock.calls.filter(([, request]) => request?.method === 'POST')).toHaveLength(1);
  expect(screen.queryByRole('alertdialog')).toBeNull();
});
it('clears previous shares and discards a late read after context invalidation', async () => {
  let finish!: (value: Response) => void;
  vi.spyOn(globalThis, 'fetch').mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  render(<SharesPage/>); fireEvent(window, new Event('drivebridge-context'));
  finish(json({ data: [{ id: 'private-share', name: 'Old identity', mode: 'fixed', audience: 'users', path: '', expires_at: '2099-01-01', revoked: false, allow_download: false }] }));
  await waitFor(() => expect(screen.queryByText('Old identity')).toBeNull());
  expect(screen.queryByText('还没有分享')).toBeNull();
  expect(screen.getByRole('alert').textContent).toContain('范围');
});
