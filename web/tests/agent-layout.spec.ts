import { expect, test } from '@playwright/test';
import path from 'node:path';

test.use({ trace: 'off', screenshot: 'off', video: 'off' });
test('populated Agent controls and actual call observations remain usable across widths and themes', async ({ page, browser }) => {
  test.setTimeout(150_000);
  const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!;
  expect(artifacts).toBeTruthy();
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.name));
  await page.goto('/');
  await page.getByRole('link', { name: '使用账号登录' }).click();
  await page.getByRole('link', { name: '以 Alice 登录' }).click();
  const fixture = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    const spaceResponse = await fetch('/api/v1/spaces', { method: 'POST', headers, body: JSON.stringify({ name: 'Agent layout fixture', connection_id: 'platform' }) });
    if (!spaceResponse.ok) throw new Error('agent_layout_space_failed');
    const space = (await spaceResponse.json()).data;
    const prefix = 'references/' + 'project-materials-'.repeat(5);
    const records: { id: string; name: string; revision: number; agent: string; invocation: string }[] = [];
    for (let index = 0; index < 3; index++) {
      const name = `资料助手 ${index + 1} · ${'长名称与独立授权'.repeat(4)}`;
      const response = await fetch('/api/v1/agent-grants', { method: 'POST', headers: { ...headers, 'Idempotency-Key': 'agent-layout-' + index }, body: JSON.stringify({ name, protocol: 'http', space_id: space.id, connection_id: 'platform', prefix, actions: ['files.read'], expiry_mode: index === 1 ? 'duration' : 'never', ...(index === 1 ? { expires_in_seconds: 3600 } : {}) }) });
      if (!response.ok) throw new Error('agent_layout_grant_failed');
      // Secret exists only in this browser lexical frame, never in DOM or runner data.
      const issued = (await response.json()).data;
      const call = await fetch(`/api/v1/files?space_id=${encodeURIComponent(space.id)}&prefix=${encodeURIComponent(prefix)}`, { credentials: 'omit', headers: { Authorization: `Bearer ${issued.credential.secret}` } });
      if (!call.ok) throw new Error('agent_layout_read_failed');
      await call.arrayBuffer();
      const observations = await fetch('/api/v1/agent-invocations?agent_id=' + issued.grant.agent_id);
      if (!observations.ok) throw new Error('agent_layout_observation_failed');
      const observed = (await observations.json()).data.items;
      if (observed.length !== 1 || observed[0].result !== 'unknown') throw new Error('agent_layout_observation_mismatch');
      records.push({ id: issued.grant.grant_id, name, revision: issued.grant.revision, agent: issued.grant.agent_id, invocation: observed[0].invocation_id });
    }
    return { prefix, records };
  });
  // Screenshots start only after issuance and authentication have left the lexical frame.
  await page.goto('/#/agents');
  const region = page.getByRole('region', { name: '新协议 Agent 授权', exact: true });
  const card = region.getByRole('listitem').filter({ has: page.getByRole('heading', { name: fixture.records[0].name, exact: true }) });
  await expect(card).toBeVisible();
  const writes: string[] = [];
  page.on('request', request => { if (request.url().includes('/api/v1/') && !['GET', 'HEAD'].includes(request.method())) writes.push(request.method()); });
  for (const theme of ['light', 'dark'] as const) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
    for (const width of [320, 390, 768, 1280, 1440, 1920]) {
      await page.setViewportSize({ width, height: 900 });
      await region.scrollIntoViewIfNeeded();
      await page.screenshot({ path: path.join(artifacts, `agents-${theme}-${width}.png`), animations: 'disabled' });
      expect(await region.evaluate(element => element.scrollWidth <= element.clientWidth), `Agent page ${theme}/${width}`).toBe(true);
      for (const item of await region.getByRole('listitem').all()) {
        expect(await item.evaluate(element => element.scrollWidth <= element.clientWidth), `Agent card ${width}`).toBe(true);
        expect((await item.boundingBox())!.height).toBeLessThanOrEqual(width < 768 ? 370 : 330);
        for (const control of await item.getByRole('button').all()) {
          const size = (await control.boundingBox())!;
          if (width < 768) expect(size.height).toBeGreaterThanOrEqual(44);
          expect(size.x + size.width).toBeLessThanOrEqual(width);
        }
      }
      const trigger = card.getByRole('button', { name: '查看调用记录 ' + fixture.records[0].name, exact: true });
      await trigger.focus(); await page.keyboard.press('Enter');
      const dialog = page.getByRole('dialog', { name: 'Agent 调用记录', exact: true });
      const list = dialog.getByRole('list', { name: '调用记录列表' });
      await expect(list.getByRole('listitem')).toHaveCount(1);
      expect(await dialog.evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true);
      const bounds = (await dialog.boundingBox())!;
      expect(bounds.x).toBeGreaterThanOrEqual(0);
      expect(bounds.x + bounds.width).toBeLessThanOrEqual(width);
      expect((await list.boundingBox())!.y - bounds.y, 'Actual observation must not be below permanently expanded filters').toBeLessThanOrEqual(460);
      await expect(list).toContainText(fixture.records[0].invocation);
      await expect(list).toContainText('结果未知 · 未独立证明业务效果');
      await page.screenshot({ path: path.join(artifacts, `agent-observations-${theme}-${width}.png`), animations: 'disabled' });
      const disclosure = dialog.locator('details').filter({ hasText: '日期与高级时间筛选' });
      await expect(disclosure).not.toHaveAttribute('open');
      const toggle = disclosure.locator('summary');
      await toggle.focus(); await page.keyboard.press('Enter');
      await expect(disclosure).toHaveAttribute('open');
      expect(await dialog.evaluate(element => element.scrollWidth <= element.clientWidth), `Expanded filters ${width}`).toBe(true);
      await dialog.getByLabel('开始日期', { exact: true }).fill('2026-01-01');
      await toggle.focus(); await page.keyboard.press('Enter');
      await expect(disclosure).not.toHaveAttribute('open');
      await expect(toggle).toBeFocused();
      await toggle.press('Enter');
      await expect(dialog.getByLabel('开始日期', { exact: true })).toHaveValue('2026-01-01');
      await page.screenshot({ path: path.join(artifacts, `agent-filters-${theme}-${width}.png`), animations: 'disabled' });
      await page.keyboard.press('Escape'); await expect(dialog).toHaveCount(0); await expect(trigger).toBeFocused();
    }
  }
  await page.setViewportSize({ width: 320, height: 900 });
  for (const [action, title] of [['接入配置', 'Agent 接入配置'], ['管理授权', '管理 Agent 授权'], ['分组与置顶', 'Agent 分组与置顶'], ['凭据管理', '轮转 Agent 凭据'], ['查看操作任务', 'Agent 操作任务']] as const) {
    const trigger = card.getByRole('button', { name: `${action} ${fixture.records[0].name}`, exact: true });
    await trigger.focus(); await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: title, exact: true });
    await expect(dialog).toBeVisible();
    if (action === '管理授权') await expect(dialog.getByRole('combobox', { name: '变更动作' })).toBeEnabled();
    else if (action === '分组与置顶') await expect(dialog.getByRole('button', { name: '保存分组与置顶' })).toBeEnabled();
    else if (action === '凭据管理') await expect(dialog.getByRole('button', { name: '确认轮转并使旧凭据失效' })).toBeEnabled();
    else if (action === '接入配置') await expect(dialog.getByLabel('占位接入配置')).toBeVisible();
    else await expect(dialog.getByRole('list', { name: '持久操作列表' })).toBeVisible();
    await expect(dialog).not.toContainText(/正在处理|正在读取|读取原请求标识|读取原变更标识|读取持久操作/);
    for (const width of [1440, 390, 320]) {
      await page.setViewportSize({ width, height: 900 });
      expect(await dialog.evaluate(element => element.scrollWidth <= element.clientWidth), title).toBe(true);
      await page.screenshot({ path: path.join(artifacts, `agent-dialog-${action}-${width}.png`), animations: 'disabled' });
    }
    await page.keyboard.press('Escape'); await expect(dialog).toHaveCount(0); await expect(trigger).toBeFocused();
  }
  const touchContext = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, colorScheme: 'dark', reducedMotion: 'no-preference' });
  try {
    const touch = await touchContext.newPage();
    await touch.goto('/');
    await touch.getByRole('link', { name: '使用账号登录' }).tap();
    await touch.getByRole('link', { name: '以 Alice 登录' }).tap();
    await touch.getByRole('heading', { name: '我的网盘', exact: true }).waitFor();
    await touch.goto('/#/agents');
    const trigger = touch.getByRole('button', { name: '查看调用记录 ' + fixture.records[0].name, exact: true });
    await trigger.tap();
    const dialog = touch.getByRole('dialog', { name: 'Agent 调用记录', exact: true });
    await expect(dialog.getByRole('listitem')).toHaveCount(1);
    const toggle = dialog.locator('summary');
    await expect.poll(async () => (await toggle.boundingBox())!.height).toBeGreaterThanOrEqual(44);
    await toggle.tap(); await expect(dialog.getByLabel('开始日期', { exact: true })).toBeVisible();
    expect(await dialog.evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true);
    await touch.screenshot({ path: path.join(artifacts, 'agent-touch-filters.png'), animations: 'disabled' });
    await dialog.getByRole('button', { name: '关闭', exact: true }).tap();
    await expect(dialog).toHaveCount(0); await expect(trigger).toBeFocused();
  } finally { await touchContext.close(); }
  expect(writes).toEqual([]);
  const readback = await page.evaluate(async records => Promise.all(records.map(async record => {
    const response = await fetch('/api/v1/agent-grants/' + record.id);
    if (!response.ok) throw new Error('agent_layout_readback_failed');
    const grant = (await response.json()).data;
    const observed = await fetch('/api/v1/agent-invocations?agent_id=' + record.agent);
    if (!observed.ok) throw new Error('agent_layout_observation_readback_failed');
    return { id: grant.grant_id, revision: grant.revision, actions: grant.actions, invocations: (await observed.json()).data.items.map((item: { invocation_id: string }) => item.invocation_id) };
  })), fixture.records);
  expect(readback).toEqual(fixture.records.map(record => ({ id: record.id, revision: record.revision, actions: ['files.read'], invocations: [record.invocation] })));
  expect(errors).toEqual([]);
});
