import type { NotebookMode } from "./notebook-state";

export interface NotebookScope { actor: string; space: string; source: string }
export interface NotebookPreferences {
  splitRatio: number;
  leftOpen: boolean;
  rightOpen: boolean;
  leftWidth: number;
  rightWidth: number;
  readingWidth: "narrow" | "standard" | "wide";
  fontSize: number;
  motion: "system" | "reduced";
  modeByNote: Record<string, NotebookMode>;
}
type PreferenceStorage = Pick<Storage, "getItem" | "setItem" | "removeItem">;
export const defaultNotebookPreferences = (): NotebookPreferences => ({
  splitRatio: 0.5, leftOpen: true, rightOpen: true, leftWidth: 208, rightWidth: 224,
  readingWidth: "standard", fontSize: 16, motion: "system", modeByNote: {},
});

export function notebookPreferenceKey(scope: NotebookScope): string | undefined {
  if (!scope.actor || !scope.space || !scope.source) return undefined;
  return `drivebridge:notebook:v1:${JSON.stringify([scope.actor, scope.space, scope.source])}`;
}

// Explicit projection prevents drafts, credentials and unknown fields from
// accidentally becoming persistent when callers pass a larger session object.
function sanitize(value: unknown): NotebookPreferences {
  const defaults = defaultNotebookPreferences();
  if (!value || typeof value !== "object" || Array.isArray(value)) return defaults;
  const input = value as Record<string, unknown>;
  const number = (key: keyof NotebookPreferences, min: number, max: number) => {
    const n = input[key];
    return typeof n === "number" && Number.isFinite(n) ? Math.max(min, Math.min(max, n)) : defaults[key] as number;
  };
  const modes = input.modeByNote && typeof input.modeByNote === "object" && !Array.isArray(input.modeByNote) ? input.modeByNote as Record<string, unknown> : {};
  const modeByNote = Object.fromEntries(Object.entries(modes).filter(([, mode]) => mode === "reading" || mode === "live" || mode === "source").slice(-50)) as Record<string, NotebookMode>;
  return {
    splitRatio: number("splitRatio", 0.25, 0.75),
    leftWidth: number("leftWidth", 180, 360),
    rightWidth: number("rightWidth", 200, 480),
    fontSize: number("fontSize", 14, 24),
    leftOpen: typeof input.leftOpen === "boolean" ? input.leftOpen : defaults.leftOpen,
    rightOpen: typeof input.rightOpen === "boolean" ? input.rightOpen : defaults.rightOpen,
    readingWidth: input.readingWidth === "narrow" || input.readingWidth === "wide" ? input.readingWidth : "standard",
    motion: input.motion === "reduced" ? "reduced" : "system",
    modeByNote,
  };
}

export function readNotebookPreferences(storage: PreferenceStorage, scope: NotebookScope): NotebookPreferences {
  const key = notebookPreferenceKey(scope);
  if (!key) return defaultNotebookPreferences();
  try { return sanitize(JSON.parse(storage.getItem(key) ?? "null")); }
  catch { return defaultNotebookPreferences(); }
}

export function writeNotebookPreferences(storage: PreferenceStorage, scope: NotebookScope, preferences: NotebookPreferences): boolean {
  const key = notebookPreferenceKey(scope);
  if (!key) return false;
  try { storage.setItem(key, JSON.stringify(sanitize(preferences))); return true; }
  catch { return false; }
}

export function clearNotebookPreferences(storage: PreferenceStorage, scope: NotebookScope): boolean {
  const key = notebookPreferenceKey(scope);
  if (!key) return false;
  try { storage.removeItem(key); return true; }
  catch { return false; }
}
