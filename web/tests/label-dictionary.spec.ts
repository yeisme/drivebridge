import { expect, test } from '@playwright/test';
import { selectChoice } from './helpers/select';
test.use({ trace: 'off', video: 'off', screenshot: 'off' });
test('manual label dictionary recovery and selected-file merge preserve unselected labels and dictionary identities', async ({ page }) => {
  const enabled = process.env.DRIVEBRIDGE_TEST_LABEL_MANAGEMENT === '1';
  await page.goto('/'); await page.getByRole('link', { name: '使用账号登录' }).click(); await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  const fixture = await page.evaluate(async enabled => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    async function write(path: string, method: string, body: unknown, label: string) {
      const response = await fetch('/api/v1' + path, { method, headers: { ...headers, 'Idempotency-Key': label }, body: JSON.stringify(body) });
      if (!response.ok) throw new Error('label_fixture_' + label + '_' + response.status); return (await response.json()).data;
    }
    const space = await write('/spaces', 'POST', { name: 'Label UI fixture', connection_id: 'platform' }, 'label-space');
    if (!enabled) return { space: space.id as string, selected: '', untouched: '', ref: '' };
    await write('/spaces/' + space.id + '/label-definitions', 'PUT', { expected_revision: 0, definitions: [{ key: 'source', display_name: '来源', group: '' }, { key: 'target', display_name: '目标', group: '' }] }, 'label-definitions');
    const files: { id: string; ref: string }[] = [];
    for (const name of ['selected.txt', 'untouched.txt']) {
      const bytes = new TextEncoder().encode(name), sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), byte => byte.toString(16).padStart(2, '0')).join('');
      const op = await write('/uploads', 'POST', { space_id: space.id, path: name, size_bytes: bytes.length, sha256 }, 'label-upload-' + name);
      const part = await fetch('/api/v1/operations/' + op.id + '/parts/0', { method: 'PUT', headers: { 'X-CSRF-Token': me.csrf_token }, body: bytes }); if (!part.ok) throw new Error('label_fixture_part');
      const done = await write('/operations/' + op.id + '/complete', 'POST', {}, 'label-complete-' + name);
      await write('/files/' + done.file_id + '/labels', 'PUT', { namespace: 'manual', labels: ['source', 'legacy label'] }, 'label-manual-' + name);
      await write('/files/' + done.file_id + '/labels', 'PUT', { namespace: 'suggested', labels: ['source'] }, 'label-suggested-' + name);
      const stat = await fetch('/api/v1/files/' + done.file_id); if (!stat.ok) throw new Error('label_fixture_stat');
      files.push({ id: done.file_id, ref: (await stat.json()).data.ref });
    }
    return { space: space.id as string, selected: files[0].id, untouched: files[1].id, ref: files[0].ref };
  }, enabled);
  await page.reload(); await page.getByRole('navigation', { name: '工作区导航' }).getByRole('button', { name: '标签与属性', exact: true }).click();
  await selectChoice(page.getByRole('combobox', { name: '属性空间' }), fixture.space); await page.getByRole('button', { name: '管理标签字典', exact: true }).click();
  let dialog = page.getByRole('dialog', { name: '标签字典与所选文件合并', exact: true });
  if (!enabled) { await expect(dialog.getByText(/未启用标签管理/)).toBeVisible(); await expect(dialog.getByRole('button', { name: '编辑标签定义' })).toHaveCount(0); return; }
  let dictionaryWrites = 0;
  await page.route('**/api/v1/spaces/' + fixture.space + '/label-definitions', async route => {
    if (route.request().method() !== 'PUT') return route.continue();
    dictionaryWrites++; const response = await route.fetch(); expect(response.status()).toBe(200); await route.abort('failed');
  });
  await dialog.getByRole('button', { name: '编辑标签定义' }).click(); await expect(dialog.getByRole('textbox', { name: '标签 1 稳定 key' })).toBeDisabled();
  await dialog.getByRole('textbox', { name: '标签 1 显示名' }).fill('来源显示更新'); await dialog.getByRole('button', { name: '预览标签字典修改' }).click(); expect(dictionaryWrites).toBe(0);
  await dialog.getByRole('button', { name: '确认保存标签字典' }).click(); await expect(dialog.getByRole('button', { name: '查询原标签请求' })).toBeEnabled(); expect(dictionaryWrites).toBe(1);
  await page.reload(); await selectChoice(page.getByRole('combobox', { name: '属性空间' }), fixture.space); await page.getByRole('button', { name: '管理标签字典', exact: true }).click();
  dialog = page.getByRole('dialog', { name: '标签字典与所选文件合并', exact: true });
  await dialog.getByRole('button', { name: '查询原标签请求' }).click(); await expect(dialog.getByText(/标签字典修订 2/)).toBeVisible(); expect(dictionaryWrites).toBe(1);
  await dialog.getByRole('textbox', { name: '所选文件固定引用' }).fill(fixture.ref); await dialog.getByRole('button', { name: '生成所选文件合并预览' }).click();
  await expect(dialog.getByText('selected.txt', { exact: true })).toBeVisible(); await expect(dialog.getByText('untouched.txt', { exact: true })).toHaveCount(0);
  await dialog.getByRole('button', { name: '确认所选文件合并计划' }).click();
  let executes = 0;
  await page.route('**/api/v1/organization-plans/*/execute', async route => { executes++; const response = await route.fetch(); expect(response.status()).toBe(200); await route.abort('failed'); });
  await dialog.getByRole('button', { name: '执行所选文件合并' }).click(); expect(executes).toBe(0); await dialog.getByRole('button', { name: '确认执行标签合并' }).click();
  await expect(dialog.getByRole('button', { name: '重读原合并计划' })).toBeEnabled(); expect(executes).toBe(1);
  await page.reload(); await selectChoice(page.getByRole('combobox', { name: '属性空间' }), fixture.space); await page.getByRole('button', { name: '管理标签字典', exact: true }).click();
  dialog = page.getByRole('dialog', { name: '标签字典与所选文件合并', exact: true }); await dialog.getByRole('button', { name: '重读原合并计划' }).click();
  await expect(dialog.getByText('合并完成', { exact: true })).toBeVisible(); expect(executes).toBe(1);
  const selected = (await (await page.request.get('/api/v1/files/' + fixture.selected + '/labels')).json()).data;
  const untouched = (await (await page.request.get('/api/v1/files/' + fixture.untouched + '/labels')).json()).data;
  expect(selected.namespaces.manual.sort()).toEqual(['legacy label', 'target']); expect(untouched.namespaces.manual.sort()).toEqual(['legacy label', 'source']); expect(selected.namespaces.suggested).toEqual(['source']);
  const dictionary = (await (await page.request.get('/api/v1/spaces/' + fixture.space + '/label-definitions?limit=200')).json()).data;
  expect(dictionary.revision).toBe(2); expect(dictionary.definitions.map((row: { key: string }) => row.key)).toEqual(['source', 'target']);
  await expect(dialog.getByText(/不删除字典身份/)).toBeVisible();
});
