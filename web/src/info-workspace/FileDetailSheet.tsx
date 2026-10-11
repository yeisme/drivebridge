// File detail drawer (文件详情): overview, versions, source & references,
// content understanding, discussion and activity for one fixed file ref.
//
// The selected version is pinned: opening a file from an old delivery keeps
// the original ref even when a newer version exists, and an unreadable
// version shows an explicit recovery path instead of silently substituting
// the newest version. Technical identifiers stay collapsed but copyable.

import { useCallback, useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import {
  api,
  bytes,
  copyReference,
  errorText,
  pinnedFile,
  type Audit,
  type DriveFile,
  type Operation,
  type Version,
} from "../api";
import { AppSheet } from "../components/AppSheet";
import { detailPreferenceKey, readDetailWidth, saveDetailWidth, type DetailPreferenceIdentity } from "../app/detail-preference";
import { Button } from "../components/ui/button";
import { Textarea } from "../components/ui/textarea";
import { auditActionLabel } from "../insights/audit-labels";
import { statusLabel } from "../i18n";
import {
  listContentBindings,
  listFileThreads,
  operationsForFile,
  readOperation,
  statFile,
  fileVersions,
} from "./client";
import { clearDraft, readDraft, writeDraft, type WorkspaceDraft } from "./compat";
import { STATE_LABELS, isAbort, type ViewState } from "./model";
import { bindingStatusLabel, threadStateLabel, utcStamp } from "./format";
import type { ContentBinding, FileThread } from "./model";
import { ContextAssistant } from "./ContextAssistant";

const TABS = [
  ["overview", "概览"],
  ["versions", "版本"],
  ["source", "来源与引用"],
  ["understanding", "内容理解"],
  ["discussion", "讨论"],
  ["activity", "活动"],
] as const;
type TabId = (typeof TABS)[number][0];

interface SectionState {
  state: ViewState;
  error?: unknown;
}

const IDLE: SectionState = { state: "loading" };

interface FileDetailProps {
  preferenceIdentity?: DetailPreferenceIdentity;
  fileId: string;
  versionId?: string;
  open: boolean;
  onClose: () => void;
}

export function FileDetailSheet(props: FileDetailProps) {
  return <ScopedFileDetail key={JSON.stringify([detailPreferenceKey(props.preferenceIdentity), props.fileId, props.versionId, props.open])} {...props} />;
}

function ScopedFileDetail(props: FileDetailProps) {
  const [invalidated, setInvalidated] = useState("");
  useEffect(() => {
    const denied = () => setInvalidated("权限或范围已变化，上下文已清除。请关闭后重新打开文件详情以鉴权。"),
      offline = () => setInvalidated("已离线，上下文已清除。恢复网络后请关闭并重新打开文件详情。");
    window.addEventListener("drivebridge-context", denied);
    window.addEventListener("offline", offline);
    if (navigator.onLine === false) offline();
    return () => { window.removeEventListener("drivebridge-context", denied); window.removeEventListener("offline", offline); };
  }, []);
  if (invalidated) return <AppSheet title="文件详情" open={props.open} onOpenChange={value => !value && props.onClose()}><p role="alert">{invalidated}</p></AppSheet>;
  return <FileDetailContent {...props} onDenied={() => setInvalidated("权限或范围已变化，上下文已清除。请关闭后重新打开文件详情以鉴权。")} />;
}

function FileDetailContent({
  preferenceIdentity,
  fileId,
  versionId,
  open,
  onClose,
  onDenied,
}: FileDetailProps & { onDenied: () => void }) {
  const [tab, setTab] = useState<TabId>("overview");
  const [file, setFile] = useState<DriveFile | null>(null);
  const [fileState, setFileState] = useState<SectionState>(IDLE);
  const [versions, setVersions] = useState<Version[]>([]);
  const [versionError, setVersionError] = useState("");
  const [selectedVersion, setSelectedVersion] = useState(versionId || "");
  const [pin, setPin] = useState<{ ref: string; sha256: string; size: number } | null>(null);
  const [pinState, setPinState] = useState<SectionState>(IDLE);
  const [copied, setCopied] = useState("");
  const [ops, setOps] = useState<Operation[]>([]);
  const [sourceState, setSourceState] = useState<SectionState>(IDLE);
  const [readback, setReadback] = useState<Operation | null>(null);
  const [readbackError, setReadbackError] = useState("");
  const [bindings, setBindings] = useState<ContentBinding[]>([]);
  const [bindingState, setBindingState] = useState<SectionState>(IDLE);
  const [threads, setThreads] = useState<FileThread[]>([]);
  const [threadState, setThreadState] = useState<SectionState>(IDLE);
  const [draft, setDraft] = useState("");
  const draftRef = useRef<WorkspaceDraft | null>(null);
  const [reload, setReload] = useState(0);
  const tablist = useRef<HTMLDivElement>(null);

  // Reset per-file state whenever a different file opens.
  useEffect(() => {
    if (!open) return;
    setTab("overview");
    setFile(null);
    setFileState(IDLE);
    setVersions([]);
    setPin(null);
    setVersionError("");
    setPinState(IDLE);
    setCopied("");
    setOps([]);
    setSourceState(IDLE);
    setReadback(null);
    setReadbackError("");
    setBindings([]);
    setBindingState(IDLE);
    setThreads([]);
    setThreadState(IDLE);
    setSelectedVersion(versionId || "");
    // The draft is UI-only recovery metadata; it survives reloads but never
    // carries credentials or file bodies.
    const stored = readDraft(sessionStorage);
    draftRef.current = stored;
    if (stored?.discussion) setDraft(stored.discussion);
  }, [open, fileId, reload]);

  // Base file metadata and version list.
  useEffect(() => {
    if (!open || !fileId) return;
    const controller = new AbortController();
    setFileState(IDLE);
    statFile(fileId, controller.signal)
      .then((value) => {
        if (controller.signal.aborted) return;
        setFile(value);
        setFileState({ state: "ready" });
        setSelectedVersion((old) => old || value.version_id);
      })
      .catch((error) => {
        if (controller.signal.aborted) return;
        if (error && typeof error === "object" && "status" in error && (error.status === 401 || error.status === 403)) { onDenied(); return; }
        if (!isAbort(error)) setFileState({ state: viewStateOf(error), error });
      });
    fileVersions(fileId, controller.signal)
      .then((value) => {
        if (controller.signal.aborted) return;
        setVersions(value);
        setVersionError("");
      })
      .catch((error) => {
        if (controller.signal.aborted) return;
        if (error && typeof error === "object" && "status" in error && (error.status === 401 || error.status === 403)) { onDenied(); return; }
        // A failed version list is surfaced with a retry, not silently
        // swallowed as "no versions": the list is the basis for pinning.
        if (!isAbort(error)) {
          setVersions([]);
          setVersionError(errorText(error));
        }
      });
    return () => controller.abort();
  }, [open, fileId, reload]);

  // Version pinning: resolve the SELECTED version, never the newest. A newer
  // upload must not change what this sheet resolves or copies.
  useEffect(() => {
    if (!open || tab !== "versions" || !file || !selectedVersion) return;
    const controller = new AbortController();
    setPinState(IDLE);
    pinnedFile(file, selectedVersion, {}, controller.signal)
      .then((result) => {
        if (controller.signal.aborted) return;
        setPin({ ref: result.ref, sha256: result.sha256, size: result.file.size_bytes });
        setPinState({ state: "ready" });
      })
      .catch((error) => {
        if (controller.signal.aborted) return;
        if (error && typeof error === "object" && "status" in error && (error.status === 401 || error.status === 403)) { onDenied(); return; }
        if (!isAbort(error)) {
          // The pinned version may be unreadable or purged; say so and offer
          // recovery without substituting a different version.
          setPin(null);
          setPinState({ state: viewStateOf(error), error });
        }
      });
    return () => controller.abort();
  }, [open, tab, file, selectedVersion, reload]);

  // Source & references: which operation produced this file.
  useEffect(() => {
    if (!open || tab !== "source" || !fileId) return;
    const controller = new AbortController();
    setSourceState(IDLE);
    operationsForFile(fileId, controller.signal, file?.space_id)
      .then((value) => {
        setOps(value);
        setSourceState({ state: value.length ? "ready" : "empty" });
      })
      .catch((error) => {
        if (!isAbort(error)) setSourceState({ state: viewStateOf(error), error });
      });
    return () => controller.abort();
  }, [open, tab, fileId, reload, file?.space_id]);

  useEffect(() => {
    if (!open || tab !== "understanding" || !fileId) return;
    // Bindings are listed per space; the file header carries the space id,
    // so the query waits for it and then narrows to this file by file_ref.
    if (!file?.space_id) return;
    const controller = new AbortController();
    setBindingState(IDLE);
    listContentBindings(file.space_id, controller.signal)
      .then((value) => {
        const scoped = value.filter((binding) =>
          binding.file_ref.includes("/file/" + fileId + "/"),
        );
        setBindings(scoped);
        setBindingState({ state: scoped.length ? "ready" : "empty" });
      })
      .catch((error) => {
        if (!isAbort(error)) setBindingState({ state: viewStateOf(error), error });
      });
    return () => controller.abort();
  }, [open, tab, fileId, reload, file?.space_id]);

  useEffect(() => {
    if (!open || tab !== "discussion" || !fileId) return;
    const controller = new AbortController();
    setThreadState(IDLE);
    listFileThreads(fileId, controller.signal)
      .then((value) => {
        setThreads(value);
        setThreadState({ state: value.length ? "ready" : "empty" });
      })
      .catch((error) => {
        if (!isAbort(error)) setThreadState({ state: viewStateOf(error), error });
      });
    return () => controller.abort();
  }, [open, tab, fileId, reload]);

  const updateDraft = useCallback(
    (text: string) => {
      setDraft(text);
      const base = draftRef.current || {
        version: 2 as const,
        filters: { space_id: "", from: "", to: "" },
      };
      if (text) writeDraft(sessionStorage, { ...base, discussion: text });
      else if (draftRef.current?.discussion) clearDraft(sessionStorage);
    },
    [],
  );

  const copy = async () => {
    if (!file) return;
    try {
      await copyReference(file, selectedVersion || file.version_id);
      setCopied("已复制固定版本引用；在已连接 DriveBridge 的 Agent 中使用");
    } catch (error) {
      if (!isAbort(error)) setCopied("复制失败：" + errorText(error));
    }
  };

  const showReadback = async (id: string) => {
    setReadbackError("");
    setReadback(null);
    try {
      // Evidence readback re-reads the original operation ID; it never
      // re-keys or substitutes a different operation.
      setReadback(await readOperation(id));
    } catch (error) {
      if (!isAbort(error)) setReadbackError(errorText(error));
    }
  };

  const onTabKeys = (event: KeyboardEvent) => {
    if (event.key !== "ArrowRight" && event.key !== "ArrowLeft") return;
    const buttons = Array.from(
      tablist.current?.querySelectorAll<HTMLButtonElement>("[role=tab]") || [],
    );
    const index = buttons.indexOf(document.activeElement as HTMLButtonElement);
    if (index < 0) return;
    event.preventDefault();
    const next = buttons[(index + (event.key === "ArrowRight" ? 1 : buttons.length - 1)) % buttons.length];
    next.focus();
    next.click();
  };

  const name = file?.path.split("/").pop() || fileId;
  const preferenceKey = detailPreferenceKey(preferenceIdentity);

  return (
    <AppSheet key={preferenceKey || "unscoped"} detailWidth={360} initialDetailWidth={readDetailWidth(preferenceKey)} onDetailWidthCommit={width => saveDetailWidth(preferenceKey, width)} title={"文件详情：" + name} open={open} onOpenChange={(value) => !value && onClose()}>
      {fileState.state === "ready" && file && selectedVersion && <ContextAssistant fileId={fileId} versionId={selectedVersion} onDenied={onDenied} />}
      <div className="iw-tabs" role="tablist" aria-label="文件详情页签" ref={tablist} onKeyDown={onTabKeys}>
        {TABS.map(([id, label]) => (
          <Button
            key={id}
            variant="ghost"
            type="button"
            role="tab"
            tabIndex={tab === id ? 0 : -1}
            aria-selected={tab === id}
            onClick={() => setTab(id)}
          >
            {label}
          </Button>
        ))}
      </div>

      {tab === "overview" && (
        <Section title="概览" section={fileState} onRetry={() => setReload((n) => n + 1)}>
          {file && (
            <dl className="iw-facts">
              <div>
                <dt>路径</dt>
                <dd>{file.path}</dd>
              </div>
              <div>
                <dt>大小</dt>
                <dd>{bytes(file.size_bytes)}</dd>
              </div>
              <div>
                <dt>类型</dt>
                <dd>{file.mime_type}</dd>
              </div>
              <div>
                <dt>更新时间</dt>
                <dd>{utcStamp(file.updated_at)}</dd>
              </div>
              <div>
                <dt>当前版本</dt>
                <dd>{file.version_id}</dd>
              </div>
              {file.trashed && (
                <div>
                  <dt>状态</dt>
                  <dd>已在回收站{file.purge_after ? `，${file.purge_after} 后永久删除` : ""}</dd>
                </div>
              )}
            </dl>
          )}
          <details className="iw-tech">
            <summary>技术标识（可复制）</summary>
            <p>
              文件 ID：<code>{fileId}</code>
            </p>
            <p>
              版本 ID：<code>{selectedVersion || file?.version_id || "未取得"}</code>
            </p>
            {file?.ref && (
              <p>
                固定引用：<code>{file.ref}</code>
              </p>
            )}
            <div className="iw-tech-actions">
              <Button variant="outline" type="button" onClick={copy}>
                复制文件引用
              </Button>
              {copied && (
                <span role="status" className="iw-copied">
                  {copied}
                </span>
              )}
            </div>
          </details>
        </Section>
      )}

      {tab === "versions" && (
        <Section title="版本" section={pinState} onRetry={() => setReload((n) => n + 1)}>
          {versionError && (
            <div className="iw-error" role="alert">
              {versionError}
              <Button variant="outline" type="button" onClick={() => setReload((n) => n + 1)}>
                重试
              </Button>
            </div>
          )}
          <ul className="iw-versions">
            {versions.map((version) => (
              <li key={version.id}>
                <Button
                  variant="ghost"
                  type="button"
                  aria-pressed={version.id === selectedVersion}
                  onClick={() => setSelectedVersion(version.id)}
                >
                  <span>{version.id === (file?.version_id || "") ? "当前版本 " : ""}
                    {utcStamp(version.created_at)} · {bytes(version.size_bytes)}
                  </span>
                  {version.id === selectedVersion && <strong>已选择</strong>}
                </Button>
              </li>
            ))}
            {!versions.length && <li className="iw-hint">版本列表未取得或为空。</li>}
          </ul>
          {pin && (
            <dl className="iw-facts">
              <div>
                <dt>所选版本</dt>
                <dd>{selectedVersion}</dd>
              </div>
              <div>
                <dt>SHA-256</dt>
                <dd>
                  <code>{pin.sha256}</code>
                </dd>
              </div>
              <div>
                <dt>固定引用</dt>
                <dd>
                  <code>{pin.ref}</code>
                </dd>
              </div>
              <div className="iw-tech-actions">
                <Button variant="outline" type="button" onClick={copy}>
                  复制所选版本引用
                </Button>
                {copied && (
                  <span role="status" className="iw-copied">
                    {copied}
                  </span>
                )}
              </div>
            </dl>
          )}
        </Section>
      )}
      {tab === "versions" && pinState.state === "error" && (
        <div className="iw-version-recovery" role="alert">
          所选版本当前不可读（可能已清理或无权限）；详情不会自动替换为最新版本。
          <div className="iw-tech-actions">
            <Button variant="outline" type="button" onClick={() => setSelectedVersion(file?.version_id || "")}>
              改选当前版本
            </Button>
            <Button variant="ghost" type="button" onClick={copy}>
              复制所选版本引用
            </Button>
          </div>
        </div>
      )}


      {tab === "source" && (
        <Section title="来源与引用" section={sourceState} onRetry={() => setReload((n) => n + 1)}>
          {!ops.length && <p className="iw-hint">未找到产生此文件的操作记录（来源检索覆盖最近的操作窗口；不在窗口内的旧操作不会伪造）。</p>}
          {ops.slice(0, 5).map((op) => (
            <div className="iw-op" key={op.id}>
              <dl className="iw-facts">
                <div>
                  <dt>来源操作</dt>
                  <dd>
                    <code>{op.id}</code>
                  </dd>
                </div>
                {op.caller && (
                  <div>
                    <dt>来源调用方</dt>
                    <dd>{op.caller}</dd>
                  </div>
                )}
                {op.task_ref && (
                  <div>
                    <dt>来源任务</dt>
                    <dd>{op.task_ref}</dd>
                  </div>
                )}
                {op.source_id && (
                  <div>
                    <dt>导入来源</dt>
                    <dd>
                      {op.source_id}
                      {op.source_version ? ` · ${op.source_version}` : ""}
                      {op.source_space_id ? ` · 空间 ${op.source_space_id}` : ""}
                    </dd>
                  </div>
                )}
                {op.source_path && (
                  <div>
                    <dt>来源路径</dt>
                    <dd>{op.source_path}</dd>
                  </div>
                )}
              </dl>
              <div className="iw-tech-actions">
                <Button variant="outline" type="button" onClick={() => void showReadback(op.id)}>
                  回读原操作
                </Button>
              </div>
            </div>
          ))}
          {readbackError && (
            <p role="alert" className="iw-error">
              {readbackError}
            </p>
          )}
          {readback && (
            <section className="iw-readback" aria-label="原操作回读">
              <h3>原操作回读（原 ID {readback.id}）</h3>
              <p>
                状态 {statusLabel(readback.status)} · 已传输 {bytes(readback.bytes_transferred)} /{" "}
                {bytes(readback.size_bytes)}
                {readback.updated_at ? ` · 更新于 ${utcStamp(readback.updated_at)}` : ""}
              </p>
            </section>
          )}
        </Section>
      )}

      {tab === "understanding" && (
        <Section title="内容理解" section={bindingState} onRetry={() => setReload((n) => n + 1)}>
          <p className="iw-hint">
            内容理解只展示内容处理 owner 的事实；处理不可用不影响文件原件读取。
          </p>
          <ul className="iw-bindings">
            {bindings.map((binding) => (
              <li key={binding.binding_id}>
                {bindingStatusLabel(binding.state)}
                {binding.purpose ? ` · ${binding.purpose}` : ""}
                {binding.project_ref ? ` · 项目 ${binding.project_ref}` : ""}
                {binding.error ? ` · ${binding.error}` : ""}
                {" · "}
                <code>{binding.file_ref}</code>
              </li>
            ))}
          </ul>
        </Section>
      )}

      {tab === "discussion" && (
        <Section title="讨论" section={threadState} onRetry={() => setReload((n) => n + 1)}>
          <ul className="iw-threads">
            {threads.map((thread) => (
              <li key={thread.thread_id}>
                {threadStateLabel(thread.state)} · 锚定{thread.version_id ? `版本 ${thread.version_id}` : " 文件级"} · 第 {thread.revision} 次修订 ·
                发起者 {thread.author_kind === "user" ? "用户" : thread.author_kind}
              </li>
            ))}
          </ul>
          <label className="iw-draft">
            <span>新讨论草稿（本地保存，未发送）</span>
            <Textarea
              value={draft}
              maxLength={10000}
              onChange={(e) => updateDraft(e.target.value)}
              placeholder="围绕固定版本发表纯文本讨论…"
            />
          </label>
          <p className="iw-hint">草稿仅保存在本机界面状态；发送入口由讨论能力 owner 提供。</p>
        </Section>
      )}

      {tab === "activity" && (
        <ActivityTab fileId={fileId} active={open && tab === "activity"} />
      )}
    </AppSheet>
  );
}

function ActivityTab({ fileId, active }: { fileId: string; active: boolean }) {
  const [events, setEvents] = useState<Audit[]>([]);
  const [section, setSection] = useState<SectionState>(IDLE);
  const [reload, setReload] = useState(0);
  useEffect(() => {
    if (!active || !fileId) return;
    const controller = new AbortController();
    setSection(IDLE);
    api<{ events: Audit[]; next_cursor: number }>(
      "/audit/events?resource=" + encodeURIComponent(fileId),
      "GET",
      undefined,
      {},
      controller.signal,
    )
      .then((page) => {
        setEvents(page.events);
        setSection({ state: page.events.length ? "ready" : "empty" });
      })
      .catch((error) => {
        if (!isAbort(error)) setSection({ state: viewStateOf(error), error });
      });
    return () => controller.abort();
  }, [active, fileId, reload]);
  return (
    <Section title="活动" section={section} onRetry={() => setReload((n) => n + 1)}>
      <ul className="iw-activity">
        {events.map((event) => (
          <li key={event.id}>
            {utcStamp(event.at)} · {auditActionLabel(event.action)}
            {event.outcome !== "success" ? ` · ${event.outcome}` : ""}
          </li>
        ))}
      </ul>
    </Section>
  );
}

// Map transport errors onto section view states; dependency unavailability is
// surfaced as an error here because the sheet holds no cached observations.
function viewStateOf(error: unknown): ViewState {
  const code = error instanceof Error ? error.message : "";
  if (!navigator.onLine) return "offline";
  if (code === "forbidden") return "forbidden";
  if (code === "unsupported") return "unsupported";
  return "error";
}

function Section({
  title,
  section,
  onRetry,
  children,
}: {
  title: string;
  section: SectionState;
  onRetry: () => void;
  children: ReactNode;
}) {
  return (
    <section className="iw-detail-section" aria-label={title}>
      {section.state === "loading" && (
        <p role="status" className="iw-loading">
          正在加载{title}…
        </p>
      )}
      {section.state === "forbidden" && <p className="iw-hint">当前授权不允许查看此内容。</p>}
      {section.state === "offline" && <p className="iw-hint">当前离线，恢复网络后重试。</p>}
      {section.state === "unsupported" && (
        <p className="iw-hint">此部署未启用对应能力（{STATE_LABELS.unsupported}）。</p>
      )}
      {section.state === "error" && (
        <div role="alert" className="iw-error">
          {section.error instanceof Error ? errorText(section.error) : "加载失败"}
          <Button variant="outline" type="button" onClick={onRetry}>
            重试
          </Button>
        </div>
      )}
      {section.state !== "loading" &&
        section.state !== "error" &&
        section.state !== "forbidden" &&
        section.state !== "offline" &&
        children}
    </section>
  );
}
