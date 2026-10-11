import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { FileProperties, FilePropertiesEntry } from './FileProperties';
import { parseValue, sidecarDigest, sidecarSlot, validSidecar, type SidecarPatch } from './sidecar-client';
import { readRecovery } from './property-recovery';
const file = 'a'.repeat(48), version = 'b'.repeat(48), slot = sidecarSlot('scope', file, version);
const catalog = { space_id: 's', revision: 1, definitions: [{ key: 'caption', display_name: '说明', group: '', type: 'text' as const }, { key: 'rank', display_name: '分数', group: '', type: 'number' as const }] };
const base = { file_id: file, version_id: version, revision: 0, definition_revision: 1, storage_kind: 'sidecar' as const, values: { rank: 1.25 }, unknown_keys: ['legacy'] };
const input: SidecarPatch = { base_version_id: version, definition_revision: 1, expected_revision: 0, updates: { caption: '私有草稿 <>&' }, remove_keys: [] };
const json = (data: unknown, status = 200) => new Response(JSON.stringify({ data }), { status });
const receipt = async () => ({ file_id: file, version_id: version, request_digest: await sidecarDigest(file, input), result_revision: 1, committed_at: '2026-01-01T00:00:00Z' });
const locks = Object.getOwnPropertyDescriptor(navigator, 'locks');
beforeEach(() => Object.defineProperty(navigator, 'locks', { configurable: true, value: { request: vi.fn(async (_name: string, options: LockOptions | (() => unknown), callback?: () => unknown) => typeof options === 'function' ? options() : callback!()) } }));
afterEach(() => { cleanup(); localStorage.clear(); vi.restoreAllMocks(); if (locks) Object.defineProperty(navigator, 'locks', locks); else Reflect.deleteProperty(navigator, 'locks'); });
const editor = (close = vi.fn(), denied = vi.fn()) => render(<FileProperties scope="scope" space="s" file={file} version={version} close={close} denied={denied}/>);
async function change() {
  fireEvent.click(await screen.findByRole('button', { name: '读取固定版本属性' }));
  fireEvent.click(await screen.findByRole('button', { name: '说明 设置值' }));
  fireEvent.change(await screen.findByRole('textbox', { name: '说明 新值' }), { target: { value: '私有草稿 <>&' } });
  fireEvent.click(screen.getByRole('button', { name: '预览属性修改' })); await screen.findByRole('button', { name: '确认保存文件属性' });
}
it('opens without network and rejects unsupported text rather than sidecar substitution', () => {
  const fetch = vi.spyOn(globalThis, 'fetch'); render(<FilePropertiesEntry scope="scope" space="s" file={file} version={version} path="note.md"/>); expect(screen.queryByRole('button', { name: '查看文件属性' })).toBeNull(); expect(screen.getByText(/独立无损合同/)).toBeTruthy(); expect(fetch).not.toHaveBeenCalled();
});
it('patches only explicit changes, preserves unknown and unchanged numeric fields, confirms original receipt', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(catalog)).mockResolvedValueOnce(json(base)).mockResolvedValueOnce(json({})).mockResolvedValueOnce(json(await receipt())); const close = vi.fn(); editor(close); expect(fetch).not.toHaveBeenCalled(); await change(); expect(fetch).toHaveBeenCalledTimes(2); expect(localStorage.length).toBe(0);
  fireEvent.click(screen.getByRole('button', { name: '确认保存文件属性' })); await screen.findByText('已确认原文件属性修改'); expect(fetch.mock.calls[2][1]?.method).toBe('PATCH'); expect(JSON.parse(fetch.mock.calls[2][1]?.body as string)).toEqual(input);
  expect(JSON.parse(localStorage.getItem(slot)!)).toEqual({ key: expect.stringMatching(/^properties-[a-f0-9]{48}$/), digest: await sidecarDigest(file, input) }); expect(localStorage.getItem(slot)).not.toContain('私有草稿'); fireEvent.click(screen.getByRole('button', { name: '确认属性回执并关闭' })); await waitFor(() => expect(close).toHaveBeenCalledOnce()); expect(readRecovery(slot)).toBeNull();
});
it('lost PATCH remount only queries original fixed version receipt', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(catalog)).mockResolvedValueOnce(json(base)).mockRejectedValueOnce(new TypeError('network')).mockResolvedValueOnce(json(await receipt())); const view = editor(); await change(); fireEvent.click(screen.getByRole('button', { name: '确认保存文件属性' })); await screen.findByRole('button', { name: '查询原属性回执' }); const original = readRecovery(slot)!; view.unmount(); editor(); await screen.findByRole('button', { name: '查询原属性回执' }); expect(fetch).toHaveBeenCalledTimes(3); fireEvent.click(screen.getByRole('button', { name: '查询原属性回执' })); await screen.findByText('已确认原文件属性修改'); expect(JSON.parse(fetch.mock.calls[3][1]?.body as string)).toEqual({ version_id: version, idempotency_key: original.key });
});
it('lookup 404 permits only explicit original-body retry; conflict destroys retry body', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(catalog)).mockResolvedValueOnce(json(base)).mockRejectedValueOnce(new TypeError('network')).mockResolvedValueOnce(json({}, 404)).mockResolvedValueOnce(json({}, 409)).mockResolvedValueOnce(json({}, 404)); editor(); await change(); fireEvent.click(screen.getByRole('button', { name: '确认保存文件属性' })); fireEvent.click(await screen.findByRole('button', { name: '查询原属性回执' })); fireEvent.click(await screen.findByRole('button', { name: '确认原键重试属性' })); await screen.findByText(/不自动换版本/); expect(fetch.mock.calls[4][1]?.body).toBe(fetch.mock.calls[2][1]?.body); expect(fetch.mock.calls[4][1]?.headers).toEqual(fetch.mock.calls[2][1]?.headers); fireEvent.click(screen.getByRole('button', { name: '查询原属性回执' })); await waitFor(() => expect(fetch).toHaveBeenCalledTimes(6)); expect(screen.queryByRole('button', { name: '确认原键重试属性' })).toBeNull();
});
it.each([{ version_id: 'c'.repeat(48) }, { request_digest: 'f'.repeat(64) }, { result_revision: 2 }, { values: {} }])('rejects mismatched receipt %j', async changeReceipt => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(catalog)).mockResolvedValueOnce(json(base)).mockResolvedValueOnce(json({})).mockResolvedValueOnce(json({ ...await receipt(), ...changeReceipt })); editor(); await change(); fireEvent.click(screen.getByRole('button', { name: '确认保存文件属性' })); await screen.findByRole('button', { name: '查询原属性回执' }); expect(screen.queryByText('已确认原文件属性修改')).toBeNull();
});
it.each(['offline', 'drivebridge-context', 'storage'])('clears late values after %s', async event => {
  let finish!: (response: Response) => void; vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(catalog)).mockImplementation(() => new Promise(resolve => { finish = resolve; })); editor(); fireEvent.click(await screen.findByRole('button', { name: '读取固定版本属性' })); await waitFor(() => expect(finish).toBeDefined()); fireEvent(window, event === 'storage' ? new StorageEvent('storage', { key: slot }) : new Event(event)); finish(json(base)); await screen.findByText(/属性草稿已清除/); expect(screen.queryByRole('button', { name: '说明 设置值' })).toBeNull();
});
it('guards discard, denied and missing lock', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(catalog)).mockResolvedValueOnce(json(base)).mockResolvedValueOnce(json({}, 403)); const close = vi.fn(), denied = vi.fn(); const view = editor(close); await change(); fireEvent.click(screen.getByRole('button', { name: '关闭' })); await screen.findByText(/关闭将丢弃/); expect(close).not.toHaveBeenCalled(); fireEvent.click(screen.getByRole('button', { name: '丢弃属性草稿并关闭' })); expect(close).toHaveBeenCalledOnce(); view.unmount(); const next = editor(vi.fn(), denied); fireEvent.click(await screen.findByRole('button', { name: '读取固定版本属性' })); await waitFor(() => expect(denied).toHaveBeenCalledOnce()); next.unmount();
  fetch.mockResolvedValueOnce(json(catalog)).mockResolvedValueOnce(json(base)); Object.defineProperty(navigator, 'locks', { configurable: true, value: undefined }); editor(); await change(); fireEvent.click(screen.getByRole('button', { name: '确认保存文件属性' })); await screen.findByText(/跨标签锁不可用/); expect(fetch).toHaveBeenCalledTimes(5);
});
it('explicit removal sends no replacement value', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(catalog)).mockResolvedValueOnce(json(base)).mockRejectedValueOnce(new TypeError('network')); editor(); fireEvent.click(await screen.findByRole('button', { name: '读取固定版本属性' })); fireEvent.click(await screen.findByRole('button', { name: '分数 删除此值' })); fireEvent.click(screen.getByRole('button', { name: '预览属性修改' })); await screen.findByText(/删除（不是空文本）/); fireEvent.click(screen.getByRole('button', { name: '确认保存文件属性' })); await screen.findByRole('button', { name: '查询原属性回执' }); expect(JSON.parse(fetch.mock.calls[2][1]?.body as string)).toEqual({ ...input, updates: {}, remove_keys: ['rank'] });
});
it('preserves multiline text and treats constructor as an own dictionary key', async () => {
  const definitions = { ...catalog, definitions: [{ key: 'constructor', display_name: '构造字段', group: '', type: 'text' as const }] };
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(definitions)).mockResolvedValueOnce(json({ ...base, values: {} })).mockRejectedValueOnce(new TypeError('network'));
  editor(); fireEvent.click(await screen.findByRole('button', { name: '读取固定版本属性' }));
  expect(await screen.findByText('constructor · 当前值：未设置')).toBeTruthy();
  fireEvent.click(screen.getByRole('button', { name: '构造字段 设置值' }));
  const field = screen.getByRole('textbox', { name: '构造字段 新值' }); expect(field.tagName).toBe('TEXTAREA');
  fireEvent.change(field, { target: { value: 'line one\nline two' } });
  fireEvent.click(screen.getByRole('button', { name: '预览属性修改' })); fireEvent.click(screen.getByRole('button', { name: '确认保存文件属性' }));
  await screen.findByRole('button', { name: '查询原属性回执' });
  expect(JSON.parse(fetch.mock.calls[2][1]?.body as string).updates).toEqual({ constructor: 'line one\nline two' });
});
it('read-only detail does not offer mutation controls', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(catalog)).mockResolvedValueOnce(json(base));
  render(<FileProperties scope="scope" space="s" file={file} version={version} readOnly close={vi.fn()} denied={vi.fn()}/>);
  fireEvent.click(await screen.findByRole('button', { name: '读取固定版本属性' })); await screen.findByText(/保留的未知字段/);
  expect(screen.queryByRole('button', { name: '说明 设置值' })).toBeNull(); expect(screen.queryByRole('button', { name: '预览属性修改' })).toBeNull();
});
it('history opens only its scope/version slot, recovers original receipt without reading values or writing', async () => {
  const record = { key: 'properties-' + 'c'.repeat(48), digest: await sidecarDigest(file, input) };
  localStorage.setItem(slot, JSON.stringify(record));
  const other = sidecarSlot('foreign-scope', file, version); localStorage.setItem(other, '{invalid');
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(await receipt()));
  render(<FilePropertiesEntry history scope="scope" space="s" file={file} version={version} path="image.png"/>);
  const trigger = screen.getByRole('button', { name: '查看历史属性与回执 ' + version }); fireEvent.click(trigger);
  await screen.findByRole('button', { name: '查询原属性回执' }); expect(fetch).not.toHaveBeenCalled();
  expect(screen.queryByRole('button', { name: '读取固定版本属性' })).toBeNull();
  fireEvent.click(screen.getByRole('button', { name: '查询原属性回执' })); await screen.findByText('已确认原文件属性修改');
  expect(fetch).toHaveBeenCalledTimes(1); expect(JSON.parse(fetch.mock.calls[0][1]?.body as string)).toEqual({ version_id: version, idempotency_key: record.key });
  fireEvent.click(screen.getByRole('button', { name: '确认属性回执并关闭' })); await waitFor(() => expect(readRecovery(slot)).toBeNull());
  expect(localStorage.getItem(other)).toBe('{invalid');
});
it('history without recovery is read only even when caller omitted readOnly', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json(catalog)).mockResolvedValueOnce(json(base));
  render(<FilePropertiesEntry history scope="scope" space="s" file={file} version={version} path="image.png"/>);
  fireEvent.click(screen.getByRole('button', { name: '查看历史属性与回执 ' + version })); fireEvent.click(await screen.findByRole('button', { name: '读取固定版本属性' }));
  await screen.findByText(/保留的未知字段/); expect(screen.queryByRole('button', { name: '说明 设置值' })).toBeNull();
});
it('validates six types, no numeric rounding, foreign view and catalog drift', async () => {
  expect(parseValue('', 'text')).toBe(''); expect(parseValue('1.25', 'number')).toBe(1.25); expect(parseValue('false', 'checkbox')).toBe(false); expect(parseValue('2024-02-29', 'date')).toBe('2024-02-29'); expect(parseValue('["a","a",""]', 'list')).toEqual(['a', 'a', '']); expect(parseValue(`drivebridge://instance/file/${file}/version/${version}`, 'reference')).toContain('drivebridge:');
  for (const number of ['9007199254740991.1', '1.250', '1e-999', '-0', 'NaN', '1.00000000000000001']) expect(() => parseValue(number, 'number')).toThrow(); expect(() => parseValue('2025-02-29', 'date')).toThrow(); expect(() => parseValue('[null]', 'list')).toThrow(); expect(() => parseValue('\ud800', 'text')).toThrow();
  expect(validSidecar(base, catalog, 's', file, version)).toBe(true); expect(validSidecar({ ...base, definition_revision: 2 }, catalog, 's', file, version)).toBe(false); expect(validSidecar({ ...base, values: { legacy: 'hidden' } }, catalog, 's', file, version)).toBe(false);
  expect(await sidecarDigest(file, input)).toBe(await sidecarDigest(file, { ...input, updates: { ...input.updates } }));
});
