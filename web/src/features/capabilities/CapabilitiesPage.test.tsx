import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { CapabilitiesPage } from './CapabilitiesPage';
import { setCSRF } from '../../app/transport';
import type { Space } from '../../api';
const spaces = [{ id: 'space', name: 'Personal' }] as Space[];
const discovery = { actions: ['file_list'], limits: { max_file_bytes: 1024 } };
const readiness = { action: 'file_list', implemented: true, configured: false, authorized: true, dependency_status: 'none', availability: 'available', connection_observed_at: null };
const json = (data: unknown, status = 200) => new Response(JSON.stringify({ data }), { status });
afterEach(() => { cleanup(); vi.restoreAllMocks(); });

it('only reads discovery on entry and checks one action explicitly with its space and CSRF', async () => {
  setCSRF('fixture-csrf');
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(discovery)).mockResolvedValueOnce(json(readiness));
  render(<CapabilitiesPage scope="alice" spaces={spaces}/>);
  const action = await screen.findByRole('button', { name: '检查能力 file_list' });
  expect(fetch).toHaveBeenCalledTimes(1);
  expect(fetch.mock.calls[0][0]).toBe('/api/v1/capabilities?space_id=space');
  fireEvent.click(action);
  await screen.findByRole('region', { name: '动作可用性观察' });
  expect(fetch.mock.calls[1][0]).toBe('/api/v1/capabilities/check');
  expect(fetch.mock.calls[1][1]).toMatchObject({ method: 'POST', headers: { 'X-CSRF-Token': 'fixture-csrf' }, body: JSON.stringify({ action: 'file_list', space_id: 'space' }) });
  expect(screen.getByText(/不是后续执行许可/)).toBeTruthy();
});
it('cannot check an action without a space', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(discovery));
  render(<CapabilitiesPage scope="alice" spaces={[]}/>);
  expect((await screen.findByRole('button', { name: '检查能力 file_list' })).hasAttribute('disabled')).toBe(true);
  expect(fetch).toHaveBeenCalledTimes(1);
});
it.each([401, 403])('clears results and disables further reads after check authorization failure %s', async status => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(discovery)).mockResolvedValueOnce(json({}, status));
  render(<CapabilitiesPage scope="alice" spaces={spaces}/>);
  fireEvent.click(await screen.findByRole('button', { name: '检查能力 file_list' }));
  await screen.findByText(/权限或范围已变化/);
  expect(screen.queryByRole('list', { name: '服务动作目录' })).toBeNull();
  expect(screen.getByRole('button', { name: '重读能力说明' }).hasAttribute('disabled')).toBe(true);
  expect(fetch).toHaveBeenCalledTimes(2);
});
it('discards an abort-insensitive check response after context invalidation', async () => {
  let resolve!: (value: Response) => void;
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(discovery)).mockImplementationOnce(() => new Promise(done => { resolve = done; }));
  render(<CapabilitiesPage scope="alice" spaces={spaces}/>);
  fireEvent.click(await screen.findByRole('button', { name: '检查能力 file_list' }));
  fireEvent(window, new Event('drivebridge-context')); resolve(json(readiness));
  await waitFor(() => expect(screen.queryByRole('region', { name: '动作可用性观察' })).toBeNull());
  expect(screen.queryByText('浏览文件')).toBeNull();
});
it('offline entry does not send requests and reconnect does not automatically retry', async () => {
  vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false);
  const fetch = vi.spyOn(globalThis, 'fetch');
  render(<CapabilitiesPage scope="alice" spaces={spaces}/>);
  await screen.findByText(/当前离线/); fireEvent(window, new Event('online'));
  expect(fetch).not.toHaveBeenCalled();
});
it.each([404, 500])('distinguishes absent discovery from read failure %s', async status => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json({}, status));
  render(<CapabilitiesPage scope="alice" spaces={spaces}/>);
  await screen.findByText(status === 404 ? /没有提供可识别/ : /能力说明读取失败/);
  expect(fetch).toHaveBeenCalledTimes(1);
});
it('scope replacement cannot retain the previous observation', async () => {
  let resolve!: (value: Response) => void;
  vi.spyOn(globalThis, 'fetch').mockImplementationOnce(() => new Promise(done => { resolve = done; })).mockResolvedValueOnce(json({ actions: [], limits: {} }));
  const view = render(<CapabilitiesPage scope="alice" spaces={spaces}/>);
  view.rerender(<CapabilitiesPage scope="bob" spaces={spaces}/>);
  await screen.findByRole('region', { name: '服务限额' }); resolve(json(discovery));
  await waitFor(() => expect(screen.queryByRole('button', { name: '检查能力 file_list' })).toBeNull());
});
