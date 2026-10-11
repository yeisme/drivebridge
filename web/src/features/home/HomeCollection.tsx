import {useEffect,useRef,useState} from 'react';
import {api,pinnedFile} from '../../api';
import {statFile} from '../../info-workspace/client';
import {Button} from '../../components/ui/button';
import type {components} from '../../generated/personal-workspace';
type Home=components['schemas']['WorkspaceHomeEnvelope']['data'];
type Item={file_id:string;version_id:string;path?:string};
type Collection={collection_id:string;kind:string;revision:number;computed_at:string;items:Item[];visible_count:number;coverage:string};
export function HomeCollection({collection,close,onDenied,onOpen,sourceLabel="首页"}:{sourceLabel?:"首页"|"集合页";collection:Home['collections'][number];close:()=>void;onDenied:()=>void;onOpen:(file:Home['recent_files'][number],trigger:HTMLButtonElement)=>void}){
 const [data,setData]=useState<Collection|null>(null),[state,setState]=useState('loading'),[attempt,setAttempt]=useState(0),[shown,setShown]=useState(50),[opening,setOpening]=useState(''),[memberError,setMemberError]=useState('');
 const heading=useRef<HTMLHeadingElement>(null),member=useRef<AbortController|null>(null),denied=useRef(onDenied);denied.current=onDenied;
 useEffect(()=>{heading.current?.focus();},[]);
 useEffect(()=>{
  const controller=new AbortController();member.current?.abort();setData(null);setState('loading');setShown(50);setOpening('');setMemberError('');
  const invalidate=()=>{controller.abort();member.current?.abort();setData(null);setOpening('');setState('denied');};
  const offline=()=>{invalidate();setState('offline');};
  window.addEventListener('drivebridge-context',invalidate);window.addEventListener('offline',offline);
  if(!navigator.onLine)offline();else api<Collection>('/file-collections/'+encodeURIComponent(collection.collection_id),'GET',undefined,{},controller.signal).then(value=>{
   if(controller.signal.aborted)return;
   if(!value||value.collection_id!==collection.collection_id||value.kind!==collection.kind||!Number.isSafeInteger(value.revision)||value.revision<1||typeof value.computed_at!=='string'||!['complete','partial'].includes(value.coverage)||!Array.isArray(value.items)||value.items.length>30000||value.visible_count!==value.items.length||!value.items.every(item=>item&&typeof item.file_id==='string'&&!!item.file_id&&typeof item.version_id==='string'&&!!item.version_id&&(item.path===undefined||typeof item.path==='string'))||new Set(value.items.map(item=>item.file_id)).size!==value.items.length)throw new Error('invalid_contract');
   setData(value);setState('ready');
  }).catch((error:{status?:number})=>{if(controller.signal.aborted)return;setData(null);if(error.status===401||error.status===403){setState('denied');denied.current();}else setState('error');});
  return()=>{controller.abort();member.current?.abort();window.removeEventListener('drivebridge-context',invalidate);window.removeEventListener('offline',offline);};
 },[collection.collection_id,collection.kind,attempt]);
 async function open(item:Item,trigger:HTMLButtonElement){
  member.current?.abort();const controller=new AbortController();member.current=controller;setOpening(item.file_id);setMemberError('');
  try{
   if(!navigator.onLine)throw new Error('offline');
   const current=await statFile(item.file_id,controller.signal);if(controller.signal.aborted)return;
   if(current.id!==item.file_id||current.trashed||!current.space_id)throw new Error('source_changed');
   const fixed=await pinnedFile(current,item.version_id,{},controller.signal);if(controller.signal.aborted)return;
   if(fixed.file.id!==item.file_id||fixed.file.space_id!==current.space_id||fixed.file.version_id!==item.version_id||typeof fixed.ref!=='string'||!fixed.ref.endsWith('/file/'+item.file_id+'/version/'+item.version_id))throw new Error('fixed_source_changed');
   onOpen({file_id:item.file_id,version_id:item.version_id,space_id:fixed.file.space_id,path:fixed.file.path,ref:fixed.ref,mime_type:fixed.file.mime_type,size_bytes:fixed.file.size_bytes,updated_at:fixed.file.updated_at},trigger);
  }catch(error){if(controller.signal.aborted)return;if([401,403].includes((error as {status:number}).status))denied.current();else setMemberError('原成员版本不可用，未改读最新版本。请重读集合确认当前成员。');}
  finally{if(!controller.signal.aborted)setOpening('');}
 }
 return <section aria-label={sourceLabel+"集合成员"} className="space-y-3 rounded-lg border border-border p-4">
  <header className="flex flex-wrap items-center gap-3"><h3 ref={heading} tabIndex={-1} className="m-0 break-all font-semibold">{collection.name} · 集合成员</h3><Button variant="outline" className="min-h-11" onClick={close}>关闭集合成员</Button></header>
  <p className="text-xs text-muted-foreground">仅观察当前授权下的动态成员，不创建快照、不修改集合。展开仅控制本地展示，不是服务端分页或固定集合快照。</p>
  {state==='loading'&&<p role="status">读取集合成员…</p>}
  {state==='error'&&<p role="alert">集合成员不可用、超出查询预算或合同不匹配，不代表空集合。</p>}
  {state==='denied'&&<p role="alert">权限或范围变化，集合成员已清除。</p>}
  {state==='offline'&&<p role="alert">当前离线，成员已清除；联网后手动重读。</p>}
  <Button variant="outline" className="min-h-11" disabled={state==='loading'||state==='denied'} onClick={()=>setAttempt(value=>value+1)}>重读集合成员</Button>
  {memberError&&<p role="alert">{memberError}</p>}
  {data&&<><p className="text-sm">本次可见成员：{data.visible_count} · {data.coverage==='complete'?'本次查询完整':'部分覆盖'} · 集合版本：{data.revision}</p>{data.revision!==collection.revision&&<p className="text-xs text-muted-foreground">集合已在{sourceLabel==='首页'?'首页':'列表'}观察后变化；以下为本次重新授权读取。</p>}{!data.items.length&&<p>当前授权下没有可见成员。</p>}
   <ul aria-label="集合成员列表" className="m-0 list-none space-y-2 p-0">{data.items.slice(0,shown).map(item=><li key={item.file_id}><Button variant="ghost" className="min-h-11 w-full justify-start whitespace-normal break-all text-left" disabled={!!opening} onClick={event=>void open(item,event.currentTarget)}>查看成员固定引用：{item.path||item.file_id}</Button></li>)}</ul>
   {opening&&<p role="status">核对成员原版本…</p>}{shown<data.items.length&&<Button variant="outline" className="min-h-11" onClick={()=>setShown(value=>value+50)}>展开更多已读取成员</Button>}
  </>}
 </section>;
}
