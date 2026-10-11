import { useEffect, useRef, useState } from 'react';
import { AppDialog } from '../../components/AppDialog';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { changeGroup, getGroup, listGroups, type AgentGroup, type GroupChange } from './organization-client';
import { claimGroupIntent, clearGroupIntent, groupSlot, readGroupIntent, type GroupIntent } from './group-recovery';

type Phase = 'loading' | 'list' | 'edit' | 'busy' | 'unknown' | 'conflict' | 'done' | 'denied' | 'error';
const labels = { create: '创建分组', rename: '修改分组名称', delete: '删除分组' };
export function AgentGroupsDialog({ scope, close }: { scope: string; close: () => void }) {
  const slot=groupSlot(scope);
  const [phase,setPhase]=useState<Phase>('loading'), [groups,setGroups]=useState<AgentGroup[]>([]), [next,setNext]=useState('');
  const [draft,setDraft]=useState<GroupChange|null>(null), [name,setName]=useState('');
  const [intent,setIntent]=useState<GroupIntent|null>(null), [message,setMessage]=useState(''), [observed,setObserved]=useState(false);
  const active=useRef(true), running=useRef(false), controller=useRef<AbortController|null>(null);
  useEffect(()=>{
    active.current=true; const abort=new AbortController();controller.current=abort;
    const invalidate=()=>{active.current=false;controller.current?.abort();setGroups([]);setDraft(null);setMessage('');setPhase('denied');};
    const storage=(event:StorageEvent)=>{if(event.key===slot||event.key===null)invalidate();};
    window.addEventListener('drivebridge-context',invalidate);window.addEventListener('storage',storage);
    async function load(){try{
      const pending=readGroupIntent(slot);setIntent(pending);
      if(pending){setDraft(pending.change);setName(pending.change.action==='delete'?'':pending.change.body.name);setPhase('unknown');return;}
      const page=await listGroups('',abort.signal);if(!active.current||abort.signal.aborted)return;
      if(!Array.isArray(page.items)||page.items.length>50||typeof page.next_cursor!=='string')throw new Error('invalid_contract');
      setGroups(page.items);setNext(page.next_cursor);setPhase('list');
    }catch(error){if(active.current&&!abort.signal.aborted)setPhase([401,403].includes((error as {status:number}).status)?'denied':'error');}}
    void load();return()=>{invalidate();window.removeEventListener('drivebridge-context',invalidate);window.removeEventListener('storage',storage);};
  },[slot]);
  function edit(change:GroupChange){setDraft(change);setName(change.action==='delete'?groups.find(group=>group.group_id===change.id)?.name||change.id:change.body.name);setMessage('');setPhase('edit');}
  async function perform(mode:'save'|'retry'|'observe'|'reload'|'more'){
    if(running.current||!active.current)return;
    if(mode==='save'&&draft?.action!=='delete'&&(!name.trim()||[...name.trim()].length>80)){setMessage('分组名称需要1–80个字符。');return;}
    running.current=true;setPhase('busy');setMessage('');const abort=new AbortController();controller.current=abort;let original=intent,sent=false;
    try{
      if(mode==='more'||mode==='observe'||mode==='reload'){
        if(mode==='reload'&&!original)throw new Error('invalid_recovery');
        if((mode==='observe'||mode==='reload')&&original&&original.change.action!=='create'){
          let group:AgentGroup|null=null;
          try{group=await getGroup(original.change.id,abort.signal);}catch(error){if((error as {status:number}).status!==404)throw error;if(mode==='reload')throw error;}
          if(!active.current||abort.signal.aborted)return;
          if(group&&(group.group_id!==original.change.id||!Number.isSafeInteger(group.revision)))throw new Error('invalid_contract');
          if(mode==='reload'&&group){
            await clearGroupIntent(slot,original);if(!active.current)return;
            setDraft(original.change.action==='rename'?{action:'rename',id:original.change.id,body:{name:original.change.body.name,expected_revision:group.revision}}:{action:'delete',id:original.change.id,body:{expected_revision:group.revision}});setIntent(null);setPhase('edit');setMessage('已重读版本，原意图保留，请重新确认。');return;
          }
          setMessage(group?`当前分组：${group.name}，版本 ${group.revision}；这不是原请求回执。`:'当前读取返回404，不能证明原删除或创建成功。');
        }else{
          const page=await listGroups(mode==='more'?next:'',abort.signal);if(!active.current||abort.signal.aborted)return;
          if(!Array.isArray(page.items)||page.items.length>50||typeof page.next_cursor!=='string'||(mode==='more'&&next&&page.next_cursor===next))throw new Error('invalid_contract');
          setGroups(previous=>mode==='more'?[...new Map([...previous,...page.items].map(group=>[group.group_id,group])).values()]:page.items);setNext(page.next_cursor);
          if(mode==='reload'&&original){await clearGroupIntent(slot,original);if(!active.current)return;setIntent(null);setPhase('edit');setMessage('已重读分组列表，原名称保留，请修正冲突后重新确认。');return;}
          if(mode==='observe')setMessage('已读取分组首页；名称匹配或未找到都不是原请求的提交证明。');
        }
        if(mode==='more')setPhase('list');else{setObserved(true);setPhase('unknown');}return;
      }
      if(mode==='save'){
        if(!draft)throw new Error('missing_intent');
        const change:GroupChange=draft.action==='delete'?draft:draft.action==='create'?{action:'create',body:{name:name.trim()}}:{action:'rename',id:draft.id,body:{name:name.trim(),expected_revision:draft.body.expected_revision}};
        const claim=await claimGroupIntent(slot,change,abort.signal);if(!active.current||abort.signal.aborted)return;
        original=claim.intent;setIntent(original);setDraft(original.change);
        if(!claim.claimed){setPhase('unknown');setObserved(false);return;}
      }
      if(!original||JSON.stringify(readGroupIntent(slot))!==JSON.stringify(original))throw new Error('recovery_identity_changed');
      sent=true;const result=await changeGroup(original.change,original.key,abort.signal);if(!active.current||abort.signal.aborted)return;
      if(original.change.action==='delete'){
        if(!('deleted' in result)||result.deleted!==true||result.group_id!==original.change.id)throw new Error('invalid_contract');
        setMessage('原删除请求已被接受；授权和置顶保留，分组关联解除。');
      }else{
        if(!('revision' in result)||!Number.isSafeInteger(result.revision)||!result.group_id||(original.change.action==='rename'&&result.group_id!==original.change.id))throw new Error('invalid_contract');
        setMessage(`原请求已被接受，返回当前分组「${result.name}」版本 ${result.revision}；幂等重放可能返回后续状态。`);
      }
      setPhase('done');
    }catch(error){
      if(!active.current||abort.signal.aborted)return;const failure=error as Error&{status?:number};
      if(failure.status===401||failure.status===403){setGroups([]);setDraft(null);setPhase('denied');}
      else if(sent&&((failure.status===409&&['revision_conflict','name_conflict'].includes(failure.message))||(failure.status===400&&failure.message==='invalid_request'))){setPhase('conflict');setMessage('原请求被明确拒绝；请重读并检查名称或版本，保留意图后重新确认。');}
      else if(original){setPhase('unknown');setObserved(false);setMessage('结果未知，原请求保留。先观察当前分组，再决定是否按原键重试。');}
      else{setPhase('error');setMessage('读取或恢复存储不可用，未安全登记的变更不会发送。');}
    }finally{running.current=false;}
  }
  async function acknowledge(){if(!intent||running.current||!active.current)return;running.current=true;try{await clearGroupIntent(slot,intent);if(active.current)close();}catch{if(active.current)setMessage('恢复标识未清除，请稍后确认。');}finally{running.current=false;}}
  return <AppDialog title="管理 Agent 分组" close={close} className="[&_input]:min-h-11 [&_[data-slot=dialog-close]]:size-11">
    <p>分组仅整理个人视图。删除只解除归组，不删除授权或文件，也不清除置顶。</p>
    {message&&<p role={phase==='done'?'status':'alert'}>{message}</p>}
    {(phase==='loading'||phase==='busy')&&<p role="status">正在处理分组…</p>}
    {phase==='list'&&<div className="space-y-3"><Button className="min-h-11" onClick={()=>edit({action:'create',body:{name:''}})}>新建分组</Button>{!groups.length&&<p>当前页没有分组。</p>}<ul className="m-0 list-none space-y-2 p-0">{groups.map(group=><li key={group.group_id} className="flex flex-wrap items-center gap-2 rounded border border-border p-3"><span className="min-w-0 flex-1 break-words">{group.name}</span><Button variant="outline" className="min-h-11" onClick={()=>edit({action:'rename',id:group.group_id,body:{name:group.name,expected_revision:group.revision}})}>改名 {group.name}</Button><Button variant="outline" className="min-h-11" onClick={()=>edit({action:'delete',id:group.group_id,body:{expected_revision:group.revision}})}>删除 {group.name}</Button></li>)}</ul>{next&&<Button variant="outline" className="min-h-11" onClick={()=>void perform('more')}>加载更多分组</Button>}</div>}
    {phase==='edit'&&draft&&<div className="space-y-3">{draft.action==='delete'?<p>确认删除「{name || draft.id}」（版本 {draft.body.expected_revision}）？所有成员将解除归组，授权和置顶仍保留。</p>:<label className="grid gap-2">分组名称<Input aria-label="分组名称" value={name} maxLength={80} onChange={event=>setName(event.target.value)}/></label>}<div className="dialog-actions"><Button variant="outline" className="min-h-11" onClick={()=>{setPhase('list');setMessage('');}}>取消变更</Button><Button variant={draft.action==='delete'?'destructive':'default'} className="min-h-11" onClick={()=>void perform('save')}>确认{labels[draft.action]}</Button></div></div>}
    {phase==='unknown'&&<div className="space-y-3"><p>待确认原动作：{intent&&labels[intent.change.action]}。不会自动再建组或改名。</p><Button variant="outline" className="min-h-11" onClick={()=>void perform('observe')}>观察当前分组</Button>{observed&&<><p>原键重试可能首次执行变更，不是只读查询。仍使用原名称、目标与版本。</p><Button className="min-h-11" onClick={()=>void perform('retry')}>确认重试原分组请求</Button></>}</div>}
    {phase==='conflict'&&<Button className="min-h-11" onClick={()=>void perform('reload')}>保留意图并重读分组</Button>}
    {phase==='done'&&<Button className="min-h-11" onClick={()=>void acknowledge()}>确认分组结果并刷新</Button>}
    {phase==='denied'&&<p role="alert">权限或范围已变化，分组数据已清除。</p>}
    {phase==='error'&&<p role="alert">分组读取或恢复存储不可用；请关闭后重试，不将故障当空列表。</p>}
  </AppDialog>;
}
