import { useEffect, useRef, useState } from 'react';
import { getDataContext, pinnedFile, type DriveFile } from '../api';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Table } from '../components/ui/table';
import { queryFiles } from '../info-workspace/client';
import { CapabilityGate, flowErrorText } from './ConsumerFlows';
import { confirmOrganizationPlan, createCompensationPlan, createOrganizationPlan, executeOrganizationPlan, getOrganizationPlan, getOrganizationPolicy, lookupOperation, type FlowCapability, type OrganizationPolicyView } from './client';
import { partialOutcome, type OrganizationItem, type OrganizationPlanView } from './model';
import { claimOrganizationIntent, clearOrganizationIntent, readOrganizationIntent, type OrganizationIntent } from './organization-recovery';

type Stage = { kind: 'source' } | { kind: 'plan' | 'executed'; plan: OrganizationPlanView; items: OrganizationItem[] };
export function OrganizeCenter({ space, capability, actor = '' }: { space: string; capability: FlowCapability; actor?: string }) {
  const context = getDataContext();
  const slot = 'drivebridge:organize:' + JSON.stringify([location.origin, actor, context.tenant, context.space, space]);
  const [stage, setStage] = useState<Stage>({ kind: 'source' });
  const [prefix, setPrefix] = useState('');
  const [policy, setPolicy] = useState<OrganizationPolicyView | null>(null);
  const [refs, setRefs] = useState<string[]>([]);
  const [partial, setPartial] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [stale, setStale] = useState(false);
  const [confirmDigest, setConfirmDigest] = useState('');
  const [replay, setReplay] = useState<OrganizationIntent | null>(null);
  const [recovery, setRecovery] = useState<OrganizationIntent | null>(null);
  const [rejectedKey, setRejectedKey] = useState('');
  const [availability, setAvailability] = useState<'ready' | 'offline' | 'denied' | 'blocked'>('ready');
  const controller = useRef<AbortController | null>(null), active = useRef(true), executing = useRef(false);
  useEffect(() => {
    active.current = true;
    const invalidate = () => { active.current = false; controller.current?.abort(); setRefs([]); setPolicy(null); setReplay(null); setStage({ kind: 'source' }); setRecovery(null); setRejectedKey(''); setAvailability('denied'); };
    const offline = () => { controller.current?.abort(); executing.current = false; setRefs([]); setPolicy(null); setReplay(null); setStage({ kind: 'source' }); setAvailability('offline'); setBusy(false); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('offline', offline);
    try { const saved = readOrganizationIntent(slot); setRecovery(saved); const rejected = localStorage.getItem(slot + ':rejected'); if (saved && rejected === saved.key) { setRejectedKey(rejected); setStale(true); } if (!navigator.onLine) offline(); } catch { setAvailability('blocked'); }
    return () => { active.current = false; controller.current?.abort(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', offline); };
  }, [slot]);
  const request = () => { if (!active.current || !navigator.onLine || executing.current || availability === 'denied' || availability === 'blocked') return null; executing.current = true; controller.current?.abort(); const value = new AbortController(); controller.current = value; setBusy(true); setError(''); return value; };
  const finish = () => { executing.current = false; if (active.current) setBusy(false); };
  const show = (plan: OrganizationPlanView, items: OrganizationItem[]) => { setConfirmDigest(plan.impact_digest); setStage({ kind: ['completed', 'partial'].includes(plan.phase) ? 'executed' : 'plan', plan, items }); };
  const failure = (err: unknown) => {
    if (err && typeof err === 'object' && 'status' in err && (err.status === 401 || err.status === 403)) {
      controller.current?.abort(); active.current = false; setRefs([]); setPolicy(null); setReplay(null); setStage({ kind: 'source' }); setRecovery(null); setRejectedKey(''); setAvailability('denied');
    } else if (err instanceof Error && err.message === 'plan_stale') {
      setStale(true);
      try { const original = readOrganizationIntent(slot); if (original?.action === 'organize_apply' || original?.action === 'organize_confirm') { localStorage.setItem(slot + ':rejected', original.key); setRejectedKey(original.key); } } catch { setAvailability('blocked'); }
    } else setError(flowErrorText(err));
  };
  const releaseRejected = async () => {
    if (!rejectedKey || !recovery || recovery.key !== rejectedKey || !navigator.onLine) return;
    const current = request(); if (!current) return;
    try { await clearOrganizationIntent(slot, rejectedKey); if (localStorage.getItem(slot + ':rejected') === rejectedKey) localStorage.removeItem(slot + ':rejected'); if (!current.signal.aborted) { setRecovery(null); setRejectedKey(''); setStage({ kind: 'source' }); setRefs([]); setPolicy(null); setStale(false); setError('原请求已明确拒绝，已释放恢复身份；请重新固定文件和预览，不会自动执行。'); } } catch (err) { if (!current.signal.aborted) failure(err); } finally { finish(); }
  };
  const materialize = async () => {
    const current = request(); if (!current) return;
    try {
      const exact = await queryFiles({ mode: 'exact', space_ids: [space], filters: prefix ? [{ field: 'path_prefix' as const, op: 'prefix' as const, value: prefix }] : [], limit: 200 }, current.signal);
      if (!exact.items.length) throw new Error('当前范围没有可整理的固定版本。');
      const fixed: string[] = [];
      for (const item of exact.items) {
        if (!item.file_id || !item.version_id) throw new Error('服务未提供固定版本，不能将当前文件替代为整理依据。');
        const pin = await pinnedFile({ id: item.file_id, space_id: space, version_id: item.version_id } as DriveFile, item.version_id, {}, current.signal);
        if (pin.file.id !== item.file_id || pin.file.space_id !== space || pin.file.version_id !== item.version_id) throw new Error('固定版本身份不匹配。');
        fixed.push(pin.ref);
      }
      const observedPolicy = await getOrganizationPolicy(space, current.signal);
      if (!current.signal.aborted) { setRefs(fixed); setPartial(!!exact.next_cursor || exact.coverage !== 'complete'); setPolicy(observedPolicy); setAvailability('ready'); }
    } catch (err) { if (!current.signal.aborted) { setRefs([]); setPolicy(null); failure(err); } } finally { finish(); }
  };
  const recover = async () => {
    if (!recovery) return;
    const current = request(); if (!current) return;
    try {
      if (recovery.action === 'organize_plan') {
        const found = await lookupOperation('organize_plan', recovery.key, { ...recovery.input }, current.signal);
        if (!found.found || found.resource_type !== 'organization_plan' || !found.resource_id) { if (found.recovery_action === 'replay_same_key_and_request') setReplay(recovery); else setError('尚未找到原计划，不代表未创建；保留原键，不创建替代计划。'); return; }
        const page = await getOrganizationPlan(found.resource_id, { limit: 200 }, current.signal);
        if (!current.signal.aborted) { await clearOrganizationIntent(slot, recovery.key); setRecovery(null); show(page.plan, page.items); }
      } else if (recovery.action === 'organize_apply' || recovery.action === 'organize_confirm') {
        const page = await getOrganizationPlan(recovery.plan, { limit: 200 }, current.signal);
        if (!current.signal.aborted) { setAvailability('ready'); show(page.plan, page.items); if (['completed', 'partial', 'cancelled'].includes(page.plan.phase)) { await clearOrganizationIntent(slot, recovery.key); setRecovery(null); } else if (recovery.action === 'organize_confirm' && page.plan.phase === 'confirmed') { await clearOrganizationIntent(slot, recovery.key); const claimed = await claimOrganizationIntent(slot, { action: 'organize_apply', plan: page.plan.plan_id, revision: page.plan.revision }, current.signal); setRecovery(claimed.intent); } else setError('原计划已回读，请显式恢复原确认或执行；不会自动写入。'); }
      } else {
        const plan = await createCompensationPlan(recovery.plan, recovery.key, current.signal); const page = await getOrganizationPlan(plan.plan_id, { limit: 200 }, current.signal);
        if (!current.signal.aborted) { await clearOrganizationIntent(slot, recovery.key); setRecovery(null); show(page.plan, page.items); }
      }
    } catch (err) { if (!current.signal.aborted) failure(err); } finally { finish(); }
  };
  const createPlanFromRecovery = async (intent: OrganizationIntent) => {
    if (intent.action !== 'organize_plan') return;
    const current = request(); if (!current) return;
    try { const plan = await createOrganizationPlan(intent.input, intent.key, current.signal); const page = await getOrganizationPlan(plan.plan_id, { limit: 200 }, current.signal); if (!current.signal.aborted) { await clearOrganizationIntent(slot, intent.key); setRecovery(null); show(page.plan, page.items); } } catch (err) { if (!current.signal.aborted) failure(err); } finally { finish(); }
  };
  const createPlan = async (kind: 'move' | 'add_label') => {
    if (!refs.length || recovery) return;
    const current = request(); if (!current) return;
    try {
      const input = { refs, actions: [kind === 'move' ? { kind, target_prefix: '/整理后/' } : { kind, namespace: 'rule', value: 'reference' }], policy_id: policy?.policy_id || '' };
      const claimed = await claimOrganizationIntent(slot, { action: 'organize_plan', input }, current.signal); setRecovery(claimed.intent);
      if (!claimed.claimed) return;
      const plan = await createOrganizationPlan(input, claimed.intent.key, current.signal);
      const page = await getOrganizationPlan(plan.plan_id, { limit: 200 }, current.signal);
      if (!current.signal.aborted) { await clearOrganizationIntent(slot, claimed.intent.key); setRecovery(null); show(page.plan, page.items); }
    } catch (err) { if (!current.signal.aborted) failure(err); } finally { finish(); }
  };
  const confirm = async () => {
    if (stage.kind !== 'plan' || recovery) return;
    const current = request(); if (!current) return;
    try {
      if (stage.plan.phase === 'stale') { setStale(true); return; }
      const confirmation = await claimOrganizationIntent(slot, { action: 'organize_confirm', plan: stage.plan.plan_id, revision: stage.plan.revision, digest: confirmDigest }, current.signal); setRecovery(confirmation.intent); if (!confirmation.claimed) return;
      const confirmed = await confirmOrganizationPlan(stage.plan.plan_id, { expected_revision: stage.plan.revision, impact_digest: confirmDigest }, current.signal);
      await clearOrganizationIntent(slot, confirmation.intent.key);
      const claimed = await claimOrganizationIntent(slot, { action: 'organize_apply', plan: confirmed.plan_id, revision: confirmed.revision }, current.signal); setRecovery(claimed.intent);
      if (!claimed.claimed) return;
      const executed = await executeOrganizationPlan(confirmed.plan_id, { expected_revision: confirmed.revision }, claimed.intent.key, current.signal);
      const page = await getOrganizationPlan(executed.plan_id, { limit: 200 }, current.signal);
      if (!current.signal.aborted) { await clearOrganizationIntent(slot, claimed.intent.key); setRecovery(null); show(page.plan, page.items); }
    } catch (err) { if (!current.signal.aborted) failure(err); } finally { finish(); }
  };
  const compensate = async () => {
    if (stage.kind !== 'executed' || recovery) return;
    const current = request(); if (!current) return;
    try {
      const claimed = await claimOrganizationIntent(slot, { action: 'organize_compensation_plan', plan: stage.plan.plan_id }, current.signal); setRecovery(claimed.intent);
      if (!claimed.claimed) return;
      const plan = await createCompensationPlan(stage.plan.plan_id, claimed.intent.key, current.signal);
      const page = await getOrganizationPlan(plan.plan_id, { limit: 200 }, current.signal);
      if (!current.signal.aborted) { await clearOrganizationIntent(slot, claimed.intent.key); setRecovery(null); show(page.plan, page.items); }
    } catch (err) { if (!current.signal.aborted) failure(err); } finally { finish(); }
  };
  const resumeExecution = async () => {
    if (!recovery || (recovery.action !== 'organize_apply' && recovery.action !== 'organize_confirm')) return;
    const current = request(); if (!current) return;
    try {
      if (recovery.action === 'organize_confirm') { const value = await confirmOrganizationPlan(recovery.plan, { expected_revision: recovery.revision, impact_digest: recovery.digest }, current.signal); await clearOrganizationIntent(slot, recovery.key); const claimed = await claimOrganizationIntent(slot, { action: 'organize_apply', plan: value.plan_id, revision: value.revision }, current.signal); setRecovery(claimed.intent); return; }
      const observed = await getOrganizationPlan(recovery.plan, { limit: 200 }, current.signal);
      if (['completed', 'partial'].includes(observed.plan.phase)) { await clearOrganizationIntent(slot, recovery.key); setRecovery(null); show(observed.plan, observed.items); return; }
      if (observed.plan.phase !== 'confirmed' || observed.plan.revision !== recovery.revision) { show(observed.plan, observed.items); setError('原计划已变化，尚不能判定原请求结果；保留恢复身份，不自动执行或释放。'); return; }
      const executed = await executeOrganizationPlan(recovery.plan, { expected_revision: recovery.revision }, recovery.key, current.signal); const page = await getOrganizationPlan(executed.plan_id, { limit: 200 }, current.signal); if (!current.signal.aborted) { await clearOrganizationIntent(slot, recovery.key); setRecovery(null); show(page.plan, page.items); }
    } catch (err) { if (!current.signal.aborted) failure(err); } finally { finish(); }
  };
  const disabled = busy || !!recovery || availability !== 'ready';
  return <CapabilityGate capability={capability} action="organize_plan"><section className="cf-center" aria-label="整理中心">
    {error && <div className="banner error" role="alert">{error}</div>}
    {availability === 'denied' && <p role="alert">权限或范围已变化，整理信息已清除。</p>}
    {availability === 'offline' && <p role="alert">当前离线，联网后请重新选择固定文件或查询原计划，不会自动执行。</p>}
    {availability === 'offline' && !recovery && <Button className="min-h-11" disabled={busy} onClick={() => void materialize()}>联网后重新读取固定文件</Button>}
    {rejectedKey && recovery?.key === rejectedKey && <Button className="min-h-11" disabled={busy || availability !== 'ready'} onClick={() => void releaseRejected()}>释放已拒绝请求并重新预览</Button>}
    {recovery && (recovery.action === 'organize_apply' || recovery.action === 'organize_confirm') && <Button className="min-h-11" disabled={busy} onClick={() => void resumeExecution()}>恢复原确认或执行</Button>}
    {availability === 'blocked' && <p role="alert">整理恢复身份无效，已阻止新计划；请检查浏览器恢复存储。</p>}
    {replay && <div role="status"><p>尚未找到原计划；可使用原键重放相同固定范围与动作。</p><Button className="min-h-11" disabled={busy} onClick={() => { setRecovery(replay); setReplay(null); void createPlanFromRecovery(replay); }}>重放原创建</Button></div>}
    {recovery && <div role="status"><p>有原整理请求待回查，不创建新的请求。</p><Button className="min-h-11" disabled={busy} onClick={() => void recover()}>查询原整理请求</Button></div>}
    {stale && <div className="banner warn" role="alert" data-stale="plan">计划在预览后已发生变化（plan_stale）。请重新选择固定版本后预览，批准不会自动重放。</div>}
    {stage.kind === 'source' && <div className="cf-card" data-region="source"><h3>固定整理范围</h3><p className="cf-hint">只固定当前查询页最多200项的受权文件版本，不创建临时集合或快照，不把当前页当作全集。</p><div className="cf-row"><Input value={prefix} onChange={event => setPrefix(event.target.value)} aria-label="目录前缀" placeholder="目录前缀，例如 docs/"/><Button disabled={disabled} onClick={() => void materialize()}>固定所选文件</Button></div>{!!refs.length && <><p role="status">已固定 {refs.length} 个文件版本。{partial && '查询仅为部分范围；未加载项不在计划内。'}</p><Button disabled={disabled} onClick={() => void createPlan('add_label')}>生成标签整理计划</Button><Button disabled={disabled} onClick={() => void createPlan('move')}>生成移动计划</Button></>}</div>}
    {stage.kind !== 'source' && <div className="cf-card" data-region="plan"><h3>计划 {stage.plan.plan_id}</h3><dl className="cf-facts"><div><dt>阶段</dt><dd>{stage.plan.phase}</dd></div><div><dt>风险</dt><dd>{stage.plan.risk}</dd></div><div><dt>规则授权</dt><dd>{stage.plan.approval_required ? '需要人工确认' : '规则范围内预授权'}</dd></div><div><dt>影响摘要</dt><dd>{stage.plan.impact_digest || '未取得'}</dd></div><div><dt>修订</dt><dd>{stage.plan.revision}</dd></div></dl><Table className="cf-table" aria-label="计划项"><thead><tr><th>文件</th><th>动作</th><th>before → after</th><th>状态</th></tr></thead><tbody>{stage.items.map(item => <tr key={item.item_key}><td>{item.file_id}</td><td>{item.action}</td><td>{item.before} → {item.after}</td><td>{item.state}{item.reason && `（${item.reason}）`}</td></tr>)}</tbody></Table>{stage.kind === 'executed' ? <><p role="status">执行结果：成功 {partialOutcome(stage.items).succeeded} · 失败 {partialOutcome(stage.items).failed} · 待处理 {partialOutcome(stage.items).pending}。已成功项不会重做。</p><Button disabled={disabled} onClick={() => void compensate()}>生成补偿计划</Button></> : <><label>确认影响摘要<Input value={confirmDigest} aria-label="确认影响摘要" onChange={event => setConfirmDigest(event.target.value)}/></label><Button disabled={disabled} onClick={() => void confirm()}>确认并执行</Button></>}<Button disabled={busy || !!recovery} onClick={() => { setStage({ kind: 'source' }); setStale(false); setRefs([]); }}>重新预览</Button><p className="cf-hint">当前服务没有取消计划入口；离开此预览不代表撤销已执行修改。补偿是新计划。</p></div>}
  </section></CapabilityGate>;
}
