import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { AppearanceProvider } from '../../appearance';
import { DesktopWorkspace } from './DesktopWorkspace';
import { isDesktopEntry, readDesktopCapabilities, type DesktopEntryBridge } from '../../app/desktop-entry';

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });
const result = { data: { schema_version: 'drivebridge.desktop.v1alpha1' as const, available: [] as string[] } };
function mount(bridge?: DesktopEntryBridge) { return render(<AppearanceProvider><DesktopWorkspace bridge={bridge}/></AppearanceProvider>); }

it('uses the real preload capability and never requests cloud session or fabricated sources', async () => {
  const fetch = vi.fn(); vi.stubGlobal('fetch', fetch);
  const capabilities = vi.fn().mockResolvedValue(result);
  mount({ capabilities });
  expect(await screen.findByText('桌面宿主已连接，尚未开放来源操作。')).toBeTruthy();
  expect(capabilities).toHaveBeenCalledWith({ input: {} }); expect(fetch).not.toHaveBeenCalled();
  expect(screen.getByText('尚未建立 Pinax 来源')).toBeTruthy();
  fireEvent.click(screen.getByRole('button', { name: '桌面能力说明' }));
  expect(screen.getByText('当前交付边界')).toBeTruthy();
  expect(screen.queryByRole('button', { name: '登录' })).toBeNull();
});
it('shows bounded safe failure and supports retry without displaying private exception messages', async () => {
  const capabilities = vi.fn().mockRejectedValueOnce(new Error('/private/secret')).mockResolvedValue(result);
  mount({ capabilities });
  expect(await screen.findByRole('alert')).toBeTruthy(); expect(screen.queryByText('/private/secret')).toBeNull();
  fireEvent.click(screen.getByRole('button', { name: '重新检查' }));
  expect(await screen.findByText('桌面宿主已连接，尚未开放来源操作。')).toBeTruthy();
  expect(capabilities).toHaveBeenCalledTimes(2);
});
it('discards late responses from replaced bridge identities', async () => {
  let resolve!: (value: typeof result) => void;
  const old = { capabilities: vi.fn(() => new Promise<typeof result>(done => { resolve = done; })) };
  const view = mount(old); await waitFor(() => expect(old.capabilities).toHaveBeenCalledTimes(1));
  view.rerender(<AppearanceProvider><DesktopWorkspace bridge={{ capabilities: vi.fn().mockResolvedValue(result) }}/></AppearanceProvider>);
  await screen.findByText('桌面宿主已连接，尚未开放来源操作。');
  resolve({ data: { ...result.data, available: ['chooseVault'] } });
  await waitFor(() => expect(screen.queryByText('可以选择已有 vault 进行只读预检。')).toBeNull());
});
it('uses native selection without a renderer path and shows cancellation and guard refusal', async () => {
  const chooseVault = vi.fn().mockResolvedValueOnce({ error: { code: 'native_selection_cancelled' } }).mockResolvedValueOnce({ data: { decision: 'blocked', reason: 'managed_scope_unverifiable' } });
  mount({ capabilities: vi.fn().mockResolvedValue({ data: { ...result.data, available: ['chooseVault'] } }), chooseVault });
  fireEvent.click(await screen.findByRole('button', { name: '选择本机 Pinax vault' }));
  expect(await screen.findByText('已取消选择，未连接来源。')).toBeTruthy();
  fireEvent.click(screen.getByRole('button', { name: '选择本机 Pinax vault' }));
  expect(await screen.findByText('受管范围保护尚未通过验证，来源未连接；正文和编辑均未开放。')).toBeTruthy();
  expect(chooseVault).toHaveBeenNthCalledWith(1, { input: {} });
  expect(chooseVault).toHaveBeenNthCalledWith(2, { input: {} });
  expect(screen.queryByRole('button', { name: '确认绑定' })).toBeNull();
});
it('fails closed when preload is missing', async () => {
  mount(); expect(await screen.findByRole('alert')).toBeTruthy();
});
it('requires confirmation and preserves the original key after a lost response', async () => {
  const preview = { preview_id: 'preview', preview_digest: 'sha256:' + 'a'.repeat(64), expected_guard_revision: 0, decision: 'ready' };
  const confirmBinding = vi.fn().mockRejectedValueOnce(new Error('response lost')).mockResolvedValue({ data: { binding: { source_id: 'source' } } });
  const chooseVault = vi.fn().mockResolvedValue({ data: preview });
  mount({ capabilities: vi.fn().mockResolvedValue({ data: { ...result.data, available: ['chooseVault', 'confirmBinding'] } }), chooseVault, confirmBinding });
  fireEvent.click(await screen.findByRole('button', { name: '选择本机 Pinax vault' }));
  const confirm = await screen.findByRole('button', { name: '确认绑定' });
  expect(confirmBinding).not.toHaveBeenCalled();
  fireEvent.click(confirm);
  await screen.findByText('绑定结果未知，请使用原请求继续确认。');
  expect((screen.getByRole('button', { name: '选择本机 Pinax vault' }) as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(confirm);
  await screen.findByText('来源已绑定，可以开始阅读。');
  expect(confirmBinding.mock.calls[0][0]).toEqual(confirmBinding.mock.calls[1][0]);
  expect(confirmBinding.mock.calls[0][0].input.confirmed).toBe(true);
});
it('selects desktop only on its packaged origin and validates response envelopes', () => {
  expect(isDesktopEntry({ protocol: 'drivebridge-app:', hostname: 'app' })).toBe(true);
  for (const location of [{ protocol: 'https:', hostname: 'app' }, { protocol: 'drivebridge-app:', hostname: 'evil' }]) expect(isDesktopEntry(location)).toBe(false);
  expect(readDesktopCapabilities(result)).toEqual([]);
  for (const value of [null, { error: { message: 'private' } }, { data: { ...result.data, token: 'secret' } }, { data: { ...result.data, available: ['chooseVault', 'chooseVault'] } }, { data: { ...result.data, schema_version: 'future' } }]) expect(() => readDesktopCapabilities(value)).toThrow();
});

it('clears a pending binding and fences its late response after native session invalidation', async () => {
  let finish!: (value: unknown) => void;
  const chooseVault = vi.fn(() => new Promise(resolve => { finish = resolve; }));
  mount({ capabilities: vi.fn().mockResolvedValue({ data: { ...result.data, available: ['chooseVault'] } }), chooseVault: chooseVault as never });
  fireEvent.click(await screen.findByRole('button', { name: '选择本机 Pinax vault' }));
  fireEvent(window, new Event('drivebridge:session-invalidated'));
  expect(await screen.findByText('本机会话已失效，正文与草稿已清除。请关闭并重新打开工作区。')).toBeTruthy();
  finish({ data: { decision: 'ready' } });
  await waitFor(() => expect(screen.queryByRole('button', { name: '确认绑定' })).toBeNull());
  expect(screen.queryByRole('button', { name: '选择本机 Pinax vault' })).toBeNull();
});
