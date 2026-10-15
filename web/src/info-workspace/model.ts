// Information workspace client model.
//
// Shapes and behavior here are derived from the frozen OpenSpec contracts:
// this package's design.md (workspace view input/result samples, view states,
// panel set) plus the backend interface tables of drivebridge-analytics-search-v1
// (analytics/queries), drivebridge-file-organization-v1 (file-queries),
// drivebridge-content-retrieval-v1 (content-bindings) and
// drivebridge-version-discussion-v1 (file threads). No field, enum, or status
// code beyond those contracts is invented here; anything the server omits or
// sends unrecognized is surfaced honestly instead of being coerced.

export type ViewState =
  | "loading"
  | "ready"
  | "empty"
  | "partial"
  | "stale"
  | "forbidden"
  | "offline"
  | "unsupported"
  | "error";

// The six overview panel groups from the package design. `dataset` is the
// analytics dataset a panel drills into; `usage` panels are served by the
// existing (contract-preserved) usage endpoints, `analytics` panels require
// the new typed analytics capability and degrade honestly when it is absent.
export type PanelId =
  | "capacity"
  | "growth"
  | "transfer"
  | "connections"
  | "organization"
  | "agent-activity";

export type AnalyticsDataset =
  | "file_current"
  | "transfer_facts"
  | "audit_facts"
  | "capacity_daily";

export interface PanelDefinition {
  id: PanelId;
  label: string;
  dataset: AnalyticsDataset;
  source: "usage" | "analytics" | "spaces";
}

export const PANELS: PanelDefinition[] = [
  { id: "capacity", label: "容量", dataset: "capacity_daily", source: "usage" },
  { id: "growth", label: "增长", dataset: "capacity_daily", source: "usage" },
  { id: "transfer", label: "传输", dataset: "transfer_facts", source: "usage" },
  { id: "connections", label: "连接", dataset: "transfer_facts", source: "spaces" },
  { id: "organization", label: "整理", dataset: "file_current", source: "analytics" },
  { id: "agent-activity", label: "Agent 活动", dataset: "audit_facts", source: "usage" },
];

// Workspace-level filters. These mirror the design's workspace view input
// sample (space_id/from/to) and are the only values shared through the URL.
export interface WorkspaceFilters {
  space_id: string;
  from: string; // YYYY-MM-DD
  to: string; // YYYY-MM-DD
}
// ---------------------------------------------------------------------------
// Analytics query contract (backend drivebridge-analytics-search-v1).
// ---------------------------------------------------------------------------

export interface QueryFilter {
  field: string;
  op: "eq" | "contains";
  value: string;
}

export interface AnalyticsQueryRequest {
  dataset: AnalyticsDataset;
  filters: QueryFilter[];
  dimensions?: string[];
  metrics?: string[];
  sort?: { field: string; direction: "asc" | "desc" }[];
  limit?: number;
  cursor?: string | null;
  consistency?: "live" | "observed";
}

// The organization panel needs the precise current inventory, so it runs
// consistency:"live" (the analytics contract reserves observed for analytics;
// this deployment answers analytics_backend_unavailable there) and always
// carries the dataset's fixed selection vocabulary — empty dimensions+metrics
// is an invalid_query, not "give me everything".
export const ORGANIZATION_PANEL_ANALYTICS: Pick<
  AnalyticsQueryRequest,
  "dataset" | "dimensions" | "metrics" | "consistency"
> = {
  dataset: "file_current",
  dimensions: ["mime_type"],
  metrics: ["logical_bytes", "file_count"],
  consistency: "live",
};

export interface AnalyticsQueryResult {
  backend?: string;
  generation?: string;
  source_watermark?: number;
  applied_watermark?: number;
  authorization_epoch?: number;
  observed_at: string;
  // "complete" is the only coverage that may be presented as a full
  // observation; every other value (including unrecognized ones) renders as
  // partial with the raw label visible.
  coverage: string;
  partial?: boolean;
  rows: Record<string, string | number | null>[];
  next_cursor: string | null;
}

// ---------------------------------------------------------------------------
// File query contract (backend drivebridge-file-organization-v1).
// ---------------------------------------------------------------------------

export interface FileQueryRequest {
  mode: "exact";
  space_ids?: string[];
  filters?: (QueryFilter | { field: "path_prefix"; op: "prefix"; value: string })[];
  sort?: { field: string; direction: "asc" | "desc" }[];
  limit?: number;
  cursor?: string | null;
}

export interface FileQueryResult {
  mode: string;
  items: { file_id: string; version_id: string }[];
  catalogue_revision?: string;
  next_cursor: string | null;
  coverage?: string;
  total_count?: number | null;
}

// ---------------------------------------------------------------------------
// File detail side contracts.
// ---------------------------------------------------------------------------

// Thread shape from the version-discussion sample result.
export interface FileThread {
  thread_id: string;
  file_id: string;
  version_id: string;
  revision: number;
  state: string;
  author_kind: string;
}

// ContentBinding projects the content owner's state only
// (drivebridge-content-retrieval-v1 ContentBindingView: binding_id, file_ref,
// state not_processed|processing|searchable|failed|stale|unsupported, …);
// unrecognized state strings are displayed verbatim.
export interface ContentBinding {
  binding_id: string;
  file_ref: string;
  project_ref?: string;
  purpose?: string;
  state: string;
  error?: string;
}

// ---------------------------------------------------------------------------
// Pagination: DriveBridge new endpoints default to 50, cap at 200.
// ---------------------------------------------------------------------------

export const DEFAULT_PAGE_LIMIT = 50;
export const MAX_PAGE_LIMIT = 200;

export function clampLimit(limit?: number): number {
  if (limit === undefined || limit === null || !Number.isFinite(limit) || limit <= 0)
    return DEFAULT_PAGE_LIMIT;
  return Math.min(MAX_PAGE_LIMIT, Math.max(1, Math.floor(limit)));
}

// ---------------------------------------------------------------------------
// Drilldown: same caliber as the panel that launched it.
// ---------------------------------------------------------------------------

export interface DrilldownQuery {
  dataset: AnalyticsDataset;
  filters: QueryFilter[];
}

// Only field/op combinations that appear in the frozen backend samples are
// produced here. The workspace from/to window is intentionally NOT translated
// into analytics filters: no date filter field is defined by the contract, and
// fabricating one would silently change the observed caliber. Date windows are
// carried by the preserved usage endpoints, which do define them.
export function sameCaliberDrilldown(
  filters: WorkspaceFilters,
  dataset: AnalyticsDataset,
  selection?: { field: string; value: string },
): DrilldownQuery {
  const out: QueryFilter[] = [];
  if (filters.space_id)
    out.push({ field: "space_id", op: "eq", value: filters.space_id });
  if (selection && selection.value)
    out.push({ field: selection.field, op: "eq", value: selection.value });
  return { dataset, filters: out };
}

// Cursors are opaque and bound to the canonical query that produced them. The
// client never replays a cursor against a different query, sort, or identity
// scope; the server remains the authority, this is a client-side guard.
export function canonicalQueryKey(
  request: AnalyticsQueryRequest | FileQueryRequest,
): string {
  const space =
    "dataset" in request
      ? request.dataset
      : (request.space_ids || []).join(",");
  const filters = (request.filters || [])
    .map((f) => `${f.field}:${f.op}:${f.value}`)
    .sort()
    .join("|");
  const sort = (request.sort || [])
    .map((s) => `${s.field}:${s.direction}`)
    .join("|");
  return [space, filters, sort, clampLimit(request.limit)].join("#");
}

// ---------------------------------------------------------------------------
// View state machine.
// ---------------------------------------------------------------------------

function errorCode(error: unknown): string {
  if (error instanceof Error && error.message) return error.message;
  if (typeof error === "object" && error !== null && "message" in error) {
    const message: unknown = error.message;
    if (typeof message === "string" && message) return message;
  }
  return "";
}

export function isAbort(error: unknown): boolean {
  return (
    errorCode(error) === "Context changed" ||
    (error instanceof DOMException && error.name === "AbortError") ||
    errorCode(error) === "Upload context changed"
  );
}

export interface StateContext {
  online: boolean;
  hasPrevious: boolean;
}

// Map a successful observation onto a view state. Unknown coverage strings are
// treated as partial: the UI must never present an unrecognized observation as
// a complete total.
export function panelStateFromResult(
  result: Pick<AnalyticsQueryResult, "coverage" | "rows" | "partial">,
  context: StateContext,
): ViewState {
  if (!context.online) return "offline";
  if (result.coverage !== "complete" || result.partial) return "partial";
  if (!result.rows.length) return "empty";
  return "ready";
}

// Map a failed observation onto a view state. dependency_unavailable keeps the
// previous observation on screen as explicitly stale (with its own observed
// time) instead of presenting the cache as realtime; forbidden drops any
// previous detail from the old scope.
export function errorViewState(error: unknown, context: StateContext): ViewState {
  if (isAbort(error)) return "loading";
  if (!context.online) return "offline";
  const code = errorCode(error);
  if (code === "forbidden") return "forbidden";
  if (code === "unsupported") return "unsupported";
  if (code === "dependency_unavailable")
    return context.hasPrevious ? "stale" : "error";
  return "error";
}

const STATE_PRIORITY: ViewState[] = [
  "loading",
  "forbidden",
  "offline",
  "unsupported",
  "error",
  "stale",
  "partial",
  "empty",
  "ready",
];

// Aggregate per-panel states into one page-level status (worst first).
export function mergeViewState(states: ViewState[]): ViewState {
  let best: ViewState = "ready";
  for (const state of states) {
    if (STATE_PRIORITY.indexOf(state) < STATE_PRIORITY.indexOf(best)) best = state;
  }
  return best;
}

export const STATE_LABELS: Record<ViewState, string> = {
  loading: "加载中",
  ready: "已观测",
  empty: "暂无数据",
  partial: "部分覆盖",
  stale: "上次观测",
  forbidden: "无权限",
  offline: "离线",
  unsupported: "能力未启用",
  error: "出错",
};

// ---------------------------------------------------------------------------
// URL codec: only safe filter/selection values travel in the URL; tokens,
// bodies and temporary URLs never do.
// ---------------------------------------------------------------------------

export interface WorkspaceURLState {
  filters: WorkspaceFilters;
  panel?: PanelId;
  table: boolean;
  file?: { id: string; version?: string };
  drill?: AnalyticsDataset;
}

const SAFE_VALUE = /^[A-Za-z0-9._~:-]{1,200}$/;
const DAY = /^\d{4}-\d{2}-\d{2}$/;

export function encodeWorkspaceQuery(state: WorkspaceURLState): string {
  const query = new URLSearchParams();
  const { filters } = state;
  if (filters.space_id) query.set("space", filters.space_id);
  if (DAY.test(filters.from)) query.set("from", filters.from);
  if (DAY.test(filters.to)) query.set("to", filters.to);
  if (state.panel) query.set("panel", state.panel);
  if (state.table) query.set("view", "table");
  if (state.drill) query.set("drill", state.drill);
  if (state.file?.id) {
    query.set("file", state.file.id);
    if (state.file.version) query.set("version", state.file.version);
  }
  return query.toString();
}

export function decodeWorkspaceQuery(search: string): WorkspaceURLState {
  const params = new URLSearchParams(search);
  const read = (key: string) => {
    const value = params.get(key) || "";
    return SAFE_VALUE.test(value) ? value : "";
  };
  const state: WorkspaceURLState = {
    filters: { space_id: read("space"), from: "", to: "" },
    table: params.get("view") === "table",
  };
  const from = read("from");
  const to = read("to");
  if (DAY.test(from)) state.filters.from = from;
  if (DAY.test(to)) state.filters.to = to;
  const panel = read("panel");
  if (PANELS.some((p) => p.id === panel)) state.panel = panel as PanelId;
  const drill = read("drill");
  if (PANELS.some((p) => p.dataset === drill))
    state.drill = drill as AnalyticsDataset;
  const file = read("file");
  if (file) {
    state.file = { id: file };
    const version = read("version");
    if (version) state.file.version = version;
  }
  return state;
}

// ---------------------------------------------------------------------------
// Scale fixture: deterministic million-entry directory pages.
// ---------------------------------------------------------------------------

function mulberry32(seed: number) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

export interface DirectoryRow {
  file_id: string;
  name: string;
  depth: number;
  mime_type: string;
  logical_bytes: number;
  updated_at: string;
}

// Rows derive deterministically from their index, so a fixture of a million
// directories is addressed page by page without ever materializing the whole
// catalogue in memory. Keyset order is the file_id itself (ID tiebreak).
export function directoryRow(total: number, index: number): DirectoryRow {
  const random = mulberry32(index + 1);
  const mimes = ["image/png", "video/mp4", "text/markdown", "application/pdf"];
  const names = ["镜头", "分镜", "草稿", "样片", "报告", "素材"];
  const depth = 1 + Math.floor(random() * 5);
  return {
    file_id: "f" + String(index + 1).padStart(String(total).length + 1, "0"),
    name: names[Math.floor(random() * names.length)] + "-" + (index + 1),
    depth,
    mime_type: mimes[Math.floor(random() * mimes.length)],
    logical_bytes: Math.floor(random() * 4 << 20),
    updated_at: new Date(Date.UTC(2026, 8, 25) - index * 60000).toISOString(),
  };
}

export function directoryPage(
  total: number,
  cursorIndex: number,
  limit: number,
): DirectoryRow[] {
  const size = clampLimit(limit);
  const start = Math.max(0, Math.min(cursorIndex, total));
  const out: DirectoryRow[] = [];
  for (let i = start; i < Math.min(start + size, total); i++) out.push(directoryRow(total, i));
  return out;
}

export function pageCount(total: number, limit: number): number {
  return Math.ceil(total / clampLimit(limit));
}
