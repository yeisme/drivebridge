import { test, expect, _electron as electron } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, rmSync, writeFileSync, mkdirSync, existsSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

test('native reader requires confirmation and reads owner search, body and links through the UI', async () => {
  const repo = resolve(import.meta.dirname, '../..');
  const desktop = join(repo, 'desktop');
  const work = mkdtempSync(join(tmpdir(), 'drivebridge-reader-ui-'));
  const vault = join(work, 'vault');
  const home = join(work, 'home');
  const env = { ...process.env, HOME: home, USERPROFILE: home, XDG_CONFIG_HOME: home, XDG_CACHE_HOME: home, GIT_CONFIG_NOSYSTEM: '1', GIT_CONFIG_GLOBAL: '/dev/null' };
  const pinax = join(desktop, 'host-bin/pinax');
  execFileSync(pinax, ['init', vault, '--title', 'Reader fixture', '--json'], { env, stdio: 'pipe' });
  execFileSync(pinax, ['note', 'add', 'Reader journey', '--dir', 'index', '--stdin', '--vault', vault, '--json'], { env, input: 'Verified local reader body.', stdio: 'pipe' });
  const picture = join(work, 'pixel.png');
  writeFileSync(picture, Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jhZkAAAAASUVORK5CYII=', 'base64'));
  const attached = JSON.parse(execFileSync(pinax, ['note', 'attach', 'Reader journey', picture, '--embed', '--vault', vault, '--json'], { env, stdio: 'pipe', encoding: 'utf8' }));
  execFileSync(pinax, ['asset', 'add', resolve(vault, attached.facts.attachment_path), '--register', '--vault', vault, '--json'], { env, stdio: 'pipe' });
  execFileSync('go', ['run', './desktop/e2e/fixture', join(home, 'drivebridge'), `${vault}:vault`], { cwd: repo, env: { ...process.env, CGO_ENABLED: '0', GOWORK: 'off' }, stdio: 'pipe' });
  const app = await electron.launch({ executablePath: join(desktop, 'node_modules/electron/dist/electron'), args: [desktop], env });
  try {
    await app.evaluate(({ dialog }, path) => { dialog.showOpenDialog = async () => ({ canceled: false, filePaths: [path] }); }, vault);
    const page = await app.firstWindow();
    const assetFailures: string[] = [];
    page.on('requestfailed', request => { if (request.url().startsWith('drivebridge-asset:')) assetFailures.push(request.failure()?.errorText || 'asset_request_failed'); });
    page.on('response', response => { if (response.url().startsWith('drivebridge-asset:') && response.status() !== 200) assetFailures.push('asset_status_' + response.status()); });
    await page.getByRole('button', { name: '选择本机 Pinax vault', exact: true }).click();
    await expect(page.getByRole('button', { name: '确认绑定', exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Reader journey', exact: true })).toHaveCount(0);
    await page.getByRole('button', { name: '确认绑定', exact: true }).click();
    await page.getByRole('button', { name: 'Reader journey', exact: true }).click();
    await expect(page.getByLabel('Pinax 固定正文')).toContainText('Verified local reader body.');
    const attachment = page.getByLabel('Pinax 附件状态').locator('img');
    const assetProjection = await page.evaluate(async () => {
      const bridge = window.drivebridgeDesktop!;
      const sources = await bridge.listSources!({ input: { limit: 50 } });
      if ('error' in sources) return { error: sources.error.code };
      const source_id = sources.data.items[0].source_id;
      const notes = await bridge.listNotes!({ source_id, input: { limit: 50, sort: 'updated_desc' } });
      if ('error' in notes) return { error: notes.error.code };
      const note = notes.data.items.find(note => note.title === 'Reader journey')!;
      const document = await bridge.readDocument!({ source_id, input: { object_id: note.object_id, revision: note.revision } });
      if ('error' in document) return { error: document.error.code };
      const collect = (nodes: typeof document.data.nodes): unknown[] => nodes.flatMap(node => [...(node.type === 'image' ? [{ scheme: node.ref?.startsWith('drivebridge-asset://') ? 'drivebridge-asset' : 'unresolved', length: node.ref?.length ?? 0 }] : []), ...collect(node.children || [])]);
      return { scopes: sources.data.items[0].capabilities, imageRefs: collect(document.data.nodes) };
    });
    expect(assetProjection).toMatchObject({ imageRefs: [expect.objectContaining({ scheme: 'drivebridge-asset' })] });

    await attachment.scrollIntoViewIfNeeded();
    await expect.poll(async () => ({ decoded: await attachment.evaluate((image: HTMLImageElement) => image.complete && image.naturalWidth > 0), failures: assetFailures })).toEqual({ decoded: true, failures: [] });
    expect(await attachment.getAttribute('src')).toMatch(/^drivebridge-asset:\/\/asset\/[a-f0-9]{64}$/);
    await page.getByRole('textbox', { name: '搜索 Pinax 笔记' }).fill('Reader journey');
    await expect(page.getByRole('button', { name: 'Reader journey', exact: true })).toBeVisible();
    await page.getByRole('button', { name: 'Reader journey', exact: true }).click();
    await expect(page.getByLabel('Pinax 固定正文')).toContainText('Verified local reader body.');
    const security = await app.evaluate(({ BrowserWindow }) => BrowserWindow.getAllWindows()[0].webContents.getLastWebPreferences());
    expect(security.sandbox).toBe(true);
    expect(security.contextIsolation).toBe(true);
    expect(await page.evaluate(() => typeof (window as any).require)).toBe('undefined');
    expect(await page.evaluate(() => typeof (window as any).process)).toBe('undefined');
    // Electron exposes actual Chromium page zoom; device pixel scaling alone
    // would not exercise the 200% reading and keyboard layout.
    await app.evaluate(({ BrowserWindow }) => { BrowserWindow.getAllWindows()[0].webContents.setZoomFactor(2); });
    expect(await app.evaluate(({ BrowserWindow }) => BrowserWindow.getAllWindows()[0].webContents.getZoomFactor())).toBe(2);
    await page.emulateMedia({ colorScheme: 'dark', reducedMotion: 'reduce' });
    await page.getByLabel('Pinax 固定正文').scrollIntoViewIfNeeded();
    await expect(page.getByLabel('Pinax 固定正文')).toContainText('Verified local reader body.');
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await expect.poll(() => page.getByLabel('Pinax 阅读器', { exact: true }).evaluate(element =>
      element.scrollWidth <= element.clientWidth && Array.from(element.querySelectorAll('main, header')).every(node => node.scrollWidth <= node.clientWidth))).toBe(true);
    const favorite = page.getByRole('button', { name: '收藏', exact: true });
    await favorite.focus();
    await page.keyboard.press('Enter');
    await expect(page.getByRole('button', { name: '已收藏', exact: true })).toHaveAttribute('aria-pressed', 'true');
    await page.screenshot({ path: join(process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!, 'desktop-reader-dark-200-percent.png'), animations: 'disabled' });
    await app.evaluate(({ BrowserWindow }) => { BrowserWindow.getAllWindows()[0].webContents.setZoomFactor(1); });
    await app.evaluate(({ powerMonitor }) => { powerMonitor.emit('suspend'); });
    await expect(page.getByLabel('Pinax 固定正文')).toHaveCount(0);
    await expect(page.getByText('本机会话已失效，正文与草稿已清除。请关闭并重新打开工作区。')).toBeVisible();
  } finally { await app.close(); rmSync(work, { recursive: true, force: true }); }
});

test('one main process isolates two native states and reuses the original window', async () => {
  const repo = resolve(import.meta.dirname, '../..');
  const desktop = join(repo, 'desktop');
  const work = mkdtempSync(join(tmpdir(), 'drivebridge-two-state-ui-'));
  const home = join(work, 'home');
  const env = { ...process.env, HOME: home, USERPROFILE: home, XDG_CONFIG_HOME: home, XDG_CACHE_HOME: home, GIT_CONFIG_NOSYSTEM: '1', GIT_CONFIG_GLOBAL: '/dev/null' };
  const pinax = join(desktop, 'host-bin/pinax');
  const vaults = [join(work, 'vault-a'), join(work, 'vault-b')];
  const states = [join(home, 'drivebridge'), join(work, 'state-b')];
  let application: Awaited<ReturnType<typeof electron.launch>> | undefined;
  try {
    for (let i = 0; i < 2; i++) {
      execFileSync(pinax, ['init', vaults[i], '--title', `State ${i}`, '--json'], { env, stdio: 'pipe' });
      execFileSync(pinax, ['note', 'add', `Isolated ${i}`, '--dir', 'index', '--stdin', '--vault', vaults[i], '--json'], { env, input: `Private body for state ${i}.` + (i === 0 ? Array.from({ length: 100 }, (_, index) => `\n\nSection ${index}: ` + 'Reading position fixture. '.repeat(20)).join('') : ''), stdio: 'pipe' });
      execFileSync('go', ['run', './desktop/e2e/fixture', states[i], `${vaults[i]}:vault`], { cwd: repo, env: { ...process.env, CGO_ENABLED: '0', GOWORK: 'off' }, stdio: 'pipe' });
    }
    application = await electron.launch({ executablePath: join(desktop, 'node_modules/electron/dist/electron'), args: [desktop], env });
    const app = application;
    const first = await app.firstWindow();
    const bind = async (page: typeof first, i: number) => {
      await app.evaluate(({ dialog }, path) => { dialog.showOpenDialog = async () => ({ canceled: false, filePaths: [path] }); }, vaults[i]);
      await page.getByRole('button', { name: '选择本机 Pinax vault', exact: true }).click();
      await page.getByRole('button', { name: '确认绑定', exact: true }).click();
      await page.getByRole('button', { name: `Isolated ${i}`, exact: true }).click();
      await expect(page.getByLabel('Pinax 固定正文')).toContainText(`Private body for state ${i}.`);
      const text = await page.getByLabel('Pinax 来源绑定').innerText();
      return /source: (\S+)/.exec(text)![1];
    };
    const firstSource = await bind(first, 0);
    await first.getByRole('button', { name: '收藏', exact: true }).click();
    await expect(first.getByRole('button', { name: '已收藏', exact: true })).toHaveAttribute('aria-pressed', 'true');
    await first.getByRole('button', { name: '稍后读', exact: true }).click();
    await expect(first.getByRole('button', { name: '已加入稍后读', exact: true })).toHaveAttribute('aria-pressed', 'true');
    const savedAnchor = await first.getByLabel('Pinax 固定正文').evaluate(element => {
      element.scrollTop = 2000;
      const top = element.getBoundingClientRect().top;
      return Array.from(element.querySelectorAll<HTMLElement>('[data-pinax-anchor]')).find(node => node.getBoundingClientRect().bottom > top)?.dataset.pinaxAnchor;
    });
    expect(savedAnchor).toBeTruthy();
    await first.getByRole('button', { name: '记住当前位置', exact: true }).click();
    await expect(first.getByRole('button', { name: '继续上次阅读', exact: true })).toBeVisible();

    const openState = async (path: string) => app.evaluate(async ({ Menu, dialog }, selectedPath) => {
      dialog.showOpenDialog = async () => ({ canceled: false, filePaths: [selectedPath] });
      const item = Menu.getApplicationMenu()!.items[0].submenu!.items.find((item: { label: string }) => item.label === '打开已有工作区…')!;
      await item.click(undefined as never, undefined as never, undefined as never);
    }, path);
    const newWindow = app.waitForEvent('window');
    await openState(states[1]);
    const second = await newWindow;
    const secondSource = await bind(second, 1);
    await expect(second.getByRole('button', { name: '收藏', exact: true })).toHaveAttribute('aria-pressed', 'false');
    expect(firstSource).not.toBe(secondSource);
    for (const [page, foreign] of [[first, secondSource], [second, firstSource]] as const) {
      const result = await page.evaluate(async source => window.drivebridgeDesktop!.listNotes!({ source_id: source, input: { limit: 50, sort: 'updated_desc' } }), foreign);
      expect('error' in result).toBe(true);
      if ('error' in result) expect(result.error.code).toBe('source_unknown');
    }
    expect(await app.evaluate(({ BrowserWindow }) => {
      const windows = BrowserWindow.getAllWindows();
      return windows.length === 2 && windows[0].webContents.session !== windows[1].webContents.session && windows.every((window: { webContents: { getLastWebPreferences(): { sandbox?: boolean } } }) => window.webContents.getLastWebPreferences().sandbox);
    })).toBe(true);
    await openState(states[0]);
    await expect.poll(() => app.evaluate(({ BrowserWindow }) => BrowserWindow.getAllWindows().length)).toBe(2);
    await expect(first.getByLabel('Pinax 固定正文')).toContainText('Private body for state 0.');
    await first.close();
    await second.getByRole('textbox', { name: '搜索 Pinax 笔记' }).fill('Isolated 1');
    await expect(second.getByRole('button', { name: 'Isolated 1', exact: true })).toBeVisible();
    expect(await app.evaluate(({ BrowserWindow }) => BrowserWindow.getAllWindows().length)).toBe(1);
    await app.close(); application = undefined;
    application = await electron.launch({ executablePath: join(desktop, 'node_modules/electron/dist/electron'), args: [desktop], env });
    const restarted = await application.firstWindow();
    await restarted.getByRole('button', { name: 'Isolated 0', exact: true }).click();
    await expect(restarted.getByLabel('Pinax 固定正文')).toContainText('Private body for state 0.');
    await expect(restarted.getByLabel('Pinax 来源绑定')).toContainText(firstSource);
    await expect(restarted.getByRole('button', { name: '已收藏', exact: true })).toHaveAttribute('aria-pressed', 'true');
    await expect(restarted.getByRole('button', { name: '已加入稍后读', exact: true })).toHaveAttribute('aria-pressed', 'true');
    await restarted.getByRole('button', { name: '继续上次阅读', exact: true }).click();
    await expect.poll(() => restarted.getByLabel('Pinax 固定正文').evaluate(element => {
      const top = element.getBoundingClientRect().top;
      return Array.from(element.querySelectorAll<HTMLElement>('[data-pinax-anchor]')).find(node => node.getBoundingClientRect().bottom > top)?.dataset.pinaxAnchor;
    })).toBe(savedAnchor);

  } finally { await application?.close(); rmSync(work, { recursive: true, force: true }); }
});

test('first setup creates an explicit owner state and registers a source through native confirmation', async () => {
 const repo = resolve(import.meta.dirname, '../..'), desktop = join(repo, 'desktop');
 const work = mkdtempSync(join(tmpdir(), 'drivebridge-setup-ui-')), home = join(work, 'home'), vault = join(work, 'vault');
 mkdirSync(home, { recursive: true, mode: 0o700 });
 const env = { ...process.env, HOME: home, USERPROFILE: home, XDG_CONFIG_HOME: home, XDG_CACHE_HOME: home, GIT_CONFIG_NOSYSTEM: '1', GIT_CONFIG_GLOBAL: '/dev/null' };
 const pinax = join(desktop, 'host-bin/pinax');
 let application: Awaited<ReturnType<typeof electron.launch>> | undefined;
 try {
  execFileSync(pinax, ['init', vault, '--title', 'Setup fixture', '--json'], { env, stdio: 'pipe' });
  execFileSync(pinax, ['note', 'add', 'Fresh setup', '--dir', 'index', '--stdin', '--vault', vault, '--json'], { env, input: 'Created through the native setup journey.', stdio: 'pipe' });
  application = await electron.launch({ executablePath: join(desktop, 'node_modules/electron/dist/electron'), args: [desktop], env });
  const app = application, original = await app.firstWindow();
  await expect(original.getByText('请通过顶部“工作区”菜单新建本机工作区，或显式迁移已有工作区。')).toBeVisible();
  expect(existsSync(join(home, 'drivebridge'))).toBe(false);
  const created = app.waitForEvent('window');
  await app.evaluate(({ dialog, Menu }, state) => {
   dialog.showSaveDialog = async () => ({ canceled: false, filePath: state });
   dialog.showMessageBox = async () => ({ response: 1, checkboxChecked: false });
   const item = Menu.getApplicationMenu()!.items[0].submenu!.items.find((item: { label: string }) => item.label === '新建本机工作区…')!;
   item.click(undefined as never, undefined as never, undefined as never);
  }, join(home, 'drivebridge'));
  const page = await created;
  await expect(page.getByRole('button', { name: '登记新的 Pinax 来源', exact: true })).toBeVisible();
  await app.evaluate(({ dialog }, path) => { dialog.showOpenDialog = async () => ({ canceled: false, filePaths: [path] }); }, vault);
  await page.getByRole('button', { name: '登记新的 Pinax 来源', exact: true }).click();
  await page.getByRole('button', { name: '确认绑定', exact: true }).click();
  await page.getByRole('button', { name: 'Fresh setup', exact: true }).click();
  await expect(page.getByLabel('Pinax 固定正文')).toContainText('Created through the native setup journey.');
  await app.close(); application = undefined;
  application = await electron.launch({ executablePath: join(desktop, 'node_modules/electron/dist/electron'), args: [desktop], env });
  const reopened = await application.firstWindow();
  await reopened.getByRole('button', { name: 'Fresh setup', exact: true }).click();
  await expect(reopened.getByLabel('Pinax 固定正文')).toContainText('Created through the native setup journey.');
 } finally { await application?.close(); rmSync(work, { recursive: true, force: true }); }
});
