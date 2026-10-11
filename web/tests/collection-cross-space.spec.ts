import { expect, test } from '@playwright/test';
test.use({ trace: 'off', video: 'off', screenshot: 'off' });
test('member picker adds same-path files from two authorized spaces without moving either file', async ({ page }) => {
  await page.goto('/'); await page.getByRole('link', { name: '使用账号登录' }).click(); await page.getByRole('link', { name: '以 Alice 登录' }).click(); await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  const fixture = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    async function write(path: string, body: unknown, key: string, method = 'POST') {
      const response = await fetch('/api/v1' + path, { method, headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token, 'Idempotency-Key': key }, body: JSON.stringify(body) });
      if (!response.ok) throw new Error('cross_space_fixture_failed'); return (await response.json()).data;
    }
    const spaces = [], files = [];
    for (const name of ['Member home', 'Member source']) {
      const space = await write('/spaces', { name, connection_id: 'platform' }, name); spaces.push(space.id);
      const bytes = new TextEncoder().encode(name); const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), b => b.toString(16).padStart(2, '0')).join('');
      const op = await write('/uploads', { space_id: space.id, path: 'same.txt', size_bytes: bytes.length, sha256 }, 'cross-' + name);
      if (op.status !== 'completed') { const part = await fetch('/api/v1/operations/' + op.id + '/parts/0', { method: 'PUT', headers: { 'X-CSRF-Token': me.csrf_token }, body: bytes }); if (!part.ok) throw new Error('cross_space_part_failed'); }
      const done = await write('/operations/' + op.id + '/complete', {}, 'complete-' + name); if (done.status !== 'completed') throw new Error('cross_space_upload_incomplete'); files.push(done.file_id);
    }
    const collection = await write('/file-collections', { space_id: spaces[0], name: 'Cross-space collection', kind: 'manual', scope: 'private' }, 'cross-space-collection');
    await write('/file-collections/' + collection.collection_id + '/members', { expected_revision: 1, include: [files[0]], exclude: [] }, 'initial-members', 'PUT');
    return { spaces: spaces as string[], files: files as string[], collection: collection.collection_id as string };
  });
  await page.reload(); await page.getByRole('navigation', { name: '工作区导航' }).getByRole('button', { name: '项目集合', exact: true }).click();
  const collections = page.getByRole('region', { name: '项目集合', exact: true }); await collections.getByRole('combobox', { name: '集合空间' }).click(); await page.getByRole('option', { name: 'Member home', exact: true }).click();
  const trigger = collections.getByRole('button', { name: '编辑成员 Cross-space collection', exact: true }); await trigger.click(); const dialog = page.getByRole('dialog', { name: '编辑集合成员', exact: true });
  const content: string[] = []; page.on('request', request => { if (new URL(request.url()).pathname.endsWith('/content')) content.push(request.url()); });
  await dialog.getByRole('button', { name: '读取完整成员决策' }).click(); await expect(dialog.getByText('来源空间：Member home · ' + fixture.spaces[0], { exact: true })).toBeVisible();
  await dialog.getByRole('textbox', { name: '成员路径关键词' }).fill('same'); await dialog.getByRole('button', { name: '搜索可选文件' }).click(); await expect(dialog.getByRole('button', { name: '添加包含 same.txt' })).toBeDisabled();
  await dialog.getByRole('combobox', { name: '成员来源空间' }).click(); await page.getByRole('option', { name: 'Member source', exact: true }).click(); await expect(dialog.getByRole('button', { name: '添加包含 same.txt' })).toHaveCount(0);
  const search = page.waitForRequest(request => request.url().endsWith('/api/v1/file-queries'));
  await dialog.getByRole('button', { name: '搜索可选文件' }).click(); expect((await search).postDataJSON().space_ids).toEqual([fixture.spaces[1]]); await dialog.getByRole('button', { name: '添加包含 same.txt' }).click(); await expect(dialog.getByRole('button', { name: '移除决策 same.txt' })).toHaveCount(2);
  let saves = 0, originalKey = '';
  await page.route('**/file-collections/' + fixture.collection + '/members/changes', async route => { saves++; originalKey = route.request().headers()['idempotency-key']; expect(route.request().postDataJSON().include.sort()).toEqual([...fixture.files].sort()); const response = await route.fetch(); expect(response.status()).toBe(200); await route.abort('failed'); });
  await dialog.getByRole('button', { name: '预览成员修改' }).click(); await expect(dialog.getByText('来源空间：Member source · ' + fixture.spaces[1], { exact: true })).toBeVisible(); await dialog.getByRole('button', { name: '确认保存成员决策' }).click(); await expect(dialog.getByRole('button', { name: '查询原成员回执' })).toBeVisible(); expect(saves).toBe(1);
  const stored = await page.evaluate(() => Object.entries(localStorage).filter(([key]) => key.startsWith('drivebridge:collection-members:')).map(([, value]) => JSON.parse(value))); expect(stored).toEqual([{ key: originalKey, digest: expect.stringMatching(/^[a-f0-9]{64}$/) }]);
  await page.reload(); await collections.getByRole('combobox', { name: '集合空间' }).click(); await page.getByRole('option', { name: 'Member home', exact: true }).click(); await trigger.click(); await dialog.getByRole('button', { name: '查询原成员回执' }).click(); await expect(dialog.getByText('已确认原成员修改', { exact: true })).toBeVisible(); expect(saves).toBe(1); await dialog.getByRole('button', { name: '确认成员回执并关闭' }).click(); await expect(trigger).toBeFocused();
  await trigger.click(); await dialog.getByRole('button', { name: '读取完整成员决策' }).click(); await expect(dialog.getByRole('button', { name: '移除决策 same.txt' })).toHaveCount(2); await expect(dialog.getByText('来源空间：Member source · ' + fixture.spaces[1], { exact: true })).toBeVisible();
  const decisions = (await (await page.request.get('/api/v1/file-collections/' + fixture.collection + '/members/decisions')).json()).data;
  expect(decisions.items.map((item: { file_id: string }) => item.file_id).sort()).toEqual([...fixture.files].sort());
  for (let i = 0; i < 2; i++) { const file = (await (await page.request.get('/api/v1/files/' + fixture.files[i])).json()).data.file; expect(file.space_id).toBe(fixture.spaces[i]); expect(file.path).toBe('same.txt'); }
  expect(content).toEqual([]);
});
