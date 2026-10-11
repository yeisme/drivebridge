import { afterEach, expect, it } from 'vitest';
import { mkdtemp, mkdir, writeFile, symlink, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { APP_URL, CSP, appResource, isAppDocument, requireEmptyRequest, requireMainFrame } from './security';

const roots: string[] = [];
afterEach(async () => { await Promise.all(roots.splice(0).map(root => rm(root, { recursive: true, force: true }))); });
async function fixture() {
  const root = await mkdtemp(join(tmpdir(), 'desktop-assets-')); roots.push(root);
  await mkdir(join(root, 'assets')); await writeFile(join(root, 'index.html'), '<main>Existing Web</main>');
  await writeFile(join(root, 'assets', 'app.js'), 'export {};');
  return root;
}
it('only accepts the exact main application document and frame', () => {
  expect(isAppDocument(APP_URL + '#knowledge')).toBe(true);
  for (const url of ['https://app/', 'file:///index.html', 'drivebridge-app://evil/', 'drivebridge-app://app:99/', 'drivebridge-app://user@app/', 'drivebridge-app://app/assets/a.html', APP_URL + '?url=evil', 'about:blank']) expect(isAppDocument(url)).toBe(false);
  const trusted = { destroyed: false, sameContents: true, sameFrame: true, url: APP_URL };
  expect(() => requireMainFrame(trusted)).not.toThrow();
  for (const bad of [{ destroyed: true }, { sameContents: false }, { sameFrame: false }, { url: 'about:blank' }]) expect(() => requireMainFrame({ ...trusted, ...bad })).toThrow();
});
it('rejects general-purpose IPC input', () => {
  expect(() => requireEmptyRequest({ input: {} })).not.toThrow();
  for (const value of [null, [], {}, { input: null }, { input: [] }, { input: { path: '/private' } }, { input: {}, method: 'GET' }]) expect(() => requireEmptyRequest(value)).toThrow();
});
it('serves the same packaged Web entry and typed assets with CSP', async () => {
  const root = await fixture();
  const entry = await appResource(root, APP_URL, 'GET');
  expect(await entry.text()).toBe('<main>Existing Web</main>');
  expect(entry.headers.get('Content-Security-Policy')).toBe(CSP);
  expect(entry.headers.get('X-Content-Type-Options')).toBe('nosniff');
  const script = await appResource(root, APP_URL + 'assets/app.js', 'GET');
  expect(script.headers.get('Content-Type')).toContain('javascript');
  expect((await appResource(root, APP_URL, 'HEAD')).body).toBeNull();
});
it('refuses traversal, arbitrary files, nested HTML, proxies and symlink escape', async () => {
  const root = await fixture(); const external = await fixture();
  await writeFile(join(root, 'assets', 'frame.html'), '<script>untrusted</script>');
  await symlink(join(external, 'assets', 'app.js'), join(root, 'assets', 'outside.js'));
  for (const path of ['../private', 'assets/%2e%2e%2fprivate', 'assets/%5cprivate', 'assets/%00.js', 'assets/outside.js', 'assets/frame.html', 'api/v1/session', 'assets/app.js?path=secret', '.env', 'package.json']) {
    expect((await appResource(root, APP_URL + path, 'GET')).status, path).toBe(404);
  }
  expect((await appResource(root, APP_URL, 'POST')).status).toBe(404);
});
