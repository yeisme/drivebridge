// Information workspace overview page (资源总览).
//
// Composes two contract-preserved surfaces:
//  - the existing usage endpoints (summary/series/breakdown/events/files) for
//    capacity, growth, transfer, connections and agent activity;
//  - the new typed analytics/file-query endpoints for the organization panel
//    and exact file drilldown, behind a capability gate that degrades
//    honestly when the deployment has not enabled them.
//
// Every panel states its scope, observation time and coverage; drilldowns
// reuse the same filter set as the panel that launched them (同口径), and
// switching tenant/space invalidates all in-flight observations so a late
// response from the old context can never render on the new page.

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api, bytes, errorText, getDataContext, type Connection, type Space } from "../api";
import { DataTable, type DataColumn } from "../components/DataTable";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { SelectAdapter } from "../components/SelectAdapter";
import { FileDetailSheet } from "./FileDetailSheet";
import { Table } from "../components/ui/table";
import type { DetailPreferenceIdentity } from "../app/detail-preference";
import { RevisionCache } from "./compat";
import { probeAnalytics, queryAnalytics, queryFiles, readOperation, type AnalyticsCapability } from "./client";
import {
  DEFAULT_PAGE_LIMIT,
  ORGANIZATION_PANEL_ANALYTICS,
  PANELS,
  STATE_LABELS,
  canonicalQueryKey,
  clampLimit,
  decodeWorkspaceQuery,
  encodeWorkspaceQuery,
  errorViewState,
  isAbort,
  mergeViewState,
  panelStateFromResult,
  sameCaliberDrilldown,
  type AnalyticsDataset,
  type AnalyticsQueryResult,
  type PanelId,
  type ViewState,
  type WorkspaceFilters,
} from "./model";
import { statusLabel } from "../i18n";
import "./info-workspace.css";
import { coverageLabel, directionLabel, utcStamp } from "./format";

// ---------------------------------------------------------------------------
// Preserved usage endpoint shapes (contract: usage_summary/series/breakdown/
// events/files keep their existing meaning and return types).
// ---------------------------------------------------------------------------

interface UsageTotals {
  effective_bytes: number;
  observed_bytes: number;
  completed_operations: number;
  failed_attempts: number;
}
interface UsageCapacity {
  space_id: string;
  connection_id: string;
  current_bytes: number;
  history_bytes: number | null;
  trash_bytes: number;
  reserved_bytes: number;
  quota_bytes: number | null;
  coverage: string;
  observed_at: string;
  day: string;
}
interface UsageSummary {
  collection?: { collection_started_at?: string };
  totals: UsageTotals;
  capacities: UsageCapacity[];
  coverage?: string[];
  observed_total_bytes: number | null;
  observed_total_status?: string;
}
interface UsageGroup extends UsageTotals {
  key: string;
  kind: string;
}
interface UsagePoint {
  day: string;
  effective_bytes: number;
  observed_bytes: number;
}
interface UsageFact {
  sequence: number;
  at: string;
  actor_id?: string;
  caller?: string;
  space_id?: string;
  direction?: string;
  purpose: string;
  metric: string;
  bytes: number;
  operation_id?: string;
  resource_id?: string;
}
interface OccupancyItems {
  items: { resource_id: string; version_id: string; name: string; size_bytes: number }[];
  partial: boolean;
  coverage: string;
}

const EMPTY_OBSERVATIONS = {
  summary: null as UsageSummary | null,
  series: [] as UsagePoint[],
  transfer: [] as UsageGroup[],
  agent: [] as UsageGroup[],
  connections: [] as Connection[],
  spaces: [] as Space[],
  organization: null as AnalyticsQueryResult | null,
};

interface PanelMeta {
  state: ViewState;
  observedAt?: string;
  coverage?: string;
  error?: unknown;
}

type PanelStates = Record<PanelId, PanelMeta>;

const initialPanelStates = (): PanelStates =>
  Object.fromEntries(
    PANELS.map((panel) => [panel.id, { state: "loading" } as PanelMeta]),
  ) as PanelStates;


function usageQuery(filters: WorkspaceFilters): string {
  const params = new URLSearchParams();
  if (filters.space_id) params.set("space_id", filters.space_id);
  if (filters.from) params.set("from", filters.from);
  if (filters.to) params.set("to", filters.to);
  return params.toString();
}

// ---------------------------------------------------------------------------
// Drilldown state: rows normalized to one display shape whatever surface
// produced them, so the table, pagination and actions stay uniform.
// ---------------------------------------------------------------------------

interface DrillRow {
  id: string;
  label: string;
  detail?: string;
  size?: number;
  file?: { file_id: string; version_id: string };
  operation_id?: string;
}

type DrillKind = "usage-files" | "usage-events" | "file-query" | "connections";

interface DrillState {
  panel: PanelId;
  kind: DrillKind;
  dataset: AnalyticsDataset;
  filters: WorkspaceFilters;
  // The cursor is only ever replayed against the canonical query it was
  // issued for; any filter/scope change starts a fresh first page.
  queryKey: string;
  cursor: string | null;
  nextCursor: string | null;
  state: ViewState;
  rows: DrillRow[];
  total: number | null;
  coverage?: string;
  error?: unknown;
}

interface OperationReadback {
  id: string;
  status: string;
  updated_at?: string;
  bytes_transferred: number;
  size_bytes: number;
}

export function InfoWorkspace({ spaces: propSpaces, preferenceIdentity }: { spaces?: Space[]; preferenceIdentity?: DetailPreferenceIdentity }) {
  const url = useMemo(
    () => decodeWorkspaceQuery((location.hash.split("?")[1] || "") + "&" + location.search.replace(/^\?/, "")),
    [],
  );
  const [filters, setFilters] = useState<WorkspaceFilters>(() => ({
    space_id: url.filters.space_id,
    from: url.filters.from,
    to: url.filters.to,
  }));
  const [observations, setObservations] = useState(EMPTY_OBSERVATIONS);
  const [panelStates, setPanelStates] = useState<PanelStates>(initialPanelStates);
  const [capability, setCapability] = useState<AnalyticsCapability>("checking");
  const [tableView, setTableView] = useState(url.table);
  const [contextEpoch, setContextEpoch] = useState(0);
  const [drill, setDrill] = useState<DrillState | null>(null);
  const [detail, setDetail] = useState<{ fileId: string; versionId?: string } | null>(
    url.file ? { fileId: url.file.id, versionId: url.file.version } : null,
  );
  const detailOpener = useRef<HTMLElement | null>(null);
  const [readback, setReadback] = useState<OperationReadback | null>(null);
  const [readbackError, setReadbackError] = useState("");
  const organizationCache = useRef(new RevisionCache<AnalyticsQueryResult>());

  // ---- URL sync: only safe filter/selection values, no tokens or bodies. --
  useEffect(() => {
    const query = encodeWorkspaceQuery({
      filters,
      table: tableView,
      panel: drill?.panel,
      drill: drill?.dataset,
      file: detail ? { id: detail.fileId, version: detail.versionId } : undefined,
    });
    const base = location.hash.split("?")[0] || "#/overview";
    history.replaceState(null, "", location.pathname + location.search + (query ? base + "?" + query : base));
  }, [filters, tableView, drill, detail]);

  // ---- Capability gate ----------------------------------------------------
  // A probe failure is only "unsupported" when it means the surface is
  // absent (not_found / request_failed / unsupported). Any other error — a
  // transient 500 or a network flap — becomes a retryable error state so a
  // momentary blip is never reported as "capability not enabled".
  const [probeEpoch, setProbeEpoch] = useState(0);
  const [probeError, setProbeError] = useState<unknown>(null);
  useEffect(() => {
    const controller = new AbortController();
    setCapability("checking");
    setProbeError(null);
    probeAnalytics(controller.signal)
      .then((result) => setCapability(result))
      .catch((error) => {
        if (isAbort(error)) return;
        const code = error instanceof Error ? error.message : "";
        if (code === "not_found" || code === "request_failed" || code === "unsupported") {
          setCapability("unsupported");
          return;
        }
        setProbeError(error);
        setCapability("error");
      });
    return () => controller.abort();
  }, [probeEpoch]);

  const load = useCallback(
    async (signal: AbortSignal) => {
      const revision = getDataContext().revision;
      const query = usageQuery(filters);
      const online = navigator.onLine;
      const jobs: { panel: PanelId; run: Promise<unknown> }[] = [
        { panel: "capacity", run: api<UsageSummary>("/usage/summary?" + query, "GET", undefined, {}, signal) },
        { panel: "growth", run: api<UsagePoint[]>("/usage/series?" + query, "GET", undefined, {}, signal) },
        { panel: "transfer", run: api<UsageGroup[]>("/usage/breakdown?" + query + "&group=direction", "GET", undefined, {}, signal) },
        { panel: "agent-activity", run: api<UsageGroup[]>("/usage/breakdown?" + query + "&group=actor", "GET", undefined, {}, signal) },
        {
          panel: "connections",
          run: Promise.all([
            api<Connection[]>("/connections", "GET", undefined, {}, signal),
            api<Space[]>("/spaces", "GET", undefined, {}, signal),
          ]),
        },
      ];
      if (capability === "available") {
        const drilldown = sameCaliberDrilldown(filters, "file_current");
        jobs.push({
          panel: "organization",
          // Precise current inventory: live consistency plus the dataset's
          // fixed dimensions/metrics (empty selections are invalid_query).
          run: queryAnalytics(
            { ...ORGANIZATION_PANEL_ANALYTICS, filters: drilldown.filters, limit: DEFAULT_PAGE_LIMIT },
            signal,
          ),
        });
      }
      const settled = await Promise.allSettled(jobs.map((job) => job.run));
      // A context switch during the await invalidates everything below; the
      // envelope helper already rejects such responses, this is the guard for
      // updates queued between microtasks.
      if (signal.aborted || revision !== getDataContext().revision) return;
      const nextPanels = initialPanelStates();
      const next = { ...EMPTY_OBSERVATIONS };
      const apply = <T,>(index: number, set: (value: T) => void) => {
        const outcome = settled[index];
        const panel = jobs[index].panel;
        if (outcome.status === "fulfilled") {
          const value = outcome.value as T;
          set(value);
        } else {
          const error = outcome.reason;
          const previous = organizationCache.current.get(revision, "organization");
          const state = errorViewState(error, {
            online,
            hasPrevious: panel === "organization" && previous !== undefined,
          });
          nextPanels[panel] = {
            state,
            error,
            ...(panel === "organization" && state === "stale" && previous
              ? { observedAt: previous.observed_at, coverage: previous.coverage }
              : {}),
          };
          if (panel === "organization" && state === "stale" && previous) next.organization = previous;
        }
      };
      apply<UsageSummary>(0, (value) => {
        next.summary = value;
        const observed = value.collection?.collection_started_at || value.capacities[0]?.observed_at;
        nextPanels.capacity = {
          state: value.coverage?.length && value.coverage.some((c) => c !== "complete") ? "partial" : value.capacities.length ? "ready" : "empty",
          observedAt: observed,
          coverage: value.coverage?.join("、"),
        };
      });
      apply<UsagePoint[]>(1, (value) => {
        next.series = value;
        nextPanels.growth = {
          state: value.length ? "ready" : "empty",
          observedAt: nextPanels.capacity.observedAt,
        };
      });
      apply<UsageGroup[]>(2, (value) => {
        next.transfer = value;
        nextPanels.transfer = {
          state: value.length ? "ready" : "empty",
          observedAt: nextPanels.capacity.observedAt,
        };
      });
      apply<UsageGroup[]>(3, (value) => {
        next.agent = value;
        nextPanels["agent-activity"] = {
          state: value.length ? "ready" : "empty",
          observedAt: nextPanels.capacity.observedAt,
        };
      });
      apply<[Connection[], Space[]]>(4, ([connections, spaces]) => {
        next.connections = connections;
        next.spaces = spaces;
        nextPanels.connections = {
          state: spaces.length ? "ready" : "empty",
          observedAt: nextPanels.capacity.observedAt,
        };
      });
      if (capability === "available") {
        apply<AnalyticsQueryResult>(5, (value) => {
          next.organization = value;
          organizationCache.current.put(revision, "organization", value);
          nextPanels.organization = {
            state: panelStateFromResult(value, { online, hasPrevious: false }),
            observedAt: value.observed_at,
            coverage: value.coverage,
          };
        });
      } else {
        nextPanels.organization =
          capability === "unsupported"
            ? { state: "unsupported" }
            : capability === "error"
              ? { state: "error", error: probeError }
              : { state: "loading" };
      }
      setObservations(next);
      setPanelStates(nextPanels);
    },
    [capability, filters, probeError],
  );

  // ---- Load lifecycle: refresh, visibility pause, context isolation. ------
  useEffect(() => {
    const controller = new AbortController();
    const running = { current: false };
    const refresh = async () => {
      if (document.hidden || running.current) return;
      running.current = true;
      try {
        await load(controller.signal);
      } finally {
        running.current = false;
      }
    };
    void refresh();
    // Default 30s refresh while the page is visible; hidden pages pause.
    const timer = setInterval(() => void refresh(), 30000);
    document.addEventListener("visibilitychange", refresh);
    return () => {
      clearInterval(timer);
      document.removeEventListener("visibilitychange", refresh);
      controller.abort();
    };
  }, [load, contextEpoch]);

  // Context switch: drop every observation immediately so nothing from the
  // old tenant/space can render while the new context loads, then reload
  // under the new revision.
  useEffect(() => {
    const onContext = () => {
      organizationCache.current.clear();
      setObservations(EMPTY_OBSERVATIONS);
      setPanelStates(initialPanelStates());
      setDrill(null);
      setReadback(null);
      setContextEpoch((n) => n + 1);
    };
    window.addEventListener("drivebridge-context", onContext);
    return () => window.removeEventListener("drivebridge-context", onContext);
  }, []);

  // ---- Drilldown -----------------------------------------------------------
  const openDrill = useCallback(
    async (panel: PanelId, daySelection?: { from: string; to: string }) => {
      const definition = PANELS.find((p) => p.id === panel)!;
      const drillFilters = daySelection ? { ...filters, ...daySelection } : filters;
      const kind: DrillKind =
        panel === "capacity"
          ? "usage-files"
          : panel === "connections"
            ? "connections"
            : panel === "organization"
              ? "file-query"
              : "usage-events";
      const queryKey =
        kind === "file-query"
          ? canonicalQueryKey({
              mode: "exact",
              space_ids: drillFilters.space_id ? [drillFilters.space_id] : [],
              filters: sameCaliberDrilldown(drillFilters, definition.dataset).filters,
              limit: DEFAULT_PAGE_LIMIT,
            })
          : usageQuery(drillFilters) + "#" + kind;
      setReadback(null);
      setReadbackError("");
      setDrill({
        panel,
        kind,
        dataset: definition.dataset,
        filters: drillFilters,
        queryKey,
        cursor: null,
        nextCursor: null,
        state: "loading",
        rows: [],
        total: null,
      });
    },
    [filters],
  );

  useEffect(() => {
    if (!drill) return;
    const controller = new AbortController();
    // Drill responses apply only while their page load is still active; a
    // later cursor/filter change or unmount invalidates them even if the
    // transport-level abort has not landed yet.
    let active = true;
    const live = () => active;
    const signal = controller.signal;
    const run = async () => {
      try {
        if (drill.kind === "connections") {
          const [connections] = await Promise.all([
            api<Connection[]>("/connections", "GET", undefined, {}, signal),
            api<Space[]>("/spaces", "GET", undefined, {}, signal),
          ]);
          const rows: DrillRow[] = connections.map((connection) => ({
            id: connection.id,
            label: connection.name,
            detail:
              connection.status === "ok" ? "连接正常" : connection.status === "error" ? "连接异常" : connection.status,
            operation_id: undefined,
          }));
          setDrill((old) =>
            old && live() && old.queryKey === drill.queryKey
              ? { ...old, state: rows.length ? "ready" : "empty", rows, nextCursor: null }
              : old,
          );
          return;
        }
        if (drill.kind === "usage-files") {
          const out = await api<OccupancyItems>(
            "/usage/files?" + usageQuery(drill.filters) + "&category=current",
            "GET",
            undefined,
            {},
            signal,
          );
          const rows: DrillRow[] = out.items.map((item) => ({
            id: item.resource_id + "/" + item.version_id,
            label: item.name,
            size: item.size_bytes,
            file: { file_id: item.resource_id, version_id: item.version_id },
          }));
          setDrill((old) =>
            old && live() && old.queryKey === drill.queryKey
              ? {
                  ...old,
                  state: out.partial ? "partial" : rows.length ? "ready" : "empty",
                  coverage: out.coverage,
                  rows,
                  nextCursor: null,
                }
              : old,
          );
          return;
        }
        if (drill.kind === "usage-events") {
          const before = Number(drill.cursor || 0) || 0;
          const page = await api<{ events: UsageFact[]; next_cursor: number }>(
            "/usage/events?" + usageQuery(drill.filters) + "&before=" + before,
            "GET",
            undefined,
            {},
            signal,
          );
          const rows: DrillRow[] = page.events.map((fact) => ({
            id: String(fact.sequence),
            label: fact.direction ? directionLabel(fact.direction) : fact.purpose,
            detail: new Date(fact.at).toISOString().slice(0, 19).replace("T", " ") + " UTC · " + fact.metric,
            size: fact.bytes,
            operation_id: fact.operation_id,
            file: fact.resource_id
              ? { file_id: fact.resource_id, version_id: fact.resource_id }
              : undefined,
          }));
          setDrill((old) =>
            old && live() && old.queryKey === drill.queryKey
              ? {
                  ...old,
                  state: rows.length ? "ready" : "empty",
                  rows,
                  cursor: String(before),
                  nextCursor: page.next_cursor ? String(page.next_cursor) : null,
                }
              : old,
          );
          return;
        }
        // file-query (exact): capability-gated precise file list.
        if (capability !== "available") {
          setDrill((old) => (old && live() && old.queryKey === drill.queryKey ? { ...old, state: "unsupported" } : old));
          return;
        }
        const request = {
          mode: "exact" as const,
          ...(drill.filters.space_id ? { space_ids: [drill.filters.space_id] } : {}),
          filters: sameCaliberDrilldown(drill.filters, drill.dataset).filters,
          limit: DEFAULT_PAGE_LIMIT,
          ...(drill.cursor ? { cursor: drill.cursor } : {}),
        };
        const result = await queryFiles(request, signal);
        const rows: DrillRow[] = result.items.map((item) => ({
          id: item.file_id + "/" + item.version_id,
          label: item.file_id,
          detail: item.version_id,
          file: item,
        }));
        setDrill((old) =>
          old && live() && old.queryKey === drill.queryKey
            ? {
                ...old,
                state:
                  result.coverage && result.coverage !== "complete"
                    ? "partial"
                    : rows.length
                      ? "ready"
                      : "empty",
                coverage: result.coverage,
                rows,
                total: result.total_count ?? null,
                nextCursor: result.next_cursor,
              }
            : old,
        );
      } catch (error) {
        if (isAbort(error)) return;
        setDrill((old) => {
          if (!live() || !old || old.queryKey !== drill.queryKey) return old;
          const state = errorViewState(error, { online: navigator.onLine, hasPrevious: false });
          // A cursor bound to a previous catalogue revision is not silently
          // retried with a different key: return to the first page.
          if (error instanceof Error && error.message === "cursor_expired") {
            return { ...old, state: "error", error, cursor: null, nextCursor: null };
          }
          return { ...old, state, error };
        });
      }
    };
    void run();
    return () => {
      active = false;
      controller.abort();
    };
  }, [drill?.panel, drill?.kind, drill?.cursor, drill?.queryKey, drill?.filters, capability]);

  const openDetail = (row: DrillRow, opener: HTMLElement) => {
    if (!row.file) return;
    detailOpener.current = opener;
    setReadback(null);
    setReadbackError("");
    setDetail({ fileId: row.file.file_id, versionId: row.file.version_id });
  };

  const showReadback = async (operationId: string) => {
    setReadbackError("");
    setReadback(null);
    try {
      // Evidence readback always re-reads the original operation ID.
      const op = await readOperation(operationId);
      setReadback({
        id: op.id,
        status: op.status,
        updated_at: op.updated_at,
        bytes_transferred: op.bytes_transferred,
        size_bytes: op.size_bytes,
      });
    } catch (error) {
      if (!isAbort(error)) setReadbackError(errorText(error));
    }
  };

  const retry = () => {
    setPanelStates(initialPanelStates());
    // A retryable probe error re-runs the capability probe; an absent
    // surface stays unsupported until the deployment actually enables it.
    if (capability === "error") setProbeEpoch((epoch) => epoch + 1);
    void load(new AbortController().signal);
  };

  const spaceOptions = (observations.spaces.length ? observations.spaces : propSpaces || []).map(
    (space) => ({ id: space.id, name: space.name }),
  );
  const scopeName =
    filters.space_id && spaceOptions.find((s) => s.id === filters.space_id)?.name;
  const pageState = mergeViewState(PANELS.map((panel) => panelStates[panel.id].state));

  return (
    <div className="iw-page" data-page-state={pageState}>
      <div className="iw-toolbar">
        <div className="iw-filters">
          <label className="iw-filter">
            <span>空间范围</span>
            <SelectAdapter
              aria-label="空间范围"
              value={filters.space_id}
              onChange={(e) => setFilters((old) => ({ ...old, space_id: e.target.value }))}
            >
              <option value="">当前授权空间</option>
              {spaceOptions.map((space) => (
                <option key={space.id} value={space.id}>
                  {space.name}
                </option>
              ))}
            </SelectAdapter>
          </label>
          <label className="iw-filter">
            <span>观测开始</span>
            <Input
              aria-label="观测开始日期"
              type="date"
              value={filters.from}
              onChange={(e) => setFilters((old) => ({ ...old, from: e.target.value }))}
            />
          </label>
          <label className="iw-filter">
            <span>观测结束</span>
            <Input
              aria-label="观测结束日期"
              type="date"
              value={filters.to}
              onChange={(e) => setFilters((old) => ({ ...old, to: e.target.value }))}
            />
          </label>
        </div>
        <div className="iw-toolbar-actions">
          <Button
            variant="outline"
            type="button"
            onClick={() => setTableView((old) => !old)}
            aria-pressed={tableView}
          >
            {tableView ? "图表视图" : "表格视图"}
          </Button>
          <Button variant="outline" type="button" onClick={retry}>
            刷新
          </Button>
        </div>
      </div>
      <p className="iw-scope" aria-live="polite">
        当前口径：{scopeName || "当前授权空间"}
        {filters.from && filters.to ? ` · ${filters.from} 至 ${filters.to}` : ""}
        {pageState !== "ready" && <span className="iw-scope-state">（{STATE_LABELS[pageState]}）</span>}
      </p>

      <div className="iw-grid">
        {PANELS.map((panel) => (
          <PanelCard
            key={panel.id}
            panel={panel}
            meta={panelStates[panel.id]}
            scopeName={scopeName}
            observations={observations}
            tableView={tableView}
            drillDisabled={panel.source === "analytics" && capability !== "available"}
            onRetry={retry}
            onDrill={() => void openDrill(panel.id)}
            onDay={(day) => void openDrill(panel.id, { from: day, to: day })}
          />
        ))}
      </div>

      {drill && (
        <section className="iw-drill" aria-label="筛选明细">
          <div className="iw-drill-head">
            <Button
              variant="ghost"
              type="button"
              onClick={() => setDrill(null)}
              aria-label="返回总览"
            >
              返回总览
            </Button>
            <h2>
              {PANELS.find((p) => p.id === drill.panel)?.label}明细
              <small>
                同口径：{drill.filters.space_id || "当前授权空间"}
                {drill.filters.from && drill.filters.to
                  ? ` · ${drill.filters.from} 至 ${drill.filters.to}`
                  : ""}
              </small>
            </h2>
          </div>
          <DrillBody
            drill={drill}
            readback={readback}
            readbackError={readbackError}
            onReadback={showReadback}
            onOpenDetail={openDetail}
            onPage={(cursor) =>
              setDrill((old) => (old ? { ...old, cursor, state: "loading" } : old))
            }
            onRetryFirstPage={() =>
              setDrill((old) =>
                old ? { ...old, cursor: null, nextCursor: null, state: "loading" } : old,
              )
            }
          />
        </section>
      )}

      <FileDetailSheet
        preferenceIdentity={preferenceIdentity}
        fileId={detail?.fileId || ""}
        versionId={detail?.versionId}
        open={!!detail}
        onClose={() => {
          setDetail(null);
          // Restore focus to the element that opened the detail view.
          detailOpener.current?.focus();
          detailOpener.current = null;
        }}
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Panel card
// ---------------------------------------------------------------------------

function PanelCard({
  panel,
  meta,
  scopeName,
  observations,
  tableView,
  drillDisabled,
  onRetry,
  onDrill,
  onDay,
}: {
  panel: (typeof PANELS)[number];
  meta: PanelMeta;
  scopeName?: string;
  observations: typeof EMPTY_OBSERVATIONS;
  tableView: boolean;
  drillDisabled: boolean;
  onRetry: () => void;
  onDrill: () => void;
  onDay: (day: string) => void;
}) {
  return (
    <section className="iw-panel" data-panel={panel.id} data-state={meta.state} aria-label={panel.label}>
      <header>
        <h3>{panel.label}</h3>
        <div className="iw-panel-meta">
          <span className={"iw-chip iw-chip-" + meta.state}>{STATE_LABELS[meta.state]}</span>
          <span className="iw-scope-label">{scopeName || "当前授权空间"}</span>
          {meta.observedAt && <span className="iw-observed">观测于 {utcStamp(meta.observedAt)}</span>}
          {meta.coverage && meta.coverage !== "complete" && (
            <span className="iw-coverage">覆盖：{coverageLabel(meta.coverage)}</span>
          )}
        </div>
      </header>
      <PanelBody
        panel={panel}
        meta={meta}
        observations={observations}
        tableView={tableView}
        onRetry={onRetry}
        onDay={onDay}
      />
      <footer>
        <Button
          variant="ghost"
          type="button"
          className="iw-drill-btn"
          disabled={drillDisabled}
          title={drillDisabled ? "后端未提供该明细数据源，已禁用钻取" : undefined}
          onClick={onDrill}
        >
          查看明细
        </Button>
        {drillDisabled && <span className="iw-drill-note">后端未提供明细，钻取已禁用</span>}
      </footer>
    </section>
  );
}

function PanelBody({
  panel,
  meta,
  observations,
  tableView,
  onRetry,
  onDay,
}: {
  panel: (typeof PANELS)[number];
  meta: PanelMeta;
  observations: typeof EMPTY_OBSERVATIONS;
  tableView: boolean;
  onRetry: () => void;
  onDay: (day: string) => void;
}) {
  if (meta.state === "loading")
    return (
      <div role="status" className="iw-loading">
        正在加载…
      </div>
    );
  if (meta.state === "forbidden")
    return <EmptyHint text="当前授权不允许查看此范围。可切换到仍授权的空间。" />;
  if (meta.state === "offline") return <EmptyHint text="当前离线，恢复网络后自动刷新。" />;
  if (meta.state === "unsupported")
    return (
      <EmptyHint text="此部署未启用对应分析能力，总览其余面板不受影响；旧版资源态势页仍可访问。" />
    );
  if (meta.state === "error")
    return (
      <div role="alert" className="iw-error">
        {meta.error instanceof Error ? errorText(meta.error) : "加载失败"}
        <Button variant="outline" type="button" onClick={onRetry}>
          重试
        </Button>
      </div>
    );
  const stale = meta.state === "stale";
  return (
    <>
      {stale && (
        <p className="iw-stale" role="status">
          分析后端暂不可用，以下为上次观测（{utcStamp(meta.observedAt || "")}），非实时数据。
        </p>
      )}
      <PanelData panel={panel} observations={observations} tableView={tableView} onDay={onDay} />
      {meta.state === "empty" && <EmptyHint text="暂无观测数据；未取得的数据不会以 0 代替。" />}
      {meta.state === "partial" && (
        <p className="iw-partial">
          部分覆盖：以上仅统计已观测范围{meta.coverage ? `（${coverageLabel(meta.coverage)}）` : ""}，不代表全盘总量。
        </p>
      )}
    </>
  );
}

function EmptyHint({ text }: { text: string }) {
  return (
    <p className="iw-hint" role="note">
      {text}
    </p>
  );
}

function PanelData({
  panel,
  observations,
  tableView,
  onDay,
}: {
  panel: (typeof PANELS)[number];
  observations: typeof EMPTY_OBSERVATIONS;
  tableView: boolean;
  onDay: (day: string) => void;
}) {
  if (panel.id === "capacity") {
    const rows = observations.summary?.capacities || [];
    return (
      <Table className="iw-table">
        <caption className="sr-only">容量口径：逻辑、历史、回收站与预留分开计量</caption>
        <thead>
          <tr>
            <th scope="col">空间</th>
            <th scope="col">当前占用</th>
            <th scope="col">历史版本</th>
            <th scope="col">回收站</th>
            <th scope="col">预留</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.space_id}>
              <th scope="row">{row.space_id}</th>
              <td>{bytes(row.current_bytes)}</td>
              <td>{row.history_bytes === null ? "未取得" : bytes(row.history_bytes)}</td>
              <td>{bytes(row.trash_bytes)}</td>
              <td>{bytes(row.reserved_bytes)}</td>
            </tr>
          ))}
        </tbody>
      </Table>
    );
  }
  if (panel.id === "growth") {
    const rows = observations.series;
    if (!rows.length) return null;
    if (tableView)
      return (
        <Table className="iw-table">
          <caption className="sr-only">每日有效文件量与观测传输量</caption>
          <thead>
            <tr>
              <th scope="col">日期</th>
              <th scope="col">有效文件量</th>
              <th scope="col">观测传输量</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.day}>
                <th scope="row">{row.day}</th>
                <td>{bytes(row.effective_bytes)}</td>
                <td>{bytes(row.observed_bytes)}</td>
              </tr>
            ))}
          </tbody>
        </Table>
      );
    const max = Math.max(1, ...rows.map((r) => Math.max(r.effective_bytes, r.observed_bytes)));
    return (
      <ul className="iw-bars" aria-label="增长趋势">
        {rows.map((row) => (
          <li key={row.day}>
            <Button variant="ghost" type="button" className="iw-bar-row" onClick={() => onDay(row.day)}>
              <span className="iw-bar-label">{row.day}</span>
              <span className="iw-bar-track">
                <i style={{ width: (row.effective_bytes / max) * 100 + "%" }} />
              </span>
              <span className="iw-bar-value">{bytes(row.effective_bytes)}</span>
            </Button>
          </li>
        ))}
      </ul>
    );
  }
  if (panel.id === "transfer" || panel.id === "agent-activity") {
    const rows = panel.id === "transfer" ? observations.transfer : observations.agent;
    const metric = observations.summary?.totals.observed_bytes;
    return (
      <Table className="iw-table">
        <caption className="sr-only">{panel.label}分组（观测口径，非账单）</caption>
        <thead>
          <tr>
            <th scope="col">{panel.id === "transfer" ? "方向" : "身份"}</th>
            <th scope="col">观测传输量</th>
            <th scope="col">成功操作</th>
            <th scope="col">失败尝试</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.kind + row.key}>
              <th scope="row">{panel.id === "transfer" ? directionLabel(row.key) : row.key}</th>
              <td>{bytes(row.observed_bytes)}</td>
              <td>{row.completed_operations}</td>
              <td>{row.failed_attempts}</td>
            </tr>
          ))}
        </tbody>
        {metric !== undefined && rows.length > 1 && (
          <tfoot>
            <tr>
              <th scope="row">合计（观测口径）</th>
              <td colSpan={3}>未取得分项之外的全量口径时不作合计声明</td>
            </tr>
          </tfoot>
        )}
      </Table>
    );
  }
  if (panel.id === "connections") {
    const rows = observations.connections;
    return (
      <Table className="iw-table">
        <caption className="sr-only">存储连接状态</caption>
        <thead>
          <tr>
            <th scope="col">连接</th>
            <th scope="col">类型</th>
            <th scope="col">状态</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.id}>
              <th scope="row">{row.name}</th>
              <td>{row.kind}</td>
              <td>{row.disabled ? "已停用" : row.status === "ok" ? "正常" : row.status}</td>
            </tr>
          ))}
        </tbody>
      </Table>
    );
  }
  // organization: typed analytics rows rendered generically; unknown columns
  // are shown as returned, never renamed or dropped client-side.
  const result = observations.organization;
  if (!result) return null;
  const columns = [...new Set(result.rows.flatMap((row) => Object.keys(row)))];
  return (
    <Table className="iw-table">
      <caption className="sr-only">文件整理分组（类型化分析口径）</caption>
      <thead>
        <tr>
          {columns.map((column) => (
            <th scope="col" key={column}>
              {column}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {result.rows.map((row, index) => (
          <tr key={index}>
            {columns.map((column) => (
              <td key={column}>
                {row[column] === null || row[column] === undefined
                  ? "未取得"
                  : typeof row[column] === "number"
                    ? bytes(row[column] as number)
                    : String(row[column])}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </Table>
  );
}

// ---------------------------------------------------------------------------
// Drilldown body
// ---------------------------------------------------------------------------

function DrillBody({
  drill,
  readback,
  readbackError,
  onReadback,
  onOpenDetail,
  onPage,
  onRetryFirstPage,
}: {
  drill: DrillState;
  readback: OperationReadback | null;
  readbackError: string;
  onReadback: (id: string) => void;
  onOpenDetail: (row: DrillRow, opener: HTMLElement) => void;
  onPage: (cursor: string | null) => void;
  onRetryFirstPage: () => void;
}) {
  const columns: DataColumn<DrillRow & object>[] = [
    {
      id: "label",
      header: "条目",
      cell: (row) => (
        <span className="iw-drill-label">
          {row.label}
          {row.detail && <small>{row.detail}</small>}
        </span>
      ),
    },
    { id: "size", header: "字节", cell: (row) => (row.size === undefined ? "未取得" : bytes(row.size)) },
    {
      id: "actions",
      header: "操作",
      cell: (row) => (
        <span className="iw-drill-actions">
          {row.file && (
            <Button
              variant="ghost"
              type="button"
              onClick={(e) => onOpenDetail(row, e.currentTarget)}
            >
              打开详情
            </Button>
          )}
          {row.operation_id && (
            <Button variant="ghost" type="button" onClick={() => onReadback(row.operation_id!)}>
              回读原操作
            </Button>
          )}
        </span>
      ),
    },
  ];
  return (
    <>
      {drill.state === "loading" && (
        <div role="status" className="iw-loading">
          正在加载明细…
        </div>
      )}
      {drill.state === "unsupported" && (
        <EmptyHint text="此部署未启用精确文件查询；文件页与旧版资源态势不受影响。" />
      )}
      {drill.state === "forbidden" && <EmptyHint text="当前授权不允许查看此明细范围。" />}
      {drill.state === "error" && (
        <div role="alert" className="iw-error">
          {drill.error instanceof Error ? errorText(drill.error) : "加载失败"}
          <Button variant="outline" type="button" onClick={onRetryFirstPage}>
            返回第一页
          </Button>
        </div>
      )}
      {(drill.state === "ready" || drill.state === "partial" || drill.state === "empty") && (
        <>
          {drill.state === "partial" && (
            <p className="iw-partial">
              部分覆盖：仅统计已观测范围{drill.coverage ? `（${coverageLabel(drill.coverage)}）` : ""}
              {drill.total === null ? "；全量计数未取得，不以 0 代替" : ""}。
            </p>
          )}
          <DataTable
            label="筛选明细数据表"
            data={drill.rows}
            getRowId={(row) => row.id}
            columns={columns}
            pageSize={clampLimit(DEFAULT_PAGE_LIMIT)}
            pagination="internal"
            searchable={false}
          />
          {drill.state === "empty" && <EmptyHint text="当前筛选下没有匹配的记录。" />}
          {drill.kind === "usage-events" && drill.nextCursor && (
            <div className="iw-pager">
              <Button variant="outline" type="button" onClick={() => onPage(drill.nextCursor)}>
                下一页
              </Button>
            </div>
          )}
          {drill.kind === "file-query" && (
            <div className="iw-pager">
              <Button variant="outline" type="button" disabled={!drill.cursor} onClick={() => onPage(null)}>
                第一页
              </Button>
              <Button
                variant="outline"
                type="button"
                disabled={!drill.nextCursor}
                onClick={() => drill.nextCursor && onPage(drill.nextCursor)}
              >
                下一页
              </Button>
            </div>
          )}
        </>
      )}
      {readbackError && (
        <p role="alert" className="iw-error">
          {readbackError}
        </p>
      )}
      {readback && (
        <section className="iw-readback" aria-label="原操作回读">
          <h3>原操作回读（原 ID）</h3>
          <p>
            操作 {readback.id} · 状态 {statusLabel(readback.status)} · 已传输{" "}
            {bytes(readback.bytes_transferred)} / {bytes(readback.size_bytes)}
            {readback.updated_at ? ` · 更新于 ${utcStamp(readback.updated_at)}` : ""}
          </p>
        </section>
      )}
    </>
  );
}
