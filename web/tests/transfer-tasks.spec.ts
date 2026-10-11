import {expect,test} from '@playwright/test';
test.use({trace:'off',video:'off',screenshot:'off'});
test('transfer tasks honor deployment gate and distinguish copy from source cleanup',async({page})=>{
 await page.goto('/');await page.getByRole('link',{name:'使用账号登录'}).click();await page.getByRole('link',{name:'以 Alice 登录'}).click();await expect(page.getByRole('heading',{name:'我的网盘',exact:true})).toBeVisible();
 if(process.env.DRIVEBRIDGE_TEST_TRANSFER_PLANS!=='1'){
  await page.goto('/#/tasks');const tasks=page.getByRole('region',{name:'持久操作任务',exact:true});await tasks.getByRole('combobox',{name:'任务类别'}).click();await page.getByRole('option',{name:'传输计划',exact:true}).click();await expect(tasks.getByText(/未启用此任务类别/)).toBeVisible();await expect(tasks.getByText('当前授权范围及筛选下没有操作任务。',{exact:true})).toHaveCount(0);return;
 }
 const plans=await page.evaluate(async()=>{
  const me=(await(await fetch('/api/v1/me')).json()).data;const headers={'Content-Type':'application/json','X-CSRF-Token':me.csrf_token};
  async function post(path:string,body:unknown,key?:string){const response=await fetch('/api/v1'+path,{method:'POST',headers:{...headers,...(key?{'Idempotency-Key':key}:{})},body:JSON.stringify(body)});if(!response.ok)throw new Error('transfer_fixture_failed');return(await response.json()).data;}
  const space=await post('/spaces',{name:'Transfer task fixture',connection_id:'platform'});const bytes=new TextEncoder().encode('transfer fixture');const sha256=Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',bytes)),b=>b.toString(16).padStart(2,'0')).join('');const op=await post('/uploads',{space_id:space.id,path:'source.txt',size_bytes:bytes.length,sha256},'transfer-source');
  const part=await fetch('/api/v1/operations/'+op.id+'/parts/0',{method:'PUT',headers:{'X-CSRF-Token':me.csrf_token},body:bytes});if(!part.ok)throw new Error('transfer_part_failed');const done=await post('/operations/'+op.id+'/complete',{});const stat=(await(await fetch('/api/v1/files/'+done.file_id)).json()).data;
  const plans=[];for(const mode of ['copy','migrate']){const plan=await post('/transfer-plans',{sources:[{ref:stat.ref}],target_space_id:space.id,target_path:mode,mode,conflict_policy:'fail'},'task-'+mode);plans.push({id:plan.plan_id as string,revision:plan.revision as number,mode});}return plans;
 });
 await page.goto('/#/tasks');const tasks=page.getByRole('region',{name:'持久操作任务',exact:true});await tasks.getByRole('combobox',{name:'任务类别'}).click();await page.getByRole('option',{name:'传输计划',exact:true}).click();
 for(const plan of plans){
  const card=tasks.getByRole('listitem').filter({has:page.getByText('原操作：transfer_plan:'+plan.id,{exact:true})});await expect(card.getByText('显示状态：待确认 · 原状态：planned',{exact:true})).toBeVisible();await expect(card.getByRole('button')).toHaveCount(0);
  const revision=await page.evaluate(async plan=>{const me=(await(await fetch('/api/v1/me')).json()).data;const response=await fetch('/api/v1/transfer-plans/'+plan.id+'/execute',{method:'POST',headers:{'Content-Type':'application/json','X-CSRF-Token':me.csrf_token},body:JSON.stringify({expected_revision:plan.revision})});if(!response.ok)throw new Error('transfer_execute_failed');const view=(await response.json()).data;if(view.phase!==(plan.mode==='copy'?'copied':'source_cleanup_pending'))throw new Error('transfer_phase_unexpected');return view.revision as number;},plan);
  await tasks.getByRole('button',{name:'刷新操作投影'}).click();await expect(card.getByText(plan.mode==='copy'?'显示状态：未知 · 原状态：copied':'显示状态：待确认 · 原状态：source_cleanup_pending',{exact:true})).toBeVisible();await expect(card.getByText('未提供',{exact:true})).toHaveCount(7);
  if(plan.mode==='migrate'){
   await expect(card.getByText(/源清理待处理：/)).toBeVisible();await page.evaluate(async({id,revision})=>{const me=(await(await fetch('/api/v1/me')).json()).data;const response=await fetch('/api/v1/transfer-plans/'+id+'/cleanup-source',{method:'POST',headers:{'Content-Type':'application/json','X-CSRF-Token':me.csrf_token},body:JSON.stringify({expected_revision:revision})});if(!response.ok)throw new Error('transfer_cleanup_failed');},{id:plan.id,revision});await tasks.getByRole('button',{name:'刷新操作投影'}).click();await expect(card.getByText('显示状态：未知 · 原状态：completed',{exact:true})).toBeVisible();
  }
 }
});
