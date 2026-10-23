import type { DriveFile } from '../../api';

export type PreviewReaderState = {
  scrollTop?: number;
  zoom?: number;
  page?: number;
};

export type PreviewTab = {
  key: string;
  spaceId: string;
  fileId: string;
  versionId: string;
  title: string;
  path: string;
  pinned: boolean;
  readerState?: PreviewReaderState;
};

export type PreviewWorkspaceState = {
  tabs: PreviewTab[];
  activeTab?: string;
  rightPanelOpen: boolean;
  rightPanelMode: 'directory' | 'details';
  treeExpanded: string[];
};

export type PreviewAction =
  | { type: 'open'; file: DriveFile; pinned?: boolean }
  | { type: 'activate'; key: string }
  | { type: 'pin'; key: string; pinned: boolean }
  | { type: 'close'; key: string }
  | { type: 'closeOthers'; key: string }
  | { type: 'closeToRight'; key: string }
  | { type: 'closeAll' }
  | { type: 'reader'; key: string; state: PreviewReaderState }
  | { type: 'panel'; open: boolean }
  | { type: 'panelMode'; mode: PreviewWorkspaceState['rightPanelMode'] }
  | { type: 'tree'; path: string; expanded: boolean }
  | { type: 'retain'; files: DriveFile[] };

export const previewKey = (file: Pick<DriveFile, 'space_id' | 'id' | 'version_id'>) =>
  JSON.stringify([file.space_id, file.id, file.version_id]);

export const previewTabFromFile = (file: DriveFile, pinned = false): PreviewTab => ({
  key: previewKey(file),
  spaceId: file.space_id,
  fileId: file.id,
  versionId: file.version_id,
  title: file.path.split('/').pop() || file.path,
  path: file.path,
  pinned,
});

export const createPreviewWorkspaceState = (): PreviewWorkspaceState => ({
  tabs: [],
  rightPanelOpen: true,
  rightPanelMode: 'directory',
  treeExpanded: [''],
});

function removeTabs(state: PreviewWorkspaceState, keys: Set<string>): PreviewWorkspaceState {
  const tabs = state.tabs.filter(tab => !keys.has(tab.key));
  const activeTab = state.activeTab && !keys.has(state.activeTab)
    ? state.activeTab
    : tabs.at(-1)?.key;
  return { ...state, tabs, activeTab };
}

export function previewReducer(state: PreviewWorkspaceState, action: PreviewAction): PreviewWorkspaceState {
  switch (action.type) {
    case 'open': {
      const key = previewKey(action.file);
      const existing = state.tabs.find(tab => tab.key === key);
      if (existing) return { ...state, activeTab: key };
      const reusable = state.tabs.find(tab => !tab.pinned && tab.key !== state.activeTab);
      const base = reusable ? removeTabs(state, new Set([reusable.key])) : state;
      const tab = previewTabFromFile(action.file, !!action.pinned);
      return { ...base, tabs: [...base.tabs, tab], activeTab: tab.key };
    }
    case 'activate':
      return state.tabs.some(tab => tab.key === action.key) ? { ...state, activeTab: action.key } : state;
    case 'pin':
      return { ...state, tabs: state.tabs.map(tab => tab.key === action.key ? { ...tab, pinned: action.pinned } : tab) };
    case 'close':
      return removeTabs(state, new Set([action.key]));
    case 'closeOthers':
      return removeTabs(state, new Set(state.tabs.filter(tab => tab.key !== action.key).map(tab => tab.key)));
    case 'closeToRight': {
      const index = state.tabs.findIndex(tab => tab.key === action.key);
      if (index < 0) return state;
      return removeTabs(state, new Set(state.tabs.slice(index + 1).map(tab => tab.key)));
    }
    case 'closeAll':
      return removeTabs(state, new Set(state.tabs.filter(tab => !tab.pinned).map(tab => tab.key)));
    case 'reader':
      return { ...state, tabs: state.tabs.map(tab => tab.key === action.key ? { ...tab, readerState: { ...tab.readerState, ...action.state } } : tab) };
    case 'panel':
      return { ...state, rightPanelOpen: action.open };
    case 'panelMode':
      return { ...state, rightPanelMode: action.mode, rightPanelOpen: true };
    case 'tree': {
      const expanded = new Set(state.treeExpanded);
      if (action.expanded) expanded.add(action.path); else expanded.delete(action.path);
      return { ...state, treeExpanded: [...expanded] };
    }
    case 'retain': {
      const allowed = new Set(action.files.map(previewKey));
      return removeTabs(state, new Set(state.tabs.filter(tab => !allowed.has(tab.key)).map(tab => tab.key)));
    }
  }
}
