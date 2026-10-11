import { useEffect, useRef, useState } from "react";
import { Link2, RefreshCw } from "lucide-react";
import { Button } from "../../components/ui/button";
import { claimDocumentSave, clearDocumentSave } from "./document-client";
import { lookupNoteLinks, readNoteLinks, rebuildNoteLinks, type NoteLinks } from "./notebook-relations";
import { noteKey, type NoteIdentity } from "./notebook-state";

export function NotebookRelations({ identity, actor }: { identity: NoteIdentity; actor: string }) {
  return <RelationsSession key={JSON.stringify([actor, noteKey(identity)])} identity={identity} actor={actor}/>;
}
function RelationsSession({ identity, actor }: { identity: NoteIdentity; actor: string }) {
  const [links, setLinks] = useState<NoteLinks>();
  const [backlinks, setBacklinks] = useState<NoteLinks>();
  const [state, setState] = useState("loading");
  const [attempt, setAttempt] = useState(0);
  const [pending, setPending] = useState("");
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");
  const controller = useRef<AbortController | null>(null);
  const running = useRef(false);
  const pageBusy = useRef(false);
  const [paging, setPaging] = useState(false);
  const slot = "drivebridge:note-links:" + JSON.stringify([actor, identity.sourceId, identity.objectId, identity.revision]);
  useEffect(() => {
    const request = new AbortController(); controller.current = request;
    setState("loading"); setLinks(undefined); setBacklinks(undefined);
    try {
      const stored = localStorage.getItem(slot);
      if (stored !== null && !/^edit-[a-f0-9]{48}$/.test(stored)) throw new Error("invalid_recovery");
      setPending(stored ?? "");
    } catch { setNotice("恢复存储不可用，不能安全重建关系索引。"); }
    const invalidate = () => { request.abort(); setLinks(undefined); setBacklinks(undefined); setState("denied"); setNotice(""); };
    window.addEventListener("drivebridge-context", invalidate);
    Promise.all([readNoteLinks(identity, false, request.signal), readNoteLinks(identity, true, request.signal)]).then(([outgoing, incoming]) => {
      if (request.signal.aborted) return;
      setLinks(outgoing); setBacklinks(incoming); setState("ready");
    }).catch(error => {
      if (request.signal.aborted) return;
      if (error.status === 401 || error.status === 403) request.abort();
      setState(error.status === 401 || error.status === 403 ? "denied" : error.status === 404 || error.status === 405 ? "unsupported" : "error");
    });
    return () => { request.abort(); window.removeEventListener("drivebridge-context", invalidate); };
  }, [identity.sourceId, identity.objectId, identity.revision, slot, attempt]);
  async function rebuild(lookup: boolean) {
    const signal = controller.current?.signal;
    if (!signal || signal.aborted || running.current) return;
    running.current = true; setBusy(true); setNotice("");
    let key = pending;
    try {
      if (!key) {
        const claim = await claimDocumentSave(slot, signal); key = claim.key;
        if (signal.aborted) return;
        setPending(key);
        if (!claim.claimed) { setNotice("已有待确认的索引请求，请查询原结果。"); return; }
      }
      const receipt = await (lookup ? lookupNoteLinks(identity, key, signal) : rebuildNoteLinks(identity, key, signal));
      if (signal.aborted) return;
      if (receipt.file_id !== identity.objectId || receipt.version_id !== identity.revision) throw new Error("relation_receipt_mismatch");
      await clearDocumentSave(slot, key);
      if (signal.aborted) return;
      setPending(""); setNotice("已确认固定版本索引；反向链接仅覆盖已建立索引的可访问笔记。"); setAttempt(value => value + 1);
    } catch (error) {
      if (signal.aborted) return;
      const status = error && typeof error === "object" && "status" in error ? error.status : undefined;
      if (status === 401 || status === 403) { controller.current?.abort(); setLinks(undefined); setBacklinks(undefined); setState("denied"); }
      else setNotice(key ? "索引结果未确认。保留原请求，只查询原结果，不自动重复重建。" : "恢复存储不可用，本次没有发送重建请求。");
    } finally { running.current = false; if (!signal.aborted) setBusy(false); }
  }
  async function more(incoming: boolean) {
    const current = incoming ? backlinks : links;
    const signal = controller.current?.signal;
    if (!current?.next_cursor || !signal || signal.aborted || pageBusy.current) return;
    pageBusy.current = true; setPaging(true);
    try {
      const next = await readNoteLinks(identity, incoming, signal, current.next_cursor);
      if (signal.aborted) return;
      const merged = { ...next, items: [...current.items, ...next.items] };
      if (incoming) setBacklinks(merged); else setLinks(merged);
    } catch (error) {
      if (signal.aborted) return;
      const status = error && typeof error === "object" && "status" in error ? error.status : undefined;
      setLinks(undefined); setBacklinks(undefined);
      if (status === 401 || status === 403) { controller.current?.abort(); setState("denied"); }
      else { setState("error"); setNotice("关系分页已失效或读取失败，请从第一页刷新；不拼接不同权限或版本的数据。"); }
    } finally { pageBusy.current = false; if (!signal.aborted) setPaging(false); }
  }
  const section = (data: NoteLinks | undefined, incoming: boolean) => <section className="space-y-2">
    <h3 className="flex items-center gap-2 text-xs font-medium text-muted-foreground"><Link2 className="size-3.5"/>{incoming ? "反向链接" : "文内链接"}</h3>
    {data && <>
      {!incoming && (!data.indexed || data.stale) && <p className="text-xs leading-5 text-muted-foreground">{!data.indexed ? "此固定版本尚未建立关系索引。" : "索引对应旧版本，请核对来源。"}</p>}
      {incoming && <p className="text-xs leading-5 text-muted-foreground">仅显示已建立索引且当前可访问的来源。</p>}
      {(incoming || data.indexed) && !data.items.length && <p className="text-xs leading-5 text-muted-foreground">{incoming ? "当前可见索引中没有反向链接，不代表全库没有关联。" : "此版本没有可访问的文内链接。"}</p>}
      <ul className="m-0 list-none space-y-2 p-0">{data.items.map((link, index) => <li key={JSON.stringify([link.file_ref, link.target_ref, link.anchor, index])} className="break-words text-xs leading-5">
        <p className="font-medium">{incoming ? link.source_path : link.target_path}</p>
        {link.anchor && <p className="text-muted-foreground">#{link.anchor}</p>}
        {link.stale && <p className="text-muted-foreground">来源版本已变化</p>}
        <details><summary className="min-h-8 cursor-pointer text-muted-foreground">固定引用</summary><code className="block break-all py-1">{incoming ? link.file_ref : link.target_ref}</code></details>
      </li>)}</ul>
      {data.next_cursor && <Button type="button" variant="ghost" className="min-h-11 w-full text-xs" disabled={paging || busy} onClick={() => void more(incoming)}>{incoming ? "更多反向链接" : "更多文内链接"}</Button>}
    </>}
  </section>;
  return <section aria-label="笔记关系能力" className="space-y-5 border-t pt-4">
    {state === "loading" && <p role="status" className="text-xs text-muted-foreground">正在读取关系索引…</p>}
    {state === "unsupported" && <p role="status" className="text-xs text-muted-foreground">当前来源暂不支持关系查询。</p>}
    {state === "denied" && <p role="alert" className="text-xs">权限或上下文已变化，关系数据已清除。</p>}
    {state === "error" && <p role="alert" className="text-xs">关系读取失败，不代表没有关联笔记。</p>}
    {section(links, false)}{section(backlinks, true)}
    {state !== "denied" && <div className="space-y-2">
      <Button type="button" variant="ghost" className="min-h-11 w-full text-xs" disabled={busy || paging || state === "loading"} onClick={() => setAttempt(value => value + 1)}><RefreshCw className="size-3.5"/>刷新关系</Button>
      {pending ? <Button type="button" variant="outline" className="min-h-11 w-full text-xs" disabled={busy || paging} onClick={() => void rebuild(true)}>查询原索引结果</Button> : state === "ready" && <Button type="button" variant="outline" className="min-h-11 w-full text-xs" disabled={busy || paging} onClick={() => void rebuild(false)}>重建此版本关系索引</Button>}
      <p className="text-xs leading-5 text-muted-foreground">重建只解析本版本 Markdown，不抓取外链、不调用模型，也不修改正文。</p>
    </div>}
    {notice && <p role="status" className="text-xs leading-5">{notice}</p>}
  </section>;
}
