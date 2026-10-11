import { expect,test } from '@playwright/test';
test.use({trace:'off',video:'off',screenshot:'off'});
test('official task kind view follows the original batch through partial publication, completion and cancellation',async({page,browser})=>{
 await page.goto('/');await page.getByRole('link',{name:'使用账号登录'}).click();await page.getByRole('link',{name:'以 Alice 登录'}).click();await expect(page.getByRole('heading',{name:'我的网盘',exact:true})).toBeVisible();
 const batch=await page.evaluate(async()=>{
  const me=(await(await fetch('/api/v1/me')).json()).data;const headers={'Content-Type':'application/json','X-CSRF-Token':me.csrf_token};
  async function post(path:string,body:unknown,key?:string){const response=await fetch('/api/v1'+path,{method:'POST',headers:{...headers,...(key?{'Idempotency-Key':key}:{})},body:JSON.stringify(body)});if(!response.ok)throw new Error('batch_fixture_setup_failed');return(await response.json()).data;}
  const space=await post('/spaces',{name:'Batch task fixture',connection_id:'platform'});
  const opened=await post('/upload-batches',{space_id:space.id,conflict_policy:'fail'},'batch-task-browser');
  const inputs=[];for(const name of ['first.txt','second.txt']){const data=new TextEncoder().encode(name);const sha256=Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',data)),b=>b.toString(16).padStart(2,'0')).join('');inputs.push({relative_path:name,size_bytes:data.length,sha256});}
  await post('/upload-batches/'+opened.batch_id+'/items',{items:inputs});
  return {id:opened.batch_id as string,space:space.id as string,inputs};
 });
 await page.goto('/#/tasks');const tasks=page.getByRole('region',{name:'持久操作任务',exact:true});await tasks.getByRole('combobox',{name:'任务类别'}).click();await page.getByRole('option',{name:'批量上传',exact:true}).click();
 const card=tasks.getByRole('listitem').filter({has:page.getByText('原操作：upload_batch:'+batch.id,{exact:true})});
 const count=(label:string)=>card.locator('dl > div').filter({has:page.getByText(label,{exact:true})}).locator('dd');
 await expect(card.getByText('显示状态：进行中 · 原状态：enumerating',{exact:true})).toBeVisible();await expect(count('总项')).toHaveText('未提供');await expect(card.getByRole('button',{name:/查看原操作详情/})).toHaveCount(0);
 const details=page.getByRole('dialog',{name:'批量条目',exact:true}).getByRole('region',{name:'批量逐项详情',exact:true});
 await card.getByRole('button',{name:'查看批量条目 '+batch.id,exact:true}).click();await expect(details.getByRole('heading',{name:'first.txt',exact:true})).toBeVisible();await expect(details.getByText('尚无关联操作',{exact:true})).toHaveCount(2);await details.getByRole('button',{name:'关闭批量详情'}).click();await expect(card.getByRole('button',{name:'查看批量条目 '+batch.id,exact:true})).toBeFocused();
 await page.evaluate(async id=>{const me=(await(await fetch('/api/v1/me')).json()).data;const response=await fetch('/api/v1/upload-batches/'+id+'/seal',{method:'POST',headers:{'Content-Type':'application/json','X-CSRF-Token':me.csrf_token},body:'{}'});if(!response.ok)throw new Error('batch_fixture_seal_failed');},batch.id);
 await tasks.getByRole('button',{name:'刷新操作投影'}).click();await expect(count('总项')).toHaveText('2');await expect(count('待处理项')).toHaveText('2');
 for(const [index,input] of batch.inputs.entries()){
  const publishedOperation=await page.evaluate(async({batch,input,index})=>{
   const me=(await(await fetch('/api/v1/me')).json()).data;const headers={'Content-Type':'application/json','X-CSRF-Token':me.csrf_token};
   const response=await fetch('/api/v1/uploads',{method:'POST',headers:{...headers,'Idempotency-Key':'batch-browser-item-'+index},body:JSON.stringify({space_id:batch.space,path:input.relative_path,size_bytes:input.size_bytes,sha256:input.sha256,batch_id:batch.id,item_key:input.relative_path+'\u0000'+input.sha256})});if(!response.ok)throw new Error('batch_fixture_upload_failed');const op=(await response.json()).data;
   const put=await fetch('/api/v1/operations/'+op.id+'/parts/0',{method:'PUT',headers:{'X-CSRF-Token':me.csrf_token},body:new TextEncoder().encode(input.relative_path)});if(!put.ok)throw new Error('batch_fixture_part_failed');
   const complete=await fetch('/api/v1/operations/'+op.id+'/complete',{method:'POST',headers,body:'{}'});if(!complete.ok)throw new Error('batch_fixture_complete_failed');return op.id as string;
  },{batch,input,index});
  await tasks.getByRole('button',{name:'刷新操作投影'}).click();await expect(count('成功项')).toHaveText(String(index+1));await expect(count('待处理项')).toHaveText(String(1-index));
  await expect(card.getByText(index===0?'显示状态：进行中 · 原状态：transferring':'显示状态：原操作已完成 · 原状态：completed',{exact:true})).toBeVisible();
  await card.getByRole('button',{name:'查看批量条目 '+batch.id,exact:true}).click();const row=details.getByRole('listitem').filter({has:page.getByRole('heading',{name:input.relative_path,exact:true})});await expect(row.getByText('关联原操作：'+publishedOperation,{exact:true})).toBeVisible();await expect(row.getByText('文件 · 原状态：completed · 已确认发布',{exact:true})).toBeVisible();await expect(details.getByRole('button',{name:/恢复/})).toHaveCount(0);await details.getByRole('button',{name:'重读原批量条目'}).click();await expect(row.getByText('关联原操作：'+publishedOperation,{exact:true})).toBeVisible();await details.getByRole('button',{name:'关闭批量详情'}).click();
 }
 await page.reload();await tasks.getByRole('combobox',{name:'任务类别'}).click();await page.getByRole('option',{name:'批量上传',exact:true}).click();await expect(count('成功项')).toHaveText('2');await expect(count('持久字节')).toHaveText('未提供');
 const cancelled=await page.evaluate(async space=>{const me=(await(await fetch('/api/v1/me')).json()).data;const headers={'Content-Type':'application/json','X-CSRF-Token':me.csrf_token};const created=await fetch('/api/v1/upload-batches',{method:'POST',headers:{...headers,'Idempotency-Key':'batch-cancel-browser'},body:JSON.stringify({space_id:space})});if(!created.ok)throw new Error('batch_fixture_cancel_create_failed');const b=(await created.json()).data;const response=await fetch('/api/v1/upload-batches/'+b.batch_id+'/cancel',{method:'POST',headers,body:'{}'});if(!response.ok)throw new Error('batch_fixture_cancel_failed');return b.batch_id as string;},batch.space);
 await tasks.getByRole('button',{name:'刷新操作投影'}).click();const cancelledCard=tasks.getByRole('listitem').filter({has:page.getByText('原操作：upload_batch:'+cancelled,{exact:true})});await expect(cancelledCard.getByText('显示状态：已取消 · 原状态：cancelled',{exact:true})).toBeVisible();
 const other=await browser.newContext();try{const bob=await other.newPage();await bob.goto('/');await bob.getByRole('link',{name:'使用账号登录'}).click();await bob.getByRole('link',{name:'以 Bob 登录'}).click();await expect(bob.getByRole('heading',{name:'我的网盘',exact:true})).toBeVisible();const response=await bob.request.get('/api/v1/workspace/tasks?kind=upload_batch');expect(response.status()).toBe(200);expect((await response.json()).data.items).toEqual([]);}finally{await other.close();}
});
