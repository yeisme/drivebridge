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
import { SelectAdapter } from "../components/SelectAdapter";
import { CapabilityGate, flowErrorText } from "./ConsumerFlows";
import { clearActorDraft, readActorDraft, writeActorDraft } from "./compat";
import {
  createFileThread,
  getDeliveryManifest,
  getPrivateNote,
  prepareDeliveryManifest,
  previewNotePublication,
  publishDeliveryManifest,
  recordDeliveryReceipt,
  setPrivateNote,
  type FlowCapability,
  type NotePublicationPreview,
  type ReceiptKind,
} from "./client";
import { listFileThreads } from "../info-workspace/client";
import { publishGuard, RECEIPT_GRADE_LABELS, type DeliveryManifestView, type ReceiptGrade } from "./model";

interface ShareRow {
  id: string;
  name: string;
  revoked: boolean;
}

export function DeliverCenter({
  spaces,
  capability,
  actor,
}: {
  spaces: Space[];
  capability: FlowCapability;
  actor: string;
}) {
  const [shares, setShares] = useState<ShareRow[]>([]);
  const [shareId, setShareId] = useState("");
  const [shareRevision, setShareRevision] = useState(1);
  const [manifest, setManifest] = useState<DeliveryManifestView | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [fileId, setFileId] = useState("");
  const [versionId, setVersionId] = useState("");
  const [reply, setReply] = useState("");
  const [note, setNote] = useState("");
  // The idempotency key minted for the CURRENT unsent reply. It persists
  // with the draft so a retry after a failure or lost response replays the
  // same key (server-side key+fingerprint dedupe) instead of minting a new
  // one that would duplicate the thread.
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

  useEffect(() => {
    api<ShareRow[]>("/shares")
      .then((rows) => setShares(rows.filter((row) => !row.revoked)))
      .catch((err) => setError(flowErrorText(err)));
  }, []);

  // Draft recovery: an unsent reply or note survives a failed post (and a
  // reload) per the discussion contract; it is UI-only state. The unsent
  // reply's idempotency key travels with it so a retry after reload still
  // replays the same key.
  useEffect(() => {
    const draft = readActorDraft(localStorage, actor);
    if (draft) {
      setFileId(draft.file_id);
      setVersionId(draft.version_id);
      setReply(draft.reply);
      setNote(draft.note);
      if (draft.thread_key) setThreadKey(draft.thread_key);
    }
  }, []);

  useEffect(() => {
    if (fileId) {
      writeActorDraft(localStorage, actor, {
        version: 2,
        file_id: fileId,
        version_id: versionId,
        reply,
        note,
        saved_at: new Date().toISOString(),
        ...(threadKey ? { thread_key: threadKey } : {}),
      });
    }
  }, [fileId, versionId, reply, note, threadKey]);

  useEffect(() => {
    if (!fileId) return;
    listFileThreads(fileId)
      .then((rows) => setThreads(rows.map((row) => ({ thread_id: row.thread_id, version_id: row.version_id, state: row.state }))))
      .catch(() => setThreads([]));
    void getPrivateNote(fileId)
      .then((view) => {
        if (view) setNote(view.body);
      })
      .catch(() => undefined);
  }, [fileId]);

  const prepare = async () => {
    if (!shareId) return;
    setBusy(true);
    setError("");
    try {
      const key = crypto.randomUUID();
      const view = await prepareDeliveryManifest(shareId, { snapshot_id: "share-current", expected_share_revision: shareRevision }, key);
      setManifest(view);
    } catch (err) {
      setError(flowErrorText(err));
    } finally {
      setBusy(false);
    }
  };

  const publish = async () => {
    if (!manifest) return;
    const guard = publishGuard(manifest);
    if (!guard.ok) return; // The button is disabled; this is a second guard.
    setBusy(true);
    try {
      setManifest(await publishDeliveryManifest(manifest.share_id, manifest.manifest_revision));
    } catch (err) {
      setError(flowErrorText(err));
    } finally {
      setBusy(false);
    }
  };

  const reloadManifest = async () => {
    if (!manifest) return;
    try {
      setManifest(await getDeliveryManifest(manifest.share_id, manifest.manifest_revision));
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
      await createFileThread(fileId, { ...(versionId ? { version_id: versionId } : {}), body: reply }, key);
      setReply("");
      setThreadKey("");
      threadKeyFor.current = "";
      clearActorDraft(localStorage, actor);
      setThreadNotice("讨论已发布到固定版本锚点。");
      const rows = await listFileThreads(fileId);
      setThreads(rows.map((row) => ({ thread_id: row.thread_id, version_id: row.version_id, state: row.state })));
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
      await setPrivateNote(fileId, { body: note });
      setThreadNotice("私人备注已保存（仅本人可见，且需保持文件权限）。");
    } catch (err) {
      setError(flowErrorText(err));
    }
  };

  const previewNote = async () => {
    if (!fileId) return;
    try {
      setNotePreview(await previewNotePublication(fileId));
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
          <div className="cf-row">
            <label className="cf-label" htmlFor="cf-share">
              分享
            </label>
            <SelectAdapter id="cf-share" value={shareId} onChange={(event) => setShareId(event.target.value)} aria-label="选择分享">
              <option value="">选择分享…</option>
              {shares.map((row) => (
                <option key={row.id} value={row.id}>
                  {row.name}
                </option>
              ))}
            </SelectAdapter>
            <label className="cf-label" htmlFor="cf-share-rev">
              期望分享版本
            </label>
            <Input
              id="cf-share-rev"
              type="number"
              min={1}
              value={shareRevision}
              onChange={(event) => setShareRevision(Number(event.target.value) || 1)}
              aria-label="期望分享版本"
            />
            <Button variant="default" type="button" disabled={!shareId || busy} onClick={() => void prepare()}>
              {busy ? "准备中…" : "准备清单"}
            </Button>
          </div>

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
          <div className="cf-row">
            <label className="cf-label" htmlFor="cf-file">
              文件 ID
            </label>
            <Input id="cf-file" type="text" value={fileId} onChange={(event) => setFileId(event.target.value)} aria-label="文件 ID" />
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
          <Button variant="default" type="button" disabled={posting} onClick={() => void postThread()}>
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
            onChange={(event) => setNote(event.target.value)}
            aria-label="私人备注"
          />
          <div className="cf-actions">
            <Button variant="outline" type="button" onClick={() => void saveNote()}>
              保存私人备注
            </Button>
            <Button variant="outline" type="button" onClick={() => void previewNote()}>
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
