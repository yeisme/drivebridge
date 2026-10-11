import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { CollectionMembersDialog } from './CollectionMembersDialog';
import { confirmRouteLeave } from '../../app/route-leave';
import { claimMembers, clearMembers, memberDigest, memberSlot, readMemberRecovery, type MemberChange } from './member-recovery';
const id = 'c'.repeat(48), a = 'a'.repeat(48), b = 'b'.repeat(48), next = 'd'.repeat(48);
const base = { collection_id: id, revision: 2, items: [{ file_id: a, space_id: 's', path: 'keep.txt', decision: 'include' }, { file_id: b, space_id: 's', path: 'excluded.txt', decision: 'exclude' }] };
const input: MemberChange = { expected_revision: 2, include: [a, b], exclude: [] };
const slot = memberSlot('scope', id), path = '/api/v1/file-collections/' + id + '/members';
const json = (data: unknown, status = 200) => new Response(JSON.stringify({ data }), { status });
const receipt = async () => ({ collection_id: id, request_digest: await memberDigest(id, input), result_revision: 3, committed_at: '2026-01-01T00:00:00Z' });
const locks = Object.getOwnPropertyDescriptor(navigator, 'locks');
beforeEach(() => Object.defineProperty(navigator, 'locks', { configurable: true, value: { request: vi.fn(async (_name: string, options: LockOptions | (() => unknown), callback?: () => unknown) => typeof options === 'function' ? options() : callback!()) } }));
afterEach(() => { cleanup(); localStorage.clear(); vi.restoreAllMocks(); if (locks) Object.defineProperty(navigator, 'locks', locks); else Reflect.deleteProperty(navigator, 'locks'); });
const mount = (close = vi.fn(), denied = vi.fn()) => render(<CollectionMembersDialog scope="scope" id={id} space="s" close={close} denied={denied}/>);
async function edit() {
  fireEvent.click(await screen.findByRole('button', { name: '读取完整成员决策' }));
  fireEvent.click(await screen.findByRole('button', { name: '改为包含 excluded.txt' }));
  fireEvent.click(screen.getByRole('button', { name: '预览成员修改' }));
  await screen.findByRole('button', { name: '确认保存成员决策' });
}
const unloadPrevented = () => { const event = new Event('beforeunload', { cancelable: true }); window.dispatchEvent(event); return event.defaultPrevented; };
it('protects only unsent member differences on reload and releases the listener on unmount', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(base)); const view = mount();
  expect(unloadPrevented()).toBe(false);
  fireEvent.click(await screen.findByRole('button', { name: '读取完整成员决策' }));
  const change = await screen.findByRole('button', { name: '改为包含 excluded.txt' });
  const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
  expect(confirmRouteLeave()).toBe(true);
  expect(unloadPrevented()).toBe(false);
  fireEvent.click(change); expect(unloadPrevented()).toBe(true);
  expect(confirmRouteLeave()).toBe(false);
  expect(confirm).toHaveBeenCalledWith('放弃未提交的集合成员草稿并离开？');
  fireEvent.click(screen.getByRole('button', { name: '改为排除 excluded.txt' })); expect(unloadPrevented()).toBe(false);
  fireEvent.click(screen.getByRole('button', { name: '改为包含 excluded.txt' }));
  fireEvent.click(screen.getByRole('button', { name: '预览成员修改' })); expect(unloadPrevented()).toBe(true);
  expect(localStorage.length).toBe(0);
  view.unmount(); expect(unloadPrevented()).toBe(false); expect(confirmRouteLeave()).toBe(true);
});
it('does not treat an original-key unknown outcome as an unsent draft', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(base)).mockRejectedValueOnce(new TypeError('network'));
  mount(); await edit(); expect(unloadPrevented()).toBe(true);
  fireEvent.click(screen.getByRole('button', { name: '确认保存成员决策' }));
  await screen.findByRole('button', { name: '查询原成员回执' });
  expect(unloadPrevented()).toBe(false); expect(readMemberRecovery(slot)).not.toBeNull();
  const confirm = vi.spyOn(window, 'confirm'); expect(confirmRouteLeave()).toBe(true); expect(confirm).not.toHaveBeenCalled();
});
it.each(['offline', 'drivebridge-context'])('removes unload protection after %s clears the draft', async event => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(base)); mount(); await edit(); expect(unloadPrevented()).toBe(true);
  fireEvent(window, new Event(event)); await screen.findByText(/成员草稿已清除/); expect(unloadPrevented()).toBe(false);
});
it('loads complete decisions explicitly, previews before mutation, validates original receipt and clears', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(base)).mockResolvedValueOnce(json({})).mockResolvedValueOnce(json(await receipt()));
  const close = vi.fn(); mount(close); expect(fetch).not.toHaveBeenCalled(); await edit(); expect(fetch).toHaveBeenCalledTimes(1); expect(localStorage.length).toBe(0);
  expect(screen.getByText('excluded.txt：排除 → 包含')).toBeTruthy();
  fireEvent.click(screen.getByRole('button', { name: '确认保存成员决策' })); await screen.findByText('已确认原成员修改');
  expect(JSON.parse(fetch.mock.calls[1][1]?.body as string)).toEqual(input);
  expect(fetch.mock.calls.map(([url]) => url)).toEqual([path + '/decisions', path + '/changes', path + '/changes/lookup']);
  const stored = localStorage.getItem(slot)!; expect(JSON.parse(stored)).toEqual({ key: expect.stringMatching(/^members-[a-f0-9]{48}$/), digest: await memberDigest(id, input) }); expect(stored).not.toContain('excluded.txt'); expect(stored).not.toContain(a);
  fireEvent.click(screen.getByRole('button', { name: '确认成员回执并关闭' })); await waitFor(() => expect(close).toHaveBeenCalledOnce()); expect(readMemberRecovery(slot)).toBeNull();
});
it('remount with lost response only looks up original request, without reloading base', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(base)).mockRejectedValueOnce(new TypeError('network')).mockResolvedValueOnce(json(await receipt()));
  const view = mount(); await edit(); fireEvent.click(screen.getByRole('button', { name: '确认保存成员决策' })); await screen.findByRole('button', { name: '查询原成员回执' });
  const original = readMemberRecovery(slot)!; view.unmount(); mount(); await screen.findByRole('button', { name: '查询原成员回执' }); expect(fetch).toHaveBeenCalledTimes(2);
  fireEvent.click(screen.getByRole('button', { name: '查询原成员回执' })); await screen.findByText('已确认原成员修改'); expect(JSON.parse(fetch.mock.calls[2][1]?.body as string)).toEqual({ idempotency_key: original.key });
});
it('404 permits only human same-key same-body retry while original remains in memory', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(base)).mockRejectedValueOnce(new TypeError('network')).mockResolvedValueOnce(json({}, 404)).mockResolvedValueOnce(json({})).mockResolvedValueOnce(json(await receipt()));
  mount(); await edit(); fireEvent.click(screen.getByRole('button', { name: '确认保存成员决策' })); fireEvent.click(await screen.findByRole('button', { name: '查询原成员回执' })); fireEvent.click(await screen.findByRole('button', { name: '确认按原键重试成员修改' })); await screen.findByText('已确认原成员修改');
  expect(fetch.mock.calls[3][1]?.body).toBe(fetch.mock.calls[1][1]?.body); expect(fetch.mock.calls[3][1]?.headers).toEqual(fetch.mock.calls[1][1]?.headers);
});
it('revision conflict never updates revision or offers replay after missing lookup', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(base)).mockResolvedValueOnce(json({}, 409)).mockResolvedValueOnce(json({}, 404)); mount(); await edit(); fireEvent.click(screen.getByRole('button', { name: '确认保存成员决策' })); await screen.findByText(/没有覆盖服务器/); fireEvent.click(screen.getByRole('button', { name: '查询原成员回执' })); await screen.findByText(/未取得匹配/); expect(screen.queryByRole('button', { name: '确认按原键重试成员修改' })).toBeNull(); expect(fetch).toHaveBeenCalledTimes(3); expect(readMemberRecovery(slot)).not.toBeNull();
});
it.each([404, 500])('unavailable complete base %s cannot become a partial editable form', async status => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(json({}, status)); mount(); fireEvent.click(await screen.findByRole('button', { name: '读取完整成员决策' })); await screen.findByText(/没有部分表单可保存/); expect(screen.queryByRole('button', { name: '预览成员修改' })).toBeNull(); expect(localStorage.length).toBe(0);
});
it.each([{ ...base, items: [...base.items, base.items[0]] }, { ...base, revision: 0 }, { ...base, collection_id: 'other' }])('rejects invalid edit base', async invalid => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(invalid)); mount(); fireEvent.click(await screen.findByRole('button', { name: '读取完整成员决策' })); await screen.findByText(/没有部分表单可保存/);
});
it('metadata picker pages explicitly, preserving excluded decisions in new request', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(base)).mockResolvedValueOnce(json({ mode: 'exact', items: [{ file_id: next, version_id: a, path: 'new.txt' }], coverage: 'partial', next_cursor: 'next-page' })).mockResolvedValueOnce(json({ mode: 'exact', items: [], coverage: 'complete' }));
  mount(); fireEvent.click(await screen.findByRole('button', { name: '读取完整成员决策' })); await screen.findByRole('button', { name: '改为包含 excluded.txt' });
  fireEvent.change(screen.getByRole('textbox', { name: '成员路径关键词' }), { target: { value: 'new' } }); fireEvent.click(screen.getByRole('button', { name: '搜索可选文件' }));
  fireEvent.click(await screen.findByRole('button', { name: '添加包含 new.txt' })); fireEvent.click(screen.getByRole('button', { name: '下一页可选文件' })); await screen.findByText('本次搜索页没有可选文件。');
  expect(JSON.parse(fetch.mock.calls[2][1]?.body as string).cursor).toBe('next-page'); expect(fetch.mock.calls.every(([url]) => !String(url).includes('/content'))).toBe(true);
  fireEvent.click(screen.getByRole('button', { name: '预览成员修改' })); expect(screen.getByText('new.txt：无显式决策 → 包含')).toBeTruthy(); expect(screen.queryByText(/excluded.txt：/)).toBeNull();
});
it.each(['offline', 'drivebridge-context', 'storage'])('rejects late base after %s', async event => {
  let finish!: (response: Response) => void; vi.spyOn(globalThis, 'fetch').mockImplementation(() => new Promise(resolve => { finish = resolve; })); mount(); fireEvent.click(await screen.findByRole('button', { name: '读取完整成员决策' })); await waitFor(() => expect(finish).toBeDefined()); fireEvent(window, event === 'storage' ? new StorageEvent('storage', { key: slot }) : new Event(event)); finish(json(base)); await screen.findByText(/成员草稿已清除/); expect(screen.queryByText(/keep.txt/)).toBeNull();
});
it.each([{ result_revision: 4 }, { request_digest: 'f'.repeat(64) }, { collection_id: 'other' }, { items: [] }])('rejects mismatched original receipt %j', async change => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(base)).mockResolvedValueOnce(json({})).mockResolvedValueOnce(json({ ...await receipt(), ...change })); mount(); await edit(); fireEvent.click(screen.getByRole('button', { name: '确认保存成员决策' })); await screen.findByRole('button', { name: '查询原成员回执' }); expect(screen.queryByText('已确认原成员修改')).toBeNull(); expect(readMemberRecovery(slot)).not.toBeNull();
});
it('guards missing locks, another claim and CAS cleanup', async () => {
  const first = await claimMembers(slot, 'a'.repeat(64), new AbortController().signal), second = await claimMembers(slot, 'b'.repeat(64), new AbortController().signal); expect(second.claimed).toBe(false); expect(second.record).toEqual(first.record);
  localStorage.setItem(slot, JSON.stringify({ key: 'members-' + 'c'.repeat(48), digest: 'c'.repeat(64) })); await expect(clearMembers(slot, first.record)).rejects.toThrow(); localStorage.clear();
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(base)); Object.defineProperty(navigator, 'locks', { configurable: true, value: undefined }); mount(); await edit(); fireEvent.click(screen.getByRole('button', { name: '确认保存成员决策' })); await screen.findByText(/跨标签锁不可用/); expect(fetch).toHaveBeenCalledTimes(1);
});
it('cross-space picker clears old cursor and preserves both same-path identities', async () => {
  const local = { ...base, items: [{ file_id: a, space_id: 's', path: 'same.txt', decision: 'include' }] };
  const change: MemberChange = { expected_revision: 2, include: [a, next], exclude: [] };
  const result = { ...await receipt(), request_digest: await memberDigest(id, change) };
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(local)).mockResolvedValueOnce(json({ mode: 'exact', items: [{ file_id: b, version_id: a, path: 'local-only.txt' }], coverage: 'partial', next_cursor: 'local-cursor' })).mockResolvedValueOnce(json({ mode: 'exact', items: [{ file_id: next, version_id: b, path: 'same.txt' }], coverage: 'complete' })).mockResolvedValueOnce(json({})).mockResolvedValueOnce(json(result));
  render(<CollectionMembersDialog scope="scope" id={id} space="s" spaces={[{id:'s',name:'Local'}, {id:'other',name:'Other space'}]} close={vi.fn()} denied={vi.fn()}/>);
  fireEvent.click(await screen.findByRole('button', { name: '读取完整成员决策' }));
  fireEvent.change(await screen.findByRole('textbox', { name: '成员路径关键词' }), {target:{value:'same'}}); fireEvent.click(screen.getByRole('button',{name:'搜索可选文件'}));
  await screen.findByRole('button',{name:'下一页可选文件'});
  screen.getByRole('combobox',{name:'成员来源空间'}).focus(); fireEvent.keyDown(screen.getByRole('combobox',{name:'成员来源空间'}), {key:'ArrowDown'}); fireEvent.change(screen.getByRole('combobox',{name:'成员来源空间'}), {target:{value:'Other'}}); const option = await screen.findByRole('option',{name:'Other space'}); fireEvent.pointerDown(option,{pointerType:'mouse'}); fireEvent.click(option);
  await waitFor(() => expect(screen.queryByRole('button',{name:'下一页可选文件'})).toBeNull()); expect(screen.queryByText('local-only.txt')).toBeNull(); expect(screen.getByRole('button',{name:'移除决策 same.txt'})).toBeTruthy();
  fireEvent.click(screen.getByRole('button',{name:'搜索可选文件'})); fireEvent.click(await screen.findByRole('button',{name:'添加包含 same.txt'}));
  expect(JSON.parse(fetch.mock.calls[2][1]?.body as string).space_ids).toEqual(['other']); expect(JSON.parse(fetch.mock.calls[2][1]?.body as string).cursor).toBeUndefined();
  expect(screen.getAllByRole('button',{name:'移除决策 same.txt'})).toHaveLength(2);
  fireEvent.click(screen.getByRole('button',{name:'预览成员修改'})); expect(screen.getByText('来源空间：Other space · other')).toBeTruthy();
  fireEvent.click(screen.getByRole('button',{name:'确认保存成员决策'})); await screen.findByText('已确认原成员修改'); expect(JSON.parse(fetch.mock.calls[3][1]?.body as string)).toEqual(change);
});
it('removing an available source invalidates pending search and clears editable baseline', async () => {
  let finish!: (response: Response) => void;
  const fetch = vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(base)).mockImplementation(() => new Promise(resolve => {finish=resolve;}));
  const props = {scope:'scope',id,space:'s',close:vi.fn(),denied:vi.fn()};
  const view = render(<CollectionMembersDialog {...props} spaces={[{id:'s',name:'Local'},{id:'other',name:'Other'}]}/>);
  fireEvent.click(await screen.findByRole('button',{name:'读取完整成员决策'})); fireEvent.change(await screen.findByRole('textbox',{name:'成员路径关键词'}),{target:{value:'late'}}); fireEvent.click(screen.getByRole('button',{name:'搜索可选文件'})); await waitFor(() => expect(finish).toBeDefined());
  view.rerender(<CollectionMembersDialog {...props} spaces={[{id:'s',name:'Local'}]}/>); finish(json({mode:'exact',items:[{file_id:next,version_id:a,path:'late.txt'}],coverage:'complete'}));
  await screen.findByRole('button',{name:'读取完整成员决策'}); expect(screen.queryByText('late.txt')).toBeNull(); expect(screen.queryByRole('button',{name:'预览成员修改'})).toBeNull(); expect(fetch).toHaveBeenCalledTimes(2);
});
it('asks before discarding unsent changes and propagates denied read', async () => {
  const close = vi.fn(), denied = vi.fn(); vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(base)).mockResolvedValueOnce(json({}, 403)); const view = mount(close); await edit(); fireEvent.click(screen.getByRole('button', { name: '关闭' })); await screen.findByText(/关闭将丢弃/); expect(close).not.toHaveBeenCalled(); fireEvent.click(screen.getByRole('button', { name: '丢弃草稿并关闭' })); expect(close).toHaveBeenCalledOnce(); view.unmount(); mount(vi.fn(), denied); fireEvent.click(await screen.findByRole('button', { name: '读取完整成员决策' })); await waitFor(() => expect(denied).toHaveBeenCalledOnce());
});
