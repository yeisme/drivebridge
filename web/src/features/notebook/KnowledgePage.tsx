import { useEffect, useReducer, useState } from "react";
import { BookOpen, CheckCircle2, Cloud, FilePlus2, FolderPlus, RefreshCw, UploadCloud } from "lucide-react";
import { getDataContext, type Space } from "../../api";
import { SelectAdapter } from "../../components/SelectAdapter";
import { Input } from "../../components/ui/input";
import { Button } from "../../components/ui/button";
import { PageToolbar, WorkspaceEmptyState, WorkspaceNotice } from "../../components/workspace";
import { CreateNotebookDirectoryDialog, CreateNotebookPageDialog, MarkdownFileDocument, NotebookDirectoryTree, NotebookWorkspace } from "./components";
import { readKnowledgeFiles } from "./clients";
import { notebookLifecycleFromError, projectKnowledgeFilePage } from "./projection";
import { initialNotebookPageUiState, notebookPageUiReducer } from "./state";
import { noteKey } from "./notebook-state";
import type { NotebookEntry } from "./notebook-navigation";
import type { NotebookLifecycle } from "./contract";

export function KnowledgePage({ actor, spaces, onDirtyChange }: { actor: string; spaces: Space[]; onDirtyChange?: (dirty: boolean) => void }) {
  const managedSpace = getDataContext().space;
  const available = spaces.filter(space => !managedSpace || space.id === managedSpace);
  const [selected, setSelected] = useState("");
  const [dirty, setDirty] = useState(false);
  useEffect(() => { onDirtyChange?.(dirty); }, [dirty, onDirtyChange]);
  useEffect(() => () => { onDirtyChange?.(false); }, [onDirtyChange]);
  const space = available.find(item => item.id === selected)?.id || available[0]?.id || "";
  return <section aria-label="知识库" className="knowledge-page flex h-full min-h-0 min-w-0 flex-col overflow-hidden bg-background [&_*]:border-border">
    <PageToolbar eyebrow="固定版本阅读" title="知识库" subtitle="从当前网盘目录打开 Markdown 笔记" actions={<label className="knowledge-source-picker flex min-w-0 items-center gap-2 text-xs text-muted-foreground"><span className="hidden shrink-0 md:inline">文件来源</span>
        <SelectAdapter className="max-w-44 min-w-0 max-md:min-h-11" aria-label="知识库文件来源" value={space} onChange={event => { if (!dirty || window.confirm("放弃未保存的笔记草稿并切换来源？")) { setDirty(false); setSelected(event.target.value); } }} disabled={!available.length}>
          {!available.length && <option value="">没有可访问的空间</option>}
          {available.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}
        </SelectAdapter>
      </label>}>
      <BookOpen className="knowledge-title-icon size-4 shrink-0 text-primary" aria-hidden="true" />
    </PageToolbar>
    {space ? <KnowledgeSource key={JSON.stringify([actor, getDataContext().tenant, managedSpace, space])} actor={actor} space={space} onDirtyChange={setDirty}/> : <p role="status" className="p-6 text-sm text-muted-foreground">没有可访问的文件空间，请先在我的网盘选择或创建空间。</p>}
  </section>;
}

function KnowledgeSource({ actor, space, onDirtyChange }: { actor: string; space: string; onDirtyChange: (dirty: boolean) => void }) {
  const [entries, setEntries] = useState<NotebookEntry[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<NotebookLifecycle>("loading");
  const [coverage, setCoverage] = useState({ scanned: 0, unavailable: 0, stale: 0 });
  const [ui, dispatchUi] = useReducer(notebookPageUiReducer, initialNotebookPageUiState);
  const query = ui.sourceQuery;
  const [directoryPath, setDirectoryPath] = useState("");
  const [directoryReload, setDirectoryReload] = useState(0);
  const [controlsOpen, setControlsOpen] = useState(false);
  const [creating, setCreating] = useState<"directory" | "page" | "daily" | null>(null);
  const [pendingOpen, setPendingOpen] = useState<{ sourceId: string; objectId: string; revision: string }>();
  const [dirty, setDirty] = useState(false);
  const [accessEpoch, setAccessEpoch] = useState(0);
  const revoke = () => { setCreating(null); setDirectoryPath(""); setEntries([]); setNextCursor(null); setPendingOpen(undefined); setState("forbidden"); setAccessEpoch(value => value + 1); };
  useEffect(() => { onDirtyChange(dirty); }, [dirty, onDirtyChange]);
  useEffect(() => {
    if (accessEpoch > 0) return;
    const controller = new AbortController();
    setState("loading");
    const invalidate = () => { controller.abort(); setEntries([]); setNextCursor(null); setCreating(null); setState("forbidden"); };
    const offline = () => { controller.abort(); setNextCursor(null); setCreating(null); setState("offline"); };
    window.addEventListener("drivebridge-context", invalidate);
    window.addEventListener("offline", offline);
    if (!navigator.onLine) offline();
    else readKnowledgeFiles(space, cursor, controller.signal, directoryPath).then(page => {
      if (controller.signal.aborted) return;
      const projection = projectKnowledgeFilePage(page);
      setEntries(previous => [...new Map([...(cursor ? previous : []), ...projection.entries].map(entry => [noteKey(entry.ref), entry])).values()]);
      setCoverage(previous => ({ scanned: (cursor ? previous.scanned : 0) + projection.scanned,
        unavailable: (cursor ? previous.unavailable : 0) + projection.unavailable, stale: (cursor ? previous.stale : 0) + projection.stale }));
      setNextCursor(projection.nextCursor);
      setState("ready");
    }).catch(error => {
      if (controller.signal.aborted) return;
      const lifecycle = notebookLifecycleFromError(error);
      if (lifecycle === "forbidden") { setEntries([]); setNextCursor(null); setState("forbidden"); }
      else if (lifecycle === "stale") { setEntries([]); setNextCursor(null); setState("stale"); }
      else if (lifecycle === "unsupported") setState("unsupported");
      else setState("error");
    });
    return () => { controller.abort(); window.removeEventListener("drivebridge-context", invalidate); window.removeEventListener("offline", offline); };
  }, [space, directoryPath, cursor, attempt, accessEpoch]);
  const refresh = (nextDirectory = directoryPath) => { if (dirty && !window.confirm("放弃未保存的笔记草稿并刷新目录？")) return; setDirectoryPath(nextDirectory); setCursor(null); setEntries([]); setNextCursor(null); setCoverage({ scanned: 0, unavailable: 0, stale: 0 }); setAttempt(value => value + 1); };
  const selectDirectory = (nextDirectory: string) => { if (nextDirectory === directoryPath) return; refresh(nextDirectory); };
  const onCreated = (page?: { file_id: string; space_id: string; version_id: string } | null) => { setCreating(null); if (page) setPendingOpen({ sourceId: page.space_id, objectId: page.file_id, revision: page.version_id }); refresh(); };
  const onDirectoryCreated = (path: string) => { setCreating(null); setControlsOpen(true); setDirectoryReload(value => value + 1); refresh(path); };
  const onDenied = () => { setCreating(null); revoke(); };
  const canCreate = !["forbidden", "offline", "unsupported"].includes(state);
  const create = (daily = false) => { if (canCreate) setCreating(daily ? "daily" : "page"); };
  return <>
    <details className="knowledge-source knowledge-controls shrink-0 border-b bg-background" open={controlsOpen} onToggle={event => setControlsOpen(event.currentTarget.open)}>
    <summary className="flex min-h-11 cursor-pointer list-none flex-wrap items-center gap-x-3 gap-y-1 px-3 py-2 text-[13px] focus-visible:outline-2 focus-visible:outline-ring"><span className="font-medium">目录与筛选</span><span className="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground" role="status">{state === "loading" ? "正在读取文件目录…" : `已检查 ${coverage.scanned} 个文件 · ${entries.length} 篇 Markdown`}</span><span className="ml-auto text-xs text-muted-foreground">{state === "ready" ? "当前来源" : "状态"}</span></summary>
    <div className="knowledge-controls-body">
    <div className="knowledge-controls-row flex flex-wrap items-center gap-2 px-3 pb-3 md:px-4">
      <Input aria-label="筛选已加载笔记" value={query} onChange={event => dispatchUi({ type: "set-query", query: event.target.value })} placeholder="筛选已加载笔记名称…" className="w-56 max-w-full max-md:min-h-11"/>
      <Button type="button" variant="outline" className="gap-1.5 max-md:min-h-11" disabled={!canCreate} onClick={() => create(false)}><FilePlus2/>新建笔记</Button>
      <Button type="button" variant="ghost" className="gap-1.5 max-md:min-h-11" disabled={!canCreate} onClick={() => create(true)}><CheckCircle2/>日记</Button>
      <Button type="button" variant="ghost" className="gap-1.5 max-md:min-h-11" disabled={!canCreate} onClick={() => setCreating("directory")}><FolderPlus/>新建目录</Button>
      <Button type="button" variant="ghost" size="icon" className="max-md:size-11" aria-label="刷新知识库" disabled={state === "loading"} onClick={() => refresh()}><RefreshCw/></Button>
      {nextCursor && <Button type="button" variant="outline" disabled={state !== "ready"} onClick={() => setCursor(nextCursor)}>继续查找笔记</Button>}
    </div>
    <p className="knowledge-path m-0 flex items-center gap-1.5 px-3 pb-3 text-xs leading-5 text-muted-foreground md:px-4"><Cloud className="size-3.5"/>当前目录：{directoryPath || "网盘根目录"} · 显示直属 Markdown；本机 Pinax 需通过桌面客户端连接。</p>
    {state !== "forbidden" && <NotebookDirectoryTree space={space} selected={directoryPath} onSelect={selectDirectory} reloadToken={directoryReload}/>}
    </div>
    </details>
    {creating === "directory" && <CreateNotebookDirectoryDialog actor={actor} space={space} parentPath={directoryPath} close={() => setCreating(null)} onCreated={onDirectoryCreated} onDenied={onDenied}/>}
    {(creating === "page" || creating === "daily") && <CreateNotebookPageDialog actor={actor} space={space} directoryPath={directoryPath} daily={creating === "daily"} close={() => setCreating(null)} onCreated={onCreated} onDenied={onDenied}/>}
    {(coverage.unavailable > 0 || coverage.stale > 0) && <WorkspaceNotice title="目录部分可用" detail="部分文件暂不可用或版本已变化；请刷新目录重新读取。" tone="warning" />}
    {state === "forbidden" && <WorkspaceNotice title="访问权限已失效" detail="访问权限已失效，笔记内容已清除。请重新登录或切换来源。" tone="danger" />}
    {state === "offline" && <WorkspaceNotice title="当前离线" detail="当前离线，目录仅为上次观察；已打开草稿保留在内存。联网后请手动刷新并重新鉴权，不会自动保存。" tone="warning" />}
    {state === "unsupported" && <WorkspaceNotice title="暂不支持文件查询" detail="当前服务尚未提供文件查询能力，不能读取知识库。" />}
    {state === "stale" && <WorkspaceNotice title="目录版本已变化" detail="文件目录已经变化，请重新查询，不能拼接不同版本的页面。" action={<Button onClick={() => refresh()}>重新查询</Button>} tone="warning" />}
    {state === "error" && <WorkspaceNotice title="无法读取笔记列表" detail="无法读取笔记列表，已打开的固定版本保留。请检查连接后重试。" action={<Button onClick={() => setAttempt(value => value + 1)}>重试</Button>} tone="danger" />}
    {state === "ready" && !entries.length && nextCursor && <p role="status" className="mx-3 my-4 rounded-lg border border-dashed border-border bg-card p-4 text-sm text-muted-foreground md:mx-4">当前页未找到 Markdown，可继续查找后续文件。</p>}
    {state === "ready" && !entries.length && !nextCursor && <KnowledgeEmptyState canCreate={Boolean(space)} onCreate={() => create(false)} onUpload={() => { window.location.hash = "#/flows?center=upload"; }}/>}
    {!!entries.length && state !== "forbidden" && state !== "unsupported" && <div className="min-h-0 flex-1">
      <NotebookWorkspace key={pendingOpen ? `${space}:${pendingOpen.objectId}` : space} scope={{ actor, space, source: space }} entries={entries} initialRef={pendingOpen} directoryQuery={query} onDirtyChange={setDirty} fileRelations
        onSaved={(previous, next) => { setEntries(items => items.map(entry => noteKey(entry.ref) === noteKey(previous) ? { ...entry, ref: next } : entry)); setNextCursor(null); }}
        renderDocument={({ ref, draft, onDraft, onSaved, editablePane, mode, onModeChange, title, path }) => <MarkdownFileDocument actor={actor} identity={ref} draft={draft} onDraft={onDraft} onSaved={onSaved} onRevoked={revoke} editablePane={editablePane} mode={mode} onModeChange={onModeChange} title={title} path={path}/>}/>
    </div>}
  </>;
}

function KnowledgeEmptyState({ canCreate, onCreate, onUpload }: { canCreate: boolean; onCreate: () => void; onUpload: () => void }) {
  return <WorkspaceEmptyState icon={<BookOpen/>} title="从一篇笔记开始建立你的知识库" description={<><span>当前来源没有可阅读的 Markdown 文件。</span><br/>创建一篇笔记，或从我的网盘上传已有资料。</>} actions={<><Button type="button" className="min-h-11 gap-1.5" disabled={!canCreate} onClick={onCreate}><FilePlus2/>新建笔记</Button><Button type="button" variant="outline" className="min-h-11 gap-1.5" onClick={onUpload}><UploadCloud/>上传 Markdown</Button></>} />;
}
