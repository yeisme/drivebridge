import { useEffect,useRef,useState } from 'react';
import { api,bytes,getDataContext,type Space } from '../../api';
import { Button } from '../../components/ui/button';
import { SelectAdapter } from '../../components/SelectAdapter';
import { WorkspacePageHeader } from '../../components/WorkspacePageHeader';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../../components/ui/table';
import { FileText, Folder, ArrowUpRight, RefreshCw, Inbox } from 'lucide-react';
import { HomeFileText } from './HomeFileText';
import { HomeCollection } from './HomeCollection';
import { submissionURL } from '../inbox/submission-route';
import type { components } from '../../generated/personal-workspace';
type Home=components['schemas']['WorkspaceHomeEnvelope']['data'];
type FileRef=Home['recent_files'][number];
const sections=['recent_files','collections','pending_counts','usage'] as const;
const validCount=(value:unknown)=>value===null||typeof value==='number'&&Number.isSafeInteger(value)&&value>=0;
function validHome(value:Home,space:string){
 return value&&value.space_id===space&&value.sections&&sections.every(key=>value.sections[key]&&['ready','partial','unavailable'].includes(value.sections[key].state))&&Array.isArray(value.recent_files)&&value.recent_files.length<=6&&value.recent_files.every(file=>file.space_id===space&&!!file.file_id&&!!file.version_id&&typeof file.path==='string'&&typeof file.ref==='string'&&file.ref.endsWith('/file/'+file.file_id+'/version/'+file.version_id)&&file.size_bytes!==null&&validCount(file.size_bytes))&&Array.isArray(value.collections)&&value.collections.length<=20&&value.collections.every(item=>!!item.collection_id&&typeof item.name==='string'&&['manual','rule'].includes(item.kind))&&value.pending_counts&&validCount(value.pending_counts.submitted)&&validCount(value.pending_counts.accepted)&&value.usage&&Object.values(value.usage).length===3&&Object.values(value.usage).every(validCount);
}
export function WorkspaceHome({scope,spaces}:{scope:string;spaces:Space[]}){
 return <ScopedHome key={scope} spaces={spaces.filter(space=>!space.team_id)}/>;
}
function ScopedHome({spaces}:{spaces:Space[]}){
 const [chosen,setChosen]=useState('');const space=spaces.some(item=>item.id===chosen)?chosen:spaces[0]?.id||'';
 return <section aria-label="个人工作台摘要" className="min-w-0 space-y-5">
  <WorkspacePageHeader title="我的工作台" actions={spaces.length ? <label className="flex min-w-0 items-center gap-2 text-[13px] text-muted-foreground"><span className="shrink-0">空间</span><SelectAdapter aria-label="摘要空间" className="min-h-11 max-w-56 md:min-h-8" value={space} onChange={event=>setChosen(event.target.value)}>{spaces.map(item=><option key={item.id} value={item.id}>{item.name}</option>)}</SelectAdapter></label> : undefined}/>
  {spaces.length?<HomeData key={space} space={space}/>:<p role="status">当前范围没有可用的个人空间；团队聚合尚未提供，可从“容量与活动观察”查看原总览。</p>}
 </section>;
}
function HomeData({space}:{space:string}){
 const [home,setHome]=useState<Home|null>(null),[state,setState]=useState('loading'),[attempt,setAttempt]=useState(0);
 const [selected,setSelected]=useState<FileRef|null>(null);const trigger=useRef<HTMLButtonElement|null>(null);
 const [collection,setCollection]=useState<Home['collections'][number]|null>(null);const collectionTrigger=useRef<HTMLButtonElement|null>(null);
 const controllerRef=useRef<AbortController|null>(null);
 const deny=()=>{controllerRef.current?.abort();setHome(null);setSelected(null);setCollection(null);setState('denied');};
 useEffect(()=>{
  const controller=new AbortController();controllerRef.current=controller;setHome(null);setSelected(null);setCollection(null);
  const invalidate=()=>{controller.abort();setHome(null);setSelected(null);setCollection(null);setState('denied');};
  const offline=()=>{controller.abort();setHome(null);setSelected(null);setCollection(null);setState('offline');};
  window.addEventListener('drivebridge-context',invalidate);window.addEventListener('offline',offline);
  if(!navigator.onLine)offline();else{
   setState('loading');api<Home>('/workspace/home?'+new URLSearchParams({space_id:space,recent_limit:'6'}),'GET',undefined,{},controller.signal).then(value=>{
    if(controller.signal.aborted)return;if(!validHome(value,space))throw new Error('invalid_contract');setHome(value);setState('ready');
   }).catch((error:{status?:number})=>{if(!controller.signal.aborted)setState(error.status===401||error.status===403?'denied':error.status===404?'unsupported':'error');});
  }
  return()=>{controller.abort();window.removeEventListener('drivebridge-context',invalidate);window.removeEventListener('offline',offline);};
 },[space,attempt]);
 const sectionNote=(key:typeof sections[number])=>home?.sections[key].state==='unavailable'?<p role="status">此段暂不可用，不代表数据为空。</p>:home?.sections[key].state==='partial'?<p className="text-xs text-muted-foreground">部分覆盖 · {key==='pending_counts'?'仅收件状态，不含其他任务、建议与审批':key==='collections'?'最多展示最近 20 个集合':'部分固定版本当前不可用'}</p>:null;
 return <>
  <div className="flex flex-wrap items-center justify-between gap-2"><p className="text-[13px] text-muted-foreground">继续整理文件，或处理 Agent 交回的成果。</p><Button variant="ghost" className="min-h-11 text-[13px] md:min-h-8" disabled={state==='loading'||state==='denied'} onClick={()=>setAttempt(value=>value+1)}><RefreshCw className="size-4"/>刷新工作台摘要</Button></div>
  {state==='loading'&&<p role="status">读取工作台摘要…</p>}
  {state==='offline'&&<p role="alert">当前离线，摘要已清除。联网后请手动刷新，不会自动重发。</p>}
  {state==='denied'&&<p role="alert">身份或范围已变化，摘要已清除，请重新进入。</p>}
  {(state==='error'||state==='unsupported')&&<p role="alert">工作台摘要不可用或服务尚未提供此入口，不代表没有文件。</p>}
  {home&&<div className="space-y-6">
   <section aria-label="集合摘要" className="space-y-3"><h3 className="m-0 text-sm font-semibold">继续项目</h3>{sectionNote('collections')}{home.sections.collections.state!=='unavailable'&&<>{!home.collections.length&&<p className="border-y border-border py-5 text-[13px] text-muted-foreground">当前没有集合。</p>}<ul className="m-0 list-none divide-y divide-border p-0">{home.collections.map(item=><li className="flex min-w-0 flex-wrap items-center gap-3 bg-card px-3 py-3" key={item.collection_id}><Folder className="size-5 shrink-0 text-muted-foreground"/><div className="min-w-0 flex-1"><Button variant="link" title={item.name} className="h-auto min-h-11 max-w-full justify-start whitespace-normal break-words p-0 text-left text-sm font-semibold text-foreground md:min-h-8" onClick={event=>{collectionTrigger.current=event.currentTarget;setSelected(null);setCollection(item);}}><span className="line-clamp-2">{item.name}</span></Button><p className="text-xs text-muted-foreground">{item.kind==='manual'?'手动集合':'规则集合'} · 引用原文件，不移动原目录</p></div><ArrowUpRight className="size-4 shrink-0 text-muted-foreground" aria-hidden="true"/></li>)}</ul></>}</section>
   <section aria-label="最近文件摘要" className="min-w-0 space-y-3"><h3 className="m-0 text-sm font-semibold">最近文件</h3>{sectionNote('recent_files')}{home.sections.recent_files.state!=='unavailable'&&<>{!home.recent_files.length?<p className="border-y border-border py-6 text-[13px] text-muted-foreground">当前观察没有可展示的最近文件。</p>:<div className="overflow-hidden rounded-lg border border-border bg-card"><Table aria-label="最近固定版本文件"><TableHeader><TableRow className="h-8"><TableHead className="h-8 text-xs text-muted-foreground">名称</TableHead><TableHead className="hidden h-8 text-xs text-muted-foreground sm:table-cell">大小</TableHead><TableHead className="hidden h-8 text-xs text-muted-foreground md:table-cell">修改时间</TableHead></TableRow></TableHeader><TableBody>{home.recent_files.map(file=><TableRow key={file.file_id+':'+file.version_id} className="h-11"><TableCell className="max-w-0 py-0"><Button variant="ghost" aria-label={'查看固定引用：'+file.path} className="h-auto min-h-11 w-full justify-start gap-2 px-0 text-left text-[13px] md:min-h-9" onClick={event=>{trigger.current=event.currentTarget;setSelected(file);}}><FileText className="size-4 shrink-0 text-muted-foreground"/><span className="truncate" title={file.path}>{file.path}</span></Button></TableCell><TableCell className="hidden w-28 text-xs text-muted-foreground sm:table-cell">{bytes(file.size_bytes)}</TableCell><TableCell className="hidden w-36 text-xs text-muted-foreground md:table-cell">{Number.isFinite(Date.parse(file.updated_at))?new Date(file.updated_at).toLocaleDateString('zh-CN'):'未提供'}</TableCell></TableRow>)}</TableBody></Table></div>}</>}</section>
   <section aria-label="收件待办摘要" className="space-y-3"><h3 className="m-0 text-sm font-semibold">待我处理</h3>{sectionNote('pending_counts')}{home.sections.pending_counts.state!=='unavailable'&&<ul className="m-0 list-none divide-y divide-border border-y border-border bg-card p-0">{([['submitted','待人工决定','查看待人工决定'],['accepted','已接收待归档','查看已接收待归档']] as const).map(([filter,label,action])=><li key={filter} className="flex flex-wrap items-center gap-3 px-3 py-2"><Inbox className="size-4 shrink-0 text-muted-foreground"/><span className="min-w-0 flex-1 text-sm">{label}</span><span className="rounded bg-muted px-2 py-1 text-xs tabular-nums">{home.pending_counts[filter]??'未知'}</span><Button variant="ghost" className="min-h-11 text-[13px] md:min-h-8" onClick={()=>{const context=getDataContext();location.hash=submissionURL(space,filter,context.tenant,context.space);}}>{action}<ArrowUpRight className="size-4"/></Button></li>)}</ul>}<p className="text-xs text-muted-foreground">接收不等于归档；打开收件后由你确认，不自动执行。</p></section>
   <section aria-label="逻辑容量摘要" className="space-y-3 border-t border-border pt-4"><h3 className="m-0 text-sm font-semibold">空间逻辑容量</h3>{sectionNote('usage')}{home.sections.usage.state!=='unavailable'&&<dl className="flex flex-wrap gap-x-8 gap-y-3">{([['used_bytes','已使用'],['reserved_bytes','上传预约'],['quota_bytes','配额']] as const).map(([key,label])=><div key={key} className="flex items-baseline gap-3 text-[13px]"><dt className="text-muted-foreground">{label}</dt><dd className="m-0 font-medium">{home.usage[key]===null?'未知':bytes(home.usage[key]!)}</dd></div>)}</dl>}<p className="text-xs text-muted-foreground">不是云盘物理容量，也不证明存储当前可用。</p></section>
  </div>}
  {collection&&<HomeCollection key={collection.collection_id} collection={collection} onDenied={deny} onOpen={(file,button)=>{trigger.current=button;setSelected(file);}} close={()=>{setCollection(null);setSelected(null);collectionTrigger.current?.focus();}}/>}
  {selected&&<FixedReference key={selected.file_id+':'+selected.version_id} file={selected} denied={deny} close={()=>{setSelected(null);trigger.current?.focus();}}/>}
 </>;
}
export function FixedReference({file,close,denied,sourceLabel="首页"}:{sourceLabel?:"首页"|"集合页";file:FileRef;close:()=>void;denied:()=>void}){
 const [reading,setReading]=useState(false);
 const [state,setState]=useState('loading');const heading=useRef<HTMLHeadingElement>(null);const onDenied=useRef(denied);onDenied.current=denied;
 useEffect(()=>{
  heading.current?.focus();const controller=new AbortController();
  api<{file:{id:string;space_id:string;version_id:string};ref:string}>('/references?'+new URLSearchParams({ref:file.ref}),'GET',undefined,{},controller.signal).then(value=>{
   if(controller.signal.aborted)return;if(value.ref!==file.ref||value.file?.id!==file.file_id||value.file.space_id!==file.space_id||value.file.version_id!==file.version_id)throw new Error('invalid_contract');setState('ready');
  }).catch((error:{status?:number})=>{if(controller.signal.aborted)return;setState('error');if(error.status===401||error.status===403)onDenied.current();});
  return()=>controller.abort();
 },[file]);
 return <section aria-label={sourceLabel+"固定引用"} className="space-y-3 rounded-lg border border-border p-4"><header className="flex flex-wrap items-center gap-3"><h3 ref={heading} tabIndex={-1}>固定版本引用</h3><Button className="min-h-11" variant="outline" onClick={close}>关闭固定引用</Button></header>{state==='loading'?<p role="status">重新鉴权并核对原版本…</p>:state==='error'?<p role="alert">原固定版本当前不可用，未改查最新版本。</p>:<><p className="break-all">{file.path}</p><p className="break-all">{file.ref}</p><p className="text-xs text-muted-foreground">已核对原文件与版本元数据；正文仅在下方明确打开后读取，实际使用时仍需鉴权。</p><Button variant="outline" className="min-h-11" aria-expanded={reading} onClick={()=>setReading(value=>!value)}>{reading?'关闭原版本正文':'读取原版本正文'}</Button>{reading&&<HomeFileText label={sourceLabel+"原版本纯文本"} space={file.space_id} file={file.file_id} version={file.version_id} onDenied={()=>onDenied.current()}/>}</>}</section>;
}
