import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { CreateCollectionDialog } from './CreateCollectionDialog';
import { confirmRouteLeave } from '../../app/route-leave';
import { claimCreation, clearCreation, collectionSlot, creationDigest, readCreation, type Creation } from './collection-recovery';
const input: Creation = { space_id: 'space', name: 'Private collection', kind: 'manual', scope: 'private' };
const slot = collectionSlot('instance:alice:space'), locks = Object.getOwnPropertyDescriptor(navigator, 'locks');
const json = (data: unknown, status = 200) => new Response(JSON.stringify({ data }), { status });
const receipt = async () => ({ collection_id: 'a'.repeat(48), space_id: 'space', request_digest: await creationDigest(input), result_revision: 1, committed_at: '2026-01-01T00:00:00Z' });
beforeEach(() => Object.defineProperty(navigator, 'locks', { configurable: true, value: { request: vi.fn(async (_name: string, options: LockOptions | (() => unknown), callback?: () => unknown) => typeof options === 'function' ? options() : callback!()) } }));
afterEach(() => { cleanup(); localStorage.clear(); vi.restoreAllMocks(); if (locks) Object.defineProperty(navigator, 'locks', locks); else Reflect.deleteProperty(navigator, 'locks'); });
const mount = (close = vi.fn(), denied = vi.fn()) => render(<CreateCollectionDialog scope="instance:alice:space" space={{ id: 'space', name: 'Space' }} close={close} denied={denied}/>);
async function prepare() {
  fireEvent.change(await screen.findByRole('textbox', { name: '集合名称' }), { target: { value: input.name } });
  fireEvent.click(screen.getByRole('button', { name: '准备创建集合' })); await screen.findByRole('button', { name: '确认创建集合' });
}
it.each(['manual', 'rule'] as const)('protects unsent %s creation across route changes, but not unknown recovery', async kind => {
  const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
  const fetch = vi.spyOn(globalThis, 'fetch').mockRejectedValue(new TypeError('network'));
  const view = render(<CreateCollectionDialog kind={kind} scope="instance:alice:space" space={{ id: 'space', name: 'Space' }} close={vi.fn()} denied={vi.fn()}/>);
  expect(confirmRouteLeave()).toBe(true);
  if (kind === 'rule') {
    fireEvent.click(await screen.findByRole('button', { name: '添加规则条件' }));
    expect(confirmRouteLeave()).toBe(false);
    fireEvent.change(screen.getByRole('textbox', { name: '条件 1 值' }), { target: { value: 'reports/' } });
  }
  await prepare();
  expect(confirmRouteLeave()).toBe(false);
  expect(fetch).not.toHaveBeenCalled();
  expect(localStorage.length).toBe(0);
  fireEvent.click(screen.getByRole('button', { name: '确认创建集合' }));
  await screen.findByRole('button', { name: '查询原集合回执' });
  confirm.mockClear(); expect(confirmRouteLeave()).toBe(true); expect(confirm).not.toHaveBeenCalled();
  expect(fetch).toHaveBeenCalledTimes(1);
  view.unmount(); expect(confirmRouteLeave()).toBe(true);
});

it.each(['offline', 'drivebridge-context'])('clears an unsent creation guard after forced %s', async event => {
  vi.spyOn(window, 'confirm').mockReturnValue(false); mount(); await prepare();
  expect(confirmRouteLeave()).toBe(false);
  fireEvent(window, new Event(event)); await screen.findByText(/表单已清除/);
  expect(confirmRouteLeave()).toBe(true);
});

it('confirms before writing, ignores current view, verifies original receipt and CAS clears', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({ revision: 9 })).mockResolvedValueOnce(json(await receipt()));
  const close = vi.fn(); mount(close); await prepare(); expect(fetch).not.toHaveBeenCalled(); expect(localStorage.length).toBe(0);
  fireEvent.click(screen.getByRole('button', { name: '确认创建集合' })); await screen.findByText('已确认原集合创建');
  expect(JSON.parse(fetch.mock.calls[0][1]?.body as string)).toEqual(input);
  expect(fetch.mock.calls.map(([path]) => path)).toEqual(['/api/v1/file-collections', '/api/v1/file-collections/lookup']);
  const stored = localStorage.getItem(slot)!; expect(JSON.parse(stored)).toEqual({ key: expect.stringMatching(/^collection-[a-f0-9]{48}$/), digest: await creationDigest(input) }); expect(stored).not.toContain(input.name);
  fireEvent.click(screen.getByRole('button', { name: '确认回执并关闭' })); await waitFor(() => expect(close).toHaveBeenCalledOnce()); expect(readCreation(slot)).toBeNull();
});
it('lost response then remount performs only explicit lookup of original key', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new TypeError('network')).mockResolvedValueOnce(json(await receipt()));
  const view = mount(); await prepare(); fireEvent.click(screen.getByRole('button', { name: '确认创建集合' })); await screen.findByRole('button', { name: '查询原集合回执' });
  const original = readCreation(slot)!; view.unmount(); mount(); await screen.findByRole('button', { name: '查询原集合回执' }); expect(fetch).toHaveBeenCalledTimes(1);
  fireEvent.click(screen.getByRole('button', { name: '查询原集合回执' })); await screen.findByText('已确认原集合创建'); expect(JSON.parse(fetch.mock.calls[1][1]?.body as string)).toEqual({ idempotency_key: original.key });
});
it('404 needs explicit same-key retry and retains identical in-memory body', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new TypeError('network')).mockResolvedValueOnce(json({}, 404)).mockResolvedValueOnce(json({})).mockResolvedValueOnce(json(await receipt()));
  mount(); await prepare(); fireEvent.click(screen.getByRole('button', { name: '确认创建集合' })); fireEvent.click(await screen.findByRole('button', { name: '查询原集合回执' }));
  fireEvent.click(await screen.findByRole('button', { name: '确认按原键重试创建' })); await screen.findByText('已确认原集合创建');
  expect(fetch.mock.calls[2][1]?.body).toBe(fetch.mock.calls[0][1]?.body); expect(fetch.mock.calls[2][1]?.headers).toEqual(fetch.mock.calls[0][1]?.headers);
});
it.each([{ result_revision: 2 }, { request_digest: 'f'.repeat(64) }, { space_id: 'other' }, { members_total: 0 }])('rejects wrong receipt %j', async change => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({})).mockResolvedValueOnce(json({ ...await receipt(), ...change })); mount(); await prepare(); fireEvent.click(screen.getByRole('button', { name: '确认创建集合' })); await screen.findByRole('button', { name: '查询原集合回执' }); expect(readCreation(slot)).not.toBeNull(); expect(screen.queryByText('已确认原集合创建')).toBeNull();
});
it.each(['offline', 'drivebridge-context', 'storage'])('discards late creation after %s', async event => {
  let finish!: (response: Response) => void;
  const fetch = vi.spyOn(globalThis, 'fetch').mockImplementation(() => new Promise(resolve => { finish = resolve; })); mount(); await prepare(); fireEvent.click(screen.getByRole('button', { name: '确认创建集合' })); await waitFor(() => expect(finish).toBeDefined());
  fireEvent(window, event === 'storage' ? new StorageEvent('storage', { key: slot }) : new Event(event)); finish(json({})); await screen.findByText(/表单已清除/); expect(fetch).toHaveBeenCalledTimes(1); expect(readCreation(slot)).not.toBeNull();
});
it('403 clears parent and recovery failure sends no request', async () => {
  const denied = vi.fn(); const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json({}, 403)); const view = mount(vi.fn(), denied); await prepare(); fireEvent.click(screen.getByRole('button', { name: '确认创建集合' })); await waitFor(() => expect(denied).toHaveBeenCalledOnce()); view.unmount(); localStorage.clear();
  Object.defineProperty(navigator, 'locks', { configurable: true, value: undefined }); mount(); await prepare(); fireEvent.click(screen.getByRole('button', { name: '确认创建集合' })); await screen.findByText(/跨标签锁不可用/); expect(fetch).toHaveBeenCalledTimes(1);
});
it('preserves other claims and rejects foreign/extra recovery values', async () => {
  const first = await claimCreation(slot, 'a'.repeat(64), new AbortController().signal), second = await claimCreation(slot, 'b'.repeat(64), new AbortController().signal); expect(second.claimed).toBe(false); expect(second.record).toEqual(first.record);
  localStorage.setItem(slot, JSON.stringify({ key: 'collection-' + 'c'.repeat(48), digest: 'c'.repeat(64) })); await expect(clearCreation(slot, first.record)).rejects.toThrow();
  localStorage.setItem(slot, JSON.stringify({ ...first.record, name: 'must not persist' })); expect(() => readCreation(slot)).toThrow();
});
it('rejects names over owner UTF-8 byte budget without networking', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch'); mount(); fireEvent.change(await screen.findByRole('textbox', { name: '集合名称' }), { target: { value: '集'.repeat(67) } }); fireEvent.click(screen.getByRole('button', { name: '准备创建集合' })); await screen.findByText(/不超过 200 字节/); expect(fetch).not.toHaveBeenCalled();
});
