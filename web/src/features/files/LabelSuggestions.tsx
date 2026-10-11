import { useEffect,useRef,useState } from 'react';
import { Button } from '../../components/ui/button';
import { Checkbox } from '../../components/ui/checkbox';
import { decideSuggestion,listSuggestions,readManualLabels,type Suggestion,type SuggestionDecision } from './suggestion-client';
import { claimSuggestionIntent,clearSuggestionIntent,readSuggestionIntent,suggestionSlot,type SuggestionIntent } from './suggestion-recovery';

export function LabelSuggestionEntry(props:{file:string;version:string;scope:string;readOnly?:boolean}){
 const [open,setOpen]=useState(false);
 return <div><Button variant="outline" className="min-h-11" aria-expanded={open} onClick={()=>setOpen(value=>!value)}>{open?'收起标签建议':'审阅 Agent 标签建议'}</Button>{open&&<LabelSuggestions {...props}/>}</div>;
}

const isOffline=()=>navigator.onLine===false;
type Phase='loading'|'ready'|'confirm'|'busy'|'unknown'|'done'|'conflict'|'denied'|'offline'|'error';
export function LabelSuggestions({file,version,scope,readOnly=false}:{file:string;version:string;scope:string;readOnly?:boolean}){
 const slot=suggestionSlot(scope,file,version);
 const [phase,setPhase]=useState<Phase>('loading'),[items,setItems]=useState<Suggestion[]>([]),[next,setNext]=useState('');
 const [choices,setChoices]=useState<Record<string,string[]>>({}),[intent,setIntent]=useState<SuggestionIntent|null>(null),[observed,setObserved]=useState(false);
 const [draft,setDraft]=useState<{suggestion:Suggestion;body:SuggestionDecision}|null>(null),[message,setMessage]=useState('');
 const [manual,setManual]=useState<string[]|null>(null);
 const groups=new Map<string,Suggestion[]>();for(const item of items)groups.set(item.agent_id,[...(groups.get(item.agent_id)||[]),item]);
 const active=useRef(true),running=useRef(false),controller=useRef<AbortController|null>(null);
 const valid=(s:Suggestion)=>s.file_id===file&&s.version_id===version&&!!s.suggestion_id&&typeof s.agent_id==='string'&&!!s.agent_id&&Number.isSafeInteger(s.revision)&&s.revision>0&&Array.isArray(s.labels)&&s.labels.length<=100&&s.labels.every(label=>typeof label==='string');
 async function page(cursor:string,signal:AbortSignal){
  const result=await listSuggestions(file,version,cursor,signal);if(!active.current||signal.aborted)return;
  if(!Array.isArray(result.items)||result.items.length>50||!result.items.every(valid)||typeof result.next_cursor!=='string'||(cursor&&result.next_cursor===cursor))throw new Error('invalid_contract');
  setItems(previous=>cursor?[...new Map([...previous,...result.items].map(item=>[item.suggestion_id,item])).values()]:result.items);setNext(result.next_cursor);
 }
 useEffect(()=>{
  active.current=true;const abort=new AbortController();controller.current=abort;
  const invalidate=()=>{active.current=false;controller.current?.abort();setItems([]);setChoices({});setDraft(null);setIntent(null);setMessage('');setManual(null);setObserved(false);setPhase('denied');};
  const offline=()=>{invalidate();setPhase('offline');};
  const storage=(event:StorageEvent)=>{if(event.key===slot||event.key===null)invalidate();};
  window.addEventListener('drivebridge-context',invalidate);window.addEventListener('storage',storage);window.addEventListener('offline',offline);
  async function load(){try{const saved=readSuggestionIntent(slot);if(saved&&saved.body.base_version_id!==version)throw new Error('invalid_recovery');setIntent(saved);await page('',abort.signal);if(active.current&&!abort.signal.aborted)setPhase(saved?'unknown':'ready');}catch(error){if(active.current&&!abort.signal.aborted){setItems([]);setPhase([401,403].includes((error as {status:number}).status)?'denied':'error');}}}
  if(isOffline())offline();else void load();return()=>{invalidate();window.removeEventListener('drivebridge-context',invalidate);window.removeEventListener('storage',storage);window.removeEventListener('offline',offline);};
 },[slot]);
 function propose(suggestion:Suggestion,decision:'accept'|'reject'){
  const labels=decision==='accept'?(choices[suggestion.suggestion_id]||[]):[];
  if(readOnly||suggestion.state!=='pending'||(decision==='accept'&&!labels.length))return;
  setDraft({suggestion,body:{expected_revision:suggestion.revision,base_version_id:version,decision,labels:[...labels]}});setMessage('');setPhase('confirm');
 }
 async function perform(mode:'refresh'|'more'|'observe'|'write'|'retry'){
  if(!active.current||running.current||((mode==='write'||mode==='retry')&&readOnly))return;
  if(isOffline()){active.current=false;controller.current?.abort();setItems([]);setChoices({});setDraft(null);setIntent(null);setMessage('');setManual(null);setObserved(false);setPhase('offline');return;}
  running.current=true;setPhase('busy');setMessage('');setObserved(false);setManual(null);const abort=new AbortController();controller.current=abort;let original=intent,sent=false;
  try{
   if(mode==='refresh'||mode==='more'||mode==='observe'){
    await page(mode==='more'?next:'',abort.signal);if(!active.current||abort.signal.aborted)return;
    if(original){
     const labels=await readManualLabels(file,abort.signal);if(!active.current||abort.signal.aborted)return;
     setManual(labels);setObserved(true);setPhase('unknown');setMessage('已读取此固定版本的建议列表和当前人工标签；当前状态不是原请求回执，未找到也不证明原决定失败。');
    }else setPhase('ready');return;
   }
   if(mode==='write'){
    if(!draft)throw new Error('missing_intent');
    const claimed=await claimSuggestionIntent(slot,draft.suggestion.suggestion_id,draft.body,abort.signal);if(!active.current||abort.signal.aborted)return;
    original=claimed.intent;setIntent(original);if(!claimed.claimed){setPhase('unknown');setObserved(false);return;}
   }
   if(!original||original.body.base_version_id!==version||JSON.stringify(readSuggestionIntent(slot))!==JSON.stringify(original))throw new Error('recovery_identity_changed');
   if(isOffline())throw new Error('offline');
   sent=true;const result=await decideSuggestion(original.suggestion,original.body,original.key,abort.signal);if(!active.current||abort.signal.aborted)return;
   if(!valid(result)||result.suggestion_id!==original.suggestion||result.state!==(original.body.decision==='accept'?'accepted':'rejected'))throw new Error('invalid_contract');
   setItems(previous=>previous.map(item=>item.suggestion_id===result.suggestion_id?result:item));setPhase('done');
  }catch(error){
   if(!active.current||abort.signal.aborted)return;const failure=error as Error&{status?:number};
   if(failure.status===401||failure.status===403){setItems([]);setChoices({});setDraft(null);setIntent(null);setManual(null);setPhase('denied');}
   else if(sent&&failure.status===409&&['revision_conflict','version_conflict'].includes(failure.message)){setPhase('conflict');setMessage('文件或建议版本已变化，本次决定被拒绝。保留原请求，不自动套用到最新文件版本。');}
   else if(original){setPhase('unknown');setObserved(false);setMessage('决定结果未知。只保留原键、原版本和原标签子集，不自动重复采纳。');}
   else{setItems([]);setNext('');setPhase('error');setMessage('建议读取或恢复存储不可用，请重读；未安全登记的决定不会发送。');}
  }finally{running.current=false;}
 }
 async function acknowledge(){if(!intent||!active.current||running.current)return;running.current=true;try{await clearSuggestionIntent(slot,intent);if(active.current){setIntent(null);setDraft(null);setChoices({});setPhase('ready');setMessage('原决定已确认。采纳只追加所选人工标签，不删除已有人工标签。');}}catch{if(active.current)setMessage('恢复记录未清除，请稍后确认。');}finally{running.current=false;}}
 return <section aria-label="固定版本标签建议" className="space-y-3 border-t border-border pt-4">
  <h3 className="text-sm font-semibold">Agent 标签建议</h3>
  <p className="break-all text-xs text-muted-foreground">版本 {version}。建议不等于人工标签；采纳仅追加所选子集，保留已有人工标签。</p>
  {message&&<p role="alert" className="text-sm">{message}</p>}
  {(phase==='loading'||phase==='busy')&&<p role="status">读取或处理标签建议…</p>}
  {phase==='offline'&&<p role="alert">当前离线，标签决定已停止。原恢复记录保留；联网后请收起并重新打开建议以重鉴权，不会自动提交。</p>}
  {phase==='denied'&&<p role="alert">权限或范围已变化，建议与选择已清除，请重新打开文件。</p>}
  {phase==='error'&&<><p role="alert">建议暂不可用，不代表没有建议。</p><Button variant="outline" className="min-h-11" onClick={()=>void perform('refresh')}>重读标签建议</Button></>}
  {phase==='ready'&&!items.length&&<p role="status">此固定版本没有已加载的标签建议。</p>}
  {phase==='ready'&&[...groups].map(([agent,suggestions])=><section key={agent} aria-label={'认证 Agent '+agent} className="space-y-2"><h4 className="break-all text-xs font-semibold">认证 Agent：{agent}</h4><p className="text-xs text-muted-foreground">仅当前已加载建议，不代表此来源的全部记录。</p>{suggestions.map(item=><article key={item.suggestion_id} className="space-y-2 rounded border border-border p-3" aria-label={'标签建议 '+item.suggestion_id}>
   <p className="m-0 break-all text-xs">建议版本 {item.revision} · {item.state==='pending'?'待人工审阅':item.state==='accepted'?'已采纳':'已拒绝'}</p>
   {item.labels.map(label=><label key={label} className="flex min-h-11 items-center gap-2 break-all text-sm"><Checkbox disabled={readOnly||item.state!=='pending'} checked={(choices[item.suggestion_id]||[]).includes(label)} onCheckedChange={checked=>setChoices(previous=>({...previous,[item.suggestion_id]:checked?[...(previous[item.suggestion_id]||[]).filter(value=>value!==label),label]:(previous[item.suggestion_id]||[]).filter(value=>value!==label)}))}/>{label}</label>)}
   {item.state==='pending'&&!readOnly&&<div className="flex flex-wrap gap-2"><Button variant="outline" className="min-h-11" disabled={!choices[item.suggestion_id]?.length} onClick={()=>propose(item,'accept')}>采纳所选标签</Button><Button variant="outline" className="min-h-11" onClick={()=>propose(item,'reject')}>拒绝此建议</Button></div>}
  </article>)}</section>)}
  {phase==='ready'&&next&&<Button variant="outline" className="min-h-11" onClick={()=>void perform('more')}>更多标签建议</Button>}
  {phase==='confirm'&&draft&&<div className="space-y-2"><p className="break-words">确认{draft.body.decision==='accept'?`采纳：${draft.body.labels?.join('、')}`:'拒绝此建议（不更改人工标签）'}？原建议 {draft.suggestion.suggestion_id}，版本 {draft.body.expected_revision}。</p><Button variant="outline" className="min-h-11" onClick={()=>setPhase('ready')}>返回选择</Button><Button className="min-h-11" onClick={()=>void perform('write')}>确认标签决定</Button></div>}
  {phase==='unknown'&&observed&&manual!==null&&<div aria-label="当前人工标签事实"><p className="text-sm">当前文件级人工标签（非固定版本快照、非原请求回执）：</p>{manual.length?<ul>{manual.map(label=><li className="break-all text-sm" key={label}>{label}</li>)}</ul>:<p>当前没有人工标签。</p>}</div>}
  {(phase==='unknown'||phase==='conflict')&&<div className="space-y-2"><Button variant="outline" className="min-h-11" onClick={()=>void perform('observe')}>观察原版本建议</Button>{observed&&!readOnly&&<><p className="text-sm">重试可能首次执行原决定；保持原键、原版本及原标签子集，不是只读查询。</p><Button className="min-h-11" onClick={()=>void perform('retry')}>确认重试原标签决定</Button></>}</div>}
  {phase==='done'&&<><p role="status">原标签决定已被服务端接受。</p><Button className="min-h-11" onClick={()=>void acknowledge()}>确认标签结果</Button></>}
 </section>;
}
