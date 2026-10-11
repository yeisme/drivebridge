import { FileToolbar } from "./CloudToolbar";
import { ViewToolbar, ListPagination } from "./CompactUI";
import { FileDetailsList, entriesForFiles } from "./FileDetailsList";
import {
  Popover,
  PopoverTrigger,
  PopoverContent,
  PopoverTitle,
} from "@/components/ui/popover";
import { useEffect, useState } from "react";
import {
  BookOpen,
  FileCode2,
  Layers,
  SlidersHorizontal,
  ArrowDownUp,
  Plus,
  Sparkles,
  X,
  ArrowRight,
  CheckCircle2,
} from "lucide-react";
import { Input } from "@/components/ui/input";
import { Btn, Choice, Empty, Heading, IconBtn, Pill, useDemo } from "./ui";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "./Dialog";
import { FileButton } from "./HoverPreview";
import { agentContext, fileRef } from "./preview-model";
import { fileUnavailable, canEditFile, contextRefs } from "./v4-model";
import { collectionFiles } from "./model";
import { MarkdownBody } from "./MarkdownBody";
import { noteBody } from "./knowledge-model";
import { FileTags } from "./TagWorkspace";
export function ResourceWorkspace() {
  const { state: s, dispatch, open } = useDemo();
  const [query, setQuery] = useState(""),
    [kind, setKind] = useState("all"),
    [status, setStatus] = useState("all"),
    [tag, setTag] = useState("all"),
    [source, setSource] = useState("all"),
    [updated, setUpdated] = useState("all"),
    [sort, setSort] = useState("recent");
  const [page, setPage] = useState(1);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [draft, setDraft] = useState({
    kind: "all",
    status: "all",
    tag: "all",
    source: "all",
    updated: "all",
  });
  useEffect(() => {
    setPage(1);
    dispatch({ type: "select", ids: [] });
  }, [query, kind, status, tag, source, updated, s.space]);
  const [detail, setDetail] = useState(""),
    [tab, setTab] = useState("description");
  const [fields, setFields] = useState(false),
    [name, setName] = useState(""),
    [description, setDescription] = useState(""),
    [tools, setTools] = useState("");
  const [manualCopy, setManualCopy] = useState("");
  const copyRefs = async (ids: string[]) => {
    const files = ids
      .map((id) => s.files.find((f) => f.id === id))
      .filter((f): f is NonNullable<typeof f> => !!f);
    if (files.some((f) => fileUnavailable(s, f))) {
      dispatch({ type: "notice", text: "部分资源已不可用，请重新选择。" });
      return;
    }
    if (files.some((f) => s.knowledge.notes[f.id]?.dirty)) {
      files
        .filter((f) => s.knowledge.notes[f.id]?.dirty)
        .forEach((f) => dispatch({ type: "k-save", id: f.id }));
      dispatch({ type: "notice", text: "先保存草稿；成功后再复制固定引用。" });
      return;
    }
    const text = files.map((f) => agentContext(s, fileRef(f))).join("\n\n");
    try {
      await navigator.clipboard.writeText(text);
      dispatch({
        type: "notice",
        text: "已复制外部应用上下文；演示固定引用，不创建分享。",
      });
    } catch {
      setDetail("");
      setManualCopy(text);
    }
  };
  const [creating, setCreating] = useState(false),
    [newKind, setNewKind] = useState("skill"),
    [newName, setNewName] = useState("");
  useEffect(() => {
    dispatch({ type: "k-seed" });
  }, [s.space]);
  const resource = s.knowledge.resources.find((r) => r.fileId === detail);
  const file = s.files.find((f) => f.id === detail && !fileUnavailable(s, f));
  const stateOf = (id: string) => {
    const r = s.knowledge.resources.find((r) => r.fileId === id)!;
    const f = s.files.find((f) => f.id === id)!;
    return r.missing.length
      ? "缺少依赖"
      : s.knowledge.notes[id]?.dirty
        ? "未保存草稿"
        : r.checkedVersion === f.version
          ? "检查通过"
          : "已存储 · 待检查";
  };
  const resources = s.knowledge.resources
    .filter((r) => {
      const f = s.files.find((f) => f.id === r.fileId);
      return (
        f &&
        f.space === s.space &&
        !fileUnavailable(s, f) &&
        (kind === "all" || kind === r.kind) &&
        (r.name + r.description + f.name)
          .toLowerCase()
          .includes(query.toLowerCase()) &&
        (status === "all" ||
          (status === "ready"
            ? r.checkedVersion === f.version
            : r.checkedVersion !== f.version)) &&
        (tag === "all" || f.tags?.some((t) => t.tagId === tag)) &&
        (source === "all" || f.connectionId === source) &&
        (updated === "all" || f.updatedAt === "2026-09-28")
      );
    })
    .sort((a, b) =>
      sort === "name"
        ? a.name.localeCompare(b.name, "zh-CN")
        : (
            s.files.find((f) => f.id === b.fileId)?.updatedAt || ""
          ).localeCompare(
            s.files.find((f) => f.id === a.fileId)?.updatedAt || "",
          ),
    );
  const packs = s.collections.filter((c) => {
    const files = collectionFiles(s, c).filter((f) => !fileUnavailable(s, f));
    const ready =
      files.length > 0 &&
      files.every((f) => {
        const r = s.knowledge.resources.find((r) => r.fileId === f.id);
        return !r || r.kind !== "skill" || r.checkedVersion === f.version;
      });
    return (
      c.space === s.space &&
      c.name.includes("资料包") &&
      c.name.includes(query) &&
      files.length > 0 &&
      (status === "all" || (status === "ready" ? ready : !ready)) &&
      (tag === "all" ||
        files.some((f) => f.tags?.some((t) => t.tagId === tag))) &&
      (source === "all" || files.some((f) => f.connectionId === source)) &&
      (updated === "all" || files.some((f) => f.updatedAt === "2026-09-28"))
    );
  });
  const rows = [
    ...resources.map((r) => ({ resource: r, pack: null })),
    ...(["all", "pack"].includes(kind)
      ? packs.map((pack) => ({ resource: null, pack }))
      : []),
  ];
  const pages = Math.max(1, Math.ceil(rows.length / 50)),
    currentPage = Math.min(page, pages);
  const shown = rows.slice((currentPage - 1) * 50, currentPage * 50);
  const visibleResources = shown.flatMap((x) =>
      x.resource ? [x.resource] : [],
    ),
    visiblePacks = shown.flatMap((x) => (x.pack ? [x.pack] : []));
  const inspect = (id: string) => {
    setDetail(id);
    setTab("description");
  };
  return (
    <>
      <Dialog open={!!manualCopy} onOpenChange={(v) => !v && setManualCopy("")}>
        <DialogContent kind="confirm">
          <DialogTitle>剪贴板不可用，请手动复制</DialogTitle>
          <DialogDescription>仅含固定引用，不授予权限。</DialogDescription>
          <div
            aria-label="手动复制资源引用"
            tabIndex={0}
            className="max-h-60 overflow-auto whitespace-pre-wrap break-words select-text text-xs leading-6"
          >
            {manualCopy}
          </div>
        </DialogContent>
      </Dialog>
      <div className="flex h-full min-h-0 flex-col" data-resource-list>
        <ViewToolbar
          title="资源库"
          onClear={() => setQuery("")}
          hasQuery={!!query}
          note="存储不代表安装或执行。脚本仅供阅读，固定引用用于外部应用。"
          search={
            <Input
              aria-label="搜索资源"
              placeholder="搜索资源"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="!h-8 text-xs max-[767px]:!h-11"
            />
          }
        >
          <Btn
            aria-label="添加资源"
            disabled={s.actor !== "yeisme" || !s.online}
            onClick={() => setCreating(true)}
          >
            <Plus size={16} />
            <span className="hidden sm:inline">添加资源</span>
          </Btn>
        </ViewToolbar>
        <div className="shrink-0 px-2">
          <FileToolbar
            count={resources.length}
            summary={`${rows.length} 项资源`}
            shown={visibleResources.map((r) => r.fileId)}
            matched={resources.map((r) => r.fileId)}
            page={currentPage}
            pages={pages}
            extra={
              <>
                <Popover
                  open={filtersOpen}
                  onOpenChange={(value) => {
                    if (value) setDraft({ kind, status, tag, source, updated });
                    setFiltersOpen(value);
                  }}
                >
                  <PopoverTrigger
                    render={
                      <Btn variant="ghost" aria-label="筛选资源">
                        <SlidersHorizontal size={15} />
                        <span className="hidden sm:inline">筛选</span>
                        {[kind, status, tag, source, updated].filter(
                          (v) => v !== "all",
                        ).length || ""}
                      </Btn>
                    }
                  />
                  <PopoverContent
                    align="end"
                    className="!w-[420px] !max-w-[calc(100vw-16px)] !max-h-[calc(100dvh-100px)] overflow-auto !p-3"
                  >
                    <PopoverTitle>筛选资源</PopoverTitle>
                    <div className="grid grid-cols-2 gap-3">
                      <label className="space-y-1 text-xs">
                        类型
                        <Choice
                          label="资源类型"
                          value={draft.kind}
                          onChange={(v) => setDraft({ ...draft, kind: v })}
                          options={[
                            { value: "all", label: "全部类型" },
                            { value: "skill", label: "Skills" },
                            { value: "template", label: "提示词模板" },
                            { value: "pack", label: "资料包" },
                          ]}
                        />
                      </label>
                      <label className="space-y-1 text-xs">
                        状态
                        <Choice
                          label="资源状态"
                          value={draft.status}
                          onChange={(v) => setDraft({ ...draft, status: v })}
                          options={[
                            { value: "all", label: "全部状态" },
                            { value: "ready", label: "检查通过" },
                            { value: "pending", label: "待检查 / 缺少依赖" },
                          ]}
                        />
                      </label>
                      <label className="space-y-1 text-xs">
                        来源
                        <Choice
                          label="资源来源"
                          value={draft.source}
                          onChange={(v) => setDraft({ ...draft, source: v })}
                          options={[
                            { value: "all", label: "全部来源" },
                            ...s.connections
                              .filter((c) => c.space === s.space)
                              .map((c) => ({ value: c.id, label: c.name })),
                          ]}
                        />
                      </label>
                      <label className="space-y-1 text-xs">
                        更新时间
                        <Choice
                          label="资源更新时间"
                          value={draft.updated}
                          onChange={(v) => setDraft({ ...draft, updated: v })}
                          options={[
                            { value: "all", label: "全部时间" },
                            { value: "today", label: "本轮更新" },
                          ]}
                        />
                      </label>
                      <label className="col-span-2 space-y-1 text-xs">
                        标签
                        <Choice
                          label="资源标签"
                          value={draft.tag}
                          onChange={(v) => setDraft({ ...draft, tag: v })}
                          options={[
                            { value: "all", label: "全部标签" },
                            ...s.tags
                              .filter((t) => t.space === s.space)
                              .map((t) => ({ value: t.id, label: t.name })),
                          ]}
                        />
                      </label>
                    </div>
                    <div className="mt-2 flex justify-end gap-2 border-t pt-2">
                      <Btn
                        variant="ghost"
                        onClick={() =>
                          setDraft({
                            kind: "all",
                            status: "all",
                            tag: "all",
                            source: "all",
                            updated: "all",
                          })
                        }
                      >
                        重置
                      </Btn>
                      <Btn
                        variant="outline"
                        onClick={() => setFiltersOpen(false)}
                      >
                        取消
                      </Btn>
                      <Btn
                        onClick={() => {
                          setKind(draft.kind);
                          setStatus(draft.status);
                          setTag(draft.tag);
                          setSource(draft.source);
                          setUpdated(draft.updated);
                          setFiltersOpen(false);
                        }}
                      >
                        应用
                      </Btn>
                    </div>
                  </PopoverContent>
                </Popover>
                <Popover>
                  <PopoverTrigger
                    render={
                      <IconBtn label="资源排序">
                        <ArrowDownUp size={15} />
                      </IconBtn>
                    }
                  />
                  <PopoverContent align="end">
                    <PopoverTitle>资源排序</PopoverTitle>
                    <Choice
                      label="资源排序"
                      className="!w-28 !min-h-0 !h-8 max-[767px]:!h-11"
                      value={sort}
                      onChange={setSort}
                      options={[
                        { value: "recent", label: "最近更新" },
                        { value: "name", label: "名称" },
                      ]}
                    />
                  </PopoverContent>
                </Popover>
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
          {!!visibleResources.length && (
            <FileDetailsList
              label="资源库"
              entries={entriesForFiles(
                s,
                visibleResources.map(
                  (r) => s.files.find((f) => f.id === r.fileId)!,
                ),
              ).map((e) => ({
                ...e,
                name: s.knowledge.resources.find((r) => r.fileId === e.ref.id)!
                  .name,
              }))}
              onOpen={(r) => inspect(r.ref.id)}
              columns={[
                {
                  id: "kind",
                  label: "类型",
                  width: 100,
                  at: 560,
                  render: (r) =>
                    s.knowledge.resources.find((x) => x.fileId === r.ref.id)
                      ?.kind === "skill"
                      ? "Skill 资源包"
                      : "提示词模板",
                },
                {
                  id: "version",
                  label: "版本",
                  width: 48,
                  at: 720,
                  render: (r) => `v${r.file?.version}`,
                },
                {
                  id: "state",
                  label: "检查状态",
                  width: 136,
                  at: 420,
                  render: (r) => stateOf(r.ref.id),
                },
                {
                  id: "date",
                  label: "更新时间",
                  width: 100,
                  at: 800,
                  render: (r) => r.file?.updatedAt || "未知",
                },
              ]}
            />
          )}
          {visiblePacks.map((c) => (
            <div
              key={c.id}
              data-resource-pack
              className="flex h-9 items-center gap-2 border-b border-border/40 px-2 text-[13px] max-[767px]:h-12 [@media(pointer:coarse)]:h-12"
            >
              <Layers size={16} />
              <Btn
                variant="ghost"
                className="min-w-0 flex-1 !justify-start !px-0"
                onClick={() => {
                  dispatch({ type: "collection-open", id: c.id });
                }}
              >
                <span className="truncate">{c.name}</span>
              </Btn>
              <span className="text-xs text-muted-foreground">
                资料包 · {collectionFiles(s, c).length} 项
              </span>
              <Btn
                variant="ghost"
                className="text-xs"
                onClick={() =>
                  void copyRefs(
                    collectionFiles(s, c)
                      .filter((f) => !fileUnavailable(s, f))
                      .map((f) => f.id),
                  )
                }
              >
                复制固定上下文
              </Btn>
            </div>
          ))}
          {!rows.length && (
            <Empty
              title="没有匹配的资源"
              text="调整筛选，或添加一个示例资源包。"
            />
          )}
        </div>
      </div>
      <Dialog open={!!detail} onOpenChange={(v) => !v && setDetail("")}>
        <DialogContent kind="detail">
          <DialogTitle>{resource?.name || "资源不可用"}</DialogTitle>
          <DialogDescription>
            固定资源版本、声明信息与使用记录。存储与执行分别管理。
          </DialogDescription>
          {resource && file ? (
            <>
              <div className="my-3 flex gap-1 overflow-x-auto">
                {[
                  ["description", "说明"],
                  ["files", "文件"],
                  ["versions", "版本"],
                  ["usage", "使用记录"],
                ].map(([id, label]) => (
                  <Btn
                    key={id}
                    variant={tab === id ? "secondary" : "ghost"}
                    onClick={() => setTab(id)}
                  >
                    {label}
                  </Btn>
                ))}
              </div>
              <div className="max-h-[55dvh] overflow-auto pr-1">
                {tab === "description" && (
                  <div className="space-y-4">
                    <div className="grid gap-3 border-y border-border py-4 text-xs sm:grid-cols-2">
                      <p>作者：{resource.author}</p>
                      <p>来源：{resource.source}</p>
                      <p>
                        版本：v{file.version} · {stateOf(file.id)}
                      </p>
                      <p>最近检查：{resource.checkedAt || "尚未检查"}</p>
                      <p>
                        所需工具：
                        {resource.tools.join("、") || "无运行工具声明"}
                      </p>
                      <p>输入：{resource.inputs}</p>
                      <p>输出：{resource.outputs}</p>
                      <p>
                        未知字段保留：
                        {Object.keys(resource.extra).join("、") || "无"}
                      </p>
                    </div>
                    {resource.missing.length > 0 && (
                      <p role="status" className="text-sm text-destructive">
                        缺少依赖：{resource.missing.join("、")}
                        ，当前不可用于任务。
                      </p>
                    )}
                    <MarkdownBody file={file} body={noteBody(s, file.id)} />
                  </div>
                )}
                {tab === "files" && (
                  <div className="divide-y divide-border">
                    {resource.files.map((item) => {
                      const f = s.files.find((f) => f.id === item.fileId);
                      return (
                        <div key={item.path} className="py-2">
                          {f && !fileUnavailable(s, f) ? (
                            <FileButton
                              reference={fileRef(f)}
                              onOpen={() => {
                                setDetail("");
                                open({
                                  type: "preview",
                                  id: f.id,
                                  single: true,
                                });
                              }}
                              variant="ghost"
                              className="!h-12 !w-full !justify-start !whitespace-normal text-left"
                            >
                              {item.path} · v{f.version}
                            </FileButton>
                          ) : (
                            <p className="py-3 text-xs">
                              {item.path} · 文件不可用
                            </p>
                          )}
                        </div>
                      );
                    })}
                  </div>
                )}
                {tab === "versions" && (
                  <div>
                    {Array.from(
                      { length: file.version },
                      (_, i) => file.version - i,
                    ).map((v) => (
                      <Btn
                        key={v}
                        variant="ghost"
                        className="!h-12 !w-full !justify-start"
                        onClick={() => {
                          setDetail("");
                          open({
                            type: "preview",
                            id: file.id,
                            version: v,
                            single: true,
                          });
                        }}
                      >
                        v{v} ·{" "}
                        {v === file.version ? "当前版本" : "固定历史版本"}
                      </Btn>
                    ))}
                  </div>
                )}
                {tab === "usage" && (
                  <div>
                    {s.tasks
                      .filter(
                        (t) => s.knowledge.taskPlans[t.id]?.skillId === file.id,
                      )
                      .map((t) => (
                        <Btn
                          key={t.id}
                          variant="outline"
                          className="mb-2 !h-auto !min-h-14 !w-full !justify-start !whitespace-normal text-left"
                          onClick={() =>
                            open({
                              type: "confirm",
                              taskId: t.id,
                              revision: t.revision,
                              contextRevision: s.contextRevision,
                            })
                          }
                        >
                          {t.title} · {t.id} · Skill v
                          {s.knowledge.taskPlans[t.id].skillVersion} ·{" "}
                          {t.status}
                        </Btn>
                      ))}
                    {!s.tasks.some(
                      (t) => s.knowledge.taskPlans[t.id]?.skillId === file.id,
                    ) && (
                      <p className="py-10 text-center text-sm text-muted-foreground">
                        尚未使用，检查不会产生执行记录。
                      </p>
                    )}
                  </div>
                )}
              </div>
              <div className="mt-4 flex flex-wrap gap-2 border-t border-border pt-4">
                <Btn
                  variant="outline"
                  disabled={!canEditFile(s, file)}
                  onClick={() => {
                    setDetail("");
                    dispatch({ type: "k-open", id: file.id, mode: "live" });
                    open({
                      type: "preview",
                      id: file.id,
                      single: true,
                      edit: true,
                    });
                  }}
                >
                  编辑正文
                </Btn>
                <Btn
                  variant="outline"
                  disabled={!canEditFile(s, file)}
                  onClick={() => {
                    setName(resource.name);
                    setDescription(resource.description);
                    setTools(resource.tools.join(", "));
                    setFields(true);
                  }}
                >
                  修改字段
                </Btn>
                <Btn
                  variant="outline"
                  onClick={() => dispatch({ type: "k-check", id: file.id })}
                >
                  检查资源
                </Btn>
                {resource.kind === "skill" && (
                  <Btn
                    disabled={
                      resource.checkedVersion !== file.version ||
                      s.knowledge.notes[file.id]?.dirty
                    }
                    onClick={() => {
                      void copyRefs([file.id]);
                      setDetail("");
                    }}
                  >
                    复制已检查版本引用
                  </Btn>
                )}
              </div>
              <p role="status" className="mt-2 text-xs text-primary">
                {s.notice}
              </p>
            </>
          ) : (
            <p>权限或来源状态已变化，请重新选择。</p>
          )}
        </DialogContent>
      </Dialog>
      <Dialog open={fields} onOpenChange={setFields}>
        <DialogContent>
          <DialogTitle>修改资源字段</DialogTitle>
          <DialogDescription>
            保存将生成新版本并清除当前检查结果，未知字段保留。
          </DialogDescription>
          <label className="mt-4 block text-sm">
            名称
            <Input
              aria-label="资源名称"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          <label className="mt-4 block text-sm">
            说明
            <Input
              aria-label="资源说明"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </label>
          <label className="mt-4 block text-sm">
            所需工具（逗号分隔）
            <Input
              aria-label="所需工具"
              value={tools}
              onChange={(e) => setTools(e.target.value)}
            />
          </label>
          <div className="mt-5 flex gap-2">
            <Btn
              disabled={!name.trim() || !description.trim()}
              onClick={() => {
                dispatch({
                  type: "k-skill-fields",
                  id: detail,
                  name,
                  description,
                  tools: tools
                    .split(",")
                    .map((x) => x.trim())
                    .filter(Boolean),
                });
                setFields(false);
              }}
            >
              保存字段
            </Btn>
            <Btn variant="ghost" onClick={() => setFields(false)}>
              取消
            </Btn>
          </div>
        </DialogContent>
      </Dialog>
      <Dialog open={creating} onOpenChange={setCreating}>
        <DialogContent>
          <DialogTitle>添加资源</DialogTitle>
          <DialogDescription>
            创建当前空间的演示资源，保存后仍需检查。
          </DialogDescription>
          <label className="my-4 block text-sm">
            资源名称
            <Input
              aria-label="新资源名称"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
            />
          </label>
          <Choice
            label="新资源类型"
            value={newKind}
            onChange={setNewKind}
            options={[
              { value: "skill", label: "Skill 资源包" },
              { value: "template", label: "提示词模板" },
            ]}
          />
          <Btn
            className="mt-4"
            disabled={!newName.trim() || /[\\/]/.test(newName)}
            onClick={() => {
              dispatch({
                type: "k-resource-new",
                name: newName,
                kind: newKind as "skill" | "template",
              });
              setCreating(false);
              setNewName("");
            }}
          >
            创建资源
          </Btn>
        </DialogContent>
      </Dialog>
    </>
  );
}
export function ResourcePlan({ taskId }: { taskId: string }) {
  const { state: s, dispatch, open, close, go } = useDemo();
  const p = s.knowledge.taskPlans[taskId],
    t = s.tasks.find((t) => t.id === taskId);
  if (!p || !t)
    return (
      <p role="status">{s.notice || "未生成计划，请检查 Skill 与输入资料。"}</p>
    );
  const refs = [...p.refs, { id: p.skillId, version: p.skillVersion }];
  if (
    refs.some((r) =>
      fileUnavailable(
        s,
        s.files.find((f) => f.id === r.id),
        r.version,
      ),
    )
  )
    return <p role="status">引用或权限已失效，未展示计划正文。</p>;
  const target = s.files.find((f) => f.id === p.target);
  return (
    <div className="space-y-4">
      <p className="text-sm">
        {t.title} · {taskId} ·{" "}
        <Pill>
          {t.status === "approval"
            ? "待确认"
            : t.status === "done"
              ? "已完成"
              : "待重新检查"}
        </Pill>
      </p>
      <p className="text-xs leading-6 text-muted-foreground">
        使用 Skill 固定 v{p.skillVersion}
        。此次范围仅包含下面列出的文件；不会沿双链自动读取其他资料。
      </p>
      <div className="max-h-32 overflow-auto">
        {p.refs.map((r) => {
          const f = s.files.find((f) => f.id === r.id)!;
          return (
            <FileButton
              key={f.id}
              reference={fileRef(f, r.version)}
              onOpen={() =>
                open({
                  type: "preview",
                  id: f.id,
                  version: r.version,
                  single: true,
                })
              }
              variant="ghost"
              className="!h-11 !w-full !justify-start !whitespace-normal text-left text-xs"
            >
              {f.name} · 固定 v{r.version}
            </FileButton>
          );
        })}
      </div>
      {p.sections?.map((section, i) => (
        <div key={i} className="rounded-lg border border-border p-3 text-xs">
          <strong>
            {section.anchor ? "章节：" + section.anchor : "选区引用"} · 固定 v
            {section.version}
          </strong>
          <p className="mt-2 max-h-28 overflow-auto whitespace-pre-wrap leading-6">
            {section.excerpt}
          </p>
        </div>
      ))}
      <div className="grid max-h-[40dvh] gap-3 overflow-auto md:grid-cols-2">
        {target && (
          <section>
            <h3 className="mb-2 text-xs font-medium">修改前 · {target.name}</h3>
            <pre className="whitespace-pre-wrap break-words rounded-lg bg-muted p-3 text-xs leading-6">
              {noteBody(
                s,
                target.id,
                p.refs.find((r) => r.id === target.id)?.version,
              )}
            </pre>
          </section>
        )}
        <section className={!target ? "md:col-span-2" : ""}>
          <h3 className="mb-2 text-xs font-medium">
            {target ? "修改后 · 生成新版本" : "待验收产物 · 收件箱"}
          </h3>
          <pre className="whitespace-pre-wrap break-words rounded-lg bg-accent p-3 text-xs leading-6">
            {p.body}
          </pre>
        </section>
      </div>
      <p className="text-xs text-muted-foreground">
        影响：
        {target
          ? "1 份文档正文；原版本保留"
          : "新增 1 份 Markdown 产物；不覆盖输入文件"}{" "}
        · 模拟执行，无模型调用
      </p>
      {t.status === "approval" ? (
        <Btn onClick={() => dispatch({ type: "k-task-apply", id: taskId })}>
          确认执行本次计划
        </Btn>
      ) : t.status === "done" ? (
        <Btn
          onClick={() => {
            close();
            go(target ? "tasks" : "inbox");
          }}
        >
          查看{target ? "任务与新版本" : "待验收产物"}
        </Btn>
      ) : (
        <Btn
          variant="outline"
          onClick={() => {
            close();
            go("resources");
          }}
        >
          返回资源重新检查
        </Btn>
      )}
      <p role="status" className="text-xs text-primary">
        {s.notice}
      </p>
    </div>
  );
}
