import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { NotebookFavorite } from './NotebookFavorite';
import type { components } from '../../generated/personal-workspace';

const space = 'a'.repeat(48), fileId = 'b'.repeat(48), version = 'c'.repeat(48), favoriteId = 'd'.repeat(48);
const identity = { sourceId: space, objectId: fileId, revision: version };
const logical = `drivebridge://fixture/file/${fileId}`;
const file = { id: fileId, space_id: space, version_id: version, path: 'note.md', mime_type: 'text/markdown', trashed: false, size_bytes: 12, updated_at: '2026-10-09T00:00:00Z' };
const row = (revision = 1, deleted = false) => ({ favorite_id: favoriteId, space_id: space, target_kind: 'file', target_ref: logical, revision, deleted });
const json = (data: unknown) => new Response(JSON.stringify({ data }));
const failure = (status: number, code: string) => new Response(JSON.stringify({ error: { code } }), { status });
const locks = Object.getOwnPropertyDescriptor(navigator, 'locks');
let entries: components['schemas']['FavoriteListEnvelope']['data']['items'] = [];
let writes: { method: string; path: string; body: Record<string, unknown>; key: string }[];
let receipt: Record<string, unknown>;
let drop = false, conflict = false;
async function transport(url: RequestInfo | URL, options?: RequestInit): Promise<Response> {
  const path = String(url), method = options?.method ?? 'GET';
  if (path === `/api/v1/files/${fileId}`) return json({ file, ref: `${logical}/version/${version}` });
  if (path.startsWith('/api/v1/references?')) return json({ file, ref: `${logical}/version/${version}`, sha256: 'e'.repeat(64) });
  if (path.startsWith('/api/v1/favorites?')) return json({ items: entries, next_cursor: '', observed_at: '2026-10-09T00:00:00Z', total: null, total_kind: 'unknown' });
  const body = JSON.parse(String(options?.body ?? '{}')) as Record<string, unknown>;
  if (path === '/api/v1/favorites/lookup') return receipt ? json(receipt) : failure(404, 'not_found');
  if (path === '/api/v1/favorites' || path === `/api/v1/favorites/${favoriteId}`) {
    const key = (options?.headers as Record<string, string>)['Idempotency-Key'];
    writes.push({ path, method, body, key });
    if (conflict) return failure(409, 'revision_conflict');
    const remove = method === 'DELETE';
    const request = { space_id: remove ? '' : space, target_kind: remove ? '' : 'file', target_ref: remove ? '' : logical, expected_revision: body.expected_revision };
    const raw = JSON.stringify({ SpaceID: space, ID: remove ? favoriteId : '', Request: request });
    const digest = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(raw))), b => b.toString(16).padStart(2, '0')).join('');
    receipt = { favorite_id: favoriteId, space_id: space, result_revision: Number(body.expected_revision) + 1, deleted: remove, request_digest: digest, committed_at: '2026-10-09T00:00:00Z' };
    entries = [row(Number(body.expected_revision) + 1, remove)];
    if (drop) throw new TypeError('lost_response');
    return json(receipt);
  }
  throw new Error('unexpected_request:' + path);
}
beforeEach(() => {
  entries = []; writes = []; receipt = undefined!; drop = false; conflict = false;
  Object.defineProperty(navigator, 'locks', { configurable: true, value: { request: vi.fn(async (_name: string, options: LockOptions | (() => unknown), callback?: () => unknown) => typeof options === 'function' ? options() : callback!()) } });
  vi.spyOn(globalThis, 'fetch').mockImplementation(transport);
});
afterEach(() => { cleanup(); localStorage.clear(); vi.restoreAllMocks(); if (locks) Object.defineProperty(navigator, 'locks', locks); else Reflect.deleteProperty(navigator, 'locks'); });

it('creates, removes only the binding and re-adds using the tombstone revision', async () => {
  render(<NotebookFavorite identity={identity} actor="alice"/>);
  fireEvent.click(await screen.findByRole('button', { name: '收藏笔记' }));
  fireEvent.click(await screen.findByRole('button', { name: '取消收藏' }));
  fireEvent.click(await screen.findByRole('button', { name: '收藏笔记' }));
  await screen.findByRole('button', { name: '取消收藏' });
  expect(writes.map(write => [write.method, write.body.expected_revision])).toEqual([['POST', 0], ['DELETE', 1], ['POST', 2]]);
  expect(writes[0].body).toEqual({ space_id: space, target_kind: 'file', target_ref: logical, expected_revision: 0 });
  expect(writes[1].path).toBe(`/api/v1/favorites/${favoriteId}`);
  expect(new Set(writes.map(write => write.key)).size).toBe(3);
  expect(localStorage.length).toBe(0);
});

it('finds tombstones beyond the first page before permitting a mutation', async () => {
  vi.mocked(fetch).mockImplementation(async (url, options) => {
    if (String(url).startsWith('/api/v1/favorites?')) {
      const more = new URL(String(url), 'http://fixture').searchParams.get('cursor');
      return json({ items: more ? [row(8, true)] : [], next_cursor: more ? '' : 'next-page', observed_at: '2026-10-09T00:00:00Z', total: null, total_kind: 'unknown' });
    }
    return transport(url, options);
  });
  render(<NotebookFavorite identity={identity} actor="alice"/>);
  fireEvent.click(await screen.findByRole('button', { name: '收藏笔记' }));
  await waitFor(() => expect(writes).toHaveLength(1));
  expect(writes[0].body.expected_revision).toBe(8);
  expect(vi.mocked(fetch).mock.calls.some(([url]) => String(url).includes('cursor=next-page'))).toBe(true);
});

it('remounts an unknown operation as lookup-only and validates the original digest', async () => {
  drop = true;
  const view = render(<NotebookFavorite identity={identity} actor="alice"/>);
  fireEvent.click(await screen.findByRole('button', { name: '收藏笔记' }));
  await waitFor(() => expect(screen.getByRole('button', { name: '查询原收藏结果' })).toHaveProperty('disabled', false));
  const stored = localStorage.getItem(localStorage.key(0)!)!;
  expect(stored).not.toContain(logical); expect(stored).not.toContain('note.md');
  view.unmount(); render(<NotebookFavorite identity={{ ...identity, revision: version }} actor="alice"/>);
  fireEvent.click(await screen.findByRole('button', { name: '查询原收藏结果' }));
  await screen.findByRole('button', { name: '取消收藏' });
  expect(writes).toHaveLength(1);
  const call = vi.mocked(fetch).mock.calls.find(([url]) => String(url) === '/api/v1/favorites/lookup')!;
  expect(JSON.parse(String(call[1]?.body))).toEqual({ space_id: space, action: 'create', idempotency_key: writes[0].key });
});

it('never clears unknown identity on a missing or mismatched receipt', async () => {
  drop = true; render(<NotebookFavorite identity={identity} actor="alice"/>);
  fireEvent.click(await screen.findByRole('button', { name: '收藏笔记' }));
  const lookup = await screen.findByRole('button', { name: '查询原收藏结果' });
  await waitFor(() => expect(lookup).toHaveProperty('disabled', false));
  receipt.request_digest = '0'.repeat(64); fireEvent.click(lookup);
  await waitFor(() => expect(lookup).toHaveProperty('disabled', false));
  expect(screen.getByText(/原结果未确认/)).toBeTruthy();
  expect(localStorage.length).toBe(1); expect(writes).toHaveLength(1);
  receipt = undefined!; fireEvent.click(lookup);
  await waitFor(() => expect(lookup).toHaveProperty('disabled', false));
  expect(localStorage.length).toBe(1); expect(screen.queryByRole('button', { name: '收藏笔记' })).toBeNull();
});

it('preserves a conflict intent, rereads and requires explicit confirmation with a fresh CAS', async () => {
  conflict = true; render(<NotebookFavorite identity={identity} actor="alice"/>);
  fireEvent.click(await screen.findByRole('button', { name: '收藏笔记' }));
  await screen.findByText(/修订冲突/); expect(localStorage.length).toBe(0);
  entries = [row(4, true)]; conflict = false;
  fireEvent.click(screen.getByRole('button', { name: '重读收藏状态' }));
  const confirm = await screen.findByRole('button', { name: '确认重新收藏' });
  expect(writes).toHaveLength(1); fireEvent.click(confirm);
  await screen.findByRole('button', { name: '取消收藏' });
  expect(writes[1].body.expected_revision).toBe(4); expect(writes[1].key).not.toBe(writes[0].key);
});

it.each(['drivebridge-context', 'offline'])('clears a late authorized result after %s', async event => {
  let finish!: (response: Response) => void;
  // ES2022 project libraries lack Promise.withResolvers; keep the existing test gate pattern.
  vi.mocked(fetch).mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  render(<NotebookFavorite identity={identity} actor="alice"/>);
  fireEvent(window, new Event(event));
  await act(async () => finish(json({ file, ref: `${logical}/version/${version}` })));
  expect(screen.getByRole('alert').textContent).toContain('已清除');
  expect(screen.queryByRole('button', { name: '收藏笔记' })).toBeNull();
});

it('denies unsafe source and mismatched pinned reference without writes', async () => {
  const view = render(<NotebookFavorite identity={{ ...identity, sourceId: 'pinax:workspace' }} actor="alice"/>);
  await screen.findByText(/仅支持普通文件/); expect(fetch).not.toHaveBeenCalled(); view.unmount();
  vi.mocked(fetch).mockImplementation(async (url, options) => String(url).startsWith('/api/v1/references?') ? json({ file, ref: `${logical}/version/foreign`, sha256: 'e'.repeat(64) }) : transport(url, options));
  render(<NotebookFavorite identity={identity} actor="alice"/>);
  await screen.findByText(/无法确认收藏状态/); expect(writes).toEqual([]);
});

it('fails closed on looping pagination and authorization loss', async () => {
  vi.mocked(fetch).mockImplementation(async (url, options) => String(url).startsWith('/api/v1/favorites?') ? json({ items: [], next_cursor: 'loop' }) : transport(url, options));
  const view = render(<NotebookFavorite identity={identity} actor="alice"/>);
  await screen.findByText(/无法确认收藏状态/); expect(writes).toEqual([]); view.unmount();
  vi.mocked(fetch).mockResolvedValue(failure(403, 'forbidden'));
  render(<NotebookFavorite identity={identity} actor="alice"/>);
  await screen.findByText(/已清除/); expect(screen.queryByRole('button', { name: '重读收藏状态' })).toBeNull();
});

it('preserves the original identity on idempotency conflict instead of allowing a new key', async () => {
  vi.mocked(fetch).mockImplementation(async (url, options) => String(url) === '/api/v1/favorites' ? failure(409, 'idempotency_conflict') : transport(url, options));
  render(<NotebookFavorite identity={identity} actor="alice"/>);
  fireEvent.click(await screen.findByRole('button', { name: '收藏笔记' }));
  await waitFor(() => expect(screen.getByRole('button', { name: '查询原收藏结果' })).toHaveProperty('disabled', false));
  expect(localStorage.length).toBe(1);
  expect(screen.queryByRole('button', { name: '重读收藏状态' })).toBeNull();
  expect(screen.queryByRole('button', { name: '确认重新收藏' })).toBeNull();
});

it('requires durable recovery storage and a cross-tab claim before sending a write', async () => {
  Object.defineProperty(navigator, 'locks', { configurable: true, value: undefined });
  render(<NotebookFavorite identity={identity} actor="alice"/>);
  fireEvent.click(await screen.findByRole('button', { name: '收藏笔记' }));
  await screen.findByText(/未发送收藏修改/);
  expect(writes).toEqual([]); expect(localStorage.length).toBe(0);
});

it('clears visible favorite facts on mutation denial but retains the original recovery record', async () => {
  entries = [row()];
  vi.mocked(fetch).mockImplementation(async (url, options) => options?.method === 'DELETE' ? failure(403, 'forbidden') : transport(url, options));
  render(<NotebookFavorite identity={identity} actor="alice"/>);
  fireEvent.click(await screen.findByRole('button', { name: '取消收藏' }));
  await screen.findByText(/已清除/);
  expect(screen.queryByText(/已收藏此笔记/)).toBeNull();
  expect(screen.queryByRole('button', { name: '查询原收藏结果' })).toBeNull();
  expect(localStorage.length).toBe(1);
});

it('does not read another actor recovery slot or infer success from the current list', async () => {
  drop = true;
  const view = render(<NotebookFavorite identity={identity} actor="alice"/>);
  fireEvent.click(await screen.findByRole('button', { name: '收藏笔记' }));
  await waitFor(() => expect(screen.getByRole('button', { name: '查询原收藏结果' })).toHaveProperty('disabled', false));
  view.unmount(); entries = [];
  render(<NotebookFavorite identity={identity} actor="bob"/>);
  await screen.findByRole('button', { name: '收藏笔记' });
  expect(screen.queryByRole('button', { name: '查询原收藏结果' })).toBeNull();
  expect(localStorage.length).toBe(1); expect(writes).toHaveLength(1);
});

it('invalidates a recovery record replaced by another tab before accepting a receipt', async () => {
  drop = true; render(<NotebookFavorite identity={identity} actor="alice"/>);
  fireEvent.click(await screen.findByRole('button', { name: '收藏笔记' }));
  await waitFor(() => expect(screen.getByRole('button', { name: '查询原收藏结果' })).toHaveProperty('disabled', false));
  const slot = localStorage.key(0)!;
  fireEvent(window, new StorageEvent('storage', { key: slot }));
  await screen.findByText(/已清除/);
  expect(screen.queryByRole('button', { name: '查询原收藏结果' })).toBeNull();
  expect(writes).toHaveLength(1); expect(localStorage.length).toBe(1);
});
