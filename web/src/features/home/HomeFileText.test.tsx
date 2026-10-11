import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {afterEach,expect,it,vi} from 'vitest';
import {HomeFileText} from './HomeFileText';
import {NOTEBOOK_READ_LIMIT} from '../notebook/notebook-adapters';
const metadata=()=>new Response(JSON.stringify({data:{file:{id:'file',space_id:'space',version_id:'latest',trashed:false}}}));
const props={space:'space',file:'file',version:'original',onDenied:vi.fn()};
afterEach(()=>{cleanup();vi.restoreAllMocks();vi.clearAllMocks();});
it('uses fixed version with authorized metadata and renders untrusted content only as text',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(metadata()).mockResolvedValueOnce(new Response('<script>window.attack=true</script>\n# original'));
 render(<HomeFileText {...props}/>);const body=await screen.findByLabelText('首页原版本纯文本');expect(body.textContent).toContain('<script>');expect(body.querySelector('script')).toBeNull();expect(fetch.mock.calls[1][0]).toBe('/api/v1/files/file/content?version=original');expect(fetch.mock.calls[1][1]?.redirect).toBe('error');
});
it('permits an empty fixed document',async()=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(metadata()).mockResolvedValueOnce(new Response(''));render(<HomeFileText {...props}/>);await screen.findByText('（空文件）');
});
it.each(['nul','utf8','size','missing'])('refuses %s without falling back to latest',async kind=>{
 const content=kind==='nul'?new Response('a\0b'):kind==='utf8'?new Response(new Uint8Array([0xff])):kind==='size'?new Response('x',{headers:{'Content-Length':String(NOTEBOOK_READ_LIMIT+1)}}):new Response('',{status:404});
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(metadata()).mockResolvedValueOnce(content);render(<HomeFileText {...props}/>);await screen.findByText(/非 UTF-8 文本或超过读取预算/);expect(screen.queryByLabelText('首页原版本纯文本')).toBeNull();expect(fetch).toHaveBeenCalledTimes(2);
});
it('enforces the stream budget without relying on Content-Length',async()=>{
 const content=new ReadableStream<Uint8Array>({start(controller){controller.enqueue(new Uint8Array(NOTEBOOK_READ_LIMIT+1));controller.close();}});
 vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(metadata()).mockResolvedValueOnce(new Response(content));render(<HomeFileText {...props}/>);await screen.findByText(/超过读取预算/);expect(screen.queryByLabelText('首页原版本纯文本')).toBeNull();
});
it('body revocation notifies the parent to clear metadata as well',async()=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(metadata()).mockResolvedValueOnce(new Response('',{status:403}));render(<HomeFileText {...props}/>);await screen.findByText(/权限或范围已变化/);expect(props.onDenied).toHaveBeenCalledTimes(1);
});
it('clears the loaded body on offline without automatically fetching on online',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(metadata()).mockResolvedValueOnce(new Response('private synthetic body'));render(<HomeFileText {...props}/>);await screen.findByText('private synthetic body');fireEvent(window,new Event('offline'));await screen.findByText(/当前离线/);expect(screen.queryByText('private synthetic body')).toBeNull();fireEvent(window,new Event('online'));expect(fetch).toHaveBeenCalledTimes(2);
});
it('ignores a late body response after context invalidation',async()=>{
 let finish!:(value:Response)=>void;
 vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(metadata()).mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;}));render(<HomeFileText {...props}/>);await waitFor(()=>expect(finish).toBeDefined());fireEvent(window,new Event('drivebridge-context'));finish(new Response('late secret'));await screen.findByText(/权限或范围已变化/);await waitFor(()=>expect(screen.queryByText('late secret')).toBeNull());
});
