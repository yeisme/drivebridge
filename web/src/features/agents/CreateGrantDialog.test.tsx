import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import fixtures from '../../../../contracts/personal-workspace/fixtures/responses.json';
import { CreateGrantDialog } from './CreateGrantDialog';
import { claimGrantRecovery, clearGrantRecovery, grantRecoverySlot } from './grant-recovery';
import type { Space } from '../../api';

const space: Space = { id: 'space-fixture', name: 'Fixture space', connection_id: 'connection-fixture', used_bytes: 0, reserved_bytes: 0, max_bytes: 100 };
const grant = fixtures.GrantView;
const slot = grantRecoverySlot('instance:alice');
const secret = 'fixture-secret-never-persist';
const issued = { data: { grant, credential: { state: 'issued', secret } } };
const replay = { data: { grant, credential: { state: 'not_replayable' } } };
const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status });
const locks = Object.getOwnPropertyDescriptor(navigator, 'locks');
beforeEach(() => {
  Object.defineProperty(navigator, 'locks', { configurable: true, value: {
    request: vi.fn(async (_name: string, options: LockOptions | (() => unknown), callback?: () => unknown) => typeof options === 'function' ? options() : callback!()),
  } });
});
afterEach(() => { cleanup(); localStorage.clear(); vi.restoreAllMocks(); if (locks) Object.defineProperty(navigator, 'locks', locks); else Reflect.deleteProperty(navigator, 'locks'); });
const mount = (close = vi.fn()) => render(<CreateGrantDialog spaces={[space]} scope="instance:alice" close={close}/>);
async function submit() {
  fireEvent.change(await screen.findByRole('textbox', { name: 'Agent 名称' }), { target: { value: 'Reader' } });
  fireEvent.click(screen.getByRole('button', { name: '确认创建授权' }));
}
it('creates a read-only never-expiring grant, persists only key and shows secret once', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(issued, 201));
  const close = vi.fn(); mount(close); await submit();
  await screen.findByText('已确认授权：Reader');
  const [url, request] = fetch.mock.calls[0];
  expect(url).toBe('/api/v1/agent-grants');
  expect(JSON.parse(request?.body as string)).toEqual({ name: 'Reader', protocol: 'mcp', space_id: space.id, connection_id: space.connection_id, prefix: '', actions: ['files.read'], expiry_mode: 'never' });
  expect(localStorage.length).toBe(1);
  expect(localStorage.getItem(slot)).toMatch(/^grant-[a-f0-9]{48}$/);
  expect(request?.headers).toMatchObject({ 'Idempotency-Key': localStorage.getItem(slot) });
  expect((screen.getByLabelText('新协议 Agent 凭据') as HTMLInputElement).value).toBe(secret);
  fireEvent.click(screen.getByRole('button', { name: '已安全保存凭据' }));
  await waitFor(() => expect(close).toHaveBeenCalledTimes(1)); expect(localStorage.getItem(slot)).toBeNull();
});
it('looks up the original key after a lost response and across remount, never recreates', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new TypeError('network failure')).mockResolvedValueOnce(json(replay));
  const view = mount(); await submit(); await screen.findByRole('button', { name: '查询原创建结果' });
  const original = localStorage.getItem(slot); view.unmount(); mount();
  fireEvent.click(await screen.findByRole('button', { name: '查询原创建结果' }));
  await screen.findByText(/原授权已存在，但凭据不能再次取回/);
  expect(fetch.mock.calls.map(([path]) => path)).toEqual(['/api/v1/agent-grants', '/api/v1/agent-grants/lookup']);
  expect(JSON.parse(fetch.mock.calls[1][1]?.body as string)).toEqual({ idempotency_key: original });
  expect(screen.queryByLabelText('新协议 Agent 凭据')).toBeNull();
});
it('404 is not permission to create another identity; explicit retry keeps exact key and payload', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new TypeError('offline')).mockResolvedValueOnce(json({},404)).mockResolvedValueOnce(json(replay));
  mount(); await submit(); fireEvent.click(await screen.findByRole('button', { name: '查询原创建结果' }));
  await screen.findByText(/暂未查到原结果/);
  expect(fetch).toHaveBeenCalledTimes(2);
  fireEvent.click(screen.getByRole('button', { name: '按原键重试原请求' }));
  await screen.findByText(/原授权已存在/);
  expect(fetch.mock.calls[2][1]?.body).toBe(fetch.mock.calls[0][1]?.body);
  expect(fetch.mock.calls[2][1]?.headers).toEqual(fetch.mock.calls[0][1]?.headers);
});
it('clears a delivered secret on scope change and discards late delivery', async () => {
  let resolve!: (value: Response) => void;
  vi.spyOn(globalThis, 'fetch').mockImplementationOnce(() => new Promise(done => { resolve = done; }));
  mount(); await submit(); await waitFor(() => expect(resolve).toBeDefined());
  fireEvent(window, new Event('drivebridge-context')); resolve(json(issued));
  await screen.findByText(/敏感结果已清除/);
  expect(screen.queryByLabelText('新协议 Agent 凭据')).toBeNull(); expect(localStorage.getItem(slot)).toBeTruthy();
});
it('clears an already displayed secret on another tab recovery change', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(issued)); mount(); await submit(); await screen.findByLabelText('新协议 Agent 凭据');
  fireEvent(window, new StorageEvent('storage', { key: slot, newValue: null }));
  expect(screen.queryByLabelText('新协议 Agent 凭据')).toBeNull();
});
it('fails closed without locks or with a corrupt recovery slot', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch');
  Object.defineProperty(navigator, 'locks', { configurable: true, value: undefined });
  const view = mount(); await submit(); await screen.findByText(/恢复存储或跨标签页锁不可用/); expect(fetch).not.toHaveBeenCalled();
  view.unmount(); localStorage.setItem(slot,'corrupt'); mount(); await screen.findByText(/恢复存储或跨标签页锁不可用/); expect(fetch).not.toHaveBeenCalled();
});
it.each([[3600,'1 小时'],[86400,'1 天'],[604800,'7 天'],[2592000,'30 天'],[7776000,'90 天'],[31536000,'365 天']])('sends explicit duration %s and narrow extra actions', async (seconds, label) => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(issued)); mount();
  fireEvent.click(await screen.findByRole('combobox', { name: '授权期限' }));
  const option = await screen.findByRole('option', { name: String(label) });
  fireEvent.pointerDown(option, { pointerType: 'mouse' });
  fireEvent.click(option);
  await waitFor(() => expect(screen.getByRole('combobox', { name: '授权期限' }).getAttribute('data-value')).toBe(String(seconds)));
  fireEvent.click(screen.getByRole('checkbox', { name: '建议标签（不直接修改）' }));
  fireEvent.click(screen.getByRole('checkbox', { name: '提交收件（等待人工验收）' }));
  await submit(); await screen.findByText('已确认授权：Reader');
  expect(JSON.parse(fetch.mock.calls[0][1]?.body as string)).toMatchObject({ expiry_mode: 'duration', expires_in_seconds: seconds, actions: ['files.read','labels.suggest','inbox.submit'] });
});
it('preserves form intent after a definitive validation rejection', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(json({ error: { code: 'invalid_request' } },422)); mount(); await submit();
  await screen.findByText(/服务端拒绝了请求格式/);
  expect((screen.getByRole('textbox', { name: 'Agent 名称' }) as HTMLInputElement).value).toBe('Reader');
  expect(localStorage.getItem(slot)).toBeNull();
});
it('claims one opaque key and never clears a different tab identity', async () => {
  const signal = new AbortController().signal;
  const first = await claimGrantRecovery(slot, signal), other = await claimGrantRecovery(slot, signal);
  expect(first.claimed).toBe(true); expect(other).toEqual({ key: first.key, claimed: false });
  const newer = 'grant-' + 'b'.repeat(48); localStorage.setItem(slot,newer); await clearGrantRecovery(slot, first.key);
  expect(localStorage.getItem(slot)).toBe(newer);
});
