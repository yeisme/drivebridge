import { expect, test, type Locator } from '@playwright/test';
import path from 'node:path';

test.use({ trace: 'off', screenshot: 'off', video: 'off' });
test('human automation config, preview, original receipt recovery and real label effects remain usable across themes and widths', async ({ page, browser }) => {
  test.skip(process.env.DRIVEBRIDGE_TEST_AUTOMATION_MANAGEMENT !== '1', 'Requires explicit disposable automation opt-in');
  test.setTimeout(150_000);
  const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!;
  expect(artifacts).toBeTruthy();
  await page.goto('/'); await page.getByRole('link', { name: '使用账号登录' }).click(); await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  const fixture = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    async function post(endpoint: string, body: unknown, key: string) {
      const response = await fetch('/api/v1' + endpoint, { method: 'POST', headers: { ...headers, 'Idempotency-Key': key }, body: JSON.stringify(body) });
      if (!response.ok) throw new Error('automation_fixture_failed'); return (await response.json()).data;
    }
    const space = await post('/spaces', { name: 'Automation management fixture', connection_id: 'platform' }, 'automation-space');
    const files: { id: string; version: string; path: string }[] = [];
    for (const name of ['auto/first.md', 'auto/second-' + 'long-reference-'.repeat(8) + '.md', 'outside/untouched.md']) {
      const bytes = new TextEncoder().encode('Original fixed bytes ' + name);
      const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), b => b.toString(16).padStart(2, '0')).join('');
      const upload = await post('/uploads', { space_id: space.id, path: name, size_bytes: bytes.length, sha256 }, name);
      const part = await fetch('/api/v1/operations/' + upload.id + '/parts/0', { method: 'PUT', headers: { 'X-CSRF-Token': me.csrf_token }, body: bytes });
      if (!part.ok) throw new Error('automation_part_failed');
      const done = await post('/operations/' + upload.id + '/complete', {}, name + '-complete');
      files.push({ id: done.file_id, version: done.version_id, path: name });
    }
    return { space: space.id as string, files };
  });
  await page.reload(); await page.goto('/#/automation');
  const region = page.getByRole('region', { name: '备份与整理', exact: true });
  await region.getByRole('combobox', { name: '整理规则空间' }).click();
  await page.getByRole('option', { name: 'Automation management fixture', exact: true }).click();
  await expect(region.getByText('当前空间没有已保存的整理规则。')).toBeVisible();
  let changes = 0, executions = 0;
  page.on('request', request => { const url = new URL(request.url()); if (request.method() === 'POST' && url.pathname.endsWith('/automation-rules/changes')) changes++; if (request.method() === 'POST' && url.pathname.endsWith('/execute')) executions++; });
  await region.getByRole('button', { name: '配置与运行', exact: true }).click();
  await region.getByRole('button', { name: '新建规则/恢复', exact: true }).click();
  const create = page.getByRole('dialog', { name: '新建整理规则', exact: true });
  await create.getByLabel('规则目录', { exact: true }).fill('auto');
  await create.getByLabel('标签名称', { exact: true }).fill('reviewed');
  await create.getByLabel('单次条目预算', { exact: true }).fill('10');
  await expect(create.getByRole('checkbox', { name: '允许手动运行' })).not.toBeChecked();
  await create.getByRole('button', { name: '预览规则修改', exact: true }).click();
  expect(changes).toBe(0); expect(executions).toBe(0);
  await create.getByRole('button', { name: '确认保存规则', exact: true }).click();
  await expect(create.getByText('已确认原规则变更', { exact: true })).toBeVisible();
  await create.getByRole('button', { name: '确认回执并刷新规则', exact: true }).click();
  const readRules = async () => { const response = await page.request.get('/api/v1/spaces/' + fixture.space + '/automation-rules'); expect(response.status()).toBe(200); return (await response.json()).data as { rule_id: string; enabled: boolean; revision: number; checkpoint: number; action_detail: string }[]; };
  const original = (await readRules())[0];
  expect(original).toMatchObject({ enabled: false, revision: 1, checkpoint: 0, action_detail: 'rule:reviewed' });
  const row = region.getByRole('list', { name: '整理规则列表' }).getByRole('listitem').filter({ has: page.getByRole('heading', { name: '添加标签 · rule:reviewed', exact: true }) });
  const configure = row.getByRole('button', { name: '配置规则 ' + original.rule_id, exact: true });
  async function configureEnabled(enabled: boolean) {
    await configure.click(); const dialog = page.getByRole('dialog', { name: '配置整理规则', exact: true });
    await dialog.getByRole('checkbox', { name: '允许手动运行' }).setChecked(enabled);
    await dialog.getByRole('button', { name: '预览规则修改', exact: true }).click();
    await dialog.getByRole('button', { name: '确认保存规则', exact: true }).click();
    await expect(dialog.getByText('已确认原规则变更', { exact: true })).toBeVisible();
    await dialog.getByRole('button', { name: '确认回执并刷新规则', exact: true }).click();
    await expect(configure).toBeFocused();
  }
  await configureEnabled(true);
  expect((await readRules())[0]).toMatchObject({ enabled: true, revision: 2, checkpoint: 0 });
  const runTrigger = row.getByRole('button', { name: '预览运行 ' + original.rule_id, exact: true });
  const dialog = page.getByRole('dialog', { name: '预览并运行整理规则', exact: true });
  const controls = async (element: Locator, width: number) => {
    // Viewport events and React's responsive shell commit are separate frames.
    // Keep the strict geometry gate after the shell has actually reflowed.
    await expect.poll(() => element.evaluate(node => node.scrollWidth <= node.clientWidth)).toBe(true);
    const overflow = await element.evaluate(node => ({ outer: [node.clientWidth, node.scrollWidth, innerWidth], children: Array.from(node.querySelectorAll('*')).filter(child => child.getBoundingClientRect().right > node.getBoundingClientRect().right + 1).slice(0, 8).map(child => ({ tag: child.tagName, className: child.className, width: child.getBoundingClientRect().width, text: child.textContent?.slice(0, 80) })) }));
    if (overflow.outer[1] > overflow.outer[0]) {
      console.log('Automation overflow', width, JSON.stringify(overflow));
      await page.screenshot({ path: path.join(artifacts, `automation-overflow-${width}.png`), animations: 'disabled' });
    }
    expect(overflow.outer[1] <= overflow.outer[0]).toBe(true);
    if (width < 768) for (const button of await element.getByRole('button').all()) if (await button.isVisible()) expect((await button.boundingBox())!.height).toBeGreaterThanOrEqual(44);
  };
  for (const theme of ['light', 'dark'] as const) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
    for (const width of [320, 390, 768, 1280, 1440, 1920]) {
      await page.setViewportSize({ width, height: 900 });
      await controls(region, width);
      await page.screenshot({ path: path.join(artifacts, `automation-${theme}-${width}.png`), animations: 'disabled' });
      await runTrigger.focus(); await page.keyboard.press('Enter');
      await dialog.getByRole('button', { name: '读取运行预览', exact: true }).click();
      await expect(dialog.getByRole('list', { name: '整理运行预览' }).getByRole('listitem')).toHaveCount(2);
      await expect(dialog).toContainText('期限：未设置');
      await controls(dialog, width);
      await page.screenshot({ path: path.join(artifacts, `automation-preview-${theme}-${width}.png`), animations: 'disabled' });
      await page.keyboard.press('Escape'); await expect(dialog).toHaveCount(0); await expect(runTrigger).toBeFocused();
    }
  }
  expect(executions).toBe(0); expect((await readRules())[0].checkpoint).toBe(0);
  for (const file of fixture.files) {
    const response = await page.request.get('/api/v1/files/' + file.id + '/labels'); expect(response.status()).toBe(200);
    expect(JSON.stringify((await response.json()).data)).not.toContain('reviewed');
  }
  await page.setViewportSize({ width: 390, height: 900 });
  let requestKey = '';
  await page.route('**/api/v1/automation-rules/' + original.rule_id + '/execute', async route => {
    requestKey = route.request().headers()['idempotency-key'];
    const response = await route.fetch(); if (response.status() !== 200) throw new Error('automation_execute_failed');
    await route.abort('failed');
  });
  await runTrigger.click(); await dialog.getByRole('button', { name: '读取运行预览', exact: true }).click();
  await dialog.getByRole('button', { name: '确认运行本次预览', exact: true }).click();
  await expect(dialog.getByRole('button', { name: '查询原自动化回执', exact: true })).toBeVisible();
  expect(executions).toBe(1);
  const recovery = await page.evaluate(() => Object.entries(localStorage).filter(([key]) => key.startsWith('drivebridge:automation:')).map(([, value]) => Object.keys(JSON.parse(value)).sort()));
  expect(recovery).toEqual([['digest', 'key']]);
  await page.unroute('**/api/v1/automation-rules/' + original.rule_id + '/execute');
  await page.reload();
  await region.getByRole('combobox', { name: '整理规则空间' }).click(); await page.getByRole('option', { name: 'Automation management fixture', exact: true }).click();
  await region.getByRole('button', { name: '配置与运行', exact: true }).click();
  await runTrigger.click();
  const lookup = page.waitForRequest(request => new URL(request.url()).pathname.endsWith('/execute/lookup'));
  await dialog.getByRole('button', { name: '查询原自动化回执', exact: true }).click();
  expect((await lookup).postDataJSON()).toEqual({ idempotency_key: requestKey });
  await expect(dialog.getByText('已确认原运行请求', { exact: true })).toBeVisible();
  await dialog.getByRole('button', { name: '确认回执并刷新规则', exact: true }).click();
  expect(executions).toBe(1);
  for (const file of fixture.files) {
    const labels = await page.request.get('/api/v1/files/' + file.id + '/labels'); expect(labels.status()).toBe(200);
    const value = JSON.stringify((await labels.json()).data);
    if (file.path.startsWith('auto/')) expect(value).toContain('reviewed'); else expect(value).not.toContain('reviewed');
    const content = await page.request.get('/api/v1/files/' + file.id + '/content?version=' + file.version);
    expect(content.status()).toBe(200); expect(await content.text()).toBe('Original fixed bytes ' + file.path);
  }
  await configureEnabled(false);
  expect((await readRules())[0]).toMatchObject({ enabled: false, revision: 3 });
  await runTrigger.click(); await dialog.getByRole('button', { name: '读取运行预览', exact: true }).click();
  await expect(dialog.getByRole('button', { name: '确认运行本次预览', exact: true })).toBeDisabled();
  await dialog.getByRole('button', { name: '关闭', exact: true }).click();
  await region.getByRole('button', { name: '查看任务与原意图', exact: true }).click();
  await expect(page.getByRole('region', { name: '持久操作任务', exact: true })).toBeVisible();
  const tasksResponse = await page.request.get('/api/v1/workspace/tasks?kind=automation_intent&space_id=' + fixture.space);
  expect(tasksResponse.status()).toBe(200);
  const tasks = (await tasksResponse.json()).data.items as { owner_id: string; owner_status: string; ui_state: string }[];
  expect(tasks).toHaveLength(2);
  for (const item of tasks) { expect(item.owner_status).toBe('completed'); expect(item.ui_state).toBe('unknown'); }
  const touchContext = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, colorScheme: 'dark', reducedMotion: 'no-preference' });
  try {
    const touch = await touchContext.newPage(); await touch.goto('/');
    await touch.getByRole('link', { name: '使用账号登录' }).tap(); await touch.getByRole('link', { name: '以 Alice 登录' }).tap();
    await touch.getByRole('heading', { name: '我的网盘', exact: true }).waitFor(); await touch.goto('/#/automation');
    await touch.getByRole('combobox', { name: '整理规则空间' }).tap(); await touch.getByRole('option', { name: 'Automation management fixture', exact: true }).tap();
    await touch.getByRole('button', { name: '配置与运行', exact: true }).tap();
    const trigger = touch.getByRole('button', { name: '配置规则 ' + original.rule_id, exact: true }); await trigger.tap();
    const sheet = touch.getByRole('dialog', { name: '配置整理规则', exact: true });
    await expect.poll(async () => (await sheet.getByLabel('单次条目预算', { exact: true }).boundingBox())!.height).toBeGreaterThanOrEqual(44);
    await expect(sheet.getByRole('checkbox', { name: '允许手动运行' })).not.toBeChecked();
    expect(await sheet.evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true);
    await touch.screenshot({ path: path.join(artifacts, 'automation-touch-config.png'), animations: 'disabled' });
    await sheet.getByRole('button', { name: '关闭', exact: true }).tap(); await expect(trigger).toBeFocused();
  } finally { await touchContext.close(); }
  const outsider = await browser.newContext();
  try {
    const bob = await outsider.newPage(); await bob.goto('/'); await bob.getByRole('link', { name: '使用账号登录' }).click(); await bob.getByRole('link', { name: '以 Bob 登录' }).click();
    await expect(bob.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
    for (const task of tasks) expect((await bob.request.get('/api/v1/automation-intents/' + task.owner_id)).status()).toBe(404);
  } finally { await outsider.close(); }
});
