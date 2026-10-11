import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { AutomationPage } from './AutomationPage';
import { projectRules } from './client';
import type { Space } from '../../api';
const spaces = [{ id: 'space', name: 'Personal' }] as Space[];
const rule = { rule_id: 'rule', space_id: 'space', action: 'add_label', action_detail: 'topic:research', prefix: 'notes', revision: 2, enabled: true, paused: false, budget_items: 10, updated_at: '2026-10-07T00:00:00Z' };
const json = (data: unknown, status = 200) => new Response(JSON.stringify({ data }), { status });
afterEach(() => { cleanup(); vi.restoreAllMocks(); });

it('reads real rule projections without running, configuring or polling them', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json([rule]));
  const navigate = vi.fn();
  render(<AutomationPage scope="alice" spaces={spaces} onNavigate={navigate}/>);
  await screen.findByText('添加标签 · topic:research');
  expect(fetch).toHaveBeenCalledTimes(1);
  expect(fetch.mock.calls[0][0]).toBe('/api/v1/spaces/space/automation-rules');
  expect(fetch.mock.calls[0][1]?.method).toBe('GET');
  expect(screen.getByText('规则已启用')).toBeTruthy();
  expect(screen.getByText(/不代表有后台执行器/)).toBeTruthy();
  fireEvent.click(screen.getByRole('button', { name: '查看任务与原意图' }));
  expect(navigate).toHaveBeenCalledWith('transfers');
  expect(fetch).toHaveBeenCalledTimes(1);
});
it.each([true, false])('only opens management after explicit capability read (%s)', async available => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json([rule])).mockResolvedValueOnce(json({ available }));
  render(<AutomationPage scope="alice" spaces={spaces} onNavigate={vi.fn()}/>);
  await screen.findByText('添加标签 · topic:research');
  expect(screen.queryByRole('button', { name: '新建规则/恢复' })).toBeNull();
  fireEvent.click(screen.getByRole('button', { name: '配置与运行' }));
  if (available) await screen.findByRole('button', { name: '新建规则/恢复' });
  else await screen.findByText(/服务端尚未开放人工管理/);
  expect(fetch).toHaveBeenCalledTimes(2); expect(fetch.mock.calls[1][0]).toBe('/api/v1/spaces/space/automation-management');
  expect(fetch.mock.calls.every(([, options]) => options?.method === 'GET')).toBe(true);
});
it('clears management buttons and rules after a denied capability read', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json([rule])).mockResolvedValueOnce(json({}, 403));
  render(<AutomationPage scope="alice" spaces={spaces} onNavigate={vi.fn()}/>);
  await screen.findByText('添加标签 · topic:research');
  fireEvent.click(screen.getByRole('button', { name: '配置与运行' }));
  await screen.findByText(/权限或范围已变化/);
  expect(screen.queryByText('添加标签 · topic:research')).toBeNull();
  expect(screen.queryByRole('button', { name: '新建规则/恢复' })).toBeNull();
});
it.each([null, []])('supports the owner empty collection response %j', async value => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(value));
  render(<AutomationPage scope="alice" spaces={spaces} onNavigate={vi.fn()}/>);
  await screen.findByText('当前空间没有已保存的整理规则。');
});
it.each([401, 403, 404, 500])('distinguishes read failure %s from an empty rule list', async status => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json({}, status));
  render(<AutomationPage scope="alice" spaces={spaces} onNavigate={vi.fn()}/>);
  await screen.findByText(status === 401 || status === 403 ? /权限或范围已变化/ : status === 404 ? /未提供整理规则读取/ : /规则读取失败/);
  expect(screen.queryByText('当前空间没有已保存的整理规则。')).toBeNull();
  expect(fetch).toHaveBeenCalledTimes(1);
});
it('clears rules on offline and never retries just because connectivity returns', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json([rule]));
  render(<AutomationPage scope="alice" spaces={spaces} onNavigate={vi.fn()}/>);
  await screen.findByText('添加标签 · topic:research');
  fireEvent(window, new Event('offline'));
  expect(screen.queryByText('添加标签 · topic:research')).toBeNull();
  fireEvent(window, new Event('online'));
  expect(fetch).toHaveBeenCalledTimes(1);
});
it('discards a previous scope response even if the transport ignores abort', async () => {
  let resolve!: (response: Response) => void;
  vi.spyOn(globalThis, 'fetch').mockImplementationOnce(() => new Promise(done => { resolve = done; })).mockResolvedValueOnce(json([]));
  const view = render(<AutomationPage scope="alice" spaces={spaces} onNavigate={vi.fn()}/>);
  view.rerender(<AutomationPage scope="bob" spaces={spaces} onNavigate={vi.fn()}/>);
  await screen.findByText('当前空间没有已保存的整理规则。');
  resolve(json([rule]));
  await waitFor(() => expect(screen.queryByText('添加标签 · topic:research')).toBeNull());
});
it('does not fetch with no space or offline at entry', () => {
  const fetch = vi.spyOn(globalThis, 'fetch');
  const view = render(<AutomationPage scope="alice" spaces={[]} onNavigate={vi.fn()}/>);
  expect(fetch).not.toHaveBeenCalled(); view.unmount();
  vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false);
  render(<AutomationPage scope="alice" spaces={spaces} onNavigate={vi.fn()}/>);
  expect(fetch).not.toHaveBeenCalled();
});
it.each([
  undefined, {}, [rule, rule], [{ ...rule, space_id: 'other' }],
  [{ ...rule, revision: -1 }], [{ ...rule, budget_items: -1 }],
  [{ ...rule, enabled: 'yes' }], [{ ...rule, updated_at: 'not-a-date' }],
  Array.from({ length: 201 }, (_, index) => ({ ...rule, rule_id: String(index) })),
])('rejects invalid and out-of-scope rule responses', value => {
  expect(() => projectRules(value, 'space')).toThrow('invalid_contract');
});
it('does not expose owner, provider payloads or raw pause error strings', () => {
  const rows = projectRules([{ ...rule, owner: 'private-owner', pause_reason: 'private-token', provider_payload: 'private-body' }], 'space');
  expect(JSON.stringify(rows)).not.toContain('private');
});
