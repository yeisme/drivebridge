import type { FileRecord, Page, Space, State } from "./model";
import { collectionFiles, matchesFilter } from "./model";
import { searchResults } from "./cloud-model";
import { fileUnavailable } from "./v4-model";

export interface PreviewRef {
  id: string;
  version: number;
  connectionId?: string;
  externalRevision?: string;
  role?: string;
}
export interface PreviewScope {
  label: string;
  refs: PreviewRef[];
}
export interface PreviewSession {
  scope: PreviewScope;
  active: PreviewRef;
  cursor: number;
  space: Space;
  origin: Page;
  returnScroll: number;
  archiveFolder?: string;
  anchor?: string;
  documentView?: { mode: "live" | "source" | "read"; showDraft: boolean };
}
export interface DiscussionDraft {
  body: string;
  reply: string;
  replyId: string;
}
export interface ViewerState {
  sheet?: import("./sheet-preview-model").SheetReading;
  discussion?: DiscussionDraft;
  scrollTop: number;
  scrollLeft: number;
  zoom: number;
  time: number;
  page: number;
}
export const defaultViewer: ViewerState = {
  scrollTop: 0,
  scrollLeft: 0,
  zoom: 1,
  time: 0,
  page: 1,
};
export function fileRef(f: FileRecord, version = f.version): PreviewRef {
  return {
    id: f.id,
    version,
    connectionId: f.connectionId || "drive",
    externalRevision: f.externalRevision,
  };
}
export function refKey(r: PreviewRef) {
  return `${r.id}@${r.version}:${r.externalRevision || ""}`;
}
export function previewUnavailable(s: State, r: PreviewRef) {
  const f = s.files.find((f) => f.id === r.id);
  return (
    fileUnavailable(s, f, r.version) ||
    (r.externalRevision && r.externalRevision !== f?.externalRevision
      ? "外部修订已变化，不能替换为当前内容。"
      : "")
  );
}
export function orderedFileScope(s: State): FileRecord[] {
  const files = s.files.filter(
    (f) => f.space === s.space && !f.trashed && !fileUnavailable(s, f),
  );
  if (s.page === "inbox") return files.filter((f) => f.inbox);
  if (s.page === "collections") {
    const c = s.collections.find(
      (c) => c.id === s.browser.collectionId && c.space === s.space,
    );
    return c ? collectionFiles(s, c) : [];
  }
  if (s.page === "files")
    return files
      .filter(
        (f) =>
          (s.activeConnection === "all" ||
            (f.connectionId || "drive") === s.activeConnection) &&
          (s.browser.folder === "全部文件" || f.folder === s.browser.folder) &&
          matchesFilter(f, s.browser.filter),
      )
      .sort((a, b) =>
        s.browser.sort === "name"
          ? a.name.localeCompare(b.name, "zh-CN")
          : (b.updatedAt || "2026-09-27").localeCompare(
              a.updatedAt || "2026-09-27",
            ),
      );
  if (s.page === "home") return files.filter((f) => !f.inbox).slice(0, 4);
  return [];
}
export function createPreviewSession(
  s: State,
  request: {
    id: string;
    version?: number;
    scope?: PreviewScope;
    single?: boolean;
    archiveFolder?: string;
    anchor?: string;
  },
  returnScroll = 0,
): PreviewSession {
  const f = s.files.find((f) => f.id === request.id);
  const captured = request.scope?.refs.find(
    (r) =>
      r.id === request.id &&
      (request.version === undefined || r.version === request.version),
  );
  const active: PreviewRef = captured
    ? { ...captured }
    : f
      ? fileRef(f, request.version)
      : { id: request.id, version: request.version || 1 };
  let scope: PreviewScope;
  if (request.scope) scope = structuredClone(request.scope);
  else if (request.single) scope = { label: "固定文件引用", refs: [active] };
  else if (s.page === "search")
    scope = {
      label: "当前搜索结果",
      refs: searchResults(s).map((r) => fileRef(r.file, r.version)),
    };
  else if (request.version !== undefined)
    scope = { label: "固定文件引用", refs: [active] };
  else
    scope = {
      label:
        (
          {
            inbox: "收件箱",
            files:
              s.browser.folder === "全部文件" ? "当前存储" : s.browser.folder,
            collections: "当前集合",
            home: "最近文件",
          } as Partial<Record<Page, string>>
        )[s.page] || "文件引用",
      refs: orderedFileScope(s).map((f) => fileRef(f)),
    };
  if (!scope.refs.some((r) => r.id === active.id))
    scope = { label: "固定文件引用", refs: [active] };
  const activeIndex = scope.refs.findIndex(
    (r) => r.id === active.id && r.version === active.version,
  );
  const replaceIndex =
    activeIndex >= 0
      ? activeIndex
      : scope.refs.findIndex((r) => r.id === active.id);
  scope.refs = scope.refs
    .map((r, i) => (i === replaceIndex ? { ...r, ...active } : r))
    .filter((r, i, all) => all.findIndex((x) => refKey(x) === refKey(r)) === i);
  return {
    scope,
    active,
    cursor: scope.refs.findIndex((r) => refKey(r) === refKey(active)),
    space: s.space,
    origin: s.page,
    returnScroll,
    archiveFolder: request.archiveFolder,
    anchor: request.anchor,
  };
}
export function taskPreviewScope(s: State, taskId: string): PreviewScope {
  const t = s.tasks.find((t) => t.id === taskId && t.space === s.space);
  if (!t) return { label: "原任务已不可访问", refs: [] };
  const refs: PreviewRef[] = t.items.flatMap((i) => [
    {
      ...(s.files.find((f) => f.id === i.fileId)
        ? fileRef(s.files.find((f) => f.id === i.fileId)!, i.version)
        : { id: i.fileId, version: i.version }),
      role: "输入",
    },
    ...(i.outputId
      ? [
          {
            ...(s.files.find((f) => f.id === i.outputId)
              ? fileRef(
                  s.files.find((f) => f.id === i.outputId)!,
                  i.outputVersion,
                )
              : { id: i.outputId, version: i.outputVersion || 1 }),
            role: "目标",
          },
        ]
      : []),
  ]);
  const output = s.files.find((f) => f.id === `${t.id}-output`);
  if (output) refs.push({ ...fileRef(output), role: "产物" });
  return { label: `任务 ${t.id}`, refs };
}
export function previewFragment(space: Space, r: PreviewRef) {
  return `#f/${space}/${encodeURIComponent(r.id)}/${r.version}${r.externalRevision ? `?revision=${encodeURIComponent(r.externalRevision)}` : ""}`;
}
export function parsePreviewFragment(
  hash: string,
): { space: Space; ref: PreviewRef } | null {
  const m =
    /^#f\/(personal|team)\/([A-Za-z0-9_-]{1,160})\/([1-9]\d{0,5})(?:\?revision=([^#]*))?$/.exec(
      hash,
    );
  if (!m) return null;
  try {
    const externalRevision = m[4] ? decodeURIComponent(m[4]) : undefined;
    if (externalRevision && externalRevision.length > 256) return null;
    return {
      space: m[1] as Space,
      ref: { id: m[2], version: Number(m[3]), externalRevision },
    };
  } catch {
    return null;
  }
}
export function agentReference(s: State, r: PreviewRef) {
  const f = s.files.find((f) => f.id === r.id);
  return r.externalRevision
    ? `演示外部引用（不可调用真实服务）\nconnection_id: ${r.connectionId || f?.connectionId || "unknown"}\nentry_ref: demo-entry/${encodeURIComponent(r.id)}\nobserved_revision: ${r.externalRevision}`
    : `drivebridge://demo-instance/file/${r.id}/version/demo-v${r.version}`;
}
export function agentContext(s: State, r: PreviewRef) {
  const f = s.files.find((f) => f.id === r.id);
  return `DriveBridge 演示上下文（不可调用真实服务）\n文件：${f?.name || "未知"}\n空间：${s.space}\n来源：${s.connections.find((c) => c.id === (r.connectionId || f?.connectionId || "drive"))?.name || "未知"}\n固定版本：${r.externalRevision || `v${r.version}`}\n引用：${agentReference(s, r)}\n复制引用不增加权限。`;
}
