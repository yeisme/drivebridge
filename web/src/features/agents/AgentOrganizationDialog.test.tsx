import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import fixtures from '../../../../contracts/personal-workspace/fixtures/responses.json';
import { AgentOrganizationDialog } from './AgentOrganizationDialog';
import { claimOrganizationIntent, clearOrganizationIntent, organizationSlot, readOrganizationIntent } from './organization-recovery';
import type { Grant } from './grant-client';
import type { AgentOrganization } from './organization-client';
const grant=fixtures.GrantView as Grant;
const org:AgentOrganization={...fixtures.AgentOrganizationView,agent_id:grant.agent_id,group_id:null,revision:0,pinned:false};
const group=fixtures.AgentGroupView;
const page={items:[group],next_cursor:'',observed_at:'2026-01-01T00:00:00Z',total:null,total_kind:'unknown'};
const slot=organizationSlot('alice',grant.agent_id);
const json=(data:unknown,status=200)=>new Response(JSON.stringify(data),{status});
const locks=Object.getOwnPropertyDescriptor(navigator,'locks');
beforeEach(()=>Object.defineProperty(navigator,'locks',{configurable:true,value:{request:vi.fn(async(_:string,options:LockOptions|(()=>unknown),callback?:()=>unknown)=>typeof options==='function'?options():callback!())}}));
afterEach(()=>{cleanup();localStorage.clear();vi.restoreAllMocks();if(locks)Object.defineProperty(navigator,'locks',locks);else Reflect.deleteProperty(navigator,'locks');});
const mount=(close=vi.fn())=>render(<AgentOrganizationDialog grant={grant} scope="alice" close={close}/>);
function mockLoad(value=org){return vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:value})).mockResolvedValueOnce(json({data:page}));}
async function choose(name:string){fireEvent.click(await screen.findByRole('combobox',{name:'所属分组'}));const option=await screen.findByRole('option',{name});fireEvent.pointerDown(option,{pointerType:'mouse'});fireEvent.click(option);}
it('assigns group and pin using organization revision and no permission payload',async()=>{
 const fetch=mockLoad().mockResolvedValueOnce(json({data:{...org,group_id:group.group_id,pinned:true,revision:1}}));const close=vi.fn();mount(close);
 await choose(group.name);fireEvent.click(screen.getByRole('checkbox',{name:'在个人组织中置顶'}));fireEvent.click(screen.getByRole('button',{name:'保存分组与置顶'}));
 await screen.findByText('原请求已被服务端接受；返回当前组织版本 1。');
 const [url,request]=fetch.mock.calls[2];expect(url).toBe('/api/v1/agent-organizations/'+grant.agent_id);expect(request?.method).toBe('PATCH');
 expect(JSON.parse(request?.body as string)).toEqual({expected_revision:0,group_id:group.group_id,pinned:true});
 expect(request?.headers).toMatchObject({'Idempotency-Key':readOrganizationIntent(slot)?.key});
 fireEvent.click(screen.getByRole('button',{name:'确认组织结果并刷新'}));await waitFor(()=>expect(close).toHaveBeenCalledOnce());expect(localStorage.getItem(slot)).toBeNull();
});
it('removing membership sends explicit null but preserves pin by omitting it',async()=>{
 const fetch=mockLoad({...org,group_id:group.group_id,revision:4,pinned:true}).mockResolvedValueOnce(json({data:{...org,revision:5,pinned:true}}));mount();await choose('未分组');fireEvent.click(screen.getByRole('button',{name:'保存分组与置顶'}));
 await screen.findByText('原请求已被服务端接受；返回当前组织版本 5。');expect(JSON.parse(fetch.mock.calls[2][1]?.body as string)).toEqual({expected_revision:4,group_id:null});
});
it('loads the next opaque group page without automatically fetching all groups',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:org})).mockResolvedValueOnce(json({data:{...page,items:[],next_cursor:'next+page'}})).mockResolvedValueOnce(json({data:page}));mount();
 fireEvent.click(await screen.findByRole('button',{name:'加载更多分组'}));await screen.findByRole('button',{name:'保存分组与置顶'});expect(fetch).toHaveBeenCalledTimes(3);expect(fetch.mock.calls[2][0]).toBe('/api/v1/agent-groups?limit=50&cursor=next%2Bpage');await choose(group.name);
});
it('restores exact original request after refresh, first observes then explicitly retries',async()=>{
 const fetch=mockLoad().mockRejectedValueOnce(new TypeError('lost'));const view=mount();
 fireEvent.click(await screen.findByRole('checkbox',{name:'在个人组织中置顶'}));fireEvent.click(screen.getByRole('button',{name:'保存分组与置顶'}));await screen.findByRole('button',{name:'读取当前组织状态'});
 const original=fetch.mock.calls[2][1];view.unmount();
 fetch.mockResolvedValueOnce(json({data:{...org,revision:7,pinned:false}})).mockResolvedValueOnce(json({data:page})).mockResolvedValueOnce(json({data:{...org,revision:7,pinned:false}})).mockResolvedValueOnce(json({data:{...org,revision:7,pinned:false}}));mount();
 await screen.findByRole('button',{name:'读取当前组织状态'});expect(screen.queryByRole('button',{name:'确认按原键重试原变更'})).toBeNull();
 fireEvent.click(screen.getByRole('button',{name:'读取当前组织状态'}));await screen.findByText(/已读取当前组织状态；这不是原请求回执/);
 fireEvent.click(screen.getByRole('button',{name:'确认按原键重试原变更'}));await screen.findByText('原请求已被服务端接受；返回当前组织版本 7。');
 const writes=fetch.mock.calls.filter(([,options])=>options?.method==='PATCH');expect(writes).toHaveLength(2);expect(writes[1][1]?.body).toBe(original?.body);expect(writes[1][1]?.headers).toEqual(original?.headers);
});
it('keeps intent after a CAS conflict and requires a new confirmation on the fresh revision',async()=>{
 const fetch=mockLoad().mockResolvedValueOnce(json({error:{code:'revision_conflict'}},409)).mockResolvedValueOnce(json({data:{...org,revision:2}})).mockResolvedValueOnce(json({data:page}));mount();
 fireEvent.click(await screen.findByRole('checkbox',{name:'在个人组织中置顶'}));fireEvent.click(screen.getByRole('button',{name:'保存分组与置顶'}));fireEvent.click(await screen.findByRole('button',{name:'保留意图并重读组织'}));
 await screen.findByText(/已重读最新版本/);expect(screen.getByRole('checkbox',{name:'在个人组织中置顶'}).getAttribute('aria-checked')).toBe('true');expect(fetch.mock.calls.filter(([,options])=>options?.method==='PATCH')).toHaveLength(1);expect(readOrganizationIntent(slot)).toBeNull();
});
it('clears organization on scope change and ignores a late patch response',async()=>{
 let finish!:(value:Response)=>void;mockLoad().mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;}));mount();
 fireEvent.click(await screen.findByRole('checkbox',{name:'在个人组织中置顶'}));fireEvent.click(screen.getByRole('button',{name:'保存分组与置顶'}));await waitFor(()=>expect(finish).toBeDefined());fireEvent(window,new Event('drivebridge-context'));finish(json({data:{...org,revision:1}}));
 await screen.findByText(/组织数据已清除/);expect(screen.queryByText(/原请求已被服务端接受/)).toBeNull();expect(readOrganizationIntent(slot)).toBeTruthy();
});
it('refuses a write without locks and clears loaded data on HTTP403',async()=>{
 Object.defineProperty(navigator,'locks',{configurable:true,value:undefined});const fetch=mockLoad();const view=mount();
 fireEvent.click(await screen.findByRole('checkbox',{name:'在个人组织中置顶'}));fireEvent.click(screen.getByRole('button',{name:'保存分组与置顶'}));await screen.findByText(/组织读取或恢复存储不可用/);expect(fetch.mock.calls.filter(([,request])=>request?.method==='PATCH')).toHaveLength(0);
 view.unmount();fetch.mockResolvedValueOnce(json({},403)).mockResolvedValueOnce(json({data:page}));mount();await screen.findByText(/组织数据已清除/);expect(screen.queryByRole('combobox')).toBeNull();
});
it('refuses malformed storage and never overwrites another pending intent',async()=>{
 const body={expected_revision:0,pinned:true};const first=await claimOrganizationIntent(slot,body,new AbortController().signal);const second=await claimOrganizationIntent(slot,{expected_revision:8,pinned:false},new AbortController().signal);
 expect(first.claimed).toBe(true);expect(second).toEqual({intent:first.intent,claimed:false});
 expect(organizationSlot('bob',grant.agent_id)).not.toBe(slot);expect(organizationSlot('alice','other')).not.toBe(slot);
 localStorage.setItem(slot,JSON.stringify({...first.intent,body:{...body,secret:'forbidden'}}));expect(()=>readOrganizationIntent(slot)).toThrow();await expect(clearOrganizationIntent(slot,first.intent)).rejects.toThrow();
 const fetch=vi.spyOn(globalThis,'fetch');mount();await screen.findByText(/组织读取或恢复存储不可用/);expect(fetch).not.toHaveBeenCalled();
});
