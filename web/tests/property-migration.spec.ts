import { expect, test } from '@playwright/test';
import { selectChoice } from './helpers/select';
test.use({ trace: 'off', video: 'off', screenshot: 'off' });
test('sidecar property migration previews confirms and recovers the original execution receipt', async ({ page }) => {
  const enabled = process.env.DRIVEBRIDGE_TEST_PROPERTY_EDITING === '1';
  await page.goto('/'); await page.getByRole('link', { name: '使用账号登录' }).click(); await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  const fixture = await page.evaluate(async enabled => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    async function write(path: string, method: string, body: unknown, key: string) {
      const response = await fetch('/api/v1' + path, { method, headers: { ...headers, 'Idempotency-Key': key }, body: JSON.stringify(body) });
      if (!response.ok) throw new Error('migration_fixture_' + key + '_' + response.status); return (await response.json()).data;
    }
    const space = await write('/spaces', 'POST', { name: 'Migration UI fixture', connection_id: 'platform' }, 'migration-space');
    await write('/spaces/' + space.id + '/property-definitions', 'PUT', { expected_revision: 0, definitions: [{ key: 'old', display_name: '原字段', group: '', type: 'text' }] }, 'migration-definitions');
    const bytes = Uint8Array.from(atob('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+X2ioAAAAASUVORK5CYII='), ch => ch.charCodeAt(0));
    const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), value => value.toString(16).padStart(2, '0')).join('');
    const op = await write('/uploads', 'POST', { space_id: space.id, path: 'migration.png', size_bytes: bytes.length, sha256 }, 'migration-upload');
    const part = await fetch('/api/v1/operations/' + op.id + '/parts/0', { method: 'PUT', headers: { 'X-CSRF-Token': me.csrf_token }, body: bytes }); if (!part.ok) throw new Error('migration_fixture_part');
    const done = await write('/operations/' + op.id + '/complete', 'POST', {}, 'migration-complete');
    if (enabled) await write('/files/' + done.file_id + '/properties', 'PATCH', { base_version_id: done.version_id, definition_revision: 1, expected_revision: 0, updates: { old: '12.50' }, remove_keys: [] }, 'migration-value');
    return { space: space.id as string, file: done.file_id as string, version: done.version_id as string };
  }, enabled);
  await page.reload(); await page.getByRole('navigation', { name: '工作区导航' }).getByRole('button', { name: '标签与属性', exact: true }).click();
  await selectChoice(page.getByRole('combobox', { name: '属性空间' }), fixture.space);
  await page.getByRole('button', { name: '字段迁移计划', exact: true }).click();
  let dialog = page.getByRole('dialog', { name: '字段 key 与类型迁移', exact: true });
  if (!enabled) { await expect(dialog.getByText(/未启用属性迁移/)).toBeVisible(); await expect(dialog.getByRole('button', { name: '生成迁移预览' })).toHaveCount(0); return; }
  await dialog.getByRole('textbox', { name: '目标字段 key' }).fill('rank');
  await selectChoice(dialog.getByRole('combobox', { name: '迁移目标类型' }), 'number');
  await selectChoice(dialog.getByRole('combobox', { name: '迁移转换策略' }), 'parse_text');
  await dialog.getByRole('button', { name: '生成迁移预览' }).click();
  await expect(dialog.getByText('"12.50"', { exact: true })).toBeVisible(); await expect(dialog.getByText('12.50', { exact: true })).toBeVisible();
  const before = (await (await page.request.get('/api/v1/files/' + fixture.file + '/properties?version_id=' + fixture.version)).json()).data;
  expect(before.values).toEqual({ old: '12.50' }); expect(before.revision).toBe(1);
  await dialog.getByRole('button', { name: '确认此迁移计划' }).click(); await expect(dialog.getByText('已确认，尚未执行', { exact: true })).toBeVisible();
  let executes = 0, key = '';
  await page.route('**/api/v1/property-migrations/*/execute', async route => {
    executes++; key = route.request().headers()['idempotency-key']; expect(route.request().postDataJSON().expected_revision).toBe(2);
    const response = await route.fetch(); expect(response.status()).toBe(200); await route.abort('failed');
  });
  await dialog.getByRole('button', { name: '执行已确认迁移' }).click(); expect(executes).toBe(0);
  await dialog.getByRole('button', { name: '确认执行字段与属性迁移' }).click();
  await expect(dialog.getByRole('button', { name: '查询原迁移回执' })).toBeEnabled(); expect(executes).toBe(1);
  await page.reload(); await selectChoice(page.getByRole('combobox', { name: '属性空间' }), fixture.space); await page.getByRole('button', { name: '字段迁移计划', exact: true }).click();
  dialog = page.getByRole('dialog', { name: '字段 key 与类型迁移', exact: true });
  const lookup = page.waitForRequest(request => new URL(request.url()).pathname.endsWith('/property-migrations/lookup'));
  await dialog.getByRole('button', { name: '查询原迁移回执' }).click(); expect((await lookup).postDataJSON()).toEqual({ stage: 'execute', idempotency_key: key });
  await expect(dialog.getByText('迁移已完成', { exact: true })).toBeVisible(); expect(executes).toBe(1);
  const after = (await (await page.request.get('/api/v1/files/' + fixture.file + '/properties?version_id=' + fixture.version)).json()).data;
  expect(after.values).toEqual({ rank: 12.5 }); expect(after.revision).toBe(2);
  const dictionary = (await (await page.request.get('/api/v1/spaces/' + fixture.space + '/property-definitions')).json()).data;
  expect(dictionary.revision).toBe(2); expect(dictionary.definitions).toEqual([{ key: 'rank', display_name: '原字段', group: '', type: 'number' }]);
  await dialog.getByRole('button', { name: '确认结果并结束本计划' }).click();
  await expect(dialog).toHaveCount(0);
  await expect(page.getByRole('button', { name: '字段迁移计划', exact: true })).toBeFocused();
  expect(await page.evaluate(() => Object.keys(localStorage).filter(key => key.startsWith('drivebridge:property-migration:')))).toEqual([]);
});
