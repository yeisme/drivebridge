import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { PropertiesPage } from './PropertiesPage';
import { PropertyEditor } from './PropertyEditor';
import { confirmRouteLeave, useRouteLeaveGuard } from '../../app/route-leave';
import userEvent from '@testing-library/user-event';
import { claimUpdate, clearUpdate, propertyDigest, propertySlot, readRecovery, TYPES, validDefinition, type Update } from './property-recovery';
const space = { id: 's', name: 'Space', connection_id: 'platform', used_bytes: 0, reserved_bytes: 0, max_bytes: 100 };
const definition = { key: 'status', display_name: 'Status', group: '', type: 'text' as const };
const base = { space_id: 's', revision: 1, definitions: [definition] };
const input: Update = { expected_revision: 1, definitions: [{ ...definition, display_name: 'Progress' }] };
const slot = propertySlot('scope', 's'), path = '/api/v1/spaces/s/property-definitions';
const json = (data: unknown, status = 200) => new Response(JSON.stringify({ data }), { status });
const receipt = async () => ({ space_id: 's', request_digest: await propertyDigest('s', input), result_revision: 2, committed_at: '2026-01-01T00:00:00Z' });
const locks = Object.getOwnPropertyDescriptor(navigator, 'locks');
beforeEach(() => Object.defineProperty(navigator, 'locks', { configurable: true, value: { request: vi.fn(async (_name: string, options: LockOptions | (() => unknown), callback?: () => unknown) => typeof options === 'function' ? options() : callback!()) } }));
afterEach(() => { cleanup(); localStorage.clear(); vi.restoreAllMocks(); if (locks) Object.defineProperty(navigator, 'locks', locks); else Reflect.deleteProperty(navigator, 'locks'); });
const editor = (close = vi.fn(), denied = vi.fn()) => render(<PropertyEditor scope="scope" space="s" close={close} denied={denied}/>);
async function edit() {
  fireEvent.click(await screen.findByRole('button', { name: '读取完整属性字典' }));
  fireEvent.change(await screen.findByRole('textbox', { name: '字段 1 显示名' }), { target: { value: 'Progress' } });
  fireEvent.click(screen.getByRole('button', { name: '预览字典修改' })); await screen.findByRole('button', { name: '确认保存属性字典' });
}
it('protects only unsent dictionary changes and releases on original-key unknown', async () => {
  const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(base)).mockRejectedValueOnce(new TypeError('network'));
  const view = editor(); expect(confirmRouteLeave()).toBe(true);
  fireEvent.click(await screen.findByRole('button', { name: '读取完整属性字典' }));
  const name = await screen.findByRole('textbox', { name: '字段 1 显示名' });
  expect(confirmRouteLeave()).toBe(true);
  fireEvent.change(name, { target: { value: 'Progress' } }); expect(confirmRouteLeave()).toBe(false);
  fireEvent.change(name, { target: { value: 'Status' } }); expect(confirmRouteLeave()).toBe(true);
  fireEvent.click(screen.getByRole('button', { name: '添加属性字段' })); expect(confirmRouteLeave()).toBe(false);
  fireEvent.click(screen.getByRole('button', { name: '移除新增字段 2' })); expect(confirmRouteLeave()).toBe(true);
  fireEvent.change(name, { target: { value: 'Progress' } });
  fireEvent.click(screen.getByRole('button', { name: '预览字典修改' })); expect(confirmRouteLeave()).toBe(false);
  expect(fetch).toHaveBeenCalledTimes(1); expect(localStorage.length).toBe(0);
  fireEvent.click(screen.getByRole('button', { name: '确认保存属性字典' }));
  await screen.findByRole('button', { name: '查询原字典回执' });
  confirm.mockClear(); expect(confirmRouteLeave()).toBe(true); expect(confirm).not.toHaveBeenCalled();
  expect(readRecovery(slot)).not.toBeNull(); view.unmount(); expect(confirmRouteLeave()).toBe(true);
});

it.each(['offline', 'drivebridge-context'])('does not let dirty guard block forced %s', async event => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(base)); vi.spyOn(window, 'confirm').mockReturnValue(false);
  editor(); await edit(); expect(confirmRouteLeave()).toBe(false);
  fireEvent(window, new Event(event)); await screen.findByText(/字典草稿已清除/); expect(confirmRouteLeave()).toBe(true);
});

it('confirms refresh before discarding a mounted dictionary draft', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockImplementation(async () => json(base));
  const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
  render(<PropertiesPage scope="scope" spaces={[space]}/>);
  await screen.findByText('字典修订 1 · 1 个定义（最多 200 个）');
  fireEvent.click(screen.getByRole('button', { name: '编辑属性字典' })); await edit();
  fireEvent.click(screen.getByRole('button', { name: '刷新属性字典', hidden: true }));
  expect(confirm).toHaveBeenCalledOnce(); expect(fetch).toHaveBeenCalledTimes(2);
  expect(screen.getByRole('button', { name: '确认保存属性字典' })).toBeTruthy();
  confirm.mockReturnValue(true);
  fireEvent.click(screen.getByRole('button', { name: '刷新属性字典', hidden: true }));
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull()); expect(fetch).toHaveBeenCalledTimes(3);
});

it('rejects active space selection before changing the controlled value or fetching', async () => {
  const user = userEvent.setup(); const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
  const fetch = vi.spyOn(globalThis, 'fetch').mockImplementation(async url => json({ ...base, space_id: String(url).includes('/other/') ? 'other' : 's' }));
  function ActiveDraft() { useRouteLeaveGuard(true, 'unsent'); return null; }
  render(<><ActiveDraft/><PropertiesPage scope="scope" spaces={[space, { ...space, id: 'other', name: 'Other space' }]}/></>);
  await screen.findByRole('heading', { name: 'Status' });
  await user.click(screen.getByRole('combobox', { name: '属性空间' })); await user.click(screen.getByRole('option', { name: 'Other space' }));
  expect(confirm).toHaveBeenCalledOnce(); expect(fetch).toHaveBeenCalledTimes(1);
  expect((screen.getByRole('combobox', { name: '属性空间' }) as HTMLInputElement).value).toBe('Space');
  confirm.mockReturnValue(true);
  await user.click(screen.getByRole('combobox', { name: '属性空间' })); await user.click(screen.getByRole('option', { name: 'Other space' }));
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2)); expect(fetch.mock.calls[1][0]).toBe('/api/v1/spaces/other/property-definitions');
});

it('lists authorized catalog, local expansion and preserves explicit no-capability labels', async () => {
  const definitions = Array.from({ length: 51 }, (_, i) => ({ ...definition, key: 'field_' + i, display_name: 'Field ' + i }));
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json({ ...base, definitions })); render(<PropertiesPage scope="scope" spaces={[space]}/>);
  await screen.findByRole('heading', { name: 'Field 0' }); expect(screen.queryByRole('heading', { name: 'Field 50' })).toBeNull(); fireEvent.click(screen.getByRole('button', { name: '展开更多已读取定义' })); expect(screen.getByRole('heading', { name: 'Field 50' })).toBeTruthy(); expect(fetch).toHaveBeenCalledTimes(1); expect(screen.getByText(/使用次数尚未提供/)).toBeTruthy(); fireEvent(window, new Event('offline')); await screen.findByText(/当前离线/); expect(screen.queryByRole('heading', { name: 'Field 0' })).toBeNull();
});
it.each([{ ...base, space_id: 'foreign' }, { ...base, definitions: [definition, definition] }, { ...base, revision: -1 }, { ...base, definitions: Array(201).fill(definition) }])('rejects invalid catalog without showing an empty success', async value => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(value)); render(<PropertiesPage scope="scope" spaces={[space]}/>); await screen.findByText(/不代表没有定义/); expect(screen.queryByText('当前空间尚无属性定义。')).toBeNull();
});
it('no-space and valid virtual-zero catalog remain distinct', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json({ space_id: 's', revision: 0, definitions: [] })); const view = render(<PropertiesPage scope="scope" spaces={[]}/>); expect(screen.getByText('当前范围没有可用空间。')).toBeTruthy(); expect(fetch).not.toHaveBeenCalled(); view.rerender(<PropertiesPage scope="scope" spaces={[space]}/>); await screen.findByText('当前空间尚无属性定义。');
});
it('edits display only, previews before PUT, verifies receipt and CAS clears', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(base)).mockResolvedValueOnce(json({})).mockResolvedValueOnce(json(await receipt())); const close = vi.fn(); editor(close); expect(fetch).not.toHaveBeenCalled();
  fireEvent.click(await screen.findByRole('button', { name: '读取完整属性字典' }));
  expect((await screen.findByRole('textbox', { name: '字段 1 稳定 key' }) as HTMLInputElement).disabled).toBe(true); expect(screen.getByRole('combobox', { name: '字段 1 类型' }).hasAttribute('disabled')).toBe(true); expect(screen.queryByRole('button', { name: '移除新增字段 1' })).toBeNull();
  fireEvent.change(screen.getByRole('textbox', { name: '字段 1 显示名' }), { target: { value: 'Progress' } }); fireEvent.click(screen.getByRole('button', { name: '预览字典修改' })); expect(fetch).toHaveBeenCalledTimes(1); expect(localStorage.length).toBe(0);
  fireEvent.click(screen.getByRole('button', { name: '确认保存属性字典' })); await screen.findByText('已确认原属性字典修改'); expect(fetch.mock.calls[1][1]?.method).toBe('PUT'); expect(JSON.parse(fetch.mock.calls[1][1]?.body as string)).toEqual(input);
  const stored = localStorage.getItem(slot)!; expect(JSON.parse(stored)).toEqual({ key: expect.stringMatching(/^properties-[a-f0-9]{48}$/), digest: await propertyDigest('s', input) }); expect(stored).not.toContain('Progress');
  fireEvent.click(screen.getByRole('button', { name: '确认字典回执并关闭' })); await waitFor(() => expect(close).toHaveBeenCalledOnce()); expect(readRecovery(slot)).toBeNull();
});
it('lost response remount only explicitly queries original receipt', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(base)).mockRejectedValueOnce(new TypeError('network')).mockResolvedValueOnce(json(await receipt())); const view = editor(); await edit(); fireEvent.click(screen.getByRole('button', { name: '确认保存属性字典' })); await screen.findByRole('button', { name: '查询原字典回执' }); const original = readRecovery(slot)!; view.unmount(); editor(); await screen.findByRole('button', { name: '查询原字典回执' }); expect(fetch).toHaveBeenCalledTimes(2);
  fireEvent.click(screen.getByRole('button', { name: '查询原字典回执' })); await screen.findByText('已确认原属性字典修改'); expect(JSON.parse(fetch.mock.calls[2][1]?.body as string)).toEqual({ idempotency_key: original.key }); expect(fetch.mock.calls[2][0]).toBe(path + '/lookup');
});
it('404 allows human same-key retry, conflict does not', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(base)).mockRejectedValueOnce(new TypeError('network')).mockResolvedValueOnce(json({}, 404)).mockResolvedValueOnce(json({}, 409)).mockResolvedValueOnce(json({}, 404)); editor(); await edit(); fireEvent.click(screen.getByRole('button', { name: '确认保存属性字典' })); fireEvent.click(await screen.findByRole('button', { name: '查询原字典回执' })); fireEvent.click(await screen.findByRole('button', { name: '确认按原键重试字典' })); await screen.findByText(/不自动换修订/);
  expect(fetch.mock.calls[3][1]?.body).toBe(fetch.mock.calls[1][1]?.body); expect(fetch.mock.calls[3][1]?.headers).toEqual(fetch.mock.calls[1][1]?.headers); fireEvent.click(screen.getByRole('button', { name: '查询原字典回执' })); await waitFor(() => expect(fetch).toHaveBeenCalledTimes(5)); expect(screen.queryByRole('button', { name: '确认按原键重试字典' })).toBeNull();
});
it.each([{ result_revision: 3 }, { space_id: 'other' }, { request_digest: 'f'.repeat(64) }, { definitions: [] }])('rejects invalid receipt %j', async change => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(base)).mockResolvedValueOnce(json({})).mockResolvedValueOnce(json({ ...await receipt(), ...change })); editor(); await edit(); fireEvent.click(screen.getByRole('button', { name: '确认保存属性字典' })); await screen.findByRole('button', { name: '查询原字典回执' }); expect(screen.queryByText('已确认原属性字典修改')).toBeNull(); expect(readRecovery(slot)).not.toBeNull();
});
it('new definitions require valid unique keys; supports six types and draft removal', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(base)); editor(); fireEvent.click(await screen.findByRole('button', { name: '读取完整属性字典' })); fireEvent.click(await screen.findByRole('button', { name: '添加属性字段' })); fireEvent.click(screen.getByRole('button', { name: '预览字典修改' })); await screen.findByText(/请检查唯一稳定/);
  fireEvent.change(screen.getByRole('textbox', { name: '字段 2 稳定 key' }), { target: { value: 'new_field' } }); fireEvent.change(screen.getByRole('textbox', { name: '字段 2 显示名' }), { target: { value: 'New' } }); fireEvent.click(screen.getByRole('button', { name: '预览字典修改' })); await screen.findByRole('button', { name: '确认保存属性字典' }); fireEvent.click(screen.getByRole('button', { name: '返回修改' })); fireEvent.click(screen.getByRole('button', { name: '移除新增字段 2' })); expect(screen.queryByRole('textbox', { name: '字段 2 稳定 key' })).toBeNull();
  for (const type of TYPES) expect(validDefinition({ ...definition, type })).toBe(true); expect(validDefinition({ ...definition, display_name: 'bad\u0080' })).toBe(false);
});
it.each(['offline', 'drivebridge-context', 'storage'])('clears late dictionary after %s', async event => {
  let finish!: (response: Response) => void; vi.spyOn(globalThis, 'fetch').mockImplementation(() => new Promise(resolve => { finish = resolve; })); editor(); fireEvent.click(await screen.findByRole('button', { name: '读取完整属性字典' })); await waitFor(() => expect(finish).toBeDefined()); fireEvent(window, event === 'storage' ? new StorageEvent('storage', { key: slot }) : new Event(event)); finish(json(base)); await screen.findByText(/字典草稿已清除/); expect(screen.queryByRole('textbox', { name: '字段 1 显示名' })).toBeNull();
});
it('asks before discarding and propagates denied reads', async () => {
  const close = vi.fn(), denied = vi.fn(); vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(base)).mockResolvedValueOnce(json({}, 403)); const view = editor(close); await edit(); fireEvent.click(screen.getByRole('button', { name: '关闭' })); await screen.findByText(/关闭将丢弃/); expect(close).not.toHaveBeenCalled(); fireEvent.click(screen.getByRole('button', { name: '丢弃字典草稿并关闭' })); expect(close).toHaveBeenCalledOnce(); view.unmount(); editor(vi.fn(), denied); fireEvent.click(await screen.findByRole('button', { name: '读取完整属性字典' })); await waitFor(() => expect(denied).toHaveBeenCalledOnce());
});
it('missing locks prevent PUT and CAS cannot clear another slot', async () => {
  const first = await claimUpdate(slot, 'a'.repeat(64), new AbortController().signal), second = await claimUpdate(slot, 'b'.repeat(64), new AbortController().signal); expect(second.claimed).toBe(false); expect(second.record).toEqual(first.record); localStorage.setItem(slot, JSON.stringify({ key: 'properties-' + 'c'.repeat(48), digest: 'c'.repeat(64) })); await expect(clearUpdate(slot, first.record)).rejects.toThrow(); localStorage.clear();
  Object.defineProperty(navigator, 'locks', { configurable: true, value: undefined }); const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(base)); editor(); await edit(); fireEvent.click(screen.getByRole('button', { name: '确认保存属性字典' })); await screen.findByText(/跨标签锁不可用/); expect(fetch).toHaveBeenCalledTimes(1);
});
