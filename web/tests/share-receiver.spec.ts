/// <reference lib="es2024.promise" />
import { expect, test, type Page } from '@playwright/test';

// Complete links, cookies, keys and passcodes must not enter screenshots/traces.
test.use({ trace: 'off', screenshot: 'off', video: 'off' });

async function createFixture(page: Page, name: string, maxOpens?: number) {
  await page.goto('/');
  await page.getByRole('link', { name: '使用账号登录' }).click();
  await page.getByRole('link', { name: '以 Alice 登录' }).click();
  return page.evaluate(async ({ name, maxOpens }) => {
    const me = (await (await fetch('/api/v1/me')).json()).data;
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf_token };
    async function post(endpoint: string, body: unknown) {
      const response = await fetch('/api/v1' + endpoint, { method: 'POST', headers: { ...headers, 'Idempotency-Key': crypto.randomUUID() }, body: JSON.stringify(body) });
      if (!response.ok) throw new Error('receiver_fixture_failed');
      return (await response.json()).data;
    }
    const space = await post('/spaces', { name, connection_id: 'platform' });
    const bytes = new TextEncoder().encode('Disposable receiver fixture content');
    const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), b => b.toString(16).padStart(2, '0')).join('');
    const upload = await post('/uploads', { space_id: space.id, path: 'receiver-fixture.txt', size_bytes: bytes.length, sha256 });
    let file = upload;
    if (upload.status !== 'completed') {
      const part = await fetch('/api/v1/operations/' + upload.id + '/parts/0', { method: 'PUT', headers: { 'X-CSRF-Token': me.csrf_token }, body: bytes });
      if (!part.ok) throw new Error('receiver_fixture_part_failed');
      file = await post('/operations/' + upload.id + '/complete', {});
    }
    const result = await post('/shares', { name, mode: 'fixed', audience: 'link', space_id: space.id, days: 1, passcode: 'fixture-passcode', allow_download: false, max_opens: maxOpens, items: [{ file_id: file.file_id, version_id: file.version_id }] });
    // Keep the secret in the owner page memory only, never in runner output.
    Object.assign(window, { receiverFixtureLink: result.link });
    return result.share.id as string;
  }, { name, maxOpens });
}

async function openFixture(page: Page) {
  await page.context().addInitScript(() => {
    // Popup initialization can run before Chromium applies the destination
    // fragment. Read the fixture secret from its same-origin opener instead;
    // an absent observer is not evidence that an outbound secret was observed.
    const opener: unknown = window.opener;
    if (!opener || typeof opener !== 'object' || !('receiverFixtureLink' in opener) || typeof opener.receiverFixtureLink !== 'string') return;
    const secret = new URL(opener.receiverFixtureLink).hash.split('/')[2] || '';
    let leaked = false;
    let requests = 0;
    const original = window.fetch.bind(window);
    window.fetch = (input, init) => {
      requests++;
      if (secret && (String(input).includes(secret) || JSON.stringify(init || {}).includes(secret))) leaked = true;
      return original(input, init);
    };
    Object.defineProperty(window, 'receiverSecretObservation', { get: () => ({ captured: !!secret, requestsObserved: requests > 0, networkLeak: leaked, localStorageLeak: !!secret && JSON.stringify(localStorage).includes(secret) }) });
  });
  const popup = page.waitForEvent('popup');
  await page.evaluate(() => {
    if (!('receiverFixtureLink' in window) || typeof window.receiverFixtureLink !== 'string') throw new Error('receiver_fixture_link_missing');
    window.open(window.receiverFixtureLink, '_blank');
  });
  const receiver = await popup;
  await receiver.getByLabel('提取码', { exact: true }).fill('fixture-passcode');
  await receiver.getByRole('button', { name: '打开分享', exact: true }).click();
  await expect(receiver.getByRole('button', { name: '记住此设备', exact: true })).toBeVisible();
  const observation = await receiver.evaluate(() => 'receiverSecretObservation' in window ? window.receiverSecretObservation : null);
  expect(observation).toEqual({ captured: true, requestsObserved: true, networkLeak: false, localStorageLeak: false });
  return receiver;
}

test('proof, persisted non-exportable device, same-origin challenge and owner revocation', async ({ page, context }) => {
  expect(process.env.DRIVEBRIDGE_TEST_SHARE_RECEIVERS).toBe('1');
  const id = await createFixture(page, 'Receiver device lifecycle');
  const receiver = await openFixture(page);
  await receiver.getByRole('button', { name: '记住此设备', exact: true }).click();
  await expect(receiver.getByText(/此设备已登记/)).toBeVisible();
  const stored = await receiver.evaluate(async id => {
    const { promise, resolve, reject } = Promise.withResolvers<IDBDatabase>();
    const request = indexedDB.open('drivebridge-share-receivers', 1);
    request.onsuccess = () => resolve(request.result); request.onerror = () => reject(new Error('device_store_read_failed'));
    const db = await promise;
    const read = Promise.withResolvers<{ privateKey: CryptoKey; publicKey: string; id: string }>();
    const tx = db.transaction('devices', 'readonly');
    const key = `${location.origin}/share/${encodeURIComponent(id)}`;
    const get = tx.objectStore('devices').get(key);
    get.onsuccess = () => read.resolve(get.result); get.onerror = () => read.reject(new Error('device_key_read_failed'));
    const device = await read.promise; db.close();
    let exportDenied = false;
    try { await crypto.subtle.exportKey('pkcs8', device.privateKey); } catch { exportDenied = true; }
    return { nonExportable: !device.privateKey.extractable, algorithm: device.privateKey.algorithm.name, exportDenied, noLocalSecret: !Object.keys(localStorage).some(key => /receiver|secret|private/i.test(key)), shareScoped: !!device.id };
  }, id);
  expect(stored).toEqual({ nonExportable: true, algorithm: 'Ed25519', exportDenied: true, noLocalSecret: true, shareScoped: true });
  await context.clearCookies();
  const challenge = receiver.waitForRequest(request => request.url().endsWith(`/shared/${id}/challenge`));
  const deviceOpen = receiver.waitForResponse(response => response.url().endsWith(`/shared/${id}/device-open`));
  await receiver.reload();
  const challengeRequest = await challenge;
  const headers = await challengeRequest.allHeaders();
  expect(headers.origin).toBeUndefined(); expect(headers['sec-fetch-site']).toBe('same-origin');
  expect((await deviceOpen).status()).toBe(200);
  await expect(receiver.getByText('receiver-fixture.txt', { exact: true })).toBeVisible();
  await expect(receiver.getByLabel('提取码', { exact: true })).toHaveCount(0);
  // Clearing cookies affected the owner too; restore its fixture human session.
  await page.goto('/'); await page.getByRole('link', { name: '使用账号登录' }).click(); await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await page.goto('/#/shares');
  await page.getByRole('button', { name: '分享 Receiver device lifecycle 的更多操作' }).click();
  await page.getByRole('menuitem', { name: '接收设备与额度' }).click();
  const dialog = page.getByRole('dialog', { name: '接收设备与额度' });
  await expect(dialog.getByText(/打开次数：2/)).toBeVisible();
  await dialog.getByRole('button', { name: '撤销设备 此浏览器' }).click();
  await page.getByRole('button', { name: '确认撤销设备', exact: true }).click();
  await expect(dialog.getByText('已撤销', { exact: true })).toBeVisible();
  let legacy = false;
  receiver.on('request', request => { if (request.url().endsWith(`/shared/${id}/open`)) legacy = true; });
  await receiver.reload();
  await expect(receiver.getByRole('alert')).toContainText('撤销');
  await expect(receiver.getByText('receiver-fixture.txt', { exact: true })).toHaveCount(0);
  expect(legacy).toBe(false);
});

test('open quota exhaustion fails explicitly without legacy bypass', async ({ page, context }) => {
  expect(process.env.DRIVEBRIDGE_TEST_SHARE_RECEIVERS).toBe('1');
  const id = await createFixture(page, 'Receiver quota lifecycle', 2);
  const receiver = await openFixture(page);
  await receiver.getByRole('button', { name: '记住此设备', exact: true }).click();
  await expect(receiver.getByText(/此设备已登记/)).toBeVisible();
  await context.clearCookies(); await receiver.reload();
  await expect(receiver.getByText('receiver-fixture.txt', { exact: true })).toBeVisible();
  await context.clearCookies();
  let legacy = false;
  receiver.on('request', request => { if (request.url().endsWith(`/shared/${id}/open`)) legacy = true; });
  await receiver.reload();
  await expect(receiver.getByRole('alert')).toContainText('额度耗尽');
  await expect(receiver.getByText('receiver-fixture.txt', { exact: true })).toHaveCount(0);
  expect(legacy).toBe(false);
});

test('unsupported Ed25519 keeps full-link access without pretending device registration', async ({ page }) => {
  expect(process.env.DRIVEBRIDGE_TEST_SHARE_RECEIVERS).toBe('1');
  await createFixture(page, 'Receiver unsupported device');
  const receiver = await openFixture(page);
  await receiver.evaluate(() => {
    Object.defineProperty(crypto.subtle, 'generateKey', { value: async () => { throw new DOMException('Unsupported algorithm', 'NotSupportedError'); } });
  });
  let registered = false;
  receiver.on('request', request => { if (request.url().endsWith('/devices')) registered = true; });
  await receiver.getByRole('button', { name: '记住此设备', exact: true }).click();
  await expect(receiver.getByRole('alert')).toContainText('请保留完整分享链接');
  await expect(receiver.getByText(/此设备已登记/)).toHaveCount(0);
  await expect(receiver.getByText('receiver-fixture.txt', { exact: true })).toBeVisible();
  expect(registered).toBe(false);
});
