import type { State, FileRecord } from "./model";
import { fileUnavailable } from "./v4-model";

// Resettable UI projection. No provider credentials or production indexing engine.
export interface DirectoryNode {
  id: string;
  space: State["space"];
  connectionId: string;
  rootId: string;
  parentId: string | null;
  name: string;
  path: string;
  status: "unloaded" | "loading" | "ready" | "partial" | "failed" | "forbidden";
}
export interface IndexRun {
  id: string;
  directoryId: string;
  connectionId: string;
  space: State["space"];
  generation: number;
  status:
    | "queued"
    | "running"
    | "complete"
    | "partial"
    | "failed"
    | "unknown"
    | "cancelled";
  observed: string[];
  failed: string[];
  total: number | null;
  checked: boolean;
  checkpoint: boolean;
  observedAt: string | null;
  revision: number;
}
export interface IndexScope {
  directoryId: string;
  generation: number;
  exclusions: string[];
  policy: "manual" | "changes" | "daily";
  budget: number;
  paused: boolean;
  confirmed: boolean;
  lastComplete?: string;
  lastRun?: string;
}
export interface StorageState {
  nodes: DirectoryNode[];
  activeId: string;
  expanded: string[];
  treeWidth: number;
  positions: Record<string, { id: string; scroll: number }>;
  scopes: Record<string, IndexScope>;
  runs: IndexRun[];
  next: number;
  tab: "connections" | "indexes" | "usage";
  inspect: string;
  run: string;
  taskConnection: string;
  usage: {
    connectionId: string;
    space: State["space"];
    actor: string;
    revision: number;
    bytes: number;
    count: number;
    at: string;
  }[];
}
export const emptyStorage = (): StorageState => ({
  nodes: [],
  activeId: "",
  expanded: ["personal:drive:root"],
  treeWidth: 220,
  positions: {},
  scopes: {},
  runs: [],
  next: 1,
  tab: "connections",
  inspect: "",
  run: "",
  taskConnection: "",
  usage: [],
});
export function rootNode(s: State, connectionId: string, space = s.space) {
  return s.storage.nodes.find(
    (n) =>
      n.connectionId === connectionId &&
      n.space === space &&
      n.parentId === null,
  );
}
export function directoryContains(
  s: State,
  parent: string,
  child: string,
): boolean {
  const seen = new Set<string>();
  let current: string | null = child;
  while (current && !seen.has(current)) {
    if (current === parent) return true;
    seen.add(current);
    current = s.storage.nodes.find((n) => n.id === current)?.parentId || null;
  }
  return false;
}
export function directoryFiles(s: State, id: string, recursive = false) {
  return s.files.filter(
    (f) =>
      f.space === s.space &&
      !f.trashed &&
      !f.inbox &&
      f.directoryId &&
      (recursive
        ? directoryContains(s, id, f.directoryId)
        : f.directoryId === id) &&
      !fileUnavailable(s, f),
  );
}
export function directoryAvailable(s: State, node?: DirectoryNode): string {
  if (!node || node.space !== s.space) return "目录不可用";
  const c = s.connections.find((c) => c.id === node.connectionId);
  if (!c || !c.authorized || c.status !== "active")
    return "连接不可用，请重新授权或连接";
  if (node.status === "forbidden" || s.viewState === "forbidden")
    return "没有读取此目录的权限";
  if (
    c.kind !== "drive" &&
    node.path !== c.root &&
    !node.path.startsWith(c.root + " / ")
  )
    return "目录已不在当前授权根范围内";
  if (c.read !== "passed") return "读取能力尚未验证";
  return "";
}
export function ensureDirectories(s: State): State {
  const add = (
    space: State["space"],
    connectionId: string,
    path: string,
  ): DirectoryNode => {
    const c = s.connections.find((c) => c.id === connectionId)!;
    const rootId = `${space}:${connectionId}:root`;
    let root = s.storage.nodes.find((n) => n.id === rootId);
    if (!root) {
      root = {
        id: rootId,
        space,
        connectionId,
        rootId: "root",
        parentId: null,
        name: c.name,
        path: c.root,
        status: "ready",
      };
      s.storage.nodes.push(root);
    }
    root.name = c.name;
    root.path = c.root;
    const relative =
      c.root === "全部文件"
        ? path === "全部文件"
          ? ""
          : path
        : path === c.root
          ? ""
          : path.startsWith(c.root + " / ")
            ? path.slice(c.root.length + 3)
            : path;
    let parent = root;
    let cumulative = c.root === "全部文件" ? "" : c.root;
    for (const segment of relative.split(" / ").filter(Boolean)) {
      cumulative = cumulative ? cumulative + " / " + segment : segment;
      let node = s.storage.nodes.find(
        (n) =>
          n.space === space &&
          n.connectionId === connectionId &&
          n.path === cumulative &&
          n.parentId === parent.id,
      );
      if (!node) {
        node = {
          id: `${rootId}:${encodeURIComponent(cumulative)}`,
          space,
          connectionId,
          rootId: "root",
          parentId: parent.id,
          name: segment,
          path: cumulative,
          status: "unloaded",
        };
        s.storage.nodes.push(node);
      }
      parent = node;
    }
    return parent;
  };
  for (const space of ["personal", "team"] as const)
    for (const c of s.connections.filter(
      (c) => c.id === "drive" || c.space === space,
    ))
      add(space, c.id, c.root);
  for (const f of s.files) {
    const c = s.connections.find((c) => c.id === (f.connectionId || "drive"));
    if (!c) continue;
    const old = s.storage.nodes.find((n) => n.id === f.directoryId);
    if (
      !old ||
      old.path !== f.folder ||
      old.connectionId !== c.id ||
      old.space !== f.space
    )
      f.directoryId = add(f.space, c.id, f.folder).id;
  }
  for (const f of s.folders)
    if (f.connectionId && s.connections.some((c) => c.id === f.connectionId))
      add(f.space, f.connectionId, f.path);
  // The original display paths are kept only as compatibility projections.
  s.folders = s.storage.nodes
    .filter((n) => n.parentId !== null)
    .map((n) => ({
      path: n.path,
      space: n.space,
      connectionId: n.connectionId,
    }));
  if (
    !s.storage.nodes.some(
      (n) =>
        n.id === s.storage.activeId &&
        n.space === s.space &&
        n.connectionId === s.activeConnection,
    )
  ) {
    const saved = s.storage.positions[`${s.space}:${s.activeConnection}`];
    s.storage.activeId =
      saved?.id ||
      rootNode(s, s.activeConnection)?.id ||
      rootNode(s, "drive")!.id;
  }
  return s;
}
export function initStorage(s: State) {
  ensureDirectories(s);
  // Explicit empty and deep directories exercise hierarchy, without copying files.
  for (const cid of ["od-personal", "od-work"]) {
    const root = rootNode(s, cid, "personal")!;
    s.storage.nodes.push({
      ...root,
      id: root.id + ":projects",
      parentId: root.id,
      name: "项目",
      path: root.path + " / 项目",
      status: "unloaded",
    });
    s.storage.nodes.push({
      ...root,
      id: root.id + ":empty",
      parentId: root.id + ":projects",
      name: "待归档",
      path: root.path + " / 项目 / 待归档",
      status: "unloaded",
    });
  }
  return ensureDirectories(s);
}
export type StorageAction =
  | { type: "usage-observe"; connectionId: string }
  | { type: "directory-open"; id: string; scroll?: number; focus?: string }
  | { type: "directory-expand"; id: string }
  | { type: "directory-status"; id: string; status: DirectoryNode["status"] }
  | { type: "directory-width"; width: number }
  | {
      type: "storage-page";
      tab: StorageState["tab"];
      directoryId?: string;
      runId?: string;
    }
  | { type: "storage-transfer"; connectionId: string }
  | {
      type: "catalog-configure";
      directoryId: string;
      exclusions: string[];
      budget: number;
      policy: IndexScope["policy"];
    }
  | { type: "catalog-start"; directoryId: string }
  | {
      type: "catalog-complete";
      id: string;
      generation: number;
      result: "complete" | "partial" | "failed" | "unknown";
    }
  | { type: "catalog-query"; id: string }
  | { type: "catalog-resume"; id: string }
  | { type: "catalog-pause"; directoryId: string }
  | { type: "catalog-clear"; directoryId: string };
export function storageReduce(state: State, a: StorageAction): State | null {
  if (
    !a.type.startsWith("directory-") &&
    !a.type.startsWith("catalog-") &&
    !["storage-page", "storage-transfer", "usage-observe"].includes(a.type)
  )
    return null;
  const s = structuredClone(state);
  const st = s.storage;
  const note = (notice: string) => ({ ...s, notice });
  if (a.type === "usage-observe") {
    const n = rootNode(s, a.connectionId),
      c = s.connections.find((c) => c.id === a.connectionId);
    if (!s.online || directoryAvailable(s, n) || !c)
      return note("连接不可用，未记录新的容量观测。");
    const files = directoryFiles(s, n!.id, true);
    st.usage.push({
      connectionId: a.connectionId,
      space: s.space,
      actor: s.actor,
      revision: c.revision,
      bytes: files.reduce((sum, f) => sum + bytes(f.size), 0),
      count: files.length,
      at: new Date().toISOString(),
    });
    return note("已记录当前授权样本容量观测；不代表全盘配额或账单。");
  }

  if (a.type === "storage-page") {
    st.tab = a.tab;
    st.inspect = a.directoryId || "";
    st.run = a.runId || "";
    s.page = "connections";
    return s;
  }
  if (a.type === "storage-transfer") {
    st.taskConnection = a.connectionId;
    s.page = "tasks";
    return s;
  }
  if (a.type === "directory-width") {
    st.treeWidth = Math.max(200, Math.min(320, a.width));
    return s;
  }
  if (a.type === "directory-open") {
    const n = st.nodes.find((n) => n.id === a.id);
    if (!n || n.space !== s.space) return note("目录不属于当前空间。");
    st.positions[`${s.space}:${s.activeConnection}`] = {
      id: st.activeId,
      scroll: a.scroll || 0,
    };
    s.connectionBrowsers[`${s.space}:${s.activeConnection}`] = {
      browser: structuredClone(s.browser),
      scroll: a.scroll || 0,
    };
    const old = s.connectionBrowsers[`${s.space}:${n.connectionId}`];
    if (n.connectionId !== s.activeConnection && old)
      s.browser = structuredClone(old.browser);
    s.activeConnection = n.connectionId;
    st.activeId = n.id;
    s.browser.folder = n.path;
    s.browser.page = 1;
    s.browser.back.push(n.id);
    s.selected = [];
    s.page = "files";
    if (n.status === "unloaded") n.status = "loading";
    return note("已切换目录，批量选择已清除。");
  }
  if (a.type === "directory-expand") {
    const n = st.nodes.find((n) => n.id === a.id);
    if (!n || n.space !== s.space) return s;
    if (directoryAvailable(s, n)) return note(directoryAvailable(s, n));
    st.expanded = st.expanded.includes(a.id)
      ? st.expanded.filter((x) => x !== a.id)
      : [...st.expanded, a.id];
    if (n.status === "unloaded") n.status = "loading";
    return s;
  }
  if (a.type === "directory-status") {
    const n = st.nodes.find((n) => n.id === a.id);
    if (n && n.space === s.space) n.status = a.status;
    return s;
  }
  const id =
    "directoryId" in a
      ? a.directoryId
      : st.runs.find((r) => r.id === a.id)?.directoryId;
  const node = st.nodes.find((n) => n.id === id);
  if (!node || node.space !== s.space) return note("索引范围不可用。");
  if (directoryAvailable(s, node)) return note(directoryAvailable(s, node));
  const c = s.connections.find((c) => c.id === node.connectionId)!;
  if (a.type === "catalog-clear") {
    const scope = st.scopes[node.id];
    if (scope) {
      scope.generation++;
      scope.confirmed = false;
      scope.lastComplete = undefined;
      scope.lastRun = undefined;
    }
    return note("目录索引已清除，原文件与运行记录保留。");
  }
  if (!s.online) return note("等待网络；未提交新的扫描。");
  if (a.type === "catalog-configure") {
    if (s.actor !== "yeisme") return note("当前身份不能配置索引范围。");
    if (a.policy === "changes" && c.kind === "s3")
      return note("此连接未提供原生变化流，请选择手动或定时扫描。");
    if (!Number.isFinite(a.budget) || a.budget < 1)
      return note("请输入有效扫描预算。");
    const old = st.scopes[node.id];
    st.scopes[node.id] = {
      directoryId: node.id,
      generation: (old?.generation || 0) + 1,
      exclusions: a.exclusions,
      budget: Math.min(100000, a.budget),
      policy: a.policy,
      paused: false,
      confirmed: true,
      lastComplete: old?.lastComplete,
    };
    return note("已确认目录元数据扫描范围；未启用正文分析。");
  }
  const scope = st.scopes[node.id];
  if (!scope?.confirmed) return note("请先确认目录、排除项与扫描预算。");
  if (a.type === "catalog-pause") {
    scope.paused = !scope.paused;
    return note(
      scope.paused
        ? "已暂停后续更新；正在执行的原扫描仍可查询。"
        : "已恢复后续更新策略；没有重复启动任务。",
    );
  }
  if (a.type === "catalog-start") {
    if (scope.paused) return note("更新策略已暂停，请先恢复。");
    const running = st.runs.find(
      (r) =>
        r.directoryId === node.id &&
        ["running", "queued", "unknown"].includes(r.status) &&
        r.generation === scope.generation,
    );
    if (running) {
      st.run = running.id;
      return note("已有原扫描任务，请查询其结果。");
    }
    const id = `SCAN-${st.next++}`;
    st.runs.unshift({
      id,
      directoryId: node.id,
      connectionId: c.id,
      space: s.space,
      generation: scope.generation,
      revision: c.revision,
      status: "running",
      observed: [],
      failed: [],
      total: null,
      checked: false,
      checkpoint: c.kind === "s3",
      observedAt: null,
    });
    scope.lastRun = id;
    st.run = id;
    return note(`扫描 ${id} 已开始（演示）。`);
  }
  if (!("id" in a)) return s;
  const run = st.runs.find((r) => r.id === a.id);
  if (
    !run ||
    run.generation !== scope.generation ||
    run.revision !== c.revision
  )
    return note("旧扫描范围或连接修订已失效；忽略迟到结果。");
  if (a.type === "catalog-query") {
    run.checked = true;
    if (run.status === "unknown") run.status = "partial";
    return note(`已查询原操作 ${run.id}；未重新执行。`);
  }
  if (a.type === "catalog-resume") {
    if (!run.checked) return note("先查询原扫描结果。");
    if (!run.checkpoint)
      return note("此连接不支持检查点续扫，请显式重新扫描。");
    if (!["partial", "failed"].includes(run.status)) return s;
    run.status = "running";
    run.checked = false;
    return note(`续扫原任务 ${run.id}，保留已完成项。`);
  }
  if (a.type === "catalog-complete") {
    if (a.generation !== scope.generation || run.status !== "running") return s;
    const files = directoryFiles(s, node.id, true).filter(
      (f) =>
        !scope.exclusions.some((x) => x && `${f.folder}/${f.name}`.includes(x)),
    );
    const nodes = st.nodes.filter(
      (n) =>
        n.id !== node.id &&
        directoryContains(s, node.id, n.id) &&
        !scope.exclusions.some((x) => x && n.path.includes(x)),
    );
    const entries = [...nodes.map((n) => n.id), ...files.map((f) => f.id)];
    const limited = entries.slice(0, scope.budget);
    run.status =
      a.result === "complete" && entries.length > scope.budget
        ? "partial"
        : a.result;
    run.failed = run.status === "partial" ? limited.slice(-1) : [];
    run.observed = run.status === "complete" ? limited : limited.slice(0, -1);
    run.total = run.status === "complete" ? entries.length : null;
    run.observedAt = "本次演示 · 刚刚";
    if (run.status === "complete") scope.lastComplete = run.id;
    return note(
      run.status === "complete"
        ? "当前授权范围扫描完成。"
        : "扫描尚不完整；保留最近完整结果，不推导文件删除。",
    );
  }
  return s;
}
export function bytes(size: string) {
  const n = parseFloat(size) || 0;
  return (
    n *
    (/GB/i.test(size)
      ? 1e9
      : /MB/i.test(size)
        ? 1e6
        : /KB/i.test(size)
          ? 1e3
          : 1)
  );
}
export function isObserved(s: State, f: FileRecord) {
  if (f.connectionId === "drive") return true;
  return Object.values(s.storage.scopes).some(
    (scope) =>
      scope.confirmed &&
      [scope.lastRun, scope.lastComplete].some((id) =>
        s.storage.runs.find((r) => r.id === id)?.observed.includes(f.id),
      ),
  );
}
