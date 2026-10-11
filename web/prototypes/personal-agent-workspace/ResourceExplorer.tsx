import { useLayoutEffect, useMemo, useRef, useState } from "react";
import {
  ChevronRight,
  ChevronDown,
  Folder,
  Search,
  Plus,
  Tags,
  ListFilter,
  Locate,
  ChevronsUp,
  Hash,
  Type,
  CalendarDays,
  ToggleLeft,
} from "lucide-react";
import { Input } from "@/components/ui/input";
import { Btn, IconBtn, Choice, useDemo } from "./ui";
import { ProviderIcon } from "./StorageWorkspace";
import { ConnectionFavorites } from "./FavoritesWorkspace";
import { directoryAvailable } from "./storage-model";
import type { DirectoryNode } from "./storage-model";
import {
  browseLocation,
  browseQuery,
  explorerState,
  tagFacets,
  propertyFacets,
} from "./browse-model";
import type { ExplorerMode } from "./browse-model";
import { parsePropertyInput } from "./properties";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "./Dialog";
export function DirectoryTree({
  onChoose,
  selected,
  favorites = false,
  query = "",
  header = true,
}: {
  onChoose?: (id: string) => void;
  selected?: string;
  favorites?: boolean;
  query?: string;
  header?: boolean;
}) {
  const { state: s, dispatch, go } = useDemo();
  const tree = useRef<HTMLDivElement>(null),
    [limits, setLimits] = useState<Record<string, number>>({});
  const nodes = s.storage.nodes.filter((n) => n.space === s.space),
    active = selected || s.storage.activeId;
  const visible = useMemo(() => {
    const set = new Set<string>();
    if (!query) return set;
    const map = new Map(nodes.map((n) => [n.id, n]));
    for (const n of nodes.filter((n) =>
      (n.name + " " + n.path)
        .toLocaleLowerCase()
        .includes(query.toLocaleLowerCase()),
    )) {
      let item: DirectoryNode | undefined = n;
      while (item && !set.has(item.id)) {
        set.add(item.id);
        item = map.get(item.parentId || "");
      }
    }
    return set;
  }, [s.storage.nodes, s.space, query]);
  useLayoutEffect(() => {
    tree.current
      ?.querySelector<HTMLElement>('[aria-current="location"]')
      ?.scrollIntoView({ block: "nearest" });
  }, [active, s.browse.revision]);
  const navigate = (n: DirectoryNode) =>
    onChoose ? onChoose(n.id) : dispatch({ type: "directory-open", id: n.id });
  const row = (n: DirectoryNode, depth = 0): React.ReactNode => {
    if (query && !visible.has(n.id)) return null;
    const c = s.connections.find((c) => c.id === n.connectionId)!;
    const children = nodes.filter((x) => x.parentId === n.id),
      expandable =
        !n.parentId ||
        children.length > 0 ||
        ["unloaded", "loading", "partial", "failed"].includes(n.status);
    const expanded =
        expandable && (!!query || s.storage.expanded.includes(n.id)),
      error = directoryAvailable(s, n);
    return (
      <div
        key={n.id}
        role="treeitem"
        aria-level={depth + 1}
        aria-expanded={expandable ? expanded : undefined}
        aria-selected={active === n.id}
      >
        <div
          className={`flex h-8 min-w-0 items-center rounded-sm pr-1 [@media(pointer:coarse)]:h-11 max-[767px]:h-11 ${active === n.id ? "bg-accent text-primary" : "hover:bg-muted/60"}`}
          style={{ paddingLeft: depth * 12 }}
        >
          {expandable ? (
            <button
              aria-label={`${expanded ? "收起" : "展开"}目录 ${c.name} ${n.name}`}
              className="flex size-6 shrink-0 items-center justify-center rounded focus-visible:outline-2 max-[767px]:size-11 [@media(pointer:coarse)]:size-11"
              onClick={() => dispatch({ type: "directory-expand", id: n.id })}
            >
              {expanded ? (
                <ChevronDown size={12} />
              ) : (
                <ChevronRight size={12} />
              )}
            </button>
          ) : (
            <span className="w-6 shrink-0 max-[767px]:w-11 [@media(pointer:coarse)]:w-11" />
          )}
          <button
            data-directory-nav={n.id}
            aria-label={n.name}
            aria-current={active === n.id ? "location" : undefined}
            title={`${c.name} / ${n.path}${error ? " · " + error : ""}`}
            className="flex h-full min-w-0 flex-1 items-center gap-2 text-left text-xs focus-visible:outline-2"
            onClick={() => navigate(n)}
          >
            {n.parentId ? (
              <Folder size={16} className="shrink-0" />
            ) : (
              <span className="shrink-0 [&_svg]:!size-4">
                <ProviderIcon kind={c.kind} />
              </span>
            )}
            <span className="min-w-0 flex-1 truncate">{n.name}</span>
            {!n.parentId && (
              <span
                title={error || "已连接"}
                className={`size-1.5 shrink-0 rounded-full ${error ? "bg-amber-600" : "bg-primary/65"}`}
              />
            )}
          </button>
        </div>
        {expanded && (
          <div role="group" className="relative">
            {favorites && !n.parentId && (
              <ConnectionFavorites
                connectionId={c.id}
                compactTree
                onChosen={() => {}}
              />
            )}
            {error ? (
              <p className="px-6 py-1 text-[10px] text-muted-foreground">
                {error}
              </p>
            ) : n.status === "loading" ? (
              <p className="px-6 py-1 text-[10px]">加载中…</p>
            ) : n.status === "failed" ? (
              <Btn
                className="text-xs"
                variant="ghost"
                onClick={() =>
                  dispatch({
                    type: "directory-status",
                    id: n.id,
                    status: "loading",
                  })
                }
              >
                加载失败，重试
              </Btn>
            ) : (
              <>
                {children
                  .slice(0, limits[n.id] || 50)
                  .map((x) => row(x, depth + 1))}
                {children.length > (limits[n.id] || 50) && (
                  <Btn
                    variant="ghost"
                    className="!h-8 text-xs"
                    onClick={() =>
                      setLimits({
                        ...limits,
                        [n.id]: (limits[n.id] || 50) + 50,
                      })
                    }
                  >
                    加载更多目录
                  </Btn>
                )}
                {n.status === "partial" && (
                  <Btn
                    variant="ghost"
                    className="text-xs"
                    onClick={() =>
                      dispatch({
                        type: "directory-status",
                        id: n.id,
                        status: "ready",
                      })
                    }
                  >
                    加载剩余目录
                  </Btn>
                )}
              </>
            )}
          </div>
        )}
      </div>
    );
  };
  return (
    <nav aria-label="存储目录树" className="flex min-h-0 flex-1 flex-col">
      {header && (
        <div className="flex h-11 shrink-0 items-center justify-between px-3 text-xs font-medium">
          存储与目录
          <IconBtn label="管理存储连接" onClick={() => go("connections")}>
            <Plus size={15} />
          </IconBtn>
        </div>
      )}
      <div
        ref={tree}
        role="tree"
        aria-label="连接根目录"
        className="min-h-0 flex-1 px-1"
        onKeyDown={(e) => {
          const el = (e.target as HTMLElement).closest<HTMLElement>(
            "[data-directory-nav]",
          );
          if (!el) return;
          const n = nodes.find((n) => n.id === el.dataset.directoryNav)!;
          const items = Array.from(
              e.currentTarget.querySelectorAll<HTMLElement>(
                "[data-directory-nav]",
              ),
            ).filter((el) => el.getClientRects().length),
            i = items.indexOf(el);
          if (["ArrowUp", "ArrowDown", "Home", "End"].includes(e.key)) {
            e.preventDefault();
            items[
              e.key === "Home"
                ? 0
                : e.key === "End"
                  ? items.length - 1
                  : Math.max(
                      0,
                      Math.min(
                        items.length - 1,
                        i + (e.key === "ArrowDown" ? 1 : -1),
                      ),
                    )
            ]?.focus();
          }
          if (e.key === "ArrowRight") {
            e.preventDefault();
            if (!s.storage.expanded.includes(n.id))
              dispatch({ type: "directory-expand", id: n.id });
            else
              items
                .find(
                  (el) =>
                    nodes.find((x) => x.id === el.dataset.directoryNav)
                      ?.parentId === n.id,
                )
                ?.focus();
          }
          if (e.key === "ArrowLeft") {
            e.preventDefault();
            if (s.storage.expanded.includes(n.id))
              dispatch({ type: "directory-expand", id: n.id });
            else
              items
                .find((el) => el.dataset.directoryNav === n.parentId)
                ?.focus();
          }
        }}
      >
        {nodes.filter((n) => n.parentId === null).map((n) => row(n))}
        {query && !visible.size && (
          <p className="p-3 text-xs text-muted-foreground">
            当前已加载目录中未找到；可使用文件检索。
          </p>
        )}
      </div>
    </nav>
  );
}
export function ResourceExplorer({
  onChoose,
  onQuery,
}: {
  onChoose: (id: string) => void;
  onQuery?: () => void;
}) {
  const { state: s, dispatch, go } = useDemo(),
    e = explorerState(s),
    loc = browseLocation(s),
    q = browseQuery(s),
    scroll = useRef<HTMLDivElement>(null);
  const [range, setRange] = useState(""),
    [low, setLow] = useState(""),
    [high, setHigh] = useState(""),
    [error, setError] = useState("");
  useLayoutEffect(() => {
    if (scroll.current) scroll.current.scrollTop = e.scroll[e.mode];
  }, [e.mode, s.activeConnection]);
  const commit = (patch: Partial<typeof q>) => {
    dispatch({
      type: "browse-query",
      query: {
        ...q,
        ...(patch.propertyConditions ? { propertyIssue: undefined } : {}),
        ...(patch.propertyIssue
          ? {
              propertyConditions: q.propertyConditions?.filter(
                (c) => c.key !== patch.propertyIssue!.key,
              ),
            }
          : {}),
        ...patch,
        connections: [loc.connectionId],
        directoryId: loc.directoryId,
        query: "",
      },
    });
    onQuery?.();
  };
  const nodes = tagFacets(s).filter((t) =>
    t.name.toLocaleLowerCase().includes(e.search.tags.toLocaleLowerCase()),
  );
  const tagMap = new Map(
    tagFacets(s).map((t) => [t.name.toLocaleLowerCase(), t]),
  );
  const tagRow = (name: string): React.ReactNode => {
    const n = tagMap.get(name.toLocaleLowerCase())!;
    const children = [...tagMap.values()].filter(
      (t) => t.name.slice(0, t.name.lastIndexOf("/")) === name,
    );
    const expanded = e.expanded.includes("tag:" + name) || !!e.search.tags;
    const depth = name.split("/").length - 1;
    if (
      e.search.tags &&
      !nodes.some((t) => t.name === name || t.name.startsWith(name + "/"))
    )
      return null;
    return (
      <div
        key={name}
        role="treeitem"
        aria-expanded={children.length ? expanded : undefined}
      >
        <div
          className="flex h-8 min-w-0 items-center [@media(pointer:coarse)]:h-11 max-[767px]:h-11"
          style={{ paddingLeft: depth * 12 }}
        >
          {children.length ? (
            <button
              aria-label={`展开标签 ${name}`}
              className="flex size-6 shrink-0 items-center justify-center [@media(pointer:coarse)]:size-11 max-[767px]:size-11"
              onClick={() =>
                dispatch({ type: "explorer-expand", id: "tag:" + name })
              }
            >
              {expanded ? (
                <ChevronDown size={12} />
              ) : (
                <ChevronRight size={12} />
              )}
            </button>
          ) : (
            <span className="w-6 shrink-0" />
          )}
          <button
            className={`flex h-full min-w-0 flex-1 items-center gap-2 rounded px-1 text-xs ${q.documentTag === name ? "bg-accent text-primary" : "hover:bg-muted"}`}
            title={n.sources.join("、")}
            onClick={() => commit({ documentTag: name, exactTag: false })}
          >
            <Hash size={14} className="shrink-0" />
            <span className="min-w-0 flex-1 truncate text-left">
              {name.split("/").at(-1)}
            </span>
            <span className="text-[10px] text-muted-foreground">{n.count}</span>
          </button>
        </div>
        {expanded && (
          <div role="group">
            {children
              .sort((a, b) =>
                e.tagSort === "count"
                  ? b.count - a.count
                  : a.name.localeCompare(b.name, "zh-CN"),
              )
              .map((t) => tagRow(t.name))}
          </div>
        )}
      </div>
    );
  };
  const defs = s.organization.definitions[s.space].filter((d) =>
    (d.label + " " + d.key + " " + d.group)
      .toLowerCase()
      .includes(e.search.properties.toLowerCase()),
  );
  return (
    <section className="flex h-full min-h-0 flex-col" aria-label="文件资源栏">
      <div
        role="tablist"
        aria-label="资源树视图"
        className="flex h-11 shrink-0 items-center gap-1 border-b px-2"
      >
        {[
          { id: "directory", name: "目录", icon: Folder },
          { id: "tags", name: "标签", icon: Tags },
          { id: "properties", name: "属性", icon: ListFilter },
        ].map((x) => (
          <button
            key={x.id}
            role="tab"
            aria-selected={e.mode === x.id}
            className={`flex h-8 min-w-0 flex-1 items-center justify-center gap-1 rounded text-xs [@media(pointer:coarse)]:h-11 max-[767px]:h-11 ${e.mode === x.id ? "bg-accent text-primary" : "text-muted-foreground hover:bg-muted"}`}
            onClick={() =>
              dispatch({ type: "explorer-mode", mode: x.id as ExplorerMode })
            }
          >
            <x.icon size={14} />
            {x.name}
          </button>
        ))}
      </div>
      <div className="flex h-10 shrink-0 items-center gap-1 px-2 [@media(pointer:coarse)]:h-12 max-[767px]:h-12">
        <Input
          className="!h-8 min-w-0 text-xs [@media(pointer:coarse)]:!h-11 max-[767px]:!h-11"
          aria-label={`查找${e.mode === "directory" ? "目录" : e.mode === "tags" ? "标签" : "属性"}树`}
          placeholder={`查找${e.mode === "directory" ? "已加载目录" : e.mode === "tags" ? "标签" : "属性"}`}
          value={e.search[e.mode]}
          onChange={(event) =>
            dispatch({
              type: "explorer-search",
              mode: e.mode,
              value: event.target.value,
            })
          }
        />
      </div>
      {e.mode !== "directory" && (
        <p
          className="truncate px-3 pb-1 text-[10px] text-muted-foreground"
          title={s.storage.nodes.find((n) => n.id === loc.directoryId)?.path}
        >
          {s.connections.find((c) => c.id === loc.connectionId)?.name} ·{" "}
          {loc.label}及子目录
        </p>
      )}
      <div
        ref={scroll}
        className="min-h-0 flex-1 overflow-auto px-1"
        onKeyDown={(event) => {
          if (
            event.defaultPrevented ||
            e.mode === "directory" ||
            !["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)
          )
            return;
          const buttons = Array.from(
            event.currentTarget.querySelectorAll<HTMLElement>("button"),
          ).filter((el) => el.getClientRects().length);
          const i = buttons.indexOf(event.target as HTMLElement);
          if (i < 0) return;
          event.preventDefault();
          buttons[
            event.key === "Home"
              ? 0
              : event.key === "End"
                ? buttons.length - 1
                : Math.max(
                    0,
                    Math.min(
                      buttons.length - 1,
                      i + (event.key === "ArrowDown" ? 1 : -1),
                    ),
                  )
          ]?.focus();
        }}
        onScroll={(event) =>
          dispatch({
            type: "explorer-scroll",
            mode: e.mode,
            value: event.currentTarget.scrollTop,
          })
        }
      >
        {e.mode === "directory" ? (
          <DirectoryTree
            onChoose={onChoose}
            favorites
            query={e.search.directory}
            header={false}
          />
        ) : e.mode === "tags" ? (
          <>
            <div className="flex h-8 justify-end text-[10px] [@media(pointer:coarse)]:h-11 max-[767px]:h-11">
              <button
                className="px-2 text-muted-foreground"
                onClick={() =>
                  dispatch({
                    type: "explorer-sort",
                    value: e.tagSort === "name" ? "count" : "name",
                  })
                }
              >
                {e.tagSort === "name" ? "按名称" : "按频次"}
              </button>
            </div>
            <div role="tree" aria-label="标签树">
              {[...tagMap.values()]
                .filter((t) => !t.name.includes("/"))
                .sort((a, b) =>
                  e.tagSort === "count"
                    ? b.count - a.count
                    : a.name.localeCompare(b.name, "zh-CN"),
                )
                .map((t) => tagRow(t.name))}
              {!tagMap.size && (
                <p className="p-3 text-xs text-muted-foreground">
                  当前已读取范围没有正式标签。
                </p>
              )}
            </div>
          </>
        ) : (
          <div role="tree" aria-label="属性树">
            {[...new Set(defs.map((d) => d.group))].map((group) => (
              <section key={group}>
                <h3 className="px-2 py-2 text-[10px] font-medium text-muted-foreground">
                  {group}
                </h3>
                {defs
                  .filter((d) => d.group === group)
                  .map((d) => {
                    const expanded = e.expanded.includes("property:" + d.key),
                      data = propertyFacets(s, d.key),
                      Icon =
                        d.type === "number"
                          ? Hash
                          : d.type === "checkbox"
                            ? ToggleLeft
                            : d.type === "date" || d.type === "datetime"
                              ? CalendarDays
                              : Type;
                    return (
                      <div key={d.key} role="treeitem" aria-expanded={expanded}>
                        <button
                          className="flex h-8 w-full items-center gap-2 rounded px-2 text-xs hover:bg-muted max-[767px]:h-11 [@media(pointer:coarse)]:h-11"
                          onClick={() =>
                            dispatch({
                              type: "explorer-expand",
                              id: "property:" + d.key,
                            })
                          }
                        >
                          {expanded ? (
                            <ChevronDown size={12} />
                          ) : (
                            <ChevronRight size={12} />
                          )}
                          <Icon size={14} />
                          <span className="min-w-0 flex-1 truncate text-left">
                            {d.label}
                          </span>
                          <small className="text-muted-foreground">
                            {data.coverage.ready}
                          </small>
                        </button>
                        {expanded && (
                          <div
                            role="group"
                            className="ml-5 border-l border-border/70 pl-2"
                          >
                            {["number", "date", "datetime"].includes(
                              d.type,
                            ) && (
                              <button
                                className="flex h-8 w-full items-center text-xs text-primary max-[767px]:h-11 [@media(pointer:coarse)]:h-11"
                                onClick={() => {
                                  setRange(d.key);
                                  setLow("");
                                  setHigh("");
                                  setError("");
                                }}
                              >
                                设置范围…
                              </button>
                            )}
                            {data.values
                              .slice(0, (e.fieldPages[d.key] || 1) * 20)
                              .map((v, i) => (
                                <button
                                  key={i}
                                  role="treeitem"
                                  className="flex h-8 w-full items-center gap-2 rounded px-2 text-xs hover:bg-muted max-[767px]:h-11 [@media(pointer:coarse)]:h-11"
                                  onClick={() =>
                                    v.state === "invalid"
                                      ? commit({
                                          propertyIssue: {
                                            key: d.key,
                                            state: "type",
                                          },
                                        })
                                      : commit({
                                          propertyConditions: [
                                            ...(
                                              q.propertyConditions || []
                                            ).filter((c) => c.key !== d.key),
                                            {
                                              key: d.key,
                                              op:
                                                v.state === "unset"
                                                  ? "unset"
                                                  : v.state === "empty"
                                                    ? "empty"
                                                    : d.type === "list"
                                                      ? "contains"
                                                      : "eq",
                                              value: v.value as any,
                                            },
                                          ],
                                        })
                                  }
                                >
                                  <span className="min-w-0 flex-1 truncate text-left">
                                    {v.label}
                                  </span>
                                  <small className="text-muted-foreground">
                                    {v.count}
                                  </small>
                                </button>
                              ))}
                            {data.values.length >
                              (e.fieldPages[d.key] || 1) * 20 && (
                              <Btn
                                variant="ghost"
                                className="text-xs"
                                onClick={() =>
                                  dispatch({
                                    type: "explorer-more",
                                    key: d.key,
                                  })
                                }
                              >
                                更多值
                              </Btn>
                            )}
                            {(["unscanned", "invalid", "stale"] as const).map(
                              (state) =>
                                data.coverage[state] > 0 && (
                                  <button
                                    key={state}
                                    className="flex min-h-8 w-full items-center px-2 text-left text-[10px] text-muted-foreground max-[767px]:min-h-11 [@media(pointer:coarse)]:min-h-11"
                                    onClick={() =>
                                      commit({
                                        propertyIssue: { key: d.key, state },
                                      })
                                    }
                                  >
                                    {
                                      {
                                        unscanned: "未提取",
                                        invalid: "解析失败",
                                        stale: "过期投影",
                                      }[state]
                                    }{" "}
                                    · {data.coverage[state]}
                                  </button>
                                ),
                            )}
                          </div>
                        )}
                      </div>
                    );
                  })}
              </section>
            ))}
          </div>
        )}
      </div>
      <footer className="flex h-11 shrink-0 items-center justify-between border-t px-2 text-[10px] text-muted-foreground">
        <span className="min-w-0 truncate">
          {e.mode === "directory"
            ? "原目录 · 按需加载"
            : "已授权样本 · 部分覆盖"}
        </span>
        {e.mode === "directory" ? (
          <>
            <IconBtn
              label="定位当前目录"
              onClick={() => dispatch({ type: "browse-reveal" })}
            >
              <Locate size={14} />
            </IconBtn>
            <IconBtn
              label="收起其他连接"
              onClick={() => dispatch({ type: "browse-collapse-others" })}
            >
              <ChevronsUp size={14} />
            </IconBtn>
          </>
        ) : (
          <IconBtn label="管理标签与属性" onClick={() => go("tags")}>
            <Plus size={14} />
          </IconBtn>
        )}
      </footer>
      <Dialog open={!!range} onOpenChange={(v) => !v && setRange("")}>
        <DialogContent>
          <DialogTitle>属性范围</DialogTitle>
          <DialogDescription>在当前连接与目录范围内检索。</DialogDescription>
          <label className="text-xs">
            最小值
            <Input
              aria-label="属性范围起点"
              value={low}
              onChange={(e) => setLow(e.target.value)}
            />
          </label>
          <label className="text-xs">
            最大值
            <Input
              aria-label="属性范围终点"
              value={high}
              onChange={(e) => setHigh(e.target.value)}
            />
          </label>
          {error && <p className="text-xs text-destructive">{error}</p>}
          <Btn
            onClick={() => {
              try {
                const d = defs.find((d) => d.key === range)!;
                const from = parsePropertyInput(low, d.type),
                  to = parsePropertyInput(high, d.type);
                if (
                  (typeof from !== "number" && typeof from !== "string") ||
                  typeof to !== typeof from ||
                  !low ||
                  !high ||
                  from > to!
                )
                  throw new Error("请输入有效且有序的范围");
                commit({
                  propertyConditions: [
                    ...(q.propertyConditions || []).filter(
                      (c) => c.key !== range,
                    ),
                    {
                      key: range,
                      op: "range",
                      value: from,
                      end: to as string | number,
                    },
                  ],
                });
                setRange("");
              } catch (e) {
                setError((e as Error).message);
              }
            }}
          >
            应用范围
          </Btn>
        </DialogContent>
      </Dialog>
    </section>
  );
}
