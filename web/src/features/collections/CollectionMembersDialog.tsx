import { useEffect, useRef, useState } from 'react';
import { api, type Space } from '../../api';
import { useRouteLeaveGuard } from '../../app/route-leave';
import { SelectAdapter } from '../../components/SelectAdapter';
import { AppDialog } from '../../components/AppDialog';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { claimMembers, clearMembers, decisionDiff, fileID, matchesMemberReceipt, memberDigest, memberSlot, readMemberRecovery, validDecisions, type Decision, type Decisions, type MemberChange, type MemberReceipt, type MemberRecovery } from './member-recovery';

type Candidate = { file_id: string; version_id: string; path: string; space_id?: string };
type QueryPage = { mode: string; items: Candidate[]; next_cursor?: string; coverage: string };
const label = (value: string) => value === 'include' ? '包含' : value === 'exclude' ? '排除' : '无显式决策';
export function CollectionMembersDialog({ scope, id, space, spaces, close, denied }: { scope: string; id: string; space: string; spaces?: Pick<Space, 'id' | 'name'>[]; close: () => void; denied: () => void }) {
  const options = spaces ?? [{ id: space, name: space }];
  const [searchSpace, setSearchSpace] = useState(space);
  const optionIDs = JSON.stringify(options.map(item => item.id).sort());
  const allowed = useRef(options); allowed.current = options;
  const sourceName = (id: string) => options.find(item => item.id === id)?.name || id;
  const [phase, setPhase] = useState('initializing'), [message, setMessage] = useState('');
  const [base, setBase] = useState<Decisions | null>(null), [draft, setDraft] = useState<Decision[]>([]), [shown, setShown] = useState(50);
  const [query, setQuery] = useState(''), [candidates, setCandidates] = useState<Candidate[]>([]), [next, setNext] = useState(''), [searched, setSearched] = useState(false);
  const [record, setRecord] = useState<MemberRecovery | null>(null), [receipt, setReceipt] = useState<MemberReceipt | null>(null), [retry, setRetry] = useState(false);
  const [busy, setBusy] = useState(false), [discard, setDiscard] = useState(false);
  const running = useRef(false), active = useRef(true), controller = useRef<AbortController | null>(null), original = useRef<MemberChange | null>(null);
  const slot = memberSlot(scope, id), path = '/file-collections/' + encodeURIComponent(id) + '/members';
  const differences = base ? decisionDiff(base.items, draft) : [];
  const unsent = !record && differences.length > 0;
  useRouteLeaveGuard(unsent, '放弃未提交的集合成员草稿并离开？');
  useEffect(() => {
    if (!unsent) return;
    const leave = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ''; };
    window.addEventListener('beforeunload', leave);
    return () => window.removeEventListener('beforeunload', leave);
  }, [unsent]);
  const wipe = () => { setBase(null); setDraft([]); setCandidates([]); setQuery(''); setNext(''); setReceipt(null); original.current = null; setDiscard(false); };
  useEffect(() => {
    active.current = true; wipe(); setBusy(false);
    try { const stored = readMemberRecovery(slot); setRecord(stored); setPhase(stored ? 'unknown' : 'idle'); } catch { setPhase('unavailable'); }
    const invalidate = () => { active.current = false; controller.current?.abort(); wipe(); setMessage(''); setPhase('denied'); };
    const storage = (event: StorageEvent) => { if (event.key === null || event.key === slot) invalidate(); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('offline', invalidate); window.addEventListener('storage', storage);
    if (!navigator.onLine) invalidate();
    return () => { active.current = false; controller.current?.abort(); original.current = null; window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', invalidate); window.removeEventListener('storage', storage); };
  }, [slot, optionIDs]);
  useEffect(() => { setCandidates([]); setNext(''); setSearched(false); setSearchSpace(space); }, [optionIDs, space]);
  async function read(mode: 'base' | 'search', cursor = '') {
    if (!active.current || running.current || !navigator.onLine) return;
    if (mode === 'search' && (!allowed.current.some(item => item.id === searchSpace) || !query.trim() || new TextEncoder().encode(query).length > 4096)) { setMessage('请输入路径关键词。'); return; }
    running.current = true; setBusy(true); setMessage('');
    const abort = new AbortController(); controller.current = abort;
    if (mode === 'search') { setCandidates([]); setNext(''); setSearched(false); }
    try {
      if (mode === 'base') {
        const result = await api<Decisions>(path + '/decisions', 'GET', undefined, {}, abort.signal);
        if (!active.current || abort.signal.aborted) return;
        if (!validDecisions(result, id)) throw new Error('invalid_contract');
        setBase(result); setDraft(result.items); setShown(50); setPhase('edit');
      } else {
        const result = await api<QueryPage>('/file-queries', 'POST', { mode: 'exact', space_ids: [searchSpace], filters: [{ field: 'path_prefix', op: 'contains', value: query }], limit: 50, ...(cursor ? { cursor } : {}) }, {}, abort.signal);
        if (!active.current || abort.signal.aborted) return;
        if (!result || result.mode !== 'exact' || !['complete', 'partial'].includes(result.coverage) || !Array.isArray(result.items) || result.items.length > 50 || !result.items.every(row => row && fileID(row.file_id) && fileID(row.version_id) && typeof row.path === 'string') || new Set(result.items.map(row => row.file_id)).size !== result.items.length || (result.next_cursor !== undefined && (typeof result.next_cursor !== 'string' || result.next_cursor.length > 16384))) throw new Error('invalid_contract');
        if (!allowed.current.some(item => item.id === searchSpace)) throw new Error('source_unavailable');
        setCandidates(result.items.map(row => ({ ...row, space_id: searchSpace }))); setNext(result.next_cursor || ''); setSearched(true);
      }
    } catch (error) {
      if (!active.current || abort.signal.aborted) return;
      if ([401, 403].includes((error as { status?: number }).status ?? 0)) { wipe(); setPhase('denied'); denied(); }
      else setMessage(mode === 'base' ? '未取得完整编辑基线：可能有不可见决策、超预算或服务错误。没有部分表单可保存。' : '搜索不可用或分页失效；请重新搜索，不代表没有文件。');
    } finally { running.current = false; if (active.current) setBusy(false); }
  }
  function choose(row: Decision, decision: 'include' | 'exclude' | 'none') {
    if (busy || phase !== 'edit') return;
    if (!allowed.current.some(item => item.id === row.space_id) && !base?.items.some(item => item.file_id === row.file_id)) { setMessage('来源空间已不在当前选择范围，请重新搜索。'); return; }
    const exists = draft.some(item => item.file_id === row.file_id);
    if (!exists && draft.length >= 10000) { setMessage('显式决策已达 10000 条，请先移除再添加。'); return; }
    setDraft(items => decision === 'none' ? items.filter(item => item.file_id !== row.file_id) : exists ? items.map(item => item.file_id === row.file_id ? { ...item, decision } : item) : [...items, { ...row, decision }]);
    setMessage('');
  }
  function prepare() {
    if (!base || !differences.length || busy) return;
    original.current = { expected_revision: base.revision, include: draft.filter(row => row.decision === 'include').map(row => row.file_id), exclude: draft.filter(row => row.decision === 'exclude').map(row => row.file_id) };
    setCandidates([]); setQuery(''); setNext(''); setShown(50); setPhase('confirm'); setMessage('');
  }
  async function submit(mode: 'save' | 'lookup' | 'retry') {
    if (!active.current || running.current || !navigator.onLine) return;
    running.current = true; setBusy(true); setMessage(''); setRetry(false);
    const abort = new AbortController(); controller.current = abort; let recovery = record;
    try {
      if (mode === 'save') {
        if (!original.current) throw new Error('missing_original');
        const claim = await claimMembers(slot, await memberDigest(id, original.current), abort.signal);
        if (!active.current || abort.signal.aborted) return;
        recovery = claim.record; setRecord(recovery);
        if (!claim.claimed) { wipe(); setPhase('unknown'); return; }
      }
      if (!recovery || JSON.stringify(readMemberRecovery(slot)) !== JSON.stringify(recovery)) throw new Error('recovery_changed');
      if (mode !== 'lookup') {
        if (!original.current || await memberDigest(id, original.current) !== recovery.digest) throw new Error('missing_original');
        if (!active.current || abort.signal.aborted || !navigator.onLine) return;
        await api(path + '/changes', 'POST', original.current, { 'Idempotency-Key': recovery.key }, abort.signal);
      }
      if (!active.current || abort.signal.aborted) return;
      const result = await api<MemberReceipt>(path + '/changes/lookup', 'POST', { idempotency_key: recovery.key }, {}, abort.signal);
      if (!matchesMemberReceipt(result, recovery, id, original.current?.expected_revision) || JSON.stringify(readMemberRecovery(slot)) !== JSON.stringify(recovery)) throw new Error('receipt_mismatch');
      if (!active.current || abort.signal.aborted) return;
      wipe(); setReceipt(result); setPhase('done');
    } catch (error) {
      if (!active.current || abort.signal.aborted) return;
      const status = (error as { status?: number }).status;
      if (status === 401 || status === 403) { wipe(); setPhase('denied'); denied(); }
      else if (!recovery) { setPhase('unavailable'); }
      else {
        if (status === 409) original.current = null;
        setPhase('unknown'); setRetry(mode === 'lookup' && status === 404 && !!original.current);
        setMessage(status === 409 ? '修订或原请求冲突。没有覆盖服务器；保留原身份查询，不自动换修订重写。' : '未取得匹配的原成员修改回执。未查到不证明从未提交，不要换键覆盖。');
      }
    } finally { running.current = false; if (active.current) setBusy(false); }
  }
  async function acknowledge() {
    if (!active.current || running.current || !receipt || !record) return;
    running.current = true;
    try { await clearMembers(slot, record); if (active.current) close(); }
    catch { if (active.current) setMessage('恢复身份已变化，不能清除其他页面的请求。'); }
    finally { running.current = false; }
  }
  function requestClose() { if (!record && differences.length) setDiscard(true); else close(); }
  return <AppDialog title="编辑集合成员" close={requestClose}>
    <p className="text-sm text-muted-foreground">只调整显式包含与排除，不移动或删除文件、不修改规则。文件身份随当前版本变化；不是固定快照。路径与选择仅留内存。</p>
    {message && <p role="alert">{message}</p>}
    {discard && <div role="alert"><p>关闭将丢弃未提交的成员草稿。</p><Button className="min-h-11" onClick={close}>丢弃草稿并关闭</Button><Button variant="outline" className="min-h-11" onClick={() => setDiscard(false)}>继续编辑</Button></div>}
    {busy && <p role="status">处理成员请求…</p>}
    {phase === 'idle' && <Button className="min-h-11" disabled={busy} onClick={() => void read('base')}>读取完整成员决策</Button>}
    {phase === 'edit' && base && <div className="space-y-4">
      <p>基线修订 {base.revision} · 显式决策 {draft.length} 条。包含与排除都保留；下面每次展开 50 条只是本地展示。</p>
      <ul aria-label="显式成员决策" className="m-0 list-none divide-y divide-border p-0">{draft.slice(0, shown).map(row => <li key={row.file_id} className="py-3"><p className="break-all">{row.path} · {label(row.decision)}</p><p className="break-all text-xs text-muted-foreground">来源空间：{sourceName(row.space_id)} · {row.space_id}</p><div className="flex flex-wrap gap-2"><Button className="min-h-11" variant="outline" disabled={busy} onClick={() => choose(row, row.decision === 'include' ? 'exclude' : 'include')}>{row.decision === 'include' ? '改为排除' : '改为包含'} {row.path}</Button><Button className="min-h-11" variant="outline" disabled={busy} onClick={() => choose(row, 'none')}>移除决策 {row.path}</Button></div></li>)}</ul>
      {shown < draft.length && <Button className="min-h-11" disabled={busy} onClick={() => setShown(value => value + 50)}>展开更多决策</Button>}
      <form onSubmit={event => { event.preventDefault(); void read('search'); }}><label className="grid gap-2">文件来源空间<SelectAdapter aria-label="成员来源空间" className="min-h-11" value={searchSpace} disabled={busy} onChange={event => { if (!options.some(item => item.id === event.target.value)) return; setSearchSpace(event.target.value); setCandidates([]); setNext(''); setSearched(false); setMessage(''); }}>{options.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</SelectAdapter></label><label className="grid gap-2">所选空间路径搜索<Input aria-label="成员路径关键词" className="min-h-11" disabled={busy} value={query} maxLength={4096} onChange={event => { setQuery(event.target.value); setCandidates([]); setNext(''); setSearched(false); }}/></label><Button className="min-h-11" type="submit" disabled={busy}>搜索可选文件</Button></form>
      {searched && !candidates.length && <p role="status">本次搜索页没有可选文件。</p>}
      <ul aria-label="可选文件" className="m-0 list-none divide-y divide-border p-0">{candidates.map(row => <li key={row.file_id} className="py-3"><p className="break-all">{row.path}</p><p className="break-all text-xs text-muted-foreground">来源空间：{sourceName(row.space_id || '')} · {row.space_id}</p><div className="flex flex-wrap gap-2">{(['include', 'exclude'] as const).map(decision => <Button key={decision} className="min-h-11" variant="outline" disabled={busy || draft.some(item => item.file_id === row.file_id)} onClick={() => choose({ file_id: row.file_id, space_id: row.space_id!, path: row.path, decision }, decision)}>添加{label(decision)} {row.path}</Button>)}</div></li>)}</ul>
      {next && <Button className="min-h-11" variant="outline" disabled={busy} onClick={() => void read('search', next)}>下一页可选文件</Button>}
      <p className="text-xs text-muted-foreground">只搜索所选授权空间，每页最多 50 项，仅元数据；当前页不是全部文件。切换来源不丢弃已选择决策，跨空间不移动文件；同名路径按文件标识区分。保存时 owner 再核当前范围、各文件权限与原修订。</p>
      <Button className="min-h-11" disabled={busy || !differences.length} onClick={prepare}>预览成员修改</Button>
    </div>}
    {phase === 'confirm' && <div className="space-y-3"><p>将按原修订 {base?.revision} 替换完整显式决策，共 {differences.length} 项变化；未变项原样保留。</p><ul aria-label="成员修改差异" className="m-0 list-none p-0">{differences.slice(0, shown).map(row => <li key={row.file_id} className="break-all py-2"><span>{row.path}：{label(row.before)} → {label(row.after)}</span><p className="break-all text-xs text-muted-foreground">来源空间：{sourceName(row.space_id)} · {row.space_id}</p></li>)}</ul>{shown < differences.length && <Button className="min-h-11" disabled={busy} onClick={() => setShown(value => value + 50)}>展开更多差异</Button>}<Button className="min-h-11" disabled={busy} onClick={() => void submit('save')}>确认保存成员决策</Button><Button className="min-h-11" variant="outline" disabled={busy} onClick={() => { original.current = null; setShown(50); setPhase('edit'); }}>返回修改</Button></div>}
    {phase === 'unknown' && <div className="space-y-3"><p>存在尚未确认的原成员请求。关闭或刷新保留原键，草稿不持久化。</p><Button className="min-h-11" disabled={busy} onClick={() => void submit('lookup')}>查询原成员回执</Button>{retry && <><p>原键重试可能首次执行，仍使用原修订和完整原决策；不会覆盖新版本。</p><Button className="min-h-11" disabled={busy} onClick={() => void submit('retry')}>确认按原键重试成员修改</Button></>}</div>}
    {phase === 'done' && receipt && <div className="space-y-3"><p role="status">已确认原成员修改</p><p>原结果修订：{receipt.result_revision} · {receipt.committed_at}</p><p>这是历史提交事实，不是当前成员。关闭后请手动刷新集合列表。</p><Button className="min-h-11" onClick={() => void acknowledge()}>确认成员回执并关闭</Button></div>}
    {phase === 'denied' && <p role="alert">范围、网络、权限或恢复身份已变化，成员草稿已清除；请重新打开。</p>}
    {phase === 'unavailable' && <p role="alert">恢复存储或跨标签锁不可用，不能安全修改成员。</p>}
  </AppDialog>;
}
