import { cleanup,fireEvent,render,screen,waitFor } from '@testing-library/react';
import { afterEach,expect,it,vi } from 'vitest';
import fixtures from '../../../../contracts/personal-workspace/fixtures/responses.json';
import { BatchTaskItems } from './BatchTaskItems';
const page=fixtures.BatchTaskItemsEnvelope;const item=page.data.items[0];
const json=(data:unknown,status=200)=>new Response(JSON.stringify(data),{status});
const props={batch:page.data.batch_id,close:vi.fn(),onDenied:vi.fn()};
afterEach(()=>{cleanup();vi.restoreAllMocks();vi.clearAllMocks();});
it('reads only safe items and never treats an item ID as a recovery operation',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(page));render(<BatchTaskItems {...props}/>);await screen.findByText('fixture.txt');expect(screen.getByText('尚无关联操作')).toBeTruthy();expect(screen.getByText(/结果未确认/)).toBeTruthy();expect(fetch.mock.calls[0][0]).toBe('/api/v1/workspace/tasks/upload_batch/batch-fixture/items?limit=50');expect(fetch.mock.calls[0][1]?.method).toBe('GET');expect(screen.queryByRole('button',{name:/恢复/})).toBeNull();
});
it('appends only same-revision pages and preserves verified original operation IDs',async()=>{
 const next={...item,item_id:'second',relative_path:'second.txt',owner_status:'completed',result_certainty:'confirmed',operation_id:'original-operation'};
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:{...page.data,next_cursor:'next+page'}})).mockResolvedValueOnce(json({data:{...page.data,items:[next]}}));render(<BatchTaskItems {...props}/>);fireEvent.click(await screen.findByRole('button',{name:'更多批量条目'}));await screen.findByText('关联原操作：original-operation');expect(screen.getAllByRole('listitem')).toHaveLength(2);expect(fetch.mock.calls[1][0]).toContain('cursor=next%2Bpage');
});
it.each(['revision','http'])('clears stale pages on %s expiry and refreshes from page one',async mode=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:{...page.data,next_cursor:'next'}})).mockResolvedValueOnce(mode==='http'?json({},410):json({data:{...page.data,revision:'different'}})).mockResolvedValueOnce(json(page));render(<BatchTaskItems {...props}/>);fireEvent.click(await screen.findByRole('button',{name:'更多批量条目'}));await screen.findByText(/旧分页已清除/);expect(screen.queryByText('fixture.txt')).toBeNull();fireEvent.click(screen.getByRole('button',{name:'重读原批量条目'}));await screen.findByText('fixture.txt');expect(fetch.mock.calls[2][0]).not.toContain('cursor=');
});
it.each([
 {...page.data,batch_id:'other'},
 {...page.data,items:[{...item,result_certainty:'confirmed'}]},
 {...page.data,items:[{...item,is_dir:true,operation_id:'op'}]},
 {...page.data,items:[item,item]},
])('rejects inconsistent item identity or certainty',async invalid=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:invalid}));render(<BatchTaskItems {...props}/>);await screen.findByText(/来源合同不匹配/);expect(screen.queryByRole('listitem')).toBeNull();
});
it('403 clears rows and notifies the parent',async()=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json({},403));render(<BatchTaskItems {...props}/>);await screen.findByText('fixture.txt');fireEvent.click(screen.getByRole('button',{name:'重读原批量条目'}));await screen.findByText(/批量条目已清除/);expect(screen.queryByText('fixture.txt')).toBeNull();expect(props.onDenied).toHaveBeenCalledTimes(1);
});
it('scope invalidation cancels and ignores a late page',async()=>{
 let finish!:(value:Response)=>void;vi.spyOn(globalThis,'fetch').mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;}));render(<BatchTaskItems {...props}/>);fireEvent(window,new Event('drivebridge-context'));finish(json(page));await screen.findByText(/批量条目已清除/);await waitFor(()=>expect(screen.queryByText('fixture.txt')).toBeNull());
});
