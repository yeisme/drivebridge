// Old-service compatibility and UI-only recovery metadata for the
// information workspace.
//
// The client consumes whatever the current deployment actually returns:
// responses may predate new optional fields, may come from routes that do not
// exist yet, or may carry unrecognized enum values. Normalizers below keep
// only what is present, never invent values, and return null for payloads
// that cannot be interpreted so callers can render an honest error state.
//
// Local state is limited to UI temporaries (filters, layout, unsent drafts).
// No domain data, credentials, tokens, or file bodies are persisted here.

import type {
  AnalyticsQueryResult,
  ContentBinding,
  FileQueryResult,
  FileThread,
  PanelId,
  WorkspaceFilters,
} from "./model";
import { PANELS } from "./model";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function asString(value: unknown): string | undefined {
  return typeof value === "string" && value.length > 0 ? value : undefined;
}

function asNumber(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

// Analytics results: every field except rows/observed_at/coverage is optional
// per the backend contract, and unrecognized coverage strings pass through so
// the UI can show the server's own words instead of guessing a caliber. The
// backend marshals an empty aggregation as "rows":null — an honestly empty
// observation, not an invalid response.
export function normalizeAnalyticsResult(raw: unknown): AnalyticsQueryResult | null {
  if (!isRecord(raw)) return null;
  const rows = Array.isArray(raw.rows) ? raw.rows : raw.rows == null ? [] : null;
  if (!rows) return null;
  const clean: Record<string, string | number | null>[] = [];
  for (const row of rows) {
    if (!isRecord(row)) continue;
    const entry: Record<string, string | number | null> = {};
    for (const [key, value] of Object.entries(row)) {
      if (typeof value === "string" || typeof value === "number" || value === null)
        entry[key] = value;
    }
    clean.push(entry);
  }
  const observed = asString(raw.observed_at);
  const coverage = asString(raw.coverage) || asString(raw.observed_coverage);
  if (!observed) return null;
  return {
    backend: asString(raw.backend),
    generation: asString(raw.generation),
    source_watermark: asNumber(raw.source_watermark),
    applied_watermark: asNumber(raw.applied_watermark),
    authorization_epoch: asNumber(raw.authorization_epoch),
    observed_at: observed,
    coverage: coverage || "unknown",
    partial: typeof raw.partial === "boolean" ? raw.partial : undefined,
    rows: clean,
    next_cursor: asString(raw.next_cursor) || null,
  };
}

// File query results: total_count may be null when the exact scope was not
// fully computed; that must surface as a partial claim, never as zero.
export function normalizeFileQueryResult(raw: unknown): FileQueryResult | null {
  if (!isRecord(raw) || !Array.isArray(raw.items)) return null;
  const items: { file_id: string; version_id: string }[] = [];
  for (const item of raw.items) {
    if (!isRecord(item)) continue;
    const file_id = asString(item.file_id);
    const version_id = asString(item.version_id);
    if (file_id && version_id) items.push({ file_id, version_id });
  }
  return {
    mode: asString(raw.mode) || "exact",
    items,
    catalogue_revision: asString(raw.catalogue_revision),
    next_cursor: asString(raw.next_cursor) || null,
    coverage: asString(raw.coverage),
    total_count:
      raw.total_count === null || raw.total_count === undefined
        ? null
        : asNumber(raw.total_count) ?? null,
  };
}

export function normalizeThreadList(raw: unknown): FileThread[] {
  const list = Array.isArray(raw) ? raw : isRecord(raw) && Array.isArray(raw.threads) ? raw.threads : [];
  const out: FileThread[] = [];
  for (const item of list) {
    if (!isRecord(item)) continue;
    const thread_id = asString(item.thread_id);
    const file_id = asString(item.file_id);
    // version_id:"" is the contract's first-class file-level anchor, not a
    // malformed row; only a non-string value is unusable.
    const version_id = typeof item.version_id === "string" ? item.version_id : undefined;
    if (!thread_id || !file_id || version_id === undefined) continue;
    const revision = asNumber(item.revision);
    out.push({
      thread_id,
      file_id,
      version_id,
      revision: revision === undefined ? 0 : revision,
      state: asString(item.state) || "unknown",
      author_kind: asString(item.author_kind) || "unknown",
    });
  }
  return out;
}

export function normalizeBindingList(raw: unknown): ContentBinding[] {
  const list = Array.isArray(raw) ? raw : isRecord(raw) && Array.isArray(raw.bindings) ? raw.bindings : [];
  const out: ContentBinding[] = [];
  for (const item of list) {
    if (!isRecord(item)) continue;
    // Implemented ContentBindingView: binding_id/file_ref/state. Items
    // without a binding identity or state cannot be interpreted and are
    // dropped rather than guessed at.
    const binding_id = asString(item.binding_id);
    const state = asString(item.state);
    if (!binding_id || !state) continue;
    out.push({
      binding_id,
      file_ref: asString(item.file_ref) ?? "",
      project_ref: asString(item.project_ref),
      purpose: asString(item.purpose),
      state,
      error: asString(item.error),
    });
  }
  return out;
}

// ---------------------------------------------------------------------------
// Workspace draft: versioned UI-only recovery metadata.
// ---------------------------------------------------------------------------

export interface WorkspaceDraft {
  version: 2;
  filters: WorkspaceFilters;
  panel?: PanelId;
  // A pinned file reference the user had open; migration must preserve it.
  selected_file_ref?: string;
  // Unsent discussion draft text (plain text, length-capped).
  discussion?: string;
}

export const DRAFT_KEY = "drivebridge:info-workspace:draft";
const DISCUSSION_MAX = 10000; // Matches the thread body contract cap.

function validDay(value: unknown): string {
  return typeof value === "string" && /^\d{4}-\d{2}-\d{2}$/.test(value) ? value : "";
}

function readFilters(raw: Record<string, unknown>): WorkspaceFilters {
  const space = typeof raw.space_id === "string" && /^[A-Za-z0-9._~:-]{1,200}$/.test(raw.space_id)
    ? raw.space_id
    : "";
  return {
    space_id: space,
    from: validDay(raw.from),
    to: validDay(raw.to),
  };
}

// Migrate any historical draft shape to the current version. Version 1 drafts
// (plain filter object without a version key) keep their references; drafts
// from an unknown future version are discarded rather than guessed at.
export function migrateWorkspaceDraft(raw: unknown): WorkspaceDraft | null {
  if (!isRecord(raw)) return null;
  // Version 1 drafts carry filter fields flat; version 2 nests them under
  // `filters`. Merge both so no historical reference is lost.
  const nested = isRecord(raw.filters) ? (raw.filters as Record<string, unknown>) : {};
  const filters = readFilters({ ...raw, ...nested });
  const draft: WorkspaceDraft = { version: 2, filters };
  const panel = typeof raw.panel === "string" ? raw.panel : "";
  if (PANELS.some((p) => p.id === panel)) draft.panel = panel as PanelId;
  if (typeof raw.selected_file_ref === "string" && /^[A-Za-z0-9._~:\/-]{1,500}$/.test(raw.selected_file_ref))
    draft.selected_file_ref = raw.selected_file_ref;
  if (typeof raw.discussion === "string" && raw.discussion.length > 0)
    draft.discussion = raw.discussion.slice(0, DISCUSSION_MAX);
  const version = raw.version;
  if (version !== undefined && version !== 2) return null;
  return draft;
}

export function readDraft(storage: Pick<Storage, "getItem">): WorkspaceDraft | null {
  try {
    const raw: unknown = JSON.parse(storage.getItem(DRAFT_KEY) || "");
    return migrateWorkspaceDraft(raw);
  } catch {
    return null;
  }
}

export function writeDraft(
  storage: Pick<Storage, "setItem" | "removeItem">,
  draft: WorkspaceDraft,
): void {
  try {
    storage.setItem(DRAFT_KEY, JSON.stringify(draft));
  } catch {
    // Local drafts are best-effort; quota failures must not break the page.
  }
}

export function clearDraft(storage: Pick<Storage, "removeItem">): void {
  try {
    storage.removeItem(DRAFT_KEY);
  } catch {
    /* Nothing to recover from. */
  }
}

// ---------------------------------------------------------------------------
// Revision-scoped cache.
// ---------------------------------------------------------------------------

// Panel observations are cached per data-context revision. Switching tenant or
// space bumps the revision, which invalidates every entry: a late response
// from the old context can never be served as if it belonged to the new one.
export class RevisionCache<T> {
  private entries = new Map<string, { revision: number; value: T }>();

  get(revision: number, key: string): T | undefined {
    const entry = this.entries.get(key);
    return entry && entry.revision === revision ? entry.value : undefined;
  }

  put(revision: number, key: string, value: T): void {
    // Drop entries from other revisions so the cache cannot grow across
    // context switches while keeping cross-context data reachable.
    for (const [k, entry] of this.entries)
      if (entry.revision !== revision) this.entries.delete(k);
    this.entries.set(key, { revision, value });
  }

  drop(key: string): void {
    this.entries.delete(key);
  }

  clear(): void {
    this.entries.clear();
  }
}
