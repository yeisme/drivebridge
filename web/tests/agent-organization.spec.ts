import { expect,test } from '@playwright/test';
test.use({trace:'off',video:'off',screenshot:'off'});
test('real organization assignment and original-key recovery preserve authorization and latest state',async({page,browser})=>{
 await page.goto('/');await page.getByRole('link',{name:'使用账号登录'}).click();await page.getByRole('link',{name:'以 Alice 登录'}).click();await expect(page.getByRole('heading',{name:'我的网盘',exact:true})).toBeVisible();
 const fixture=await page.evaluate(async()=>{
  const me=(await(await fetch('/api/v1/me')).json()).data;const headers={'Content-Type':'application/json','X-CSRF-Token':me.csrf_token};
  async function post(path:string,body:unknown,key:string){const response=await fetch('/api/v1'+path,{method:'POST',headers:{...headers,'Idempotency-Key':key},body:JSON.stringify(body)});if(!response.ok)throw new Error('fixture_setup_failed');return(await response.json()).data;}
  const space=await post('/spaces',{name:'Organization fixture',connection_id:'platform'},'org-space');
  const {grant}=await post('/agent-grants',{name:'Organization fixture',protocol:'http',space_id:space.id,connection_id:'platform',prefix:'references',actions:['files.read'],expiry_mode:'never'},'org-grant');
  const group=await post('/agent-groups',{name:'Research fixture'},'org-group');return{agent:grant.agent_id as string,grant:grant.grant_id as string,group:group.group_id as string};
 });
 await page.goto('/#/agents');const overview=page.getByRole('region',{name:'新协议 Agent 授权'});const dialog=page.getByRole('dialog',{name:'Agent 分组与置顶',exact:true});
 const open=()=>overview.getByRole('button',{name:'分组与置顶 Organization fixture',exact:true}).click();
 const choose=async(name:string)=>{await dialog.getByRole('combobox',{name:'所属分组'}).click();await page.getByRole('option',{name,exact:true}).click();};
 const acknowledge=async(revision:number)=>{await expect(dialog.getByText(`原请求已被服务端接受；返回当前组织版本 ${revision}。`)).toBeVisible();await dialog.getByRole('button',{name:'确认组织结果并刷新'}).click();await expect(dialog).toHaveCount(0);};
 const read=()=>page.evaluate(async agent=>(await(await fetch('/api/v1/agent-organizations/'+agent)).json()).data,fixture.agent);
 await open();await choose('Research fixture');await dialog.getByRole('checkbox',{name:'在个人组织中置顶'}).check();await dialog.getByRole('button',{name:'保存分组与置顶'}).click();await acknowledge(1);
 expect(await read()).toMatchObject({group_id:fixture.group,pinned:true,revision:1});
 await page.reload();await open();await expect(dialog.getByRole('checkbox',{name:'在个人组织中置顶'})).toBeChecked();await choose('未分组');await dialog.getByRole('button',{name:'保存分组与置顶'}).click();await acknowledge(2);expect(await read()).toMatchObject({group_id:null,pinned:true,revision:2});
 let key='',body='',patches=0;
 await page.route(`**/api/v1/agent-organizations/${fixture.agent}`,async route=>{
  if(route.request().method()!=='PATCH')return route.continue();patches++;key=route.request().headers()['idempotency-key'];body=route.request().postData()||'';const response=await route.fetch();if(response.status()!==200)throw new Error('fixture_patch_failed');await route.abort('failed');
 });
 await open();await dialog.getByRole('checkbox',{name:'在个人组织中置顶'}).uncheck();await dialog.getByRole('button',{name:'保存分组与置顶'}).click();await expect(dialog.getByRole('button',{name:'读取当前组织状态'})).toBeVisible();await page.unroute(`**/api/v1/agent-organizations/${fixture.agent}`);
 // Another explicit fixture operation changes latest state after the original commit.
 await page.evaluate(async agent=>{const me=(await(await fetch('/api/v1/me')).json()).data;const response=await fetch('/api/v1/agent-organizations/'+agent,{method:'PATCH',headers:{'Content-Type':'application/json','X-CSRF-Token':me.csrf_token,'Idempotency-Key':'org-later'},body:JSON.stringify({expected_revision:3,pinned:true})});if(!response.ok)throw new Error('fixture_later_failed');},fixture.agent);
 await page.reload();await open();await expect(dialog.getByRole('button',{name:'确认按原键重试原变更'})).toHaveCount(0);await dialog.getByRole('button',{name:'读取当前组织状态'}).click();
 await expect(dialog.getByText(/当前观察：版本 4，已置顶/)).toBeVisible();
 const retried=page.waitForRequest(request=>request.method()==='PATCH'&&request.url().endsWith('/agent-organizations/'+fixture.agent));
 await dialog.getByRole('button',{name:'确认按原键重试原变更'}).click();const request=await retried;expect(request.headers()['idempotency-key']).toBe(key);expect(request.postData()).toBe(body);await acknowledge(4);expect(patches).toBe(1);expect(await read()).toMatchObject({pinned:true,revision:4});
 const grantState=await page.evaluate(async id=>(await(await fetch('/api/v1/agent-grants/'+id)).json()).data,fixture.grant);expect(grantState.revision).toBe(1);expect(grantState.state).toBe('active');expect(grantState.credential_generation).toBe(1);
 const bobContext=await browser.newContext();try{const bob=await bobContext.newPage();await bob.goto('/');await bob.getByRole('link',{name:'使用账号登录'}).click();await bob.getByRole('link',{name:'以 Bob 登录'}).click();await expect(bob.getByRole('heading',{name:'我的网盘',exact:true})).toBeVisible();expect((await bob.request.get('/api/v1/agent-organizations/'+fixture.agent)).status()).toBe(404);}finally{await bobContext.close();}
});
