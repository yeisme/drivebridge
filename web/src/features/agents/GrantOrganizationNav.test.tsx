import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import fixtures from '../../../../contracts/personal-workspace/fixtures/responses.json';
import { GrantOverview } from './GrantOverview';
import { GrantOrganizationNav } from './GrantOrganizationNav';
const json=(data:unknown,status=200)=>new Response(JSON.stringify(data),{status});
const group=fixtures.AgentGroupView;
const grantPage=(input:unknown)=>({data:{...fixtures.GrantPageEnvelope.data,items:fixtures.GrantPageEnvelope.data.items.map(grant=>({...grant,...(String(input).includes('order=pinned')?{organization:{...fixtures.AgentOrganizationView,agent_id:grant.agent_id,group_id:String(input).includes('ungrouped=true')?null:group.group_id}}:{})}))}});
const groups={data:{items:[group],next_cursor:'',observed_at:'2026-01-01T00:00:00Z',total:null,total_kind:'unknown'}};
afterEach(()=>{cleanup();vi.restoreAllMocks();});
it('changes server filters from page one rather than filtering the loaded grant page',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockImplementation(async input=>String(input).includes('agent-groups')?json(groups):json(grantPage(input)));render(<GrantOverview scope="alice"/>);
 await screen.findByText('Reader');await screen.findByRole('button',{name:'分组：Research'});
 for(const [label,query] of [['置顶优先','order=pinned'],['仅置顶','pinned=true&order=pinned'],['未分组','ungrouped=true&order=pinned'],['分组：Research','group_id=group-fixture&order=pinned'],['全部授权','']] as const){
  fireEvent.click(screen.getByRole('button',{name:label}));await waitFor(()=>expect(fetch.mock.calls.some(([url])=>url==='/api/v1/agent-grants?limit=50'+(query?'&'+query:''))).toBe(true));await screen.findByText('Reader');
 }
 expect(fetch.mock.calls.filter(([url])=>String(url).includes('agent-grants')).every(([url])=>!String(url).includes('cursor='))).toBe(true);
});
it('does not let a late prior-filter page repopulate the new view',async()=>{
 let finish!:(response:Response)=>void;
 const fetch=vi.spyOn(globalThis,'fetch').mockImplementation(async input=>{
  if(String(input).includes('agent-groups'))return json(groups);
  if(String(input).includes('pinned=true'))return new Promise(resolve=>{finish=resolve;});
  return json(grantPage(input));
 });render(<GrantOverview scope="alice"/>);await screen.findByText('Reader');
 fireEvent.click(screen.getByRole('button',{name:'仅置顶'}));await waitFor(()=>expect(finish).toBeDefined());fireEvent.click(screen.getByRole('button',{name:'未分组'}));await screen.findByText('Reader');
 finish(json({data:{...fixtures.GrantPageEnvelope.data,items:[{...fixtures.GrantView,name:'Stale pinned'}]}}));await waitFor(()=>expect(screen.queryByText('Stale pinned')).toBeNull());expect(fetch).toHaveBeenCalled();
});
it('loads group navigation pages only on demand and reports expiry without a fake empty result',async()=>{
 const fetch=vi.spyOn(globalThis,'fetch').mockResolvedValueOnce(json({data:{...groups.data,next_cursor:'next+group'}})).mockResolvedValueOnce(json({},410)).mockResolvedValueOnce(json(groups));
 render(<GrantOrganizationNav selected={{kind:'all'}} onSelect={vi.fn()} onDenied={vi.fn()}/>);fireEvent.click(await screen.findByRole('button',{name:'更多分组导航'}));await screen.findByText('分组分页已过期。');
 expect(fetch.mock.calls[1][0]).toBe('/api/v1/agent-groups?limit=50&cursor=next%2Bgroup');expect(screen.queryByRole('button',{name:'分组：Research'})).toBeNull();
 fireEvent.click(screen.getByRole('button',{name:'重读分组导航'}));await screen.findByRole('button',{name:'分组：Research'});expect(fetch.mock.calls[2][0]).toBe('/api/v1/agent-groups?limit=50');
});
it('a forbidden organization catalogue clears parent grants and actions',async()=>{
 vi.spyOn(globalThis,'fetch').mockImplementation(async input=>String(input).includes('agent-groups')?json({},403):json(fixtures.GrantPageEnvelope));render(<GrantOverview scope="alice"/>);
 await screen.findByText(/访问权限已失效/);expect(screen.queryByText('Reader')).toBeNull();expect(screen.queryByRole('navigation',{name:'Agent 组织视图'})).toBeNull();
});
