import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { SubmissionInbox } from './SubmissionInbox';
import type { Submission } from './submission-client';
const sub:Submission={submission_id:'sub-one',revision:2,space_id:'space-one',agent_id:'agent-trusted',grant_id:'grant-one',source_kind:'agent',source_claim:'Claimed assistant',file_ref:'file-one',version_id:'version-original',upload_operation_ref:'upload-one',state:'submitted',created_at:'2026-01-01T00:00:00Z',updated_at:'2026-01-01T00:00:00Z'};
const page={data:{items:[sub],next_cursor:'',observed_at:'2026-01-01T00:00:00Z',total:null,total_kind:'unknown'}};
const json=(data:unknown,status=200)=>new Response(JSON.stringify(data),{status});
afterEach(()=>{cleanup();vi.restoreAllMocks();});
it('leads with real file and Agent names while keeping original identities in a closed disclosure',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValue(json({data:{...page.data,items:[{...sub,file_name:'研究成果.png',agent_name:'资料助理',mime_type:'image/png',size_bytes:2048}]}}));
 render(<SubmissionInbox/>);
 const heading=await screen.findByRole('heading',{name:'研究成果.png'});expect(heading.textContent).not.toContain('sub-one');
 expect(screen.getByText('来自 资料助理')).toBeTruthy();
 const details=screen.getByText('收件 sub-one').closest('details');expect(details?.open).toBe(false);
 expect(screen.getByText(/认证 Agent：agent-trusted/).closest('details')).toBe(details);
 expect(screen.getByText(/固定版本：version-original/).closest('details')).toBe(details);
 expect(fetch).toHaveBeenCalledTimes(1);
});
it('does not promote caller claims to missing old-response display names or show unknown size as zero',async()=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValue(json(page));render(<SubmissionInbox/>);
 await screen.findByRole('heading',{name:'文件名称未提供'});expect(screen.getByText('来自 Agent（名称未提供）')).toBeTruthy();
 expect(screen.getByText('大小未提供')).toBeTruthy();expect(screen.queryByText('0 B')).toBeNull();
 expect(screen.getByText('来源自述（未经身份验证）：Claimed assistant').closest('details')?.open).toBe(false);
});
it.each(['same','removed','error'])('restores a refreshed decision trigger or a stable fallback: %s',async result=>{
 let reads=0;const fetch=vi.spyOn(globalThis,'fetch').mockImplementation(async input=>{
  if(String(input).includes('/inbox-submissions?')){reads++;if(reads>1&&result==='error')return json({},503);return json(reads>1&&result==='removed'?{data:{...page.data,items:[]}}:page);}
  if(String(input).endsWith('/inbox-submissions/sub-one'))return json({data:sub});
  throw new Error('unexpected_test_request');
 });
 render(<SubmissionInbox scope="focus-inbox"/>);const trigger=await screen.findByRole('button',{name:'人工验收或恢复 sub-one'});trigger.focus();fireEvent.click(trigger);
 await screen.findByRole('button',{name:'确认接收原版本'});fireEvent.click(screen.getByRole('button',{name:'关闭'}));
 await waitFor(()=>expect(reads).toBe(2));const restored=await screen.findByRole('button',{name:result==='same'?'人工验收或恢复 sub-one':'刷新提交收件'});
 await waitFor(()=>expect(document.activeElement).toBe(restored));expect(trigger.isConnected).toBe(false);expect(fetch.mock.calls.every(([,request])=>request?.method==='GET')).toBe(true);
});
it('separates uploading, pending decision, trusted identity and source claim',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValue(json({data:{...page.data,items:[sub,{...sub,submission_id:'uploading',state:'uploading',file_ref:undefined,version_id:undefined}]}}));render(<SubmissionInbox/>);
 await screen.findByText('待人工验收');expect(screen.getByText('上传中')).toBeTruthy();expect(screen.getAllByText(/认证 Agent：agent-trusted/)).toHaveLength(2);expect(screen.getAllByText('来源自述（未经身份验证）：Claimed assistant')).toHaveLength(2);
 expect(screen.queryByRole('button',{name:'查看原提交文件 uploading'})).toBeNull();expect(fetch).toHaveBeenCalledTimes(1);
});
it('previews exact owner version as escaped text, not current file version or executable markup',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockImplementation(async input=>{
  const url=String(input);if(url.includes('inbox-submissions?'))return json(page);if(url.endsWith('/inbox-submissions/sub-one'))return json({data:sub});
  const file={id:'file-one',space_id:'space-one',version_id:'new-version',trashed:false,path:'changed.png',mime_type:'image/png',size_bytes:999};
  if(url.endsWith('/files/file-one'))return json({data:{file,ref:'drivebridge://files/file-one/version/new-version'}});
  if(url.includes('/references?'))return json({data:{file:{...file,version_id:'version-original',path:'result.txt',mime_type:'text/plain',size_bytes:60}}});
  return new Response('<script>not executable</script>\nOriginal body');
 });render(<SubmissionInbox/>);fireEvent.click(await screen.findByRole('button',{name:'查看原提交文件 sub-one'}));
 await screen.findByText(/Original body/);expect(fetch.mock.calls.some(([url])=>url==='/api/v1/files/file-one/content?version=version-original')).toBe(true);
 expect(document.querySelector('pre script')).toBeNull();expect(fetch.mock.calls.every(([url])=>!String(url).includes('version=new-version'))).toBe(true);
 fireEvent(window,new Event('drivebridge-context'));expect(screen.queryByText(/Original body/)).toBeNull();await screen.findByText(/列表与预览已清除/);
});
it('refuses mismatched owner binding without fetching bytes',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(page)).mockResolvedValueOnce(json({data:{...sub,version_id:'changed'}}));render(<SubmissionInbox/>);
 fireEvent.click(await screen.findByRole('button',{name:'查看原提交文件 sub-one'}));await screen.findByText(/未改读最新版本/);expect(fetch).toHaveBeenCalledTimes(2);
});
it('sends server filter and resets expired pagination without guessing an empty inbox',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:{...page.data,next_cursor:'next+submission'}})).mockResolvedValueOnce(json({},410)).mockResolvedValueOnce(json({data:{...page.data,items:[]}}));render(<SubmissionInbox/>);
 fireEvent.click(await screen.findByRole('button',{name:'加载更多提交收件'}));await screen.findByText(/提交收件分页已变化/);expect(screen.queryByText('收件 sub-one')).toBeNull();expect(fetch.mock.calls[1][0]).toBe('/api/v1/inbox-submissions?limit=50&cursor=next%2Bsubmission');
 fireEvent.click(screen.getByRole('combobox',{name:'提交收件状态'}));const option=await screen.findByRole('option',{name:'已拒绝（文件保留）'});fireEvent.pointerDown(option,{pointerType:'mouse'});fireEvent.click(option);await screen.findByText('当前范围没有匹配的提交收件。');expect(fetch.mock.calls[2][0]).toBe('/api/v1/inbox-submissions?limit=50&state=rejected');
});
it('clears denied data and drops a late page after scope change',async()=>{
 let finish!:(response:Response)=>void;vi.spyOn(globalThis,'fetch').mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;}));render(<SubmissionInbox/>);await waitFor(()=>expect(finish).toBeDefined());fireEvent(window,new Event('drivebridge-context'));finish(json(page));await screen.findByText(/列表与预览已清除/);expect(screen.queryByText('收件 sub-one')).toBeNull();
});
it('applies deep link space and state as server filters and rejects mismatched rows',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json(page));render(<SubmissionInbox routeHash="#/inbox?submission_space=other&submission_state=accepted"/>);await screen.findByText(/不能将失败当作空收件箱/);expect(fetch.mock.calls[0][0]).toBe('/api/v1/inbox-submissions?limit=50&space_id=other&state=accepted');expect(screen.queryByText('收件 sub-one')).toBeNull();
});
it('does not read a malformed deep link as an unfiltered list',()=>{
 const fetch=vi.spyOn(globalThis,'fetch');render(<SubmissionInbox routeHash="#/inbox?submission_state=bogus"/>);expect(screen.getByText(/未扩大范围查询/)).toBeTruthy();expect(fetch).not.toHaveBeenCalled();
});
it('new deep link discards previous cursor and late page',async()=>{
 let finish!:(response:Response)=>void;
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:{...page.data,next_cursor:'next'}})).mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;})).mockResolvedValueOnce(json({data:{...page.data,items:[{...sub,state:'accepted'}]}}));
 const view=render(<SubmissionInbox routeHash="#/inbox?submission_space=space-one&submission_state=submitted"/>);fireEvent.click(await screen.findByRole('button',{name:'加载更多提交收件'}));view.rerender(<SubmissionInbox routeHash="#/inbox?submission_space=space-one&submission_state=accepted"/>);await screen.findByText('已接收，尚未归档');finish(json(page));await waitFor(()=>expect(screen.queryByText('待人工验收')).toBeNull());expect(fetch.mock.calls[2][0]).not.toContain('cursor=');
});
it('does not offer fixed preview when older servers omit version_id',async()=>{
 vi.spyOn(globalThis,'fetch').mockResolvedValue(json({data:{...page.data,items:[{...sub,version_id:undefined}]}}));render(<SubmissionInbox/>);await screen.findByText(/不使用最新版本替代/);expect(screen.queryByRole('button',{name:'查看原提交文件 sub-one'})).toBeNull();
});
