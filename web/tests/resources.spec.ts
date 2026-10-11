import { expect, test } from '@playwright/test';
import path from 'node:path';
test.use({ trace: 'off', video: 'off', screenshot: 'off' });
test('official resources navigation reads real descriptors without installing or executing', async ({ page, browser }) => {
  await page.goto('/'); await page.getByRole('link', { name: '使用账号登录' }).click(); await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  const original = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    async function write(path: string, body: unknown, key?: string) {
      const response = await fetch('/api/v1' + path, { method: 'POST', headers: { ...headers, ...(key ? { 'Idempotency-Key': key } : {}) }, body: JSON.stringify(body) });
      if (!response.ok) throw new Error('resource_fixture_failed'); return (await response.json()).data;
    }
    const space = await write('/spaces', { name: 'Resource fixture', connection_id: 'platform' });
    const bytes = new TextEncoder().encode('# Synthetic template\nThis is never executed.');
    const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), b => b.toString(16).padStart(2, '0')).join('');
    const op = await write('/uploads', { space_id: space.id, path: 'templates/reference.md', size_bytes: bytes.length, sha256 }, 'resource-ui-file');
    const part = await fetch('/api/v1/operations/' + op.id + '/parts/0', { method: 'PUT', headers: { 'X-CSRF-Token': me.csrf_token }, body: bytes });
    if (!part.ok) throw new Error('resource_part_failed'); const done = await write('/operations/' + op.id + '/complete', {});
    const stat = (await (await fetch('/api/v1/files/' + done.file_id)).json()).data;
    const descriptor = await write('/resource-descriptors', { space_id: space.id, file_ref: stat.ref, kind: 'template', display_name: 'Reference template' }, 'resource-ui-original');
    return { id: descriptor.descriptor_id as string, ref: descriptor.file_ref as string };
  });
  const writes: string[] = [], bodies: string[] = [];
  page.on('request', request => { const path = new URL(request.url()).pathname; if (path.includes('/resource-descriptors') && request.method() !== 'GET') writes.push(path); if (path.endsWith('/content')) bodies.push(path); });
  await page.getByRole('navigation', { name: '工作区导航' }).getByRole('button', { name: '资源库', exact: true }).click();
  await expect(page).toHaveURL(/#\/resources$/);
  const resources = page.getByRole('region', { name: '资源库', exact: true });
  await expect(resources.getByRole('heading', { name: 'Reference template', exact: true })).toBeVisible();
  await expect(resources.getByText('模板 · 未检查', { exact: true })).toBeVisible();
  const trigger = resources.getByRole('button', { name: '查看资源描述 Reference template', exact: true });
  await trigger.click(); const detail = resources.getByRole('region', { name: '固定资源描述', exact: true });
  await expect(detail.getByText(original.ref, { exact: true })).toBeVisible();
  await expect(detail.getByText(/未检查，不代表内容安全/)).toBeVisible();
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 }); await detail.scrollIntoViewIfNeeded();
    await page.screenshot({ path: path.join(process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!, `resource-detail-${width}.png`), animations: 'disabled' });
  }
  await page.setViewportSize({ width: 1440, height: 900 });
  await detail.getByRole('button', { name: '关闭资源描述' }).click(); await expect(trigger).toBeFocused();
  await page.reload(); await trigger.click(); await expect(detail.getByText('描述标识：' + original.id + ' · 修订 1', { exact: true })).toBeVisible();
  await detail.getByRole('button', { name: '关闭资源描述' }).click();
  expect(writes).toEqual([]); expect(bodies).toEqual([]);
  let lost = false, createCount = 0;
  await page.route('**/api/v1/resource-descriptors', async route => {
    if (route.request().method() !== 'POST') return route.continue();
    createCount++;
    const response = await route.fetch();
    if (!lost && response.status() === 201) { lost = true; return route.abort('failed'); }
    return route.fulfill({ response });
  });
  await resources.getByRole('button', { name: '登记固定资源', exact: true }).click();
  let register = page.getByRole('dialog', { name: '登记固定资源', exact: true });
  await register.getByRole('textbox', { name: '资源名称' }).fill('UI registered resource');
  await register.getByRole('combobox', { name: '登记空间' }).click();
  await page.getByRole('option', { name: 'Resource fixture', exact: true }).click();
  await register.getByRole('textbox', { name: '固定目标' }).fill(original.ref);
  await register.getByRole('button', { name: '核对固定目标' }).click();
  await register.getByRole('button', { name: '确认登记资源' }).click();
  await expect(register.getByRole('button', { name: '查询原登记回执' })).toBeVisible();
  expect(lost).toBe(true); expect(createCount).toBe(1);
  await page.reload();
  await resources.getByRole('button', { name: '登记固定资源', exact: true }).click();
  register = page.getByRole('dialog', { name: '登记固定资源', exact: true });
  await register.getByRole('button', { name: '查询原登记回执' }).click();
  await expect(register.getByText('已确认原登记：UI registered resource', { exact: true })).toBeVisible();
  expect(createCount).toBe(1); expect(bodies).toEqual([]);
  await register.getByRole('button', { name: '确认回执并关闭' }).click();
  await expect(resources.getByRole('heading', { name: 'UI registered resource', exact: true })).toHaveCount(1);
  const recovered = await page.evaluate(async () => {
    const response = await fetch('/api/v1/resource-descriptors');
    if (!response.ok) throw new Error('resource_readback_failed');
    return (await response.json()).data.items.filter((row: { display_name: string }) => row.display_name === 'UI registered resource').length;
  });
  expect(recovered).toBe(1);
  let checks = 0; let committedCheck = '';
  await page.route('**/api/v1/resource-descriptors/' + original.id + '/checks', async route => {
    checks++; const response = await route.fetch();
    if (response.status() !== 200) throw new Error('integrity_fixture_failed');
    committedCheck = (await response.json()).data.check_id;
    await route.abort('failed');
  });
  await trigger.click(); await detail.getByRole('button', { name: '查看编码检查', exact: true }).click();
  let check = page.getByRole('region', { name: '固定字节编码检查', exact: true });
  expect(checks).toBe(0);
  await check.getByRole('button', { name: '准备检查固定字节' }).click(); expect(checks).toBe(0);
  await check.getByRole('button', { name: '确认检查固定字节' }).click();
  await expect(check.getByText(/结果未知/)).toBeVisible(); expect(checks).toBe(1); expect(committedCheck).not.toBe('');
  await page.reload(); await trigger.click();
  const checkTrigger = detail.getByRole('button', { name: '查看编码检查', exact: true }); await checkTrigger.click();
  check = page.getByRole('region', { name: '固定字节编码检查', exact: true });
  await check.getByRole('button', { name: '查询原编码检查回执' }).click();
  await expect(check.getByText('原检查标识：' + committedCheck, { exact: true })).toBeVisible();
  await expect(check.getByText('历史编码与字节完整性检查通过', { exact: true })).toBeVisible();
  await expect(detail.getByText(/未检查，不代表内容安全/)).toBeVisible();
  expect(checks).toBe(1); expect(bodies).toEqual([]);
  await check.getByRole('button', { name: '关闭编码检查' }).click(); await expect(checkTrigger).toBeFocused();
  const context = await browser.newContext();
  try {
    const bob = await context.newPage(); await bob.goto('/'); await bob.getByRole('link', { name: '使用账号登录' }).click(); await bob.getByRole('link', { name: '以 Bob 登录' }).click();
    await bob.goto('/#/resources');
    await expect(bob.getByText(/当前范围与筛选下没有可见资源描述/)).toBeVisible();
    const response = await bob.request.get('/api/v1/resource-descriptors/' + original.id); expect(response.status()).toBe(404);
  } finally { await context.close(); }
});
