import { useEffect,useRef,useState } from 'react';
import type { Operation,Space } from '../../api';
import { AppDialog } from '../../components/AppDialog';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { SelectAdapter } from '../../components/SelectAdapter';
import { archiveSubmission,getArchiveFile,getArchiveOperation,getSubmission,resumeArchiveOperation,type Submission } from './submission-client';
import { archiveSlot,claimArchive,clearArchive,readArchive,type ArchiveIntent } from './archive-recovery';
import { invalidArchivePrefix } from './archive-target';

type Phase='loading'|'confirm'|'busy'|'unknown'|'invalid-target'|'operation'|'done'|'denied'|'error';
export function SubmissionArchiveDialog({submission,scope,spaces,close}:{submission:Submission;scope:string;spaces:Space[];close:()=>void}){
 const slot=archiveSlot(scope,submission.submission_id);
 const [phase,setPhase]=useState<Phase>('loading'),[current,setCurrent]=useState(submission),[intent,setIntent]=useState<ArchiveIntent|null>(null);
 const [target,setTarget]=useState(spaces.some(space=>space.id===submission.space_id)?submission.space_id:''),[prefix,setPrefix]=useState('');
 const [operation,setOperation]=useState<Operation|null>(null),[observed,setObserved]=useState(false),[message,setMessage]=useState('');
 const active=useRef(true),running=useRef(false),controller=useRef<AbortController|null>(null);
 const valid=(value:Submission)=>value.submission_id===submission.submission_id&&value.space_id===submission.space_id&&value.file_ref===submission.file_ref&&value.version_id===submission.version_id;
 async function inspect(signal:AbortSignal,original:ArchiveIntent|null,expectedOperation?:string){
  const sub=await getSubmission(submission.submission_id,signal);if(!active.current||signal.aborted)return;
  if(!valid(sub))throw new Error('fixed_source_changed');setCurrent(sub);
  if(!sub.archive_operation_ref){
   if(expectedOperation)throw new Error('archive_binding_missing');
   setOperation(null);setObserved(true);
   const rejectedTarget=original&&invalidArchivePrefix(original.body.target_prefix)&&sub.state==='accepted'&&sub.revision===original.body.expected_revision&&sub.version_id===original.body.version_id;
   setPhase(rejectedTarget?'invalid-target':original?'unknown':'confirm');return;
  }
  if(original&&!expectedOperation){setOperation(null);setObserved(true);setPhase('unknown');setMessage('收件已关联归档任务，但当前状态不能证明它属于原请求键。请明确按原键回查／重试以验证关联。');return;}
  if(expectedOperation&&expectedOperation!==sub.archive_operation_ref)throw new Error('archive_identity_changed');
  const {operation:op}=await getArchiveOperation(sub.archive_operation_ref,signal);if(!active.current||signal.aborted)return;
  if(op.id!==sub.archive_operation_ref||op.kind!=='archive'||op.source_id!==sub.file_ref||op.source_version!==sub.version_id||(original&&op.space_id!==original.body.target_space_id))throw new Error('archive_identity_changed');
  setOperation(op);
  if(sub.state==='archived'&&op.status==='completed'&&sub.archived_file_ref&&sub.archived_file_ref===op.file_id&&op.version_id){
   const file=await getArchiveFile(op.file_id,signal);if(!active.current||signal.aborted)return;
   if(file.id!==op.file_id||file.space_id!==op.space_id||file.trashed)throw new Error('archive_result_unavailable');
   setPhase('done');
  }else setPhase('operation');
 }
 useEffect(()=>{
  active.current=true;const abort=new AbortController();controller.current=abort;
  const invalidate=()=>{active.current=false;controller.current?.abort();setOperation(null);setMessage('');setPhase('denied');};
  const storage=(event:StorageEvent)=>{if(event.key===slot||event.key===null)invalidate();};
  window.addEventListener('drivebridge-context',invalidate);window.addEventListener('storage',storage);
  async function load(){try{const saved=readArchive(slot);setIntent(saved);if(saved){setTarget(saved.body.target_space_id);setPrefix(saved.body.target_prefix);setPhase('unknown');return;}await inspect(abort.signal,null);}catch(error){if(active.current&&!abort.signal.aborted)setPhase([401,403].includes((error as {status:number}).status)?'denied':'error');}}
  void load();return()=>{invalidate();window.removeEventListener('drivebridge-context',invalidate);window.removeEventListener('storage',storage);};
 },[slot]);
 async function perform(mode:'archive'|'observe'|'retry'|'resume'|'correct'){ 
  if(!active.current||running.current)return;
  if(mode==='archive'&&(current.state!=='accepted'||!current.version_id||!spaces.some(space=>space.id===target)))return;
  if(mode==='archive'&&invalidArchivePrefix(prefix)){setMessage('目录格式无效：请使用规范相对目录（最多1024个UTF-8字节），不含空段、点段、反斜杠、控制字符或受保护的暂存目录。未发送归档。');return;}
  running.current=true;setPhase('busy');setMessage('');const abort=new AbortController();controller.current=abort;let original=intent;
  try{
   if(mode==='observe'){await inspect(abort.signal,original);return;}
   if(mode==='correct'){
    // Only an unconditionally invalid prefix is known never to allocate. A
    // negative status read alone is not proof that a valid request failed.
    if(!original||!invalidArchivePrefix(original.body.target_prefix))throw new Error('correction_unavailable');
    const sub=await getSubmission(submission.submission_id,abort.signal);if(!active.current||abort.signal.aborted)return;
    if(!valid(sub)||sub.state!=='accepted'||sub.archive_operation_ref||sub.revision!==original.body.expected_revision||sub.version_id!==original.body.version_id)throw new Error('correction_unavailable');
    await clearArchive(slot,original);if(!active.current||abort.signal.aborted)return;
    setIntent(null);setOperation(null);setCurrent(sub);setObserved(false);setPhase('confirm');setMessage('已核对原请求目录必定被拒绝，且原收件未改变、未关联归档任务。目标意图保留；请修正目录并重新确认，未自动提交。');return;
   }
   if(mode==='resume'){
    // Re-read the owner binding before resuming, never use a renderer-supplied arbitrary ID.
    if(!operation||!['failed','paused','awaiting_upload','verifying'].includes(operation.status))throw new Error('resume_unavailable');
    const sub=await getSubmission(submission.submission_id,abort.signal);if(!active.current||abort.signal.aborted)return;
    if(!valid(sub)||sub.archive_operation_ref!==operation.id)throw new Error('archive_identity_changed');
    await resumeArchiveOperation(operation.id,abort.signal);await inspect(abort.signal,original,operation.id);return;
   }
   if(mode==='archive'){
    const claim=await claimArchive(slot,{expected_revision:current.revision,version_id:current.version_id!,target_space_id:target,target_prefix:prefix},abort.signal);if(!active.current||abort.signal.aborted)return;
    original=claim.intent;setIntent(original);if(!claim.claimed){setPhase('unknown');setObserved(false);return;}
   }
   if(!original||JSON.stringify(readArchive(slot))!==JSON.stringify(original))throw new Error('recovery_identity_changed');
   const result=await archiveSubmission(submission.submission_id,original.body,original.key,abort.signal);if(!active.current||abort.signal.aborted)return;
   if(!result.operation_ref)throw new Error('invalid_archive_response');
   // 202 and even a file_ref alone are not completion evidence.
   await inspect(abort.signal,original,result.operation_ref);
  }catch(error){
   if(!active.current||abort.signal.aborted)return;const status=(error as {status?:number}).status;
   setOperation(null);setObserved(false);
   if(status===401||status===403){setMessage('');setPhase('denied');}
   else if(original||mode==='resume'){setPhase('unknown');setMessage('归档结果尚未确认。只回查此收件的原操作，不自动再分配归档任务。');}
   else{setPhase('error');setMessage('固定来源、目标或恢复存储不可用，未安全登记的归档不会发送。');}
  }finally{running.current=false;}
 }
 async function acknowledge(){if(!active.current||running.current)return;running.current=true;try{if(intent)await clearArchive(slot,intent);if(active.current)close();}catch{if(active.current)setMessage('恢复标识未清除，请稍后确认。');}finally{running.current=false;}}
 return <AppDialog title="归档已接收收件" close={close} className="[&_input]:min-h-11 [&_[role=combobox]]:min-h-11 [&_[data-slot=dialog-close]]:size-11">
  {phase!=='denied'&&<><p className="break-all">收件 {submission.submission_id} · 原版本 {submission.version_id}</p><p>归档将原版本发布到所选目标；不会删除原提交。必须先人工接收，归档与验收分别记账。</p></>}
  {message&&<p role="alert">{message}</p>}
  {(phase==='loading'||phase==='busy')&&<p role="status">正在核对原归档任务…</p>}
  {phase==='confirm'&&(current.state==='accepted'&&current.version_id?<div className="space-y-3"><label className="grid gap-2">归档目标空间<SelectAdapter aria-label="归档目标空间" value={target} onChange={event=>setTarget(event.target.value)}><option value="">选择目标空间</option>{spaces.map(space=><option key={space.id} value={space.id}>{space.name}</option>)}</SelectAdapter></label><label className="grid gap-2">归档目标目录<Input aria-label="归档目标目录" value={prefix} maxLength={1024} onChange={event=>setPrefix(event.target.value)} placeholder="留空表示目标空间根目录"/></label><p>目标：{spaces.find(space=>space.id===target)?.name||'未选择'} / {prefix||'（根目录）'} · 收件版本 {current.revision}</p><Button className="min-h-11" disabled={!target} onClick={()=>void perform('archive')}>确认归档原版本</Button></div>:<p role="status">当前未处于已接收状态，不能新建归档。</p>)}
  {phase==='unknown'&&<div className="space-y-3"><Button variant="outline" className="min-h-11" onClick={()=>void perform('observe')}>查询原归档状态</Button>{observed&&intent&&<><p>原请求键与任务关联尚未确认。按原键重试可能首次执行原目标归档；若原回执已存在，仅返回原任务，不重新启动传输。</p><Button className="min-h-11" onClick={()=>void perform('retry')}>确认按原键重试归档</Button></>}</div>}
  {phase==='invalid-target'&&<div className="space-y-3"><p>原请求目录格式会在分配归档任务前被拒绝，当前原收件仍已接收且未关联任务。修正前将再次核对并只清除此原恢复记录。</p><Button className="min-h-11" onClick={()=>void perform('correct')}>保留目标并修正非法目录</Button></div>}
  {phase==='operation'&&operation&&<div className="space-y-3"><p role="status">原归档任务尚未核实完成：{operation.status}</p><p className="break-all">操作：{operation.id}</p><Button variant="outline" className="min-h-11" onClick={()=>void perform('observe')}>刷新原归档状态</Button>{['failed','paused','awaiting_upload','verifying'].includes(operation.status)&&<Button className="min-h-11" onClick={()=>void perform('resume')}>确认继续原归档任务</Button>}</div>}
  {phase==='done'&&operation&&<div className="space-y-3"><p role="status">归档完成，原操作与结果文件已核对。</p><p className="break-all">原操作：{operation.id} · 结果文件：{operation.file_id} · 发布版本：{operation.version_id}</p><Button className="min-h-11" onClick={()=>void acknowledge()}>确认归档结果并刷新</Button></div>}
  {phase==='denied'&&<p role="alert">来源或目标权限／范围已变化，归档信息已清除。</p>}
  {phase==='error'&&<p role="alert">无法安全读取归档来源或恢复存储。请关闭后重试。</p>}
 </AppDialog>;
}
