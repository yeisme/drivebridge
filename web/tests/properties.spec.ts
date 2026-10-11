import { expect, test } from '@playwright/test';
import path from 'node:path';
test.use({ trace: 'off', video: 'off', screenshot: 'off' });
test('formal property dictionary preserves original receipt after lost PUT and rejects stale edit', async ({ page, browser }) => {
  await page.goto('/'); await page.getByRole('link', { name: '使用账号登录' }).click(); await page.getByRole('link', { name: '以 Alice 登录' }).click(); await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  const space = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const response = await fetch('/api/v1/spaces', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token }, body: JSON.stringify({ name: 'Property UI fixture', connection_id: 'platform' }) });
    if (!response.ok) throw new Error('property_fixture_failed'); return (await response.json()).data.id as string;
  });
  await page.reload(); await page.getByRole('navigation', { name: '工作区导航' }).getByRole('button', { name: '标签与属性', exact: true }).click(); await expect(page).toHaveURL(/#\/properties$/);
  const properties = page.getByRole('region', { name: '标签与属性', exact: true });
  await properties.getByRole('combobox', { name: '属性空间' }).click(); await page.getByRole('option', { name: 'Property UI fixture', exact: true }).click(); await expect(properties.getByText('当前空间尚无属性定义。')).toBeVisible();
  let puts = 0, key = ''; const bodies: string[] = [];
  page.on('request', request => { if (new URL(request.url()).pathname.endsWith('/content')) bodies.push(request.url()); });
  const endpoint = '**/api/v1/spaces/' + space + '/property-definitions';
  await page.route(endpoint, async route => {
    if (route.request().method() !== 'PUT') return route.continue();
    puts++; key = route.request().headers()['idempotency-key']; const response = await route.fetch(); if (response.status() !== 200) throw new Error('property_write_failed'); await route.abort('failed');
  });
  const trigger = properties.getByRole('button', { name: '编辑属性字典', exact: true }); await trigger.click(); let dialog = page.getByRole('dialog', { name: '编辑属性字典', exact: true });
  await dialog.getByRole('button', { name: '读取完整属性字典' }).click(); await dialog.getByRole('button', { name: '添加属性字段' }).click();
  const name = '状态 <>& "quoted" \\ line\u2028break\u2029end';
  await dialog.getByRole('textbox', { name: '字段 1 稳定 key' }).fill('status'); await dialog.getByRole('textbox', { name: '字段 1 显示名' }).fill(name); await dialog.getByRole('textbox', { name: '字段 1 分组' }).fill('项目');
  await dialog.getByRole('button', { name: '预览字典修改' }).click(); expect(puts).toBe(0);
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await expect(dialog.getByLabel('字典修改差异')).toContainText(name);
    await page.screenshot({ path: path.join(process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!, `property-diff-${width}.png`), animations: 'disabled' });
  }
  await page.setViewportSize({ width: 1440, height: 900 });
  const rejected = page.waitForEvent('dialog');
  const firstNavigation = page.evaluate(() => { location.hash = '#/files'; });
  const warning = await rejected; expect(warning.type()).toBe('confirm'); expect(warning.message()).toContain('属性字典草稿'); await warning.dismiss(); await firstNavigation;
  await expect(page).toHaveURL(/#\/properties$/);
  await expect(dialog.getByLabel('字典修改差异')).toContainText(name);
  expect(puts).toBe(0);
  const approved = page.waitForEvent('dialog');
  const nextNavigation = page.evaluate(() => { location.hash = '#/files'; });
  await (await approved).accept(); await nextNavigation;
  await expect(page).toHaveURL(/#\/files$/); await expect(dialog).toHaveCount(0); expect(puts).toBe(0);
  expect(await page.evaluate(() => Object.keys(localStorage).filter(key => key.startsWith('drivebridge:property-definitions:')))).toEqual([]);
  await page.getByRole('navigation', { name: '工作区导航' }).getByRole('button', { name: '标签与属性', exact: true }).click();
  await properties.getByRole('combobox', { name: '属性空间' }).click(); await page.getByRole('option', { name: 'Property UI fixture', exact: true }).click(); await trigger.click();
  dialog = page.getByRole('dialog', { name: '编辑属性字典', exact: true });
  await dialog.getByRole('button', { name: '读取完整属性字典' }).click(); await expect(dialog.getByRole('textbox', { name: '字段 1 显示名' })).toHaveCount(0);
  await dialog.getByRole('button', { name: '添加属性字段' }).click();
  await dialog.getByRole('textbox', { name: '字段 1 稳定 key' }).fill('status'); await dialog.getByRole('textbox', { name: '字段 1 显示名' }).fill(name); await dialog.getByRole('textbox', { name: '字段 1 分组' }).fill('项目');
  await dialog.getByRole('button', { name: '预览字典修改' }).click();
  await dialog.getByRole('button', { name: '确认保存属性字典' }).click(); await expect(dialog.getByRole('button', { name: '查询原字典回执' })).toBeVisible(); expect(puts).toBe(1);
  const stored = await page.evaluate(() => Object.entries(localStorage).filter(([key]) => key.startsWith('drivebridge:property-definitions:')).map(([, value]) => JSON.parse(value)));
  expect(stored).toEqual([{ key, digest: expect.stringMatching(/^[a-f0-9]{64}$/) }]);
  await page.reload(); await properties.getByRole('combobox', { name: '属性空间' }).click(); await page.getByRole('option', { name: 'Property UI fixture', exact: true }).click(); await trigger.click(); dialog = page.getByRole('dialog', { name: '编辑属性字典', exact: true });
  await dialog.getByRole('button', { name: '查询原字典回执' }).click(); await expect(dialog.getByText('已确认原属性字典修改', { exact: true })).toBeVisible(); expect(puts).toBe(1); await dialog.getByRole('button', { name: '确认字典回执并关闭' }).click(); await expect(trigger).toBeFocused();
  await properties.getByRole('button', { name: '刷新属性字典' }).click(); await expect(properties.getByRole('heading', { name, exact: true })).toBeVisible(); await page.unroute(endpoint);
  await trigger.click(); dialog = page.getByRole('dialog', { name: '编辑属性字典', exact: true }); await dialog.getByRole('button', { name: '读取完整属性字典' }).click(); await expect(dialog.getByRole('textbox', { name: '字段 1 稳定 key' })).toBeDisabled(); await expect(dialog.getByRole('combobox', { name: '字段 1 类型' })).toBeDisabled();
  await dialog.getByRole('textbox', { name: '字段 1 显示名' }).fill('Stale UI name');
  await page.evaluate(async space => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const response = await fetch('/api/v1/spaces/' + space + '/property-definitions', { method: 'PUT', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token, 'Idempotency-Key': 'concurrent-property-edit' }, body: JSON.stringify({ expected_revision: 1, definitions: [{ key: 'status', display_name: 'Concurrent owner name', group: '', type: 'text' }] }) }); if (!response.ok) throw new Error('concurrent_property_failed');
  }, space);
  await dialog.getByRole('button', { name: '预览字典修改' }).click(); await dialog.getByRole('button', { name: '确认保存属性字典' }).click(); await expect(dialog.getByText(/不自动换修订/)).toBeVisible(); await dialog.getByRole('button', { name: '关闭', exact: true }).click();
  await properties.getByRole('button', { name: '刷新属性字典' }).click(); await expect(properties.getByRole('heading', { name: 'Concurrent owner name', exact: true })).toBeVisible(); expect(bodies).toEqual([]);
  const context = await browser.newContext();
  try {
    const bob = await context.newPage(); await bob.goto('/'); await bob.getByRole('link', { name: '使用账号登录' }).click(); await bob.getByRole('link', { name: '以 Bob 登录' }).click(); expect((await bob.request.get('/api/v1/spaces/' + space + '/property-definitions')).status()).toBe(404);
    const status = await bob.evaluate(async ({ space, key }) => { const me = (await (await fetch('/api/v1/me')).json()).data; return (await fetch('/api/v1/spaces/' + space + '/property-definitions/lookup', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token }, body: JSON.stringify({ idempotency_key: key }) })).status; }, { space, key }); expect(status).toBe(404);
  } finally { await context.close(); }
});
