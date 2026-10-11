import { expect, test, type Page } from '@playwright/test';
test.use({ trace: 'off', video: 'off', screenshot: 'off' });
async function upload(page: Page, space: string, path: string, key: string) {
  return page.evaluate(async ({ space, path, key }) => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    async function post(path: string, body: unknown) {
      const response = await fetch('/api/v1' + path, { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token, 'Idempotency-Key': key }, body: JSON.stringify(body) });
      if (!response.ok) throw new Error('rule_fixture_failed'); return (await response.json()).data;
    }
    const bytes = new TextEncoder().encode(key);
    const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), b => b.toString(16).padStart(2, '0')).join('');
    const op = await post('/uploads', { space_id: space, path, size_bytes: bytes.length, sha256 });
    if (op.status !== 'completed') { const part = await fetch('/api/v1/operations/' + op.id + '/parts/0', { method: 'PUT', headers: { 'X-CSRF-Token': me.csrf_token }, body: bytes }); if (!part.ok) throw new Error('rule_part_failed'); }
    const done = await post('/operations/' + op.id + '/complete', {}); if (done.status !== 'completed') throw new Error('rule_upload_incomplete'); return done.file_id as string;
  }, { space, path, key });
}
test('formal rule creation preserves original receipt and observes dynamic members after new upload', async ({ page, browser }) => {
  await page.goto('/'); await page.getByRole('link', { name: '使用账号登录' }).click(); await page.getByRole('link', { name: '以 Alice 登录' }).click(); await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  const space = await page.evaluate(async () => { const me = (await (await fetch('/api/v1/me')).json()).data; const response = await fetch('/api/v1/spaces', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token }, body: JSON.stringify({ name: 'Rule creation space', connection_id: 'platform' }) }); if (!response.ok) throw new Error('rule_space_failed'); return (await response.json()).data.id as string; });
  const first = await upload(page, space, 'reports/first.txt', 'rule-first'); await upload(page, space, 'other.txt', 'rule-other');
  await page.reload(); await page.getByRole('navigation', { name: '工作区导航' }).getByRole('button', { name: '项目集合', exact: true }).click();
  const collections = page.getByRole('region', { name: '项目集合', exact: true }); await collections.getByRole('combobox', { name: '集合空间' }).click(); await page.getByRole('option', { name: 'Rule creation space', exact: true }).click();
  const trigger = collections.getByRole('button', { name: '新建私有规则集合', exact: true }); await trigger.click();
  const dialog = page.getByRole('dialog', { name: '新建私有规则集合', exact: true }); const name = '规则 <>& "quote" \\ \u2028line\u2029';
  await dialog.getByRole('textbox', { name: '集合名称', exact: true }).fill(name); await dialog.getByRole('button', { name: '添加规则条件' }).click(); await dialog.getByRole('textbox', { name: '条件 1 值', exact: true }).fill('reports/');
  // Include special characters inside the rule itself to exercise the nested
  // JSON-string fingerprint, not just the outer collection name.
  await dialog.getByRole('button', { name: '添加规则条件' }).click(); await dialog.getByRole('combobox', { name: '条件 2 字段' }).click(); await page.getByRole('option', { name: '人工标签', exact: true }).click(); await dialog.getByRole('combobox', { name: '条件 2 操作' }).click(); await page.getByRole('option', { name: '不等于', exact: true }).click();
  const label = '未标记 <>& "quote" \\ \u2028line\u2029'; await dialog.getByRole('textbox', { name: '条件 2 值' }).fill(label);
  const discardedWrites: string[] = [];
  const observeWrites = (request: import('@playwright/test').Request) => { if (new URL(request.url()).pathname.startsWith('/api/v1/') && request.method() !== 'GET') discardedWrites.push(request.url()); };
  page.on('request', observeWrites);
  const deniedLeave = page.waitForEvent('dialog');
  const deniedHash = page.evaluate(() => { location.hash = '#/files'; });
  const warning = await deniedLeave; expect(warning.type()).toBe('confirm'); await warning.dismiss(); await deniedHash;
  await expect(page).toHaveURL(/#\/collections$/);
  await expect(dialog.getByRole('textbox', { name: '集合名称', exact: true })).toHaveValue(name);
  await expect(dialog.getByRole('textbox', { name: '条件 2 值' })).toHaveValue(label);
  const acceptedLeave = page.waitForEvent('dialog');
  const acceptedHash = page.evaluate(() => { location.hash = '#/files'; });
  await (await acceptedLeave).accept(); await acceptedHash;
  await expect(page).toHaveURL(/#\/files$/); await expect(dialog).toHaveCount(0);
  expect(discardedWrites).toEqual([]); page.off('request', observeWrites);
  expect(await page.evaluate(() => Object.keys(localStorage).filter(key => key.startsWith('drivebridge:collection-create:')))).toEqual([]);
  await page.getByRole('navigation', { name: '工作区导航' }).getByRole('button', { name: '项目集合', exact: true }).click();
  await collections.getByRole('combobox', { name: '集合空间' }).click(); await page.getByRole('option', { name: 'Rule creation space', exact: true }).click(); await trigger.click();
  await expect(dialog.getByRole('textbox', { name: '集合名称', exact: true })).toHaveValue('');
  await dialog.getByRole('textbox', { name: '集合名称', exact: true }).fill(name);
  await dialog.getByRole('button', { name: '添加规则条件' }).click(); await dialog.getByRole('textbox', { name: '条件 1 值', exact: true }).fill('reports/');
  await dialog.getByRole('button', { name: '添加规则条件' }).click(); await dialog.getByRole('combobox', { name: '条件 2 字段' }).click(); await page.getByRole('option', { name: '人工标签', exact: true }).click(); await dialog.getByRole('combobox', { name: '条件 2 操作' }).click(); await page.getByRole('option', { name: '不等于', exact: true }).click(); await dialog.getByRole('textbox', { name: '条件 2 值' }).fill(label);
  let creates = 0, key = '', id = ''; const content: string[] = [];
  page.on('request', req => { if (new URL(req.url()).pathname.endsWith('/content')) content.push(req.url()); });
  await page.route('**/api/v1/file-collections', async route => { if (route.request().method() !== 'POST') return route.continue(); creates++; key = route.request().headers()['idempotency-key']; const response = await route.fetch(); expect(response.status()).toBe(200); const data = (await response.json()).data; id = data.collection_id; expect(JSON.parse(data.rule).filters[1].value).toBe(label); await route.abort('failed'); });
  await dialog.getByRole('button', { name: '准备创建集合' }).click(); expect(creates).toBe(0); await expect(dialog.getByText(/不是固定快照/)).toBeVisible(); await dialog.getByRole('button', { name: '确认创建集合' }).click(); await expect(dialog.getByRole('button', { name: '查询原集合回执' })).toBeVisible(); expect(creates).toBe(1);
  const stored = await page.evaluate(() => Object.entries(localStorage).filter(([key]) => key.startsWith('drivebridge:collection-create:')).map(([, value]) => JSON.parse(value))); expect(stored).toEqual([{ key, digest: expect.stringMatching(/^[a-f0-9]{64}$/) }]);
  await page.reload(); await collections.getByRole('combobox', { name: '集合空间' }).click(); await page.getByRole('option', { name: 'Rule creation space', exact: true }).click(); await trigger.click(); await dialog.getByRole('button', { name: '查询原集合回执' }).click(); await expect(dialog.getByText('已确认原集合创建', { exact: true })).toBeVisible(); expect(creates).toBe(1); await dialog.getByRole('button', { name: '确认回执并关闭' }).click(); await expect(trigger).toBeFocused();
  const before = (await (await page.request.get('/api/v1/file-collections/' + id)).json()).data; expect(before.items.map((item: { file_id: string }) => item.file_id)).toEqual([first]);
  const second = await upload(page, space, 'reports/second.txt', 'rule-second'); const after = (await (await page.request.get('/api/v1/file-collections/' + id)).json()).data; expect(after.items.map((item: { file_id: string }) => item.file_id).sort()).toEqual([first, second].sort()); expect(after.revision).toBe(before.revision);
  await collections.getByRole('button', { name: '刷新集合列表' }).click(); await collections.getByRole('button', { name: '查看集合 ' + name, exact: true }).click(); await expect(page.getByRole('button', { name: '查看成员固定引用：reports/first.txt', exact: true })).toBeVisible(); await expect(page.getByRole('button', { name: '查看成员固定引用：reports/second.txt', exact: true })).toBeVisible(); expect(content).toEqual([]);
  const context = await browser.newContext(); try { const bob = await context.newPage(); await bob.goto('/'); await bob.getByRole('link', { name: '使用账号登录' }).click(); await bob.getByRole('link', { name: '以 Bob 登录' }).click(); await expect(bob.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible(); expect((await bob.request.get('/api/v1/file-collections/' + id)).status()).toBe(404); const status = await bob.evaluate(async key => { const me = (await (await fetch('/api/v1/me')).json()).data; return (await fetch('/api/v1/file-collections/lookup', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token }, body: JSON.stringify({ idempotency_key: key }) })).status; }, key); expect(status).toBe(404); } finally { await context.close(); }
});
