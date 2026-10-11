import { useEffect, useRef, useState } from 'react';
import type { Space } from '../../api';
import { AppDialog } from '../../components/AppDialog';
import { SelectAdapter } from '../../components/SelectAdapter';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { Checkbox } from '../../components/ui/checkbox';
import { createGrant, lookupGrant, type CreateGrant, type GrantWrite } from './grant-client';
import { claimGrantRecovery, clearGrantRecovery, grantRecoverySlot, readGrantRecovery } from './grant-recovery';

const durations = [[3600, '1 小时'], [86400, '1 天'], [604800, '7 天'], [2592000, '30 天'], [7776000, '90 天'], [31536000, '365 天']] as const;
const actionOptions = [['files.read', '读取文件'], ['labels.suggest', '建议标签（不直接修改）'], ['inbox.submit', '提交收件（等待人工验收）']] as const;
type Phase = 'initializing' | 'edit' | 'busy' | 'unknown' | 'done' | 'denied' | 'unavailable';

export function CreateGrantDialog({ spaces, scope, close }: { spaces: Space[]; scope: string; close: () => void }) {
  const [phase, setPhase] = useState<Phase>('initializing');
  const [name, setName] = useState('');
  const [spaceID, setSpaceID] = useState(spaces.length === 1 ? spaces[0].id : '');
  const [protocol, setProtocol] = useState<CreateGrant['protocol']>('mcp');
  const [prefix, setPrefix] = useState('');
  const [actions, setActions] = useState<CreateGrant['actions']>(['files.read']);
  const [expiry, setExpiry] = useState('never');
  const [message, setMessage] = useState('');
  const [key, setKey] = useState('');
  const [result, setResult] = useState<GrantWrite | null>(null);
  const [retryAllowed, setRetryAllowed] = useState(false);
  const slot = grantRecoverySlot(scope);
  const original = useRef<CreateGrant | null>(null);
  const running = useRef(false);
  const controller = useRef<AbortController | null>(null);
  const active = useRef(true);
  useEffect(() => {
    active.current = true;
    try { const saved = readGrantRecovery(slot); setKey(saved || ''); setPhase(saved ? 'unknown' : 'edit'); }
    catch { setPhase('unavailable'); }
    const invalid = () => { active.current = false; controller.current?.abort(); original.current = null; setResult(null); setPhase('denied'); };
    const storage = (event: StorageEvent) => {
      // Any change from another tab invalidates local intent. Never overwrite
      // another tab's unresolved request or clear a newly claimed identity.
      if (event.key === slot || event.key === null) invalid();
    };
    window.addEventListener('drivebridge-context', invalid);
    window.addEventListener('storage', storage);
    return () => { invalid(); window.removeEventListener('drivebridge-context', invalid); window.removeEventListener('storage', storage); };
  }, [slot]);
  async function perform(mode: 'create' | 'lookup' | 'retry') {
    if (running.current || !active.current) return;
    const selected = spaces.find(space => space.id === spaceID);
    if (mode === 'create' && (!selected || !name.trim() || !actions.length)) { setMessage('请填写名称、选择空间并至少勾选一个动作。'); return; }
    running.current = true; setPhase('busy'); setMessage(''); setRetryAllowed(false);
    const abort = new AbortController(); controller.current = abort;
    let requestKey = key;
    let sent = false;
    try {
      if (mode === 'create') {
        const claim = await claimGrantRecovery(slot, abort.signal);
        if (!active.current || abort.signal.aborted) return;
        requestKey = claim.key; setKey(requestKey);
        if (!claim.claimed) { setPhase('unknown'); return; }
        original.current = {
          name: name.trim(), protocol, space_id: selected!.id, connection_id: selected!.connection_id,
          prefix, actions: [...actions],
          ...(expiry === 'never' ? { expiry_mode: 'never' as const } : { expiry_mode: 'duration' as const, expires_in_seconds: Number(expiry) as CreateGrant['expires_in_seconds'] }),
        };
      }
      if (!requestKey || readGrantRecovery(slot) !== requestKey || (mode !== 'lookup' && !original.current)) throw new Error('recovery_identity_invalid');
      sent = true;
      const response = mode === 'lookup' ? await lookupGrant(requestKey, abort.signal) : await createGrant(original.current!, requestKey, abort.signal);
      if (!active.current || abort.signal.aborted) return;
      if (!response.grant?.grant_id || !response.credential || !['issued', 'not_replayable'].includes(response.credential.state)) throw new Error('invalid_contract');
      if (response.credential.state === 'issued' && (typeof response.credential.secret !== 'string' || !response.credential.secret)) throw new Error('invalid_contract');
      setResult(response); setPhase('done'); original.current = null;
    } catch (error) {
      if (!active.current || abort.signal.aborted) return;
      const status = (error as { status?: number }).status;
      if (status === 401 || status === 403) { setResult(null); setPhase('denied'); original.current = null; }
      else if (!sent) { setPhase('unavailable'); setMessage('无法安全登记原请求，未发送创建操作。'); }
      else if (mode !== 'lookup' && (status === 400 || status === 422)) {
        // These responses reject the request before creation. Network failure,
        // 404, conflict and server errors are not proof of non-commit.
        try {
          await clearGrantRecovery(slot, requestKey);
          if (!active.current) return;
          original.current = null; setKey(''); setPhase('edit');
          setMessage('服务端拒绝了请求格式，请检查名称、目录和授权范围后再提交。');
        } catch { if (active.current) setPhase('unavailable'); }
      }
      else {
        setPhase('unknown');
        if (mode === 'lookup' && status === 404) {
          setMessage('暂未查到原结果；这不证明创建从未发生。请继续查询，不要换键重新创建。');
          setRetryAllowed(!!original.current);
        } else setMessage('未取得可确认的结果。仅查询原请求，不会自动重新签发凭据。');
      }
    } finally { running.current = false; }
  }
  async function acknowledge() {
    if (!active.current || running.current) return;
    running.current = true;
    try {
      await clearGrantRecovery(slot, key);
      if (active.current) { setResult(null); close(); }
    } catch { if (active.current) setMessage('无法清除恢复标识；请保留本次结果，稍后再确认。'); }
    finally { running.current = false; }
  }
  return <AppDialog title="接入 Agent" close={close} className="[&_input:not([type=hidden])]:min-h-11 [&_[role=combobox]]:min-h-11 [&_[data-slot=dialog-close]]:size-11">
    <p className="text-sm text-muted-foreground">授权只保存访问范围，不启动 Agent。凭据填写到你自己的 MCP / HTTP 客户端；不要粘贴到聊天、URL 或共享文档。</p>
    {message && <p role="alert">{message}</p>}
    {phase === 'initializing' && <p role="status">检查原请求恢复标识…</p>}
    {(phase === 'edit' || phase === 'busy') && <form onSubmit={event => { event.preventDefault(); void perform('create'); }}>
      <fieldset disabled={phase === 'busy'} className="space-y-4 border-0 p-0">
        <label className="grid gap-2 text-sm">Agent 名称<Input aria-label="Agent 名称" required maxLength={120} value={name} onChange={event => setName(event.target.value)}/></label>
        <label className="grid gap-2 text-sm">授权空间<SelectAdapter aria-label="授权空间" value={spaceID} required onChange={event => setSpaceID(event.target.value)}><option value="">选择空间</option>{spaces.map(space => <option value={space.id} key={space.id}>{space.name}</option>)}</SelectAdapter></label>
        {!spaces.length && <p role="status">当前没有可用空间，请先创建或取得空间权限。</p>}
        <label className="grid gap-2 text-sm">接入协议<SelectAdapter aria-label="接入协议" value={protocol} onChange={event => setProtocol(event.target.value as CreateGrant['protocol'])}><option value="mcp">MCP</option><option value="http">HTTP API</option></SelectAdapter></label>
        <label className="grid gap-2 text-sm">限定目录<Input aria-label="限定目录" value={prefix} onChange={event => setPrefix(event.target.value)} placeholder="例如 references；留空表示空间内全部目录"/></label>
        <fieldset className="space-y-1 border-0 p-0"><legend className="text-sm">允许的动作</legend>{actionOptions.map(([action, label]) => <label key={action} className="flex min-h-11 items-center gap-2 text-sm"><Checkbox checked={actions.includes(action)} onCheckedChange={checked => setActions(previous => checked ? [...previous.filter(value => value !== action), action] : previous.filter(value => value !== action))}/>{label}</label>)}</fieldset>
        <label className="grid gap-2 text-sm">授权期限<SelectAdapter aria-label="授权期限" value={expiry} onChange={event => setExpiry(event.target.value)}><option value="never">长期（可随时撤销）</option>{durations.map(([seconds, label]) => <option key={seconds} value={seconds}>{label}</option>)}</SelectAdapter></label>
        <p className="text-xs text-muted-foreground">限时从服务端成功创建时起算。验证配置和轮转凭据都不会延长期限。</p>
        <div className="dialog-actions"><Button type="button" variant="outline" className="min-h-11" onClick={close}>取消</Button><Button type="submit" className="min-h-11" disabled={!spaces.length || !actions.length}>{phase === 'busy' ? '正在处理…' : '确认创建授权'}</Button></div>
      </fieldset>
    </form>}
    {phase === 'unknown' && <div className="space-y-3"><p>此范围有尚未确认的原创建请求。关闭或刷新不会替换请求身份。</p><Button className="min-h-11" onClick={() => void perform('lookup')}>查询原创建结果</Button>{retryAllowed && <Button variant="outline" className="min-h-11" onClick={() => void perform('retry')}>按原键重试原请求</Button>}</div>}
    {phase === 'done' && result && <div className="space-y-3"><p role="status">已确认授权：{result.grant.name}</p>{result.credential.state === 'issued' ? <><p>凭据仅显示这一次。关闭后不会再次显示；尚未保存时请勿关闭。</p><Input aria-label="新协议 Agent 凭据" type="password" readOnly autoComplete="off" value={result.credential.secret} onFocus={event => event.target.select()}/><p className="text-xs text-muted-foreground">选中后复制到你的客户端。此页不会保存凭据。</p></> : <p>原授权已存在，但凭据不能再次取回。请使用原授权的凭据轮转流程；不要重复创建身份。</p>}<Button className="min-h-11" onClick={() => void acknowledge()}>{result.credential.state === 'issued' ? '已安全保存凭据' : '已了解原授权结果'}</Button></div>}
    {phase === 'denied' && <p role="alert">账户、空间或恢复标识已变化，敏感结果已清除。请关闭后在正确范围重新打开。</p>}
    {phase === 'unavailable' && <p role="alert">恢复存储或跨标签页锁不可用，不能安全创建授权。请检查浏览器存储与安全上下文后重新打开。</p>}
  </AppDialog>;
}
