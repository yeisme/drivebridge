import { api } from '../../api';
import type { components } from '../../generated/personal-workspace';
type Schemas=components['schemas'];
export type Suggestion=Schemas['LabelSuggestionEnvelope']['data'];
export type SuggestionDecision=Schemas['SuggestionDecisionRequest'];
export const listSuggestions=(file:string,version:string,cursor:string,signal:AbortSignal)=>{
 const query=new URLSearchParams({version_id:version,limit:'50'});if(cursor)query.set('cursor',cursor);
 return api<Schemas['SuggestionPageEnvelope']['data']>(`/files/${encodeURIComponent(file)}/label-suggestions?${query}`,'GET',undefined,{},signal);
};
// Labels are current file-level facts, not a version snapshot or a decision receipt.
export async function readManualLabels(file:string,signal:AbortSignal):Promise<string[]>{
 const result=await api<{file_id:string;namespaces:Record<string,unknown>}>(`/files/${encodeURIComponent(file)}/labels`,'GET',undefined,{},signal);
 if(result?.file_id!==file||!result.namespaces||typeof result.namespaces!=='object'||Array.isArray(result.namespaces))throw new Error('invalid_contract');
 const manual=result.namespaces.manual;
 if(manual===undefined)return [];
 if(!Array.isArray(manual)||!manual.every(label=>typeof label==='string'))throw new Error('invalid_contract');
 return manual;
}
export const decideSuggestion=(id:string,body:SuggestionDecision,key:string,signal:AbortSignal)=>api<Suggestion>(`/label-suggestions/${encodeURIComponent(id)}/decision`,'POST',body,{'Idempotency-Key':key},signal);
