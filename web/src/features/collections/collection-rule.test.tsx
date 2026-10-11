import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { CreateCollectionDialog } from './CreateCollectionDialog';
import { compileRule, allowedOps, type RuleDraft } from './collection-rule';
import { creationDigest, collectionSlot, readCreation, type Creation } from './collection-recovery';
const slot = collectionSlot('scope');
const value = '报告 <>& "quoted" \\ line\u2028end\u2029';
const rule = compileRule('space', [{ field: 'path_prefix', op: 'contains', value }], { field: 'updated_at', direction: 'desc' });
const input: Creation = { space_id: 'space', name: 'Rule collection', kind: 'rule', scope: 'private', rule };
const json = (data: unknown, status = 200) => new Response(JSON.stringify({ data }), { status });
const receipt = async () => ({ collection_id: 'a'.repeat(48), space_id: 'space', result_revision: 1, request_digest: await creationDigest(input), committed_at: '2026-01-01T00:00:00Z' });
const locks = Object.getOwnPropertyDescriptor(navigator, 'locks');
beforeEach(() => Object.defineProperty(navigator, 'locks', { configurable: true, value: { request: vi.fn(async (_key: string, options: LockOptions | (() => unknown), cb?: () => unknown) => typeof options === 'function' ? options() : cb!()) } }));
afterEach(() => { cleanup(); localStorage.clear(); vi.restoreAllMocks(); if (locks) Object.defineProperty(navigator, 'locks', locks); else Reflect.deleteProperty(navigator, 'locks'); });
const mount = (close = vi.fn(), kind: 'manual' | 'rule' = 'rule') => render(<CreateCollectionDialog scope="scope" kind={kind} space={{ id: 'space', name: 'Space' }} denied={vi.fn()} close={close}/>);
async function prepare() {
  fireEvent.change(await screen.findByRole('textbox', { name: '集合名称' }), { target: { value: input.name } });
  fireEvent.click(screen.getByRole('button', { name: '添加规则条件' }));
  fireEvent.change(screen.getByRole('textbox', { name: '条件 1 值' }), { target: { value } });
  fireEvent.click(screen.getByRole('button', { name: '准备创建集合' })); await screen.findByRole('button', { name: '确认创建集合' });
}
it('rule creation uses the same recovery engine and exact nested JSON wire digest', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({ revision: 7 })).mockResolvedValueOnce(json(await receipt())); const close = vi.fn(); mount(close); await prepare(); expect(fetch).not.toHaveBeenCalled(); expect(screen.getByLabelText('待确认集合规则').textContent).toBe(JSON.stringify(rule, null, 2));
  fireEvent.click(screen.getByRole('button', { name: '确认创建集合' })); await screen.findByText('已确认原集合创建');
  const wire = fetch.mock.calls[0][1]?.body as string; expect(JSON.parse(wire)).toEqual(input); expect(wire).toContain(JSON.stringify(rule));
  expect(JSON.parse(localStorage.getItem(slot)!)).toEqual({ key: expect.stringMatching(/^collection-[a-f0-9]{48}$/), digest: await creationDigest(input) }); expect(localStorage.getItem(slot)).not.toContain(value);
  fireEvent.click(screen.getByRole('button', { name: '确认回执并关闭' })); await waitFor(() => expect(close).toHaveBeenCalledOnce()); expect(readCreation(slot)).toBeNull();
});
it('a rule unknown can recover through the manual entry without recreating or exposing the draft', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new TypeError('network')).mockResolvedValueOnce(json(await receipt())); const view = mount(); await prepare(); fireEvent.click(screen.getByRole('button', { name: '确认创建集合' })); await screen.findByRole('button', { name: '查询原集合回执' }); const original = readCreation(slot)!;
  view.unmount(); mount(vi.fn(), 'manual'); fireEvent.click(await screen.findByRole('button', { name: '查询原集合回执' })); await screen.findByText('已确认原集合创建'); expect(fetch).toHaveBeenCalledTimes(2); expect(JSON.parse(fetch.mock.calls[1][1]?.body as string)).toEqual({ idempotency_key: original.key });
});
it('invalid rule is rejected before claim and confirms draft discard', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch'); const close = vi.fn(); mount(close); fireEvent.change(await screen.findByRole('textbox', { name: '集合名称' }), { target: { value: input.name } }); fireEvent.click(screen.getByRole('button', { name: '添加规则条件' })); fireEvent.click(screen.getByRole('button', { name: '准备创建集合' })); await screen.findByText(/请检查规则条件/); expect(fetch).not.toHaveBeenCalled(); expect(localStorage.length).toBe(0);
  fireEvent.click(screen.getByRole('button', { name: '关闭' })); await screen.findByText(/关闭将丢弃/); expect(close).not.toHaveBeenCalled(); fireEvent.click(screen.getByRole('button', { name: '丢弃集合草稿并关闭' })); expect(close).toHaveBeenCalledOnce();
});
it('empty rule explicitly confirms all-space dynamic membership without a query', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch'); mount(); fireEvent.change(await screen.findByRole('textbox', { name: '集合名称' }), { target: { value: 'All visible' } }); fireEvent.click(screen.getByRole('button', { name: '准备创建集合' })); await screen.findByText(/无过滤条件：读取时匹配/); expect(fetch).not.toHaveBeenCalled();
});
it('definitive invalid create retains identity but does not offer a new or original-body retry', async () => {
  const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(json({}, 400)).mockResolvedValueOnce(json({}, 404)); mount(); await prepare(); fireEvent.click(screen.getByRole('button', { name: '确认创建集合' })); fireEvent.click(await screen.findByRole('button', { name: '查询原集合回执' })); await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2)); expect(screen.queryByRole('button', { name: '确认按原键重试创建' })).toBeNull(); expect(readCreation(slot)).not.toBeNull();
});
it('typed rules cover all admitted field families and budgets without implicit coercion', () => {
  const rows: RuleDraft[] = [
    { field: 'path_prefix', op: 'prefix', value: 'docs' }, { field: 'mime_type', op: 'eq', value: 'text/plain' },
    { field: 'caller', op: 'eq', value: '' }, { field: 'task_ref', op: 'eq', value: '' },
    { field: 'labels.manual', op: 'ne', value: 'skip' }, { field: 'labels.rule', op: 'eq', value: 'a' }, { field: 'labels.suggested', op: 'contains', value: 'b' },
    { field: 'size_bytes', op: 'gte', value: '0' }, { field: 'updated_at', op: 'lt', value: '2030-01-01T00:00:00Z' }, { field: 'trashed', op: 'eq', value: '' },
  ];
  const compiled = compileRule('s', rows, { field: 'size_bytes', direction: 'asc' }); expect(compiled.filters?.[7].value).toBe(0); expect(compiled.filters?.[9].value).toBe(false); expect(compiled.space_ids).toEqual(['s']);
  expect(allowedOps('mime_type')).toEqual(['eq']);
  for (const bad of [{ field: 'size_bytes', op: 'gt', value: '1.5' }, { field: 'size_bytes', op: 'gt', value: '9007199254740992' }, { field: 'updated_at', op: 'lt', value: 'yesterday' }, { field: 'mime_type', op: 'contains', value: 'text' }, { field: 'path_prefix', op: 'contains', value: '\ud800' }, { field: 'path_prefix', op: 'contains', value: '中'.repeat(1400) }] as RuleDraft[]) expect(() => compileRule('s', [bad], { field: 'name', direction: 'asc' })).toThrow();
  expect(() => compileRule('s', Array(65).fill(rows[0]), { field: 'name', direction: 'asc' })).toThrow();
});
