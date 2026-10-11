import { expect, test } from '@playwright/test';
import path from 'node:path';
test.use({ trace: 'off', video: 'off', screenshot: 'off' });
test('formal collections route reads original observed member after head changes', async ({ page, browser }) => {
  await page.goto('/'); await page.getByRole('link', { name: '使用账号登录' }).click(); await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  const original = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    async function write(path: string, body: unknown, key?: string, method = 'POST') {
      const response = await fetch('/api/v1' + path, { method, headers: { ...headers, ...(key ? { 'Idempotency-Key': key } : {}) }, body: JSON.stringify(body) });
      if (!response.ok) throw new Error('collection_fixture_failed'); return (await response.json()).data;
    }
    const space = await write('/spaces', { name: 'Collection UI fixture', connection_id: 'platform' });
    const bytes = new TextEncoder().encode('Original observed collection member');
    const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), b => b.toString(16).padStart(2, '0')).join('');
    const op = await write('/uploads', { space_id: space.id, path: 'collection/reference.txt', size_bytes: bytes.length, sha256 }, 'collection-source');
    const part = await fetch('/api/v1/operations/' + op.id + '/parts/0', { method: 'PUT', headers: { 'X-CSRF-Token': me.csrf_token }, body: bytes });
    if (!part.ok) throw new Error('collection_part_failed'); const file = await write('/operations/' + op.id + '/complete', {});
    const collection = await write('/file-collections', { space_id: space.id, name: 'Project references', kind: 'manual', scope: 'private' }, 'collection-ui');
    await write('/file-collections/' + collection.collection_id + '/members', { expected_revision: collection.revision, include: [file.file_id] }, undefined, 'PUT');
    return { space: space.id as string, collection: collection.collection_id as string, file: file.file_id as string, version: file.version_id as string };
  });
  await page.reload();
  await page.getByRole('navigation', { name: '工作区导航' }).getByRole('button', { name: '项目集合', exact: true }).click();
  await expect(page).toHaveURL(/#\/collections$/);
  const collections = page.getByRole('region', { name: '项目集合', exact: true });
  await collections.getByRole('combobox', { name: '集合空间' }).click(); await page.getByRole('option', { name: 'Collection UI fixture', exact: true }).click();
  const trigger = collections.getByRole('button', { name: '查看集合 Project references', exact: true }); await trigger.click();
  const member = collections.getByRole('button', { name: '查看成员固定引用：collection/reference.txt', exact: true }); await expect(member).toBeVisible();
  const latest = await page.evaluate(async original => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const response = await fetch('/api/v1/files/' + original.file + '/document', { method: 'PUT', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token, 'Idempotency-Key': 'collection-head-change' }, body: JSON.stringify({ base_version_id: original.version, text: 'New head must not replace observed member' }) });
    if (!response.ok) throw new Error('collection_head_failed'); return (await response.json()).data.version_id as string;
  }, original);
  const bodies: string[] = [], writes: string[] = [];
  page.on('request', request => { const url = new URL(request.url()); if (url.pathname.endsWith('/content')) bodies.push(url.searchParams.get('version') || ''); if (url.pathname.includes('/api/v1/') && request.method() !== 'GET') writes.push(url.pathname); });
  await member.click(); const fixed = collections.getByRole('region', { name: '集合页固定引用', exact: true });
  await expect(fixed.getByRole('button', { name: '读取原版本正文', exact: true })).toBeVisible(); expect(bodies).toEqual([]);
  await fixed.getByRole('button', { name: '读取原版本正文', exact: true }).click(); await expect(fixed.getByLabel('集合页原版本纯文本', { exact: true })).toHaveText('Original observed collection member');
  expect(latest).not.toBe(original.version); expect(bodies).toEqual([original.version]); expect(writes).toEqual([]);
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 }); await fixed.scrollIntoViewIfNeeded();
    await page.screenshot({ path: path.join(process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!, `collection-fixed-member-${width}.png`), animations: 'disabled' });
  }
  await page.setViewportSize({ width: 1440, height: 900 });
  await fixed.getByRole('button', { name: '关闭固定引用' }).click(); await expect(member).toBeFocused();
  await collections.getByRole('button', { name: '关闭集合成员' }).click(); await expect(trigger).toBeFocused();
  let createCount = 0, committed = '', originalKey = '';
  const createdName = 'UI <集合>& "quoted" \\ line\u2028break\u2029end';
  await page.route('**/api/v1/file-collections', async route => {
    if (route.request().method() !== 'POST') return route.continue();
    createCount++; originalKey = route.request().headers()['idempotency-key'];
    const response = await route.fetch();
    if (response.status() !== 200) throw new Error('collection_creation_failed');
    committed = (await response.json()).data.collection_id; await route.abort('failed');
  });
  const createTrigger = collections.getByRole('button', { name: '新建私有手动集合', exact: true });
  await createTrigger.click(); let dialog = page.getByRole('dialog', { name: '新建私有手动集合', exact: true });
  await dialog.getByRole('textbox', { name: '集合名称' }).fill(createdName);
  await dialog.getByRole('button', { name: '准备创建集合' }).click(); expect(createCount).toBe(0);
  await dialog.getByRole('button', { name: '确认创建集合' }).click(); await expect(dialog.getByRole('button', { name: '查询原集合回执' })).toBeVisible();
  expect(createCount).toBe(1); expect(committed).not.toBe('');
  const stored = await page.evaluate(() => Object.entries(localStorage).filter(([key]) => key.startsWith('drivebridge:collection-create:')).map(([, value]) => JSON.parse(value)));
  expect(stored).toEqual([{ key: originalKey, digest: expect.stringMatching(/^[a-f0-9]{64}$/) }]);
  await page.reload(); await collections.getByRole('combobox', { name: '集合空间' }).click(); await page.getByRole('option', { name: 'Collection UI fixture', exact: true }).click();
  await createTrigger.click(); dialog = page.getByRole('dialog', { name: '新建私有手动集合', exact: true });
  await dialog.getByRole('button', { name: '查询原集合回执' }).click(); await expect(dialog.getByText('已确认原集合创建', { exact: true })).toBeVisible();
  await expect(dialog.getByText('集合标识：' + committed, { exact: true })).toBeVisible(); expect(createCount).toBe(1); expect(bodies).toEqual([original.version]);
  await dialog.getByRole('button', { name: '确认回执并关闭' }).click(); await expect(createTrigger).toBeFocused();
  await collections.getByRole('button', { name: '刷新集合列表' }).click();
  await expect(collections.getByRole('heading', { name: createdName, exact: true })).toHaveCount(1);
  const editTrigger = collections.getByRole('button', { name: '编辑成员 ' + createdName, exact: true });
  let memberWrites = 0, memberKey = '';
  await page.route('**/api/v1/file-collections/' + committed + '/members/changes', async route => {
    memberWrites++; memberKey = route.request().headers()['idempotency-key'];
    const response = await route.fetch(); if (response.status() !== 200) throw new Error('member_change_failed');
    await route.abort('failed');
  });
  await editTrigger.click(); let editor = page.getByRole('dialog', { name: '编辑集合成员', exact: true });
  await editor.getByRole('button', { name: '读取完整成员决策' }).click();
  await editor.getByRole('textbox', { name: '成员路径关键词' }).fill('collection/reference');
  await editor.getByRole('button', { name: '搜索可选文件' }).click();
  await editor.getByRole('button', { name: '添加包含 collection/reference.txt' }).click();
  await editor.getByRole('button', { name: '预览成员修改' }).click(); expect(memberWrites).toBe(0);
  await expect(editor.getByText('collection/reference.txt：无显式决策 → 包含', { exact: true })).toBeVisible();
  // Hash navigation must be rejected before ScopedWorkspace changes identity.
  for (const destination of ['#/files', '#/drive/' + original.space + '?tenant=foreign-fixture&scope=managed']) {
    const warningPromise = page.waitForEvent('dialog');
    const attempt = page.evaluate(hash => { location.hash = hash; }, destination);
    const leaveWarning = await warningPromise;
    expect(leaveWarning.type()).toBe('confirm');
    expect(leaveWarning.message()).toContain('未提交的集合成员草稿');
    await leaveWarning.dismiss(); await attempt;
    await expect(page).toHaveURL(/#\/collections$/);
    await expect(editor.getByText('collection/reference.txt：无显式决策 → 包含', { exact: true })).toBeVisible();
    expect(memberWrites).toBe(0);
  }
  const unloading = page.waitForEvent('dialog');
  // Chromium may leave the reload promise pending after a dismissed native
  // beforeunload dialog. Bound that wait; DOM and zero-write assertions below
  // establish that the original editor survived, not the timeout itself.
  const cancelledReload = page.reload({ timeout: 5000, waitUntil: 'commit' }).then(() => 'loaded', () => 'cancelled');
  const warning = await unloading; expect(warning.type()).toBe('beforeunload'); await warning.dismiss();
  expect(await cancelledReload).toBe('cancelled');
  await expect(editor.getByText('collection/reference.txt：无显式决策 → 包含', { exact: true })).toBeVisible();
  expect(memberWrites).toBe(0);
  expect(await page.evaluate(() => Object.keys(localStorage).filter(key => key.startsWith('drivebridge:collection-members:')))).toEqual([]);
  await editor.getByRole('button', { name: '确认保存成员决策' }).click();
  await expect(editor.getByRole('button', { name: '查询原成员回执' })).toBeVisible(); expect(memberWrites).toBe(1);
  const memberStored = await page.evaluate(() => Object.entries(localStorage).filter(([key]) => key.startsWith('drivebridge:collection-members:')).map(([, value]) => JSON.parse(value)));
  expect(memberStored).toEqual([{ key: memberKey, digest: expect.stringMatching(/^[a-f0-9]{64}$/) }]);
  const unexpectedDialogs: string[] = [];
  const recordUnexpected = async (dialog: import('@playwright/test').Dialog) => { unexpectedDialogs.push(dialog.type()); await dialog.accept(); };
  page.on('dialog', recordUnexpected);
  await page.reload();
  page.off('dialog', recordUnexpected); expect(unexpectedDialogs).toEqual([]); await collections.getByRole('combobox', { name: '集合空间' }).click(); await page.getByRole('option', { name: 'Collection UI fixture', exact: true }).click();
  await editTrigger.click(); editor = page.getByRole('dialog', { name: '编辑集合成员', exact: true });
  await editor.getByRole('button', { name: '查询原成员回执' }).click(); await expect(editor.getByText('已确认原成员修改', { exact: true })).toBeVisible();
  expect(memberWrites).toBe(1); expect(bodies).toEqual([original.version]);
  await editor.getByRole('button', { name: '确认成员回执并关闭' }).click(); await expect(editTrigger).toBeFocused();
  await page.unroute('**/api/v1/file-collections/' + committed + '/members/changes');
  await editTrigger.click(); editor = page.getByRole('dialog', { name: '编辑集合成员', exact: true });
  await editor.getByRole('button', { name: '读取完整成员决策' }).click(); await expect(editor.getByRole('button', { name: '改为排除 collection/reference.txt' })).toBeVisible();
  // A real second writer advances the revision after the edit base was read.
  await page.evaluate(async ({ id, file }) => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const response = await fetch('/api/v1/file-collections/' + id + '/members', { method: 'PUT', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token }, body: JSON.stringify({ expected_revision: 2, include: [], exclude: [file] }) });
    if (!response.ok) throw new Error('concurrent_members_failed');
  }, { id: committed, file: original.file });
  await editor.getByRole('button', { name: '移除决策 collection/reference.txt' }).click(); await editor.getByRole('button', { name: '预览成员修改' }).click(); await editor.getByRole('button', { name: '确认保存成员决策' }).click();
  await expect(editor.getByText(/没有覆盖服务器/)).toBeVisible(); await editor.getByRole('button', { name: '关闭', exact: true }).click();
  const actual = await page.evaluate(async id => (await (await fetch('/api/v1/file-collections/' + id + '/members/decisions')).json()).data, committed);
  expect(actual.revision).toBe(3); expect(actual.items.map((row: { decision: string }) => row.decision)).toEqual(['exclude']);
  await page.evaluate(async original => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    const update = await fetch('/api/v1/file-collections/' + original.collection + '/members', { method: 'PUT', headers, body: JSON.stringify({ expected_revision: 2, include: [], exclude: [original.file] }) });
    if (!update.ok) throw new Error('hidden_exclude_fixture_failed');
    if (!(await fetch('/api/v1/files/' + original.file + '/trash', { method: 'POST', headers })).ok) throw new Error('fixture_trash_failed');
  }, original);
  await collections.getByRole('button', { name: '编辑成员 Project references', exact: true }).click();
  editor = page.getByRole('dialog', { name: '编辑集合成员', exact: true }); await editor.getByRole('button', { name: '读取完整成员决策' }).click();
  await expect(editor.getByText(/没有部分表单可保存/)).toBeVisible(); await expect(editor.getByRole('button', { name: '预览成员修改' })).toHaveCount(0);
  await editor.getByRole('button', { name: '关闭', exact: true }).click(); expect(bodies).toEqual([original.version]);
  const context = await browser.newContext();
  try {
    const bob = await context.newPage(); await bob.goto('/'); await bob.getByRole('link', { name: '使用账号登录' }).click(); await bob.getByRole('link', { name: '以 Bob 登录' }).click(); await bob.goto('/#/collections');
    await expect(bob.getByRole('heading', { name: '项目集合', exact: true })).toBeVisible(); await expect(bob.getByRole('button', { name: '查看集合 Project references', exact: true })).toHaveCount(0);
    const response = await bob.request.get('/api/v1/file-collections?space_id=' + original.space); expect(response.status()).toBe(404);
    const hidden = await bob.evaluate(async key => {
      const me = (await (await fetch('/api/v1/me')).json()).data;
      return (await fetch('/api/v1/file-collections/lookup', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token }, body: JSON.stringify({ idempotency_key: key }) })).status;
    }, originalKey); expect(hidden).toBe(404);
    expect((await bob.request.get('/api/v1/file-collections/' + committed + '/members/decisions')).status()).toBe(404);
    const memberHidden = await bob.evaluate(async ({ id, key }) => {
      const me = (await (await fetch('/api/v1/me')).json()).data;
      return (await fetch('/api/v1/file-collections/' + id + '/members/changes/lookup', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token }, body: JSON.stringify({ idempotency_key: key }) })).status;
    }, { id: committed, key: memberKey }); expect(memberHidden).toBe(404);
  } finally { await context.close(); }
});
