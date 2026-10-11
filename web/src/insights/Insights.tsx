import { SlidersHorizontal, X } from "lucide-react";
import { Popover, PopoverTrigger, PopoverContent, PopoverTitle } from "../components/ui/popover";
import { auditActionLabel, auditActorName, shortIdentifier } from "./audit-labels";
import { Input } from "../components/ui/input";
import { DataTable } from "../components/DataTable";
import { Button } from "../components/ui/button";
import { AppDialog } from "../components/AppDialog";
import { SelectAdapter } from "./../components/SelectAdapter";
import React, { useCallback, useEffect, useRef, useState } from "react";
import { api, bytes, errorText, type Space } from "../api";
import { CapacityDonut } from "../CapacityView";
import { backendOf, capabilityText } from "../storage";
import { StorageBrowser } from "./StorageBrowser";
import { MeterBar, Trend } from "./charts";
import "./insights.css";
export { Trend, MeterBar };

type Tab = "resources" | "transfers" | "agents" | "audit" | "storage";
interface Totals {
  effective_bytes: number;
  observed_bytes: number;
  completed_operations: number;
  failed_attempts: number;
}
interface Capacity {
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
interface Scan {
  id: string;
  space_id: string;
  status: string;
  entries: number;
  observed_bytes: number;
  started_at: string;
  finished_at?: string;
  error?: string;
  resumable: boolean;
}
interface Summary {
  metric_version: string;
  collection: {
    collection_started_at: string;
    gap_detected: boolean;
    last_pruned_at: string;
  };
  totals: Totals;
  capacities: Capacity[];
  scans: Scan[];
  scan_policies: {
    space_id: string;
    enabled: boolean;
    max_entries: number;
    max_seconds: number;
  }[];
  alerts: { code: string; severity: string; space_id?: string }[];
  observed_total_bytes: number | null;
  observed_total_status: string;
  coverage: string[];
}
interface Point extends Totals {
  day: string;
  capacities: Capacity[];
}
interface Group extends Totals {
  key: string;
  kind: string;
}
interface Fact {
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
interface Audit {
  resource_kind?: string;
  id: number;
  at: string;
  actor_id?: string;
  actor_kind: string;
  action: string;
  outcome: string;
  resource_id?: string;
  operation_id?: string;
  correlation_id: string;
}
interface AuditDay {
  day: string;
  action: string;
  outcome: string;
  count: number;
}
interface Filters {
  space_id: string;
  connection_id: string;
  from: string;
  to: string;
  actor: string;
  caller: string;
  direction: string;
  purpose: string;
  action: string;
  outcome: string;
  resource: string;
  correlation: string;
}
const utcDay = (date: Date) => date.toISOString().slice(0, 10);
function initialFilters(): Filters {
  const now = new Date(),
    from = new Date(now);
  from.setUTCDate(from.getUTCDate() - 29);
  const defaults: Filters = {
    space_id: "",
    connection_id: "",
    from: utcDay(from),
    to: utcDay(now),
    actor: "",
    caller: "",
    direction: "",
    purpose: "",
    action: "",
    outcome: "",
    resource: "",
    correlation: "",
  };
  const query = new URLSearchParams(location.search);
  for (const key of Object.keys(defaults) as (keyof Filters)[]) {
    const value = query.get("insight_" + key);
    if (value && value.length <= 200) defaults[key] = value;
  }
  return defaults;
}
const labels: Record<string, string> = {
  client_to_server: "客户端 → 服务端",
  server_to_client: "服务端 → 客户端",
  server_to_storage: "服务端 → 存储",
  storage_to_server: "存储 → 服务端",
  local_to_storage: "本机 → 存储",
  storage_to_local: "存储 → 本机",
  ingest: "成功入库",
  egress: "成功取用",
  transfer: "正文传输",
  verification: "校验回读",
  download: "下载",
  upload: "上传",
  effective_bytes: "有效文件量",
  observed_bytes: "观测传输量",
  failed_attempt: "失败尝试",
  authenticated_agent: "已认证 Agent",
  local_user: "本机用户",
  user: "用户",
  declared_label: "声明标签",
  quota_critical: "容量接近额度上限",
  quota_warning: "容量达到提醒线",
  collection_gap: "统计历史存在缺口",
  audit_unavailable: "审计暂不可写",
  scan_incomplete: "扫描未完整完成",
  scan_stale: "目录观测已陈旧",
  rapid_growth: "目录占用快速增长",
  complete: "完整观测",
  partial: "部分覆盖",
  failed: "失败",
  cancelled: "已取消",
  interrupted: "已中断",
  running: "扫描中",
  success: "成功",
  intent: "动作意图",
  completed: "已完成",
  forbidden: "权限拒绝",
  unauthorized: "未认证",
  unknown: "未知",
};
const label = (v: string) => labels[v] || v;
const fmtDate = (v?: string) =>
  v
    ? new Date(v).toISOString().replace("T", " ").slice(0, 19) + " UTC"
    : "未采集";
function saveJSON(value: unknown, filename: string) {
  const u = URL.createObjectURL(
    new Blob([JSON.stringify(value, null, 2)], { type: "application/json" }),
  );
  const a = document.createElement("a");
  a.href = u;
  a.download = filename;
  a.click();
  setTimeout(() => URL.revokeObjectURL(u), 1000);
}
function Ranking({
  title,
  groups,
  onSelect,
}: {
  title: string;
  groups: Group[];
  onSelect: (g: Group) => void;
}) {
  const [metric, setMetric] = useState<"observed_bytes" | "effective_bytes">("observed_bytes");
  const max = Math.max(
    1,
    ...groups.map((g) => g[metric]),
  );
  return (
    <section className="insight-card">
      <h3>{title}</h3>
      <SelectAdapter aria-label={title + "指标"} value={metric} onChange={e => setMetric(e.target.value as typeof metric)}><option value="observed_bytes">应用观测量</option><option value="effective_bytes">有效文件量</option></SelectAdapter>
      {!groups.length ? (
        <p className="insight-empty">尚无匹配的观测记录</p>
      ) : (
        [...groups].sort((a,b) => b[metric] - a[metric]).slice(0, 20).map((g) => (
          <Button variant="ghost" type="submit"
            className="insight-rank"
            key={g.kind + g.key}
            onClick={() => onSelect(g)}
          >
            <span className="rank-name">
              {g.key}
              <small>{label(g.kind)}</small>
            </span>
            <span className="rank-track">
              <i
                style={{
                  width:
                    (g[metric] / max) * 100 + "%",
                }}
              />
            </span>
            <strong>{bytes(g[metric])}</strong>
          </Button>
        ))
      )}
    </section>
  );
}
function ScanDialog({
  close,
  children,
}: {
  close: () => void;
  children: React.ReactNode;
}) {
  return <AppDialog title="启用授权目录扫描" close={close}>{children}</AppDialog>;
}

export function InsightsPanel({
  local = false,
  onNavigate,
  view,
  initialSpace,
  actorNames = {},
  connectionNames = {},
}: {
  local?: boolean;
  onNavigate?: (kind: "file" | "operation", id: string) => void;
  view?: string;
  initialSpace?: string;
  actorNames?: {users?: Record<string, string>; agents?: Record<string, string>};
  connectionNames?: Record<string, string>;
}) {
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [tab, setTab] = useState<Tab>(view === "transfers" ? "transfers" : view === "robots" ? "agents" : view === "audit" ? "audit" : "resources"),
    [filters, setFilters] = useState(() => ({ ...initialFilters(), ...(initialSpace ? {space_id: initialSpace} : {}) })),
    [summary, setSummary] = useState<Summary | null>(null),
    [series, setSeries] = useState<Point[]>([]),
    [groups, setGroups] = useState<Group[]>([]),
    [spaces, setSpaces] = useState<Space[]>([]),
    [facts, setFacts] = useState<Fact[]>([]),
    [category, setCategory] = useState("current"),
    [occupancy, setOccupancy] = useState<{
      items: {
        resource_id: string;
        version_id: string;
        name: string;
        size_bytes: number;
      }[];
      partial: boolean;
      coverage: string;
    }>({ items: [], partial: false, coverage: "" }),
    [audit, setAudit] = useState<Audit[]>([]),
    [auditDays, setAuditDays] = useState<AuditDay[]>([]),
    [before, setBefore] = useState(0),
    [next, setNext] = useState(0),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [loading, setLoading] = useState(true),
    [drill, setDrill] = useState(false),
    [groupBy, setGroupBy] = useState("actor"),
    [scanConfirm, setScanConfirm] = useState(false),
    [notice, setNotice] = useState("");
  const update = useCallback((patch: Partial<Filters>) => {
    setFilters((old) => {
      const value = { ...old, ...patch };
      const query = new URLSearchParams(location.search);
      for (const [key, v] of Object.entries(value)) {
        if (v) query.set("insight_" + key, v);
        else query.delete("insight_" + key);
      }
      const hashQuery = new URLSearchParams(location.hash.split("?")[1] || "");
      if (view && patch.space_id !== undefined) {
        if (value.space_id) hashQuery.set("space", value.space_id); else hashQuery.delete("space");
      }
      const hash = view ? location.hash.split("?")[0] + "?" + hashQuery : location.hash;
      history.replaceState(
        null,
        "",
        location.pathname + "?" + query.toString() + hash,
      );
      return value;
    });
    setBefore(0);
  }, [view]);
  const query = new URLSearchParams(
    Object.entries(filters).filter(([, v]) => !!v),
  ).toString();
  const load = useCallback(
    async (signal?: AbortSignal) => {
      // 存储浏览自带两级加载，只需要空间清单；跳过统计查询。
      if (tab === "storage") {
        const sp = await api<typeof spaces>(
          "/spaces",
          "GET",
          undefined,
          {},
          signal,
        );
        setSpaces(sp);
        return;
      }
      const group =
        tab === "agents"
          ? groupBy
          : tab === "transfers"
            ? "direction"
            : "space";
      const [s, points, ranked, sp] = await Promise.all([
        api<Summary>("/usage/summary?" + query, "GET", undefined, {}, signal),
        api<Point[]>("/usage/series?" + query, "GET", undefined, {}, signal),
        api<Group[]>(
          "/usage/breakdown?" + query + "&group=" + group,
          "GET",
          undefined,
          {},
          signal,
        ),
        api<typeof spaces>("/spaces", "GET", undefined, {}, signal),
      ]);
      if (signal?.aborted) return;
      setSummary(s);
      setSeries(points);
      setGroups(ranked);
      setSpaces(sp);

      if (tab === "audit") {
        const [page, trend] = await Promise.all([
          api<{ events: Audit[]; next_cursor: number }>(
            "/audit/events?" + query + "&before=" + before,
            "GET",
            undefined,
            {},
            signal,
          ),
          api<AuditDay[]>(
            "/audit/series?" + query,
            "GET",
            undefined,
            {},
            signal,
          ),
        ]);
        setAudit(page.events);
        setNext(page.next_cursor);
        setAuditDays(trend);
      } else if (drill && tab === "resources") {
        const out = await api<typeof occupancy>(
          "/usage/files?" + query + "&category=" + category,
          "GET",
          undefined,
          {},
          signal,
        );
        setOccupancy(out);
      } else if (drill) {
        const page = await api<{ events: Fact[]; next_cursor: number }>(
          "/usage/events?" + query + "&before=" + before,
          "GET",
          undefined,
          {},
          signal,
        );
        setFacts(page.events);
        setNext(page.next_cursor);
      }
    },
    [query, tab, before, drill, groupBy, category],
  );
  useEffect(() => {
    const controller = new AbortController();
    let running = false;
    const refresh = async () => {
      if (document.hidden || running) return;
      running = true;
      try {
        await load(controller.signal);
        setError("");
      } catch (e) {
        if (!controller.signal.aborted) { setError(errorText(e)); setSummary(null); setSpaces([]); }
      } finally {
        if (!controller.signal.aborted) setLoading(false);
        running = false;
      }
    };
    void refresh();
    const timer = setInterval(() => void refresh(), 30000);
    document.addEventListener("visibilitychange", refresh);
    return () => {
      clearInterval(timer);
      document.removeEventListener("visibilitychange", refresh);
      controller.abort();
    };
  }, [load]);
  const act = async (fn: () => Promise<void>) => {
    setBusy(true);
    setError("");
    try {
      await fn();
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  };
  const dayDrill = (day: string) => {
    update({ from: day, to: day });
    setDrill(true);
  };
  const open = (kind: "file" | "operation", id: string) => {
    if (onNavigate) onNavigate(kind, id);
    else
      setNotice(
        kind === "operation"
          ? `在已有 CLI 查看：drivebridge operation --id '${id}' --json`
          : `在已有 CLI 查看：drivebridge stat --ref 'drivebridge://file/${id}' --json`,
      );
  };
  const single =
    summary?.capacities.length === 1 ? summary.capacities[0] : null;
  const logical = single
    ? single.current_bytes + (single.history_bytes || 0) + single.trash_bytes
    : null;
  const connections = [
    ...new Set(spaces.map((s) => s.connection_id).filter(Boolean)),
  ];
  // 只在本机 view（backend 字段存在）时展示存储后端；服务端 /spaces 未携带
  // backend 前不猜测家族，保持静默。
  const backendSpaces = spaces.filter((s) => !!s.backend);
  // 存储浏览仅在本机 view 提供：服务端网页已有完整文件页。
  const tabs: [Tab, string][] = [
    ["resources", "资源概览"],
    ["transfers", "传输用量"],
    ["agents", "机器人用量"],
    ["audit", "安全审计"],
  ];
  if (local) tabs.push(["storage", "存储浏览"]);
  const dateRows = (): { day: string; values: (number | null)[] }[] => {
    const start = new Date(filters.from + "T00:00:00Z"),
      end = new Date(filters.to + "T00:00:00Z");
    const rows = [];
    if (!Number.isFinite(+start) || !Number.isFinite(+end)) return [];
    for (
      let d = start, n = 0;
      d <= end && n < 367;
      d = new Date(+d + 86400000), n++
    ) {
      const day = utcDay(d),
        point = series.find((s) => s.day === day);
      if (tab === "resources") {
        const cap = point?.capacities.length === 1 ? point.capacities[0] : null;
        rows.push({
          day,
          values: cap
            ? [cap.current_bytes, cap.history_bytes, cap.trash_bytes]
            : [null, null, null],
        });
      } else
        rows.push({
          day,
          values: point
            ? [point.effective_bytes, point.observed_bytes]
            : [null, null],
        });
    }
    return rows;
  };
  return (
    <div className="insights" aria-label="资源统计与安全审计">
      {!view && <div className="insight-tabs" role="tablist" aria-label="统计与浏览视图">
        {tabs.map(([id, title]) => (
          <Button variant="ghost" type="submit"
            key={id}
            role="tab"
            aria-selected={tab === id}
            tabIndex={tab === id ? 0 : -1}
            onKeyDown={(e) => {
              const ids = tabs.map(([tab]) => tab);
              const i = ids.indexOf(id);
              const next =
                e.key === "ArrowRight"
                  ? (i + 1) % ids.length
                  : e.key === "ArrowLeft"
                    ? (i + ids.length - 1) % ids.length
                    : e.key === "Home"
                      ? 0
                      : e.key === "End"
                        ? ids.length - 1
                        : -1;
              if (next >= 0) {
                e.preventDefault();
                e.currentTarget.closest(".insights")?.scrollIntoView?.({block:"start"});
                setTab(ids[next]);
                setBefore(0);
                setDrill(false);
                (
                  e.currentTarget.parentElement?.querySelectorAll("button")[
                    next
                  ] as HTMLButtonElement
                )?.focus();
              }
            }}
            onClick={(e) => {
              e.currentTarget.closest(".insights")?.scrollIntoView?.({block:"start"});
              setTab(id);
              setBefore(0);
              setDrill(false);
            }}
          >
            {title}
          </Button>
        ))}
        <Button variant="ghost" type="submit" className="insight-top" aria-label="回到分析顶部" onClick={e=>e.currentTarget.closest(".insights")?.scrollIntoView?.({block:"start",behavior:matchMedia("(prefers-reduced-motion: reduce)").matches?"auto":"smooth"})}>↑ 顶部</Button>
      </div>}
      {backendSpaces.length > 0 && (
        <div className="backend-strip" aria-label="存储后端">
          {backendSpaces.map((s) => {
            const backend = backendOf(s.backend);
            return (
              <span
                className={
                  "backend-chip" + (backend.unrecognized ? " unrecognized" : "")
                }
                key={s.id}
              >
                <strong>{s.name || s.id}</strong>
                <span>{backend.label}</span>
                <small>{capabilityText(backend)}</small>
              </span>
            );
          })}
        </div>
      )}
      <div className="insight-filters">
        <label>
          空间
          <SelectAdapter
            aria-label="空间"
            value={filters.space_id}
            onChange={(e) => update({ space_id: e.target.value })}
          >
            <option value="">全部授权空间</option>
            {spaces.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name || s.id}
                {s.backend ? " · " + backendOf(s.backend).label : ""}
              </option>
            ))}
          </SelectAdapter>
        </label>
        <div className="insight-advanced min-w-0">
          <Popover open={advancedOpen} onOpenChange={setAdvancedOpen}>
            <PopoverTrigger render={<Button type="button" variant="outline" />} aria-label="更多筛选"><SlidersHorizontal />更多筛选{filters.connection_id ? " · 1" : ""}</PopoverTrigger>
            <PopoverContent align="start" aria-label="高级筛选" className="w-80 max-w-[calc(100vw-2rem)] max-h-[var(--available-height)] overflow-y-auto p-4">
              <div className="mb-3 flex items-center justify-between gap-2"><PopoverTitle className="text-base">高级筛选</PopoverTitle><Button type="button" variant="ghost" size="icon" aria-label="关闭筛选" onClick={()=>setAdvancedOpen(false)}><X /></Button></div>
              <div className="grid gap-4 [&>label]:grid [&>label]:gap-2 [&>label]:min-w-0 [&>label]:text-sm [&_[data-slot=select-trigger]]:w-full">
        <label>
          存储连接
          <SelectAdapter
            aria-label="存储连接筛选"
            value={filters.connection_id}
            onChange={(e) => update({ connection_id: e.target.value })}
          >
            <option value="">全部连接</option>
            {connections.map((c) => (
              <option key={c} value={c}>{connectionNames[c] || shortIdentifier(c)}</option>
            ))}
          </SelectAdapter>
        </label>
        <label>
          开始日期（UTC）
          <Input
            type="date"
            className="[color-scheme:light] dark:[color-scheme:dark]"
            value={filters.from}
            onChange={(e) => update({ from: e.target.value })}
          />
        </label>
        <label>
          结束日期（UTC）
          <Input
            type="date"
            className="[color-scheme:light] dark:[color-scheme:dark]"
            value={filters.to}
            onChange={(e) => update({ to: e.target.value })}
          />
        </label>
              </div>
              <div className="mt-4 flex justify-end border-t border-border pt-4"><Button type="button" onClick={()=>setAdvancedOpen(false)}>完成</Button></div>
            </PopoverContent>
          </Popover>
          <span className="block text-xs text-muted-foreground">{filters.from} — {filters.to}（UTC）</span>
        </div>
        <div className="insight-period">
          {[7, 30, 90].map((days) => (
            <Button variant="ghost" type="submit"
              className="text-button"
              key={days}
              onClick={() => {
                const today = new Date(),
                  from = new Date(today);
                from.setUTCDate(from.getUTCDate() - days + 1);
                update({ from: utcDay(from), to: utcDay(today) });
              }}
            >
              {days} 天
            </Button>
          ))}
        </div>
      </div>
      <div className="insight-toolbar">
        <Button variant="ghost" type="submit"
          className="text-button"
          onClick={() => {
            const reset = Object.fromEntries(
              Object.keys(filters).map((k) => [k, ""]),
            ) as unknown as Filters;
            const now = new Date(),
              start = new Date(now);
            start.setUTCDate(start.getUTCDate() - 29);
            reset.from = utcDay(start);
            reset.to = utcDay(now);
            update(reset);
            setDrill(false);
          }}
        >
          重置筛选
        </Button>
        <span>
          {local ? "本机数据 · 不自动上传" : "当前实例 · 按身份隔离"} · UTC ·{" "}
          {summary ? "口径 " + summary.metric_version : "加载口径"}
        </span>
        <Button variant="outline" size="sm" type="submit"

          disabled={busy}
          onClick={() => act(() => load())}
        >
          刷新统计
        </Button>
        <Button variant="outline" size="sm" type="submit"

          disabled={busy}
          onClick={() =>
            act(async () => {
              const endpoint =
                tab === "audit" ? "/audit/export?" : "/usage/export?";
              const out = await api(
                endpoint + query + (tab === "audit" ? "&before=" + before : ""),
              );
              saveJSON(
                {
                  filters,
                  exported_scope:
                    tab === "audit" ? "current_page" : "selected_aggregates",
                  data: out,
                },
                "drivebridge-" + tab + ".json",
              );
            })
          }
        >
          {tab === "audit" ? "导出当前审计页" : "导出筛选统计"}
        </Button>
      </div>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {notice && (
        <p className="insight-notice" role="status">
          {notice}
        </p>
      )}
      {loading && <p role="status">正在读取已有统计；不会自动访问远端目录。</p>}
      {summary && (
        <>
          <div className="insight-kpis">
            <article>
              <span>受管逻辑用量</span>
              <strong>
                {logical === null ? "按空间查看" : bytes(logical)}
              </strong>
              <small>
                {single?.coverage === "managed_complete"
                  ? "当前版本＋历史版本＋回收站"
                  : local
                    ? "本机为已观察索引；历史字节不保证保留"
                    : "等待受管容量快照"}
              </small>
            </article>
            <article>
              <span>目录观测用量</span>
              <strong>
                {summary.observed_total_bytes === null
                  ? "未汇总"
                  : bytes(summary.observed_total_bytes)}
              </strong>
              <small>只使用完整扫描；重叠或部分覆盖不直接相加</small>
            </article>
            <article>
              <span>应用观测传输量</span>
              <strong>{bytes(summary.totals.observed_bytes)}</strong>
              <small>多个应用边界合计 · 非网络账单</small>
            </article>
            <article>
              <span>待处理提醒</span>
              <strong>{summary.alerts.length}</strong>
              <small>容量、扫描与审计状态</small>
            </article>
          </div>
          <details className="insight-caption collection-info">
            <summary>统计说明 · 最近更新 {fmtDate(summary.capacities.reduce((latest,c)=>c.observed_at > latest ? c.observed_at : latest, summary.collection.collection_started_at))}</summary>
          <p className="insight-caption">
            开始采集：{fmtDate(summary.collection.collection_started_at)}
            。未采集历史不填零。明细 90 天、日汇总 1 年；最后清理：
            {fmtDate(
              summary.collection.last_pruned_at?.startsWith("0001")
                ? undefined
                : summary.collection.last_pruned_at,
            )}
            。
          </p>
          </details>
          {summary.alerts.length > 0 && (
            <section className="insight-alerts" aria-label="待处理提醒">
              {summary.alerts.map((a, i) => (
                <Button variant="ghost" type="submit"
                  className={"insight-alert " + a.severity}
                  key={a.code + a.space_id + i}
                  onClick={() => {
                    if (a.space_id) update({ space_id: a.space_id });
                    if (a.code.startsWith("audit")) setTab("audit");
                  }}
                >
                  <strong>{label(a.code)}</strong>
                  <span>{a.space_id || "当前实例"}</span>
                </Button>
              ))}
            </section>
          )}
          {tab === "resources" && (
            <>
              {!local && spaces.find(s => s.id === filters.space_id) && <section className="capacity-page" aria-label="当前空间配额"><CapacityDonut space={spaces.find(s => s.id === filters.space_id)!} /></section>}
              <div className="insight-grid">
                <Trend
                  title="容量构成与趋势"
                  rows={dateRows()}
                  fields={["当前文件", "历史版本", "回收站"]}
                  format={bytes}
                  onDay={dayDrill}
                />
                <section className="insight-card">
                  <h3>历史快照 · 空间占用与额度</h3>
                  {summary.capacities.length === 0 && (
                    <p className="insight-empty">
                      尚无容量快照，或当前权限不允许查看整空间统计。
                    </p>
                  )}
                  {summary.capacities.map((c) => (
                    <Button layout="surface" variant="ghost" type="button"
                      className="capacity-row"
                      key={c.space_id}
                      onClick={() => {
                        update({ space_id: c.space_id });
                        setDrill(true);
                      }}
                    >
                      <strong className="min-w-0 truncate" title={spaces.find(s=>s.id===c.space_id)?.name || c.space_id}>
                        {spaces.find((s) => s.id === c.space_id)?.name ||
                          c.space_id}
                      </strong>
                      <span>
                        {bytes(
                          c.current_bytes +
                            (c.history_bytes || 0) +
                            c.trash_bytes,
                        )}
                      </span>
                      <small>
                        历史{" "}
                        {c.history_bytes === null
                          ? "未保证保留"
                          : bytes(c.history_bytes)}{" "}
                        · 回收站 {bytes(c.trash_bytes)} · 预留{" "}
                        {bytes(c.reserved_bytes)}
                      </small>
                      <small>
                        {c.quota_bytes
                          ? "额度 " + bytes(c.quota_bytes)
                          : "未提供空间总额度；不使用单文件上限"}{" "}
                      </small>
                      <small>观测时间 {fmtDate(c.observed_at)}</small>
                      <MeterBar
                        current={c.current_bytes}
                        history={c.history_bytes}
                        trash={c.trash_bytes}
                        reserved={c.reserved_bytes}
                        quota={c.quota_bytes}
                        format={bytes}
                      />
                    </Button>
                  ))}
                </section>
              </div>
              <section className="insight-card">
                <div className="insight-heading">
                  <h3>授权目录观测</h3>
                  <Button variant="outline" size="sm" type="submit"

                    disabled={!filters.space_id || busy}
                    onClick={() => setScanConfirm(true)}
                  >
                    扫描所选空间
                  </Button>
                </div>
                <p>
                  只枚举已授权范围。每日扫描仅在 owner
                  进程运行时进行；页面刷新不启动扫描。
                </p>
                <div className="insight-table-wrap">
                  <DataTable label="记录" data={summary.scans} getRowId={(s)=>String(s.id)} columns={[{id:"c0",header:<> 空间 </>,cell:(s)=><>{s.space_id}</>},
{id:"c1",header:<> 状态 </>,cell:(s)=><>
                            {label(s.status)}
                            {s.status === "running" && (
                              <Button variant="ghost" type="submit"
                                className="text-button"
                                onClick={() => {
                                  void api("/usage/scans", "POST", {
                                    space_id: s.space_id,
                                    cancel_id: s.id,
                                  })
                                    .then(() => {
                                      setNotice("已请求取消，等待扫描停止");
                                      return load();
                                    })
                                    .catch((e) => setError(errorText(e)));
                                }}
                              >
                                取消扫描
                              </Button>
                            )}
                            {s.resumable &&
                              ["partial", "interrupted"].includes(s.status) && (
                                <Button variant="ghost" type="submit"
                                  className="text-button"
                                  onClick={() =>
                                    act(async () => {
                                      await api("/usage/scans", "POST", {
                                        space_id: s.space_id,
                                        resume_id: s.id,
                                      });
                                      await load();
                                    })
                                  }
                                >
                                  继续此扫描
                                </Button>
                              )}
                          </>},
{id:"c2",header:<> 观测字节 </>,cell:(s)=><>{bytes(s.observed_bytes)}</>},
{id:"c3",header:<> 条目 </>,cell:(s)=><>{s.entries}</>},
{id:"c4",header:<> 观测时间 </>,cell:(s)=><>{fmtDate(s.finished_at || s.started_at)}</>},
{id:"c5",header:<> 覆盖 </>,cell:(s)=><>
                            {s.status === "complete"
                              ? "可见对象；不含后端隐藏版本"
                              : "不替代上次完整扫描"}
                          </>}]} />
                </div>
                {summary.scan_policies.map((p) => (
                  <div className="insight-toolbar" key={p.space_id}>
                    <span>
                      {p.space_id} · 每日扫描{p.enabled ? "已启用" : "已暂停"} ·{" "}
                      {p.max_entries} 条目／{p.max_seconds} 秒
                    </span>
                    <Button variant="ghost" type="submit"
                      className="text-button"
                      onClick={() =>
                        act(async () => {
                          await api("/usage/scans", "POST", {
                            space_id: p.space_id,
                            enabled: !p.enabled,
                            configure_only: true,
                            max_entries: p.max_entries,
                            max_seconds: p.max_seconds,
                          });
                          await load();
                        })
                      }
                    >
                      {p.enabled ? "暂停每日扫描" : "恢复每日扫描"}
                    </Button>
                  </div>
                ))}
                {summary.scans.length === 0 && (
                  <p className="insight-empty">
                    尚未扫描。启用前不会访问存储目录。
                  </p>
                )}
              </section>
            </>
          )}
          {tab === "transfers" && (
            <>
              <div className="insight-grid">
                <Trend
                  title="有效文件量与应用观测量（并列比较，不作差额结算）"
                  stacked={false}
                  rows={dateRows()}
                  fields={["有效文件量", "应用观测传输量"]}
                  format={bytes}
                  onDay={dayDrill}
                />
                <Ranking
                  title="按传输方向"
                  groups={groups.map((g) => ({ ...g, key: g.key }))}
                  onSelect={(g) => {
                    update({ direction: g.key });
                    setDrill(true);
                  }}
                />
              </div>
              <p className="insight-caption">
                校验回读、已观测重试和 SDK 签名回读可能增加观测量；SDK
                内部未经过计量边界的重试不保证被记录。
              </p>
              <div className="insight-filters">
                <label>
                  方向
                  <Input
                    value={filters.direction}
                    onChange={(e) => update({ direction: e.target.value })}
                    placeholder="例如 server_to_storage"
                  />
                </label>
                <label>
                  用途
                  <SelectAdapter
                    value={filters.purpose}
                    onChange={(e) => {
                      update({ purpose: e.target.value });
                      setDrill(true);
                    }}
                  >
                    <option value="">全部用途</option>
                    {[
                      "transfer",
                      "verification",
                      "download",
                      "upload",
                      "import",
                    ].map((p) => (
                      <option key={p} value={p}>
                        {label(p)}
                      </option>
                    ))}
                  </SelectAdapter>
                </label>
                <Button variant="outline" size="sm" type="submit"  onClick={() => setDrill(true)}>
                  查看传输事实
                </Button>
              </div>
            </>
          )}
          {tab === "agents" && (
            <>
              <div className="insight-filters">
                <label>
                  归因口径
                  <SelectAdapter
                    value={groupBy}
                    onChange={(e) => setGroupBy(e.target.value)}
                  >
                    <option value="actor">已认证身份／本机用户</option>
                    <option value="caller">声明调用方标签</option>
                  </SelectAdapter>
                </label>
                <label>
                  身份 ID
                  <Input
                    value={filters.actor}
                    onChange={(e) => update({ actor: e.target.value })}
                  />
                </label>
                <label>
                  声明调用方
                  <Input
                    value={filters.caller}
                    onChange={(e) => update({ caller: e.target.value })}
                  />
                </label>
              </div>
              <p className="insight-caption">
                caller 是调用方声明，不能证明是哪一个
                Agent。未经认证的本机调用不会出现在“已认证 Agent”名下。
              </p>
              <Ranking
                title="用量归因"
                groups={groups}
                onSelect={(g) => {
                  update(
                    groupBy === "actor" ? { actor: g.key } : { caller: g.key },
                  );
                  setDrill(true);
                }}
              />
            </>
          )}
          {tab === "audit" && (
            <>
              <div className="insight-filters">
                <label>
                  动作
                  <Input
                    value={filters.action}
                    onChange={(e) => update({ action: e.target.value })}
                    placeholder="例如 agents.revoke"
                  />
                </label>
                <label>
                  结果
                  <Input
                    value={filters.outcome}
                    onChange={(e) => update({ outcome: e.target.value })}
                    placeholder="例如 forbidden"
                  />
                </label>
                <label>
                  资源 ID
                  <Input
                    value={filters.resource}
                    onChange={(e) => update({ resource: e.target.value })}
                  />
                </label>
                <label>
                  关联号
                  <Input
                    value={filters.correlation}
                    onChange={(e) => update({ correlation: e.target.value })}
                  />
                </label>
              </div>
              <section className="insight-card">
                <h3>安全事件趋势</h3>
                <p>
                  实例匿名失败不归属到用户。当前页面只显示此身份可见的事件；记录不提供防篡改保证。
                </p>
                <Trend
                  title="每日安全事件"
                  fields={["成功结果", "动作意图", "拒绝与其他结果"]}
                  format={(n) => n + " 次"}
                  onDay={dayDrill}
                  rows={[...new Set(auditDays.map((d) => d.day))]
                    .sort()
                    .map((day) => ({
                      day,
                      values: [
                        auditDays
                          .filter(
                            (d) =>
                              d.day === day &&
                              (d.outcome === "success" ||
                                d.outcome === "completed"),
                          )
                          .reduce((n, d) => n + d.count, 0),
                        auditDays
                          .filter(
                            (d) => d.day === day && d.outcome === "intent",
                          )
                          .reduce((n, d) => n + d.count, 0),
                        auditDays
                          .filter(
                            (d) =>
                              d.day === day &&
                              !["success", "completed", "intent"].includes(
                                d.outcome,
                              ),
                          )
                          .reduce((n, d) => n + d.count, 0),
                      ],
                    }))}
                />
                {auditDays.length === 0 && (
                  <p className="insight-empty">
                    当前范围没有可显示的聚合；空间级或部分权限查询不推断全局安全事件。
                  </p>
                )}
              </section>
              <section className="insight-card">
                <h3>事件时间线</h3>
                <div className="insight-table-wrap">
                  <DataTable label="记录" data={audit} getRowId={(a)=>String(a.id)} columns={[{id:"c0",header:<> 时间（UTC） </>,cell:(a)=><>{fmtDate(a.at)}</>},
{id:"c1",header:<> 身份 </>,value:a=>`${auditActorName(a.actor_kind,a.actor_id,actorNames)} ${a.actor_kind} ${a.actor_id || ""}`,cell:(a)=><>
                            <strong className="block max-w-48 truncate" title={auditActorName(a.actor_kind,a.actor_id,actorNames)}>{auditActorName(a.actor_kind,a.actor_id,actorNames)}</strong>
                            <small title={a.actor_id}>{label(a.actor_kind)} · {a.actor_id ? shortIdentifier(a.actor_id) : "未知"}</small>
                          </>},
{id:"c2",header:<> 动作／结果 </>,value:a=>`${auditActionLabel(a.action)} ${a.action} ${label(a.outcome)}`,cell:(a)=><>
                            <span title={a.action}>{auditActionLabel(a.action)}</span>
                            <small>{label(a.outcome)}</small>
                          </>},
{id:"c3",header:<> 关联与资源 </>,value:a=>[a.correlation_id,a.operation_id,a.resource_id].filter(Boolean).join(" "),cell:(a)=><>
                            <code title={a.correlation_id}>{shortIdentifier(a.correlation_id)}</code>
                            {a.operation_id && (
                              <Button variant="ghost" type="submit"
                                className="text-button"
                                onClick={() =>
                                  open("operation", a.operation_id!)
                                }
                              >
                                关联操作
                              </Button>
                            )}
                            {a.resource_id && a.resource_kind === "file" && (
                              <Button variant="ghost" type="submit"
                                className="text-button"
                                onClick={() => open("file", a.resource_id!)}
                              >
                                查看当前可见资源 {a.resource_id.slice(0, 8)}
                              </Button>
                            )}
                          </>}]} />
                </div>
                {audit.length === 0 && (
                  <p className="insight-empty">
                    没有匹配记录；未采集历史不能推断为没有发生。
                  </p>
                )}
                {next > 0 && (
                  <Button variant="outline" size="sm" type="submit"

                    onClick={() => setBefore(next)}
                  >
                    更早事件
                  </Button>
                )}
                {before > 0 && (
                  <Button variant="ghost" type="submit" className="text-button" onClick={() => setBefore(0)}>
                    返回最新
                  </Button>
                )}
              </section>
            </>
          )}
          {tab === "storage" && <StorageBrowser spaces={spaces} />}
          {drill && tab === "resources" && (
            <section className="insight-card">
              <h3>容量占用明细</h3>
              <label>
                占用分类
                <SelectAdapter
                  value={category}
                  onChange={(e) => setCategory(e.target.value)}
                >
                  <option value="current">当前版本</option>
                  <option value="history">历史版本</option>
                  <option value="trash">回收站</option>
                </SelectAdapter>
              </label>
              <p>名称按当前权限解析；本机历史引用不代表保留了历史字节。</p>
              <div className="insight-table-wrap">
                <DataTable label="记录" data={occupancy.items} getRowId={(f)=>String(f.resource_id + f.version_id)} columns={[{id:"c0",header:<> 当前可见名称 </>,cell:(f)=><>{f.name}</>},
{id:"c1",header:<> 逻辑字节 </>,cell:(f)=><>{bytes(f.size_bytes)}</>},
{id:"c2",header:<> 资源 </>,cell:(f)=><>
                          <Button variant="ghost" type="submit"
                            className="text-button"
                            onClick={() => open("file", f.resource_id)}
                          >
                            打开已有文件入口
                          </Button>
                        </>}]} />
              </div>
              {!occupancy.items.length && (
                <p>没有可展示的记录，或此口径不适用于当前空间。</p>
              )}
              {occupancy.partial && (
                <p>当前仅显示前 200 条，请缩小空间或连接范围。</p>
              )}
            </section>
          )}
          {drill && tab !== "audit" && tab !== "resources" && (
            <section className="insight-card">
              <h3>匹配的传输事实</h3>
              <p>
                只保留最近 90 天的明细。更早的日汇总可能仍在，但不伪造操作列表。
              </p>
              <div className="insight-table-wrap">
                <DataTable label="记录" data={facts} getRowId={(f)=>String(f.sequence)} columns={[{id:"c0",header:<> 时间 </>,cell:(f)=><>{fmtDate(f.at)}</>},
{id:"c1",header:<> 口径 </>,cell:(f)=><>{label(f.metric)}</>},
{id:"c2",header:<> 方向／用途 </>,cell:(f)=><>
                          {label(f.direction || "")} · {label(f.purpose)}
                        </>},
{id:"c3",header:<> 字节 </>,cell:(f)=><>{bytes(f.bytes)}</>},
{id:"c4",header:<> 出处 </>,cell:(f)=><>
                          {f.caller || f.actor_id || "未知"}
                          {f.operation_id && (
                            <Button variant="ghost" type="submit"
                              className="text-button"
                              onClick={() => open("operation", f.operation_id!)}
                            >
                              查看原操作
                            </Button>
                          )}
                          {!f.operation_id && f.resource_id && (
                            <Button variant="ghost" type="submit"
                              className="text-button"
                              onClick={() => open("file", f.resource_id!)}
                            >
                              查看资源
                            </Button>
                          )}
                        </>}]} />
              </div>
              {facts.length === 0 && (
                <p className="insight-empty">没有匹配明细。</p>
              )}
              {next > 0 && (
                <Button variant="outline" size="sm" type="submit"

                  onClick={() => setBefore(next)}
                >
                  更早记录
                </Button>
              )}
            </section>
          )}
        </>
      )}
      {scanConfirm && (
        <ScanDialog close={() => setScanConfirm(false)}>
          <h3>扫描所选授权空间</h3>
          <p>
            将启用每日元数据扫描，并立即运行一次。默认上限为 10 分钟或 10
            万条目；不读取文件正文、不删除文件。后端可能产生列目录请求费用。
          </p>
          <Button variant="default" type="submit"

            disabled={busy}
            onClick={() =>
              act(async () => {
                setScanConfirm(false);
                const result = await api<Scan>("/usage/scans", "POST", {
                  space_id: filters.space_id,
                  enabled: true,
                  max_entries: 100000,
                  max_seconds: 600,
                });
                setNotice("扫描结果：" + label(result.status));
                await load();
              })
            }
          >
            启用并扫描
          </Button>
          <Button variant="outline" type="submit"  onClick={() => setScanConfirm(false)}>
            取消
          </Button>
        </ScanDialog>
      )}
    </div>
  );
}
