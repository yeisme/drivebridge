import { expect, test } from '@playwright/test';
test.use({ trace: 'off', video: 'off', screenshot: 'off' });
test('real upload advances head while history recovers original sidecar receipt without inheriting values', async ({ page }) => {
  expect(process.env.DRIVEBRIDGE_TEST_PROPERTY_EDITING).toBe('1');
  await page.goto('/'); await page.getByRole('link', { name: '使用账号登录' }).click(); await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  const original = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    async function write(path: string, body: unknown, key?: string, method = 'POST') {
      const response = await fetch('/api/v1' + path, { method, headers: { ...headers, ...(key ? { 'Idempotency-Key': key } : {}) }, body: JSON.stringify(body) });
      if (!response.ok) throw new Error('property_history_fixture_failed'); return (await response.json()).data;
    }
    const space = await write('/spaces', { name: 'Property history fixture', connection_id: 'platform' });
    const bytes = Uint8Array.from(atob('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+X2ioAAAAASUVORK5CYII='), ch => ch.charCodeAt(0));
    const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), byte => byte.toString(16).padStart(2, '0')).join('');
    const op = await write('/uploads', { space_id: space.id, path: 'history.png', size_bytes: bytes.length, sha256 }, 'property-history-file');
    const part = await fetch('/api/v1/operations/' + op.id + '/parts/0', { method: 'PUT', headers: { 'X-CSRF-Token': me.csrf_token }, body: bytes }); if (!part.ok) throw new Error('history_part_failed');
    const result = await write('/operations/' + op.id + '/complete', {}); if (result.status !== 'completed') throw new Error('history_upload_not_completed');
    await write('/spaces/' + space.id + '/property-definitions', { expected_revision: 0, definitions: [{ key: 'caption', display_name: '说明', group: '', type: 'text' }] }, 'property-history-definitions', 'PUT');
    return { space: space.id as string, file: result.file_id as string, version: result.version_id as string };
  });
  await page.reload(); await page.getByRole('button', { name: '打开网盘 Property history fixture', exact: true }).click();
  await page.getByRole('button', { name: '详情 history.png', exact: true }).click(); await page.getByRole('button', { name: '查看文件属性', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: '固定版本文件属性', exact: true });
  await dialog.getByRole('button', { name: '读取固定版本属性' }).click(); await dialog.getByRole('button', { name: '说明 设置值' }).click(); await dialog.getByRole('textbox', { name: '说明 新值' }).fill('Only the original version');
  const path = '/api/v1/files/' + original.file + '/properties'; let writes = 0, key = '';
  await page.route('**' + path, async route => {
    if (route.request().method() !== 'PATCH') return route.continue();
    writes++; key = route.request().headers()['idempotency-key']; const response = await route.fetch(); expect(response.status()).toBe(200); await route.abort('failed');
  });
  await dialog.getByRole('button', { name: '预览属性修改' }).click(); await dialog.getByRole('button', { name: '确认保存文件属性' }).click(); await expect(dialog.getByRole('button', { name: '查询原属性回执' })).toBeVisible(); expect(writes).toBe(1);
  const newer = await page.evaluate(async original => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    async function write(path: string, body: unknown, key?: string) {
      const response = await fetch('/api/v1' + path, { method: 'POST', headers: { ...headers, ...(key ? { 'Idempotency-Key': key } : {}) }, body: JSON.stringify(body) });
      if (!response.ok) throw new Error('new_version_upload_failed'); return (await response.json()).data;
    }
    const bytes = Uint8Array.from(atob('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+X2ioAAAAASUVORK5CYII='), ch => ch.charCodeAt(0));
    const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), byte => byte.toString(16).padStart(2, '0')).join('');
    const op = await write('/uploads', { space_id: original.space, path: 'history.png', file_id: original.file, expected_version: original.version, size_bytes: bytes.length, sha256 }, 'property-history-new-head');
    const part = await fetch('/api/v1/operations/' + op.id + '/parts/0', { method: 'PUT', headers: { 'X-CSRF-Token': me.csrf_token }, body: bytes }); if (!part.ok) throw new Error('new_head_part_failed');
    return await write('/operations/' + op.id + '/complete', {});
  }, original);
  expect(newer.status).toBe('completed'); expect(newer.file_id).toBe(original.file); expect(newer.version_id).not.toBe(original.version);
  const current = (await (await page.request.get('/api/v1/files?space_id=' + original.space)).json()).data.find((row: { id: string }) => row.id === original.file); expect(current.version_id).toBe(newer.version_id);
  await page.reload(); await page.getByRole('button', { name: '详情 history.png', exact: true }).click();
  await page.getByRole('button', { name: '查看文件属性', exact: true }).click(); await dialog.getByRole('button', { name: '读取固定版本属性' }).click(); await expect(dialog.getByText('caption · 当前值：未设置', { exact: true })).toBeVisible(); await dialog.getByRole('button', { name: '关闭', exact: true }).click();
  await page.getByRole('button', { name: '版本记录（2）', exact: true }).click();
  const history = page.getByRole('button', { name: '查看历史属性与回执 ' + original.version, exact: true }); await history.click();
  await expect(dialog.getByRole('button', { name: '查询原属性回执' })).toBeVisible(); await expect(dialog.getByRole('button', { name: '读取固定版本属性' })).toHaveCount(0);
  const request = page.waitForRequest(req => req.url().endsWith(path + '/lookup'));
  await dialog.getByRole('button', { name: '查询原属性回执' }).click(); expect((await request).postDataJSON()).toEqual({ version_id: original.version, idempotency_key: key });
  await expect(dialog.getByText('已确认原文件属性修改', { exact: true })).toBeVisible(); expect(writes).toBe(1); await dialog.getByRole('button', { name: '确认属性回执并关闭' }).click(); await expect(history).toBeFocused();
  await history.click(); await dialog.getByRole('button', { name: '读取固定版本属性' }).click(); await expect(dialog.getByText('caption · 当前值："Only the original version"', { exact: true })).toBeVisible(); await expect(dialog.getByRole('button', { name: '说明 设置值' })).toHaveCount(0);
  const old = (await (await page.request.get(path + '?version_id=' + original.version)).json()).data;
  const fresh = (await (await page.request.get(path + '?version_id=' + newer.version_id)).json()).data;
  expect(old.revision).toBe(1); expect(old.values).toEqual({ caption: 'Only the original version' }); expect(fresh.revision).toBe(0); expect(fresh.values).toEqual({});
  expect(await page.evaluate(() => Object.keys(localStorage).filter(key => key.startsWith('drivebridge:property-sidecar:')))).toEqual([]);
});
