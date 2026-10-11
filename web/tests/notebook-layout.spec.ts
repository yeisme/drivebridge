import { expect, test } from '@playwright/test';
import path from 'node:path';

// Real fixture data, never prototype notes. This is reading/layout acceptance,
// not proof of the complete save/recovery chain.
test('ordinary notebook keeps reading space, controls and fixed identities across themes and widths', async ({ page }) => {
  test.setTimeout(120_000);
  const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS;
  expect(artifacts).toBeTruthy();
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.goto('/');
  await page.getByRole('link', { name: '使用账号登录' }).click();
  await page.getByRole('link', { name: '以 Alice 登录' }).click();
  const space = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const response = await fetch('/api/v1/spaces', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token }, body: JSON.stringify({ name: 'Notebook layout fixture', connection_id: 'platform' }) });
    if (!response.ok) throw new Error('fixture_space_failed');
    return (await response.json()).data.id as string;
  });
  await page.reload();
  await page.getByRole('button', { name: '打开网盘 Notebook layout fixture', exact: true }).click();
  await page.locator('input[type=file]').setInputFiles({ name: 'reading.md', mimeType: 'text/markdown', buffer: Buffer.from('# 阅读布局验收\n\n普通 Markdown 的固定版本正文。\n\n## 项目资料\n\n来源与筛选是目录操作，不占据主要阅读区域。\n\n- 一个条目\n- 另一个条目\n\n## 下一步\n\n保存必须经过差异确认。') });
  await expect(page.getByRole('button', { name: '详情 reading.md', exact: true })).toBeVisible();
  const file = (await (await page.request.get('/api/v1/files?space_id=' + encodeURIComponent(space))).json()).data.find((row: { path: string }) => row.path === 'reading.md');
  expect(file).toBeTruthy();
  await page.goto('/#/knowledge');
  await page.getByRole('combobox', { name: '知识库文件来源' }).click();
  await page.getByRole('option', { name: 'Notebook layout fixture', exact: true }).click();
  const notebook = page.getByLabel('笔记工作台', { exact: true });
  await notebook.getByRole('button', { name: '快速打开', exact: true }).click();
  await page.getByRole('dialog', { name: '快速打开', exact: true }).getByRole('button', { name: 'reading.md', exact: true }).click();
  await expect(notebook.getByRole('heading', { name: '阅读布局验收', exact: true })).toBeVisible();
  let reads = 0;
  page.on('request', request => { if (new URL(request.url()).pathname === '/api/v1/files/' + file.id + '/content') reads++; });

  for (const theme of ['light', 'dark'] as const) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
    for (const width of [320, 390, 768, 1280, 1440, 1920]) {
      await page.setViewportSize({ width, height: 900 });
      const doc = notebook.getByRole('region', { name: 'Markdown 文档', exact: true });
      await expect(doc).toBeVisible();
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      expect(await notebook.evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true);
      const pane = notebook.getByRole('region', { name: 'reading.md', exact: true });
      const bounds = (await pane.boundingBox())!;
      // The document starts in the top third, leaving useful reading height.
      expect(bounds.y, `Document top ${width}`).toBeLessThanOrEqual(300);
      expect(bounds.height, `Reading height ${width}`).toBeGreaterThanOrEqual(450);
      const outline = notebook.getByRole('button', { name: '文档大纲', exact: true });
      await expect(outline).toHaveAttribute('aria-expanded', 'false');
      await expect(notebook.getByRole('navigation', { name: '文档大纲' })).toBeHidden();
      expect((await notebook.getByRole('heading', { name: '阅读布局验收', exact: true }).boundingBox())!.y).toBeLessThanOrEqual(400);
      if (width < 768) {
        for (const button of await notebook.getByRole('toolbar', { name: '笔记工具栏' }).getByRole('button').all()) {
          const box = (await button.boundingBox())!;
          expect(box.width).toBeGreaterThanOrEqual(44);
          expect(box.height).toBeGreaterThanOrEqual(44);
        }
        const tabBox = (await notebook.getByRole('tab', { name: 'reading.md', exact: true }).boundingBox())!;
        expect(tabBox.height).toBeGreaterThanOrEqual(44);
        for (const button of await doc.getByRole('group', { name: '文档模式' }).getByRole('button').all()) expect((await button.boundingBox())!.height).toBeGreaterThanOrEqual(44);
      }
      await page.screenshot({ path: path.join(artifacts!, `notebook-reading-${theme}-${width}.png`), animations: 'disabled' });
    }
  }
  const outline = notebook.getByRole('button', { name: '文档大纲', exact: true });
  await outline.focus();
  await page.keyboard.press('Enter');
  await expect(outline).toHaveAttribute('aria-expanded', 'true');
  await notebook.getByRole('navigation', { name: '文档大纲' }).getByRole('button', { name: '下一步', exact: true }).click();
  await expect(notebook.getByRole('heading', { name: '下一步', exact: true })).toBeFocused();
  expect(new URL(page.url()).hash).toBe('#/knowledge');
  await notebook.getByRole('button', { name: '阅读设置', exact: true }).click();
  const appearance = page.getByRole('dialog', { name: '阅读设置', exact: true });
  await expect(appearance).toBeVisible();
  await appearance.getByRole('button', { name: '增大阅读字号', exact: true }).click();
  await expect(appearance.getByLabel('当前阅读字号', { exact: true })).toHaveText('17');
  await appearance.getByRole('button', { name: '阅读行宽：窄', exact: true }).click();
  await expect(notebook).toHaveCSS('--notebook-line-width', '640px');
  await page.screenshot({ path: path.join(artifacts!, 'notebook-reading-appearance.png'), animations: 'disabled' });
  await page.keyboard.press('Escape');
  await expect(notebook.getByRole('button', { name: '阅读设置', exact: true })).toBeFocused();
  await outline.focus();
  await page.keyboard.press('Enter');
  await expect(notebook.getByRole('navigation', { name: '文档大纲' })).toBeHidden();
  expect(reads, 'Resize, outline and theme changes must not refetch the document').toBe(0);
  const fixed = await page.request.get(`/api/v1/files/${file.id}/content?version=${file.version_id}`);
  expect(fixed.status()).toBe(200);
  expect(await fixed.text()).toContain('# 阅读布局验收');
  expect(errors).toEqual([]);
});
