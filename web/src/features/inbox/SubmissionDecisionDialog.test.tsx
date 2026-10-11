import { cleanup,fireEvent,render,screen,waitFor } from '@testing-library/react';
import { afterEach,beforeEach,expect,it,vi } from 'vitest';
import { SubmissionDecisionDialog } from './SubmissionDecisionDialog';
import { claimDecision,clearDecision,decisionSlot,readDecision } from './decision-recovery';
import { setCSRF } from '../../app/transport';
import type { Submission } from './submission-client';
const sub:Submission={submission_id:'sub',revision:2,space_id:'space',agent_id:'agent',source_kind:'agent',file_ref:'file',version_id:'version-fixed',upload_operation_ref:'upload',state:'submitted',created_at:'2026-01-01T00:00:00Z',updated_at:'2026-01-01T00:00:00Z'};
const decided=(decision:'accept'|'reject',state:Submission['state']=decision==='accept'?'accepted':'rejected'):Submission=>({...sub,state,revision:3,decision:{decision,version_id:sub.version_id!,decided_at:'2026-01-01T01:00:00Z'}});
const slot=decisionSlot('alice','sub'),json=(data:unknown,status=200)=>new Response(JSON.stringify(data),{status});
const locks=Object.getOwnPropertyDescriptor(navigator,'locks');
beforeEach(()=>Object.defineProperty(navigator,'locks',{configurable:true,value:{request:vi.fn(async(_:string,options:LockOptions|(()=>unknown),callback?:()=>unknown)=>typeof options==='function'?options():callback!())}}));
afterEach(()=>{cleanup();localStorage.clear();vi.restoreAllMocks();if(locks)Object.defineProperty(navigator,'locks',locks);else Reflect.deleteProperty(navigator,'locks');});
const mount=(value=sub,close=vi.fn())=>render(<SubmissionDecisionDialog submission={value} scope="alice" close={close}/>);
it.each(['accept','reject'] as const)('confirms %s with fixed revision/version, original key and CSRF',async decision=>{
 setCSRF('fixture-csrf');const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:sub})).mockResolvedValueOnce(json({data:decided(decision)}));const close=vi.fn();mount(sub,close);
 await screen.findByRole('button',{name:'确认接收原版本'});
 if(decision==='reject'){fireEvent.click(screen.getByRole('combobox',{name:'人工验收决定'}));const option=await screen.findByRole('option',{name:'拒绝（保留文件）'});fireEvent.pointerDown(option,{pointerType:'mouse'});fireEvent.click(option);}
 expect(fetch).toHaveBeenCalledTimes(1);fireEvent.click(screen.getByRole('button',{name:decision==='accept'?'确认接收原版本':'确认拒绝原版本'}));await screen.findByText(/原验收请求已被接受/);
 expect(fetch.mock.calls[1][0]).toBe('/api/v1/inbox-submissions/sub/decision');expect(JSON.parse(fetch.mock.calls[1][1]?.body as string)).toEqual({expected_revision:2,version_id:'version-fixed',decision});expect(fetch.mock.calls[1][1]?.headers).toMatchObject({'X-CSRF-Token':'fixture-csrf','Idempotency-Key':readDecision(slot)?.key});
 expect(Object.keys(readDecision(slot)!.body).sort()).toEqual(['decision','expected_revision','version_id']);fireEvent.click(screen.getByRole('button',{name:'确认验收结果并刷新'}));await waitFor(()=>expect(close).toHaveBeenCalledOnce());expect(readDecision(slot)).toBeNull();
});
it('recovers lost response across reload with exact original body and no new key',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:sub})).mockRejectedValueOnce(new TypeError('lost'));const view=mount();fireEvent.click(await screen.findByRole('button',{name:'确认接收原版本'}));await screen.findByRole('button',{name:'读取当前验收状态'});const original=fetch.mock.calls[1][1];view.unmount();
 fetch.mockResolvedValueOnce(json({data:decided('accept','archived')})).mockResolvedValueOnce(json({data:decided('accept','archived')}));mount(decided('accept','archived'));
 expect(screen.queryByRole('button',{name:'确认按原键重试验收'})).toBeNull();fireEvent.click(await screen.findByRole('button',{name:'读取当前验收状态'}));await screen.findByText(/这是当前观察，不是原请求回执/);fireEvent.click(screen.getByRole('button',{name:'确认按原键重试验收'}));await screen.findByText(/原验收请求已被接受/);
 const writes=fetch.mock.calls.filter(([,options])=>options?.method==='POST');expect(writes).toHaveLength(2);expect(writes[1][1]?.body).toBe(original?.body);expect(writes[1][1]?.headers).toEqual(original?.headers);
});
it('never silently accepts a changed fixed version after conflict',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:sub})).mockResolvedValueOnce(json({error:{code:'version_conflict'}},409)).mockResolvedValueOnce(json({data:{...sub,version_id:'other-version',revision:4}}));mount();fireEvent.click(await screen.findByRole('button',{name:'确认接收原版本'}));fireEvent.click(await screen.findByRole('button',{name:'保留决定并重读原版本'}));await screen.findByText(/结果未知/);expect(readDecision(slot)?.body.version_id).toBe('version-fixed');expect(fetch.mock.calls.filter(([,options])=>options?.method==='POST')).toHaveLength(1);
});
it('does not allow a new decision on terminal states',async()=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValue(json({data:decided('reject')}));mount(decided('reject'));await screen.findByText(/当前不是待验收状态/);expect(screen.queryByRole('button',{name:'确认接收原版本'})).toBeNull();
});
it('clears result on revocation and rejects a late decision response',async()=>{
 let finish!:(response:Response)=>void;vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:sub})).mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;}));mount();fireEvent.click(await screen.findByRole('button',{name:'确认接收原版本'}));await waitFor(()=>expect(finish).toBeDefined());fireEvent(window,new Event('drivebridge-context'));finish(json({data:decided('accept')}));await screen.findByText(/验收操作已停止/);expect(screen.queryByText(/原验收请求已被接受/)).toBeNull();expect(readDecision(slot)).toBeTruthy();
});
it('refuses missing locks and corrupt recovery, and prevents replacing another pending request',async()=>{
 const first=await claimDecision(slot,{expected_revision:2,version_id:'version-fixed',decision:'accept'},new AbortController().signal);const second=await claimDecision(slot,{expected_revision:2,version_id:'version-fixed',decision:'reject'},new AbortController().signal);expect(second).toEqual({intent:first.intent,claimed:false});expect(decisionSlot('bob','sub')).not.toBe(slot);expect(decisionSlot('alice','other')).not.toBe(slot);
 localStorage.setItem(slot,JSON.stringify({...first.intent,body:{...first.intent.body,reason:'must not persist'}}));expect(()=>readDecision(slot)).toThrow();await expect(clearDecision(slot,first.intent)).rejects.toThrow();localStorage.clear();Object.defineProperty(navigator,'locks',{configurable:true,value:undefined});const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:sub}));mount();fireEvent.click(await screen.findByRole('button',{name:'确认接收原版本'}));await screen.findByText(/不能安全提交决定/);expect(fetch).toHaveBeenCalledTimes(1);
});
