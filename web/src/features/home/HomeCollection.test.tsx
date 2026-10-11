import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {afterEach,expect,it,vi} from 'vitest';
import {HomeCollection} from './HomeCollection';
const collection={collection_id:'collection',name:'Research',kind:'manual' as const,revision:1};
const item={file_id:'file',version_id:'old',path:'note.txt'};
const data={collection_id:'collection',kind:'manual',revision:2,computed_at:'2026-01-01T00:00:00Z',coverage:'complete',visible_count:1,items:[item]};
const props={collection,close:vi.fn(),onDenied:vi.fn(),onOpen:vi.fn()};
const json=(value:unknown,status=200)=>new Response(JSON.stringify({data:value}),{status});
afterEach(()=>{cleanup();vi.restoreAllMocks();vi.clearAllMocks();});
it('shows current collection revision and expands locally without new server pages',async()=>{
 const items=Array.from({length:51},(_,i)=>({...item,file_id:'file-'+i}));const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({...data,items,visible_count:51}));render(<HomeCollection {...props}/>);await screen.findByText(/集合已在首页观察后变化/);expect(screen.getAllByRole('listitem')).toHaveLength(50);fireEvent.click(screen.getByRole('button',{name:'展开更多已读取成员'}));expect(screen.getAllByRole('listitem')).toHaveLength(51);expect(fetch).toHaveBeenCalledTimes(1);expect(fetch.mock.calls[0][0]).toBe('/api/v1/file-collections/collection');
});
it('resolves cross-space original membership with the pinned version rather than latest metadata',async()=>{
 const current={id:'file',space_id:'other-space',version_id:'latest',path:'renamed.txt',trashed:false};const ref='drivebridge://instance/file/file/version/old';
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(data)).mockResolvedValueOnce(json({file:current})).mockResolvedValueOnce(json({ref:'drivebridge://instance/file/file/version/latest'})).mockResolvedValueOnce(json({file:{...current,version_id:'old',size_bytes:1,mime_type:'text/plain',updated_at:data.computed_at},ref}));
 render(<HomeCollection {...props}/>);const button=await screen.findByRole('button',{name:'查看成员固定引用：note.txt'});fireEvent.click(button);await waitFor(()=>expect(props.onOpen).toHaveBeenCalledTimes(1));expect(props.onOpen.mock.calls[0][0]).toMatchObject({space_id:'other-space',version_id:'old',ref});expect(fetch.mock.calls[3][0]).toBe('/api/v1/references?ref='+encodeURIComponent(ref));expect(fetch.mock.calls.every(([url])=>!String(url).includes('/content'))).toBe(true);
});
it.each([{...data,collection_id:'other'},{...data,visible_count:0},{...data,items:[{...item,version_id:''}]},{...data,coverage:'invented'}])('rejects invalid collection envelopes',async invalid=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(invalid));render(<HomeCollection {...props}/>);await screen.findByText(/合同不匹配/);expect(screen.queryByRole('listitem')).toBeNull();
});
it('does not call missing members an empty collection after a failed read',async()=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({},503));render(<HomeCollection {...props}/>);await screen.findByText(/不代表空集合/);expect(screen.queryByText('当前授权下没有可见成员。')).toBeNull();
});
it('re-read replaces the list rather than merging new revisions',async()=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(data)).mockResolvedValueOnce(json({...data,revision:3,visible_count:0,items:[]}));render(<HomeCollection {...props}/>);await screen.findByRole('listitem');fireEvent.click(screen.getByRole('button',{name:'重读集合成员'}));await screen.findByText('当前授权下没有可见成员。');expect(screen.queryByRole('listitem')).toBeNull();
});
it('notifies the parent when collection access is denied',async()=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({},403));render(<HomeCollection {...props}/>);await screen.findByText(/权限或范围变化/);expect(props.onDenied).toHaveBeenCalledTimes(1);
});
it('offline cancels late member metadata and does not open the file',async()=>{
 let finish!:(value:Response)=>void;vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(data)).mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;}));render(<HomeCollection {...props}/>);fireEvent.click(await screen.findByRole('button',{name:'查看成员固定引用：note.txt'}));fireEvent(window,new Event('offline'));finish(json({file:{id:'file',space_id:'space',version_id:'new'}}));await screen.findByText(/当前离线/);await waitFor(()=>expect(props.onOpen).not.toHaveBeenCalled());
});
