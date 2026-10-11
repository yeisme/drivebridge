import { useEffect, useRef, useState } from 'react';
import { api, getDataContext } from '../../api';
import { type SessionView, verifySessionIdentity } from '../../app/session';
import { useRouteLeaveGuard } from '../../app/route-leave';
import type { components } from '../../generated/personal-workspace';
import { AppDialog } from '../../components/AppDialog';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { SelectAdapter } from '../../components/SelectAdapter';
import { TYPES, typeLabel, validCatalog, type Catalog } from './property-recovery';

type Schemas = components['schemas'];
type Capability = Schemas['PropertyMigrationCapabilityEnvelope']['data'];
type Plan = Schemas['PropertyMigrationViewEnvelope']['data'];
type Preview = Schemas['PropertyMigrationPreview'];
type Stage = Schemas['PropertyMigrationLookup']['stage'];
type Receipt = Schemas['PropertyMigrationReceiptEnvelope']['data'];
type Pending = { stage: Stage; key: string; digest: string; expected: number };
type Recovery = { plan: string; pending: Pending | null };
const policyLabels: Record<Preview['conversion_policy'], string> = { preserve: '原类型原值保留（仅改 key）', parse_text: '按目标类型严格解析文本', scalar_text: '标量转文本', singleton_list: '单个文本转列表' };
const phaseLabels: Record<Plan['phase'], string> = { previewed: '待人工确认', confirmed: '已确认，尚未执行', completed: '迁移已完成', partial: '迁移受阻，未切换字典或属性值' };
const statusLabels: Record<Plan['items'][number]['status'], string> = { pending: '待执行', done: '完成', failed: '失败', stale: '来源已变化' };
const isID = (value: string) => /^[a-f0-9]{48}$/.test(value);
function recoveryAt(slot: string): Recovery | null {
  const raw = localStorage.getItem(slot); if (raw === null) return null;
  if (raw.length > 768) throw new Error('invalid_recovery');
  const row = JSON.parse(raw) as Recovery;
  if (!row || Object.keys(row).sort().join(',') !== 'pending,plan' || typeof row.plan !== 'string' || (row.plan !== '' && !isID(row.plan)) || (!row.plan && !row.pending)) throw new Error('invalid_recovery');
  if (row.pending && (Object.keys(row.pending).sort().join(',') !== 'digest,expected,key,stage' || !['preview', 'confirm', 'execute'].includes(row.pending.stage) || !/^migration-[a-f0-9]{48}$/.test(row.pending.key) || !/^[a-f0-9]{64}$/.test(row.pending.digest) || !Number.isSafeInteger(row.pending.expected) || row.pending.expected < 0 || row.pending.expected >= Number.MAX_SAFE_INTEGER || (row.pending.stage !== 'preview' && !row.plan))) throw new Error('invalid_recovery');
  return row;
}
export function PropertyMigration({ space, scope }: { space: string; scope: string }) {
  const [open, setOpen] = useState(false); const trigger = useRef<HTMLButtonElement | null>(null);
  return <><Button ref={trigger} variant="outline" className="min-h-11" onClick={() => setOpen(true)}>字段迁移计划</Button>{open && <MigrationSession key={JSON.stringify([scope, space])} space={space} scope={scope} close={() => { setOpen(false); trigger.current?.focus(); }}/>}</>;
}
function MigrationSession({ space, scope, close }: { space: string; scope: string; close: () => void }) {
  const context = getDataContext();
  const [state, setState] = useState('loading'), [attempt, setAttempt] = useState(0), [busy, setBusy] = useState(false), [notice, setNotice] = useState('');
  const [catalog, setCatalog] = useState<Catalog | null>(null), [capability, setCapability] = useState<Capability | null>(null), [plan, setPlan] = useState<Plan | null>(null);
  const [record, setRecord] = useState<Recovery | null>(null), [source, setSource] = useState(''), [target, setTarget] = useState(''), [type, setType] = useState<Preview['target_type']>('text'), [policy, setPolicy] = useState<Preview['conversion_policy']>('preserve'), [risk, setRisk] = useState(false), [shown, setShown] = useState(25);
  const active = useRef<AbortController | null>(null), running = useRef(false), slot = useRef(''), actor = useRef({ user: '', instance: '' });
  const path = '/spaces/' + encodeURIComponent(space) + '/property-migrations';
  const dirty = !record && target !== '';
  useRouteLeaveGuard(dirty, '放弃未提交的字段迁移配置并离开？');
  const invalidate = (abort: AbortController) => { abort.abort(); setCatalog(null); setCapability(null); setPlan(null); setRecord(null); setSource(''); setTarget(''); setRisk(false); setNotice(''); setState('denied'); };
  useEffect(() => {
    const abort = new AbortController(); active.current = abort; setCatalog(null); setCapability(null); setPlan(null); setRisk(false); setState('loading');
    const denied = () => invalidate(abort);
    const storage = (event: StorageEvent) => { if (event.key === null || event.key === slot.current) denied(); };
    window.addEventListener('drivebridge-context', denied); window.addEventListener('offline', denied); window.addEventListener('storage', storage);
    async function load() {
      try {
        if (!navigator.onLine) { denied(); return; }
        const session = await api<SessionView>('/me', 'GET', undefined, {}, abort.signal); abort.signal.throwIfAborted();
        if (!session?.user?.id || !session.instance_id) throw new Error('invalid_session');
        actor.current = { user: session.user.id, instance: session.instance_id };
        slot.current = 'drivebridge:property-migration:' + JSON.stringify([session.instance_id, session.user.id, scope, context.tenant, context.space, space]);
        const stored = recoveryAt(slot.current); setRecord(stored);
        const cap = await api<Capability>(path + '/capability', 'GET', undefined, {}, abort.signal); abort.signal.throwIfAborted();
        if (!cap || cap.space_id !== space || typeof cap.editing_enabled !== 'boolean' || cap.storage_kind !== 'sidecar' || cap.markdown_supported !== false || !Number.isSafeInteger(cap.max_sidecars) || cap.max_sidecars < 1 || cap.max_sidecars > 200 || !Array.isArray(cap.conversion_policies) || cap.conversion_policies.some(value => !Object.hasOwn(policyLabels, value))) throw new Error('invalid_capability');
        setCapability(cap);
        if (stored?.pending) { setState('ready'); return; }
        if (stored?.plan) await loadPlan(stored.plan, abort.signal);
        else {
          const value = await api<Catalog>('/spaces/' + encodeURIComponent(space) + '/property-definitions', 'GET', undefined, {}, abort.signal); abort.signal.throwIfAborted();
          if (!validCatalog(value, space)) throw new Error('invalid_catalog');
          setCatalog(value); setSource(old => value.definitions.some(row => row.key === old) ? old : value.definitions[0]?.key ?? '');
        }
        setState('ready');
      } catch (error) {
        if (abort.signal.aborted) return;
        const status = error && typeof error === 'object' && 'status' in error ? error.status : undefined;
        if (status === 401 || status === 403) denied(); else { setPlan(null); setCatalog(null); setState('error'); }
      }
    }
    void load();
    return () => { abort.abort(); window.removeEventListener('drivebridge-context', denied); window.removeEventListener('offline', denied); window.removeEventListener('storage', storage); };
  }, [space, scope, context.tenant, context.space, attempt, path]);
  async function loadPlan(id: string, signal: AbortSignal) {
    const value = await api<Plan>('/property-migrations/' + encodeURIComponent(id), 'GET', undefined, {}, signal); signal.throwIfAborted();
    if (!value || value.plan_id !== id || value.space_id !== space || !Number.isSafeInteger(value.revision) || value.revision < 1 || !/^[a-f0-9]{64}$/.test(value.impact_digest) || !Object.hasOwn(phaseLabels, value.phase) || typeof value.editing_enabled !== 'boolean' || !value.request || !TYPES.includes(value.request.target_type) || !Object.hasOwn(policyLabels, value.request.conversion_policy) || typeof value.request.source_key !== 'string' || typeof value.request.target_key !== 'string' || !Array.isArray(value.items) || value.items.length > 200 || value.items.some(row => !row || !isID(row.file_id) || !isID(row.version_id) || !Number.isSafeInteger(row.sidecar_revision) || row.sidecar_revision < 0 || typeof row.before !== 'string' || typeof row.after !== 'string' || typeof row.error !== 'string' || !Object.hasOwn(statusLabels, row.status)) || new Set(value.items.map(row => row.file_id + ':' + row.version_id)).size !== value.items.length) throw new Error('invalid_plan');
    setPlan(value); setShown(25); setRisk(false);
  }
  async function replaceRecord(before: Recovery | null, after: Recovery | null, signal: AbortSignal) {
    if (!navigator.locks?.request) throw new Error('lock_unavailable');
    await navigator.locks.request(slot.current, { signal }, () => { signal.throwIfAborted(); if (JSON.stringify(recoveryAt(slot.current)) !== JSON.stringify(before)) throw new Error('recovery_changed'); if (after) localStorage.setItem(slot.current, JSON.stringify(after)); else localStorage.removeItem(slot.current); });
  }
  async function submit(stage: Stage | 'lookup') {
    const abort = active.current;
    if (!abort || abort.signal.aborted || running.current || !navigator.onLine || (stage !== 'lookup' && (state !== 'ready' || !capability?.editing_enabled || record?.pending)) || (stage === 'lookup' && !record?.pending)) return;
    running.current = true; setBusy(true); setNotice(''); let current = record, sent = false;
    try {
      await verifySessionIdentity(actor.current.user, actor.current.instance, abort.signal);
      if (stage !== 'lookup') {
        let input: Preview | Schemas['PropertyMigrationDecision'];
        if (stage === 'preview') {
          if (!catalog || !source || !/^[a-z][a-z0-9_]{0,63}$/.test(target) || !capability?.conversion_policies.includes(policy)) throw new Error('invalid_form');
          input = { expected_definition_revision: catalog.revision, source_key: source, target_key: target, target_type: type, conversion_policy: policy };
        } else {
          if (!plan || !plan.editing_enabled || (stage === 'confirm' ? plan.phase !== 'previewed' : plan.phase !== 'confirmed' || !risk)) return;
          input = { expected_revision: plan.revision, impact_digest: plan.impact_digest };
        }
        const raw = JSON.stringify({ resource_id: stage === 'preview' ? space : plan!.plan_id, request: input }).replace(/[<>&\u2028\u2029]/g, ch => '\\u' + ch.charCodeAt(0).toString(16).padStart(4, '0'));
        const digest = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(raw))), byte => byte.toString(16).padStart(2, '0')).join('');
        abort.signal.throwIfAborted();
        const next: Recovery = { plan: stage === 'preview' ? '' : plan!.plan_id, pending: { stage, key: 'migration-' + Array.from(crypto.getRandomValues(new Uint8Array(24)), byte => byte.toString(16).padStart(2, '0')).join(''), digest, expected: stage === 'preview' ? 0 : plan!.revision } };
        await replaceRecord(current, next, abort.signal); current = next; setRecord(next); setRisk(false); sent = true;
        await accept(await api<Receipt>(stage === 'preview' ? path : '/property-migrations/' + encodeURIComponent(next.plan) + '/' + stage, 'POST', input, { 'Idempotency-Key': next.pending!.key }, abort.signal), next, abort.signal);
      } else if (current?.pending) {
        if (JSON.stringify(recoveryAt(slot.current)) !== JSON.stringify(current)) throw new Error('recovery_changed');
        await accept(await api<Receipt>(path + '/lookup', 'POST', { stage: current.pending.stage, idempotency_key: current.pending.key } satisfies Schemas['PropertyMigrationLookup'], {}, abort.signal), current, abort.signal);
      }
    } catch (error) {
      if (abort.signal.aborted) return;
      const status = error && typeof error === 'object' && 'status' in error ? error.status : undefined;
      if (status === 401 || status === 403) invalidate(abort);
      else if (sent && status === 409 && error instanceof Error && error.message === 'revision_conflict' && current) {
        const next = current.plan ? { plan: current.plan, pending: null } : null;
        try { await replaceRecord(current, next, abort.signal); if (!abort.signal.aborted) { setRecord(next); setPlan(null); setState('error'); setNotice('修订冲突，本次未提交。保留原计划或配置，请手动重读后重新审阅。'); } }
        catch { if (!abort.signal.aborted) setNotice('恢复身份变化，只查询原阶段回执。'); }
      } else { setRisk(false); setNotice(current?.pending ? '原阶段结果未知或回执不匹配。只查询原迁移回执，不自动重放或更换请求键；未找到不代表未提交。' : '无法安全创建迁移请求，请检查目标 key、恢复存储与跨标签锁。'); }
    } finally { running.current = false; if (!abort.signal.aborted) setBusy(false); }
  }
  async function accept(value: Receipt, current: Recovery, signal: AbortSignal) {
    signal.throwIfAborted(); const pending = current.pending;
    if (!pending || !value || value.space_id !== space || !isID(value.plan_id) || (current.plan && value.plan_id !== current.plan) || value.stage !== pending.stage || value.request_digest !== pending.digest || value.result_revision !== pending.expected + 1 || typeof value.committed_at !== 'string' || !Number.isFinite(Date.parse(value.committed_at))) throw new Error('receipt_mismatch');
    const next: Recovery = { plan: value.plan_id, pending: null };
    await replaceRecord(current, next, signal); signal.throwIfAborted(); setRecord(next); setTarget(''); setNotice('原阶段回执已确认；以下重新读取原计划，不以当前值猜测历史结果。');
    try { await loadPlan(value.plan_id, signal); setState('ready'); }
    catch (error) {
      if (signal.aborted) return;
      const status = error && typeof error === 'object' && 'status' in error ? error.status : undefined;
      if ((status === 401 || status === 403) && active.current) invalidate(active.current);
      else { setPlan(null); setState('error'); setNotice('原阶段回执已确认，但原计划读取失败。保留计划 ID，请手动重读，不重放操作。'); }
    }
  }
  async function finish() {
    const signal = active.current?.signal;
    if (!signal || signal.aborted || running.current || !record || record.pending || !plan || !['completed', 'partial'].includes(plan.phase)) return;
    try { await replaceRecord(record, null, signal); signal.throwIfAborted(); close(); } catch { if (!signal.aborted) setNotice('恢复身份已变化，不能清除其他页面的计划。'); }
  }
  async function leavePlan() {
    const signal = active.current?.signal;
    if (!signal || signal.aborted || running.current || !record || record.pending || !window.confirm('仅移除本浏览器的计划入口，不取消服务端计划、不回滚执行，也不自动创建新计划。继续？')) return;
    running.current = true; setBusy(true);
    try { await replaceRecord(record, null, signal); signal.throwIfAborted(); setRecord(null); setPlan(null); setTarget(''); setNotice('已移除本浏览器的计划入口，服务端原计划仍保留。'); setAttempt(value => value + 1); }
    catch { if (!signal.aborted) setNotice('恢复身份已变化，不能移除其他页面的计划。'); }
    finally { running.current = false; if (!signal.aborted) setBusy(false); }
  }
  return <AppDialog title="字段 key 与类型迁移" close={() => { if (!dirty || window.confirm('放弃未提交的字段迁移配置？')) close(); }}>
    <p className="text-sm">仅迁移二进制 sidecar 属性，不支持 Markdown／YAML／受管项目资产。预览会保存计划但不改变字典和值；确认不执行，执行需再次确认。</p>
    {state === 'loading' && <p role="status">读取迁移能力与原计划…</p>}{state === 'denied' && <p role="alert">权限、网络或范围变化，迁移数据已清除，请重新进入。</p>}{state === 'error' && <p role="alert">迁移读取失败或已冲突，不能使用旧预览继续执行。</p>}
    {capability && !capability.editing_enabled && <p role="status">服务未启用属性迁移；当前只读，原阶段回执仍可查询。</p>}
    {state === 'ready' && catalog && !record && capability?.editing_enabled && <div className="space-y-3">
      <p>字典修订 {catalog.revision} · 最多 {capability.max_sidecars} 个 sidecar；超过上限由服务端拒绝，不截取部分假装完整。</p>
      {!catalog.definitions.length ? <p role="status">没有可迁移属性定义。</p> : <>
        <label className="grid gap-2">原字段<SelectAdapter aria-label="原字段" value={source} disabled={busy} onChange={event => setSource(event.target.value)}>{catalog.definitions.map(row => <option key={row.key} value={row.key}>{row.display_name} · {row.key} · {typeLabel[row.type]}</option>)}</SelectAdapter></label>
        <label className="grid gap-2">目标字段 key<Input aria-label="目标字段 key" value={target} maxLength={64} disabled={busy} onChange={event => setTarget(event.target.value)}/></label>
        <label className="grid gap-2">目标类型<SelectAdapter aria-label="迁移目标类型" value={type} disabled={busy} onChange={event => { const next = TYPES.find(value => value === event.target.value); if (next) setType(next); }}>{TYPES.map(value => <option key={value} value={value}>{typeLabel[value]}</option>)}</SelectAdapter></label>
        <label className="grid gap-2">转换策略<SelectAdapter aria-label="迁移转换策略" value={policy} disabled={busy} onChange={event => { const next = (Object.keys(policyLabels) as Preview['conversion_policy'][]).find(value => value === event.target.value); if (next) setPolicy(next); }}>{(Object.keys(policyLabels) as Preview['conversion_policy'][]).filter(value => capability.conversion_policies.includes(value)).map(value => <option key={value} value={value}>{policyLabels[value]}</option>)}</SelectAdapter></label>
        <Button className="min-h-11" disabled={busy || !source || !target} onClick={() => void submit('preview')}>生成迁移预览</Button>
      </>}
    </div>}
    {plan && <div className="space-y-3">
      <p role="status">{phaseLabels[plan.phase]}</p><p className="break-all text-xs">原计划 {plan.plan_id} · 修订 {plan.revision} · 影响摘要 {plan.impact_digest}</p>
      <p>{plan.request.source_key} → {plan.request.target_key} · {typeLabel[plan.request.target_type]} · {policyLabels[plan.request.conversion_policy]}</p>
      <p>{plan.items.length} 个固定版本 sidecar；不兼容项不会被静默转换。执行再次检查权限、字典与值修订，任一受阻不切换字典和值。</p>
      {!plan.items.length && <p>此计划没有 sidecar 值，但执行仍会迁移字典定义。</p>}
      <ul className="m-0 list-none space-y-3 p-0">{plan.items.slice(0, shown).map(row => <li key={row.file_id + ':' + row.version_id} className="space-y-2 rounded border p-3"><p className="break-all text-xs">文件 {row.file_id} · 版本 {row.version_id} · sidecar 修订 {row.sidecar_revision}</p><p>{statusLabels[row.status]}{row.error ? ' · ' + row.error : ''}</p><div className="grid gap-2 sm:grid-cols-2"><div><p>迁移前</p><pre className="whitespace-pre-wrap break-all text-xs">{row.before === '' ? '无源字段值或不可读取值' : row.before}</pre></div><div><p>预期迁移后</p><pre className="whitespace-pre-wrap break-all text-xs">{row.after === '' ? '无转换结果' : row.after}</pre></div></div></li>)}</ul>
      {shown < plan.items.length && <Button variant="outline" className="min-h-11" onClick={() => setShown(value => value + 25)}>展开更多迁移项</Button>}
      {!record?.pending && state === 'ready' && capability?.editing_enabled && plan.editing_enabled && plan.phase === 'previewed' && <Button className="min-h-11" disabled={busy} onClick={() => void submit('confirm')}>确认此迁移计划</Button>}
      {!record?.pending && state === 'ready' && capability?.editing_enabled && plan.editing_enabled && plan.phase === 'confirmed' && <Button variant="destructive" className="min-h-11" disabled={busy} onClick={() => setRisk(true)}>执行已确认迁移</Button>}
      {risk && <div role="alert" className="space-y-2"><p>执行将按以上原计划更改字段 key／类型及空间内 sidecar 值。不会自动回滚已完成迁移，不修改文件正文。确认执行？</p><Button variant="destructive" className="min-h-11" disabled={busy} onClick={() => void submit('execute')}>确认执行字段与属性迁移</Button><Button variant="outline" className="min-h-11" disabled={busy} onClick={() => setRisk(false)}>暂不执行</Button></div>}
      {!record?.pending && ['completed', 'partial'].includes(plan.phase) && <Button className="min-h-11" disabled={busy} onClick={() => void finish()}>确认结果并结束本计划</Button>}
    </div>}
    {state !== 'denied' && <div className="flex flex-wrap gap-2">{record?.pending && <Button className="min-h-11" disabled={busy || state === 'loading'} onClick={() => void submit('lookup')}>查询原迁移回执</Button>}<Button variant="outline" className="min-h-11" disabled={busy || state === 'loading'} onClick={() => setAttempt(value => value + 1)}>重读迁移状态</Button></div>}
    {state !== 'denied' && record?.plan && !record.pending && <Button variant="ghost" className="min-h-11" disabled={busy || state === 'loading'} onClick={() => void leavePlan()}>移除本地计划入口（不取消计划）</Button>}
    {notice && <p role="status">{notice}</p>}
  </AppDialog>;
}
