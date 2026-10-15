import { cleanup,fireEvent,render,screen,waitFor,within } from '@testing-library/react';
import { afterEach,beforeEach,expect,it,vi } from 'vitest';
import fixtures from '../../../../contracts/personal-workspace/fixtures/responses.json';
import { LabelSuggestionEntry,LabelSuggestions } from './LabelSuggestions';
import { claimSuggestionIntent,readSuggestionIntent,suggestionSlot } from './suggestion-recovery';
const suggestion=fixtures.LabelSuggestionEnvelope.data;
const props={scope:'alice',file:suggestion.file_id,version:suggestion.version_id};
const page={data:{items:[suggestion],next_cursor:'',observed_at:'2026-01-01T00:00:00Z',total:null,total_kind:'unknown'}};
const json=(data:unknown,status=200)=>new Response(JSON.stringify(data),{status});
const slot=suggestionSlot(props.scope,props.file,props.version),locks=Object.getOwnPropertyDescriptor(navigator,'locks');
beforeEach(()=>Object.defineProperty(navigator,'locks',{configurable:true,value:{request:vi.fn(async(_:string,options:LockOptions|(()=>unknown),callback?:()=>unknown)=>typeof options==='function'?options():callback!())}}));
afterEach(()=>{cleanup();localStorage.clear();vi.restoreAllMocks();if(locks)Object.defineProperty(navigator,'locks',locks);else Reflect.deleteProperty(navigator,'locks');});
it('loads on demand, defaults to zero labels, and confirms only the selected subset with CAS',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:{...page.data,items:[{...suggestion,labels:['keep','new']}]}})).mockResolvedValueOnce(json({data:{...suggestion,state:'accepted',revision:2}}));render(<LabelSuggestionEntry {...props}/>);expect(fetch).not.toHaveBeenCalled();fireEvent.click(screen.getByRole('button',{name:'审阅 Agent 标签建议'}));await screen.findByText(/认证 Agent/);
 expect((screen.getByRole('button',{name:'采纳所选标签'}) as HTMLButtonElement).disabled).toBe(true);fireEvent.click(screen.getByRole('checkbox',{name:'new'}));fireEvent.click(screen.getByRole('button',{name:'采纳所选标签'}));expect(fetch).toHaveBeenCalledTimes(1);fireEvent.click(screen.getByRole('button',{name:'确认标签决定'}));await screen.findByText('原标签决定已被服务端接受。');
 expect(fetch.mock.calls[0][0]).toBe('/api/v1/files/file-fixture/label-suggestions?version_id=version-fixture&limit=50');expect(JSON.parse(fetch.mock.calls[1][1]?.body as string)).toEqual({expected_revision:1,base_version_id:'version-fixture',decision:'accept',labels:['new']});expect(fetch.mock.calls[1][1]?.headers).toMatchObject({'Idempotency-Key':readSuggestionIntent(slot)?.key});fireEvent.click(screen.getByRole('button',{name:'确认标签结果'}));await waitFor(()=>expect(readSuggestionIntent(slot)).toBeNull());
});
it('rejects without sending labels into manual namespace',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json({data:{...suggestion,state:'rejected',revision:2}}));render(<LabelSuggestions {...props}/>);fireEvent.click(await screen.findByRole('button',{name:'拒绝此建议'}));fireEvent.click(screen.getByRole('button',{name:'确认标签决定'}));await screen.findByText('原标签决定已被服务端接受。');expect(JSON.parse(fetch.mock.calls[1][1]?.body as string)).toEqual({expected_revision:1,base_version_id:'version-fixture',decision:'reject',labels:[]});
});
it('restores exact original subset across remount and never treats current accepted state as its receipt',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(page)).mockRejectedValueOnce(new TypeError('lost'));const view=render(<LabelSuggestions {...props}/>);fireEvent.click(await screen.findByRole('checkbox',{name:'reference'}));fireEvent.click(screen.getByRole('button',{name:'采纳所选标签'}));fireEvent.click(screen.getByRole('button',{name:'确认标签决定'}));await screen.findByRole('button',{name:'观察原版本建议'});const original=fetch.mock.calls[1][1];view.unmount();
 fetch.mockResolvedValueOnce(json({data:{...page.data,items:[{...suggestion,state:'accepted',revision:2}]}})).mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json({data:{file_id:props.file,namespaces:{manual:['existing-manual','reference']}}})).mockResolvedValueOnce(json({data:{...suggestion,state:'accepted',revision:2}}));render(<LabelSuggestions {...props}/>);
 fireEvent.click(await screen.findByRole('button',{name:'观察原版本建议'}));await screen.findByText(/当前状态不是原请求回执/);expect(within(screen.getByLabelText('当前人工标签事实')).getByText('existing-manual')).toBeDefined();expect(screen.queryByText('原标签决定已被服务端接受。')).toBeNull();fireEvent.click(screen.getByRole('button',{name:'确认重试原标签决定'}));await screen.findByText('原标签决定已被服务端接受。');const writes=fetch.mock.calls.filter(([,options])=>options?.method==='POST');expect(writes).toHaveLength(2);expect(writes[1][1]?.body).toBe(original?.body);expect(writes[1][1]?.headers).toEqual(original?.headers);
});
it('groups loaded suggestions by authenticated Agent across page boundaries',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:{...page.data,next_cursor:'page-2'}})).mockResolvedValueOnce(json({data:{...page.data,items:[{...suggestion,suggestion_id:'same-agent-second'},{...suggestion,suggestion_id:'other-agent-first',agent_id:'other-agent'}]}}));
 render(<LabelSuggestions {...props}/>);fireEvent.click(await screen.findByRole('button',{name:'更多标签建议'}));await screen.findByRole('region',{name:'认证 Agent other-agent'});
 expect(within(screen.getByRole('region',{name:'认证 Agent '+suggestion.agent_id})).getAllByRole('article')).toHaveLength(2);expect(within(screen.getByRole('region',{name:'认证 Agent other-agent'})).getAllByRole('article')).toHaveLength(1);expect(fetch).toHaveBeenCalledTimes(2);
});
it.each([503,403])('failed manual fact read (%s) never enables retry or clears original intent',async status=>{
 const original=await claimSuggestionIntent(slot,suggestion.suggestion_id,{expected_revision:1,base_version_id:props.version,decision:'reject',labels:[]},new AbortController().signal);
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json({error:{code:status===403?'forbidden':'unavailable'}},status));
 render(<LabelSuggestions {...props}/>);fireEvent.click(await screen.findByRole('button',{name:'观察原版本建议'}));await screen.findByText(status===403?/权限或范围已变化/:/决定结果未知/);
 expect(screen.queryByRole('button',{name:'确认重试原标签决定'})).toBeNull();expect(screen.queryByLabelText('当前人工标签事实')).toBeNull();expect(readSuggestionIntent(slot)).toEqual(original.intent);expect(fetch.mock.calls.filter(([,options])=>options?.method==='POST')).toHaveLength(0);
});
it('a version conflict keeps original fixed identity and never retries automatically',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json({error:{code:'version_conflict'}},409));render(<LabelSuggestions {...props}/>);fireEvent.click(await screen.findByRole('button',{name:'拒绝此建议'}));fireEvent.click(screen.getByRole('button',{name:'确认标签决定'}));await screen.findByText(/不自动套用到最新文件版本/);expect(fetch).toHaveBeenCalledTimes(2);expect(readSuggestionIntent(slot)?.body.base_version_id).toBe(props.version);
});
it('bounds paging and rejects entries for a different file version',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:{...page.data,next_cursor:'next+page'}})).mockResolvedValueOnce(json({data:{...page.data,items:[{...suggestion,version_id:'other'}]}}));render(<LabelSuggestions {...props}/>);fireEvent.click(await screen.findByRole('button',{name:'更多标签建议'}));await screen.findByText('建议暂不可用，不代表没有建议。');expect(fetch.mock.calls[1][0]).toContain('cursor=next%2Bpage');expect(screen.queryByRole('checkbox')).toBeNull();
});
it('clears choices on scope revocation and drops late responses',async()=>{
 let finish!:(response:Response)=>void;vi.spyOn(globalThis,'fetch').mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;}));render(<LabelSuggestions {...props}/>);await waitFor(()=>expect(finish).toBeDefined());fireEvent(window,new Event('drivebridge-context'));finish(json(page));await screen.findByText(/建议与选择已清除/);expect(screen.queryByRole('checkbox')).toBeNull();
});
it('returns focus to the fixed-version suggestion region when its state becomes unavailable',async()=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValue(json(page));render(<LabelSuggestionEntry {...props}/>);
 fireEvent.click(screen.getByRole('button',{name:'审阅 Agent 标签建议'}));await screen.findByText(/认证 Agent/);
 document.body.focus();fireEvent(window,new Event('drivebridge-context'));await screen.findByText(/建议与选择已清除/);
 expect(document.activeElement).toBe(screen.getByRole('region',{name:'固定版本标签建议'}));
});
it('missing locks and malformed recovery never send a decision',async()=>{
 Object.defineProperty(navigator,'locks',{configurable:true,value:undefined});const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValue(json(page));const view=render(<LabelSuggestions {...props}/>);fireEvent.click(await screen.findByRole('button',{name:'拒绝此建议'}));fireEvent.click(screen.getByRole('button',{name:'确认标签决定'}));await screen.findByText('建议暂不可用，不代表没有建议。');expect(fetch).toHaveBeenCalledTimes(1);view.unmount();
 localStorage.setItem(slot,JSON.stringify({key:'suggestion-'+'a'.repeat(48),suggestion:suggestion.suggestion_id,body:{expected_revision:1,base_version_id:props.version,decision:'reject',labels:[],secret:'forbidden'}}));expect(()=>readSuggestionIntent(slot)).toThrow();render(<LabelSuggestions {...props}/>);await screen.findByText('建议暂不可用，不代表没有建议。');expect(fetch).toHaveBeenCalledTimes(1);
});
it('offline confirmation never claims or sends; online requires reopening and a fresh read',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockImplementation(async()=>json(page));render(<LabelSuggestionEntry {...props}/>);fireEvent.click(screen.getByRole('button',{name:'审阅 Agent 标签建议'}));fireEvent.click(await screen.findByRole('button',{name:'拒绝此建议'}));const online=vi.spyOn(navigator,'onLine','get').mockReturnValue(false);fireEvent.click(screen.getByRole('button',{name:'确认标签决定'}));await screen.findByText(/当前离线/);expect(readSuggestionIntent(slot)).toBeNull();expect(fetch).toHaveBeenCalledTimes(1);
 online.mockReturnValue(true);fireEvent(window,new Event('online'));expect(screen.queryByRole('button',{name:'确认标签决定'})).toBeNull();expect(fetch).toHaveBeenCalledTimes(1);fireEvent.click(screen.getByRole('button',{name:'收起标签建议'}));fireEvent.click(screen.getByRole('button',{name:'审阅 Agent 标签建议'}));await screen.findByRole('button',{name:'拒绝此建议'});expect(fetch).toHaveBeenCalledTimes(2);
});
it('offline event aborts an in-flight decision but retains its original recovery identity',async()=>{
 let finish!:(response:Response)=>void;const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(page)).mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;}));render(<LabelSuggestions {...props}/>);fireEvent.click(await screen.findByRole('button',{name:'拒绝此建议'}));fireEvent.click(screen.getByRole('button',{name:'确认标签决定'}));await waitFor(()=>expect(fetch).toHaveBeenCalledTimes(2));const saved=readSuggestionIntent(slot);expect(saved).not.toBeNull();fireEvent(window,new Event('offline'));await screen.findByText(/当前离线/);finish(json({data:{...suggestion,state:'rejected',revision:2}}));expect(readSuggestionIntent(slot)).toEqual(saved);expect(screen.queryByText('原标签决定已被服务端接受。')).toBeNull();expect((fetch.mock.calls[1][1]?.signal as AbortSignal).aborted).toBe(true);
});
it('read-only view cannot decide and recovery keys isolate file version and identity',async()=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValue(json(page));render(<LabelSuggestions {...props} readOnly/>);await screen.findByText(/认证 Agent/);expect(screen.queryByRole('button',{name:'拒绝此建议'})).toBeNull();expect(suggestionSlot('bob',props.file,props.version)).not.toBe(slot);expect(suggestionSlot('alice',props.file,'new')).not.toBe(slot);
 const body={expected_revision:1,base_version_id:props.version,decision:'accept' as const,labels:['reference']};const first=await claimSuggestionIntent(slot,suggestion.suggestion_id,body,new AbortController().signal);const second=await claimSuggestionIntent(slot,'other',{...body,labels:['new']},new AbortController().signal);expect(second).toEqual({intent:first.intent,claimed:false});
});
