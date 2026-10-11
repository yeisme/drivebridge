import { useEffect, useRef, useState } from 'react';
import { api, type Space } from '../../api';
import { useRouteLeaveGuard } from '../../app/route-leave';
import { AppDialog } from '../../components/AppDialog';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { SelectAdapter } from '../../components/SelectAdapter';
import { claimResource, clearResource, matchesResourceReceipt, readResourceRecovery, resourceDigest, resourceSlot, type Recovery, type RegisterResource, type ResourceReceipt } from './resource-recovery';

export function RegisterResourceDialog({ scope, spaces, close }: { scope: string; spaces: Space[]; close: () => void }) {
  const [phase, setPhase] = useState('initializing'), [message, setMessage] = useState('');
  const [name, setName] = useState(''), [space, setSpace] = useState(spaces.length === 1 ? spaces[0].id : ''), [kind, setKind] = useState<RegisterResource['kind']>('skill');
  const [targetKind, setTargetKind] = useState('file'), [target, setTarget] = useState('');
  const [record, setRecord] = useState<Recovery | null>(null), [receipt, setReceipt] = useState<ResourceReceipt | null>(null);
  const [retry, setRetry] = useState(false), [discard, setDiscard] = useState(false);
  const initialSpace = useRef(spaces.length === 1 ? spaces[0].id : '');
  const dirty = !!name || !!target || space !== initialSpace.current || kind !== 'skill' || targetKind !== 'file';
  const unsent = dirty && !record && phase !== 'denied';
  useRouteLeaveGuard(unsent, '放弃未提交的资源登记草稿并离开？');
  const clearDraft = () => { setName(''); setTarget(''); setSpace(initialSpace.current); setKind('skill'); setTargetKind('file'); setDiscard(false); };
  useEffect(() => {
    if (!unsent) return;
    const leave = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ''; };
    window.addEventListener('beforeunload', leave);
    return () => window.removeEventListener('beforeunload', leave);
  }, [unsent]);
  const original = useRef<RegisterResource | null>(null), running = useRef(false), active = useRef(true), controller = useRef<AbortController | null>(null);
  const slot = resourceSlot(scope);
  useEffect(() => {
    active.current = true;
    try { const stored = readResourceRecovery(slot); setRecord(stored); setPhase(stored ? 'unknown' : 'edit'); } catch { setPhase('unavailable'); }
    const invalidate = () => { active.current = false; controller.current?.abort(); original.current = null; setReceipt(null); clearDraft(); setPhase('denied'); };
    const storage = (event: StorageEvent) => { if (event.key === null || event.key === slot) invalidate(); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('offline', invalidate); window.addEventListener('storage', storage);
    if (!navigator.onLine) invalidate();
    return () => { invalidate(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', invalidate); window.removeEventListener('storage', storage); };
  }, [slot]);
  async function perform(mode: 'preview' | 'create' | 'lookup' | 'retry') {
    if (running.current || !active.current) return;
    if (!navigator.onLine) { setMessage('当前离线，请联网后重新打开；未发送登记。'); return; }
    running.current = true; setMessage(''); setRetry(false); setPhase('busy');
    const abort = new AbortController(); controller.current = abort;
    let recovery = record;
    try {
      if (mode === 'preview') {
        if (!spaces.some(item => item.id === space) || !name.trim() || [...name].length > 120 || /[\x00-\x1f\x7f]/.test(name) || !target || (kind === 'reference_pack' && targetKind !== 'snapshot')) throw new Error('invalid_input');
        const input: RegisterResource = { space_id: space, kind, display_name: name, ...(targetKind === 'file' ? { file_ref: target } : { collection_snapshot_id: target }) };
        if (targetKind === 'file') {
          const url = new URL(target);
          if (target.length > 2048 || url.protocol !== 'drivebridge:' || url.search || url.hash || url.username || url.password || !/^\/file\/[a-f0-9]{48}\/version\/[a-f0-9]{48}$/.test(url.pathname)) throw new Error('invalid_ref');
          const value = await api<{ ref: string; file: { id: string; version_id: string; space_id: string; trashed?: boolean } }>('/references?' + new URLSearchParams({ ref: target }), 'GET', undefined, {}, abort.signal);
          const parts = url.pathname.split('/');
          if (value.ref !== target || value.file?.id !== parts[2] || value.file.version_id !== parts[4] || value.file.space_id !== space || value.file.trashed) throw new Error('target_mismatch');
        } else {
          if (!/^[a-f0-9]{48}$/.test(target)) throw new Error('invalid_snapshot');
          const value = await api<{ snapshot_id: string; collection_id: string; entries_total: number; entries_visible: number; items: unknown[] }>('/collection-snapshots/' + target, 'GET', undefined, {}, abort.signal);
          if (value.snapshot_id !== target || !/^[a-f0-9]{48}$/.test(value.collection_id) || !Number.isSafeInteger(value.entries_total) || value.entries_total < 0 || value.entries_total > 1000 || value.entries_visible !== value.entries_total || !Array.isArray(value.items) || value.items.length !== value.entries_total) throw new Error('incomplete_snapshot');
          // Snapshot reads do not expose the collection space. Do not infer
          // it from members; the register service validates it atomically.
        }
        if (!active.current || abort.signal.aborted) return;
        original.current = input; setPhase('confirm'); return;
      }
      if (mode === 'create') {
        if (!original.current) throw new Error('missing_original');
        const digest = await resourceDigest(original.current);
        const claim = await claimResource(slot, digest, abort.signal);
        if (!active.current || abort.signal.aborted) return;
        recovery = claim.record; setRecord(recovery);
        if (!claim.claimed) { original.current = null; setPhase('unknown'); return; }
      }
      if (!recovery || JSON.stringify(readResourceRecovery(slot)) !== JSON.stringify(recovery)) throw new Error('recovery_changed');
      if (mode !== 'lookup') {
        if (!original.current || await resourceDigest(original.current) !== recovery.digest) throw new Error('missing_original');
        if (!active.current || abort.signal.aborted || !navigator.onLine) return;
        await api('/resource-descriptors', 'POST', original.current, { 'Idempotency-Key': recovery.key }, abort.signal);
      }
      if (!active.current || abort.signal.aborted) return;
      const result = await api<ResourceReceipt>('/resource-descriptors/lookup', 'POST', { idempotency_key: recovery.key }, {}, abort.signal);
      if (!await matchesResourceReceipt(result, recovery)) throw new Error('receipt_mismatch');
      if (!active.current || abort.signal.aborted) return;
      setReceipt(result); setPhase('done'); original.current = null;
    } catch (error) {
      if (!active.current || abort.signal.aborted) return;
      const status = (error as { status?: number }).status;
      if (status === 401 || status === 403) { original.current = null; setReceipt(null); clearDraft(); setPhase('denied'); }
      else if (mode === 'preview') { original.current = null; setPhase('edit'); setMessage('无法核对固定目标，请检查空间、原版本引用或完整集合快照；没有登记资源。'); }
      else if (!recovery) { setPhase('unavailable'); setMessage('无法登记恢复身份，未发送创建请求。'); }
      else { setPhase('unknown'); setRetry(mode === 'lookup' && status === 404 && !!original.current); setMessage('未取得匹配的原提交回执。未查到不证明请求从未执行；不要换键重建。'); }
    } finally { running.current = false; }
  }
  async function acknowledge() {
    if (!active.current || running.current || !record) return;
    running.current = true;
    try { await clearResource(slot, record); if (active.current) close(); } catch { if (active.current) setMessage('恢复槽已变化或不可清除，请保留原回执，不要清除其他页面请求。'); }
    finally { running.current = false; }
  }
  return <AppDialog title="登记固定资源" close={() => { if (unsent) setDiscard(true); else close(); }}>
    {discard && <div role="alert"><p>关闭将丢弃未提交的资源登记草稿。</p><Button className="min-h-11" onClick={close}>丢弃登记草稿并关闭</Button><Button variant="outline" className="min-h-11" onClick={() => setDiscard(false)}>继续编辑登记</Button></div>}
    <p className="text-sm text-muted-foreground">登记不是安装或执行，也不是内容安全检查。名称、引用和表单仅留内存；浏览器只保存原请求键与摘要，刷新后只能查询原结果。</p>
    {message && <p role="alert">{message}</p>}
    {phase === 'edit' && <form onSubmit={event => { event.preventDefault(); void perform('preview'); }} className="space-y-4">
      <label className="grid gap-2">资源名称<Input aria-label="资源名称" className="min-h-11" value={name} maxLength={120} required onChange={event => setName(event.target.value)}/></label>
      <label className="grid gap-2">登记空间<SelectAdapter aria-label="登记空间" className="min-h-11" value={space} onChange={event => setSpace(event.target.value)}><option value="">选择空间</option>{spaces.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</SelectAdapter></label>
      <label className="grid gap-2">登记类型<SelectAdapter aria-label="登记类型" className="min-h-11" value={kind} onChange={event => { const value = event.target.value as RegisterResource['kind']; setKind(value); if (value === 'reference_pack') { setTargetKind('snapshot'); setTarget(''); } }}><option value="skill">Skill</option><option value="template">模板</option><option value="reference_pack">资料包</option></SelectAdapter></label>
      <label className="grid gap-2">固定目标类型<SelectAdapter aria-label="固定目标类型" className="min-h-11" value={targetKind} onChange={event => { setTargetKind(event.target.value); setTarget(''); }}><option value="file" disabled={kind === 'reference_pack'}>文件原版本引用</option><option value="snapshot">集合快照标识</option></SelectAdapter></label>
      <label className="grid gap-2">固定目标<Input aria-label="固定目标" className="min-h-11" value={target} required maxLength={2048} onChange={event => setTarget(event.target.value)}/></label>
      <Button type="submit" className="min-h-11">核对固定目标</Button>
    </form>}
    {phase === 'confirm' && <div className="space-y-3"><p className="break-all">{name} · {kind} · {target}</p><p>已读取目标元数据；快照预读不证明所属空间。提交时服务端仍会重新检查空间、全部权限与固定版本。未读取正文、未执行检查器。</p><Button className="min-h-11" onClick={() => void perform('create')}>确认登记资源</Button><Button variant="outline" className="min-h-11" onClick={() => { original.current = null; setPhase('edit'); }}>返回修改</Button></div>}
    {phase === 'busy' && <p role="status">处理原请求…</p>}
    {phase === 'unknown' && <div className="space-y-3"><p>此范围有尚未确认的原登记请求；关闭或刷新不会替换原键。</p><Button className="min-h-11" onClick={() => void perform('lookup')}>查询原登记回执</Button>{retry && <><p>原键重试可能首次执行，仅在确认仍要登记原目标时使用。</p><Button variant="outline" className="min-h-11" onClick={() => void perform('retry')}>确认按原键重试登记</Button></>}</div>}
    {phase === 'done' && receipt && <div className="space-y-3"><p role="status">已确认原登记：{receipt.resource.display_name}</p><p className="break-all">描述标识：{receipt.resource.descriptor_id}</p><p>原提交时间：{receipt.committed_at} · 未检查</p><Button className="min-h-11" onClick={() => void acknowledge()}>确认回执并关闭</Button></div>}
    {phase === 'denied' && <p role="alert">权限、范围、网络或其他页面的恢复身份已变化，内容已清除。请在正确范围重新打开。</p>}
    {phase === 'unavailable' && <p role="alert">恢复存储或跨标签锁不可用，不能安全登记资源。</p>}
  </AppDialog>;
}
