import { contextBridge, ipcRenderer } from 'electron';
import type { DesktopBridge } from '../../web/src/generated/desktop-ipc';

// Expose individual methods, never invoke/send, raw events or Electron APIs.
// Every method is one declared contract method; none accepts a path, bearer or
// arbitrary argument bag.
type Exposed = Pick<DesktopBridge, 'capabilities' | 'chooseVault' | 'registerVault' | 'confirmBinding' | 'listSources' | 'readReadingState' | 'updateReadingState' | 'lookupReadingMutation' | 'listNotes' | 'searchNotes' | 'readDocument' | 'readLinks' | 'readEditorSource' | 'proposeEditorDraft' | 'releaseProtection'>;
const bridge: Exposed = Object.freeze({
  capabilities: (request: Parameters<DesktopBridge['capabilities']>[0]) => ipcRenderer.invoke('desktop:capabilities', request),
  registerVault: (request: Parameters<DesktopBridge['registerVault']>[0]) => ipcRenderer.invoke('desktop:registerVault', request),
  chooseVault: (request: Parameters<DesktopBridge['chooseVault']>[0]) => ipcRenderer.invoke('desktop:chooseVault', request),
  confirmBinding: (request: Parameters<DesktopBridge['confirmBinding']>[0]) => ipcRenderer.invoke('desktop:confirmBinding', request),
  readReadingState: (request: Parameters<DesktopBridge['readReadingState']>[0]) => ipcRenderer.invoke('desktop:readReadingState', request),
  updateReadingState: (request: Parameters<DesktopBridge['updateReadingState']>[0]) => ipcRenderer.invoke('desktop:updateReadingState', request),
  lookupReadingMutation: (request: Parameters<DesktopBridge['lookupReadingMutation']>[0]) => ipcRenderer.invoke('desktop:lookupReadingMutation', request),
  listSources: (request: Parameters<DesktopBridge['listSources']>[0]) => ipcRenderer.invoke('desktop:listSources', request),
  listNotes: (request: Parameters<DesktopBridge['listNotes']>[0]) => ipcRenderer.invoke('desktop:listNotes', request),
  searchNotes: (request: Parameters<DesktopBridge['searchNotes']>[0]) => ipcRenderer.invoke('desktop:searchNotes', request),
  readDocument: (request: Parameters<DesktopBridge['readDocument']>[0]) => ipcRenderer.invoke('desktop:readDocument', request),
  readLinks: (request: Parameters<DesktopBridge['readLinks']>[0]) => ipcRenderer.invoke('desktop:readLinks', request),
  readEditorSource: (request: Parameters<DesktopBridge['readEditorSource']>[0]) => ipcRenderer.invoke('desktop:readEditorSource', request),
  proposeEditorDraft: (request: Parameters<DesktopBridge['proposeEditorDraft']>[0]) => ipcRenderer.invoke('desktop:proposeEditorDraft', request),
  releaseProtection: (request: Parameters<DesktopBridge['releaseProtection']>[0]) => ipcRenderer.invoke('desktop:releaseProtection', request),
});
contextBridge.exposeInMainWorld('drivebridgeDesktop', bridge);
// A payload-free revocation signal can only clear local presentation state.
ipcRenderer.on('desktop:session-invalidated', () => {
  window.dispatchEvent(new Event('drivebridge:session-invalidated'));
});
