import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { SnapshotHistory } from './SnapshotHistory';
const snapshot = 'a'.repeat(48), date = '2026-01-01T00:00:00Z';
const row = { snapshot_id: snapshot, collection_revision: 2, created_at: date };
const index = { collection_id: 'collection', items: [row], observed_at: date };
const json = (data: unknown, status = 200) => new Response(JSON.stringify({ data }), { status });
const mount = (denied = vi.fn()) => render(<SnapshotHistory id="collection" close={vi.fn()} denied={denied}/>);
afterEach(() => { cleanup(); vi.restoreAllMocks(); });
it('reads only explicitly, expands locally, never queries receipts or registers resources', async () => {
  const items = Array.from({ length: 51 }, (_, i) => ({ ...row, snapshot_id: i.toString(16).padStart(48, '0') }));
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json({ ...index, items })); mount(); expect(fetch).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: '读取历史快照索引' })); await screen.findByRole('button', { name: '展开更多已读取快照' }); expect(screen.getAllByRole('listitem')).toHaveLength(50);
  fireEvent.click(screen.getByRole('button', { name: '展开更多已读取快照' })); expect(screen.getAllByRole('listitem')).toHaveLength(51); expect(fetch).toHaveBeenCalledTimes(1); expect(fetch.mock.calls[0][0]).toBe('/api/v1/file-collections/collection/snapshots'); expect(fetch.mock.calls[0][1]?.method).toBe('GET');
});
it.each([{ ...index, collection_id: 'other' }, { ...index, items: null }, { ...index, items: [row, row] }, { ...index, items: Array(1001).fill(row) }, { ...index, request_digest: 'invented' }, { ...index, items: [{ ...row, entries_total: 1 }] }, { ...index, items: [{ ...row, collection_revision: 0 }] }])('rejects incomplete or expanded index contract', async data => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(data)); mount(); fireEvent.click(screen.getByRole('button', { name: '读取历史快照索引' })); await screen.findByText(/合同不匹配/); expect(screen.queryByRole('listitem')).toBeNull();
});
it.each([[429, '查询预算'], [404, '入口不可用'], [500, '合同不匹配']])('keeps %s distinct from empty success', async (status, message) => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(json({}, Number(status))); mount(); fireEvent.click(screen.getByRole('button', { name: '读取历史快照索引' })); await screen.findByText(new RegExp(String(message))); expect(screen.queryByText('当前授权集合没有已返回的历史快照。')).toBeNull();
});
it('accepts a real empty list and propagates denied read', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({ ...index, items: [] })).mockResolvedValueOnce(json({}, 403)); const denied = vi.fn(); mount(denied); fireEvent.click(screen.getByRole('button', { name: '读取历史快照索引' })); await screen.findByText('当前授权集合没有已返回的历史快照。'); fireEvent.click(screen.getByRole('button', { name: '刷新历史快照索引' })); await waitFor(() => expect(denied).toHaveBeenCalledOnce()); expect(screen.queryByRole('listitem')).toBeNull();
});
it.each(['offline', 'drivebridge-context'])('drops late index on %s', async event => {
  let finish!: (value: Response) => void; const fetch = vi.spyOn(globalThis, 'fetch').mockImplementation(() => new Promise(resolve => { finish = resolve; })); mount(); fireEvent.click(screen.getByRole('button', { name: '读取历史快照索引' })); fireEvent(window, new Event(event)); finish(json(index)); await waitFor(() => expect(screen.queryByRole('listitem')).toBeNull()); fireEvent(window, new Event('online')); expect(fetch).toHaveBeenCalledTimes(1);
});
it('reuses original snapshot reader, rejects wrong collection and restores index focus', async () => {
  const fixed = { snapshot_id: snapshot, collection_id: 'collection', collection_revision: 2, entries_total: 0, entries_visible: 0, created_at: date, items: [] };
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(index)).mockResolvedValueOnce(json(fixed)).mockResolvedValueOnce(json({ ...fixed, collection_id: 'foreign' })); mount(); fireEvent.click(screen.getByRole('button', { name: '读取历史快照索引' })); const trigger = await screen.findByRole('button', { name: '查看历史快照 ' + snapshot }); fireEvent.click(trigger); await screen.findByText('历史条目：0 · 本次可见：0'); expect(fetch.mock.calls[1][0]).toBe('/api/v1/collection-snapshots/' + snapshot);
  fireEvent.click(screen.getByRole('button', { name: '关闭资料包成员' })); expect(document.activeElement).toBe(trigger); fireEvent.click(trigger); await screen.findByText(/未换用当前集合/); expect(screen.queryByText('历史条目：0 · 本次可见：0')).toBeNull(); expect(fetch.mock.calls.every(([, init]) => init?.method === 'GET')).toBe(true);
});
