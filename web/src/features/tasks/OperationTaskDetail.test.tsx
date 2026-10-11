import { cleanup,fireEvent,render,screen,waitFor,within } from '@testing-library/react';
import { afterEach,expect,it,vi } from 'vitest';
import fixtures from '../../../../contracts/personal-workspace/fixtures/responses.json';
import { OperationTasks } from './OperationTasks';
import { OperationTaskDetail } from './OperationTaskDetail';
const task={...fixtures.TaskPageEnvelope.data.items[0],owner_kind:'operation' as const,actor_kind:'unknown' as const,ui_state:'running' as const};
const output={file_id:'original-file',version_id:'original-version',space_id:task.space_id};
const detail={task:{...task,owner_status:'completed',ui_state:'completed'},outputs:[output],output_state:'available'};
const json=(data:unknown,status=200)=>new Response(JSON.stringify(data),{status});
afterEach(()=>{cleanup();vi.restoreAllMocks();});
it('opens metadata only by original ID, shows fixed output and restores keyboard focus',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(fixtures.TaskPageEnvelope)).mockResolvedValueOnce(json({data:detail}));render(<OperationTasks scope="alice"/>);
 const trigger=await screen.findByRole('button',{name:'查看原操作详情 '+task.owner_id});fireEvent.click(trigger);await screen.findByText('original-version');expect(fetch.mock.calls[1][0]).toBe('/api/v1/workspace/tasks/operation/'+task.owner_id);
 expect(fetch.mock.calls).toHaveLength(2);expect(fetch.mock.calls.every(([url,options])=>!String(url).includes('/content')&&options?.method==='GET')).toBe(true);
 fireEvent.click(screen.getByRole('button',{name:'关闭操作详情'}));await waitFor(()=>expect(screen.queryByRole('region',{name:'原操作详情'})).toBeNull());expect(document.activeElement).toBe(trigger);
});
it.each(['unavailable','not_published','not_projected'])('handles %s without substituting a current output',async state=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:{...detail,outputs:[],output_state:state}}));render(<OperationTaskDetail task={task} close={vi.fn()} onDenied={vi.fn()}/>);await waitFor(()=>expect(screen.queryByText('正在读取原操作详情…')).toBeNull());expect(screen.queryByLabelText('固定输出引用')).toBeNull();expect(fetch).toHaveBeenCalledTimes(1);
});
it.each([
 {...detail,task:{...detail.task,owner_id:'other'}},
 {...detail,task:{...detail.task,agent_id:'other-agent'}},
 {...detail,outputs:[{...output,space_id:'other'}]},
 {...detail,output_state:'unavailable'},
 {...detail,outputs:[]},
 {...detail,outputs:[{...output,version_id:''}]},
])('rejects mismatched identity or inconsistent output envelope',async invalid=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:invalid}));render(<OperationTaskDetail task={task} close={vi.fn()} onDenied={vi.fn()}/>);await screen.findByText(/未将失败当作没有输出/);expect(screen.queryByText('original-version')).toBeNull();
});
it('403 detail response clears the parent list rather than leaving readable cards',async()=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(fixtures.TaskPageEnvelope)).mockResolvedValueOnce(json({},403));render(<OperationTasks scope="alice"/>);fireEvent.click(await screen.findByRole('button',{name:'查看原操作详情 '+task.owner_id}));await screen.findByText(/任务数据已清除/);expect(screen.queryByText('Synthetic upload')).toBeNull();expect(screen.queryByRole('region',{name:'原操作详情'})).toBeNull();
});
it('re-read failure clears a previously available fixed reference',async()=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:detail})).mockResolvedValueOnce(json({},503));render(<OperationTaskDetail task={task} close={vi.fn()} onDenied={vi.fn()}/>);await screen.findByText('original-version');fireEvent.click(screen.getByRole('button',{name:'重读原操作详情'}));await screen.findByText(/未将失败当作没有输出/);expect(screen.queryByText('original-version')).toBeNull();
});
it('ignores detail arriving after a scope change',async()=>{
 let finish!:(value:Response)=>void;const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(fixtures.TaskPageEnvelope)).mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;}));render(<OperationTasks scope="alice"/>);fireEvent.click(await screen.findByRole('button',{name:'查看原操作详情 '+task.owner_id}));await waitFor(()=>expect(finish).toBeDefined());fireEvent(window,new Event('drivebridge-context'));finish(json({data:detail}));await screen.findByText(/任务数据已清除/);expect(screen.queryByText('original-version')).toBeNull();expect(fetch).toHaveBeenCalledTimes(2);
});
it('switching original task identity never retains a prior reference',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:detail})).mockResolvedValueOnce(json({data:{...detail,task:{...detail.task,owner_id:'second'},outputs:[],output_state:'not_published'}}));const view=render(<OperationTaskDetail task={task} close={vi.fn()} onDenied={vi.fn()}/>);await screen.findByText('original-version');view.rerender(<OperationTaskDetail task={{...task,owner_id:'second'}} close={vi.fn()} onDenied={vi.fn()}/>);await screen.findByText('原操作尚未发布输出');expect(within(screen.getByRole('region',{name:'原操作详情'})).queryByText('original-version')).toBeNull();expect(fetch.mock.calls[1][0]).toContain('/operation/second');
});
