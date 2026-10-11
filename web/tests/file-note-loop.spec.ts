import { expect, test, type Page } from '@playwright/test';
import path from 'node:path';

test.use({ trace: 'off', screenshot: 'off', video: 'off' });
async function openNote(page: Page, spaceName: string) {
  await page.goto('/#/knowledge');
  await page.getByRole('combobox', { name: '知识库文件来源' }).click();
  await page.getByRole('option', { name: spaceName, exact: true }).click();
  const notebook = page.getByLabel('笔记工作台', { exact: true });
  await notebook.getByRole('button', { name: '快速打开', exact: true }).click();
  await page.getByRole('dialog', { name: '快速打开', exact: true }).getByRole('button', { name: 'local-loop.md', exact: true }).click();
  return notebook;
}

test('real local file and note loop preserves fixed versions and recovers a lost save by its original key', async ({ page, browser }) => {
  test.setTimeout(120_000);
  const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!;
  expect(artifacts).toBeTruthy();
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.name));
  await page.goto('/');
  await page.getByRole('link', { name: '使用账号登录' }).click();
  await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  const spaceName = 'File note complete local loop';
  const space = await page.evaluate(async name => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const response = await fetch('/api/v1/spaces', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token }, body: JSON.stringify({ name, connection_id: 'platform' }) });
    if (!response.ok) throw new Error('loop_space_failed');
    return (await response.json()).data.id as string;
  }, spaceName);
  await page.reload();
  await page.getByRole('button', { name: '打开网盘 ' + spaceName, exact: true }).click();
  const chooser = page.waitForEvent('filechooser');
  await page.getByRole('button', { name: '上传文件', exact: true }).click();
  await (await chooser).setFiles({ name: 'local-loop.md', mimeType: 'text/markdown', buffer: Buffer.from('# Original local note\n\nOriginal immutable bytes.') });
  await expect(page.getByRole('button', { name: '详情 local-loop.md', exact: true })).toBeVisible();
  const listing = await page.request.get('/api/v1/files?space_id=' + encodeURIComponent(space));
  expect(listing.status()).toBe(200);
  const files = (await listing.json()).data as { id: string; path: string; version_id: string }[];
  const original = files.find(file => file.path === 'local-loop.md')!;
  expect(original?.version_id).toBeTruthy();
  await page.reload();
  if (await page.getByRole('button', { name: '打开网盘 ' + spaceName, exact: true }).isVisible()) await page.getByRole('button', { name: '打开网盘 ' + spaceName, exact: true }).click();
  const fixedRead = page.waitForRequest(request => new URL(request.url()).pathname === '/api/v1/files/' + original.id + '/media', { timeout: 10_000 });
  await page.getByRole('button', { name: '详情 local-loop.md', exact: true }).click();
  expect(new URL((await fixedRead).url()).searchParams.get('version')).toBe(original.version_id);
  const preview = page.getByRole('dialog', { name: 'local-loop.md', exact: true });
  await expect(preview).toContainText('Original local note');
  await preview.getByRole('button', { name: '关闭', exact: true }).click();

  let notebook = await openNote(page, spaceName);
  await expect(notebook.getByRole('heading', { name: 'Original local note', exact: true })).toBeVisible();
  await notebook.getByRole('button', { name: '源码', exact: true }).click();
  const updated = '# First confirmed version\n\nHuman-reviewed replacement.';
  await notebook.getByRole('textbox', { name: 'Markdown 源码' }).fill(updated);
  await notebook.getByRole('button', { name: '预览保存', exact: true }).click();
  const diff = page.getByRole('dialog', { name: '确认文档修改', exact: true });
  await expect(diff.getByText('保存前', { exact: true })).toBeVisible();
  await expect(diff).toContainText('Original immutable bytes.');
  await expect(diff).toContainText('Human-reviewed replacement.');
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await page.screenshot({ path: path.join(artifacts, `note-save-diff-${width}.png`), animations: 'disabled' });
  }
  const firstSave = page.waitForResponse(response => response.request().method() === 'PUT' && new URL(response.url()).pathname === '/api/v1/files/' + original.id + '/document');
  await diff.getByRole('button', { name: '确认保存新版本', exact: true }).click();
  const firstResponse = await firstSave;
  expect(firstResponse.status()).toBe(200);
  const saved = (await firstResponse.json()).data as { version_id: string; operation_id: string };
  expect(saved.version_id).not.toBe(original.version_id);
  await expect(notebook.getByRole('heading', { name: 'First confirmed version', exact: true })).toBeVisible();
  await page.reload();
  notebook = await openNote(page, spaceName);
  await expect(notebook.getByRole('heading', { name: 'First confirmed version', exact: true })).toBeVisible();

  // The owner commits the real request, then only its response is dropped.
  // The browser must keep the original non-secret key across reload, not text.
  const lostText = '# Recovered exact original save\n\nCommitted despite missing response.';
  let writes = 0, originalKey = '';
  let committed: { version_id: string; operation_id: string } | undefined;
  const documentPath = '/api/v1/files/' + original.id + '/document';
  await page.route('**' + documentPath, async route => {
    if (route.request().method() !== 'PUT') return route.continue();
    writes++; originalKey = route.request().headers()['idempotency-key'];
    expect(route.request().postDataJSON()).toEqual({ base_version_id: saved.version_id, text: lostText });
    const response = await route.fetch();
    if (response.status() !== 200) throw new Error('loop_lost_save_not_committed');
    committed = (await response.json()).data;
    await route.abort('failed');
  });
  await notebook.getByRole('button', { name: '源码', exact: true }).click();
  await notebook.getByRole('textbox', { name: 'Markdown 源码' }).fill(lostText);
  await notebook.getByRole('button', { name: '预览保存', exact: true }).click();
  await page.getByRole('button', { name: '确认保存新版本', exact: true }).click();
  await expect(notebook.getByText(/保存结果尚未确认/)).toBeVisible();
  await expect(notebook.getByRole('textbox', { name: 'Markdown 源码' }).locator('.cm-line')).toHaveText(lostText.split('\n'));
  expect(writes).toBe(1); expect(committed?.operation_id).toBeTruthy();
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await page.screenshot({ path: path.join(artifacts, `note-save-unknown-${width}.png`), animations: 'disabled' });
  }
  const recovery = await page.evaluate(({ id, text }) => ({
    values: Object.entries(localStorage).filter(([key]) => key.startsWith('drivebridge:document-save:') && key.includes(id)).map(([, value]) => value),
    persistedBody: [...Object.values(localStorage), ...Object.values(sessionStorage)].some(value => value.includes(text)),
  }), { id: original.id, text: lostText });
  expect(recovery).toEqual({ values: [originalKey], persistedBody: false });
  await page.unroute('**' + documentPath);
  let laterWrites = 0;
  page.on('request', request => { if (request.method() === 'PUT' && new URL(request.url()).pathname === documentPath) laterWrites++; });
  page.once('dialog', dialog => dialog.accept());
  await page.reload();
  notebook = await openNote(page, spaceName);
  await expect(notebook.getByRole('button', { name: '查询原保存结果', exact: true })).toBeVisible();
  const lookup = page.waitForResponse(response => new URL(response.url()).pathname === documentPath + '/lookup');
  await notebook.getByRole('button', { name: '查询原保存结果', exact: true }).click();
  const lookupResponse = await lookup;
  expect(lookupResponse.request().postDataJSON()).toEqual({ idempotency_key: originalKey });
  expect(lookupResponse.status()).toBe(200);
  expect((await lookupResponse.json()).data.save).toMatchObject(committed!);
  await expect(notebook.getByRole('heading', { name: 'Recovered exact original save', exact: true })).toBeVisible();
  await expect(notebook.getByRole('button', { name: '查询原保存结果', exact: true })).toHaveCount(0);
  expect(laterWrites).toBe(0);
  const readback = await page.evaluate(async ({ original, saved, committed }) => {
    const versions = await fetch('/api/v1/files/' + original.id + '/versions');
    const rows = (await versions.json()).data;
    const contents = await Promise.all([original.version_id, saved.version_id, committed.version_id].map(async version => {
      const response = await fetch('/api/v1/files/' + original.id + '/content?version=' + encodeURIComponent(version));
      return { version, status: response.status, text: await response.text() };
    }));
    const operation = await fetch('/api/v1/operations/' + committed.operation_id);
    const op = (await operation.json()).data.operation;
    return { versionCount: rows.length, contents, operationStatus: operation.status, op: { id: op.id, status: op.status, file: op.file_id, version: op.version_id }, recoveryCount: Object.keys(localStorage).filter(key => key.startsWith('drivebridge:document-save:') && key.includes(original.id)).length };
  }, { original, saved, committed: committed! });
  expect(readback.versionCount).toBe(3);
  expect(readback.contents).toEqual([
    { version: original.version_id, status: 200, text: '# Original local note\n\nOriginal immutable bytes.' },
    { version: saved.version_id, status: 200, text: updated },
    { version: committed!.version_id, status: 200, text: lostText },
  ]);
  expect(readback.operationStatus).toBe(200);
  expect(readback.op).toEqual({ id: committed!.operation_id, status: 'completed', file: original.id, version: committed!.version_id });
  expect(readback.recoveryCount).toBe(0);
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await page.screenshot({ path: path.join(artifacts, `file-note-recovered-${width}.png`), animations: 'disabled' });
  }
  const outsider = await browser.newContext();
  try {
    const bob = await outsider.newPage(); await bob.goto('/');
    await bob.getByRole('link', { name: '使用账号登录' }).click(); await bob.getByRole('link', { name: '以 Bob 登录' }).click();
    await expect(bob.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
    const denied = await bob.evaluate(async ({ id, key, version }) => {
      const me = (await (await fetch('/api/v1/me')).json()).data;
      const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token, 'Idempotency-Key': key };
      const statuses = [];
      for (const [suffix, method, body] of [['/content?version=' + version, 'GET', undefined], ['/document/lookup', 'POST', { idempotency_key: key }], ['/document', 'PUT', { base_version_id: version, text: 'foreign overwrite refused' }]] as const) {
        const response = await fetch('/api/v1/files/' + id + suffix, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
        statuses.push(response.status); await response.body?.cancel();
      }
      return statuses;
    }, { id: original.id, key: originalKey, version: committed!.version_id });
    expect(denied).toEqual([404, 404, 404]);
  } finally { await outsider.close(); }
  expect(errors).toEqual([]);
});
