import type { SubmissionArchive } from './submission-client';
export type ArchiveIntent = { key: string; body: SubmissionArchive };
export const archiveSlot = (scope: string, id: string) => 'drivebridge:submission-archive:' + JSON.stringify([scope,id]);
const exact = (value: unknown, keys: string[]) => typeof value === 'object' && value !== null && !Array.isArray(value) && Object.keys(value).sort().join(',') === keys.sort().join(',');
export function readArchive(slot: string): ArchiveIntent | null {
 const raw=localStorage.getItem(slot);if(raw===null)return null;if(raw.length>8192)throw new Error('recovery_identity_invalid');
 let intent:ArchiveIntent;try{intent=JSON.parse(raw) as ArchiveIntent;}catch{throw new Error('recovery_identity_invalid');}
 if(!exact(intent,['key','body'])||!/^archive-[a-f0-9]{48}$/.test(intent.key)||!exact(intent.body,['expected_revision','version_id','target_space_id','target_prefix']))throw new Error('recovery_identity_invalid');
 const b=intent.body;
 if(!Number.isSafeInteger(b.expected_revision)||b.expected_revision<1||typeof b.version_id!=='string'||!b.version_id||b.version_id.length>512||typeof b.target_space_id!=='string'||!b.target_space_id||b.target_space_id.length>512||typeof b.target_prefix!=='string'||b.target_prefix.length>1024)throw new Error('recovery_identity_invalid');
 return intent;
}
export async function claimArchive(slot: string, body: SubmissionArchive, signal: AbortSignal){
 if(!navigator.locks?.request)throw new Error('recovery_lock_unavailable');
 return navigator.locks.request(slot,{signal},()=>{
  signal.throwIfAborted();const existing=readArchive(slot);if(existing)return{intent:existing,claimed:false};
  const key='archive-'+Array.from(crypto.getRandomValues(new Uint8Array(24)),b=>b.toString(16).padStart(2,'0')).join('');
  const intent={key,body};localStorage.setItem(slot,JSON.stringify(intent));return{intent,claimed:true};
 });
}
export async function clearArchive(slot: string,intent: ArchiveIntent){
 if(!navigator.locks?.request)throw new Error('recovery_lock_unavailable');
 return navigator.locks.request(slot,()=>{if(JSON.stringify(readArchive(slot))!==JSON.stringify(intent))throw new Error('recovery_identity_changed');localStorage.removeItem(slot);});
}
