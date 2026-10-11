import { expect, test, type Page } from '@playwright/test';

test.use({ trace: 'off', video: 'off', screenshot: 'off' });
async function setup(page: Page, suffix: string) {
  await page.goto('/');
  await page.getByRole('link', { name: '使用账号登录' }).click();
  await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  const space = await page.evaluate(async name => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const response = await fetch('/api/v1/spaces', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token }, body: JSON.stringify({ name, connection_id: 'platform' }) });
    if (!response.ok) throw new Error('fixture_space_failed');
    return (await response.json()).data.id as string;
  }, 'Grant create ' + suffix);
  await page.goto('/#/agents');
  await page.reload(); // API fixture setup changed spaces outside the UI's loaded snapshot.
  await page.getByRole('button', { name: '接入 Agent', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: '接入 Agent', exact: true });
  await dialog.getByRole('textbox', { name: 'Agent 名称' }).fill('Browser ' + suffix);
  await dialog.getByRole('combobox', { name: '授权空间' }).click();
  await page.getByRole('option', { name: 'Grant create ' + suffix, exact: true }).click();
  return { space, dialog };
}

test('creates from the official form with explicit duration, single-use credential and refresh readback', async ({ page }) => {
  const { space, dialog } = await setup(page, 'duration');
  await dialog.getByRole('combobox', { name: '授权期限' }).click();
  await page.getByRole('option', { name: '7 天', exact: true }).click();
  await dialog.getByRole('textbox', { name: '限定目录' }).fill('references');
  await dialog.getByRole('button', { name: '确认创建授权' }).click();
  await expect(dialog.getByText('已确认授权：Browser duration')).toBeVisible();
  // Only return a boolean from the sensitive DOM, not the credential.
  expect(await page.evaluate(() => {
    const input = document.querySelector<HTMLInputElement>('[aria-label="新协议 Agent 凭据"]');
    return !!input?.value && input.type === 'password' && !Object.values(localStorage).some(value => value.includes(input.value));
  })).toBe(true);
  await dialog.getByRole('button', { name: '已安全保存凭据' }).click();
  await expect(dialog).toHaveCount(0);
  await page.reload();
  const overview = page.getByRole('region', { name: '新协议 Agent 授权' });
  await expect(overview.getByRole('heading', { name: 'Browser duration', exact: true })).toBeVisible();
  const result = await page.evaluate(async spaceID => {
    const response = await fetch('/api/v1/agent-grants?space_id=' + encodeURIComponent(spaceID));
    if (!response.ok) throw new Error('readback_failed');
    const data = (await response.json()).data;
    const grant = data.items[0];
    return { count: data.items.length, actions: grant.actions, mode: grant.expiry_mode, duration: (Date.parse(grant.expires_at) - Date.parse(grant.issued_at)) / 1000, prefix: grant.prefix, revision: grant.revision,
      pending: Object.keys(localStorage).some(key => key.startsWith('drivebridge:grant-create:')) };
  }, space);
  expect(result).toEqual({ count: 1, actions: ['files.read'], mode: 'duration', duration: 604800, prefix: 'references', revision: 1, pending: false });
});

test('recovers a committed creation with a lost response by original lookup after reload without signing twice', async ({ page }) => {
  const { space, dialog } = await setup(page, 'lost-response');
  let creates = 0;
  let originalKey = '';
  await page.route('**/api/v1/agent-grants', async route => {
    if (route.request().method() !== 'POST') return route.continue();
    creates++; originalKey = route.request().headers()['idempotency-key'];
    const response = await route.fetch();
    // Do not inspect or log the issued secret. Persist real backend effects,
    // then simulate only the loss of the response to the browser.
    if (response.status() !== 201) throw new Error('fixture_create_failed');
    await route.abort('failed');
  });
  await dialog.getByRole('button', { name: '确认创建授权' }).click();
  await expect(dialog.getByRole('button', { name: '查询原创建结果' })).toBeVisible();
  expect(creates).toBe(1);
  await page.reload();
  await page.getByRole('button', { name: '接入 Agent', exact: true }).click();
  const restored = page.getByRole('dialog', { name: '接入 Agent', exact: true });
  const lookup = page.waitForRequest(request => request.url().endsWith('/agent-grants/lookup'));
  await restored.getByRole('button', { name: '查询原创建结果' }).click();
  expect((await lookup).postDataJSON()).toEqual({ idempotency_key: originalKey });
  await expect(restored.getByText(/原授权已存在，但凭据不能再次取回/)).toBeVisible();
  await expect(restored.getByLabel('新协议 Agent 凭据')).toHaveCount(0);
  expect(creates).toBe(1);
  const count = await page.evaluate(async spaceID => (await (await fetch('/api/v1/agent-grants?space_id=' + encodeURIComponent(spaceID))).json()).data.items.length, space);
  expect(count).toBe(1);
  await restored.getByRole('button', { name: '已了解原授权结果' }).click();
  await expect(restored).toHaveCount(0);
});
