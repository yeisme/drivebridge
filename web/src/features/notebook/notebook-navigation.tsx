import { useRef, useState, type KeyboardEvent as ReactKeyboardEvent } from "react";
import { AppDialog } from "../../components/AppDialog";
import { Input } from "../../components/ui/input";
import { Button } from "../../components/ui/button";
import { noteKey, type NoteIdentity } from "./notebook-state";

export type NotebookShortcut = "commands" | "open" | "next-tab" | "previous-tab";
export function notebookShortcut(event: KeyboardEvent): NotebookShortcut | undefined {
  if (event.defaultPrevented || event.isComposing || event.keyCode === 229 || event.altKey || !(event.metaKey || event.ctrlKey)) return;
  const target = event.target;
  if (target instanceof Element && target.closest('input, textarea, select, [contenteditable]:not([contenteditable="false"]), .cm-editor, [role="textbox"]')) return;
  if (event.key === "Tab") return event.shiftKey ? "previous-tab" : "next-tab";
  if (event.shiftKey) return;
  if (event.key.toLowerCase() === "p") return "commands";
  if (event.key.toLowerCase() === "o") return "open";
}

export interface NotebookEntry { ref: NoteIdentity; title: string; path?: string; parentFileId?: string; icon?: string }
export interface NotebookCommand { id: string; label: string; run: () => void; disabled?: boolean }

// Entries are authorized, already-loaded projections supplied by the owner
// adapter. This dialog neither fetches broader sources nor invents search hits.
export function NotebookNavigator({ mode, entries, commands, sourceId, onOpen, close }: {
  mode: "commands" | "open";
  entries: NotebookEntry[];
  commands: NotebookCommand[];
  sourceId: string;
  onOpen: (ref: NoteIdentity) => void;
  close: () => void;
}) {
  const [query, setQuery] = useState("");
  const results = useRef<HTMLUListElement>(null);
  const navigate = (event: ReactKeyboardEvent, fromSearch = false) => {
    if (event.nativeEvent.isComposing || event.nativeEvent.keyCode === 229) return;
    const buttons = Array.from(results.current?.querySelectorAll<HTMLButtonElement>('button:not(:disabled)') ?? []);
    if (!buttons.length) return;
    if (fromSearch && event.key === "Enter") { event.preventDefault(); buttons[0].click(); return; }
    if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return;
    event.preventDefault();
    const index = buttons.indexOf(document.activeElement as HTMLButtonElement);
    const next = index < 0 ? (event.key === "ArrowDown" ? 0 : buttons.length - 1) : (index + (event.key === "ArrowDown" ? 1 : -1) + buttons.length) % buttons.length;
    buttons[next].focus();
    buttons[next].scrollIntoView?.({ block: "nearest" });
  };
  const title = mode === "commands" ? "命令面板" : "快速打开";
  const matches = (value: string) => value.toLocaleLowerCase().includes(query.trim().toLocaleLowerCase());
  const items = mode === "commands"
    ? commands.filter(command => matches(command.label))
    : entries.filter(entry => entry.ref.sourceId === sourceId && matches(entry.title)).map(entry => ({
      id: noteKey(entry.ref), label: entry.title, run: () => onOpen(entry.ref), disabled: false,
    }));
  return <AppDialog title={title} close={close}>
    <p className="text-sm text-muted-foreground">{mode === "open" ? "仅当前来源已加载笔记；不是全文搜索" : "仅当前工作台可用动作"}</p>
    <Input autoFocus aria-label={title === "快速打开" ? "搜索笔记标题" : "搜索命令"} value={query} onChange={event => setQuery(event.target.value)} onKeyDown={event => navigate(event, true)} />
    <ul ref={results} onKeyDown={event => navigate(event)} className="m-0 max-h-80 list-none overflow-y-auto p-0" aria-label={`${title}结果`}>
      {items.map(item => <li key={item.id}><Button type="button" variant="ghost" className="w-full justify-start" disabled={item.disabled} onClick={() => { close(); item.run(); }}>{item.label}</Button></li>)}
    </ul>
    {!items.length && <p role="status">没有匹配结果</p>}
  </AppDialog>;
}
