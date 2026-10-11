import { useEffect, useRef, useState } from 'react';
import { api } from '../../api';
import { useRouteLeaveGuard } from '../../app/route-leave';
import { AppDialog } from '../../components/AppDialog';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { claimCreation, clearCreation, collectionSlot, creationDigest, matchesCreation, readCreation, type CollectionReceipt, type Creation, type Recovery } from './collection-recovery';

import { compileRule, RuleFields, type RuleDraft, type Sort } from './collection-rule';

export function CreateCollectionDialog({ scope, space, close, denied, kind = 'manual' }: { scope: string; space: { id: string; name: string }; close: () => void; denied: () => void; kind?: 'manual' | 'rule' }) {
  const [phase, setPhase] = useState('initializing'), [name, setName] = useState(''), [message, setMessage] = useState('');
  const [record, setRecord] = useState<Recovery | null>(null), [receipt, setReceipt] = useState<CollectionReceipt | null>(null), [retry, setRetry] = useState(false);
  const active = useRef(true), running = useRef(false), original = useRef<Creation | null>(null), controller = useRef<AbortController | null>(null);
  const slot = collectionSlot(scope);
  const [rows, setRows] = useState<RuleDraft[]>([]), [sort, setSort] = useState<Sort>({ field: 'updated_at', direction: 'desc' }), [discard, setDiscard] = useState(false);
  const clearDraft = () => { setName(''); setRows([]); setSort({ field: 'updated_at', direction: 'desc' }); setDiscard(false); };
  const dirty = !!name || rows.length > 0 || sort.field !== 'updated_at' || sort.direction !== 'desc';
  useRouteLeaveGuard(dirty && !record, '放弃未提交的集合创建草稿并离开？');
  useEffect(() => { const leave = (event: BeforeUnloadEvent) => { if (dirty && !record) { event.preventDefault(); event.returnValue = ''; } }; window.addEventListener('beforeunload', leave); return () => window.removeEventListener('beforeunload', leave); }, [dirty, record]);
  useEffect(() => {
    active.current = true;
    try { const stored = readCreation(slot); setRecord(stored); setPhase(stored ? 'unknown' : 'edit'); } catch { setPhase('unavailable'); }
    const invalidate = () => { active.current = false; controller.current?.abort(); original.current = null; clearDraft(); setReceipt(null); setMessage(''); setPhase('denied'); };
    const storage = (event: StorageEvent) => { if (event.key === null || event.key === slot) invalidate(); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('offline', invalidate); window.addEventListener('storage', storage);
    if (!navigator.onLine) invalidate();
    return () => { active.current = false; controller.current?.abort(); original.current = null; window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', invalidate); window.removeEventListener('storage', storage); };
  }, [slot]);
  function prepare() {
    if (!active.current || running.current) return;
    // The owner bounds UTF-8 bytes, not JavaScript UTF-16 units.
    if (!name.trim() || new TextDecoder().decode(new TextEncoder().encode(name)) !== name || new TextEncoder().encode(name).length > 200 || /[\x00-\x1f\x7f]/.test(name)) { setMessage('请输入有效名称，UTF-8 编码不超过 200 字节。'); return; }
    try { original.current = kind === 'rule' ? { space_id: space.id, name, kind, scope: 'private', rule: compileRule(space.id, rows, sort) } : { space_id: space.id, name, kind, scope: 'private' }; }
    catch { setMessage('请检查规则条件：字段与操作匹配，值在类型与字节预算内；大小为非负安全整数，日期含明确时区。'); return; }
    setMessage(''); setPhase('confirm');
  }
  async function perform(mode: 'create' | 'lookup' | 'retry') {
    if (!active.current || running.current || !navigator.onLine) return;
    running.current = true; setPhase('busy'); setMessage(''); setRetry(false);
    const abort = new AbortController(); controller.current = abort; let recovery = record;
    try {
      if (mode === 'create') {
        if (!original.current) throw new Error('missing_original');
        const claim = await claimCreation(slot, await creationDigest(original.current), abort.signal);
        if (!active.current || abort.signal.aborted) return;
        recovery = claim.record; setRecord(recovery);
        if (!claim.claimed) { original.current = null; clearDraft(); setPhase('unknown'); return; }
      }
      if (!recovery || JSON.stringify(readCreation(slot)) !== JSON.stringify(recovery)) throw new Error('recovery_changed');
      if (mode !== 'lookup') {
        if (!original.current || await creationDigest(original.current) !== recovery.digest) throw new Error('missing_original');
        if (!active.current || abort.signal.aborted || !navigator.onLine) return;
        await api('/file-collections', 'POST', original.current, { 'Idempotency-Key': recovery.key }, abort.signal);
      }
      if (!active.current || abort.signal.aborted) return;
      const result = await api<CollectionReceipt>('/file-collections/lookup', 'POST', { idempotency_key: recovery.key }, {}, abort.signal);
      if (!matchesCreation(result, recovery, space.id) || JSON.stringify(readCreation(slot)) !== JSON.stringify(recovery)) throw new Error('receipt_mismatch');
      if (!active.current || abort.signal.aborted) return;
      setReceipt(result); setPhase('done'); original.current = null; clearDraft();
    } catch (error) {
      if (!active.current || abort.signal.aborted) return;
      const status = (error as { status?: number }).status;
      if (status === 401 || status === 403) { original.current = null; clearDraft(); setReceipt(null); setPhase('denied'); denied(); }
      else if (!recovery) { setPhase('unavailable'); }
      else { if ([400,409,422].includes(status || 0)) { original.current = null; clearDraft(); } setPhase('unknown'); setRetry(mode === 'lookup' && status === 404 && !!original.current); setMessage('未取得匹配的原创建回执。未查到不证明从未提交；不能换键重建。'); }
    } finally { running.current = false; }
  }
  async function acknowledge() {
    if (!active.current || running.current || !record || !receipt) return;
    running.current = true;
    try { await clearCreation(slot, record); if (active.current) close(); }
    catch { if (active.current) setMessage('恢复身份已变化，不能清除其他页面的请求。'); }
    finally { running.current = false; }
  }
  return <AppDialog title={kind === 'rule' ? '新建私有规则集合' : '新建私有手动集合'} close={() => { if (dirty && !record) setDiscard(true); else close(); }}>
    <p className="text-sm text-muted-foreground">空间：{space.name}。创建{kind === 'rule' ? '规则' : '空'}集合不会复制、移动或读取文件。名称与规则只留内存，浏览器仅保存原请求键与摘要；刷新后只能查询原回执。</p>
    {discard && <div role="alert"><p>关闭将丢弃未提交的集合草稿。</p><Button className="min-h-11" onClick={close}>丢弃集合草稿并关闭</Button><Button variant="outline" className="min-h-11" onClick={() => setDiscard(false)}>继续编辑集合</Button></div>}
    {message && <p role="alert">{message}</p>}
    {phase === 'edit' && <form className="space-y-4" onSubmit={event => { event.preventDefault(); prepare(); }}><label className="grid gap-2">集合名称<Input aria-label="集合名称" className="min-h-11" value={name} maxLength={200} required onChange={event => setName(event.target.value)}/></label>{kind === 'rule' && <RuleFields rows={rows} sort={sort} change={setRows} changeSort={setSort}/>}<Button type="submit" className="min-h-11">准备创建集合</Button></form>}
    {phase === 'confirm' && <div className="space-y-3"><p className="break-all">{name} · 私有 · {kind === 'rule' ? '规则' : '手动'}</p><p>提交时由服务端重新核对当前空间与创建权限。不创建快照、不添加显式成员。</p>{original.current?.kind === 'rule' && <><p>{rows.length ? `${rows.length} 条条件同时匹配，读取时动态计算。` : '无过滤条件：读取时匹配此空间全部可见未回收文件。'} 不是固定快照。</p><pre aria-label="待确认集合规则" className="max-h-72 overflow-auto whitespace-pre-wrap break-all rounded border border-border p-3 text-xs">{JSON.stringify(original.current.rule, null, 2)}</pre></>}<Button className="min-h-11" onClick={() => void perform('create')}>确认创建集合</Button><Button variant="outline" className="min-h-11" onClick={() => { original.current = null; setPhase('edit'); }}>返回修改</Button></div>}
    {phase === 'busy' && <p role="status">处理原集合请求…</p>}
    {phase === 'unknown' && <div className="space-y-3"><p>有尚未确认的原创建请求；关闭或刷新不会更换原键。</p><Button className="min-h-11" onClick={() => void perform('lookup')}>查询原集合回执</Button>{retry && <><p>原键重试可能首次创建，仅在仍要提交原名称与空间时确认。</p><Button variant="outline" className="min-h-11" onClick={() => void perform('retry')}>确认按原键重试创建</Button></>}</div>}
    {phase === 'done' && receipt && <div className="space-y-3"><p role="status">已确认原集合创建</p><p className="break-all">集合标识：{receipt.collection_id}</p><p>原提交修订：1 · {receipt.committed_at}</p><p>这不是当前成员或最新修订。关闭后请手动刷新集合列表。</p><Button className="min-h-11" onClick={() => void acknowledge()}>确认回执并关闭</Button></div>}
    {phase === 'denied' && <p role="alert">权限、范围、网络或其他页面的恢复身份已变化，表单已清除。请在正确范围重新打开。</p>}
    {phase === 'unavailable' && <p role="alert">恢复存储或跨标签锁不可用，不能安全创建集合。</p>}
  </AppDialog>;
}
