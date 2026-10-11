import { useEffect, useRef, useState } from 'react';
import { api, getDataContext, type DriveFile } from '../../api';
import { type SessionView, verifySessionIdentity } from '../../app/session';
import type { components } from '../../generated/personal-workspace';
import { Button } from '../../components/ui/button';
import { AppDialog } from '../../components/AppDialog';

type Schemas = components['schemas'];
type HoldPage = Schemas['WorkspaceHoldListEnvelope']['data'];
type Hold = HoldPage['items'][number];
type Receipt = Schemas['WorkspaceHoldReceiptEnvelope']['data'];
type Recovery = { key: string; hold: string; expected: number; digest: string };
const stateLabels: Record<Hold['state'], string> = { active: '保护生效', released: '已解除', inactive_source_or_expiry: '来源失效或已到期' };
const purposeLabels: Record<string, string> = { fixed_delivery: '固定交付', legal_hold: '法律保留', transfer_window: '传输窗口' };
const hexID = (value: string) => /^[a-f0-9]{48}$/.test(value);
function readRecovery(slot: string): Recovery | null {
  const raw = localStorage.getItem(slot); if (raw === null) return null;
  if (raw.length > 512) throw new Error('invalid_recovery');
  const value = JSON.parse(raw) as Recovery;
  if (!value || Object.keys(value).sort().join(',') !== 'digest,expected,hold,key' || !/^hold-[a-f0-9]{48}$/.test(value.key) || !hexID(value.hold) || !/^[a-f0-9]{64}$/.test(value.digest) || !Number.isSafeInteger(value.expected) || value.expected < 1 || value.expected >= Number.MAX_SAFE_INTEGER) throw new Error('invalid_recovery');
  return value;
}
async function clearRecovery(slot: string, record: Recovery, signal: AbortSignal) {
  if (!navigator.locks?.request) throw new Error('lock_unavailable');
  await navigator.locks.request(slot, { signal }, () => {
    signal.throwIfAborted();
    if (JSON.stringify(readRecovery(slot)) !== JSON.stringify(record)) throw new Error('recovery_changed');
    localStorage.removeItem(slot);
  });
}
export function TrashHolds({ file, restricted }: { file: DriveFile; restricted: boolean }) {
  const [open, setOpen] = useState(false);
  const trigger = useRef<HTMLButtonElement | null>(null);
  return <><Button ref={trigger} type="button" variant="outline" className="min-h-11 max-w-full text-xs md:min-h-8" aria-label={'保留保护 ' + file.path} onClick={() => setOpen(true)}>保留保护</Button>{open && <HoldDialog key={JSON.stringify([file.space_id, file.id, file.version_id, restricted])} file={file} restricted={restricted} close={() => { setOpen(false); trigger.current?.focus(); }}/>}</>;
}
function HoldDialog({ file, restricted, close }: { file: DriveFile; restricted: boolean; close: () => void }) {
  const context = getDataContext();
  const [state, setState] = useState('loading'), [attempt, setAttempt] = useState(0), [busy, setBusy] = useState(false), [notice, setNotice] = useState('');
  const [page, setPage] = useState<HoldPage | null>(null), [confirm, setConfirm] = useState<Hold | null>(null), [pending, setPending] = useState<Recovery | null>(null);
  const active = useRef<AbortController | null>(null), running = useRef(false), slot = useRef(''), actor = useRef({ user: '', instance: '' }), cursors = useRef(new Set<string>());
  const path = '/files/' + encodeURIComponent(file.id) + '/retention-holds';
  const invalidate = (abort: AbortController) => { abort.abort(); setPage(null); setConfirm(null); setPending(null); setNotice(''); setState('denied'); };
  useEffect(() => {
    const abort = new AbortController(); active.current = abort; cursors.current.clear(); setPage(null); setConfirm(null); setPending(null); setState('loading');
    const denied = () => invalidate(abort);
    const storage = (event: StorageEvent) => { if (event.key === null || event.key === slot.current) denied(); };
    window.addEventListener('drivebridge-context', denied); window.addEventListener('offline', denied); window.addEventListener('storage', storage);
    async function load() {
      try {
        if (!navigator.onLine) { denied(); return; }
        if (!hexID(file.id) || !hexID(file.version_id) || (context.space && context.space !== file.space_id)) throw new Error('invalid_file');
        const session = await api<SessionView>('/me', 'GET', undefined, {}, abort.signal);
        abort.signal.throwIfAborted();
        if (!session?.user?.id || !session.instance_id) throw new Error('invalid_session');
        actor.current = { user: session.user.id, instance: session.instance_id };
        slot.current = 'drivebridge:hold-release:' + JSON.stringify([session.instance_id, session.user.id, context.tenant, context.space, file.space_id, file.id, file.version_id]);
        const stored = readRecovery(slot.current); setPending(stored);
        const value = await fetchPage('', abort.signal);
        abort.signal.throwIfAborted(); setPage(value); setState('ready');
      } catch (error) {
        if (abort.signal.aborted) return;
        const status = error && typeof error === 'object' && 'status' in error ? error.status : undefined;
        if (status === 401 || status === 403) denied();
        else setState(status === 404 || status === 405 ? 'unavailable' : 'error');
      }
    }
    void load();
    return () => { abort.abort(); window.removeEventListener('drivebridge-context', denied); window.removeEventListener('offline', denied); window.removeEventListener('storage', storage); };
  }, [file.id, file.space_id, file.version_id, context.tenant, context.space, path, attempt]);
  async function fetchPage(cursor: string, signal: AbortSignal) {
    const query = new URLSearchParams({ version_id: file.version_id, limit: '50' }); if (cursor) query.set('cursor', cursor);
    const value = await api<HoldPage>(path + '?' + query, 'GET', undefined, {}, signal);
    if (!value || typeof value.management_enabled !== 'boolean' || typeof value.next_cursor !== 'string' || !Array.isArray(value.items) || value.items.length > 50 || typeof value.observed_at !== 'string' || !Number.isFinite(Date.parse(value.observed_at)) || !['unknown', 'exact'].includes(value.total_kind) || (value.total !== null && (!Number.isSafeInteger(value.total) || value.total < 0))) throw new Error('invalid_hold_page');
    for (const row of value.items) if (!row || !hexID(row.hold_id) || row.file_id !== file.id || row.version_id !== file.version_id || !Number.isSafeInteger(row.revision) || row.revision < 1 || row.revision >= Number.MAX_SAFE_INTEGER || !Object.hasOwn(stateLabels, row.state) || typeof row.purpose !== 'string' || !['operation', 'share'].includes(row.source_kind) || !hexID(row.source_ref) || typeof row.expires_at !== 'string' || !Number.isFinite(Date.parse(row.expires_at)) || !Array.isArray(row.allowed_actions) || row.allowed_actions.some(action => action !== 'release')) throw new Error('invalid_hold');
    if (new Set(value.items.map(row => row.hold_id)).size !== value.items.length) throw new Error('duplicate_hold');
    return value;
  }
  async function more() {
    const abort = active.current;
    if (!abort || abort.signal.aborted || !page?.next_cursor || running.current || page.items.length >= 10000 || cursors.current.size >= 199) return;
    running.current = true; setBusy(true); setConfirm(null);
    try {
      const cursor = page.next_cursor;
      if (cursors.current.has(cursor)) throw new Error('cursor_loop');
      const next = await fetchPage(cursor, abort.signal); abort.signal.throwIfAborted();
      const items = [...page.items, ...next.items];
      if (items.length > 10000 || new Set(items.map(row => row.hold_id)).size !== items.length || next.next_cursor === cursor || next.management_enabled !== page.management_enabled) throw new Error('page_changed');
      cursors.current.add(cursor); setPage({ ...next, items });
    } catch (error) {
      if (abort.signal.aborted) return;
      const status = error && typeof error === 'object' && 'status' in error ? error.status : undefined;
      if (status === 401 || status === 403) invalidate(abort);
      else { setPage(null); setConfirm(null); setState('error'); setNotice('保护分页已失效或读取失败，请从第一页重读；旧保护状态不再可操作。'); }
    } finally { running.current = false; if (!abort.signal.aborted) setBusy(false); }
  }
  async function release(lookup: boolean) {
    const abort = active.current;
    if (!abort || abort.signal.aborted || running.current || !navigator.onLine || (lookup ? !pending : restricted || !page?.management_enabled || !confirm || !confirm.allowed_actions.includes('release') || !!pending)) return;
    running.current = true; setBusy(true); setNotice(''); let record = pending, submitted = false;
    try {
      await verifySessionIdentity(actor.current.user, actor.current.instance, abort.signal);
      if (!lookup && confirm) {
        const body = { expected_revision: confirm.revision } satisfies Schemas['WorkspaceHoldRelease'];
        const raw = JSON.stringify({ FileID: file.id, HoldID: confirm.hold_id, Request: body });
        const digest = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(raw))), byte => byte.toString(16).padStart(2, '0')).join('');
        abort.signal.throwIfAborted();
        if (!navigator.locks?.request) throw new Error('lock_unavailable');
        const claim = await navigator.locks.request(slot.current, { signal: abort.signal }, () => {
          abort.signal.throwIfAborted(); const current = readRecovery(slot.current);
          if (current) return { record: current, claimed: false };
          const next: Recovery = { key: 'hold-' + Array.from(crypto.getRandomValues(new Uint8Array(24)), byte => byte.toString(16).padStart(2, '0')).join(''), hold: confirm.hold_id, expected: confirm.revision, digest };
          localStorage.setItem(slot.current, JSON.stringify(next)); return { record: next, claimed: true };
        });
        abort.signal.throwIfAborted(); record = claim.record; setPending(record); setConfirm(null);
        if (!claim.claimed) { setNotice('已有未确认保护请求，请查询原回执。'); return; }
        if (JSON.stringify(readRecovery(slot.current)) !== JSON.stringify(record)) throw new Error('recovery_changed');
        submitted = true;
        await accept(await api<Receipt>('/retention/holds/' + encodeURIComponent(record.hold) + '/release-cas', 'POST', body, { 'Idempotency-Key': record.key }, abort.signal), record, abort.signal);
      } else if (record) {
        if (JSON.stringify(readRecovery(slot.current)) !== JSON.stringify(record)) throw new Error('recovery_changed');
        await accept(await api<Receipt>(path + '/lookup', 'POST', { action: 'release', idempotency_key: record.key } satisfies Schemas['WorkspaceHoldLookup'], {}, abort.signal), record, abort.signal);
      }
    } catch (error) {
      if (abort.signal.aborted) return;
      const status = error && typeof error === 'object' && 'status' in error ? error.status : undefined;
      if (status === 401 || status === 403) invalidate(abort);
      else if (submitted && status === 409 && error instanceof Error && error.message === 'revision_conflict' && record) {
        try { await clearRecovery(slot.current, record, abort.signal); if (!abort.signal.aborted) { setPending(null); setPage(null); setState('error'); setNotice('保护修订冲突，本次没有解除。请重读当前状态后重新选择并确认。'); } }
        catch { if (!abort.signal.aborted) setNotice('恢复身份变化，保留原请求，只查询原回执。'); }
      } else { setConfirm(null); setNotice(record ? '解除结果未知或原回执不匹配。只查询原键；未找到不代表从未提交，不自动重发。' : '恢复存储或锁不可用，没有发送解除请求。'); }
    } finally { running.current = false; if (!abort.signal.aborted) setBusy(false); }
  }
  async function accept(value: Receipt, record: Recovery, signal: AbortSignal) {
    signal.throwIfAborted();
    if (!value || value.file_id !== file.id || value.hold_id !== record.hold || value.action !== 'release' || value.request_digest !== record.digest || value.result_revision !== record.expected + 1 || typeof value.committed_at !== 'string' || !Number.isFinite(Date.parse(value.committed_at))) throw new Error('receipt_mismatch');
    await clearRecovery(slot.current, record, signal); signal.throwIfAborted(); setPending(null); setNotice('已确认原解除回执；解除保护不删除文件，也不启用永久删除。'); setAttempt(value => value + 1);
  }
  return <AppDialog title="保留保护详情" close={close}>
    <p className="break-all text-sm">{file.path} · 固定版本 {file.version_id}</p>
    <p className="text-xs text-muted-foreground">只展示此固定版本的保护；其他历史版本可能仍有保护。保护不是永久存储承诺。</p>
    {state === 'loading' && <p role="status">读取当前授权保护状态…</p>}
    {state === 'denied' && <p role="alert">权限、网络或范围变化，保护详情已清除。请关闭后重新进入。</p>}
    {state === 'unavailable' && <p role="alert">文件或版本不可见，或服务暂不提供保护详情。</p>}
    {state === 'error' && <p role="alert">保护读取失败，不能判断为无保护。</p>}
    {page && <>
      <p className="text-xs text-muted-foreground">观察时间 {page.observed_at} · 已读取 {page.items.length} 条{page.next_cursor ? '，还有更多' : ''}</p>
      {!page.management_enabled && <p role="status">服务未启用保护管理，当前只读；原请求仍可回查。</p>}
      {restricted && <p role="status">当前身份仅可读取此入口，不可解除保护。</p>}
      {!page.items.length && !page.next_cursor && <p role="status">此固定版本没有保留保护记录，不代表所有历史版本无保护。</p>}
      <ul className="m-0 list-none space-y-3 p-0">{page.items.map(row => <li key={row.hold_id} className="space-y-2 rounded border p-3 text-sm">
        <p className="font-medium">{stateLabels[row.state]}</p><p>{purposeLabels[row.purpose] ?? row.purpose} · 修订 {row.revision}</p>
        <dl className="space-y-1 break-all text-xs"><dt>保护 ID</dt><dd>{row.hold_id}</dd><dt>授权来源（{row.source_kind === 'share' ? '分享' : '操作'}）</dt><dd>{row.source_ref}</dd><dt>到期</dt><dd>{row.expires_at.startsWith('0001-') ? '未设置固定到期；仍取决于来源有效性' : row.expires_at}</dd></dl>
        {!restricted && page.management_enabled && row.allowed_actions.includes('release') && !pending && <Button variant="outline" className="min-h-11" disabled={busy} onClick={() => setConfirm(row)}>解除保护</Button>}
      </li>)}</ul>
      {page.next_cursor && <Button variant="outline" className="min-h-11" disabled={busy || page.items.length >= 10000 || cursors.current.size >= 199} onClick={() => void more()}>更多保护记录</Button>}
      {page.next_cursor && (page.items.length >= 10000 || cursors.current.size >= 199) && <p role="alert">已达到读取上限，不能将当前记录当作完整列表。</p>}
      <p className="text-xs text-muted-foreground">新增保护需要已授权且绑定此文件版本的分享或操作来源。当前回收站没有可用的来源选择入口，不能在此创建保护；此处不猜测来源 ID。</p>
    </>}
    {confirm && !pending && <div role="alert" className="space-y-2 rounded border p-3"><p className="break-all">确认解除保护 {confirm.hold_id}（修订 {confirm.revision}）？解除后服务端保留策略可能清理该版本；此操作不删除文件，不能恢复已清理的内容。</p><Button variant="destructive" className="min-h-11" disabled={busy} onClick={() => void release(false)}>确认解除此保护</Button><Button variant="outline" className="min-h-11" disabled={busy} onClick={() => setConfirm(null)}>保留保护不变</Button></div>}
    {state !== 'denied' && <div className="flex flex-wrap gap-2"><Button variant="outline" className="min-h-11" disabled={busy || state === 'loading'} onClick={() => setAttempt(value => value + 1)}>重读保护状态</Button>{pending && <Button className="min-h-11" disabled={busy || state === 'loading'} onClick={() => void release(true)}>查询原保护回执</Button>}</div>}
    {notice && <p role="status">{notice}</p>}
  </AppDialog>;
}
