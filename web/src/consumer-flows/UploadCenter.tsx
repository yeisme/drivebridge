// Upload center (上传中心): batch upload with durable recovery identity.
//
// The interaction follows the frozen upload-batch-recovery contract:
//  1. selections are enumerated (directory support is progressive — the
//     fallback states plainly that directory structure is not preserved);
//  2. SHA-256 digests are computed per file before anything is sent;
//  3. a non-secret idempotency key is minted and persisted to the recovery
//     store BEFORE the create request leaves, so a lost create response is
//     resolved by POST /operations/lookup with the original key;
//  4. items are added in pages of ≤500 and the batch is sealed;
//  5. progress separates digest, sent, durable, verified and published
//     facts — 100% network-sent is displayed as 验证中, never 已完成.
//
// Recovery after a refresh re-reads the original operation by its original
// ID, asks the user to re-select the same files (handles do not survive a
// refresh) and verifies size + SHA-256 per item; a mismatch is
// content_mismatch with verified progress kept, never a silent re-upload.

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { sha256 } from "@noble/hashes/sha2.js";
import { bytes, getDataContext, type UserIdentity } from "../api";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { Table } from "../components/ui/table";
import { CapabilityGate, flowErrorText } from "./ConsumerFlows";
import type { RecoveryStore } from "./compat";
import {
  addUploadBatchItems,
  controlUploadBatch,
  createUploadBatch,
  getUploadBatch,
  lookupOperation,
  sealUploadBatch,
  type BatchControl,
  type BatchItemInput,
  type FlowCapability,
} from "./client";
import {
  decideAfterLookup,
  directoryInputSupport,
  UPLOAD_PHASE_LABELS,
  checkReselectedSource,
  selectedEntries,
  validRelativePath,
  type LookupDecision,
  type RecoveryIdentity,
  type RecoveryRecord,
  type SelectedEntry,
  type UploadBatchItem,
  type UploadBatchView,
  type UploadProgressFact,
  uploadDisplayPhase,
} from "./model";

const DIGEST_CHUNK = 8 << 20; // Same chunking as the preserved single-file path.

async function digestFile(file: File): Promise<string> {
  const hash = sha256.create();
  for (let offset = 0; offset < file.size; offset += DIGEST_CHUNK) {
    // Blob.arrayBuffer is missing in some runtimes (older jsdom); reading
    // through Response keeps the chunked path portable.
    const chunk = new Uint8Array(await new Response(file.slice(offset, offset + DIGEST_CHUNK)).arrayBuffer());
    hash.update(chunk);
  }
  return Array.from(hash.digest(), (b) => b.toString(16).padStart(2, "0")).join("");
}

type Screen =
  | { kind: "select" }
  | { kind: "digesting"; done: number; total: number }
  | { kind: "creating" }
  | { kind: "transferring"; batch: UploadBatchView }
  | { kind: "resuming"; decision: LookupDecision; record: RecoveryRecord }
  | { kind: "content_mismatch"; record: RecoveryRecord; mismatched: string[] };

export function UploadCenter({
  user,
  space,
  capability,
  store,
}: {
  user: UserIdentity;
  space: string;
  capability: FlowCapability;
  store: RecoveryStore;
}) {
  const support = useMemo(directoryInputSupport, []);
  const [screen, setScreen] = useState<Screen>({ kind: "select" });
  const [entries, setEntries] = useState<SelectedEntry[]>([]);
  const [records, setRecords] = useState<RecoveryRecord[]>([]);
  const [items, setItems] = useState<UploadBatchItem[]>([]);
  const [error, setError] = useState<string>("");
  const [notice, setNotice] = useState<string>("");
  const [fact, setFact] = useState<UploadProgressFact | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const controllerRef = useRef<AbortController | null>(null);

  const identity: RecoveryIdentity = { instance: location.origin, actor: user.id, space };

  const reloadRecords = useCallback(async () => {
    setRecords(await store.list(identity));
  }, [store, identity.instance, identity.actor, identity.space]);

  useEffect(() => {
    void reloadRecords();
  }, [reloadRecords]);

  // Identity switch cleanup: records of another actor/space are unreachable by
  // scope, and clearing the display cache on unmount keeps this page from
  // showing another identity's recovery rows after logout.
  useEffect(() => {
    return () => {
      controllerRef.current?.abort();
    };
  }, []);

  const onFiles = (files: File[]) => {
    setError("");
    const next = selectedEntries(files, support);
    const invalid = next.filter((entry) => !validRelativePath(entry.relative_path));
    if (invalid.length) {
      setError("选择的路径包含绝对路径或“..”，请调整后再上传。");
      return;
    }
    // A digest completion must not kick the recovery flow back to the plain
    // selection screen: the user is re-selecting originals to resume.
    const wasResuming = screen.kind === "resuming";
    const resumed = screen.kind === "resuming" ? screen : null;
    setEntries(next);
    setScreen({ kind: "digesting", done: 0, total: next.length });
    void (async () => {
      // Digest every file before any request: item keys bind the relative
      // path to the source fingerprint, so a later re-selection can be
      // verified instead of trusted by name.
      const digested: SelectedEntry[] = [];
      for (const entry of next) {
        const sha = await digestFile(entry.file);
        digested.push({ ...entry, sha256: sha });
        setScreen({ kind: "digesting", done: digested.length, total: next.length });
      }
      setEntries(digested);
      setScreen(wasResuming && resumed ? resumed : { kind: "select" });
      setNotice(wasResuming ? "已重新计算摘要，请校验并继续原任务。" : `已为 ${digested.length} 个文件计算摘要，可以创建批次。`);
    })();
  };

  // Drop handler with directory enumeration: DataTransferItem entries are
  // walked so empty directories can be uploaded as explicit entries when the
  // browser supports it (progressive enhancement, honest fallback otherwise).
  const onDrop = async (event: React.DragEvent) => {
    event.preventDefault();
    const files: File[] = [];
    const explicitDirs: string[] = [];
    const walk = async (entry: unknown, prefix: string): Promise<void> => {
      if (!entry || typeof entry !== "object") return;
      const item = entry as { isFile?: boolean; isDirectory?: boolean; name?: string; file?: (cb: (f: File) => void) => void; createReader?: () => { readEntries: (cb: (entries: unknown[]) => void) => void } };
      if (item.isFile && item.file && typeof item.name === "string") {
        await new Promise<void>((resolve) => {
          item.file!((f) => {
            const withPath = Object.defineProperty(f, "webkitRelativePath", { value: prefix + item.name, configurable: true });
            files.push(withPath);
            resolve();
          });
        });
      } else if (item.isDirectory && item.createReader && typeof item.name === "string") {
        const reader = item.createReader();
        const children = await new Promise<unknown[]>((resolve) => reader.readEntries(resolve));
        if (children.length === 0) explicitDirs.push(prefix + item.name + "/");
        for (const child of children) await walk(child, prefix + item.name + "/");
      }
    };
    const transfer = event.dataTransfer;
    if (transfer && typeof transfer.items !== "undefined" && transfer.items[0] && "webkitGetAsEntry" in transfer.items[0]) {
      for (const item of Array.from(transfer.items)) {
        await walk((item as DataTransferItem & { webkitGetAsEntry: () => unknown }).webkitGetAsEntry(), "");
      }
      for (const dir of explicitDirs) {
        // Empty directories upload as explicit placeholder entries.
        files.push(new File([], dir.split("/").slice(-2, -1)[0] || dir) as File & { webkitRelativePath?: string });
        (files[files.length - 1] as File & { webkitRelativePath?: string }).webkitRelativePath = dir;
      }
      onFiles(files);
    } else if (transfer) {
      onFiles(Array.from(transfer.files));
    }
  };

  const persistBeforeCreate = async (key: string, originalRequest: Record<string, unknown>): Promise<RecoveryRecord> => {
    const record: RecoveryRecord = {
      version: 2,
      identity,
      action: "upload_batch_create",
      idempotency_key: key,
      original_request: originalRequest,
      saved_at: new Date().toISOString(),
    };
    await store.put(record);
    await reloadRecords();
    return record;
  };

  const createBatch = async () => {
    setError("");
    const ready = entries.filter((entry) => entry.sha256);
    if (!ready.length || !space) {
      setError("请先选择文件并等待摘要完成。");
      return;
    }
    setScreen({ kind: "creating" });
    // The key is minted once per user intent and persisted BEFORE the
    // request: a lost response is recoverable by lookup, and the same key
    // must never be reused for different content.
    const key = crypto.randomUUID();
    const originalRequest = { space_id: space, conflict_policy: "fail" };
    const record = await persistBeforeCreate(key, originalRequest);
    try {
      const batch = await createUploadBatch(originalRequest, key);
      // The batch id joins the persisted identity so a later refresh can
      // read the original batch without waiting for a lookup round trip.
      await store.put({ ...record, batch_id: batch.batch_id });
      await reloadRecords();
      const inputs: BatchItemInput[] = ready.map((entry) => ({
        relative_path: entry.relative_path,
        size_bytes: entry.file.size,
        sha256: entry.sha256!,
        item_key: entry.relative_path + ":" + entry.sha256,
      }));
      await addUploadBatchItems(batch.batch_id, inputs);
      const sealed = await sealUploadBatch(batch.batch_id);
      const page = await getUploadBatch(batch.batch_id, { limit: 50 });
      setItems(page.items);
      setFact({
        bytes_total: ready.reduce((sum, entry) => sum + entry.file.size, 0),
        digest_computed: true,
        bytes_sent: 0,
        bytes_durable: 0,
        bytes_verified: 0,
        published: false,
      });
      setScreen({ kind: "transferring", batch: sealed });
    } catch (err) {
      setScreen({ kind: "select" });
      setError(flowErrorText(err));
    }
  };

  const control = async (action: BatchControl) => {
    if (screen.kind !== "transferring") return;
    try {
      const batch = await controlUploadBatch(screen.batch.batch_id, action);
      setScreen({ kind: "transferring", batch });
    } catch (err) {
      setError(flowErrorText(err));
    }
  };

  const resume = async (record: RecoveryRecord) => {
    setError("");
    try {
      const lookup = await lookupOperation(record.action, record.idempotency_key, record.original_request);
      const decision = decideAfterLookup(lookup);
      if (decision.kind === "stop") {
        setError(flowErrorText(new Error("idempotency_conflict")));
        return;
      }
      // File handles do not survive a refresh: ask the user to re-select the
      // same files, then verify size + SHA-256 against the persisted server
      // fingerprints (source_reselection_required).
      // The lookup decision may carry the original batch id even when the
      // local record predates the create response; merge it so verification
      // reads the original batch, never a re-keyed one.
      const merged: RecoveryRecord =
        decision.kind === "read_original_operation" && decision.batch_id && !record.batch_id
          ? { ...record, batch_id: decision.batch_id }
          : record;
      setScreen({ kind: "resuming", decision, record: merged });
    } catch (err) {
      setError(flowErrorText(err));
    }
  };

  const verifyReselection = async (record: RecoveryRecord) => {
    // Digest every freshly selected file first: the comparison is against
    // the SERVER's persisted fingerprints for the original batch, never
    // against the new selection itself.
    const checked: SelectedEntry[] = [];
    for (const entry of entries) {
      const sha = entry.sha256 ?? (await digestFile(entry.file));
      checked.push({ ...entry, sha256: sha });
    }
    setEntries(checked);
    const mismatched: string[] = [];
    try {
      if (record.batch_id) {
        const page = await getUploadBatch(record.batch_id, { limit: 200 });
        setItems(page.items);
        const fingerprints = new Map(page.items.map((item) => [item.relative_path, item.source_fingerprint]));
        for (const entry of checked) {
          if (!entry.sha256) continue;
          const expected = fingerprints.get(entry.relative_path);
          if (expected && checkReselectedSource({ sha256: expected }, { size: entry.file.size, sha256: entry.sha256 }) !== "match")
            mismatched.push(entry.relative_path);
        }
        if (mismatched.length) {
          // Verified progress is kept; only a genuine re-selection of the
          // original files can continue.
          setScreen({ kind: "content_mismatch", record, mismatched });
          return;
        }
        setScreen({ kind: "transferring", batch: page.batch });
      } else if (screen.kind === "resuming" && screen.decision.kind === "replay_same_key") {
        // found=false: the only legal retry is the SAME key with the SAME
        // original_request — never a new key.
        const batch = await createUploadBatch(record.original_request as { space_id: string; conflict_policy: string }, record.idempotency_key);
        setScreen({ kind: "transferring", batch });
      }
    } catch (err) {
      setError(flowErrorText(err));
    }
  };

  const phase = fact ? uploadDisplayPhase(fact) : null;

  return (
    <CapabilityGate capability={capability} action="upload_batch_create">
      <section className="cf-center" aria-label="上传中心" data-support={support}>
        {error && (
          <div className="banner error" role="alert">
            {error}
          </div>
        )}
        {notice && !error && (
          <div className="banner info" role="status">
            {notice}
          </div>
        )}

        {records.length > 0 && (
          <div className="cf-card" data-region="recovery">
            <h3>可恢复的上传身份</h3>
            <p className="cf-hint">
              {store.durable
                ? "恢复身份保存在本浏览器的 IndexedDB（仅非秘密字段），刷新或重开后可按原键恢复。"
                : "当前环境没有可用 IndexedDB，恢复身份仅在本页会话内保留，刷新后需要重新发起。"}
            </p>
            <ul>
              {records.map((record) => (
                <li key={record.idempotency_key}>
                  <span>{new Date(record.saved_at || Date.now()).toLocaleString("zh-CN")} · {record.action}</span>
                  {record.batch_id ? <code>{record.batch_id}</code> : <span>创建响应未确认</span>}
                  <Button variant="outline" type="button" onClick={() => void resume(record)}>
                    按原键恢复
                  </Button>
                  <Button
                    variant="ghost"
                    type="button"
                    onClick={async () => {
                      await store.remove(record.identity, record.action, record.idempotency_key);
                      await reloadRecords();
                    }}
                  >
                    移除记录
                  </Button>
                </li>
              ))}
            </ul>
          </div>
        )}

        {screen.kind === "select" && (
          <div
            className="cf-card cf-dropzone"
            data-region="select"
            onDragOver={(event) => event.preventDefault()}
            onDrop={(event) => void onDrop(event)}
          >
            <h3>选择要上传的文件{support === "directory" ? "或文件夹" : ""}</h3>
            {support === "files-only" && (
              <p className="cf-hint">当前浏览器不支持目录选择，将以普通多文件方式上传，不保留目录结构。</p>
            )}
            <Input
              ref={inputRef}
              type="file"
              multiple
              {...(support === "directory" ? ({ webkitdirectory: "", directory: "" } as Record<string, string>) : {})}
              onChange={(event) => event.target.files && onFiles(Array.from(event.target.files))}
              aria-label="选择文件"
            />
            {entries.length > 0 && (
              <ul className="cf-filelist" aria-label="已选文件">
                {entries.map((entry) => (
                  <li key={entry.relative_path}>
                    <span>{entry.relative_path}</span>
                    <span>{bytes(entry.file.size)}</span>
                    <span>{entry.sha256 ? "摘要已计算" : "摘要计算中…"}</span>
                  </li>
                ))}
              </ul>
            )}
            <Button
              variant="default"
              type="button"
              disabled={!entries.length || !entries.every((entry) => entry.sha256) || !space}
              onClick={() => void createBatch()}
            >
              创建上传批次
            </Button>
          </div>
        )}

        {screen.kind === "digesting" && (
          <section className="cf-state" role="status" data-state="loading">
            <p>正在计算文件摘要（{screen.done}/{screen.total}）…</p>
          </section>
        )}

        {screen.kind === "creating" && (
          <section className="cf-state" role="status" data-state="loading">
            <p>正在保存恢复身份并创建批次…</p>
          </section>
        )}

        {screen.kind === "resuming" && (
          <div className="cf-card" data-region="reselect">
            <h3>重新选择原文件</h3>
            <p className="cf-hint">
              浏览器刷新后无法保留文件句柄。请重新选择与原批次相同的文件；系统将按大小与 SHA-256 校验，不会只按文件名匹配。
            </p>
            <Input
              type="file"
              multiple
              {...(support === "directory" ? ({ webkitdirectory: "", directory: "" } as Record<string, string>) : {})}
              onChange={(event) => event.target.files && onFiles(Array.from(event.target.files))}
              aria-label="重新选择原文件"
            />
            <Button variant="default" type="button" disabled={!entries.length} onClick={() => void verifyReselection(screen.record)}>
              校验并继续原任务
            </Button>
          </div>
        )}

        {screen.kind === "content_mismatch" && (
          <section className="cf-state cf-error" role="alert" data-state="error">
            <h3>文件校验不一致</h3>
            <p>以下文件与原批次指纹不符，已验证的进度保留；请重新选择原文件，或对新内容发起新的上传：</p>
            <ul>
              {screen.mismatched.map((path) => (
                <li key={path}>{path}</li>
              ))}
            </ul>
            <Button variant="outline" type="button" onClick={() => setScreen({ kind: "select" })}>
              返回重新选择
            </Button>
          </section>
        )}

        {screen.kind === "transferring" && (
          <div className="cf-card" data-region="progress">
            <h3>批次 {screen.batch.batch_id}</h3>
            <dl className="cf-facts">
              <div>
                <dt>阶段</dt>
                <dd data-fact="phase">{UPLOAD_PHASE_LABELS[phase ?? "transferring"]}</dd>
              </div>
              <div>
                <dt>文件进度</dt>
                <dd>
                  完成 {screen.batch.items_completed ?? 0} / 总数 {screen.batch.items_total ?? "未封存"} · 失败 {screen.batch.items_failed ?? 0} · 等待 {screen.batch.items_waiting ?? 0}
                </dd>
              </div>
              {fact && (
                <>
                  <div>
                    <dt>已发送</dt>
                    <dd>{bytes(fact.bytes_sent)}</dd>
                  </div>
                  <div>
                    <dt>已耐久确认</dt>
                    <dd>{bytes(fact.bytes_durable)}</dd>
                  </div>
                  <div>
                    <dt>服务端已验证</dt>
                    <dd>{bytes(fact.bytes_verified)}</dd>
                  </div>
                </>
              )}
            </dl>
            <p className="cf-hint">发送进度不代表完成：只有服务端读回校验并发布后才显示已完成。</p>
            <div className="cf-actions">
              <Button variant="outline" type="button" onClick={() => void control("pause")}>
                暂停
              </Button>
              <Button variant="outline" type="button" onClick={() => void control("resume")}>
                继续
              </Button>
              <Button variant="outline" type="button" onClick={() => void control("cancel")}>
                取消（保留已提交文件）
              </Button>
            </div>
            {items.length > 0 && (
              <Table className="cf-table" aria-label="批次文件">
                <thead>
                  <tr>
                    <th>相对路径</th>
                    <th>状态</th>
                    <th>原操作</th>
                  </tr>
                </thead>
                <tbody>
                  {items.map((item) => (
                    <tr key={item.item_key}>
                      <td>{item.relative_path}</td>
                      <td>{item.result_certainty === "confirmed" ? "已确认" : item.result_certainty === "unknown" ? "结果未知，回读原操作" : item.result_certainty}</td>
                      <td>
                        <code>{item.operation_id}</code>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </Table>
            )}
          </div>
        )}
      </section>
    </CapabilityGate>
  );
}
