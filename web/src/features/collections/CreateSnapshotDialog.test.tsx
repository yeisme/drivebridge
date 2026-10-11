import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { CreateSnapshotDialog } from './CreateSnapshotDialog';
import { claimSnapshot, clearSnapshot, readSnapshotRecovery, snapshotDigest, snapshotSlot } from './snapshot-recovery';
const id = 'c'.repeat(48), snapshot = 'd'.repeat(48), file = 'a'.repeat(48), version = 'b'.repeat(48);
const observation = { collection_id: id, revision: 2, kind: 'manual', coverage: 'complete', visible_count: 1, items: [{ file_id: file, version_id: version, path: 'private/reference.txt' }] };
const slot = snapshotSlot('scope', id), path = '/api/v1/file-collections/' + id;
const json = (data: unknown, status = 200) => new Response(JSON.stringify({ data }), { status });
const receipt = async () => ({ collection_id: id, snapshot_id: snapshot, request_digest: await snapshotDigest(id, 2), collection_revision: 2, committed_at: '2026-01-01T00:00:00Z' });
const locks = Object.getOwnPropertyDescriptor(navigator, 'locks');
beforeEach(() => Object.defineProperty(navigator, 'locks', { configurable: true, value: { request: vi.fn(async (_name: string, options: LockOptions | (() => unknown), callback?: () => unknown) => typeof options === 'function' ? options() : callback!()) } }));
afterEach(() => { cleanup(); localStorage.clear(); vi.restoreAllMocks(); if (locks) Object.defineProperty(navigator, 'locks', locks); else Reflect.deleteProperty(navigator, 'locks'); });
const mount = (close = vi.fn(), denied = vi.fn()) => render(<CreateSnapshotDialog scope="scope" id={id} close={close} denied={denied}/>);
async function preview() { fireEvent.click(await screen.findByRole('button', { name: '预览当前可见成员' })); await screen.findByRole('button', { name: '确认创建提交时快照' }); }
it('previews metadata then confirms, verifies original receipt and explicitly opens shared reader', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(observation)).mockResolvedValueOnce(json({})).mockResolvedValueOnce(json(await receipt())).mockResolvedValueOnce(json({ snapshot_id: snapshot, collection_id: id, collection_revision: 2, entries_total: 1, entries_visible: 1, created_at: '2026-01-01T00:00:00Z', items: observation.items }));
  const close = vi.fn(); mount(close); expect(fetch).not.toHaveBeenCalled(); await preview(); expect(fetch).toHaveBeenCalledTimes(1); expect(localStorage.length).toBe(0);
  fireEvent.click(screen.getByRole('button', { name: '确认创建提交时快照' })); await screen.findByText('已确认原快照创建'); expect(fetch).toHaveBeenCalledTimes(3);
  expect(JSON.parse(fetch.mock.calls[1][1]?.body as string)).toEqual({ expected_revision: 2 });
  const stored = localStorage.getItem(slot)!; expect(JSON.parse(stored)).toEqual({ key: expect.stringMatching(/^snapshot-[a-f0-9]{48}$/), digest: await snapshotDigest(id, 2) }); expect(stored).not.toContain('private'); expect(stored).not.toContain(version);
  fireEvent.click(screen.getByRole('button', { name: '查看原快照成员' })); await screen.findByRole('button', { name: '打开固定成员：private/reference.txt' });
  expect(fetch.mock.calls[3][0]).toBe('/api/v1/collection-snapshots/' + snapshot); expect(fetch.mock.calls.every(([url]) => !String(url).includes('/content'))).toBe(true);
  fireEvent.click(screen.getByRole('button', { name: '关闭资料包成员' })); expect(document.activeElement).toBe(screen.getByRole('button', { name: '查看原快照成员' }));
  fireEvent.click(screen.getByRole('button', { name: '确认快照回执并关闭' })); await waitFor(() => expect(close).toHaveBeenCalledOnce()); expect(readSnapshotRecovery(slot)).toBeNull();
});
it('lost create response then remount only looks up original key', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(observation)).mockRejectedValueOnce(new TypeError('network')).mockResolvedValueOnce(json(await receipt()));
  const view = mount(); await preview(); fireEvent.click(screen.getByRole('button', { name: '确认创建提交时快照' })); await screen.findByRole('button', { name: '查询原快照回执' }); const original = readSnapshotRecovery(slot)!;
  view.unmount(); mount(); await screen.findByRole('button', { name: '查询原快照回执' }); expect(fetch).toHaveBeenCalledTimes(2);
  fireEvent.click(screen.getByRole('button', { name: '查询原快照回执' })); await screen.findByText('已确认原快照创建'); expect(JSON.parse(fetch.mock.calls[2][1]?.body as string)).toEqual({ idempotency_key: original.key }); expect(fetch.mock.calls.filter(([url]) => url === path + '/snapshots/creations')).toHaveLength(1);
});
it('404 may retry only original revision and key with human confirmation', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(observation)).mockRejectedValueOnce(new TypeError('network')).mockResolvedValueOnce(json({}, 404)).mockResolvedValueOnce(json({})).mockResolvedValueOnce(json(await receipt()));
  mount(); await preview(); fireEvent.click(screen.getByRole('button', { name: '确认创建提交时快照' })); fireEvent.click(await screen.findByRole('button', { name: '查询原快照回执' })); fireEvent.click(await screen.findByRole('button', { name: '确认按原键重试快照' })); await screen.findByText('已确认原快照创建');
  expect(fetch.mock.calls[3][1]?.body).toBe(fetch.mock.calls[1][1]?.body); expect(fetch.mock.calls[3][1]?.headers).toEqual(fetch.mock.calls[1][1]?.headers);
});
it('conflict never silently changes revision or offers retry after missing lookup', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(observation)).mockResolvedValueOnce(json({}, 409)).mockResolvedValueOnce(json({}, 404)); mount(); await preview(); fireEvent.click(screen.getByRole('button', { name: '确认创建提交时快照' })); fireEvent.click(await screen.findByRole('button', { name: '查询原快照回执' })); await screen.findByText(/未取得原快照提交回执/); expect(screen.queryByRole('button', { name: '确认按原键重试快照' })).toBeNull(); expect(readSnapshotRecovery(slot)).not.toBeNull();
});
it.each([{ collection_revision: 3 }, { snapshot_id: 'bad' }, { collection_id: 'other' }, { request_digest: 'f'.repeat(64) }, { items: [] }])('rejects mismatched original snapshot receipt %j', async change => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(observation)).mockResolvedValueOnce(json({})).mockResolvedValueOnce(json({ ...await receipt(), ...change })); mount(); await preview(); fireEvent.click(screen.getByRole('button', { name: '确认创建提交时快照' })); await screen.findByRole('button', { name: '查询原快照回执' }); expect(screen.queryByText('已确认原快照创建')).toBeNull(); expect(readSnapshotRecovery(slot)).not.toBeNull();
});
it.each([{ ...observation, coverage: 'partial' }, { ...observation, items: [...observation.items, observation.items[0]], visible_count: 2 }, { ...observation, revision: 0 }])('incomplete or invalid observation cannot create', async value => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(value)); mount(); fireEvent.click(await screen.findByRole('button', { name: '预览当前可见成员' })); await screen.findByText(/无法取得完整当前观察/); expect(screen.queryByRole('button', { name: '确认创建提交时快照' })).toBeNull(); expect(fetch).toHaveBeenCalledTimes(1); expect(localStorage.length).toBe(0);
});
it.each(['offline', 'drivebridge-context', 'storage'])('discards late preview after %s', async event => {
  let finish!: (response: Response) => void; vi.spyOn(globalThis, 'fetch').mockImplementation(() => new Promise(resolve => { finish = resolve; })); mount(); fireEvent.click(await screen.findByRole('button', { name: '预览当前可见成员' })); await waitFor(() => expect(finish).toBeDefined()); fireEvent(window, event === 'storage' ? new StorageEvent('storage', { key: slot }) : new Event(event)); finish(json(observation)); await screen.findByText(/快照信息已清除/); expect(screen.queryByText('private/reference.txt')).toBeNull();
});
it('denied reads clear parent and missing locks prevent creation', async () => {
  const denied = vi.fn(); const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({}, 403)).mockResolvedValueOnce(json(observation)); const view = mount(vi.fn(), denied); fireEvent.click(await screen.findByRole('button', { name: '预览当前可见成员' })); await waitFor(() => expect(denied).toHaveBeenCalledOnce()); view.unmount(); Object.defineProperty(navigator, 'locks', { configurable: true, value: undefined }); mount(); await preview(); fireEvent.click(screen.getByRole('button', { name: '确认创建提交时快照' })); await screen.findByText(/跨标签锁不可用/); expect(fetch).toHaveBeenCalledTimes(2);
});
it('does not replace existing claims or clear newer recovery', async () => {
  const first = await claimSnapshot(slot, 'a'.repeat(64), new AbortController().signal), second = await claimSnapshot(slot, 'b'.repeat(64), new AbortController().signal); expect(second.claimed).toBe(false); expect(second.record).toEqual(first.record);
  localStorage.setItem(slot, JSON.stringify({ key: 'snapshot-' + 'c'.repeat(48), digest: 'c'.repeat(64) })); await expect(clearSnapshot(slot, first.record)).rejects.toThrow(); localStorage.setItem(slot, JSON.stringify({ ...first.record, revision: 1 })); expect(() => readSnapshotRecovery(slot)).toThrow();
});
