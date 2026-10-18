import { useEffect, useState } from "react";
import { BookOpen, CheckCircle2, Cloud, FilePlus2, RefreshCw, UploadCloud } from "lucide-react";
import { getDataContext, type Space } from "../../api";
import { SelectAdapter } from "../../components/SelectAdapter";
import { Input } from "../../components/ui/input";
import { Button } from "../../components/ui/button";
import { MarkdownFileDocument, NotebookPageTree, NotebookWorkspace, CreateNotebookPageDialog } from "./components";
import { readKnowledgeFiles } from "./knowledge-files";
import { noteKey } from "./notebook-state";
import type { NotebookEntry } from "./notebook-navigation";

export function KnowledgePage({ actor, spaces, onDirtyChange }: { actor: string; spaces: Space[]; onDirtyChange?: (dirty: boolean) => void }) {
  const managedSpace = getDataContext().space;
  const available = spaces.filter(space => !managedSpace || space.id === managedSpace);
  const [selected, setSelected] = useState("");
  const [dirty, setDirty] = useState(false);
  useEffect(() => { onDirtyChange?.(dirty); }, [dirty, onDirtyChange]);
  useEffect(() => () => { onDirtyChange?.(false); }, [onDirtyChange]);
  const space = available.find(item => item.id === selected)?.id || available[0]?.id || "";
  return <section aria-label="知识库" className="flex h-full min-h-0 min-w-0 flex-col overflow-hidden bg-background [&_*]:border-border">
    <header className="flex min-h-12 shrink-0 flex-wrap items-center gap-3 border-b bg-card/70 px-3 py-2 md:px-4">
      <div className="flex min-w-0 items-center gap-2"><BookOpen className="size-4 shrink-0 text-primary"/><h2 className="m-0 truncate text-[15px] font-semibold tracking-tight">知识库</h2><span className="hidden rounded-full bg-primary/10 px-2 py-0.5 text-[11px] font-medium text-primary md:inline">固定版本阅读</span></div>
      <label className="ml-auto flex min-w-0 items-center gap-2 text-xs text-muted-foreground"><span className="hidden shrink-0 md:inline">文件来源</span>
        <SelectAdapter className="max-w-44 min-w-0 max-md:min-h-11" aria-label="知识库文件来源" value={space} onChange={event => { if (!dirty || window.confirm("放弃未保存的笔记草稿并切换来源？")) { setDirty(false); setSelected(event.target.value); } }} disabled={!available.length}>
          {!available.length && <option value="">没有可访问的空间</option>}
          {available.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}
        </SelectAdapter>
      </label>
    </header>
    {space ? <KnowledgeSource key={JSON.stringify([actor, getDataContext().tenant, managedSpace, space])} actor={actor} space={space} onDirtyChange={setDirty}/> : <p role="status" className="p-6 text-sm text-muted-foreground">没有可访问的文件空间，请先在我的网盘选择或创建空间。</p>}
  </section>;
}

function KnowledgeSource({ actor, space, onDirtyChange }: { actor: string; space: string; onDirtyChange: (dirty: boolean) => void }) {
  const [entries, setEntries] = useState<NotebookEntry[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<"loading" | "ready" | "error" | "forbidden" | "stale" | "unsupported" | "offline">("loading");
  const [coverage, setCoverage] = useState({ scanned: 0, unavailable: 0, stale: 0 });
  const [query, setQuery] = useState("");
  const [parentFileId, setParentFileId] = useState<string | undefined>();
  const [creating, setCreating] = useState<"page" | "daily" | null>(null);
  const [dirty, setDirty] = useState(false);
  const [accessEpoch, setAccessEpoch] = useState(0);
  const revoke = () => { setCreating(null); setParentFileId(undefined); setEntries([]); setNextCursor(null); setState("forbidden"); setAccessEpoch(value => value + 1); };
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
    else readKnowledgeFiles(space, cursor, controller.signal).then(page => {
      if (controller.signal.aborted) return;
      setEntries(previous => [...new Map([...(cursor ? previous : []), ...page.entries].map(entry => [noteKey(entry.ref), entry])).values()]);
      setCoverage(previous => ({ scanned: (cursor ? previous.scanned : 0) + page.scanned,
        unavailable: (cursor ? previous.unavailable : 0) + page.unavailable, stale: (cursor ? previous.stale : 0) + page.stale }));
      setNextCursor(page.nextCursor);
      setState("ready");
    }).catch(error => {
      if (controller.signal.aborted) return;
      if (error.status === 401 || error.status === 403) { setEntries([]); setNextCursor(null); setState("forbidden"); }
      else if (error.status === 409 || error.status === 410) { setEntries([]); setNextCursor(null); setState("stale"); }
      else if (error.status === 404 || error.status === 405 || error.message === "unsupported_action") setState("unsupported");
      else setState("error");
    });
    return () => { controller.abort(); window.removeEventListener("drivebridge-context", invalidate); window.removeEventListener("offline", offline); };
  }, [space, cursor, attempt, accessEpoch]);
  const refresh = () => { if (dirty && !window.confirm("放弃未保存的笔记草稿并刷新目录？")) return; setParentFileId(undefined); setCursor(null); setEntries([]); setNextCursor(null); setCoverage({ scanned: 0, unavailable: 0, stale: 0 }); setAttempt(value => value + 1); };
  const onCreated = () => { setCreating(null); refresh(); };
  const onDenied = () => { setCreating(null); revoke(); };
  const create = (daily = false) => { if (state === "ready") setCreating(daily ? "daily" : "page"); };
  return <>
    <details className="shrink-0 border-b bg-background">
    <summary className="flex min-h-11 cursor-pointer list-none flex-wrap items-center gap-x-3 gap-y-1 px-3 py-2 text-[13px] focus-visible:outline-2 focus-visible:outline-ring"><span className="font-medium">目录与筛选</span><span className="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground" role="status">{state === "loading" ? "正在读取文件目录…" : `已检查 ${coverage.scanned} 个文件 · ${entries.length} 篇 Markdown`}</span><span className="ml-auto text-xs text-muted-foreground">{state === "ready" ? "当前来源" : "状态"}</span></summary>
    <div className="flex flex-wrap items-center gap-2 px-3 pb-3 md:px-4">
      <Input aria-label="筛选已加载笔记" value={query} onChange={event => setQuery(event.target.value)} placeholder="筛选已加载笔记名称…" className="w-56 max-w-full max-md:min-h-11"/>
      <Button type="button" variant="outline" className="gap-1.5 max-md:min-h-11" disabled={state !== "ready"} onClick={() => create(false)}><FilePlus2/>新建笔记</Button>
      <Button type="button" variant="ghost" className="gap-1.5 max-md:min-h-11" disabled={state !== "ready"} onClick={() => create(true)}><CheckCircle2/>日记</Button>
      <Button type="button" variant="ghost" size="icon" className="max-md:size-11" aria-label="刷新知识库" disabled={state === "loading"} onClick={refresh}><RefreshCw/></Button>
      {nextCursor && <Button type="button" variant="outline" disabled={state !== "ready"} onClick={() => setCursor(nextCursor)}>继续查找笔记</Button>}
    </div>
    <p className="m-0 flex items-center gap-1.5 px-3 pb-3 text-xs leading-5 text-muted-foreground md:px-4"><Cloud className="size-3.5"/>当前显示已加载的普通 Markdown；标题筛选不代表全文搜索。本机 Pinax 需通过桌面客户端连接。</p>
    {state !== "forbidden" && <NotebookPageTree key={attempt} space={space} selected={parentFileId} onSelect={setParentFileId} onLoaded={pages => {
      const metadata = new Map(pages.map(page => [page.file_id, page]));
      setEntries(items => items.map(entry => {
        const page = metadata.get(entry.ref.objectId);
        return page ? { ...entry, title: page.path.split("/").pop()?.replace(/\.(?:md|markdown)$/i, "") || entry.title, path: page.path, parentFileId: page.parent_file_id, icon: page.icon } : entry;
      }));
    }}/>}
    </details>
    {creating && <CreateNotebookPageDialog actor={actor} space={space} parent={parentFileId} daily={creating === "daily"} close={() => setCreating(null)} onCreated={onCreated} onDenied={onDenied}/>}
    {(coverage.unavailable > 0 || coverage.stale > 0) && <p role="status" className="px-3 text-sm">部分文件暂不可用或版本已变化；请刷新目录重新读取。</p>}
    {state === "forbidden" && <p role="alert" className="p-4">访问权限已失效，笔记内容已清除。请重新登录或切换来源。</p>}
    {state === "offline" && <p role="alert" className="p-4">当前离线，目录仅为上次观察；已打开草稿保留在内存。联网后请手动刷新并重新鉴权，不会自动保存。</p>}
    {state === "unsupported" && <p role="status" className="p-4">当前服务尚未提供文件查询能力，不能读取知识库。</p>}
    {state === "stale" && <div role="alert" className="p-4"><p>文件目录已经变化，请重新查询，不能拼接不同版本的页面。</p><Button onClick={refresh}>重新查询</Button></div>}
    {state === "error" && <div role="alert" className="p-4"><p>无法读取笔记列表，已打开的固定版本保留。请检查连接后重试。</p><Button onClick={() => setAttempt(value => value + 1)}>重试</Button></div>}
    {state === "ready" && !entries.length && nextCursor && <p role="status" className="mx-3 my-4 rounded-lg border border-dashed border-border bg-card p-4 text-sm text-muted-foreground md:mx-4">当前页未找到 Markdown，可继续查找后续文件。</p>}
    {state === "ready" && !entries.length && !nextCursor && <KnowledgeEmptyState canCreate={Boolean(space)} onCreate={() => create(false)} onUpload={() => { window.location.hash = "#/flows?center=upload"; }}/>}
    {!!entries.length && state !== "forbidden" && state !== "unsupported" && <div className="min-h-0 flex-1">
      <NotebookWorkspace scope={{ actor, space, source: space }} entries={entries} directoryQuery={query} onDirtyChange={setDirty} fileRelations
        onSaved={(previous, next) => { setEntries(items => items.map(entry => noteKey(entry.ref) === noteKey(previous) ? { ...entry, ref: next } : entry)); setNextCursor(null); }}
        renderDocument={({ ref, draft, onDraft, onSaved, editablePane }) => <MarkdownFileDocument actor={actor} identity={ref} draft={draft} onDraft={onDraft} onSaved={onSaved} onRevoked={revoke} editablePane={editablePane}/>}/>
    </div>}
  </>;
}

function KnowledgeEmptyState({ canCreate, onCreate, onUpload }: { canCreate: boolean; onCreate: () => void; onUpload: () => void }) {
  return <section role="status" className="mx-3 my-5 grid min-h-64 place-items-center rounded-xl border border-dashed border-border bg-gradient-to-br from-card via-card to-primary/5 p-8 text-center md:mx-4">
    <div className="max-w-md">
      <div className="mx-auto grid size-12 place-items-center rounded-2xl bg-primary/10 text-primary"><BookOpen/></div>
      <h3 className="mt-4 text-base font-semibold tracking-tight">从一篇笔记开始建立你的知识库</h3>
      <p className="mt-2 text-sm leading-6 text-muted-foreground">当前来源没有可阅读的 Markdown 文件。</p>
      <p className="mt-1 text-sm leading-6 text-muted-foreground">创建一篇笔记，或从我的网盘上传已有资料。</p>
      <div className="mt-5 flex flex-wrap justify-center gap-2">
        <Button type="button" className="min-h-11 gap-1.5" disabled={!canCreate} onClick={onCreate}><FilePlus2/>新建笔记</Button>
        <Button type="button" variant="outline" className="min-h-11 gap-1.5" onClick={onUpload}><UploadCloud/>上传 Markdown</Button>
      </div>
      <p className="mt-4 text-xs text-muted-foreground">桌面客户端可另外连接 Pinax，本页不会读取本机 vault。</p>
    </div>
  </section>;
}
