import { useEffect, useMemo, useRef, useState } from "react";
import { Input } from "../../components/ui/input";
import { Button } from "../../components/ui/button";
import { AsyncState } from "../../components/AsyncState";
import type { DesktopBridge, DesktopResult } from "../../generated/desktop-ipc";
import type { components as Desktop } from "../../generated/desktop";
import type { components as Pinax } from "../../generated/pinax-reader";
import { PinaxReadingStateControls } from "./PinaxReadingStateControls";
import { PinaxSyncPanel } from "./PinaxSyncPanel";
import { createPinaxWorkbenchState, transitionPinaxWorkbench, type PinaxWorkbenchState } from "./pinax-workbench-state";

type ReaderBridge = Pick<DesktopBridge, "listNotes" | "searchNotes" | "readDocument" | "readLinks"> & Partial<Pick<DesktopBridge, "syncStatus" | "syncDiff" | "listSyncConflicts" | "readReadingState" | "updateReadingState" | "lookupReadingMutation">>;
type Note = Pinax["schemas"]["pinax.reader.note_card.v1alpha1"];
type Document = Pinax["schemas"]["pinax.reader.document.v1alpha1"];

function errorCode(result: DesktopResult<unknown>) { return "error" in result ? result.error?.code || "pinax_reader_failed" : undefined; }
function textOf(nodes: Pinax["schemas"]["pinax.reader.document_node.v1alpha1"][]): string {
  return nodes.map(node => `${node.text ?? node.payload?.text ?? node.payload?.value ?? ""}${node.children ? `\n${textOf(node.children)}` : ""}`).join("\n").trim();
}
function imagesOf(nodes: Pinax["schemas"]["pinax.reader.document_node.v1alpha1"][]): { ref: string; alt: string }[] {
  return nodes.flatMap(node => [ ...(node.type === 'image' && /^drivebridge-asset:\/\/asset\/[a-f0-9]{64}$/.test(node.ref ?? '') ? [{ ref: node.ref!, alt: node.alt || '笔记附件' }] : []), ...imagesOf(node.children ?? []) ]);
}
function attachmentStates(nodes: Pinax["schemas"]["pinax.reader.document_node.v1alpha1"][]): string[] {
  return nodes.flatMap(node => [
    ...(node.type === "image" ? [node.ref?.startsWith("drivebridge-asset://asset/") ? "图片附件已绑定当前笔记版本。" : "image: owner attachment resolution required"] : []),
    ...(node.type === "unsupported" || node.type === "fence_failed" ? [`${node.type}: owner did not provide a readable projection`] : []),
    ...(node.children ? attachmentStates(node.children) : []),
  ]);
}

export function PinaxReaderWorkspace({ bridge, sourceId, displayName = "Pinax 来源" }: { bridge: ReaderBridge; sourceId: string; displayName?: string }) {
  const bodyElement = useRef<HTMLElement>(null);
  const [scrollTarget, setScrollTarget] = useState<NonNullable<Desktop['schemas']['ReadingState']['position']>>();
  const [query, setQuery] = useState("");
  const [notes, setNotes] = useState<Note[]>([]);
  const [selected, setSelected] = useState<Note>();
  const [document, setDocument] = useState<Document>();
  const [backlinks, setBacklinks] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const [attempt, setAttempt] = useState(0);
  const [nextCursor, setNextCursor] = useState<string>();
  const [paging, setPaging] = useState(false);
  const [earlierPagesReleased, setEarlierPagesReleased] = useState(false);
  const retainedPages = useRef<Document[]>([]);
  const [workbenchState, setWorkbenchState] = useState<PinaxWorkbenchState>(() => createPinaxWorkbenchState(sourceId));
  const generation = useRef(0);
  const request = useMemo(() => query.trim(), [query]);

  useEffect(() => {
    const epoch = ++generation.current;
    setLoading(true); setError(undefined); setSelected(undefined); setDocument(undefined); setBacklinks([]); setNextCursor(undefined); setPaging(false); setWorkbenchState(createPinaxWorkbenchState(sourceId));
    const call: Promise<DesktopResult<any>> = request
      ? bridge.searchNotes({ input: { q: request, limit: 50, sort: "relevance_desc" }, source_id: sourceId })
      : bridge.listNotes({ input: { limit: 50, sort: "updated_desc" }, source_id: sourceId });
    call.then(result => {
      if (generation.current !== epoch) return;
      const code = errorCode(result);
      if (code) { setError(code); setLoading(false); return; }
      if (!("data" in result) || !Array.isArray(result.data.items)) { setError("pinax_reader_invalid_response"); setLoading(false); return; }
      setNotes(result.data.items); setNextCursor(result.data.next_cursor); setLoading(false);
    }).catch(() => { if (generation.current === epoch) { setError("pinax_reader_unavailable"); setLoading(false); } });
    return () => { generation.current++; };
  }, [attempt, bridge, request, sourceId]);

  useEffect(() => {
    if (!selected) return;
    setWorkbenchState(current => transitionPinaxWorkbench(current, { type: "read" }));
    const epoch = ++generation.current;
    setDocument(undefined); setBacklinks([]); setError(undefined); setLoading(true);
    retainedPages.current = []; setEarlierPagesReleased(false); setPaging(false);
    Promise.all([
      bridge.readDocument({ input: { object_id: selected.object_id, revision: selected.revision }, source_id: sourceId }),
      bridge.readLinks({ input: { direction: "backlinks", limit: 50, object_id: selected.object_id, revision: selected.revision }, source_id: sourceId }),
    ]).then(([documentResult, linksResult]) => {
      if (generation.current !== epoch) return;
      const documentError = errorCode(documentResult), linksError = errorCode(linksResult);
      if (documentError || linksError || !("data" in documentResult) || !("data" in linksResult)) { setError(documentError || linksError || "pinax_reader_invalid_response"); setLoading(false); return; }
      if (documentResult.data.object_id !== selected.object_id || documentResult.data.revision !== selected.revision) { setError("pinax_reader_identity_mismatch"); setLoading(false); return; }
      retainedPages.current = [documentResult.data];
      setDocument(documentResult.data); setBacklinks(linksResult.data.items.map(item => `${item.object_id}${item.anchor ? `#${item.anchor}` : ""}`)); setLoading(false);
    }).catch(() => { if (generation.current === epoch) { setError("pinax_reader_unavailable"); setLoading(false); } });
  }, [bridge, selected, sourceId]);

  async function loadMoreNotes() {
    if (!nextCursor || paging) return;
    const epoch = generation.current, cursor = nextCursor;
    setPaging(true);
    try {
      const result = request
        ? await bridge.searchNotes({ source_id: sourceId, input: { q: request, limit: 50, sort: 'relevance_desc', cursor } })
        : await bridge.listNotes({ source_id: sourceId, input: { limit: 50, sort: 'updated_desc', cursor } });
      if (epoch !== generation.current) return;
      if ('error' in result) { setError(result.error.code); return; }
      if (result.data.next_cursor === cursor) { setError('pinax_reader_invalid_response'); return; }
      setNotes(current => [...current, ...result.data.items.filter(note => !current.some(old => old.object_id === note.object_id && old.revision === note.revision))]);
      setNextCursor(result.data.next_cursor);
    } catch { if (epoch === generation.current) setError('pinax_reader_unavailable'); }
    finally { if (epoch === generation.current) setPaging(false); }
  }

  async function loadMoreDocument() {
    if (!document?.next_cursor || !selected || paging) return;
    const epoch = generation.current, cursor = document.next_cursor;
    setPaging(true);
    try {
      const result = await bridge.readDocument({ source_id: sourceId, input: { object_id: selected.object_id, revision: selected.revision, cursor } });
      if (epoch !== generation.current) return;
      if ('error' in result) { setError(result.error.code); return; }
      if (result.data.object_id !== selected.object_id || result.data.revision !== selected.revision || result.data.next_cursor === cursor) { setError('pinax_reader_identity_mismatch'); return; }
      const pages = [...retainedPages.current, result.data];
      const size = (values: Document[]) => new TextEncoder().encode(JSON.stringify(values)).byteLength;
      if (size([result.data]) > 8 * 1024 * 1024) { setError('document_budget_exceeded'); return; }
      while (pages.length > 8 || size(pages) > 8 * 1024 * 1024) { pages.shift(); setEarlierPagesReleased(true); }
      retainedPages.current = pages;
      setDocument({ ...result.data, nodes: pages.flatMap(page => page.nodes), outline: pages.flatMap(page => page.outline) });
    } catch { if (epoch === generation.current) setError('pinax_reader_unavailable'); }
    finally { if (epoch === generation.current) setPaging(false); }
  }

  function capturePosition(): NonNullable<Desktop['schemas']['ReadingState']['position']> | undefined {
    const body = bodyElement.current;
    if (!body || !selected) return;
    const top = body.getBoundingClientRect().top;
    const node = Array.from(body.querySelectorAll<HTMLElement>('[data-pinax-anchor]')).find(node => node.getBoundingClientRect().bottom > top);
    const anchor = node?.dataset.pinaxAnchor;
    if (!node || !anchor) return;
    return { revision: selected.revision, anchor, offset: Math.max(0, Math.min(8388608, Math.round(top - node.getBoundingClientRect().top))) };
  }
  async function restorePosition(position: NonNullable<Desktop['schemas']['ReadingState']['position']>) {
    if (!selected || position.revision !== selected.revision) return;
    const epoch = generation.current;
    try {
      const result = await bridge.readDocument({ source_id: sourceId, input: { object_id: selected.object_id, revision: selected.revision, anchor: position.anchor } });
      if (generation.current !== epoch) return;
      if ('error' in result) { setError(result.error.code); return; }
      if (result.data.object_id !== selected.object_id || result.data.revision !== selected.revision) { setError('pinax_reader_identity_mismatch'); return; }
      retainedPages.current = [result.data]; setDocument(result.data); setScrollTarget(position);
    } catch { if (generation.current === epoch) setError('pinax_reader_unavailable'); }
  }
  useEffect(() => {
    const body = bodyElement.current;
    if (!body || !scrollTarget || scrollTarget.revision !== selected?.revision) return;
    const node = Array.from(body.querySelectorAll<HTMLElement>('[data-pinax-anchor]')).find(node => node.dataset.pinaxAnchor === scrollTarget.anchor);
    if (node) body.scrollTop = body.scrollTop + node.getBoundingClientRect().top - body.getBoundingClientRect().top + scrollTarget.offset;
    setScrollTarget(undefined);
  }, [document, scrollTarget, selected]);
  const readingBridge = useMemo(() => bridge.readReadingState && bridge.updateReadingState && bridge.lookupReadingMutation ? { readReadingState: bridge.readReadingState, updateReadingState: bridge.updateReadingState, lookupReadingMutation: bridge.lookupReadingMutation } : undefined, [bridge]);
  const readingTarget = useMemo(() => selected ? { kind: 'pinax' as const, instance_id: 'cli', source_id: sourceId, object_id: selected.object_id, revision: selected.revision, uri: `pinax://note/${selected.object_id}?v=${selected.revision}` } : undefined, [sourceId, selected]);
  const images = document ? imagesOf(document.nodes) : [];
  const attachments = document ? attachmentStates(document.nodes) : [];
  return <section aria-label="Pinax 阅读器" className="grid min-h-0 min-w-0 gap-3 md:grid-cols-[minmax(13rem,18rem)_minmax(0,1fr)]">
    <aside className="min-w-0 space-y-2 rounded-md border p-3">
      <div aria-label="Pinax 来源绑定" className="rounded border bg-muted/30 p-2 text-xs"><strong className="font-medium">{displayName}</strong><p className="m-0 break-all text-muted-foreground">source: {sourceId}</p><p className="m-0 text-muted-foreground">只读 owner binding；页面不提供写入入口。</p></div>
      <div className="flex items-center justify-between gap-2"><h2 className="m-0 text-sm font-semibold">笔记目录</h2><Button type="button" variant="ghost" className="min-h-11 md:min-h-8" onClick={() => setAttempt(value => value + 1)}>刷新</Button></div>
      <Input aria-label="搜索 Pinax 笔记" value={query} onChange={event => setQuery(event.target.value)} placeholder="搜索标题…" className="min-h-11 md:min-h-8" />
      <AsyncState loading={loading && !notes.length} error={error} onRetry={() => setAttempt(value => value + 1)}>
        <ul className="m-0 max-h-[50vh] list-none space-y-1 overflow-auto p-0">{notes.map(note => <li key={`${note.object_id}@${note.revision}`}><Button type="button" variant="ghost" className="min-h-11 w-full justify-start text-left text-xs md:min-h-8" aria-current={selected?.object_id === note.object_id && selected.revision === note.revision ? "page" : undefined} onClick={() => setSelected(note)}><span className="truncate">{note.title_state === "present" ? note.title : "（标题不可用）"}</span></Button></li>)}</ul>
        {!notes.length && <p role="status" className="text-xs text-muted-foreground">当前来源没有可阅读笔记。</p>}
        {nextCursor && <Button disabled={paging} onClick={() => void loadMoreNotes()}>更多笔记</Button>}
      </AsyncState>
    </aside>
    <main className="min-w-0 rounded-md border p-4">
      {readingBridge && readingTarget && <PinaxReadingStateControls bridge={readingBridge} target={readingTarget} capturePosition={capturePosition} restorePosition={position => void restorePosition(position)} onRevoked={() => { generation.current++; setDocument(undefined); setSelected(undefined); setNotes([]); setError("source_unknown"); }}/>}
      {!selected && <p role="status" className="text-sm text-muted-foreground">请选择固定版本笔记。</p>}
      {selected && <><header className="border-b pb-3"><h2 className="m-0 text-lg font-semibold">{selected.title}</h2><p className="m-0 break-all text-xs text-muted-foreground">{selected.uri} · revision {selected.revision}</p></header><AsyncState loading={loading && !document} error={error} onRetry={() => setAttempt(value => value + 1)}>{document && <><article ref={bodyElement} aria-label="Pinax 固定正文" className="prose relative max-h-[60vh] max-w-none overflow-auto whitespace-pre-wrap py-4 text-sm">{document.nodes.length ? document.nodes.map((node, index) => <div key={`${node.anchor}:${index}`} data-pinax-anchor={node.anchor}>{textOf([node])}</div>) : "（正文为空）"}</article>{document.warnings?.length ? <p role="status" className="text-xs text-muted-foreground">正文包含 owner 警告，未执行不受支持的内容。</p> : null}<section aria-label="Pinax 反向链接" className="border-t pt-3"><h3 className="text-sm font-semibold">反向链接</h3>{backlinks.length ? <ul className="m-0 list-disc pl-5 text-xs">{backlinks.map(ref => <li key={ref}>{ref}</li>)}</ul> : <p className="text-xs text-muted-foreground">当前没有可见反向链接，或来源尚未提供关系。</p>}</section><section aria-label="Pinax 附件状态" className="border-t pt-3"><h3 className="text-sm font-semibold">附件与嵌入</h3>{images.map(image => <img key={image.ref} src={image.ref} alt={image.alt} loading="lazy" decoding="async" className="max-h-[60vh] max-w-full object-contain"/>)}{attachments.length ? <ul className="m-0 list-disc pl-5 text-xs">{attachments.map(item => <li key={item}>{item}</li>)}</ul> : <p className="text-xs text-muted-foreground">当前正文没有需要单独读取的附件投影。</p>}</section><p className="mt-4 text-xs text-muted-foreground">当前为只读 Pinax reader；正文、标签和来源文件不会在此页面写入。</p></>}</AsyncState></>}
      {document?.next_cursor && <Button disabled={paging} onClick={() => void loadMoreDocument()}>继续阅读</Button>}
      {earlierPagesReleased && selected && <p role="status" className="text-xs text-muted-foreground">较早页面已从内存释放，原文仍在来源中。<Button variant="link" onClick={() => setSelected({ ...selected })}>从头阅读固定版本</Button></p>}
      {bridge.syncStatus && bridge.syncDiff && bridge.listSyncConflicts && <div className="mt-4"><PinaxSyncPanel bridge={{ syncStatus: bridge.syncStatus, syncDiff: bridge.syncDiff, listSyncConflicts: bridge.listSyncConflicts }} sourceId={sourceId}/></div>}
      <p className="mt-4 text-xs text-muted-foreground">工作台状态：{workbenchState.kind}；草稿只保存在内存，恢复只使用 opaque operation key。</p>
    </main>
  </section>;
}
