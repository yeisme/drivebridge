import { expect,test } from '@playwright/test';
test.use({trace:'off',video:'off',screenshot:'off'});
test('official authorization state filter survives reload and stays isolated from another user',async({page,browser})=>{
 await page.goto('/');await page.getByRole('link',{name:'使用账号登录'}).click();await page.getByRole('link',{name:'以 Alice 登录'}).click();await expect(page.getByRole('heading',{name:'我的网盘',exact:true})).toBeVisible();
 await page.evaluate(async()=>{
  const me=(await(await fetch('/api/v1/me')).json()).data;const headers={'Content-Type':'application/json','X-CSRF-Token':me.csrf_token};
  async function send(path:string,method:string,body:unknown,key:string){const response=await fetch('/api/v1'+path,{method,headers:{...headers,'Idempotency-Key':key},body:JSON.stringify(body)});if(!response.ok)throw new Error('fixture_setup_failed');return(await response.json()).data;}
  const space=await send('/spaces','POST',{name:'State filter fixture',connection_id:'platform'},'state-space');
  for(const [name,disabled] of [['State active',false],['State disabled',true]] as const){const {grant}=await send('/agent-grants','POST',{name,protocol:'http',space_id:space.id,connection_id:'platform',prefix:'references',actions:['files.read'],expiry_mode:'never'},name.replaceAll(' ','-'));if(disabled)await send('/agent-grants/'+grant.grant_id,'PATCH',{expected_revision:1,enabled:false},'state-disable');}
 });
 await page.goto('/#/agents');const overview=page.getByRole('region',{name:'新协议 Agent 授权'});const cards=overview.getByRole('list',{name:'新协议授权列表'});
 await expect(cards.getByRole('heading',{name:'State active',exact:true})).toBeVisible();
 await overview.getByRole('combobox',{name:'授权状态筛选'}).click();const filtered=page.waitForRequest(request=>new URL(request.url()).pathname==='/api/v1/agent-grants'&&new URL(request.url()).searchParams.get('state')==='disabled');await page.getByRole('option',{name:'已禁用',exact:true}).click();await filtered;
 await expect(cards.getByRole('heading',{name:'State disabled',exact:true})).toBeVisible();await expect(cards.getByRole('heading',{name:'State active',exact:true})).toHaveCount(0);
 await page.reload();await expect(overview.getByRole('combobox',{name:'授权状态筛选'})).toContainText('已禁用');await expect(cards.getByRole('heading',{name:'State disabled',exact:true})).toBeVisible();await expect(cards.getByRole('heading',{name:'State active',exact:true})).toHaveCount(0);
 // Copy only public UI preferences into Bob's browser, never cookies or tokens.
 const preferences=await page.evaluate(()=>Object.entries(localStorage).filter(([key])=>key.startsWith('drivebridge:grant-state:')));expect(preferences).toHaveLength(1);expect(preferences[0][1]).toBe('disabled');
 const bobContext=await browser.newContext();try{const bob=await bobContext.newPage();await bob.goto('/');await bob.evaluate(values=>{for(const [key,value] of values)localStorage.setItem(key,value);},preferences);await bob.getByRole('link',{name:'使用账号登录'}).click();await bob.getByRole('link',{name:'以 Bob 登录'}).click();await expect(bob.getByRole('heading',{name:'我的网盘',exact:true})).toBeVisible();await bob.goto('/#/agents');await expect(bob.getByRole('combobox',{name:'授权状态筛选'})).toContainText('全部状态');}finally{await bobContext.close();}
});
