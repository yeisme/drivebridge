// Private view state only. Mount a fresh session for each actor/space/source.
// Draft bodies stay in memory and must never be serialized as preferences.
export interface NoteIdentity {
  sourceId: string;
  objectId: string;
  revision: string;
}
export type Pane = "primary" | "secondary";
export interface NotebookTab {
  key: string;
  ref: NoteIdentity;
  pinned: boolean;
  dirty: boolean;
}
export interface PanePosition {
  scrollTop: number;
  selection?: { from: number; to: number };
}
export interface NotebookState {
  tabs: NotebookTab[];
  activeTab?: string;
  secondTab?: string;
  split: boolean;
  splitRatio: number;
  closed: NotebookTab[];
  drafts: Record<string, string>;
  positions: Record<Pane, Record<string, PanePosition>>;
}
export const noteKey = (ref: NoteIdentity) => JSON.stringify([ref.sourceId, ref.objectId, ref.revision]);
export const createNotebookState = (): NotebookState => ({
  tabs: [], split: false, splitRatio: 0.5, closed: [], drafts: {},
  positions: { primary: {}, secondary: {} },
});
export type NotebookAction =
  | { type: "open"; ref: NoteIdentity; pinned?: boolean }
  | { type: "activate"; key: string; pane?: Pane }
  | { type: "pin"; key: string; pinned: boolean }
  | { type: "draft"; key: string; body: string }
  | { type: "saved"; key: string; ref: NoteIdentity }
  | { type: "close"; key: string; discard?: boolean }
  | { type: "reopen" }
  | { type: "split"; key?: string }
  | { type: "cycle"; direction: 1 | -1 }
  | { type: "ratio"; value: number }
  | { type: "position"; pane: Pane; key: string; scrollTop: number; selection?: PanePosition["selection"] }
  | { type: "retain"; keys: string[] }
  | { type: "revoke"; sourceId: string };

function removeTabs(state: NotebookState, keys: Set<string>): NotebookState {
  const tabs = state.tabs.filter(tab => !keys.has(tab.key));
  const filter = <T,>(values: Record<string, T>) => Object.fromEntries(Object.entries(values).filter(([key]) => !keys.has(key)));
  const secondTab = state.secondTab && !keys.has(state.secondTab) ? state.secondTab : undefined;
  return {
    ...state, tabs,
    activeTab: state.activeTab && !keys.has(state.activeTab) ? state.activeTab : tabs.at(-1)?.key,
    secondTab, split: state.split && !!secondTab,
    drafts: filter(state.drafts),
    positions: { primary: filter(state.positions.primary), secondary: filter(state.positions.secondary) },
  };
}

export function notebookReducer(state: NotebookState, action: NotebookAction): NotebookState {
  switch (action.type) {
    case "open": {
      const key = noteKey(action.ref);
      const existing = state.tabs.find(tab => tab.key === key);
      if (existing) return { ...state, activeTab: key, tabs: state.tabs.map(tab => tab.key === key ? { ...tab, pinned: tab.pinned || !!action.pinned } : tab) };
      const preview = state.tabs.find(tab => !tab.pinned && !tab.dirty && tab.key !== state.secondTab);
      const base = preview ? removeTabs(state, new Set([preview.key])) : state;
      const tab = { key, ref: { ...action.ref }, pinned: !!action.pinned, dirty: false };
      return { ...base, tabs: [...base.tabs, tab], activeTab: key };
    }
    case "activate":
      if (!state.tabs.some(tab => tab.key === action.key)) return state;
      return action.pane === "secondary" ? { ...state, secondTab: action.key, split: true } : { ...state, activeTab: action.key };
    case "pin":
      return { ...state, tabs: state.tabs.map(tab => tab.key === action.key ? { ...tab, pinned: action.pinned || tab.dirty } : tab) };
    case "draft":
      if (!state.tabs.some(tab => tab.key === action.key)) return state;
      return { ...state, drafts: { ...state.drafts, [action.key]: action.body }, tabs: state.tabs.map(tab => tab.key === action.key ? { ...tab, dirty: true, pinned: true } : tab) };
    case "saved": {
      const tab = state.tabs.find(item => item.key === action.key);
      if (!tab || tab.ref.sourceId !== action.ref.sourceId || tab.ref.objectId !== action.ref.objectId || !action.ref.revision) return state;
      const key = noteKey(action.ref);
      const base = removeTabs(state, new Set([action.key]));
      const existing = base.tabs.find(item => item.key === key);
      const saved = { key, ref: { ...action.ref }, dirty: false, pinned: true };
      const preservePosition = (pane: Pane) => ({ ...base.positions[pane],
        [key]: base.positions[pane][key] ?? state.positions[pane][action.key] ?? { scrollTop: 0 } });
      return { ...base, tabs: existing ? base.tabs : state.tabs.map(item => item.key === action.key ? saved : item),
        activeTab: state.activeTab === action.key ? key : base.activeTab,
        secondTab: state.secondTab === action.key ? key : base.secondTab, split: state.split,
        positions: { primary: preservePosition("primary"), secondary: preservePosition("secondary") } };
    }
    case "close": {
      const tab = state.tabs.find(tab => tab.key === action.key);
      if (!tab || (tab.dirty && !action.discard)) return state;
      return { ...removeTabs(state, new Set([action.key])), closed: [...state.closed.filter(item => item.key !== tab.key), { ...tab, dirty: false }].slice(-20) };
    }
    case "reopen": {
      const tab = state.closed.at(-1);
      if (!tab) return state;
      return notebookReducer({ ...state, closed: state.closed.slice(0, -1) }, { type: "open", ref: tab.ref, pinned: true });
    }
    case "split": {
      const key = action.key ?? state.activeTab;
      if (!key || !state.tabs.some(tab => tab.key === key)) return state;
      return { ...state, split: !state.split, secondTab: state.split ? undefined : key };
    }
    case "cycle": {
      if (!state.tabs.length) return state;
      const index = state.tabs.findIndex(tab => tab.key === state.activeTab);
      return { ...state, activeTab: state.tabs[(index + action.direction + state.tabs.length) % state.tabs.length].key };
    }
    case "ratio":
      return Number.isFinite(action.value) ? { ...state, splitRatio: Math.max(0.25, Math.min(0.75, action.value)) } : state;
    case "position":
      if (!state.tabs.some(tab => tab.key === action.key) || !Number.isFinite(action.scrollTop)) return state;
      return { ...state, positions: { ...state.positions, [action.pane]: { ...state.positions[action.pane], [action.key]: { scrollTop: Math.max(0, action.scrollTop), selection: action.selection } } } };
    case "retain": {
      const allowed = new Set(action.keys);
      const removed = new Set(state.tabs.filter(tab => !allowed.has(tab.key)).map(tab => tab.key));
      if (!removed.size && state.closed.every(tab => allowed.has(tab.key))) return state;
      return { ...removeTabs(state, removed), closed: state.closed.filter(tab => allowed.has(tab.key)) };
    }
    case "revoke": {
      const keys = new Set(state.tabs.filter(tab => tab.ref.sourceId === action.sourceId).map(tab => tab.key));
      return { ...removeTabs(state, keys), closed: state.closed.filter(tab => tab.ref.sourceId !== action.sourceId) };
    }
  }
}
