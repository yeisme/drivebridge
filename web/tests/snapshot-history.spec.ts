import { expect, test } from '@playwright/test';
test.use({ trace: 'off', video: 'off', screenshot: 'off' });
test('formal snapshot history observes legacy and keyed headers then reads the old fixed version', async ({ page, browser }) => {
  await page.goto('/'); await page.getByRole('link', { name: '使用账号登录' }).click(); await page.getByRole('link', { name: '以 Alice 登录' }).click(); await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  const original = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    async function write(path: string, body: unknown, key?: string, method = 'POST') {
      const response = await fetch('/api/v1' + path, { method, headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token, ...(key ? { 'Idempotency-Key': key } : {}) }, body: JSON.stringify(body) });
      if (!response.ok) throw new Error('history_fixture_failed'); return (await response.json()).data;
    }
    const space = await write('/spaces', { name: 'Snapshot history UI', connection_id: 'platform' });
    const bytes = new TextEncoder().encode('Original history member bytes'); const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), b => b.toString(16).padStart(2, '0')).join('');
    const op = await write('/uploads', { space_id: space.id, path: 'history-source.txt', size_bytes: bytes.length, sha256 }, 'history-ui-upload');
    if (op.status !== 'completed') { const part = await fetch('/api/v1/operations/' + op.id + '/parts/0', { method: 'PUT', headers: { 'X-CSRF-Token': me.csrf_token }, body: bytes }); if (!part.ok) throw new Error('history_part_failed'); }
    const file = await write('/operations/' + op.id + '/complete', {}); if (file.status !== 'completed') throw new Error('history_upload_incomplete');
    const collection = await write('/file-collections', { space_id: space.id, name: 'History collection', kind: 'manual', scope: 'private' }, 'history-ui-collection');
    await write('/file-collections/' + collection.collection_id + '/members', { expected_revision: 1, include: [file.file_id], exclude: [] }, undefined, 'PUT');
    const legacy = await write('/file-collections/' + collection.collection_id + '/snapshots', {});
    const saved = await write('/files/' + file.file_id + '/document', { base_version_id: file.version_id, text: 'New head history bytes' }, 'history-new-head', 'PUT');
    const later = await write('/file-collections/' + collection.collection_id + '/snapshots/creations', { expected_revision: 2 }, 'history-later-snapshot');
    return { collection: collection.collection_id as string, version: file.version_id as string, newer: saved.version_id as string, legacy: legacy.snapshot_id as string, later: later.snapshot_id as string };
  });
  expect(original.newer).not.toBe(original.version);
  await page.reload(); await page.getByRole('navigation', { name: '工作区导航' }).getByRole('button', { name: '项目集合', exact: true }).click();
  const collections = page.getByRole('region', { name: '项目集合', exact: true }); await collections.getByRole('combobox', { name: '集合空间' }).click(); await page.getByRole('option', { name: 'Snapshot history UI', exact: true }).click();
  const reads: string[] = [], writes: string[] = [], bodies: string[] = [];
  page.on('request', req => { const url = new URL(req.url()); if (url.pathname.startsWith('/api/v1/')) { if (req.method() !== 'GET') writes.push(url.pathname); else reads.push(url.pathname); if (url.pathname.endsWith('/content')) bodies.push(url.searchParams.get('version') || ''); } });
  const trigger = collections.getByRole('button', { name: '历史快照 History collection', exact: true }); await trigger.click();
  const dialog = page.getByRole('dialog', { name: '集合历史快照', exact: true }); await expect(dialog.getByRole('button', { name: '读取历史快照索引' })).toBeVisible(); expect(reads).toEqual([]);
  await dialog.getByRole('button', { name: '读取历史快照索引' }).click(); const historyList = dialog.getByRole('list', { name: '历史快照索引' }); await expect(historyList.getByRole('listitem')).toHaveCount(2);
  expect(reads).toEqual(['/api/v1/file-collections/' + original.collection + '/snapshots']); expect(writes).toEqual([]); expect(bodies).toEqual([]);
  await expect(historyList.getByRole('button').first()).toHaveText('查看历史快照 ' + original.later);
  const old = dialog.getByRole('button', { name: '查看历史快照 ' + original.legacy, exact: true }); await old.click();
  const member = dialog.getByRole('button', { name: '打开固定成员：history-source.txt', exact: true }); await member.click();
  const read = dialog.getByRole('button', { name: '读取资料包成员原版本正文', exact: true }); await expect(read).toBeVisible(); expect(bodies).toEqual([]); await read.click();
  await expect(dialog.getByLabel('资料包成员原版本纯文本', { exact: true })).toHaveText('Original history member bytes'); expect(bodies).toEqual([original.version]); expect(writes).toEqual([]);
  await dialog.getByRole('button', { name: '关闭资料包成员结果引用' }).click(); await expect(member).toBeFocused(); await dialog.getByRole('button', { name: '关闭资料包成员', exact: true }).click(); await expect(old).toBeFocused();
  await dialog.getByRole('button', { name: '关闭', exact: true }).click(); await expect(trigger).toBeFocused();
  const context = await browser.newContext(); try { const bob = await context.newPage(); await bob.goto('/'); await bob.getByRole('link', { name: '使用账号登录' }).click(); await bob.getByRole('link', { name: '以 Bob 登录' }).click(); await expect(bob.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible(); expect((await bob.request.get('/api/v1/file-collections/' + original.collection + '/snapshots')).status()).toBe(404); expect((await bob.request.get('/api/v1/collection-snapshots/' + original.legacy)).status()).toBe(404); } finally { await context.close(); }
});
