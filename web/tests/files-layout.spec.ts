import { expect, test } from '@playwright/test';
import path from 'node:path';

// Real fixture files; layout evidence is separate from transfer recovery acceptance.
test('file browser preserves reading area, directory navigation and desktop preference across viewports', async ({ page, browser: engine }) => {
  test.setTimeout(120_000);
  const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS;
  expect(artifacts).toBeTruthy();
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/');
  await page.getByRole('link', { name: '使用账号登录' }).click();
  await page.getByRole('link', { name: '以 Alice 登录' }).click();
  const space = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const response = await fetch('/api/v1/spaces', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token }, body: JSON.stringify({ name: 'Files layout fixture', connection_id: 'platform' }) });
    if (!response.ok) throw new Error('fixture_space_failed');
    return (await response.json()).data.id as string;
  });
  await page.reload();
  await page.getByRole('button', { name: '打开网盘 Files layout fixture', exact: true }).click();
  const browser = page.getByRole('region', { name: '文件浏览', exact: true });
  const chooser = page.waitForEvent('filechooser');
  await browser.getByRole('button', { name: '上传文件', exact: true }).click();
  await (await chooser).setFiles([
    { name: 'reference.md', mimeType: 'text/markdown', buffer: Buffer.from('# Layout evidence\n\nAn ordinary versioned file.') },
    { name: '资料说明与来源记录的长文件名称.txt', mimeType: 'text/plain', buffer: Buffer.from('Real file layout evidence') },
  ]);
  await expect(browser.locator('.finder-file')).toHaveCount(2);
  await browser.getByRole('button', { name: '新建', exact: true }).click();
  await page.getByRole('menuitem', { name: '新建文件夹', exact: true }).click();
  await page.getByRole('textbox', { name: '名称', exact: true }).fill('资料');
  await page.getByRole('button', { name: '确认', exact: true }).click();
  await expect(browser.getByRole('button', { name: '打开文件夹 资料', exact: true })).toBeVisible();
  const notice = page.getByRole('button', { name: '关闭通知', exact: true });
  if (await notice.isVisible()) await notice.click();
  const desktopTree = page.getByRole('complementary', { name: '目录树', exact: true });
  await expect(desktopTree).toBeVisible();
  const preference = await page.evaluate(() => localStorage.getItem('drivebridge:tree-collapsed'));

  for (const theme of ['light', 'dark'] as const) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
    for (const width of [320, 390, 768, 1280, 1440, 1920]) {
      await page.setViewportSize({ width, height: 900 });
      await expect(browser).toBeVisible();
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      expect(await browser.evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true);
      // Modal sheets make the background inaccessible by design; use its DOM
      // locator only for geometry while the drawer owns the accessibility tree.
      const files = page.locator('.finder-body .finder-scroll');
      expect((await files.boundingBox())!.y, `Files top at ${width}`).toBeLessThanOrEqual(300);
      expect((await files.boundingBox())!.height, `Files height at ${width}`).toBeGreaterThanOrEqual(450);
      if (width < 768) {
        await expect(desktopTree).toBeHidden();
        for (const control of await browser.locator('.finder-toolbar button, .finder-toolbar a, .finder-actions button, .finder-row-actions button').all()) {
          if (!(await control.isVisible())) continue;
          const box = (await control.boundingBox())!;
          expect(box.height, await control.getAttribute('aria-label') || await control.textContent() || 'control').toBeGreaterThanOrEqual(44);
          expect(box.width).toBeGreaterThanOrEqual(44);
        }
      }
      await page.screenshot({ path: path.join(artifacts!, `files-${theme}-${width}.png`), animations: 'disabled' });
      if (width < 768) {
        const toggle = browser.getByRole('button', { name: '展开目录树', exact: true });
        const before = await files.boundingBox();
        await toggle.focus();
        await page.keyboard.press('Enter');
        const drawer = page.getByRole('dialog', { name: '目录树', exact: true });
        await expect(drawer).toBeVisible();
        expect((await drawer.boundingBox())!.width).toBe(Math.min(320, width));
        expect((await files.boundingBox())!.y).toBe(before!.y);
        for (const control of await drawer.getByRole('button').all()) {
          const box = (await control.boundingBox())!;
          expect(box.height).toBeGreaterThanOrEqual(44);
          expect(box.width).toBeGreaterThanOrEqual(44);
        }
        await page.screenshot({ path: path.join(artifacts!, `files-directory-${theme}-${width}.png`), animations: 'disabled' });
        await page.keyboard.press('Escape');
        await expect(drawer).toBeHidden();
        await expect(toggle).toBeFocused();
        await toggle.click();
        await drawer.getByRole('button', { name: '资料', exact: true }).click();
        await expect(drawer).toBeHidden();
        await expect(browser.getByRole('navigation', { name: '目录路径' }).getByRole('button', { name: '资料', exact: true })).toBeVisible();
        await expect(toggle).toBeFocused();
        await browser.getByRole('navigation', { name: '目录路径' }).getByRole('button', { name: '全部文件', exact: true }).click();
        await expect(browser.locator('.finder-file')).toHaveCount(2);
      } else {
        await expect(desktopTree).toBeVisible();
        const checkbox = browser.getByRole('checkbox', { name: '选择 reference.md', exact: true });
        await checkbox.focus();
        await expect(checkbox).toHaveCSS('opacity', '1');
        expect((await browser.locator('.finder-file').first().boundingBox())!.height).toBeLessThanOrEqual(40);
      }
      expect(await page.evaluate(() => localStorage.getItem('drivebridge:tree-collapsed'))).toBe(preference);
    }
  }
  const touch = await engine.newContext({ storageState: await page.context().storageState(), viewport: { width: 390, height: 900 }, hasTouch: true, isMobile: true, reducedMotion: 'reduce' });
  try {
    const mobile = await touch.newPage();
    await mobile.goto(page.url());
    const toggle = mobile.getByRole('button', { name: '展开目录树', exact: true });
    await toggle.tap();
    const drawer = mobile.getByRole('dialog', { name: '目录树', exact: true });
    await drawer.getByRole('button', { name: '资料', exact: true }).tap();
    await expect(drawer).toBeHidden();
    await expect(toggle).toBeFocused();
    await mobile.getByRole('navigation', { name: '目录路径' }).getByRole('button', { name: '全部文件', exact: true }).tap();
    await expect(mobile.locator('.finder-file')).toHaveCount(2);
    await mobile.screenshot({ path: path.join(artifacts!, 'files-touch-390.png'), animations: 'disabled' });
  } finally { await touch.close(); }
  const files = (await (await page.request.get(`/api/v1/files?space_id=${space}`)).json()).data;
  const note = files.find((file: { path: string }) => file.path === 'reference.md');
  const response = await page.request.get(`/api/v1/files/${note.id}/content?version=${note.version_id}`);
  expect(response.status()).toBe(200);
  expect(await response.text()).toContain('# Layout evidence');
  expect(errors).toEqual([]);
});
