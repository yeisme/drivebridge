import { useEffect, useRef, useState } from 'react';
import { Button } from '../../components/ui/button';
import { listGroups, type AgentGroup } from './organization-client';
export type OrganizationView = { kind: 'all' | 'ordered' | 'pinned' | 'ungrouped' } | { kind: 'group'; id: string; name: string };
export function GrantOrganizationNav({ selected, onSelect, onDenied }: { selected: OrganizationView; onSelect: (view: OrganizationView) => void; onDenied: () => void }) {
  const [groups,setGroups]=useState<AgentGroup[]>([]),[cursor,setCursor]=useState(''),[next,setNext]=useState(''),[retry,setRetry]=useState(0);
  const [status,setStatus]=useState<'loading'|'ready'|'error'|'stale'>('loading');
  const deny=useRef(onDenied);deny.current=onDenied;
  useEffect(()=>{
    const controller=new AbortController();setStatus('loading');
    listGroups(cursor,controller.signal).then(page=>{
      if(controller.signal.aborted)return;
      if(!Array.isArray(page.items)||page.items.length>50||typeof page.next_cursor!=='string'||(cursor&&page.next_cursor===cursor))throw new Error('invalid_contract');
      setGroups(previous=>[...new Map([...(cursor?previous:[]),...page.items].map(group=>[group.group_id,group])).values()]);setNext(page.next_cursor);setStatus('ready');
    }).catch((error:{status?:number})=>{
      if(controller.signal.aborted)return;
      setGroups([]);setNext('');setStatus(error.status===410?'stale':'error');
      if(error.status===401||error.status===403)deny.current();
    });
    return()=>controller.abort();
  },[cursor,retry]);
  return <nav aria-label="Agent 组织视图" className="space-y-2 border-b border-border pb-3">
    <div className="flex flex-wrap gap-2">{([['all','全部授权'],['ordered','置顶优先'],['pinned','仅置顶'],['ungrouped','未分组']] as const).map(([kind,label])=><Button key={kind} variant={selected.kind===kind?'secondary':'ghost'} className="min-h-11 text-[13px] md:min-h-8" aria-pressed={selected.kind===kind} onClick={()=>onSelect({kind})}>{label}</Button>)}</div>
    <div className="flex flex-wrap gap-2">{groups.map(group=><Button key={group.group_id} variant={selected.kind==='group'&&selected.id===group.group_id?'secondary':'outline'} className="min-h-11 max-w-full whitespace-normal break-words text-[13px] md:min-h-8" aria-pressed={selected.kind==='group'&&selected.id===group.group_id} onClick={()=>onSelect({kind:'group',id:group.group_id,name:group.name})}>分组：{group.name}</Button>)}</div>
    {selected.kind==='group'&&<p className="m-0 text-sm">当前筛选：{selected.name}</p>}
    {status==='loading'&&<p role="status" className="text-sm">读取分组导航…</p>}
    {(status==='error'||status==='stale')&&<div><p role="alert">{status==='stale'?'分组分页已过期。':'分组导航读取失败，不代表没有分组。'}</p><Button variant="outline" className="min-h-11" onClick={()=>{setCursor('');setRetry(value=>value+1);}}>重读分组导航</Button></div>}
    {next&&status==='ready'&&<Button variant="outline" className="min-h-11" onClick={()=>setCursor(next)}>更多分组导航</Button>}
    <p className="m-0 text-xs text-muted-foreground">组织视图由服务端按可管理范围筛选并分页，置顶在分页前排序；全部授权保留原读取范围。分组导航按需加载，不显示推测的总量。</p>
  </nav>;
}
