import { BrowseControls } from "./BrowseControls";
import { ScopedFilter } from "./ScopedFilter";
import { FindResults, PropertyFilters } from "./UnifiedFind";
import { browseLocation } from "./browse-model";
import { useLayoutEffect, useState } from "react";
import {
  Search,
  SlidersHorizontal,
  Bookmark,
  FolderOpen,
  X,
} from "lucide-react";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Btn, Choice, Empty, IconBtn, Pill, useDemo } from "./ui";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "./Dialog";
import { FileToolbar } from "./CloudToolbar";
import { TagPicker } from "./TagWorkspace";
import { FileButton } from "./HoverPreview";
import { fileRef } from "./preview-model";
import { searchResults, scopeFiles } from "./cloud-model";
import { directoryContains, isObserved, rootNode } from "./storage-model";
export function FileSearchWorkspace() {
  const { state: s, dispatch, go, open } = useDemo();
  const q = s.search;
  useLayoutEffect(() => {
    const main = document.getElementById("main-content");
    const loc = browseLocation(s);
    if (!main || loc.surface !== "search") return;
    main.scrollTop = loc.scroll;
    if (q.tab === "files") main.dataset.browseRestoring = "true";
    else delete main.dataset.browseRestoring;
    const onScroll = () =>
      !main.dataset.browseRestoring &&
      dispatch({ type: "browse-checkpoint", scroll: main.scrollTop });
    main.addEventListener("scroll", onScroll);
    return () => {
      main.removeEventListener("scroll", onScroll);
      delete main.dataset.browseRestoring;
    };
  }, [s.browse.revision, q.tab]);
  const [composing, setComposing] = useState(false),
    [refresh, setRefresh] = useState(0);
  const [saving, setSaving] = useState(false),
    [views, setViews] = useState(false),
    [name, setName] = useState("");
  const patch = (patch: Partial<typeof q>) =>
    dispatch({ type: "search-patch", patch });
  const source = scopeFiles(s, q);
  const found = s.searchSubmitted
    ? searchResults(s).filter(
        (r) => q.tab === "content" || isObserved(s, r.file),
      )
    : [];
  const incomplete =
    q.connections.some(
      (id) =>
        id !== "drive" &&
        !Object.values(s.storage.scopes).some(
          (sc) =>
            sc.confirmed &&
            s.storage.nodes.find((n) => n.id === sc.directoryId)
              ?.connectionId === id &&
            !!sc.lastComplete,
        ),
    ) ||
    q.connections.some(
      (id) => s.connections.find((c) => c.id === id)?.status !== "active",
    );
  const node = s.storage.nodes.find((n) => n.id === q.directoryId);
  const connections = s.connections.filter(
    (c) => c.id === "drive" || c.space === s.space,
  );
  const scopeLabel = node
    ? `${connections.find((c) => c.id === node.connectionId)?.name} / ${node.path}`
    : q.connections
        .map((id) => connections.find((c) => c.id === id)?.name || "连接不可用")
        .join("、");
  const searchActions = (
    <>
      {[
        { id: "files", name: "文件" },
        { id: "content", name: "内容" },
      ].map((x) => (
        <Btn
          key={x.id}
          aria-pressed={q.tab === x.id}
          variant={q.tab === x.id ? "secondary" : "ghost"}
          onClick={() => patch({ tab: x.id as "files" | "content" })}
        >
          {x.name}
        </Btn>
      ))}
      <IconBtn label="保存搜索" onClick={() => setSaving(true)}>
        <Bookmark size={16} />
      </IconBtn>
      <IconBtn label="保存视图" onClick={() => setViews(true)}>
        <FolderOpen size={16} />
      </IconBtn>
    </>
  );
  return (
    <section className="mx-auto max-w-none" aria-label="文件检索">
      <form
        className="flex h-11 min-w-0 items-center gap-1 border-b"
        onSubmit={(e) => {
          e.preventDefault();
          if (composing) return;
          setRefresh((x) => x + 1);
          dispatch({ type: "search-submit" });
        }}
      >
        <BrowseControls more />
        <h1 className="hidden shrink-0 text-sm font-semibold sm:block">检索</h1>
        <Input
          title={`范围：${scopeLabel}`}
          autoComplete="off"
          aria-label="搜索内容"
          placeholder={
            q.tab === "content"
              ? "搜索已授权索引中的正文"
              : "搜索文件名称或目录"
          }
          onCompositionStart={() => setComposing(true)}
          onCompositionEnd={() => setComposing(false)}
          value={q.query}
          onChange={(e) => patch({ query: e.target.value })}
          className="!h-8 min-w-0 flex-1 text-xs max-[767px]:!h-11"
        />
        <IconBtn
          label="清除搜索"
          className={!q.query ? "invisible" : ""}
          disabled={!q.query}
          onClick={() => patch({ query: "" })}
        >
          <X size={14} />
        </IconBtn>
        <IconBtn label="执行搜索" type="submit">
          <Search size={17} />
        </IconBtn>
        <ScopedFilter
          label="检索范围与筛选"
          value={q}
          allowConnections
          onApply={(query) =>
            dispatch({ type: "browse-query", surface: "search", query })
          }
        />
      </form>
      {q.tab === "content" && (
        <div className="my-2 rounded-lg border bg-card p-3 text-xs leading-6">
          内容索引独立授权；未索引不代表没有相关内容。已加载文本匹配与语义候选分别展示。
          <Btn
            variant="ghost"
            onClick={() =>
              dispatch({
                type: "storage-page",
                tab: "indexes",
                directoryId: q.directoryId || rootNode(s, q.connections[0])?.id,
              })
            }
          >
            管理内容索引
          </Btn>
        </div>
      )}
      {q.tab !== "content" && (
        <FindResults
          query={q}
          composing={composing}
          refresh={refresh}
          toolbarExtra={searchActions}
          rangeLabel={`${scopeLabel} · ${q.recursive === false ? "仅本层" : "包含子目录"}`}
        />
      )}
      {q.tab === "content" && (
        <>
          <FileToolbar
            extra={searchActions}
            summary={`${found.length} 项 · ${scopeLabel}`}
            count={found.length}
            shown={[...new Set(found.map((r) => r.file.id))]}
          />
          {(incomplete || q.tab === "content") && (
            <p className="mb-3 text-xs text-muted-foreground">
              覆盖：{incomplete ? "部分或未知" : "当前已授权索引"} ·{" "}
              {q.tab === "content"
                ? `${s.indexes.filter((i) => i.space === s.space).length} 个内容范围`
                : "元数据索引"}{" "}
              · 观测时间见来源索引。
              <button
                className="ml-2 min-h-11 text-primary"
                onClick={() =>
                  dispatch({ type: "storage-page", tab: "indexes" })
                }
              >
                查看缺口
              </button>
            </p>
          )}
          <div
            className="overflow-hidden rounded-lg border bg-card"
            aria-label="检索结果"
          >
            {found.map((r) => (
              <article
                key={`${r.file.id}@${r.version}`}
                className="flex min-w-0 items-start gap-3 border-b p-3"
              >
                <Checkbox
                  className="mt-3"
                  aria-label={`选择 ${r.file.name}`}
                  checked={s.selected.includes(r.file.id)}
                  onCheckedChange={() =>
                    dispatch({
                      type: "select",
                      ids: s.selected.includes(r.file.id)
                        ? s.selected.filter((id) => id !== r.file.id)
                        : [...s.selected, r.file.id],
                    })
                  }
                />
                <div className="min-w-0 flex-1">
                  <FileButton
                    reference={fileRef(r.file, r.version)}
                    variant="ghost"
                    className="!h-auto min-h-11 max-w-full !justify-start !px-0 !whitespace-normal text-left"
                    onOpen={() =>
                      open({
                        type: "preview",
                        id: r.file.id,
                        version: r.version,
                        scope: {
                          label: "检索结果",
                          refs: found.map((x) => fileRef(x.file, x.version)),
                        },
                      })
                    }
                  >
                    {r.file.name}
                  </FileButton>
                  <button
                    className="flex min-h-11 max-w-full items-center gap-1 text-left text-xs text-muted-foreground"
                    onClick={() =>
                      r.file.directoryId &&
                      dispatch({
                        type: "directory-open",
                        id: r.file.directoryId,
                      })
                    }
                  >
                    <FolderOpen size={12} />
                    <span className="truncate">
                      {
                        connections.find((c) => c.id === r.file.connectionId)
                          ?.name
                      }{" "}
                      / {r.file.folder}
                    </span>
                  </button>
                  <p className="text-xs leading-6">{r.excerpt}</p>
                  <p className="text-[10px] text-muted-foreground">
                    {r.file.externalRevision || `v${r.version}`} · {r.file.size}{" "}
                    · {r.indexed ? "正文文本匹配 · 固定引用" : "名称／路径匹配"}
                    {r.stale ? " · 索引过期，请更新" : ""}
                  </p>
                </div>
              </article>
            ))}
            {!found.length && (
              <Empty
                title={
                  s.searchSubmitted
                    ? incomplete || q.tab === "content"
                      ? "当前已索引范围内未找到"
                      : "没有符合条件的文件"
                    : "选择范围，开始检索"
                }
                text={
                  incomplete
                    ? "部分存储未扫描或连接不可用，可在索引中心查看。"
                    : "搜索不会改变目录、标签或访问权限。"
                }
              />
            )}
          </div>
        </>
      )}
      {q.tab === "content" && (
        <div className="mt-4 rounded-lg border border-dashed p-4 text-sm">
          <strong>语义候选</strong>
          <p className="mt-2 text-xs text-muted-foreground">
            真实语义服务待接入。候选不等于全部匹配；不自动分析新目录，不扩大授权。
          </p>
        </div>
      )}

      <Dialog open={saving} onOpenChange={setSaving}>
        <DialogContent>
          <DialogTitle>保存搜索视图</DialogTitle>
          <DialogDescription>
            保存条件，不复制文件；再次查询时重新检查权限。
          </DialogDescription>
          <Input
            aria-label="保存搜索名称"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="搜索视图名称"
          />
          <Btn
            disabled={!name.trim()}
            onClick={() => {
              dispatch({ type: "search-save", name });
              setSaving(false);
              setName("");
            }}
          >
            保存视图
          </Btn>
        </DialogContent>
      </Dialog>
      <Dialog open={views} onOpenChange={setViews}>
        <DialogContent>
          <DialogTitle>已保存搜索</DialogTitle>
          <DialogDescription>
            每个视图保留显式范围与组合筛选。
          </DialogDescription>
          {s.savedSearches
            .filter((v) => v.space === s.space)
            .map((v) => (
              <div key={v.id} className="flex items-center">
                <Btn
                  variant="ghost"
                  className="min-w-0 flex-1 !justify-start"
                  onClick={() => {
                    dispatch({ type: "search-open", id: v.id });
                    setViews(false);
                  }}
                >
                  {v.name}
                </Btn>
                <Btn
                  variant="ghost"
                  onClick={() => dispatch({ type: "search-delete", id: v.id })}
                >
                  移除
                </Btn>
              </div>
            ))}
        </DialogContent>
      </Dialog>
    </section>
  );
}
