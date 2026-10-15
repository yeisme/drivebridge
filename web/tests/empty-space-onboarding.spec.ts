import { test, expect } from '@playwright/test';

test('an account without spaces can create its first space and upload a real file', async ({ page }) => {
  await page.goto('/');
  await page.locator('#login-key').fill('dbu_' + '1'.repeat(96));
  await page.getByRole('button', { name: '登录', exact: true }).click();
  await expect(page.getByRole('button', { name: '账户菜单' })).toBeVisible();
  const spaces = await page.evaluate(async () => (await (await fetch('/api/v1/spaces')).json()).data);
  expect(spaces).toEqual([]);
  await page.goto('/#/storage');
  await page.getByRole('button', { name: '用于空间', exact: false }).first().click();
  await page.getByRole('textbox', { name: '新空间名称' }).fill('正式文件空间');
  const created = page.waitForResponse(response => response.url().endsWith('/api/v1/spaces') && response.request().method() === 'POST');
  await page.getByRole('button', { name: '确定使用', exact: true }).click();
  expect((await created).ok()).toBe(true);
  await page.goto('/#/files');
  await page.getByRole('button', { name: '打开网盘 正式文件空间', exact: true }).click();
  const picker = page.waitForEvent('filechooser');
  await page.getByRole('button', { name: '上传文件', exact: true }).click();
  await (await picker).setFiles({ name: 'first.txt', mimeType: 'text/plain', buffer: Buffer.from('First real file') });
  await expect(page.locator('.finder-file').filter({ hasText: 'first.txt' })).toBeVisible();
  await page.reload();
  await expect(page.locator('.finder-file').filter({ hasText: 'first.txt' })).toBeVisible();
});
