import { Button } from "../../components/ui/button";
import { noteKey, type NoteIdentity } from "./notebook-state";
import type { NotebookEntry } from "./notebook-navigation";

export function NotebookDirectory({ entries, activeKey, onOpen, onPin }: {
  entries: NotebookEntry[]; activeKey?: string;
  onOpen: (ref: NoteIdentity) => void; onPin: (ref: NoteIdentity) => void;
}) {
  const groups = new Map<string, NotebookEntry[]>();
  for (const entry of entries) {
    const path = entry.path?.split("/").slice(0, -1).join("/") || "当前来源";
    groups.set(path, [...(groups.get(path) ?? []), entry]);
  }
  return <nav aria-label="笔记目录列表" className="min-w-0 overflow-auto p-2">
    <p className="mb-2 text-xs text-muted-foreground">当前来源 · 已加载笔记</p>
    {[...groups].map(([path, notes]) => <details key={path} open>
      <summary className="min-h-11 cursor-pointer break-words py-2 text-sm">{path}</summary>
      <ul className="m-0 list-none p-0">
        {notes.map(entry => <li key={noteKey(entry.ref)}><Button type="button" variant="ghost" className="min-h-11 w-full justify-start overflow-hidden rounded-md text-[13px] aria-[current=page]:bg-accent aria-[current=page]:font-semibold md:min-h-9" aria-current={activeKey === noteKey(entry.ref) ? "page" : undefined}
          onClick={() => onOpen(entry.ref)} onDoubleClick={() => onPin(entry.ref)} title={entry.path ?? entry.title}><span className="truncate">{entry.title}</span></Button></li>)}
      </ul>
    </details>)}
    {!entries.length && <p role="status">当前来源没有已加载笔记。</p>}
  </nav>;
}
