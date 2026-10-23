import { test, expect } from '@playwright/test';

test('creates a physical folder and Markdown note from the knowledge workspace', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto('/');
  await page.getByRole('link', { name: '使用账号登录' }).click();
  await page.getByRole('link', { name: '以 Alice 登录' }).click();
  const space = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const response = await fetch('/api/v1/spaces', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token },
      body: JSON.stringify({ name: 'Knowledge create', connection_id: 'platform' }),
    });
    if (!response.ok) throw new Error('Fixture space creation failed');
    return (await response.json()).data.id as string;
  });
  await page.reload();
  await page.getByRole('button', { name: '打开网盘 Knowledge create', exact: true }).click();
  await page.locator('.sidebar').getByRole('button', { name: '知识库', exact: true }).click();
  await page.getByRole('combobox', { name: '知识库文件来源' }).click();
  await page.getByRole('option', { name: 'Knowledge create', exact: true }).click();
  await page.getByText('目录与筛选').click();
  await expect(page.getByRole('button', { name: '新建目录', exact: true })).toBeEnabled();
  await page.getByRole('button', { name: '新建目录', exact: true }).click();
  await page.getByRole('textbox', { name: '目录名称' }).fill('notion');
  const directoryRequest = page.waitForRequest(request => new URL(request.url()).pathname === '/api/v1/directories' && request.method() === 'POST');
  await page.getByRole('button', { name: '确认创建', exact: true }).click();
  expect((await directoryRequest).postDataJSON()).toMatchObject({ space_id: space, path: 'notion' });
  await expect(page.getByText('当前目录：notion', { exact: true })).toBeVisible();
  await page.locator('section[aria-label="知识库"] > details').getByRole('button', { name: '新建笔记', exact: true }).click();
  await page.getByRole('textbox', { name: '笔记标题' }).fill('第一篇');
  const noteRequest = page.waitForRequest(request => new URL(request.url()).pathname === '/api/v1/notebook/pages' && request.method() === 'POST');
  await page.getByRole('button', { name: '确认创建', exact: true }).click();
  expect((await noteRequest).postDataJSON()).toMatchObject({ space_id: space, directory_path: 'notion', title: '第一篇' });
  await expect(page.getByRole('heading', { name: '第一篇', exact: true })).toBeVisible();
  const physical = await page.evaluate(async (spaceId) => {
    const response = await fetch(`/api/v1/browser?space_id=${encodeURIComponent(spaceId)}&path=notion`);
    return (await response.json()).data.entries.map((entry: { path: string }) => entry.path);
  }, space);
  expect(physical).toContain('notion/第一篇.md');
  await page.setViewportSize({ width: 390, height: 900 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});

test('fails closed with a precise message when Web Locks is unavailable', async ({ page }) => {
  await page.addInitScript(() => {
    Object.defineProperty(Navigator.prototype, 'locks', { configurable: true, get: () => undefined });
  });
  await page.goto('/');
  await page.getByRole('link', { name: '使用账号登录' }).click();
  await page.getByRole('link', { name: '以 Alice 登录' }).click();
  const space = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const response = await fetch('/api/v1/spaces', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token },
      body: JSON.stringify({ name: 'Knowledge locks blocked', connection_id: 'platform' }),
    });
    if (!response.ok) throw new Error('Fixture space creation failed');
    return (await response.json()).data.id as string;
  });
  await page.reload();
  await page.getByRole('button', { name: '打开网盘 Knowledge locks blocked', exact: true }).click();
  await page.locator('.sidebar').getByRole('button', { name: '知识库', exact: true }).click();
  await page.getByRole('combobox', { name: '知识库文件来源' }).click();
  await page.getByRole('option', { name: 'Knowledge locks blocked', exact: true }).click();
  await page.getByText('目录与筛选').click();
  const writes: string[] = [];
  page.on('request', request => { if (new URL(request.url()).pathname === '/api/v1/notebook/pages') writes.push(request.method()); });
  await page.locator('section[aria-label="知识库"] > details').getByRole('button', { name: '新建笔记', exact: true }).click();
  await expect(page.getByText(/Web Locks/)).toBeVisible();
  expect(writes).toEqual([]);
  expect(space).toBeTruthy();
});
