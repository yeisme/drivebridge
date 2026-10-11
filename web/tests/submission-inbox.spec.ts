import { expect,test } from '@playwright/test';
test.use({trace:'off',video:'off',screenshot:'off'});
const archiveFailure = process.env.DRIVEBRIDGE_TEST_ARCHIVE_FAILURE === '1';
for (const decision of ['accept','reject'] as const) test(`real Agent submission fixed preview and human ${decision} preserve separate delivery stages`,async({page,browser,context})=>{
 await page.goto('/');await page.getByRole('link',{name:'使用账号登录'}).click();await page.getByRole('link',{name:'以 Alice 登录'}).click();await expect(page.getByRole('heading',{name:'我的网盘',exact:true})).toBeVisible();
 const fixed=await page.evaluate(async decision=>{
  const me=(await(await fetch('/api/v1/me')).json()).data;
  const humanHeaders={'Content-Type':'application/json','X-CSRF-Token':me.csrf_token};
  const sp=await fetch('/api/v1/spaces',{method:'POST',headers:humanHeaders,body:JSON.stringify({name:'Submission browser fixture',connection_id:'platform'})});if(!sp.ok)throw new Error('fixture_space_failed');const space=(await sp.json()).data;
  const issued=await fetch('/api/v1/agent-grants',{method:'POST',headers:{...humanHeaders,'Idempotency-Key':'submission-browser-grant-'+decision},body:JSON.stringify({name:'Submission browser Agent',protocol:'http',space_id:space.id,connection_id:'platform',prefix:'references',actions:['inbox.submit'],expiry_mode:'never'})});if(!issued.ok)throw new Error('fixture_grant_failed');
  const {credential,grant}=(await issued.json()).data;
  const headers={'Content-Type':'application/json',Authorization:'Bearer '+credential.secret};
  const bytes=new TextEncoder().encode('# Original submitted text\n\n<script>untrusted fixture only</script>');
  const digest=Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',bytes)),b=>b.toString(16).padStart(2,'0')).join('');
  const create=await fetch('/api/v1/inbox-submissions',{method:'POST',credentials:'omit',headers:{...headers,'Idempotency-Key':'submission-browser-upload'},body:JSON.stringify({name:'result.md',mime_type:'text/markdown',size_bytes:bytes.length,sha256:digest})});if(!create.ok)throw new Error('fixture_submission_failed');const upload=(await create.json()).data;
  if(upload.part_path!=='/api/v1/operations/'+encodeURIComponent(upload.operation_id)+'/parts/{index}')throw new Error('unexpected_fixture_part_path');
  const put=await fetch('/api/v1/operations/'+encodeURIComponent(upload.operation_id)+'/parts/0',{method:'PUT',credentials:'omit',headers:{Authorization:headers.Authorization},body:bytes});if(!put.ok)throw new Error('fixture_part_failed');
  const complete=await fetch('/api/v1/operations/'+upload.operation_id+'/complete',{method:'POST',credentials:'omit',headers,body:'{}'});if(!complete.ok)throw new Error('fixture_complete_failed');
  const response=await fetch('/api/v1/inbox-submissions/'+upload.submission_id);if(!response.ok)throw new Error('fixture_read_failed');const sub=(await response.json()).data;
  return{id:sub.submission_id as string,file:sub.file_ref as string,version:sub.version_id as string,state:sub.state as string,agent:grant.agent_id as string};
 },decision);
 expect(fixed.state).toBe('submitted');expect(Boolean(fixed.version)).toBe(true);
 await page.goto('/#/inbox');const inbox=page.getByRole('region',{name:'Agent 提交收件',exact:true});
 const card=inbox.locator(`[data-submission-id="${fixed.id}"]`);
 await expect(card.getByRole('heading',{name:'result.md',exact:true})).toBeVisible();await expect(card.getByText('待人工验收',{exact:true})).toBeVisible();await expect(card.getByText('来自 Submission browser Agent',{exact:true})).toBeVisible();
 await card.locator('summary').click();await expect(card.getByText(new RegExp('认证 Agent：'+fixed.agent))).toBeVisible();await card.locator('summary').click();
 const content=page.waitForRequest(request=>new URL(request.url()).pathname==='/api/v1/files/'+fixed.file+'/content');await inbox.getByRole('button',{name:'查看原提交文件 '+fixed.id,exact:true}).click();
 expect(new URL((await content).url()).searchParams.get('version')).toBe(fixed.version);
 const dialog=page.getByRole('dialog',{name:'收件固定版本预览',exact:true});await expect(dialog.getByLabel('收件原版本纯文本')).toContainText('# Original submitted text');await expect(dialog.locator('pre script')).toHaveCount(0);
 await dialog.getByRole('button',{name:'关闭',exact:true}).click();await page.reload();await expect(card.getByText('待人工验收',{exact:true})).toBeVisible();
 let originalKey='',originalBody='',writes=0;
 if(decision==='reject')await page.route(`**/api/v1/inbox-submissions/${fixed.id}/decision`,async route=>{writes++;originalKey=route.request().headers()['idempotency-key'];originalBody=route.request().postData()||'';const response=await route.fetch();if(response.status()!==200)throw new Error('fixture_decision_failed');await route.abort('failed');});
 await card.getByRole('button',{name:'人工验收或恢复 '+fixed.id,exact:true}).click();const review=page.getByRole('dialog',{name:'人工验收提交收件',exact:true});
 const competing=decision==='accept'?await context.newPage():null;
 let decisionPosts=0;
 const countDecision=(request:import('@playwright/test').Request)=>{if(request.method()==='POST'&&new URL(request.url()).pathname===`/api/v1/inbox-submissions/${fixed.id}/decision`)decisionPosts++;};
 context.on('request',countDecision);
 if(competing){
  await competing.goto('/#/inbox');await competing.getByRole('button',{name:'人工验收或恢复 '+fixed.id,exact:true}).click();
  const otherReview=competing.getByRole('dialog',{name:'人工验收提交收件',exact:true});await otherReview.getByRole('combobox',{name:'人工验收决定'}).click();await competing.getByRole('option',{name:'拒绝（保留文件）',exact:true}).click();await expect(otherReview.getByRole('button',{name:'确认拒绝原版本'})).toBeVisible();
 }
 if(decision==='reject'){await review.getByRole('combobox',{name:'人工验收决定'}).click();await page.getByRole('option',{name:'拒绝（保留文件）',exact:true}).click();}
 await review.getByRole('button',{name:decision==='accept'?'确认接收原版本':'确认拒绝原版本',exact:true}).click();
 if(decision==='reject'){
   await expect(review.getByRole('button',{name:'读取当前验收状态'})).toBeVisible();await page.unroute(`**/api/v1/inbox-submissions/${fixed.id}/decision`);await page.reload();await card.getByRole('button',{name:'人工验收或恢复 '+fixed.id,exact:true}).click();
   await expect(review.getByRole('button',{name:'确认按原键重试验收'})).toHaveCount(0);await review.getByRole('button',{name:'读取当前验收状态'}).click();await expect(review.getByText(/这是当前观察，不是原请求回执/)).toBeVisible();
   const retry=page.waitForRequest(request=>request.method()==='POST'&&request.url().endsWith(`/inbox-submissions/${fixed.id}/decision`));await review.getByRole('button',{name:'确认按原键重试验收'}).click();const sent=await retry;expect(sent.headers()['idempotency-key']).toBe(originalKey);expect(sent.postData()).toBe(originalBody);expect(writes).toBe(1);
 }
 await expect(review.getByText(/原验收请求已被接受/)).toBeVisible();
 if(competing){
  const otherReview=competing.getByRole('dialog',{name:'人工验收提交收件',exact:true});await expect(otherReview.getByText(/验收操作已停止/)).toBeVisible();await expect(otherReview.getByRole('button',{name:'确认拒绝原版本'})).toHaveCount(0);
  const saved=await page.evaluate(id=>Object.entries(localStorage).filter(([key])=>key.startsWith('drivebridge:submission-decision:')&&key.includes(id)).map(([,value])=>{const intent=JSON.parse(value);return{decision:intent.body.decision,revision:intent.body.expected_revision,version:intent.body.version_id};}),fixed.id);
  expect(saved).toEqual([{decision:'accept',revision:2,version:fixed.version}]);expect(decisionPosts).toBe(1);
 }
 await review.getByRole('button',{name:'确认验收结果并刷新'}).click();await expect(review).toHaveCount(0);
 if(competing){
  await competing.reload();await competing.getByRole('button',{name:'人工验收或恢复 '+fixed.id,exact:true}).click();
  const terminal=competing.getByRole('dialog',{name:'人工验收提交收件',exact:true});await expect(terminal.getByText(/当前不是待验收状态/)).toBeVisible();await expect(terminal.getByRole('button',{name:'确认拒绝原版本'})).toHaveCount(0);expect(decisionPosts).toBe(1);
  await competing.close();
 }
 context.off('request',countDecision);
 const readback=await page.evaluate(async fixed=>{const sub=(await(await fetch('/api/v1/inbox-submissions/'+fixed.id)).json()).data;const file=await fetch('/api/v1/files/'+fixed.file+'/content?version='+encodeURIComponent(fixed.version));return{state:sub.state,revision:sub.revision,decision:sub.decision.decision,version:sub.decision.version_id,archive:sub.archive_operation_ref||null,contentStatus:file.status,text:await file.text()};},fixed);
 expect(readback).toMatchObject({state:decision==='accept'?'accepted':'rejected',revision:3,decision,version:fixed.version,archive:null,contentStatus:200});expect(readback.text).toContain('# Original submitted text');
 if(decision==='accept'){
  let archiveKey='',archiveBody='',archiveID='',allocations=0;
  await page.route(`**/api/v1/inbox-submissions/${fixed.id}/archive`,async route=>{allocations++;archiveKey=route.request().headers()['idempotency-key'];archiveBody=route.request().postData()||'';const response=await route.fetch();if(archiveFailure){if(response.status()!==503)throw new Error('fixture_expected_archive_failure');await route.fulfill({response});}else{if(response.status()!==202)throw new Error('fixture_archive_failed');archiveID=(await response.json()).data.operation_ref;await route.abort('failed');}});
  await card.getByRole('button',{name:'归档或恢复 '+fixed.id,exact:true}).click();const archive=page.getByRole('dialog',{name:'归档已接收收件',exact:true});
  await archive.getByRole('textbox',{name:'归档目标目录'}).fill('reviewed/../wrong');await archive.getByRole('button',{name:'确认归档原版本'}).click();await expect(archive.getByText(/目录格式无效/)).toBeVisible();expect(allocations).toBe(0);
  expect(await page.evaluate(()=>Object.keys(localStorage).some(key=>key.startsWith('drivebridge:submission-archive:')))).toBe(false);
  await archive.getByRole('textbox',{name:'归档目标目录'}).fill('reviewed');await archive.getByRole('button',{name:'确认归档原版本'}).click();await expect(archive.getByRole('button',{name:'查询原归档状态'})).toBeVisible();
  await page.unroute(`**/api/v1/inbox-submissions/${fixed.id}/archive`);await page.reload();await card.getByRole('button',{name:'归档或恢复 '+fixed.id,exact:true}).click();await archive.getByRole('button',{name:'查询原归档状态'}).click();await expect(archive.getByText(/当前状态不能证明它属于原请求键/)).toBeVisible();
  const replay=page.waitForRequest(request=>request.method()==='POST'&&request.url().endsWith(`/inbox-submissions/${fixed.id}/archive`));await archive.getByRole('button',{name:'确认按原键重试归档'}).click();const request=await replay;expect(request.headers()['idempotency-key']).toBe(archiveKey);expect(request.postData()).toBe(archiveBody);if(archiveFailure){
    await expect(archive.getByText(/原归档任务尚未核实完成/)).toBeVisible();
    const pending=await page.evaluate(async id=>{const sub=(await(await fetch('/api/v1/inbox-submissions/'+id)).json()).data;const op=(await(await fetch('/api/v1/operations/'+sub.archive_operation_ref)).json()).data.operation;return{state:sub.state,id:op.id,status:op.status,result:sub.archived_file_ref||null};},fixed.id);
    expect(pending.state).toBe('archiving');expect(pending.status).not.toBe('completed');expect(pending.result).toBeNull();archiveID=pending.id;
    const outsider=await browser.newContext();try{
      const bob=await outsider.newPage();await bob.goto('/');await bob.getByRole('link',{name:'使用账号登录'}).click();await bob.getByRole('link',{name:'以 Bob 登录'}).click();await expect(bob.getByRole('heading',{name:'我的网盘',exact:true})).toBeVisible();
      const denied=await bob.evaluate(async id=>{const me=(await(await fetch('/api/v1/me')).json()).data;const response=await fetch('/api/v1/operations/'+id+'/resume',{method:'POST',headers:{'Content-Type':'application/json','X-CSRF-Token':me.csrf_token},body:'{}'});return response.status;},archiveID);expect(denied).toBe(404);
    }finally{await outsider.close();}
    const resumed=page.waitForRequest(request=>request.method()==='POST'&&request.url().endsWith('/operations/'+archiveID+'/resume'));
    await archive.getByRole('button',{name:'确认继续原归档任务'}).click();await resumed;
  }
  await expect(archive.getByText('归档完成，原操作与结果文件已核对。')).toBeVisible();
  const proof=await page.evaluate(async({fixed,archiveID})=>{const sub=(await(await fetch('/api/v1/inbox-submissions/'+fixed.id)).json()).data;const op=(await(await fetch('/api/v1/operations/'+archiveID)).json()).data.operation;const content=await fetch('/api/v1/files/'+op.file_id+'/content?version='+encodeURIComponent(op.version_id));const source=await fetch('/api/v1/files/'+fixed.file+'/content?version='+encodeURIComponent(fixed.version));return{state:sub.state,operation:sub.archive_operation_ref,file:sub.archived_file_ref,opID:op.id,opFile:op.file_id,status:op.status,path:op.path,contentStatus:content.status,text:await content.text(),sourceStatus:source.status};},{fixed,archiveID});
  expect(proof).toMatchObject({state:'archived',operation:archiveID,opID:archiveID,status:'completed',contentStatus:200,sourceStatus:200});expect(proof.file).toBe(proof.opFile);expect(proof.path).toContain('reviewed/');expect(proof.text).toContain('# Original submitted text');expect(allocations).toBe(1);
  await archive.getByRole('button',{name:'确认归档结果并刷新'}).click();await page.reload();await expect(card.getByText('已归档',{exact:true})).toBeVisible();
 }
 const bobContext=await browser.newContext();try{const bob=await bobContext.newPage();await bob.goto('/');await bob.getByRole('link',{name:'使用账号登录'}).click();await bob.getByRole('link',{name:'以 Bob 登录'}).click();await expect(bob.getByRole('heading',{name:'我的网盘',exact:true})).toBeVisible();await bob.goto('/#/inbox');await expect(bob.getByRole('region',{name:'Agent 提交收件'}).getByText('当前范围没有匹配的提交收件。')).toBeVisible();expect((await bob.request.get('/api/v1/inbox-submissions/'+fixed.id)).status()).toBe(404);}finally{await bobContext.close();}
});
