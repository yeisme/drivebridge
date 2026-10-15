import { useEffect, useRef, useState } from 'react';
import { Button } from '../../components/ui/button';
import { SelectAdapter } from '../../components/SelectAdapter';
import { WorkspacePageHeader } from '../../components/WorkspacePageHeader';
import { RefreshCw, Inbox } from 'lucide-react';
import { listSubmissions, type Submission } from './submission-client';
import { SubmissionPreview } from './SubmissionPreview';
import { SubmissionDecisionDialog } from './SubmissionDecisionDialog';
import { SubmissionArchiveDialog } from './SubmissionArchiveDialog';
import { bytes, getDataContext, type Space } from '../../api';
import { MediaIcon } from '../../Media';
import { submissionRoute, submissionURL } from './submission-route';
export const submissionStates: Record<Submission['state'], string> = { uploading: '上传中', submitted: '待人工验收', accepted: '已接收，尚未归档', rejected: '已拒绝（文件保留）', archiving: '归档处理中', archived: '已归档' };
export function SubmissionInbox({ scope, spaces = [], routeHash }: { scope?: string; spaces?: Space[]; routeHash?: string }) {
  const route=submissionRoute(routeHash||'#/inbox');
  if(!route.valid)return <section aria-label="Agent 提交收件"><p role="alert">收件筛选链接无效，未扩大范围查询。请从收件入口重新进入。</p></section>;
  return <ScopedSubmissionInbox key={JSON.stringify([scope,route.space,route.state])} scope={scope} spaces={spaces} spaceFilter={route.space} initialState={route.state} syncRoute={routeHash!==undefined}/>;
}
function ScopedSubmissionInbox({scope,spaces,spaceFilter,initialState,syncRoute}:{scope?:string;spaces:Space[];spaceFilter:string;initialState:Submission['state']|'';syncRoute:boolean}) {
  const [archiving,setArchiving]=useState<Submission|null>(null);
  const [deciding,setDeciding]=useState<Submission|null>(null);
  const [items,setItems]=useState<Submission[]>([]), [filter,setFilter]=useState<Submission['state']|''>(initialState);
  const [cursor,setCursor]=useState(''), [next,setNext]=useState(''), [attempt,setAttempt]=useState(0);
  const [state,setState]=useState<'loading'|'ready'|'stale'|'error'|'denied'|'unsupported'|'offline'>('loading');
  const [preview,setPreview]=useState<Submission|null>(null);
  const epoch=useRef(0);
  const focusTargets=useRef(new Map<string,HTMLButtonElement>());
  const pendingFocus=useRef<string|null>(null);
  const previewTrigger=useRef<HTMLButtonElement|null>(null);
  const focusRef=(key:string)=>(element:HTMLButtonElement|null)=>{
    if(element)focusTargets.current.set(key,element);else focusTargets.current.delete(key);
  };
  useEffect(()=>{
    if(state==='loading'||!pendingFocus.current)return;
    const target=focusTargets.current.get(pendingFocus.current);pendingFocus.current=null;
    (target&&!target.disabled?target:focusTargets.current.get('refresh'))?.focus();
  },[state,items]);
  useEffect(()=>{
    const abort=new AbortController(),generation=++epoch.current;setState('loading');
    const deny=()=>{pendingFocus.current=null;epoch.current++;abort.abort();setItems([]);setNext('');setPreview(null);setDeciding(null);setArchiving(null);setState('denied');};
    const offline=()=>{pendingFocus.current=null;epoch.current++;abort.abort();setItems([]);setNext('');setPreview(null);setDeciding(null);setArchiving(null);setState('offline');};
    window.addEventListener('drivebridge-context',deny);
    window.addEventListener('offline',offline);
    if(!navigator.onLine){offline();return()=>{abort.abort();window.removeEventListener('drivebridge-context',deny);window.removeEventListener('offline',offline);};}
    listSubmissions(filter,cursor,abort.signal,spaceFilter).then(page=>{
      if(abort.signal.aborted||generation!==epoch.current)return;
      if(!Array.isArray(page.items)||page.items.some(item=>(spaceFilter&&item.space_id!==spaceFilter)||(filter&&item.state!==filter))||page.items.length>50||typeof page.next_cursor!=='string'||(cursor&&page.next_cursor===cursor))throw new Error('invalid_contract');
      setItems(previous=>[...new Map([...(cursor?previous:[]),...page.items].map(item=>[item.submission_id,item])).values()]);setNext(page.next_cursor);setState('ready');
    }).catch((error:{status?:number})=>{
      if(abort.signal.aborted||generation!==epoch.current)return;
      setPreview(null);setDeciding(null);setArchiving(null);setItems([]);setNext('');
      if(error.status===401||error.status===403)deny();
      else setState(error.status===410?'stale':error.status===404||error.status===405?'unsupported':'error');
    });
    return()=>{abort.abort();epoch.current++;window.removeEventListener('drivebridge-context',deny);window.removeEventListener('offline',offline);};
  },[filter,cursor,attempt,spaceFilter]);
  function reset(value=filter){epoch.current++;setState('loading');setItems([]);setNext('');setCursor('');setPreview(null);setDeciding(null);setArchiving(null);setFilter(value);setAttempt(value=>value+1);}
  const resetAndRestore=(key:string)=>{pendingFocus.current=key;reset();};
  return <section aria-label="Agent 提交收件" className="min-w-0 space-y-4">
    <WorkspacePageHeader title="产物与收件" actions={<Button ref={focusRef('refresh')} variant="outline" className="min-h-11 text-[13px] md:min-h-8" onClick={()=>reset()} disabled={state==='loading'}><RefreshCw className="size-4"/>刷新提交收件</Button>}/>
    <p className="m-0 text-[13px] text-muted-foreground">检查原版本，再决定接收、拒绝或归档。上传成功不等于通过验收；来源自述不是认证身份。</p>
    {spaceFilter&&<p className="break-all text-xs text-muted-foreground">收件空间筛选：{spaces.find(space=>space.id===spaceFilter)?.name||spaceFilter}（不扩大当前授权范围）</p>}
    {state!=='denied'&&<label className="flex max-w-sm items-center gap-3 text-[13px] text-muted-foreground"><span className="shrink-0">收件状态</span><SelectAdapter className="min-h-11 md:min-h-8" aria-label="提交收件状态" value={filter} onChange={event=>{const value=event.target.value as Submission['state']|'';if(syncRoute){const context=getDataContext();location.hash=submissionURL(spaceFilter,value,context.tenant,context.space);}else reset(value);}}><option value="">全部提交状态</option>{Object.entries(submissionStates).map(([value,label])=><option key={value} value={value}>{label}</option>)}</SelectAdapter></label>}
    {state==='loading'&&<p role="status">正在读取提交收件…</p>}
    {state==='ready'&&!items.length&&<div role="status" className="flex items-start gap-3 border-y border-border py-8"><Inbox className="mt-0.5 size-5 shrink-0 text-muted-foreground"/><div className="space-y-2"><p className="text-sm font-medium">当前范围没有匹配的提交收件。</p><p className="text-[13px] text-muted-foreground">Agent 交回成果后，可在这里检查固定版本并进行人工验收。</p></div></div>}
    {state==='stale'&&<p role="alert">提交收件分页已变化，请刷新从第一页读取。</p>}
    {state==='offline'&&<p role="alert">当前离线，收件列表与预览已清除；联网后请手动刷新原提交，不会重复验收或归档。</p>}
    {state==='error'&&<p role="alert">提交收件读取失败，不能将失败当作空收件箱。</p>}
    {state==='unsupported'&&<p role="status">当前服务不提供新提交收件接口，请使用兼容入口。</p>}
    {state==='denied'&&<p role="alert">收件权限或范围已变化，列表与预览已清除。</p>}
    <ul aria-label="提交收件列表" className="m-0 list-none divide-y divide-border rounded-lg border border-border bg-card p-0">{items.map(item=><li key={item.submission_id} data-submission-id={item.submission_id} className="min-w-0 space-y-2 p-3">
      <div className="flex items-start gap-3">
        <span className="flex size-9 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground" aria-hidden="true"><MediaIcon file={{ path: item.file_name || '', mime_type: item.mime_type || '' }}/></span>
        <div className="min-w-0 flex-1 space-y-1"><h3 title={item.file_name || '文件名称未提供'} className="m-0 line-clamp-2 break-all text-sm font-semibold">{item.file_name || '文件名称未提供'}</h3>
          <p title={item.agent_name} className="m-0 line-clamp-2 break-words text-[13px] text-muted-foreground">{item.source_kind==='agent'?`来自 ${item.agent_name || 'Agent（名称未提供）'}`:'来源：人工导入'}</p>
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground"><span className="rounded bg-muted px-2 py-1 text-foreground">{submissionStates[item.state]||'未知状态'}</span><span>{typeof item.size_bytes === 'number' ? bytes(item.size_bytes) : '大小未提供'}</span><time dateTime={item.created_at}>{new Date(item.created_at).toLocaleString('zh-CN')}</time></div>
      {item.file_ref&&!item.version_id&&<p className="m-0 text-xs">固定版本未提供，不使用最新版本替代。</p>}
      {item.decision&&<p className="m-0 text-[13px]">人工已{item.decision.decision==='accept'?'接收':'拒绝'}原提交版本{item.decision.reason?' · 评语见详情':''}</p>}
      <details className="text-xs text-muted-foreground"><summary className="min-h-11 cursor-pointer py-3 focus-visible:outline-2 focus-visible:outline-ring md:min-h-8 md:py-2">来源、任务与文件标识</summary><div className="space-y-1 pb-2">
        <p className="m-0 break-all">收件 {item.submission_id}</p>
        <p className="m-0 break-all">完整名称：{item.file_name || '未提供'}</p>
        {item.source_kind==='agent'&&<><p className="m-0 break-all">认证 Agent：{item.agent_id || '未提供'}</p><p className="m-0 break-all">当前名称：{item.agent_name || '未提供'} · 名称可变，身份以标识为准</p><p className="m-0 break-all">授权：{item.grant_id||'未提供'}</p></>}
        {item.source_claim&&<p className="m-0 break-all">来源自述（未经身份验证）：{item.source_claim}</p>}
        {item.version_id&&<p className="m-0 break-all">固定版本：{item.version_id}</p>}
        {item.decision&&<p className="m-0 break-all">人工决定版本：{item.decision.version_id}{item.decision.reason?` · ${item.decision.reason}`:''}</p>}
        <p className="m-0 break-all">上传任务：{item.upload_operation_ref} · 收件版本 {item.revision}</p>
        {item.file_ref&&<p className="m-0 break-all">文件：{item.file_ref}</p>}
        {item.archive_operation_ref&&<p className="m-0 break-all">归档任务：{item.archive_operation_ref}</p>}
        {item.archived_file_ref&&<p className="m-0 break-all">归档结果引用：{item.archived_file_ref}</p>}
      </div></details>
      <div className="flex flex-wrap gap-1 border-t border-border pt-1">
        {item.file_ref&&item.version_id&&<Button variant="ghost" className="min-h-11 px-2 text-[13px] md:min-h-8" aria-label={`查看原提交文件 ${item.submission_id}`} disabled={state!=='ready'} onClick={event=>{previewTrigger.current=event.currentTarget;setPreview(item);}}>查看产物</Button>}
        {scope&&item.file_ref&&item.version_id&&<Button ref={focusRef(`decision:${item.submission_id}`)} variant="ghost" className="min-h-11 px-2 text-[13px] md:min-h-8" aria-label={`人工验收或恢复 ${item.submission_id}`} disabled={state!=='ready'} onClick={()=>setDeciding(item)}>人工验收</Button>}
        {scope&&item.file_ref&&item.version_id&&['accepted','archiving','archived'].includes(item.state)&&<Button ref={focusRef(`archive:${item.submission_id}`)} variant="ghost" className="min-h-11 px-2 text-[13px] md:min-h-8" aria-label={`归档或恢复 ${item.submission_id}`} disabled={state!=='ready'} onClick={()=>setArchiving(item)}>归档/恢复</Button>}
      </div>
    </li>)}</ul>
    {next&&<Button variant="outline" className="min-h-11" disabled={state!=='ready'} onClick={()=>setCursor(next)}>加载更多提交收件</Button>}
    {archiving&&scope&&<SubmissionArchiveDialog key={archiving.submission_id} submission={archiving} scope={scope} spaces={spaces} close={()=>resetAndRestore(`archive:${archiving.submission_id}`)}/>}
    {deciding&&scope&&<SubmissionDecisionDialog key={deciding.submission_id} submission={deciding} scope={scope} close={()=>resetAndRestore(`decision:${deciding.submission_id}`)}/>}
    {preview&&<SubmissionPreview submission={preview} close={()=>{setPreview(null);previewTrigger.current?.focus();}}/>}
  </section>;
}
