import { expect,test } from '@playwright/test';
test.use({trace:'off',video:'off',screenshot:'off'});
test('real group CRUD preserves grant and pin; committed create response loss reuses original key',async({page})=>{
 await page.goto('/');await page.getByRole('link',{name:'使用账号登录'}).click();await page.getByRole('link',{name:'以 Alice 登录'}).click();await expect(page.getByRole('heading',{name:'我的网盘',exact:true})).toBeVisible();await page.goto('/#/agents');
 const dialog=page.getByRole('dialog',{name:'管理 Agent 分组',exact:true});const open=()=>page.getByRole('button',{name:'管理分组',exact:true}).click();
 const acknowledge=async()=>{await dialog.getByRole('button',{name:'确认分组结果并刷新'}).click();await expect(dialog).toHaveCount(0);};
 await open();await dialog.getByRole('button',{name:'新建分组'}).click();await dialog.getByRole('textbox',{name:'分组名称'}).fill('Group CRUD fixture');await dialog.getByRole('button',{name:'确认创建分组'}).click();await expect(dialog.getByText(/原请求已被接受.*Group CRUD fixture/)).toBeVisible();await acknowledge();
 const fixture=await page.evaluate(async()=>{
  const me=(await(await fetch('/api/v1/me')).json()).data;const headers={'Content-Type':'application/json','X-CSRF-Token':me.csrf_token};
  async function send(path:string,method:string,body:unknown,key:string){const response=await fetch('/api/v1'+path,{method,headers:{...headers,'Idempotency-Key':key},body:JSON.stringify(body)});if(!response.ok)throw new Error('fixture_setup_failed');return(await response.json()).data;}
  const groups=(await(await fetch('/api/v1/agent-groups')).json()).data.items;const group=groups.find((g:{name:string})=>g.name==='Group CRUD fixture');if(!group)throw new Error('fixture_group_missing');
  const space=await send('/spaces','POST',{name:'Group CRUD space',connection_id:'platform'},'crud-space');const {grant}=await send('/agent-grants','POST',{name:'Group CRUD Agent',protocol:'http',space_id:space.id,connection_id:'platform',prefix:'references',actions:['files.read'],expiry_mode:'never'},'crud-grant');
  await send('/agent-organizations/'+grant.agent_id,'PATCH',{expected_revision:0,group_id:group.group_id,pinned:true},'crud-org');return{group:group.group_id as string,grant:grant.grant_id as string,agent:grant.agent_id as string};
 });
 await open();await dialog.getByRole('button',{name:'改名 Group CRUD fixture',exact:true}).click();await dialog.getByRole('textbox',{name:'分组名称'}).fill('Renamed CRUD fixture');await dialog.getByRole('button',{name:'确认修改分组名称'}).click();await expect(dialog.getByText(/Renamed CRUD fixture.*版本 2/)).toBeVisible();await acknowledge();
 await open();await dialog.getByRole('button',{name:'删除 Renamed CRUD fixture',exact:true}).click();await expect(dialog.getByText(/所有成员将解除归组/)).toBeVisible();await dialog.getByRole('button',{name:'确认删除分组'}).click();await expect(dialog.getByText(/原删除请求已被接受/)).toBeVisible();await acknowledge();
 const state=await page.evaluate(async f=>{const org=(await(await fetch('/api/v1/agent-organizations/'+f.agent)).json()).data;const grant=(await(await fetch('/api/v1/agent-grants/'+f.grant)).json()).data;const deleted=await fetch('/api/v1/agent-groups/'+f.group);return{group:org.group_id,pin:org.pinned,orgRevision:org.revision,grantRevision:grant.revision,grantState:grant.state,deleted:deleted.status};},fixture);
 expect(state).toEqual({group:null,pin:true,orgRevision:2,grantRevision:1,grantState:'active',deleted:404});
 let key='',body='',creates=0;
 await page.route('**/api/v1/agent-groups',async route=>{if(route.request().method()!=='POST')return route.continue();creates++;key=route.request().headers()['idempotency-key'];body=route.request().postData()||'';const response=await route.fetch();if(response.status()!==201)throw new Error('fixture_create_failed');await route.abort('failed');});
 await open();await dialog.getByRole('button',{name:'新建分组'}).click();await dialog.getByRole('textbox',{name:'分组名称'}).fill('Lost CRUD fixture');await dialog.getByRole('button',{name:'确认创建分组'}).click();await expect(dialog.getByRole('button',{name:'观察当前分组'})).toBeVisible();await page.unroute('**/api/v1/agent-groups');await page.reload();await open();
 await expect(dialog.getByRole('button',{name:'确认重试原分组请求'})).toHaveCount(0);await dialog.getByRole('button',{name:'观察当前分组'}).click();await expect(dialog.getByText(/名称匹配或未找到都不是/)).toBeVisible();
 const retry=page.waitForRequest(request=>request.method()==='POST'&&new URL(request.url()).pathname==='/api/v1/agent-groups');await dialog.getByRole('button',{name:'确认重试原分组请求'}).click();const request=await retry;expect(request.headers()['idempotency-key']).toBe(key);expect(request.postData()).toBe(body);await expect(dialog.getByText(/原请求已被接受.*Lost CRUD fixture/)).toBeVisible();await acknowledge();
 expect(creates).toBe(1);const count=await page.evaluate(async()=>((await(await fetch('/api/v1/agent-groups')).json()).data.items as {name:string}[]).filter(group=>group.name==='Lost CRUD fixture').length);expect(count).toBe(1);
});
