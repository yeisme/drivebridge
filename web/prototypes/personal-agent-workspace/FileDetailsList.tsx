import type { ReactNode } from "react";
import type { State, FileRecord } from "./model";
import {
  Folder,
  Star,
  MoreHorizontal,
  FolderOpen,
  FileText,
  Image,
  Music2,
  Video,
  Sheet,
  Pencil,
} from "lucide-react";
import { Checkbox } from "@/components/ui/checkbox";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
import { Btn, IconBtn, Empty, useDemo } from "./ui";
import { FileButton } from "./HoverPreview";
import { fileRef } from "./preview-model";
import { FileTags } from "./TagWorkspace";
import { bookmarkFor, entryUnavailable, entryRef } from "./organization-model";
import type { FindEntry } from "./organization-model";
function Highlight({ name, query }: { name: string; query?: string }) {
  const i = query
    ? name.toLocaleLowerCase().indexOf(query.toLocaleLowerCase())
    : -1;
  return (
    <span className="block truncate">
      {i < 0 ? (
        name
      ) : (
        <>
          {name.slice(0, i)}
          <mark className="bg-primary/15 text-primary">
            {name.slice(i, i + query!.length)}
          </mark>
          {name.slice(i + query!.length)}
        </>
      )}
    </span>
  );
}
export function entriesForFiles(s: State, files: FileRecord[]): FindEntry[] {
  return files.flatMap((file) => {
    const ref = entryRef(s, file.id);
    return ref
      ? [
          {
            ref,
            file,
            name: file.name,
            path: file.folder,
            reason: "当前视图",
            available: entryUnavailable(s, ref),
            source: "当前视图",
            rank: 0,
          },
        ]
      : [];
  });
}
export interface DetailColumn {
  id: string;
  label: string;
  width: number;
  at: 420 | 560 | 720 | 800;
  render: (entry: FindEntry) => ReactNode;
  sort?: string;
}
const visibility = {
  420: ["hidden @[420px]:table-column", "hidden @[420px]:table-cell"],
  560: ["hidden @[560px]:table-column", "hidden @[560px]:table-cell"],
  720: ["hidden @[720px]:table-column", "hidden @[720px]:table-cell"],
  800: ["hidden @[800px]:table-column", "hidden @[800px]:table-cell"],
};
export function FileDetailsList({
  entries,
  label = "文件区域",
  query = "",
  onDirectory,
  sort,
  onSort,
  crossConnections = false,
  columns,
  onOpen,
  onEdit,
  knowledge = false,
  rowActions,
  bookmarkIds,
  selectable = true,
}: {
  entries: FindEntry[];
  label?: string;
  query?: string;
  onDirectory?: (id: string) => void;
  sort?: string;
  onSort?: (value: string) => void;
  crossConnections?: boolean;
  columns?: DetailColumn[];
  onOpen?: (entry: FindEntry) => void;
  onEdit?: (entry: FindEntry) => void;
  knowledge?: boolean;
  rowActions?: (entry: FindEntry) => ReactNode;
  bookmarkIds?: Record<string, string>;
  selectable?: boolean;
}) {
  const { state: s, dispatch, open, compact } = useDemo();
  const SortButton = onSort ? "button" : "span";
  const navigate = (r: FindEntry) => {
    if (entryUnavailable(s, r.ref) || r.available) return;
    if (onOpen) return onOpen(r);
    if (r.file)
      open({
        type: "preview",
        id: r.ref.id,
        scope: {
          label,
          refs: entries
            .filter(
              (x) => x.file && !entryUnavailable(s, x.ref) && !x.available,
            )
            .map((x) => fileRef(x.file!)),
        },
      });
    else if (onDirectory) onDirectory(r.ref.id);
    else dispatch({ type: "directory-open", id: r.ref.id });
  };
  const select = (r: FindEntry, range = false) => {
    if (!r.file || !selectable) return;
    const ids = entries
      .filter((x) => x.file && !entryUnavailable(s, x.ref))
      .map((x) => x.ref.id);
    const anchor = s.browser.focusedId;
    if (range && ids.includes(anchor)) {
      const a = ids.indexOf(anchor),
        b = ids.indexOf(r.ref.id);
      dispatch({
        type: "select",
        ids: [
          ...new Set([
            ...s.selected,
            ...ids.slice(Math.min(a, b), Math.max(a, b) + 1),
          ]),
        ],
      });
    } else
      dispatch({
        type: "select",
        ids: s.selected.includes(r.ref.id)
          ? s.selected.filter((id) => id !== r.ref.id)
          : [...s.selected, r.ref.id],
      });
    dispatch({ type: "browser", patch: { focusedId: r.ref.id } });
  };
  const icon = (r: FindEntry) => {
    const I = !r.file
      ? Folder
      : r.file.kind === "image"
        ? Image
        : r.file.kind === "video"
          ? Video
          : r.file.kind === "audio"
            ? Music2
            : r.file.kind === "sheet"
              ? Sheet
              : FileText;
    return <I size={16} strokeWidth={1.65} className="shrink-0 text-primary" />;
  };
  const title = (r: FindEntry) =>
    r.file ? (
      <FileButton
        reference={fileRef(r.file)}
        variant="ghost"
        aria-label={`预览 ${r.name}`}
        data-entry-name
        title={`${r.name} · ${r.file?.externalRevision || (r.file ? `v${r.file.version}` : "目录")} · ${r.path} · ${r.reason}`}
        className="!h-8 !min-h-0 !w-full !min-w-0 !justify-start !rounded-sm !px-0 !text-[13px] !font-normal max-[767px]:!h-11 [@media(pointer:coarse)]:!h-11"
        onOpen={() => navigate(r)}
      >
        <Highlight name={r.name} query={query} />
        {knowledge && s.knowledge.notes[r.ref.id]?.dirty && (
          <span className="shrink-0 text-[10px] text-amber-600">草稿</span>
        )}
      </FileButton>
    ) : (
      <button
        data-entry-name
        title={`${r.name} · 目录 · ${r.path} · ${r.reason}`}
        disabled={!!r.available}
        className="flex h-8 min-w-0 w-full items-center text-left text-[13px] max-[767px]:h-11 [@media(pointer:coarse)]:h-11"
        onClick={() => navigate(r)}
      >
        <Highlight name={r.name} query={query} />
      </button>
    );
  const actions = (r: FindEntry) => {
    const b = bookmarkFor(s, r.ref);
    return (
      <div className="flex items-center justify-end">
        {rowActions?.(r)}
        {onEdit && (
          <IconBtn
            label="继续编辑"
            title={`编辑 ${r.name}`}
            className="!size-8 !min-h-0 !min-w-0 opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 max-[767px]:!size-11 max-[767px]:opacity-100 [@media(pointer:coarse)]:!size-11 [@media(pointer:coarse)]:opacity-100"
            onClick={() => onEdit(r)}
          >
            <Pencil size={14} />
          </IconBtn>
        )}
        <IconBtn
          label={`${b ? "取消收藏" : "收藏"} ${r.name}`}
          aria-pressed={!!b}
          className="!size-8 !min-h-0 !min-w-0 max-[767px]:!size-11 [@media(pointer:coarse)]:!size-11"
          onClick={() => dispatch({ type: "bookmark-toggle", ref: r.ref })}
        >
          <Star
            size={14}
            className={
              b ? "fill-amber-500/20 text-amber-600" : "text-muted-foreground"
            }
          />
        </IconBtn>
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <IconBtn
                label={`更多操作 ${r.name}`}
                className="!size-8 !min-h-0 !min-w-0 max-[767px]:!size-11 [@media(pointer:coarse)]:!size-11"
              >
                <MoreHorizontal size={15} />
              </IconBtn>
            }
          />
          <DropdownMenuContent align="end">
            <DropdownMenuItem
              className="min-h-11"
              disabled={!!r.available}
              onClick={() => navigate(r)}
            >
              打开{r.file ? "文件" : "目录"}
            </DropdownMenuItem>
            {r.file && (
              <>
                <DropdownMenuItem
                  className="min-h-11"
                  onClick={() =>
                    dispatch({
                      type: "directory-open",
                      id: r.file!.directoryId!,
                      focus: r.file!.id,
                    })
                  }
                >
                  所在目录并定位
                </DropdownMenuItem>
                <DropdownMenuItem
                  className="min-h-11"
                  onClick={() =>
                    open({ type: "bookmark-groups", ids: [r.file!.id] })
                  }
                >
                  选择收藏夹
                </DropdownMenuItem>
                <DropdownMenuItem
                  className="min-h-11"
                  onClick={() => open({ type: "label", ids: [r.file!.id] })}
                >
                  标签
                </DropdownMenuItem>
              </>
            )}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    );
  };
  const check = (r: FindEntry) =>
    !selectable ? null : r.file ? (
      <label className="flex size-8 items-center justify-center max-[767px]:size-11 [@media(pointer:coarse)]:size-11">
        <Checkbox
          aria-label={`选择 ${r.name}`}
          disabled={!!r.available}
          checked={s.selected.includes(r.ref.id)}
          onCheckedChange={(_, d) =>
            select(r, !!(d.event as MouseEvent).shiftKey)
          }
        />
      </label>
    ) : (
      <span className="block w-8 max-[767px]:w-11 [@media(pointer:coarse)]:w-11" />
    );
  return (
    <div
      aria-label={label}
      data-detail-list
      className="@container min-w-0"
      onKeyDown={(e) => {
        if (selectable && (e.ctrlKey || e.metaKey) && e.key === "a") {
          e.preventDefault();
          dispatch({
            type: "select",
            ids: entries
              .filter((r) => r.file && !r.available)
              .map((r) => r.ref.id),
          });
        }
        if (["ArrowDown", "ArrowUp"].includes(e.key)) {
          const buttons = Array.from(
              e.currentTarget.querySelectorAll<HTMLElement>(
                "[data-entry-name]",
              ),
            ),
            i = buttons.indexOf(e.target as HTMLElement);
          if (i >= 0) {
            e.preventDefault();
            buttons[
              Math.max(
                0,
                Math.min(
                  buttons.length - 1,
                  i + (e.key === "ArrowDown" ? 1 : -1),
                ),
              )
            ]?.focus();
          }
        }
      }}
    >
      {!entries.length ? (
        <Empty
          title="当前范围没有条目"
          text="目录层级保持原样；未扫描范围不代表没有文件。"
        />
      ) : compact ? (
        <div className="divide-y divide-border/60">
          {entries.map((r) => (
            <article
              key={r.ref.id}
              data-bookmark={bookmarkIds?.[r.ref.id]}
              data-file-id={r.file?.id}
              data-knowledge-file={knowledge ? r.file?.id : undefined}
              data-directory-id={!r.file ? r.ref.id : undefined}
              data-find-entry={r.ref.id}
              className={`group flex min-h-12 items-center gap-2 ${s.selected.includes(r.ref.id) ? "bg-accent/60" : ""}`}
              onClick={(e) => {
                if (
                  !(e.target as HTMLElement).closest(
                    'button,input,label,[role="checkbox"]',
                  )
                )
                  navigate(r);
              }}
            >
              {check(r)}
              {icon(r)}
              <div className="min-w-0 flex-1">
                {title(r)}
                <p className="-mt-1 truncate pb-1 text-[10px] text-muted-foreground">
                  {r.available ||
                    [
                      r.file?.size || "文件夹",
                      crossConnections
                        ? s.connections.find((c) => c.id === r.ref.connectionId)
                            ?.name
                        : r.file?.updatedAt,
                      r.file?.externalRevision ||
                        (r.file ? `v${r.file.version}` : ""),
                      crossConnections ? r.path + " · " + r.reason : "",
                    ]
                      .filter(Boolean)
                      .join(" · ")}
                </p>
              </div>
              {actions(r)}
            </article>
          ))}
        </div>
      ) : (
        <table className="w-full table-fixed border-collapse text-[12px]">
          <colgroup>
            <col
              className={
                selectable ? "w-8 [@media(pointer:coarse)]:w-11" : "w-1"
              }
            />
            <col />
            {columns ? (
              columns.map((c) => (
                <col
                  key={c.id}
                  className={visibility[c.at][0]}
                  style={{ width: c.width }}
                />
              ))
            ) : (
              <>
                <col className="hidden w-20 @[420px]:table-column" />
                <col className="hidden w-28 @[560px]:table-column" />
                {crossConnections && (
                  <col className="hidden w-40 @[800px]:table-column" />
                )}
                <col className="hidden w-36 @[720px]:table-column" />
              </>
            )}
            <col
              className={
                onEdit || rowActions
                  ? "w-24 [@media(pointer:coarse)]:w-[132px]"
                  : "w-16 [@media(pointer:coarse)]:w-[88px]"
              }
            />
          </colgroup>
          <thead className="sticky top-0 z-10 h-8 border-b bg-background text-muted-foreground">
            <tr>
              <th />
              <th className="text-left font-normal">
                <SortButton
                  className="h-8 w-full text-left [@media(pointer:coarse)]:h-11"
                  onClick={() =>
                    onSort?.(sort === "name" ? "name-desc" : "name")
                  }
                >
                  名称 {sort === "name" ? "↑" : sort === "name-desc" ? "↓" : ""}
                </SortButton>
              </th>
              {columns ? (
                columns.map((c) => (
                  <th
                    key={c.id}
                    className={`${visibility[c.at][1]} text-left font-normal`}
                  >
                    {c.label}
                  </th>
                ))
              ) : (
                <>
                  <th className="hidden text-left font-normal @[420px]:table-cell">
                    <SortButton
                      className="h-8 [@media(pointer:coarse)]:h-11"
                      onClick={() =>
                        onSort?.(sort === "size" ? "size-asc" : "size")
                      }
                    >
                      大小
                    </SortButton>
                  </th>
                  <th className="hidden text-left font-normal @[560px]:table-cell">
                    <SortButton
                      className="h-8 [@media(pointer:coarse)]:h-11"
                      onClick={() =>
                        onSort?.(sort === "recent" ? "oldest" : "recent")
                      }
                    >
                      修改时间{" "}
                      {sort === "recent" ? "↓" : sort === "oldest" ? "↑" : ""}
                    </SortButton>
                  </th>
                  {crossConnections && (
                    <th className="hidden text-left font-normal @[800px]:table-cell">
                      来源与目录
                    </th>
                  )}
                  <th className="hidden text-left font-normal @[720px]:table-cell">
                    标签
                  </th>
                </>
              )}
              <th>
                <span className="sr-only">操作</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {entries.map((r) => (
              <tr
                key={r.ref.id}
                data-bookmark={bookmarkIds?.[r.ref.id]}
                data-file-id={r.file?.id}
                data-knowledge-file={knowledge ? r.file?.id : undefined}
                data-directory-id={!r.file ? r.ref.id : undefined}
                data-find-entry={r.ref.id}
                data-entry-row
                tabIndex={-1}
                className={`group border-b border-border/35 ${s.preferences.density === "compact" ? "h-8" : "h-9"} [@media(pointer:coarse)]:h-12 ${s.selected.includes(r.ref.id) ? "bg-accent/60" : "hover:bg-muted/60"}`}
                onClick={(e) => {
                  if (
                    !(e.target as HTMLElement).closest(
                      'button,input,[role="checkbox"],label',
                    )
                  )
                    navigate(r);
                }}
              >
                <td>{check(r)}</td>
                <td className="pr-3">
                  <div className="flex min-w-0 items-center gap-2">
                    {icon(r)}
                    <div className="min-w-0 flex-1">{title(r)}</div>
                    {r.available && (
                      <span
                        className="shrink-0 text-[10px] text-amber-600"
                        title={r.available}
                      >
                        不可用
                      </span>
                    )}
                  </div>
                </td>
                {columns ? (
                  columns.map((c) => (
                    <td
                      key={c.id}
                      className={`${visibility[c.at][1]} truncate pr-2 text-xs text-muted-foreground`}
                    >
                      {c.render(r)}
                    </td>
                  ))
                ) : (
                  <>
                    <td className="hidden text-muted-foreground @[420px]:table-cell">
                      {r.file?.size || "—"}
                    </td>
                    <td
                      className="hidden truncate text-muted-foreground @[560px]:table-cell"
                      title={r.file?.updatedAt}
                    >
                      {r.file?.updatedAt || "—"}
                    </td>
                    {crossConnections && (
                      <td className="hidden @[800px]:table-cell">
                        <button
                          title={`${s.connections.find((c) => c.id === r.ref.connectionId)?.name} / ${r.path}`}
                          className="flex h-8 max-w-full items-center gap-1 text-muted-foreground"
                          onClick={() =>
                            dispatch({
                              type: "directory-open",
                              id: r.file?.directoryId || r.ref.id,
                              focus: r.file?.id,
                            })
                          }
                        >
                          <FolderOpen size={12} />
                          <span className="truncate">
                            {
                              s.connections.find(
                                (c) => c.id === r.ref.connectionId,
                              )?.name
                            }{" "}
                            / {r.path} · {r.reason} ·{" "}
                            {r.file?.externalRevision ||
                              (r.file ? `v${r.file.version}` : "目录")}
                          </span>
                        </button>
                      </td>
                    )}
                    <td className="hidden overflow-hidden @[720px]:table-cell">
                      {r.file && <FileTags file={r.file} />}
                    </td>
                  </>
                )}
                <td>{actions(r)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
