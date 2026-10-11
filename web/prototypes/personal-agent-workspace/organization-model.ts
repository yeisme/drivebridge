import type { State, FileRecord, Space } from "./model";
import type { DirectoryNode } from "./storage-model";
import type { SearchSpec } from "./cloud-model";
import { scopeFiles } from "./cloud-model";
import {
  directoryAvailable,
  directoryContains,
  isObserved,
} from "./storage-model";
import { fileUnavailable, canEditFile } from "./v4-model";
import {
  isMarkdown,
  noteBody,
  writableNote,
  knowledgeReduce,
  syncResourceFields,
} from "./knowledge-model";
import {
  defaultDefinitions,
  documentTags,
  patchProperty,
  readProperties,
  typeMatches,
} from "./properties";
import type { PropertyDefinition, PropertyValue } from "./properties";
export type EntryRef = {
  kind: "file" | "directory";
  id: string;
  connectionId: string;
  rootId: string;
  space: Space;
};
export interface Bookmark {
  id: string;
  user: string;
  ref: EntryRef;
  groups: string[];
  pin: number | null;
  name: string;
  path: string;
  invalid?: string;
}
export interface BookmarkGroup {
  id: string;
  user: string;
  space: Space;
  connectionId: string;
  name: string;
}
export interface PropertyProjection {
  version: number;
  at: string;
  status: "ready" | "invalid";
  values: Record<string, PropertyValue>;
  errors: string[];
  bodyTags: boolean;
}
export interface PropertyCondition {
  key: string;
  op: "eq" | "ne" | "contains" | "in" | "range" | "empty" | "unset";
  value?: PropertyValue;
  end?: string | number;
}
export interface PropertyBatch {
  perItemValues?: boolean;
  tagMerge?: { from: string; to: string };
  migration?: { from: string; to: string; definition: PropertyDefinition };
  id: string;
  key: string;
  value: PropertyValue | undefined;
  items: {
    id: string;
    version: number;
    before: string;
    after: string;
    nextValue?: PropertyValue;
    metadataRevision?: number;
    status: "pending" | "done" | "failed" | "skipped";
    reason?: string;
  }[];
  confirmed: boolean;
}
export interface OrganizationState {
  lastBookmark?: EntryRef;
  returnFavorite: boolean;
  large: boolean;
  bookmarks: Bookmark[];
  groups: BookmarkGroup[];
  next: number;
  favorites: {
    connectionId: string;
    group: string;
    query: string;
    scroll: number;
  };
  recent: string[];
  definitions: Record<Space, PropertyDefinition[]>;
  metadata: Record<string, Record<string, PropertyValue>>;
  metadataRevisions: Record<string, number>;
  projections: Record<string, PropertyProjection>;
  extraction: Record<
    string,
    {
      bodyTags: boolean;
      generation: number;
      status: "ready" | "paused" | "partial";
    }
  >;
  batch: PropertyBatch | null;
  table: boolean;
  columns: string[];
  widths: Record<string, number>;
  groupBy: string;
  sortKey: string;
}
export function emptyOrganization(): OrganizationState {
  return {
    returnFavorite: false,
    large: false,
    bookmarks: [],
    groups: [],
    next: 1,
    favorites: { connectionId: "drive", group: "all", query: "", scroll: 0 },
    recent: [],
    definitions: {
      personal: structuredClone(defaultDefinitions),
      team: structuredClone(defaultDefinitions),
    },
    metadata: {},
    metadataRevisions: {},
    projections: {},
    extraction: {},
    batch: null,
    table: false,
    columns: ["project", "status", "tags"],
    widths: {},
    groupBy: "",
    sortKey: "name",
  };
}
export function entryRef(
  s: State,
  id: string,
  kind: EntryRef["kind"] = "file",
): EntryRef | undefined {
  const x =
    kind === "file"
      ? s.files.find((f) => f.id === id)
      : s.storage.nodes.find((n) => n.id === id);
  if (!x) return;
  return {
    kind,
    id,
    space: x.space,
    connectionId: x.connectionId || "drive",
    rootId: x.rootId || "",
  };
}
export function sameRef(a: EntryRef, b: EntryRef) {
  return (
    a.kind === b.kind &&
    a.id === b.id &&
    a.connectionId === b.connectionId &&
    a.space === b.space &&
    a.rootId === b.rootId
  );
}
export function bookmarkFor(s: State, r: EntryRef) {
  return s.organization.bookmarks.find(
    (b) => b.user === s.actor && sameRef(b.ref, r),
  );
}
export function currentBookmarks(s: State, c = s.activeConnection) {
  return s.organization.bookmarks.filter(
    (b) =>
      b.user === s.actor && b.ref.space === s.space && b.ref.connectionId === c,
  );
}
export function entryUnavailable(s: State, r: EntryRef) {
  if (r.space !== s.space) return "不在当前空间";
  const x =
    r.kind === "file"
      ? s.files.find((f) => f.id === r.id)
      : s.storage.nodes.find((n) => n.id === r.id);
  if (!x) return "原条目已删除或尚未取得";
  if (x.connectionId !== r.connectionId || x.rootId !== r.rootId)
    return "身份或根范围变化，需重新定位";
  return r.kind === "file"
    ? fileUnavailable(s, x as FileRecord) ||
        directoryAvailable(
          s,
          s.storage.nodes.find((n) => n.id === (x as FileRecord).directoryId),
        )
    : directoryAvailable(s, x as any);
}
export function syncOrganization(s: State) {
  syncResourceFields(s);
  for (const b of s.organization.bookmarks.filter(
    (b) => b.user === s.actor && b.ref.space === s.space && !b.invalid,
  )) {
    if (entryUnavailable(s, b.ref)) continue;
    const item =
      b.ref.kind === "file"
        ? s.files.find((f) => f.id === b.ref.id)
        : s.storage.nodes.find((n) => n.id === b.ref.id);
    if (!item) continue;
    const path = "folder" in item ? item.folder : item.path;
    if (
      s.connections.find((c) => c.id === b.ref.connectionId)?.kind === "s3" &&
      (path !== b.path || item.name !== b.name)
    ) {
      b.invalid = "S3 对象或前缀变化，需重新定位";
      continue;
    }
    b.name = item.name;
    b.path = path;
  }
  for (const f of s.files) f.favorite = !!bookmarkFor(s, entryRef(s, f.id)!);
  return s;
}
export function migrateOrganization(s: State) {
  for (const f of s.files.filter((f) => f.favorite)) {
    const ref = entryRef(s, f.id)!;
    s.organization.bookmarks.push({
      id: `bookmark-${s.organization.next++}`,
      user: "yeisme",
      ref,
      groups: [],
      pin: null,
      name: f.name,
      path: f.folder,
    });
  }
  return syncOrganization(s);
}
export function propertyState(
  s: State,
  f: FileRecord,
  version = f.version,
  draft = true,
) {
  if (!isMarkdown(f))
    return {
      values: s.organization.metadata[f.id] || {},
      errors: [] as string[],
      status: "ready",
      version: f.version,
      at: "本轮元数据",
      fields: [] as ReturnType<typeof readProperties>["fields"],
    };
  const note = s.knowledge.notes[f.id];
  if (note && (note.versions[version] !== undefined || version === f.version)) {
    const p = readProperties(
      draft && version === f.version ? note.draft : noteBody(s, f.id, version),
    );
    return {
      values: Object.fromEntries(
        p.fields.filter((f) => !f.complex).map((f) => [f.key, f.value]),
      ),
      errors: p.errors,
      status: p.valid ? "ready" : "invalid",
      version,
      at: "本次已读取文档 · 部分覆盖",
      fields: p.fields,
    };
  }
  const p = s.organization.projections[f.id];
  return {
    values: p?.values || {},
    errors: p?.errors || [],
    status: !p ? "unscanned" : p.version !== version ? "stale" : p.status,
    version: p?.version,
    at: p?.at || "尚未提取",
    fields: [] as ReturnType<typeof readProperties>["fields"],
  };
}
export function allFileTags(s: State, f: FileRecord) {
  const out: { name: string; source: string; from?: number }[] = (f.tags || [])
    .filter((t) => t.source !== "suggested")
    .flatMap((t) => {
      const tag = s.tags.find((x) => x.id === t.tagId);
      return tag
        ? [
            {
              name: tag.name,
              source: t.source === "rule" ? "规则标签" : "DriveBridge 标签",
            },
          ]
        : [];
    });
  if (isMarkdown(f) && s.knowledge.notes[f.id])
    out.push(
      ...documentTags(noteBody(s, f.id)).map((t) => ({
        ...t,
        source: t.source === "inline" ? "正文标签" : "文首标签",
      })),
    );
  return out;
}
export function conditionMatches(
  s: State,
  f: FileRecord,
  c: PropertyCondition,
) {
  const p = propertyState(s, f);
  if (p.status !== "ready") return false;
  const def = s.organization.definitions[s.space].find((d) => d.key === c.key),
    has = Object.hasOwn(p.values, c.key),
    v = p.values[c.key];
  if (has && def && !typeMatches(v, def.type)) return false;
  if (c.op === "unset") return !has;
  if (!has) return false;
  if (c.op === "empty")
    return v === null || v === "" || (Array.isArray(v) && !v.length);
  if (c.op === "eq" || c.op === "ne")
    return c.op === "eq"
      ? JSON.stringify(v) === JSON.stringify(c.value)
      : JSON.stringify(v) !== JSON.stringify(c.value);
  if (c.op === "contains")
    return Array.isArray(v)
      ? v.includes(String(c.value))
      : typeof v === "string" && v.includes(String(c.value));
  if (c.op === "in")
    return Array.isArray(c.value) && c.value.includes(String(v));
  return (
    (typeof v === "string" || typeof v === "number") &&
    typeof v === typeof c.value &&
    v >= c.value! &&
    v <= (c.end ?? c.value!)
  );
}
export type OrganizationAction =
  | { type: "large-mode"; value: boolean }
  | { type: "large-load"; files: FileRecord[]; directories: DirectoryNode[] }
  | { type: "favorite"; id: string }
  | { type: "bookmark-toggle"; ref: EntryRef }
  | { type: "bookmark-add"; refs: EntryRef[]; groups?: string[] }
  | {
      type: "bookmark-update";
      id: string;
      groups?: string[];
      pin?: number | null;
      remove?: boolean;
      invalid?: string;
    }
  | {
      type: "bookmark-group";
      connectionId: string;
      id?: string;
      name?: string;
      remove?: boolean;
    }
  | { type: "bookmark-directory-visit"; id: string }
  | { type: "favorites-open"; connectionId: string; group?: string }
  | { type: "favorites-view"; patch: Partial<OrganizationState["favorites"]> }
  | { type: "entry-visit"; id: string }
  | {
      type: "property-set";
      id: string;
      key: string;
      value: PropertyValue | undefined;
      version: number;
    }
  | { type: "property-definition"; definition: PropertyDefinition }
  | { type: "document-tag-merge"; from: string; to: string }
  | {
      type: "property-migrate";
      from: string;
      to: string;
      definition: PropertyDefinition;
    }
  | {
      type: "property-table";
      patch: Partial<
        Pick<
          OrganizationState,
          "table" | "columns" | "widths" | "groupBy" | "sortKey"
        >
      >;
    }
  | {
      type: "property-batch";
      ids: string[];
      key: string;
      value: PropertyValue | undefined;
    }
  | { type: "property-batch-run"; id: string; partial?: boolean }
  | {
      type: "property-extract";
      directoryId: string;
      bodyTags: boolean;
      generation?: number;
      pause?: boolean;
      clear?: boolean;
    };
function setProperty(
  s: State,
  f: FileRecord,
  key: string,
  value: PropertyValue | undefined,
  version: number,
  reviewMigration = false,
) {
  if (["__proto__", "constructor", "prototype"].includes(key))
    throw new Error("保留字段键，请在源码中处理");
  if (fileUnavailable(s, f) || !canEditFile(s, f) || !s.online)
    throw new Error("无写入权限、连接断开或等待网络");
  if (f.version !== version)
    throw new Error("版本已变化，保留当前文件并重新预览差异");
  const def = s.organization.definitions[s.space].find((d) => d.key === key);
  if (
    !reviewMigration &&
    def &&
    value !== undefined &&
    !typeMatches(value, def.type)
  )
    throw new Error("属性类型与定义不一致");
  if (isMarkdown(f)) {
    const n = s.knowledge.notes[f.id];
    if (!n || !writableNote(s, f))
      throw new Error("正文未读取或写入能力尚未验证，请保存副本");
    if (n.composing) throw new Error("请先完成输入法组合");
    if (n.base !== version) throw new Error("草稿起始版本已过期");
    const draft = patchProperty(n.draft, key, value);
    if (draft === n.draft) return;
    n.draft = draft;
    n.showDraft = true;
    n.dirty = true;
    n.status = "dirty";
    const resource = s.knowledge.resources.find((r) => r.fileId === f.id);
    if (resource) resource.checkedVersion = undefined;
  } else {
    const values = (s.organization.metadata[f.id] ||= {});
    if (value === undefined) delete values[key];
    else
      Object.defineProperty(values, key, {
        value,
        enumerable: true,
        writable: true,
        configurable: true,
      });
    s.organization.metadataRevisions[f.id] =
      (s.organization.metadataRevisions[f.id] || 0) + 1;
  }
}
export function organizationReduce(
  state: State,
  a: OrganizationAction,
): State | null {
  if (
    ![
      "large-mode",
      "large-load",
      "favorite",
      "bookmark-toggle",
      "bookmark-add",
      "bookmark-update",
      "bookmark-group",
      "bookmark-directory-visit",
      "favorites-open",
      "favorites-view",
      "entry-visit",
      "property-set",
      "document-tag-merge",
      "property-migrate",
      "property-definition",
      "property-table",
      "property-batch",
      "property-batch-run",
      "property-extract",
    ].includes(a.type)
  )
    return null;
  const s = structuredClone(state),
    o = s.organization;
  const add = (ref: EntryRef, groups: string[] = []) => {
    if (ref.space !== s.space || entryUnavailable(s, ref)) return;
    let b = bookmarkFor(s, ref);
    const x =
      ref.kind === "file"
        ? s.files.find((f) => f.id === ref.id)!
        : s.storage.nodes.find((n) => n.id === ref.id)!;
    if (!b) {
      b = {
        id: `bookmark-${o.next++}`,
        user: s.actor,
        ref,
        groups: [],
        pin: null,
        name: x.name,
        path: "folder" in x ? x.folder : x.path,
      };
      o.bookmarks.push(b);
    }
    b.groups = [
      ...new Set([
        ...b.groups,
        ...groups.filter((id) =>
          o.groups.some(
            (g) =>
              g.id === id &&
              g.user === s.actor &&
              g.space === s.space &&
              g.connectionId === ref.connectionId,
          ),
        ),
      ]),
    ];
  };
  switch (a.type) {
    case "large-mode":
      o.large = a.value;
      s.selected = [];
      break;
    case "large-load":
      for (const n of a.directories)
        if (!s.storage.nodes.some((x) => x.id === n.id))
          s.storage.nodes.push(n);
      for (const f of a.files)
        if (!s.files.some((x) => x.id === f.id)) s.files.push(f);
      break;
    case "favorite":
    case "bookmark-toggle": {
      const ref = a.type === "favorite" ? entryRef(s, a.id) : a.ref;
      if (!ref) break;
      const b = bookmarkFor(s, ref);
      if (b) o.bookmarks = o.bookmarks.filter((x) => x.id !== b.id);
      else add(ref);
      o.lastBookmark = b ? undefined : ref;
      s.notice = b
        ? "已取消收藏，原文件与目录不变。"
        : "已加入本盘未分组收藏；可选择收藏夹。";
      break;
    }
    case "bookmark-add":
      a.refs.forEach((r) => add(r, a.groups));
      s.notice = "已收藏可访问条目，原目录不变。";
      break;
    case "bookmark-update": {
      const b = o.bookmarks.find(
        (b) => b.id === a.id && b.user === s.actor && b.ref.space === s.space,
      );
      if (!b) break;
      if (a.remove) o.bookmarks = o.bookmarks.filter((x) => x !== b);
      else {
        if (a.groups)
          b.groups = a.groups.filter((id) =>
            o.groups.some(
              (g) =>
                g.id === id &&
                g.user === s.actor &&
                g.space === s.space &&
                g.connectionId === b.ref.connectionId,
            ),
          );
        if (a.pin !== undefined) b.pin = a.pin;
        if (a.invalid !== undefined) b.invalid = a.invalid;
      }
      s.notice = "个人收藏已更新，原条目不变。";
      break;
    }
    case "bookmark-group": {
      const g = o.groups.find(
        (g) =>
          g.id === a.id &&
          g.user === s.actor &&
          g.space === s.space &&
          g.connectionId === a.connectionId,
      );
      if (a.remove && g) {
        o.groups = o.groups.filter((x) => x !== g);
        o.bookmarks.forEach(
          (b) => (b.groups = b.groups.filter((id) => id !== g.id)),
        );
        o.favorites.group = "all";
      } else if (a.name?.trim()) {
        if (g) g.name = a.name.trim();
        else
          o.groups.push({
            id: `group-${o.next++}`,
            user: s.actor,
            space: s.space,
            connectionId: a.connectionId,
            name: a.name.trim(),
          });
      }
      s.notice = a.remove
        ? "收藏夹已删除；独属条目保留在未分组，原文件不变。"
        : "收藏夹已保存。";
      break;
    }
    case "bookmark-directory-visit": {
      const n = s.storage.nodes.find((n) => n.id === a.id);
      if (!n || directoryAvailable(s, n)) break;
      s.storage.activeId = n.id;
      s.activeConnection = n.connectionId;
      s.browser.folder = n.path;
      s.browser.page = 1;
      s.selected = [];
      o.large = false;
      o.returnFavorite = true;
      s.page = "files";
      break;
    }
    case "favorites-open":
      s.page = "favorites";
      s.selected = [];
      o.returnFavorite = false;
      if (o.favorites.connectionId !== a.connectionId)
        o.favorites = {
          connectionId: a.connectionId,
          group: a.group || "all",
          query: "",
          scroll: 0,
        };
      else if (a.group) o.favorites.group = a.group;
      break;
    case "favorites-view":
      Object.assign(o.favorites, a.patch);
      break;
    case "entry-visit":
      o.recent = [a.id, ...o.recent.filter((id) => id !== a.id)].slice(0, 50);
      break;
    case "property-set": {
      const f = s.files.find((f) => f.id === a.id);
      if (f)
        try {
          setProperty(s, f, a.key, a.value, a.version);
          s.notice = isMarkdown(f)
            ? "属性已写入同一文档草稿，等待自动保存。"
            : "已保存到本轮演示元数据。";
        } catch (e) {
          s.notice = (e as Error).message;
        }
      break;
    }
    case "document-tag-merge": {
      const from = a.from.trim().replace(/^#/, ""),
        to = a.to.trim().replace(/^#/, "");
      if (!from || !to || /\s/.test(to) || from === to) {
        s.notice = "请填写不同的有效标签名称";
        break;
      }
      const items: PropertyBatch["items"] = [];
      for (const f of s.files.filter(
        (f) =>
          f.space === s.space &&
          f.connectionId === s.activeConnection &&
          isMarkdown(f) &&
          !fileUnavailable(s, f),
      )) {
        const p = propertyState(s, f);
        const tags = p.values.tags;
        if (
          !Array.isArray(tags) ||
          !tags.some((t) => t === from || t.startsWith(from + "/"))
        )
          continue;
        const nextValue = [
          ...new Set(
            tags.map((t) =>
              t === from
                ? to
                : t.startsWith(from + "/")
                  ? to + t.slice(from.length)
                  : t,
            ),
          ),
        ];
        const before = noteBody(s, f.id);
        let after = before,
          reason = "";
        try {
          if (s.knowledge.notes[f.id]?.dirty) throw new Error("先保存当前草稿");
          after = patchProperty(before, "tags", nextValue);
        } catch (e) {
          reason = (e as Error).message;
        }
        items.push({
          id: f.id,
          version: f.version,
          metadataRevision: isMarkdown(f)
            ? undefined
            : o.metadataRevisions[f.id] || 0,
          before,
          after,
          nextValue,
          status: reason ? "skipped" : "pending",
          reason,
        });
      }
      o.batch = {
        id: `properties-${o.next++}`,
        key: "tags",
        value: undefined,
        perItemValues: true,
        tagMerge: { from, to },
        confirmed: false,
        items,
      };
      break;
    }
    case "property-migrate": {
      const { from, to, definition } = a;
      if (!/^[\p{L}_][\p{L}\p{N}_ /.-]*$/u.test(to)) {
        s.notice = "目标字段键格式不支持";
        break;
      }
      const items: PropertyBatch["items"] = [];
      for (const f of s.files.filter(
        (f) => f.space === s.space && !f.trashed && !fileUnavailable(s, f),
      )) {
        const p = propertyState(s, f);
        if (!Object.hasOwn(p.values, from)) continue;
        const before = isMarkdown(f)
          ? noteBody(s, f.id)
          : JSON.stringify(p.values, null, 2);
        let after = before,
          reason = "",
          nextValue: PropertyValue = p.values[from];
        try {
          if (p.status !== "ready" || s.knowledge.notes[f.id]?.dirty)
            throw new Error("解析或草稿状态需要先处理");
          if (from !== to && Object.hasOwn(p.values, to))
            throw new Error("目标字段已经存在，保留两者原值");
          if (nextValue !== null && !typeMatches(nextValue, definition.type)) {
            if (definition.type === "text")
              nextValue = Array.isArray(nextValue)
                ? nextValue.join(", ")
                : String(nextValue);
            else if (definition.type === "list")
              nextValue = Array.isArray(nextValue)
                ? nextValue
                : [String(nextValue)];
            else if (
              definition.type === "number" &&
              String(nextValue).trim() &&
              Number.isFinite(Number(nextValue))
            )
              nextValue = Number(nextValue);
            else if (
              definition.type === "checkbox" &&
              ["true", "false"].includes(String(nextValue))
            )
              nextValue = String(nextValue) === "true";
            else throw new Error("此值无法安全转换，请单独处理");
          }
          if (!typeMatches(nextValue, definition.type))
            throw new Error("类型转换无效");
          if (isMarkdown(f))
            after = patchProperty(
              from === to ? before : patchProperty(before, from, undefined),
              to,
              nextValue,
            );
          else {
            const values = { ...p.values };
            delete values[from];
            values[to] = nextValue;
            after = JSON.stringify(values, null, 2);
          }
        } catch (e) {
          reason = (e as Error).message;
        }
        items.push({
          id: f.id,
          version: f.version,
          metadataRevision: isMarkdown(f)
            ? undefined
            : o.metadataRevisions[f.id] || 0,
          before,
          after,
          nextValue,
          status: reason ? "skipped" : "pending",
          reason,
        });
      }
      o.batch = {
        id: `properties-${o.next++}`,
        key: to,
        value: undefined,
        confirmed: false,
        migration: { from, to, definition },
        items,
      };
      s.notice =
        "已生成字段迁移差异；仅覆盖已读取且可解析的文件，不代表全空间扫描。";
      break;
    }
    case "property-definition": {
      const d = o.definitions[s.space];
      if (!/^[\p{L}_][\p{L}\p{N}_ /.-]*$/u.test(a.definition.key)) {
        s.notice = "字段键格式不支持";
        break;
      }
      const old = d.find((x) => x.key === a.definition.key);
      if (old && old.type !== a.definition.type) {
        s.notice = "类型转换需要逐文件差异计划；此处只修改显示定义。";
        break;
      }
      if (old) Object.assign(old, a.definition);
      else d.push(a.definition);
      break;
    }
    case "property-table":
      Object.assign(o, a.patch);
      break;
    case "property-batch":
      o.batch = {
        id: `properties-${o.next++}`,
        key: a.key,
        value: a.value,
        confirmed: false,
        items: [...new Set(a.ids)].map((id) => {
          const f = s.files.find((f) => f.id === id);
          if (!f || f.space !== s.space)
            return {
              id,
              version: 0,
              before: "",
              after: "",
              status: "skipped" as const,
              reason: "文件已不可用",
            };
          const p = propertyState(s, f);
          let after = "",
            reason = "";
          try {
            if (isMarkdown(f)) {
              if (s.knowledge.notes[id]?.dirty)
                throw new Error("存在未保存草稿，先保存后再批量修改");
              after = patchProperty(noteBody(s, id), a.key, a.value);
            } else
              after = JSON.stringify(
                { ...p.values, [a.key]: a.value },
                null,
                2,
              );
          } catch (e) {
            reason = (e as Error).message;
          }
          return {
            id,
            version: f.version,
            metadataRevision: isMarkdown(f)
              ? undefined
              : o.metadataRevisions[f.id] || 0,
            before: isMarkdown(f)
              ? noteBody(s, id)
              : JSON.stringify(p.values, null, 2),
            after,
            status: reason ? "skipped" : "pending",
            reason,
          };
        }),
      };
      break;
    case "property-batch-run": {
      const b = o.batch;
      if (!b || b.id !== a.id) break;
      b.confirmed = true;
      let i = 0;
      for (const item of b.items) {
        if (item.status === "done" || item.status === "skipped") continue;
        const f = s.files.find((f) => f.id === item.id);
        try {
          if (!f) throw new Error("文件已删除");
          if (
            item.metadataRevision !== undefined &&
            item.metadataRevision !== (o.metadataRevisions[f.id] || 0)
          )
            throw new Error("元数据修订已变化，未覆盖后来的属性修改");
          if (a.partial && i++ === 1)
            throw new Error("模拟暂时写入失败，可恢复原操作");
          if (isMarkdown(f) && s.knowledge.notes[f.id]?.dirty)
            throw new Error("文件存在新草稿，未覆盖");
          if (b.migration) {
            setProperty(
              s,
              f,
              b.migration.to,
              item.nextValue,
              item.version,
              true,
            );
            if (b.migration.from !== b.migration.to)
              setProperty(
                s,
                f,
                b.migration.from,
                undefined,
                item.version,
                true,
              );
          } else
            setProperty(
              s,
              f,
              b.key,
              b.perItemValues ? item.nextValue : b.value,
              item.version,
            );
          if (isMarkdown(f)) {
            knowledgeReduce(s, { type: "k-save", id: f.id });
            if (s.knowledge.notes[f.id].dirty)
              throw new Error("保存失败，草稿已保留，请先处理文档保存");
          }
          item.status = "done";
          item.reason = undefined;
        } catch (e) {
          item.status = "failed";
          item.reason = (e as Error).message;
        }
      }
      if (
        b.migration &&
        b.items.length &&
        b.items.every((i) => i.status === "done")
      ) {
        const { from, to, definition } = b.migration;
        o.definitions[s.space] = o.definitions[s.space].filter(
          (d) => d.key !== from && d.key !== to,
        );
        o.definitions[s.space].push({ ...definition, key: to });
        o.columns = o.columns.map((key) => (key === from ? to : key));
        if (o.groupBy === from) o.groupBy = to;
        if (o.sortKey === from) o.sortKey = to;
        for (const q of [
          s.search,
          ...s.savedSearches
            .filter((v) => v.space === s.space)
            .map((v) => v.spec),
        ]) {
          if (!q) continue;
          q.propertyConditions?.forEach((c) => {
            if (c.key === from) c.key = to;
          });
          if (q.view) {
            q.view.columns = q.view.columns.map((key) =>
              key === from ? to : key,
            );
            if (q.view.groupBy === from) q.view.groupBy = to;
          }
        }
      }
      if (
        b.tagMerge &&
        b.items.length &&
        b.items.every((i) => i.status === "done")
      ) {
        for (const q of [
          s.search,
          ...s.savedSearches
            .filter((v) => v.space === s.space)
            .map((v) => v.spec),
        ]) {
          const old = q.documentTag;
          if (old === b.tagMerge.from || old?.startsWith(b.tagMerge.from + "/"))
            q.documentTag = b.tagMerge.to + old.slice(b.tagMerge.from.length);
        }
      }
      s.notice = "原属性计划已处理；完成项不再重做。";
      break;
    }
    case "property-extract": {
      const node = s.storage.nodes.find((n) => n.id === a.directoryId);
      if (!node || directoryAvailable(s, node)) break;
      const prev = o.extraction[node.id];
      if (a.generation !== undefined && a.generation !== prev?.generation)
        break;
      const generation = (prev?.generation || 0) + 1;
      if (a.clear) {
        delete o.extraction[node.id];
        for (const f of s.files.filter((f) =>
          directoryContains(s, node.id, f.directoryId || ""),
        ))
          delete o.projections[f.id];
        s.notice = "已清除属性投影，文件内容未删除。";
        break;
      }
      o.extraction[node.id] = {
        bodyTags: a.bodyTags,
        generation,
        status: a.pause ? "paused" : "ready",
      };
      if (!a.pause)
        for (const f of s.files.filter(
          (f) =>
            f.space === s.space &&
            isMarkdown(f) &&
            directoryContains(s, node.id, f.directoryId || "") &&
            !fileUnavailable(s, f),
        )) {
          const n = s.knowledge.notes[f.id];
          if (!n) {
            o.extraction[node.id].status = "partial";
            continue;
          }
          const p = readProperties(noteBody(s, f.id));
          o.projections[f.id] = {
            version: f.version,
            at: "本轮演示 · 已加载样本",
            status: p.valid ? "ready" : "invalid",
            values: Object.fromEntries(
              p.fields.filter((f) => !f.complex).map((f) => [f.key, f.value]),
            ),
            errors: p.errors,
            bodyTags: a.bodyTags,
          };
          if (!p.valid) o.extraction[node.id].status = "partial";
        }
      s.notice = "仅处理所选范围的模拟属性；未读取的文件仍为未扫描。";
      break;
    }
  }
  return syncOrganization(s);
}
export interface FindEntry {
  ref: EntryRef;
  name: string;
  path: string;
  file?: FileRecord;
  reason: string;
  available: string;
  source: string;
  rank: number;
}
export function findEntries(s: State, q: SearchSpec): FindEntry[] {
  const name = q.query.trim().toLocaleLowerCase(),
    out: FindEntry[] = [];
  const score = (title: string, path: string) =>
    !name
      ? 4
      : title.toLowerCase() === name
        ? 0
        : title.toLowerCase().startsWith(name)
          ? 1
          : name.split(/\s+/).every((t) => title.toLowerCase().includes(t))
            ? 2
            : path.toLowerCase().includes(name)
              ? 3
              : 99;
  const add = (
    ref: EntryRef,
    title: string,
    path: string,
    file?: FileRecord,
  ) => {
    const b = bookmarkFor(s, ref);
    if (
      (q.onlyFavorites && !b) ||
      (q.favoriteGroup && !b?.groups.includes(q.favoriteGroup))
    )
      return;
    if (
      q.favoriteDirectories &&
      !currentBookmarks(s, ref.connectionId).some(
        (x) =>
          x.ref.kind === "directory" &&
          directoryContains(
            s,
            x.ref.id,
            file?.directoryId || (ref.kind === "directory" ? ref.id : ""),
          ),
      )
    )
      return;
    if (
      q.propertyConditions?.length &&
      (!file ||
        !q.propertyConditions.every((c) => conditionMatches(s, file, c)))
    )
      return;
    if (
      q.documentTag &&
      (!file ||
        !allFileTags(s, file).some(
          (t) =>
            t.name.toLocaleLowerCase() === q.documentTag!.toLocaleLowerCase() ||
            (!q.exactTag &&
              t.name
                .toLocaleLowerCase()
                .startsWith(q.documentTag!.toLocaleLowerCase() + "/")),
        ))
    )
      return;
    if (q.propertyIssue) {
      if (!file) return;
      const p = propertyState(s, file, file.version, false),
        issue = q.propertyIssue,
        def = s.organization.definitions[s.space].find(
          (d) => d.key === issue.key,
        );
      if (
        issue.state === "type"
          ? !(
              p.status === "ready" &&
              Object.hasOwn(p.values, issue.key) &&
              def &&
              !typeMatches(p.values[issue.key], def.type)
            )
          : p.status !== issue.state
      )
        return;
    }
    const rank = score(title, path);
    if (rank === 99) return;
    const unavailable = entryUnavailable(s, ref);
    // Do not expose revoked file metadata through search; favorites retain only their user's known record.
    if (unavailable && !b) return;
    out.push({
      ref,
      name: unavailable && b ? b.name : title,
      path: unavailable && b ? b.path : path,
      file,
      rank,
      available: unavailable || b?.invalid || "",
      reason: [
        "名称完全匹配",
        "名称前缀",
        "名称关键词",
        "路径匹配",
        "当前范围",
      ][rank],
      source: b
        ? "个人收藏"
        : file && isObserved(s, file)
          ? "目录索引"
          : "本次已加载目录",
    });
  };
  for (const f of scopeFiles(s, { ...q, query: "", tagIds: q.tagIds || [] })) {
    if (
      !isObserved(s, f) &&
      !bookmarkFor(s, entryRef(s, f.id)!) &&
      !s.storage.nodes.some(
        (n) => n.id === f.directoryId && n.status === "ready",
      )
    )
      continue;
    add(entryRef(s, f.id)!, f.name, f.folder, f);
  }
  if (q.kind === "all" && !q.tagIds.length && q.fileIds === undefined)
    for (const n of s.storage.nodes) {
      if (n.space !== s.space || !q.connections.includes(n.connectionId))
        continue;
      if (
        q.directoryId &&
        (q.recursive !== false
          ? !directoryContains(s, q.directoryId, n.id)
          : n.parentId !== q.directoryId)
      )
        continue;
      if (n.id === q.directoryId) continue;
      add(entryRef(s, n.id, "directory")!, n.name, n.path);
    }
  // Revoked known favorites remain locatable without disclosing fresh provider data.
  for (const b of s.organization.bookmarks.filter(
    (b) =>
      b.user === s.actor &&
      b.ref.space === s.space &&
      q.connections.includes(b.ref.connectionId),
  )) {
    if (out.some((x) => sameRef(x.ref, b.ref))) continue;
    if (
      q.directoryId &&
      !directoryContains(
        s,
        q.directoryId,
        b.ref.kind === "directory"
          ? b.ref.id
          : s.files.find((f) => f.id === b.ref.id)?.directoryId || "",
      )
    )
      continue;
    if (entryUnavailable(s, b.ref) || b.invalid) add(b.ref, b.name, b.path);
  }
  return out.sort(
    (a, b) =>
      a.rank - b.rank ||
      Number(!!bookmarkFor(s, b.ref)) - Number(!!bookmarkFor(s, a.ref)) ||
      (s.organization.recent.indexOf(b.ref.id) < 0 ? 0 : 1) -
        (s.organization.recent.indexOf(a.ref.id) < 0 ? 0 : 1) ||
      a.name.localeCompare(b.name, "zh-CN"),
  );
}
export function pageEntries<T>(items: T[], page: number) {
  return items.slice((Math.max(1, page) - 1) * 50, Math.max(1, page) * 50);
}
