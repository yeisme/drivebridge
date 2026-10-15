import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';
import { CollectionsPage } from './CollectionsPage';
import { useRouteLeaveGuard } from '../../app/route-leave';
import { workspaceRoute } from '../../workspace-route';
import { visibleNavGroups } from '../../nav-groups';
const row = { collection_id: 'collection', space_id: 'space', name: 'References', kind: 'manual', scope: 'private', revision: 1, members_total: 999 };
const props = { scope: 'alice', spaces: [{ id: 'space', name: 'Workspace', connection_id: 'platform', used_bytes: 0, reserved_bytes: 0, max_bytes: 1000 }] };
const json = (data: unknown, status = 200) => new Response(JSON.stringify({ data }), { status });
afterEach(() => { cleanup(); vi.restoreAllMocks(); });
it('exposes collections in formal navigation and does not read members or body automatically', async () => {
  expect(workspaceRoute('#/collections').page).toBe('collections'); expect(visibleNavGroups(['collections'], {})[0].id).toBe('workbench');
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json([row])); render(<CollectionsPage {...props}/>);
  await screen.findByRole('button', { name: '查看集合 References' }); expect(fetch).toHaveBeenCalledTimes(1);
  expect(fetch.mock.calls[0][0]).toBe('/api/v1/file-collections?space_id=space'); expect(screen.queryByText(/999/)).toBeNull();
});
it('confirms list refresh and space selection before unmounting unsent creation', async () => {
  const user = userEvent.setup();
  const fetch = vi.spyOn(globalThis, 'fetch').mockImplementation(async () => json([]));
  const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
  render(<CollectionsPage {...props} spaces={[...props.spaces, { ...props.spaces[0], id: 'other', name: 'Other space' }]}/>);
  fireEvent.click(await screen.findByRole('button', { name: '新建私有手动集合' }));
  const input = await screen.findByRole('textbox', { name: '集合名称' });
  fireEvent.change(input, { target: { value: 'unsent' } });
  // The modal makes background controls inert for humans; invoke the page
  // callback in this component test to cover programmatic/remount callers.
  fireEvent.click(screen.getByRole('button', { name: '刷新集合列表', hidden: true }));
  expect(confirm).toHaveBeenCalledTimes(1); expect(fetch).toHaveBeenCalledTimes(1);
  expect((input as HTMLInputElement).value).toBe('unsent');
  confirm.mockReturnValue(true);
  fireEvent.click(screen.getByRole('button', { name: '刷新集合列表', hidden: true }));
  await waitFor(() => expect(screen.queryByRole('textbox', { name: '集合名称' })).toBeNull());
  expect(fetch).toHaveBeenCalledTimes(2);
  // A clean space change needs no confirmation.
  confirm.mockClear();
  await user.click(screen.getByRole('combobox', { name: '集合空间' }));
  await user.click(screen.getByRole('option', { name: 'Other space' }));
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(3));
  expect(fetch.mock.calls[2][0]).toBe('/api/v1/file-collections?space_id=other');
  expect(confirm).not.toHaveBeenCalled();
});

it('keeps the original space and issues no request when the shared dirty guard rejects selection', async () => {
  const user = userEvent.setup();
  const fetch = vi.spyOn(globalThis, 'fetch').mockImplementation(async () => json([]));
  const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
  function ActiveDraft() { useRouteLeaveGuard(true, 'draft'); return null; }
  render(<><ActiveDraft/><CollectionsPage {...props} spaces={[...props.spaces, { ...props.spaces[0], id: 'other', name: 'Other space' }]}/></>);
  await user.click(screen.getByRole('combobox', { name: '集合空间' }));
  await user.click(screen.getByRole('option', { name: 'Other space' }));
  expect(confirm).toHaveBeenCalledTimes(1); expect(fetch).toHaveBeenCalledTimes(1);
  expect((screen.getByRole('combobox', { name: '集合空间' }) as HTMLInputElement).value).toBe('Workspace');
  confirm.mockReturnValue(true);
  await user.click(screen.getByRole('combobox', { name: '集合空间' }));
  await user.click(screen.getByRole('option', { name: 'Other space' }));
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
  expect(fetch.mock.calls[1][0]).toBe('/api/v1/file-collections?space_id=other');
});

it('opens authorized dynamic members only on request and restores focus', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json([row])).mockResolvedValueOnce(json({ collection_id: 'collection', kind: 'manual', revision: 2, computed_at: '2026-10-06T00:00:00Z', visible_count: 0, coverage: 'complete', items: [] }));
  render(<CollectionsPage {...props}/>); const trigger = await screen.findByRole('button', { name: '查看集合 References' }); fireEvent.click(trigger);
  await screen.findByText(/当前授权下没有可见成员/); expect(screen.getByText(/集合已在列表观察后变化/)).toBeTruthy();
  expect(fetch).toHaveBeenCalledTimes(2); fireEvent.click(screen.getByRole('button', { name: '关闭集合成员' })); expect(document.activeElement).toBe(trigger);
});
it('locally expands list without pretending to paginate the server', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(Array.from({ length: 51 }, (_, i) => ({ ...row, collection_id: 'c' + i, name: 'Collection ' + i }))));
  render(<CollectionsPage {...props}/>); await screen.findByRole('button', { name: '展开更多已读取集合' }); expect(screen.getAllByRole('listitem')).toHaveLength(50);
  fireEvent.click(screen.getByRole('button', { name: '展开更多已读取集合' })); expect(screen.getAllByRole('listitem')).toHaveLength(51); expect(fetch).toHaveBeenCalledTimes(1);
});
it.each([{ data: [row, row] }, { data: [{ ...row, space_id: 'foreign' }] }, { data: [{ ...row, kind: 'unknown' }] }, { data: [{ ...row, revision: 0 }] }, { data: Array(1001).fill(row) }])('rejects invalid list contract', async ({ data }) => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(data)); render(<CollectionsPage {...props}/>); await screen.findByText(/合同不匹配/); expect(screen.queryByRole('listitem')).toBeNull();
});
it.each(['offline', 'drivebridge-context'])('ignores late data and online after %s', async event => {
  let finish!: (value: Response) => void; const fetch = vi.spyOn(globalThis, 'fetch').mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  render(<CollectionsPage {...props}/>); fireEvent(window, new Event(event)); await act(async () => { finish(json([row])); });
  expect(screen.queryByRole('listitem')).toBeNull(); fireEvent(window, new Event('online')); expect(fetch).toHaveBeenCalledTimes(1);
});
it('member revocation clears list and blocks refresh', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json([row])).mockResolvedValueOnce(json({}, 403)); render(<CollectionsPage {...props}/>);
  fireEvent.click(await screen.findByRole('button', { name: '查看集合 References' })); await screen.findByText(/集合及正文已清除/); expect(screen.queryByRole('listitem')).toBeNull(); expect(screen.getByRole('button', { name: '刷新集合列表' })).toHaveProperty('disabled', true);
});
it('scope remount does not retain earlier collections', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json([row])).mockResolvedValueOnce(json([])); const view = render(<CollectionsPage {...props}/>); await screen.findByRole('listitem');
  view.rerender(<CollectionsPage {...props} scope="bob"/>); await waitFor(() => expect(screen.queryByRole('listitem')).toBeNull()); expect(fetch).toHaveBeenCalledTimes(2);
});
