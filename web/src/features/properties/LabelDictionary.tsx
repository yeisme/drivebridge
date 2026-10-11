import { useEffect, useRef, useState } from 'react';
import { api, getDataContext } from '../../api';
import { type SessionView, verifySessionIdentity } from '../../app/session';
import { useRouteLeaveGuard } from '../../app/route-leave';
import type { components } from '../../generated/personal-workspace';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { Textarea } from '../../components/ui/textarea';
import { SelectAdapter } from '../../components/SelectAdapter';
import { AppDialog } from '../../components/AppDialog';
import { validDefinition } from './property-recovery';

type S = components['schemas'];
type Catalog = S['LabelDefinitionPageEnvelope']['data'];
type Update = S['LabelDefinitionUpdate'];
type Definition = Update['definitions'][number];
type Receipt = S['LabelDefinitionReceiptEnvelope']['data'];
type Plan = S['LabelMergePlanEnvelope']['data'];
type Preview = S['LabelMergePreview'];
type Recovery = { kind: 'dictionary' | 'preview' | 'confirm' | 'execute' | 'plan'; key: string; digest: string; expected: number; plan: string };
const phases: Record<string, string> = { previewed: '待确认合并', confirmed: '已确认合并，尚未执行', executing: '合并执行中', completed: '合并完成', partial: '部分合并，逐项查看结果' };
const itemLabels: Record<string, string> = { pending: '待执行', done: '完成', failed: '失败', stale: '来源已变化', skipped: '跳过' };
const isID = (value: string) => /^[a-f0-9]{48}$/.test(value);
async function digest(value: unknown) {
  const raw = JSON.stringify(value).replace(/[<>&\u2028\u2029]/g, ch => '\\u' + ch.charCodeAt(0).toString(16).padStart(4, '0'));
  return Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(raw))), b => b.toString(16).padStart(2, '0')).join('');
}
function stored(slot: string): Recovery | null {
  const raw = localStorage.getItem(slot); if (raw === null) return null;
  if (raw.length > 768) throw new Error('invalid_recovery');
  const row = JSON.parse(raw) as Recovery;
  if (!row || Object.keys(row).sort().join(',') !== 'digest,expected,key,kind,plan' || !['dictionary', 'preview', 'confirm', 'execute', 'plan'].includes(row.kind) || !/^labels-[a-f0-9]{48}$/.test(row.key) || !/^[a-f0-9]{64}$/.test(row.digest) || !Number.isSafeInteger(row.expected) || row.expected < 0 || row.expected >= Number.MAX_SAFE_INTEGER || typeof row.plan !== 'string' || (row.plan !== '' && !isID(row.plan)) || (['confirm', 'execute', 'plan'].includes(row.kind) && !row.plan)) throw new Error('invalid_recovery');
  return row;
}
export function LabelDictionary({ space, scope }: { space: string; scope: string }) {
  const [open, setOpen] = useState(false); const trigger = useRef<HTMLButtonElement | null>(null);
  return <><Button ref={trigger} variant="outline" className="min-h-11" onClick={() => setOpen(true)}>管理标签字典</Button>{open && <LabelSession key={JSON.stringify([scope, space])} space={space} scope={scope} close={() => { setOpen(false); trigger.current?.focus(); }}/>}</>;
}
function LabelSession({ space, scope, close }: { space: string; scope: string; close: () => void }) {
  const context = getDataContext();
  const [state, setState] = useState('loading'), [busy, setBusy] = useState(false), [attempt, setAttempt] = useState(0), [notice, setNotice] = useState('');
  const [catalog, setCatalog] = useState<Catalog | null>(null), [draft, setDraft] = useState<Definition[] | null>(null), [review, setReview] = useState(false);
  const [draftBase, setDraftBase] = useState<Catalog | null>(null);
  const [record, setRecord] = useState<Recovery | null>(null), [plan, setPlan] = useState<Plan | null>(null), [risk, setRisk] = useState(false), [shown, setShown] = useState(25);
  const [source, setSource] = useState(''), [target, setTarget] = useState(''), [refs, setRefs] = useState('');
  const controller = useRef<AbortController | null>(null), running = useRef(false), slot = useRef(''), actor = useRef({ user: '', instance: '' });
  const prefix = '/spaces/' + encodeURIComponent(space), dirty = !record && (!!draft || !!refs);
  useRouteLeaveGuard(dirty, '放弃未提交的标签配置并离开？');
  const deny = (abort: AbortController) => { abort.abort(); setCatalog(null); setDraft(null); setDraftBase(null); setPlan(null); setRecord(null); setRefs(''); setSource(''); setTarget(''); setReview(false); setRisk(false); setNotice(''); setState('denied'); };
  async function replace(before: Recovery | null, after: Recovery | null, signal: AbortSignal) {
    if (!navigator.locks?.request) throw new Error('lock_unavailable');
    await navigator.locks.request(slot.current, { signal }, () => { signal.throwIfAborted(); if (JSON.stringify(stored(slot.current)) !== JSON.stringify(before)) throw new Error('recovery_changed'); if (after) localStorage.setItem(slot.current, JSON.stringify(after)); else localStorage.removeItem(slot.current); });
  }
  async function readCatalog(signal: AbortSignal) {
    let current: Catalog | null = null, cursor = ''; const seen = new Set<string>();
    const definitions: Catalog['definitions'] = [];
    for (let index = 0; index < 4; index++) {
      const query = new URLSearchParams({ namespace: 'manual', limit: '50' }); if (cursor) query.set('cursor', cursor);
      const page = await api<Catalog>(prefix + '/label-definitions?' + query, 'GET', undefined, {}, signal); signal.throwIfAborted();
      if (!page || page.space_id !== space || page.namespace !== 'manual' || !Number.isSafeInteger(page.revision) || page.revision < 0 || page.revision >= Number.MAX_SAFE_INTEGER || typeof page.management_enabled !== 'boolean' || !Array.isArray(page.definitions) || page.definitions.length > 50 || (page.next_cursor !== undefined && typeof page.next_cursor !== 'string') || page.definitions.some(row => !row || !validDefinition({ key: row.key, display_name: row.display_name, group: row.group, type: 'text' }) || !Number.isSafeInteger(row.revision) || row.revision < 1 || row.revision > page.revision)) throw new Error('invalid_dictionary');
      if (current && (page.revision !== current.revision || page.management_enabled !== current.management_enabled)) throw new Error('dictionary_changed');
      definitions.push(...page.definitions);
      current = { ...page, definitions };
      if (new Set(current.definitions.map(row => row.key)).size !== current.definitions.length) throw new Error('duplicate_definition');
      if (!page.next_cursor) return current;
      if (seen.has(page.next_cursor)) throw new Error('cursor_loop'); cursor = page.next_cursor; seen.add(cursor);
    }
    throw new Error('dictionary_budget');
  }
  async function readPlan(id: string, identity: Recovery, signal: AbortSignal) {
    let current: Plan | null = null, cursor = ''; const seen = new Set<string>();
    const items: NonNullable<Plan['items']> = [];
    for (let index = 0; index < 4; index++) {
      const query = new URLSearchParams({ limit: '50' }); if (cursor) query.set('cursor', cursor);
      const page = await api<Plan>('/organization-plans/' + encodeURIComponent(id) + '?' + query, 'GET', undefined, {}, signal); signal.throwIfAborted();
      if (!page || page.plan_id !== id || page.space_id !== space || !Number.isSafeInteger(page.revision) || page.revision < 1 || !/^[a-f0-9]{64}$/.test(page.impact_digest) || !Object.hasOwn(phases, page.phase) || !Array.isArray(page.items) || page.items.length > 50 || !Number.isSafeInteger(page.items_total) || page.items_total < 1 || page.items_total > 200 || [page.items_done, page.items_stale, page.items_failed].some(value => !Number.isSafeInteger(value) || value < 0 || value > page.items_total) || (page.next_cursor !== undefined && typeof page.next_cursor !== 'string')) throw new Error('invalid_plan');
      if (current && (current.revision !== page.revision || current.impact_digest !== page.impact_digest || current.phase !== page.phase || current.items_total !== page.items_total || current.items_done !== page.items_done || current.items_stale !== page.items_stale || current.items_failed !== page.items_failed)) throw new Error('plan_changed');
      items.push(...page.items);
      current = { ...page, items };
      if (!page.next_cursor) break;
      if (seen.has(page.next_cursor)) throw new Error('cursor_loop'); cursor = page.next_cursor; seen.add(cursor);
    }
    if (!current || current.next_cursor || !Array.isArray(current.items) || current.items.length !== current.items_total) throw new Error('incomplete_plan');
    const details: Preview[] = []; const fixed: string[] = [];
    for (const item of current.items) {
      if (!isID(item.file_id) || !isID(item.version_id) || item.action !== 'merge_label' || typeof item.path !== 'string' || typeof item.action_detail !== 'string' || !Object.hasOwn(itemLabels, item.status)) throw new Error('invalid_merge_item');
      const q = JSON.parse(item.action_detail) as Preview;
      if (!q || q.namespace !== 'manual' || typeof q.source_key !== 'string' || typeof q.target_key !== 'string' || !Number.isSafeInteger(q.expected_revision) || !Number.isSafeInteger(q.source_revision) || !Number.isSafeInteger(q.target_revision)) throw new Error('invalid_merge_intent');
      details.push(q); fixed.push(`drivebridge://${actor.current.instance}/file/${item.file_id}/version/${item.version_id}`);
    }
    if (new Set(current.items.map(item => item.file_id)).size !== current.items.length || details.some(q => JSON.stringify(q) !== JSON.stringify(details[0]))) throw new Error('merge_intent_changed');
    const q = details[0];
    const original: Preview = { namespace: 'manual', source_key: q.source_key, target_key: q.target_key, expected_revision: q.expected_revision, source_revision: q.source_revision, target_revision: q.target_revision, refs: fixed.sort() };
    if (await digest({ space_id: space, request: original }) !== identity.digest) throw new Error('merge_fingerprint_mismatch');
    if (current.items_done !== current.items.filter(item => item.status === 'done').length || current.items_stale !== current.items.filter(item => item.status === 'stale').length || current.items_failed !== current.items.filter(item => item.status === 'failed').length || (current.phase === 'completed' && current.items_done !== current.items_total)) throw new Error('invalid_outcome_counts');
    signal.throwIfAborted(); return current;
  }
  useEffect(() => {
    const abort = new AbortController(); controller.current = abort; setState('loading'); setCatalog(null); setPlan(null); setReview(false); setRisk(false);
    const invalidate = () => deny(abort); const storage = (event: StorageEvent) => { if (event.key === null || event.key === slot.current) invalidate(); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('offline', invalidate); window.addEventListener('storage', storage);
    async function load() {
      try {
        if (!navigator.onLine) { invalidate(); return; }
        const session = await api<SessionView>('/me', 'GET', undefined, {}, abort.signal); abort.signal.throwIfAborted();
        if (!session?.user?.id || !session.instance_id) throw new Error('invalid_session'); actor.current = { user: session.user.id, instance: session.instance_id };
        slot.current = 'drivebridge:label-management:' + JSON.stringify([session.instance_id, session.user.id, scope, context.tenant, context.space, space]);
        const saved = stored(slot.current); setRecord(saved);
        const value = await readCatalog(abort.signal); abort.signal.throwIfAborted(); setCatalog(value); setSource(value.definitions[0]?.key ?? ''); setTarget(value.definitions[1]?.key ?? '');
        if (saved?.kind === 'plan') { const value = await readPlan(saved.plan, saved, abort.signal); abort.signal.throwIfAborted(); setPlan(value); }
        setState('ready');
      } catch (error) { if (abort.signal.aborted) return; const status = error && typeof error === 'object' && 'status' in error ? error.status : undefined; if (status === 401 || status === 403) invalidate(); else { setCatalog(null); setPlan(null); setState('error'); } }
    }
    void load(); return () => { abort.abort(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', invalidate); window.removeEventListener('storage', storage); };
  }, [space, scope, context.tenant, context.space, attempt, prefix]);
  async function acceptDictionary(value: Receipt, current: Recovery, signal: AbortSignal) {
    if (!value || value.space_id !== space || value.namespace !== 'manual' || value.request_digest !== current.digest || value.result_revision !== current.expected + 1 || typeof value.committed_at !== 'string' || !Number.isFinite(Date.parse(value.committed_at))) throw new Error('invalid_receipt');
    await replace(current, null, signal); signal.throwIfAborted(); setRecord(null); setDraft(null); setDraftBase(null); setReview(false); setNotice('原标签字典回执已确认；不会重命名现有文件标签。'); setAttempt(value => value + 1);
  }
  async function acceptPlan(id: string, current: Recovery, signal: AbortSignal, direct = false) {
    if (!isID(id) || (current.plan && current.plan !== id)) throw new Error('invalid_plan_identity');
    const value = await readPlan(id, current, signal); signal.throwIfAborted(); setPlan(value); setShown(25); setState('ready');
    const observed = current.kind === 'confirm' ? ['confirmed', 'executing', 'completed', 'partial'].includes(value.phase) : current.kind === 'execute' ? ['completed', 'partial'].includes(value.phase) : true;
    if (direct || observed) { const next: Recovery = { ...current, kind: 'plan', plan: id, expected: value.revision }; await replace(current, next, signal); signal.throwIfAborted(); setRecord(next); setRefs(''); setNotice('已读取原合并计划当前事实；确认／执行没有幂等回执，不把当前状态冒充原请求回执。'); }
    else setNotice('原计划仍未出现可确认的阶段结果。保持原计划，只能重读，不重复提交未知操作。');
  }
  async function action(kind: 'dictionary' | 'preview' | 'confirm' | 'execute' | 'lookup') {
    const abort = controller.current;
    if (!abort || abort.signal.aborted || running.current || !navigator.onLine || (kind !== 'lookup' && (state !== 'ready' || !catalog?.management_enabled || (record && record.kind !== 'plan')))) return;
    running.current = true; setBusy(true); setNotice(''); let current = record, sent = false;
    try {
      await verifySessionIdentity(actor.current.user, actor.current.instance, abort.signal);
      if (kind === 'lookup') {
        if (!current || JSON.stringify(stored(slot.current)) !== JSON.stringify(current)) throw new Error('recovery_changed');
        if (current.kind === 'dictionary') await acceptDictionary(await api<Receipt>(prefix + '/label-definitions/lookup', 'POST', { idempotency_key: current.key } satisfies S['IdempotencyLookupRequest'], {}, abort.signal), current, abort.signal);
        else if (current.kind === 'preview') { const value = await api<Plan>(prefix + '/label-merge-plans/lookup', 'POST', { idempotency_key: current.key } satisfies S['IdempotencyLookupRequest'], {}, abort.signal); await acceptPlan(value.plan_id, current, abort.signal); }
        else await acceptPlan(current.plan, current, abort.signal);
      } else if (kind === 'dictionary' || kind === 'preview') {
        if (current || !catalog) return;
        let body: Update | Preview;
        if (kind === 'dictionary') {
          if (!review || !draft || !draftBase || draft.length > 200 || draft.some(row => !validDefinition({ ...row, type: 'text' })) || new Set(draft.map(row => row.key)).size !== draft.length) throw new Error('invalid_definitions');
          body = { expected_revision: draftBase.revision, definitions: draft.map(row => ({ key: row.key, display_name: row.display_name, group: row.group })) };
        } else {
          const from = catalog.definitions.find(row => row.key === source), to = catalog.definitions.find(row => row.key === target), fixed = refs.split(/\r?\n/).map(value => value.trim()).filter(Boolean).sort();
          if (!from || !to || from.key === to.key || !fixed.length || fixed.length > 200 || new Set(fixed).size !== fixed.length) throw new Error('invalid_selection');
          const files = new Set<string>();
          for (const raw of fixed) { const url = new URL(raw); const parts = url.pathname.split('/'); if (url.protocol !== 'drivebridge:' || url.host !== actor.current.instance || url.username || url.password || url.search || url.hash || raw !== url.href || parts.length !== 5 || parts[1] !== 'file' || parts[3] !== 'version' || !isID(parts[2]) || !isID(parts[4]) || files.has(parts[2])) throw new Error('invalid_fixed_ref'); files.add(parts[2]); }
          body = { namespace: 'manual', source_key: from.key, target_key: to.key, expected_revision: catalog.revision, source_revision: from.revision, target_revision: to.revision, refs: fixed };
        }
        const next: Recovery = { kind, key: 'labels-' + Array.from(crypto.getRandomValues(new Uint8Array(24)), byte => byte.toString(16).padStart(2, '0')).join(''), digest: await digest({ space_id: space, request: body }), expected: body.expected_revision, plan: '' };
        await replace(null, next, abort.signal); current = next; setRecord(next); setReview(false); sent = true;
        if (kind === 'dictionary') await acceptDictionary(await api<Receipt>(prefix + '/label-definitions', 'PUT', body, { 'Idempotency-Key': next.key }, abort.signal), next, abort.signal);
        else { const value = await api<Plan>(prefix + '/label-merge-plans', 'POST', body, { 'Idempotency-Key': next.key }, abort.signal); await acceptPlan(value.plan_id, next, abort.signal); }
      } else {
        if (!plan || !current || current.kind !== 'plan' || (kind === 'confirm' ? plan.phase !== 'previewed' : plan.phase !== 'confirmed' || !risk)) return;
        const next: Recovery = { ...current, kind, expected: plan.revision }; await replace(current, next, abort.signal); current = next; setRecord(next); setRisk(false); sent = true;
        const body = kind === 'confirm' ? { expected_revision: plan.revision, impact_digest: plan.impact_digest } satisfies S['LabelMergeConfirm'] : { expected_revision: plan.revision } satisfies S['LabelMergeExecute'];
        const value = await api<Plan>('/organization-plans/' + encodeURIComponent(plan.plan_id) + '/' + kind, 'POST', body, {}, abort.signal);
        await acceptPlan(value.plan_id, next, abort.signal, true);
      }
    } catch (error) {
      if (abort.signal.aborted) return; const status = error && typeof error === 'object' && 'status' in error ? error.status : undefined;
      if (status === 401 || status === 403) deny(abort);
      else if (sent && current && status === 409 && error instanceof Error && error.message === 'revision_conflict' && ['dictionary', 'preview', 'confirm'].includes(current.kind)) {
        const next = current.plan ? { ...current, kind: 'plan' as const } : null;
        try { await replace(current, next, abort.signal); abort.signal.throwIfAborted(); setRecord(next); setPlan(null); setCatalog(null); setState('error'); setNotice('标签修订冲突，本次未提交。请重读原字典或计划后重新审阅，不自动覆盖。'); } catch { if (!abort.signal.aborted) setNotice('恢复身份变化，只查询原请求。'); }
      } else { if (current) { setPlan(null); setState('error'); } setNotice(current ? '结果未知或合同不匹配；保留原键／原计划，只查询，不自动重发。' : '未发送修改：请检查唯一 key、显示名、固定引用、来源目标及恢复存储。'); }
    } finally { running.current = false; if (!abort.signal.aborted) setBusy(false); }
  }
  async function forgetPlan() {
    const signal = controller.current?.signal;
    if (!signal || signal.aborted || !record || record.kind !== 'plan' || running.current || !window.confirm('只移除本浏览器的合并计划入口，不取消服务端计划、不回滚已完成文件。继续？')) return;
    try { await replace(record, null, signal); signal.throwIfAborted(); setRecord(null); setPlan(null); setRefs(''); setAttempt(value => value + 1); } catch { if (!signal.aborted) setNotice('不能移除已变化的恢复身份。'); }
  }
  const changeDefinition = (index: number, change: Partial<Definition>) => setDraft(rows => rows?.map((row, i) => i === index ? { ...row, ...change } : row) ?? null);
  return <AppDialog title="标签字典与所选文件合并" close={() => { if (!dirty || window.confirm('放弃未提交的标签配置？')) close(); }}>
    <p className="text-sm">仅管理 manual 标签定义。合并只移动明确选定文件的 manual 来源标签，不删除字典身份、不处理全空间、不改变 rule／suggested 或其他标签。</p>
    {state === 'loading' && <p role="status">读取完整授权标签字典…</p>}{state === 'error' && <p role="alert">标签读取失败或修订冲突，不能使用局部或旧数据继续写入。</p>}{state === 'denied' && <p role="alert">权限、范围或网络变化，标签数据已清除。</p>}
    {catalog && <><p>标签字典修订 {catalog.revision} · {catalog.definitions.length}/200 个定义</p>{!catalog.management_enabled && <p role="status">服务未启用标签管理，当前只读；原请求仍可查询。</p>}
      {!draft && <ul className="m-0 list-none space-y-2 p-0">{catalog.definitions.map(row => <li key={row.key} className="break-all text-sm">{row.display_name} · {row.key} · {row.group || '未分组'} · 修订 {row.revision}</li>)}</ul>}
      {!catalog.definitions.length && <p role="status">当前空间尚无 manual 标签定义，既有自由标签不会自动导入。</p>}
      {catalog.management_enabled && !record && !draft && <Button variant="outline" className="min-h-11" disabled={busy} onClick={() => { setDraftBase(catalog); setDraft(catalog.definitions.map(row => ({ key: row.key, display_name: row.display_name, group: row.group }))); }}>编辑标签定义</Button>}
      {draft && draftBase && catalog.revision !== draftBase.revision && <p role="alert">字典已变化，草稿仍绑定原修订 {draftBase.revision}，不会自动合并覆盖。请保留草稿处理冲突，或明确放弃后重新编辑。</p>}
      {draft && draftBase && !record && !review && <div className="space-y-3">{draft.map((row, index) => <fieldset key={index} className="space-y-2 rounded border p-3"><legend>标签 {index + 1}</legend><Input aria-label={`标签 ${index + 1} 稳定 key`} value={row.key} disabled={busy || index < draftBase.definitions.length} onChange={event => changeDefinition(index, { key: event.target.value })}/><Input aria-label={`标签 ${index + 1} 显示名`} value={row.display_name} disabled={busy} onChange={event => changeDefinition(index, { display_name: event.target.value })}/><Input aria-label={`标签 ${index + 1} 分组`} value={row.group} disabled={busy} onChange={event => changeDefinition(index, { group: event.target.value })}/>{index >= draftBase.definitions.length && <Button variant="outline" className="min-h-11" onClick={() => setDraft(rows => rows?.filter((_, i) => i !== index) ?? null)}>移除新增标签 {index + 1}</Button>}</fieldset>)}<Button variant="outline" className="min-h-11" disabled={busy || draft.length >= 200} onClick={() => setDraft(rows => [...(rows ?? []), { key: '', display_name: '', group: '' }])}>添加标签定义</Button><Button className="min-h-11" disabled={busy} onClick={() => { if (draft.every(row => validDefinition({ ...row, type: 'text' })) && new Set(draft.map(row => row.key)).size === draft.length) { setReview(true); setNotice(''); } else setNotice('请检查唯一稳定 key、非空显示名与分组。'); }}>预览标签字典修改</Button><Button variant="ghost" className="min-h-11" onClick={() => { setDraft(null); setDraftBase(null); }}>放弃标签字典草稿</Button></div>}
      {review && draft && draftBase && !record && <div className="space-y-2"><p>按修订 {draftBase.revision} 保存完整定义；只改显示信息和新增 key，不删除已有定义、不更改文件标签。</p>{draft.map(row => { const old = draftBase.definitions.find(item => item.key === row.key); return <p key={row.key} className="break-all text-sm">{row.key}：{old ? old.display_name + ' / ' + old.group : '新增'} → {row.display_name} / {row.group}</p>; })}<Button className="min-h-11" disabled={busy} onClick={() => void action('dictionary')}>确认保存标签字典</Button><Button variant="outline" className="min-h-11" disabled={busy} onClick={() => setReview(false)}>返回标签修改</Button></div>}
      {catalog.management_enabled && !record && !draft && catalog.definitions.length >= 2 && <div className="space-y-3 border-t pt-3"><h3>所选文件标签合并</h3><label className="grid gap-2">来源标签<SelectAdapter aria-label="合并来源标签" value={source} onChange={event => setSource(event.target.value)}>{catalog.definitions.map(row => <option key={row.key} value={row.key}>{row.display_name} · {row.key}</option>)}</SelectAdapter></label><label className="grid gap-2">目标标签<SelectAdapter aria-label="合并目标标签" value={target} onChange={event => setTarget(event.target.value)}>{catalog.definitions.map(row => <option key={row.key} value={row.key}>{row.display_name} · {row.key}</option>)}</SelectAdapter></label><label className="grid gap-2">所选文件固定引用<Textarea aria-label="所选文件固定引用" className="min-h-28 w-full rounded border bg-background p-2 text-sm" value={refs} onChange={event => setRefs(event.target.value)} maxLength={100000} /></label><p className="text-xs">从文件详情复制固定版本引用，每行一个，最多 200 个。同一文件不可重复；服务端重新验证当前版本、manual 来源标签与权限。</p><Button className="min-h-11" disabled={busy || !refs.trim() || source === target} onClick={() => void action('preview')}>生成所选文件合并预览</Button></div>}
    </>}
    {plan && <div className="space-y-3"><p role="status">{phases[plan.phase]}</p><p className="break-all text-xs">原计划 {plan.plan_id} · 修订 {plan.revision} · 影响摘要 {plan.impact_digest}</p><p>{plan.items_total} 项 · 完成 {plan.items_done} · 变化 {plan.items_stale} · 失败 {plan.items_failed}。只代表此选择集，不是全局合并。</p><ul className="m-0 list-none space-y-3 p-0">{plan.items?.slice(0, shown).map(item => <li key={item.item_key} className="rounded border p-3 text-sm"><p className="break-all">{item.path}</p><p className="break-all text-xs">{item.file_id} / {item.version_id}</p><p>{itemLabels[item.status]}{item.error ? ' · ' + item.error : ''}</p><p className="break-all text-xs">原标签集合摘要：{item.before || '未提供'}（不是标签正文）</p><p>合并到 manual:{item.after}</p></li>)}</ul>{shown < (plan.items?.length ?? 0) && <Button variant="outline" className="min-h-11" onClick={() => setShown(value => value + 25)}>展开更多合并项</Button>}
      {record?.kind === 'plan' && catalog?.management_enabled && plan.phase === 'previewed' && <Button className="min-h-11" disabled={busy} onClick={() => void action('confirm')}>确认所选文件合并计划</Button>}
      {record?.kind === 'plan' && catalog?.management_enabled && plan.phase === 'confirmed' && <Button variant="destructive" className="min-h-11" disabled={busy} onClick={() => setRisk(true)}>执行所选文件合并</Button>}
      {risk && <div role="alert"><p>执行仅移除所选文件的来源 manual 标签并加入目标，其他标签和字典身份保留。可能部分完成，不自动回滚。确认执行？</p><Button variant="destructive" className="min-h-11" disabled={busy} onClick={() => void action('execute')}>确认执行标签合并</Button><Button variant="outline" className="min-h-11" disabled={busy} onClick={() => setRisk(false)}>暂不执行合并</Button></div>}
    </div>}
    {state !== 'denied' && <div className="flex flex-wrap gap-2">{record && <Button className="min-h-11" disabled={busy || state === 'loading'} onClick={() => void action('lookup')}>{['dictionary', 'preview'].includes(record.kind) ? '查询原标签请求' : '重读原合并计划'}</Button>}<Button variant="outline" className="min-h-11" disabled={busy || state === 'loading'} onClick={() => setAttempt(value => value + 1)}>重读标签字典</Button>{record?.kind === 'plan' && <Button variant="ghost" className="min-h-11" disabled={busy} onClick={() => void forgetPlan()}>移除本地合并入口（不取消计划）</Button>}</div>}
    {notice && <p role="status">{notice}</p>}
  </AppDialog>;
}
