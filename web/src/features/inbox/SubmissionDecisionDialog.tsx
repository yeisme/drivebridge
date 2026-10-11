import { useEffect,useRef,useState } from 'react';
import { AppDialog } from '../../components/AppDialog';
import { Button } from '../../components/ui/button';
import { SelectAdapter } from '../../components/SelectAdapter';
import { decideSubmission,getSubmission,type Submission } from './submission-client';
import { claimDecision,clearDecision,decisionSlot,readDecision,type DecisionIntent } from './decision-recovery';

type Phase='loading'|'confirm'|'busy'|'unknown'|'conflict'|'done'|'denied'|'error';
export function SubmissionDecisionDialog({submission,scope,close}:{submission:Submission;scope:string;close:()=>void}){
 const slot=decisionSlot(scope,submission.submission_id);
 const [phase,setPhase]=useState<Phase>('loading'),[current,setCurrent]=useState(submission),[decision,setDecision]=useState<'accept'|'reject'>('accept');
 const [intent,setIntent]=useState<DecisionIntent|null>(null),[observed,setObserved]=useState(false),[message,setMessage]=useState('');
 const active=useRef(true),running=useRef(false),controller=useRef<AbortController|null>(null);
 const eligible=current.state==='submitted'&&!!current.file_ref&&!!current.version_id;
 const valid=(value:Submission)=>value.submission_id===submission.submission_id&&value.space_id===submission.space_id&&value.file_ref===submission.file_ref&&value.version_id===submission.version_id&&Number.isSafeInteger(value.revision);
 useEffect(()=>{
  active.current=true;const abort=new AbortController();controller.current=abort;
  const invalidate=()=>{active.current=false;controller.current?.abort();setMessage('');setPhase('denied');};
  const storage=(event:StorageEvent)=>{if(event.key===slot||event.key===null)invalidate();};
  window.addEventListener('drivebridge-context',invalidate);window.addEventListener('storage',storage);
  async function load(){try{
    const saved=readDecision(slot);setIntent(saved);if(saved){setDecision(saved.body.decision);setPhase('unknown');return;}
    const latest=await getSubmission(submission.submission_id,abort.signal);if(!active.current||abort.signal.aborted)return;if(!valid(latest))throw new Error('fixed_source_changed');setCurrent(latest);setPhase('confirm');
  }catch(error){if(active.current&&!abort.signal.aborted)setPhase([401,403].includes((error as {status:number}).status)?'denied':'error');}}
  void load();return()=>{invalidate();window.removeEventListener('drivebridge-context',invalidate);window.removeEventListener('storage',storage);};
 },[slot]);
 async function perform(mode:'write'|'observe'|'retry'|'reload'){
  if(!active.current||running.current||(mode==='write'&&!eligible))return;
  running.current=true;setPhase('busy');setMessage('');const abort=new AbortController();controller.current=abort;let original=intent,sent=false;
  try{
    if(mode==='observe'||mode==='reload'){
      const latest=await getSubmission(submission.submission_id,abort.signal);if(!active.current||abort.signal.aborted)return;
      if(!valid(latest))throw new Error('fixed_source_changed');
      if(mode==='reload'){
        if(original)await clearDecision(slot,original);if(!active.current)return;setIntent(null);setCurrent(latest);setPhase('confirm');setMessage('已重读原提交版本，决定意图保留。若仍待验收，请重新确认；未自动提交。');
      }else{setCurrent(latest);setObserved(true);setPhase('unknown');setMessage(`当前状态 ${latest.state}，收件版本 ${latest.revision}；这是当前观察，不是原请求回执。`);}return;
    }
    if(mode==='write'){
      const claim=await claimDecision(slot,{expected_revision:current.revision,version_id:current.version_id!,decision},abort.signal);if(!active.current||abort.signal.aborted)return;
      original=claim.intent;setIntent(original);if(!claim.claimed){setDecision(original.body.decision);setPhase('unknown');setObserved(false);return;}
    }
    if(!original||JSON.stringify(readDecision(slot))!==JSON.stringify(original))throw new Error('recovery_identity_changed');
    sent=true;const result=await decideSubmission(submission.submission_id,original.body,original.key,abort.signal);
    if(!active.current||abort.signal.aborted)return;
    if(!valid(result)||result.decision?.version_id!==original.body.version_id||result.decision.decision!==original.body.decision)throw new Error('invalid_contract');
    setCurrent(result);setPhase('done');
  }catch(error){
    if(!active.current||abort.signal.aborted)return;const failure=error as Error&{status?:number};
    if(failure.status===401||failure.status===403)setPhase('denied');
    else if(sent&&failure.status===409&&['revision_conflict','version_conflict'].includes(failure.message)){setPhase('conflict');setMessage('收件或文件版本已变化，原决定被拒绝。先重读，不自动套用到新的版本。');}
    else if(original){setPhase('unknown');setObserved(false);setMessage('结果未知。保留原键与固定版本，先读取当前状态再决定是否重试。');}
    else{setPhase('error');setMessage('原版本或恢复存储不可用，未安全登记的决定不会发送。');}
  }finally{running.current=false;}
 }
 async function acknowledge(){if(!active.current||running.current||!intent)return;running.current=true;try{await clearDecision(slot,intent);if(active.current)close();}catch{if(active.current)setMessage('恢复标识未清除，请稍后确认。');}finally{running.current=false;}}
 return <AppDialog title="人工验收提交收件" close={close} className="[&_[role=combobox]]:min-h-11 [&_[data-slot=dialog-close]]:size-11">
  {phase!=='denied'&&<><p className="break-all">收件 {submission.submission_id} · 固定文件版本 {intent?.body.version_id||current.version_id||'未提供'}</p><p>接收只记录人工决定，不自动归档；拒绝保留文件，不执行删除。请先查看原提交内容再确认。</p></>}
  {message&&<p role="alert">{message}</p>}
  {(phase==='loading'||phase==='busy')&&<p role="status">正在处理原验收请求…</p>}
  {phase==='confirm'&&(eligible?<div className="space-y-3"><label className="grid gap-2">人工决定<SelectAdapter aria-label="人工验收决定" value={decision} onChange={event=>setDecision(event.target.value as 'accept'|'reject')}><option value="accept">接收（不自动归档）</option><option value="reject">拒绝（保留文件）</option></SelectAdapter></label><p>基于收件版本 {current.revision}。本入口不收集自由文本理由，不向浏览器持久化评语。</p><Button className="min-h-11" variant={decision==='reject'?'destructive':'default'} onClick={()=>void perform('write')}>{decision==='accept'?'确认接收原版本':'确认拒绝原版本'}</Button></div>:<p role="status">当前不是待验收状态，或缺少固定版本，不能提交新的决定。</p>)}
  {phase==='unknown'&&<div className="space-y-3"><p>待确认原决定：{intent?.body.decision==='reject'?'拒绝':'接收'}。关闭和刷新不会改用新键。</p><Button variant="outline" className="min-h-11" onClick={()=>void perform('observe')}>读取当前验收状态</Button>{observed&&<><p>原键重试可能首次执行决定，不是只读查询；使用原收件版本与原文件版本。</p><Button className="min-h-11" onClick={()=>void perform('retry')}>确认按原键重试验收</Button></>}</div>}
  {phase==='conflict'&&<Button className="min-h-11" onClick={()=>void perform('reload')}>保留决定并重读原版本</Button>}
  {phase==='done'&&<div className="space-y-3"><p role="status">原验收请求已被接受：{intent?.body.decision==='reject'?'拒绝，文件保留':'接收，归档另行确认'}。</p><p>当前收件状态：{current.state}。重放可能返回后续状态，不冒称原提交版本。</p><Button className="min-h-11" onClick={()=>void acknowledge()}>确认验收结果并刷新</Button></div>}
  {phase==='denied'&&<p role="alert">权限或范围已变化，验收操作已停止。请关闭后重新确认。</p>}
  {phase==='error'&&<p role="alert">固定版本读取或恢复存储不可用，不能安全提交决定。</p>}
 </AppDialog>;
}
