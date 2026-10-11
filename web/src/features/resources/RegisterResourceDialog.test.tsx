import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { RegisterResourceDialog } from './RegisterResourceDialog';
import { confirmRouteLeave } from '../../app/route-leave';
import { claimResource, clearResource, readResourceRecovery, resourceSlot } from './resource-recovery';
import type { Space } from '../../api';
const file = 'a'.repeat(48), version = 'b'.repeat(48), ref = `drivebridge://instance/file/${file}/version/${version}`;
const space: Space = { id: 'space', name: 'Space', connection_id: 'platform', used_bytes: 0, reserved_bytes: 0, max_bytes: 100 };
const input = { space_id: space.id, kind: 'skill', display_name: 'Private name', file_ref: ref };
const resource = { descriptor_id: 'descriptor', ...input, descriptor_revision: 1, check_state: 'unknown', created_at: '2026-01-01T00:00:00Z' };
const receipt = { resource, result_revision: 1, committed_at: '2026-01-01T00:00:00Z' };
const json = (data: unknown, status = 200) => new Response(JSON.stringify({ data }), { status });
const metadata = () => json({ ref, file: { id: file, version_id: version, space_id: space.id } });
const slot = resourceSlot('instance:alice'), locks = Object.getOwnPropertyDescriptor(navigator, 'locks');
beforeEach(() => Object.defineProperty(navigator, 'locks', { configurable: true, value: { request: vi.fn(async (_name: string, options: LockOptions | (() => unknown), callback?: () => unknown) => typeof options === 'function' ? options() : callback!()) } }));
afterEach(() => { cleanup(); localStorage.clear(); vi.restoreAllMocks(); if (locks) Object.defineProperty(navigator, 'locks', locks); else Reflect.deleteProperty(navigator, 'locks'); });
const mount = (close = vi.fn()) => render(<RegisterResourceDialog scope="instance:alice" spaces={[space]} close={close}/>);
async function preview() {
  fireEvent.change(await screen.findByRole('textbox', { name: '资源名称' }), { target: { value: 'Private name' } });
  fireEvent.change(screen.getByRole('textbox', { name: '固定目标' }), { target: { value: ref } });
  fireEvent.click(screen.getByRole('button', { name: '核对固定目标' }));
  await screen.findByRole('button', { name: '确认登记资源' });
}
const unloadBlocked = () => { const event = new Event('beforeunload', { cancelable: true }); window.dispatchEvent(event); return event.defaultPrevented; };
it('protects unsent registration on close, route and reload, but releases unknown recovery', async () => {
  const close = vi.fn(), confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(metadata()).mockRejectedValueOnce(new TypeError('network'));
  const view = mount(close); expect(confirmRouteLeave()).toBe(true); expect(unloadBlocked()).toBe(false);
  await preview(); expect(confirmRouteLeave()).toBe(false); expect(unloadBlocked()).toBe(true);
  fireEvent.click(screen.getByRole('button', { name: '关闭' }));
  await screen.findByText('关闭将丢弃未提交的资源登记草稿。'); expect(close).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: '继续编辑登记' }));
  expect(fetch).toHaveBeenCalledTimes(1); expect(localStorage.length).toBe(0);
  fireEvent.click(screen.getByRole('button', { name: '确认登记资源' }));
  await screen.findByRole('button', { name: '查询原登记回执' });
  confirm.mockClear(); expect(confirmRouteLeave()).toBe(true); expect(confirm).not.toHaveBeenCalled(); expect(unloadBlocked()).toBe(false);
  fireEvent.click(screen.getByRole('button', { name: '关闭' })); expect(close).toHaveBeenCalledOnce();
  view.unmount(); expect(confirmRouteLeave()).toBe(true); expect(unloadBlocked()).toBe(false);
});
it.each(['offline', 'drivebridge-context', 'storage'])('clears unsent resource fields and protection after %s', async event => {
  vi.spyOn(window, 'confirm').mockReturnValue(false); mount();
  fireEvent.click(await screen.findByRole('combobox', { name: '登记类型' }));
  const option = await screen.findByRole('option', { name: '资料包' }); fireEvent.pointerDown(option, { pointerType: 'mouse' }); fireEvent.click(option);
  expect(confirmRouteLeave()).toBe(false);
  fireEvent(window, event === 'storage' ? new StorageEvent('storage', { key: slot }) : new Event(event));
  await screen.findByText(/内容已清除/); expect(confirmRouteLeave()).toBe(true); expect(unloadBlocked()).toBe(false);
});
it('explicitly discards a resource draft without a request', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch'), close = vi.fn(); mount(close);
  fireEvent.change(await screen.findByRole('textbox', { name: '资源名称' }), { target: { value: 'Unsent' } });
  fireEvent.click(screen.getByRole('button', { name: '关闭' }));
  fireEvent.click(await screen.findByRole('button', { name: '丢弃登记草稿并关闭' }));
  expect(close).toHaveBeenCalledOnce(); expect(fetch).not.toHaveBeenCalled();
});
it('previews without body then registers and verifies receipt, storing only key and digest', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(metadata()).mockResolvedValueOnce(json(resource, 201)).mockResolvedValueOnce(json(receipt));
  const close = vi.fn(); mount(close); await preview(); expect(fetch).toHaveBeenCalledTimes(1); expect(localStorage.length).toBe(0);
  fireEvent.click(screen.getByRole('button', { name: '确认登记资源' })); await screen.findByText('已确认原登记：Private name');
  const stored = localStorage.getItem(slot)!;
  expect(JSON.parse(stored)).toEqual({ key: expect.stringMatching(/^resource-[a-f0-9]{48}$/), digest: expect.stringMatching(/^[a-f0-9]{64}$/) });
  expect(stored).not.toContain('Private name'); expect(stored).not.toContain(ref);
  expect(JSON.parse(fetch.mock.calls[1][1]?.body as string)).toEqual(input);
  expect(fetch.mock.calls.map(([path]) => String(path))).toEqual(['/api/v1/references?' + new URLSearchParams({ ref }), '/api/v1/resource-descriptors', '/api/v1/resource-descriptors/lookup']);
  fireEvent.click(screen.getByRole('button', { name: '确认回执并关闭' })); await waitFor(() => expect(close).toHaveBeenCalledOnce()); expect(localStorage.getItem(slot)).toBeNull();
});
it('lost response followed by remount only looks up the original key', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(metadata()).mockRejectedValueOnce(new TypeError('network')).mockResolvedValueOnce(json(receipt));
  const view = mount(); await preview(); fireEvent.click(screen.getByRole('button', { name: '确认登记资源' })); await screen.findByRole('button', { name: '查询原登记回执' });
  const original = readResourceRecovery(slot)!; view.unmount(); mount();
  fireEvent.click(await screen.findByRole('button', { name: '查询原登记回执' })); await screen.findByText('已确认原登记：Private name');
  expect(JSON.parse(fetch.mock.calls[2][1]?.body as string)).toEqual({ idempotency_key: original.key });
  expect(fetch.mock.calls.filter(([path]) => path === '/api/v1/resource-descriptors')).toHaveLength(1);
});
it('404 retains original identity and explicit retry uses identical body and key', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(metadata()).mockRejectedValueOnce(new TypeError('network')).mockResolvedValueOnce(json({}, 404)).mockResolvedValueOnce(json(resource)).mockResolvedValueOnce(json(receipt));
  mount(); await preview(); fireEvent.click(screen.getByRole('button', { name: '确认登记资源' }));
  fireEvent.click(await screen.findByRole('button', { name: '查询原登记回执' }));
  fireEvent.click(await screen.findByRole('button', { name: '确认按原键重试登记' })); await screen.findByText('已确认原登记：Private name');
  expect(fetch.mock.calls[3][1]?.body).toBe(fetch.mock.calls[1][1]?.body); expect(fetch.mock.calls[3][1]?.headers).toEqual(fetch.mock.calls[1][1]?.headers);
});
it('rejects mismatched receipt instead of clearing recovery', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(metadata()).mockResolvedValueOnce(json(resource)).mockResolvedValueOnce(json({ ...receipt, resource: { ...resource, display_name: 'Other' } }));
  mount(); await preview(); fireEvent.click(screen.getByRole('button', { name: '确认登记资源' })); await screen.findByRole('button', { name: '查询原登记回执' }); expect(localStorage.getItem(slot)).toBeTruthy(); expect(screen.queryByText(/已确认原登记/)).toBeNull();
});
it('blocks missing locks and corrupt recovery before creation', async () => {
  Object.defineProperty(navigator, 'locks', { configurable: true, value: undefined });
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(metadata()); const view = mount(); await preview(); fireEvent.click(screen.getByRole('button', { name: '确认登记资源' })); await screen.findByText(/跨标签锁不可用/); expect(fetch).toHaveBeenCalledTimes(1);
  view.unmount(); localStorage.setItem(slot, 'bad'); mount(); await screen.findByText(/跨标签锁不可用/);
});
it('other-tab slot changes discard a late response and preserve recovery', async () => {
  let finish!: (value: Response) => void;
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(metadata()).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  mount(); await preview(); fireEvent.click(screen.getByRole('button', { name: '确认登记资源' })); await waitFor(() => expect(finish).toBeDefined());
  fireEvent(window, new StorageEvent('storage', { key: slot })); finish(json(resource)); await screen.findByText(/内容已清除/); expect(screen.queryByText(/已确认原登记/)).toBeNull(); expect(localStorage.getItem(slot)).toBeTruthy();
});
it('previews a package snapshot without treating it as a single file', async () => {
  const snapshot = 'c'.repeat(48);
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({ snapshot_id: snapshot, collection_id: 'd'.repeat(48), entries_total: 0, entries_visible: 0, items: [] }));
  mount(); fireEvent.change(await screen.findByRole('textbox', { name: '资源名称' }), { target: { value: 'Pack' } });
  fireEvent.click(screen.getByRole('combobox', { name: '登记类型' }));
  const option = await screen.findByRole('option', { name: '资料包' }); fireEvent.pointerDown(option, { pointerType: 'mouse' }); fireEvent.click(option);
  fireEvent.change(screen.getByRole('textbox', { name: '固定目标' }), { target: { value: snapshot } });
  fireEvent.click(screen.getByRole('button', { name: '核对固定目标' })); await screen.findByRole('button', { name: '确认登记资源' });
  expect(fetch.mock.calls[0][0]).toBe('/api/v1/collection-snapshots/' + snapshot);
  expect(fetch).toHaveBeenCalledTimes(1); expect(localStorage.length).toBe(0);
  expect(screen.getByText(/快照预读不证明所属空间/)).toBeTruthy();
});
it('does not overwrite another claim or clear a newer slot', async () => {
  const signal = new AbortController().signal;
  const first = await claimResource(slot, 'a'.repeat(64), signal), second = await claimResource(slot, 'b'.repeat(64), signal);
  expect(second.claimed).toBe(false); expect(second.record).toEqual(first.record);
  const newer = { key: 'resource-' + 'c'.repeat(48), digest: 'c'.repeat(64) }; localStorage.setItem(slot, JSON.stringify(newer));
  await expect(clearResource(slot, first.record)).rejects.toThrow(); expect(readResourceRecovery(slot)).toEqual(newer);
});
