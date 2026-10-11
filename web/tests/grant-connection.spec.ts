import { expect, test } from '@playwright/test';
import path from 'node:path';

test.use({ trace: 'off', screenshot: 'off', video: 'off' });
test('grant-scoped MCP and HTTP templates stay read-only, responsive and keyboard accessible', async ({ page, browser }) => {
  test.setTimeout(120_000);
  const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!;
  expect(artifacts).toBeTruthy();
  await page.goto('/');
  await page.getByRole('link', { name: '使用账号登录' }).click();
  await page.getByRole('link', { name: '以 Alice 登录' }).click();
  const records = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    const response = await fetch('/api/v1/spaces', { method: 'POST', headers, body: JSON.stringify({ name: 'Connection guide fixture', connection_id: 'platform' }) });
    if (!response.ok) throw new Error('connection_fixture_space_failed');
    const space = (await response.json()).data;
    const records: { id: string; name: string; protocol: string; space: string; prefix: string }[] = [];
    for (const protocol of ['mcp', 'http']) {
      const name = '连接配置 ' + protocol + ' · ' + '独立目录与长期只读授权'.repeat(3);
      const prefix = "资料/a'b &c/" + 'references-long-'.repeat(5);
      const result = await fetch('/api/v1/agent-grants', { method: 'POST', headers: { ...headers, 'Idempotency-Key': crypto.randomUUID() }, body: JSON.stringify({ name, protocol, space_id: space.id, connection_id: 'platform', prefix, actions: ['files.read'] }) });
      if (!result.ok) throw new Error('connection_fixture_grant_failed');
      // Only non-secret projection leaves this lexical scope; automatic artifacts are disabled.
      const issued = (await result.json()).data;
      records.push({ id: issued.grant.grant_id, name, protocol, space: space.id, prefix });
    }
    return records;
  });
  await page.goto('/#/agents');
  await expect(page.getByRole('button', { name: '接入配置 ' + records[0].name, exact: true })).toBeVisible();
  const requests: string[] = [];
  page.on('request', request => { if (request.url().includes('/api/v1/')) requests.push(request.method()); });
  for (const record of records) {
    for (const theme of ['light', 'dark'] as const) {
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      for (const width of [320, 390, 768, 1280, 1440, 1920]) {
        await page.setViewportSize({ width, height: 900 });
        const trigger = page.getByRole('button', { name: '接入配置 ' + record.name, exact: true });
        await trigger.focus(); await page.keyboard.press('Enter');
        const dialog = page.getByRole('dialog', { name: 'Agent 接入配置', exact: true });
        await expect(dialog).toBeVisible();
        const content = await dialog.getByLabel('占位接入配置', { exact: true }).textContent();
        expect(content).toContain('Bearer <AGENT_TOKEN>');
        if (record.protocol === 'mcp') expect(JSON.parse(content!).mcpServers.drivebridge.url).toBe(new URL('/mcp', page.url()).href);
        else {
          const target = new URL(content!.split('\n')[0].slice(4));
          expect(target.pathname).toBe('/api/v1/files');
          expect(target.searchParams.get('space_id')).toBe(record.space);
          expect(target.searchParams.get('prefix')).toBe(record.prefix);
        }
        expect(await dialog.evaluate(element => element.scrollWidth <= element.clientWidth), `${record.protocol}/${theme}/${width}`).toBe(true);
        const bounds = (await dialog.boundingBox())!;
        expect((await dialog.getByLabel('占位接入配置', { exact: true }).boundingBox())!.y - bounds.y, 'Template should precede extended guidance').toBeLessThanOrEqual(440);
        expect(bounds.x).toBeGreaterThanOrEqual(0); expect(bounds.x + bounds.width).toBeLessThanOrEqual(width);
        if (width < 768) {
          for (const button of await dialog.getByRole('button').all()) expect((await button.boundingBox())!.height).toBeGreaterThanOrEqual(44);
        }
        await page.screenshot({ path: path.join(artifacts, `grant-connection-${record.protocol}-${theme}-${width}.png`), animations: 'disabled' });
        await dialog.getByLabel('占位接入配置', { exact: true }).scrollIntoViewIfNeeded();
        await page.screenshot({ path: path.join(artifacts, `grant-template-${record.protocol}-${theme}-${width}.png`), animations: 'disabled' });
        const guidance = dialog.locator('details').filter({ hasText: '发现能力与失败恢复' });
        await expect(guidance).not.toHaveAttribute('open');
        await guidance.locator('summary').focus(); await page.keyboard.press('Enter');
        await expect(guidance).toHaveAttribute('open');
        await expect(dialog.getByLabel('Agent 服务地址')).toHaveValue(new URL(page.url()).origin);
        expect(await dialog.evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true);
        await page.keyboard.press('Escape'); await expect(dialog).toHaveCount(0); await expect(trigger).toBeFocused();
      }
    }
  }
  expect(requests).toEqual([]);
  const touchContext = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true, colorScheme: 'dark', reducedMotion: 'no-preference' });
  try {
    const touch = await touchContext.newPage();
    await touch.goto('/'); await touch.getByRole('link', { name: '使用账号登录' }).tap(); await touch.getByRole('link', { name: '以 Alice 登录' }).tap();
    await touch.getByRole('heading', { name: '我的网盘', exact: true }).waitFor();
    await touch.goto('/#/agents');
    const trigger = touch.getByRole('button', { name: '接入配置 ' + records[0].name, exact: true });
    await trigger.tap();
    const dialog = touch.getByRole('dialog', { name: 'Agent 接入配置', exact: true });
    const address = dialog.getByLabel('Agent 服务地址', { exact: true });
    await expect.poll(async () => (await address.boundingBox())!.height).toBeGreaterThanOrEqual(44);
    await address.fill('https://example.invalid/nested');
    await expect(dialog.getByLabel('占位接入配置')).toHaveCount(0);
    await expect(dialog.getByRole('button', { name: '复制占位配置' })).toBeDisabled();
    await address.fill('https://example.invalid');
    await expect(dialog.getByLabel('占位接入配置')).toContainText('https://example.invalid/mcp');
    await touch.screenshot({ path: path.join(artifacts, 'grant-connection-touch.png'), animations: 'disabled' });
    await dialog.getByRole('button', { name: '关闭', exact: true }).tap();
    await expect(dialog).toHaveCount(0); await expect(trigger).toBeFocused();
  } finally { await touchContext.close(); }
});
