import { expect, test, type Page } from '@playwright/test';

async function login(page: Page, user: string) {
  await page.goto('/');
  await page.getByRole('link', { name: '使用账号登录' }).click();
  await page.getByRole('link', { name: `以 ${user} 登录` }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
}

test('real grant listing and configuration checks preserve expiry and isolate users', async ({ page, browser }) => {
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.name));
  await login(page, 'Alice');
  // Create only disposable fixture data. Never return the credential to the
  // test runner, logs or artifacts; browser trace/video/screenshots stay off.
  const before = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    const spaceResponse = await fetch('/api/v1/spaces', { method: 'POST', headers,
      body: JSON.stringify({ name: 'Grant integration', connection_id: 'platform' }) });
    if (!spaceResponse.ok) throw new Error('fixture_space_failed');
    const space = (await spaceResponse.json()).data;
    const created = await fetch('/api/v1/agent-grants', { method: 'POST', headers: { ...headers, 'Idempotency-Key': 'grant-overview-fixture' },
      body: JSON.stringify({ name: 'Read-only fixture', protocol: 'http', space_id: space.id, connection_id: 'platform', prefix: 'references', actions: ['files.read'], expiry_mode: 'never' }) });
    if (!created.ok) throw new Error('fixture_grant_failed');
    const { grant } = (await created.json()).data;
    return { id: grant.grant_id as string, revision: grant.revision as number, issuedAt: grant.issued_at as string, generation: grant.credential_generation as number };
  });
  await page.goto('/#/agents');
  const overview = page.getByRole('region', { name: '新协议 Agent 授权', exact: true });
  await expect(overview.getByRole('heading', { name: 'Read-only fixture', exact: true })).toBeVisible();
  await expect(overview.getByRole('listitem').filter({ has: page.getByRole('heading', { name: 'Read-only fixture', exact: true }) }).getByText(/长期有效/)).toBeVisible();
  const observation = page.waitForResponse(response => response.url().endsWith(`/agent-grants/${before.id}/checks`));
  await overview.getByRole('button', { name: '检查 Read-only fixture', exact: true }).click();
  expect((await observation).status()).toBe(200);
  await expect(overview.getByText('授权状态：本地检查通过')).toBeVisible();
  await expect(overview.getByText('远端连接：未验证')).toBeVisible();
  const readback = await page.evaluate(async id => {
    const result = await fetch(`/api/v1/agent-grants/${id}`);
    if (!result.ok) throw new Error('readback_failed');
    const grant = (await result.json()).data;
    return { id: grant.grant_id, revision: grant.revision, issuedAt: grant.issued_at, generation: grant.credential_generation, expiry: grant.expires_at, mode: grant.expiry_mode };
  }, before.id);
  expect(readback).toEqual({ ...before, expiry: null, mode: 'never' });
  await page.reload();
  await expect(overview.getByRole('heading', { name: 'Read-only fixture', exact: true })).toBeVisible();
  await expect(overview.getByText('远端连接：未验证')).toHaveCount(0);

  const bobContext = await browser.newContext();
  try {
    const bob = await bobContext.newPage(); await login(bob, 'Bob'); await bob.goto('/#/agents');
    const bobOverview = bob.getByRole('region', { name: '新协议 Agent 授权', exact: true });
    await expect(bobOverview.getByText(/当前范围没有新协议授权/)).toBeVisible();
    await expect(bobOverview.getByText('Read-only fixture')).toHaveCount(0);
    const denied = await bob.request.get(`/api/v1/agent-grants/${before.id}`);
    expect(denied.status()).toBe(404);
  } finally { await bobContext.close(); }
  expect(errors).toEqual([]);
});
