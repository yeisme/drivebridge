import { useEffect, useRef, useState } from 'react';
import { AppDialog } from '../../components/AppDialog';
import { SelectAdapter } from '../../components/SelectAdapter';
import { Button } from '../../components/ui/button';
import { changeGrant, getGrant, lookupGrantChange, type Grant, type GrantChangeAction, type GrantChangeReceipt, type GrantPatch } from './grant-client';
import { changeSlot, claimChange, clearChange, readChange, sameChange, type PendingChange } from './grant-change-recovery';

const labels: Record<GrantChangeAction, string> = { disable: '禁用授权', enable: '恢复授权', revoke: '永久撤销', expiry: '修改期限' };
const durations = [[3600, '1 小时'], [86400, '1 天'], [604800, '7 天'], [2592000, '30 天'], [7776000, '90 天'], [31536000, '365 天']] as const;
type Phase = 'initializing' | 'edit' | 'busy' | 'unknown' | 'conflict' | 'done' | 'denied' | 'unavailable';
export function GrantLifecycleDialog({ grant, scope, close }: { grant: Grant; scope: string; close: () => void }) {
  const slot = changeSlot(scope, grant.grant_id);
  const [current, setCurrent] = useState(grant);
  const [action, setAction] = useState<GrantChangeAction>(grant.state === 'disabled' ? 'enable' : 'disable');
  const [expiry, setExpiry] = useState('never');
  const [phase, setPhase] = useState<Phase>('initializing');
  const [pending, setPending] = useState<PendingChange | null>(null);
  const [receipt, setReceipt] = useState<GrantChangeReceipt | null>(null);
  const [message, setMessage] = useState('');
  const active = useRef(true), running = useRef(false), rejectedCAS = useRef(false);
  const controller = useRef<AbortController | null>(null);
  const eligible = current.state === 'active' || current.state === 'disabled';
  useEffect(() => {
    active.current = true;
    try { const saved = readChange(slot); setPending(saved); if (saved) setAction(saved.action); setPhase(saved ? 'unknown' : 'edit'); }
    catch { setPhase('unavailable'); }
    const invalidate = () => { active.current = false; controller.current?.abort(); setReceipt(null); setPhase('denied'); };
    const storage = (event: StorageEvent) => { if (event.key === slot || event.key === null) invalidate(); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('storage', storage);
    return () => { invalidate(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('storage', storage); };
  }, [slot]);
  async function perform(mode: 'write' | 'lookup' | 'refresh') {
    if (!active.current || running.current || (mode === 'write' && !eligible)) return;
    running.current = true; setPhase('busy'); setMessage('');
    const abort = new AbortController(); controller.current = abort;
    let original = pending, sentWrite = false;
    try {
      if (mode === 'refresh') {
        // Available only after an explicit CAS rejection plus a negative lookup.
        if (!original || !rejectedCAS.current) throw new Error('invalid_recovery');
        const fresh = await getGrant(grant.grant_id, abort.signal);
        if (!active.current || abort.signal.aborted) return;
        if (fresh.grant_id !== grant.grant_id || !Number.isSafeInteger(fresh.revision)) throw new Error('invalid_contract');
        await clearChange(slot, original);
        if (!active.current) return;
        setCurrent(fresh); setPending(null); rejectedCAS.current = false; setPhase('edit');
        setMessage('已重读授权，原操作意图保留。请检查最新状态与版本，重新确认后才会提交。');
        return;
      }
      if (mode === 'write') {
        const claim = await claimChange(slot, action, abort.signal);
        if (!active.current || abort.signal.aborted) return;
        original = claim.pending; setPending(original);
        if (!claim.claimed) { setAction(original.action); setPhase('unknown'); return; }
      }
      if (!original || !sameChange(readChange(slot), original)) throw new Error('recovery_identity_changed');
      if (mode === 'write') {
        const input: GrantPatch = { expected_revision: current.revision };
        if (action === 'disable' || action === 'enable') input.enabled = action === 'enable';
        if (action === 'expiry') {
          input.expiry_mode = expiry === 'never' ? 'never' : 'duration';
          if (expiry !== 'never') input.expires_in_seconds = Number(expiry) as GrantPatch['expires_in_seconds'];
        }
        sentWrite = true;
        await changeGrant(grant.grant_id, action, input, original.key, abort.signal);
        if (!active.current || abort.signal.aborted) return;
      }
      // Even a successful write response is only a latest-state projection.
      const result = await lookupGrantChange(grant.grant_id, original.action, original.key, abort.signal);
      if (!active.current || abort.signal.aborted) return;
      if (result.grant_id !== grant.grant_id || result.action !== original.action || !Number.isSafeInteger(result.result_revision) || result.result_revision < 1 || !Number.isFinite(Date.parse(result.committed_at))) throw new Error('invalid_contract');
      setReceipt(result); setPhase('done');
    } catch (error) {
      if (!active.current || abort.signal.aborted) return;
      const failure = error as Error & { status?: number };
      if (failure.status === 401 || failure.status === 403) { setReceipt(null); setPhase('denied'); }
      else if (!original) setPhase('unavailable');
      else {
        if (sentWrite && failure.status === 409 && failure.message === 'revision_conflict') rejectedCAS.current = true;
        if (mode === 'lookup' && failure.status === 404 && rejectedCAS.current) {
          setPhase('conflict'); setMessage('原变更被版本冲突拒绝，且未查到回执。可重读最新授权，保留意图后再次确认。');
        } else {
          setPhase('unknown');
          setMessage(failure.status === 404 ? '暂未查到原回执，不代表变更从未发生。请继续查询，不要换键重发。' : '尚未取得可确认的原回执。只查询原动作，不自动重复变更。');
        }
      }
    } finally { running.current = false; }
  }
  async function acknowledge() {
    if (!pending || running.current || !active.current) return;
    running.current = true;
    try { await clearChange(slot, pending); if (active.current) close(); }
    catch { if (active.current) setMessage('恢复标识未清除，请保留本结果并稍后重试。'); }
    finally { running.current = false; }
  }
  return <AppDialog title="管理 Agent 授权" close={close} className="[&_[role=combobox]]:min-h-11 [&_[data-slot=dialog-close]]:size-11">
    <p>{current.name} · 当前读取版本 {current.revision}</p>
    <p className="text-sm text-muted-foreground">禁用可恢复；永久撤销不可恢复且不删除文件。期限修改从服务端成功提交起算，既有凭据不会轮转。</p>
    {message && <p role="alert">{message}</p>}
    {phase === 'initializing' && <p role="status">读取原变更标识…</p>}
    {phase === 'edit' && (eligible ? <div className="space-y-4">
      <label className="grid gap-2">变更动作<SelectAdapter aria-label="变更动作" value={action} onChange={event => setAction(event.target.value as GrantChangeAction)}>{Object.entries(labels).map(([value, label]) => <option key={value} value={value} disabled={(value === 'disable' && current.state === 'disabled') || (value === 'enable' && current.state === 'active')}>{label}</option>)}</SelectAdapter></label>
      {action === 'expiry' && <label className="grid gap-2">新的授权期限<SelectAdapter aria-label="新的授权期限" value={expiry} onChange={event => setExpiry(event.target.value)}><option value="never">长期（可随时撤销）</option>{durations.map(([seconds, label]) => <option key={seconds} value={seconds}>{label}</option>)}</SelectAdapter></label>}
      <p>本次确认：{labels[action]}{action === 'expiry' ? ` · ${expiry === 'never' ? '长期' : durations.find(([seconds]) => String(seconds) === expiry)?.[1]}` : ''}，基于版本 {current.revision}。</p>
      <div className="dialog-actions"><Button variant="outline" className="min-h-11" onClick={close}>取消</Button><Button variant={action === 'revoke' ? 'destructive' : 'default'} className="min-h-11" disabled={(action === 'disable' && current.state === 'disabled') || (action === 'enable' && current.state === 'active')} onClick={() => void perform('write')}>确认{labels[action]}</Button></div>
    </div> : <p role="status">授权已过期或撤销，不能恢复或续期；需要重新接入新的授权。</p>)}
    {phase === 'busy' && <p role="status">正在处理原授权变更…</p>}
    {phase === 'unknown' && <div className="space-y-3"><p>待确认原动作：{pending && labels[pending.action]}。刷新不会重新提交。</p><Button className="min-h-11" onClick={() => void perform('lookup')}>查询原变更结果</Button></div>}
    {phase === 'conflict' && <Button className="min-h-11" onClick={() => void perform('refresh')}>保留意图并重读授权</Button>}
    {phase === 'done' && receipt && <div className="space-y-3"><p role="status">原变更已提交：{labels[receipt.action]} · 结果版本 {receipt.result_revision}</p><p>这是原请求的提交回执，不代表当前状态；确认后刷新查看最新授权。</p><Button className="min-h-11" onClick={() => void acknowledge()}>确认回执并刷新</Button></div>}
    {phase === 'denied' && <p role="alert">权限或范围已变化，已停止操作并清除结果。请关闭后重新确认身份。</p>}
    {phase === 'unavailable' && <p role="alert">恢复存储或跨标签页锁不可用，不能安全发起授权变更。</p>}
  </AppDialog>;
}
