import { useEffect,useRef,useState } from 'react';
import { NotebookReadError,readMarkdownFile } from '../notebook/notebook-adapters';

// Explicit, read-only ordinary-file consumption. Never HTML/Markdown embeds,
// provider URLs, Pinax identifiers, or persisted body recovery.
export function HomeFileText({space,file,version,onDenied,label="首页原版本纯文本"}:{space:string;file:string;version:string;onDenied:()=>void;label?:string}){
 const [text,setText]=useState(''),[state,setState]=useState<'loading'|'ready'|'unavailable'|'denied'|'offline'>('loading');
 const denied=useRef(onDenied);denied.current=onDenied;
 useEffect(()=>{
  const controller=new AbortController();setText('');setState('loading');
  const invalidate=()=>{controller.abort();setText('');setState('denied');};
  const offline=()=>{controller.abort();setText('');setState('offline');};
  window.addEventListener('drivebridge-context',invalidate);window.addEventListener('offline',offline);
  if(!navigator.onLine)offline();else void readMarkdownFile({sourceId:space,objectId:file,revision:version},controller.signal).then(body=>{
   if(controller.signal.aborted)return;if(body.includes('\0'))throw new Error('binary_content');setText(body);setState('ready');
  }).catch(error=>{
   if(controller.signal.aborted)return;setText('');
   if(error instanceof NotebookReadError&&error.state==='forbidden'){setState('denied');denied.current();}else setState('unavailable');
  });
  return()=>{controller.abort();window.removeEventListener('drivebridge-context',invalidate);window.removeEventListener('offline',offline);};
 },[space,file,version]);
 return <div className="min-w-0 space-y-2">
  <p className="text-xs text-muted-foreground">只读原版本 UTF-8 纯文本，最多 2 MiB。HTML、脚本与嵌入内容不会执行；正文仅保留在当前页面内存。</p>
  {state==='loading'&&<p role="status">读取固定版本正文…</p>}
  {state==='ready'&&<pre aria-label={label} tabIndex={0} className="max-h-[55dvh] overflow-auto whitespace-pre-wrap break-words rounded border border-border p-3 text-sm">{text||'（空文件）'}</pre>}
  {state==='unavailable'&&<p role="alert">此固定版本不可用、非 UTF-8 文本或超过读取预算。未改读最新版本；可关闭正文后重试原版本。</p>}
  {state==='denied'&&<p role="alert">读取权限或范围已变化，正文已清除。</p>}
  {state==='offline'&&<p role="alert">当前离线，正文已清除；联网不会自动重读。</p>}
 </div>;
}
