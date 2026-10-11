// Native Linux package startup only; CDP inspects renderer, never Node/main.
// Xvfb is not platform acceptance. No sandbox or fuse overrides are permitted.
import { chromium, expect } from '@playwright/test';
import { spawn } from 'node:child_process';
import { mkdtemp, mkdir, readFile, readlink, realpath, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

if (process.platform !== 'linux' || process.getuid?.() === 0 || !process.argv[2]) throw new Error('desktop_package_smoke_unsupported');
const root = await realpath(resolve(process.argv[2]));
const fixture = await mkdtemp(join(tmpdir(), 'drivebridge-package-smoke-'));
const home = join(fixture, 'home'); await mkdir(home);
const env = { ...process.env, HOME: home, USERPROFILE: home, XDG_CONFIG_HOME: home, XDG_CACHE_HOME: home };
for (const key of Object.keys(env)) if (/^(ELECTRON_|NODE_OPTIONS$|PINAX_|DRIVEBRIDGE_)/.test(key)) delete env[key];
let child, browser, ownerPID, exited = false, force = false;
try {
  child = spawn(join(root, 'drivebridge-desktop'), ['--user-data-dir=' + join(fixture, 'profile'), '--remote-debugging-address=127.0.0.1', '--remote-debugging-port=0'], { cwd: root, env, detached: true, stdio: ['ignore', 'ignore', 'pipe'] });
  const exit = new Promise(resolve => child.once('exit', (code, signal) => { exited = true; resolve({ code, signal }); }));
  const endpoint = await new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('desktop_package_debug_timeout')), 25000);
    let tail = '', size = 0;
    child.once('error', () => { clearTimeout(timer); reject(new Error('desktop_package_start_failed')); });
    child.once('exit', () => { clearTimeout(timer); reject(new Error('desktop_package_exited')); });
    child.stderr.on('data', chunk => {
      size += chunk.length;
      if (size > 1024 * 1024) { clearTimeout(timer); reject(new Error('desktop_package_output_limit')); return; }
      tail = (tail + chunk.toString()).slice(-8192);
      const match = tail.match(/DevTools listening on (ws:\/\/127\.0\.0\.1:\d+\/devtools\/browser\/[a-zA-Z0-9-]+)/);
      if (match) { clearTimeout(timer); resolve(match[1]); }
    });
  });
  browser = await chromium.connectOverCDP(endpoint);
  const context = browser.contexts()[0];
  await expect.poll(() => context.pages().length, { timeout: 20000 }).toBeGreaterThan(0);
  const page = context.pages()[0];
  await page.getByText('可以选择已有 vault 进行只读预检。').waitFor({ timeout: 25000 });
  const boundary = await page.evaluate(async () => ({
    require: typeof window.require, process: typeof window.process,
    keys: Object.keys(window.drivebridgeDesktop || {}).sort(),
    capability: await window.drivebridgeDesktop.capabilities({ input: {} }),
  }));
  if (boundary.require !== 'undefined' || boundary.process !== 'undefined' || boundary.keys.join() !== 'capabilities,chooseVault' || boundary.capability.data.available.join() !== 'chooseVault') throw new Error('desktop_package_renderer_boundary_invalid');
  // Verify the live direct child is from this package, not a development binary.
  const children = (await readFile(`/proc/${child.pid}/task/${child.pid}/children`, 'utf8')).trim().split(/\s+/).filter(Boolean);
  for (const pid of children) {
    try { if (await readlink(`/proc/${pid}/exe`) === join(root, 'resources', 'host-bin', 'drivebridge')) ownerPID = pid; } catch { /* A renderer process may have exited. */ }
  }
  if (!ownerPID) throw new Error('desktop_package_owner_missing');
  const ownerArgs = (await readFile(`/proc/${ownerPID}/cmdline`, 'utf8')).split('\0');
  if (ownerArgs.slice(1, 3).join() !== 'desktop,serve') throw new Error('desktop_package_owner_identity_invalid');
  // window.close traverses the application's normal window-all-closed/quit path.
  await page.evaluate(() => window.close()).catch(() => {});
  const outcome = await Promise.race([exit, new Promise((_, reject) => { const timer = setTimeout(() => reject(new Error('desktop_package_exit_timeout')), 15000); timer.unref(); })]);
  if (outcome.code !== 0 || outcome.signal !== null) throw new Error('desktop_package_exit_failed');
  await expect.poll(async () => { try { await readlink(`/proc/${ownerPID}/exe`); return false; } catch (error) { if (error.code === 'ENOENT') return true; throw error; } }, { timeout: 5000 }).toBe(true);
  console.log('Packaged Linux ASAR UI and bundled Go host started and exited; renderer boundary verified. Xvfb is not platform acceptance.');
} finally {
  if (browser) await browser.close().catch(() => {});
  if (child?.pid && !exited) {
    force = true;
    // Only this test-created process group; never reuse or kill unknown services.
    try { process.kill(-child.pid, 'SIGKILL'); } catch (error) { if (error.code !== 'ESRCH') throw error; }
    await new Promise(resolve => { if (exited) resolve(); else child.once('exit', resolve); });
  }
  await rm(fixture, { recursive: true, force: true });
  if (force) console.error('desktop_package_smoke_required_forced_cleanup');
}
