import { useEffect, useState } from "react";
import { BookOpen, RefreshCw } from "lucide-react";
import { getDataContext, type Space } from "../../api";
import { SelectAdapter } from "../../components/SelectAdapter";
import { Input } from "../../components/ui/input";
import { Button } from "../../components/ui/button";
import { MarkdownFileDocument } from "./MarkdownFileDocument";
import { NotebookWorkspace } from "./NotebookWorkspace";
import { readKnowledgeFiles } from "./knowledge-files";
import { noteKey } from "./notebook-state";
import type { NotebookEntry } from "./notebook-navigation";
import { createNotebookDaily, createNotebookPage } from "./notebook-client";

export function KnowledgePage({ actor, spaces, onDirtyChange }: { actor: string; spaces: Space[]; onDirtyChange?: (dirty: boolean) => void }) {
  const managedSpace = getDataContext().space;
  const available = spaces.filter(space => !managedSpace || space.id === managedSpace);
  const [selected, setSelected] = useState("");
  const [dirty, setDirty] = useState(false);
  useEffect(() => { onDirtyChange?.(dirty); }, [dirty, onDirtyChange]);
  useEffect(() => () => { onDirtyChange?.(false); }, [onDirtyChange]);
  const space = available.find(item => item.id === selected)?.id || available[0]?.id || "";
  return <section aria-label="知识库" className="flex h-full min-h-0 min-w-0 flex-col overflow-hidden bg-background [&_*]:border-border">
    <header className="flex min-h-11 shrink-0 items-center gap-2 border-b px-3">
      <BookOpen className="size-4 shrink-0 text-primary"/><h2 className="m-0 shrink-0 text-[15px] font-semibold">知识库</h2>
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
  const [state, setState] = useState<"loading" | "ready" | "error" | "forbidden" | "stale" | "unsupported">("loading");
  const [coverage, setCoverage] = useState({ scanned: 0, unavailable: 0, stale: 0 });
  const [query, setQuery] = useState("");
  const [dirty, setDirty] = useState(false);
  const [accessEpoch, setAccessEpoch] = useState(0);
  const revoke = () => { setEntries([]); setNextCursor(null); setState("forbidden"); setAccessEpoch(value => value + 1); };
  useEffect(() => { onDirtyChange(dirty); }, [dirty, onDirtyChange]);
  useEffect(() => {
    if (accessEpoch > 0) return;
    const controller = new AbortController();
    setState("loading");
    const invalidate = () => { controller.abort(); setEntries([]); setNextCursor(null); setState("forbidden"); };
    window.addEventListener("drivebridge-context", invalidate);
    readKnowledgeFiles(space, cursor, controller.signal).then(page => {
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
    return () => { controller.abort(); window.removeEventListener("drivebridge-context", invalidate); };
  }, [space, cursor, attempt, accessEpoch]);
  const refresh = () => { if (dirty && !window.confirm("放弃未保存的笔记草稿并刷新目录？")) return; setCursor(null); setEntries([]); setNextCursor(null); setCoverage({ scanned: 0, unavailable: 0, stale: 0 }); setAttempt(value => value + 1); };
  const create = async (daily = false) => {
    const title = daily ? new Date().toISOString().slice(0, 10) : window.prompt("新建笔记标题", "未命名笔记")?.trim();
    if (!title) return;
    const key = `notebook-${Array.from(crypto.getRandomValues(new Uint8Array(16)), b => b.toString(16).padStart(2, "0")).join("")}`;
    const controller = new AbortController();
    try { await (daily ? createNotebookDaily(space, title, key, controller.signal) : createNotebookPage(space, title, "", "file", key, controller.signal)); refresh(); }
    catch (error) { if (!(error instanceof DOMException && error.name === "AbortError")) window.alert("笔记创建失败，请保留原请求后重试。"); }
  };
  return <>
    <details className="shrink-0 border-b bg-background">
    <summary className="flex min-h-11 cursor-pointer flex-wrap items-center gap-x-3 gap-y-1 px-3 py-2 text-[13px] focus-visible:outline-2 focus-visible:outline-ring"><span>目录与筛选</span><span className="text-xs text-muted-foreground" role="status">{state === "loading" ? "正在读取文件目录…" : `已检查 ${coverage.scanned} 个文件 · ${entries.length} 篇 Markdown`}</span></summary>
    <div className="flex flex-wrap items-center gap-2 px-3 pb-2">
      <Input aria-label="筛选已加载笔记" value={query} onChange={event => setQuery(event.target.value)} placeholder="筛选已加载笔记名称…" className="w-56 max-w-full max-md:min-h-11"/>
      <Button type="button" variant="outline" onClick={() => void create(false)}>新建笔记</Button>
      <Button type="button" variant="ghost" onClick={() => void create(true)}>日记</Button>
      <Button type="button" variant="ghost" size="icon" className="max-md:size-11" aria-label="刷新知识库" disabled={state === "loading"} onClick={refresh}><RefreshCw/></Button>
      {nextCursor && <Button type="button" variant="outline" disabled={state !== "ready"} onClick={() => setCursor(nextCursor)}>继续查找笔记</Button>}
    </div>
    <p className="m-0 px-3 pb-3 text-xs leading-5 text-muted-foreground">当前显示已加载的普通 Markdown；标题筛选不代表全文搜索。本机 Pinax 需通过桌面客户端连接。</p>
    </details>
    {(coverage.unavailable > 0 || coverage.stale > 0) && <p role="status" className="px-3 text-sm">部分文件暂不可用或版本已变化；请刷新目录重新读取。</p>}
    {state === "forbidden" && <p role="alert" className="p-4">访问权限已失效，笔记内容已清除。请重新登录或切换来源。</p>}
    {state === "unsupported" && <p role="status" className="p-4">当前服务尚未提供文件查询能力，不能读取知识库。</p>}
    {state === "stale" && <div role="alert" className="p-4"><p>文件目录已经变化，请重新查询，不能拼接不同版本的页面。</p><Button onClick={refresh}>重新查询</Button></div>}
    {state === "error" && <div role="alert" className="p-4"><p>无法读取笔记列表，已打开的固定版本保留。请检查连接后重试。</p><Button onClick={() => setAttempt(value => value + 1)}>重试</Button></div>}
    {state === "ready" && !entries.length && <p role="status" className="p-4">{nextCursor ? "当前页未找到 Markdown，可继续查找后续文件。" : "当前来源没有可阅读的 Markdown 文件。"}</p>}
    {!!entries.length && state !== "forbidden" && state !== "unsupported" && <div className="min-h-0 flex-1">
      <NotebookWorkspace scope={{ actor, space, source: space }} entries={entries} directoryQuery={query} onDirtyChange={setDirty} fileRelations
        onSaved={(previous, next) => { setEntries(items => items.map(entry => noteKey(entry.ref) === noteKey(previous) ? { ...entry, ref: next } : entry)); setNextCursor(null); }}
        renderDocument={({ ref, draft, onDraft, onSaved, editablePane }) => <MarkdownFileDocument actor={actor} identity={ref} draft={draft} onDraft={onDraft} onSaved={onSaved} onRevoked={revoke} editablePane={editablePane}/>}/>
    </div>}
  </>;
}
