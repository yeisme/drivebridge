import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import fixtures from '../../../../contracts/personal-workspace/fixtures/responses.json';
import { ResourcesPage } from './ResourcesPage';
import { workspaceRoute } from '../../workspace-route';
import { visibleNavGroups } from '../../nav-groups';
const page = fixtures.ResourcePageEnvelope, row = page.data.items[0];
const json = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status });
const props = { scope: 'alice', spaces: [] };
afterEach(() => { cleanup(); vi.restoreAllMocks(); });
it.each([200, 503])('restores registration focus after list refresh, including failure: %s', async status => {
  let finish!: (response: Response) => void;
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(page)).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  render(<ResourcesPage {...props}/>);
  const trigger = await screen.findByRole('button', { name: '登记固定资源' }); trigger.focus(); fireEvent.click(trigger);
  await screen.findByRole('textbox', { name: '资源名称' }); fireEvent.click(screen.getByRole('button', { name: '关闭' }));
  await waitFor(() => expect(finish).toBeDefined()); finish(json(status === 200 ? page : {}, status));
  const restored = await screen.findByRole('button', { name: status === 200 ? '登记固定资源' : '刷新资源库' });
  await waitFor(() => expect(document.activeElement).toBe(restored));
  expect(fetch).toHaveBeenCalledTimes(2);
});
it('does not steal a newer focus selection when registration refresh finishes late', async () => {
  let finish!: (response: Response) => void;
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(page)).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  render(<ResourcesPage {...props}/>);
  const trigger = await screen.findByRole('button', { name: '登记固定资源' }); trigger.focus(); fireEvent.click(trigger);
  await screen.findByRole('textbox', { name: '资源名称' }); fireEvent.click(screen.getByRole('button', { name: '关闭' }));
  await waitFor(() => expect(finish).toBeDefined());
  const selector = screen.getByRole('combobox', { name: '资源类型' }); selector.focus(); finish(json(page));
  await screen.findByRole('button', { name: '登记固定资源' }); expect(document.activeElement).toBe(selector);
});
it('exposes the canonical route in the workbench group', () => {
  expect(workspaceRoute('#/resources').page).toBe('resources');
  expect(visibleNavGroups(['resources'], {})[0]).toMatchObject({ id: 'workbench', items: ['resources'] });
});
it('reads unknown-check resources and explicitly revalidates details without executing', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json({ data: row }));
  render(<ResourcesPage {...props}/>); const button = await screen.findByRole('button', { name: '查看资源描述 Synthetic resource' });
  expect(fetch).toHaveBeenCalledTimes(1); fireEvent.click(button);
  await screen.findByText(row.file_ref);
  expect(screen.getByText('检查状态：未检查，不代表内容安全、可执行或可安装。')).toBeTruthy();
  expect(fetch.mock.calls.every(([, options]) => options?.method === 'GET')).toBe(true);
  fireEvent.click(screen.getByRole('button', { name: '关闭资源描述' })); expect(document.activeElement).toBe(button);
});
it('keeps snapshots distinct from single-file targets', async () => {
  const pack = { ...row, kind: 'reference_pack', file_ref: undefined, collection_snapshot_id: 'snapshot', display_name: 'Pack' };
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({ data: { ...page.data, items: [pack] } })).mockResolvedValueOnce(json({ data: pack }));
  render(<ResourcesPage {...props}/>); fireEvent.click(await screen.findByRole('button', { name: '查看资源描述 Pack' }));
  await screen.findByText('集合快照：snapshot'); expect(screen.queryByRole('button', { name: /正文|安装|执行/ })).toBeNull();
});
it('clears expired pages and explicitly restarts', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({ data: { ...page.data, next_cursor: 'next+resource' } })).mockResolvedValueOnce(json({}, 410)).mockResolvedValueOnce(json(page));
  render(<ResourcesPage {...props}/>); fireEvent.click(await screen.findByRole('button', { name: '更多资源描述' }));
  await screen.findByText(/资源分页已过期/); expect(screen.queryByRole('listitem')).toBeNull();
  fireEvent.click(screen.getByRole('button', { name: '刷新资源库' })); await screen.findByRole('listitem'); expect(fetch.mock.calls[2][0]).toBe('/api/v1/resource-descriptors?limit=50');
});
it('detail identity drift never substitutes the fixed target', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json({ data: { ...row, file_ref: 'other' } }));
  render(<ResourcesPage {...props}/>); fireEvent.click(await screen.findByRole('button', { name: '查看资源描述 Synthetic resource' }));
  await screen.findByText(/未替换目标/); expect(screen.queryByText('other')).toBeNull();
});
it('detail 403 clears the parent list', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json({}, 403));
  render(<ResourcesPage {...props}/>); fireEvent.click(await screen.findByRole('button', { name: '查看资源描述 Synthetic resource' }));
  await screen.findByText(/资源列表与详情已清除/); expect(screen.queryByRole('listitem')).toBeNull();
});
it.each(['offline', 'drivebridge-context'])('drops late list on %s and does not auto reload', async event => {
  let finish!: (value: Response) => void;
  const fetch = vi.spyOn(globalThis, 'fetch').mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  render(<ResourcesPage {...props}/>); fireEvent(window, new Event(event)); finish(json(page));
  await waitFor(() => expect(screen.queryByRole('listitem')).toBeNull()); fireEvent(window, new Event('online')); expect(fetch).toHaveBeenCalledTimes(1);
});
it('uses kind filtering on the server and rejects mismatched returned kinds', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json(page));
  render(<ResourcesPage {...props}/>); await screen.findByRole('listitem');
  fireEvent.click(screen.getByRole('combobox', { name: '资源类型' })); const option = await screen.findByRole('option', { name: '模板' }); fireEvent.pointerDown(option, { pointerType: 'mouse' }); fireEvent.click(option);
  await screen.findByText(/合同不匹配/); expect(fetch.mock.calls[1][0]).toContain('kind=template'); expect(screen.queryByRole('listitem')).toBeNull();
});
