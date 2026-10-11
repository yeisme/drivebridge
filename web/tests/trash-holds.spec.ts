import { expect, test } from '@playwright/test';
import { selectChoice } from './helpers/select';
test.use({ trace: 'off', video: 'off', screenshot: 'off' });

test('trash hold read-only gate or enabled original-key release preserves the actual file', async ({ page }) => {
  await page.goto('/'); await page.getByRole('link', { name: '使用账号登录' }).click(); await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  const fixture = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    async function post(path: string, body: unknown, key?: string) {
      const response = await fetch('/api/v1' + path, { method: 'POST', headers: { ...headers, ...(key ? { 'Idempotency-Key': key } : {}) }, body: JSON.stringify(body) });
      if (!response.ok) throw new Error('hold_fixture_' + response.status);
      return (await response.json()).data;
    }
    const space = await post('/spaces', { name: 'Hold UI fixture', connection_id: 'platform' });
    const bytes = new TextEncoder().encode('Disposable held file');
    const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), value => value.toString(16).padStart(2, '0')).join('');
    const op = await post('/uploads', { space_id: space.id, path: 'held-fixture.txt', size_bytes: bytes.length, sha256 }, 'hold-ui-upload');
    const part = await fetch('/api/v1/operations/' + op.id + '/parts/0', { method: 'PUT', headers: { 'X-CSRF-Token': me.csrf_token }, body: bytes });
    if (!part.ok) throw new Error('hold_fixture_part_failed');
    const done = await post('/operations/' + op.id + '/complete', {});
    const listResponse = await fetch('/api/v1/files/' + done.file_id + '/retention-holds?version_id=' + done.version_id + '&limit=50');
    if (!listResponse.ok) throw new Error('hold_fixture_list_failed');
    const list = (await listResponse.json()).data;
    let hold = '';
    if (list.management_enabled) {
      const receipt = await post('/files/' + done.file_id + '/retention-holds', { version_id: done.version_id, purpose: 'legal_hold', source_kind: 'operation', source_ref: op.id, expires_at: '' }, 'hold-ui-create');
      hold = receipt.hold_id;
    }
    await post('/files/' + done.file_id + '/trash', {});
    return { space: space.id as string, file: done.file_id as string, version: done.version_id as string, hold, enabled: list.management_enabled as boolean };
  });
  expect(fixture.enabled).toBe(process.env.DRIVEBRIDGE_TEST_HOLD_MANAGEMENT === '1');
  await page.reload();
  await page.getByRole('navigation', { name: '工作区导航' }).getByRole('button', { name: '回收站', exact: true }).click();
  await selectChoice(page.getByRole('combobox', { name: '选择空间', exact: true }), fixture.space);
  const trigger = page.getByRole('button', { name: '保留保护 held-fixture.txt', exact: true }); await trigger.click();
  let dialog = page.getByRole('dialog', { name: '保留保护详情' });
  if (!fixture.enabled) {
    await expect(dialog.getByText(/服务未启用保护管理/)).toBeVisible();
    await expect(dialog.getByText(/此固定版本没有保留保护记录/)).toBeVisible();
    await expect(dialog.getByRole('button', { name: '解除保护', exact: true })).toHaveCount(0);
  } else {
    await expect(dialog.getByText('保护生效', { exact: true })).toBeVisible();
    await expect(dialog.getByText(fixture.hold, { exact: true })).toBeVisible();
    let releaseCount = 0, key = '';
    await page.route('**/api/v1/retention/holds/' + fixture.hold + '/release-cas', async route => {
      releaseCount++; key = route.request().headers()['idempotency-key'];
      expect(route.request().postDataJSON()).toEqual({ expected_revision: 1 });
      const response = await route.fetch(); expect(response.status()).toBe(200); await route.abort('failed');
    });
    await dialog.getByRole('button', { name: '解除保护', exact: true }).click();
    await expect(dialog.getByRole('button', { name: '确认解除此保护' })).toBeVisible(); expect(releaseCount).toBe(0);
    await dialog.getByRole('button', { name: '确认解除此保护' }).click();
    await expect(dialog.getByRole('button', { name: '查询原保护回执' })).toBeEnabled(); expect(releaseCount).toBe(1);
    const stored = await page.evaluate(() => Object.entries(localStorage).filter(([key]) => key.startsWith('drivebridge:hold-release:')).map(([, value]) => JSON.parse(value)));
    expect(stored).toEqual([{ key, hold: fixture.hold, expected: 1, digest: expect.stringMatching(/^[a-f0-9]{64}$/) }]);
    await page.reload(); await selectChoice(page.getByRole('combobox', { name: '选择空间', exact: true }), fixture.space); await trigger.click();
    dialog = page.getByRole('dialog', { name: '保留保护详情' });
    await expect(dialog.getByText('已解除', { exact: true })).toBeVisible();
    const lookup = page.waitForRequest(request => new URL(request.url()).pathname.endsWith('/retention-holds/lookup'));
    await dialog.getByRole('button', { name: '查询原保护回执' }).click();
    expect((await lookup).postDataJSON()).toEqual({ action: 'release', idempotency_key: key });
    await expect(dialog.getByText(/已确认原解除回执/)).toBeVisible(); expect(releaseCount).toBe(1);
    expect(await page.evaluate(() => Object.keys(localStorage).filter(key => key.startsWith('drivebridge:hold-release:')))).toEqual([]);
    const actual = await page.request.get('/api/v1/files/' + fixture.file + '/retention-holds?version_id=' + fixture.version + '&limit=50');
    expect(actual.ok()).toBe(true); expect((await actual.json()).data.items).toEqual([expect.objectContaining({ hold_id: fixture.hold, state: 'released', revision: 2, allowed_actions: [] })]);
  }
  await dialog.getByRole('button', { name: '关闭', exact: true }).click();
  await expect(page.getByRole('button', { name: '永久删除 held-fixture.txt', exact: true })).toBeDisabled();
  const stat = await page.request.get('/api/v1/files/' + fixture.file); expect(stat.ok()).toBe(true);
  const data = (await stat.json()).data; expect(data.file.id).toBe(fixture.file); expect(data.file.trashed).toBe(true);
});
