import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { PropertyMigration } from './PropertyMigration';
import type { components } from '../../generated/personal-workspace';
type Plan = components['schemas']['PropertyMigrationViewEnvelope']['data'];
const space = 'a'.repeat(48), planId = 'b'.repeat(48), impact = 'c'.repeat(64);
const request = { expected_definition_revision: 1, source_key: 'old', target_key: 'new', target_type: 'text' as const, conversion_policy: 'preserve' as const };
const json = (data: unknown) => new Response(JSON.stringify({ data }));
let plan: Plan, receipts: Record<string, unknown>, writes: string[], lost: string, enabled: boolean;
const locks = Object.getOwnPropertyDescriptor(navigator, 'locks');
beforeEach(() => {
  plan = { plan_id: planId, space_id: space, revision: 1, impact_digest: impact, phase: 'previewed', editing_enabled: true, request, items: [{ file_id: 'd'.repeat(48), version_id: 'e'.repeat(48), sidecar_revision: 1, before: '"value"', after: '"value"', status: 'pending', error: '' }] };
  receipts = {}; writes = []; lost = ''; enabled = true;
  Object.defineProperty(navigator, 'locks', { configurable: true, value: { request: vi.fn(async (_name: string, options: LockOptions | (() => unknown), callback?: () => unknown) => typeof options === 'function' ? options() : callback!()) } });
  vi.spyOn(globalThis, 'fetch').mockImplementation(async (url, options) => {
    const path = String(url);
    if (path === '/api/v1/me') return json({ user: { id: 'alice' }, instance_id: 'instance' });
    if (path.endsWith('/capability')) return json({ space_id: space, editing_enabled: enabled, storage_kind: 'sidecar', max_sidecars: 200, markdown_supported: false, conversion_policies: ['preserve', 'parse_text', 'scalar_text', 'singleton_list'] });
    if (path.endsWith('/property-definitions')) return json({ space_id: space, revision: 1, definitions: [{ key: 'old', display_name: '原字段', group: '', type: 'text' }] });
    if (path === '/api/v1/property-migrations/' + planId) return json(plan);
    const body = JSON.parse(String(options?.body));
    if (path.endsWith('/lookup')) return receipts[body.stage] ? json(receipts[body.stage]) : new Response(JSON.stringify({ error: { code: 'not_found' } }), { status: 404 });
    const stage = path.endsWith('/confirm') ? 'confirm' : path.endsWith('/execute') ? 'execute' : 'preview';
    writes.push(stage);
    const raw = JSON.stringify({ resource_id: stage === 'preview' ? space : planId, request: body });
    const digest = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(raw))), byte => byte.toString(16).padStart(2, '0')).join('');
    if (stage !== 'preview') { expect(body).toEqual({ expected_revision: plan.revision, impact_digest: impact }); plan = { ...plan, revision: plan.revision + 1, phase: stage === 'confirm' ? 'confirmed' : 'completed', items: plan.items.map(row => ({ ...row, status: stage === 'execute' ? 'done' : row.status })) }; }
    receipts[stage] = { space_id: space, plan_id: planId, stage, request_digest: digest, result_revision: plan.revision, committed_at: '2026-10-09T00:00:00Z' };
    if (lost === stage) throw new TypeError('lost');
    return json(receipts[stage]);
  });
});
afterEach(() => { cleanup(); localStorage.clear(); vi.restoreAllMocks(); if (locks) Object.defineProperty(navigator, 'locks', locks); else Reflect.deleteProperty(navigator, 'locks'); });
async function preview() {
  fireEvent.click(screen.getByRole('button', { name: '字段迁移计划' }));
  fireEvent.change(await screen.findByRole('textbox', { name: '目标字段 key' }), { target: { value: 'new' } });
  fireEvent.click(screen.getByRole('button', { name: '生成迁移预览' }));
}
it('separates preview, human confirmation and explicit risk execution with exact revision and digest', async () => {
  render(<PropertyMigration space={space} scope="actor"/>); await preview();
  await screen.findAllByText('"value"', { selector: 'pre' }); expect(writes).toEqual(['preview']);
  fireEvent.click(screen.getByRole('button', { name: '确认此迁移计划' }));
  fireEvent.click(await screen.findByRole('button', { name: '执行已确认迁移' })); expect(writes).toEqual(['preview', 'confirm']);
  fireEvent.click(screen.getByRole('button', { name: '确认执行字段与属性迁移' }));
  await screen.findByText(/迁移已完成/); expect(writes).toEqual(['preview', 'confirm', 'execute']);
});
it.each(['preview', 'confirm', 'execute'])('recovers lost %s by original stage lookup after remount without replay', async stage => {
  lost = stage; const view = render(<PropertyMigration space={space} scope="actor"/>); await preview();
  if (stage !== 'preview') fireEvent.click(await screen.findByRole('button', { name: '确认此迁移计划' }));
  if (stage === 'execute') { fireEvent.click(await screen.findByRole('button', { name: '执行已确认迁移' })); fireEvent.click(screen.getByRole('button', { name: '确认执行字段与属性迁移' })); }
  await waitFor(() => expect(screen.getByRole('button', { name: '查询原迁移回执' })).toHaveProperty('disabled', false));
  const count = writes.length; const stored = localStorage.getItem(localStorage.key(0)!)!; expect(stored).not.toContain('value'); expect(stored).not.toContain('source_key');
  view.unmount(); render(<PropertyMigration space={space} scope="actor"/>); fireEvent.click(screen.getByRole('button', { name: '字段迁移计划' }));
  await waitFor(() => expect(screen.getByRole('button', { name: '查询原迁移回执' })).toHaveProperty('disabled', false));
  fireEvent.click(screen.getByRole('button', { name: '查询原迁移回执' }));
  await screen.findByText(/原阶段回执已确认/); expect(writes).toHaveLength(count);
});
it('keeps the disabled gate read-only and explains Markdown exclusion', async () => {
  enabled = false; render(<PropertyMigration space={space} scope="actor"/>); fireEvent.click(screen.getByRole('button', { name: '字段迁移计划' }));
  await screen.findByText(/未启用属性迁移/); expect(screen.queryByRole('button', { name: '生成迁移预览' })).toBeNull(); expect(writes).toEqual([]);
  expect(screen.getByText(/不支持 Markdown/)).toBeTruthy();
});
it('shows incompatible values without claiming a completed migration', async () => {
  plan.items[0] = { ...plan.items[0], after: '', status: 'failed', error: 'incompatible_value' };
  render(<PropertyMigration space={space} scope="actor"/>); await preview(); await screen.findByText(/incompatible_value/);
  expect(screen.queryByText(/迁移已完成/)).toBeNull(); expect(writes).toEqual(['preview']);
});
it.each(['offline', 'drivebridge-context'])('clears plan values and confirmation on %s', async event => {
  render(<PropertyMigration space={space} scope="actor"/>); await preview(); await screen.findByRole('button', { name: '确认此迁移计划' });
  fireEvent(window, new Event(event)); await screen.findByText(/迁移数据已清除/);
  expect(screen.queryByRole('button', { name: '确认此迁移计划' })).toBeNull(); expect(screen.queryByText('"value"', { selector: 'pre' })).toBeNull();
});

it('rejects mismatched original receipts and never sends a replacement preview', async () => {
  lost = 'preview'; render(<PropertyMigration space={space} scope="actor"/>); await preview();
  await waitFor(() => expect(screen.getByRole('button', { name: '查询原迁移回执' })).toHaveProperty('disabled', false));
  receipts.preview = { space_id: space, plan_id: planId, stage: 'preview', request_digest: '0'.repeat(64), result_revision: 1, committed_at: '2026-10-09T00:00:00Z' };
  fireEvent.click(screen.getByRole('button', { name: '查询原迁移回执' }));
  await waitFor(() => expect(screen.getByRole('button', { name: '查询原迁移回执' })).toHaveProperty('disabled', false));
  expect(writes).toEqual(['preview']); expect(screen.queryByRole('button', { name: '生成迁移预览' })).toBeNull();
  expect(localStorage.getItem(localStorage.key(0)!)).toContain('migration-');
});

it('allows original lookup with the editing gate off after a lost preview', async () => {
  lost = 'preview'; const view = render(<PropertyMigration space={space} scope="actor"/>); await preview();
  await waitFor(() => expect(screen.getByRole('button', { name: '查询原迁移回执' })).toHaveProperty('disabled', false));
  view.unmount(); enabled = false; plan.editing_enabled = false;
  render(<PropertyMigration space={space} scope="actor"/>); fireEvent.click(screen.getByRole('button', { name: '字段迁移计划' }));
  await waitFor(() => expect(screen.getByRole('button', { name: '查询原迁移回执' })).toHaveProperty('disabled', false));
  fireEvent.click(screen.getByRole('button', { name: '查询原迁移回执' })); await screen.findByText(/原阶段回执已确认/);
  expect(screen.queryByRole('button', { name: '确认此迁移计划' })).toBeNull(); expect(writes).toEqual(['preview']);
});

it('clears plans on forbidden responses rather than offering retry execution', async () => {
  vi.mocked(fetch).mockResolvedValue(new Response(JSON.stringify({ error: { code: 'forbidden' } }), { status: 403 }));
  render(<PropertyMigration space={space} scope="actor"/>); fireEvent.click(screen.getByRole('button', { name: '字段迁移计划' }));
  await screen.findByText(/迁移数据已清除/); expect(screen.queryByRole('button', { name: '重读迁移状态' })).toBeNull(); expect(writes).toEqual([]);
});

it('clears only a proven revision conflict and preserves configuration for reread', async () => {
  const transport = vi.mocked(fetch).getMockImplementation()!;
  vi.mocked(fetch).mockImplementation(async (url, options) => String(url).endsWith('/property-migrations') ? new Response(JSON.stringify({ error: { code: 'revision_conflict' } }), { status: 409 }) : transport(url, options));
  render(<PropertyMigration space={space} scope="actor"/>); await preview(); await screen.findByText(/修订冲突，本次未提交/);
  expect(localStorage.length).toBe(0); expect(screen.queryByRole('button', { name: '查询原迁移回执' })).toBeNull();
  fireEvent.click(screen.getByRole('button', { name: '重读迁移状态' })); expect(await screen.findByRole('textbox', { name: '目标字段 key' })).toHaveProperty('value', 'new');
});
