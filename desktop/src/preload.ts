import { contextBridge, ipcRenderer } from 'electron';
import type { DesktopBridge } from '../../web/src/generated/desktop-ipc';

// Expose individual methods, never invoke/send, raw events or Electron APIs.
const bridge: Pick<DesktopBridge, 'capabilities' | 'chooseVault'> = Object.freeze({
  capabilities: (request: Parameters<DesktopBridge['capabilities']>[0]) => ipcRenderer.invoke('desktop:capabilities', request),
  chooseVault: (request: Parameters<DesktopBridge['chooseVault']>[0]) => ipcRenderer.invoke('desktop:chooseVault', request),
});
contextBridge.exposeInMainWorld('drivebridgeDesktop', bridge);
