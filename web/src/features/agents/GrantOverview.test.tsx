import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import fixtures from '../../../../contracts/personal-workspace/fixtures/responses.json';
import { GrantOverview } from './GrantOverview';
import type { GrantChecks } from './grant-client';
import { setCSRF } from '../../app/transport';

const page = fixtures.GrantPageEnvelope;
const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status, headers: { 'Content-Type': 'application/json' } });
const observed: GrantChecks = { checked_at: '2026-01-01T00:00:00Z', checks: [{ name: 'grant_active', status: 'observed_ok' }], connection_state: 'unknown' };
afterEach(() => { cleanup(); vi.restoreAllMocks(); });
it('reads generated paginated grants and explicitly checks the fixed revision with CSRF', async () => {
  setCSRF('fixture-csrf');
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json({ data: observed }));
  render(<GrantOverview/>);
  await screen.findByText('Reader');
  expect(screen.getByText(/长期有效/)).toBeTruthy();
  expect(fetch).toHaveBeenCalledTimes(1);
  expect(fetch.mock.calls[0][0]).toBe('/api/v1/agent-grants?limit=50');
  fireEvent.click(screen.getByRole('button', { name: '检查 Reader' }));
  await screen.findByText('授权状态：本地检查通过');
  expect(screen.getByText('远端连接：未验证')).toBeTruthy();
  const [url, options] = fetch.mock.calls[1];
  expect(url).toBe('/api/v1/agent-grants/grant-fixture/checks');
  expect(options?.method).toBe('POST');
  expect(JSON.parse(options?.body as string)).toEqual({ expected_revision: 1 });
  expect(options?.headers).toMatchObject({ 'X-CSRF-Token': 'fixture-csrf' });
  expect(fetch.mock.calls.every(([url]) => !String(url).includes('rotate'))).toBe(true);
});
it('opens a grant-scoped template without extra requests and clears it on context invalidation', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(page));
  render(<GrantOverview/>);
  const trigger = await screen.findByRole('button', { name: '接入配置 Reader' });
  trigger.focus(); fireEvent.click(trigger);
  expect(screen.getByRole('dialog', { name: 'Agent 接入配置' })).toBeTruthy();
  expect(screen.getByLabelText('占位接入配置').textContent).toContain('Bearer <AGENT_TOKEN>');
  fireEvent.click(screen.getByRole('button', { name: '关闭' }));
  await waitFor(() => expect(document.activeElement).toBe(trigger));
  expect(fetch).toHaveBeenCalledTimes(1);
  fireEvent.click(trigger); fireEvent(window, new Event('drivebridge-context'));
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
  expect(screen.queryByLabelText('占位接入配置')).toBeNull();
  expect(fetch).toHaveBeenCalledTimes(1);
});
it('uses opaque pagination and resets stale cursor rather than joining generations', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(json({ data: { ...page.data, next_cursor: 'cursor+fixture' } }))
    .mockResolvedValueOnce(json({ error: { code: 'cursor_expired' } }, 410))
    .mockResolvedValueOnce(json(page));
  render(<GrantOverview/>); await screen.findByText('Reader');
  fireEvent.click(screen.getByRole('button', { name: '加载更多授权' }));
  await screen.findByText('分页已过期，请刷新授权后从第一页重新读取。');
  expect(screen.queryByText('Reader')).toBeNull();
  expect(fetch.mock.calls[1][0]).toBe('/api/v1/agent-grants?limit=50&cursor=cursor%2Bfixture');
  fireEvent.click(screen.getByRole('button', { name: '刷新授权' }));
  await screen.findByText('Reader'); expect(fetch.mock.calls[2][0]).toBe('/api/v1/agent-grants?limit=50');
});
it('clears grants on permission loss and discards outstanding check responses', async () => {
  let finish!: (value: Response) => void;
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(page)).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  render(<GrantOverview/>); await screen.findByText('Reader');
  fireEvent.click(screen.getByRole('button', { name: '检查 Reader' }));
  fireEvent(window, new Event('drivebridge-context'));
  expect((await screen.findByRole('alert')).textContent).toContain('访问权限已失效');
  finish(json({ data: observed }));
  await waitFor(() => expect(screen.queryByText('授权状态：本地检查通过')).toBeNull());
  expect(screen.queryByText('Reader')).toBeNull();
});
it('409 preserves a clear refresh action and never automatically reruns a check', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json({ error: { code: 'revision_conflict' } }, 409));
  render(<GrantOverview/>); await screen.findByText('Reader');
  fireEvent.click(screen.getByRole('button', { name: '检查 Reader' }));
  await screen.findByText('授权版本已变化，请刷新后重新检查；没有执行任何任务。');
  expect(fetch).toHaveBeenCalledTimes(2); expect(screen.getByText('Reader')).toBeTruthy();
});
it('distinguishes unsupported service, empty scope and forbidden access', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({},404));
  const view=render(<GrantOverview/>);await screen.findByText('当前服务尚未提供新授权列表，请切换到“旧版机器人授权”。');view.unmount();
  fetch.mockResolvedValueOnce(json({ data: { ...page.data, items: [] } }));
  const empty=render(<GrantOverview/>);await screen.findByText(/当前范围没有新协议授权/);empty.unmount();
  fetch.mockResolvedValueOnce(json({},403));render(<GrantOverview/>);await screen.findByText(/访问权限已失效/);
});
