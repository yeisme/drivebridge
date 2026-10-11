import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import fixtures from '../../../../contracts/personal-workspace/fixtures/responses.json';
import { AgentInvocations } from './AgentInvocations';
const page = fixtures.InvocationPageEnvelope;
const item = page.data.items[0];
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });
const props = { agent: item.agent_id, scope: 'fixture-scope', onDenied: vi.fn() };
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.clearAllMocks(); });
it('keeps time filters closed initially and preserves unapplied inputs without additional reads', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(page));
  render(<AgentInvocations {...props}/>); await screen.findByRole('listitem');
  const toggle = screen.getByText('日期与高级时间筛选');
  const disclosure = toggle.closest('details')!;
  expect(disclosure.open).toBe(false);
  fireEvent.click(toggle); expect(disclosure.open).toBe(true);
  fireEvent.change(screen.getByLabelText('开始时间晚于'), { target: { value: '2026-01-01T00:00:00Z' } });
  fireEvent.click(toggle); expect(disclosure.open).toBe(false);
  fireEvent.click(toggle); expect((screen.getByLabelText('开始时间晚于') as HTMLInputElement).value).toBe('2026-01-01T00:00:00Z');
  expect(fetch).toHaveBeenCalledTimes(1);
  expect(screen.getByRole('listitem').textContent).toContain(item.invocation_id);
});
it('changes owner ordering without reusing a pending cursor and retains date bounds', async () => {
  let finish!: (response: Response) => void;
  const newest = { ...item, invocation_id: 'z-new', started_at: '2026-01-01T00:00:01Z' };
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json({ data: { ...page.data, next_cursor: 'old-order-page' } })).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; })).mockResolvedValueOnce(json({ data: { ...page.data, items: [newest, item], next_cursor: 'new-order-page' } })).mockResolvedValueOnce(json({ data: { ...page.data, items: [] } })).mockResolvedValueOnce(json(page));
  render(<AgentInvocations {...props}/>); await screen.findByRole('listitem');
  fireEvent.change(screen.getByLabelText('开始日期'), { target: { value: '2026-01-01' } }); fireEvent.change(screen.getByLabelText('结束日期（含当日）'), { target: { value: '2026-01-01' } });
  fireEvent.click(screen.getByRole('button', { name: '应用日期筛选' }));
  fireEvent.click(await screen.findByRole('button', { name: '更多调用记录' })); await waitFor(() => expect(fetch).toHaveBeenCalledTimes(3));
  const signal = fetch.mock.calls[2][1]?.signal;
  fireEvent.click(screen.getByRole('combobox', { name: '调用记录排序' }));
  const choice = await screen.findByRole('option', { name: '开始时间最新优先' }); fireEvent.pointerDown(choice, { pointerType: 'mouse' }); fireEvent.click(choice);
  await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(2)); expect(signal?.aborted).toBe(true);
  finish(json({ data: { ...page.data, items: [{ ...item, invocation_id: 'late' }] } }));
  await waitFor(() => expect(screen.queryByText('调用标识：late')).toBeNull());
  expect(screen.getAllByRole('listitem')[0].textContent).toContain('z-new');
  const query = new URL(String(fetch.mock.calls[3][0]), 'http://localhost').searchParams;
  expect(query.get('order')).toBe('started_desc'); expect(query.has('cursor')).toBe(false); expect(query.get('started_at_or_after')).toBe('2026-01-01T00:00:00.000Z');
  fireEvent.click(screen.getByRole('button', { name: '更多调用记录' })); await waitFor(() => expect(fetch).toHaveBeenCalledTimes(5));
  expect(String(fetch.mock.calls[4][0])).toContain('order=started_desc'); expect(String(fetch.mock.calls[4][0])).toContain('cursor=new-order-page');
  fireEvent.click(screen.getByRole('combobox', { name: '调用记录排序' })); const legacy = await screen.findByRole('option', { name: '原记录标识顺序' }); fireEvent.pointerDown(legacy, { pointerType: 'mouse' }); fireEvent.click(legacy);
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(6)); expect(String(fetch.mock.calls[5][0])).not.toContain('order='); expect(String(fetch.mock.calls[5][0])).not.toContain('cursor=');
});
it('does not silently fall back when an older service rejects time ordering', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json({}, 400));
  render(<AgentInvocations {...props}/>); await screen.findByRole('listitem');
  fireEvent.click(screen.getByRole('combobox', { name: '调用记录排序' }));
  const choice = await screen.findByRole('option', { name: '开始时间最新优先' }); fireEvent.pointerDown(choice, { pointerType: 'mouse' }); fireEvent.click(choice);
  await screen.findByText(/不会自动放宽查询/); expect(screen.queryByRole('listitem')).toBeNull(); expect(fetch).toHaveBeenCalledTimes(2);
  expect(String(fetch.mock.calls[1][0])).toContain('order=started_desc');
});
it('applies explicit UTC bounds on the server, clears the cursor and leaves drafts unapplied', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({ data: { ...page.data, next_cursor: 'old-page' } })).mockResolvedValueOnce(json({ data: { ...page.data, items: [] } })).mockResolvedValueOnce(json(page));
  render(<AgentInvocations {...props}/>); await screen.findByRole('listitem');
  fireEvent.change(screen.getByRole('textbox', { name: '开始时间晚于' }), { target: { value: '2026-10-06T09:00:00+08:00' } });
  fireEvent.change(screen.getByRole('textbox', { name: '开始时间早于' }), { target: { value: '2026-10-06T02:00:00Z' } });
  expect(fetch).toHaveBeenCalledTimes(1);
  fireEvent.click(screen.getByRole('button', { name: '应用时间筛选' })); await screen.findByText(/当前筛选没有可见观察记录/);
  const url = new URL(String(fetch.mock.calls[1][0]), 'http://localhost');
  expect(url.searchParams.get('started_after')).toBe('2026-10-06T01:00:00.000Z'); expect(url.searchParams.get('started_before')).toBe('2026-10-06T02:00:00.000Z'); expect(url.searchParams.has('cursor')).toBe(false);
  expect(screen.queryByRole('button', { name: '更多调用记录' })).toBeNull();
  fireEvent.click(screen.getByRole('button', { name: '清除时间筛选' })); await screen.findByRole('listitem');
  expect(String(fetch.mock.calls[2][0])).not.toContain('started_');
});
it('keeps applied bounds across result filtering and paging, then aborts an old page when bounds change', async () => {
  let finish!: (response: Response) => void;
  const empty = { data: { ...page.data, items: [], next_cursor: '' } };
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json({ data: { ...page.data, next_cursor: 'bounded-page' } })).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; })).mockResolvedValueOnce(json(empty)).mockResolvedValueOnce(json(empty));
  render(<AgentInvocations {...props}/>); await screen.findByRole('listitem');
  fireEvent.change(screen.getByRole('textbox', { name: '开始时间晚于' }), { target: { value: '2025-01-01T08:00:00+08:00' } });
  fireEvent.click(screen.getByRole('button', { name: '应用时间筛选' }));
  fireEvent.click(await screen.findByRole('button', { name: '更多调用记录' }));
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(3));
  const pending = fetch.mock.calls[2][1]?.signal;
  expect(String(fetch.mock.calls[2][0])).toContain('cursor=bounded-page');
  expect(new URL(String(fetch.mock.calls[2][0]), 'http://localhost').searchParams.get('started_after')).toBe('2025-01-01T00:00:00.000Z');
  fireEvent.change(screen.getByRole('textbox', { name: '开始时间晚于' }), { target: { value: '2027-01-01T00:00:00Z' } });
  fireEvent.click(screen.getByRole('button', { name: '应用时间筛选' }));
  await screen.findByText(/当前筛选没有可见观察记录/); expect(pending?.aborted).toBe(true);
  finish(json(page)); await waitFor(() => expect(screen.queryByRole('listitem')).toBeNull());
  expect(String(fetch.mock.calls[3][0])).not.toContain('cursor=');
  fireEvent.click(screen.getByRole('combobox', { name: '调用结果筛选' }));
  const option = await screen.findByRole('option', { name: '请求被拒绝' }); fireEvent.pointerDown(option, { pointerType: 'mouse' }); fireEvent.click(option);
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(5));
  const filtered = new URL(String(fetch.mock.calls[4][0]), 'http://localhost');
  expect(filtered.searchParams.get('result')).toBe('denied'); expect(filtered.searchParams.get('started_after')).toBe('2027-01-01T00:00:00.000Z');
  expect(fetch.mock.calls.every(([, options]) => options?.method === 'GET')).toBe(true);
});
it('rejects invalid bounds without changing the active query or issuing requests', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(page)); render(<AgentInvocations {...props}/>); await screen.findByRole('listitem');
  fireEvent.change(screen.getByRole('textbox', { name: '开始时间晚于' }), { target: { value: '2026-02-30T00:00:00Z' } });
  fireEvent.click(screen.getByRole('button', { name: '应用时间筛选' })); await screen.findByText(/请输入有效的带时区时间/);
  expect(fetch).toHaveBeenCalledTimes(1); expect(screen.getByRole('listitem')).toBeTruthy();
});
it('applies calendar dates as inclusive lower and exclusive next-day upper, without reusing legacy bounds', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(page)); render(<AgentInvocations {...props}/>); await screen.findByRole('listitem');
  fireEvent.change(screen.getByLabelText('开始日期'), { target: { value: '2024-02-29' } });
  fireEvent.change(screen.getByLabelText('结束日期（含当日）'), { target: { value: '2024-02-29' } });
  expect(fetch).toHaveBeenCalledTimes(1);
  fireEvent.click(screen.getByRole('button', { name: '应用日期筛选' })); await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
  const query = new URL(String(fetch.mock.calls[1][0]), 'http://localhost').searchParams;
  expect(query.get('started_at_or_after')).toBe('2024-02-29T00:00:00.000Z'); expect(query.get('started_before')).toBe('2024-03-01T00:00:00.000Z'); expect(query.has('started_after')).toBe(false);
  expect(screen.getByText(/已应用.*UTC.*包含起点/)).toBeTruthy();
  fireEvent.change(screen.getByRole('textbox', { name: '开始时间晚于' }), { target: { value: '2026-01-01T00:00:00Z' } });
  fireEvent.click(screen.getByRole('button', { name: '应用时间筛选' })); await waitFor(() => expect(fetch).toHaveBeenCalledTimes(3));
  const legacy = new URL(String(fetch.mock.calls[2][0]), 'http://localhost').searchParams;
  expect(legacy.has('started_at_or_after')).toBe(false); expect(legacy.get('started_after')).toBe('2026-01-01T00:00:00.000Z');
});
it('rejects incomplete dates without network or clearing the active list', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(page)); render(<AgentInvocations {...props}/>); await screen.findByRole('listitem');
  fireEvent.click(screen.getByRole('button', { name: '应用日期筛选' })); await screen.findByText(/日期无效/); expect(fetch).toHaveBeenCalledTimes(1); expect(screen.getByRole('listitem')).toBeTruthy();
});
it('reads safe original identities with unknown effects and no execution controls', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(page));
  render(<AgentInvocations {...props}/>);
  await screen.findByText(`调用标识：${item.invocation_id}`);
  expect(screen.getByText(/结果未知 · 未独立证明业务效果/)).toBeTruthy();
  expect(screen.getByText('未观察到返回时间（不代表仍在运行）')).toBeTruthy();
  expect(fetch.mock.calls[0][0]).toBe(`/api/v1/agent-invocations?agent_id=${item.agent_id}&limit=50`);
  expect(fetch.mock.calls[0][1]?.method).toBe('GET');
  expect(screen.queryByRole('button', { name: /执行|恢复|重试/ })).toBeNull();
});
it('uses opaque pages and resets expired pages without joining observations', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({ data: { ...page.data, next_cursor: 'next+call' } })).mockResolvedValueOnce(json({}, 410)).mockResolvedValueOnce(json(page));
  render(<AgentInvocations {...props}/>);
  fireEvent.click(await screen.findByRole('button', { name: '更多调用记录' }));
  await screen.findByText(/旧分页已清除/);
  expect(screen.queryByRole('listitem')).toBeNull();
  expect(fetch.mock.calls[1][0]).toContain('cursor=next%2Bcall');
  fireEvent.click(screen.getByRole('button', { name: '重读调用记录' }));
  await screen.findByRole('listitem'); expect(fetch.mock.calls[2][0]).not.toContain('cursor=');
});
it.each([401, 403])('clears records and notifies parent on %s', async status => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json({}, status));
  render(<AgentInvocations {...props}/>); await screen.findByRole('listitem');
  fireEvent.click(screen.getByRole('button', { name: '重读调用记录' }));
  await screen.findByText(/权限或范围已变化/);
  expect(screen.queryByRole('listitem')).toBeNull(); expect(props.onDenied).toHaveBeenCalledTimes(1);
});
it.each(['drivebridge-context', 'offline'])('discards late %s responses without online auto-reload', async event => {
  let finish!: (value: Response) => void;
  const fetch = vi.spyOn(globalThis, 'fetch').mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  render(<AgentInvocations {...props}/>); fireEvent(window, new Event(event)); finish(json(page));
  await waitFor(() => expect(screen.queryByRole('listitem')).toBeNull());
  fireEvent(window, new Event('online')); expect(fetch).toHaveBeenCalledTimes(1);
});
it('remounts on scope change and ignores the old request', async () => {
  let finish!: (value: Response) => void;
  vi.spyOn(globalThis, 'fetch').mockImplementationOnce(() => new Promise(resolve => { finish = resolve; })).mockResolvedValueOnce(json({ data: { ...page.data, items: [] } }));
  const view = render(<AgentInvocations {...props}/>); view.rerender(<AgentInvocations {...props} scope="other"/>);
  await screen.findByText(/当前筛选没有可见观察记录/); finish(json(page));
  await waitFor(() => expect(screen.queryByRole('listitem')).toBeNull());
});
it.each([
  { ...page.data, coverage: 'complete' },
  { ...page.data, items: [{ ...item, agent_id: 'other' }] },
  { ...page.data, items: [item, item] },
  { ...page.data, items: [{ ...item, finished_at: '2020-01-01T00:00:00Z' }] },
  { ...page.data, items: [{ ...item, operation_ref: 'unproven' }] },
])('rejects mismatched or unsupported observation contracts', async data => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({ data })); render(<AgentInvocations {...props}/>);
  await screen.findByText(/合同不匹配/); expect(screen.queryByRole('listitem')).toBeNull();
});
it('filters on the server and never treats an empty observation as no calls', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json({ data: { ...page.data, items: [] } }));
  render(<AgentInvocations {...props}/>); await screen.findByRole('listitem');
  fireEvent.click(screen.getByRole('combobox', { name: '调用结果筛选' }));
  const option = await screen.findByRole('option', { name: '请求被拒绝' });
  fireEvent.pointerDown(option, { pointerType: 'mouse' }); fireEvent.click(option);
  await screen.findByText(/当前筛选没有可见观察记录，不代表没有发生调用/);
  expect(fetch.mock.calls[1][0]).toContain('result=denied'); expect(fetch.mock.calls.every(([, options]) => options?.method === 'GET')).toBe(true);
});
