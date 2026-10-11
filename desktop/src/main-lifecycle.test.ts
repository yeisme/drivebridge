import { EventEmitter } from 'node:events';
import { beforeEach, expect, it, vi } from 'vitest';
const fixture = vi.hoisted(() => ({
  squirrelStartup: false,
  start: vi.fn(), dialog: vi.fn(), app: undefined as unknown as EventEmitter & Record<string, any>,
  power: undefined as unknown as EventEmitter,
  window: undefined as unknown as EventEmitter & Record<string, any>,
  handlers: new Map<string, (...args: any[]) => Promise<any>>(),
}));
vi.mock('./host-client', () => ({ HostClient: { start: fixture.start } }));
vi.mock('electron-squirrel-startup', () => ({ get default() { return fixture.squirrelStartup; } }));
vi.mock('./security', () => ({ APP_URL: 'drivebridge-app://app/', appResource: vi.fn(), isAppDocument: () => true, requireEmptyRequest: vi.fn(), requireMainFrame: vi.fn() }));
vi.mock('electron', () => ({
  get app() { return fixture.app; }, get powerMonitor() { return fixture.power; },
  BrowserWindow: class { constructor() { return fixture.window; } },
  dialog: { showOpenDialog: fixture.dialog },
  ipcMain: { handle: (name: string, handler: (...args: any[]) => Promise<any>) => fixture.handlers.set(name, handler), removeHandler: (name: string) => fixture.handlers.delete(name) },
  protocol: { registerSchemesAsPrivileged: vi.fn() },
  session: { fromPartition: () => ({ setPermissionRequestHandler: vi.fn(), setPermissionCheckHandler: vi.fn(), on: vi.fn(), protocol: { handle: vi.fn(async () => {}) } }) },
}));
beforeEach(() => {
  vi.resetModules(); fixture.handlers.clear(); fixture.start.mockReset(); fixture.dialog.mockReset();
  fixture.squirrelStartup = false;
  fixture.app = Object.assign(new EventEmitter(), { enableSandbox: vi.fn(), requestSingleInstanceLock: vi.fn(() => true), whenReady: vi.fn(async () => {}), getAppPath: () => '/synthetic-app', isPackaged: false, quit: vi.fn() });
  fixture.power = new EventEmitter();
  const frame = { url: 'drivebridge-app://app/' };
  const contents = Object.assign(new EventEmitter(), { mainFrame: frame, isDestroyed: () => false, setWindowOpenHandler: vi.fn() });
  fixture.window = Object.assign(new EventEmitter(), { webContents: contents, isDestroyed: () => false, show: vi.fn(), loadURL: vi.fn(async () => {}) });
});
it('quits installer event launches before acquiring the lock, opening a window or starting a host', async () => {
  // Re-import intentionally exercises main's startup boundary after resetting mocks.
  fixture.squirrelStartup = true;
  await import('./main');
  expect(fixture.app.quit).toHaveBeenCalledTimes(1);
  expect(fixture.app.requestSingleInstanceLock).not.toHaveBeenCalled();
  expect(fixture.app.whenReady).not.toHaveBeenCalled();
  expect(fixture.window.loadURL).not.toHaveBeenCalled();
  expect(fixture.start).not.toHaveBeenCalled();
  expect(fixture.handlers.size).toBe(0);
  expect(fixture.app.enableSandbox).toHaveBeenCalledTimes(1);
});
it.each(['dialog', 'preview'])('suspend during %s fences the late result and resume does not start another host', async stage => {
  let release!: () => void;
  const pending = new Promise<void>(resolve => { release = resolve; });
  fixture.dialog.mockImplementation(async () => { if (stage === 'dialog') await pending; return { canceled: false, filePaths: ['/synthetic-vault'] }; });
  const host = { alive: true, close: vi.fn(async () => {}), preview: vi.fn(async () => { if (stage === 'preview') await pending; return { decision: 'blocked' }; }) };
  fixture.start.mockResolvedValue(host);
  await import('./main'); await vi.waitFor(() => expect(fixture.handlers.has('desktop:chooseVault')).toBe(true));
  const event = { sender: fixture.window.webContents, senderFrame: fixture.window.webContents.mainFrame };
  const capabilities = fixture.handlers.get('desktop:capabilities')!;
  expect((await capabilities(event, {})).data.available).toEqual(['chooseVault']);
  const result = fixture.handlers.get('desktop:chooseVault')!(event, {});
  await vi.waitFor(() => expect(stage === 'dialog' ? fixture.dialog : host.preview).toHaveBeenCalledTimes(1));
  fixture.power.emit('suspend');
  expect((await capabilities(event, {})).data.available).toEqual([]);
  release(); expect((await result).error.code).toBe('desktop_session_invalid');
  expect(host.preview).toHaveBeenCalledTimes(stage === 'dialog' ? 0 : 1);
  fixture.power.emit('resume'); expect((await capabilities(event, {})).data.available).toEqual([]);
  expect((await fixture.handlers.get('desktop:chooseVault')!(event, {})).error.code).toBe('owner_unavailable');
  const quitting = { preventDefault: vi.fn() }; fixture.app.emit('before-quit', quitting);
  await vi.waitFor(() => expect(fixture.app.quit).toHaveBeenCalledTimes(1));
  expect(quitting.preventDefault).toHaveBeenCalledTimes(1); expect(host.close).toHaveBeenCalledTimes(1); expect(fixture.start).toHaveBeenCalledTimes(1);
});
