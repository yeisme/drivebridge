import { expect, test } from '@playwright/test';

test.use({ trace: 'off', video: 'off', screenshot: 'off' });
test('two submit-only Agents stay isolated after same-key uploads and one grant revocation', async ({ page, browser }) => {
  await page.goto('/');
  await page.getByRole('link', { name: '使用账号登录' }).click();
  await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  // All credentials stay in this evaluation's lexical scope. Return only safe
  // statuses, booleans and public submission identifiers to the test runner.
  const proof = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const humanHeaders = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    async function human(path: string, body: unknown, key: string) {
      const response = await fetch('/api/v1' + path, { method: 'POST', headers: { ...humanHeaders, 'Idempotency-Key': key }, body: JSON.stringify(body) });
      if (!response.ok) throw new Error('fixture_human_setup_failed');
      return (await response.json()).data;
    }
    const space = await human('/spaces', { name: 'Two Agent isolation', connection_id: 'platform' }, 'isolation-space');
    const a = await human('/agent-grants', { name: 'Isolation A', protocol: 'http', space_id: space.id, connection_id: 'platform', prefix: 'refs', actions: ['inbox.submit'], expiry_mode: 'never' }, 'isolation-a');
    const b = await human('/agent-grants', { name: 'Isolation B', protocol: 'http', space_id: space.id, connection_id: 'platform', prefix: 'refs', actions: ['inbox.submit'], expiry_mode: 'never' }, 'isolation-b');
    async function call(secret: string, path: string, method = 'GET', body?: unknown, key?: string) {
      // Every caller below supplies a fixed same-origin API path.
      const response = await fetch('/api/v1' + path, { method, credentials: 'omit', headers: {
        Authorization: 'Bearer ' + secret, 'Content-Type': 'application/json', ...(key ? { 'Idempotency-Key': key } : {}),
      }, body: body === undefined ? undefined : JSON.stringify(body) });
      const envelope = await response.json().catch(() => ({}));
      return { status: response.status, data: envelope.data };
    }
    const bytes = new TextEncoder().encode('# Two Agent fixed original\n\nOnly the human may review this submit-only result.');
    const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), byte => byte.toString(16).padStart(2, '0')).join('');
    const input = { name: 'isolation.md', mime_type: 'text/markdown', size_bytes: bytes.length, sha256 };
    const sa = await call(a.credential.secret, '/inbox-submissions', 'POST', input, 'same-key');
    const sb = await call(b.credential.secret, '/inbox-submissions', 'POST', input, 'same-key');
    if (sa.status !== 201 || sb.status !== 201) throw new Error('fixture_submission_setup_failed');
    const foreign: { read: number; operation: number; part: number; complete: number; spoofFilter: number }[] = [];
    const own: { list: boolean; lookup: boolean }[] = [];
    async function put(secret: string, id: string) {
      return (await fetch('/api/v1/operations/' + encodeURIComponent(id) + '/parts/0', { method: 'PUT', credentials: 'omit', headers: { Authorization: 'Bearer ' + secret }, body: bytes })).status;
    }
    for (const [actor, mine, other, otherAgent] of [[a, sa, sb, b], [b, sb, sa, a]]) {
      const list = await call(actor.credential.secret, '/inbox-submissions?state=uploading');
      const lookup = await call(actor.credential.secret, '/inbox-submissions/lookup', 'POST', { idempotency_key: 'same-key' });
      own.push({ list: list.status === 200 && list.data.items.length === 1 && list.data.items[0].submission_id === mine.data.submission_id,
        lookup: lookup.status === 200 && lookup.data.submission_id === mine.data.submission_id && lookup.data.operation_id === mine.data.operation_id });
      foreign.push({
        read: (await call(actor.credential.secret, '/inbox-submissions/' + other.data.submission_id)).status,
        operation: (await call(actor.credential.secret, '/operations/' + other.data.operation_id)).status,
        part: await put(actor.credential.secret, other.data.operation_id),
        complete: (await call(actor.credential.secret, '/operations/' + other.data.operation_id + '/complete', 'POST', {})).status,
        spoofFilter: (await call(actor.credential.secret, '/inbox-submissions?agent_id=' + otherAgent.grant.agent_id)).status,
      });
    }
    const recoverA = await call(a.credential.secret, '/operations/' + sa.data.operation_id);
    const recoverB = await call(b.credential.secret, '/operations/' + sb.data.operation_id);
    const untouched = recoverA.data.received_parts.length === 0 && recoverB.data.received_parts.length === 0;
    if (await put(a.credential.secret, sa.data.operation_id) !== 200 || await put(b.credential.secret, sb.data.operation_id) !== 200) throw new Error('fixture_own_part_failed');
    const publishA = await call(a.credential.secret, '/operations/' + sa.data.operation_id + '/complete', 'POST', {});
    if (publishA.status !== 200) throw new Error('fixture_own_complete_failed');
    const published = (await (await fetch('/api/v1/inbox-submissions/' + sa.data.submission_id)).json()).data;
    const agentContent = await fetch('/api/v1/files/' + encodeURIComponent(published.file_ref) + '/content?version=' + encodeURIComponent(published.version_id), { credentials: 'omit', headers: { Authorization: 'Bearer ' + a.credential.secret } });
    const contentDenied = agentContent.status; await agentContent.body?.cancel();
    const selfDecision = await call(a.credential.secret, '/inbox-submissions/' + published.submission_id + '/decision', 'POST', { expected_revision: published.revision, version_id: published.version_id, decision: 'accept' }, 'self-decision');
    const selfArchive = await call(a.credential.secret, '/inbox-submissions/' + published.submission_id + '/archive', 'POST', { expected_revision: published.revision, version_id: published.version_id, target_space_id: space.id, target_prefix: 'reviewed' }, 'self-archive');
    const pendingA = await call(a.credential.secret, '/inbox-submissions', 'POST', input, 'pending-before-revoke');
    if (pendingA.status !== 201) throw new Error('fixture_pending_failed');
    await human('/agent-grants/' + a.grant.grant_id + '/revoke', { expected_revision: 1 }, 'isolation-revoke');
    const revoked = {
      create: (await call(a.credential.secret, '/inbox-submissions', 'POST', input, 'after-revoke')).status,
      lookup: (await call(a.credential.secret, '/inbox-submissions/lookup', 'POST', { idempotency_key: 'same-key' })).status,
      read: (await call(a.credential.secret, '/inbox-submissions/' + sa.data.submission_id)).status,
      operation: (await call(a.credential.secret, '/operations/' + pendingA.data.operation_id)).status,
      part: await put(a.credential.secret, pendingA.data.operation_id),
      complete: (await call(a.credential.secret, '/operations/' + pendingA.data.operation_id + '/complete', 'POST', {})).status,
    };
    const bComplete = await call(b.credential.secret, '/operations/' + sb.data.operation_id + '/complete', 'POST', {});
    const bRead = await call(b.credential.secret, '/inbox-submissions/' + sb.data.submission_id);
    const humanA = (await (await fetch('/api/v1/inbox-submissions/' + sa.data.submission_id)).json()).data;
    const humanPending = (await (await fetch('/api/v1/inbox-submissions/' + pendingA.data.submission_id)).json()).data;
    return { independent: sa.data.submission_id !== sb.data.submission_id && sa.data.operation_id !== sb.data.operation_id,
      own, foreign, untouched, contentDenied, selfDecision: selfDecision.status, selfArchive: selfArchive.status, revoked,
      bUnaffected: bComplete.status === 200 && bRead.status === 200 && bRead.data.state === 'submitted',
      pendingUnpublished: humanPending.state === 'uploading' && !humanPending.file_ref && !humanPending.version_id,
      a: { id: humanA.submission_id as string, version: humanA.version_id as string, state: humanA.state as string, operation:sa.data.operation_id as string, agent:a.grant.agent_id as string },
      bOperation:sb.data.operation_id as string, pendingOperation:pendingA.data.operation_id as string,
      b: { id: sb.data.submission_id as string }, pending: pendingA.data.submission_id as string };
  });
  expect(proof.independent).toBe(true);
  expect(proof.own).toEqual([{ list: true, lookup: true }, { list: true, lookup: true }]);
  expect(proof.foreign).toEqual(Array.from({ length: 2 }, () => ({ read: 404, operation: 403, part: 403, complete: 403, spoofFilter: 403 })));
  expect(proof.untouched).toBe(true);
  // Unsupported routes/actions are refused at workspace credential ingress.
  expect(proof.contentDenied).toBe(401); expect(proof.selfDecision).toBe(401); expect(proof.selfArchive).toBe(401);
  expect(proof.revoked).toEqual({ create: 401, lookup: 401, read: 401, operation: 401, part: 401, complete: 401 });
  expect(proof.bUnaffected).toBe(true); expect(proof.pendingUnpublished).toBe(true); expect(proof.a.state).toBe('submitted');
  await page.goto('/#/agents');
  await page.getByRole('button',{name:'查看操作任务 Isolation A',exact:true}).click();
  const tasks=page.getByRole('dialog',{name:'Agent 操作任务',exact:true});
  await expect(tasks.getByText('原操作：operation:'+proof.a.operation,{exact:true})).toBeVisible();
  await expect(tasks.getByText('原操作：operation:'+proof.pendingOperation,{exact:true})).toBeVisible();
  await expect(tasks.getByText('原操作：operation:'+proof.bOperation,{exact:true})).toHaveCount(0);
  await expect(tasks.getByText('显示状态：原操作已完成 · 原状态：completed',{exact:true})).toBeVisible();
  await expect(tasks.getByText('显示状态：待恢复 · 原状态：awaiting_upload',{exact:true})).toBeVisible();
  await expect(tasks.getByText('认证 Agent：'+proof.a.agent,{exact:true})).toHaveCount(2);
  const automatic=await page.waitForResponse(response=>{const url=new URL(response.url());return url.pathname==='/api/v1/workspace/tasks'&&url.searchParams.get('agent_id')===proof.a.agent&&response.request().method()==='GET';},{timeout:10000});expect(automatic.status()).toBe(200);
  await tasks.getByRole('button',{name:'查看原操作详情 '+proof.a.operation,exact:true}).click();
  const detailSheet=page.getByRole('dialog',{name:'操作详情',exact:true});
  const operationDetail=detailSheet.getByRole('region',{name:'原操作详情',exact:true});await expect(operationDetail.getByText(proof.a.version,{exact:true})).toBeVisible();
  await page.keyboard.press('Escape');await expect(detailSheet).toHaveCount(0);await expect(tasks).toBeVisible();
  await expect(tasks.getByRole('button',{name:'查看原操作详情 '+proof.a.operation,exact:true})).toBeFocused();
  await tasks.getByRole('button',{name:'查看原操作详情 '+proof.pendingOperation,exact:true}).click();await expect(operationDetail.getByText('原操作尚未发布输出',{exact:true})).toBeVisible();await expect(operationDetail.getByLabel('固定输出引用')).toHaveCount(0);
  await operationDetail.getByRole('button',{name:'关闭操作详情'}).click();
  await tasks.getByRole('button',{name:'关闭',exact:true}).click();
  await page.goto('/#/inbox');
  const inbox = page.getByRole('region', { name: 'Agent 提交收件', exact: true });
  for (const id of [proof.a.id, proof.b.id, proof.pending]) await expect(inbox.locator(`[data-submission-id="${id}"]`).getByRole('heading')).toBeVisible();
  const pendingCard = inbox.locator(`[data-submission-id="${proof.pending}"]`);
  await expect(pendingCard.getByText('上传中', { exact: true })).toBeVisible();
  await expect(pendingCard.getByRole('button', { name: /查看原提交文件/ })).toHaveCount(0);
  await inbox.getByRole('button', { name: '查看原提交文件 ' + proof.a.id, exact: true }).click();
  const preview = page.getByRole('dialog', { name: '收件固定版本预览', exact: true });
  await expect(preview.getByLabel('收件原版本纯文本')).toContainText('# Two Agent fixed original');
  await preview.locator('summary').click();
  await expect(preview.getByText('提交版本：' + proof.a.version, { exact: true })).toBeVisible();
  await preview.getByRole('button', { name: '关闭', exact: true }).click();
  const bobContext = await browser.newContext();
  try {
    const bob = await bobContext.newPage(); await bob.goto('/');
    await bob.getByRole('link', { name: '使用账号登录' }).click(); await bob.getByRole('link', { name: '以 Bob 登录' }).click();
    await expect(bob.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible(); await bob.goto('/#/inbox');
    await expect(bob.getByRole('region', { name: 'Agent 提交收件' }).getByText('当前范围没有匹配的提交收件。')).toBeVisible();
    for (const id of [proof.a.id, proof.b.id, proof.pending]) expect((await bob.request.get('/api/v1/inbox-submissions/' + id)).status()).toBe(404);
  } finally { await bobContext.close(); }
});
