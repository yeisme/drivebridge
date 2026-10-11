import { useEffect, useRef, useState } from 'react';
import { api } from '../../api';
import { useRouteLeaveGuard } from '../../app/route-leave';
import { AppDialog } from '../../components/AppDialog';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { SelectAdapter } from '../../components/SelectAdapter';
import { claimUpdate, clearUpdate, matchesReceipt, propertyDigest, propertySlot, readRecovery, TYPES, typeLabel, validCatalog, validDefinition, type Catalog, type Definition, type Receipt, type Recovery, type Update } from './property-recovery';

export function PropertyEditor({ scope, space, close, denied }: { scope: string; space: string; close: () => void; denied: () => void }) {
  const [phase, setPhase] = useState('initializing'), [message, setMessage] = useState(''), [busy, setBusy] = useState(false), [discard, setDiscard] = useState(false);
  const [base, setBase] = useState<Catalog | null>(null), [draft, setDraft] = useState<Definition[]>([]), [shown, setShown] = useState(50);
  const [record, setRecord] = useState<Recovery | null>(null), [receipt, setReceipt] = useState<Receipt | null>(null), [retry, setRetry] = useState(false);
  const original = useRef<Update | null>(null), active = useRef(true), running = useRef(false), controller = useRef<AbortController | null>(null);
  const slot = propertySlot(scope, space), path = '/spaces/' + encodeURIComponent(space) + '/property-definitions';
  const dirty = !!base && JSON.stringify(base.definitions) !== JSON.stringify(draft);
  useRouteLeaveGuard(dirty && !record, '放弃未提交的属性字典草稿并离开？');
  const clear = () => { setBase(null); setDraft([]); setReceipt(null); original.current = null; setDiscard(false); };
  useEffect(() => {
    active.current = true;
    try { const stored = readRecovery(slot); setRecord(stored); setPhase(stored ? 'unknown' : 'idle'); } catch { setPhase('unavailable'); }
    const invalidate = () => { active.current = false; controller.current?.abort(); clear(); setMessage(''); setPhase('denied'); };
    const storage = (event: StorageEvent) => { if (event.key === null || event.key === slot) invalidate(); };
    window.addEventListener('offline', invalidate); window.addEventListener('drivebridge-context', invalidate); window.addEventListener('storage', storage);
    if (!navigator.onLine) invalidate();
    return () => { active.current = false; controller.current?.abort(); original.current = null; window.removeEventListener('offline', invalidate); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('storage', storage); };
  }, [slot]);
  useEffect(() => {
    const leave = (event: BeforeUnloadEvent) => { if (dirty && !record) { event.preventDefault(); event.returnValue = ''; } };
    window.addEventListener('beforeunload', leave); return () => window.removeEventListener('beforeunload', leave);
  }, [dirty, record]);
  async function perform(mode: 'load' | 'save' | 'lookup' | 'retry') {
    if (!active.current || running.current || !navigator.onLine) return;
    running.current = true; setBusy(true); setMessage(''); setRetry(false);
    const abort = new AbortController(); controller.current = abort; let recovery = record;
    try {
      if (mode === 'load') {
        const value = await api<Catalog>(path, 'GET', undefined, {}, abort.signal);
        if (!active.current || abort.signal.aborted) return;
        if (!validCatalog(value, space) || value.revision === Number.MAX_SAFE_INTEGER) throw new Error('invalid_catalog');
        setBase(value); setDraft(value.definitions); setShown(50); setPhase('edit'); return;
      }
      if (mode === 'save') {
        if (!original.current) throw new Error('missing_original');
        const claim = await claimUpdate(slot, await propertyDigest(space, original.current), abort.signal);
        if (!active.current || abort.signal.aborted) return;
        recovery = claim.record; setRecord(recovery);
        if (!claim.claimed) { clear(); setPhase('unknown'); return; }
      }
      if (!recovery || JSON.stringify(readRecovery(slot)) !== JSON.stringify(recovery)) throw new Error('recovery_changed');
      if (mode !== 'lookup') {
        if (!original.current || await propertyDigest(space, original.current) !== recovery.digest) throw new Error('missing_original');
        if (!active.current || abort.signal.aborted || !navigator.onLine) return;
        await api(path, 'PUT', original.current, { 'Idempotency-Key': recovery.key }, abort.signal);
      }
      if (!active.current || abort.signal.aborted) return;
      const value = await api<Receipt>(path + '/lookup', 'POST', { idempotency_key: recovery.key }, {}, abort.signal);
      if (!matchesReceipt(value, recovery, space, original.current?.expected_revision) || JSON.stringify(readRecovery(slot)) !== JSON.stringify(recovery)) throw new Error('receipt_mismatch');
      if (!active.current || abort.signal.aborted) return;
      clear(); setReceipt(value); setPhase('done');
    } catch (error) {
      if (!active.current || abort.signal.aborted) return;
      const status = (error as { status?: number }).status;
      if (status === 401 || status === 403) { clear(); setPhase('denied'); denied(); }
      else if (mode === 'load') setMessage('未取得完整可编辑字典，可能是旧数据需迁移、权限或服务问题；没有部分表单可保存。');
      else if (!recovery) setPhase('unavailable');
      else { if (status === 409 || status === 422) original.current = null; setPhase('unknown'); setRetry(mode === 'lookup' && status === 404 && !!original.current); setMessage('未取得原字典修改回执，可能冲突或结果未知。保留原身份，不自动换修订／换键覆盖；未查到不证明从未提交。'); }
    } finally { running.current = false; if (active.current) setBusy(false); }
  }
  function prepare() {
    if (!base || busy || !dirty) return;
    if (draft.length > 200 || !draft.every(validDefinition) || new Set(draft.map(row => row.key)).size !== draft.length) { setMessage('请检查唯一稳定 key、字段名称、分组与类型；不允许空名称或控制字符。'); return; }
    original.current = { expected_revision: base.revision, definitions: draft.map(row => ({ ...row })) }; setShown(50); setMessage(''); setPhase('confirm');
  }
  async function acknowledge() {
    if (!active.current || running.current || !record || !receipt) return;
    running.current = true;
    try { await clearUpdate(slot, record); if (active.current) close(); } catch { if (active.current) setMessage('恢复身份已变化，不能清除其他页面请求。'); } finally { running.current = false; }
  }
  const differences = draft.filter(row => { const old = base?.definitions.find(item => item.key === row.key); return !old || old.display_name !== row.display_name || old.group !== row.group; });
  const change = (index: number, patch: Partial<Definition>) => setDraft(rows => rows.map((row, i) => i === index ? { ...row, ...patch } : row));
  return <AppDialog title="编辑属性字典" close={() => { if (dirty && !record) setDiscard(true); else close(); }}>
    <p className="text-sm text-muted-foreground">定义不会写入文件属性值。已有稳定 key 与类型不可直接更改或删除，需要单独迁移。草稿仅留内存，浏览器只保存原请求键与摘要。</p>
    {message && <p role="alert">{message}</p>}{busy && <p role="status">处理属性字典请求…</p>}
    {discard && <div role="alert"><p>关闭将丢弃未提交的字典草稿。</p><Button className="min-h-11" onClick={close}>丢弃字典草稿并关闭</Button><Button variant="outline" className="min-h-11" onClick={() => setDiscard(false)}>继续编辑</Button></div>}
    {phase === 'idle' && <Button className="min-h-11" disabled={busy} onClick={() => void perform('load')}>读取完整属性字典</Button>}
    {phase === 'edit' && base && <div className="space-y-4"><p>基线修订 {base.revision} · {draft.length}/200 个定义。每次展开 50 个为本地展示。</p>
      {draft.slice(0, shown).map((row, index) => { const existing = index < base.definitions.length; return <fieldset key={index} className="space-y-3 rounded-lg border border-border p-3"><legend>字段 {index + 1}{existing ? ' · 已保存' : ' · 新增草稿'}</legend><label className="grid gap-2">稳定 key<Input aria-label={`字段 ${index + 1} 稳定 key`} className="min-h-11" value={row.key} maxLength={64} disabled={busy || existing} onChange={event => change(index, { key: event.target.value })}/></label><label className="grid gap-2">显示名<Input aria-label={`字段 ${index + 1} 显示名`} className="min-h-11" value={row.display_name} maxLength={240} disabled={busy} onChange={event => change(index, { display_name: event.target.value })}/></label><label className="grid gap-2">分组<Input aria-label={`字段 ${index + 1} 分组`} className="min-h-11" value={row.group} maxLength={160} disabled={busy} onChange={event => change(index, { group: event.target.value })}/></label><label className="grid gap-2">类型<SelectAdapter aria-label={`字段 ${index + 1} 类型`} className="min-h-11" value={row.type} disabled={busy || existing} onChange={event => change(index, { type: event.target.value as Definition['type'] })}>{TYPES.map(type => <option key={type} value={type}>{typeLabel[type]}</option>)}</SelectAdapter></label>{!existing && <Button className="min-h-11" variant="outline" disabled={busy} onClick={() => setDraft(rows => rows.filter((_, i) => i !== index))}>移除新增字段 {index + 1}</Button>}</fieldset>; })}
      {shown < draft.length && <Button className="min-h-11" disabled={busy} onClick={() => setShown(value => value + 50)}>展开更多字段</Button>}
      <Button className="min-h-11" variant="outline" disabled={busy || draft.length >= 200} onClick={() => { setDraft(rows => [...rows, { key: '', display_name: '', group: '', type: 'text' }]); setShown(Math.ceil((draft.length + 1) / 50) * 50); }}>添加属性字段</Button>
      <Button className="min-h-11" disabled={busy || !dirty} onClick={prepare}>预览字典修改</Button>
    </div>}
    {phase === 'confirm' && <div className="space-y-3"><p>按原修订 {base?.revision} 保存完整字典；{differences.length} 个字段新增或显示信息改变，未变定义原样保留。</p><ul aria-label="字典修改差异" className="m-0 list-none p-0">{differences.slice(0, shown).map(row => { const old = base?.definitions.find(item => item.key === row.key); return <li key={row.key} className="break-all py-2">{row.key} · {typeLabel[row.type]}：{old ? `${old.display_name} / ${old.group || '未分组'}` : '新增'} → {row.display_name} / {row.group || '未分组'}</li>; })}</ul>{shown < differences.length && <Button className="min-h-11" disabled={busy} onClick={() => setShown(value => value + 50)}>展开更多字典差异</Button>}<Button className="min-h-11" disabled={busy} onClick={() => void perform('save')}>确认保存属性字典</Button><Button variant="outline" className="min-h-11" disabled={busy} onClick={() => { original.current = null; setPhase('edit'); }}>返回修改</Button></div>}
    {phase === 'unknown' && <div className="space-y-3"><p>存在尚未确认的原字典请求；刷新后只查原回执，不读取当前字典猜测成功。</p><Button className="min-h-11" disabled={busy} onClick={() => void perform('lookup')}>查询原字典回执</Button>{retry && <><p>原键重试可能首次执行，仍使用原修订和原定义，不能覆盖新版本。</p><Button className="min-h-11" disabled={busy} onClick={() => void perform('retry')}>确认按原键重试字典</Button></>}</div>}
    {phase === 'done' && receipt && <div className="space-y-3"><p role="status">已确认原属性字典修改</p><p>原结果修订：{receipt.result_revision} · {receipt.committed_at}</p><p>此回执不是当前字典，也不表示已更改文件属性值。关闭后请手动刷新。</p><Button className="min-h-11" onClick={() => void acknowledge()}>确认字典回执并关闭</Button></div>}
    {phase === 'denied' && <p role="alert">范围、网络、权限或恢复身份已变化，字典草稿已清除。</p>}
    {phase === 'unavailable' && <p role="alert">恢复存储或跨标签锁不可用，不能安全修改字典。</p>}
  </AppDialog>;
}
