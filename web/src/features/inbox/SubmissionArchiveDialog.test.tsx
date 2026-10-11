import { cleanup,fireEvent,render,screen,waitFor } from '@testing-library/react';
import { afterEach,beforeEach,expect,it,vi } from 'vitest';
import { SubmissionArchiveDialog } from './SubmissionArchiveDialog';
import { archiveSlot,claimArchive,readArchive } from './archive-recovery';
import { invalidArchivePrefix } from './archive-target';
import type { Submission } from './submission-client';
import type { Operation,Space } from '../../api';
const sub:Submission={submission_id:'sub',revision:3,space_id:'space',source_kind:'agent',file_ref:'source',version_id:'source-v',upload_operation_ref:'upload',state:'accepted',created_at:'2026-01-01T00:00:00Z',updated_at:'2026-01-01T00:00:00Z'};
const space:Space={id:'space',name:'Target',connection_id:'platform',used_bytes:0,reserved_bytes:0,max_bytes:100};
const archived:Submission={...sub,state:'archived',revision:5,archive_operation_ref:'original-op',archived_file_ref:'result'};
const operation:Operation={id:'original-op',kind:'archive',source_id:'source',source_version:'source-v',space_id:'space',path:'archive/result.md',size_bytes:1,sha256:'fixture',status:'completed',bytes_transferred:1,file_id:'result',version_id:'result-v'};
const slot=archiveSlot('alice','sub');
const json=(data:unknown,status=200)=>new Response(JSON.stringify(data),{status});
const locks=Object.getOwnPropertyDescriptor(navigator,'locks');
beforeEach(()=>Object.defineProperty(navigator,'locks',{configurable:true,value:{request:vi.fn(async(_:string,options:LockOptions|(()=>unknown),callback?:()=>unknown)=>typeof options==='function'?options():callback!())}}));
afterEach(()=>{cleanup();localStorage.clear();vi.restoreAllMocks();if(locks)Object.defineProperty(navigator,'locks',locks);else Reflect.deleteProperty(navigator,'locks');});
const mount=(value=sub,close=vi.fn())=>render(<SubmissionArchiveDialog submission={value} scope="alice" spaces={[space]} close={close}/>);
function completed(fetch:ReturnType<typeof vi.spyOn>){return fetch.mockResolvedValueOnce(json({data:archived})).mockResolvedValueOnce(json({data:{operation}})).mockResolvedValueOnce(json({data:{file:{id:'result',space_id:'space',version_id:'result-v',trashed:false},ref:'fixture-result-ref'}}));}
it('archives the fixed version only after confirmation and proves completion through original operation and file',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:sub})).mockResolvedValueOnce(json({data:{operation_ref:'original-op',file_ref:'result'}},202));completed(fetch);const close=vi.fn();mount(sub,close);
 fireEvent.change(await screen.findByRole('textbox',{name:'归档目标目录'}),{target:{value:'archive'}});expect(fetch).toHaveBeenCalledTimes(1);fireEvent.click(screen.getByRole('button',{name:'确认归档原版本'}));await screen.findByText('归档完成，原操作与结果文件已核对。');
 expect(fetch.mock.calls[1][0]).toBe('/api/v1/inbox-submissions/sub/archive');expect(JSON.parse(fetch.mock.calls[1][1]?.body as string)).toEqual({expected_revision:3,version_id:'source-v',target_space_id:'space',target_prefix:'archive'});expect(fetch.mock.calls[1][1]?.headers).toMatchObject({'Idempotency-Key':readArchive(slot)?.key});
 fireEvent.click(screen.getByRole('button',{name:'确认归档结果并刷新'}));await waitFor(()=>expect(close).toHaveBeenCalledOnce());expect(readArchive(slot)).toBeNull();
});
it('202 with file_ref is not completion if the original operation is still pending',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:sub})).mockResolvedValueOnce(json({data:{operation_ref:'original-op',file_ref:'result'}},202)).mockResolvedValueOnce(json({data:{...sub,state:'archiving',archive_operation_ref:'original-op'}})).mockResolvedValueOnce(json({data:{operation:{...operation,status:'running'}}}));mount();fireEvent.click(await screen.findByRole('button',{name:'确认归档原版本'}));await screen.findByText('原归档任务尚未核实完成：running');expect(screen.queryByText('归档完成，原操作与结果文件已核对。')).toBeNull();expect(screen.queryByRole('button',{name:'确认继续原归档任务'})).toBeNull();expect(fetch).toHaveBeenCalledTimes(4);
});
it('restores a lost archive with exact original key and body, not the latest revision',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:sub})).mockRejectedValueOnce(new TypeError('lost'));const view=mount();fireEvent.click(await screen.findByRole('button',{name:'确认归档原版本'}));await screen.findByRole('button',{name:'查询原归档状态'});const original=fetch.mock.calls[1][1];view.unmount();
 fetch.mockResolvedValueOnce(json({data:archived})).mockResolvedValueOnce(json({data:{operation_ref:'original-op',file_ref:'result'}},202));completed(fetch);mount(archived);
 fireEvent.click(await screen.findByRole('button',{name:'查询原归档状态'}));await screen.findByText(/当前状态不能证明它属于原请求键/);expect(screen.queryByText('归档完成，原操作与结果文件已核对。')).toBeNull();
 fireEvent.click(screen.getByRole('button',{name:'确认按原键重试归档'}));await screen.findByText('归档完成，原操作与结果文件已核对。');const writes=fetch.mock.calls.filter(([,options])=>options?.method==='POST');expect(writes).toHaveLength(2);expect(writes[1][1]?.body).toBe(original?.body);expect(writes[1][1]?.headers).toEqual(original?.headers);
});
it('resumes only the existing bound operation and rechecks completion, with no second archive allocation',async()=>{
 const pending:Submission={...sub,state:'archiving',archive_operation_ref:'original-op'};
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:pending})).mockResolvedValueOnce(json({data:{operation:{...operation,status:'failed'}}})).mockResolvedValueOnce(json({data:pending})).mockResolvedValueOnce(json({data:operation}));completed(fetch);mount(pending);
 fireEvent.click(await screen.findByRole('button',{name:'确认继续原归档任务'}));await screen.findByText('归档完成，原操作与结果文件已核对。');const writes=fetch.mock.calls.filter(([,options])=>options?.method==='POST');expect(writes).toHaveLength(1);expect(writes[0][0]).toBe('/api/v1/operations/original-op/resume');
});
it('rejects a mismatched original operation source instead of resuming it',async()=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:archived})).mockResolvedValueOnce(json({data:{operation:{...operation,source_version:'other'}}}));mount(archived);await screen.findByText(/无法安全读取归档来源/);expect(screen.queryByRole('button',{name:'确认继续原归档任务'})).toBeNull();expect(screen.queryByText('归档完成，原操作与结果文件已核对。')).toBeNull();
});
it('clears data on scope loss and ignores a late archive response',async()=>{
 let finish!:(response:Response)=>void;const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:sub})).mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;}));mount();fireEvent.click(await screen.findByRole('button',{name:'确认归档原版本'}));await waitFor(()=>expect(finish).toBeDefined());fireEvent(window,new Event('drivebridge-context'));finish(json({data:{operation_ref:'original-op',file_ref:'result'}},202));await screen.findByText(/归档信息已清除/);expect(fetch).toHaveBeenCalledTimes(2);expect(readArchive(slot)).toBeTruthy();
});
it.each(['/absolute','a//b','a/./b','a/../b','a/','a\\b','a\u0001b','refs/.inbox-staging','界'.repeat(342)])('rejects invalid prefix before claiming or sending: %s',async prefix=>{
 expect(invalidArchivePrefix(prefix)).toBe(true);
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:sub}));mount();
 fireEvent.change(await screen.findByRole('textbox',{name:'归档目标目录'}),{target:{value:prefix}});fireEvent.click(screen.getByRole('button',{name:'确认归档原版本'}));
 await screen.findByText(/目录格式无效/);expect(readArchive(slot)).toBeNull();expect(fetch).toHaveBeenCalledTimes(1);
});
it.each(['','reviewed','a b',' .hidden','界'.repeat(341)])('permits owner-valid prefix syntax without silently normalizing: %s',prefix=>{expect(invalidArchivePrefix(prefix)).toBe(false);});
it('corrects an old invalid prefix only after rechecking the same unallocated source',async()=>{
 const saved=await claimArchive(slot,{expected_revision:3,version_id:'source-v',target_space_id:'space',target_prefix:'a/../b'},new AbortController().signal);
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:sub})).mockResolvedValueOnce(json({data:sub}));mount();
 fireEvent.click(await screen.findByRole('button',{name:'查询原归档状态'}));fireEvent.click(await screen.findByRole('button',{name:'保留目标并修正非法目录'}));
 await screen.findByText(/目标意图保留/);expect((screen.getByRole('textbox',{name:'归档目标目录'}) as HTMLInputElement).value).toBe(saved.intent.body.target_prefix);expect(readArchive(slot)).toBeNull();expect(fetch.mock.calls.every(([,options])=>options?.method==='GET')).toBe(true);
});
it.each(['changed-revision','allocated','changed-slot'] as const)('does not clear an invalid target when recheck reports %s',async kind=>{
 const saved=await claimArchive(slot,{expected_revision:3,version_id:'source-v',target_space_id:'space',target_prefix:'a/../b'},new AbortController().signal);
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:sub})).mockResolvedValueOnce(json({data:kind==='allocated'?{...sub,state:'archiving',archive_operation_ref:'other-op'}:kind==='changed-revision'?{...sub,revision:4}:sub}));mount();
 fireEvent.click(await screen.findByRole('button',{name:'查询原归档状态'}));await screen.findByRole('button',{name:'保留目标并修正非法目录'});
 if(kind==='changed-slot')localStorage.setItem(slot,JSON.stringify({...saved.intent,key:'archive-'+'b'.repeat(48)}));
 fireEvent.click(screen.getByRole('button',{name:'保留目标并修正非法目录'}));await screen.findByText(/归档结果尚未确认/);expect(readArchive(slot)).not.toBeNull();expect(fetch).toHaveBeenCalledTimes(2);expect(screen.queryByRole('textbox',{name:'归档目标目录'})).toBeNull();
});
it('a valid-target network unknown and absent operation never permit clearing the request',async()=>{
 await claimArchive(slot,{expected_revision:3,version_id:'source-v',target_space_id:'space',target_prefix:'valid'},new AbortController().signal);
 vi.spyOn(globalThis,'fetch').mockResolvedValue(json({data:sub}));mount();fireEvent.click(await screen.findByRole('button',{name:'查询原归档状态'}));await screen.findByRole('button',{name:'确认按原键重试归档'});expect(screen.queryByRole('button',{name:'保留目标并修正非法目录'})).toBeNull();expect(readArchive(slot)).toBeTruthy();
});
it('fails closed without locks and refuses secrets in recovery data',async()=>{
 Object.defineProperty(navigator,'locks',{configurable:true,value:undefined});const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:sub}));const view=mount();fireEvent.click(await screen.findByRole('button',{name:'确认归档原版本'}));await screen.findByText(/无法安全读取归档来源/);expect(fetch).toHaveBeenCalledTimes(1);view.unmount();
 localStorage.setItem(slot,JSON.stringify({key:'archive-'+'a'.repeat(48),body:{expected_revision:3,version_id:'source-v',target_space_id:'space',target_prefix:'archive',secret:'forbidden'}}));expect(()=>readArchive(slot)).toThrow();expect(archiveSlot('bob','sub')).not.toBe(slot);expect(archiveSlot('alice','other')).not.toBe(slot);
 await expect(claimArchive(slot,{expected_revision:3,version_id:'source-v',target_space_id:'space',target_prefix:''},new AbortController().signal)).rejects.toThrow();
});
