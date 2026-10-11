import { expect, test } from '@playwright/test';
import path from 'node:path';
import { selectChoice } from './helpers/select';

test.use({ trace: 'off', screenshot: 'off', video: 'off' });
test('management pages retain real local shares, trash and settings across widths and themes', async ({ page }) => {
  test.setTimeout(150_000);
  const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!;
  expect(artifacts).toBeTruthy();
  await page.goto('/'); await page.getByRole('link', { name: '使用账号登录' }).click(); await page.getByRole('link', { name: '以 Alice 登录' }).click();
  const fixture = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    async function post(endpoint: string, input: unknown, key: string) {
      const response = await fetch('/api/v1' + endpoint, { method: 'POST', headers: { ...headers, 'Idempotency-Key': key }, body: JSON.stringify(input) });
      if (!response.ok) throw new Error('management_fixture_failed:' + endpoint);
      return (await response.json()).data;
    }
    const space = await post('/spaces', { name: 'Management layout fixture', connection_id: 'platform' }, 'management-space');
    const files: { id: string; version: string; name: string }[] = [];
    const shares: string[] = [];
    for (let i = 0; i < 3; i++) {
      const name = `${i}-` + 'long-management-file-'.repeat(7) + '.txt';
      const body = new TextEncoder().encode('Disposable management file ' + i);
      const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', body)), b => b.toString(16).padStart(2, '0')).join('');
      const upload = await post('/uploads', { space_id: space.id, path: name, size_bytes: body.length, sha256 }, 'management-file-' + i);
      const put = await fetch('/api/v1/operations/' + upload.id + '/parts/0', { method: 'PUT', headers: { 'X-CSRF-Token': me.csrf_token }, body });
      if (!put.ok) throw new Error('management_part_failed');
      const done = await post('/operations/' + upload.id + '/complete', {}, 'management-complete-' + i);
      files.push({ id: done.file_id, version: done.version_id, name });
      if (i < 2) await post('/files/' + done.file_id + '/trash', {}, 'management-trash-' + i);
      else for (let j = 0; j < 2; j++) {
        // Private, disposable shares to the same fixture user. Never return the link token.
        const created = await post('/shares', { name: `Management ${j} ` + 'long-share-name-'.repeat(4), mode: 'fixed', audience: 'users', space_id: space.id, days: 7, recipients: [me.user.id], allow_download: false, items: [{ file_id: done.file_id, version_id: done.version_id }] }, 'management-share-' + j);
        shares.push(created.share.id);
      }
    }
    return { space: space.id as string, files, shares };
  });
  await page.reload();
  for (const route of ['storage', 'shares', 'trash', 'settings']) {
    await page.goto('/#/' + route);
    const region = route === 'settings' ? page.getByTestId('personal-settings') : route === 'trash' ? page.locator('.content') : page.getByRole('region', { name: route === 'storage' ? '存储管理' : '分享管理', exact: true });
    await expect(region).toBeVisible();
    if (route === 'trash') {
      await selectChoice(page.getByRole('combobox', { name: '选择空间', exact: true }), fixture.space);
      await expect(page.getByRole('button', { name: '恢复 ' + fixture.files[0].name, exact: true })).toBeVisible();
    }
    if (route === 'shares') await expect(region.getByRole('checkbox', { name: '选择记录 ' + fixture.shares[0], exact: true })).toBeVisible();
    for (const theme of ['light', 'dark'] as const) {
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      for (const width of [320, 390, 768, 1280, 1440, 1920]) {
        await page.setViewportSize({ width, height: 900 });
        await region.scrollIntoViewIfNeeded();
        await page.screenshot({ path: path.join(artifacts, `management-${route}-${theme}-${width}.png`), animations: 'disabled' });
        expect(await region.evaluate(element => element.scrollWidth <= element.clientWidth), `${route}/${theme}/${width}`).toBe(true);
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
        if (width < 768) {
          for (const button of await region.locator('button:not([role=checkbox]):not([role=radio]):not([role=switch])').all()) {
            if (!await button.isVisible()) continue;
            expect((await button.boundingBox())!.height, `${route}: ${await button.getAttribute('aria-label') || await button.textContent()}`).toBeGreaterThanOrEqual(44);
          }
          if (route === 'shares') {
            for (const cell of await region.locator('td[data-label="内容与状态"]').all()) expect((await cell.boundingBox())!.width).toBeGreaterThanOrEqual(170);
          }
          if (route === 'settings') expect((await region.getByRole('radiogroup', { name: '主题模式' }).boundingBox())!.height).toBeLessThanOrEqual(260);
          if (route === 'trash') expect((await region.getByRole('combobox', { name: '选择空间', exact: true }).boundingBox())!.width).toBeGreaterThanOrEqual(180);
        }
      }
    }
    if (route === 'shares') {
      await page.setViewportSize({ width: 390, height: 900 });
      await region.getByRole('checkbox', { name: '选择记录 ' + fixture.shares[0], exact: true }).check();
      await region.getByRole('button', { name: '撤销所选', exact: true }).click();
      const confirm = page.getByRole('alertdialog', { name: '批量撤销分享', exact: true });
      await expect(confirm.getByRole('button', { name: '确认撤销所选' })).toBeEnabled();
      for (const width of [1440, 390]) {
        await page.setViewportSize({ width, height: 900 });
        await page.screenshot({ path: path.join(artifacts, `share-revoke-${width}.png`), animations: 'disabled' });
      }
      await confirm.getByRole('button', { name: '取消', exact: true }).click();
      const list = (await (await page.request.get('/api/v1/shares')).json()).data;
      expect(list.find((row: { id: string }) => row.id === fixture.shares[0]).revoked).toBe(false);
      await region.getByRole('button', { name: '撤销所选', exact: true }).click();
      await confirm.getByRole('button', { name: '确认撤销所选', exact: true }).click();
      await expect(confirm).toHaveCount(0);
      await page.reload();
      const rows = (await (await page.request.get('/api/v1/shares')).json()).data;
      expect(rows.find((row: { id: string }) => row.id === fixture.shares[0]).revoked).toBe(true);
    }
    if (route === 'trash') {
      await page.setViewportSize({ width: 390, height: 900 });
      await page.getByRole('checkbox', { name: '全选回收站文件', exact: true }).check();
      await expect(page.getByRole('button', { name: '永久删除所选', exact: true })).toBeDisabled();
      await page.getByRole('button', { name: '恢复所选', exact: true }).click();
      await expect(page.getByRole('button', { name: '恢复 ' + fixture.files[0].name, exact: true })).toHaveCount(0);
      await page.reload();
      for (const file of fixture.files.slice(0, 2)) {
        const response = await page.request.get('/api/v1/files/' + file.id);
        expect(response.status()).toBe(200);
        const result = (await response.json()).data.file;
        expect(result.trashed).toBe(false); expect(result.version_id).toBe(file.version);
      }
    }
    if (route === 'settings') {
      await page.setViewportSize({ width: 390, height: 900 });
      await region.getByRole('navigation', { name: '设置大纲' }).getByRole('button', { name: '个人资料', exact: true }).click();
      await expect(region.getByRole('region', { name: '个人资料设置', exact: true })).toBeFocused();
      await region.getByLabel('显示名称', { exact: true }).fill('Management layout verified');
      await region.getByRole('button', { name: '保存资料', exact: true }).click();
      await expect(region.getByText('个人资料已保存。', { exact: true })).toBeVisible();
      await page.reload(); await expect(region.getByLabel('显示名称', { exact: true })).toHaveValue('Management layout verified');
    }
  }
});
