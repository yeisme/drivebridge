import { useEffect, useRef, useState } from "react";
import { Button } from "../../components/ui/button";
import { AppDialog } from "../../components/AppDialog";
import { NotebookDocument } from "./NotebookDocument";
import { DocumentConflictMerge } from "./DocumentConflictMerge";
import { NotebookReadError, readMarkdownFile } from "./notebook-adapters";
import { noteKey } from "./notebook-state";
import type { NoteIdentity } from "./notebook-state";
import { claimDocumentSave, clearDocumentSave, documentCapability, documentRecoveryKey, lookupDocumentSave, saveDocument, type DocumentCapability, type DocumentSave } from "./clients";

interface Props {
  identity: NoteIdentity;
  actor?: string;
  draft?: string;
  onDraft?: (body: string) => void;
  onSaved?: (ref: NoteIdentity) => void;
  onRevoked?: () => void;
  editablePane?: boolean;
}
interface SaveIntent { ref: NoteIdentity; before: string; after: string }
interface MergeSession { baseVersion: string; base: string; ref: NoteIdentity; remote: string; originalDraft: string; maxBytes: number }
export function MarkdownFileDocument(props: Props) {
  return <FileDocumentSession key={JSON.stringify([props.actor, noteKey(props.identity)])} {...props}/>;
}
function FileDocumentSession({ identity, actor, draft, onDraft, onSaved, onRevoked, editablePane = false }: Props) {
  const [attempt, setAttempt] = useState(0);
  const [online, setOnline] = useState(navigator.onLine);
  useEffect(() => {
    const offline = () => { setOnline(false); controller.current?.abort(); submitted.current = false; setStatus("unknown"); setPreview(undefined); setMessage("当前离线，草稿保留在内存，保存已暂停；联网后请重新读取并鉴权。"); };
    window.addEventListener("offline", offline);
    return () => window.removeEventListener("offline", offline);
  }, []);
  const [result, setResult] = useState<{ body: string } | { error: NotebookReadError }>();
  const [capability, setCapability] = useState<DocumentCapability>();
  const [preview, setPreview] = useState<SaveIntent>();
  const [status, setStatus] = useState<"idle" | "saving" | "unknown" | "conflict" | "error">("idle");
  const [message, setMessage] = useState("");
  const [pendingKey, setPendingKey] = useState("");
  const [canResend, setCanResend] = useState(false);
  const [canRebase, setCanRebase] = useState(false);
  const [merge, setMerge] = useState<MergeSession>();
  const [mergeOpen, setMergeOpen] = useState(false);
  const [localDraft, setLocalDraft] = useState<string>();
  const submitted = useRef(false);
  const intent = useRef<SaveIntent | undefined>(undefined);
  const rejected = useRef(false);
  const currentDraft = useRef(draft);
  currentDraft.current = result && "error" in result ? undefined : localDraft ?? draft;
  // Keep the owner's dirty/navigation state and any cross-pane edit in sync.
  useEffect(() => { if (draft !== undefined) setLocalDraft(value => value === undefined ? value : draft); }, [draft]);
  const revokeCallback = useRef(onRevoked);
  revokeCallback.current = onRevoked;
  const controller = useRef<AbortController | null>(null);
  const recoveryKey = actor ? documentRecoveryKey(actor, identity) : "";
  const revoke = () => {
    controller.current?.abort(); intent.current = undefined; currentDraft.current = undefined; rejected.current = false;
    setPreview(undefined); setMerge(undefined); setMergeOpen(false); setLocalDraft(undefined); setCapability(undefined);
    setCanRebase(false); setCanResend(false); setPendingKey(""); setMessage("");
    setResult({ error: new NotebookReadError("forbidden", "Notebook access revoked") }); revokeCallback.current?.();
  };
  useEffect(() => {
    const request = new AbortController(); controller.current = request;
    setResult(undefined); setCapability(undefined);
    setOnline(navigator.onLine);
    if (!navigator.onLine) return () => request.abort();
    let recoveryAvailable = true;
    if (recoveryKey && editablePane) {
      try {
        const saved = localStorage.getItem(recoveryKey);
        if (saved !== null) {
          if (!/^edit-[a-f0-9]{48}$/.test(saved)) throw new Error("recovery_identity_invalid");
          setPendingKey(saved); setStatus("unknown");
        }
        if (!navigator.locks?.request) throw new Error("recovery_lock_unavailable");
      } catch { recoveryAvailable = false; setMessage("无法安全登记保存恢复身份，当前保持只读。"); }
    }
    window.addEventListener("drivebridge-context", revoke);
    readMarkdownFile(identity, request.signal).then(async body => {
      if (request.signal.aborted) return;
      setResult({ body });
      if (!editablePane || !onSaved || !actor || !recoveryAvailable) return;
      try {
        const next = await documentCapability(identity, request.signal);
        if (!request.signal.aborted && next.file_id === identity.objectId) {
          if (next.reason === "forbidden") { revoke(); return; }
          setCapability(next);
        }
      } catch (error) {
        if (!request.signal.aborted && error instanceof Error && "status" in error && (error.status === 401 || error.status === 403)) revoke();
      }
    }).catch(error => {
      if (!request.signal.aborted && !(error instanceof DOMException && error.name === "AbortError")) {
        const failure = error instanceof NotebookReadError ? error : new NotebookReadError("error", "Notebook read failed");
        if (failure.state === "forbidden") revoke();
        else setResult({ error: failure });
      }
    });
    return () => { request.abort(); intent.current = undefined; currentDraft.current = undefined; window.removeEventListener("drivebridge-context", revoke); };
  }, [identity.sourceId, identity.objectId, identity.revision, attempt, recoveryKey, editablePane, actor]);
  const release = async (key: string, signal: AbortSignal) => {
    await clearDocumentSave(recoveryKey, key);
    if (signal.aborted) return false;
    setPendingKey(""); setCanResend(false); setPreview(undefined);
    return true;
  };
  const complete = async (saved: DocumentSave, signal: AbortSignal, key: string) => {
    if (saved.file_id !== identity.objectId || saved.status !== "completed" || !saved.version_id) throw new Error("unconfirmed_save");
    const ownBody = intent.current?.after;
    if (!await release(key, signal)) return;
    setStatus("idle");
    const body = currentDraft.current;
    const dirty = body !== undefined && result && "body" in result && body !== result.body;
    if ((ownBody !== undefined && body !== ownBody) || (ownBody === undefined && dirty)) {
      // A receipt confirms one immutable intent, never the parent's newer draft.
      setCanRebase(true); setMerge(undefined); setMergeOpen(false);
      setCapability(value => value ? { ...value, version_id: saved.version_id } : value);
      setMessage("原保存已确认，但不是当前草稿。草稿保留，请读取当前版本并人工合并。");
      return;
    }
    intent.current = undefined; setMerge(undefined); setMergeOpen(false); setLocalDraft(undefined); setCanRebase(false);
    onSaved?.({ ...identity, revision: saved.version_id });
  };
  const handleFailure = (error: unknown) => {
    if (controller.current?.signal.aborted) return;
    const status = error instanceof Error && "status" in error ? error.status : undefined;
    if (status === 401 || status === 403 || (error instanceof NotebookReadError && error.state === "forbidden")) { revoke(); return; }
    setStatus(status === 409 ? "conflict" : "unknown");
    setMessage(status === 409 ? "文件版本已变化，草稿保留；请回查原保存结果。不要直接覆盖新版本。" : "保存结果尚未确认，请查询原操作，不能创建新的保存请求。");
  };
  const submit = async (next: SaveIntent | undefined = intent.current ?? preview) => {
    const signal = controller.current?.signal;
    if (!next || !signal || signal.aborted || !recoveryKey || submitted.current) return;
    submitted.current = true;
    let key = pendingKey;
    if (!key) {
      try {
        const claim = await claimDocumentSave(recoveryKey, signal);
        key = claim.key;
        if (signal.aborted) { submitted.current = false; return; }
        setPendingKey(key); setCanRebase(false);
        if (!claim.claimed) {
          intent.current = undefined; rejected.current = false;
          submitted.current = false; setPreview(undefined); setCanResend(false); setStatus("unknown");
          setMessage("另一个标签页已有待确认的保存，请先查询原保存结果。"); return;
        }
        intent.current = next; rejected.current = false;
      } catch {
        submitted.current = false;
        if (!signal.aborted) { setMessage("无法安全登记恢复身份，本次未发送正文。请检查浏览器权限后重试。"); setStatus("error"); }
        return;
      }
    }
    setStatus("saving"); setCanResend(false); setPreview(undefined);
    try { const saved = await saveDocument(next.ref, next.after, key, signal); if (!signal.aborted) await complete(saved, signal, key); }
    catch (error) {
      // Only this exact deterministic CAS error plus original-key lookup absence
      // proves rejection. A transport failure or unrelated 409 never does.
      if (!signal.aborted && error instanceof Error && "status" in error && error.status === 409 && error.message === "version_conflict") rejected.current = true;
      handleFailure(error);
    }
    finally { submitted.current = false; }
  };
  const recover = async () => {
    const signal = controller.current?.signal;
    if (!signal || signal.aborted || !pendingKey || submitted.current) return;
    submitted.current = true; setStatus("saving"); setCanResend(false);
    try {
      const found = await lookupDocumentSave(identity, pendingKey, signal);
      if (signal.aborted) return;
      if (found.found && found.save?.file_id !== identity.objectId) throw new Error("unconfirmed_save");
      if (found.found && found.save?.status === "completed") { await complete(found.save, signal, pendingKey); return; }
      if ((found.found && found.save?.status === "cancelled") || (!found.found && rejected.current && intent.current)) {
        if (!await release(pendingKey, signal)) return;
        setStatus("conflict"); setCanRebase(true); setMerge(undefined);
        setMessage("原保存已回查，确认没有发布新版本。草稿保留，可以读取当前版本并人工合并。"); return;
      }
      setStatus("unknown");
      setCanResend(Boolean(intent.current) && !rejected.current && (!found.found || ["awaiting_upload", "transferring"].includes(found.save?.status || "")));
      setMessage(found.found ? "原保存尚未完成；仅能继续原请求，或在任务中心查看原操作。" : "尚未找到原保存记录；保留原恢复身份，不自动重复提交。");
    } catch (error) { handleFailure(error); }
    finally { submitted.current = false; }
  };
  const loadMerge = async () => {
    const signal = controller.current?.signal;
    if (!signal || signal.aborted || !canRebase || pendingKey || submitted.current || !result || !("body" in result)) return;
    submitted.current = true; setStatus("saving");
    try {
      const head = await documentCapability(identity, signal);
      if (signal.aborted) return;
      if (head.reason === "forbidden") { revoke(); return; }
      if (head.file_id !== identity.objectId || !head.version_id || !head.editable) throw new Error("head_not_editable");
      const ref = { ...identity, revision: head.version_id };
      const remote = await readMarkdownFile(ref, signal);
      if (signal.aborted) return;
      const body = currentDraft.current ?? result.body;
      setCapability(head); setLocalDraft(body);
      setMerge({ baseVersion: intent.current?.ref.revision ?? identity.revision, base: intent.current?.before ?? result.body, ref, remote, originalDraft: body, maxBytes: head.max_bytes });
      setMergeOpen(true); setStatus("idle"); setMessage("");
    } catch (error) {
      if (signal.aborted) return;
      if ((error instanceof NotebookReadError && error.state === "forbidden") || (error instanceof Error && "status" in error && (error.status === 401 || error.status === 403))) revoke();
      else { setStatus("error"); setMessage("无法读取可编辑的当前固定版本，草稿保留；请重试，不会发送新的保存。"); }
    } finally { submitted.current = false; }
  };
  const reauthenticate = async () => {
    if (!navigator.onLine || submitted.current) return;
    const request = new AbortController(); controller.current = request; submitted.current = true;
    try {
      const next = await documentCapability(identity, request.signal);
      if (request.signal.aborted) return;
      if (next.file_id !== identity.objectId || next.reason === "forbidden") { revoke(); return; }
      await readMarkdownFile(identity, request.signal);
      if (request.signal.aborted) return;
      setCapability(next); setOnline(true); setStatus(pendingKey ? "unknown" : "idle"); setMessage(pendingKey ? "已重新鉴权，草稿和原保存身份保留；请查询原保存结果。" : "已重新鉴权，内存草稿保留，请预览后保存。");
    } catch (error) { if (!request.signal.aborted) handleFailure(error); } finally { submitted.current = false; }
  };
  if (!result) return online
    ? <p role="status" className="p-4">正在读取固定版本…</p>
    : <div role="alert" className="p-4"><p>当前离线，固定版本读取已暂停；联网后请手动重新读取。</p><Button type="button" variant="outline" className="min-h-11" onClick={() => { if (navigator.onLine) setAttempt(value => value + 1); }}>重新读取固定版本</Button></div>;
  if ("error" in result) {
    const messages = { forbidden: "访问权限已失效，正文已清除。", stale: "此固定版本不可用，请返回文件列表重新选择。", unsupported: "当前来源或正文大小不支持在线阅读。", error: "无法读取正文，请重试。" };
    return <div role="alert" className="p-4"><p>{messages[result.error.state]}</p>{result.error.state === "error" && <Button type="button" variant="outline" onClick={() => setAttempt(value => value + 1)}>重试</Button>}</div>;
  }
  const value = localDraft ?? draft;
  const historical = Boolean(capability && capability.version_id !== identity.revision);
  const editable = Boolean(online && editablePane && capability?.editable && !historical && onDraft && onSaved && !pendingKey && !canRebase && status !== "saving");
  const dirty = value !== undefined && value !== result.body;
  return <>
    {!online && <Button type="button" variant="outline" className="min-h-11" onClick={() => void reauthenticate()}>保留草稿并重新鉴权</Button>}
    {pendingKey && <div role="status" className="flex flex-wrap items-center gap-2 border-b p-2 text-sm"><span>{message || "有一个待确认的保存请求。"}</span><Button type="button" variant="outline" disabled={status === "saving"} onClick={() => void recover()}>查询原保存结果</Button>{canResend && <Button type="button" variant="outline" onClick={() => void submit()}>继续原保存</Button>}</div>}
    {message && !pendingKey && <p role="alert" className="p-2 text-sm">{message}</p>}
    {canRebase && !pendingKey && <div className="border-b p-2"><Button type="button" variant="outline" disabled={status === "saving"} onClick={() => void loadMerge()}>读取当前版本并人工合并</Button></div>}
    <NotebookDocument identity={identity} body={result.body} draft={value} historical={historical} editable={editable}
      readonlyMessage={pendingKey ? "原保存结果待确认，暂时锁定编辑；内存草稿保留。" : canRebase ? "草稿尚未保存，请在人工合并窗口审阅新基线。" : !editablePane && actor ? "此窗格只读，编辑请使用主窗格。" : undefined}
      maxBytes={capability?.max_bytes ?? 2 * 1024 * 1024} onDraft={onDraft ?? (() => {})}
      actions={editablePane && <Button type="button" variant="outline" disabled={!editable || !dirty || new TextEncoder().encode(value ?? "").byteLength > (capability?.max_bytes ?? 0)} onClick={() => { setMessage(""); setPreview({ ref: identity, before: result.body, after: value! }); }}>预览保存</Button>}/>
    {preview && !pendingKey && <AppDialog title="确认文档修改" close={() => setPreview(undefined)}>
      <p className="text-sm text-muted-foreground">保存将创建新版本，旧版本保持不变。服务端会再次核对原版本与权限。</p>
      <div className="grid max-h-[55dvh] gap-3 overflow-auto md:grid-cols-2">
        <section><h3 className="text-sm font-medium">保存前</h3><pre className="whitespace-pre-wrap break-words rounded-md border border-border p-3 text-xs">{preview.before}</pre></section>
        <section><h3 className="text-sm font-medium">保存后</h3><pre className="whitespace-pre-wrap break-words rounded-md border border-border p-3 text-xs">{preview.after}</pre></section>
      </div>
      <div className="mt-4 flex justify-end gap-2"><Button type="button" variant="outline" onClick={() => setPreview(undefined)}>取消</Button><Button type="button" disabled={status === "saving"} onClick={() => void submit(preview)}>确认保存新版本</Button></div>
    </AppDialog>}
    {merge && mergeOpen && canRebase && !pendingKey && <DocumentConflictMerge baseVersion={merge.baseVersion} base={merge.base} remoteVersion={merge.ref.revision} remote={merge.remote} originalDraft={merge.originalDraft} draft={value ?? result.body} maxBytes={merge.maxBytes}
      onDraft={body => { currentDraft.current = body; setLocalDraft(body); onDraft?.(body); }} onClose={() => setMergeOpen(false)}
      onConfirm={() => { const body = currentDraft.current ?? result.body; if (new TextEncoder().encode(body).byteLength > merge.maxBytes) return; setMergeOpen(false); void submit({ ref: merge.ref, before: merge.remote, after: body }); }}/>}
  </>;
}
