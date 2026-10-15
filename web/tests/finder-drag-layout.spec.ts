import { test, expect } from '@playwright/test';
import path from 'node:path';

test('menu columns align and native file drag persists a move', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'dark', reducedMotion: 'reduce' });
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/');
  await page.getByRole('link', { name: '使用账号登录' }).click();
  await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const response = await fetch('/api/v1/spaces', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token }, body: JSON.stringify({ name: 'Drag acceptance', connection_id: 'platform' }) });
    if (!response.ok) throw new Error('fixture_space_failed');
  });
  await page.reload();
  await page.getByRole('button', { name: '打开网盘 Drag acceptance', exact: true }).click();
  const picker = page.waitForEvent('filechooser');
  await page.getByRole('button', { name: '上传文件', exact: true }).click();
  await (await picker).setFiles({ name: 'drag-proof.txt', mimeType: 'text/plain', buffer: Buffer.from('Durable drag proof') });
  const file = page.locator('.finder-file').filter({ hasText: 'drag-proof.txt' });
  await expect(file).toBeVisible();
  await page.getByRole('button', { name: '新建', exact: true }).click();
  await page.getByRole('menuitem', { name: '新建文件夹', exact: true }).click();
  await page.getByRole('textbox', { name: '名称', exact: true }).fill('归档');
  await page.getByRole('button', { name: '确认', exact: true }).click();
  const folder = page.locator('.finder-folder').filter({ hasText: '归档' });
  await expect(folder).toBeVisible();
  await file.click({ button: 'right' });
  const menu = page.getByRole('menu', { name: '文件操作菜单' });
  const labels = await menu.locator('.menu-label').evaluateAll(nodes => nodes.map(node => node.getBoundingClientRect().x));
  expect(Math.max(...labels) - Math.min(...labels)).toBeLessThan(2);
  await expect(menu.getByRole('menuitem', { name: '移动', exact: true })).toBeEnabled();
  const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS || 'temp/browser-artifacts';
  await page.screenshot({ path: path.join(artifacts, 'menu-left-aligned.png'), animations: 'disabled' });
  await page.keyboard.press('Escape');
  // Record only drag preview geometry, never request payloads or credentials.
  await page.evaluate(() => {
    const original = DataTransfer.prototype.setDragImage;
    DataTransfer.prototype.setDragImage = function(element, x, y) {
      const box = element.getBoundingClientRect();
      document.body.dataset.dragPreview = JSON.stringify({ width: box.width, height: box.height });
      return original.call(this, element, x, y);
    };
  });
  const moved = page.waitForResponse(response => response.url().endsWith('/move') && response.request().method() === 'POST');
  await file.locator('.finder-name').dragTo(folder.locator('.finder-name'));
  expect((await moved).ok()).toBe(true);
  await expect(file).toHaveCount(0);
  const preview = JSON.parse((await page.locator('body').getAttribute('data-drag-preview'))!);
  expect(preview.width).toBeLessThanOrEqual(314);
  expect(preview.height).toBeLessThan(80);
  await page.reload();
  await page.getByRole('button', { name: '打开文件夹 归档', exact: true }).click();
  await expect(page.locator('.finder-file').filter({ hasText: 'drag-proof.txt' })).toBeVisible();
  await page.screenshot({ path: path.join(artifacts, 'drag-move-readback.png') });
});

test('search and workspace pages have readable gutters at desktop and mobile sizes', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'dark', reducedMotion: 'reduce' });
  await page.goto('/');
  await page.locator('#login-key').fill('dbu_' + '2'.repeat(96));
  await page.getByRole('button', { name: '登录', exact: true }).click();
  await expect(page.getByRole('button', { name: '账户菜单' })).toBeVisible();
  const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS || 'temp/browser-artifacts';
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    for (const [route, title] of [['search', '搜索'], ['resources', '资源库'], ['collections', '项目集合'], ['properties', '标签与属性'], ['tasks', '任务中心'], ['devices', '设备与离线'], ['automation', '备份与整理'], ['settings', '个人设置']]) {
      await page.goto('/#/' + route);
      await page.waitForLoadState('networkidle');
      const content = page.locator('.workspace-page-content');
      await expect(content).toBeVisible();
      await expect(content.getByRole('heading', { name: title, exact: true }).last()).toBeVisible();
      await expect(content.getByText('正在加载页面…', { exact: true })).toHaveCount(0);
      expect(await content.evaluate(el => Number.parseFloat(getComputedStyle(el).paddingLeft))).toBeGreaterThanOrEqual(16);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      if (route === 'search') {
        await expect(page.getByRole('searchbox', { name: '检索条件' })).toBeVisible();
        expect((await page.getByRole('searchbox', { name: '检索条件' }).boundingBox())!.height).toBeGreaterThanOrEqual(44);
        await expect(page.getByText('从一个文件名开始')).toBeVisible();
      }
      await page.screenshot({ path: path.join(artifacts, `workspace-${route}-${width}.png`) });
    }
  }
});
