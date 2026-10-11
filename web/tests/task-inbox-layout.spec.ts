import { expect, test } from '@playwright/test';
import path from 'node:path';

test.use({ trace: 'off', video: 'off', screenshot: 'off' });
test('real task and submission lists preserve fixed identity with responsive review controls', async ({ page, browser }) => {
  test.setTimeout(150_000);
  const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!;
  expect(artifacts).toBeTruthy();
  await page.goto('/');
  await page.getByRole('link', { name: '使用账号登录' }).click();
  await page.getByRole('link', { name: '以 Alice 登录' }).click();
  const fixture = await page.evaluate(async () => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const human = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    async function write(endpoint: string, data: unknown, key: string) {
      const response = await fetch('/api/v1' + endpoint, { method: 'POST', headers: { ...human, 'Idempotency-Key': key }, body: JSON.stringify(data) });
      if (!response.ok) throw new Error('task_inbox_fixture_write_failed');
      return (await response.json()).data;
    }
    const space = await write('/spaces', { name: 'Task inbox layout fixture', connection_id: 'platform' }, 'task-inbox-space');
    // Secret is used only inside this lexical frame, never returned to the runner or rendered.
    const issued = await write('/agent-grants', { name: 'Task inbox layout Agent', protocol: 'http', space_id: space.id, connection_id: 'platform', prefix: 'references', actions: ['inbox.submit'], expiry_mode: 'never' }, 'task-inbox-grant');
    const auth = { 'Content-Type': 'application/json', Authorization: 'Bearer ' + issued.credential.secret };
    const body = new TextEncoder().encode('# Review original text\n\n<script>window.fixtureInjected = true</script>\n\n' + 'Unbroken-reference-'.repeat(25));
    const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', body)), byte => byte.toString(16).padStart(2, '0')).join('');
    const longTask = await write('/uploads', { space_id: space.id, path: 'long-human-task-'.repeat(12) + '.md', size_bytes: body.length, sha256 }, 'long-task-upload');
    const longPart = await fetch('/api/v1/operations/' + longTask.id + '/parts/0', { method: 'PUT', headers: { 'X-CSRF-Token': me.csrf_token }, body });
    if (!longPart.ok) throw new Error('long_task_fixture_part_failed');
    await write('/operations/' + longTask.id + '/complete', {}, 'long-task-complete');
    const records: { id: string; op: string; file: string; version: string; state: string; revision: number }[] = [];
    for (let i = 0; i < 3; i++) {
      const response = await fetch('/api/v1/inbox-submissions', { method: 'POST', credentials: 'omit', headers: { ...auth, 'Idempotency-Key': 'task-inbox-sub-' + i }, body: JSON.stringify({ name: `review-${i}-${'long-name-'.repeat(10)}.md`, mime_type: 'text/markdown', size_bytes: body.length, sha256 }) });
      if (!response.ok) throw new Error('task_inbox_fixture_submission_failed');
      const upload = (await response.json()).data;
      const part = await fetch('/api/v1/operations/' + upload.operation_id + '/parts/0', { method: 'PUT', credentials: 'omit', headers: { Authorization: auth.Authorization }, body });
      if (!part.ok) throw new Error('task_inbox_fixture_part_failed');
      const done = await fetch('/api/v1/operations/' + upload.operation_id + '/complete', { method: 'POST', credentials: 'omit', headers: auth, body: '{}' });
      if (!done.ok) throw new Error('task_inbox_fixture_complete_failed');
      const read = await fetch('/api/v1/inbox-submissions/' + upload.submission_id);
      if (!read.ok) throw new Error('task_inbox_fixture_read_failed');
      let sub = (await read.json()).data;
      if (i > 0) sub = await write('/inbox-submissions/' + sub.submission_id + '/decision', { expected_revision: sub.revision, version_id: sub.version_id, decision: i === 1 ? 'accept' : 'reject' }, 'task-inbox-decision-' + i);
      records.push({ id: sub.submission_id, op: upload.operation_id, file: sub.file_ref, version: sub.version_id, state: sub.state, revision: sub.revision });
    }
    return { space: space.id as string, records };
  });
  await page.reload();
  const writes: string[] = [];
  page.on('request', request => { if (request.url().includes('/api/v1/') && !['GET', 'HEAD'].includes(request.method())) writes.push(request.method()); });
  for (const route of ['tasks', 'inbox']) {
    await page.goto('/#/' + route);
    const region = page.getByRole('region', { name: route === 'tasks' ? '持久操作任务' : 'Agent 提交收件', exact: true });
    const list = region.getByRole('list', { name: route === 'tasks' ? '持久操作列表' : '提交收件列表', exact: true });
    await expect(list).toContainText(route === 'tasks' ? fixture.records[0].op : fixture.records[0].id);
    for (const theme of ['light', 'dark'] as const) {
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      for (const width of [320, 390, 768, 1280, 1440, 1920]) {
        await page.setViewportSize({ width, height: 900 });
        await region.scrollIntoViewIfNeeded();
        await page.screenshot({ path: path.join(artifacts, `${route}-${theme}-${width}.png`), animations: 'disabled' });
        expect(await region.evaluate(element => element.scrollWidth <= element.clientWidth), `${route}/${theme}/${width}`).toBe(true);
        for (const row of await list.getByRole('listitem').all()) {
          expect(await row.evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true);
          expect((await row.boundingBox())!.height, `${route} row height at ${width}`).toBeLessThanOrEqual(route === 'tasks' ? (width < 768 ? 340 : 230) : (width < 768 ? 450 : 290));
          if (width < 768) for (const button of await row.getByRole('button').all()) expect((await button.boundingBox())!.height).toBeGreaterThanOrEqual(44);
        }
        const trigger = region.getByRole('button', { name: (route === 'tasks' ? '查看原操作详情 ' + fixture.records[0].op : '查看原提交文件 ' + fixture.records[0].id), exact: true });
        await trigger.focus(); await page.keyboard.press('Enter');
        const detail = route === 'tasks' ? page.getByRole('region', { name: '原操作详情', exact: true }) : page.getByRole('dialog', { name: '收件固定版本预览', exact: true });
        await expect(detail).toContainText(fixture.records[0].version);
        if (route === 'tasks') {
          const sheet = page.getByRole('dialog', { name: '操作详情', exact: true });
          await expect.poll(async () => Math.round((await sheet.boundingBox())!.width)).toBe(width < 768 ? width : 440);
          if (width >= 768) {
            const separator = sheet.getByRole('separator', { name: '调整详情宽度' });
            await separator.focus(); await separator.press('ArrowLeft');
            await expect(separator).toHaveAttribute('aria-valuenow', '456');
            await separator.press('Home'); await expect(separator).toHaveAttribute('aria-valuenow', '320');
            await separator.press('Enter'); await expect(separator).toHaveAttribute('aria-valuenow', '440');
            await expect(detail).toContainText(fixture.records[0].version);
          }
        }
        if (route === 'inbox') {
          await expect(detail.getByLabel('收件原版本纯文本')).toContainText('Review original text');
          await expect(detail.locator('pre script')).toHaveCount(0);
        }
        expect(await detail.evaluate(element => element.scrollWidth <= element.clientWidth), `${route} detail ${width}`).toBe(true);
        await page.screenshot({ path: path.join(artifacts, `${route}-detail-${theme}-${width}.png`), animations: 'disabled' });
        await detail.getByRole('button', { name: route === 'tasks' ? '关闭操作详情' : '关闭', exact: true }).click();
        await expect(detail).toHaveCount(0); await expect(trigger).toBeFocused();
      }
    }
    if (route === 'inbox') {
      await page.setViewportSize({ width: 320, height: 900 });
      for (const [name, title] of [['人工验收或恢复 ' + fixture.records[0].id, '人工验收提交收件'], ['归档或恢复 ' + fixture.records[1].id, '归档已接收收件']]) {
        const trigger = region.getByRole('button', { name, exact: true });
        await trigger.focus(); await page.keyboard.press('Enter');
        const dialog = page.getByRole('dialog', { name: title, exact: true });
        await expect(dialog.getByRole('button', { name: title === '人工验收提交收件' ? '确认接收原版本' : '确认归档原版本', exact: true })).toBeEnabled();
        for (const width of [1440, 390, 320]) {
          await page.setViewportSize({ width, height: 900 });
          expect(await dialog.evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true);
          await page.screenshot({ path: path.join(artifacts, `inbox-${title}-${width}.png`), animations: 'disabled' });
        }
        await page.keyboard.press('Escape'); await expect(dialog).toHaveCount(0); await expect(trigger).toBeFocused();
      }
    }
  }
  const touchContext = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true, reducedMotion: 'no-preference', colorScheme: 'dark' });
  try {
    const touch = await touchContext.newPage();
    await touch.goto('/'); await touch.getByRole('link', { name: '使用账号登录' }).tap(); await touch.getByRole('link', { name: '以 Alice 登录' }).tap();
    await touch.getByRole('heading', { name: '我的网盘', exact: true }).waitFor();
    await touch.goto('/#/tasks');
    const trigger = touch.getByRole('button', { name: '查看原操作详情 ' + fixture.records[0].op, exact: true });
    await trigger.tap();
    const sheet = touch.getByRole('dialog', { name: '操作详情', exact: true });
    await expect(sheet).toContainText(fixture.records[0].version);
    await expect.poll(async () => Math.round((await sheet.boundingBox())!.width)).toBe(390);
    await touch.screenshot({ path: path.join(artifacts, 'tasks-touch-detail.png'), animations: 'disabled' });
    await sheet.getByRole('button', { name: '关闭', exact: true }).tap(); await expect(sheet).toHaveCount(0); await expect(trigger).toBeFocused();
    await touch.goto('/#/inbox');
    const preview = touch.getByRole('button', { name: '查看原提交文件 ' + fixture.records[0].id, exact: true });
    await preview.tap(); const dialog = touch.getByRole('dialog', { name: '收件固定版本预览', exact: true });
    await expect(dialog.getByLabel('收件原版本纯文本')).toContainText('Review original text');
    await dialog.getByRole('button', { name: '关闭', exact: true }).tap(); await expect(dialog).toHaveCount(0); await expect(preview).toBeFocused();
  } finally { await touchContext.close(); }
  expect(writes).toEqual([]);
  const readback = await page.evaluate(async records => Promise.all(records.map(async record => {
    const sub = (await (await fetch('/api/v1/inbox-submissions/' + record.id)).json()).data;
    const content = await fetch('/api/v1/files/' + record.file + '/content?version=' + record.version);
    if (!content.ok) throw new Error('task_inbox_readback_failed');
    return { id: sub.submission_id, state: sub.state, revision: sub.revision, version: sub.version_id, sameBody: (await content.text()).includes('Review original text') };
  })), fixture.records);
  expect(readback).toEqual(fixture.records.map(record => ({ id: record.id, state: record.state, revision: record.revision, version: record.version, sameBody: true })));
});
