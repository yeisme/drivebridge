import { expect, test } from '@playwright/test';
import path from 'node:path';

test('fixed-version preview preserves body and usable information controls across widths and themes', async ({ page, browser }) => {
  test.setTimeout(120_000);
  const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!;
  expect(artifacts).toBeTruthy();
  const name = '普通笔记与固定版本阅读的长文件名称.md';
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/');
  await page.getByRole('link', { name: '使用账号登录' }).click();
  await page.getByRole('link', { name: '以 Alice 登录' }).click();
  const space = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const response = await fetch('/api/v1/spaces', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token }, body: JSON.stringify({ name: 'Preview layout fixture', connection_id: 'platform' }) });
    if (!response.ok) throw new Error('fixture_space_failed');
    return (await response.json()).data.id as string;
  });
  await page.reload();
  await page.getByRole('button', { name: '打开网盘 Preview layout fixture', exact: true }).click();
  const chooser = page.waitForEvent('filechooser');
  await page.getByRole('button', { name: '上传文件', exact: true }).click();
  await (await chooser).setFiles({ name, mimeType: 'text/markdown', buffer: Buffer.from('# Fixed preview\n\nBody remains mounted when information changes.') });
  const details = page.getByRole('button', { name: `详情 ${name}`, exact: true });
  await details.click();
  const dialog = page.getByRole('dialog', { name, exact: true });
  await expect(dialog.locator('.text-preview')).toContainText('Body remains mounted');
  const files = (await (await page.request.get(`/api/v1/files?space_id=${space}`)).json()).data;
  const file = files.find((item: { path: string }) => item.path === name);
  let reads = 0;
  page.on('request', request => { if (new URL(request.url()).pathname === `/api/v1/files/${file.id}/content`) reads++; });
  const body = await dialog.locator('.text-preview').elementHandle();
  for (const theme of ['light', 'dark'] as const) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
    for (const width of [320, 390, 768, 1280, 1440, 1920]) {
      await page.setViewportSize({ width, height: 900 });
      const sidebar = dialog.getByRole('complementary', { name: '预览文件列表与信息', exact: true });
      const title = dialog.getByRole('heading', { name, exact: true });
      if (width < 1280) await expect(sidebar).toBeHidden(); else await expect(sidebar).toBeVisible();
      await expect(title).toBeVisible();
      await expect.poll(() => dialog.evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true);
      expect((await title.boundingBox())!.width).toBeGreaterThanOrEqual(width < 768 ? 210 : 180);
      const stage = dialog.locator('.quicklook-stage');
      const stageBounds = (await stage.boundingBox())!;
      const dialogBounds = (await dialog.boundingBox())!;
      expect(stageBounds.height).toBeGreaterThanOrEqual(550);
      expect(dialogBounds.y + dialogBounds.height - stageBounds.y - stageBounds.height, 'Reading stage fills the dialog, without legacy empty rows').toBeLessThanOrEqual(2);
      for (const control of await dialog.locator(':scope > header button, .quicklook-controls button').all()) {
        const box = (await control.boundingBox())!;
        expect(box.height).toBeGreaterThanOrEqual(width < 768 ? 44 : 32);
      }
      await page.screenshot({ path: path.join(artifacts, `preview-reading-${theme}-${width}.png`), animations: 'disabled' });
      if (width < 1280) {
        const toggle = dialog.getByRole('button', { name: '展开信息栏', exact: true });
        await toggle.focus();
        await page.keyboard.press('Enter');
        await expect(sidebar).toBeVisible();
        expect(await sidebar.evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true);
        await expect(sidebar.getByRole('button', { name: '关闭信息栏', exact: true })).toBeFocused();
        await sidebar.getByRole('button', { name: '文件标识与引用', exact: true }).click();
        await expect(sidebar).toContainText(file.version_id);
        await page.screenshot({ path: path.join(artifacts, `preview-information-${theme}-${width}.png`), animations: 'disabled' });
        await page.keyboard.press('Escape');
        await expect(sidebar).toBeHidden();
        await expect(toggle).toBeFocused();
        await expect(dialog).toBeVisible();
      } else {
        await expect(sidebar).toBeVisible();
        expect((await sidebar.boundingBox())!.width).toBe(360);
        expect((await stage.boundingBox())!.width).toBeGreaterThanOrEqual(640);
      }
      expect(await body!.evaluate(element => element.isConnected)).toBe(true);
    }
  }
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    const information = dialog.getByRole('complementary', { name: '预览文件列表与信息' });
    if (width < 1280) await dialog.getByRole('button', { name: '展开信息栏', exact: true }).click();
    const versions = information.getByRole('button', { name: '版本记录（1）', exact: true });
    await versions.click(); await expect(information.getByRole('button', { name: '下载此版本', exact: true })).toBeVisible();
    await page.screenshot({ path: path.join(artifacts, `preview-versions-${width}.png`), animations: 'disabled' });
    await versions.click();
    const copy = information.getByRole('button', { name: '复制到其他网盘', exact: true });
    await copy.click(); await expect(information.getByRole('button', { name: '复制为受管副本', exact: true })).toBeVisible();
    await page.screenshot({ path: path.join(artifacts, `preview-copy-${width}.png`), animations: 'disabled' });
    await copy.click();
    if (width < 1280) await dialog.getByRole('button', { name: '关闭信息栏', exact: true }).click();
  }
  await page.setViewportSize({ width: 1440, height: 900 });
  await dialog.getByRole('button', { name: '关闭信息栏', exact: true }).focus();
  await page.setViewportSize({ width: 390, height: 900 });
  await expect(dialog.getByRole('complementary', { name: '预览文件列表与信息' })).toBeHidden();
  await expect(dialog.getByRole('button', { name: '展开信息栏', exact: true })).toBeFocused();
  expect(reads, 'Theme, information and viewport changes must not reload fixed content').toBe(0);
  const pinned = await page.request.get(`/api/v1/files/${file.id}/content?version=${file.version_id}`);
  expect(pinned.status()).toBe(200);
  expect(await pinned.text()).toContain('# Fixed preview');
  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();
  await expect(details).toBeFocused();
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.getByRole('checkbox', { name: '选择 ' + name, exact: true }).check();
  await page.locator('.finder-actions').getByRole('button', { name: /分享/ }).click();
  const share = page.getByRole('dialog', { name: '创建分享', exact: true });
  await expect(share.getByText('1 个文件 · 固定所选版本', { exact: true })).toBeVisible();
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    if (width < 768) for (const control of await share.locator('input:not([type=hidden]):not([type=checkbox]),button:not([role=checkbox])').all()) {
      if (!await control.isVisible()) continue;
      expect((await control.boundingBox())!.height).toBeGreaterThanOrEqual(44);
    }
    if (width < 768) {
      const downloadLabel = share.locator('label').filter({ hasText: '允许下载原文件' });
      expect((await downloadLabel.boundingBox())!.height).toBeGreaterThanOrEqual(44);
      await downloadLabel.click();
      await expect(share.getByRole('checkbox', { name: '允许下载原文件' })).not.toBeChecked();
      await downloadLabel.click();
      await expect(share.getByRole('checkbox', { name: '允许下载原文件' })).toBeChecked();
    }
    await page.screenshot({ path: path.join(artifacts, `share-create-${width}.png`), animations: 'disabled' });
  }
  // This captures configuration only: no link or credential is issued.
  await share.getByRole('button', { name: '关闭', exact: true }).click();
  const touch = await browser.newContext({ storageState: await page.context().storageState(), viewport: { width: 390, height: 900 }, hasTouch: true, isMobile: true, reducedMotion: 'reduce' });
  try {
    const mobile = await touch.newPage();
    await mobile.goto(page.url());
    const open = mobile.getByRole('button', { name: `详情 ${name}`, exact: true });
    await open.tap();
    const preview = mobile.getByRole('dialog', { name, exact: true });
    await expect(preview.locator('.text-preview')).toContainText('Body remains mounted');
    await preview.getByRole('button', { name: '展开信息栏', exact: true }).tap();
    await expect(preview.getByRole('button', { name: '关闭信息栏', exact: true })).toBeFocused();
    await preview.getByRole('button', { name: '关闭信息栏', exact: true }).tap();
    await expect(preview.getByRole('button', { name: '展开信息栏', exact: true })).toBeFocused();
    await preview.getByRole('button', { name: '关闭', exact: true }).tap();
    await expect(open).toBeFocused();
  } finally { await touch.close(); }
  expect(errors).toEqual([]);
});
