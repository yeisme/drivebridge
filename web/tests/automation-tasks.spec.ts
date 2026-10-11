import {expect,test} from '@playwright/test';
test.use({trace:'off',video:'off',screenshot:'off'});
test('automation task reads original intent without running rules or changing checkpoints',async({page})=>{
 await page.goto('/');await page.getByRole('link',{name:'使用账号登录'}).click();await page.getByRole('link',{name:'以 Alice 登录'}).click();await expect(page.getByRole('heading',{name:'我的网盘',exact:true})).toBeVisible();
 const original=await page.evaluate(async()=>{
  const me=(await(await fetch('/api/v1/me')).json()).data;const headers={'Content-Type':'application/json','X-CSRF-Token':me.csrf_token};
  async function write(path:string,body:unknown,method='POST',key?:string){const response=await fetch('/api/v1'+path,{method,headers:{...headers,...(key?{'Idempotency-Key':key}:{})},body:JSON.stringify(body)});if(!response.ok)throw new Error('automation_fixture_failed');return(await response.json()).data;}
  const space=await write('/spaces',{name:'Automation task fixture',connection_id:'platform'});const bytes=new TextEncoder().encode('automation fixture');const sha256=Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',bytes)),b=>b.toString(16).padStart(2,'0')).join('');const op=await write('/uploads',{space_id:space.id,path:'auto/source.txt',size_bytes:bytes.length,sha256},'POST','automation-source');
  const part=await fetch('/api/v1/operations/'+op.id+'/parts/0',{method:'PUT',headers:{'X-CSRF-Token':me.csrf_token},body:bytes});if(!part.ok)throw new Error('automation_part_failed');const done=await write('/operations/'+op.id+'/complete',{});
  const rules=await write('/spaces/'+space.id+'/automation-rules',{action:'add_label',action_detail:'rule:task-observation',prefix:'auto',enabled:true},'PUT');const created=await write('/automation-rules/'+rules[0].rule_id+'/run',{});if(created.length!==1)throw new Error('automation_intent_missing');
  const intent=(await(await fetch('/api/v1/automation-intents/'+created[0].intent_id)).json()).data;if(intent.status!=='completed')throw new Error('automation_persisted_state_missing');
  const current=(await(await fetch('/api/v1/spaces/'+space.id+'/automation-rules')).json()).data[0];const labels=(await(await fetch('/api/v1/files/'+done.file_id+'/labels')).json()).data;
  return {id:intent.intent_id as string,space:space.id as string,file:done.file_id as string,rule:current,labels};
 });
 const writes:string[]=[];page.on('request',request=>{const path=new URL(request.url()).pathname;if(path.includes('/automation-')&&request.method()!=='GET')writes.push(path);});
 await page.goto('/#/tasks');const tasks=page.getByRole('region',{name:'持久操作任务',exact:true});await tasks.getByRole('combobox',{name:'任务类别'}).click();await page.getByRole('option',{name:'自动化意图',exact:true}).click();
 const card=tasks.getByRole('listitem').filter({has:page.getByText('原操作：automation_intent:'+original.id,{exact:true})});await expect(card.getByText('显示状态：未知 · 原状态：completed',{exact:true})).toBeVisible();await expect(card.getByText('未提供',{exact:true})).toHaveCount(7);await expect(card.getByRole('button')).toHaveCount(0);await expect(tasks.getByText(/读取自动化意图不会运行规则/)).toBeVisible();
 await tasks.getByRole('button',{name:'刷新操作投影'}).click();await expect(card).toBeVisible();
 const after=await page.evaluate(async original=>{const rule=(await(await fetch('/api/v1/spaces/'+original.space+'/automation-rules')).json()).data[0];const labels=(await(await fetch('/api/v1/files/'+original.file+'/labels')).json()).data;const intent=(await(await fetch('/api/v1/automation-intents/'+original.id)).json()).data;return{rule,labels,id:intent.intent_id,status:intent.status};},original);
 expect(after.rule).toEqual(original.rule);expect(after.labels).toEqual(original.labels);expect(after.id).toBe(original.id);expect(after.status).toBe('completed');expect(writes).toEqual([]);
});
