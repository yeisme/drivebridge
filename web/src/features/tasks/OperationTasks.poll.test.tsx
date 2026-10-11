import { act,cleanup,fireEvent,render,screen } from '@testing-library/react';
import { afterEach,beforeEach,expect,it,vi } from 'vitest';
import fixtures from '../../../../contracts/personal-workspace/fixtures/responses.json';
import { OperationTasks } from './OperationTasks';
const scope=JSON.stringify(['instance','alice','','']);
const page=fixtures.TaskPageEnvelope;
const json=(data:unknown,status=200,headers?:HeadersInit)=>new Response(JSON.stringify(data),{status,headers});
const me={data:{user:{id:'alice'},instance_id:'instance'}};
const tick=async(ms:number)=>{await act(async()=>{await vi.advanceTimersByTimeAsync(ms);});};
beforeEach(()=>{vi.useFakeTimers();vi.spyOn(document,'hidden','get').mockReturnValue(false);});
afterEach(()=>{cleanup();vi.useRealTimers();vi.restoreAllMocks();});
it('checks identity before sequential polling and changes to idle cadence on completion',async()=>{
 let finish!:(value:Response)=>void;
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json(me)).mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;})).mockResolvedValueOnce(json(me)).mockResolvedValueOnce(json(page));
 render(<OperationTasks scope={scope}/>);await tick(0);await tick(1999);expect(fetch).toHaveBeenCalledTimes(1);await tick(1);expect(fetch.mock.calls[1][0]).toBe('/api/v1/me');expect(fetch).toHaveBeenCalledTimes(3);await tick(10000);expect(fetch).toHaveBeenCalledTimes(3);
 await act(async()=>finish(json({data:{...page.data,items:page.data.items.map(item=>({...item,owner_status:'completed',ui_state:'completed'}))}})));await tick(29999);expect(fetch).toHaveBeenCalledTimes(3);await tick(1);expect(fetch).toHaveBeenCalledTimes(5);
});
it.each([401,403])('stops permanently on %s and never issues an unverified task read',async status=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json({},status));render(<OperationTasks scope={scope}/>);await tick(0);await tick(2000);expect(screen.getByText(/任务数据已清除/)).toBeTruthy();expect(screen.queryByText('Synthetic upload')).toBeNull();await tick(60000);expect(fetch).toHaveBeenCalledTimes(2);
});
it('rejects a different signed-in user before querying their tasks',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json({data:{user:{id:'bob'},instance_id:'instance'}}));render(<OperationTasks scope={scope}/>);await tick(0);await tick(2000);expect(screen.getByText(/任务数据已清除/)).toBeTruthy();expect(fetch).toHaveBeenCalledTimes(2);
});
it('retains a visibly stale observation and respects Retry-After on transient failure',async()=>{
 vi.spyOn(Math,'random').mockReturnValue(0);
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json(me)).mockResolvedValueOnce(json({},503,{'Retry-After':'60'})).mockResolvedValueOnce(json(me)).mockResolvedValueOnce(json(page));render(<OperationTasks scope={scope}/>);await tick(0);await tick(2000);expect(screen.getByText(/以下为上次观察/)).toBeTruthy();expect(screen.getByText('Synthetic upload')).toBeTruthy();await tick(59999);expect(fetch).toHaveBeenCalledTimes(3);await tick(1);expect(fetch).toHaveBeenCalledTimes(5);expect(screen.queryByText(/以下为上次观察/)).toBeNull();
});
it('keeps polling a recoverable batch even when its owner phase says completed',async()=>{
 const batch={data:{...page.data,items:page.data.items.map(item=>({...item,owner_kind:'upload_batch',owner_status:'completed',ui_state:'recoverable'}))}};
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json(batch)).mockResolvedValueOnce(json(me)).mockResolvedValueOnce(json(batch));render(<OperationTasks scope={scope}/>);await tick(0);
 fireEvent.click(screen.getByRole('combobox',{name:'任务类别'}));await tick(0);const option=screen.getByRole('option',{name:'批量上传'});fireEvent.pointerDown(option,{pointerType:'mouse'});fireEvent.click(option);await tick(0);expect(fetch).toHaveBeenCalledTimes(2);
 await tick(2000);expect(fetch).toHaveBeenCalledTimes(4);expect(fetch.mock.calls[3][0]).toBe('/api/v1/workspace/tasks?kind=upload_batch&limit=50');
});
it.each(['initial','poll'])('stops automatic reads when the transfer deployment gate closes during %s',async phase=>{
 const transfer={data:{...page.data,items:page.data.items.map(item=>({...item,owner_kind:'transfer_plan',owner_status:'source_cleanup_pending',ui_state:'waiting_confirmation'}))}};
 const disabled={error:{code:'unsupported'}};
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(page));
 if(phase==='initial')fetch.mockResolvedValueOnce(json(disabled,400));else fetch.mockResolvedValueOnce(json(transfer)).mockResolvedValueOnce(json(me)).mockResolvedValueOnce(json(disabled,400));
 render(<OperationTasks scope={scope}/>);await tick(0);fireEvent.click(screen.getByRole('combobox',{name:'任务类别'}));await tick(0);const option=screen.getByRole('option',{name:'传输计划'});fireEvent.pointerDown(option,{pointerType:'mouse'});fireEvent.click(option);await tick(0);if(phase==='poll')await tick(2000);
 expect(screen.getByText(/未启用此任务类别/)).toBeTruthy();expect(screen.queryByText('当前授权范围及筛选下没有操作任务。')).toBeNull();const calls=fetch.mock.calls.length;await tick(60000);expect(fetch).toHaveBeenCalledTimes(calls);
});
it('pauses in background and rechecks identity on foreground',async()=>{
 const hidden=vi.spyOn(document,'hidden','get');const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json(me)).mockResolvedValueOnce(json(page));render(<OperationTasks scope={scope}/>);await tick(0);hidden.mockReturnValue(true);fireEvent(document,new Event('visibilitychange'));await tick(60000);expect(fetch).toHaveBeenCalledTimes(1);hidden.mockReturnValue(false);fireEvent(document,new Event('visibilitychange'));await tick(2000);expect(fetch.mock.calls[1][0]).toBe('/api/v1/me');expect(fetch).toHaveBeenCalledTimes(3);
});
it('pauses polling while viewing detail or accumulating additional cursor pages',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:{...page.data,next_cursor:'page2'}})).mockResolvedValueOnce(json(fixtures.OperationTaskDetailEnvelope)).mockResolvedValueOnce(json({data:{...page.data,items:[]}}));render(<OperationTasks scope={scope}/>);await tick(0);fireEvent.click(screen.getByRole('button',{name:'查看原操作详情 operation-fixture'}));await tick(0);await tick(60000);expect(fetch).toHaveBeenCalledTimes(2);fireEvent.click(screen.getByRole('button',{name:'关闭操作详情'}));fireEvent.click(screen.getByRole('button',{name:'更多操作任务'}));await tick(0);await tick(60000);expect(fetch).toHaveBeenCalledTimes(3);expect(fetch.mock.calls[2][0]).toContain('cursor=page2');
});
