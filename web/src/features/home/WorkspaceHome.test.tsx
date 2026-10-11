import { cleanup,fireEvent,render,screen,waitFor } from '@testing-library/react';
import { afterEach,expect,it,vi } from 'vitest';
import { WorkspaceHome } from './WorkspaceHome';
import type { Space } from '../../api';
const spaces=[{id:'space',name:'Personal',team_id:''}] as Space[];
const section={state:'ready',reason:'',observed_at:'2026-01-01T00:00:00Z'};
const file={file_id:'file',version_id:'original',space_id:'space',path:'note.txt',ref:'drivebridge://instance/file/file/version/original',size_bytes:4,mime_type:'text/plain',updated_at:section.observed_at};
const home={space_id:'space',recent_files:[file],collections:[{collection_id:'c',name:'Research',kind:'manual',revision:1}],pending_counts:{submitted:2,accepted:1},usage:{used_bytes:4,reserved_bytes:0,quota_bytes:100},sections:{recent_files:section,collections:section,pending_counts:{...section,state:'partial',reason:'submission_states_only'},usage:section},observed_at:section.observed_at};
const json=(data:unknown,status=200)=>new Response(JSON.stringify({data}),{status});
afterEach(()=>{cleanup();vi.restoreAllMocks();});
it('renders independent sections without presenting submission counts as all tasks',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(home));render(<WorkspaceHome scope="alice" spaces={spaces}/>);await screen.findByText('Research');expect(screen.getByText(/仅收件状态/)).toBeTruthy();expect(fetch.mock.calls[0][0]).toBe('/api/v1/workspace/home?space_id=space&recent_limit=6');expect(fetch.mock.calls).toHaveLength(1);
});
it('keeps a healthy section when usage is unavailable',async()=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({...home,sections:{...home.sections,usage:{...section,state:'unavailable'}},usage:{used_bytes:null,reserved_bytes:null,quota_bytes:null}}));render(<WorkspaceHome scope="alice" spaces={spaces}/>);await screen.findByText('Research');expect(screen.getByText('此段暂不可用，不代表数据为空。')).toBeTruthy();expect(screen.queryByText('已使用')).toBeNull();
});
it('re-authorizes the exact original reference and restores focus without body requests',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(home)).mockResolvedValueOnce(json({file:{id:'file',space_id:'space',version_id:'original'},ref:file.ref}));render(<WorkspaceHome scope="alice" spaces={spaces}/>);const trigger=await screen.findByRole('button',{name:'查看固定引用：note.txt'});fireEvent.click(trigger);await screen.findByText(file.ref);expect(fetch.mock.calls[1][0]).toBe('/api/v1/references?ref='+encodeURIComponent(file.ref));expect(fetch.mock.calls).toHaveLength(2);fireEvent.click(screen.getByRole('button',{name:'关闭固定引用'}));expect(document.activeElement).toBe(trigger);
});
it('clears both the parent summary and reader when body access is revoked',async()=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(home)).mockResolvedValueOnce(json({file:{id:'file',space_id:'space',version_id:'original'},ref:file.ref})).mockResolvedValueOnce(json({file:{id:'file',space_id:'space',version_id:'latest',trashed:false}})).mockResolvedValueOnce(new Response('',{status:403}));
 render(<WorkspaceHome scope="alice" spaces={spaces}/>);fireEvent.click(await screen.findByRole('button',{name:'查看固定引用：note.txt'}));fireEvent.click(await screen.findByRole('button',{name:'读取原版本正文'}));await screen.findByText(/身份或范围已变化/);expect(screen.queryByText('Research')).toBeNull();expect(screen.queryByRole('region',{name:'首页固定引用'})).toBeNull();
});
it('rejects a substituted latest reference',async()=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(home)).mockResolvedValueOnce(json({file:{id:'file',space_id:'space',version_id:'new'},ref:file.ref}));render(<WorkspaceHome scope="alice" spaces={spaces}/>);fireEvent.click(await screen.findByRole('button',{name:'查看固定引用：note.txt'}));await screen.findByText(/未改查最新版本/);expect(screen.queryByText(file.ref)).toBeNull();
});
it.each([403,401])('clears the summary when fixed-reference authorization returns %s',async status=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(home)).mockResolvedValueOnce(json({},status));render(<WorkspaceHome scope="alice" spaces={spaces}/>);fireEvent.click(await screen.findByRole('button',{name:'查看固定引用：note.txt'}));await screen.findByText(/身份或范围已变化/);expect(screen.queryByText('Research')).toBeNull();
});
it('rejects cross-space home envelopes',async()=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({...home,space_id:'other'}));render(<WorkspaceHome scope="alice" spaces={spaces}/>);await screen.findByText(/摘要不可用或服务/);expect(screen.queryByText('Research')).toBeNull();
});
it('context invalidation ignores an abort-insensitive late response',async()=>{
 let finish!:(value:Response)=>void;vi.spyOn(globalThis,'fetch').mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;}));render(<WorkspaceHome scope="alice" spaces={spaces}/>);fireEvent(window,new Event('drivebridge-context'));finish(json(home));await screen.findByText(/身份或范围已变化/);await waitFor(()=>expect(screen.queryByText('Research')).toBeNull());
});
it('offline clears memory and coming online does not automatically refetch',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockImplementation(async()=>json(home));render(<WorkspaceHome scope="alice" spaces={spaces}/>);await screen.findByText('Research');fireEvent(window,new Event('offline'));await screen.findByText(/当前离线/);expect(screen.queryByText('Research')).toBeNull();fireEvent(window,new Event('online'));expect(fetch).toHaveBeenCalledTimes(1);fireEvent.click(screen.getByRole('button',{name:'刷新工作台摘要'}));await screen.findByText('Research');expect(fetch).toHaveBeenCalledTimes(2);
});
it('scope replacement discards a late result from the previous actor',async()=>{
 let finish!:(value:Response)=>void;
 vi.spyOn(globalThis,'fetch').mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;})).mockResolvedValueOnce(json({...home,recent_files:[],collections:[]}));
 const view=render(<WorkspaceHome scope="alice" spaces={spaces}/>);view.rerender(<WorkspaceHome scope="bob" spaces={spaces}/>);await screen.findByText('当前没有集合。');finish(json(home));await waitFor(()=>expect(screen.queryByText('Research')).toBeNull());expect(screen.queryByRole('button',{name:'查看固定引用：note.txt'})).toBeNull();
});
it('offline at entry does not send a request',()=>{
 vi.spyOn(navigator,'onLine','get').mockReturnValue(false);const fetch=vi.spyOn(globalThis,'fetch');render(<WorkspaceHome scope="alice" spaces={spaces}/>);expect(screen.getByText(/当前离线/)).toBeTruthy();expect(fetch).not.toHaveBeenCalled();
});
it('makes no request without personal spaces',()=>{
 const fetch=vi.spyOn(globalThis,'fetch');render(<WorkspaceHome scope="alice" spaces={[]}/>);expect(screen.getByText(/没有可用的个人空间/)).toBeTruthy();expect(fetch).not.toHaveBeenCalled();
});
