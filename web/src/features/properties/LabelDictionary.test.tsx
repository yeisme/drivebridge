import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { LabelDictionary } from './LabelDictionary';
import type { components } from '../../generated/personal-workspace';
type Page = components['schemas']['LabelDefinitionPageEnvelope']['data'];
const space = 'a'.repeat(48), file = 'b'.repeat(48), version = 'c'.repeat(48), planId = 'd'.repeat(48);
const ref = `drivebridge://fixture/file/${file}/version/${version}`;
let catalog: Page, writes: string[], lost: boolean, receipt: unknown;
const json = (data: unknown) => new Response(JSON.stringify({ data }));
const locks = Object.getOwnPropertyDescriptor(navigator, 'locks');
beforeEach(() => {
  catalog = { space_id: space, namespace: 'manual', revision: 1, management_enabled: true, definitions: [{ key: 'source', display_name: '来源', group: '', revision: 1 }, { key: 'target', display_name: '目标', group: '', revision: 1 }] }; writes = []; lost = false; receipt = null;
  Object.defineProperty(navigator, 'locks', { configurable: true, value: { request: vi.fn(async (_name: string, options: LockOptions | (() => unknown), callback?: () => unknown) => typeof options === 'function' ? options() : callback!()) } });
  vi.spyOn(globalThis, 'fetch').mockImplementation(async (url, options) => {
    const path = String(url);
    if (path === '/api/v1/me') return json({ user: { id: 'alice' }, instance_id: 'fixture' });
    if (path.includes('/label-definitions?')) return json(catalog);
    if (path.endsWith('/label-definitions/lookup')) return json(receipt);
    if (options?.method === 'PUT') {
      writes.push('dictionary'); const body = JSON.parse(String(options.body));
      const raw = JSON.stringify({ space_id: space, request: body });
      const digest = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(raw))), b => b.toString(16).padStart(2, '0')).join('');
      receipt = { space_id: space, namespace: 'manual', request_digest: digest, result_revision: 2, committed_at: '2026-10-09T00:00:00Z' };
      catalog = { ...catalog, revision: 2, definitions: body.definitions.map((row: { key: string; display_name: string; group: string }) => ({ ...row, revision: 2 })) };
      if (lost) throw new TypeError('lost'); return json(receipt);
    }
    throw new Error('unexpected:' + path);
  });
});
afterEach(() => { cleanup(); localStorage.clear(); vi.restoreAllMocks(); if (locks) Object.defineProperty(navigator, 'locks', locks); else Reflect.deleteProperty(navigator, 'locks'); });
async function open() { fireEvent.click(screen.getByRole('button', { name: '管理标签字典' })); await screen.findByText(/标签字典修订 1/); }
it('loads manual dictionary and confirms a full CAS update without renaming saved keys', async () => {
  render(<LabelDictionary space={space} scope="actor"/>); await open();
  fireEvent.click(screen.getByRole('button', { name: '编辑标签定义' })); expect(screen.getByRole('textbox', { name: '标签 1 稳定 key' })).toHaveProperty('disabled', true);
  fireEvent.change(screen.getByRole('textbox', { name: '标签 1 显示名' }), { target: { value: '新显示名' } });
  fireEvent.click(screen.getByRole('button', { name: '预览标签字典修改' })); expect(writes).toEqual([]);
  fireEvent.click(screen.getByRole('button', { name: '确认保存标签字典' })); await screen.findByText(/标签字典修订 2/); expect(writes).toEqual(['dictionary']);
});
it('recovers a lost dictionary write with its original key and digest after remount', async () => {
  lost = true; const view = render(<LabelDictionary space={space} scope="actor"/>); await open(); fireEvent.click(screen.getByRole('button', { name: '编辑标签定义' }));
  fireEvent.change(screen.getByRole('textbox', { name: '标签 1 显示名' }), { target: { value: '恢复名称' } }); fireEvent.click(screen.getByRole('button', { name: '预览标签字典修改' })); fireEvent.click(screen.getByRole('button', { name: '确认保存标签字典' }));
  await waitFor(() => expect(screen.getByRole('button', { name: '查询原标签请求' })).toHaveProperty('disabled', false));
  expect(localStorage.getItem(localStorage.key(0)!)).not.toContain('恢复名称'); view.unmount(); render(<LabelDictionary space={space} scope="actor"/>); fireEvent.click(screen.getByRole('button', { name: '管理标签字典' }));
  await waitFor(() => expect(screen.getByRole('button', { name: '查询原标签请求' })).toHaveProperty('disabled', false)); fireEvent.click(screen.getByRole('button', { name: '查询原标签请求' }));
  await screen.findByText(/标签字典修订 2/); expect(writes).toEqual(['dictionary']); expect(localStorage.length).toBe(0);
});
it('makes gate-off read-only and clears details on revocation', async () => {
  catalog.management_enabled = false; render(<LabelDictionary space={space} scope="actor"/>); await open();
  expect(screen.queryByRole('button', { name: '编辑标签定义' })).toBeNull(); expect(screen.queryByRole('button', { name: '生成所选文件合并预览' })).toBeNull();
  fireEvent(window, new Event('drivebridge-context')); await screen.findByText(/标签数据已清除/); expect(screen.queryByText('来源')).toBeNull();
});
it('reads sealed dictionary pages before editing and rejects revision drift', async () => {
  const transport = vi.mocked(fetch).getMockImplementation()!;
  vi.mocked(fetch).mockImplementation(async (url, options) => String(url).includes('/label-definitions?') ? json(String(url).includes('cursor=next') ? { ...catalog, revision: 2, definitions: [catalog.definitions[1]] } : { ...catalog, definitions: [catalog.definitions[0]], next_cursor: 'next' }) : transport(url, options));
  render(<LabelDictionary space={space} scope="actor"/>); fireEvent.click(screen.getByRole('button', { name: '管理标签字典' })); await screen.findByText(/标签读取失败/);
  expect(screen.queryByRole('button', { name: '编辑标签定义' })).toBeNull();
});
it('previews fixed selected refs then recovers a lost execute by GET without resubmitting', async () => {
  const transport = vi.mocked(fetch).getMockImplementation()!;
  let phase = 'previewed', revision = 1;
  const detail = { namespace: 'manual', source_key: 'source', target_key: 'target', expected_revision: 1, source_revision: 1, target_revision: 1, refs: null };
  const plan = () => ({ plan_id: planId, space_id: space, revision, phase, risk: 'approval_required', approval_required: true, impact_digest: 'e'.repeat(64), items_total: 1, items_done: phase === 'completed' ? 1 : 0, items_failed: 0, items_stale: 0, items: [{ item_key: file, file_id: file, version_id: version, path: 'chosen.txt', action: 'merge_label', action_detail: JSON.stringify(detail), before: 'f'.repeat(64), after: 'target', status: phase === 'completed' ? 'done' : 'pending' }] });
  vi.mocked(fetch).mockImplementation(async (url, options) => {
    const path = String(url);
    if (path.endsWith('/label-merge-plans')) { writes.push('preview'); return json(plan()); }
    if (path.includes('/organization-plans/')) {
      if (path.endsWith('/confirm')) { writes.push('confirm'); phase = 'confirmed'; revision = 2; }
      if (path.endsWith('/execute')) { writes.push('execute'); phase = 'completed'; revision = 4; throw new TypeError('lost'); }
      return json(plan());
    }
    return transport(url, options);
  });
  render(<LabelDictionary space={space} scope="actor"/>); await open();
  fireEvent.change(screen.getByRole('textbox', { name: '所选文件固定引用' }), { target: { value: ref } });
  fireEvent.click(screen.getByRole('button', { name: '生成所选文件合并预览' })); await screen.findByText('chosen.txt');
  fireEvent.click(screen.getByRole('button', { name: '确认所选文件合并计划' }));
  fireEvent.click(await screen.findByRole('button', { name: '执行所选文件合并' })); expect(writes).toEqual(['preview', 'confirm']);
  fireEvent.click(screen.getByRole('button', { name: '确认执行标签合并' }));
  await waitFor(() => expect(screen.getByRole('button', { name: '重读原合并计划' })).toHaveProperty('disabled', false));
  fireEvent.click(screen.getByRole('button', { name: '重读原合并计划' })); await screen.findByText(/合并完成/); expect(writes).toEqual(['preview', 'confirm', 'execute']);
});

it('rejects foreign and non-fixed merge refs before sending a preview', async () => {
  render(<LabelDictionary space={space} scope="actor"/>); await open();
  fireEvent.change(screen.getByRole('textbox', { name: '所选文件固定引用' }), { target: { value: `drivebridge://foreign/file/${file}/version/${version}` } });
  fireEvent.click(screen.getByRole('button', { name: '生成所选文件合并预览' })); await screen.findByText(/未发送修改/);
  expect(writes).toEqual([]); expect(localStorage.length).toBe(0);
});

it('retains recovery identity on a mismatched dictionary receipt', async () => {
  lost = true; render(<LabelDictionary space={space} scope="actor"/>); await open(); fireEvent.click(screen.getByRole('button', { name: '编辑标签定义' }));
  fireEvent.change(screen.getByRole('textbox', { name: '标签 1 显示名' }), { target: { value: '修改' } }); fireEvent.click(screen.getByRole('button', { name: '预览标签字典修改' })); fireEvent.click(screen.getByRole('button', { name: '确认保存标签字典' }));
  await waitFor(() => expect(screen.getByRole('button', { name: '查询原标签请求' })).toHaveProperty('disabled', false));
  receipt = { space_id: space, namespace: 'manual', request_digest: '0'.repeat(64), result_revision: 2, committed_at: '2026-10-09T00:00:00Z' };
  fireEvent.click(screen.getByRole('button', { name: '查询原标签请求' }));
  await waitFor(() => expect(screen.getByRole('button', { name: '查询原标签请求' })).toHaveProperty('disabled', false));
  expect(localStorage.length).toBe(1); expect(writes).toEqual(['dictionary']);
});

it('keeps unauthorized reads distinct from an empty dictionary', async () => {
  vi.mocked(fetch).mockResolvedValue(new Response(JSON.stringify({ error: { code: 'forbidden' } }), { status: 403 }));
  render(<LabelDictionary space={space} scope="actor"/>); fireEvent.click(screen.getByRole('button', { name: '管理标签字典' })); await screen.findByText(/标签数据已清除/);
  expect(screen.queryByText(/尚无 manual/)).toBeNull(); expect(screen.queryByRole('button', { name: '重读标签字典' })).toBeNull();
});

it('recovers a lost merge preview by original key and revalidates the full frozen selection', async () => {
  const transport = vi.mocked(fetch).getMockImplementation()!;
  let originalKey = '', lookupKey = '';
  const detail = { namespace: 'manual', source_key: 'source', target_key: 'target', expected_revision: 1, source_revision: 1, target_revision: 1, refs: null };
  const plan = { plan_id: planId, space_id: space, revision: 1, phase: 'previewed', risk: 'approval_required', approval_required: true, impact_digest: 'e'.repeat(64), items_total: 1, items_done: 0, items_failed: 0, items_stale: 0, items: [{ item_key: file, file_id: file, version_id: version, path: 'chosen.txt', action: 'merge_label', action_detail: JSON.stringify(detail), before: 'f'.repeat(64), after: 'target', status: 'pending' }] };
  vi.mocked(fetch).mockImplementation(async (url, options) => {
    const path = String(url);
    if (path.endsWith('/label-merge-plans')) { writes.push('preview'); originalKey = new Headers(options?.headers).get('Idempotency-Key')!; throw new TypeError('lost'); }
    if (path.endsWith('/label-merge-plans/lookup')) { lookupKey = JSON.parse(String(options?.body)).idempotency_key; return json(plan); }
    if (path.includes('/organization-plans/')) return json(plan);
    return transport(url, options);
  });
  const view = render(<LabelDictionary space={space} scope="actor"/>); await open();
  fireEvent.change(screen.getByRole('textbox', { name: '所选文件固定引用' }), { target: { value: ref } }); fireEvent.click(screen.getByRole('button', { name: '生成所选文件合并预览' }));
  await waitFor(() => expect(screen.getByRole('button', { name: '查询原标签请求' })).toHaveProperty('disabled', false));
  expect(localStorage.getItem(localStorage.key(0)!)).not.toContain(ref); view.unmount();
  render(<LabelDictionary space={space} scope="actor"/>); await open(); fireEvent.click(screen.getByRole('button', { name: '查询原标签请求' }));
  await screen.findByRole('button', { name: '确认所选文件合并计划' }); expect(writes).toEqual(['preview']); expect(lookupKey).toBe(originalKey);
});

it('keeps the original draft CAS after refresh so a concurrent untouched definition cannot be overwritten', async () => {
  const transport = vi.mocked(fetch).getMockImplementation()!;
  let submitted: components['schemas']['LabelDefinitionUpdate'] | undefined;
  vi.mocked(fetch).mockImplementation(async (url, options) => {
    if (options?.method === 'PUT') {
      submitted = JSON.parse(String(options.body));
      if (submitted!.expected_revision !== catalog.revision) return new Response(JSON.stringify({ error: { code: 'revision_conflict' } }), { status: 409 });
    }
    return transport(url, options);
  });
  render(<LabelDictionary space={space} scope="actor"/>); await open();
  fireEvent.click(screen.getByRole('button', { name: '编辑标签定义' }));
  fireEvent.change(screen.getByRole('textbox', { name: '标签 1 显示名' }), { target: { value: '我的来源修改' } });
  catalog = { ...catalog, revision: 2, definitions: [catalog.definitions[0], { ...catalog.definitions[1], display_name: '他人目标修改', revision: 2 }] };
  fireEvent.click(screen.getByRole('button', { name: '重读标签字典' }));
  await screen.findByText(/标签字典修订 2/);
  expect(screen.getByRole('textbox', { name: '标签 1 显示名' })).toHaveProperty('value', '我的来源修改');
  fireEvent.click(screen.getByRole('button', { name: '预览标签字典修改' }));
  expect(screen.getByText(/按修订 1 保存完整定义/)).toBeTruthy();
  fireEvent.click(screen.getByRole('button', { name: '确认保存标签字典' }));
  await screen.findByText(/标签修订冲突，本次未提交/);
  expect(submitted?.expected_revision).toBe(1); expect(catalog.definitions[1].display_name).toBe('他人目标修改'); expect(writes).toEqual([]);
  fireEvent.click(screen.getByRole('button', { name: '重读标签字典' })); await screen.findByText(/标签字典修订 2/);
  expect(screen.getByRole('textbox', { name: '标签 1 显示名' })).toHaveProperty('value', '我的来源修改');
  fireEvent.click(screen.getByRole('button', { name: '放弃标签字典草稿' }));
  fireEvent.click(screen.getByRole('button', { name: '编辑标签定义' }));
  expect(screen.getByRole('textbox', { name: '标签 2 显示名' })).toHaveProperty('value', '他人目标修改');
});
