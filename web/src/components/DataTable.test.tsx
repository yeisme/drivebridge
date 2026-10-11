import {render,screen,fireEvent,cleanup,within} from '@testing-library/react';
import {afterEach,it,expect} from 'vitest';
import {DataTable} from './DataTable';
afterEach(cleanup);
it('renders a complete server page without a second local pagination layer',()=>{
 const data=Array.from({length:25},(_,i)=>({id:String(i),name:`记录 ${i}`}));
 render(<DataTable label="服务端分页" pagination="external" data={data} getRowId={r=>r.id} columns={[{id:'name',header:'名称',cell:r=>r.name}]}/>);
 expect(screen.getAllByRole('row')).toHaveLength(26);
 expect(screen.queryByRole('button',{name:'下一页'})).toBeNull();
 expect(screen.getByText('本页 25 条记录')).toBeTruthy();
});
it('supports numeric alignment without changing raw sorting values',()=>{
 render(<DataTable label="容量" data={[{id:'a',size:9},{id:'b',size:100}]} getRowId={r=>r.id} columns={[{id:'size',header:'大小',align:'right',value:r=>r.size,cell:r=>`${r.size} B`}]}/>);
 fireEvent.click(screen.getByRole('button',{name:'大小'}));
 const first=screen.getAllByRole('row')[1];
 expect(within(first).getByText('9 B').className).toContain('text-right');
 expect(within(first).getByText('9 B').className).toContain('tabular-nums');
});
it('sorts, filters, paginates and keeps selection bound to stable row IDs',()=>{
 const data=Array.from({length:25},(_,i)=>({id:String(i),name:`成员 ${String(i).padStart(2,'0')}`,count:25-i}));
 render(<DataTable label="验收成员" data={data} getRowId={r=>r.id} columns={[{id:'name',header:'名称',cell:r=>r.name},{id:'count',header:'数量',value:r=>r.count,cell:r=>r.count}]}/>);
 expect(screen.getAllByRole('row')).toHaveLength(21);
 fireEvent.click(screen.getByRole('button',{name:'下一页'}));
 expect(screen.getAllByRole('row')).toHaveLength(6);
 fireEvent.click(screen.getByRole('checkbox',{name:'选择记录 24'}));
 expect(screen.getByText('已选 1 项')).toBeTruthy();
 fireEvent.change(screen.getByRole('searchbox',{name:'搜索验收成员'}),{target:{value:'成员 24'}});
 expect(screen.getAllByRole('row')).toHaveLength(2);
 expect(screen.getByRole('checkbox',{name:'选择记录 24'}).getAttribute('aria-checked')).toBe('true');
 fireEvent.change(screen.getByRole('searchbox',{name:'搜索验收成员'}),{target:{value:''}});
 fireEvent.click(screen.getByRole('button',{name:'数量'}));
 const first=screen.getAllByRole('row')[1];
 expect(within(first).getByRole('checkbox').getAttribute('aria-label')).toBe('选择记录 24');
});
