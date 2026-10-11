import type { Submission } from './submission-client';
const states=['uploading','submitted','accepted','rejected','archiving','archived'];
export function submissionRoute(hash:string):{space:string;state:Submission['state']|'';valid:boolean}{
 try{
  const url=new URL(hash.startsWith('#')?hash.slice(1):hash,'http://workspace.invalid');
  if(url.pathname!=='/inbox')return {space:'',state:'',valid:true};
  const spaces=url.searchParams.getAll('submission_space'),filters=url.searchParams.getAll('submission_state');
  const space=spaces[0]||'',state=filters[0]||'';
  if(spaces.length>1||filters.length>1||(spaces.length&&(!space||space.length>512||/[\x00-\x20]/.test(space)))||(filters.length&&state!==''&&!states.includes(state)))return {space:'',state:'',valid:false};
  return {space,state:state as Submission['state']|'',valid:true};
 }catch{return {space:'',state:'',valid:false};}
}
// These are list filters, never an authorization scope header.
export function submissionURL(space:string,state:Submission['state']|'',tenant='',scope=''){
 const query=new URLSearchParams();if(tenant)query.set('tenant',tenant);if(scope)query.set('scope',scope);if(space)query.set('submission_space',space);if(state)query.set('submission_state',state);
 return '#/inbox'+(query.size?'?'+query:'');
}
