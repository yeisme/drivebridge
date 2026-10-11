import { expect, test } from '@playwright/test';

test.use({ trace: 'off', video: 'off', screenshot: 'off' });
test('real rotation preserves expiry; lost-response recovery returns original generation even after a later rotation', async ({ page }) => {
  await page.goto('/'); await page.getByRole('link', { name: '使用账号登录' }).click(); await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  const original = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    const spaceResponse = await fetch('/api/v1/spaces', { method: 'POST', headers, body: JSON.stringify({ name: 'Rotation fixture', connection_id: 'platform' }) });
    if (!spaceResponse.ok) throw new Error('fixture_space_failed'); const space = (await spaceResponse.json()).data;
    const response = await fetch('/api/v1/agent-grants', { method: 'POST', headers: { ...headers, 'Idempotency-Key': 'rotation-fixture-create' }, body: JSON.stringify({ name: 'Rotation fixture', protocol: 'http', space_id: space.id, connection_id: 'platform', prefix: 'references', actions: ['files.read'], expiry_mode: 'duration', expires_in_seconds: 604800 }) });
    if (!response.ok) throw new Error('fixture_grant_failed'); const { grant } = (await response.json()).data;
    return { id: grant.grant_id as string, expires: grant.expires_at as string, issued: grant.issued_at as string };
  });
  await page.goto('/#/agents');
  const overview = page.getByRole('region', { name: '新协议 Agent 授权' });
  await overview.getByRole('button', { name: '凭据管理 Rotation fixture', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: '轮转 Agent 凭据' });
  await dialog.getByRole('button', { name: '确认轮转并使旧凭据失效' }).click();
  await expect(dialog.getByText('原请求已确认：凭据代次 2')).toBeVisible();
  expect(await page.evaluate(() => {
    const input = document.querySelector<HTMLInputElement>('[aria-label="轮转后的 Agent 凭据"]');
    return !!input?.value && !Object.values(localStorage).some(value => value.includes(input.value));
  })).toBe(true);
  await dialog.getByRole('button', { name: '已安全保存新凭据' }).click();
  const readback = await page.evaluate(async id => {
    const response = await fetch('/api/v1/agent-grants/' + id); if (!response.ok) throw new Error('readback_failed'); const grant = (await response.json()).data;
    return { expires: grant.expires_at, issued: grant.issued_at, generation: grant.credential_generation, revision: grant.revision };
  }, original.id);
  expect(readback).toEqual({ expires: original.expires, issued: original.issued, generation: 2, revision: 2 });

  // Commit the actual mutation, then drop only the browser's response.
  let rotations = 0; let key = '';
  await page.route(`**/api/v1/agent-grants/${original.id}/rotate`, async route => {
    rotations++; key = route.request().headers()['idempotency-key'];
    const response = await route.fetch(); if (response.status() !== 200) throw new Error('fixture_rotate_failed');
    await route.abort('failed');
  });
  await overview.getByRole('button', { name: '凭据管理 Rotation fixture', exact: true }).click();
  await dialog.getByRole('button', { name: '确认轮转并使旧凭据失效' }).click();
  await expect(dialog.getByRole('button', { name: '查询原轮转结果' })).toBeVisible();
  await page.unroute(`**/api/v1/agent-grants/${original.id}/rotate`);
  // A separately confirmed fixture operation advances the current generation.
  // Keep both issued credentials out of runner results and diagnostic artifacts.
  const later = await page.evaluate(async id => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const response = await fetch(`/api/v1/agent-grants/${id}/rotate`, { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token, 'Idempotency-Key': 'rotation-fixture-later' }, body: JSON.stringify({ expected_revision: 3 }) });
    if (!response.ok) throw new Error('fixture_later_failed'); const { grant } = (await response.json()).data;
    return grant.credential_generation as number;
  }, original.id);
  expect(later).toBe(4);
  await page.reload();
  await overview.getByRole('button', { name: '凭据管理 Rotation fixture', exact: true }).click();
  await expect(dialog.getByRole('button', { name: '确认轮转并使旧凭据失效' })).toHaveCount(0);
  const lookup = page.waitForRequest(request => request.url().endsWith(`/agent-grants/${original.id}/rotate/lookup`));
  await dialog.getByRole('button', { name: '查询原轮转结果' }).click();
  expect((await lookup).postDataJSON()).toEqual({ idempotency_key: key });
  await expect(dialog.getByText('原请求已确认：凭据代次 3')).toBeVisible();
  await expect(dialog.getByLabel('轮转后的 Agent 凭据')).toHaveCount(0);
  expect(rotations).toBe(1);
  await dialog.getByRole('button', { name: '已了解原轮转结果' }).click();
  await expect(dialog).toHaveCount(0);
});
