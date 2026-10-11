import { FileDetailsList, entriesForFiles } from "./FileDetailsList";
import { ListPagination } from "./CompactUI";
import { CloudExplorer } from "./CloudExplorer";

import { FileButton } from "./HoverPreview";
import { fileRef } from "./preview-model";

import { contextUnavailable, fileUnavailable } from "./v4-model";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "./Dialog";
import { FileTags, TagPicker } from "./TagWorkspace";
import { FileToolbar } from "./CloudToolbar";
import React, { useEffect, useRef, useState } from "react";
import {
  ArrowDownToLine,
  ArrowLeft,
  ArrowRight,
  ArrowUp,
  ArrowUpRight,
  Check,
  ChevronDown,
  ChevronRight,
  Columns2,
  Folder,
  FolderPlus,
  Grid2X2,
  Image,
  Inbox,
  List,
  MoreHorizontal,
  Pencil,
  Plus,
  Search,
  SlidersHorizontal,
  Sparkles,
  Star,
  Tag,
  X,
} from "lucide-react";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Checkbox } from "@/components/ui/checkbox";
import { cn } from "@/lib/utils";
import type { FileRecord, State } from "./model";
import {
  collectionFiles,
  currentConversation,
  emptyFilter,
  matchesFilter,
} from "./model";
import {
  Btn,
  Choice,
  Empty,
  FileArt,
  FileIcon,
  Heading,
  IconBtn,
  Origin,
  panel,
  Pill,
  SectionTitle,
  useDemo,
} from "./ui";

export function Home() {
  const { state: s, dispatch, open, go, compact } = useDemo();
  const files = s.files.filter(
    (f) =>
      f.space === s.space && !f.trashed && !f.inbox && !fileUnavailable(s, f),
  );
  const featured = [
    files.find((f) => f.kind === "image"),
    files.find((f) => f.kind === "video"),
    files.find((f) => f.kind === "document"),
  ].filter((f): f is FileRecord => !!f);
  const collections = s.collections.filter((c) => c.space === s.space);
  const pending = s.tasks.filter(
    (t) =>
      t.space === s.space &&
      ["approval", "partial", "stale", "unknown"].includes(t.status),
  );
  return (
    <>
      <Heading title="素材，接着创作。" note="你的参考、工作稿与新灵感。">
        <Btn onClick={() => open({ type: "upload" })}>
          <Plus size={16} />
          添加素材
        </Btn>
      </Heading>
      <div
        className={cn(
          "mb-4 grid gap-3",
          compact
            ? "grid-cols-1"
            : "grid-cols-[minmax(0,1.5fr)_minmax(220px,1fr)]",
        )}
      >
        <section className="overflow-hidden rounded-2xl border border-border bg-card">
          <div className="flex items-center justify-between gap-3 px-3 py-2">
            <div>
              <p className="text-xs text-muted-foreground">继续上次的项目</p>
              <h2 className="mt-1 text-lg font-semibold">
                {collections[0]?.name || "创建你的第一个素材集"}
              </h2>
            </div>
            <Btn
              variant="outline"
              onClick={() =>
                collections[0]
                  ? dispatch({ type: "collection-open", id: collections[0].id })
                  : open({ type: "collection" })
              }
            >
              打开集合
              <ArrowUpRight size={15} />
            </Btn>
          </div>
          <div className="grid max-h-32 grid-cols-3 gap-1 overflow-hidden px-1 pb-1">
            {featured.map((f) => (
              <FileButton
                reference={fileRef(f)}
                key={f.id}
                layout="surface"
                className="!block !h-auto !min-w-0 overflow-hidden !p-0"
                aria-label={`预览 ${f.name}`}
                onOpen={() =>
                  open({
                    type: "preview",
                    id: f.id,
                    scope: {
                      label: "首页精选",
                      refs: featured.map((f) => fileRef(f)),
                    },
                  })
                }
              >
                <FileArt file={f} />
              </FileButton>
            ))}
          </div>
        </section>
        <section
          className={cn(
            "flex flex-col justify-between rounded-2xl bg-card",
            compact ? "p-3" : "p-5",
          )}
        >
          <div>
            <div className="mb-3 flex items-center justify-between">
              <h2 className="font-semibold">待你处理</h2>
              <Pill tone="amber">{pending.length} 项</Pill>
            </div>
            <Btn
              layout="surface"
              className="!h-auto !w-full !justify-start !whitespace-normal !px-0 !py-3 text-left"
              onClick={() => go("tasks")}
            >
              <span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-[var(--warning-soft)] text-[var(--warning)]">
                <Sparkles size={18} />
              </span>
              <span className="min-w-0 flex-1">
                <span className="block text-sm font-medium">
                  {pending[0]?.title || "没有待确认的操作"}
                </span>
                <span
                  className={cn(
                    "mt-1 block text-xs text-muted-foreground",
                    compact && "hidden",
                  )}
                >
                  {pending.length
                    ? "查看修改前后，再决定执行"
                    : "可以放心回到创作中"}
                </span>
              </span>
              <ChevronRight size={15} />
            </Btn>
            <Btn
              layout="surface"
              className="!h-auto !w-full !justify-start !whitespace-normal !px-0 !py-3 text-left"
              onClick={() => go("inbox")}
            >
              <span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-accent text-primary">
                <Inbox size={18} />
              </span>
              <span className="flex-1">
                <span className="block text-sm font-medium">
                  {
                    s.files.filter(
                      (f) => f.space === s.space && f.inbox && !f.trashed,
                    ).length
                  }{" "}
                  份新素材待归档
                </span>
                <span
                  className={cn(
                    "mt-1 block text-xs text-muted-foreground",
                    compact && "hidden",
                  )}
                >
                  手机采集与 Agent 产物
                </span>
              </span>
              <ChevronRight size={15} />
            </Btn>
          </div>
          {!compact && (
            <Btn
              variant="link"
              className="mt-2 !justify-start !px-0"
              onClick={() => go("search")}
            >
              检索并整理资料
              <ArrowRight size={14} />
            </Btn>
          )}
        </section>
      </div>
      <SectionTitle
        title="最近的素材"
        action="查看全部"
        onAction={() => {
          dispatch({ type: "filter", patch: emptyFilter() });
          dispatch({ type: "folder", path: "全部文件" });
        }}
      />
      <div className="mb-4">
        <FileDetailsList
          selectable={false}
          entries={entriesForFiles(s, files.slice(0, 6))}
          label="首页最近文件"
        />
      </div>
      <SectionTitle
        title="常用目录"
        action="管理存储"
        onAction={() => go("connections")}
      />
      <div
        className={cn("grid gap-3", compact ? "grid-cols-1" : "grid-cols-3")}
      >
        {s.folders
          .filter(
            (f) =>
              f.space === s.space &&
              ["山野计划", "我的资料", "生活记录", "共同创作"].includes(f.path),
          )
          .map((f) => (
            <Btn
              key={f.path}
              layout="surface"
              className="!h-auto !justify-start rounded-xl bg-card !p-4 text-left"
              onClick={() => {
                dispatch({ type: "filter", patch: emptyFilter() });
                dispatch({ type: "folder", path: f.path });
              }}
            >
              <Folder size={22} className="text-primary" />
              <span className="min-w-0 flex-1">
                <span className="block font-medium">{f.path}</span>
                <span className="mt-1 block text-xs text-muted-foreground">
                  {files.filter((x) => x.folder === f.path).length} 份文件
                </span>
              </span>
              <ChevronRight size={14} />
            </Btn>
          ))}
      </div>
    </>
  );
}

export function Files({ trash = false }: { trash?: boolean }) {
  return trash ? <TrashFiles /> : <CloudExplorer />;
}
function TrashFiles() {
  const { state: s, dispatch, open } = useDemo();
  const files = s.files.filter((f) => f.space === s.space && f.trashed);
  return (
    <>
      <Heading
        title="回收站"
        note="恢复文件与历史版本；永久删除不属于自主操作。"
      />
      <div className="overflow-hidden rounded-xl border bg-card">
        {files.map((f) => (
          <AssetCard
            key={f.id}
            file={f}
            list
            trash
            selected={s.selected.includes(f.id)}
            onSelect={() => dispatch({ type: "select", ids: [f.id] })}
            onOpen={() => open({ type: "preview", id: f.id })}
          />
        ))}
        {!files.length && (
          <Empty title="回收站是空的" text="移入回收站的文件会显示在这里。" />
        )}
      </div>
    </>
  );
}

export function AssetCard({
  file: f,
  list = false,
  selected = false,
  onSelect,
  onOpen,
  trash = false,
  compactCard = false,
  previewRef,
}: {
  file: FileRecord;
  list?: boolean;
  selected?: boolean;
  onSelect?: (range?: boolean) => void;
  onOpen: () => void;
  trash?: boolean;
  compactCard?: boolean;
  previewRef?: import("./preview-model").PreviewRef;
}) {
  const { state: s, dispatch, open, compact } = useDemo();
  const reference = previewRef || fileRef(f);
  const hold = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const long = useRef(false);
  return (
    <article
      data-file-id={f.id}
      className={cn(
        "group/asset relative min-w-0",
        list
          ? "flex items-center gap-3 border-b border-border p-3 last:border-0"
          : "overflow-hidden rounded-xl border border-border/80 bg-card",
        selected && "ring-2 ring-primary ring-inset bg-accent/40",
      )}
      onPointerDown={(e) => {
        if (e.pointerType === "touch" && onSelect) {
          long.current = false;
          hold.current = setTimeout(() => {
            long.current = true;
            onSelect();
          }, 550);
        }
      }}
      onPointerUp={() => clearTimeout(hold.current)}
      onPointerCancel={() => clearTimeout(hold.current)}
      onPointerMove={() => clearTimeout(hold.current)}
    >
      {!list && (
        <FileButton
          reference={reference}
          layout="surface"
          className="!block !h-auto !w-full !p-0"
          aria-label={`预览 ${f.name}`}
          onOpen={() => !long.current && onOpen()}
        >
          <FileArt file={f} />
        </FileButton>
      )}
      {onSelect && !trash && (
        <label
          className={cn(
            "flex min-h-[44px] min-w-[44px] items-center justify-center",
            !list && "absolute left-2 top-2 z-10 rounded-lg bg-card/95",
          )}
        >
          <Checkbox
            aria-label={`选择 ${f.name}`}
            checked={selected}
            onCheckedChange={(_, details) =>
              !long.current &&
              onSelect(!!(details.event as MouseEvent).shiftKey)
            }
          />
        </label>
      )}
      {list && (
        <FileButton
          reference={reference}
          variant="ghost"
          className="!size-11 !shrink-0 !p-0"
          aria-label={`打开 ${f.name}`}
          onOpen={onOpen}
        >
          <FileIcon file={f} />
        </FileButton>
      )}
      <div className={cn("min-w-0 flex-1", !list && "p-3")}>
        <FileButton
          reference={reference}
          layout="surface"
          className={cn(
            "!block !h-auto !w-full !p-0 text-left",
            compact ? "!min-h-11" : "!min-h-0",
          )}
          onOpen={() => !long.current && onOpen()}
          title={f.name}
          aria-label={list ? `预览 ${f.name}` : undefined}
        >
          <span
            className={cn(
              "block text-sm font-medium leading-6",
              list && !compact ? "truncate" : "line-clamp-2 min-h-12",
            )}
          >
            {f.name}
          </span>
        </FileButton>
        <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1">
          <span className="text-xs text-muted-foreground">
            {f.metadata || f.size}
          </span>
          {list && <Origin file={f} />}
          <span className="ml-auto text-xs text-muted-foreground">
            {f.externalRevision ? "外部修订" : `v${f.version}`}
          </span>
        </div>

        <p className="mt-1 h-4 truncate text-[10px] text-muted-foreground">
          {s.connections.find((c) => c.id === (f.connectionId || "drive"))
            ?.name || "来源未知"}
        </p>
        <div className="mt-1">
          <FileTags file={f} />
        </div>
      </div>
      {!compactCard && !trash && (
        <div
          className={cn(
            "flex",
            list
              ? "shrink-0"
              : "justify-between border-t border-border/60 px-2 py-1",
          )}
        >
          <IconBtn
            label={`${f.favorite ? "取消收藏" : "收藏"} ${f.name}`}
            aria-pressed={f.favorite}
            onClick={() => dispatch({ type: "favorite", id: f.id })}
          >
            <Star
              size={16}
              className={
                f.favorite
                  ? "fill-primary/20 text-primary"
                  : "text-muted-foreground"
              }
            />
          </IconBtn>
          {!list && (
            <Btn
              variant="ghost"
              className="text-xs"
              aria-pressed={!!f.picked}
              onClick={() => dispatch({ type: "pick", id: f.id })}
            >
              <Check size={14} />
              {f.picked ? "已选片" : "选片"}
            </Btn>
          )}
        </div>
      )}
      {trash && (
        <Btn
          variant="outline"
          onClick={() =>
            open({
              type: "danger",
              title: "恢复这个文件？",
              text: "恢复到原位置，历史版本保持不变。",
              action: { type: "restore-version", id: f.id, version: f.version },
            })
          }
        >
          恢复
        </Btn>
      )}
    </article>
  );
}

export function Collections() {
  const { state: s, dispatch, open, compact } = useDemo();
  const active = s.collections.find(
    (c) => c.id === s.browser.collectionId && c.space === s.space,
  );
  const [group, setGroup] = useState("全部");
  const [gallery, setGallery] = useState(false);
  const [page, setPage] = useState(1);
  if (!active)
    return (
      <>
        <Heading title="项目集合" note="把有关的素材放在一起，原文件留在原处。">
          <Btn onClick={() => open({ type: "collection" })}>
            <Plus size={15} />
            新建集合
          </Btn>
          <Btn variant="ghost" onClick={() => setGallery(!gallery)}>
            {gallery ? "紧凑列表" : "封面预览"}
          </Btn>
        </Heading>
        <div
          className={
            gallery
              ? cn("grid gap-3", compact ? "grid-cols-1" : "grid-cols-2")
              : "divide-y divide-border/50"
          }
        >
          {s.collections
            .filter((c) => c.space === s.space)
            .map((c) => {
              const files = collectionFiles(s, c);
              return (
                <Btn
                  key={c.id}
                  layout="surface"
                  className={cn(
                    gallery ? panel : "!rounded-none",
                    "!block !h-auto !w-full overflow-hidden !p-0 text-left",
                  )}
                  onClick={() =>
                    dispatch({ type: "collection-open", id: c.id })
                  }
                >
                  <div
                    className={
                      gallery
                        ? "grid max-h-36 grid-cols-3 gap-1 overflow-hidden bg-muted"
                        : "hidden"
                    }
                  >
                    {files.slice(0, 3).map((f) => (
                      <FileArt key={f.id} file={f} />
                    ))}
                  </div>
                  <div
                    className={
                      gallery
                        ? "flex items-center justify-between p-3"
                        : "flex h-12 items-center justify-between gap-3 px-2"
                    }
                  >
                    <div>
                      <h2 className="text-[13px] font-medium">{c.name}</h2>
                      <p className="text-[11px] text-muted-foreground">
                        {files.length} 份素材 ·{" "}
                        {c.query ? "智能集合" : "手动集合"}
                      </p>
                    </div>
                    <ArrowUpRight size={19} />
                  </div>
                </Btn>
              );
            })}
        </div>
      </>
    );
  const files = collectionFiles(s, active).filter(
    (f) => group === "全部" || (active.groups[f.id] || "未分组") === group,
  );
  const pages = Math.max(1, Math.ceil(files.length / 50)),
    currentPage = Math.min(page, pages),
    shown = files.slice((currentPage - 1) * 50, currentPage * 50);
  return (
    <>
      <Btn
        variant="ghost"
        className="mb-3 !px-0"
        onClick={() =>
          dispatch({ type: "browser", patch: { collectionId: "" } })
        }
      >
        <ArrowLeft size={15} />
        全部集合
      </Btn>
      <Heading
        title={active.name}
        note={
          active.query
            ? "按保存的筛选动态呈现，每次查看都检查当前可见范围。"
            : "参考、工作稿和交付文件。集合内整理不改变原目录。"
        }
      >
        <Btn
          onClick={() =>
            open({ type: "collection", id: active.id, ids: s.selected })
          }
        >
          <Plus size={15} />
          添加已选素材
        </Btn>
      </Heading>
      <div className="mb-5 flex flex-wrap gap-1">
        {["全部", "参考", "工作稿", "交付", "未分组"].map((g) => (
          <Btn
            key={g}
            variant={group === g ? "secondary" : "ghost"}
            onClick={() => setGroup(g)}
          >
            {g}
          </Btn>
        ))}
      </div>
      <FileToolbar
        count={files.length}
        shown={shown.map((f) => f.id)}
        matched={files.map((f) => f.id)}
        page={currentPage}
        pages={pages}
        extra={
          <>
            <Btn variant="ghost" onClick={() => setGallery(!gallery)}>
              {gallery ? "紧凑列表" : "图库"}
            </Btn>
            {pages > 1 && (
              <ListPagination
                page={currentPage}
                pages={pages}
                onChange={setPage}
              />
            )}
          </>
        }
      />
      {!gallery ? (
        <FileDetailsList
          entries={entriesForFiles(s, shown)}
          label={`集合 · ${active.name} · ${group}`}
          columns={[
            {
              id: "group",
              label: "分组",
              width: 120,
              at: 420,
              render: (r) => (
                <Choice
                  label={`分组 ${r.name}`}
                  className="!h-7 !min-h-0 !w-28 text-xs max-[767px]:!h-11"
                  value={active.groups[r.ref.id] || "未分组"}
                  options={["未分组", "参考", "工作稿", "交付"].map((v) => ({
                    value: v,
                    label: v,
                  }))}
                  onChange={(g) =>
                    dispatch({
                      type: "collection-group",
                      id: active.id,
                      fileId: r.ref.id,
                      group: g,
                    })
                  }
                />
              ),
            },
            {
              id: "size",
              label: "大小",
              width: 80,
              at: 560,
              render: (r) => r.file?.size,
            },
            {
              id: "tags",
              label: "标签",
              width: 144,
              at: 720,
              render: (r) => r.file && <FileTags file={r.file} />,
            },
          ]}
        />
      ) : (
        <>
          <div
            className={cn(
              "grid gap-4",
              compact
                ? "grid-cols-2"
                : "grid-cols-[repeat(auto-fill,minmax(220px,1fr))]",
            )}
          >
            {shown.map((f) => (
              <div key={f.id}>
                <AssetCard
                  file={f}
                  selected={s.selected.includes(f.id)}
                  onSelect={() =>
                    dispatch({
                      type: "select",
                      ids: s.selected.includes(f.id)
                        ? s.selected.filter((id) => id !== f.id)
                        : [...s.selected, f.id],
                    })
                  }
                  onOpen={() =>
                    open({
                      type: "preview",
                      id: f.id,
                      scope: {
                        label: `集合 · ${active.name} · ${group}`,
                        refs: shown.map((f) => fileRef(f)),
                      },
                    })
                  }
                />
                <Choice
                  label={`分组 ${f.name}`}
                  className="mt-2 w-full"
                  value={active.groups[f.id] || "未分组"}
                  onChange={(g) =>
                    dispatch({
                      type: "collection-group",
                      id: active.id,
                      fileId: f.id,
                      group: g,
                    })
                  }
                  options={["未分组", "参考", "工作稿", "交付"].map((v) => ({
                    value: v,
                    label: v,
                  }))}
                />
              </div>
            ))}
          </div>
        </>
      )}
      {!files.length && (
        <Empty title="这个分组还没有素材" text="从素材库选择文件后加入集合。" />
      )}
    </>
  );
}
