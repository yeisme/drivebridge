// Deliver center (交付与讨论): fixed delivery manifests, graded receipt
// facts, and version-anchored discussion.
//
// Publishing requires an integral manifest: a missing member disables the
// publish action and surfaces the recovery path back to the original
// preparation — an incomplete set is never published. Receipt facts are
// graded (accessible/opened/bytes_served/receiver_verified/receiver_confirmed)
// and opened/bytes_served are never displayed as receiver acceptance.
// Discussion threads anchor a version; a failed post keeps the draft; the
// private note is visible to its author only and its publication is always
// previewed to the target team before any explicit thread creation.

import { useEffect, useRef, useState } from "react";
import { api, type Space } from "../api";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { Textarea } from "../components/ui/textarea";
import { CapabilityGate, flowErrorText } from "./ConsumerFlows";
import {
  createFileThread,
  getDeliveryManifest,
  getPrivateNote,
  previewNotePublication,
  publishDeliveryManifest,
  recordDeliveryReceipt,
  setPrivateNote,
  type FlowCapability,
  type NotePublicationPreview,
  type ReceiptKind,
} from "./client";
import { listFileThreads } from "../info-workspace/client";
import { publishGuard, RECEIPT_GRADE_LABELS, type DeliveryManifestView } from "./model";
import { PrepareDelivery } from './PrepareDelivery';


export function DeliverCenter({
  spaces,
  capability,
  actor,
}: {
  spaces: Space[];
  capability: FlowCapability;
  actor: string;
}) {
  const [manifest, setManifest] = useState<DeliveryManifestView | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [fileId, setFileId] = useState("");
  const [versionId, setVersionId] = useState("");
  const [reply, setReply] = useState("");
  const [note, setNote] = useState("");
  const [threadKey, setThreadKey] = useState("");
  const [posting, setPosting] = useState(false);
  // retrying the SAME draft replays it, while editing the draft is a new
  // logical operation and mints a new key (same key + different content is
  // an idempotency conflict on the server, never a silent second thread).
  const threadKeyFor = useRef("");
  const [threadNotice, setThreadNotice] = useState("");
  const [notePreview, setNotePreview] = useState<NotePublicationPreview | null>(null);
  // observation twice replays the same key.
  const receiptKeys = useRef(new Map<string, string>());
  const [receiptBusy, setReceiptBusy] = useState(false);
  const [threads, setThreads] = useState<{ thread_id: string; version_id: string; state: string }[]>([]);
  const [readAttempt, setReadAttempt] = useState(0);
  const [threadState, setThreadState] = useState<'idle' | 'loading' | 'ready' | 'error' | 'denied' | 'offline'>('idle');
  const [noteState, setNoteState] = useState<'idle' | 'loading' | 'ready' | 'error' | 'denied' | 'offline'>('idle');
  const [noteRevision, setNoteRevision] = useState<number | undefined>();
  const noteDirty = useRef(false);
  const noteGeneration = useRef(0);
  const discussionController = useRef<AbortController | null>(null);


  // Draft text stays in memory. A failed read is never an empty note/thread list.
  useEffect(() => {
    const request = new AbortController(); discussionController.current = request;
    setThreads([]); setNotePreview(null); setThreadNotice('');
    const invalidate = () => { request.abort(); setReply(''); setNote(''); setThreads([]); setNotePreview(null); setThreadKey(''); threadKeyFor.current = ''; setThreadState('denied'); setNoteState('denied'); };
    const offline = () => { request.abort(); setThreads([]); setNotePreview(null); setThreadState('offline'); setNoteState('offline'); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('offline', offline);
    if (!fileId) { setThreadState('idle'); setNoteState('idle'); }
    else if (!navigator.onLine) offline();
    else {
      setThreadState('loading'); setNoteState('loading');
      listFileThreads(fileId, request.signal).then(rows => { if (!request.signal.aborted) { setThreads(rows.map(row => ({ thread_id: row.thread_id, version_id: row.version_id, state: row.state }))); setThreadState('ready'); } }).catch(error => {
        if (request.signal.aborted) return;
        const status = error && typeof error === 'object' && 'status' in error ? error.status : undefined;
        if (status === 401 || status === 403) invalidate(); else setThreadState('error');
      });
      getPrivateNote(fileId, request.signal).then(view => { if (!request.signal.aborted) { if (!noteDirty.current) setNote(view?.body || ''); setNoteRevision(view?.revision); setNoteState('ready'); } }).catch(error => {
        if (request.signal.aborted) return;
        const status = error && typeof error === 'object' && 'status' in error ? error.status : undefined;
        if (status === 401 || status === 403) invalidate();
        else if (status === 404) { api<{ file: { id: string } }>('/files/' + encodeURIComponent(fileId), 'GET', undefined, {}, request.signal).then(value => { if (!request.signal.aborted && value.file?.id === fileId) { setNoteRevision(0); setNoteState('ready'); } }).catch(error => { if (request.signal.aborted) return; if (error?.status === 401 || error?.status === 403) invalidate(); else setNoteState('error'); }); }
        else setNoteState('error');
      });
    }
    return () => { request.abort(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', offline); };
  }, [fileId, readAttempt]);


  const publish = async () => {
    if (!manifest) return;
    const guard = publishGuard(manifest);
    if (!guard.ok) return; // The button is disabled; this is a second guard.
    setBusy(true);
    try {
      const signal = discussionController.current?.signal;
      if (!signal || signal.aborted || !navigator.onLine) return;
      const value = await publishDeliveryManifest(manifest.share_id, manifest.manifest_revision, signal);
      if (!signal.aborted) setManifest(value);
    } catch (err) {
      setError(flowErrorText(err));
    } finally {
      setBusy(false);
    }
  };

  const reloadManifest = async () => {
    if (!manifest) return;
    try {
      const signal = discussionController.current?.signal;
      if (!signal || signal.aborted || !navigator.onLine) return;
      const value = await getDeliveryManifest(manifest.share_id, manifest.manifest_revision, signal);
      if (!signal.aborted) setManifest(value);
    } catch (err) {
      setError(flowErrorText(err));
    }
  };
  const recordReceipt = async (kind: ReceiptKind) => {
    if (!manifest || receiptBusy) return;
    // Re-entrancy guard plus key reuse per (manifest revision, kind): a
    // double click or a retry after a lost response replays the SAME key so
    // the server can dedupe the observation.
    setReceiptBusy(true);
    const scope = manifest.manifest_revision + ":" + kind;
    try {
      let key = receiptKeys.current.get(scope);
      if (!key) {
        key = crypto.randomUUID();
        receiptKeys.current.set(scope, key);
      }
      await recordDeliveryReceipt({ share_id: manifest.share_id, manifest_revision: manifest.manifest_revision, kind }, key);
      receiptKeys.current.delete(scope);
      setThreadNotice(`已登记「${RECEIPT_GRADE_LABELS[kind]}」。自报的打开/传出不会被服务端升级为已验证或已确认。`);
    } catch (err) {
      setError(flowErrorText(err));
    } finally {
      setReceiptBusy(false);
    }
  };

  const postThread = async () => {
    if (posting) return;
    if (!fileId || !reply.trim()) {
      setError("请填写文件与讨论内容。");
      return;
    }
    setError("");
    setPosting(true);
    // Re-entrancy guard plus content-scoped key reuse: the key for this
    // exact draft is minted once and retried verbatim until the post
    // succeeds; editing the draft or a successful post invalidates it.
    const contentKey = fileId + "\u0000" + versionId + "\u0000" + reply;
    let key = threadKeyFor.current === contentKey ? threadKey : "";
    if (!key) {
      key = crypto.randomUUID();
      setThreadKey(key);
      threadKeyFor.current = contentKey;
    }
    try {
      const signal = discussionController.current?.signal;
      if (!signal || signal.aborted || !navigator.onLine || threadState !== 'ready') return;
      await createFileThread(fileId, { ...(versionId ? { version_id: versionId } : {}), body: reply }, key, signal);
      if (signal.aborted) return;
      setReply(''); setThreadKey(''); threadKeyFor.current = ''; setReadAttempt(value => value + 1); setThreadNotice('讨论已发布到固定版本锚点。');
    } catch (err) {
      // The draft AND its key stay in place for a retry; nothing is lost.
      setError(flowErrorText(err) + " 草稿已保留。");
    } finally {
      setPosting(false);
    }
  };
  const saveNote = async () => {
    if (!fileId) return;
    try {
      const signal = discussionController.current?.signal;
      if (!signal || signal.aborted || !navigator.onLine || noteState !== 'ready') return;
      const submittedGeneration = noteGeneration.current;
      await setPrivateNote(fileId, { body: note, ...(noteRevision === undefined ? {} : { expected_revision: noteRevision }) }, signal);
      if (signal.aborted) return;
      if (noteGeneration.current === submittedGeneration) noteDirty.current = false;
      setReadAttempt(value => value + 1);
      setThreadNotice("私人备注已保存（仅本人可见，且需保持文件权限）。");
    } catch (err) {
      setError(flowErrorText(err));
    }
  };

  const previewNote = async () => {
    if (!fileId) return;
    try {
      const signal = discussionController.current?.signal;
      if (!signal || signal.aborted || !navigator.onLine || noteState !== 'ready') return;
      const value = await previewNotePublication(fileId, signal);
      if (!signal.aborted) setNotePreview(value);
    } catch (err) {
      setError(flowErrorText(err));
    }
  };

  const guard = manifest ? publishGuard(manifest) : null;

  return (
    <CapabilityGate capability={capability} action="delivery_prepare">
      <section className="cf-center" aria-label="交付与讨论">
        {error && (
          <div className="banner error" role="alert">
            {error}
          </div>
        )}
        {threadNotice && (
          <div className="banner info" role="status">
            {threadNotice}
          </div>
        )}

        <div className="cf-card" data-region="delivery">
          <h3>固定交付清单</h3>
          <p className="cf-hint">发布仅在全部必需成员可读、摘要校验、清单提交且保留保护就绪后可用；缺项时不能发布不完整集合。</p>
          <PrepareDelivery actor={actor} spaces={spaces} onManifest={setManifest}/>

          {manifest && (
            <div data-region="manifest" data-phase={manifest.phase}>
              <dl className="cf-facts">
                <div>
                  <dt>清单版本</dt>
                  <dd>{manifest.manifest_revision}</dd>
                </div>
                <div>
                  <dt>阶段</dt>
                  <dd>{manifest.phase}</dd>
                </div>
                <div>
                  <dt>校验进度</dt>
                  <dd>
                    {manifest.verified_items} / {manifest.required_items}
                  </dd>
                </div>
                <div>
                  <dt>完整性</dt>
                  <dd>{manifest.complete ? "完整" : "缺项"}</dd>
                </div>
                <div>
                  <dt>保留保护</dt>
                  <dd>{manifest.retention_ready ? "就绪" : "未就绪"}</dd>
                </div>
              </dl>
              {guard && !guard.ok && (
                <div className="banner warn" role="alert">
                  {guard.blockers.includes("delivery_incomplete") && <p>交付缺 {manifest.required_items - manifest.verified_items} 项：不能发布不完整集合。请先恢复原准备任务补齐缺项。</p>}
                  {guard.blockers.includes("retention_not_ready") && <p>保留保护尚未就绪（retention_not_ready）：发布前需要先建立真实的保留保护。</p>}
                </div>
              )}
              <div className="cf-actions">
                <Button variant="default" type="button" disabled={!guard?.ok || busy} onClick={() => void publish()}>
                  发布完整清单
                </Button>
                <Button variant="outline" type="button" onClick={() => void reloadManifest()}>
                  刷新清单状态
                </Button>
                {(["opened", "bytes_served"] as const).map((kind) => (
                  <Button key={kind} variant="ghost" type="button" disabled={receiptBusy} onClick={() => void recordReceipt(kind)}>
                    登记「{RECEIPT_GRADE_LABELS[kind]}」
                  </Button>
                ))}
              </div>
              <p className="cf-hint">接收事实分级：{Object.values(RECEIPT_GRADE_LABELS).join(" → ")}。前两级只是观测，不等于接收确认。</p>
            </div>
          )}
        </div>

        <div className="cf-card" data-region="discussion">
          <h3>版本讨论与私人备注</h3>
          <p className="cf-hint">讨论锚定固定文件版本；旧版本讨论保留旧标记。正文为纯文本，最长 10000 字符。</p>
          <p className="cf-hint">讨论和私人备注草稿仅保留在当前页面内存，不写入浏览器存储。</p>
          {(threadState === 'loading' || noteState === 'loading') && <p role="status">正在读取当前文件的讨论与私人备注…</p>}
          {threadState === 'error' && <p role="alert">讨论读取失败，不代表没有讨论。</p>}
          {noteState === 'error' && <p role="alert">私人备注读取失败，不代表没有备注；已输入草稿保留，请重读后再保存。</p>}
          {(threadState === 'denied' || noteState === 'denied') && <p role="alert">权限或范围已变化，讨论与私人备注已清除。</p>}
          {(threadState === 'offline' || noteState === 'offline') && <p role="alert">当前离线，草稿保留在内存；联网后请手动重读，不会自动发布。</p>}
          {fileId && <Button variant="outline" className="min-h-11" disabled={threadState === 'loading' || noteState === 'loading' || threadState === 'denied'} onClick={() => setReadAttempt(value => value + 1)}>重读讨论与私人备注</Button>}
          {threadState === 'ready' && !threads.length && <p role="status">当前文件没有可见讨论。</p>}
          <div className="cf-row">
            <label className="cf-label" htmlFor="cf-file">
              文件 ID
            </label>
            <Input id="cf-file" type="text" value={fileId} onChange={event => { noteDirty.current = false; setNote(''); setReply(''); setThreadKey(''); setNoteRevision(undefined); discussionController.current?.abort(); setFileId(event.target.value); }} aria-label="文件 ID" />
            <label className="cf-label" htmlFor="cf-version">
              版本锚点
            </label>
            <Input id="cf-version" type="text" value={versionId} onChange={(event) => setVersionId(event.target.value)} placeholder="留空则使用文件级锚点" aria-label="版本锚点" />
          </div>
          <label className="cf-label" htmlFor="cf-reply">
            讨论
          </label>
          <Textarea
            id="cf-reply"
            className="cf-textarea"
            rows={3}
            maxLength={10000}
            value={reply}
            onChange={(event) => setReply(event.target.value)}
            aria-label="讨论内容"
          />
          <Button variant="default" type="button" disabled={posting || threadState !== 'ready'} onClick={() => void postThread()}>
            发布讨论
          </Button>
          {threads.length > 0 && (
            <ul className="cf-threads" aria-label="已有讨论">
              {threads.map((thread) => (
                <li key={thread.thread_id}>
                  <code>{thread.thread_id}</code> · 锚点 {thread.version_id || "文件级"} · {thread.state === "open" ? "进行中" : thread.state}
                </li>
              ))}
            </ul>
          )}
          <label className="cf-label" htmlFor="cf-note">
            私人备注（仅本人可见）
          </label>
          <Textarea
            id="cf-note"
            className="cf-textarea"
            rows={3}
            maxLength={10000}
            value={note}
            onChange={event => { noteGeneration.current++; noteDirty.current = true; setNote(event.target.value); setNotePreview(null); }}
            aria-label="私人备注"
          />
          <div className="cf-actions">
            <Button variant="outline" type="button" disabled={noteState !== 'ready'} onClick={() => void saveNote()}>
              保存私人备注
            </Button>
            <Button variant="outline" type="button" disabled={noteState !== 'ready'} onClick={() => void previewNote()}>
              公开前预览
            </Button>
          </div>
          {notePreview && (
            <div className="cf-note-preview" data-region="note-preview" role="region" aria-label="私人备注公开预览">
              <p>
                将公开到：{notePreview.target === "team_thread" ? "团队讨论" : notePreview.target}（基于备注第 {notePreview.note_revision} 次修订）
              </p>
              <p>正文预览：{notePreview.body_preview}</p>
              <p className="cf-hint">
                {notePreview.note || "预览不会产生团队消息；实际公开需要明确发布为团队讨论。"}
              </p>
            </div>
          )}
        </div>
      </section>
    </CapabilityGate>
  );
}
