import { expect, test } from '@playwright/test';
import path from 'node:path';

test.use({ trace: 'off', video: 'off', screenshot: 'off' });
test('stable creation and search dialogs have explicit desktop and mobile evidence without submitting', async ({ page }) => {
  test.setTimeout(120_000);
  const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!; expect(artifacts).toBeTruthy();
  await page.goto('/'); await page.getByRole('link', { name: '使用账号登录' }).click(); await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  const writes: string[] = [];
  page.on('request', request => {
    const endpoint = new URL(request.url()).pathname;
    // The existing capability probe uses a registered read-only POST lookup;
    // it is not a mutation and must not be changed just for visual capture.
    if (request.method() === 'POST' && endpoint === '/api/v1/operations/lookup') {
      expect(request.postDataJSON()).toMatchObject({ idempotency_key: expect.any(String) }); return;
    }
    if (endpoint.startsWith('/api/v1/') && !['GET', 'HEAD'].includes(request.method())) writes.push(endpoint);
  });
  for (const theme of ['light', 'dark'] as const) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
    for (const width of [320, 390, 768, 1280, 1440, 1920]) {
      await page.setViewportSize({ width, height: 900 });
      for (const [id, route, triggerName, title, readyLabel] of [
        ['agent-create', 'agents', '接入 Agent', '接入 Agent', 'Agent 名称'],
        ['resource-register', 'resources', '登记固定资源', '登记固定资源', '资源名称'],
        ['collection-create', 'collections', '新建私有手动集合', '新建私有手动集合', '集合名称'],
        ['collection-rule', 'collections', '新建私有规则集合', '新建私有规则集合', '集合名称'],
        ['storage-add', 'storage', '连接存储', '连接存储', '连接名称'],
      ]) {
        await page.goto('/#/' + route);
        const trigger = page.getByRole('button', { name: triggerName, exact: true });
        await expect(trigger).toBeEnabled(); await trigger.focus(); await page.keyboard.press('Enter');
        const dialog = page.getByRole('dialog', { name: title, exact: true });
        await expect(dialog.getByRole('textbox', { name: readyLabel, exact: true })).toBeEnabled();
        await expect(dialog).not.toContainText(/正在读取|正在加载|读取中/);
        expect(await dialog.evaluate(element => element.scrollWidth <= element.clientWidth), `${id}/${theme}/${width}`).toBe(true);
        if (width < 768) for (const button of await dialog.locator('button:not([role=checkbox]):not([role=radio])').all()) {
          if (!await button.isVisible()) continue;
          expect((await button.boundingBox())!.height, `${id}: ${await button.textContent()}`).toBeGreaterThanOrEqual(44);
        }
        await page.screenshot({ path: path.join(artifacts, `dialog-${id}-${theme}-${width}.png`), animations: 'disabled' });
        await page.keyboard.press('Escape'); await expect(dialog).toHaveCount(0); await expect(trigger).toBeFocused();
      }
      await page.goto('/#/search');
      await expect(page.getByText('输入检索条件后开始查询，尚未读取搜索结果。', { exact: true })).toBeVisible();
      await page.keyboard.press('Control+k');
      const search = page.getByRole('dialog', { name: /搜索/ });
      await expect(search.getByText('输入检索条件后开始查询，尚未读取搜索结果。', { exact: true })).toBeVisible();
      await page.screenshot({ path: path.join(artifacts, `dialog-search-${theme}-${width}.png`), animations: 'disabled' });
      await page.keyboard.press('Escape'); await expect(search).toHaveCount(0);
    }
  }
  expect(writes).toEqual([]);
});
