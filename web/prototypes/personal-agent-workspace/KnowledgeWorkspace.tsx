import {
  Popover,
  PopoverTrigger,
  PopoverContent,
  PopoverTitle,
} from "@/components/ui/popover";
import { ViewToolbar, ListPagination } from "./CompactUI";
import { FileDetailsList, entriesForFiles } from "./FileDetailsList";
import { entryRef, bookmarkFor } from "./organization-model";
import { FileToolbar } from "./CloudToolbar";
import { PropertyTable } from "./UnifiedFind";
import { useEffect, useState } from "react";
import {
  BookOpen,
  FileText,
  Network,
  Paperclip,
  Plus,
  ArrowRight,
  Link2,
  Sparkles,
  Pencil,
  Trash2,
  ArrowLeft,
  Columns3,
} from "lucide-react";
import { Input } from "@/components/ui/input";
import {
  Btn,
  Choice,
  Empty,
  FileIcon,
  Heading,
  Pill,
  IconBtn,
  useDemo,
} from "./ui";
import { FileButton } from "./HoverPreview";
import { FileTags } from "./TagWorkspace";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "./Dialog";
import type { FileRecord } from "./model";
import {
  isMarkdown,
  linkedNotes,
  noteBody,
  parseNoteLinks,
  resolveNoteLink,
} from "./knowledge-model";
import { fileUnavailable, canEditFile } from "./v4-model";
import { fileRef } from "./preview-model";

export function KnowledgeHome() {
  const { go } = useDemo();
  return (
    <div className="mb-3 grid grid-cols-2 gap-1 border-b border-border pb-2 lg:grid-cols-4">
      {[
        {
          name: "继续写作",
          note: "Markdown 与草稿",
          icon: Pencil,
          page: "knowledge",
        },
        { name: "最近资料", note: "文档与附件", icon: BookOpen, page: "files" },
        {
          name: "常用 Skills",
          note: "检查后受控选用",
          icon: Sparkles,
          page: "resources",
        },
        {
          name: "待验收结果",
          note: "来源、引用与归档",
          icon: Paperclip,
          page: "inbox",
        },
      ].map((x) => (
        <Btn
          key={x.name}
          variant="ghost"
          className="!h-12 !justify-start !whitespace-normal !px-2 text-left"
          onClick={() =>
            go(x.page as "knowledge" | "files" | "resources" | "inbox")
          }
        >
          <x.icon size={18} className="shrink-0 text-primary" />
          <span className="min-w-0">
            <strong className="block text-sm font-medium">{x.name}</strong>
            <small className="text-muted-foreground">{x.note}</small>
          </span>
        </Btn>
      ))}
    </div>
  );
}
export function KnowledgeWorkspace() {
  const { state: s, dispatch, open, compact } = useDemo();
  const [viewOptions, setViewOptions] = useState(false);
  const [query, setQuery] = useState(""),
    [filter, setFilter] = useState("all"),
    [create, setCreate] = useState(false),
    [name, setName] = useState(""),
    [error, setError] = useState("");
  const [page, setPage] = useState(1);
  useEffect(() => {
    setPage(1);
    dispatch({ type: "select", ids: [] });
  }, [query, filter, s.space]);
  useEffect(() => {
    dispatch({ type: "k-seed" });
  }, [s.space]);
  const docs = s.files
    .filter(
      (f) =>
        f.space === s.space &&
        isMarkdown(f) &&
        !fileUnavailable(s, f) &&
        !f.inbox &&
        !f.folder.startsWith("Agent 资源"),
    )
    .filter((f) =>
      (f.name + " " + noteBody(s, f.id))
        .toLowerCase()
        .includes(query.toLowerCase()),
    )
    .filter(
      (f) =>
        filter === "all" ||
        (filter === "draft"
          ? s.knowledge.notes[f.id]?.dirty
          : !!bookmarkFor(s, entryRef(s, f.id)!)),
    )
    .sort((a, b) => (b.updatedAt || "").localeCompare(a.updatedAt || ""));
  const pages = Math.max(1, Math.ceil(docs.length / 50));
  const currentPage = Math.min(page, pages);
  const shown = docs.slice((currentPage - 1) * 50, currentPage * 50);
  const launch = (f: FileRecord, edit = false) => {
    if (edit)
      dispatch({
        type: "k-open",
        id: f.id,
        mode: s.knowledge.options.defaultMode,
      });
    open({
      type: "preview",
      id: f.id,
      edit,
      scope: { label: "知识库 · 当前文档", refs: shown.map((f) => fileRef(f)) },
    });
  };
  const viewControls = (
    <>
      {" "}
      <Choice
        label="文档筛选"
        value={filter}
        onChange={(value) => {
          setFilter(value);
          setViewOptions(false);
        }}
        className="!h-8 !min-h-0 !w-24 text-xs max-[767px]:!h-11"
        options={[
          { value: "all", label: "全部文档" },
          { value: "draft", label: "未保存草稿" },
          { value: "favorite", label: "已收藏" },
        ]}
      />
      <IconBtn
        label={s.organization.table ? "紧凑列表" : "属性表格"}
        onClick={() =>
          dispatch({
            type: "property-table",
            patch: { table: !s.organization.table },
          })
        }
      >
        <Columns3 size={16} />
      </IconBtn>
      <IconBtn
        label="关系图"
        disabled={!docs.length}
        onClick={() => dispatch({ type: "k-graph", id: docs[0].id })}
      >
        <Network size={16} />
      </IconBtn>
    </>
  );
  return (
    <>
      <div className="flex h-full min-h-0 flex-col" data-knowledge-list>
        <ViewToolbar
          title="知识库"
          onClear={() => setQuery("")}
          hasQuery={!!query}
          note="文档保留原目录；只查询已加载内容，不自动分析全盘。"
          search={
            <Input
              aria-label="搜索知识库"
              placeholder="搜索文档"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="!h-8 text-xs max-[767px]:!h-11"
            />
          }
        >
          <Btn
            aria-label="新建文档"
            disabled={s.actor !== "yeisme" || !s.online}
            onClick={() => {
              setCreate(true);
              setError("");
            }}
          >
            <Plus size={16} />
            <span className="hidden sm:inline">新建文档</span>
          </Btn>
        </ViewToolbar>
        <div className="shrink-0 px-2">
          <FileToolbar
            count={docs.length}
            shown={shown.map((f) => f.id)}
            matched={docs.map((f) => f.id)}
            page={currentPage}
            pages={pages}
            summary={`${docs.length} 个文档`}
            extra={
              <>
                {compact ? (
                  <Popover open={viewOptions} onOpenChange={setViewOptions}>
                    <PopoverTrigger
                      render={
                        <IconBtn label="文档视图">
                          <Columns3 size={16} />
                        </IconBtn>
                      }
                    />
                    <PopoverContent align="end">
                      <PopoverTitle>文档视图</PopoverTitle>
                      <div className="flex items-center gap-2">
                        {viewControls}
                      </div>
                    </PopoverContent>
                  </Popover>
                ) : (
                  viewControls
                )}
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
        </div>
        <div className="min-h-0 flex-1 overflow-auto px-2">
          {s.organization.table ? (
            <PropertyTable files={shown} />
          ) : (
            <FileDetailsList
              knowledge
              entries={entriesForFiles(s, shown)}
              label="知识库 · 当前文档"
              onOpen={(r) => r.file && launch(r.file)}
              onEdit={(r) => r.file && launch(r.file, true)}
              columns={[
                {
                  id: "location",
                  label: "所在位置",
                  width: 150,
                  at: 800,
                  render: (r) => (
                    <span
                      title={`${s.connections.find((c) => c.id === r.ref.connectionId)?.name} / ${r.path}`}
                    >
                      {r.path}
                    </span>
                  ),
                },
                {
                  id: "updated",
                  label: "修改时间",
                  width: 100,
                  at: 560,
                  render: (r) => r.file?.updatedAt || "未知",
                },
                {
                  id: "tags",
                  label: "标签",
                  width: 140,
                  at: 720,
                  render: (r) => r.file && <FileTags file={r.file} />,
                },
                {
                  id: "links",
                  label: "引用",
                  width: 54,
                  at: 420,
                  render: (r) =>
                    r.file && (
                      <span className="tabular-nums">
                        {linkedNotes(s, r.file).length}
                      </span>
                    ),
                },
              ]}
            />
          )}
        </div>
      </div>
      <Dialog open={create} onOpenChange={setCreate}>
        <DialogContent>
          <DialogTitle>新建 Markdown 文档</DialogTitle>
          <DialogDescription>
            当前空间 / 知识库。内容仅保存于本轮演示。
          </DialogDescription>
          <label className="my-4 block text-sm">
            文档名称
            <Input
              className="mt-2"
              aria-label="新文档名称"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="例如：研究记录"
            />
          </label>
          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          )}
          <Btn
            onClick={() => {
              const title = name.trim().replace(/\.md$/i, "") + ".md";
              if (
                !name.trim() ||
                /[\\/]/.test(name) ||
                s.files.some(
                  (f) =>
                    f.space === s.space &&
                    !f.trashed &&
                    f.name === title &&
                    f.folder === "知识库",
                )
              ) {
                setError("名称无效或同名文档已存在。");
                return;
              }
              const id = `note-${s.space}-${s.knowledge.counter}`;
              dispatch({ type: "k-new", name });
              setCreate(false);
              setName("");
              open({ type: "preview", id, single: true, edit: true });
            }}
          >
            创建并编辑
          </Btn>
        </DialogContent>
      </Dialog>
    </>
  );
}
export function NoteRelations({
  file: f,
  version,
}: {
  file: FileRecord;
  version: number;
}) {
  const { state: s, dispatch, open, go } = useDemo();
  const [rename, setRename] = useState(false),
    [name, setName] = useState(f.name),
    [folder, setFolder] = useState(f.folder);
  const links = parseNoteLinks(noteBody(s, f.id, version));
  const incoming = linkedNotes(s, f);
  const mentions = s.files.filter(
    (x) =>
      x.id !== f.id &&
      x.space === s.space &&
      isMarkdown(x) &&
      !fileUnavailable(s, x) &&
      noteBody(s, x.id).includes(f.name.replace(/\.md$/i, "")) &&
      !incoming.some((r) => r.file.id === x.id),
  );
  const reference = (x: FileRecord, label?: string) => (
    <FileButton
      reference={fileRef(x)}
      onOpen={() =>
        open({ type: "preview", id: x.id, version: x.version, single: true })
      }
      variant="ghost"
      className="!h-auto !min-h-11 !w-full !justify-start !whitespace-normal !px-0 text-left text-xs text-primary"
    >
      {label || x.name} · v{x.version}
    </FileButton>
  );
  return (
    <div className="space-y-5 text-xs">
      <div className="flex flex-wrap gap-2">
        <Btn
          variant="outline"
          onClick={() => dispatch({ type: "k-graph", id: f.id })}
        >
          <Network size={14} />
          关系图
        </Btn>
        <Btn
          variant="outline"
          disabled={!canEditFile(s, f)}
          onClick={() => {
            setName(f.name);
            setFolder(f.folder);
            setRename(true);
          }}
        >
          重命名与移动
        </Btn>
      </div>
      <p className="leading-6 text-muted-foreground">
        当前空间已加载范围 · 2026-09-28 更新 · 出链来自 v{version}
        ，目标默认跟随最新。关联不会增加授权。
      </p>
      {isMarkdown(f) && (
        <section>
          <h3 className="border-b border-border pb-2 font-semibold">
            出链 · {links.filter((l) => !l.embed).length}
          </h3>
          {links
            .filter((l) => !l.embed)
            .map((l, i) => {
              const r = resolveNoteLink(s, f, l);
              return (
                <div key={i} className="py-1">
                  {r.file && r.status === "可访问" ? (
                    reference(r.file, l.label)
                  ) : (
                    <p className="py-3 text-muted-foreground">
                      {l.label} · {r.status}
                    </p>
                  )}
                </div>
              );
            })}
          {!links.filter((l) => !l.embed).length && (
            <p className="py-3 text-muted-foreground">当前版本没有文档链接。</p>
          )}
        </section>
      )}
      <section>
        <h3 className="border-b border-border pb-2 font-semibold">
          {isMarkdown(f) ? "反链" : "被文档引用"} · {incoming.length}
        </h3>
        {incoming.map((x, i) => (
          <div key={i} className="border-b border-border py-2">
            {reference(x.file)}
            <p className="line-clamp-3 whitespace-pre-wrap leading-6 text-muted-foreground">
              {x.snippet}
            </p>
          </div>
        ))}
        {!incoming.length && (
          <p className="py-3 text-muted-foreground">
            当前已扫描范围暂无引用，不能据此判断可安全删除。
          </p>
        )}
      </section>
      <section>
        <h3 className="border-b border-border pb-2 font-semibold">
          未关联提及 · {mentions.length}
        </h3>
        {mentions.map((x) => (
          <div key={x.id}>
            {reference(x)}
            <Btn
              variant="outline"
              onClick={() =>
                open({
                  type: "danger",
                  title: "建立这处双链？",
                  text: `将在 ${x.name} 中把首次文本提及替换为文档链接，生成新版本。`,
                  action: { type: "k-link-mention", id: x.id, target: f.id },
                })
              }
            >
              确认建立链接
            </Btn>
          </div>
        ))}
        {!mentions.length && (
          <p className="py-3 text-muted-foreground">没有待确认的文本提及。</p>
        )}
      </section>
      <section>
        <h3 className="border-b border-border pb-2 font-semibold">
          附件 · {links.filter((l) => l.embed).length}
        </h3>
        {links
          .filter((l) => l.embed)
          .map((l, i) => {
            const r = resolveNoteLink(s, f, l);
            return (
              <div key={i} className="py-2">
                {r.file && r.status === "可访问" ? (
                  reference(r.file)
                ) : (
                  <p>
                    {l.label} · {r.status}
                  </p>
                )}
              </div>
            );
          })}
      </section>
      {!isMarkdown(f) && (
        <Btn
          variant="outline"
          onClick={() =>
            open({
              type: "danger",
              title: "检查附件删除影响",
              text: `当前已扫描范围有 ${incoming.length} 处引用。继续将创建回收站计划，其他范围可能仍有引用。`,
              action: {
                type: "plan",
                kind: "trash",
                ids: [f.id],
                target: "回收站",
              },
            })
          }
        >
          <Trash2 size={14} />
          预览删除影响
        </Btn>
      )}
      <Dialog open={rename} onOpenChange={setRename}>
        <DialogContent>
          <DialogTitle>重命名与移动</DialogTitle>
          <DialogDescription>
            先检查引用影响；有未保存草稿或版本变化时停止修改。
          </DialogDescription>
          <label className="my-3 block">
            名称
            <Input
              aria-label="文档新名称"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          <label className="my-3 block">
            目标目录
            <Input
              aria-label="文档目标目录"
              value={folder}
              onChange={(e) => setFolder(e.target.value)}
            />
          </label>
          <Btn
            onClick={() =>
              dispatch({ type: "k-rename-plan", id: f.id, name, folder })
            }
          >
            预览引用影响
          </Btn>
          {s.knowledge.renamePlan?.id === f.id && (
            <div className="mt-4 space-y-3 border-t border-border pt-4">
              <p>
                名称：{f.name} → {s.knowledge.renamePlan.name}
              </p>
              <p>
                目录：{f.folder} → {s.knowledge.renamePlan.folder}
              </p>
              <p>{s.knowledge.renamePlan.refs.length} 份文档需要更新引用</p>
              {s.knowledge.renamePlan.refs.map((r) => (
                <p key={r.id}>
                  {s.files.find((x) => x.id === r.id)?.name} · 固定 v{r.version}
                </p>
              ))}
              <Btn onClick={() => dispatch({ type: "k-rename-apply" })}>
                确认更新文件与引用
              </Btn>
            </div>
          )}
          <p role="status" className="mt-3 text-xs text-primary">
            {s.notice}
          </p>
        </DialogContent>
      </Dialog>
    </div>
  );
}
export function KnowledgeGraph() {
  const { state: s, dispatch, open, go } = useDemo();
  const [view, setView] = useState("graph");
  const f = s.files.find(
    (f) => f.id === s.knowledge.graphFile && !fileUnavailable(s, f),
  );
  if (!f)
    return (
      <Empty
        title="选择一份文档查看关系"
        text="图谱只显示当前空间有权访问的已解析关系。"
        action="返回知识库"
        onAction={() => go("knowledge")}
      />
    );
  const kind = s.knowledge.graphKind;
  const links = parseNoteLinks(noteBody(s, f.id))
    .map((l) => ({ link: l, target: resolveNoteLink(s, f, l).file }))
    .filter(
      (x) => x.target && (kind === "attachment" ? x.link.embed : !x.link.embed),
    );
  const incoming =
    kind === "document"
      ? linkedNotes(s, f).map((x) => ({ link: x.link, target: x.file }))
      : [];
  const targets = [
    ...new Map(
      [...links, ...incoming]
        .filter((x) => x.target && !fileUnavailable(s, x.target))
        .map((x) => [x.target!.id, x.target!]),
    ).values(),
  ];
  const tags = s.tags.filter((t) => f.tags?.some((a) => a.tagId === t.id));
  return (
    <>
      <Heading
        title="知识关系"
        note={`${f.name} · 一跳关系 · 当前空间授权范围 · 2026-09-28 更新`}
        children={
          <Btn variant="outline" onClick={() => go("knowledge")}>
            <ArrowLeft size={15} />
            知识库
          </Btn>
        }
      />
      <div className="mb-5 flex flex-wrap gap-2">
        {(["document", "attachment", "tag"] as const).map((k) => (
          <Btn
            key={k}
            variant={kind === k ? "secondary" : "ghost"}
            onClick={() => dispatch({ type: "k-graph", id: f.id, kind: k })}
          >
            {k === "document"
              ? "文档关系"
              : k === "attachment"
                ? "附件关系"
                : "标签关系"}
          </Btn>
        ))}
        <Btn
          variant="outline"
          onClick={() => setView(view === "graph" ? "list" : "graph")}
        >
          {view === "graph" ? "切换列表" : "切换关系图"}
        </Btn>
      </div>
      <div className="min-h-72 rounded-xl border border-border bg-card p-5">
        {view === "graph" && (
          <div className="mb-8 flex justify-center">
            <div className="max-w-full rounded-lg border border-primary bg-accent px-5 py-4 text-center">
              <BookOpen className="mx-auto mb-2 text-primary" size={20} />
              <strong className="break-words text-sm">{f.name}</strong>
              <small className="mt-1 block text-muted-foreground">
                当前文档 · v{f.version}
              </small>
            </div>
          </div>
        )}
        <div
          className={
            view === "graph"
              ? "grid gap-6 border-t border-primary/25 pt-6 sm:grid-cols-2 lg:grid-cols-3"
              : "divide-y divide-border"
          }
        >
          {kind === "tag"
            ? tags.map((t) => (
                <Btn key={t.id} variant="outline" onClick={() => go("tags")}>
                  {t.name}
                </Btn>
              ))
            : targets.map((x) => (
                <FileButton
                  key={x.id}
                  reference={fileRef(x)}
                  onOpen={() =>
                    open({ type: "preview", id: x.id, single: true })
                  }
                  variant="outline"
                  className={
                    view === "graph"
                      ? "relative !h-auto !min-h-20 !justify-start !whitespace-normal !p-4 text-left before:absolute before:-top-6 before:left-1/2 before:h-6 before:border-l before:border-primary/25"
                      : "!h-auto !min-h-20 !justify-start !whitespace-normal !p-4 text-left"
                  }
                >
                  <Link2 size={16} className="shrink-0 text-primary" />
                  <span className="min-w-0">
                    <strong className="block break-words text-sm">
                      {x.name}
                    </strong>
                    <small className="text-muted-foreground">
                      {incoming.some((i) => i.target?.id === x.id)
                        ? "引用当前文档"
                        : "当前文档引用"}{" "}
                      · v{x.version}
                    </small>
                  </span>
                </FileButton>
              ))}
        </div>
        {!(kind === "tag" ? tags.length : targets.length) && (
          <p className="py-8 text-center text-sm text-muted-foreground">
            当前已解析范围没有此类关系。
          </p>
        )}
      </div>
      <p className="mt-4 text-xs leading-6 text-muted-foreground">
        图谱是有范围的演示投影。未解析、未授权的目录不参与计数；关系不代表读取或编辑授权。
      </p>
    </>
  );
}
