import { useEffect,useRef,useState } from 'react';
import { api } from '../../api';
import { Button } from '../../components/ui/button';
import type { components } from '../../generated/personal-workspace';
type Page=components['schemas']['BatchTaskItemsEnvelope']['data'];
export function BatchTaskItems({batch,close,onDenied}:{batch:string;close:()=>void;onDenied:()=>void}){
 const [items,setItems]=useState<Page['items']>([]),[cursor,setCursor]=useState(''),[next,setNext]=useState(''),[attempt,setAttempt]=useState(0);
 const [state,setState]=useState<'loading'|'ready'|'error'|'stale'|'denied'>('loading');
 const revision=useRef(''),denied=useRef(onDenied),heading=useRef<HTMLHeadingElement>(null);denied.current=onDenied;
 useEffect(()=>{heading.current?.focus();},[]);
 useEffect(()=>{
  const controller=new AbortController();setState('loading');
  const invalidate=()=>{controller.abort();setItems([]);setNext('');revision.current='';setState('denied');};
  window.addEventListener('drivebridge-context',invalidate);
  const query=new URLSearchParams({limit:'50'});if(cursor)query.set('cursor',cursor);
  api<Page>(`/workspace/tasks/upload_batch/${encodeURIComponent(batch)}/items?${query}`,'GET',undefined,{},controller.signal).then(page=>{
   if(controller.signal.aborted)return;
   if(!page||page.batch_id!==batch||typeof page.revision!=='string'||!page.revision||!Array.isArray(page.items)||page.items.length>50||typeof page.next_cursor!=='string'||(cursor&&page.next_cursor===cursor)||!page.items.every(item=>item&&typeof item.item_id==='string'&&!!item.item_id&&typeof item.relative_path==='string'&&typeof item.is_dir==='boolean'&&typeof item.owner_status==='string'&&['confirmed','unknown'].includes(item.result_certainty)&&(item.operation_id===null||typeof item.operation_id==='string'&&!!item.operation_id)&&(!item.is_dir||item.operation_id===null)&&(item.result_certainty!=='confirmed'||item.owner_status==='completed'&&item.operation_id!==null)))throw new Error('invalid_contract');
   if(cursor&&page.revision!==revision.current)throw Object.assign(new Error('cursor_expired'),{status:410});
   if(new Set(page.items.map(item=>item.item_id)).size!==page.items.length)throw new Error('invalid_contract');
   revision.current=page.revision;
   setItems(previous=>cursor?[...new Map([...previous,...page.items].map(item=>[item.item_id,item])).values()]:page.items);setNext(page.next_cursor);setState('ready');
  }).catch((error:{status?:number})=>{
   if(controller.signal.aborted)return;setItems([]);setNext('');revision.current='';
   if(error.status===401||error.status===403){setState('denied');denied.current();}else setState(error.status===410?'stale':'error');
  });
  return()=>{controller.abort();window.removeEventListener('drivebridge-context',invalidate);};
 },[batch,cursor,attempt]);
 return <section aria-label="批量逐项详情" className="min-w-0 space-y-3">
  <header className="flex flex-wrap items-center gap-3"><h3 ref={heading} tabIndex={-1} className="m-0 text-base font-semibold">批量逐项详情</h3><Button variant="outline" className="min-h-11" onClick={close}>关闭批量详情</Button></header>
  <p className="text-sm">只读条目观察；条目标识不是恢复操作标识。未确认不等于失败，已确认发布不等于人工验收。不执行恢复或读取正文。</p>
  {state==='loading'&&<p role="status">读取原批量条目…</p>}
  {state==='stale'&&<p role="alert">批量条目版本已变化，旧分页已清除，请从第一页重读。</p>}
  {state==='error'&&<p role="alert">批量条目不可用或来源合同不匹配，不代表没有条目。</p>}
  {state==='denied'&&<p role="alert">权限或范围已变化，批量条目已清除。</p>}
  {state!=='denied'&&<Button variant="outline" className="min-h-11" disabled={state==='loading'} onClick={()=>{setItems([]);setCursor('');setNext('');revision.current='';setAttempt(value=>value+1);}}>重读原批量条目</Button>}
  {state==='ready'&&!items.length&&<p role="status">原批量当前没有已枚举条目。</p>}
  <ul className="m-0 list-none space-y-2 p-0" aria-label="批量条目列表">{items.map(item=><li key={item.item_id} className="space-y-1 rounded border border-border p-3">
   <h4 className="m-0 break-all text-sm">{item.relative_path}</h4><p className="break-all text-xs">条目标识：{item.item_id}</p>
   <p className="text-sm">{item.is_dir?'目录':'文件'} · 原状态：{item.owner_status} · {item.result_certainty==='confirmed'?'已确认发布':'结果未确认'}</p>
   <p className="break-all text-xs">{item.operation_id?`关联原操作：${item.operation_id}`:'尚无关联操作'}</p>
  </li>)}</ul>
  {state==='ready'&&next&&<Button variant="outline" className="min-h-11" onClick={()=>setCursor(next)}>更多批量条目</Button>}
 </section>;
}
