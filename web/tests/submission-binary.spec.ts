import { expect, test, type Page } from '@playwright/test';
import fs from 'node:fs/promises';
import path from 'node:path';
import { createHash } from 'node:crypto';

test.use({ trace: 'off', video: 'off', screenshot: 'off' });
async function login(page: Page, user = 'Alice') {
  await page.goto('/'); await page.getByRole('link', { name: '使用账号登录' }).click(); await page.getByRole('link', { name: `以 ${user} 登录` }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
}
test('real image, PDF and binary submissions retain original bytes through preview, download, decision, archive and revocation', async ({ page, browser }) => {
  test.setTimeout(180_000);
  const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!; expect(artifacts).toBeTruthy();
  const pdf = (await fs.readFile('tests/fixtures/reader-cjk-scan.pdf')).toString('base64');
  await login(page);
  // Credentials exist only in this browser-side closure, never in runner data,
  // page globals, storage, screenshot DOM or automatic capture.
  const client = await page.evaluateHandle(async pdf => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const human = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    async function write(endpoint: string, body: unknown, key: string) {
      const response = await fetch('/api/v1' + endpoint, { method: 'POST', headers: { ...human, 'Idempotency-Key': key }, body: JSON.stringify(body) });
      if (!response.ok) throw new Error('binary_fixture_write_' + response.status);
      return (await response.json()).data;
    }
    const space = await write('/spaces', { name: '多格式成果验收', connection_id: 'platform' }, 'binary-space');
    const issued = await write('/agent-grants', { name: '视觉研究助理', protocol: 'http', space_id: space.id, connection_id: 'platform', prefix: 'references', actions: ['inbox.submit'], expiry_mode: 'never' }, 'binary-grant');
    const secret = issued.credential.secret;
    const auth = { 'Content-Type': 'application/json', Authorization: 'Bearer ' + secret };
    async function digest(body: Uint8Array) { return Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', body as Uint8Array<ArrayBuffer>)), byte => byte.toString(16).padStart(2, '0')).join(''); }
    const canvas = document.createElement('canvas'); canvas.width = 320; canvas.height = 180;
    const paint = canvas.getContext('2d')!; paint.fillStyle = '#315b49'; paint.fillRect(0, 0, 320, 180); paint.fillStyle = '#f5e7bf'; paint.font = '24px sans-serif'; paint.fillText('Original artifact', 40, 90);
    const png = Uint8Array.from(atob(canvas.toDataURL('image/png').split(',')[1]), value => value.charCodeAt(0));
    const samples = [
      { name: '视觉研究成果.png', mime: 'image/png', bytes: png },
      { name: '扫描研究报告.pdf', mime: 'application/pdf', bytes: Uint8Array.from(atob(pdf), value => value.charCodeAt(0)) },
      { name: '原始资料包.bin', mime: 'application/octet-stream', bytes: new Uint8Array([0, 255, 17, 0, 129, 23, 255, 98]) },
      { name: '版本冲突反例.png', mime: 'image/png', bytes: png },
    ];
    const records: { id: string; file: string; version: string; sha256: string; name: string; size: number; mime: string }[] = [];
    for (const sample of samples) {
      const sha256 = await digest(sample.bytes);
      const start = await fetch('/api/v1/inbox-submissions', { method: 'POST', credentials: 'omit', headers: { ...auth, 'Idempotency-Key': 'binary-' + records.length + '-' + sha256 }, body: JSON.stringify({ name: sample.name, mime_type: sample.mime, size_bytes: sample.bytes.length, sha256 }) });
      if (!start.ok) throw new Error('binary_submission_failed');
      const upload = (await start.json()).data;
      const part = await fetch('/api/v1/operations/' + upload.operation_id + '/parts/0', { method: 'PUT', credentials: 'omit', headers: { Authorization: auth.Authorization }, body: sample.bytes });
      if (!part.ok) throw new Error('binary_part_failed');
      const completed = await fetch('/api/v1/operations/' + upload.operation_id + '/complete', { method: 'POST', credentials: 'omit', headers: auth, body: '{}' });
      if (!completed.ok) throw new Error('binary_complete_failed');
      const response = await fetch('/api/v1/inbox-submissions/' + upload.submission_id); if (!response.ok) throw new Error('binary_read_failed');
      const sub = (await response.json()).data;
      records.push({ id: sub.submission_id as string, file: sub.file_ref as string, version: sub.version_id as string, sha256, name: sample.name, size: sample.bytes.length, mime: sample.mime });
    }
    // Replace the image head with different bytes and MIME through the ordinary
    // human CAS upload contract. The received version must remain a real image.
    const first = records[3];
    const current = (await (await fetch('/api/v1/files/' + first.file)).json()).data.file;
    const changed = new TextEncoder().encode('New head is not the submitted image.');
    const version = await write('/uploads', { space_id: space.id, file_id: first.file, path: current.path, expected_version: first.version, size_bytes: changed.length, sha256: await digest(changed) }, 'binary-new-head');
    const part = await fetch('/api/v1/operations/' + version.id + '/parts/0', { method: 'PUT', headers: { 'X-CSRF-Token': me.csrf_token }, body: changed });
    if (!part.ok) throw new Error('binary_head_part_failed');
    const completed = await write('/operations/' + version.id + '/complete', {}, 'binary-head-complete');
    const destination = (await (await fetch('/api/v1/spaces')).json()).data.find((item: { id: string }) => item.id !== space.id);
    if (!destination) throw new Error('binary_destination_missing');
    return {
      evidence: { records, space: space.id as string, head: completed.version_id as string, destinationName: destination.name as string },
      async revoke() {
        await write('/agent-grants/' + issued.grant.grant_id + '/revoke', { expected_revision: 1 }, 'binary-revoke');
        return (await fetch('/api/v1/inbox-submissions/' + first.id, { credentials: 'omit', headers: auth })).status;
      },
    };
  }, pdf);
  try {
    const fixture = await client.evaluate(value => value.evidence);
    expect(fixture.head).not.toBe(fixture.records[3].version);
    await page.goto('/#/inbox?submission_space=' + fixture.space);
    const row = (id: string) => page.locator(`[data-submission-id="${id}"]`);
    const dialog = page.getByRole('dialog', { name: '收件固定版本预览', exact: true });
    const writes: string[] = [];
    page.on('request', request => { if (request.url().includes('/api/v1/') && !['GET', 'HEAD'].includes(request.method())) writes.push(new URL(request.url()).pathname); });
    for (const record of fixture.records) {
      await expect(row(record.id).getByRole('heading', { name: record.name, exact: true })).toBeVisible();
      await expect(row(record.id).getByText('来自 视觉研究助理', { exact: true })).toBeVisible();
      await row(record.id).getByRole('button', { name: '查看原提交文件 ' + record.id, exact: true }).click();
      if (record.mime === 'image/png') {
        const image = dialog.getByRole('img', { name: record.name, exact: true });
        await expect.poll(() => image.evaluate((element: HTMLImageElement) => element.complete && element.naturalWidth)).toBe(320);
        expect(new URL((await image.getAttribute('src'))!, page.url()).searchParams.get('version')).toBe(record.version);
      } else if (record.mime === 'application/pdf') {
        await expect(dialog.locator('.textLayer')).toContainText('中文测试');
        await expect(dialog.locator('.reader-status')).toContainText('第 1 / 2 页');
        await dialog.getByRole('button', { name: '适应宽度', exact: true }).click();
        await expect.poll(() => dialog.locator('canvas').evaluate((canvas: HTMLCanvasElement) => Array.from(canvas.getContext('2d')!.getImageData(0, 0, canvas.width, canvas.height).data).filter((value, index) => index % 4 === 0 && value < 100).length)).toBeGreaterThan(50);
        await page.screenshot({ path: path.join(artifacts, 'binary-pdf-first-page.png') });
      } else await expect(dialog.getByText(/此格式无法在浏览器内直接预览/)).toBeVisible();
      const downloaded = page.waitForEvent('download'); await dialog.getByRole('button', { name: '下载原提交版本' }).click();
      const download = await downloaded; expect(await download.failure()).toBeNull();
      expect(createHash('sha256').update(await fs.readFile((await download.path())!)).digest('hex')).toBe(record.sha256);
      await dialog.getByRole('button', { name: '关闭', exact: true }).click();
    }
    expect(writes).toEqual([]);
    const imageRecord = fixture.records[0];
    for (const theme of ['light', 'dark'] as const) {
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      for (const width of [320, 390, 768, 1280, 1440, 1920]) {
        await page.setViewportSize({ width, height: 900 });
        await expect(row(imageRecord.id).getByRole('heading', { name: imageRecord.name })).toBeVisible();
        await page.getByRole('region', { name: 'Agent 提交收件', exact: true }).getByRole('heading', { name: '产物与收件', exact: true }).scrollIntoViewIfNeeded();
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
        await page.screenshot({ path: path.join(artifacts, `binary-inbox-${theme}-${width}.png`), animations: 'disabled' });
        const trigger = row(imageRecord.id).getByRole('button', { name: '查看原提交文件 ' + imageRecord.id, exact: true });
        await trigger.focus(); await page.keyboard.press('Enter');
        await expect.poll(() => dialog.getByRole('img').evaluate((image: HTMLImageElement) => image.complete && image.naturalWidth)).toBe(320);
        expect(await dialog.evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true);
        if (width < 768) for (const control of await dialog.getByRole('button').all()) expect((await control.boundingBox())!.height).toBeGreaterThanOrEqual(44);
        await page.screenshot({ path: path.join(artifacts, `binary-preview-${theme}-${width}.png`), animations: 'disabled' });
        await page.keyboard.press('Escape'); await expect(dialog).toHaveCount(0); await expect(trigger).toBeFocused();
      }
    }
    expect(writes).toEqual([]);
    const drift = fixture.records[3];
    await row(drift.id).getByRole('button', { name: '人工验收或恢复 ' + drift.id, exact: true }).click();
    const conflict = page.getByRole('dialog', { name: '人工验收提交收件', exact: true });
    await conflict.getByRole('button', { name: '确认接收原版本' }).click();
    await expect(conflict.getByText(/收件或文件版本已变化，原决定被拒绝/)).toBeVisible();
    await conflict.getByRole('button', { name: '关闭', exact: true }).click();
    for (const [index, decision] of ['accept', 'reject'].entries()) {
      const record = fixture.records[index];
      await row(record.id).getByRole('button', { name: '人工验收或恢复 ' + record.id, exact: true }).click();
      const review = page.getByRole('dialog', { name: '人工验收提交收件', exact: true });
      if (decision === 'reject') { await review.getByRole('combobox', { name: '人工验收决定' }).click(); await page.getByRole('option', { name: '拒绝（保留文件）', exact: true }).click(); }
      await review.getByRole('button', { name: decision === 'accept' ? '确认接收原版本' : '确认拒绝原版本', exact: true }).click();
      await expect(review.getByText(/原验收请求已被接受/)).toBeVisible(); await review.getByRole('button', { name: '确认验收结果并刷新' }).click();
    }
    await row(imageRecord.id).getByRole('button', { name: '归档或恢复 ' + imageRecord.id, exact: true }).click();
    const archive = page.getByRole('dialog', { name: '归档已接收收件', exact: true });
    await archive.getByRole('combobox', { name: '归档目标空间' }).click();
    await page.getByRole('option', { name: fixture.destinationName, exact: true }).click();
    await archive.getByRole('textbox', { name: '归档目标目录' }).fill('reviewed-images');
    await expect(archive.getByRole('button', { name: '确认归档原版本' })).toBeEnabled();
    await archive.getByRole('button', { name: '确认归档原版本' }).click();
    await expect(archive.getByText('归档完成，原操作与结果文件已核对。', { exact: true })).toBeVisible(); await archive.getByRole('button', { name: '确认归档结果并刷新' }).click();
    expect(await client.evaluate(value => value.revoke())).toBe(401);
    await client.dispose(); await page.reload(); await page.goto('/#/inbox');
    await expect(row(imageRecord.id).getByText('已归档', { exact: true })).toBeVisible();
    await expect(row(fixture.records[1].id).getByText('已拒绝（文件保留）', { exact: true })).toBeVisible();
    const proof = await page.evaluate(async records => {
      const results = [];
      async function hash(file: string, version: string) {
        const response = await fetch(`/api/v1/files/${file}/content?version=${version}`); if (!response.ok) throw new Error('binary_readback_failed');
        return Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', await response.arrayBuffer())), byte => byte.toString(16).padStart(2, '0')).join('');
      }
      for (const record of records) {
        const sub = (await (await fetch('/api/v1/inbox-submissions/' + record.id)).json()).data;
        let archived = '', archivedFile = '';
        if (sub.archive_operation_ref) { const op = (await (await fetch('/api/v1/operations/' + sub.archive_operation_ref)).json()).data.operation; archived = await hash(op.file_id, op.version_id); archivedFile = op.file_id; }
        results.push({ version: sub.version_id, state: sub.state, name: sub.file_name, author: sub.agent_name, digest: await hash(sub.file_ref, sub.version_id), archived, archivedFile });
      }
      return results;
    }, fixture.records);
    expect(proof[3].state).toBe('submitted');
    proof.forEach((result, index) => expect(result).toMatchObject({ version: fixture.records[index].version, name: fixture.records[index].name, author: '视觉研究助理', digest: fixture.records[index].sha256, archived: index === 0 ? fixture.records[0].sha256 : '' }));
    const outsider = await browser.newContext();
    try { const bob = await outsider.newPage(); await login(bob, 'Bob'); for (const record of fixture.records) for (const endpoint of ['/inbox-submissions/' + record.id, '/files/' + record.file + '/media?version=' + record.version, '/files/' + record.file + '/content?version=' + record.version]) expect((await bob.request.get('/api/v1' + endpoint)).status()).toBe(404); expect((await bob.request.get('/api/v1/files/' + proof[0].archivedFile)).status()).toBe(404); }
    finally { await outsider.close(); }
    const touchContext = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, colorScheme: 'dark', reducedMotion: 'no-preference' });
    try {
      const touch = await touchContext.newPage(); await login(touch); await touch.goto('/#/inbox');
      const trigger = touch.getByRole('button', { name: '查看原提交文件 ' + imageRecord.id, exact: true }); await trigger.tap();
      const view = touch.getByRole('dialog', { name: '收件固定版本预览', exact: true }); await expect.poll(() => view.getByRole('img').evaluate((image: HTMLImageElement) => image.complete && image.naturalWidth)).toBe(320);
      for (const button of await view.getByRole('button').all()) await expect.poll(async () => (await button.boundingBox())!.height).toBeGreaterThanOrEqual(44);
      await touch.screenshot({ path: path.join(artifacts, 'binary-touch-original.png') });
      await view.getByRole('button', { name: '关闭', exact: true }).tap(); await expect(trigger).toBeFocused();
    } finally { await touchContext.close(); }
  } finally { await client.dispose(); }
});
