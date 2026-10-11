import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { AutomationActionDialog } from './AutomationActionDialog';
import { automationDigest, automationSlot, readAutomationRecovery, type AutomationChange } from './automation-recovery';
const id = 'a'.repeat(48), file = 'b'.repeat(48), version = 'c'.repeat(48), space = 'space';
const rule = { id, action: 'add_label', detail: 'rule:reviewed', prefix: 'auto', revision: 1, enabled: true, paused: false, budget: 10, updatedAt: '2026-01-01T00:00:00Z', policyID: '' };
const preview = { rule_id: id, space_id: space, revision: 1, checkpoint: 0, scanned: 1, items: [{ file_id: file, version_id: version, path: 'auto/input.txt', size_bytes: 10 }], coverage: 'complete', preview_digest: 'd'.repeat(64), executable: true, policy_max_bytes: 0, policy_expires_at: null };
const json = (data: unknown, status = 200, code = 'failure') => new Response(JSON.stringify(status < 400 ? { data } : { error: { code } }), { status });
const locks = Object.getOwnPropertyDescriptor(navigator, 'locks');
beforeEach(() => Object.defineProperty(navigator, 'locks', { configurable: true, value: { request: vi.fn(async (_name: string, options: LockOptions | (() => unknown), callback?: () => unknown) => typeof options === 'function' ? options() : callback!()) } }));
afterEach(() => { cleanup(); localStorage.clear(); vi.restoreAllMocks(); if (locks) Object.defineProperty(navigator, 'locks', locks); else Reflect.deleteProperty(navigator, 'locks'); });
const mount = (action: 'change' | 'execute', existing = false) => render(<AutomationActionDialog scope="scope" space={space} action={action} rule={existing ? rule : undefined} close={vi.fn()} denied={vi.fn()}/>);
it('creates a disabled rule only after explicit review, preserving a secret-free recovery identity', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockImplementation(async (_path, options) => {
    const input = JSON.parse(options?.body as string) as AutomationChange;
    return json({ rule_id: id, space_id: space, action: 'automation.change', request_digest: await automationDigest(space, 'change', input), result_revision: 1, committed_at: '2026-01-01T00:00:00Z' });
  });
  mount('change');
  await screen.findByRole('button', { name: '预览规则修改' });
  expect((screen.getByRole('checkbox', { name: '允许手动运行' }) as HTMLElement).getAttribute('aria-checked')).toBe('false');
  fireEvent.change(screen.getByLabelText('标签名称'), { target: { value: 'reviewed' } });
  fireEvent.change(screen.getByLabelText('规则目录'), { target: { value: 'auto' } });
  fireEvent.click(screen.getByRole('button', { name: '预览规则修改' }));
  expect(fetch).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: '确认保存规则' }));
  await screen.findByText('已确认原规则变更');
  expect(JSON.parse(fetch.mock.calls[0][1]?.body as string)).toMatchObject({ enabled: false, prefix: 'auto', action_detail: 'rule:reviewed', expected_revision: 0 });
  const stored = localStorage.getItem(automationSlot('scope', space, 'change'))!;
  expect(Object.keys(JSON.parse(stored)).sort()).toEqual(['digest', 'key']); expect(stored).not.toContain('reviewed');
});
it('preserves an existing custom label namespace when changing only enabled state', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockImplementation(async (_path, options) => {
    const input = JSON.parse(options?.body as string) as AutomationChange;
    return json({ rule_id: id, space_id: space, action: 'automation.change', request_digest: await automationDigest(space, 'change', input), result_revision: 2, committed_at: '2026-01-01T00:00:00Z' });
  });
  render(<AutomationActionDialog scope="scope" space={space} action="change" rule={{ ...rule, detail: 'topic:reviewed' }} close={vi.fn()} denied={vi.fn()}/>);
  fireEvent.click(await screen.findByRole('checkbox', { name: '允许手动运行' }));
  fireEvent.click(screen.getByRole('button', { name: '预览规则修改' }));
  fireEvent.click(await screen.findByRole('button', { name: '确认保存规则' }));
  await screen.findByText('已确认原规则变更');
  expect(JSON.parse(fetch.mock.calls[0][1]?.body as string)).toMatchObject({ action_detail: 'topic:reviewed', enabled: false, expected_revision: 1 });
});
it('executes only a validated fixed preview and looks up the original receipt after a lost response and remount', async () => {
  const digest = await automationDigest(id, 'execute', { expected_revision: 1, preview_digest: preview.preview_digest });
  const receipt = { rule_id: id, space_id: space, action: 'automation.execute', request_digest: digest, result_revision: 1, committed_at: '2026-01-01T00:00:00Z' };
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(preview)).mockRejectedValueOnce(new TypeError('network')).mockResolvedValueOnce(json(receipt));
  const view = mount('execute', true);
  fireEvent.click(await screen.findByRole('button', { name: '读取运行预览' }));
  await screen.findByText('auto/input.txt'); expect(localStorage.length).toBe(0);
  fireEvent.click(screen.getByRole('button', { name: '确认运行本次预览' }));
  await screen.findByRole('button', { name: '查询原自动化回执' });
  const original = readAutomationRecovery(automationSlot('scope', space, 'execute', id))!;
  view.unmount(); mount('execute', true);
  fireEvent.click(await screen.findByRole('button', { name: '查询原自动化回执' }));
  await screen.findByText('已确认原运行请求');
  expect(fetch.mock.calls[2][0]).toBe('/api/v1/automation-rules/' + id + '/execute/lookup');
  expect(JSON.parse(fetch.mock.calls[2][1]?.body as string)).toEqual({ idempotency_key: original.key });
  expect(fetch.mock.calls.filter(([url]) => String(url).endsWith('/execute'))).toHaveLength(1);
});
it('keeps a conflict separate from an unknown write and requires explicit restart without automatic execution', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(preview)).mockResolvedValueOnce(json({}, 409, 'plan_stale'));
  mount('execute', true); fireEvent.click(await screen.findByRole('button', { name: '读取运行预览' }));
  fireEvent.click(await screen.findByRole('button', { name: '确认运行本次预览' }));
  await screen.findByText(/服务端已拒绝此过期预览/);
  expect(fetch).toHaveBeenCalledTimes(2);
  expect(screen.queryByRole('button', { name: '确认运行本次预览' })).toBeNull();
});
it.each([{ request_digest: 'f'.repeat(64) }, { space_id: 'other' }, { rule_id: 'e'.repeat(48) }, { action: 'automation.change' }])('rejects a mismatched receipt %j', async mismatch => {
  const digest = await automationDigest(id, 'execute', { expected_revision: 1, preview_digest: preview.preview_digest });
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(preview)).mockResolvedValueOnce(json({ rule_id: id, space_id: space, action: 'automation.execute', request_digest: digest, result_revision: 1, committed_at: '2026-01-01T00:00:00Z', ...mismatch }));
  mount('execute', true); fireEvent.click(await screen.findByRole('button', { name: '读取运行预览' }));
  fireEvent.click(await screen.findByRole('button', { name: '确认运行本次预览' }));
  await screen.findByRole('button', { name: '查询原自动化回执' });
  expect(screen.queryByText('已确认原运行请求')).toBeNull();
  expect(readAutomationRecovery(automationSlot('scope', space, 'execute', id))).not.toBeNull();
});
it('missing Web Locks prevents a mutation after preview', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(preview));
  Object.defineProperty(navigator, 'locks', { configurable: true, value: undefined });
  mount('execute', true); fireEvent.click(await screen.findByRole('button', { name: '读取运行预览' }));
  fireEvent.click(await screen.findByRole('button', { name: '确认运行本次预览' }));
  await screen.findByText(/恢复存储或跨标签锁不可用/); expect(fetch).toHaveBeenCalledTimes(1);
});
it.each(['drivebridge-context', 'offline', 'storage'])('clears a late preview after %s', async event => {
  let finish!: (response: Response) => void;
  vi.spyOn(globalThis, 'fetch').mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  mount('execute', true); fireEvent.click(await screen.findByRole('button', { name: '读取运行预览' }));
  await waitFor(() => expect(finish).toBeDefined());
  fireEvent(window, event === 'storage' ? new StorageEvent('storage', { key: null }) : new Event(event));
  finish(json(preview)); await screen.findByText(/操作已停止，预览与结果已清除/);
  expect(screen.queryByText('auto/input.txt')).toBeNull();
});
it.each([{ ...preview, space_id: 'other' }, { ...preview, items: null }, { ...preview, preview_digest: 'bad' }])('rejects invalid preview %j', async value => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(value));
  mount('execute', true); fireEvent.click(await screen.findByRole('button', { name: '读取运行预览' }));
  await screen.findByText(/预览读取失败/);
  expect(screen.queryByRole('button', { name: '确认运行本次预览' })).toBeNull(); expect(fetch).toHaveBeenCalledTimes(1);
});
