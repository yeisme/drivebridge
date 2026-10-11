import { expect,test } from '@playwright/test';
test.use({trace:'off',video:'off',screenshot:'off'});
test('official grant list uses server-side organization filters and pinned order',async({page})=>{
 await page.goto('/');await page.getByRole('link',{name:'使用账号登录'}).click();await page.getByRole('link',{name:'以 Alice 登录'}).click();await expect(page.getByRole('heading',{name:'我的网盘',exact:true})).toBeVisible();
 const fixture=await page.evaluate(async()=>{
  const me=(await(await fetch('/api/v1/me')).json()).data;const headers={'Content-Type':'application/json','X-CSRF-Token':me.csrf_token};
  async function send(path:string,method:string,body:unknown,key:string){const res=await fetch('/api/v1'+path,{method,headers:{...headers,'Idempotency-Key':key},body:JSON.stringify(body)});if(!res.ok)throw new Error('fixture_setup_failed');return(await res.json()).data;}
  const space=await send('/spaces','POST',{name:'Navigation fixture',connection_id:'platform'},'nav-space');const group=await send('/agent-groups','POST',{name:'Navigation group'},'nav-group');
  const grants=[];for(const name of ['Navigation one','Navigation two']){const {grant}=await send('/agent-grants','POST',{name,protocol:'http',space_id:space.id,connection_id:'platform',prefix:'references',actions:['files.read'],expiry_mode:'never'},name.replaceAll(' ','-'));grants.push({id:grant.grant_id as string,agent:grant.agent_id as string,name});}
  grants.sort((a,b)=>a.id.localeCompare(b.id));const pinned=grants[1],plain=grants[0];await send('/agent-organizations/'+pinned.agent,'PATCH',{expected_revision:0,group_id:group.group_id,pinned:true},'nav-org');return{pinned,plain,group:group.group_id as string};
 });
 await page.goto('/#/agents');const overview=page.getByRole('region',{name:'新协议 Agent 授权'});const cards=overview.locator('article[role="listitem"]');
 const sortedResponse=page.waitForResponse(response=>{const url=new URL(response.url());return url.pathname==='/api/v1/agent-grants'&&url.searchParams.get('order')==='pinned';});
 await overview.getByRole('button',{name:'置顶优先',exact:true}).click();
 const response=await sortedResponse;expect(response.status()).toBe(200);
 const ordered=(await response.json()).data.items as {grant_id:string}[];const ids=ordered.map(item=>item.grant_id);
 expect(ids).toContain(fixture.pinned.id);expect(ids).toContain(fixture.plain.id);expect(ids.indexOf(fixture.pinned.id)).toBeLessThan(ids.indexOf(fixture.plain.id));
 // Group sections preserve server order within each group, not global DOM rank.
 await expect(cards.getByRole('heading',{name:fixture.pinned.name,exact:true})).toBeVisible();await expect(cards.getByRole('heading',{name:fixture.plain.name,exact:true})).toBeVisible();
 await overview.getByRole('button',{name:'全部收起',exact:true}).click();await expect(cards.getByRole('heading',{name:fixture.pinned.name,exact:true})).toBeHidden();await expect(cards.getByRole('heading',{name:fixture.plain.name,exact:true})).toBeHidden();
 await overview.getByRole('button',{name:'全部展开',exact:true}).click();await expect(cards.getByRole('heading',{name:fixture.pinned.name,exact:true})).toBeVisible();await expect(cards.getByRole('heading',{name:fixture.plain.name,exact:true})).toBeVisible();
 const filtered=page.waitForRequest(req=>new URL(req.url()).pathname==='/api/v1/agent-grants'&&new URL(req.url()).searchParams.get('group_id')===fixture.group);
 await overview.getByRole('button',{name:'分组：Navigation group',exact:true}).click();await filtered;
 await expect(cards.getByRole('heading',{name:fixture.pinned.name,exact:true})).toBeVisible();await expect(cards.getByRole('heading',{name:fixture.plain.name,exact:true})).toHaveCount(0);
 await overview.getByRole('button',{name:'未分组',exact:true}).click();await expect(cards.getByRole('heading',{name:fixture.plain.name,exact:true})).toBeVisible();await expect(cards.getByRole('heading',{name:fixture.pinned.name,exact:true})).toHaveCount(0);
 await overview.getByRole('button',{name:'仅置顶',exact:true}).click();await expect(cards.getByRole('heading',{name:fixture.pinned.name,exact:true})).toBeVisible();await expect(cards.getByRole('heading',{name:fixture.plain.name,exact:true})).toHaveCount(0);
 await overview.getByRole('button',{name:'全部授权',exact:true}).click();await expect(cards.getByRole('heading',{name:fixture.plain.name,exact:true})).toBeVisible();await expect(cards.getByRole('heading',{name:fixture.pinned.name,exact:true})).toBeVisible();
});
