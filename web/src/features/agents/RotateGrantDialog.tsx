import { useEffect, useRef, useState } from 'react';
import { AppDialog } from '../../components/AppDialog';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { lookupRotation, rotateGrant, type Grant, type RotationReceipt } from './grant-client';
import { claimGrantRecovery, clearGrantRecovery, readGrantRecovery } from './grant-recovery';

export const rotationSlot = (scope: string, id: string) => 'drivebridge:grant-rotate:' + JSON.stringify([scope, id]);
type Phase = 'initializing' | 'confirm' | 'busy' | 'unknown' | 'done' | 'denied' | 'unavailable';
// A historical receipt is not the latest credential generation. In particular,
// do not use the GrantWrite replay projection to infer the original result.
export function RotateGrantDialog({ grant, scope, close }: { grant: Grant; scope: string; close: () => void }) {
  const slot = rotationSlot(scope, grant.grant_id);
  const [phase, setPhase] = useState<Phase>('initializing');
  const [key, setKey] = useState('');
  const [secret, setSecret] = useState('');
  const [receipt, setReceipt] = useState<RotationReceipt | null>(null);
  const [message, setMessage] = useState('');
  const [canRefresh, setCanRefresh] = useState(false);
  const active = useRef(true), running = useRef(false), rejectedCAS = useRef(false);
  const controller = useRef<AbortController | null>(null);
  const eligible = grant.state === 'active' || grant.state === 'disabled';
  useEffect(() => {
    active.current = true;
    try { const pending = readGrantRecovery(slot); setKey(pending || ''); setPhase(pending ? 'unknown' : 'confirm'); }
    catch { setPhase('unavailable'); }
    const invalidate = () => { active.current = false; controller.current?.abort(); setSecret(''); setReceipt(null); setPhase('denied'); };
    const storage = (event: StorageEvent) => { if (event.key === slot || event.key === null) invalidate(); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('storage', storage);
    return () => { invalidate(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('storage', storage); };
  }, [slot]);
  async function perform(lookup: boolean) {
    if (!active.current || running.current || (!lookup && !eligible)) return;
    running.current = true; setPhase('busy'); setMessage(''); setCanRefresh(false);
    const abort = new AbortController(); controller.current = abort;
    let requestKey = key, sent = false;
    try {
      if (!lookup) {
        const claim = await claimGrantRecovery(slot, abort.signal);
        if (!active.current || abort.signal.aborted) return;
        requestKey = claim.key; setKey(requestKey);
        if (!claim.claimed) { setPhase('unknown'); return; }
      }
      if (!requestKey || readGrantRecovery(slot) !== requestKey) throw new Error('recovery_identity_invalid');
      let original: RotationReceipt;
      sent = true;
      if (lookup) original = await lookupRotation(grant.grant_id, requestKey, abort.signal);
      else {
        const result = await rotateGrant(grant.grant_id, grant.revision, requestKey, abort.signal);
        if (!active.current || abort.signal.aborted) return;
        if (result.grant?.grant_id !== grant.grant_id) throw new Error('invalid_contract');
        if (result.credential?.state === 'issued') {
          if (!result.credential.secret || typeof result.credential.secret !== 'string' || !Number.isSafeInteger(result.grant.credential_generation) || result.grant.credential_generation < 1) throw new Error('invalid_contract');
          setSecret(result.credential.secret);
          original = { grant_id: grant.grant_id, credential_generation: result.grant.credential_generation, credential: { state: 'not_replayable' } };
        } else if (result.credential?.state === 'not_replayable') {
          original = await lookupRotation(grant.grant_id, requestKey, abort.signal);
        } else throw new Error('invalid_contract');
      }
      if (!active.current || abort.signal.aborted) return;
      if (original.grant_id !== grant.grant_id || !Number.isSafeInteger(original.credential_generation) || original.credential_generation < 1 || original.credential?.state !== 'not_replayable') throw new Error('invalid_contract');
      setReceipt(original); setPhase('done');
    } catch (error) {
      if (!active.current || abort.signal.aborted) return;
      setSecret('');
      const failure = error as Error & { status?: number };
      if (failure.status === 401 || failure.status === 403) { setReceipt(null); setPhase('denied'); }
      else if (!sent) setPhase('unavailable');
      else {
        if (!lookup && failure.status === 409 && failure.message === 'revision_conflict') rejectedCAS.current = true;
        setPhase('unknown');
        if (lookup && failure.status === 404) {
          setCanRefresh(rejectedCAS.current);
          setMessage(rejectedCAS.current ? '原轮转因版本冲突被明确拒绝，且未查到原回执。可以结束此次尝试，刷新后重新确认。' : '暂未查到原轮转回执；这不证明轮转没有发生。保留原请求继续查询，不自动再轮转。');
        } else setMessage('结果尚未确认。请查询原轮转回执；不会自动重试或延长期限。');
      }
    } finally { running.current = false; }
  }
  async function acknowledge() {
    if (!active.current || running.current) return;
    running.current = true;
    try { await clearGrantRecovery(slot, key); if (active.current) { setSecret(''); close(); } }
    catch { if (active.current) setMessage('恢复标识未能清除，请稍后重试；不要重复轮转。'); }
    finally { running.current = false; }
  }
  return <AppDialog title="轮转 Agent 凭据" close={close} className="[&_input]:min-h-11 [&_[data-slot=dialog-close]]:size-11">
    <p className="break-words">{grant.name} · 当前读取的授权版本 {grant.revision}</p>
    <p className="text-sm text-muted-foreground">确认后旧凭据立即失效，新凭据仅显示一次。范围、期限和禁用状态不变；这不是续期或重新接入。</p>
    {message && <p role="alert">{message}</p>}
    {phase === 'initializing' && <p role="status">读取原请求标识…</p>}
    {phase === 'confirm' && (eligible ? <div className="dialog-actions"><Button variant="outline" className="min-h-11" onClick={close}>取消</Button><Button variant="destructive" className="min-h-11" onClick={() => void perform(false)}>确认轮转并使旧凭据失效</Button></div> : <p role="status">当前授权已过期或撤销，不能轮转；需要重新接入新的授权。</p>)}
    {phase === 'busy' && <p role="status">正在处理原轮转请求…</p>}
    {phase === 'unknown' && <div className="space-y-3"><p>该授权有尚未确认的轮转请求，先查询原代次。</p><Button className="min-h-11" onClick={() => void perform(true)}>查询原轮转结果</Button>{canRefresh && <Button variant="outline" className="min-h-11" onClick={() => void acknowledge()}>结束已拒绝尝试并刷新</Button>}</div>}
    {phase === 'done' && receipt && <div className="space-y-3"><p role="status">原请求已确认：凭据代次 {receipt.credential_generation}</p>{secret ? <><p>请保存到你自己的客户端。关闭后无法再次取回。</p><Input aria-label="轮转后的 Agent 凭据" type="password" autoComplete="off" readOnly value={secret} onFocus={event => event.target.select()}/></> : <p>原凭据不能再次显示。此代次是历史回执，不代表当前仍有效。若凭据已丢失，请先确认本结果，再刷新授权并单独确认下一次轮转。</p>}<Button className="min-h-11" onClick={() => void acknowledge()}>{secret ? '已安全保存新凭据' : '已了解原轮转结果'}</Button></div>}
    {phase === 'denied' && <p role="alert">权限、范围或另一标签页的恢复标识已变化，敏感结果已清除。请关闭后重新确认。</p>}
    {phase === 'unavailable' && <p role="alert">恢复存储或跨标签页锁不可用，未安全发起轮转。请关闭并检查浏览器存储。</p>}
  </AppDialog>;
}
