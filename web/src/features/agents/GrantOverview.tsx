import { useEffect, useRef, useState } from 'react';
import { Bot, RefreshCw, Plus } from 'lucide-react';
import { WorkspacePageHeader } from '../../components/WorkspacePageHeader';
import { Button } from '../../components/ui/button';
import { listGrants, checkGrant, type Grant, type GrantChecks } from './grant-client';
import type { Space } from '../../api';
import { CreateGrantDialog } from './CreateGrantDialog';
import { RotateGrantDialog } from './RotateGrantDialog';
import { GrantConnectionDialog } from './GrantConnectionDialog';
import { GrantLifecycleDialog } from './GrantLifecycleDialog';
import { AgentOrganizationDialog } from './AgentOrganizationDialog';
import { AgentGroupsDialog } from './AgentGroupsDialog';
import { OperationTasks } from '../tasks/OperationTasks';
import { AgentInvocations } from './AgentInvocations';
import { AppDialog } from '../../components/AppDialog';
import { GrantSections } from './GrantSections';
import { GrantOrganizationNav, type OrganizationView } from './GrantOrganizationNav';
import type { GrantOrganizationFilter } from './grant-client';
import { SelectAdapter } from '../../components/SelectAdapter';
import { useGrantStatePreference, type GrantStateFilter } from './grant-state-preference';

const states: Record<Grant['state'], string> = { active: '有效', disabled: '已禁用', expired: '已到期', revoked: '已撤销' };
const actions: Record<string, string> = { 'files.read': '读取文件', 'labels.suggest': '建议标签', 'inbox.submit': '提交收件' };
const checks: Record<string, string> = { grant_active: '授权状态', identity_binding: '身份与目录绑定', protocol_configured: '协议配置', storage_configuration: '存储配置' };
const observations: Record<string, string> = { unknown: '未确定', observed_ok: '本地检查通过', observed_error: '本地检查异常' };

// Secrets live only in credential dialogs, never in this list's state.
// The containing scope boundary remounts it on actor/tenant/space changes.
export function GrantOverview({ spaces = [], scope }: { spaces?: Space[]; scope?: string }) {
  const [grantState, setGrantState] = useGrantStatePreference(scope);
  const [organizationView, setOrganizationView] = useState<OrganizationView>({ kind: 'all' });
  const [taskAgent,setTaskAgent]=useState<Grant|null>(null);
  const [invocationAgent, setInvocationAgent] = useState<Grant | null>(null);
  const invocationTrigger = useRef<HTMLButtonElement | null>(null);
  const closeInvocations = () => { setInvocationAgent(null); invocationTrigger.current?.focus(); };
  const [creating, setCreating] = useState(false);
  const [groupManager, setGroupManager] = useState(false);
  const [rotating, setRotating] = useState<Grant | null>(null);
  const [configuring, setConfiguring] = useState<Grant | null>(null);
  const [managing, setManaging] = useState<Grant | null>(null);
  const [organizing, setOrganizing] = useState<Grant | null>(null);
  const [grants, setGrants] = useState<Grant[]>([]);
  const [cursor, setCursor] = useState('');
  const [next, setNext] = useState('');
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<'loading' | 'ready' | 'error' | 'forbidden' | 'stale' | 'unsupported' | 'offline'>('loading');
  const [checking, setChecking] = useState('');
  const [reports, setReports] = useState<Record<string, GrantChecks>>({});
  const [notice, setNotice] = useState('');
  const epoch = useRef(0);
  const checkController = useRef<AbortController | null>(null);
  const focusTargets = useRef(new Map<string, HTMLButtonElement>());
  const pendingFocus = useRef<string | null>(null);
  const focusRef = (key: string) => (element: HTMLButtonElement | null) => {
    if (element) focusTargets.current.set(key, element);
    else focusTargets.current.delete(key);
  };
  useEffect(() => {
    if (state === 'loading' || !pendingFocus.current) return;
    const target = focusTargets.current.get(pendingFocus.current);
    pendingFocus.current = null;
    (target && !target.disabled ? target : focusTargets.current.get('refresh'))?.focus();
  }, [state, grants]);
  const revoke = () => { pendingFocus.current = null; epoch.current++; checkController.current?.abort(); setCreating(false); setTaskAgent(null); setInvocationAgent(null); setGroupManager(false); setRotating(null); setConfiguring(null); setManaging(null); setOrganizing(null); setGrants([]); setNext(''); setReports({}); setChecking(''); setState('forbidden'); };
  useEffect(() => {
    const controller = new AbortController();
    const generation = ++epoch.current;
    checkController.current?.abort(); setChecking(''); setReports({}); setNotice(''); setState('loading');
    const invalidate = () => { controller.abort(); revoke(); };
    const offline = () => { controller.abort(); revoke(); setState('offline'); };
    window.addEventListener('drivebridge-context', invalidate);
    window.addEventListener('offline', offline);
    if (!navigator.onLine) { offline(); return () => { controller.abort(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', offline); }; }
    const filter: GrantOrganizationFilter = organizationView.kind === 'all' ? {} : organizationView.kind === 'group' ? { group_id: organizationView.id, order: 'pinned' } : organizationView.kind === 'pinned' ? { pinned: true, order: 'pinned' } : organizationView.kind === 'ungrouped' ? { ungrouped: true, order: 'pinned' } : { order: 'pinned' };
    if (grantState !== 'all') filter.state = grantState;
    listGrants(cursor, controller.signal, filter).then(page => {
      if (controller.signal.aborted || generation !== epoch.current) return;
      if (!Array.isArray(page.items) || typeof page.next_cursor !== 'string' || page.items.length > 50) throw new Error('invalid_contract');
      setGrants(previous => [...new Map([...(cursor ? previous : []), ...page.items].map(grant => [grant.grant_id, grant])).values()]);
      setNext(page.next_cursor); setState('ready');
    }).catch((error: { status?: number }) => {
      if (controller.signal.aborted || generation !== epoch.current) return;
      if (error.status === 401 || error.status === 403) revoke();
      else if (error.status === 409 || error.status === 410) { setGrants([]); setNext(''); setState('stale'); }
      else if (error.status === 404 || error.status === 405) { setGrants([]); setState('unsupported'); }
      else setState('error');
    });
    return () => { controller.abort(); checkController.current?.abort(); epoch.current++; window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', offline); };
  }, [cursor, attempt, organizationView, grantState]);
  const selectOrganization = (view: OrganizationView) => {
    epoch.current++; checkController.current?.abort(); setGrants([]); setReports({}); setCursor(''); setNext(''); setState('loading'); setOrganizationView(view);
  };
  const selectGrantState = (value: GrantStateFilter) => {
    if (value === grantState) return;
    epoch.current++; checkController.current?.abort(); setGrants([]); setReports({}); setCursor(''); setNext(''); setState('loading'); setGrantState(value);
  };
  const refresh = () => { setState('loading'); setCursor(''); setNext(''); setGrants([]); setAttempt(value => value + 1); };
  const refreshAndRestore = (key: string) => { pendingFocus.current = key; refresh(); };
  async function inspect(grant: Grant) {
    if (checkController.current && !checkController.current.signal.aborted) return;
    const controller = new AbortController(); checkController.current = controller;
    const generation = epoch.current;
    setChecking(grant.grant_id); setNotice('');
    setReports(previous => { const next = { ...previous }; delete next[grant.grant_id]; return next; });
    try {
      const report = await checkGrant(grant, controller.signal);
      if (controller.signal.aborted || generation !== epoch.current) return;
      if (!Array.isArray(report.checks) || report.checks.length > 32) throw new Error('invalid_contract');
      setReports(previous => ({ ...previous, [grant.grant_id]: report }));
    } catch (error) {
      if (controller.signal.aborted || generation !== epoch.current) return;
      const status = (error as { status?: number }).status;
      if (status === 401 || status === 403) revoke();
      else if (status === 409 || status === 410) { setReports({}); setNext(''); setState('error'); setNotice('授权版本已变化，请刷新后重新检查；没有执行任何任务。'); }
      else setNotice('检查未完成，请重试。未执行 Agent，也未延长授权期限。');
    } finally {
      controller.abort();
      if (generation === epoch.current) setChecking('');
    }
  }
  return <section aria-label="新协议 Agent 授权" className="min-w-0 space-y-4">
    <WorkspacePageHeader title="Agent 管理" actions={<>
      {scope && <Button ref={focusRef('groups')} variant="ghost" className="min-h-11 text-[13px] md:min-h-8" disabled={state !== 'ready'} onClick={() => setGroupManager(true)}>管理分组</Button>}
      <Button ref={focusRef('refresh')} variant="outline" className="min-h-11 text-[13px] md:min-h-8" disabled={state === 'loading'} onClick={refresh}><RefreshCw className="size-4"/>刷新授权</Button>
      {scope && <Button ref={focusRef('create')} className="min-h-11 text-[13px] md:min-h-8" disabled={state !== 'ready'} onClick={() => setCreating(true)}><Plus className="size-4"/>接入 Agent</Button>}
    </>}/>
    <p className="m-0 text-[13px] text-muted-foreground">每个 Agent 有独立目录、动作与有效期。检查只观察本地配置，不执行任务或续期。</p>
    {scope && state !== 'forbidden' && state !== 'offline' && <GrantOrganizationNav key={attempt} selected={organizationView} onSelect={selectOrganization} onDenied={revoke}/>}
    {state !== 'forbidden' && <label className="flex max-w-sm items-center gap-3 text-[13px] text-muted-foreground"><span className="shrink-0">授权状态筛选</span><SelectAdapter className="min-h-11 md:min-h-8" aria-label="授权状态筛选" value={grantState} onChange={event => selectGrantState(event.target.value as GrantStateFilter)}><option value="all">全部状态</option>{Object.entries(states).map(([value,label]) => <option key={value} value={value}>{label}</option>)}</SelectAdapter></label>}
    {state === 'loading' && <p role="status">正在读取授权…</p>}
    {state === 'forbidden' && <p role="alert">访问权限已失效，授权与检查结果已清除。请重新确认当前账户与空间。</p>}
    {state === 'offline' && <p role="alert">当前离线，授权与检查观察已清除；联网后请手动刷新，不会自动签发、轮转或检查。</p>}
    {state === 'unsupported' && <p role="status">当前服务尚未提供新授权列表，请切换到“旧版机器人授权”。</p>}
    {state === 'error' && <p role="alert">无法读取授权列表。请刷新重试，不将失败视为没有授权。</p>}
    {state === 'stale' && <p role="alert">分页已过期，请刷新授权后从第一页重新读取。</p>}
    {notice && <p role="alert">{notice}</p>}
    {state === 'ready' && !grants.length && <div role="status" className="space-y-2 border-y border-border py-8"><h3 className="text-sm font-semibold">当前视图暂无 Agent</h3><p className="max-w-2xl text-[13px] leading-6 text-muted-foreground">当前范围没有新协议授权匹配所选筛选。可切回全部状态查看；接入 Agent 默认只读、长期有效。</p></div>}
    <GrantSections key={JSON.stringify([scope,organizationView,grantState,attempt,state==='forbidden'])} grants={grants} grouped={organizationView.kind!=='all'}>{grant => <article key={grant.grant_id} role="listitem" className="min-w-0 space-y-2 rounded-lg border border-border bg-card p-3">
      <div className="flex items-center gap-2"><Bot className="size-5 shrink-0 text-primary"/><h3 className="m-0 line-clamp-2 min-w-0 flex-1 break-words text-sm font-semibold" title={grant.name}>{grant.name}</h3><span className="shrink-0 rounded bg-muted px-2 py-1 text-xs">{states[grant.state] ?? '未知状态'}</span></div>
      <p className="m-0 break-all text-[13px]">{grant.protocol.toUpperCase()} · {grant.prefix || '空间内全部目录'}</p>
      <p className="m-0 text-sm text-muted-foreground">{grant.actions.map(action => actions[action] || action).join(' · ')}</p>
      <p className="m-0 text-xs text-muted-foreground">{grant.expiry_mode === 'never' ? '长期有效（可随时撤销）' : grant.expires_at ? `有效期至 ${new Date(grant.expires_at).toLocaleString('zh-CN')}` : '期限不可用'} · 版本 {grant.revision}</p>
      <div className="grid grid-cols-2 gap-1 border-t border-border pt-2 sm:grid-cols-3">
        <Button variant="ghost" className="min-h-11 px-2 text-[13px] md:min-h-8" ref={focusRef(`configure:${grant.grant_id}`)} aria-label={`接入配置 ${grant.name}`} disabled={state !== 'ready'} onClick={() => setConfiguring(grant)}>接入配置</Button>
        <Button variant="ghost" className="min-h-11 px-2 text-[13px] md:min-h-8" aria-label={checking === grant.grant_id ? '正在检查…' : `检查 ${grant.name}`} disabled={!!checking || state !== 'ready'} onClick={() => void inspect(grant)}>{checking === grant.grant_id ? '正在检查…' : '检查配置'}</Button>
        {scope && <Button variant="ghost" className="min-h-11 px-2 text-[13px] md:min-h-8" ref={focusRef(`rotate:${grant.grant_id}`)} aria-label={`凭据管理 ${grant.name}`} disabled={state !== 'ready' || !!checking} onClick={() => setRotating(grant)}>凭据管理</Button>}
        {scope && <Button variant="ghost" className="min-h-11 px-2 text-[13px] md:min-h-8" ref={focusRef(`manage:${grant.grant_id}`)} aria-label={`管理授权 ${grant.name}`} disabled={state !== 'ready' || !!checking} onClick={() => setManaging(grant)}>管理授权</Button>}
        {scope && <Button variant="ghost" className="min-h-11 px-2 text-[13px] md:min-h-8" ref={focusRef(`organize:${grant.grant_id}`)} aria-label={`分组与置顶 ${grant.name}`} disabled={state !== 'ready' || !!checking} onClick={() => setOrganizing(grant)}>分组与置顶</Button>}
        {scope && <Button variant="ghost" className="min-h-11 px-2 text-[13px] md:min-h-8" aria-label={`查看操作任务 ${grant.name}`} disabled={state !== 'ready'} onClick={()=>setTaskAgent(grant)}>操作任务</Button>}
        {scope && <Button variant="ghost" className="min-h-11 px-2 text-[13px] md:min-h-8" aria-label={`查看调用记录 ${grant.name}`} disabled={state !== 'ready'} onClick={event => { invocationTrigger.current = event.currentTarget; setInvocationAgent(grant); }}>调用记录</Button>}
      </div>
      {reports[grant.grant_id] && <div className="rounded-md bg-muted/40 p-3 text-sm" role="status"><ul className="m-0 space-y-1 pl-4">{reports[grant.grant_id].checks.map((check, index) => <li key={index}>{checks[check.name] || '其他本地检查'}：{observations[check.status] || '未确定'}</li>)}</ul><p className="mb-0 text-xs text-muted-foreground">远端连接：{reports[grant.grant_id].connection_state === 'unknown' ? '未验证' : '观察状态，不代表端到端可用'}</p></div>}
    </article>}</GrantSections>
    {configuring && <GrantConnectionDialog key={configuring.grant_id} grant={configuring} close={() => { setConfiguring(null); focusTargets.current.get(`configure:${configuring.grant_id}`)?.focus(); }}/>}
    {invocationAgent && scope && <AppDialog title="Agent 调用记录" close={closeInvocations}><AgentInvocations scope={scope} agent={invocationAgent.agent_id} onDenied={revoke}/></AppDialog>}
    {taskAgent && scope && <AppDialog title="Agent 操作任务" close={()=>setTaskAgent(null)}><OperationTasks scope={scope} agent={taskAgent.agent_id}/></AppDialog>}
    {creating && scope && <CreateGrantDialog spaces={spaces} scope={scope} close={() => { setCreating(false); refreshAndRestore('create'); }}/>} 
    {rotating && scope && <RotateGrantDialog key={rotating.grant_id} grant={rotating} scope={scope} close={() => { setRotating(null); refreshAndRestore(`rotate:${rotating.grant_id}`); }}/>} 
    {managing && scope && <GrantLifecycleDialog key={managing.grant_id} grant={managing} scope={scope} close={() => { setManaging(null); refreshAndRestore(`manage:${managing.grant_id}`); }}/>} 
    {organizing && scope && <AgentOrganizationDialog key={organizing.agent_id} grant={organizing} scope={scope} close={() => { setOrganizing(null); refreshAndRestore(`organize:${organizing.grant_id}`); }}/>} 
    {groupManager && scope && <AgentGroupsDialog scope={scope} close={() => { setGroupManager(false); refreshAndRestore('groups'); }}/>} 
    {next && <Button variant="outline" className="min-h-11" disabled={state !== 'ready'} onClick={() => setCursor(next)}>加载更多授权</Button>}
  </section>;
}
