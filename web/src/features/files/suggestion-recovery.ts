import type { SuggestionDecision } from './suggestion-client';
export type SuggestionIntent={key:string;suggestion:string;body:SuggestionDecision};
export const suggestionSlot=(scope:string,file:string,version:string)=>'drivebridge:suggestion:'+JSON.stringify([scope,file,version]);
const exact=(value:unknown,keys:string[])=>typeof value==='object'&&value!==null&&!Array.isArray(value)&&Object.keys(value).sort().join(',')===keys.sort().join(',');
export function readSuggestionIntent(slot:string):SuggestionIntent|null{
 const raw=localStorage.getItem(slot);if(raw===null)return null;if(raw.length>40000)throw new Error('recovery_identity_invalid');
 let v:SuggestionIntent;try{v=JSON.parse(raw) as SuggestionIntent;}catch{throw new Error('recovery_identity_invalid');}
 if(!exact(v,['key','suggestion','body'])||!/^suggestion-[a-f0-9]{48}$/.test(v.key)||typeof v.suggestion!=='string'||!v.suggestion||v.suggestion.length>512||!exact(v.body,['expected_revision','base_version_id','decision','labels']))throw new Error('recovery_identity_invalid');
 const b=v.body;
 if(!Number.isSafeInteger(b.expected_revision)||b.expected_revision<1||typeof b.base_version_id!=='string'||!b.base_version_id||b.base_version_id.length>512||!['accept','reject'].includes(b.decision)||!Array.isArray(b.labels)||b.labels.length>100||new Set(b.labels).size!==b.labels.length||b.labels.some(label=>typeof label!=='string'||!label.trim()||[...label].length>64)||(b.decision==='accept'?b.labels.length===0:b.labels.length!==0))throw new Error('recovery_identity_invalid');
 return v;
}
export async function claimSuggestionIntent(slot:string,suggestion:string,body:SuggestionDecision,signal:AbortSignal){
 if(!navigator.locks?.request)throw new Error('recovery_lock_unavailable');
 return navigator.locks.request(slot,{signal},()=>{signal.throwIfAborted();const existing=readSuggestionIntent(slot);if(existing)return{intent:existing,claimed:false};const key='suggestion-'+Array.from(crypto.getRandomValues(new Uint8Array(24)),b=>b.toString(16).padStart(2,'0')).join('');const intent={key,suggestion,body};localStorage.setItem(slot,JSON.stringify(intent));return{intent,claimed:true};});
}
export async function clearSuggestionIntent(slot:string,intent:SuggestionIntent){
 if(!navigator.locks?.request)throw new Error('recovery_lock_unavailable');
 return navigator.locks.request(slot,()=>{if(JSON.stringify(readSuggestionIntent(slot))!==JSON.stringify(intent))throw new Error('recovery_identity_changed');localStorage.removeItem(slot);});
}
