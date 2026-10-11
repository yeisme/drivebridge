import type { SubmissionDecision } from './submission-client';
export type DecisionIntent = { key: string; body: Omit<SubmissionDecision, 'reason'> };
export const decisionSlot = (scope: string, id: string) => 'drivebridge:submission-decision:' + JSON.stringify([scope,id]);
const exact = (value: unknown, keys: string[]) => typeof value === 'object' && value !== null && !Array.isArray(value) && Object.keys(value).sort().join(',') === keys.sort().join(',');
export function readDecision(slot: string): DecisionIntent | null {
  const raw=localStorage.getItem(slot);if(raw===null)return null;
  if(raw.length>2048)throw new Error('recovery_identity_invalid');
  let intent:DecisionIntent;try{intent=JSON.parse(raw) as DecisionIntent;}catch{throw new Error('recovery_identity_invalid');}
  if(!exact(intent,['key','body'])||!/^decision-[a-f0-9]{48}$/.test(intent.key)||!exact(intent.body,['expected_revision','version_id','decision']))throw new Error('recovery_identity_invalid');
  const body=intent.body;
  if(!Number.isSafeInteger(body.expected_revision)||body.expected_revision<1||typeof body.version_id!=='string'||!body.version_id||body.version_id.length>512||!['accept','reject'].includes(body.decision))throw new Error('recovery_identity_invalid');
  return intent;
}
export async function claimDecision(slot: string, body: DecisionIntent['body'], signal: AbortSignal) {
  if(!navigator.locks?.request)throw new Error('recovery_lock_unavailable');
  return navigator.locks.request(slot,{signal},()=>{
    signal.throwIfAborted();const existing=readDecision(slot);if(existing)return{intent:existing,claimed:false};
    const key='decision-'+Array.from(crypto.getRandomValues(new Uint8Array(24)),b=>b.toString(16).padStart(2,'0')).join('');
    const intent={key,body};localStorage.setItem(slot,JSON.stringify(intent));return{intent,claimed:true};
  });
}
export async function clearDecision(slot: string, intent: DecisionIntent) {
  if(!navigator.locks?.request)throw new Error('recovery_lock_unavailable');
  return navigator.locks.request(slot,()=>{
    if(JSON.stringify(readDecision(slot))!==JSON.stringify(intent))throw new Error('recovery_identity_changed');
    localStorage.removeItem(slot);
  });
}
