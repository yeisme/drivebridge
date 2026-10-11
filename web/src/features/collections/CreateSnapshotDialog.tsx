import { useEffect, useRef, useState } from 'react';
import { api } from '../../api';
import { AppDialog } from '../../components/AppDialog';
import { Button } from '../../components/ui/button';
import { ResourceSnapshot } from '../resources/ResourceSnapshot';
import { claimSnapshot, clearSnapshot, matchesSnapshot, readSnapshotRecovery, snapshotDigest, snapshotSlot, type SnapshotReceipt, type SnapshotRecovery } from './snapshot-recovery';

type Observation = { collection_id: string; revision: number; kind: string; coverage: string; visible_count: number; items: { file_id: string; version_id: string; path: string }[] };
export function CreateSnapshotDialog({ scope, id, close, denied }: { scope: string; id: string; close: () => void; denied: () => void }) {
  const [phase, setPhase] = useState('initializing'), [message, setMessage] = useState(''), [observation, setObservation] = useState<Observation | null>(null), [shown, setShown] = useState(50);
  const [record, setRecord] = useState<SnapshotRecovery | null>(null), [receipt, setReceipt] = useState<SnapshotReceipt | null>(null), [retry, setRetry] = useState(false), [reading, setReading] = useState(false);
  const active = useRef(true), running = useRef(false), original = useRef<number | null>(null), controller = useRef<AbortController | null>(null), readTrigger = useRef<HTMLButtonElement | null>(null);
  const slot = snapshotSlot(scope, id), path = '/file-collections/' + encodeURIComponent(id);
  const wipe = () => { original.current = null; setObservation(null); setReceipt(null); setReading(false); setMessage(''); };
  useEffect(() => {
    active.current = true;
    try { const stored = readSnapshotRecovery(slot); setRecord(stored); setPhase(stored ? 'unknown' : 'idle'); } catch { setPhase('unavailable'); }
    const invalidate = () => { active.current = false; controller.current?.abort(); wipe(); setPhase('denied'); };
    const storage = (event: StorageEvent) => { if (event.key === null || event.key === slot) invalidate(); };
    window.addEventListener('offline', invalidate); window.addEventListener('drivebridge-context', invalidate); window.addEventListener('storage', storage);
    if (!navigator.onLine) invalidate();
    return () => { active.current = false; controller.current?.abort(); original.current = null; window.removeEventListener('offline', invalidate); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('storage', storage); };
  }, [slot]);
  async function perform(mode: 'preview' | 'create' | 'lookup' | 'retry') {
    if (!active.current || running.current || !navigator.onLine) return;
    running.current = true; setPhase('busy'); setMessage(''); setRetry(false);
    const abort = new AbortController(); controller.current = abort; let recovery = record;
    try {
      if (mode === 'preview') {
        const value = await api<Observation>(path, 'GET', undefined, {}, abort.signal);
        if (!active.current || abort.signal.aborted) return;
        if (!value || value.collection_id !== id || !Number.isSafeInteger(value.revision) || value.revision < 1 || !['manual', 'rule'].includes(value.kind) || value.coverage !== 'complete' || !Array.isArray(value.items) || value.items.length > 30000 || value.visible_count !== value.items.length || !value.items.every(row => row && /^[a-f0-9]{48}$/.test(row.file_id) && /^[a-f0-9]{48}$/.test(row.version_id) && typeof row.path === 'string' && row.path.length <= 4096) || new Set(value.items.map(row => row.file_id)).size !== value.items.length) throw new Error('invalid_observation');
        original.current = value.revision; setObservation(value); setShown(50); setPhase('confirm'); return;
      }
      if (mode === 'create') {
        if (original.current === null) throw new Error('missing_original');
        const claim = await claimSnapshot(slot, await snapshotDigest(id, original.current), abort.signal);
        if (!active.current || abort.signal.aborted) return;
        recovery = claim.record; setRecord(recovery);
        if (!claim.claimed) { original.current = null; setObservation(null); setPhase('unknown'); return; }
      }
      if (!recovery || JSON.stringify(readSnapshotRecovery(slot)) !== JSON.stringify(recovery)) throw new Error('recovery_changed');
      if (mode !== 'lookup') {
        if (original.current === null || await snapshotDigest(id, original.current) !== recovery.digest) throw new Error('missing_original');
        if (!active.current || abort.signal.aborted || !navigator.onLine) return;
        await api(path + '/snapshots/creations', 'POST', { expected_revision: original.current }, { 'Idempotency-Key': recovery.key }, abort.signal);
      }
      if (!active.current || abort.signal.aborted) return;
      const result = await api<SnapshotReceipt>(path + '/snapshots/creations/lookup', 'POST', { idempotency_key: recovery.key }, {}, abort.signal);
      if (!await matchesSnapshot(result, recovery, id) || JSON.stringify(readSnapshotRecovery(slot)) !== JSON.stringify(recovery)) throw new Error('receipt_mismatch');
      if (!active.current || abort.signal.aborted) return;
      original.current = null; setObservation(null); setReceipt(result); setPhase('done');
    } catch (error) {
      if (!active.current || abort.signal.aborted) return;
      const status = (error as { status?: number }).status;
      if (status === 401 || status === 403) { wipe(); setPhase('denied'); denied(); }
      else if (mode === 'preview') { original.current = null; setObservation(null); setPhase('idle'); setMessage('无法取得完整当前观察；未创建快照。请核对权限、预算或服务状态。'); }
      else if (!recovery) setPhase('unavailable');
      else { if (status === 409) original.current = null; setObservation(null); setPhase('unknown'); setRetry(mode === 'lookup' && status === 404 && original.current !== null); setMessage('未取得原快照提交回执，可能存在冲突或未知结果；不自动换修订或换键重建。未查到不证明从未提交。'); }
    } finally { running.current = false; }
  }
  async function acknowledge() {
    if (!active.current || running.current || !record || !receipt) return;
    running.current = true;
    try { await clearSnapshot(slot, record); if (active.current) close(); } catch { if (active.current) setMessage('恢复身份已变化，不能清除其他页面请求。'); }
    finally { running.current = false; }
  }
  async function copyID() {
    if (!active.current || !receipt) return;
    try { await navigator.clipboard.writeText(receipt.snapshot_id); if (active.current) setMessage('已复制快照标识；可在资源库人工登记资料包。未自动登记或执行检查。'); }
    catch { if (active.current) setMessage('剪贴板不可用，请手动选择并复制下方快照标识。'); }
  }
  return <AppDialog title="创建集合快照" close={close}>
    <p className="text-sm text-muted-foreground">快照固定提交阶段捕获的成员版本，不复制正文、不改集合。预览只是当前观察：提交前文件或规则结果可能变化，最终不保证与预览逐字节相同，也不是全库原子快照。</p>
    {message && <p role="alert">{message}</p>}
    {phase === 'idle' && <Button className="min-h-11" onClick={() => void perform('preview')}>预览当前可见成员</Button>}
    {phase === 'busy' && <p role="status">处理原快照请求…</p>}
    {phase === 'confirm' && observation && <div className="space-y-3"><p>集合修订 {observation.revision} · 本次可见 {observation.visible_count} 项。不可见对象不进入观察，不代表全量集合。</p><ul aria-label="快照创建预览" className="m-0 list-none p-0">{observation.items.slice(0, shown).map(row => <li className="break-all py-2" key={row.file_id}>{row.path || row.file_id}</li>)}</ul>{shown < observation.items.length && <Button variant="outline" className="min-h-11" onClick={() => setShown(value => value + 50)}>展开更多观察成员</Button>}<p>此操作将持久化一份新快照。资料包登记、内容检查与执行许可是另外的操作。</p><Button className="min-h-11" onClick={() => void perform('create')}>确认创建提交时快照</Button><Button variant="outline" className="min-h-11" onClick={() => { setObservation(null); original.current = null; setPhase('idle'); }}>返回重新预览</Button></div>}
    {phase === 'unknown' && <div className="space-y-3"><p>存在未确认的原快照请求；刷新后只查原回执，不查最新快照代替。</p><Button className="min-h-11" onClick={() => void perform('lookup')}>查询原快照回执</Button>{retry && <><p>原键重试可能首次创建，只在仍要冻结提交时观察时确认。</p><Button variant="outline" className="min-h-11" onClick={() => void perform('retry')}>确认按原键重试快照</Button></>}</div>}
    {phase === 'done' && receipt && <div className="space-y-3"><p role="status">已确认原快照创建</p><p className="break-all select-text">快照标识：{receipt.snapshot_id}</p><p>原集合修订：{receipt.collection_revision} · {receipt.committed_at}</p><p>回执仅证明原提交，不证明全部条目当前可见或内容安全。确认关闭前可复制标识供资源库登记。</p><Button variant="outline" className="min-h-11" onClick={() => void copyID()}>复制快照标识</Button><Button ref={readTrigger} variant="outline" className="min-h-11" onClick={() => setReading(true)}>查看原快照成员</Button>{reading && <ResourceSnapshot snapshot={receipt.snapshot_id} close={() => { setReading(false); readTrigger.current?.focus(); }} onDenied={() => { wipe(); setPhase('denied'); denied(); }}/>}<Button className="min-h-11" onClick={() => void acknowledge()}>确认快照回执并关闭</Button></div>}
    {phase === 'denied' && <p role="alert">范围、网络、权限或恢复身份已变化，快照信息已清除，请重新打开。</p>}
    {phase === 'unavailable' && <p role="alert">恢复存储或跨标签锁不可用，不能安全创建快照。</p>}
  </AppDialog>;
}
