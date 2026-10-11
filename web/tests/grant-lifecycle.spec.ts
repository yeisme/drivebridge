import { expect, test } from '@playwright/test';

test.use({ trace:'off',video:'off',screenshot:'off' });
test('real lifecycle changes and historical receipt recovery do not confuse current grant state',async({page})=>{
  await page.goto('/');await page.getByRole('link',{name:'使用账号登录'}).click();await page.getByRole('link',{name:'以 Alice 登录'}).click();await expect(page.getByRole('heading',{name:'我的网盘',exact:true})).toBeVisible();
  const id=await page.evaluate(async()=>{
    const me=(await(await fetch('/api/v1/me')).json()).data;
    const headers={'Content-Type':'application/json','X-CSRF-Token':me.csrf_token};
    const spaceRes=await fetch('/api/v1/spaces',{method:'POST',headers,body:JSON.stringify({name:'Lifecycle fixture',connection_id:'platform'})});if(!spaceRes.ok)throw new Error('fixture_space_failed');const space=(await spaceRes.json()).data;
    const created=await fetch('/api/v1/agent-grants',{method:'POST',headers:{...headers,'Idempotency-Key':'lifecycle-fixture-create'},body:JSON.stringify({name:'Lifecycle fixture',protocol:'http',space_id:space.id,connection_id:'platform',prefix:'references',actions:['files.read'],expiry_mode:'never'})});if(!created.ok)throw new Error('fixture_grant_failed');
    return (await created.json()).data.grant.grant_id as string;
  });
  await page.goto('/#/agents');
  const overview=page.getByRole('region',{name:'新协议 Agent 授权'});
  const dialog=page.getByRole('dialog',{name:'管理 Agent 授权',exact:true});
  const open=()=>overview.getByRole('button',{name:'管理授权 Lifecycle fixture',exact:true}).click();
  const choose=async(label:string,choice:string)=>{await dialog.getByRole('combobox',{name:label}).click();await page.getByRole('option',{name:choice,exact:true}).click();};
  const acknowledge=async(text:string)=>{await expect(dialog.getByText(text,{exact:true})).toBeVisible();await dialog.getByRole('button',{name:'确认回执并刷新'}).click();await expect(dialog).toHaveCount(0);};
  const read=()=>page.evaluate(async grantID=>{const res=await fetch('/api/v1/agent-grants/'+grantID);if(!res.ok)throw new Error('readback_failed');const grant=(await res.json()).data;return {state:grant.state,revision:grant.revision,generation:grant.credential_generation,mode:grant.expiry_mode,expires:grant.expires_at,changed:grant.expiry_changed_at};},id);
  await open();await dialog.getByRole('button',{name:'确认禁用授权'}).click();await acknowledge('原变更已提交：禁用授权 · 结果版本 2');expect((await read()).state).toBe('disabled');
  await open();await dialog.getByRole('button',{name:'确认恢复授权'}).click();await acknowledge('原变更已提交：恢复授权 · 结果版本 3');expect((await read()).state).toBe('active');
  await open();await choose('变更动作','修改期限');await choose('新的授权期限','1 小时');await dialog.getByRole('button',{name:'确认修改期限'}).click();await acknowledge('原变更已提交：修改期限 · 结果版本 4');
  const timed=await read();expect(timed.mode).toBe('duration');expect((Date.parse(timed.expires)-Date.parse(timed.changed))/1000).toBe(3600);expect(timed.generation).toBe(1);

  let key='',writes=0;
  await page.route(`**/api/v1/agent-grants/${id}`,async route=>{
    if(route.request().method()!=='PATCH')return route.continue();
    writes++;key=route.request().headers()['idempotency-key'];const response=await route.fetch();if(response.status()!==200)throw new Error('fixture_change_failed');await route.abort('failed');
  });
  await open();await dialog.getByRole('button',{name:'确认禁用授权'}).click();await expect(dialog.getByRole('button',{name:'查询原变更结果'})).toBeVisible();
  await page.unroute(`**/api/v1/agent-grants/${id}`);
  // A later, independent fixture change supersedes state, not the old receipt.
  await page.evaluate(async grantID=>{
    const me=(await(await fetch('/api/v1/me')).json()).data;
    const res=await fetch('/api/v1/agent-grants/'+grantID,{method:'PATCH',headers:{'Content-Type':'application/json','X-CSRF-Token':me.csrf_token,'Idempotency-Key':'lifecycle-fixture-later'},body:JSON.stringify({expected_revision:5,enabled:true})});if(!res.ok)throw new Error('fixture_later_failed');
  },id);
  await page.reload();await open();const request=page.waitForRequest(req=>req.url().endsWith(`/agent-grants/${id}/changes/lookup`));await dialog.getByRole('button',{name:'查询原变更结果'}).click();
  expect((await request).postDataJSON()).toEqual({action:'disable',idempotency_key:key});await acknowledge('原变更已提交：禁用授权 · 结果版本 5');expect(writes).toBe(1);
  const latest=await read();expect(latest.state).toBe('active');expect(latest.revision).toBe(6);expect(latest.expires).toBe(timed.expires);
  await open();await choose('变更动作','永久撤销');await dialog.getByRole('button',{name:'确认永久撤销'}).click();await acknowledge('原变更已提交：永久撤销 · 结果版本 7');
  expect((await read()).state).toBe('revoked');await open();await expect(dialog.getByText(/不能恢复或续期/)).toBeVisible();await expect(dialog.getByRole('combobox',{name:'变更动作'})).toHaveCount(0);
});
