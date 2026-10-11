import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { Annotation, Compartment, EditorState, Transaction } from "@codemirror/state";
import { EditorView, keymap } from "@codemirror/view";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { markdown } from "@codemirror/lang-markdown";
import { searchKeymap } from "@codemirror/search";
import { Button } from "../../components/ui/button";
import { NotebookRenderer } from "./notebook-renderer";
import { NotebookRenderBoundary } from "./NotebookRenderBoundary";
import { noteKey, type NoteIdentity } from "./notebook-state";

const externalChange = Annotation.define<boolean>();
export type NotebookMode = "reading" | "live" | "source";
interface Props {
  identity: NoteIdentity;
  body: string;
  draft?: string;
  historical: boolean;
  editable: boolean;
  maxBytes?: number;
  readonlyMessage?: string;
  actions?: React.ReactNode;
  onDraft: (body: string) => void;
}
export function NotebookDocument(props: Props) {
  return <DocumentSession key={noteKey(props.identity)} {...props}/>;
}
function DocumentSession({ body, draft, historical, editable, maxBytes = 64 * 1024, readonlyMessage, actions, onDraft }: Props) {
  const [mode, setMode] = useState<NotebookMode>("reading");
  const value = draft ?? body;
  const tooLarge = new TextEncoder().encode(value).byteLength > maxBytes;
  const readonly = historical || !editable || tooLarge;
  return <section className="flex min-h-full min-w-0 flex-col bg-card" aria-label="Markdown 文档">
    <div role="group" aria-label="文档模式" className="sticky top-0 z-10 flex min-h-11 shrink-0 flex-wrap items-center gap-1 border-b bg-card px-2 max-md:[&_button]:min-h-11 max-md:[&_button]:px-2">
      {([['reading', '阅读'], ['live', '实时预览'], ['source', '源码']] as const).map(([id, label]) => <Button key={id} type="button" variant="ghost" className="text-[13px] aria-pressed:bg-accent aria-pressed:font-semibold" aria-pressed={mode === id} onClick={() => setMode(id)}>{label}</Button>)}
      {actions && <div className="ml-auto flex items-center gap-1 pr-2">{actions}</div>}
    </div>
    {readonly && <p role="status" className="m-0 border-b px-3 py-2 text-xs leading-5 text-muted-foreground">{readonlyMessage ?? (historical ? "历史版本只读" : tooLarge ? `正文超过 ${maxBytes / 1024}KiB 编辑预算，当前只读` : "当前来源未授予编辑能力")}</p>}
    <div className="flex min-w-0 flex-1 flex-col gap-5 px-4 pb-16 pt-5 md:px-10 md:pt-6">
      <div hidden={mode === "reading"} className="mx-auto w-full max-w-[var(--notebook-line-width,720px)]"><NotebookSource value={value} readonly={readonly} onChange={onDraft}/></div>
      <div hidden={mode === "source"}><NotebookRenderBoundary body={value}><NotebookRenderer body={value}/></NotebookRenderBoundary></div>
    </div>
  </section>;
}

export function NotebookSource({ value, readonly, onChange }: { value: string; readonly: boolean; onChange: (body: string) => void }) {
  const host = useRef<HTMLDivElement>(null);
  const editor = useRef<EditorView | null>(null);
  const callback = useRef(onChange);
  callback.current = onChange;
  const access = useRef(new Compartment());
  useLayoutEffect(() => {
    if (!host.current) return;
    const view = new EditorView({ parent: host.current, state: EditorState.create({ doc: value, extensions: [
      history(), markdown(), keymap.of([...defaultKeymap, ...historyKeymap, ...searchKeymap]), EditorView.lineWrapping,
      access.current.of(EditorState.readOnly.of(readonly)),
      EditorView.contentAttributes.of({ "aria-label": "Markdown 源码", "aria-multiline": "true" }),
      EditorView.updateListener.of(update => {
        if (update.docChanged && !update.transactions.some(transaction => transaction.annotation(externalChange))) callback.current(update.state.doc.toString());
      }),
    ] }) });
    editor.current = view;
    return () => { editor.current = null; view.destroy(); };
  }, []);
  useEffect(() => { editor.current?.dispatch({ effects: access.current.reconfigure(EditorState.readOnly.of(readonly)) }); }, [readonly]);
  useEffect(() => {
    const view = editor.current;
    if (!view || view.state.doc.toString() === value) return;
    // Cross-pane updates are not a new local undo step. Preserve unaffected
    // selection offsets by replacing only the changed span.
    const old = view.state.doc.toString();
    let start = 0, end = 0;
    while (start < old.length && start < value.length && old[start] === value[start]) start++;
    while (end < old.length - start && end < value.length - start && old[old.length - 1 - end] === value[value.length - 1 - end]) end++;
    view.dispatch({ changes: { from: start, to: old.length - end, insert: value.slice(start, value.length - end) }, annotations: [externalChange.of(true), Transaction.addToHistory.of(false)] });
  }, [value]);
  return <div ref={host} className="min-w-0 rounded border bg-card text-foreground [&_.cm-cursor]:border-foreground [&_.cm-content]:caret-foreground"/>;
}
