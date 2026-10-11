import { cleanup,fireEvent,render,screen,within } from '@testing-library/react';
import { afterEach,expect,it } from 'vitest';
import fixtures from '../../../../contracts/personal-workspace/fixtures/responses.json';
import { GrantSections } from './GrantSections';
import type { Grant } from './grant-client';
const base=fixtures.GrantPageEnvelope.data.items[0] as Grant;
const grant=(id:string,group:string|null):Grant=>({...base,grant_id:id,agent_id:id,name:id,organization:{agent_id:id,group_id:group,pinned:false,revision:1,updated_at:base.updated_at}});
const card=(g:Grant)=><article role="listitem" key={g.grant_id}><button>{g.name}</button></article>;
afterEach(cleanup);
it('groups only loaded atomic projections and collapses visible groups without hiding controls',()=>{
 render(<GrantSections grouped grants={[grant('A','team'),grant('B',null),grant('C','team')]}>{card}</GrantSections>);
 expect(within(screen.getByRole('region',{name:'分组 team'})).getAllByRole('listitem')).toHaveLength(2);
 expect(screen.getAllByRole('listitem')).toHaveLength(3);
 fireEvent.click(screen.getByRole('button',{name:'全部收起'}));expect(screen.queryAllByRole('listitem')).toHaveLength(0);
 expect(screen.getByRole('button',{name:'展开 分组 team · 已加载 2 项'}).getAttribute('aria-expanded')).toBe('false');
 fireEvent.click(screen.getByRole('button',{name:'展开 分组 team · 已加载 2 项'}));expect(screen.getAllByRole('listitem')).toHaveLength(2);
 fireEvent.click(screen.getByRole('button',{name:'全部展开'}));expect(screen.getAllByRole('listitem')).toHaveLength(3);
});
it('preserves collapsed existing groups on pagination but does not silently collapse newly visible groups',()=>{
 const view=render(<GrantSections grouped grants={[grant('A','team')]}>{card}</GrantSections>);fireEvent.click(screen.getByRole('button',{name:'全部收起'}));
 view.rerender(<GrantSections grouped grants={[grant('A','team'),grant('B','team'),grant('C','new')]}>{card}</GrantSections>);
 expect(screen.getAllByRole('listitem')).toHaveLength(1);expect(screen.getByRole('button',{name:'C'})).toBeTruthy();
});
it('never calls missing organization ungrouped, but leaves default read-only listing compatible',()=>{
 const view=render(<GrantSections grouped grants={[base]}>{card}</GrantSections>);expect(screen.getByRole('alert').textContent).toContain('不能推断为未分组');expect(screen.queryByRole('region',{name:'未分组'})).toBeNull();
 view.rerender(<GrantSections grouped={false} grants={[base]}>{card}</GrantSections>);expect(screen.getByRole('listitem')).toBeTruthy();expect(screen.queryByRole('button',{name:'全部收起'})).toBeNull();
});
