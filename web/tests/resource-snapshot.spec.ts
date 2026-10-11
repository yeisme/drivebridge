import { expect, test } from '@playwright/test';
test.use({ trace: 'off', video: 'off', screenshot: 'off' });
test('resource package reads captured member version after the live file changes', async ({ page }) => {
  await page.goto('/'); await page.getByRole('link', { name: '使用账号登录' }).click(); await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  const original = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    async function write(path: string, body: unknown, key?: string, method = 'POST') {
      const response = await fetch('/api/v1' + path, { method, headers: { ...headers, ...(key ? { 'Idempotency-Key': key } : {}) }, body: JSON.stringify(body) });
      if (!response.ok) throw new Error('package_fixture_failed'); return (await response.json()).data;
    }
    const space = await write('/spaces', { name: 'Snapshot resource fixture', connection_id: 'platform' });
    async function upload(text: string, key: string, prior?: { file_id: string; version_id: string }) {
      const bytes = new TextEncoder().encode(text);
      const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), b => b.toString(16).padStart(2, '0')).join('');
      const op = await write('/uploads', { space_id: space.id, path: 'package/source.txt', size_bytes: bytes.length, sha256, ...(prior ? { file_id: prior.file_id, expected_version: prior.version_id } : {}) }, key);
      const part = await fetch('/api/v1/operations/' + op.id + '/parts/0', { method: 'PUT', headers: { 'X-CSRF-Token': me.csrf_token }, body: bytes });
      if (!part.ok) throw new Error('package_part_failed'); return write('/operations/' + op.id + '/complete', {});
    }
    const file = await upload('Original package member', 'package-source');
    const collection = await write('/file-collections', { space_id: space.id, name: 'Frozen references', kind: 'manual', scope: 'private' }, 'package-collection');
    await write('/file-collections/' + collection.collection_id + '/members', { expected_revision: collection.revision, include: [file.file_id] }, undefined, 'PUT');
    const snapshot = await write('/file-collections/' + collection.collection_id + '/snapshots', {});
    await write('/resource-descriptors', { space_id: space.id, collection_snapshot_id: snapshot.snapshot_id, kind: 'reference_pack', display_name: 'Frozen package' }, 'package-descriptor');
    const latest = await upload('Replacement must not appear', 'package-replacement', file);
    return { file: file.file_id as string, version: file.version_id as string, latest: latest.version_id as string };
  });
  const bodies: string[] = [], writes: string[] = [];
  page.on('request', request => { const url = new URL(request.url()); if (url.pathname.endsWith('/content')) bodies.push(url.searchParams.get('version') || ''); if (url.pathname.includes('/api/v1/') && request.method() !== 'GET') writes.push(url.pathname); });
  await page.goto('/#/resources');
  const resources = page.getByRole('region', { name: '资源库', exact: true });
  const descriptor = resources.getByRole('button', { name: '查看资源描述 Frozen package', exact: true }); await descriptor.click();
  const openMembers = resources.getByRole('button', { name: '查看固定快照成员', exact: true }); await openMembers.click();
  const members = resources.getByRole('region', { name: '资料包固定成员', exact: true });
  await expect(members.getByText('历史条目：1 · 本次可见：1', { exact: true })).toBeVisible();
  const member = members.getByRole('button', { name: '打开固定成员：package/source.txt', exact: true }); await member.click();
  const read = members.getByRole('button', { name: '读取资料包成员原版本正文', exact: true }); await expect(read).toBeVisible();
  expect(bodies).toEqual([]); await read.click();
  await expect(members.getByLabel('资料包成员原版本纯文本', { exact: true })).toHaveText('Original package member');
  expect(bodies).toEqual([original.version]); expect(original.version).not.toBe(original.latest); expect(writes).toEqual([]);
  await members.getByRole('button', { name: '关闭资料包成员结果引用' }).click(); await expect(member).toBeFocused();
  await members.getByRole('button', { name: '关闭资料包成员' }).click(); await expect(openMembers).toBeFocused();
  await resources.getByRole('button', { name: '关闭资源描述' }).click(); await expect(descriptor).toBeFocused();
});
