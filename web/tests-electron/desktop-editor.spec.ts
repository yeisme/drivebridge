import { test, expect, _electron as electron, type ElectronApplication, type Page } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

const desktopDir = resolve(import.meta.dirname, '../../desktop');
const repo = resolve(desktopDir, '..');
const ORIGINAL = 'disposable original body';

type DesktopResult<T> = { data?: T; error?: { code: string } };

test.describe.configure({ mode: 'serial' });

test('real Electron binding, editor source, draft proposal and revocation', async () => {
  test.setTimeout(240_000);
  const work = mkdtempSync(join(tmpdir(), 'drivebridge-electron-'));
  const vault = join(work, 'vault');
  const otherVault = join(work, 'vault-b');
  const home = join(work, 'home');
  // Default owner state location for a GUI app: os.UserConfigDir()/drivebridge
  const state = join(home, 'drivebridge');
  const pinax = join(desktopDir, 'host-bin/pinax');
  const env = {
    ...process.env,
    HOME: home,
    USERPROFILE: home,
    XDG_CONFIG_HOME: home,
    XDG_CACHE_HOME: home,
    GIT_CONFIG_NOSYSTEM: '1',
    GIT_CONFIG_GLOBAL: '/dev/null',
  };
  execFileSync(pinax, ['init', vault, '--title', 'Desktop E2E', '--json'], { env, stdio: 'pipe' });
  execFileSync(pinax, ['note', 'add', 'E2E note', '--dir', 'index', '--stdin', '--vault', vault, '--json'], { env, input: ORIGINAL, stdio: ['pipe', 'pipe', 'pipe'] });
  execFileSync(pinax, ['init', otherVault, '--title', 'Second vault', '--json'], { env, stdio: 'pipe' });
  execFileSync(pinax, ['note', 'add', 'Other vault note', '--dir', 'index', '--stdin', '--vault', otherVault, '--json'], { env, input: 'other vault body', stdio: ['pipe', 'pipe', 'pipe'] });
  // Disposable, fenced owner state registered for both throwaway vaults.
  execFileSync('go', ['run', './desktop/e2e/fixture', state, `${vault}:vault`, `${otherVault}:vault-b`], {
    cwd: repo,
    env: { ...process.env, CGO_ENABLED: '0', GOFLAGS: '', GOWORK: 'off' },
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  let app: ElectronApplication | undefined;
  try {
    app = await electron.launch({
      executablePath: join(desktopDir, 'node_modules/electron/dist/electron'),
      args: [desktopDir],
      env: { ...env, DRIVEBRIDGE_CONFIG: '/nonexistent/desktop-e2e' },
    });
    // The native file picker is the only OS surface replaced here; every guard,
    // IPC check, host process and owner CAS call stays real.
    await app.evaluate(({ dialog }, selected) => {
      dialog.showOpenDialog = async () => ({ canceled: false, filePaths: [selected] });
    }, vault);
    const page: Page = await app.firstWindow();
    await page.waitForLoadState('domcontentloaded');
    const call = <T>(name: string, request: unknown) =>
      page.evaluate(
        async ([method, payload]) => (window as unknown as { drivebridgeDesktop: Record<string, (arg: unknown) => Promise<unknown>> }).drivebridgeDesktop[method](payload),
        [name, request] as const,
      ) as Promise<DesktopResult<T>>;

    const capabilities = await call<{ available: string[] }>('capabilities', { input: {} });
    expect(capabilities.data?.available).toContain('chooseVault');
    expect(capabilities.data?.available).toContain('proposeEditorDraft');

    const preview = await call<{ decision: string; preview_id: string; preview_digest: string; expected_guard_revision: number }>('chooseVault', { input: {} });
    expect(preview.error).toBeUndefined();
    expect(preview.data?.decision, JSON.stringify(preview.data)).toBe('ready');

    const confirmed = await call<{ binding: { binding_id: string; source_id: string; state: string; revision: number; protection_active: boolean }; operation_id: string }>('confirmBinding', {
      input: { preview_id: preview.data!.preview_id, preview_digest: preview.data!.preview_digest, expected_guard_revision: preview.data!.expected_guard_revision, confirmed: true },
      idempotency_key: 'electron-confirm-1',
    });
    expect(confirmed.error).toBeUndefined();
    expect(confirmed.data?.binding.state).toBe('active');
    expect(confirmed.data?.binding.protection_active).toBe(true);
    const sourceID = confirmed.data!.binding.source_id;

    const notes = await call<{ items: { object_id: string; revision: string; title: string }[] }>('listNotes', { input: {}, source_id: sourceID });
    expect(notes.error).toBeUndefined();
    const note = notes.data!.items.find(item => item.title === 'E2E note');
    expect(note).toBeTruthy();

    const source = await call<{ object_id: string; revision: string; body: string; bytes: number }>('readEditorSource', {
      input: { object_id: note!.object_id, revision: note!.revision },
      source_id: sourceID,
    });
    expect(source.error).toBeUndefined();
    expect(source.data?.body).toContain(ORIGINAL);
    expect(source.data?.bytes).toBeGreaterThan(0);

    const proposal = await call<{ object_id: string; revision: string; preview_digest: string; state: string }>('proposeEditorDraft', {
      input: { object_id: note!.object_id, revision: note!.revision, body: ORIGINAL + '\n\nappended by the desktop draft' },
      source_id: sourceID,
    });
    expect(proposal.error).toBeUndefined();
    expect(proposal.data?.preview_digest).toMatch(/^sha256:[a-f0-9]{64}$/);

    // Revision conflict: a draft pinned to a revision the owner no longer has is
    // rejected with a stable owner code, never silently applied.
    const stale = await call<unknown>('proposeEditorDraft', {
      input: { object_id: note!.object_id, revision: 'sha256:' + '0'.repeat(64), body: 'stale draft' },
      source_id: sourceID,
    });
    expect(stale.error?.code).toBeTruthy();
    expect(stale.data).toBeUndefined();

    // Restart read-back: a new Electron process re-binds the same vault and the
    // source is still the untouched original, so a proposal never half-applies.
    await app.close();
    app = await electron.launch({
      executablePath: join(desktopDir, 'node_modules/electron/dist/electron'),
      args: [desktopDir],
      env: { ...env, DRIVEBRIDGE_CONFIG: '/nonexistent/desktop-e2e' },
    });
    await app.evaluate(({ dialog }, selected) => {
      dialog.showOpenDialog = async () => ({ canceled: false, filePaths: [selected] });
    }, vault);
    const restarted: Page = await app.firstWindow();
    await restarted.waitForLoadState('domcontentloaded');
    const restartedCall = <T>(name: string, request: unknown) =>
      restarted.evaluate(
        async ([method, payload]) => (window as unknown as { drivebridgeDesktop: Record<string, (arg: unknown) => Promise<unknown>> }).drivebridgeDesktop[method](payload),
        [name, request] as const,
      ) as Promise<DesktopResult<T>>;
    // capabilities awaits host readiness; without it the first IPC call races the
    // host start and would report an unavailable owner.
    const reopenCaps = await restartedCall<{ available: string[] }>('capabilities', { input: {} });
    expect(reopenCaps.data?.available).toContain('confirmBinding');
    const reopenPreview = await restartedCall<{ decision: string; preview_id: string; preview_digest: string; expected_guard_revision: number }>('chooseVault', { input: {} });
    expect(reopenPreview.data?.decision, JSON.stringify(reopenPreview)).toBe('ready');
    expect(reopenPreview.data?.expected_guard_revision).toBeGreaterThan(0);
    const reopenConfirmed = await restartedCall<{ binding: { binding_id: string; source_id: string; state: string; revision: number } }>('confirmBinding', {
      input: { preview_id: reopenPreview.data!.preview_id, preview_digest: reopenPreview.data!.preview_digest, expected_guard_revision: reopenPreview.data!.expected_guard_revision, confirmed: true },
      idempotency_key: 'electron-confirm-restart',
    });
    expect(reopenConfirmed.error).toBeUndefined();
    expect(reopenConfirmed.data?.binding.state).toBe('active');
    const reopenSource = reopenConfirmed.data!.binding.source_id;
    const reopenNotes = await restartedCall<{ items: { object_id: string; revision: string; title: string }[] }>('listNotes', { input: {}, source_id: reopenSource });
    const reopenNote = reopenNotes.data!.items.find(item => item.title === 'E2E note');
    expect(reopenNote).toBeTruthy();
    const reopenBody = await restartedCall<{ body: string }>('readEditorSource', {
      input: { object_id: reopenNote!.object_id, revision: reopenNote!.revision },
      source_id: reopenSource,
    });
    expect(reopenBody.error).toBeUndefined();
    expect(reopenBody.data?.body).toContain(ORIGINAL);
    expect(reopenBody.data?.body).not.toContain('appended by the desktop draft');

    // Cross-vault identity: a second vault binds into its own source, and a note
    // identity from the first vault is not readable through it.
    await app.evaluate(({ dialog }, selected) => {
      dialog.showOpenDialog = async () => ({ canceled: false, filePaths: [selected] });
    }, otherVault);
    const secondPreview = await restartedCall<{ decision: string; preview_id: string; preview_digest: string; expected_guard_revision: number }>('chooseVault', { input: {} });
    expect(secondPreview.data?.decision).toBe('ready');
    const secondConfirmed = await restartedCall<{ binding: { binding_id: string; source_id: string; state: string } }>('confirmBinding', {
      input: { preview_id: secondPreview.data!.preview_id, preview_digest: secondPreview.data!.preview_digest, expected_guard_revision: secondPreview.data!.expected_guard_revision, confirmed: true },
      idempotency_key: 'electron-confirm-2',
    });
    expect(secondConfirmed.error).toBeUndefined();
    expect(secondConfirmed.data?.binding.state).toBe('active');
    const secondSource = secondConfirmed.data!.binding.source_id;
    expect(secondSource).not.toBe(reopenSource);
    const crossVault = await restartedCall<unknown>('readEditorSource', {
      input: { object_id: reopenNote!.object_id, revision: reopenNote!.revision },
      source_id: secondSource,
    });
    expect(crossVault.data).toBeUndefined();
    expect(crossVault.error?.code).toBeTruthy();
    const otherNotes = await restartedCall<{ items: { title: string }[] }>('listNotes', { input: {}, source_id: secondSource });
    expect(otherNotes.data?.items.some(item => item.title === 'E2E note')).toBe(false);
    expect(otherNotes.data?.items.some(item => item.title === 'Other vault note')).toBe(true);

    // Lost response: replaying the release with the original key must return the
    // same operation identity instead of a second release.
    const releaseRequest = {
      input: { expected_revision: reopenConfirmed.data!.binding.revision, confirmed: true },
      binding_id: reopenConfirmed.data!.binding.binding_id,
      idempotency_key: 'electron-release-1',
    };
    const released = await restartedCall<{ operation_id: string; binding: { state: string; protection_active: boolean } }>('releaseProtection', releaseRequest);
    expect(released.error).toBeUndefined();
    expect(released.data?.binding.state).toBe('released');
    expect(released.data?.binding.protection_active).toBe(false);
    const replay = await restartedCall<{ operation_id: string; binding: { state: string } }>('releaseProtection', releaseRequest);
    expect(replay.error).toBeUndefined();
    expect(replay.data?.operation_id).toBe(released.data?.operation_id);
    expect(replay.data?.binding.state).toBe('released');

    // Revoked: the released source can no longer read.
    const revoked = await restartedCall<unknown>('listNotes', { input: {}, source_id: reopenSource });
    expect(revoked.error?.code).toBe('source_unknown');
  } finally {
    await app?.close().catch(() => {});
  }
});
