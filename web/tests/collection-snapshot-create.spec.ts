import { expect, test } from '@playwright/test';
import path from 'node:path';
test.use({ trace: 'off', video: 'off', screenshot: 'off' });
test('formal snapshot creation recovers original identity and reads captured version', async ({ page, browser }) => {
  await page.goto('/'); await page.getByRole('link', { name: '使用账号登录' }).click(); await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  const original = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    async function write(path: string, body: unknown, key?: string, method = 'POST') {
      const response = await fetch('/api/v1' + path, { method, headers: { ...headers, ...(key ? { 'Idempotency-Key': key } : {}) }, body: JSON.stringify(body) });
      if (!response.ok) throw new Error('snapshot_fixture_failed'); return (await response.json()).data;
    }
    const space = await write('/spaces', { name: 'Snapshot creation UI', connection_id: 'platform' });
    const bytes = new TextEncoder().encode('Captured by original snapshot creation');
    const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), byte => byte.toString(16).padStart(2, '0')).join('');
    const operation = await write('/uploads', { space_id: space.id, path: 'snapshot/source.txt', size_bytes: bytes.length, sha256 }, 'snapshot-ui-file');
    const part = await fetch('/api/v1/operations/' + operation.id + '/parts/0', { method: 'PUT', headers: { 'X-CSRF-Token': me.csrf_token }, body: bytes });
    if (!part.ok) throw new Error('snapshot_fixture_part_failed');
    const file = await write('/operations/' + operation.id + '/complete', {});
    const collection = await write('/file-collections', { space_id: space.id, name: 'Snapshot UI collection', kind: 'manual', scope: 'private' }, 'snapshot-ui-collection');
    await write('/file-collections/' + collection.collection_id + '/members', { expected_revision: 1, include: [file.file_id] }, undefined, 'PUT');
    return { space: space.id as string, collection: collection.collection_id as string, file: file.file_id as string, version: file.version_id as string };
  });
  await page.reload(); await page.getByRole('navigation', { name: '工作区导航' }).getByRole('button', { name: '项目集合', exact: true }).click();
  const collections = page.getByRole('region', { name: '项目集合', exact: true });
  await collections.getByRole('combobox', { name: '集合空间' }).click(); await page.getByRole('option', { name: 'Snapshot creation UI', exact: true }).click();
  let creates = 0, key = '', snapshot = ''; const bodies: string[] = []; const registrations: string[] = [];
  page.on('request', request => { const url = new URL(request.url()); if (url.pathname.endsWith('/content')) bodies.push(url.searchParams.get('version') || ''); if (request.method() === 'POST' && url.pathname === '/api/v1/resource-descriptors') registrations.push(url.pathname); });
  await page.route('**/api/v1/file-collections/' + original.collection + '/snapshots/creations', async route => {
    creates++; key = route.request().headers()['idempotency-key']; const response = await route.fetch();
    if (response.status() !== 200) throw new Error('snapshot_ui_creation_failed'); snapshot = (await response.json()).data.snapshot_id; await route.abort('failed');
  });
  const trigger = collections.getByRole('button', { name: '创建快照 Snapshot UI collection', exact: true }); await trigger.click();
  let dialog = page.getByRole('dialog', { name: '创建集合快照', exact: true }); await dialog.getByRole('button', { name: '预览当前可见成员' }).click();
  await expect(dialog.getByText('snapshot/source.txt', { exact: true })).toBeVisible(); expect(creates).toBe(0); expect(bodies).toEqual([]);
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await page.screenshot({ path: path.join(process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!, `snapshot-confirm-${width}.png`), animations: 'disabled' });
  }
  await page.setViewportSize({ width: 1440, height: 900 });
  await dialog.getByRole('button', { name: '确认创建提交时快照' }).click(); await expect(dialog.getByRole('button', { name: '查询原快照回执' })).toBeVisible();
  expect(creates).toBe(1); expect(snapshot).not.toBe('');
  const stored = await page.evaluate(() => Object.entries(localStorage).filter(([key]) => key.startsWith('drivebridge:snapshot-create:')).map(([, value]) => JSON.parse(value)));
  expect(stored).toEqual([{ key, digest: expect.stringMatching(/^[a-f0-9]{64}$/) }]);
  await page.evaluate(async original => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const response = await fetch('/api/v1/files/' + original.file + '/document', { method: 'PUT', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token, 'Idempotency-Key': 'snapshot-ui-new-head' }, body: JSON.stringify({ base_version_id: original.version, text: 'Later head must not replace snapshot' }) });
    if (!response.ok) throw new Error('snapshot_ui_head_failed');
  }, original);
  await page.reload(); await collections.getByRole('combobox', { name: '集合空间' }).click(); await page.getByRole('option', { name: 'Snapshot creation UI', exact: true }).click(); await trigger.click();
  dialog = page.getByRole('dialog', { name: '创建集合快照', exact: true }); await dialog.getByRole('button', { name: '查询原快照回执' }).click();
  await expect(dialog.getByText('快照标识：' + snapshot, { exact: true })).toBeVisible(); expect(creates).toBe(1); expect(bodies).toEqual([]);
  await dialog.getByRole('button', { name: '查看原快照成员' }).click(); await dialog.getByRole('button', { name: '打开固定成员：snapshot/source.txt' }).click();
  const read = dialog.getByRole('button', { name: '读取资料包成员原版本正文' }); await expect(read).toBeVisible(); expect(bodies).toEqual([]); await read.click();
  await expect(dialog.getByLabel('资料包成员原版本纯文本', { exact: true })).toHaveText('Captured by original snapshot creation'); expect(bodies).toEqual([original.version]); expect(registrations).toEqual([]);
  await dialog.getByRole('button', { name: '关闭资料包成员结果引用' }).click(); await dialog.getByRole('button', { name: '关闭资料包成员' }).click(); await expect(dialog.getByRole('button', { name: '查看原快照成员' })).toBeFocused();
  await dialog.getByRole('button', { name: '确认快照回执并关闭' }).click(); await expect(trigger).toBeFocused();
  const context = await browser.newContext();
  try {
    const bob = await context.newPage(); await bob.goto('/'); await bob.getByRole('link', { name: '使用账号登录' }).click(); await bob.getByRole('link', { name: '以 Bob 登录' }).click();
    expect((await bob.request.get('/api/v1/collection-snapshots/' + snapshot)).status()).toBe(404);
    const status = await bob.evaluate(async ({ collection, key }) => {
      const me = (await (await fetch('/api/v1/me')).json()).data;
      return (await fetch('/api/v1/file-collections/' + collection + '/snapshots/creations/lookup', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token }, body: JSON.stringify({ idempotency_key: key }) })).status;
    }, { collection: original.collection, key }); expect(status).toBe(404);
  } finally { await context.close(); }
});
