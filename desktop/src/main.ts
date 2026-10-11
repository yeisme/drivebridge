import { app, BrowserWindow, dialog, ipcMain, protocol, session, powerMonitor } from 'electron';
import squirrelStartup from 'electron-squirrel-startup';
import { HostClient } from './host-client';
import { HostLifetime } from './host-lifecycle';
import { join } from 'node:path';
import { APP_URL, appResource, isAppDocument, requireEmptyRequest, requireMainFrame } from './security';

protocol.registerSchemesAsPrivileged([{ scheme: 'drivebridge-app', privileges: { standard: true, secure: true, supportFetchAPI: true, corsEnabled: true } }]);
app.enableSandbox();

// Squirrel owns shortcut creation/removal. Installer launches never start a host.
if (squirrelStartup) app.quit();
else if (!app.requestSingleInstanceLock()) app.quit();
else {
  app.whenReady().then(async () => {
    const isolated = session.fromPartition('drivebridge-desktop');
    isolated.setPermissionRequestHandler((_contents, _permission, callback) => callback(false));
    isolated.setPermissionCheckHandler(() => false);
    isolated.on('will-download', event => event.preventDefault());
    await isolated.protocol.handle('drivebridge-app', request => appResource(join(app.getAppPath(), 'web'), request.url, request.method));
    const window = new BrowserWindow({
      width: 1440, height: 940, minWidth: 320, minHeight: 480, show: false,
      webPreferences: { session: isolated, preload: join(__dirname, 'preload.js'), sandbox: true, contextIsolation: true,
        nodeIntegration: false, webSecurity: true, webviewTag: false, allowRunningInsecureContent: false, devTools: !app.isPackaged },
    });
    window.webContents.setWindowOpenHandler(() => ({ action: 'deny' }));
    window.webContents.on('will-attach-webview', event => event.preventDefault());
    window.webContents.on('will-navigate', (event, url) => { if (!isAppDocument(url)) event.preventDefault(); });
    window.webContents.on('will-frame-navigate', event => { if (!event.isMainFrame || !isAppDocument(event.url)) event.preventDefault(); });
    window.webContents.on('will-redirect', event => event.preventDefault());
    const lifetime = new HostLifetime(() => HostClient.start(join(app.isPackaged ? process.resourcesPath : app.getAppPath(), 'host-bin')));
    const suspend = () => { void lifetime.stop().catch(() => { console.error('desktop_host_cleanup_failed'); }); };
    powerMonitor.on('suspend', suspend);
    let choosing = false;
    let quitting = false;
    app.on('before-quit', event => {
      if (quitting) return;
      event.preventDefault(); quitting = true;
      void lifetime.stop().catch(() => { console.error('desktop_host_cleanup_failed'); }).finally(() => app.quit());
    });
    ipcMain.handle('desktop:capabilities', async (event, request: unknown) => {
      requireMainFrame({ destroyed: window.isDestroyed() || event.sender.isDestroyed(), sameContents: event.sender === window.webContents,
        sameFrame: event.senderFrame === window.webContents.mainFrame, url: event.senderFrame?.url ?? '' });
      requireEmptyRequest(request);
      await lifetime.ready();
      return { data: { schema_version: 'drivebridge.desktop.v1alpha1', available: lifetime.current ? ['chooseVault'] : [] } };
    });
    ipcMain.handle('desktop:chooseVault', async (event, request: unknown) => {
      requireMainFrame({ destroyed: window.isDestroyed() || event.sender.isDestroyed(), sameContents: event.sender === window.webContents,
        sameFrame: event.senderFrame === window.webContents.mainFrame, url: event.senderFrame?.url ?? '' });
      requireEmptyRequest(request);
      const failure = (code: string) => ({ error: { origin: 'drivebridge', code, message: 'The source was not connected.', recovery: 'none', operation_id: null } });
      const host = lifetime.current;
      if (choosing || !host) return failure('owner_unavailable');
      choosing = true;
      try {
        const selected = await dialog.showOpenDialog(window, { title: '选择已有 Pinax vault', properties: ['openDirectory', 'dontAddToRecent'] });
        if (selected.canceled || selected.filePaths.length !== 1) return failure('native_selection_cancelled');
        if (lifetime.current !== host || window.isDestroyed() || event.senderFrame !== window.webContents.mainFrame || !isAppDocument(event.senderFrame?.url ?? '')) return failure('desktop_session_invalid');
        const data = await host.preview(selected.filePaths[0]);
        if (lifetime.current !== host || window.isDestroyed() || event.senderFrame !== window.webContents.mainFrame || !isAppDocument(event.senderFrame?.url ?? '')) return failure('desktop_session_invalid');
        return { data };
      } catch { return failure('owner_unavailable'); }
      finally { choosing = false; }
    });
    window.on('closed', () => { powerMonitor.removeListener('suspend', suspend); suspend(); ipcMain.removeHandler('desktop:capabilities'); ipcMain.removeHandler('desktop:chooseVault'); });
    window.once('ready-to-show', () => window.show());
    await window.loadURL(APP_URL);
  }).catch(() => { console.error('desktop_start_failed'); app.quit(); });
  app.on('window-all-closed', () => app.quit());
}
