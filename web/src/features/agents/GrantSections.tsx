import { useId, useState, type ReactNode } from 'react';
import { Button } from '../../components/ui/button';
import type { Grant } from './grant-client';

// Groups describe only the already-authorized loaded page set, never a total.
export function GrantSections({grants,grouped,children}:{grants:Grant[];grouped:boolean;children:(grant:Grant)=>ReactNode}){
 const prefix=useId();const [collapsed,setCollapsed]=useState<Set<string>>(new Set());
 if(!grouped)return <div className="grid gap-3 lg:grid-cols-2" role="list" aria-label="新协议授权列表">{grants.map(children)}</div>;
 if(grants.some(grant=>!grant.organization||grant.organization.agent_id!==grant.agent_id||!(grant.organization.group_id===null||typeof grant.organization.group_id==='string'&&grant.organization.group_id.length>0)))return <p role="alert">组织归属投影不可用，请刷新；不能推断为未分组。</p>;
 const groups=new Map<string,{id:string|null;grants:Grant[]}>();
 for(const grant of grants){
  const id=grant.organization?.group_id??null;
  const key=JSON.stringify(id);const group=groups.get(key)||{id,grants:[]};group.grants.push(grant);groups.set(key,group);
 }
 return <div className="space-y-3">
  <p className="text-xs text-muted-foreground">按当前已加载授权分组；收起不撤权、不停止任务；服务端先分页，组内保留返回顺序，组顺序取首次出现位置，不是跨组全量排名。组标识不冒称最新组名。</p>
  {groups.size>0&&<div className="flex flex-wrap gap-2"><Button variant="outline" className="min-h-11" onClick={()=>setCollapsed(new Set())}>全部展开</Button><Button variant="outline" className="min-h-11" onClick={()=>setCollapsed(new Set(groups.keys()))}>全部收起</Button></div>}
  {[...groups].map(([key,group],index)=>{
   const open=!collapsed.has(key),label=group.id===null?'未分组':`分组 ${group.id}`,panel=`${prefix}-${index}`;
   return <section key={key} aria-label={label} className="space-y-3 rounded-lg border border-border p-3">
    <h3 className="m-0"><Button variant="ghost" className="min-h-11 max-w-full whitespace-normal break-all text-left" aria-expanded={open} aria-controls={panel} onClick={()=>setCollapsed(previous=>{const next=new Set(previous);if(open)next.add(key);else next.delete(key);return next;})}>{open?'收起':'展开'} {label} · 已加载 {group.grants.length} 项</Button></h3>
    <div id={panel} hidden={!open} className="grid gap-3 lg:grid-cols-2" role="list" aria-label={label+'授权'}>{group.grants.map(children)}</div>
   </section>;
  })}
 </div>;
}
