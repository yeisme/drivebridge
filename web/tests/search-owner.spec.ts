import { expect, test } from '@playwright/test';
test.use({ trace: 'off', video: 'off', screenshot: 'off' });
test('real path search and saved conditions use the same owner contract in page and command dialog', async ({ page }) => {
  await page.goto('/'); await page.getByRole('link', { name: '使用账号登录' }).click(); await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  const files = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    async function write(path: string, body: unknown, key?: string) {
      const res = await fetch('/api/v1' + path, { method: 'POST', headers: { ...headers, ...(key ? { 'Idempotency-Key': key } : {}) }, body: JSON.stringify(body) });
      if (!res.ok) throw new Error('search_fixture_failed'); return (await res.json()).data;
    }
    const space = await write('/spaces', { name: 'Search owner fixture', connection_id: 'platform' });
    const result: { file: string; version: string }[] = [];
    for (const path of ['search-owner/100%_report.md', 'search-owner/100XXreport.md']) {
      const bytes = new TextEncoder().encode('synthetic search content: ' + path);
      const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), b => b.toString(16).padStart(2, '0')).join('');
      const op = await write('/uploads', { space_id: space.id, path, size_bytes: bytes.length, sha256 }, 'search-owner-' + path);
      const part = await fetch('/api/v1/operations/' + op.id + '/parts/0', { method: 'PUT', headers: { 'X-CSRF-Token': me.csrf_token }, body: bytes });
      if (!part.ok) throw new Error('search_part_failed');
      const done = await write('/operations/' + op.id + '/complete', {});
      result.push({ file: done.file_id, version: done.version_id });
    }
    return { space: space.id as string, result };
  });
  const queryText = 'search-owner/100%_';
  await page.goto('/#/search'); await page.reload(); // Reload the shell's authorized spaces after fixture creation.
  const search = page.getByRole('region', { name: '云端检索', exact: true });
  await search.getByRole('searchbox').fill(queryText);
  await search.getByRole('button', { name: '检索', exact: true }).click();
  await expect(search.getByText(files.result[0].file, { exact: true })).toBeVisible();
  await expect(search.getByText(files.result[0].version, { exact: true })).toBeVisible();
  await expect(search.getByText(files.result[1].file, { exact: true })).toHaveCount(0);
  await search.getByRole('button', { name: '保存条件', exact: true }).click();
  await expect(search.getByText('检索条件已保存；再次使用时会重新授权读取。', { exact: true })).toBeVisible();
  const saved = await page.evaluate(async text => {
    const response = await fetch('/api/v1/saved-searches'); if (!response.ok) throw new Error('saved_search_read_failed');
    const rows = (await response.json()).data;
    const row = rows.find((candidate: { name: string }) => candidate.name === text);
    if (!row) throw new Error('saved_search_missing');
    return row.query;
  }, queryText);
  expect(saved.mode).toBe('exact'); expect(saved.space_ids).toContain(files.space);
  expect(saved.filters).toEqual([{ field: 'path_prefix', op: 'contains', value: queryText }]);
  const replacement = await page.evaluate(async original => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    const bytes = new TextEncoder().encode('updated search document');
    const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), b => b.toString(16).padStart(2, '0')).join('');
    const created = await fetch('/api/v1/uploads', { method: 'POST', headers: { ...headers, 'Idempotency-Key': 'search-fixed-replacement' }, body: JSON.stringify({ space_id: original.space, path: 'search-owner/100%_report.md', file_id: original.result[0].file, expected_version: original.result[0].version, size_bytes: bytes.length, sha256 }) });
    if (!created.ok) throw new Error('replacement_create_failed'); const op = (await created.json()).data;
    const part = await fetch('/api/v1/operations/' + op.id + '/parts/0', { method: 'PUT', headers: { 'X-CSRF-Token': me.csrf_token }, body: bytes });
    if (!part.ok) throw new Error('replacement_part_failed');
    const completed = await fetch('/api/v1/operations/' + op.id + '/complete', { method: 'POST', headers, body: '{}' });
    if (!completed.ok) throw new Error('replacement_complete_failed');
    return (await completed.json()).data.version_id as string;
  }, files);
  const bodyVersions: string[] = [];
  page.on('request', request => { const url = new URL(request.url()); if (url.pathname === '/api/v1/files/' + files.result[0].file + '/content') bodyVersions.push(url.searchParams.get('version') || ''); });
  const open = search.getByRole('button', { name: '查看搜索固定版本 ' + files.result[0].file, exact: true });
  await open.click();
  const fixed = search.getByRole('region', { name: '搜索结果固定引用', exact: true });
  await expect(fixed.getByRole('button', { name: '读取搜索原版本正文' })).toBeVisible();
  expect(bodyVersions).toEqual([]);
  await fixed.getByRole('button', { name: '读取搜索原版本正文' }).click();
  await expect(fixed.getByLabel('搜索原版本纯文本', { exact: true })).toHaveText('synthetic search content: search-owner/100%_report.md');
  expect(bodyVersions).toEqual([files.result[0].version]);
  await fixed.getByRole('button', { name: '关闭搜索结果引用' }).click();
  await expect(open).toBeFocused(); await expect(fixed).toHaveCount(0);
  await page.keyboard.press('Control+k');
  const dialog = page.getByRole('dialog', { name: '搜索工作区', exact: true });
  await dialog.getByRole('searchbox').fill(queryText);
  await dialog.getByRole('button', { name: '检索', exact: true }).click();
  await expect(dialog.getByText(files.result[0].file, { exact: true })).toBeVisible();
  await expect(dialog.getByText(files.result[1].file, { exact: true })).toHaveCount(0);
  await expect(dialog.getByText(replacement, { exact: true })).toBeVisible();
  await dialog.getByRole('button', { name: '查看搜索固定版本 ' + files.result[0].file, exact: true }).click();
  await dialog.getByRole('button', { name: '读取搜索原版本正文' }).click();
  await expect(dialog.getByLabel('搜索原版本纯文本', { exact: true })).toHaveText('updated search document');
  expect(bodyVersions).toEqual([files.result[0].version, replacement]);
  await dialog.getByRole('button', { name: '关闭', exact: true }).click();
  const resumedQueries: string[] = [], savedWrites: string[] = [];
  page.on('request', request => {
    const path = new URL(request.url()).pathname;
    if (path === '/api/v1/file-queries') resumedQueries.push(path);
    if (path === '/api/v1/saved-searches' && request.method() !== 'GET') savedWrites.push(path);
  });
  await page.reload();
  await search.getByRole('button', { name: '查看已保存条件', exact: true }).click();
  const stored = search.getByRole('region', { name: '已保存检索条件', exact: true });
  await stored.getByRole('button', { name: '选用条件 ' + queryText, exact: true }).click();
  await expect(search.getByRole('searchbox')).toHaveValue(queryText);
  await expect(search.getByRole('searchbox')).toBeFocused();
  expect(resumedQueries).toEqual([]); expect(savedWrites).toEqual([]);
  await search.getByRole('button', { name: '检索', exact: true }).click();
  await expect(search.getByText(files.result[0].file, { exact: true })).toBeVisible();
  expect(resumedQueries).toHaveLength(1); expect(savedWrites).toEqual([]);
});
