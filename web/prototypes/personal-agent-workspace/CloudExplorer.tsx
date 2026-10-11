import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import {
  Folder,
  ChevronRight,
  Search,
  Plus,
  List,
  Grid2X2,
  Columns2,
  ArrowLeft,
  RefreshCw,
  X,
  Bookmark,
} from "lucide-react";
import { Input } from "@/components/ui/input";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Btn, IconBtn, Choice, Empty, useDemo } from "./ui";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "./Dialog";
import { ResourceExplorer } from "./ResourceExplorer";
export { DirectoryTree } from "./ResourceExplorer";
import { BrowseControls } from "./BrowseControls";
import { ScopedFilter, filterCount, FilterConditions } from "./ScopedFilter";
import { FileDetailsList } from "./FileDetailsList";
import { LargeCatalog } from "./LargeCatalog";
import { PropertyTable, useFind } from "./UnifiedFind";
import { FavoriteButton } from "./FavoritesWorkspace";
import { AssetCard } from "./CreativeWorkspace";
import { FileToolbar } from "./CloudToolbar";
import { entryRef } from "./organization-model";
import type { FindEntry } from "./organization-model";
import { directoryAvailable, directoryFiles, bytes } from "./storage-model";
import type { DirectoryNode } from "./storage-model";
import { matchesFilter } from "./model";
import { fileRef } from "./preview-model";
import { browseLocation, browseQuery, sortEntries } from "./browse-model";
export function StorageRuntime() {
  const { state: s, dispatch } = useDemo();
  const pending = s.storage.nodes
    .filter((n) => n.status === "loading" && n.space === s.space)
    .map((n) => n.id)
    .join("|");
  useEffect(() => {
    const timers = pending
      .split("|")
      .filter(Boolean)
      .map((id) =>
        setTimeout(
          () => dispatch({ type: "directory-status", id, status: "ready" }),
          180,
        ),
      );
    return () => timers.forEach(clearTimeout);
  }, [pending]);
  const runs = s.storage.runs
    .filter((r) => r.status === "running" && r.space === s.space)
    .map((r) => `${r.id}:${r.generation}`)
    .join("|");
  useEffect(() => {
    if (!s.online) return;
    const timers = s.storage.runs
      .filter((r) => r.status === "running" && r.space === s.space)
      .map((r) =>
        setTimeout(
          () =>
            dispatch({
              type: "catalog-complete",
              id: r.id,
              generation: r.generation,
              result: "complete",
            }),
          2200,
        ),
      );
    return () => timers.forEach(clearTimeout);
  }, [runs, s.online]);
  return null;
}

export function CloudExplorer() {
  const { state: s, dispatch, open, go, compact } = useDemo();
  const root = useRef<HTMLDivElement>(null),
    scroller = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(0),
    [drag, setDrag] = useState(false),
    [treeOpen, setTreeOpen] = useState(false),
    [mobileSearch, setMobileSearch] = useState(false);
  const loc = browseLocation(s),
    node = s.storage.nodes.find((n) => n.id === loc.directoryId),
    c = s.connections.find((c) => c.id === loc.connectionId);
  const [query, setQuery] = useState(loc.query?.query || ""),
    [composing, setComposing] = useState(false),
    [refresh, setRefresh] = useState(0),
    [saveOpen, setSaveOpen] = useState(false),
    [viewName, setViewName] = useState("");
  const q = useMemo(() => ({ ...browseQuery(s), query }), [loc, query]);
  const { found, pending, ready } = useFind(
    q,
    composing,
    refresh + s.browse.revision,
  );
  const searching = loc.kind === "query" || !!query.trim();
  useLayoutEffect(() => {
    const observer = new ResizeObserver(([e]) => setWidth(e.contentRect.width));
    if (root.current) observer.observe(root.current);
    return () => observer.disconnect();
  }, []);
  const restoration = useRef<{
    scroll: number;
    focus: string;
    pending: boolean;
  }>({ scroll: 0, focus: "", pending: false });
  useLayoutEffect(() => {
    setQuery(loc.query?.query || "");
    setMobileSearch(false);
    restoration.current = {
      scroll: loc.scroll,
      focus: loc.focus,
      pending: true,
    };
  }, [s.browse.revision, loc.directoryId, loc.kind]);
  useLayoutEffect(() => {
    if (!restoration.current.pending || !scroller.current) return;
    if (loc.kind === "query" && (!ready || query !== (loc.query?.query || "")))
      return;
    if (node && ["unloaded", "loading"].includes(node.status)) return;
    const target = restoration.current;
    target.pending = false;
    scroller.current.scrollTop = target.scroll;
    if (target.focus)
      scroller.current
        .querySelector<HTMLElement>(
          `[data-find-entry="${CSS.escape(target.focus)}"] [data-entry-name]`,
        )
        ?.focus({ preventScroll: true });
  }, [s.browse.revision, ready, query, node?.status, loc.kind]);
  const checkpoint = () =>
    dispatch({
      type: "browse-checkpoint",
      scroll: scroller.current?.scrollTop || 0,
      focus:
        (document.activeElement as HTMLElement)?.closest<HTMLElement>(
          "[data-find-entry]",
        )?.dataset.findEntry || loc.focus,
    });
  const navigate = (id: string) => {
    checkpoint();
    dispatch({
      type: "directory-open",
      id,
      scroll: scroller.current?.scrollTop || 0,
    });
    setTreeOpen(false);
  };
  const submit = () => {
    if (composing) return;
    checkpoint();
    dispatch({ type: "browse-query", query: q });
    setRefresh((v) => v + 1);
  };
  const direct: FindEntry[] = [
    ...s.storage.nodes
      .filter((n) => n.parentId === loc.directoryId)
      .sort((a, b) => a.name.localeCompare(b.name, "zh-CN"))
      .map((n) => ({
        ref: entryRef(s, n.id, "directory")!,
        name: n.name,
        path: n.path,
        reason: "当前目录",
        available: directoryAvailable(s, n),
        source: "本次已加载目录",
        rank: 0,
      })),
    ...directoryFiles(s, loc.directoryId)
      .filter((f) => matchesFilter(f, s.browser.filter))
      .sort((a, b) =>
        s.browser.sort === "name"
          ? a.name.localeCompare(b.name, "zh-CN")
          : s.browser.sort === "size"
            ? bytes(b.size) - bytes(a.size)
            : (b.updatedAt || "").localeCompare(a.updatedAt || ""),
      )
      .map((f) => ({
        ref: entryRef(s, f.id)!,
        name: f.name,
        path: f.folder,
        file: f,
        reason: "当前目录",
        available: "",
        source: "本次已加载目录",
        rank: 0,
      })),
  ];
  const resultSort = loc.kind === "query" ? s.browser.sort : "relevance";
  const entries = searching
      ? sortEntries(found, resultSort)
      : sortEntries(direct, s.browser.sort),
    pageSize = s.browser.layout === "list" || searching ? 50 : 24,
    page = Math.min(
      searching && query !== (loc.query?.query || "") ? 1 : s.browser.page,
      Math.max(1, Math.ceil(entries.length / pageSize)),
    ),
    shown = entries.slice((page - 1) * pageSize, page * pageSize),
    error = directoryAvailable(s, node),
    narrow = width < 760;
  const ancestors: DirectoryNode[] = [];
  let ancestor = node;
  while (ancestor) {
    ancestors.unshift(ancestor);
    ancestor = s.storage.nodes.find((n) => n.id === ancestor!.parentId);
  }
  if (s.organization.large) return <LargeCatalog />;
  const searchForm = (
    <form
      className="flex min-w-0 flex-1 items-center"
      onSubmit={(e) => {
        e.preventDefault();
        submit();
      }}
    >
      <Input
        autoFocus={mobileSearch}
        aria-label="搜索当前目录"
        className="!h-8 min-w-0 text-xs [@media(pointer:coarse)]:!h-11 max-[767px]:!h-11"
        placeholder="搜索此目录及子目录"
        value={query}
        onCompositionStart={() => setComposing(true)}
        onCompositionEnd={() => {
          setComposing(false);
          if (s.selected.length) dispatch({ type: "select", ids: [] });
        }}
        onChange={(e) => {
          setQuery(e.target.value);
          if (!composing && s.selected.length)
            dispatch({ type: "select", ids: [] });
        }}
      />
      <IconBtn
        label="检索当前目录"
        type="submit"
        className="!size-8 !min-w-0 [@media(pointer:coarse)]:!size-11 max-[767px]:!size-11"
      >
        <Search size={16} />
      </IconBtn>
      {mobileSearch && (
        <IconBtn label="关闭目录搜索" onClick={() => setMobileSearch(false)}>
          <X size={15} />
        </IconBtn>
      )}
    </form>
  );
  return (
    <div
      ref={root}
      data-cloud-explorer
      tabIndex={-1}
      className="flex h-full min-h-0 min-w-0 outline-none"
      onKeyDown={(e) => {
        if (
          !e.altKey ||
          e.defaultPrevented ||
          !(e.target instanceof HTMLElement) ||
          e.target.closest(
            'input,textarea,[contenteditable],[role="menu"],[role="dialog"],[role="listbox"]',
          ) ||
          document.querySelector('[role="dialog"],[role="menu"]')
        )
          return;
        if (["ArrowLeft", "ArrowRight", "ArrowUp"].includes(e.key)) {
          e.preventDefault();
          checkpoint();
          dispatch(
            e.key === "ArrowUp"
              ? { type: "browse-up" }
              : {
                  type: "browse-history",
                  direction: e.key === "ArrowLeft" ? "back" : "forward",
                },
          );
        }
      }}
    >
      {!narrow && (
        <>
          <aside
            className="min-h-0 shrink-0 border-r bg-card"
            style={{ width: s.storage.treeWidth }}
          >
            <ResourceExplorer onChoose={navigate} />
          </aside>
          <div
            role="separator"
            tabIndex={0}
            aria-label="调整目录树宽度"
            aria-orientation="vertical"
            aria-valuemin={200}
            aria-valuemax={320}
            aria-valuenow={s.storage.treeWidth}
            className="w-1 shrink-0 cursor-col-resize hover:bg-primary/25 focus:bg-primary/25"
            onDoubleClick={() =>
              dispatch({ type: "directory-width", width: 220 })
            }
            onKeyDown={(e) => {
              if (["ArrowLeft", "ArrowRight", "Home"].includes(e.key)) {
                e.preventDefault();
                dispatch({
                  type: "directory-width",
                  width:
                    e.key === "Home"
                      ? 220
                      : s.storage.treeWidth +
                        (e.key === "ArrowRight" ? 16 : -16),
                });
              }
            }}
            onPointerDown={(e) => {
              setDrag(true);
              e.currentTarget.setPointerCapture(e.pointerId);
            }}
            onPointerMove={(e) => {
              if (drag && root.current)
                dispatch({
                  type: "directory-width",
                  width: e.clientX - root.current.getBoundingClientRect().left,
                });
            }}
            onPointerUp={() => setDrag(false)}
            onPointerCancel={() => setDrag(false)}
          />
        </>
      )}
      <section className="flex min-h-0 min-w-0 flex-1 flex-col">
        <header
          data-directory-toolbar
          className="flex h-11 shrink-0 items-center gap-1 border-b bg-card px-1 sm:px-2"
        >
          {narrow && (
            <IconBtn
              label="打开存储目录"
              className="!size-8 !min-w-0 [@media(pointer:coarse)]:!size-11 max-[767px]:!size-11"
              onClick={() => setTreeOpen(true)}
            >
              <Folder size={16} />
            </IconBtn>
          )}
          {!mobileSearch && <BrowseControls more />}
          {!mobileSearch && (
            <div
              aria-label="目录路径"
              className="flex min-w-0 flex-1 items-center overflow-hidden text-xs"
            >
              {ancestors.map((a, i) => {
                if (ancestors.length > 3 && i > 0 && i < ancestors.length - 2) {
                  if (i !== 1) return null;
                  return (
                    <DropdownMenu key="ancestors">
                      <DropdownMenuTrigger
                        render={
                          <Btn
                            variant="ghost"
                            className="!h-8 !min-w-6 !px-1"
                            aria-label="中间目录层级"
                          >
                            …
                          </Btn>
                        }
                      />
                      <DropdownMenuContent>
                        {ancestors.slice(1, -2).map((a) => (
                          <DropdownMenuItem
                            key={a.id}
                            className="min-h-11"
                            onClick={() => navigate(a.id)}
                          >
                            {a.name}
                          </DropdownMenuItem>
                        ))}
                      </DropdownMenuContent>
                    </DropdownMenu>
                  );
                }
                return (
                  <button
                    key={a.id}
                    className={`${i < ancestors.length - 1 ? "hidden lg:flex" : "flex"} h-8 min-w-0 items-center gap-1 px-1 [@media(pointer:coarse)]:h-11`}
                    title={a.path}
                    onClick={() => navigate(a.id)}
                  >
                    {i > 0 && <ChevronRight size={12} className="shrink-0" />}
                    <span className="truncate">{a.name}</span>
                  </button>
                );
              })}
              {searching && (
                <span className="shrink-0 text-muted-foreground"> / 检索</span>
              )}
            </div>
          )}
          {compact ? (
            mobileSearch ? (
              searchForm
            ) : (
              <IconBtn
                label="搜索当前目录"
                onClick={() => setMobileSearch(true)}
              >
                <Search size={16} />
              </IconBtn>
            )
          ) : (
            <div className="min-w-0 max-w-64 flex-1">{searchForm}</div>
          )}
          {!mobileSearch && (
            <>
              <ScopedFilter
                value={q}
                onApply={(query) => {
                  checkpoint();
                  dispatch({ type: "browse-query", query });
                }}
              />
              {!compact && node && (
                <FavoriteButton
                  reference={entryRef(s, node.id, "directory")!}
                  label="当前目录"
                />
              )}
              <IconBtn
                className="max-[767px]:hidden"
                label="上传或新建"
                onClick={() => open({ type: "upload" })}
              >
                <Plus size={17} />
              </IconBtn>
            </>
          )}
        </header>
        <div className="px-2">
          <FileToolbar
            count={entries.filter((x) => x.file).length}
            shown={shown
              .filter((x) => x.file && !x.available)
              .map((x) => x.ref.id)}
            matched={entries
              .filter((x) => x.file && !x.available)
              .map((x) => x.ref.id)}
            page={page}
            pages={Math.max(1, Math.ceil(entries.length / pageSize))}
            summary={
              !searching
                ? `${entries.filter((x) => !x.file).length} 个文件夹 · ${entries.filter((x) => x.file).length} 个文件`
                : undefined
            }
            extra={
              !s.selected.length && searching ? (
                <div className="flex min-w-0 items-center gap-1 overflow-hidden text-[10px]">
                  <span
                    className="truncate text-muted-foreground"
                    title={`${c?.name} / ${node?.path}`}
                  >
                    {q.recursive === false ? "仅本层" : "含子目录"} ·{" "}
                    {pending ? "查询中…" : "已加载及已索引范围"}
                  </span>
                  <FilterConditions
                    value={q}
                    onChange={(query) =>
                      dispatch({ type: "browse-query", query })
                    }
                  />
                  <IconBtn
                    label="保存当前检索"
                    className="!size-8 !min-w-0 [@media(pointer:coarse)]:!size-11 max-[767px]:!size-11"
                    onClick={() => setSaveOpen(true)}
                  >
                    <Bookmark size={14} />
                  </IconBtn>
                </div>
              ) : undefined
            }
          />
        </div>
        <div
          ref={scroller}
          data-directory-scroll
          className="min-h-0 flex-1 overflow-auto px-2 pb-2"
          onFocusCapture={(e) => {
            const row = (e.target as HTMLElement).closest<HTMLElement>(
              "[data-find-entry]",
            );
            if (row)
              dispatch({
                type: "browse-checkpoint",
                focus: row.dataset.findEntry,
              });
          }}
          onScroll={(e) =>
            !restoration.current.pending &&
            dispatch({
              type: "browse-checkpoint",
              scroll: e.currentTarget.scrollTop,
            })
          }
          onDragOver={(e) => e.preventDefault()}
          onDrop={(e) => {
            e.preventDefault();
            open({ type: "upload" });
          }}
        >
          {error ? (
            <Empty
              title="目录暂不可用"
              text={error}
              action="连接说明"
              onAction={() =>
                dispatch({ type: "connection-inspect", id: loc.connectionId })
              }
            />
          ) : !searching &&
            node &&
            ["loading", "unloaded", "failed"].includes(node.status) ? (
            <Empty
              title={
                node.status === "failed"
                  ? "目录加载失败"
                  : node.status === "unloaded"
                    ? "目录尚未加载"
                    : "正在加载目录…"
              }
              text="此状态不代表目录为空。"
              action={node.status === "failed" ? "重试原目录" : "加载此目录"}
              onAction={() =>
                dispatch({
                  type: "directory-status",
                  id: node.id,
                  status: "loading",
                })
              }
            />
          ) : s.organization.table ? (
            <>
              <FileDetailsList
                entries={shown.filter((x) => !x.file)}
                onDirectory={navigate}
              />
              <PropertyTable
                files={shown.flatMap((x) => (x.file ? [x.file] : []))}
              />
            </>
          ) : searching || s.browser.layout === "list" ? (
            <FileDetailsList
              entries={shown}
              label={searching ? "当前目录检索结果" : "文件区域"}
              query={query}
              onDirectory={navigate}
              sort={searching ? resultSort : s.browser.sort}
              onSort={(sort) => {
                if (searching && loc.kind !== "query")
                  dispatch({ type: "browse-query", query: q });
                dispatch({ type: "browser", patch: { sort } });
              }}
              crossConnections={searching}
            />
          ) : (
            <div
              aria-label="文件区域"
              className="grid grid-cols-2 gap-3 lg:grid-cols-[repeat(auto-fill,minmax(180px,1fr))]"
            >
              {shown.map((x) =>
                x.file ? (
                  <AssetCard
                    key={x.ref.id}
                    file={x.file}
                    selected={s.selected.includes(x.ref.id)}
                    onSelect={() =>
                      dispatch({
                        type: "select",
                        ids: s.selected.includes(x.ref.id)
                          ? s.selected.filter((id) => id !== x.ref.id)
                          : [...s.selected, x.ref.id],
                      })
                    }
                    onOpen={() =>
                      open({
                        type: "preview",
                        id: x.ref.id,
                        scope: {
                          label: `${c?.name} / ${node?.path}`,
                          refs: entries.flatMap((x) =>
                            x.file ? [fileRef(x.file)] : [],
                          ),
                        },
                      })
                    }
                  />
                ) : (
                  <button
                    key={x.ref.id}
                    data-directory-id={x.ref.id}
                    className="flex h-28 flex-col items-center justify-center gap-2 rounded-lg border bg-card text-xs"
                    onClick={() => navigate(x.ref.id)}
                  >
                    <Folder size={24} />
                    {x.name}
                  </button>
                ),
              )}
            </div>
          )}
          {node?.status === "partial" && (
            <Btn
              variant="ghost"
              onClick={() =>
                dispatch({
                  type: "directory-status",
                  id: node.id,
                  status: "ready",
                })
              }
            >
              当前为部分目录，加载剩余条目
            </Btn>
          )}
        </div>
        <footer className="flex h-11 shrink-0 items-center gap-1 border-t px-2 text-xs text-muted-foreground">
          <span className="mr-auto min-w-0 truncate">
            {searching ? "检索不会改变原目录" : "原目录"} · 第 {page} 页
          </span>
          {[
            { id: "list", name: "列表视图", icon: List },
            { id: "grid", name: "图库视图", icon: Grid2X2 },
            { id: "gallery", name: "画廊视图", icon: Columns2 },
          ].map((x) => (
            <IconBtn
              key={x.id}
              label={x.name}
              aria-pressed={s.browser.layout === x.id && !s.organization.table}
              onClick={() => {
                dispatch({ type: "property-table", patch: { table: false } });
                dispatch({
                  type: "browser",
                  patch: { layout: x.id as "list" | "grid" | "gallery" },
                });
              }}
            >
              <x.icon size={15} />
            </IconBtn>
          ))}
          <Btn
            variant="ghost"
            aria-label="切换属性表格"
            onClick={() =>
              dispatch({
                type: "property-table",
                patch: { table: !s.organization.table },
              })
            }
          >
            属性
          </Btn>
          <IconBtn
            label="上一页"
            disabled={page <= 1}
            onClick={() =>
              dispatch({ type: "browser", patch: { page: page - 1 } })
            }
          >
            <ArrowLeft size={15} />
          </IconBtn>
          <IconBtn
            label="下一页"
            disabled={page * pageSize >= entries.length}
            onClick={() =>
              dispatch({ type: "browser", patch: { page: page + 1 } })
            }
          >
            <ChevronRight size={15} />
          </IconBtn>
        </footer>
      </section>
      <Dialog open={treeOpen} onOpenChange={setTreeOpen}>
        <DialogContent className="!h-[85dvh] !gap-0 !p-0">
          <div className="p-3">
            <DialogTitle>文件资源</DialogTitle>
            <DialogDescription>
              目录、标签和属性共用当前连接范围。
            </DialogDescription>
          </div>
          <ResourceExplorer
            onChoose={navigate}
            onQuery={() => setTreeOpen(false)}
          />
        </DialogContent>
      </Dialog>
      <Dialog open={saveOpen} onOpenChange={setSaveOpen}>
        <DialogContent>
          <DialogTitle>保存搜索视图</DialogTitle>
          <DialogDescription>
            保留当前范围、条件与属性视图设置。
          </DialogDescription>
          <Input
            aria-label="保存搜索名称"
            value={viewName}
            onChange={(e) => setViewName(e.target.value)}
          />
          <Btn
            disabled={!viewName.trim()}
            onClick={() => {
              dispatch({
                type: "search-save",
                name: viewName,
                spec: {
                  ...q,
                  view: {
                    columns: s.organization.columns,
                    widths: s.organization.widths,
                    groupBy: s.organization.groupBy,
                    sortKey: s.organization.sortKey,
                    listSort: resultSort,
                  },
                },
              });
              setSaveOpen(false);
            }}
          >
            保存视图
          </Btn>
        </DialogContent>
      </Dialog>
    </div>
  );
}
