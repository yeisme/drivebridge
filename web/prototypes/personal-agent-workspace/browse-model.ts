import type { Action, State, FileFilter } from "./model";
import type { SearchSpec } from "./cloud-model";
import { emptySearch, scopeFiles } from "./cloud-model";
import { directoryAvailable, rootNode, bytes } from "./storage-model";
import type { FindEntry } from "./organization-model";
import { allFileTags, propertyState } from "./organization-model";
import { displayProperty, typeMatches } from "./properties";
export type ExplorerMode = "directory" | "tags" | "properties";
export interface BrowseLocation {
  kind: "directory" | "favorites" | "query";
  surface: "files" | "favorites" | "search";
  connectionId: string;
  directoryId: string;
  label: string;
  query?: SearchSpec;
  group?: string;
  favoritesQuery?: string;
  origin?: BrowseLocation;
  filter: FileFilter;
  sort: string;
  page: number;
  scroll: number;
  focus: string;
}
export interface BrowseHistory {
  current: BrowseLocation;
  back: BrowseLocation[];
  forward: BrowseLocation[];
}
export interface ExplorerState {
  mode: ExplorerMode;
  search: Record<ExplorerMode, string>;
  scroll: Record<ExplorerMode, number>;
  expanded: string[];
  tagSort: "name" | "count";
  fieldPages: Record<string, number>;
}
export interface BrowseState {
  histories: Record<string, BrowseHistory>;
  explorers: Record<string, ExplorerState>;
  revision: number;
}
export const emptyBrowse = (): BrowseState => ({
  histories: {},
  explorers: {},
  revision: 0,
});
const scopeKey = (s: State) => `${s.actor}:${s.space}`;
export const explorerKey = (s: State) => `${scopeKey(s)}:${s.activeConnection}`;
export const emptyExplorer = (): ExplorerState => ({
  mode: "directory",
  search: { directory: "", tags: "", properties: "" },
  scroll: { directory: 0, tags: 0, properties: 0 },
  expanded: [],
  tagSort: "name",
  fieldPages: {},
});
export function explorerState(s: State) {
  return s.browse.explorers[explorerKey(s)] || emptyExplorer();
}
export function directoryLocation(
  s: State,
  id = s.storage.activeId,
): BrowseLocation {
  const n = s.storage.nodes.find((n) => n.id === id);
  return {
    kind: "directory",
    surface: "files",
    connectionId: n?.connectionId || s.activeConnection,
    directoryId: id,
    label: n?.name || "目录不可用",
    filter: structuredClone(s.browser.filter),
    sort: s.browser.sort,
    page: s.browser.page,
    scroll: 0,
    focus: "",
  };
}
export function browseHistory(s: State): BrowseHistory {
  return (
    s.browse.histories[scopeKey(s)] || {
      current: directoryLocation(s),
      back: [],
      forward: [],
    }
  );
}
export function browseLocation(s: State) {
  return browseHistory(s).current;
}
export function browseQuery(s: State): SearchSpec {
  const loc = browseLocation(s);
  return loc.kind === "query" && loc.query
    ? loc.query
    : {
        ...emptySearch(),
        connections: [loc.connectionId],
        directoryId: loc.directoryId,
        recursive: true,
      };
}
function snapshot(s: State): BrowseLocation {
  const loc = structuredClone(browseLocation(s));
  if (["files", "favorites"].includes(s.page)) {
    loc.page = s.browser.page;
    loc.sort = s.browser.sort;
    loc.filter = structuredClone(s.browser.filter);
  }
  if (s.page === "favorites") {
    loc.group = s.organization.favorites.group;
    loc.favoritesQuery = s.organization.favorites.query;
    loc.scroll = s.organization.favorites.scroll;
  }
  return loc;
}
function locationKey(loc: BrowseLocation) {
  return JSON.stringify([
    loc.kind,
    loc.surface,
    loc.directoryId,
    loc.connectionId,
    loc.query,
    loc.group,
  ]);
}
function reveal(s: State, id: string) {
  let n = s.storage.nodes.find((n) => n.id === id);
  const ids = new Set(s.storage.expanded);
  while (n) {
    if (n.parentId) ids.add(n.parentId);
    n = s.storage.nodes.find((x) => x.id === n!.parentId);
  }
  s.storage.expanded = [...ids];
}
function applyLocation(s: State, loc: BrowseLocation) {
  s.activeConnection = loc.connectionId;
  s.storage.activeId = loc.directoryId;
  s.page = loc.surface;
  const node = s.storage.nodes.find((n) => n.id === loc.directoryId);
  s.browser.folder = node?.path || loc.label;
  s.browser.filter = structuredClone(loc.filter);
  s.browser.sort = loc.sort;
  s.browser.page = loc.page;
  s.browser.focusedId = loc.focus;
  s.selected = [];
  s.organization.large = false;
  if (loc.kind === "favorites")
    s.organization.favorites = {
      ...s.organization.favorites,
      connectionId: loc.connectionId,
      group: loc.group || "all",
      query: loc.favoritesQuery || "",
      scroll: loc.scroll,
    };
  if (loc.surface === "search" && loc.query) {
    s.search = structuredClone(loc.query);
    s.searchSubmitted = true;
  }
  reveal(s, loc.directoryId);
  if (node?.status === "unloaded") node.status = "loading";
  const unavailable = directoryAvailable(s, node);
  if (unavailable) s.notice = `原位置暂不可用：${unavailable}`;
}
function commit(previous: State, s: State, loc: BrowseLocation) {
  const h = structuredClone(browseHistory(previous)),
    old = snapshot(previous);
  if (locationKey(old) !== locationKey(loc)) {
    h.back = [...h.back, old].slice(-100);
    h.forward = [];
  }
  h.current = loc;
  s.browse.histories[scopeKey(s)] = h;
  applyLocation(s, loc);
  s.browse.revision++;
}
export type BrowseAction =
  | { type: "browse-history"; direction: "back" | "forward"; index?: number }
  | {
      type:
        | "browse-up"
        | "browse-return"
        | "browse-refresh"
        | "browse-reveal"
        | "browse-collapse-others";
    }
  | { type: "browse-query"; query: SearchSpec; surface?: "files" | "search" }
  | {
      type: "browse-checkpoint";
      scroll?: number;
      focus?: string;
      page?: number;
      sort?: string;
    }
  | { type: "explorer-mode"; mode: ExplorerMode }
  | { type: "explorer-search"; mode: ExplorerMode; value: string }
  | { type: "explorer-scroll"; mode: ExplorerMode; value: number }
  | { type: "explorer-expand"; id: string }
  | { type: "explorer-sort"; value: "name" | "count" }
  | { type: "explorer-more"; key: string };
export function browseReduce(state: State, a: BrowseAction): State | null {
  if (!a.type.startsWith("browse-") && !a.type.startsWith("explorer-"))
    return null;
  if (a.type === "browse-checkpoint") {
    const old = browseLocation(state);
    if (
      (a.scroll === undefined || a.scroll === old.scroll) &&
      (a.focus === undefined || a.focus === old.focus) &&
      (a.page === undefined || a.page === old.page) &&
      (a.sort === undefined || a.sort === old.sort)
    )
      return state;
  }
  if (
    a.type === "explorer-scroll" &&
    explorerState(state).scroll[a.mode] === a.value
  )
    return state;
  const s = structuredClone(state),
    h = (s.browse.histories[scopeKey(s)] ||= structuredClone(browseHistory(s))),
    e = (s.browse.explorers[explorerKey(s)] ||= emptyExplorer());
  switch (a.type) {
    case "browse-history": {
      const source = a.direction === "back" ? h.back : h.forward,
        target = a.direction === "back" ? h.forward : h.back;
      const steps = Math.max(1, Math.min(source.length, (a.index ?? 0) + 1));
      if (!source.length) break;
      h.current = snapshot(state);
      for (let i = 0; i < steps; i++) {
        target.push(h.current);
        h.current = source.pop()!;
      }
      applyLocation(s, h.current);
      s.browse.revision++;
      break;
    }
    case "browse-query": {
      const q = structuredClone(a.query),
        id = q.directoryId || rootNode(s, q.connections[0])?.id;
      if (!id) break;
      const loc = {
        ...directoryLocation(s, id),
        kind: "query" as const,
        surface: a.surface || ("files" as const),
        query: q,
        sort: q.view?.listSort || "relevance",
        origin:
          h.current.kind === "query" && h.current.directoryId === id
            ? h.current.origin
            : snapshot(state),
        page: 1,
        scroll: 0,
        focus: "",
      };
      commit(state, s, loc);
      break;
    }
    case "browse-up": {
      const loc = h.current;
      if (loc.kind !== "directory") break;
      const n = s.storage.nodes.find((n) => n.id === loc.directoryId);
      if (n?.parentId)
        commit(state, s, {
          ...directoryLocation(s, n.parentId),
          page: 1,
          scroll: 0,
        });
      break;
    }
    case "browse-return": {
      const loc = h.current;
      const origin =
        loc.origin?.kind === "directory"
          ? loc.origin
          : { ...directoryLocation(s, loc.directoryId), page: 1, scroll: 0 };
      commit(state, s, structuredClone(origin));
      break;
    }
    case "browse-checkpoint":
      Object.assign(
        h.current,
        a.scroll === undefined ? {} : { scroll: a.scroll },
        a.focus === undefined ? {} : { focus: a.focus },
        a.page === undefined ? {} : { page: a.page },
        a.sort === undefined ? {} : { sort: a.sort },
      );
      break;
    case "browse-refresh": {
      const n = s.storage.nodes.find((n) => n.id === h.current.directoryId);
      if (n && !directoryAvailable(s, n) && h.current.kind === "directory")
        n.status = "loading";
      s.browse.revision++;
      s.notice = "已刷新当前范围；没有发起全盘索引任务。";
      break;
    }
    case "browse-reveal":
      e.mode = "directory";
      e.search.directory = "";
      reveal(s, h.current.directoryId);
      s.browse.revision++;
      break;
    case "browse-collapse-others":
      s.storage.expanded = s.storage.expanded.filter(
        (id) =>
          s.storage.nodes.find((n) => n.id === id)?.connectionId ===
          s.activeConnection,
      );
      reveal(s, h.current.directoryId);
      break;
    case "explorer-mode":
      e.mode = a.mode;
      break;
    case "explorer-search":
      e.search[a.mode] = a.value;
      break;
    case "explorer-scroll":
      e.scroll[a.mode] = a.value;
      break;
    case "explorer-expand":
      e.expanded = e.expanded.includes(a.id)
        ? e.expanded.filter((id) => id !== a.id)
        : [...e.expanded, a.id];
      break;
    case "explorer-sort":
      e.tagSort = a.value;
      break;
    case "explorer-more":
      e.fieldPages[a.key] = (e.fieldPages[a.key] || 1) + 1;
      break;
  }
  return s;
}
export function browseAfter(previous: State, s: State, a: Action) {
  if (a.type === "reset") return s;
  if (a.type === "space" || a.type === "actor") {
    s.browse.histories[scopeKey(previous)] = {
      ...structuredClone(browseHistory(previous)),
      current: snapshot(previous),
    };
    const h = s.browse.histories[scopeKey(s)];
    if (h) applyLocation(s, h.current);
    else
      s.browse.histories[scopeKey(s)] = {
        current: directoryLocation(s),
        back: [],
        forward: [],
      };
    if (
      a.type === "actor" &&
      !["files", "favorites", "search"].includes(previous.page)
    )
      s.page = previous.page;
    s.browse.revision++;
    return s;
  }
  if (
    a.type === "directory-open" ||
    a.type === "bookmark-directory-visit" ||
    a.type === "folder" ||
    a.type === "connection-select"
  ) {
    if (
      s.page !== "files" ||
      (a.type === "directory-open" && s.storage.activeId !== a.id)
    )
      return s;
    const loc = directoryLocation(s);
    loc.page = a.type === "connection-select" ? s.browser.page : 1;
    loc.scroll =
      a.type === "connection-select"
        ? s.storage.positions[`${s.space}:${a.id}`]?.scroll || 0
        : 0;
    loc.focus = "focus" in a ? String(a.focus || "") : "";
    if (a.type === "directory-open" && a.scroll !== undefined) {
      const h = structuredClone(browseHistory(previous));
      h.current.scroll = a.scroll;
      previous = {
        ...previous,
        browse: {
          ...previous.browse,
          histories: { ...previous.browse.histories, [scopeKey(previous)]: h },
        },
      };
    }
    commit(previous, s, loc);
  }
  if (a.type === "favorites-open")
    commit(previous, s, {
      ...directoryLocation(s, rootNode(s, a.connectionId)?.id),
      kind: "favorites",
      surface: "favorites",
      connectionId: a.connectionId,
      group: s.organization.favorites.group,
      page: 1,
      scroll: s.organization.favorites.scroll,
    });
  if (a.type === "search-submit" || a.type === "search-open") {
    if (s.page === "search") {
      const q = structuredClone(s.search);
      const id = q.directoryId || rootNode(s, q.connections[0])?.id;
      if (id)
        commit(previous, s, {
          ...directoryLocation(s, id),
          kind: "query",
          surface: "search",
          query: q,
          sort: q.view?.listSort || "relevance",
          origin:
            snapshot(previous).kind === "query"
              ? snapshot(previous).origin
              : snapshot(previous),
          page: 1,
          scroll: 0,
        });
    }
  }
  if (a.type === "navigate" && a.page === "files") {
    const current = browseHistory(s).current;
    const target =
      current.kind === "favorites"
        ? directoryLocation(s)
        : current.surface === "search"
          ? current.origin?.kind === "directory"
            ? current.origin
            : directoryLocation(s, current.directoryId)
          : current;
    s.browse.histories[scopeKey(s)] = {
      ...browseHistory(s),
      current: { ...target, surface: "files" },
    };
    applyLocation(s, s.browse.histories[scopeKey(s)].current);
    s.browse.revision++;
  }
  if (a.type === "browser") {
    const h = s.browse.histories[scopeKey(s)];
    if (h) {
      h.current.page = s.browser.page;
      h.current.sort = s.browser.sort;
    }
  }
  return s;
}
export function facetFiles(s: State, q = browseQuery(s)) {
  return scopeFiles(s, {
    ...emptySearch(),
    connections: q.connections,
    directoryId: q.directoryId,
    recursive: q.recursive !== false,
  });
}
export function tagFacets(s: State, q = browseQuery(s)) {
  const map = new Map<
    string,
    { name: string; ids: Set<string>; sources: Set<string> }
  >();
  for (const f of facetFiles(s, q))
    for (const tag of allFileTags(s, f)) {
      const parts = tag.name.split("/");
      for (let i = 1; i <= parts.length; i++) {
        const name = parts.slice(0, i).join("/"),
          key = name.toLocaleLowerCase(),
          node = map.get(key) || {
            name,
            ids: new Set<string>(),
            sources: new Set<string>(),
          };
        node.ids.add(f.id);
        node.sources.add(tag.source);
        map.set(key, node);
      }
    }
  return [...map.values()].map((x) => ({
    name: x.name,
    count: x.ids.size,
    sources: [...x.sources],
  }));
}
export function propertyFacets(s: State, key: string, q = browseQuery(s)) {
  const def = s.organization.definitions[s.space].find((d) => d.key === key)!;
  const values = new Map<
    string,
    {
      value: unknown;
      label: string;
      count: number;
      state: "value" | "unset" | "empty" | "invalid";
    }
  >();
  const coverage = { unscanned: 0, stale: 0, invalid: 0, ready: 0 };
  for (const f of facetFiles(s, q)) {
    const p = propertyState(s, f, f.version, false);
    if (p.status !== "ready") {
      coverage[p.status as "unscanned" | "stale" | "invalid"]++;
      continue;
    }
    coverage.ready++;
    const has = Object.hasOwn(p.values, key),
      value = p.values[key],
      state = !has
        ? "unset"
        : !typeMatches(value, def.type)
          ? "invalid"
          : value === null ||
              value === "" ||
              (Array.isArray(value) && !value.length)
            ? "empty"
            : "value";
    const list =
      state === "value" && Array.isArray(value) ? [...new Set(value)] : [value];
    for (const item of list) {
      const identity = JSON.stringify([state, item]),
        old = values.get(identity);
      if (old) old.count++;
      else
        values.set(identity, {
          value: item,
          label:
            state === "unset"
              ? "未设置"
              : state === "empty"
                ? "空值"
                : state === "invalid"
                  ? "类型不一致"
                  : displayProperty(item as any),
          count: 1,
          state,
        });
    }
  }
  return {
    values: [...values.values()].sort(
      (a, b) => b.count - a.count || a.label.localeCompare(b.label, "zh-CN"),
    ),
    coverage,
  };
}

export function sortEntries(entries: FindEntry[], sort: string) {
  if (sort === "relevance") return entries;
  return [...entries].sort((a, b) => {
    if (!a.file !== !b.file) return !a.file ? -1 : 1;
    if (!a.file || sort.startsWith("name"))
      return (
        a.name.localeCompare(b.name, "zh-CN") * (sort === "name-desc" ? -1 : 1)
      );
    if (sort.startsWith("size"))
      return (
        (bytes(b.file?.size || "0") - bytes(a.file?.size || "0")) *
        (sort === "size-asc" ? -1 : 1)
      );
    return (
      (b.file?.updatedAt || "").localeCompare(a.file?.updatedAt || "") *
      (sort === "oldest" ? -1 : 1)
    );
  });
}
