import { FileDetailsList } from "./FileDetailsList";
import { ListPagination } from "./CompactUI";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import {
  Star,
  Folder,
  Pin,
  MoreHorizontal,
  ArrowUp,
  Search,
  Plus,
  X,
} from "lucide-react";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Btn, IconBtn, Choice, Empty, useDemo } from "./ui";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "./Dialog";
import {
  bookmarkFor,
  currentBookmarks,
  entryRef,
  entryUnavailable,
} from "./organization-model";
import type { EntryRef, Bookmark } from "./organization-model";
import { FileButton } from "./HoverPreview";
import { fileRef } from "./preview-model";
import { BrowseControls } from "./BrowseControls";
import { FileToolbar } from "./CloudToolbar";
export function FavoriteButton({
  reference: r,
  label,
}: {
  reference: EntryRef;
  label: string;
}) {
  const { state: s, dispatch } = useDemo();
  const b = bookmarkFor(s, r);
  return (
    <IconBtn
      label={`${b ? "取消收藏" : "收藏"} ${label}`}
      aria-pressed={!!b}
      className="!size-8 !min-h-0 !min-w-0 shrink-0 max-[767px]:!size-11 [@media(pointer:coarse)]:!size-11"
      onClick={(e) => {
        e.stopPropagation();
        dispatch({ type: "bookmark-toggle", ref: r });
      }}
    >
      <Star size={16} className={b ? "fill-amber-500/20 text-amber-600" : ""} />
    </IconBtn>
  );
}
export function ConnectionFavorites({
  connectionId,
  onChosen,
  compactTree = false,
}: {
  connectionId: string;
  onChosen?: () => void;
  compactTree?: boolean;
}) {
  const { state: s, dispatch, open } = useDemo();
  const c = s.connections.find((c) => c.id === connectionId)!;
  const pins = currentBookmarks(s, connectionId)
    .filter((b) => b.pin !== null)
    .sort((a, b) => a.pin! - b.pin!)
    .slice(0, 5);
  return (
    <div
      className={
        compactTree
          ? "ml-6 border-l border-border/50 pl-1 text-xs"
          : "ml-5 border-l pl-2 text-xs"
      }
    >
      <Btn
        className={
          compactTree
            ? "!h-8 !min-h-0 w-full !justify-start !px-2 text-xs max-[767px]:!h-11 [@media(pointer:coarse)]:!h-11"
            : "!h-11 w-full !justify-start !px-2"
        }
        variant="ghost"
        onClick={() => {
          dispatch({ type: "favorites-open", connectionId });
          onChosen?.();
        }}
      >
        <Star size={15} />
        <span>本盘收藏</span>
        <span className="ml-auto text-muted-foreground">
          {currentBookmarks(s, connectionId).length}
        </span>
        <span className="sr-only">{c.name}</span>
      </Btn>
      {connectionId === s.activeConnection &&
        pins.map((b) => (
          <button
            key={b.id}
            className={`flex ${compactTree ? "h-8 max-[767px]:h-11 [@media(pointer:coarse)]:h-11" : "h-11"} w-full items-center gap-2 px-2 text-left`}
            title={`${b.name} · ${b.path}`}
            onClick={() => {
              if (entryUnavailable(s, b.ref) || b.invalid) {
                dispatch({
                  type: "notice",
                  text: b.invalid || entryUnavailable(s, b.ref),
                });
                return;
              }
              if (b.ref.kind === "directory")
                dispatch({ type: "directory-open", id: b.ref.id });
              else open({ type: "preview", id: b.ref.id, single: true });
              onChosen?.();
            }}
          >
            <Pin size={12} />
            <span className="truncate">{b.name}</span>
          </button>
        ))}
    </div>
  );
}
export function BookmarkGroups({ refs }: { refs: EntryRef[] }) {
  const { state: s, dispatch } = useDemo();
  const [name, setName] = useState("");
  const cs = [...new Set(refs.map((r) => r.connectionId))];
  return (
    <div className="space-y-3">
      <p className="text-xs text-muted-foreground">
        收藏只属于当前用户，不写入源网盘。选择组会追加关系；取消勾选仅从此组移除。
      </p>
      {cs.map((connectionId) => (
        <section key={connectionId} className="space-y-2 border-b pb-3">
          <h3 className="text-sm font-medium">
            {s.connections.find((c) => c.id === connectionId)?.name}
          </h3>
          {s.organization.groups
            .filter(
              (g) =>
                g.user === s.actor &&
                g.space === s.space &&
                g.connectionId === connectionId,
            )
            .map((g) => {
              const current = refs.filter(
                  (r) => r.connectionId === connectionId,
                ),
                count = current.filter((r) =>
                  bookmarkFor(s, r)?.groups.includes(g.id),
                ).length;
              return (
                <label
                  key={g.id}
                  className="flex min-h-11 items-center gap-2 text-sm"
                >
                  <Checkbox
                    checked={count === current.length}
                    indeterminate={count > 0 && count < current.length}
                    onCheckedChange={(v) => {
                      if (v)
                        dispatch({
                          type: "bookmark-add",
                          refs: current,
                          groups: [g.id],
                        });
                      else
                        current.forEach((r) => {
                          const b = bookmarkFor(s, r);
                          if (b)
                            dispatch({
                              type: "bookmark-update",
                              id: b.id,
                              groups: b.groups.filter((id) => id !== g.id),
                            });
                        });
                    }}
                  />
                  {g.name}
                  <span className="ml-auto text-xs text-muted-foreground">
                    {count === current.length
                      ? "全部拥有"
                      : count
                        ? "部分拥有"
                        : "未加入"}
                  </span>
                </label>
              );
            })}
          <div className="flex gap-2">
            <Input
              aria-label={`新收藏夹 ${connectionId}`}
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="单层收藏夹名称"
            />
            <Btn
              disabled={!name.trim()}
              onClick={() => {
                dispatch({ type: "bookmark-group", connectionId, name });
                setName("");
              }}
            >
              创建
            </Btn>
          </div>
        </section>
      ))}
      <Btn onClick={() => dispatch({ type: "bookmark-add", refs })}>
        加入未分组收藏
      </Btn>
    </div>
  );
}
export function FavoritesWorkspace() {
  const { state: s, dispatch, open } = useDemo();
  const o = s.organization,
    v = o.favorites,
    c = s.connections.find((c) => c.id === v.connectionId);
  const [editing, setEditing] = useState<Bookmark | null>(null),
    [groupEdit, setGroupEdit] = useState<string | null>(null),
    [name, setName] = useState("");
  const [page, setPage] = useState(1);
  useEffect(() => setPage(1), [v.group, v.query, v.connectionId]);
  const scroller = useRef<HTMLDivElement>(null);
  useLayoutEffect(() => {
    if (scroller.current) scroller.current.scrollTop = v.scroll;
  }, []);
  const groups = o.groups.filter(
    (g) =>
      g.user === s.actor &&
      g.space === s.space &&
      g.connectionId === v.connectionId,
  );
  const all = currentBookmarks(s, v.connectionId),
    items = all
      .filter(
        (b) =>
          (v.group === "all" || v.group === "ungrouped"
            ? !b.groups.length || v.group === "all"
            : b.groups.includes(v.group)) &&
          (b.name + " " + b.path).toLowerCase().includes(v.query.toLowerCase()),
      )
      .sort((a, b) => (a.pin ?? 999) - (b.pin ?? 999));
  const visit = (b: Bookmark) => {
    dispatch({
      type: "favorites-view",
      patch: { scroll: scroller.current?.scrollTop || 0 },
    });
    dispatch({ type: "entry-visit", id: b.ref.id });
    if (b.ref.kind === "directory")
      dispatch({ type: "bookmark-directory-visit", id: b.ref.id });
    else
      open({
        type: "preview",
        id: b.ref.id,
        scope: {
          label: `${c?.name} · 收藏`,
          refs: items
            .filter(
              (x) =>
                x.ref.kind === "file" &&
                !entryUnavailable(s, x.ref) &&
                !x.invalid,
            )
            .map((x) => fileRef(s.files.find((f) => f.id === x.ref.id)!)),
        },
      });
  };
  return (
    <section aria-label="本盘收藏" className="mx-auto max-w-6xl">
      <header className="flex h-11 items-center gap-2 border-b">
        <BrowseControls more />
        <Star size={18} className="hidden sm:block text-amber-600" />
        <h1 className="hidden min-w-0 max-w-48 truncate text-sm font-semibold sm:block">
          {c?.name} · 收藏
        </h1>
        <IconBtn
          label="新建收藏夹"
          onClick={() => {
            setName("");
            setGroupEdit("new");
          }}
        >
          <Plus size={16} />
        </IconBtn>
        <Input
          aria-label="搜索本盘收藏"
          value={v.query}
          placeholder="按已知名称、完整路径查找"
          onChange={(e) =>
            dispatch({
              type: "favorites-view",
              patch: { query: e.target.value },
            })
          }
        />
        <IconBtn
          label="清除收藏搜索"
          disabled={!v.query}
          className={!v.query ? "invisible" : ""}
          onClick={() =>
            dispatch({ type: "favorites-view", patch: { query: "" } })
          }
        >
          <X size={14} />
        </IconBtn>
      </header>
      <FileToolbar
        extra={
          <>
            {" "}
            <Choice
              label="收藏夹"
              value={v.group}
              onChange={(group) =>
                dispatch({
                  type: "favorites-view",
                  patch: { group, scroll: 0 },
                })
              }
              options={[
                { value: "all", label: "全部收藏" },
                { value: "ungrouped", label: "未分组" },
                ...groups.map((g) => ({ value: g.id, label: g.name })),
              ]}
            />
            {groups.some((g) => g.id === v.group) && (
              <IconBtn
                label="管理收藏夹"
                onClick={() => {
                  setGroupEdit(v.group);
                  setName(groups.find((g) => g.id === v.group)!.name);
                }}
              >
                <MoreHorizontal size={17} />
              </IconBtn>
            )}
            <ListPagination
              page={page}
              pages={Math.max(1, Math.ceil(items.length / 50))}
              onChange={setPage}
            />
          </>
        }
        count={items.length}
        page={page}
        pages={Math.max(1, Math.ceil(items.length / 50))}
        shown={items
          .slice((page - 1) * 50, page * 50)
          .filter((b) => b.ref.kind === "file" && !entryUnavailable(s, b.ref))
          .map((b) => b.ref.id)}
      />
      <div ref={scroller} className="max-h-[calc(100dvh-160px)] overflow-auto">
        <FileDetailsList
          label="本盘收藏"
          entries={items.slice((page - 1) * 50, page * 50).map((b) => ({
            ref: b.ref,
            name: b.name,
            path: b.path,
            file:
              b.ref.kind === "file"
                ? s.files.find((f) => f.id === b.ref.id)
                : undefined,
            reason: "个人收藏",
            source: "个人收藏",
            available: b.invalid || entryUnavailable(s, b.ref),
            rank: 0,
          }))}
          bookmarkIds={Object.fromEntries(items.map((b) => [b.ref.id, b.id]))}
          onOpen={(r) => {
            const b = items.find((b) => b.ref.id === r.ref.id);
            if (b) visit(b);
          }}
          rowActions={(r) => (
            <IconBtn
              label={`管理收藏 ${r.name}`}
              className="!size-8 !min-h-0 !min-w-0 max-[767px]:!size-11 [@media(pointer:coarse)]:!size-11"
              onClick={() =>
                setEditing(items.find((b) => b.ref.id === r.ref.id)!)
              }
            >
              <MoreHorizontal size={15} />
            </IconBtn>
          )}
          columns={[
            {
              id: "path",
              label: "所在目录",
              width: 180,
              at: 560,
              render: (r) => (
                <span title={`${c?.name} / ${r.path}`}>{r.path}</span>
              ),
            },
            {
              id: "group",
              label: "收藏夹",
              width: 120,
              at: 720,
              render: (r) =>
                items
                  .find((b) => b.ref.id === r.ref.id)
                  ?.groups.map((id) => groups.find((g) => g.id === id)?.name)
                  .join("、") || "未分组",
            },
          ]}
        />
      </div>
      <Dialog open={!!editing} onOpenChange={(v) => !v && setEditing(null)}>
        <DialogContent>
          <DialogTitle>管理个人收藏</DialogTitle>
          <DialogDescription>
            {editing?.name} · {c?.name}
          </DialogDescription>
          {editing && (
            <>
              <BookmarkGroups refs={[editing.ref]} />
              <Btn
                variant="outline"
                onClick={() => {
                  const b = all.find((b) => b.id === editing.id)!;
                  dispatch({
                    type: "bookmark-update",
                    id: b.id,
                    pin:
                      b.pin === null
                        ? Math.max(0, ...all.map((b) => b.pin ?? 0)) + 1
                        : null,
                  });
                }}
              >
                {all.find((b) => b.id === editing.id)?.pin === null
                  ? "置顶快捷入口"
                  : "取消置顶"}
              </Btn>
              <Btn
                variant="ghost"
                onClick={() =>
                  dispatch({
                    type: "bookmark-update",
                    id: editing.id,
                    pin: Math.min(0, ...all.map((b) => b.pin ?? 0)) - 1,
                  })
                }
              >
                <ArrowUp size={15} />
                移到置顶第一位
              </Btn>
              {v.group !== "all" && v.group !== "ungrouped" && (
                <Btn
                  variant="ghost"
                  onClick={() => {
                    dispatch({
                      type: "bookmark-update",
                      id: editing.id,
                      groups: all
                        .find((b) => b.id === editing.id)!
                        .groups.filter((id) => id !== v.group),
                    });
                    setEditing(null);
                  }}
                >
                  从本收藏夹移除
                </Btn>
              )}
              {editing.ref.kind === "directory" && (
                <Btn
                  variant="outline"
                  onClick={() => {
                    dispatch({ type: "navigate", page: "search" });
                    dispatch({
                      type: "search-patch",
                      patch: {
                        directoryId: editing.ref.id,
                        connections: [editing.ref.connectionId],
                        recursive: true,
                        folder: "全部文件",
                        query: "",
                      },
                    });
                    dispatch({ type: "search-submit" });
                    setEditing(null);
                  }}
                >
                  <Search size={15} />
                  在收藏目录内检索
                </Btn>
              )}
              <Btn
                variant="ghost"
                onClick={() => {
                  dispatch({
                    type: "bookmark-update",
                    id: editing.id,
                    remove: true,
                  });
                  setEditing(null);
                }}
              >
                取消收藏（全部分组）
              </Btn>
            </>
          )}
        </DialogContent>
      </Dialog>
      <Dialog
        open={groupEdit !== null}
        onOpenChange={(v) => !v && setGroupEdit(null)}
      >
        <DialogContent>
          <DialogTitle>收藏夹名称</DialogTitle>
          <DialogDescription>
            仅组织快捷引用，不创建真实目录。
          </DialogDescription>
          <Input
            aria-label="收藏夹名称"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
          <Btn
            disabled={!name.trim()}
            onClick={() => {
              dispatch({
                type: "bookmark-group",
                connectionId: v.connectionId,
                id: groupEdit === "new" ? undefined : groupEdit!,
                name,
              });
              setGroupEdit(null);
            }}
          >
            保存收藏夹
          </Btn>
          {groupEdit !== "new" && (
            <Btn
              variant="ghost"
              onClick={() => {
                dispatch({
                  type: "bookmark-group",
                  connectionId: v.connectionId,
                  id: groupEdit!,
                  remove: true,
                });
                setGroupEdit(null);
              }}
            >
              删除收藏夹，保留收藏条目
            </Btn>
          )}
        </DialogContent>
      </Dialog>
    </section>
  );
}
