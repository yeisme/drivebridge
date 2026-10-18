import { useEffect, useRef, useState } from "react";
import { Button } from "../../components/ui/button";
import { readNotebookPageTree } from './clients';
import type { NotebookPageMeta } from './contract';
export type { NotebookPageMeta } from './contract';

function titleOf(page: NotebookPageMeta) {
  const name = page.path.split("/").pop() || page.path;
  return name.replace(/\.(?:md|markdown)$/i, "") || name;
}

function Tree({ items, parent, selected, onSelect, seen = new Set<string>() }: { items: NotebookPageMeta[]; parent: string; selected?: string; onSelect: (id: string) => void; seen?: Set<string> }) {
  return <ul className="m-0 list-none space-y-1 pl-3">
    {items.filter(item => item.parent_file_id === parent && !seen.has(item.file_id)).map(item => <li key={item.file_id}>
      <Button type="button" variant="ghost" className="min-h-11 w-full justify-start gap-2 text-left text-xs md:min-h-8" aria-pressed={selected === item.file_id} onClick={() => onSelect(item.file_id)} title={item.path}>
        <span aria-hidden="true">{item.icon === "calendar" ? "◷" : item.icon === "book" ? "▣" : item.icon === "idea" ? "✦" : item.icon === "project" ? "◆" : "•"}</span>
        <span className="truncate">{titleOf(item)}</span>
      </Button>
      <Tree items={items} parent={item.file_id} selected={selected} onSelect={onSelect} seen={new Set([...seen, item.file_id] as string[])}/>
    </li>)}
  </ul>;
}

export function NotebookPageTree({ space, selected, onSelect, onLoaded }: { space: string; selected?: string; onSelect: (id: string | undefined) => void; onLoaded?: (items: NotebookPageMeta[]) => void }) {
  const [state, setState] = useState<"idle" | "loading" | "ready" | "error" | "forbidden" | "offline" | "unsupported">("idle");
  const [items, setItems] = useState<NotebookPageMeta[]>([]);
  const [partial, setPartial] = useState(false);
  const onLoadedRef = useRef(onLoaded);
  onLoadedRef.current = onLoaded;
  const [open, setOpen] = useState(false);
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    if (!open) return;
    const controller = new AbortController();
    setItems([]); setPartial(false); setState("loading");
    const invalidate = () => { controller.abort(); setItems([]); setPartial(false); onSelect(undefined); setState("forbidden"); };
    const offline = () => { controller.abort(); setItems([]); setPartial(false); onSelect(undefined); setState("offline"); };
    window.addEventListener("drivebridge-context", invalidate); window.addEventListener("offline", offline);
    if (!navigator.onLine) offline();
    else (async () => {
      const pages: NotebookPageMeta[] = [];
      let cursor: string | null = null;
      let remaining = false;
      for (let pageNumber = 0; pageNumber < 10; pageNumber++) {
        const page = await readNotebookPageTree(space, cursor, controller.signal);
        const valid: NotebookPageMeta[] = Array.isArray(page.items) ? page.items.filter((item: NotebookPageMeta) => item && item.space_id === space && item.file_id && item.version_id && item.path && typeof item.parent_file_id === "string" && ["file", "book", "calendar", "idea", "project"].includes(item.icon) && Number.isSafeInteger(item.revision)) : [];
        pages.push(...valid);
        cursor = typeof page.next_cursor === "string" && page.next_cursor ? page.next_cursor : null;
        if (!cursor) break;
        if (pageNumber === 9) remaining = true;
      }
      if (controller.signal.aborted) return;
      const loaded = [...new Map(pages.map(item => [item.file_id, item])).values()];
      setItems(loaded);
      if (selected && !loaded.some(item => item.file_id === selected)) onSelect(undefined);
      onLoadedRef.current?.(loaded);
      setPartial(remaining);
      setState("ready");
    })().catch(error => {
      if (controller.signal.aborted) return;
      setItems([]); setPartial(false); onSelect(undefined);
      setState(error?.status === 401 || error?.status === 403 ? "forbidden" : error?.status === 404 || error?.status === 405 ? "unsupported" : "error");
    });
    return () => { controller.abort(); window.removeEventListener("drivebridge-context", invalidate); window.removeEventListener("offline", offline); };
  }, [open, space, attempt]);
  return <details className="border-t px-3 py-2" onToggle={event => { const next = (event.currentTarget as HTMLDetailsElement).open; setOpen(next); if (!next) onSelect(undefined); }}>
    <summary className="cursor-pointer text-xs font-medium focus-visible:outline-2 focus-visible:outline-ring">页面树与新建父页面</summary>
    <div className="space-y-2 pt-2">
      <p className="m-0 text-xs text-muted-foreground">选择父页面后，新建笔记会通过服务端页面合同建立真实 parent_file_id；未选择时创建顶层页面。</p>
      {selected && <Button type="button" variant="outline" className="min-h-11 text-xs md:min-h-8" onClick={() => onSelect(undefined)}>清除父页面：{items.find(item => item.file_id === selected)?.path ?? selected}</Button>}
      {state === "loading" && <p role="status" className="text-xs">正在读取页面树…</p>}
      {state === "forbidden" && <p role="alert" className="text-xs">页面树权限已失效。</p>}
      {state === "error" && <p role="alert" className="text-xs">页面树暂不可用，请稍后重试。</p>}
      {state === "offline" && <p role="alert" className="text-xs">当前离线，页面树已清除；联网后请手动重读。</p>}
      {state === "unsupported" && <p role="status" className="text-xs">当前服务未提供页面树能力，普通文件阅读仍可用。</p>}
      {state !== "loading" && state !== "forbidden" && <Button type="button" variant="outline" className="min-h-11 text-xs md:min-h-8" onClick={() => { onSelect(undefined); setAttempt(value => value + 1); }}>重读页面树</Button>}
      {state === "ready" && !items.length && <p role="status" className="text-xs">当前来源还没有服务端页面元数据。</p>}
      {state === "ready" && partial && <p role="status" className="text-xs">页面树达到本地读取上限，仅显示已加载部分。</p>}
      {state === "ready" && <Tree items={items} parent="" selected={selected} onSelect={onSelect}/>}
    </div>
  </details>;
}
