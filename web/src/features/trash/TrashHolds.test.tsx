import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { TrashHolds } from './TrashHolds';
import type { components } from '../../generated/personal-workspace';
const file = { id: 'a'.repeat(48), space_id: 'b'.repeat(48), version_id: 'c'.repeat(48), path: 'held.txt', mime_type: 'text/plain', size_bytes: 4, trashed: true, updated_at: '2026-10-09T00:00:00Z' };
const hold = { hold_id: 'd'.repeat(48), file_id: file.id, version_id: file.version_id, revision: 1, state: 'active' as const, purpose: 'legal_hold', source_kind: 'operation', source_ref: 'e'.repeat(48), expires_at: '0001-01-01T00:00:00Z', allowed_actions: ['release'] };
type Page = components['schemas']['WorkspaceHoldListEnvelope']['data'];
let page: Page, lost = false, releases = 0;
let receipt: components['schemas']['WorkspaceHoldReceiptEnvelope']['data'] | undefined;
const json = (data: unknown) => new Response(JSON.stringify({ data }));
const error = (status: number, code: string) => new Response(JSON.stringify({ error: { code } }), { status });
const locks = Object.getOwnPropertyDescriptor(navigator, 'locks');
beforeEach(() => {
  page = { items: [{ ...hold }], next_cursor: '', observed_at: '2026-10-09T00:00:00Z', total: null, total_kind: 'unknown', management_enabled: true }; lost = false; releases = 0; receipt = undefined;
  Object.defineProperty(navigator, 'locks', { configurable: true, value: { request: vi.fn(async (_name: string, options: LockOptions | (() => unknown), callback?: () => unknown) => typeof options === 'function' ? options() : callback!()) } });
  vi.spyOn(globalThis, 'fetch').mockImplementation(async (url, options) => {
    const path = String(url);
    if (path === '/api/v1/me') return json({ user: { id: 'alice' }, instance_id: 'fixture' });
    if (path.includes('/retention-holds?')) return json(page);
    if (path.endsWith('/lookup')) return receipt ? json(receipt) : error(404, 'not_found');
    if (path.endsWith('/release-cas')) {
      releases++;
      const body = JSON.parse(String(options?.body));
      expect(body).toEqual({ expected_revision: 1 });
      const raw = JSON.stringify({ FileID: file.id, HoldID: hold.hold_id, Request: body });
      const digest = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(raw))), b => b.toString(16).padStart(2, '0')).join('');
      receipt = { file_id: file.id, hold_id: hold.hold_id, action: 'release', request_digest: digest, result_revision: 2, committed_at: '2026-10-09T00:00:00Z' };
      page.items = [{ ...hold, revision: 2, state: 'released', allowed_actions: [] }];
      if (lost) throw new TypeError('lost_response');
      return json(receipt);
    }
    throw new Error('unexpected:' + path);
  });
});
afterEach(() => { cleanup(); localStorage.clear(); vi.restoreAllMocks(); if (locks) Object.defineProperty(navigator, 'locks', locks); else Reflect.deleteProperty(navigator, 'locks'); });
async function open() { fireEvent.click(screen.getByRole('button', { name: '保留保护 held.txt' })); await screen.findByText('保护生效'); }
it('reads on demand and explicitly confirms a fixed hold CAS without deleting the file', async () => {
  render(<TrashHolds file={file} restricted={false}/>); expect(fetch).not.toHaveBeenCalled(); await open();
  expect(screen.getByText(/未设置固定到期/)).toBeTruthy();
  fireEvent.click(screen.getByRole('button', { name: '解除保护' })); expect(releases).toBe(0);
  fireEvent.click(screen.getByRole('button', { name: '确认解除此保护' }));
  await screen.findByText('已解除'); expect(releases).toBe(1); expect(localStorage.length).toBe(0);
  expect(vi.mocked(fetch).mock.calls.every(([, options]) => options?.method !== 'DELETE')).toBe(true);
});
it('recovers a lost release from the original key after closing and reopening', async () => {
  lost = true; const view = render(<TrashHolds file={file} restricted={false}/>); await open();
  fireEvent.click(screen.getByRole('button', { name: '解除保护' })); fireEvent.click(screen.getByRole('button', { name: '确认解除此保护' }));
  await waitFor(() => expect(screen.getByRole('button', { name: '查询原保护回执' })).toHaveProperty('disabled', false));
  const saved = localStorage.getItem(localStorage.key(0)!)!; expect(saved).not.toContain(file.path); expect(saved).not.toContain(hold.source_ref);
  view.unmount(); render(<TrashHolds file={file} restricted={false}/>);
  fireEvent.click(screen.getByRole('button', { name: '保留保护 held.txt' }));
  await waitFor(() => expect(screen.getByRole('button', { name: '查询原保护回执' })).toHaveProperty('disabled', false));
  fireEvent.click(screen.getByRole('button', { name: '查询原保护回执' }));
  await screen.findByText(/已确认原解除回执/); expect(releases).toBe(1); expect(localStorage.length).toBe(0);
});
it('keeps management-disabled, restricted and unapproved rows read-only', async () => {
  page.management_enabled = false; const view = render(<TrashHolds file={file} restricted={false}/>); await open();
  expect(screen.queryByRole('button', { name: '解除保护' })).toBeNull(); expect(screen.getByText(/未启用保护管理/)).toBeTruthy(); view.unmount();
  page.management_enabled = true; render(<TrashHolds file={file} restricted/>); await open();
  expect(screen.queryByRole('button', { name: '解除保护' })).toBeNull(); expect(releases).toBe(0);
});
it('distinguishes empty state and denies malformed fixed-version rows', async () => {
  page.items = []; const view = render(<TrashHolds file={file} restricted={false}/>);
  fireEvent.click(screen.getByRole('button', { name: '保留保护 held.txt' })); await screen.findByText(/此固定版本没有保留保护记录/); view.unmount();
  page.items = [{ ...hold, version_id: 'f'.repeat(48) }]; render(<TrashHolds file={file} restricted={false}/>);
  fireEvent.click(screen.getByRole('button', { name: '保留保护 held.txt' })); await screen.findByText(/不能判断为无保护/); expect(releases).toBe(0);
});
it.each(['drivebridge-context', 'offline'])('clears hold rows and pending confirmation after %s', async event => {
  render(<TrashHolds file={file} restricted={false}/>); await open(); fireEvent.click(screen.getByRole('button', { name: '解除保护' }));
  fireEvent(window, new Event(event)); await screen.findByText(/保护详情已清除/);
  expect(screen.queryByText(hold.source_ref)).toBeNull(); expect(screen.queryByRole('button', { name: '确认解除此保护' })).toBeNull();
});
it('invalidates stale pagination instead of retaining release controls', async () => {
  page.next_cursor = 'next'; render(<TrashHolds file={file} restricted={false}/>); await open();
  vi.mocked(fetch).mockResolvedValue(error(410, 'cursor_expired'));
  fireEvent.click(screen.getByRole('button', { name: '更多保护记录' })); await screen.findByText(/分页已失效/);
  expect(screen.queryByRole('button', { name: '解除保护' })).toBeNull();
});
it('retains unknown identity when a lookup receipt mismatches', async () => {
  lost = true; render(<TrashHolds file={file} restricted={false}/>); await open();
  fireEvent.click(screen.getByRole('button', { name: '解除保护' })); fireEvent.click(screen.getByRole('button', { name: '确认解除此保护' }));
  await waitFor(() => expect(screen.getByRole('button', { name: '查询原保护回执' })).toHaveProperty('disabled', false));
  receipt!.request_digest = '0'.repeat(64); fireEvent.click(screen.getByRole('button', { name: '查询原保护回执' }));
  await waitFor(() => expect(screen.getByRole('button', { name: '查询原保护回执' })).toHaveProperty('disabled', false));
  expect(localStorage.length).toBe(1); expect(releases).toBe(1);
});

it('retains original lookup while management is switched off after a lost release', async () => {
  lost = true; render(<TrashHolds file={file} restricted={false}/>); await open();
  fireEvent.click(screen.getByRole('button', { name: '解除保护' })); fireEvent.click(screen.getByRole('button', { name: '确认解除此保护' }));
  await waitFor(() => expect(screen.getByRole('button', { name: '查询原保护回执' })).toHaveProperty('disabled', false));
  page.management_enabled = false;
  fireEvent.click(screen.getByRole('button', { name: '重读保护状态' }));
  await screen.findByText(/未启用保护管理/); fireEvent.click(screen.getByRole('button', { name: '查询原保护回执' }));
  await screen.findByText(/已确认原解除回执/); expect(releases).toBe(1); expect(localStorage.length).toBe(0);
});

it('clears a known revision conflict and requires reread before another release', async () => {
  render(<TrashHolds file={file} restricted={false}/>); await open();
  vi.mocked(fetch).mockImplementation(async url => String(url) === '/api/v1/me' ? json({ user: { id: 'alice' }, instance_id: 'fixture' }) : error(409, 'revision_conflict'));
  fireEvent.click(screen.getByRole('button', { name: '解除保护' })); fireEvent.click(screen.getByRole('button', { name: '确认解除此保护' }));
  await screen.findByText(/保护修订冲突/);
  expect(localStorage.length).toBe(0); expect(screen.queryByRole('button', { name: '解除保护' })).toBeNull();
  expect(screen.getByRole('button', { name: '重读保护状态' })).toBeTruthy();
});

it('clears private hold details on denied reads and never treats denial as no protection', async () => {
  vi.mocked(fetch).mockResolvedValue(error(403, 'forbidden'));
  render(<TrashHolds file={file} restricted={false}/>);
  fireEvent.click(screen.getByRole('button', { name: '保留保护 held.txt' })); await screen.findByText(/保护详情已清除/);
  expect(screen.queryByText(/没有保留保护记录/)).toBeNull(); expect(screen.queryByRole('button', { name: '重读保护状态' })).toBeNull();
});

it('accepts only server release capability, not an active state alone', async () => {
  page.items[0].allowed_actions = [];
  render(<TrashHolds file={file} restricted={false}/>); await open();
  expect(screen.queryByRole('button', { name: '解除保护' })).toBeNull(); expect(releases).toBe(0);
});
