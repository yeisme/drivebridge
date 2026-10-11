import { expect, test } from '@playwright/test';
test.use({ trace: 'off', video: 'off', screenshot: 'off' });
test('reference property requires current target visibility while original receipt remains a historical fact', async ({ page, browser }) => {
  expect(process.env.DRIVEBRIDGE_TEST_PROPERTY_EDITING).toBe('1');
  await page.goto('/'); await page.getByRole('link', { name: '使用账号登录' }).click(); await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  const fixture = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    async function write(path: string, body: unknown, key: string, method = 'POST') {
      const response = await fetch('/api/v1' + path, { method, headers: { ...headers, 'Idempotency-Key': key }, body: JSON.stringify(body) });
      if (!response.ok) throw new Error('reference_fixture_failed'); return (await response.json()).data;
    }
    const space = await write('/spaces', { name: 'Reference property fixture', connection_id: 'platform' }, 'reference-space');
    async function upload(path: string, bytes: Uint8Array) {
      const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes as Uint8Array<ArrayBuffer>)), byte => byte.toString(16).padStart(2, '0')).join('');
      const op = await write('/uploads', { space_id: space.id, path, size_bytes: bytes.length, sha256 }, 'reference-upload-' + path);
      // Another test may already have published these synthetic image bytes;
      // completed blob-reuse operations must not receive another upload part.
      if (op.status !== 'completed') {
        const part = await fetch('/api/v1/operations/' + op.id + '/parts/0', { method: 'PUT', headers: { 'X-CSRF-Token': me.csrf_token }, body: bytes as Uint8Array<ArrayBuffer> }); if (!part.ok) throw new Error('reference_part_failed');
      }
      const result = await write('/operations/' + op.id + '/complete', {}, 'reference-complete-' + path);
      if (result.status !== 'completed') throw new Error('reference_upload_incomplete'); return result;
    }
    const image = await upload('reference.png', Uint8Array.from(atob('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+X2ioAAAAASUVORK5CYII='), ch => ch.charCodeAt(0)));
    const target = await upload('source.txt', new TextEncoder().encode('Synthetic referenced content'));
    await write('/spaces/' + space.id + '/property-definitions', { expected_revision: 0, definitions: [{ key: 'source', display_name: '来源引用', group: '', type: 'reference' }] }, 'reference-definitions', 'PUT');
    const stat = await fetch('/api/v1/files/' + target.file_id); if (!stat.ok) throw new Error('reference_stat_failed');
    return { file: image.file_id as string, version: image.version_id as string, target: target.file_id as string, ref: (await stat.json()).data.ref as string };
  });
  await page.reload(); await page.getByRole('button', { name: '打开网盘 Reference property fixture', exact: true }).click();
  async function open(detail = true) { if (detail) await page.getByRole('button', { name: '详情 reference.png', exact: true }).click(); await page.getByRole('button', { name: '查看文件属性', exact: true }).click(); }
  const dialog = page.getByRole('dialog', { name: '固定版本文件属性', exact: true });
  const path = '/api/v1/files/' + fixture.file + '/properties';
  async function mutation(path: string, body: unknown = {}, method = 'POST', key = '') {
    return page.evaluate(async ({ path, body, method, key }) => {
      const me = (await (await fetch('/api/v1/me')).json()).data;
      const response = await fetch(path, { method, headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token, ...(key ? { 'Idempotency-Key': key } : {}) }, body: JSON.stringify(body) });
      return { status: response.status, body: await response.json() };
    }, { path, body, method, key });
  }
  await open(); await dialog.getByRole('button', { name: '读取固定版本属性' }).click(); await dialog.getByRole('button', { name: '来源引用 设置值' }).click(); await dialog.getByRole('textbox', { name: '来源引用 新值' }).fill(fixture.ref);
  let writes = 0, originalKey = ''; let originalReceipt: unknown;
  await page.route('**' + path, async route => {
    if (route.request().method() !== 'PATCH') return route.continue();
    writes++; originalKey = route.request().headers()['idempotency-key']; const response = await route.fetch(); expect(response.status()).toBe(200); originalReceipt = (await response.json()).data; await route.abort('failed');
  });
  await dialog.getByRole('button', { name: '预览属性修改' }).click(); await dialog.getByRole('button', { name: '确认保存文件属性' }).click(); await expect(dialog.getByRole('button', { name: '查询原属性回执' })).toBeVisible(); expect(writes).toBe(1);
  expect((await mutation('/api/v1/files/' + fixture.target + '/trash')).status).toBe(200);
  const hidden = await page.request.get(path + '?version_id=' + fixture.version); expect(hidden.status()).toBe(404); const hiddenBody = await hidden.json(); expect(hiddenBody.data).toBeUndefined(); expect(JSON.stringify(hiddenBody)).not.toContain(fixture.ref);
  await page.reload(); await open(); await expect(dialog.getByRole('button', { name: '读取固定版本属性' })).toHaveCount(0);
  await dialog.getByRole('button', { name: '查询原属性回执' }).click(); await expect(dialog.getByText('已确认原文件属性修改', { exact: true })).toBeVisible(); expect(writes).toBe(1);
  const recovered = await mutation(path + '/lookup', { version_id: fixture.version, idempotency_key: originalKey }); expect(recovered.status).toBe(200); expect(recovered.body.data).toEqual(originalReceipt); expect(recovered.body.data.values).toBeUndefined();
  await dialog.getByRole('button', { name: '确认属性回执并关闭' }).click(); await open(false); await dialog.getByRole('button', { name: '读取固定版本属性' }).click(); await expect(dialog.getByText(/不代表没有属性/)).toBeVisible(); await expect(dialog.getByRole('button', { name: '来源引用 设置值' })).toHaveCount(0); await expect(dialog.getByText(fixture.ref, { exact: true })).toHaveCount(0);
  expect((await mutation('/api/v1/files/' + fixture.target + '/restore')).status).toBe(200);
  await dialog.getByRole('button', { name: '读取固定版本属性' }).click(); await expect(dialog.getByText('source · 当前值：' + JSON.stringify(fixture.ref), { exact: true })).toBeVisible();
  await page.unroute('**' + path);
  const foreignRef = fixture.ref.replace(/^drivebridge:\/\/[^/]+\//, 'drivebridge://foreign-instance/');
  const invalid = await mutation(path, { base_version_id: fixture.version, definition_revision: 1, expected_revision: 1, updates: { source: foreignRef }, remove_keys: [] }, 'PATCH', 'foreign-reference'); expect(invalid.status).toBe(403);
  expect((await mutation(path + '/lookup', { version_id: fixture.version, idempotency_key: 'foreign-reference' })).status).toBe(404);
  expect((await (await page.request.get(path + '?version_id=' + fixture.version)).json()).data.values).toEqual({ source: fixture.ref });
  const context = await browser.newContext();
  try {
    const bob = await context.newPage(); await bob.goto('/'); await bob.getByRole('link', { name: '使用账号登录' }).click(); await bob.getByRole('link', { name: '以 Bob 登录' }).click(); await expect(bob.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
    expect((await bob.request.get(path + '?version_id=' + fixture.version)).status()).toBe(404);
    const status = await bob.evaluate(async ({ path, version, key }) => { const me = (await (await fetch('/api/v1/me')).json()).data; return (await fetch(path + '/lookup', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token }, body: JSON.stringify({ version_id: version, idempotency_key: key }) })).status; }, { path, version: fixture.version, key: originalKey }); expect(status).toBe(404);
  } finally { await context.close(); }
  // The receipt is historical, but the containing file still needs current authorization.
  expect((await mutation('/api/v1/files/' + fixture.file + '/trash')).status).toBe(200);
  expect((await mutation(path + '/lookup', { version_id: fixture.version, idempotency_key: originalKey })).status).toBe(404);
  expect((await mutation('/api/v1/files/' + fixture.file + '/restore')).status).toBe(200);
  expect((await mutation(path + '/lookup', { version_id: fixture.version, idempotency_key: originalKey })).body.data).toEqual(originalReceipt);
});
