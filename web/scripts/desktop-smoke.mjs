// Development-only Xvfb integration: dialog selection is controlled; the
// Electron -> Go -> Pinax metadata path is real. Not platform acceptance.
import { _electron as electron } from '@playwright/test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { mkdtemp, mkdir, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { execFileSync } from 'node:child_process';
const require = createRequire(import.meta.url);
if (process.getuid?.() === 0) throw new Error('Desktop smoke refuses root; sandbox must remain enabled.');
const path = fileURLToPath(new URL('../../desktop/', import.meta.url));
const executablePath = require('../../desktop/node_modules/electron');
const fixture = await mkdtemp(join(tmpdir(), 'desktop-smoke-'));
const home = join(fixture, 'home'); await mkdir(home);
const vault = join(fixture, 'vault');
const setupEnv = { PATH: process.env.PATH, HOME: home, USERPROFILE: home, XDG_CONFIG_HOME: home, XDG_CACHE_HOME: home, GIT_CONFIG_NOSYSTEM: '1' };
const pinax = join(path, 'host-bin', process.platform === 'win32' ? 'pinax.exe' : 'pinax');
const env = { ...process.env }; delete env.ELECTRON_RUN_AS_NODE;
let app;
try {
  execFileSync(pinax, ['init', vault, '--json'], { env: setupEnv, cwd: fixture, stdio: 'pipe', timeout: 15000 });
  execFileSync(pinax, ['note', 'add', 'Desktop smoke', '--dir', 'index', '--stdin', '--vault', vault, '--json'], { input: 'desktop-smoke-private-body', env: setupEnv, cwd: fixture, stdio: 'pipe', timeout: 15000 });
  app = await electron.launch({ executablePath, args: [path], env, timeout: 20000 });
  const page = await app.firstWindow();
  await page.getByText('可以选择已有 vault 进行只读预检。').waitFor({ timeout: 25000 });
  const preferences = await app.evaluate(({ BrowserWindow }) => {
    const prefs = BrowserWindow.getAllWindows()[0].webContents.getLastWebPreferences();
    return { sandbox: prefs.sandbox, contextIsolation: prefs.contextIsolation, nodeIntegration: prefs.nodeIntegration, webSecurity: prefs.webSecurity };
  });
  if (!preferences.sandbox || !preferences.contextIsolation || preferences.nodeIntegration || !preferences.webSecurity) throw new Error('Unsafe Electron preferences.');
  const boundary = await page.evaluate(async () => ({ require: typeof window.require, bridgeKeys: Object.keys(window.drivebridgeDesktop || {}).sort(), capabilities: await window.drivebridgeDesktop.capabilities({ input: {} }) }));
  if (boundary.require !== 'undefined' || boundary.bridgeKeys.join() !== 'capabilities,chooseVault' || boundary.capabilities.data.available.join() !== 'chooseVault') throw new Error('Unexpected preload exposure.');
  // Native dialog cannot be automated in Xvfb. Only its result is substituted
  // in the main process; there is no product flag/path bypass for tests.
  await app.evaluate(({ dialog }) => { dialog.showOpenDialog = async () => ({ canceled: true, filePaths: [] }); });
  await page.getByRole('button', { name: '选择本机 Pinax vault' }).click();
  await page.getByText('已取消选择，未连接来源。').waitFor();
  await app.evaluate(({ dialog }, selected) => { dialog.showOpenDialog = async () => ({ canceled: false, filePaths: [selected] }); }, vault);
  await page.getByRole('button', { name: '选择本机 Pinax vault' }).click();
  await page.getByText('受管范围保护尚未通过验证，来源未连接；正文和编辑均未开放。').waitFor({ timeout: 30000 });
  const body = await page.locator('body').innerText();
  if (body.includes(vault) || body.includes('desktop-smoke-private-body')) throw new Error('Private source data reached renderer.');
  await page.getByRole('button', { name: '桌面能力说明' }).click();
  await page.getByText('当前交付边界').waitFor();
  console.log('Electron -> DriveBridge -> Pinax metadata preflight passed; dialog result controlled, guard refusal preserved. Xvfb is not platform acceptance.');
} finally {
  if (app) await app.close();
  await rm(fixture, { recursive: true, force: true });
}
