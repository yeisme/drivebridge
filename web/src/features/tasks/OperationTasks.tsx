import { useEffect,useRef,useState } from 'react';
import { poll } from '../../app/poll';
import { taskPollDelay } from '../../app/task-poll-delay';
import { verifySessionIdentity } from '../../app/session';
import { BatchTaskItems } from './BatchTaskItems';
import { OperationTaskDetail } from './OperationTaskDetail';
import { api } from '../../api';
import { Button } from '../../components/ui/button';
import { SelectAdapter } from '../../components/SelectAdapter';
import { WorkspacePageHeader } from '../../components/WorkspacePageHeader';
import { AppSheet } from '../../components/AppSheet';
import { RefreshCw } from 'lucide-react';
import type { components } from '../../generated/personal-workspace';

type TaskPage=components['schemas']['TaskPageEnvelope']['data'];
type Task=TaskPage['items'][number];
const states:Record<Task['ui_state'],string>={waiting_confirmation:'待确认',running:'进行中',recoverable:'待恢复',completed:'原操作已完成',failed:'失败',cancelled:'已取消',unknown:'未知'};
function validPage(page:TaskPage,kind:Task['owner_kind'],agent?:string,cursor=''){
 return page&&Array.isArray(page.items)&&page.items.length<=50&&typeof page.next_cursor==='string'&&(!cursor||cursor!==page.next_cursor)&&page.items.every(item=>item.owner_kind===kind&&typeof item.owner_id==='string'&&!!item.owner_id&&typeof item.space_id==='string'&&!!item.space_id&&typeof item.title==='string'&&typeof item.owner_status==='string'&&Object.hasOwn(states,item.ui_state)&&(!agent||item.agent_id===agent)&&item.progress&&(['total','succeeded','failed','pending','bytes_sent','durable_bytes','verified_bytes'] as const).every(key=>item.progress[key]===null||Number.isSafeInteger(item.progress[key])&&item.progress[key]!>=0));
}
export function OperationTasks({scope,agent}:{scope:string;agent?:string}){
 // Only the trusted shell's complete scope enables automatic reads.
 let identity:string[]=[];try{const value:unknown=JSON.parse(scope);if(Array.isArray(value)&&value.length===4&&value.every(part=>typeof part==='string')&&value[0]&&value[1])identity=value;}catch{/* Incomplete callers remain manual-only. */}
 return <ScopedOperationTasks key={JSON.stringify([scope,agent])} agent={agent} instance={identity[0]} user={identity[1]}/>;
}
function ScopedOperationTasks({agent,instance,user}:{agent?:string;instance?:string;user?:string}){
 const [items,setItems]=useState<Task[]>([]),[cursor,setCursor]=useState(''),[next,setNext]=useState(''),[filter,setFilter]=useState(''),[attempt,setAttempt]=useState(0);
 const [kind,setKind]=useState<Task['owner_kind']>('operation');
 const [selected,setSelected]=useState<Task|null>(null);
 const [pollError,setPollError]=useState(false);
 const cadence=useRef(30000);
 const detailTrigger=useRef<HTMLButtonElement|null>(null);
 const listController=useRef<AbortController|null>(null);
 const [status,setStatus]=useState<'loading'|'ready'|'error'|'denied'|'stale'|'unsupported'>('loading');
 useEffect(()=>{
  const controller=new AbortController();listController.current=controller;setStatus('loading');
  const invalidate=()=>{controller.abort();setItems([]);setNext('');setSelected(null);setStatus('denied');};
  window.addEventListener('drivebridge-context',invalidate);
  const query=new URLSearchParams({kind,limit:'50'});if(agent)query.set('agent_id',agent);if(filter)query.set('state',filter);if(cursor)query.set('cursor',cursor);
  api<TaskPage>('/workspace/tasks?'+query,'GET',undefined,{},controller.signal).then(page=>{
   if(controller.signal.aborted)return;
   if(!validPage(page,kind,agent,cursor))throw new Error('invalid_contract');
   setItems(previous=>[...new Map([...(cursor?previous:[]),...page.items].map(item=>[item.owner_kind+':'+item.owner_id,item])).values()]);setNext(page.next_cursor);cadence.current=taskPollDelay(page.items.map(item=>({status:item.ui_state})));setPollError(false);setStatus('ready');
  }).catch((error:{status?:number;message?:string})=>{
   if(controller.signal.aborted)return;setItems([]);setNext('');setSelected(null);
   setStatus(error.status===401||error.status===403?'denied':error.status===410?'stale':error.status===404||error.status===405||error.message==='unsupported'?'unsupported':'error');
  });
  return()=>{controller.abort();window.removeEventListener('drivebridge-context',invalidate);};
 },[kind,agent,filter,cursor,attempt]);
 useEffect(()=>{
  const hide=()=>{if(document.hidden)setSelected(null);};
  document.addEventListener('visibilitychange',hide);return()=>document.removeEventListener('visibilitychange',hide);
 },[]);
 useEffect(()=>{
  if(!instance||!user||cursor||selected||status!=='ready')return;
  const query=new URLSearchParams({kind,limit:'50'});if(agent)query.set('agent_id',agent);if(filter)query.set('state',filter);
  return poll(async signal=>{
   try{
    await verifySessionIdentity(user,instance,signal);
    const page=await api<TaskPage>('/workspace/tasks?'+query,'GET',undefined,{},signal);
    if(!validPage(page,kind,agent))throw new Error('invalid_contract');
    return page;
   }catch(error){if(!signal.aborted){if((error as Error).message==='unsupported'){listController.current?.abort();setItems([]);setNext('');setSelected(null);setPollError(false);setStatus('unsupported');}else setPollError(true);}throw error;}
  },page=>{setItems(page.items);setNext(page.next_cursor);setPollError(false);cadence.current=taskPollDelay(page.items.map(item=>({status:item.ui_state})));},()=>cadence.current,()=>{listController.current?.abort();setItems([]);setNext('');setSelected(null);setPollError(false);setStatus('denied');});
 },[instance,user,kind,agent,filter,cursor,attempt,selected,status]);
 const closeDetail=()=>{setSelected(null);detailTrigger.current?.focus();};
 const denyDetail=()=>{listController.current?.abort();setSelected(null);setItems([]);setNext('');setStatus('denied');};
 const refresh=()=>{setSelected(null);setItems([]);setCursor('');setNext('');setAttempt(value=>value+1);};
 return <section aria-label="持久操作任务" className="min-w-0 space-y-4">
  <WorkspacePageHeader title="任务中心" actions={<Button variant="outline" className="min-h-11 text-[13px] md:min-h-8" disabled={status==='loading'||status==='denied'} onClick={refresh}><RefreshCw className="size-4"/>刷新操作投影</Button>}/>
  <p className="m-0 text-[13px] leading-5 text-muted-foreground">只读原 operation、upload_batch、organization_plan、transfer_plan 或 automation_intent。操作完成不等于收件已验收或已归档。此入口不执行或恢复任务。</p>
  <div className="flex flex-wrap items-center gap-4">
  {status!=='denied'&&<label className="flex max-w-full items-center gap-2 text-[13px]"><span className="shrink-0 text-muted-foreground">任务类别</span><SelectAdapter aria-label="任务类别" className="min-h-11 md:min-h-8" value={kind} onChange={event=>{const value=event.target.value;if((value!=='operation'&&value!=='upload_batch'&&value!=='organization_plan'&&value!=='transfer_plan'&&value!=='automation_intent')||value===kind)return;listController.current?.abort();setSelected(null);setItems([]);setNext('');setCursor('');setFilter('');setKind(value);}}><option value="operation">单操作</option><option value="upload_batch">批量上传</option><option value="organization_plan">整理计划</option><option value="transfer_plan">传输计划</option><option value="automation_intent">自动化意图</option></SelectAdapter></label>}
  {status!=='denied'&&<label className="flex max-w-full items-center gap-2 text-[13px]"><span className="shrink-0 text-muted-foreground">任务显示状态</span><SelectAdapter aria-label="任务显示状态" className="min-h-11 md:min-h-8" value={filter} onChange={event=>{if(event.target.value===filter)return;setSelected(null);setItems([]);setNext('');setCursor('');setFilter(event.target.value);}}><option value="">全部操作状态</option>{Object.entries(states).map(([value,label])=><option key={value} value={value}>{label}</option>)}</SelectAdapter></label>}
  </div>
  {instance&&user&&<p className="text-xs text-muted-foreground">第一页在可见且未打开详情时自动回读；加载更多或查看详情期间暂停。每次自动回读先核对登录身份。</p>}
  {pollError&&status==='ready'&&<p role="alert">自动回读暂未成功，以下为上次观察，正在退避重试；不代表当前操作失败。</p>}
  {status==='loading'&&<p role="status">读取持久操作…</p>}
  {status==='denied'&&<p role="alert">操作读取权限或范围已变化，任务数据已清除，请重新进入。</p>}
  {status==='error'&&<p role="alert">操作投影不可用，可能是来源故障或超出查询预算；不代表没有任务。</p>}
  {status==='stale'&&<p role="alert">操作分页已过期，请刷新从第一页读取。</p>}
  {status==='unsupported'&&<p role="status">当前服务不支持或未启用此任务类别，已停止自动回读；旧传输入口仍保留。</p>}
  {kind==='organization_plan'&&<p className="text-sm text-muted-foreground">整理计划未记录可证明的创建者；确认人不等于创建者。文件数与动作数不同，进度尚未提供，原 completed 不代表已证明全部动作成功。</p>}
  {kind==='transfer_plan'&&<p className="text-sm text-muted-foreground">复制与迁移源清理是独立阶段。copied 不等于迁移完成；原 completed 的逐项结果尚未独立证明。创建者与进度未知，不将执行锚点归为创建者。</p>}
  {kind==='automation_intent'&&<p className="text-sm text-muted-foreground">读取自动化意图不会运行规则或推进 checkpoint。当前规则不代表历史动作；原 completed 的效果尚未独立证明，创建者及进度未知。</p>}
  {kind==='automation_intent'&&agent&&<p role="status">当前不能证明此类意图属于此 Agent；空结果不表示该 Agent 没有自动化意图。</p>}
  {(kind==='organization_plan'||kind==='transfer_plan')&&agent&&<p role="status">当前不能证明此类计划属于此 Agent；空结果不表示该 Agent 没有计划。</p>}
  {status==='ready'&&!items.length&&!((kind==='organization_plan'||kind==='transfer_plan'||kind==='automation_intent')&&agent)&&<p role="status">当前授权范围及筛选下没有操作任务。</p>}
  {selected&&(selected.owner_kind==='operation'||selected.owner_kind==='upload_batch')&&<AppSheet title={selected.owner_kind==='operation'?'操作详情':'批量条目'} open onOpenChange={open=>{if(!open)closeDetail();}} detailWidth={440}>
   {selected.owner_kind==='operation'?<OperationTaskDetail key={selected.owner_id} task={selected} close={closeDetail} onDenied={denyDetail}/>:<BatchTaskItems key={selected.owner_id} batch={selected.owner_id} close={closeDetail} onDenied={denyDetail}/>}
  </AppSheet>}
  <ul aria-label="持久操作列表" className="m-0 list-none divide-y divide-border rounded-lg border border-border bg-card p-0">{items.map(item=><li key={item.owner_kind+':'+item.owner_id} className="min-w-0 space-y-2 p-3">
   <div className="flex min-w-0 items-center gap-3">
    <h3 title={item.title||'未命名操作'} className="m-0 min-w-0 flex-1 line-clamp-2 break-all text-sm font-semibold">{item.title||'未命名操作'}</h3>
    {(item.owner_kind==='operation'||item.owner_kind==='upload_batch')&&<Button variant="ghost" className="min-h-11 px-2 text-[13px] md:min-h-8" disabled={status!=='ready'} aria-label={`${item.owner_kind==='operation'?'查看原操作详情':'查看批量条目'} ${item.owner_id}`} aria-expanded={selected?.owner_id===item.owner_id&&selected.owner_kind===item.owner_kind} onClick={event=>{detailTrigger.current=event.currentTarget;setSelected(item);}}>{item.owner_kind==='operation'?'查看详情':'查看条目'}</Button>}
   </div>
   {item.owner_kind!=='operation'&&item.owner_kind!=='upload_batch'&&<p className="text-xs text-muted-foreground">{item.owner_kind==='automation_intent'?'自动化意图详情与执行入口尚未接入；仅展示原意图状态。':'计划详情与执行入口尚未接入；仅展示原计划状态。'}</p>}
   <p className="break-all text-xs">原操作：{item.owner_kind}:{item.owner_id}</p>
   {item.owner_kind==='transfer_plan'&&item.blocked_reason==='source_cleanup_pending'&&<p className="text-sm">源清理待处理：复制结果不代表源文件已清理，本页不会执行清理。</p>}
   <p className="text-sm">显示状态：{states[item.ui_state]} · 原状态：{item.owner_status}</p>
   <p className="break-all text-xs">{item.actor_kind==='agent'?`认证 Agent：${item.agent_id}`:item.actor_kind==='human'?'来源：人类':'来源：未知（不从调用方声明推断）'}</p>
   <dl className="grid grid-cols-2 gap-x-4 gap-y-1 text-xs sm:grid-cols-4 xl:grid-cols-7">{([['total','总项'],['succeeded','成功项'],['failed','失败项'],['pending','待处理项'],['bytes_sent','已发送字节'],['durable_bytes','持久字节'],['verified_bytes','已校验字节']] as const).map(([key,label])=><div key={key} className="flex min-w-0 flex-wrap items-baseline justify-between gap-x-2"><dt className="text-muted-foreground">{label}</dt><dd className="m-0 break-all tabular-nums">{item.progress[key]===null?'未提供':item.progress[key]}</dd></div>)}</dl>
   <details className="text-xs text-muted-foreground"><summary className="min-h-11 cursor-pointer py-3 focus-visible:outline-2 focus-visible:outline-ring md:min-h-8 md:py-2">观察时间与变更标识</summary><p className="break-all">完整名称：{item.title||'未命名操作'}</p><p className="break-all">观察时间：{item.observed_at} · 变更标识：{item.revision}</p></details>
  </li>)}</ul>
  {next&&status==='ready'&&<Button variant="outline" className="min-h-11" onClick={()=>setCursor(next)}>更多操作任务</Button>}
 </section>;
}
