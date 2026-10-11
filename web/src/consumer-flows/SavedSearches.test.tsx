import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { savedPath, SavedSearches } from './SavedSearches';
const query = { mode: 'exact', space_ids: ['saved-space'], filters: [{ field: 'path_prefix', op: 'contains', value: 'report%' }], limit: 50 };
const row = { query_id: 'saved', name: 'Report', query, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' };
const json = (data: unknown, status = 200) => new Response(JSON.stringify({ data }), { status });
const props = { onSelect: vi.fn(), onDenied: vi.fn() };
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.clearAllMocks(); });
it('reads once and selects without executing or saving', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json([row]));
  render(<SavedSearches {...props}/>);
  fireEvent.click(await screen.findByRole('button', { name: '选用条件 Report' }));
  expect(props.onSelect).toHaveBeenCalledWith({ text: 'report%', spaces: ['saved-space'] });
  expect(fetch).toHaveBeenCalledTimes(1); expect(fetch.mock.calls[0][0]).toBe('/api/v1/saved-searches'); expect(fetch.mock.calls[0][1]?.method).toBe('GET');
});
it.each([
  { ...query, mode: 'duplicates' }, { ...query, sort: [{ field: 'name', direction: 'asc' }] },
  { ...query, limit: 10 }, { ...query, cursor: 'old-page' }, { ...query, filters: [...query.filters, { field: 'mime_type', op: 'eq', value: 'text/plain' }] },
  { ...query, unknown: true }, { ...query, space_ids: [] }, { ...query, space_ids: ['x', 'x'] },
])('never simplifies unsupported saved conditions', async unsupported => {
  expect(savedPath(unsupported)).toBeNull();
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json([{ ...row, query: unsupported }])); render(<SavedSearches {...props}/>);
  await screen.findByText(/暂不能完整表达/); expect(screen.queryByRole('button', { name: /选用条件/ })).toBeNull();
});
it.each([{ data: [row, row] }, { data: [{ ...row, query: 'invalid' }] }, { data: [{ ...row, query_id: '' }] }])('rejects invalid arrays', async ({ data }) => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(data)); render(<SavedSearches {...props}/>);
  await screen.findByText(/合同不匹配/); expect(screen.queryByText('Report')).toBeNull();
});
it.each([401, 403])('clears revoked %s and propagates to parent', async status => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json([row])).mockResolvedValueOnce(json({}, status)); render(<SavedSearches {...props}/>);
  await screen.findByText('Report'); fireEvent.click(screen.getByRole('button', { name: '重读已保存条件' }));
  await screen.findByText(/权限或范围已变化/); expect(props.onDenied).toHaveBeenCalledTimes(1); expect(screen.queryByText('Report')).toBeNull();
});
it.each(['offline', 'drivebridge-context'])('ignores late response on %s without auto reload', async event => {
  let finish!: (value: Response) => void;
  const fetch = vi.spyOn(globalThis, 'fetch').mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  render(<SavedSearches {...props}/>); fireEvent(window, new Event(event)); finish(json([row]));
  await waitFor(() => expect(screen.queryByText('Report')).toBeNull()); fireEvent(window, new Event('online')); expect(fetch).toHaveBeenCalledTimes(1);
});
