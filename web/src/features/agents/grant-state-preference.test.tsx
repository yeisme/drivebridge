import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import fixtures from '../../../../contracts/personal-workspace/fixtures/responses.json';
import { GrantOverview } from './GrantOverview';
import { grantStateKey, readGrantState, useGrantStatePreference } from './grant-state-preference';
const scope=JSON.stringify(['instance','alice','tenant','space']);
const json=(value:unknown)=>new Response(JSON.stringify(value),{status:200});
afterEach(()=>{cleanup();localStorage.clear();vi.restoreAllMocks();});
function mockAPI(){return vi.spyOn(globalThis,'fetch').mockImplementation(async input=>String(input).includes('agent-groups')?json({data:{items:[],next_cursor:''}}):json({data:{...fixtures.GrantPageEnvelope.data,items:fixtures.GrantPageEnvelope.data.items.map(grant=>({...grant,...(String(input).includes('order=pinned')?{organization:{...fixtures.AgentOrganizationView,agent_id:grant.agent_id}}:{})}))}}));}
async function choose(label:string){fireEvent.click(screen.getByRole('combobox',{name:'授权状态筛选'}));const option=await screen.findByRole('option',{name:label});fireEvent.pointerDown(option,{pointerType:'mouse'});fireEvent.click(option);}
it('restores only a validated enum and isolates all four identity dimensions',()=>{
 localStorage.setItem(grantStateKey(scope),'disabled');expect(readGrantState(grantStateKey(scope))).toBe('disabled');
 for(const other of [['other','alice','tenant','space'],['instance','bob','tenant','space'],['instance','alice','other','space'],['instance','alice','tenant','other']])expect(readGrantState(grantStateKey(JSON.stringify(other)))).toBe('all');
 localStorage.setItem(grantStateKey(scope),'{"token":"bad"}');expect(readGrantState(grantStateKey(scope))).toBe('all');
 const get=vi.spyOn(Storage.prototype,'getItem');expect(readGrantState('')).toBe('all');expect(get).not.toHaveBeenCalled();
});
it('initial request restores state and state changes combine with server organization filters',async()=>{
 localStorage.setItem(grantStateKey(scope),'disabled');const fetch=mockAPI();render(<GrantOverview scope={scope}/>);await screen.findByText('Reader');
 expect(fetch.mock.calls.some(([url])=>url==='/api/v1/agent-grants?limit=50&state=disabled')).toBe(true);
 fireEvent.click(screen.getByRole('button',{name:'仅置顶'}));await screen.findByText('Reader');
 await choose('有效');await screen.findByText('Reader');
 expect(fetch.mock.calls.some(([url])=>url==='/api/v1/agent-grants?limit=50&pinned=true&order=pinned&state=active')).toBe(true);
 expect(localStorage.length).toBe(1);expect(localStorage.getItem(grantStateKey(scope))).toBe('active');
});
it('clears cursor and ignores a late response when changing authorization filter',async()=>{
 let finish!:(response:Response)=>void;
 const fetch=vi.spyOn(globalThis,'fetch').mockImplementation(async input=>{
  const url=String(input);if(url.includes('cursor='))return new Promise(resolve=>{finish=resolve;});
  if(url.includes('state=revoked'))return json({data:{...fixtures.GrantPageEnvelope.data,items:[]}});
  return json({data:{...fixtures.GrantPageEnvelope.data,next_cursor:'page-two'}});
 });render(<GrantOverview/>);await screen.findByText('Reader');fireEvent.click(screen.getByRole('button',{name:'加载更多授权'}));await waitFor(()=>expect(finish).toBeDefined());
 await choose('已撤销');await screen.findByText(/匹配所选筛选/);finish(json({data:{...fixtures.GrantPageEnvelope.data,items:[{...fixtures.GrantView,name:'Late grant'}]}}));
 await waitFor(()=>expect(screen.queryByText('Late grant')).toBeNull());expect(fetch.mock.calls.some(([url])=>url==='/api/v1/agent-grants?limit=50&state=revoked')).toBe(true);
});
it('unavailable preference storage does not block server filtering',async()=>{
 vi.spyOn(Storage.prototype,'getItem').mockImplementation(()=>{throw new Error('denied');});vi.spyOn(Storage.prototype,'setItem').mockImplementation(()=>{throw new Error('denied');});const fetch=mockAPI();render(<GrantOverview scope={scope}/>);await screen.findByText('Reader');await choose('已到期');await screen.findByText('Reader');expect(fetch.mock.calls.some(([url])=>url==='/api/v1/agent-grants?limit=50&state=expired')).toBe(true);
});
it('switching the hook identity reads its own preference without writing another scope',()=>{
 function Probe({scope}:{scope:string}){const [value,change]=useGrantStatePreference(scope);return <button onClick={()=>change('revoked')}>{value}</button>;}
 localStorage.setItem(grantStateKey('bob'),'disabled');const view=render(<Probe scope="alice"/>);fireEvent.click(screen.getByRole('button'));expect(localStorage.getItem(grantStateKey('alice'))).toBe('revoked');view.rerender(<Probe scope="bob"/>);expect(screen.getByRole('button').textContent).toBe('disabled');expect(localStorage.getItem(grantStateKey('bob'))).toBe('disabled');
});
