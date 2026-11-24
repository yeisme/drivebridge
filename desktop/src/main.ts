import { app, BrowserWindow, dialog, ipcMain, protocol, session, powerMonitor, Menu } from 'electron';
import squirrelStartup from 'electron-squirrel-startup';
import { HostClient, desktopErrorOf } from './host-client';
import { HostLifetime } from './host-lifecycle';
import { validIPCRequest } from './ipc-validation';
import { join } from 'node:path';
import { randomUUID } from 'node:crypto';
import { APP_URL, appResource, isAppDocument, requireEmptyRequest, requireMainFrame } from './security';

protocol.registerSchemesAsPrivileged(['drivebridge-app', 'drivebridge-asset'].map(scheme => ({ scheme, privileges: { standard: true, secure: true, supportFetchAPI: true, corsEnabled: true } })));
app.enableSandbox();

// Squirrel owns shortcut creation/removal. Installer launches never start a host.
if (squirrelStartup) app.quit();
else if (!app.requestSingleInstanceLock()) app.quit();
else {
  app.whenReady().then(async () => {
    const states = new Map<string, BrowserWindow>();
    const lifetimes = new Set<HostLifetime<HostClient>>();
    type Handler = (event: Electron.IpcMainInvokeEvent, request: unknown) => Promise<unknown>;
    const windows = new Map<number, Map<string, Handler>>();
    const installed = new Set<string>();
    let quitting = false;
    app.on('before-quit', event => {
      if (quitting) return;
      event.preventDefault(); quitting = true;
      void Promise.allSettled([...lifetimes].map(lifetime => lifetime.stop())).then(() => app.quit());
    });
    const openWindow = async (statePath?: string, stateAction?: 'create' | 'migrate') => {
    const isolated = session.fromPartition('drivebridge-desktop-' + randomUUID());
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
    const lifetime = new HostLifetime(() => HostClient.start(join(app.isPackaged ? process.resourcesPath : app.getAppPath(), 'host-bin'), statePath, stateAction));
    lifetimes.add(lifetime);
    isolated.webRequest.onBeforeRequest({ urls: ['drivebridge-asset://*/*'] }, (details, callback) => {
      callback({ cancel: window.isDestroyed() || details.webContentsId !== window.webContents.id || details.frame !== window.webContents.mainFrame || !isAppDocument(details.frame?.url ?? '') || !['image', 'xhr'].includes(details.resourceType) });
    });
    await isolated.protocol.handle('drivebridge-asset', async request => {
      const denied = () => new Response('Unavailable', { status: 404, headers: { 'Cache-Control': 'no-store' } });
      try {
        const url = new URL(request.url), client = lifetime.current;
        if (!client || request.method !== 'GET' || url.hostname !== 'asset' || url.username || url.password || url.port || url.search || url.hash || !/^\/[a-f0-9]{64}$/.test(url.pathname)) return denied();
        const range = request.headers.get('Range') ?? undefined;
        if (range) { const parts = /^bytes=([0-9]+)-([0-9]+)$/.exec(range); if (!parts || !Number.isSafeInteger(Number(parts[1])) || !Number.isSafeInteger(Number(parts[2])) || Number(parts[2]) < Number(parts[1]) || Number(parts[2]) - Number(parts[1]) >= 8 * 1024 * 1024) return new Response('Invalid range', { status: 416 }); }
        return await lifetime.request(client, () => { if (window.isDestroyed() || !isAppDocument(window.webContents.getURL())) throw new Error('desktop_session_invalid'); }, () => client.asset(url.pathname.slice(1), range));
      } catch { return denied(); }
    });

    const handlers = new Map<string, Handler>();
    windows.set(window.webContents.id, handlers);
    const contentsID = window.webContents.id;
    const handle = (channel: string, handler: Handler) => {
      handlers.set(channel, handler);
      if (installed.has(channel)) return;
      installed.add(channel);
      ipcMain.handle(channel, (event, request: unknown) => {
        const current = windows.get(event.sender.id)?.get(channel);
        if (!current) throw new Error('desktop_sender_invalid');
        return current(event, request);
      });
    };
    const suspend = () => { pendingPreview = null; if (!window.isDestroyed()) window.webContents.send('desktop:session-invalidated'); void lifetime.stop().catch(() => { console.error('desktop_host_cleanup_failed'); }); };
    powerMonitor.on('suspend', suspend);
    let choosing = false;
    // The confirmed preview is main-owned session state: the renderer only echoes
    // its identity, so a renderer cannot manufacture a preview or revision.
    let pendingPreview: { preview_id: string; preview_digest: string; expected_guard_revision: number; decision: string } | null = null;
    handle('desktop:capabilities', async (event, request: unknown) => {
      requireMainFrame({ destroyed: window.isDestroyed() || event.sender.isDestroyed(), sameContents: event.sender === window.webContents,
        sameFrame: event.senderFrame === window.webContents.mainFrame, url: event.senderFrame?.url ?? '' });
      requireEmptyRequest(request);
      await lifetime.ready();
      const client = lifetime.current;
      const unavailable = { data: { schema_version: 'drivebridge.desktop.v1alpha1', available: [] as string[] } };
      if (!client) return unavailable;
      try {
        const data = await lifetime.request(client, () => guarded(event), () => client.capabilities());
        if (lifetime.current !== client) return unavailable;
        const installed = new Set(['capabilities', 'chooseVault', 'registerVault', 'confirmBinding', 'listSources', 'readReadingState', 'updateReadingState', 'lookupReadingMutation', 'listNotes', 'searchNotes', 'readDocument', 'readLinks', 'proposeEditorDraft', 'readEditorSource', 'releaseProtection']);
        return { data: { ...data, available: data.available.filter(name => installed.has(name)) } };
      } catch { return unavailable; }
    });
    const guarded = (event: Electron.IpcMainInvokeEvent) => requireMainFrame({ destroyed: window.isDestroyed() || event.sender.isDestroyed(), sameContents: event.sender === window.webContents,
      sameFrame: event.senderFrame === window.webContents.mainFrame, url: event.senderFrame?.url ?? '' });
    const failure = (code: string) => ({ error: { origin: 'drivebridge', code, message: 'The request was not completed.', recovery: 'none', operation_id: null } });
    const opaque = (value: unknown): value is string => typeof value === 'string' && /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(value);
    const revision = (value: unknown): value is string => typeof value === 'string' && /^sha256:[a-f0-9]{64}$/.test(value);
    const host = () => lifetime.current;
    // Propagate the owner's closed error value; only an uncoded throw collapses.
    const failed = (error: unknown) => error instanceof Error && error.message === 'desktop_session_invalid' ? failure('desktop_session_invalid') : ({ error: desktopErrorOf(error) ?? { origin: 'drivebridge', code: 'owner_unavailable', message: 'The request was not completed.', recovery: 'none', operation_id: null } });
    // chooseVault keeps a ready preview open for confirmation. A blocked preview
    // is still refused by the owner, and main never fabricates a binding.
    for (const method of ['chooseVault', 'registerVault'] as const) {
    handle('desktop:' + method, async (event, request: unknown) => {
      guarded(event); requireEmptyRequest(request);
      const client = host();
      if (!client || choosing) return failure('owner_unavailable');
      choosing = true;
      try {
        const selected = await dialog.showOpenDialog(window, { title: '选择已有 Pinax vault', properties: ['openDirectory', 'dontAddToRecent'] });
        if (selected.canceled || selected.filePaths.length !== 1) return failure('native_selection_cancelled');
        if (method === 'registerVault') {
          const confirmation = await dialog.showMessageBox(window, { type: 'question', title: '登记 Pinax 阅读来源', message: '将此目录登记为 Pinax 管理的本机来源？普通文件入口对该目录的写入会受到保护，笔记正文仍由 Pinax 管理。', buttons: ['取消', '登记来源'], defaultId: 0, cancelId: 0, noLink: true });
          if (confirmation.response !== 1) return failure('native_selection_cancelled');
        }

        if (host() !== client || window.isDestroyed() || event.senderFrame !== window.webContents.mainFrame) return failure('desktop_session_invalid');
        const data = await client.bindSource(selected.filePaths[0], method === 'registerVault');
        // A suspend or window change during the owner probe wins over the late
        // result: the fenced preview never reaches the renderer.
        if (host() !== client || window.isDestroyed() || event.senderFrame !== window.webContents.mainFrame || !isAppDocument(event.senderFrame?.url ?? '')) return failure('desktop_session_invalid');
        pendingPreview = data.decision === 'ready' ? data : null;
        return { data };
      } catch (error) { return failed(error); }
      finally { choosing = false; }
    });
    }
    handle('desktop:confirmBinding', async (event, request: unknown) => {
      guarded(event);
      if (!validIPCRequest('confirmBinding', request)) return failure('request_invalid');
      const client = host();
      const value = request as { input?: { preview_id?: unknown; preview_digest?: unknown; expected_guard_revision?: unknown; confirmed?: unknown }; idempotency_key?: unknown };
      if (!client || !opaque(value?.idempotency_key)) return failure('request_invalid');
      const input = value?.input;
      if (!input || Object.keys(input).sort().join() !== 'confirmed,expected_guard_revision,preview_digest,preview_id' || !opaque(input.preview_id) || !revision(input.preview_digest) || !Number.isSafeInteger(input.expected_guard_revision) || input.confirmed !== true) return failure('request_invalid');
      const preview = pendingPreview;
      if (!preview || preview.preview_id !== input.preview_id || preview.preview_digest !== input.preview_digest || preview.expected_guard_revision !== input.expected_guard_revision) return failure('binding_preview_expired');
      try {
        const data = await lifetime.request(client, () => guarded(event), () => client.confirmBinding(preview as never, value.idempotency_key as string));
        pendingPreview = null;
        return { data };
      } catch (error) { return failed(error); }
    });
    for (const method of ['readReadingState', 'updateReadingState', 'lookupReadingMutation'] as const) {
      handle('desktop:' + method, async (event, request: unknown) => {
        guarded(event); if (!validIPCRequest(method, request)) return failure('request_invalid');
        const client = host(); if (!client) return failure('owner_unavailable');
        const value = request as { input: never; idempotency_key?: string };
        try { return { data: await lifetime.request(client, () => guarded(event), () => method === 'updateReadingState' ? client.updateReadingState(value.input, value.idempotency_key!) : client[method](value.input)) }; }
        catch (error) { return failed(error); }
      });
    }
    handle('desktop:listSources', async (event, request: unknown) => {
      guarded(event);
      if (!validIPCRequest('listSources', request)) return failure('request_invalid');
      const client = host(); if (!client) return failure('owner_unavailable');
      const value = request as { input: { limit?: number; cursor?: string } };
      try { return { data: await lifetime.request(client, () => guarded(event), () => client.listSources(value.input)) }; } catch (error) { return failed(error); }
    });
    handle('desktop:listNotes', async (event, request: unknown) => {
      guarded(event);
      if (!validIPCRequest('listNotes', request)) return failure('request_invalid');
      const client = host();
      const value = request as { source_id?: unknown; input: { limit?: number; cursor?: string; sort?: string } };
      if (!client || !opaque(value?.source_id)) return failure('request_invalid');
      try { return { data: await lifetime.request(client, () => guarded(event), () => client.listNotes(value.source_id as string, value.input)) }; } catch (error) { return failed(error); }
    });
    handle('desktop:searchNotes', async (event, request: unknown) => {
      guarded(event); const client = host(); const value = request as { source_id?: unknown; input?: unknown };
      if (!validIPCRequest('searchNotes', request)) return failure('request_invalid');
      if (!client || !opaque(value?.source_id) || !value?.input || typeof value.input !== 'object') return failure('request_invalid');
      try { return { data: await lifetime.request(client, () => guarded(event), () => client.searchNotes(value.source_id as string, value.input as never)) }; } catch (error) { return failed(error); }
    });
    handle('desktop:readDocument', async (event, request: unknown) => {
      guarded(event); const client = host(); const value = request as { source_id?: unknown; input?: { object_id?: unknown; revision?: unknown; cursor?: unknown; anchor?: unknown } };
      if (!validIPCRequest('readDocument', request)) return failure('request_invalid');
      if (!client || !opaque(value?.source_id) || !opaque(value?.input?.object_id) || !revision(value?.input?.revision)) return failure('request_invalid');
      try { return { data: await lifetime.request(client, () => guarded(event), () => client.readDocument(value.source_id as string, value.input as never)) }; } catch (error) { return failed(error); }
    });
    handle('desktop:readLinks', async (event, request: unknown) => {
      guarded(event); const client = host(); const value = request as { source_id?: unknown; input?: { object_id?: unknown; revision?: unknown; direction?: unknown; limit?: unknown; cursor?: unknown } };
      if (!validIPCRequest('readLinks', request)) return failure('request_invalid');
      if (!client || !opaque(value?.source_id) || !opaque(value?.input?.object_id) || !revision(value?.input?.revision)) return failure('request_invalid');
      try { return { data: await lifetime.request(client, () => guarded(event), () => client.readLinks(value.source_id as string, value.input as never)) }; } catch (error) { return failed(error); }
    });
    handle('desktop:readEditorSource', async (event, request: unknown) => {
      guarded(event);
      if (!validIPCRequest('readEditorSource', request)) return failure('request_invalid');
      const client = host();
      const value = request as { source_id?: unknown; input?: { object_id?: unknown; revision?: unknown } };
      if (!client || !opaque(value?.source_id) || !opaque(value?.input?.object_id) || !revision(value?.input?.revision)) return failure('request_invalid');
      try { return { data: await lifetime.request(client, () => guarded(event), () => client.readEditorSource(value.source_id as string, value.input!.object_id as string, value.input!.revision as string)) }; }
      catch (error) { return failed(error); }
    });
    handle('desktop:proposeEditorDraft', async (event, request: unknown) => {
      guarded(event);
      if (!validIPCRequest('proposeEditorDraft', request)) return failure('request_invalid');
      const client = host();
      const value = request as { source_id?: unknown; input?: { object_id?: unknown; revision?: unknown; body?: unknown } };
      if (!client || !opaque(value?.source_id) || !opaque(value?.input?.object_id) || !revision(value?.input?.revision) || typeof value?.input?.body !== 'string' || (value.input.body as string).length > 65536) return failure('request_invalid');
      try { return { data: await lifetime.request(client, () => guarded(event), () => client.proposeEditorDraft(value.source_id as string, { object_id: value.input!.object_id as string, revision: value.input!.revision as string, body: value.input!.body as string })) }; }
      catch (error) { return failed(error); }
    });
    handle('desktop:releaseProtection', async (event, request: unknown) => {
      guarded(event);
      if (!validIPCRequest('releaseProtection', request)) return failure('request_invalid');
      const client = host();
      const value = request as { binding_id?: unknown; idempotency_key?: unknown; input?: { expected_revision?: unknown; confirmed?: unknown } };
      if (!client || !opaque(value?.binding_id) || !opaque(value?.idempotency_key) || !Number.isSafeInteger(value?.input?.expected_revision) || (value.input?.expected_revision as number) < 1 || value.input?.confirmed !== true) return failure('request_invalid');
      lifetime.invalidateRequests();
      try { return { data: await lifetime.request(client, () => guarded(event), () => client.releaseProtection(value.binding_id as string, value.input!.expected_revision as number, value.idempotency_key as string)) }; }
      catch (error) { return failed(error); }
    });
    window.on('closed', () => {
      pendingPreview = null; powerMonitor.removeListener('suspend', suspend); suspend();
      windows.delete(contentsID); handlers.clear();
      for (const [ref, current] of states) if (current === window) states.delete(ref);
      void lifetime.stop().finally(() => lifetimes.delete(lifetime)).catch(() => { console.error('desktop_host_cleanup_failed'); });
    });
    await lifetime.ready();
    if (window.isDestroyed()) return;
    const reference = lifetime.current?.stateRef;
    if (reference) {
      const existing = states.get(reference);
      if (existing && !existing.isDestroyed()) { window.close(); existing.show(); existing.focus(); return; }
      states.set(reference, window);
    }
    window.once('ready-to-show', () => window.show());
    await window.loadURL(APP_URL);
    };
    let selectingState = false;
    const prepareWorkspace = async (action: 'create' | 'migrate') => {
      if (selectingState) return; selectingState = true;
      try {
        let path: string | undefined;
        if (action === 'create') { const selected = await dialog.showSaveDialog({ title: '创建新的本机工作区目录', buttonLabel: '选择新目录' }); if (!selected.canceled) path = selected.filePath; }
        else { const selected = await dialog.showOpenDialog({ title: '迁移已有本机工作区', properties: ['openDirectory', 'dontAddToRecent'] }); if (!selected.canceled && selected.filePaths.length === 1) path = selected.filePaths[0]; }
        if (!path) return;
        const confirmation = await dialog.showMessageBox({ type: 'question', title: action === 'create' ? '创建本机工作区' : '迁移本机工作区', message: action === 'create' ? '在所选新目录创建独立本机状态？' : '将所选旧状态迁移为受管布局？原数据库快照会保留；存在运行中的进程时拒绝迁移。', buttons: ['取消', action === 'create' ? '创建' : '确认迁移'], defaultId: 0, cancelId: 0, noLink: true });
        if (confirmation.response === 1) await openWindow(path, action);
      } catch { dialog.showErrorBox('工作区准备失败', '所选状态未能完成准备。请检查原状态；不会自动创建替代状态。'); }
      finally { selectingState = false; }
    };
    Menu.setApplicationMenu(Menu.buildFromTemplate([{ label: '工作区', submenu: [
      { label: '新建本机工作区…', click: () => { void prepareWorkspace('create'); } },
      { label: '迁移已有工作区…', click: () => { void prepareWorkspace('migrate'); } },

      { label: '打开已有工作区…', accelerator: 'CmdOrCtrl+Shift+O', click: async () => {
        if (selectingState) return;
        selectingState = true;
        try {
          const selected = await dialog.showOpenDialog({ title: '选择已有 DriveBridge state', properties: ['openDirectory', 'dontAddToRecent'] });
          if (!selected.canceled && selected.filePaths.length === 1) await openWindow(selected.filePaths[0]);
        } catch { dialog.showErrorBox('无法打开工作区', '所选状态无法验证。未建立替代状态，请检查原状态或恢复备份。'); }
        finally { selectingState = false; }
      } }, { role: 'close' }, { role: 'quit' },
    ] }]));
    app.on('second-instance', () => { const window = BrowserWindow.getAllWindows()[0]; if (window) { window.show(); window.focus(); } });
    await openWindow();
  }).catch((error: unknown) => { const code = typeof (error as { code?: unknown })?.code === 'string' ? (error as { code: string }).code : 'unknown'; console.error('desktop_start_failed:' + code); app.quit(); });
  app.on('window-all-closed', () => app.quit());
}
