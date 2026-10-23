import { expect, test, type Page } from '@playwright/test';

async function openWorkspace(page: Page) {
  await page.goto('/');
  await page.getByRole('link', { name: '使用账号登录' }).click();
  await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  await page.getByRole('button', { name: /^打开网盘 / }).first().click();
}

async function upload(page: Page, name: string) {
  const chooser = page.waitForEvent('filechooser');
  await page.getByRole('button', { name: '上传文件', exact: true }).click();
  await (await chooser).setFiles({ name, mimeType: 'text/plain', buffer: Buffer.from(`preview body ${name}`) });
  await expect(page.getByRole('button', { name: `详情 ${name}`, exact: true })).toBeVisible();
}

test('文件预览保留主导航，支持标签、目录树和右栏收起', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await openWorkspace(page);
  const names = [`preview-one-${Date.now()}.txt`, `preview-two-${Date.now()}.txt`, `preview-three-${Date.now()}.txt`];
  for (const name of names) await upload(page, name);
  await page.getByRole('button', { name: `详情 ${names[0]}`, exact: true }).click();
  const dialog = page.locator('.quicklook-dialog');
  await expect(dialog).toBeVisible();
  await expect(page.locator('.sidebar')).toBeVisible();
  const dialogBox = await dialog.boundingBox();
  const sidebarBox = await page.locator('.sidebar').boundingBox();
  expect(dialogBox!.x).toBeGreaterThanOrEqual(Math.round(sidebarBox!.width));
  await expect(page.getByRole('complementary', { name: '预览文件列表与信息' })).toBeVisible();
  await page.getByRole('button', { name: '收起信息栏' }).click();
  await expect(page.getByRole('complementary', { name: '预览文件列表与信息' })).toBeHidden();
  await page.getByRole('button', { name: '展开信息栏' }).click();
  await expect(page.getByRole('complementary', { name: '预览文件列表与信息' })).toBeVisible();
  await expect(page.getByRole('button', { name: '目录', exact: true })).toBeVisible();
  await expect.poll(() => page.locator('.preview-tree-file').count()).toBeGreaterThanOrEqual(3);
  const firstTab = page.getByRole('tab', { name: names[0] });
  await firstTab.dblclick();
  await page.locator('.preview-tree-file').filter({ hasText: names[1] }).click();
  await page.getByRole('tab', { name: names[1] }).dblclick();
  await page.locator('.preview-tree-file').filter({ hasText: names[2] }).click();
  await expect(page.getByRole('tab', { name: names[0] })).toHaveAttribute('aria-selected', 'false');
  await expect(page.getByRole('tab', { name: names[2] })).toHaveAttribute('aria-selected', 'true');
  await page.keyboard.press('Control+Tab');
  await expect(page.getByRole('tab', { name: names[0] })).toHaveAttribute('aria-selected', 'true');
  await page.getByRole('button', { name: '标签操作' }).click();
  await expect(page.getByRole('menuitem', { name: '关闭其他标签' })).toBeVisible();
});

test('移动端目录树使用抽屉，不撑宽预览内容', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await openWorkspace(page);
  const name = `preview-mobile-${Date.now()}.txt`;
  await upload(page, name);
  await page.getByRole('button', { name: `详情 ${name}`, exact: true }).click();
  await expect(page.locator('.quicklook-dialog')).toBeVisible();
  await expect(page.getByRole('complementary', { name: '预览文件列表与信息' })).toBeHidden();
  await page.getByRole('button', { name: '展开信息栏' }).click();
  await expect(page.getByRole('complementary', { name: '预览文件列表与信息' })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});
