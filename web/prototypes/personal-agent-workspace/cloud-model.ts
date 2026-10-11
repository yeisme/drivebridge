import { directoryAvailable, directoryContains, bytes } from "./storage-model";
import { fileUnavailable } from "./v4-model";
import type { FileRecord, Space, State } from "./model";

export type TagSource = "manual" | "rule" | "suggested";
export interface TagAssignment {
  tagId: string;
  source: TagSource;
}
export interface CloudTag {
  id: string;
  space: Space;
  name: string;
  group: string;
  color: string;
  favorite: boolean;
}
export interface Preferences {
  theme: "system" | "light" | "dark";
  density: "comfortable" | "compact";
  layout: "list" | "grid" | "gallery";
  startup: "files" | "home";
  reduceMotion: boolean;
  agentWidth: number;
  agentRememberOpen: boolean;
  navCollapsed: boolean;
  previewListWidth: number;
  inspectorPosition: "auto" | "right" | "bottom";
}
export const defaultPreferences: Preferences = {
  theme: "system",
  density: "comfortable",
  layout: "list",
  startup: "files",
  reduceMotion: false,
  agentWidth: 360,
  agentRememberOpen: false,
  navCollapsed: false,
  previewListWidth: 220,
  inspectorPosition: "auto",
};
export const preferenceKey = "drivebridge-prototype-v3-preferences";
export function safePreferences(input: unknown): Preferences {
  const p = (
    input && typeof input === "object" ? input : {}
  ) as Partial<Preferences>;
  return {
    theme: ["system", "light", "dark"].includes(p.theme || "")
      ? p.theme!
      : "system",
    density: p.density === "compact" ? "compact" : "comfortable",
    layout: ["list", "grid", "gallery"].includes(p.layout || "")
      ? p.layout!
      : "list",
    startup: p.startup === "home" ? "home" : "files",
    reduceMotion: p.reduceMotion === true,
    agentWidth:
      typeof p.agentWidth === "number" && Number.isFinite(p.agentWidth)
        ? Math.min(480, Math.max(320, p.agentWidth))
        : 360,
    agentRememberOpen: p.agentRememberOpen === true,
    navCollapsed: p.navCollapsed === true,
    previewListWidth:
      typeof p.previewListWidth === "number" &&
      Number.isFinite(p.previewListWidth)
        ? Math.max(200, Math.min(320, p.previewListWidth))
        : 220,
    inspectorPosition: p.inspectorPosition === "right" ? "right" : "auto",
  };
}
export interface SearchSpec {
  propertyIssue?: {
    key: string;
    state: "type" | "unscanned" | "invalid" | "stale";
  };
  onlyFavorites?: boolean;
  favoriteGroup?: string;
  favoriteDirectories?: boolean;
  documentTag?: string;
  exactTag?: boolean;
  propertyConditions?: import("./organization-model").PropertyCondition[];
  view?: {
    columns: string[];
    widths: Record<string, number>;
    groupBy: string;
    sortKey: string;
    listSort?: string;
  };
  directoryId?: string;
  recursive?: boolean;
  minSize?: number;
  maxSize?: number;
  fileIds?: string[];
  query: string;
  tab: "files" | "content" | "tags" | "settings";
  folder: string;
  connections: string[];
  tagIds: string[];
  tagMode: "all" | "any" | "exclude";
  kind: string;
  updated: string;
  history: boolean;
}
export const emptySearch = (): SearchSpec => ({
  query: "",
  tab: "files",
  folder: "全部文件",
  connections: ["drive"],
  tagIds: [],
  tagMode: "all",
  kind: "all",
  updated: "all",
  history: false,
});
export interface IndexJob {
  directoryId?: string;
  connectionId?: string;
  id: string;
  space: Space;
  folder: string;
  exclusions: string[];
  kinds: string[];
  policy: "manual" | "changes";
  status: "queued" | "processing" | "ready" | "partial" | "paused" | "stale";
  refs: { id: string; version: number }[];
  failed: string[];
  observedAt: string | null;
}
export interface ExternalApp {
  directoryId?: string;
  connectionId?: string;
  id: string;
  name: string;
  protocol: "MCP" | "API";
  space: Space;
  folder: string;
  actions: string[];
  hours: number;
  state: "configured" | "verified" | "paused" | "revoked" | "expired";
  observedAt: string | null;
  writeVerified: boolean;
}
export interface CallRecord {
  id: string;
  appId: string;
  space: Space;
  action: string;
  scope: string;
  operationId: string;
  result: "done" | "unknown" | "denied";
  duration: string;
  outputId?: string;
}
export interface CloudState {
  tags: CloudTag[];
  tagGroups: { space: Space; name: string }[];
  nextTag: number;
  preferences: Preferences;
  agentOpen: boolean;
  agentTab: "chat" | "search" | "tasks";
  search: SearchSpec;
  searchSubmitted: boolean;
  nextSavedSearch: number;
  savedSearches: { id: string; space: Space; name: string; spec: SearchSpec }[];
  indexes: IndexJob[];
  nextIndex: number;
  apps: ExternalApp[];
  calls: CallRecord[];
  nextApp: number;
  nextCall: number;
  settingSection: string;
  settingsQuery: string;
  settingsNotice: string;
  capabilityFocus: string;
  options: {
    uploadTarget: string;
    wifiOnly: boolean;
    notifyTransfers: boolean;
    notifyTasks: boolean;
    searchFolder: string;
    agentFolder: string;
    showSuggestions: boolean;
  };
}
export function cloudInitial(files: FileRecord[]): CloudState {
  const tags: CloudTag[] = [
    {
      id: "t-project",
      space: "personal",
      name: "项目资料",
      group: "项目",
      color: "pine",
      favorite: true,
    },
    {
      id: "t-outdoor",
      space: "personal",
      name: "自然",
      group: "主题",
      color: "blue",
      favorite: false,
    },
    {
      id: "t-delivery",
      space: "personal",
      name: "交付",
      group: "状态",
      color: "amber",
      favorite: true,
    },
    {
      id: "t-review",
      space: "personal",
      name: "待审阅",
      group: "状态",
      color: "rose",
      favorite: false,
    },
    {
      id: "t-team",
      space: "team",
      name: "共同创作",
      group: "项目",
      color: "pine",
      favorite: true,
    },
  ];
  files.forEach((f) => {
    f.tags = f.label ? [{ tagId: "t-project", source: "manual" }] : [];
    if (f.id === "f1") f.tags.push({ tagId: "t-delivery", source: "manual" });
    if (f.id === "f2")
      f.tags.push(
        { tagId: "t-outdoor", source: "manual" },
        { tagId: "t-review", source: "suggested" },
      );
    if (f.id === "f3") f.tags.push({ tagId: "t-project", source: "rule" });
    if (f.space === "team") f.tags.push({ tagId: "t-team", source: "manual" });
  });
  return {
    tags,
    tagGroups: (["personal", "team"] as Space[]).flatMap((space) =>
      ["项目", "主题", "状态"].map((name) => ({ space, name })),
    ),
    nextTag: 6,
    preferences: { ...defaultPreferences },
    agentOpen: false,
    agentTab: "chat",
    search: emptySearch(),
    searchSubmitted: false,
    savedSearches: [],
    nextSavedSearch: 1,
    indexes: [],
    nextIndex: 1,
    apps: [],
    calls: [],
    nextApp: 1,
    nextCall: 1,
    settingSection: "",
    settingsQuery: "",
    settingsNotice: "",
    capabilityFocus: "",
    options: {
      uploadTarget: "待整理",
      wifiOnly: true,
      notifyTransfers: true,
      notifyTasks: true,
      searchFolder: "全部文件",
      agentFolder: "已选文件",
      showSuggestions: true,
    },
  };
}
export function assignedTags(file: FileRecord, suggestions = false): string[] {
  return [
    ...new Set(
      (file.tags || [])
        .filter((t) => suggestions || t.source !== "suggested")
        .map((t) => t.tagId),
    ),
  ];
}
export function matchesTags(
  file: FileRecord,
  ids: string[] = [],
  mode = "all",
): boolean {
  const current = assignedTags(file);
  return (
    !ids.length ||
    (mode === "exclude"
      ? ids.every((id) => !current.includes(id))
      : mode === "any"
        ? ids.some((id) => current.includes(id))
        : ids.every((id) => current.includes(id)))
  );
}
export const fileConnection = (f: FileRecord) => f.connectionId || "drive";
export function scopeFiles(s: State, q: SearchSpec): FileRecord[] {
  return s.files.filter(
    (f) =>
      f.space === s.space &&
      !f.trashed &&
      (q.fileIds === undefined || q.fileIds.includes(f.id)) &&
      !f.inbox &&
      q.connections.includes(fileConnection(f)) &&
      !fileUnavailable(s, f) &&
      (!q.directoryId ||
        (f.directoryId &&
          (q.recursive === false
            ? f.directoryId === q.directoryId
            : directoryContains(s, q.directoryId, f.directoryId)))) &&
      (q.minSize === undefined || bytes(f.size) >= q.minSize) &&
      (q.maxSize === undefined || bytes(f.size) <= q.maxSize) &&
      (q.directoryId ||
        q.folder === "全部文件" ||
        f.folder === q.folder ||
        f.folder.startsWith(q.folder + " / ")) &&
      (q.kind === "all" ||
        f.kind === q.kind ||
        (q.kind === "pdf" && f.name.toLowerCase().endsWith(".pdf"))) &&
      matchesTags(f, q.tagIds, q.tagMode) &&
      (q.updated === "all" ||
        (q.updated === "last-month"
          ? (f.updatedAt || "").startsWith("2026-08")
          : (f.updatedAt || "") >= "2026-09-20")),
  );
}
export function searchResults(s: State, q = s.search) {
  if (s.viewState === "forbidden") return [];
  const files = scopeFiles(s, q);
  const needle = q.query.trim().toLowerCase();
  return files.flatMap((f) => {
    if (q.tab !== "content")
      return !needle || `${f.name} ${f.folder}`.toLowerCase().includes(needle)
        ? [
            {
              file: f,
              version: f.version,
              excerpt: "文件名或路径精确匹配",
              indexed: false,
              stale: false,
            },
          ]
        : [];
    const index = s.indexes.find(
      (i) =>
        i.space === s.space &&
        i.refs.some((r) => r.id === f.id) &&
        !["queued", "processing"].includes(i.status),
    );
    if (!index) return [];
    const pinned = index.refs.find((r) => r.id === f.id)!;
    const versions = q.history
      ? Array.from({ length: pinned.version }, (_, n) => n + 1)
      : [pinned.version];
    return versions.flatMap((version) => {
      const text = f.snapshots?.find((v) => v.version === version)?.content;
      return text &&
        (!needle ||
          text.toLowerCase().includes(needle) ||
          f.name.toLowerCase().includes(needle))
        ? [
            {
              file: f,
              version,
              excerpt: text,
              indexed: true,
              stale: pinned.version !== f.version || index.status === "stale",
            },
          ]
        : [];
    });
  });
}
export function tagImpact(s: State, id: string) {
  return {
    files: s.files.filter(
      (f) =>
        f.space === s.space &&
        !f.trashed &&
        f.tags?.some((a) => a.tagId === id),
    ).length,
    views:
      s.savedSearches.filter(
        (v) => v.space === s.space && v.spec.tagIds.includes(id),
      ).length +
      s.collections.filter(
        (c) =>
          c.space === s.space &&
          (c.query?.tagIds?.includes(id) || c.query?.tag === id),
      ).length,
  };
}
export type CloudAction =
  | { type: "agent-panel"; open: boolean }
  | { type: "agent-tab"; tab: CloudState["agentTab"] }
  | { type: "preferences"; patch: Partial<Preferences> }
  | { type: "settings-section"; section: string }
  | { type: "settings-query"; query: string }
  | { type: "options"; patch: Partial<CloudState["options"]> }
  | { type: "settings-reset" }
  | { type: "tag-create"; name: string; group: string; color: string }
  | { type: "tag-group"; name: string }
  | {
      type: "tag-update";
      id: string;
      name?: string;
      group?: string;
      color?: string;
      favorite?: boolean;
    }
  | { type: "tag-merge"; from: string; to: string }
  | { type: "tag-delete"; id: string }
  | {
      type: "tag-apply";
      ids: string[];
      tagIds: string[];
      mode: "add" | "remove" | "replace";
      confirmed?: boolean;
    }
  | { type: "tag-suggestion"; fileId: string; tagId: string; accept: boolean }
  | { type: "search-patch"; patch: Partial<SearchSpec> }
  | { type: "search-submit" }
  | { type: "search-save"; name: string; spec?: SearchSpec }
  | { type: "search-open"; id: string }
  | { type: "search-delete"; id: string }
  | {
      type: "index-create";
      directoryId?: string;
      connectionId?: string;
      folder: string;
      exclusions: string[];
      kinds: string[];
      policy: IndexJob["policy"];
    }
  | { type: "index-status"; id: string; status: IndexJob["status"] }
  | { type: "index-recover"; id: string }
  | { type: "index-clear"; id: string }
  | {
      type: "app-create";
      directoryId?: string;
      name: string;
      protocol: ExternalApp["protocol"];
      folder: string;
      actions: string[];
      hours: number;
    }
  | { type: "app-state"; id: string; state: ExternalApp["state"] }
  | { type: "app-call"; id: string; action: string; unknown?: boolean }
  | { type: "call-reconcile"; id: string }
  | { type: "capability-focus"; id: string }
  | { type: "cache-clear" }
  | { type: "unlink-device" };

export function cloudReduce(s: State, a: CloudAction): State | undefined {
  const note = (text: string) => {
    s.notice = text;
    return s;
  };
  const mutating = [
    "tag-create",
    "tag-group",
    "tag-update",
    "tag-merge",
    "tag-delete",
    "tag-apply",
    "tag-suggestion",
    "index-create",
    "index-recover",
    "app-create",
    "app-call",
  ];
  if (
    mutating.includes(a.type) &&
    (!s.online || ["forbidden", "disconnected"].includes(s.viewState))
  )
    return note("请恢复连接与权限，当前操作尚未执行。");
  switch (a.type) {
    case "agent-panel":
      s.agentOpen = a.open;
      s.preferences.agentRememberOpen = a.open;
      return s;
    case "agent-tab":
      s.agentTab = a.tab;
      return s;
    case "preferences":
      s.preferences = safePreferences({ ...s.preferences, ...a.patch });
      s.settingsNotice = "已保存到本设备（原型偏好）。";
      return s;
    case "settings-section":
      s.settingSection = a.section;
      s.settingsNotice = "";
      s.page = "settings";
      return s;
    case "settings-query":
      s.settingsQuery = a.query;
      return s;
    case "options":
      s.options = { ...s.options, ...a.patch };
      s.settingsNotice = "已保存演示设置，刷新后恢复。";
      return s;
    case "settings-reset":
      s.preferences = { ...defaultPreferences };
      s.settingsNotice = "已恢复本设备默认偏好。";
      return s;
    case "tag-group": {
      const name = a.name.trim();
      if (
        !name ||
        s.tagGroups.some((g) => g.space === s.space && g.name === name)
      )
        return note("分组名称为空或已存在。");
      s.tagGroups.push({ space: s.space, name });
      return note("标签分组已创建。");
    }
    case "tag-create": {
      const name = a.name.trim();
      if (!name || s.tags.some((t) => t.space === s.space && t.name === name))
        return note("标签名称为空或已存在。");
      s.tags.push({
        id: `tag-${s.nextTag++}`,
        space: s.space,
        name,
        group: a.group,
        color: a.color,
        favorite: false,
      });
      return note("标签已创建。");
    }
    case "tag-update": {
      const t = s.tags.find((t) => t.id === a.id && t.space === s.space);
      if (!t) return s;
      if (
        a.name !== undefined &&
        (!a.name.trim() ||
          s.tags.some(
            (x) =>
              x.id !== t.id && x.space === s.space && x.name === a.name?.trim(),
          ))
      )
        return note("标签名称为空或重复。");
      if (a.name !== undefined) t.name = a.name.trim();
      if (a.group !== undefined) t.group = a.group;
      if (a.color !== undefined) t.color = a.color;
      if (a.favorite !== undefined) t.favorite = a.favorite;
      return note("标签已更新，文件引用保持不变。");
    }
    case "tag-merge":
    case "tag-delete": {
      const id = a.type === "tag-merge" ? a.from : a.id;
      const tag = s.tags.find((t) => t.id === id && t.space === s.space);
      if (!tag) return s;
      const target =
        a.type === "tag-merge"
          ? s.tags.find(
              (t) => t.id === a.to && t.space === s.space && t.id !== id,
            )
          : undefined;
      if (a.type === "tag-merge" && !target)
        return note("请选择同空间的另一个标签。");
      s.files
        .filter((f) => f.space === s.space)
        .forEach((f) => {
          const next = (f.tags || []).flatMap((x) =>
            x.tagId !== id ? [x] : target ? [{ ...x, tagId: target.id }] : [],
          );
          f.tags = next.filter(
            (x, i, all) =>
              all.findIndex(
                (y) => y.tagId === x.tagId && y.source === x.source,
              ) === i,
          );
        });
      const rewrite = (ids: string[]) => [
        ...new Set(
          ids.flatMap((x) => (x === id ? (target ? [target.id] : []) : [x])),
        ),
      ];
      s.savedSearches
        .filter((v) => v.space === s.space)
        .forEach((v) => (v.spec.tagIds = rewrite(v.spec.tagIds)));
      s.collections
        .filter((c) => c.space === s.space && c.query)
        .forEach((c) => (c.query!.tagIds = rewrite(c.query!.tagIds || [])));
      s.search.tagIds = rewrite(s.search.tagIds);
      s.browser.filter.tagIds = rewrite(s.browser.filter.tagIds || []);
      s.collections
        .filter((c) => c.space === s.space && c.query?.tag === id)
        .forEach((c) => (c.query!.tag = target?.id || "all"));
      if (s.browser.filter.tag === id)
        s.browser.filter.tag = target?.id || "all";
      s.tags = s.tags.filter((t) => t.id !== id);
      return note(
        target
          ? "标签已合并，相关视图已更新；原文件保留。"
          : "标签已删除并从相关视图移除；文件保留，请复查视图范围。",
      );
    }
    case "tag-apply": {
      if (a.mode === "replace" && !a.confirmed)
        return note("替换所有人工标签需要单独确认。");
      const ids = a.tagIds.filter((id) =>
        s.tags.some((t) => t.id === id && t.space === s.space),
      );
      s.files
        .filter(
          (f) => f.space === s.space && !f.trashed && a.ids.includes(f.id),
        )
        .forEach((f) => {
          const old = f.tags || [];
          f.tags =
            a.mode === "replace"
              ? old.filter((t) => t.source !== "manual")
              : a.mode === "remove"
                ? old.filter(
                    (t) => t.source !== "manual" || !ids.includes(t.tagId),
                  )
                : old;
          if (a.mode !== "remove")
            ids.forEach((id) => {
              if (!f.tags!.some((t) => t.tagId === id && t.source === "manual"))
                f.tags!.push({ tagId: id, source: "manual" });
            });
        });
      return note("已更新人工标签，规则与建议层保持独立。");
    }
    case "tag-suggestion": {
      const f = s.files.find(
        (f) => f.id === a.fileId && f.space === s.space && !f.trashed,
      );
      if (
        !f?.tags?.some((t) => t.tagId === a.tagId && t.source === "suggested")
      )
        return s;
      f.tags = f.tags.filter(
        (t) => !(t.tagId === a.tagId && t.source === "suggested"),
      );
      if (
        a.accept &&
        !f.tags.some((t) => t.tagId === a.tagId && t.source === "manual")
      )
        f.tags.push({ tagId: a.tagId, source: "manual" });
      return note(a.accept ? "已接受为人工标签。" : "已忽略本条建议。");
    }
    case "search-patch":
      s.search = { ...s.search, ...a.patch };
      s.searchSubmitted = false;
      s.selected = [];
      return s;
    case "search-submit":
      s.searchSubmitted = true;
      s.selected = [];
      return s;
    case "search-save": {
      const name = a.name.trim();
      if (!name) return note("请填写视图名称。");
      s.savedSearches.push({
        id: `saved-${s.nextSavedSearch++}`,
        space: s.space,
        name,
        spec: structuredClone(a.spec || s.search),
      });
      return note("搜索视图已保存，打开时重新检查当前范围。");
    }
    case "search-open": {
      const saved = s.savedSearches.find(
        (v) => v.id === a.id && v.space === s.space,
      );
      if (saved) {
        s.search = structuredClone(saved.spec);
        s.searchSubmitted = true;
        s.selected = [];
        s.page = "search";
      }
      return s;
    }
    case "search-delete":
      s.savedSearches = s.savedSearches.filter(
        (v) => v.id !== a.id || v.space !== s.space,
      );
      return note("保存视图已移除，文件不变。");
    case "index-create": {
      const node = s.storage.nodes.find(
        (n) =>
          n.space === s.space &&
          (a.directoryId
            ? n.id === a.directoryId
            : n.path === a.folder &&
              n.connectionId === (a.connectionId || s.activeConnection)),
      );
      const conn = s.connections.find((c) => c.id === node?.connectionId);
      if (
        !node ||
        !a.kinds.length ||
        !conn?.authorized ||
        conn.status !== "active" ||
        conn.read !== "passed"
      )
        return note("请确认可访问的连接目录及文件类型。");
      s.indexes.unshift({
        id: `IDX-${s.nextIndex++}`,
        directoryId: node.id,
        connectionId: node.connectionId,
        space: s.space,
        folder: a.folder,
        exclusions: a.exclusions,
        kinds: a.kinds,
        policy: a.policy,
        status: "queued",
        refs: [],
        failed: [],
        observedAt: null,
      });
      return note("已明确授权示例目录，索引任务已排队；不分析其他目录。");
    }
    case "index-status": {
      const job = s.indexes.find((i) => i.id === a.id && i.space === s.space);
      if (!job) return s;
      if (a.status === "processing" && job.status !== "queued") return s;
      if (
        ["ready", "partial"].includes(a.status) &&
        job.status !== "processing"
      )
        return s;
      if (["ready", "partial"].includes(a.status)) {
        const files = s.files.filter(
          (f) =>
            f.space === job.space &&
            f.connectionId === job.connectionId &&
            (!job.directoryId ||
              (!!f.directoryId &&
                directoryContains(s, job.directoryId, f.directoryId))) &&
            !fileUnavailable(s, f) &&
            !f.trashed &&
            (job.directoryId ||
              f.folder === job.folder ||
              f.folder.startsWith(job.folder + " / ")) &&
            job.kinds.includes(f.kind) &&
            !job.exclusions.some(
              (x) => x && `${f.folder}/${f.name}`.includes(x),
            ),
        );
        job.failed =
          a.status === "partial" && files.length > 1 ? [files.at(-1)!.id] : [];
        job.refs = files
          .filter((f) => !job.failed.includes(f.id))
          .map((f) => ({ id: f.id, version: f.version }));
        job.observedAt = "刚刚 · 演示";
      }
      job.status =
        a.status === "partial" && !job.failed.length ? "ready" : a.status;
      return s;
    }
    case "index-recover": {
      const job = s.indexes.find((i) => i.id === a.id && i.space === s.space);
      if (!job) return s;
      const files = s.files.filter(
        (f) =>
          f.space === job.space &&
          f.connectionId === job.connectionId &&
          (!job.directoryId ||
            (!!f.directoryId &&
              directoryContains(s, job.directoryId, f.directoryId))) &&
          !fileUnavailable(s, f) &&
          !f.trashed &&
          (job.directoryId ||
            f.folder === job.folder ||
            f.folder.startsWith(job.folder + " / ")) &&
          job.kinds.includes(f.kind) &&
          !job.exclusions.some((x) => x && `${f.folder}/${f.name}`.includes(x)),
      );
      job.refs = files.map((f) => ({ id: f.id, version: f.version }));
      job.failed = [];
      job.status = "ready";
      job.observedAt = "刚刚 · 演示";
      return note(`已恢复原索引任务 ${job.id}，不创建替代任务。`);
    }
    case "index-clear":
      s.indexes = s.indexes.filter((i) => i.id !== a.id || i.space !== s.space);
      return note("演示索引已清除，原文件和标签保留。");
    case "app-create": {
      const node = s.storage.nodes.find(
        (n) =>
          n.space === s.space &&
          (a.directoryId
            ? n.id === a.directoryId
            : n.path === a.folder && n.connectionId === "drive"),
      );
      if (
        !a.name.trim() ||
        !a.actions.length ||
        a.actions.some((x) => !["read", "suggest", "upload"].includes(x)) ||
        !Number.isInteger(a.hours) ||
        a.hours < 1 ||
        a.hours > 168 ||
        !node ||
        !!directoryAvailable(s, node)
      )
        return note("请填写应用名称、当前目录和允许动作。");
      s.apps.unshift({
        id: `APP-${s.nextApp++}`,
        directoryId: node!.id,
        connectionId: node!.connectionId,
        name: a.name.trim(),
        protocol: a.protocol,
        space: s.space,
        folder: a.folder,
        actions: a.actions,
        hours: a.hours,
        state: "configured",
        observedAt: null,
        writeVerified: false,
      });
      return note("配置已保存（演示），尚未验证连接。");
    }
    case "app-state": {
      const app = s.apps.find((x) => x.id === a.id && x.space === s.space);
      if (!app) return s;
      if (
        a.state === "verified" &&
        directoryAvailable(
          s,
          s.storage.nodes.find((n) => n.id === app.directoryId),
        )
      )
        return note("连接或目录权限不可用，未完成只读验证。");
      app.state = a.state;
      if (a.state === "verified") app.observedAt = "刚刚 · 只读模拟验证";
      return note(
        a.state === "verified"
          ? "只读验证通过；写入尚未验证。"
          : "授权状态已更新，后续调用按新状态检查。",
      );
    }
    case "app-call": {
      const app = s.apps.find((x) => x.id === a.id && x.space === s.space);
      if (!app) return s;
      const allowed =
        app.state === "verified" &&
        app.actions.includes(a.action) &&
        !directoryAvailable(
          s,
          s.storage.nodes.find((n) => n.id === app.directoryId),
        );
      const seq = s.nextCall++;
      s.calls.unshift({
        id: `CALL-${seq}`,
        appId: app.id,
        space: s.space,
        action: a.action,
        scope: app.folder,
        operationId: `EXT-${seq}`,
        result: !allowed ? "denied" : a.unknown ? "unknown" : "done",
        duration: allowed ? "120 ms · 演示" : "未执行",
      });
      if (allowed && !a.unknown && a.action === "upload") {
        const id = `EXT-${seq}-output`;
        const refs = s.files
          .filter(
            (f) =>
              f.space === s.space &&
              !f.trashed &&
              f.connectionId === app.connectionId &&
              !fileUnavailable(s, f) &&
              (f.folder === app.folder ||
                f.folder.startsWith(app.folder + " / ")),
          )
          .slice(0, 2)
          .map((f) => ({ id: f.id, version: f.version }));
        s.files.unshift({
          id,
          connectionId: "drive",
          rootId: "root",
          name: `${app.name} · 检索摘要.md`,
          kind: "document",
          size: "12 KB",
          folder: "待整理",
          version: 1,
          space: s.space,
          origin: "agent",
          favorite: false,
          inbox: true,
          tags: [],
          inputRefs: refs,
          snapshots: [
            {
              version: 1,
              accent: 1,
              content: "外部应用在授权目录内生成的示例摘要，等待用户验收。",
            },
          ],
        });
        s.calls[0].outputId = id;
        app.writeVerified = true;
      }
      if (allowed && !a.unknown && a.action === "suggest") {
        const f = s.files.find(
          (f) =>
            f.space === s.space &&
            !f.trashed &&
            f.folder === app.folder &&
            f.connectionId === app.connectionId &&
            !fileUnavailable(s, f),
        );
        const tag = s.tags.find((t) => t.space === s.space);
        if (
          f &&
          tag &&
          !f.tags?.some((x) => x.tagId === tag.id && x.source === "suggested")
        )
          f.tags = [...(f.tags || []), { tagId: tag.id, source: "suggested" }];
      }
      return note(
        !allowed
          ? "权限或连接不满足，本次调用未执行。"
          : a.unknown
            ? "结果未知，请查询原操作。"
            : "已完成示例调用。",
      );
    }
    case "call-reconcile": {
      const call = s.calls.find((c) => c.id === a.id && c.space === s.space);
      if (!call || call.result !== "unknown") return s;
      const app = s.apps.find((x) => x.id === call.appId);
      if (
        !app ||
        app.state !== "verified" ||
        directoryAvailable(
          s,
          s.storage.nodes.find((n) => n.id === app.directoryId),
        )
      )
        return note(
          `原操作 ${call.operationId} 的当前权限已失效；保留未知结果，未重新执行。`,
        );
      call.result = "done";
      return note(`已查询原操作 ${call.operationId}，没有重新提交。`);
    }
    case "capability-focus":
      s.capabilityFocus = a.id;
      s.page = "capabilities";
      return s;
    case "cache-clear":
      s.files
        .filter((f) => f.space === s.space)
        .forEach((f) => delete f.offlineVersion);
      s.settingsNotice = "已清除本页模拟离线副本，云端文件保留。";
      return s;
    case "unlink-device":
      s.deviceLinked = false;
      s.settingsNotice = "演示设备已解除关联，不撤销独立应用授权。";
      return s;
  }
}
