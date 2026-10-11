import { useEffect, useRef, useState } from 'react';
import { AppDialog } from '../../components/AppDialog';
import { Button } from '../../components/ui/button';
import { Checkbox } from '../../components/ui/checkbox';
import { SelectAdapter } from '../../components/SelectAdapter';
import { getOrganization, listGroups, patchOrganization, type AgentGroup, type AgentOrganization, type OrganizationPatch } from './organization-client';
import { claimOrganizationIntent, clearOrganizationIntent, organizationSlot, readOrganizationIntent, type OrganizationIntent } from './organization-recovery';
import type { Grant } from './grant-client';

type Phase = 'loading' | 'edit' | 'busy' | 'unknown' | 'conflict' | 'done' | 'denied' | 'error';
export function AgentOrganizationDialog({ grant, scope, close }: { grant: Grant; scope: string; close: () => void }) {
  const slot = organizationSlot(scope, grant.agent_id);
  const [phase,setPhase]=useState<Phase>('loading');
  const [current,setCurrent]=useState<AgentOrganization|null>(null);
  const [groups,setGroups]=useState<AgentGroup[]>([]), [next,setNext]=useState('');
  const [group,setGroup]=useState(''), [pinned,setPinned]=useState(false);
  const [intent,setIntent]=useState<OrganizationIntent|null>(null);
  const [observed,setObserved]=useState(false), [message,setMessage]=useState('');
  const active=useRef(true), running=useRef(false), controller=useRef<AbortController|null>(null);
  const validOrg=(value:AgentOrganization)=>value?.agent_id===grant.agent_id && Number.isSafeInteger(value.revision) && value.revision>=0 && typeof value.pinned==='boolean' && (value.group_id===null||typeof value.group_id==='string');
  useEffect(()=>{
    active.current=true;const abort=new AbortController();controller.current=abort;
    const invalidate=()=>{active.current=false;controller.current?.abort();setCurrent(null);setGroups([]);setMessage('');setPhase('denied');};
    const storage=(event:StorageEvent)=>{if(event.key===slot||event.key===null)invalidate();};
    window.addEventListener('drivebridge-context',invalidate);window.addEventListener('storage',storage);
    async function load(){
      try{
        const saved=readOrganizationIntent(slot);setIntent(saved);
        const [org,page]=await Promise.all([getOrganization(grant.agent_id,abort.signal),listGroups('',abort.signal)]);
        if(!active.current||abort.signal.aborted)return;
        if(!validOrg(org)||!Array.isArray(page.items)||page.items.length>50||typeof page.next_cursor!=='string')throw new Error('invalid_contract');
        setCurrent(org);setGroups(page.items);setNext(page.next_cursor);
        setGroup(saved?.body.group_id!==undefined?saved.body.group_id||'':org.group_id||'');setPinned(saved?.body.pinned??org.pinned);
        setPhase(saved?'unknown':'edit');
      }catch(error){if(active.current&&!abort.signal.aborted){setPhase([401,403].includes((error as {status:number}).status)?'denied':'error');}}
    }
    void load();return()=>{invalidate();window.removeEventListener('drivebridge-context',invalidate);window.removeEventListener('storage',storage);};
  },[slot]);
  async function perform(mode:'save'|'retry'|'observe'|'reload'|'more'){
    if(!active.current||running.current||!current)return;
    running.current=true;setPhase('busy');setMessage('');const abort=new AbortController();controller.current=abort;
    let original=intent, sent=false;
    try{
      if(mode==='more'){
        const page=await listGroups(next,abort.signal);if(!active.current||abort.signal.aborted)return;
        if(!Array.isArray(page.items)||page.items.length>50||typeof page.next_cursor!=='string'||(next&&page.next_cursor===next))throw new Error('invalid_contract');
        setGroups(previous=>[...new Map([...previous,...page.items].map(item=>[item.group_id,item])).values()]);setNext(page.next_cursor);setPhase('edit');return;
      }
      if(mode==='observe'||mode==='reload'){
        const org=await getOrganization(grant.agent_id,abort.signal);if(!active.current||abort.signal.aborted)return;if(!validOrg(org))throw new Error('invalid_contract');
        if(mode==='reload'){
          const page=await listGroups('',abort.signal);if(!active.current||abort.signal.aborted)return;
          if(!Array.isArray(page.items)||page.items.length>50||typeof page.next_cursor!=='string')throw new Error('invalid_contract');
          if(original)await clearOrganizationIntent(slot,original);if(!active.current)return;
          setGroups(page.items);setNext(page.next_cursor);setIntent(null);setPhase('edit');setMessage('已重读最新版本，分组和置顶意图保留。请再次确认，未自动提交。');
        }else{setObserved(true);setPhase('unknown');setMessage('已读取当前组织状态；这不是原请求回执，不能据此判断原请求成功。');}
        setCurrent(org);return;
      }
      if(mode==='save'){
        const body:OrganizationPatch={expected_revision:current.revision};
        if(group!==(current.group_id||''))body.group_id=group||null;
        if(pinned!==current.pinned)body.pinned=pinned;
        if(body.group_id===undefined&&body.pinned===undefined){setPhase('edit');setMessage('没有需要保存的变更。');return;}
        const claim=await claimOrganizationIntent(slot,body,abort.signal);if(!active.current||abort.signal.aborted)return;
        original=claim.intent;setIntent(original);
        if(!claim.claimed){setPhase('unknown');setObserved(false);return;}
      }
      if(!original||JSON.stringify(readOrganizationIntent(slot))!==JSON.stringify(original))throw new Error('recovery_identity_changed');
      sent=true;const org=await patchOrganization(grant.agent_id,original.body,original.key,abort.signal);
      if(!active.current||abort.signal.aborted)return;if(!validOrg(org))throw new Error('invalid_contract');
      setCurrent(org);setPhase('done');
    }catch(error){
      if(!active.current||abort.signal.aborted)return;const failure=error as Error & {status?:number};
      if(failure.status===401||failure.status===403){setCurrent(null);setGroups([]);setPhase('denied');}
      else if(sent&&failure.status===409&&failure.message==='revision_conflict'){setPhase('conflict');setMessage('版本或目标分组已变化，原变更被拒绝。请重读后确认保留的意图。');}
      else if(original){setPhase('unknown');setObserved(false);setMessage('结果未知，保留原键和原参数。先读取当前状态，再决定是否重试原变更。');}
      else{setPhase('error');setMessage('读取或恢复存储不可用，请关闭后重试。未安全登记时不会发送变更。');}
    }finally{running.current=false;}
  }
  async function acknowledge(){
    if(!active.current||running.current||!intent)return;running.current=true;
    try{await clearOrganizationIntent(slot,intent);if(active.current)close();}catch{if(active.current)setMessage('恢复标识未清除，请稍后再确认。');}finally{running.current=false;}
  }
  const unknownGroup=group&&!groups.some(item=>item.group_id===group);
  return <AppDialog title="Agent 分组与置顶" close={close} className="[&_[role=combobox]]:min-h-11 [&_[data-slot=dialog-close]]:size-11">
    <p>{grant.name} · 仅整理个人视图，不改变文件权限、期限或凭据。</p>
    {message&&<p role="alert">{message}</p>}
    {(phase==='loading'||phase==='busy')&&<p role="status">正在处理组织数据…</p>}
    {phase==='edit'&&current&&<div className="space-y-4">
      <label className="grid gap-2">所属分组<SelectAdapter aria-label="所属分组" value={group} onChange={event=>setGroup(event.target.value)}><option value="">未分组</option>{unknownGroup&&<option value={group} disabled>原分组尚未加载或已不可用</option>}{groups.map(item=><option key={item.group_id} value={item.group_id}>{item.name}</option>)}</SelectAdapter></label>
      {next&&<Button variant="outline" className="min-h-11" onClick={()=>void perform('more')}>加载更多分组</Button>}
      {!groups.length&&<p>当前页没有分组；仍可置顶。关闭后可从“管理分组”创建。</p>}
      <label className="flex min-h-11 items-center gap-2"><Checkbox checked={pinned} onCheckedChange={setPinned}/>在个人组织中置顶</label>
      <p className="text-sm text-muted-foreground">移出分组不会撤销授权；只修改有变化的字段。当前组织版本 {current.revision}。</p>
      <Button className="min-h-11" disabled={!!unknownGroup&&group!==(current.group_id||'')} onClick={()=>void perform('save')}>保存分组与置顶</Button>
    </div>}
    {phase==='unknown'&&<div className="space-y-3"><p>有尚未确认的组织变更。恢复记录只包含原键、版本及分组／置顶参数。</p>{current&&<p>当前观察：版本 {current.revision}，{current.pinned?'已置顶':'未置顶'}；不是原回执。</p>}<Button variant="outline" className="min-h-11" onClick={()=>void perform('observe')}>读取当前组织状态</Button>{observed&&<><p>重试可能首次执行原变更，不是只读查询；使用原键和原版本，不扩大修改范围。</p><Button className="min-h-11" onClick={()=>void perform('retry')}>确认按原键重试原变更</Button></>}</div>}
    {phase==='conflict'&&<Button className="min-h-11" onClick={()=>void perform('reload')}>保留意图并重读组织</Button>}
    {phase==='done'&&current&&<div className="space-y-3"><p role="status">原请求已被服务端接受；返回当前组织版本 {current.revision}。</p><p>幂等重放可能返回后续状态，不将该版本视为原提交版本。{current.pinned?'当前已置顶。':'当前未置顶。'}</p><Button className="min-h-11" onClick={()=>void acknowledge()}>确认组织结果并刷新</Button></div>}
    {phase==='denied'&&<p role="alert">权限或范围已变化，组织数据已清除。请关闭后重新确认身份。</p>}
    {phase==='error'&&<p role="alert">组织读取或恢复存储不可用；没有把故障当成未分组。</p>}
  </AppDialog>;
}
