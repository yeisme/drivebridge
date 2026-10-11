import { expect, test } from '@playwright/test';
import path from 'node:path';

test('populated information pages retain real identities with compact responsive layout', async ({ page }) => {
  test.setTimeout(150_000);
  const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!;
  expect(artifacts).toBeTruthy();
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.goto('/');
  await page.getByRole('link', { name: '使用账号登录' }).click();
  await page.getByRole('link', { name: '以 Alice 登录' }).click();
  const fixture = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    async function write(endpoint: string, body: unknown, key: string, method = 'POST') {
      const response = await fetch('/api/v1' + endpoint, { method, headers: { ...headers, 'Idempotency-Key': key }, body: JSON.stringify(body) });
      if (!response.ok) throw new Error('information_fixture_failed: ' + endpoint);
      return (await response.json()).data;
    }
    const space = await write('/spaces', { name: 'Information layout fixture', connection_id: 'platform' }, 'info-layout-space');
    const body = new TextEncoder().encode('# Information layout\n\nFixed real file, never prototype data.');
    const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', body)), byte => byte.toString(16).padStart(2, '0')).join('');
    const op = await write('/uploads', { space_id: space.id, path: 'information-layout/项目资料与普通笔记.md', size_bytes: body.length, sha256 }, 'info-layout-upload');
    const part = await fetch('/api/v1/operations/' + op.id + '/parts/0', { method: 'PUT', headers: { 'X-CSRF-Token': me.csrf_token }, body });
    if (!part.ok) throw new Error('information_part_failed');
    const done = await write('/operations/' + op.id + '/complete', {}, 'info-layout-complete');
    const stat = (await (await fetch('/api/v1/files/' + done.file_id)).json()).data;
    for (let index = 0; index < 3; index++) {
      const name = `资料项目 ${index + 1} · ${'长名称与固定版本'.repeat(5)}`;
      const collection = await write('/file-collections', { space_id: space.id, name, kind: 'manual', scope: 'private' }, `info-layout-collection-${index}`);
      await write('/file-collections/' + collection.collection_id + '/members', { expected_revision: collection.revision, include: [done.file_id] }, `info-layout-members-${index}`, 'PUT');
      await write('/resource-descriptors', { space_id: space.id, file_ref: stat.ref, kind: 'template', display_name: name }, `info-layout-resource-${index}`);
    }
    await write('/spaces/' + space.id + '/property-definitions', { expected_revision: 0, definitions: [
      { key: 'caption', display_name: '项目说明与来源', group: '资料', type: 'text' },
      { key: 'stage', display_name: '当前状态（与文件验收状态独立）', group: '资料', type: 'text' },
      { key: 'tags', display_name: '补充标签列表', group: '资料', type: 'list' },
    ] }, 'info-layout-dictionary', 'PUT');
    return { space: space.id as string, file: done.file_id as string, version: done.version_id as string };
  });
  await page.reload();
  const cases = [
    { route: 'home', region: '个人工作台摘要', selector: '摘要空间', list: '集合摘要' },
    { route: 'resources', region: '资源库', selector: '资源空间', list: '资源描述列表' },
    { route: 'collections', region: '项目集合', selector: '集合空间', list: '集合索引' },
    { route: 'properties', region: '标签与属性', selector: '属性空间', list: '属性定义' },
    { route: 'search', region: '云端检索', selector: '', list: '' },
  ];
  for (const target of cases) {
    await page.goto('/#/' + target.route);
    const region = page.getByRole('region', { name: target.region, exact: true });
    if (target.selector) {
      await region.getByRole('combobox', { name: target.selector, exact: true }).click();
      await page.getByRole('option', { name: 'Information layout fixture', exact: true }).click();
    } else {
      await region.getByRole('searchbox', { name: '检索条件' }).fill('information-layout/');
      await region.getByRole('button', { name: '检索', exact: true }).click();
      await expect(region.getByText(fixture.file, { exact: true })).toBeVisible();
      await expect(page.getByRole('tablist', { name: '文件流程视图' })).toHaveCount(0);
    }
    if (target.list) await expect(region.getByLabel(target.list, { exact: true }).getByRole('listitem')).toHaveCount(3);
    for (const theme of ['light', 'dark'] as const) {
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      for (const width of [320, 390, 768, 1280, 1440, 1920]) {
        await page.setViewportSize({ width, height: 900 });
        await region.scrollIntoViewIfNeeded();
        const overflow = await region.evaluate(element => {
          const bounds = element.getBoundingClientRect();
          return Array.from(element.querySelectorAll("*")).filter(node => { const box = node.getBoundingClientRect(); return box.width > 0 && box.right > bounds.right + 1; }).slice(0, 8).map(node => ({ tag: node.tagName, slot: node.getAttribute("data-slot"), width: node.getBoundingClientRect().width }));
        });
        if (overflow.length) await page.screenshot({ path: path.join(artifacts, `overflow-${target.route}-${theme}-${width}.png`), animations: 'disabled' });
        await expect.poll(() => region.evaluate(element => element.scrollWidth <= element.clientWidth), { message: `${target.route}/${theme}/${width}: ${JSON.stringify(overflow)}` }).toBe(true);
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
        if (target.route === 'resources' || target.route === 'collections' || target.route === 'properties') {
          const rows = region.getByRole('list', { name: target.list, exact: true }).getByRole('listitem');
          for (const row of await rows.all()) {
            expect(await row.evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true);
            expect((await row.boundingBox())!.height).toBeLessThanOrEqual(width < 768 ? 200 : 140);
          }
        }
        if (width < 768) {
          for (const control of await region.locator('button').all()) {
            if (!(await control.isVisible())) continue;
            expect((await control.boundingBox())!.height, await control.getAttribute('aria-label') || await control.textContent() || 'control').toBeGreaterThanOrEqual(44);
          }
          const title = page.getByRole('heading', { name: target.route === 'home' ? '我的工作台' : target.route === 'search' ? '搜索' : target.region, level: 2, exact: true });
          expect((await title.boundingBox())!.height, 'Page title must not be squeezed into vertical glyphs').toBeLessThanOrEqual(48);
        }
        if (target.route === 'search') {
          const contrasts = await region.locator('[data-region=exact] code, [data-region=exact] button, [data-region=exact] th').evaluateAll(elements => {
            const luminance = (value: string) => value.match(/[\d.]+/g)!.slice(0, 3).map(Number).reduce((sum, channel, index) => {
              const c = channel / 255;
              return sum + (c <= .04045 ? c / 12.92 : ((c + .055) / 1.055) ** 2.4) * [.2126, .7152, .0722][index];
            }, 0);
            return elements.map(element => {
              const foreground = luminance(getComputedStyle(element).color);
              const background = luminance(getComputedStyle(element.closest('[data-region=exact]')!).backgroundColor);
              return (Math.max(foreground, background) + .05) / (Math.min(foreground, background) + .05);
            });
          });
          expect(contrasts.length).toBeGreaterThan(0);
          for (const contrast of contrasts) expect(contrast, `Search result contrast ${theme}/${width}`).toBeGreaterThanOrEqual(4.5);
        }
        await page.screenshot({ path: path.join(artifacts, `information-${target.route}-${theme}-${width}.png`), animations: 'disabled' });
      }
    }
    await page.setViewportSize({ width: 390, height: 900 });
    if (target.route === 'resources') {
      const trigger = region.getByRole('button', { name: /^查看资源描述 / }).first();
      await trigger.click();
      const detail = region.getByRole('region', { name: '固定资源描述', exact: true });
      await expect(detail).toContainText('描述标识：');
      await expect(detail).toContainText(fixture.version);
      await detail.getByRole('button', { name: '关闭资源描述' }).click();
      await expect(trigger).toBeFocused();
    }
    if (target.route === 'collections') {
      const trigger = region.getByRole('button', { name: /^查看集合 / }).first();
      await trigger.click();
      await expect(region.getByRole('button', { name: '查看成员固定引用：information-layout/项目资料与普通笔记.md', exact: true })).toBeVisible();
      await region.getByRole('button', { name: '关闭集合成员' }).click();
      await expect(trigger).toBeFocused();
    }
    if (target.route === 'search') {
      const trigger = region.getByRole('button', { name: '查看搜索固定版本 ' + fixture.file, exact: true });
      await trigger.click();
      await region.getByRole('button', { name: '读取搜索原版本正文' }).click();
      await expect(region.getByLabel('搜索原版本纯文本', { exact: true })).toContainText('Fixed real file');
      await region.getByRole('button', { name: '关闭搜索结果引用' }).click();
      await expect(trigger).toBeFocused();
    }
  }
  expect(errors).toEqual([]);
});
