// Browser component acceptance only: real React/CSS, synthetic fixed IPC.
// No Electron, Pinax process, user vault, or write admission is involved.
import { chromium, expect } from '@playwright/test';
import { build, preview } from 'vite';
import react from '@vitejs/plugin-react';
import tailwind from '@tailwindcss/vite';
import { existsSync, readdirSync } from 'node:fs';
import { mkdir, mkdtemp, writeFile, rm } from 'node:fs/promises';
import { homedir } from 'node:os';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS || resolve(root, '../temp/pinax-workbench-browser');
await mkdir(artifacts, { recursive: true });
await mkdir(join(root, 'temp'), { recursive: true });
const html = `<!doctype html><html lang="zh-CN"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head><body><div id="fixture"></div>
<script type="module">
import React from 'react';
import { createRoot } from 'react-dom/client';
import { PinaxSyncPanel } from ${JSON.stringify(join(root, 'src/features/notebook/PinaxSyncPanel.tsx'))};
import ${JSON.stringify(join(root, 'src/style.css'))};
import { claimPinaxRecovery, inspectPinaxRecovery, clearPinaxRecovery } from ${JSON.stringify(join(root, 'src/features/notebook/pinax-workbench-recovery.ts'))};
const recoveryScope = { actor:'fixture-actor', sourceId:'fixture-source', bindingId:'fixture-binding' };
window.claimRecovery = key => claimPinaxRecovery(recoveryScope,key,new AbortController().signal);
window.inspectRecovery = () => inspectPinaxRecovery(recoveryScope);
window.clearRecovery = key => clearPinaxRecovery(recoveryScope,key);
const view = createRoot(document.getElementById('fixture'));
const projection = (command, facts) => ({ data: { spec_version: '1.0', command, status: 'success', summary: 'Fixture sync', facts } });
let slow;
const methods = {
  syncStatus: ({source_id}) => source_id === 'slow' ? new Promise(resolve => { slow = resolve; }) : Promise.resolve(source_id === 'revoked' ? {error:{code:'source_revoked'}} : projection('sync.status',{sync_status:'configured',last_synced_revision:source_id+'-local',remote_revision:source_id+'-remote',remote_write:'false'})),
  syncDiff: () => Promise.resolve(projection('sync.diff',{})),
  listSyncConflicts: () => Promise.resolve(projection('sync.conflicts.list',{conflicts:'3'})),
};
window.renderFixture = source => view.render(React.createElement(PinaxSyncPanel, {bridge:{...methods},sourceId:source}));
window.finishSlow = () => slow(projection('sync.status',{sync_status:'configured',last_synced_revision:'late-private-source'}));
window.renderFixture('first');
</script></body></html>`;
const fixture = await mkdtemp(join(root, 'temp/pinax-workbench-smoke-'));
await writeFile(join(fixture, 'index.html'), html);
let server;
let browser;
try {
  await build({ root: fixture, configFile: false, plugins: [react(), tailwind()],
    resolve: { alias: { '@': join(root, 'src') }, dedupe: ['react', 'react-dom'] },
    build: { outDir: join(fixture, 'dist'), emptyOutDir: true }, logLevel: 'error' });
  server = await preview({ root: fixture, configFile: false, build: { outDir: join(fixture, 'dist') }, preview: { host: '127.0.0.1', port: 0 }, logLevel: 'error' });
  const cache = process.env.PLAYWRIGHT_BROWSERS_PATH || join(homedir(), '.cache/ms-playwright');
  const executablePath = process.env.PLAYWRIGHT_CHROMIUM || readdirSync(cache).filter(name => /^chromium-\d+$/.test(name)).sort().reverse().map(name => join(cache, name, 'chrome-linux64/chrome')).find(existsSync);
  browser = await chromium.launch({ executablePath });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/`);
  await expect(page.getByText('first-local', { exact: true })).toBeVisible();
  await expect(page.getByText('3', { exact: true })).toBeVisible();
  for (const name of ['确认 Push', '确认 Pull', '解决冲突']) await expect(page.getByRole('button', { name, exact: true })).toBeDisabled();
  await page.keyboard.press('Tab');
  await expect(page.getByRole('button', { name: '刷新状态' })).toBeFocused();
  await page.keyboard.press('Enter');
  await expect(page.getByText('first-local', { exact: true })).toBeVisible();
  await page.screenshot({ path: join(artifacts, 'pinax-sync-wide.png'), animations: 'disabled' });
  await page.evaluate(() => window.renderFixture('slow'));
  await expect(page.getByText('first-local', { exact: true })).toHaveCount(0);
  await page.evaluate(() => window.renderFixture('current'));
  await expect(page.getByText('current-local', { exact: true })).toBeVisible();
  await page.evaluate(() => window.finishSlow());
  await expect(page.getByText('late-private-source', { exact: true })).toHaveCount(0);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.evaluate(() => document.documentElement.classList.add('dark'));
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await expect(page.getByText('current-local', { exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.screenshot({ path: join(artifacts, 'pinax-sync-mobile-dark.png'), animations: 'disabled' });
  await page.evaluate(() => window.renderFixture('revoked'));
  await expect(page.getByRole('alert')).toContainText('source_revoked');
  await expect(page.getByText('current-local', { exact: true })).toHaveCount(0);
  expect(await page.evaluate(() => ({ local: localStorage.length, session: sessionStorage.length }))).toEqual({ local: 0, session: 0 });
  expect(await page.evaluate(() => window.claimRecovery('original'))).toEqual({record:'original',claimed:true});
  expect(await page.evaluate(() => Object.values(localStorage))).toEqual(['original']);
  await page.reload();
  await expect(page.getByText('first-local', { exact: true })).toBeVisible();
  expect(await page.evaluate(() => window.inspectRecovery())).toBe('original');
  expect(await page.evaluate(() => window.claimRecovery('replacement'))).toEqual({record:'original',claimed:false});
  expect(await page.evaluate(async () => {try {await window.clearRecovery('replacement');return 'unexpected';} catch(error) {return error.reason;} })).toBe('recovery_changed');
  await page.evaluate(() => window.clearRecovery('original'));
  expect(await page.evaluate(() => localStorage.length)).toBe(0);
  expect(errors).toEqual([]);
  console.log('Pinax browser component checks passed: conflict facts, keyboard, source isolation, stale response, revoked state, mobile dark/reduced-motion, disabled mutations and opaque-key recovery across reload using native Web Locks. Fixture IPC only.');
} finally {
  if (browser) await browser.close();
  if (server) await new Promise(resolve => server.httpServer.close(resolve));
  await rm(fixture, { recursive: true, force: true });
}
