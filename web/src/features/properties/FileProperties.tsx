import { useEffect, useRef, useState } from 'react';
import { api } from '../../api';
import { AppDialog } from '../../components/AppDialog';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { Textarea } from '../../components/ui/textarea';
import { claimUpdate, clearUpdate, readRecovery, typeLabel, type Catalog, type Recovery } from './property-recovery';
import { matchesSidecarReceipt, parseValue, sidecarDigest, sidecarSlot, validSidecar, type Sidecar, type SidecarPatch, type SidecarReceipt } from './sidecar-client';

type Target = { scope: string; space: string; file: string; version: string; readOnly?: boolean; onDirty?: (dirty: boolean) => void };
export function FilePropertiesEntry(props: Target & { path: string; history?: boolean }) {
  const [open, setOpen] = useState(false), [denied, setDenied] = useState(false);
  const trigger = useRef<HTMLButtonElement>(null);
  if (!/\.(pdf|png|jpe?g|gif|webp|mp3|wav|mp4|webm)$/i.test(props.path)) return <p className="text-xs text-muted-foreground">此格式暂不支持属性值编辑；Markdown/frontmatter 须独立无损合同，不使用 sidecar 替代。</p>;
  return <div><Button ref={trigger} className="min-h-11" variant="outline" disabled={denied} onClick={() => setOpen(true)}>{props.history ? `查看历史属性与回执 ${props.version}` : '查看文件属性'}</Button>{denied && <p role="alert">属性权限或范围已变化，请重新打开文件。</p>}{open && <FileProperties {...props} readOnly={props.readOnly || props.history} denied={() => { setDenied(true); setOpen(false); }} close={() => { setOpen(false); trigger.current?.focus(); }}/>}</div>;
}

export function FileProperties({ scope, space, file, version, readOnly = false, onDirty, close, denied }: Target & { close: () => void; denied: () => void }) {
  const [phase, setPhase] = useState('initializing'), [busy, setBusy] = useState(false), [message, setMessage] = useState(''), [discard, setDiscard] = useState(false);
  const [catalog, setCatalog] = useState<Catalog | null>(null), [base, setBase] = useState<Sidecar | null>(null), [shown, setShown] = useState(50);
  const [draft, setDraft] = useState<Record<string, { mode: 'set' | 'remove'; raw: string }>>({});
  const [record, setRecord] = useState<Recovery | null>(null), [receipt, setReceipt] = useState<SidecarReceipt | null>(null), [retry, setRetry] = useState(false);
  const original = useRef<SidecarPatch | null>(null), active = useRef(true), running = useRef(false), controller = useRef<AbortController | null>(null);
  const slot = sidecarSlot(scope, file, version), path = '/files/' + encodeURIComponent(file) + '/properties';
  const dirty = Object.keys(draft).length > 0;
  const clear = () => { setBase(null); setCatalog(null); setDraft({}); setReceipt(null); original.current = null; setDiscard(false); };
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
    onDirty?.(dirty && !record);
    const leave = (event: BeforeUnloadEvent) => { if (dirty && !record) { event.preventDefault(); event.returnValue = ''; } };
    window.addEventListener('beforeunload', leave); return () => { onDirty?.(false); window.removeEventListener('beforeunload', leave); };
  }, [dirty, record, onDirty]);
  async function perform(mode: 'load' | 'save' | 'lookup' | 'retry') {
    if (!active.current || running.current || !navigator.onLine || ((mode === 'save' || mode === 'retry') && readOnly)) return;
    running.current = true; setBusy(true); setMessage(''); setRetry(false); let recovery = record;
    const abort = new AbortController(); controller.current = abort;
    try {
      if (mode === 'load') {
        const definitions = await api<Catalog>('/spaces/' + encodeURIComponent(space) + '/property-definitions', 'GET', undefined, {}, abort.signal);
        if (!active.current || abort.signal.aborted) return;
        const value = await api<Sidecar>(path + '?version_id=' + encodeURIComponent(version), 'GET', undefined, {}, abort.signal);
        if (!active.current || abort.signal.aborted) return;
        if (!validSidecar(value, definitions, space, file, version)) throw new Error('invalid_sidecar');
        setCatalog(definitions); setBase(value); setDraft({}); setShown(50); setPhase('edit'); return;
      }
      if (mode === 'save') {
        if (!original.current) throw new Error('missing_original');
        const claim = await claimUpdate(slot, await sidecarDigest(file, original.current), abort.signal);
        if (!active.current || abort.signal.aborted) return;
        recovery = claim.record; setRecord(recovery);
        if (!claim.claimed) { clear(); setPhase('unknown'); return; }
      }
      if (!recovery || JSON.stringify(readRecovery(slot)) !== JSON.stringify(recovery)) throw new Error('recovery_changed');
      if (mode !== 'lookup') {
        if (!original.current || await sidecarDigest(file, original.current) !== recovery.digest) throw new Error('missing_original');
        if (!active.current || abort.signal.aborted || !navigator.onLine) return;
        await api(path, 'PATCH', original.current, { 'Idempotency-Key': recovery.key }, abort.signal);
      }
      if (!active.current || abort.signal.aborted) return;
      const value = await api<SidecarReceipt>(path + '/lookup', 'POST', { version_id: version, idempotency_key: recovery.key }, {}, abort.signal);
      if (!active.current || abort.signal.aborted) return;
      if (!matchesSidecarReceipt(value, recovery, file, version, original.current?.expected_revision) || JSON.stringify(readRecovery(slot)) !== JSON.stringify(recovery)) throw new Error('receipt_mismatch');
      clear(); setReceipt(value); setPhase('done');
    } catch (error) {
      if (!active.current || abort.signal.aborted) return;
      const status = (error as { status?: number }).status;
      if (status === 401 || status === 403) { clear(); setPhase('denied'); denied(); }
      else if (mode === 'load') { clear(); setPhase('idle'); setMessage('未取得一致的固定版本属性与字典；可能是格式不支持、引用失权、旧数据待迁移或服务问题，不代表没有属性。'); }
      else if (!recovery) setPhase('unavailable');
      else { if (status === 409 || status === 422) { original.current = null; setDraft({}); } setPhase('unknown'); setRetry(mode === 'lookup' && status === 404 && !!original.current); setMessage('未取得原属性修改回执；写入可能未启用、发生冲突或结果未知。保留原身份，不自动换版本／换键；未查到不证明从未提交。'); }
    } finally { running.current = false; if (active.current) setBusy(false); }
  }
  function prepare() {
    if (!base || !catalog || busy || readOnly || !dirty) return;
    try {
      const updates: SidecarPatch['updates'] = Object.create(null), remove_keys: string[] = [];
      for (const [key, change] of Object.entries(draft)) {
        const definition = catalog.definitions.find(row => row.key === key); if (!definition) throw new Error('unknown_key');
        if (change.mode === 'remove') remove_keys.push(key); else updates[key] = parseValue(change.raw, definition.type);
      }
      const input: SidecarPatch = { base_version_id: version, definition_revision: base.definition_revision, expected_revision: base.revision, updates, remove_keys: remove_keys.sort() };
      if (new TextEncoder().encode(JSON.stringify(input)).length > 256 * 1024) throw new Error('budget_exceeded');
      original.current = input; setMessage(''); setShown(50); setPhase('confirm');
    } catch { setMessage('请检查类型与预算：数字使用无多余零的标准 JSON 表示，列表使用 JSON 文本数组；日期须真实、引用须固定版本，空文本与删除不同。'); }
  }
  async function acknowledge() {
    if (!active.current || running.current || !record || !receipt) return;
    running.current = true;
    try { await clearUpdate(slot, record); if (active.current) close(); } catch { if (active.current) setMessage('恢复身份已变化，不能清除其他页面请求。'); } finally { running.current = false; }
  }
  const display = (value: unknown) => value === undefined ? '未设置' : JSON.stringify(value);
  return <AppDialog title="固定版本文件属性" close={() => { if (dirty && !record) setDiscard(true); else close(); }}>
    <div data-property-editor className="space-y-4">
      <p className="break-all text-xs text-muted-foreground">固定版本 {version}。仅编辑二进制 sidecar，不修改文件字节；写入默认关闭，服务端授权与双修订校验决定是否可保存。历史成功不代表当前引用仍有权读取。</p>
      {readOnly && <p>此入口只读，可查询原回执；不重试旧写入，不将旧值复制到最新版本。</p>}
      {message && <p role="alert">{message}</p>}{busy && <p role="status">处理文件属性请求…</p>}
      {discard && <div role="alert"><p>关闭将丢弃未提交的属性草稿。</p><Button className="min-h-11" onClick={close}>丢弃属性草稿并关闭</Button><Button variant="outline" className="min-h-11" onClick={() => setDiscard(false)}>继续编辑属性</Button></div>}
      {phase === 'idle' && <Button className="min-h-11" disabled={busy} onClick={() => void perform('load')}>读取固定版本属性</Button>}
      {phase === 'edit' && catalog && base && <div className="space-y-3"><p>字典修订 {base.definition_revision} · 属性修订 {base.revision} · {catalog.definitions.length} 个定义，每次本地展开 50 个。</p>
        {!!base.unknown_keys.length && <p className="break-all">保留的未知字段：{base.unknown_keys.join('、')}。不会显示、解释或删除其原始值。</p>}
        {!catalog.definitions.length && <p>此空间尚无属性定义，请先在“标签与属性”管理定义。</p>}
        {catalog.definitions.slice(0, shown).map(row => { const change = Object.hasOwn(draft, row.key) ? draft[row.key] : undefined, value = Object.hasOwn(base.values, row.key) ? base.values[row.key] : undefined; return <fieldset key={row.key} className="min-w-0 space-y-2 rounded-lg border border-border p-3"><legend className="break-all">{row.display_name} · {typeLabel[row.type]}</legend><p className="break-all text-xs">{row.key} · 当前值：{display(value)}</p>
          {!readOnly && <div role="group" aria-label={row.display_name + ' 修改方式'} className="flex flex-wrap gap-2">{(['keep', 'set', 'remove'] as const).map(mode => { const label = mode === 'keep' ? '保持原值' : mode === 'set' ? '设置值' : '删除此值'; return <Button key={mode} variant="outline" className="min-h-11" aria-label={row.display_name + ' ' + label} aria-pressed={(change?.mode || 'keep') === mode} disabled={busy || (mode === 'remove' && value === undefined)} onClick={() => setDraft(previous => { const next = { ...previous }; if (mode === 'keep') delete next[row.key]; else next[row.key] = { mode, raw: typeof value === 'string' ? value : value === undefined ? row.type === 'checkbox' ? 'false' : row.type === 'list' ? '[]' : '' : JSON.stringify(value) }; return next; })}>{label}</Button>; })}</div>}
          {change?.mode === 'set' && <label className="grid gap-2">{row.display_name} 新值{row.type === 'checkbox' ? <Button aria-label={row.display_name + ' 新值'} aria-pressed={change.raw === 'true'} className="min-h-11" variant="outline" disabled={busy} onClick={() => setDraft(previous => ({ ...previous, [row.key]: { mode: 'set', raw: change.raw === 'true' ? 'false' : 'true' } }))}>{change.raw === 'true' ? '是' : '否'}</Button> : row.type === 'text' || row.type === 'list' ? <Textarea aria-label={row.display_name + ' 新值'} className="min-h-24 w-full rounded-md border border-input bg-background p-3" maxLength={16384} disabled={busy} value={change.raw} onChange={event => setDraft(previous => ({ ...previous, [row.key]: { mode: 'set', raw: event.target.value } }))}/> : <Input aria-label={row.display_name + ' 新值'} className="min-h-11" type={row.type === 'date' ? 'date' : 'text'} maxLength={16384} disabled={busy} value={change.raw} onChange={event => setDraft(previous => ({ ...previous, [row.key]: { mode: 'set', raw: event.target.value } }))}/>}</label>}
          {change?.mode === 'set' && row.type === 'list' && <p className="text-xs">JSON 文本数组，例如 ["项目", "参考"]；保留顺序与重复值，[] 是空列表。</p>}
        </fieldset>; })}
        {shown < catalog.definitions.length && <Button className="min-h-11" disabled={busy} onClick={() => setShown(value => value + 50)}>展开更多属性</Button>}
        {!readOnly && <Button className="min-h-11" disabled={busy || !dirty} onClick={prepare}>预览属性修改</Button>}
      </div>}
      {phase === 'confirm' && original.current && <div className="space-y-3"><p>仅提交以下 {Object.keys(draft).length} 项改动，其他已知值与未知字段原样保留。按原版本、字典修订 {base?.definition_revision}、属性修订 {base?.revision} 保存。</p><ul aria-label="属性修改差异" className="list-none p-0">{Object.entries(draft).slice(0, shown).map(([key, change]) => <li key={key} className="break-all py-2">{key}：{display(base && Object.hasOwn(base.values, key) ? base.values[key] : undefined)} → {change.mode === 'remove' ? '删除（不是空文本）' : display(original.current?.updates[key])}</li>)}</ul>{shown < Object.keys(draft).length && <Button className="min-h-11" onClick={() => setShown(value => value + 50)}>展开更多属性差异</Button>}<Button className="min-h-11" disabled={busy} onClick={() => void perform('save')}>确认保存文件属性</Button><Button className="min-h-11" variant="outline" disabled={busy} onClick={() => { original.current = null; setPhase('edit'); }}>返回属性修改</Button></div>}
      {phase === 'unknown' && <div className="space-y-3"><p>存在尚未确认的原属性请求；刷新后只查询原版本的回执，不读取当前值猜测成功。</p><Button className="min-h-11" disabled={busy} onClick={() => void perform('lookup')}>查询原属性回执</Button>{retry && !readOnly && <><p>重试可能首次执行；保留原版本、双修订与原键。</p><Button className="min-h-11" disabled={busy} onClick={() => void perform('retry')}>确认原键重试属性</Button></>}</div>}
      {phase === 'done' && receipt && <div className="space-y-3"><p role="status">已确认原文件属性修改</p><p>原结果修订 {receipt.result_revision} · {receipt.committed_at}</p><p>此回执不是当前值；关闭后重新打开并手动读取。</p><Button className="min-h-11" onClick={() => void acknowledge()}>确认属性回执并关闭</Button></div>}
      {phase === 'denied' && <p role="alert">权限、范围、网络或恢复身份已变化，属性草稿已清除；请关闭后重新鉴权。</p>}
      {phase === 'unavailable' && <p role="alert">恢复存储或跨标签锁不可用，不能安全修改文件属性。</p>}
    </div>
  </AppDialog>;
}
