import { FileText, Pin } from "lucide-react";
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
  return <nav aria-label="笔记目录列表" className="min-w-0 overflow-auto bg-card/40 p-3">
    <div className="mb-3 flex items-center justify-between gap-2"><p className="m-0 text-xs font-medium text-muted-foreground">当前来源 · 已加载笔记</p><span className="rounded-full bg-muted px-2 py-0.5 text-[11px] tabular-nums text-muted-foreground">{entries.length}</span></div>
    {[...groups].map(([path, notes]) => <details key={path} open className="group mb-2 last:mb-0">
      <summary aria-label={`${path}，${notes.length} 篇笔记`} className="flex min-h-10 cursor-pointer items-center justify-between gap-2 rounded-md px-2 py-1.5 text-xs font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"><span className="min-w-0 break-words">{path}</span></summary>
      <ul className="m-0 list-none p-0">
        {notes.map(entry => <li key={noteKey(entry.ref)}><Button type="button" variant="ghost" className="group/note min-h-11 w-full justify-start gap-2 overflow-hidden rounded-md px-2 text-left text-[13px] aria-[current=page]:bg-primary/10 aria-[current=page]:font-semibold aria-[current=page]:text-primary md:min-h-9" aria-current={activeKey === noteKey(entry.ref) ? "page" : undefined}
          onClick={() => onOpen(entry.ref)} onDoubleClick={() => onPin(entry.ref)} title={entry.path ?? entry.title}><FileText className="size-3.5 shrink-0 text-muted-foreground"/><span className="min-w-0 flex-1 truncate">{entry.title}</span>{activeKey === noteKey(entry.ref) && <span className="size-1.5 shrink-0 rounded-full bg-primary" aria-hidden="true"/>}</Button></li>)}
      </ul>
    </details>)}
    {!entries.length && <div role="status" className="rounded-lg border border-dashed border-border bg-background p-4 text-xs leading-5 text-muted-foreground"><FileText className="mb-2 size-4 text-primary"/><p className="m-0">当前来源没有已加载笔记。</p></div>}
    {!!entries.length && <p className="mt-3 flex items-center gap-1.5 text-[11px] text-muted-foreground"><Pin className="size-3"/>双击笔记可固定到工作区。</p>}
  </nav>;
}
