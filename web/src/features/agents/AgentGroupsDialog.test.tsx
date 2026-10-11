import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import fixtures from '../../../../contracts/personal-workspace/fixtures/responses.json';
import { AgentGroupsDialog } from './AgentGroupsDialog';
import { claimGroupIntent, clearGroupIntent, groupSlot, readGroupIntent } from './group-recovery';
const group=fixtures.AgentGroupView,slot=groupSlot('alice');
const page={items:[group],next_cursor:'',observed_at:'2026-01-01T00:00:00Z',total:null,total_kind:'unknown'};
const json=(data:unknown,status=200)=>new Response(JSON.stringify(data),{status});
const locks=Object.getOwnPropertyDescriptor(navigator,'locks');
beforeEach(()=>Object.defineProperty(navigator,'locks',{configurable:true,value:{request:vi.fn(async(_:string,options:LockOptions|(()=>unknown),callback?:()=>unknown)=>typeof options==='function'?options():callback!())}}));
afterEach(()=>{cleanup();localStorage.clear();vi.restoreAllMocks();if(locks)Object.defineProperty(navigator,'locks',locks);else Reflect.deleteProperty(navigator,'locks');});
const mount=(close=vi.fn())=>render(<AgentGroupsDialog scope="alice" close={close}/>);
it('creates only on confirmation and acknowledges the scoped original key',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:{...page,items:[]}})).mockResolvedValueOnce(json({data:group}));const close=vi.fn();mount(close);
 fireEvent.click(await screen.findByRole('button',{name:'新建分组'}));fireEvent.change(screen.getByRole('textbox',{name:'分组名称'}),{target:{value:group.name}});expect(fetch).toHaveBeenCalledTimes(1);
 fireEvent.click(screen.getByRole('button',{name:'确认创建分组'}));await screen.findByText(/原请求已被接受/);
 expect(fetch.mock.calls[1][0]).toBe('/api/v1/agent-groups');expect(fetch.mock.calls[1][1]?.method).toBe('POST');expect(JSON.parse(fetch.mock.calls[1][1]?.body as string)).toEqual({name:group.name});expect(fetch.mock.calls[1][1]?.headers).toMatchObject({'Idempotency-Key':readGroupIntent(slot)?.key});
 fireEvent.click(screen.getByRole('button',{name:'确认分组结果并刷新'}));await waitFor(()=>expect(close).toHaveBeenCalledOnce());expect(readGroupIntent(slot)).toBeNull();
});
it.each(['rename','delete'] as const)('binds %s to the selected group and revision',async action=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:page})).mockResolvedValueOnce(json({data:action==='delete'?{group_id:group.group_id,deleted:true}:{...group,name:'New name',revision:2}}));mount();
 fireEvent.click(await screen.findByRole('button',{name:(action==='delete'?'删除 ':'改名 ')+group.name}));
 if(action==='rename')fireEvent.change(screen.getByRole('textbox',{name:'分组名称'}),{target:{value:'New name'}});
 expect(fetch).toHaveBeenCalledTimes(1);fireEvent.click(screen.getByRole('button',{name:action==='delete'?'确认删除分组':'确认修改分组名称'}));await screen.findByRole('button',{name:'确认分组结果并刷新'});
 expect(fetch.mock.calls[1][0]).toBe('/api/v1/agent-groups/'+group.group_id);expect(fetch.mock.calls[1][1]?.method).toBe(action==='delete'?'DELETE':'PATCH');expect(JSON.parse(fetch.mock.calls[1][1]?.body as string)).toEqual(action==='delete'?{expected_revision:1}:{name:'New name',expected_revision:1});
});
it('recovers lost create across remount without duplicate creation before explicit retry',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:page})).mockRejectedValueOnce(new TypeError('lost'));const view=mount();
 fireEvent.click(await screen.findByRole('button',{name:'新建分组'}));fireEvent.change(screen.getByRole('textbox',{name:'分组名称'}),{target:{value:'Fresh'}});fireEvent.click(screen.getByRole('button',{name:'确认创建分组'}));await screen.findByRole('button',{name:'观察当前分组'});
 const original=fetch.mock.calls[1][1];view.unmount();fetch.mockResolvedValueOnce(json({data:page})).mockResolvedValueOnce(json({data:{...group,name:'Later name',revision:4}}));mount();
 expect(screen.queryByRole('button',{name:'确认重试原分组请求'})).toBeNull();fireEvent.click(await screen.findByRole('button',{name:'观察当前分组'}));await screen.findByText(/名称匹配或未找到都不是/);fireEvent.click(screen.getByRole('button',{name:'确认重试原分组请求'}));await screen.findByText(/Later name.*版本 4/);
 const writes=fetch.mock.calls.filter(([,options])=>options?.method==='POST');expect(writes).toHaveLength(2);expect(writes[1][1]?.body).toBe(original?.body);expect(writes[1][1]?.headers).toEqual(original?.headers);
});
it('keeps rename intent on CAS conflict and waits after fresh revision read',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:page})).mockResolvedValueOnce(json({error:{code:'revision_conflict'}},409)).mockResolvedValueOnce(json({data:{...group,name:'Other',revision:3}}));mount();
 fireEvent.click(await screen.findByRole('button',{name:'改名 '+group.name}));fireEvent.change(screen.getByRole('textbox',{name:'分组名称'}),{target:{value:'My intent'}});fireEvent.click(screen.getByRole('button',{name:'确认修改分组名称'}));fireEvent.click(await screen.findByRole('button',{name:'保留意图并重读分组'}));await screen.findByText(/已重读版本/);
 expect((screen.getByRole('textbox',{name:'分组名称'}) as HTMLInputElement).value).toBe('My intent');expect(fetch).toHaveBeenCalledTimes(3);expect(readGroupIntent(slot)).toBeNull();
});
it('paginates only on demand and drops late data after a scope change',async()=>{
 let finish!:(response:Response)=>void;const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:{...page,next_cursor:'opaque+cursor'}})).mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;}));mount();fireEvent.click(await screen.findByRole('button',{name:'加载更多分组'}));await waitFor(()=>expect(finish).toBeDefined());expect(fetch.mock.calls[1][0]).toBe('/api/v1/agent-groups?limit=50&cursor=opaque%2Bcursor');fireEvent(window,new Event('drivebridge-context'));finish(json({data:page}));await screen.findByText(/分组数据已清除/);expect(screen.queryByRole('button',{name:'新建分组'})).toBeNull();
});
it('fails closed without a lock and distinguishes forbidden from an empty group list',async()=>{
 Object.defineProperty(navigator,'locks',{configurable:true,value:undefined});const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:page}));const view=mount();fireEvent.click(await screen.findByRole('button',{name:'新建分组'}));fireEvent.change(screen.getByRole('textbox',{name:'分组名称'}),{target:{value:'Blocked'}});fireEvent.click(screen.getByRole('button',{name:'确认创建分组'}));await screen.findByText(/分组读取或恢复存储不可用/);expect(fetch).toHaveBeenCalledTimes(1);
 view.unmount();fetch.mockResolvedValueOnce(json({},403));mount();await screen.findByText(/分组数据已清除/);expect(screen.queryByRole('button',{name:'新建分组'})).toBeNull();
});
it('serializes actions and validates closed recovery records without secrets',async()=>{
 const first=await claimGroupIntent(slot,{action:'create',body:{name:'A'}},new AbortController().signal);const second=await claimGroupIntent(slot,{action:'delete',id:group.group_id,body:{expected_revision:1}},new AbortController().signal);expect(second).toEqual({intent:first.intent,claimed:false});expect(groupSlot('bob')).not.toBe(slot);
 localStorage.setItem(slot,JSON.stringify({...first.intent,secret:'forbidden'}));expect(()=>readGroupIntent(slot)).toThrow();await expect(clearGroupIntent(slot,first.intent)).rejects.toThrow();const fetch=vi.spyOn(globalThis,'fetch');mount();await screen.findByText(/分组读取或恢复存储不可用/);expect(fetch).not.toHaveBeenCalled();
});
