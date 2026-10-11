import { expect, test } from '@playwright/test';
import path from 'node:path';

test.use({ trace: 'off', screenshot: 'off', video: 'off' });
test('official Agent creation and scoped API configuration lead to real calls, human acceptance and fixed archive readback', async ({ page, browser }) => {
  test.setTimeout(120_000);
  const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!;
  expect(artifacts).toBeTruthy();
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.name));
  await page.goto('/');
  await page.getByRole('link', { name: '使用账号登录' }).click();
  await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  const fixture = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    async function write(endpoint: string, body: unknown, key: string) {
      const response = await fetch('/api/v1' + endpoint, { method: 'POST', headers: { ...headers, 'Idempotency-Key': key }, body: JSON.stringify(body) });
      if (!response.ok) throw new Error('agent_loop_setup_failed');
      return (await response.json()).data;
    }
    const space = await write('/spaces', { name: 'Agent complete local loop', connection_id: 'platform' }, 'agent-loop-space');
    const files: { id: string; version: string; path: string }[] = [];
    for (const prefix of ['references', 'private']) {
      const body = new TextEncoder().encode('# Source ' + prefix + '\n\nFixed ordinary input.');
      const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', body)), byte => byte.toString(16).padStart(2, '0')).join('');
      const upload = await write('/uploads', { space_id: space.id, path: prefix + '/input.md', size_bytes: body.length, sha256 }, 'agent-loop-input-' + prefix);
      const part = await fetch('/api/v1/operations/' + upload.id + '/parts/0', { method: 'PUT', headers: { 'X-CSRF-Token': me.csrf_token }, body });
      if (!part.ok) throw new Error('agent_loop_source_part_failed');
      const complete = await write('/operations/' + upload.id + '/complete', {}, 'agent-loop-source-complete-' + prefix);
      files.push({ id: complete.file_id, version: complete.version_id, path: complete.path });
    }
    return { space: space.id as string, files };
  });
  await page.reload(); await page.goto('/#/agents');
  await page.getByRole('button', { name: '接入 Agent', exact: true }).click();
  const create = page.getByRole('dialog', { name: '接入 Agent', exact: true });
  await create.getByRole('textbox', { name: 'Agent 名称' }).fill('Complete loop Agent');
  await create.getByRole('combobox', { name: '授权空间' }).click();
  await page.getByRole('option', { name: 'Agent complete local loop', exact: true }).click();
  await create.getByRole('combobox', { name: '接入协议' }).click();
  await page.getByRole('option', { name: 'HTTP API', exact: true }).click();
  await create.getByRole('textbox', { name: '限定目录' }).fill('references');
  await expect(create.getByRole('checkbox', { name: '读取文件', exact: true })).toBeChecked();
  await expect(create.getByRole('checkbox', { name: '建议标签（不直接修改）', exact: true })).not.toBeChecked();
  await expect(create.getByRole('checkbox', { name: '提交收件（等待人工验收）', exact: true })).not.toBeChecked();
  await expect(create.getByRole('combobox', { name: '授权期限' })).toContainText('长期');
  // The human explicitly adds submit-only delivery, not a broad file-write grant.
  await create.getByRole('checkbox', { name: '提交收件（等待人工验收）', exact: true }).check();
  await create.getByRole('button', { name: '确认创建授权', exact: true }).click();
  await expect(create.getByText('已确认授权：Complete loop Agent', { exact: true })).toBeVisible();
  // Keep the real one-time credential in a browser-side closure. Neither the
  // value nor a response containing it leaves the browser or enters artifacts.
  const client = await page.evaluateHandle(() => {
    const input = document.querySelector<HTMLInputElement>('[aria-label="新协议 Agent 凭据"]');
    const secret = input?.value;
    if (!secret || input?.type !== 'password' || Object.values(localStorage).some(value => value.includes(secret))) throw new Error('agent_loop_credential_boundary');
    async function request(endpoint: string, method = 'GET', body?: unknown, key?: string) {
      const response = await fetch(endpoint, { method, credentials: 'omit', headers: { Authorization: 'Bearer ' + secret, 'Content-Type': 'application/json', ...(key ? { 'Idempotency-Key': key } : {}) }, body: body === undefined ? undefined : JSON.stringify(body) });
      return { status: response.status, data: (await response.json()).data };
    }
    return {
      async run(input: { template: string; space: string; inside: { id: string; version: string }; outside: { id: string; version: string } }) {
        const lines = input.template.split('\n');
        const target = new URL(lines[0].slice(4));
        if (!lines[0].startsWith('GET ') || target.origin !== location.origin || target.pathname !== '/api/v1/files' || target.searchParams.get('space_id') !== input.space || target.searchParams.get('prefix') !== 'references' || lines[1] !== 'Authorization: Bearer <AGENT_TOKEN>') throw new Error('agent_loop_template_mismatch');
        const listed = await request(target.href);
        const read = await fetch('/api/v1/files/' + input.inside.id + '/content?version=' + input.inside.version, { credentials: 'omit', headers: { Authorization: 'Bearer ' + secret } });
        const sourceText = await read.text();
        const deniedList = await request('/api/v1/files?space_id=' + input.space + '&prefix=private');
        const deniedFile = await request('/api/v1/files/' + input.outside.id);
        const body = new TextEncoder().encode('# Agent loop result\n\nReviewed source: ' + sourceText + '\n<script>untrusted result</script>');
        const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', body)), byte => byte.toString(16).padStart(2, '0')).join('');
        const created = await request('/api/v1/inbox-submissions', 'POST', { name: 'agent-loop-result.md', mime_type: 'text/markdown', size_bytes: body.length, sha256 }, 'agent-loop-submit');
        if (created.status !== 201) throw new Error('agent_loop_submission_failed');
        const submission = created.data;
        const part = await fetch('/api/v1/operations/' + submission.operation_id + '/parts/0', { method: 'PUT', credentials: 'omit', headers: { Authorization: 'Bearer ' + secret }, body });
        if (!part.ok) throw new Error('agent_loop_result_part_failed');
        const complete = await request('/api/v1/operations/' + submission.operation_id + '/complete', 'POST', {});
        if (complete.status !== 200) throw new Error('agent_loop_complete_failed');
        const current = await request('/api/v1/inbox-submissions/' + submission.submission_id);
        const deniedDecision = await request('/api/v1/inbox-submissions/' + submission.submission_id + '/decision', 'POST', { expected_revision: current.data.revision, version_id: current.data.version_id, decision: 'accept' }, 'self-decision-refused');
        return { listed: listed.status, fileIDs: listed.data.map((file: { id: string }) => file.id), read: read.status, sourceText, deniedList: deniedList.status, deniedFile: deniedFile.status, deniedDecision: deniedDecision.status, id: current.data.submission_id as string, file: current.data.file_ref as string, version: current.data.version_id as string, state: current.data.state as string, operation: submission.operation_id as string, body: new TextDecoder().decode(body) };
      },
      async probe(space: string) { return (await request('/api/v1/files?space_id=' + space + '&prefix=references')).status; },
    };
  });
  try {
    await create.getByRole('button', { name: '已安全保存凭据', exact: true }).click();
    await expect(create).toHaveCount(0); await expect(page.getByLabel('新协议 Agent 凭据')).toHaveCount(0);
    await page.getByRole('button', { name: '接入配置 Complete loop Agent', exact: true }).click();
    const config = page.getByRole('dialog', { name: 'Agent 接入配置', exact: true });
    const template = (await config.getByLabel('占位接入配置', { exact: true }).textContent())!;
    await expect(config).toContainText('HTTP');
    const fixed = await client.evaluate((client, input) => client.run(input), { template, space: fixture.space, inside: fixture.files[0], outside: fixture.files[1] });
    expect(fixed).toMatchObject({ listed: 200, fileIDs: [fixture.files[0].id], read: 200, deniedList: 403, deniedFile: 403, deniedDecision: 401, state: 'submitted' });
    expect(fixed.sourceText).toContain('# Source references');
    await config.getByRole('button', { name: '关闭', exact: true }).click();
    const grants = await page.request.get('/api/v1/agent-grants?space_id=' + fixture.space);
    expect(grants.status()).toBe(200);
    const grant = (await grants.json()).data.items[0];
    expect(grant).toMatchObject({ name: 'Complete loop Agent', actions: ['files.read', 'inbox.submit'], expiry_mode: 'never', prefix: 'references', revision: 1 });
    await page.getByRole('button', { name: '查看调用记录 Complete loop Agent', exact: true }).click();
    const calls = page.getByRole('dialog', { name: 'Agent 调用记录', exact: true });
    await expect(calls.getByRole('list', { name: '调用记录列表' })).toContainText('files.list');
    await expect(calls.getByText('结果未知 · 未独立证明业务效果', { exact: true }).first()).toBeVisible();
    await calls.getByRole('button', { name: '关闭', exact: true }).click();
    await page.getByRole('button', { name: '查看操作任务 Complete loop Agent', exact: true }).click();
    const tasks = page.getByRole('dialog', { name: 'Agent 操作任务', exact: true });
    await expect(tasks).toContainText('原操作：operation:' + fixed.operation);
    await expect(tasks).toContainText('认证 Agent：' + grant.agent_id);
    await tasks.getByRole('button', { name: '查看原操作详情 ' + fixed.operation, exact: true }).click();
    const detail = page.getByRole('dialog', { name: '操作详情', exact: true });
    await expect(detail.getByLabel('固定输出引用')).toContainText(fixed.version);
    await detail.getByRole('button', { name: '关闭操作详情', exact: true }).click();
    await tasks.getByRole('button', { name: '关闭', exact: true }).click();

    // Hash navigation retains only the test client's closure, never a token global.
    await page.goto('/#/inbox');
    const row = page.getByRole('region', { name: 'Agent 提交收件', exact: true }).locator(`[data-submission-id="${fixed.id}"]`);
    await expect(row.getByRole('heading', { name: 'agent-loop-result.md', exact: true })).toBeVisible();
    await expect(row.getByText('来自 Complete loop Agent', { exact: true })).toBeVisible();
    await row.getByRole('button', { name: '查看原提交文件 ' + fixed.id, exact: true }).click();
    const preview = page.getByRole('dialog', { name: '收件固定版本预览', exact: true });
    await expect(preview.getByLabel('收件原版本纯文本')).toHaveText(fixed.body);
    await expect(preview.locator('pre script')).toHaveCount(0);
    await preview.getByRole('button', { name: '关闭', exact: true }).click();
    await row.getByRole('button', { name: '人工验收或恢复 ' + fixed.id, exact: true }).click();
    const review = page.getByRole('dialog', { name: '人工验收提交收件', exact: true });
    await review.getByRole('button', { name: '确认接收原版本', exact: true }).click();
    await expect(review.getByText(/原验收请求已被接受/)).toBeVisible();
    await review.getByRole('button', { name: '确认验收结果并刷新', exact: true }).click();
    await expect(row.getByText('已接收，尚未归档', { exact: true })).toBeVisible();
    await row.getByRole('button', { name: '归档或恢复 ' + fixed.id, exact: true }).click();
    const archive = page.getByRole('dialog', { name: '归档已接收收件', exact: true });
    await archive.getByRole('textbox', { name: '归档目标目录' }).fill('reviewed');
    await archive.getByRole('button', { name: '确认归档原版本', exact: true }).click();
    await expect(archive.getByText('归档完成，原操作与结果文件已核对。', { exact: true })).toBeVisible();
    await archive.getByRole('button', { name: '确认归档结果并刷新', exact: true }).click();
    await expect(row.getByText('已归档', { exact: true })).toBeVisible();
    await page.goto('/#/agents');
    await page.getByRole('button', { name: '管理授权 Complete loop Agent', exact: true }).click();
    const manage = page.getByRole('dialog', { name: '管理 Agent 授权', exact: true });
    await manage.getByRole('combobox', { name: '变更动作' }).click();
    await page.getByRole('option', { name: '永久撤销', exact: true }).click();
    await manage.getByRole('button', { name: '确认永久撤销', exact: true }).click();
    await expect(manage.getByText('原变更已提交：永久撤销 · 结果版本 2', { exact: true })).toBeVisible();
    await manage.getByRole('button', { name: '确认回执并刷新', exact: true }).click();
    expect(await client.evaluate((client, space) => client.probe(space), fixture.space)).toBe(401);
    await client.dispose();
    await page.reload(); await page.goto('/#/inbox');
    await expect(row.getByText('已归档', { exact: true })).toBeVisible();
    const proof = await page.evaluate(async id => {
      const read = await fetch('/api/v1/inbox-submissions/' + id);
      if (!read.ok) throw new Error('agent_loop_readback_failed');
      const sub = (await read.json()).data;
      const response = await fetch('/api/v1/operations/' + sub.archive_operation_ref);
      if (!response.ok) throw new Error('agent_loop_operation_readback_failed');
      const op = (await response.json()).data.operation;
      const archived = await fetch('/api/v1/files/' + op.file_id + '/content?version=' + op.version_id);
      const source = await fetch('/api/v1/files/' + sub.file_ref + '/content?version=' + sub.version_id);
      return { state: sub.state, decision: sub.decision.decision, decisionVersion: sub.decision.version_id, archiveID: sub.archive_operation_ref, file: sub.archived_file_ref, operation: { id: op.id, state: op.status, source: op.source_id, sourceVersion: op.source_version, file: op.file_id, path: op.path }, archivedStatus: archived.status, sourceStatus: source.status, archived: await archived.text(), source: await source.text() };
    }, fixed.id);
    expect(proof).toMatchObject({ state: 'archived', decision: 'accept', decisionVersion: fixed.version, operation: { id: proof.archiveID, state: 'completed', source: fixed.file, sourceVersion: fixed.version, file: proof.file }, archivedStatus: 200, sourceStatus: 200, archived: fixed.body, source: fixed.body });
    expect(proof.operation.path).toMatch(/^reviewed\//);
    for (const width of [1440, 390]) {
      await page.setViewportSize({ width, height: 900 });
      await page.screenshot({ path: path.join(artifacts, `agent-loop-archived-${width}.png`), animations: 'disabled' });
    }
    const outsider = await browser.newContext();
    try {
      const bob = await outsider.newPage(); await bob.goto('/');
      await bob.getByRole('link', { name: '使用账号登录' }).click(); await bob.getByRole('link', { name: '以 Bob 登录' }).click();
      await expect(bob.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
      for (const endpoint of ['/inbox-submissions/' + fixed.id, '/operations/' + proof.archiveID, '/files/' + proof.file]) expect((await bob.request.get('/api/v1' + endpoint)).status()).toBe(404);
    } finally { await outsider.close(); }
  } finally { await client.dispose(); }
  expect(errors).toEqual([]);
});
