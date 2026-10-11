import { Input } from "./components/ui/input";
import { Button } from "./components/ui/button";
import { SelectField } from "./components/SelectField";
import React, {useState} from "react";
import type {Operation} from "./api";
import {t, statusLabel} from "./i18n";
const historyStatuses = ["all","active","completed","failed","interrupted","cancelled"] as const;
type HistoryStatus = typeof historyStatuses[number];
export const historyStatusKey = (scope: string) => scope ? `drivebridge:history-status:${scope}` : "";
export function readHistoryStatus(scope: string): HistoryStatus {
 try { const value = scope ? window.localStorage.getItem(historyStatusKey(scope)) : null;
  return historyStatuses.find(status => status === value) ?? "all";
 } catch { return "all"; }
}
export const isFinished = (op: Operation) => ["completed","cancelled"].includes(op.status);
export function filterHistory(ops: Operation[], query: string, status: string, source: string, hidden: string[]) {
 return ops.filter(op=>!hidden.includes(op.id) && (status==="all" || (status==="active" ? !isFinished(op) : op.status===status)) && (!source || op.caller===source) && `${op.path} ${op.caller||""} ${op.task_ref||""}`.toLowerCase().includes(query.toLowerCase()));
}
type HistoryProps = {ops:Operation[];scope:string;children:(ops:Operation[])=>React.ReactNode};
export function History(props: HistoryProps) {
 return <ScopedHistory key={props.scope} {...props} />;
}
function ScopedHistory({ops,scope,children}: HistoryProps) {
 const key=`drivebridge:history:${scope}`;
 const [hidden,setHidden]=useState<string[]>(()=>{try {const v=JSON.parse((scope ? window.localStorage.getItem(key) : null)||"[]");return Array.isArray(v)?v.filter(x=>typeof x==="string"):[];}catch{return [];}});
 const [query,setQuery]=useState(""),[status,setStatus]=useState<HistoryStatus>(()=>readHistoryStatus(scope)),[source,setSource]=useState(""),[confirm,setConfirm]=useState(false),[undo,setUndo]=useState<string[]>([]),[error,setError]=useState("");
 const changeStatus=(value:string)=>{
  const valid=historyStatuses.find(status=>status===value);if(!valid)return;
  setStatus(valid);setConfirm(false);
  try{if(scope)window.localStorage.setItem(historyStatusKey(scope),valid);}catch{/* Display filtering still works in memory. */}
 };
 const visible=filterHistory(ops,query,status,source,hidden), completed=visible.filter(isFinished);
 const save=(next:string[])=>{try{if(scope)window.localStorage.setItem(key,JSON.stringify(next));setHidden(next);setError("");return true;}catch{setError(t("history.storageFailed"));return false;}};
 return <><div className="history-toolbar"><Input type="search" aria-label={t("history.search")} placeholder={t("history.search")} value={query} onChange={e=>setQuery(e.target.value)}/><SelectField aria-label={t("history.status")} value={status} onValueChange={changeStatus} options={historyStatuses.map(value=>({value,label:statusLabel(value)}))}/><SelectField aria-label={t("history.source")} value={source} onValueChange={setSource} options={[{value:"",label:t("history.allSources")}, ...[...new Set(ops.map(o=>o.caller).filter((c): c is string=>!!c))].map(value=>({value,label:value}))]}/><span>{visible.length} {t("history.records")}</span><Button variant="outline" size="sm" type="submit"  disabled={!completed.length} onClick={()=>setConfirm(true)}>{t("history.clear")}</Button>{hidden.length>0 && <Button variant="ghost" type="submit" className="text-button" onClick={()=>save([])}>{t("history.restore")}</Button>}</div>
 <p className="history-hint">{t("history.scope")}</p>
 {confirm && <div className="history-confirm" role="group" aria-label={t("history.confirm")}><span>{t("history.confirm")}（{completed.length} {t("history.records")}）</span><Button variant="outline" size="sm" type="submit"  onClick={()=>{const ids=completed.map(o=>o.id);if(save([...new Set([...hidden,...ids])])){setUndo(ids);setConfirm(false);}}}>{t("history.confirmButton")}</Button><Button variant="ghost" type="submit" className="text-button" onClick={()=>setConfirm(false)}>{t("cancel")}</Button></div>}
 {!!undo.length && <p role="status">{t("history.cleared")} <Button variant="ghost" type="submit" className="text-button" onClick={()=>{if(save(hidden.filter(id=>!undo.includes(id))))setUndo([]);}}>{t("undo")}</Button></p>}
 {error&&<p role="alert">{error}</p>}
 {visible.length===0 && ops.length>0?<div className="empty"><h3>{t("history.empty")}</h3><p>{t("history.adjust")}</p></div>:children(visible)}
 </>;
}
