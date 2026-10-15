/// <reference lib="es2024.promise" />
import { cleanup,fireEvent,render,screen,waitFor } from '@testing-library/react';
import { afterEach,expect,it,vi } from 'vitest';
import fixtures from '../../../../contracts/personal-workspace/fixtures/responses.json';
import { OperationTasks } from './OperationTasks';
const page=fixtures.TaskPageEnvelope;const task=page.data.items[0];
const json=(data:unknown,status=200)=>new Response(JSON.stringify(data),{status});
afterEach(()=>{cleanup();vi.restoreAllMocks();});
it('reads persisted owner identity and keeps unknown progress distinct from completion',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(page));render(<OperationTasks scope="alice"/>);await screen.findByText('Synthetic upload');expect(fetch.mock.calls[0][0]).toBe('/api/v1/workspace/tasks?kind=operation&limit=50');expect(screen.getByText('原操作：operation:operation-fixture')).toBeTruthy();expect(screen.getByText('显示状态：进行中 · 原状态：verifying')).toBeTruthy();expect(screen.getAllByText('未提供')).toHaveLength(7);expect(screen.queryByRole('button',{name:'恢复'})).toBeNull();expect(fetch.mock.calls[0][1]?.method).toBe('GET');
});
it('applies status filtering server-side from page one and drops the previous pending page',async()=>{
 let finish!:(r:Response)=>void;const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({...page,data:{...page.data,next_cursor:'later'}})).mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;})).mockResolvedValueOnce(json({data:{...page.data,items:[]}}));render(<OperationTasks scope="alice"/>);fireEvent.click(await screen.findByRole('button',{name:'更多操作任务'}));await waitFor(()=>expect(finish).toBeDefined());
 fireEvent.click(screen.getByRole('combobox',{name:'任务显示状态'}));const option=await screen.findByRole('option',{name:'已取消'});fireEvent.pointerDown(option,{pointerType:'mouse'});fireEvent.click(option);
 await screen.findByText('当前授权范围及筛选下没有操作任务。');expect(fetch.mock.calls[2][0]).toBe('/api/v1/workspace/tasks?kind=operation&limit=50&state=cancelled');finish(json(page));await waitFor(()=>expect(screen.queryByText('Synthetic upload')).toBeNull());
});
it('switches kind from page one, drops old pages and never opens an operation detail for a batch',async()=>{
 let finish!:(r:Response)=>void;
 const batch={...task,owner_kind:'upload_batch',owner_id:'batch-original',owner_status:'completed',ui_state:'recoverable',title:'Batch fixture',progress:{...task.progress,total:2,succeeded:1,failed:1,pending:0}};
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:{...page.data,next_cursor:'old-page'}})).mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;})).mockResolvedValueOnce(json({data:{...page.data,items:[batch]}}));render(<OperationTasks scope="alice"/>);fireEvent.click(await screen.findByRole('button',{name:'更多操作任务'}));await waitFor(()=>expect(finish).toBeDefined());
 fireEvent.click(screen.getByRole('combobox',{name:'任务类别'}));const option=await screen.findByRole('option',{name:'批量上传'});fireEvent.pointerDown(option,{pointerType:'mouse'});fireEvent.click(option);await screen.findByText('Batch fixture');
 expect(fetch.mock.calls[2][0]).toBe('/api/v1/workspace/tasks?kind=upload_batch&limit=50');expect(screen.getByText('显示状态：待恢复 · 原状态：completed')).toBeTruthy();expect(screen.getByText('原操作：upload_batch:batch-original')).toBeTruthy();expect(screen.queryByRole('button',{name:/查看原操作详情/})).toBeNull();finish(json(page));await waitFor(()=>expect(screen.queryByText('Synthetic upload')).toBeNull());
});
it('reads original organization plans without misrouting IDs to batch or operation details',async()=>{
 const plan={...task,owner_kind:'organization_plan',owner_id:'plan-original',owner_status:'completed',ui_state:'unknown',actor_kind:'unknown',agent_id:null,title:'Organization plan',blocked_reason:'organization_result_unproven'};
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json({data:{...page.data,items:[plan]}}));render(<OperationTasks scope="alice"/>);await screen.findByText('Synthetic upload');fireEvent.click(screen.getByRole('combobox',{name:'任务类别'}));const option=await screen.findByRole('option',{name:'整理计划'});fireEvent.pointerDown(option,{pointerType:'mouse'});fireEvent.click(option);await screen.findByText('原操作：organization_plan:plan-original');expect(screen.getByText('显示状态：未知 · 原状态：completed')).toBeTruthy();expect(screen.getAllByText('未提供')).toHaveLength(7);expect(screen.queryByRole('button',{name:/查看原操作详情|查看批量条目|执行计划/})).toBeNull();expect(fetch.mock.calls[1][0]).toBe('/api/v1/workspace/tasks?kind=organization_plan&limit=50');expect(fetch).toHaveBeenCalledTimes(2);
});
it('does not present missing Agent attribution as no plans',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockImplementation(async()=>json({data:{...page.data,items:[]}}));render(<OperationTasks scope="alice" agent="agent-a"/>);await screen.findByText('当前授权范围及筛选下没有操作任务。');fireEvent.click(screen.getByRole('combobox',{name:'任务类别'}));const option=await screen.findByRole('option',{name:'整理计划'});fireEvent.pointerDown(option,{pointerType:'mouse'});fireEvent.click(option);await screen.findByText(/空结果不表示该 Agent 没有计划/);await waitFor(()=>expect(fetch).toHaveBeenCalledTimes(2));expect(screen.queryByText('当前授权范围及筛选下没有操作任务。')).toBeNull();expect(fetch.mock.calls[1][0]).toContain('kind=organization_plan&limit=50&agent_id=agent-a');
});
it('shows migration cleanup as a separate stage without a batch detail or cleanup action',async()=>{
 const plan={...task,owner_kind:'transfer_plan',owner_id:'transfer-original',title:'Transfer plan: migrate',owner_status:'source_cleanup_pending',ui_state:'waiting_confirmation',blocked_reason:'source_cleanup_pending'};
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json({data:{...page.data,items:[plan]}}));render(<OperationTasks scope="alice"/>);await screen.findByText('Synthetic upload');fireEvent.click(screen.getByRole('combobox',{name:'任务类别'}));const option=await screen.findByRole('option',{name:'传输计划'});fireEvent.pointerDown(option,{pointerType:'mouse'});fireEvent.click(option);await screen.findByText(/源清理待处理：/);expect(screen.getByText('显示状态：待确认 · 原状态：source_cleanup_pending')).toBeTruthy();expect(screen.queryByRole('button',{name:/查看批量条目|查看原操作详情|执行清理/})).toBeNull();expect(fetch.mock.calls[1][0]).toContain('kind=transfer_plan');
});
it('switches to automation from an in-flight page without issuing any rule action',async()=>{
 let finish!:(value:Response)=>void;const intent={...task,owner_kind:'automation_intent',owner_id:'intent-original',title:'Automation intent',owner_status:'completed',ui_state:'unknown',actor_kind:'unknown',agent_id:null};
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:{...page.data,next_cursor:'old-page'}})).mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;})).mockResolvedValueOnce(json({data:{...page.data,items:[intent]}}));render(<OperationTasks scope="alice"/>);fireEvent.click(await screen.findByRole('button',{name:'更多操作任务'}));fireEvent.click(screen.getByRole('combobox',{name:'任务类别'}));const option=await screen.findByRole('option',{name:'自动化意图'});fireEvent.pointerDown(option,{pointerType:'mouse'});fireEvent.click(option);await screen.findByText('原操作：automation_intent:intent-original');expect(screen.getByText('显示状态：未知 · 原状态：completed')).toBeTruthy();expect(screen.getAllByText('未提供')).toHaveLength(7);expect(screen.queryByRole('button',{name:/查看原操作详情|查看批量条目|运行规则/})).toBeNull();expect(fetch.mock.calls[2][0]).toBe('/api/v1/workspace/tasks?kind=automation_intent&limit=50');expect(fetch.mock.calls.every(([,options])=>options?.method==='GET')).toBe(true);finish(json(page));await waitFor(()=>expect(screen.queryByText('Synthetic upload')).toBeNull());
});
it('does not label unproven Agent automation ownership as empty tasks',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockImplementation(async()=>json({data:{...page.data,items:[]}}));render(<OperationTasks scope="alice" agent="agent-a"/>);await screen.findByText('当前授权范围及筛选下没有操作任务。');fireEvent.click(screen.getByRole('combobox',{name:'任务类别'}));const option=await screen.findByRole('option',{name:'自动化意图'});fireEvent.pointerDown(option,{pointerType:'mouse'});fireEvent.click(option);await screen.findByText(/空结果不表示该 Agent 没有自动化意图/);await waitFor(()=>expect(fetch).toHaveBeenCalledTimes(2));expect(screen.queryByText('当前授权范围及筛选下没有操作任务。')).toBeNull();
});
it('rejects a response from the wrong owner kind',async()=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:{...page.data,items:[{...task,owner_kind:'upload_batch'}]}}));render(<OperationTasks scope="alice"/>);await screen.findByText(/操作投影不可用/);expect(screen.queryByText('Synthetic upload')).toBeNull();
});
it('uses authenticated Agent filtering and rejects mismatched response identity',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(page));render(<OperationTasks scope="alice" agent="agent-a"/>);await screen.findByText(/操作投影不可用/);expect(screen.queryByText('Synthetic upload')).toBeNull();expect(fetch.mock.calls[0][0]).toContain('agent_id=agent-a');
});
it('invalidates stale pages rather than combining different generations',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:{...page.data,next_cursor:'next+page'}})).mockResolvedValueOnce(json({},410)).mockResolvedValueOnce(json(page));render(<OperationTasks scope="alice"/>);fireEvent.click(await screen.findByRole('button',{name:'更多操作任务'}));await screen.findByText(/操作分页已过期/);expect(screen.queryByText('Synthetic upload')).toBeNull();expect(fetch.mock.calls[1][0]).toContain('cursor=next%2Bpage');fireEvent.click(screen.getByRole('button',{name:'刷新操作投影'}));await screen.findByText('Synthetic upload');expect(fetch.mock.calls[2][0]).not.toContain('cursor=');
});
it('scope revocation clears data and discards a late response',async()=>{
 let finish!:(r:Response)=>void;vi.spyOn(globalThis,'fetch').mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;}));render(<OperationTasks scope="alice"/>);fireEvent(window,new Event('drivebridge-context'));finish(json(page));await screen.findByText(/任务数据已清除/);expect(screen.queryByText('Synthetic upload')).toBeNull();
});
it('replaces scope before late previous data can populate the new view',async()=>{
 let finish!:(r:Response)=>void;const fetch=vi.spyOn(globalThis,'fetch').mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;})).mockResolvedValueOnce(json({data:{...page.data,items:[]}}));const view=render(<OperationTasks scope="alice"/>);view.rerender(<OperationTasks scope="bob"/>);await screen.findByText('当前授权范围及筛选下没有操作任务。');finish(json(page));await waitFor(()=>expect(screen.queryByText('Synthetic upload')).toBeNull());expect(fetch).toHaveBeenCalledTimes(2);
});
it.each([403,429,503,404])('distinguishes unavailable status %s from an empty list',async status=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({},status));render(<OperationTasks scope="alice"/>);await screen.findByText(status===403?/任务数据已清除/:status===404?/当前服务不支持/:/操作投影不可用/);expect(screen.queryByText('当前授权范围及筛选下没有操作任务。')).toBeNull();
});
it('rejects missing progress fields rather than rendering undefined as a known value',async()=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:{...page.data,items:[{...task,progress:{}}]}}));render(<OperationTasks scope="alice"/>);await screen.findByText(/操作投影不可用/);
});
it('clears the task projection and reports offline without treating it as empty',async()=>{
 const gate=Promise.withResolvers<Response>();
 const fetch=vi.spyOn(globalThis,'fetch').mockImplementationOnce(()=>gate.promise);
 render(<OperationTasks scope="alice"/>);
 await waitFor(()=>expect(fetch).toHaveBeenCalled());
 fireEvent(window,new Event('offline'));
 await screen.findByText(/当前离线，任务观察已清除/);
 expect(screen.queryByText('当前没有匹配的任务')).toBeNull();
 gate.resolve(json(page));
 await waitFor(()=>expect(screen.queryByText('Synthetic upload')).toBeNull());
});
