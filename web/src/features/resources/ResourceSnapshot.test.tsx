import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { ResourceSnapshot } from './ResourceSnapshot';
const item = { file_id: 'file', version_id: 'old', path: 'note.txt' };
const snapshot = { snapshot_id: 'snapshot', collection_id: 'collection', collection_revision: 2, entries_total: 2, entries_visible: 1, created_at: '2026-01-01T00:00:00Z', items: [item] };
const props = { snapshot: 'snapshot', close: vi.fn(), onDenied: vi.fn() };
const json = (data: unknown, status = 200) => new Response(JSON.stringify({ data }), { status });
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.clearAllMocks(); });
it('keeps partial fixed snapshot distinct from the current collection and content checks', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(snapshot)); render(<ResourceSnapshot {...props}/>);
  await screen.findByText(/部分成员不可见/);
  expect(fetch.mock.calls[0][0]).toBe('/api/v1/collection-snapshots/snapshot'); expect(fetch).toHaveBeenCalledTimes(1);
  expect(screen.getByRole('heading', { name: '资料包固定成员' })).toBe(document.activeElement);
});
it('expands already-read items without claiming server pagination', async () => {
  const items = Array.from({ length: 51 }, (_, i) => ({ ...item, file_id: 'file-' + i }));
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({ ...snapshot, items, entries_total: 51, entries_visible: 51 })); render(<ResourceSnapshot {...props}/>);
  await screen.findByRole('button', { name: '展开更多快照成员' }); expect(screen.getAllByRole('listitem')).toHaveLength(50);
  fireEvent.click(screen.getByRole('button', { name: '展开更多快照成员' })); expect(screen.getAllByRole('listitem')).toHaveLength(51); expect(fetch).toHaveBeenCalledTimes(1);
});
it('opens the original member version without automatic body reads and returns focus', async () => {
  const file = { id: 'file', space_id: 'space', version_id: 'new', path: 'renamed.txt' }, ref = 'drivebridge://instance/file/file/version/old';
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(snapshot)).mockResolvedValueOnce(json({ file })).mockResolvedValueOnce(json({ ref: 'drivebridge://instance/file/file/version/new' })).mockResolvedValueOnce(json({ file: { ...file, version_id: 'old' }, ref }));
  render(<ResourceSnapshot {...props}/>); const trigger = await screen.findByRole('button', { name: '打开固定成员：note.txt' }); fireEvent.click(trigger);
  await screen.findByText(ref); expect(fetch.mock.calls.every(([path]) => !String(path).includes('/content'))).toBe(true);
  expect(fetch.mock.calls[3][0]).toContain(encodeURIComponent(ref));
  fireEvent.click(screen.getByRole('button', { name: '关闭资料包成员结果引用' })); expect(document.activeElement).toBe(trigger);
});
it.each([{ ...snapshot, snapshot_id: 'other' }, { ...snapshot, entries_visible: 2 }, { ...snapshot, items: [item, item], entries_visible: 2 }, { ...snapshot, items: [{ ...item, version_id: '' }] }])('rejects inconsistent fixed snapshots', async data => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(data)); render(<ResourceSnapshot {...props}/>);
  await screen.findByText(/未换用当前集合/); expect(screen.queryByRole('listitem')).toBeNull();
});
it.each(['offline', 'drivebridge-context'])('drops late response on %s', async event => {
  let finish!: (value: Response) => void;
  const fetch = vi.spyOn(globalThis, 'fetch').mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  render(<ResourceSnapshot {...props}/>); fireEvent(window, new Event(event)); finish(json(snapshot));
  await waitFor(() => expect(screen.queryByRole('listitem')).toBeNull()); fireEvent(window, new Event('online')); expect(fetch).toHaveBeenCalledTimes(1);
});
it('propagates 403 to clear parent metadata', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({}, 403)); render(<ResourceSnapshot {...props}/>);
  await waitFor(() => expect(props.onDenied).toHaveBeenCalledOnce()); expect(screen.queryByRole('listitem')).toBeNull();
});
