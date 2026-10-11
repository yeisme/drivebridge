import { expect, test } from '@playwright/test';
test.use({ trace: 'off', video: 'off', screenshot: 'off' });
test('official file detail sidecar uses fixed-version original receipts and explicit edits', async ({ page, browser }) => {
  const enabled = process.env.DRIVEBRIDGE_TEST_PROPERTY_EDITING === '1';
  await page.goto('/'); await page.getByRole('link', { name: '使用账号登录' }).click(); await page.getByRole('link', { name: '以 Alice 登录' }).click(); await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  const space = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const response = await fetch('/api/v1/spaces', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token }, body: JSON.stringify({ name: 'Sidecar browser space', connection_id: 'platform' }) }); if (!response.ok) throw new Error('fixture_space_failed'); return (await response.json()).data.id as string;
  });
  await page.reload(); await page.getByRole('button', { name: '打开网盘 Sidecar browser space', exact: true }).click();
  await page.locator('input[type=file]').setInputFiles({ name: 'sidecar.png', mimeType: 'image/png', buffer: Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+X2ioAAAAASUVORK5CYII=', 'base64') }); await expect(page.getByRole('button', { name: '详情 sidecar.png', exact: true })).toBeVisible();
  const target = await page.evaluate(async space => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const definitions = [{ key: 'caption', display_name: '说明', group: '', type: 'text' }, { key: 'rank', display_name: '分数', group: '', type: 'number' }, { key: 'tags', display_name: '列表', group: '', type: 'list' }, { key: 'active', display_name: '启用', group: '', type: 'checkbox' }, { key: 'date', display_name: '日期', group: '', type: 'date' }];
    const response = await fetch('/api/v1/spaces/' + space + '/property-definitions', { method: 'PUT', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token, 'Idempotency-Key': 'sidecar-ui-definitions' }, body: JSON.stringify({ expected_revision: 0, definitions }) }); if (!response.ok) throw new Error('fixture_definitions_failed');
    const files = (await (await fetch('/api/v1/files?space_id=' + space)).json()).data; const file = files.find((f: { path: string }) => f.path === 'sidecar.png'); if (!file) throw new Error('fixture_file_missing'); return { file: file.id as string, version: file.version_id as string };
  }, space);
  const path = '/api/v1/files/' + target.file + '/properties', endpoint = '**' + path;
  async function open(detail = true) { if (detail) await page.getByRole('button', { name: '详情 sidecar.png', exact: true }).click(); await page.getByRole('button', { name: '查看文件属性', exact: true }).click(); }
  const dialog = page.getByRole('dialog', { name: '固定版本文件属性', exact: true });
  async function set(label: string, value: string) { await dialog.getByRole('button', { name: label + ' 设置值', exact: true }).click(); await dialog.getByLabel(label + ' 新值', { exact: true }).fill(value); }
  await open(); const bodyRequests: string[] = []; page.on('request', request => { if (new URL(request.url()).pathname.endsWith('/content')) bodyRequests.push(request.url()); });
  await dialog.getByRole('button', { name: '读取固定版本属性' }).click();
  const caption = '说明 <>& "quote" \\ line\u2028end\u2029'; await set('说明', caption); await set('分数', '1.25'); await set('列表', '["甲","甲",""]'); await set('日期', '2024-02-29');
  await dialog.getByRole('button', { name: '启用 设置值' }).click(); await dialog.getByRole('button', { name: '启用 新值' }).click();
  let patches = 0, key = '', status = 0;
  await page.route(endpoint, async route => { if (route.request().method() !== 'PATCH') return route.continue(); patches++; key = route.request().headers()['idempotency-key']; const response = await route.fetch(); status = response.status(); if (enabled && patches === 1 && status === 200) await route.abort('failed'); else await route.fulfill({ response }); });
  await dialog.getByRole('button', { name: '预览属性修改' }).click(); expect(patches).toBe(0); await dialog.getByRole('button', { name: '确认保存文件属性' }).click(); await expect(dialog.getByRole('button', { name: '查询原属性回执' })).toBeVisible(); expect(patches).toBe(1);
  const stored = await page.evaluate(() => Object.entries(localStorage).filter(([key]) => key.startsWith('drivebridge:property-sidecar:')).map(([, value]) => JSON.parse(value))); expect(stored).toEqual([{ key, digest: expect.stringMatching(/^[a-f0-9]{64}$/) }]);
  if (!enabled) { expect(status).toBe(422); const value = (await (await page.request.get(path + '?version_id=' + target.version)).json()).data; expect(value.revision).toBe(0); expect(value.values).toEqual({}); await dialog.getByRole('button', { name: '查询原属性回执' }).click(); await expect(dialog.getByRole('button', { name: '确认原键重试属性' })).toHaveCount(0); return; }
  expect(status).toBe(200); expect(bodyRequests).toEqual([]);
  await page.reload(); await open(); await expect(dialog.getByRole('button', { name: '查询原属性回执' })).toBeVisible(); await dialog.getByRole('button', { name: '查询原属性回执' }).click(); await expect(dialog.getByText('已确认原文件属性修改', { exact: true })).toBeVisible(); expect(patches).toBe(1); await dialog.getByRole('button', { name: '确认属性回执并关闭' }).click(); await expect(page.getByRole('button', { name: '查看文件属性', exact: true })).toBeFocused();
  const current = (await (await page.request.get(path + '?version_id=' + target.version)).json()).data; expect(current.values).toEqual({ caption, rank: 1.25, tags: ['甲', '甲', ''], date: '2024-02-29', active: true });
  await open(false); await dialog.getByRole('button', { name: '读取固定版本属性' }).click(); await dialog.getByRole('button', { name: '分数 删除此值' }).click(); await dialog.getByRole('button', { name: '预览属性修改' }).click(); await dialog.getByRole('button', { name: '确认保存文件属性' }).click(); await expect(dialog.getByText('已确认原文件属性修改', { exact: true })).toBeVisible(); await dialog.getByRole('button', { name: '确认属性回执并关闭' }).click();
  const after = (await (await page.request.get(path + '?version_id=' + target.version)).json()).data; expect(after.values).toEqual({ caption, tags: ['甲', '甲', ''], date: '2024-02-29', active: true }); expect(after.revision).toBe(2);
  await open(false); await dialog.getByRole('button', { name: '读取固定版本属性' }).click(); await set('说明', 'stale draft');
  await page.evaluate(async ({ path, version }) => { const me = (await (await fetch('/api/v1/me')).json()).data; const response = await fetch(path, { method: 'PATCH', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token, 'Idempotency-Key': 'sidecar-concurrent' }, body: JSON.stringify({ base_version_id: version, definition_revision: 1, expected_revision: 2, updates: { caption: 'concurrent owner value' }, remove_keys: [] }) }); if (!response.ok) throw new Error('fixture_concurrent_failed'); }, { path, version: target.version });
  await dialog.getByRole('button', { name: '预览属性修改' }).click(); await dialog.getByRole('button', { name: '确认保存文件属性' }).click(); await expect(dialog.getByText(/不自动换版本/)).toBeVisible(); expect(status).toBe(409); expect((await (await page.request.get(path + '?version_id=' + target.version)).json()).data.values.caption).toBe('concurrent owner value');
  const context = await browser.newContext(); try { const bob = await context.newPage(); await bob.goto('/'); await bob.getByRole('link', { name: '使用账号登录' }).click(); await bob.getByRole('link', { name: '以 Bob 登录' }).click(); expect((await bob.request.get(path + '?version_id=' + target.version)).status()).toBe(404); } finally { await context.close(); }
});
