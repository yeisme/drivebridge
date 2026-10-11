import { createPortal } from "react-dom";
import { agentContext } from "./preview-model";
import { createRoot } from "react-dom/client";
import type { Root } from "react-dom/client";
import type { Extension } from "@codemirror/state";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { EditorState, Compartment } from "@codemirror/state";
import {
  EditorView,
  keymap,
  lineNumbers,
  highlightActiveLine,
  drawSelection,
} from "@codemirror/view";
import {
  history,
  historyField,
  historyKeymap,
  defaultKeymap,
  indentWithTab,
} from "@codemirror/commands";
import { markdown } from "@codemirror/lang-markdown";
import {
  syntaxHighlighting,
  defaultHighlightStyle,
} from "@codemirror/language";
import {
  searchKeymap,
  highlightSelectionMatches,
  openSearchPanel,
} from "@codemirror/search";
import { autocompletion, completionKeymap } from "@codemirror/autocomplete";
import { Paperclip, Search, Save, Sparkles } from "lucide-react";
import { Btn, IconBtn, useDemo, Demo } from "./ui";
import { fileRef } from "./preview-model";
import type { FileRecord } from "./model";
import { fileUnavailable } from "./v4-model";
import { useFileHover, FileHoverBridge } from "./HoverPreview";
import { createLiveFormatting, refreshLive } from "./markdown-live";
import { MarkdownBody } from "./MarkdownBody";
import { noteBody, writableNote, markdownTasks } from "./knowledge-model";
import type { EditorMode } from "./knowledge-model";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "./Dialog";
import { Input } from "@/components/ui/input";
const readingPositions = new Map<string, number>();
const sessions = new Map<string, { json: unknown; scroll: number }>();
export const clearEditorSessions = () => {
  sessions.clear();
  readingPositions.clear();
};
const statuses = {
  saved: "已保存到本轮演示",
  dirty: "未保存",
  saving: "保存中…",
  offline: "等待网络 · 草稿保留",
  failed: "保存失败 · 草稿保留",
  conflict: "版本冲突 · 草稿保留",
};
export function MarkdownEditor({
  file: f,
  version,
  onVersion,
  anchor,
  toolbar,
}: {
  file: FileRecord;
  version: number;
  onVersion: (v: number) => void;
  anchor?: string;
  toolbar?: HTMLElement | null;
}) {
  const demo = useDemo(),
    { state: s, dispatch, open } = demo;
  const n = s.knowledge.notes[f.id];
  const [formatOpen, setFormatOpen] = useState(false);
  const [manualCopy, setManualCopy] = useState("");
  const previousBase = useRef(n?.base);
  const host = useRef<HTMLDivElement>(null),
    editor = useRef<EditorView | null>(null);
  const reader = useRef<HTMLDivElement>(null);
  const hover = useFileHover();
  const liveField = useRef<Extension>([]);
  const blocks = useRef(
    new Map<
      HTMLElement,
      { root: Root; body: string; from: number; to: number }
    >(),
  );
  const demoRef = useRef(demo);
  demoRef.current = demo;
  const [chapters, setChapters] = useState(false);
  const [attachments, setAttachments] = useState(false),
    [diff, setDiff] = useState(false),
    [reload, setReload] = useState(false);
  const [copyName, setCopyName] = useState(""),
    [attachmentQuery, setAttachmentQuery] = useState("");
  const input = useRef<HTMLInputElement>(null);
  const mode = n?.mode || "read";
  const followingSavedVersion =
    mode !== "read" &&
    n?.base === f.version &&
    previousBase.current === version &&
    n.base !== version;
  const historyVersion =
    version !== f.version &&
    !followingSavedVersion &&
    !(n?.dirty && mode !== "read" && version === n.base);
  const canWrite = writableNote(s, f);
  const readOnly = historyVersion || !canWrite;
  const latest = useRef({ s, f, n, version, onVersion, open, dispatch });
  latest.current = { s, f, n, version, onVersion, open, dispatch };
  const compartment = useRef(new Compartment());
  const editing = mode !== "read" && !readOnly;
  const readingKey = `${f.id}@${version}`;
  useLayoutEffect(() => {
    if (reader.current)
      reader.current.scrollTop = readingPositions.get(readingKey) || 0;
  }, [readingKey]);
  useEffect(() => {
    dispatch({ type: "k-open", id: f.id });
  }, [f.id]);
  useLayoutEffect(() => {
    if (
      n?.base !== previousBase.current &&
      previousBase.current !== undefined &&
      version === previousBase.current
    )
      onVersion(n!.base);
    previousBase.current = n?.base;
  }, [n?.base, version, mode]);
  const save = () => {
    dispatch({ type: "k-save", id: f.id });
  };
  const paint = (root: Root, body: string, from: number, to: number) =>
    root.render(
      <Demo.Provider value={demoRef.current}>
        <FileHoverBridge value={hover}>
          <MarkdownBody
            file={latest.current.f}
            body={body}
            onTaskToggle={(index, checked) => {
              const view = editor.current,
                current = latest.current;
              if (
                !view ||
                view.composing ||
                !writableNote(current.s, current.f) ||
                view.state.doc.sliceString(from, to) !== body
              )
                return;
              const task = markdownTasks(body)[index];
              if (!task) return;
              view.dispatch({
                changes: {
                  from: from + task.offset,
                  to: from + task.offset + 1,
                  insert: checked ? "x" : " ",
                },
              });
            }}
          />
        </FileHoverBridge>
      </Demo.Provider>,
    );
  useLayoutEffect(() => {
    if (!host.current || !n) return;
    const cache = sessions.get(f.id);
    liveField.current = createLiveFormatting((node, body, from, to) => {
      const root = blocks.current.get(node)?.root || createRoot(node);
      blocks.current.set(node, { root, body, from, to });
      paint(root, body, from, to);
      return () => {
        blocks.current.delete(node);
        queueMicrotask(() => root.unmount());
      };
    });
    const extensions = [
      markdown(),
      history(),
      drawSelection(),
      syntaxHighlighting(defaultHighlightStyle),
      EditorView.lineWrapping,
      highlightSelectionMatches(),
      compartment.current.of(
        mode === "source"
          ? [lineNumbers(), highlightActiveLine()]
          : mode === "live"
            ? [liveField.current]
            : [],
      ),
      keymap.of([
        {
          key: "Mod-s",
          run: () => {
            const x = latest.current;
            x.dispatch({ type: "k-save", id: x.f.id });
            return true;
          },
        },
        ...completionKeymap,
        ...defaultKeymap,
        ...historyKeymap,
        ...searchKeymap,
        indentWithTab,
      ]),
      autocompletion({
        override: [
          (context) => {
            if (editor.current?.composing) return null;
            const match = context.matchBefore(/\[\[[^\]\n]*/);
            if (!match) return null;
            const { s, f } = latest.current;
            const options = s.files
              .filter((x) => x.space === f.space && !fileUnavailable(s, x))
              .map((x) => ({
                label: x.name.replace(/\.md$/i, ""),
                detail: `${x.folder} · ${s.connections.find((c) => c.id === x.connectionId)?.name || "来源未知"}`,
                apply: (
                  view: EditorView,
                  _c: unknown,
                  from: number,
                  to: number,
                ) => {
                  const duplicates =
                    s.files.filter(
                      (y) =>
                        y.space === x.space &&
                        y.name === x.name &&
                        !fileUnavailable(s, y),
                    ).length > 1;
                  const target =
                    (duplicates ? x.folder + "/" : "") +
                    x.name.replace(/\.md$/i, "");
                  const insert =
                    s.knowledge.options.linkFormat === "markdown"
                      ? `[${x.name}](${encodeURI((duplicates ? x.folder + "/" : "") + x.name)})`
                      : `[[${target}]]`;
                  view.dispatch({
                    changes: { from: from - 2, to, insert },
                    selection: { anchor: from - 2 + insert.length },
                  });
                },
              }));
            return { from: match.from + 2, options, validFor: /^[^\]\n]*$/ };
          },
        ],
      }),
      EditorView.contentAttributes.of({
        "aria-label": "Markdown 编辑器",
        spellcheck: "false",
      }),
      EditorView.theme({
        "&": {
          height: "100%",
          fontSize: "15px",
          backgroundColor: "var(--card)",
          color: "var(--foreground)",
        },
        ".cm-scroller": {
          overflow: "auto",
          fontFamily: "inherit",
          lineHeight: "2",
        },
        ".cm-content": {
          padding: "24px",
          maxWidth: "760px",
          width: "100%",
          margin: "0 auto",
          minHeight: "100%",
        },
        ".cm-focused": { outline: "none" },
        ".cm-gutters": {
          backgroundColor: "var(--muted)",
          borderColor: "var(--border)",
          color: "var(--muted-foreground)",
        },
        ".cm-tooltip": {
          backgroundColor: "var(--card)",
          color: "var(--foreground)",
          border: "1px solid var(--border)",
          maxWidth: "min(420px,90vw)",
        },
        ".cm-tooltip-autocomplete ul": { maxWidth: "min(420px,90vw)" },
        ".cm-completionDetail": {
          display: "block",
          marginLeft: "0",
          fontSize: "11px",
        },
        ".cm-tooltip-autocomplete ul li": {
          padding: "8px 12px",
          minHeight: "44px",
        },
        ".cm-cursor": { borderLeftColor: "var(--foreground)" },
        ".cm-panels": {
          backgroundColor: "var(--muted)",
          color: "var(--foreground)",
        },
        ".cm-search input": { maxWidth: "120px", minHeight: "44px" },
        ".cm-search input[type=checkbox]": { minHeight: "auto" },
        ".cm-search label": {
          display: "inline-flex",
          alignItems: "center",
          minHeight: "44px",
        },
        ".cm-search button": { minHeight: "44px", minWidth: "44px" },
      }),
      EditorView.updateListener.of((update) => {
        if (update.docChanged) {
          const { f, dispatch } = latest.current;
          dispatch({
            type: "k-edit",
            id: f.id,
            body: update.state.doc.toString(),
            selection: {
              from: update.state.selection.main.from,
              to: update.state.selection.main.to,
            },
          });
        }
      }),
      EditorView.domEventHandlers({
        compositionstart: (_event, view) => {
          dispatch({ type: "k-compose", id: f.id, value: true });
          view.dispatch({ effects: refreshLive.of(true) });
          return false;
        },
        compositionend: (_event, view) => {
          dispatch({ type: "k-compose", id: f.id, value: false });
          setTimeout(() => {
            if (editor.current === view)
              view.dispatch({ effects: refreshLive.of(false) });
          }, 0);
          return false;
        },
        drop: (event) => {
          if (event.dataTransfer?.files.length) {
            event.preventDefault();
            enqueue(event.dataTransfer.files);
            return true;
          }
          return false;
        },
        paste: (event) => {
          if (event.clipboardData?.files.length) {
            event.preventDefault();
            enqueue(event.clipboardData.files);
            return true;
          }
          return false;
        },
      }),
    ];
    let state: EditorState;
    try {
      state = cache
        ? EditorState.fromJSON(
            cache.json,
            { extensions },
            { history: historyField },
          )
        : EditorState.create({ doc: n.draft, extensions });
    } catch {
      state = EditorState.create({ doc: n.draft, extensions });
    }
    const view = new EditorView({ state, parent: host.current });
    editor.current = view;
    if (view.state.doc.toString() !== n.draft)
      view.dispatch({
        changes: minimalDocumentChange(view.state.doc.toString(), n.draft),
      });
    if (cache) view.scrollDOM.scrollTop = cache.scroll;
    return () => {
      sessions.set(f.id, {
        json: view.state.toJSON({ history: historyField }),
        scroll: view.scrollDOM.scrollTop,
      });
      editor.current = null;
      view.destroy();
    };
  }, [f.id, !!n]);
  useEffect(() => {
    for (const { root, body, from, to } of blocks.current.values())
      paint(root, body, from, to);
  }, [s]);
  useEffect(() => {
    const view = editor.current;
    if (!view) return;
    view.dispatch({
      effects: compartment.current.reconfigure(
        mode === "source"
          ? [
              lineNumbers(),
              highlightActiveLine(),
              EditorView.theme({ ".cm-content": { fontFamily: "monospace" } }),
            ]
          : mode === "live"
            ? [liveField.current]
            : [],
      ),
    });
  }, [mode, !!n]);
  useEffect(() => {
    const view = editor.current;
    if (view && n && view.state.doc.toString() !== n.draft && !view.composing)
      view.dispatch({
        changes: minimalDocumentChange(view.state.doc.toString(), n.draft),
      });
  }, [n?.draft]);
  useEffect(() => {
    if (!anchor || !reader.current) return;
    Array.from(reader.current.querySelectorAll("[id]"))
      .find((el) => el.id === anchor)
      ?.scrollIntoView({ block: "start" });
  }, [anchor, version, mode]);
  useEffect(() => {
    const locate = (event: Event) => {
      const detail = (event as CustomEvent<{ id: string; from: number }>)
        .detail;
      if (detail.id !== f.id) return;
      requestAnimationFrame(() => {
        const view = editor.current;
        if (!view) return;
        const from = Math.min(detail.from, view.state.doc.length);
        view.dispatch({ selection: { anchor: from }, scrollIntoView: true });
        view.focus();
      });
    };
    window.addEventListener("drivebridge-source-position", locate);
    return () =>
      window.removeEventListener("drivebridge-source-position", locate);
  }, [f.id]);
  function enqueue(files: FileList) {
    for (const item of Array.from(files))
      dispatch({
        type: "k-attach",
        id: f.id,
        name: item.name,
        kind: item.type.startsWith("image")
          ? "image"
          : item.type.startsWith("video")
            ? "video"
            : item.type.startsWith("audio")
              ? "audio"
              : "document",
        size: `${Math.max(1, Math.round(item.size / 1024))} KB`,
      });
  }
  const insert = (text: string) => {
    const view = editor.current;
    if (!view) return;
    const selection = view.state.selection.main;
    view.dispatch({
      changes: { from: selection.from, to: selection.to, insert: text },
      selection: { anchor: selection.from + text.length },
    });
    view.focus();
  };
  const context = (section?: { title: string; body: string }) => {
    if (!historyVersion && n?.showDraft && (n?.dirty || n?.composing)) {
      save();
      dispatch({
        type: "notice",
        text: "先保存当前正文；保存完成后再加入固定上下文。",
      });
      return;
    }
    const selection = editor.current?.state.selection.main;
    const domSelection = window.getSelection();
    const excerpt =
      section?.body ||
      (editing && selection && !selection.empty
        ? n?.draft.slice(selection.from, selection.to)
        : !editing &&
            domSelection?.anchorNode &&
            reader.current?.contains(domSelection.anchorNode)
          ? domSelection.toString() || undefined
          : undefined);
    const refVersion = historyVersion || !n?.showDraft ? version : f.version;
    const text =
      agentContext(s, fileRef(f, refVersion)) +
      (section ? `\n章节：${section.title}` : "") +
      (excerpt ? `\n选定内容：\n${excerpt}` : "");
    navigator.clipboard
      .writeText(text)
      .then(() =>
        dispatch({
          type: "notice",
          text: "已复制外部 Agent 上下文（演示固定引用）；未创建分享。",
        }),
      )
      .catch(() => setManualCopy(text));
    setChapters(false);
  };
  const body =
    !historyVersion && n?.showDraft ? n.draft : noteBody(s, f.id, version);
  const uploads = s.knowledge.uploads.filter(
    (u) => u.noteId === f.id && !["done", "cancelled"].includes(u.status),
  );
  return (
    <div
      className="flex h-full min-h-0 min-w-0 flex-col bg-card"
      data-markdown-editor
    >
      {toolbar
        ? createPortal(
            <>
              <div
                className="flex h-11 min-w-0 items-center gap-0.5"
                aria-label="Markdown 模式"
              >
                {(["live", "source", "read"] as const).map((m) => (
                  <Btn
                    key={m}
                    variant={mode === m ? "secondary" : "ghost"}
                    className="shrink-0 !px-2 text-xs"
                    aria-pressed={mode === m}
                    disabled={m !== "read" && readOnly}
                    onClick={() =>
                      dispatch({ type: "k-mode", id: f.id, mode: m })
                    }
                  >
                    {m === "live"
                      ? "实时编辑"
                      : m === "source"
                        ? "源码"
                        : "阅读"}
                  </Btn>
                ))}

                <IconBtn
                  label="保存 Markdown"
                  disabled={!n?.dirty || readOnly || n.composing}
                  onClick={save}
                >
                  <Save size={15} />
                </IconBtn>
                <IconBtn
                  label="查找与替换"
                  disabled={!editing}
                  onClick={() =>
                    editor.current && openSearchPanel(editor.current)
                  }
                >
                  <Search size={15} />
                </IconBtn>
                <IconBtn label="格式与引用" onClick={() => setFormatOpen(true)}>
                  <Paperclip size={15} />
                </IconBtn>
                <span
                  role="status"
                  data-save-status={n?.status}
                  className="hidden truncate text-[10px] text-muted-foreground min-[1200px]:block"
                >
                  {historyVersion
                    ? `历史 v${version} · 只读`
                    : statuses[n?.status || "saved"]}
                </span>
              </div>
            </>,
            toolbar,
          )
        : null}
      {(readOnly ||
        n?.status === "conflict" ||
        n?.status === "failed" ||
        n?.status === "offline") && (
        <div className="shrink-0 space-y-1 border-b border-border bg-muted p-2 text-xs">
          <p>
            {historyVersion
              ? "当前为固定历史版本。"
              : !canWrite
                ? "当前文件不可写入。"
                : statuses[n!.status]}
          </p>
          <div className="flex flex-wrap gap-1">
            {historyVersion && n?.dirty && version === n.base && (
              <Btn
                variant="outline"
                onClick={() =>
                  dispatch({ type: "k-mode", id: f.id, mode: "live" })
                }
              >
                继续编辑草稿
              </Btn>
            )}
            {historyVersion && canWrite && (
              <Btn
                variant="outline"
                onClick={() => {
                  onVersion(f.version);
                  dispatch({ type: "k-mode", id: f.id, mode: "live" });
                }}
              >
                编辑最新版本
              </Btn>
            )}
            {n?.status === "conflict" && (
              <Btn variant="outline" onClick={() => setDiff(true)}>
                查看差异
              </Btn>
            )}
            <Btn
              variant="outline"
              onClick={() =>
                setCopyName(f.name.replace(/\.md$/i, "") + " · 副本")
              }
            >
              另存副本
            </Btn>
            {n?.status === "conflict" && (
              <Btn variant="ghost" onClick={() => setReload(true)}>
                重新载入
              </Btn>
            )}
            {n?.status === "failed" && (
              <Btn variant="outline" onClick={save}>
                重试保存
              </Btn>
            )}
            {!s.online && (
              <Btn
                variant="outline"
                onClick={() => dispatch({ type: "network", online: true })}
              >
                恢复网络
              </Btn>
            )}
          </div>
        </div>
      )}
      <div
        ref={host}
        data-editor-host
        hidden={!editing}
        className="min-h-0 min-w-0 flex-1 overflow-hidden"
      />
      <div
        ref={reader}
        hidden={editing}
        onScroll={(e) =>
          readingPositions.set(readingKey, e.currentTarget.scrollTop)
        }
        data-reader
        className="min-h-0 flex-1 overflow-auto p-5 sm:p-8"
      >
        <MarkdownBody file={f} body={body} />
      </div>
      {uploads.length > 0 && (
        <div className="max-h-24 shrink-0 overflow-auto border-t border-border px-3 text-xs">
          {uploads.map((u) => (
            <div key={u.id} className="flex min-h-11 items-center gap-2">
              <Paperclip size={13} />
              <span className="min-w-0 flex-1 truncate">
                {u.name} ·{" "}
                {u.status === "failed"
                  ? "上传未完成，尚未插入引用"
                  : "正在上传"}{" "}
                · {u.taskId}
              </span>
              {u.status === "failed" && (
                <Btn
                  variant="outline"
                  onClick={() =>
                    dispatch({ type: "k-upload-recover", id: u.id })
                  }
                >
                  恢复
                </Btn>
              )}
            </div>
          ))}
        </div>
      )}
      <div className="flex h-6 shrink-0 items-center justify-between px-3 text-[10px] text-muted-foreground">
        <span>{statuses[n?.status || "saved"]}</span>
        <span>UTF-8 · {body.length} 字符 · 演示</span>
      </div>
      <Dialog open={formatOpen} onOpenChange={setFormatOpen}>
        <DialogContent>
          <DialogTitle>格式与引用</DialogTitle>
          <DialogDescription>
            编辑当前正文，或复制固定版本供外部应用使用。
          </DialogDescription>
          <div
            className="flex flex-wrap items-center gap-2"
            aria-label="文档编辑工具"
            onClick={() => setFormatOpen(false)}
          >
            <Btn
              variant="outline"
              disabled={readOnly || !n?.dirty || n.composing}
              onClick={save}
            >
              保存正文
            </Btn>
            <Btn
              variant="outline"
              disabled={!editing}
              onClick={() => editor.current && openSearchPanel(editor.current)}
            >
              查找与替换正文
            </Btn>
            {!readOnly && (
              <>
                <IconBtn
                  label="插入标题"
                  onClick={() => {
                    dispatch({ type: "k-mode", id: f.id, mode: "live" });
                    insert("\n## 新标题\n");
                  }}
                >
                  H₂
                </IconBtn>
                <IconBtn
                  label="插入任务清单"
                  onClick={() => insert("\n- [ ] 待办事项")}
                >
                  ☑
                </IconBtn>
                <IconBtn
                  label="添加文档附件"
                  onClick={() => setAttachments(true)}
                >
                  <Paperclip size={15} />
                </IconBtn>
              </>
            )}
            <Btn
              variant="ghost"
              className="shrink-0 text-xs"
              onClick={() => context()}
            >
              <Sparkles size={14} />
              复制外部 Agent 上下文
            </Btn>
            <Btn
              variant="ghost"
              className="shrink-0 text-xs"
              onClick={() => setChapters(true)}
            >
              引用章节
            </Btn>
            <span className="flex-1" />
            <span className="shrink-0 text-[10px] text-muted-foreground">
              刷新恢复样本
            </span>
          </div>
        </DialogContent>
      </Dialog>
      <Dialog open={!!manualCopy} onOpenChange={(v) => !v && setManualCopy("")}>
        <DialogContent kind="confirm">
          <DialogTitle>剪贴板不可用，请手动复制</DialogTitle>
          <DialogDescription>
            包含文件来源与固定版本，不授予访问权限。
          </DialogDescription>
          <div
            aria-label="手动复制上下文"
            tabIndex={0}
            className="max-h-60 overflow-auto whitespace-pre-wrap break-words select-text text-xs leading-6"
          >
            {manualCopy}
          </div>
        </DialogContent>
      </Dialog>
      <Dialog open={chapters} onOpenChange={setChapters}>
        <DialogContent>
          <DialogTitle>引用文档章节</DialogTitle>
          <DialogDescription>
            仅将选定章节与固定文件版本加入本次上下文。
          </DialogDescription>
          <div className="max-h-72 overflow-auto">
            {Array.from(body.matchAll(/^#{1,6} (.+)$/gm)).map(
              (match, i, all) => (
                <Btn
                  key={match.index}
                  variant="ghost"
                  className="!h-12 !w-full !justify-start !whitespace-normal text-left"
                  onClick={() =>
                    context({
                      title: match[1],
                      body: body.slice(
                        match.index,
                        all[i + 1]?.index ?? body.length,
                      ),
                    })
                  }
                >
                  {match[1]}
                </Btn>
              ),
            )}
            {!/^#{1,6} /m.test(body) && (
              <p className="py-5 text-sm text-muted-foreground">
                当前文档没有标题，可以选择正文后加入上下文。
              </p>
            )}
          </div>
        </DialogContent>
      </Dialog>
      <input
        ref={input}
        type="file"
        multiple
        className="hidden"
        onChange={(e) => {
          if (e.target.files) enqueue(e.target.files);
          e.target.value = "";
        }}
      />
      <Dialog open={attachments} onOpenChange={setAttachments}>
        <DialogContent>
          <DialogTitle>文档附件</DialogTitle>
          <DialogDescription>
            选择已有文件或模拟上传。添加引用不会改变文件权限。
          </DialogDescription>
          <div className="my-3 flex flex-wrap gap-2">
            <Btn onClick={() => input.current?.click()}>从设备选择</Btn>
            <Btn
              variant="outline"
              onClick={() => {
                dispatch({
                  type: "k-attach",
                  id: f.id,
                  name: "现场照片.png",
                  kind: "image",
                  size: "2.4 MB",
                  fail: true,
                });
                setAttachments(false);
              }}
            >
              演示上传中断
            </Btn>
          </div>
          <Input
            aria-label="搜索已有附件"
            value={attachmentQuery}
            onChange={(e) => setAttachmentQuery(e.target.value)}
            placeholder="名称、附件"
          />
          <div className="mt-3 max-h-72 overflow-auto">
            {s.files
              .filter(
                (x) =>
                  x.id !== f.id &&
                  x.space === s.space &&
                  !fileUnavailable(s, x) &&
                  x.name.includes(attachmentQuery),
              )
              .map((x) => (
                <Btn
                  key={x.id}
                  variant="ghost"
                  className="!h-12 !w-full !justify-start truncate"
                  onClick={() => {
                    dispatch({ type: "k-mode", id: f.id, mode: "live" });
                    insert(`\n![[${x.name}]]\n`);
                    setAttachments(false);
                  }}
                >
                  {x.name} · v{x.version}
                </Btn>
              ))}
          </div>
        </DialogContent>
      </Dialog>
      <Dialog open={diff} onOpenChange={setDiff}>
        <DialogContent className="max-w-4xl">
          <DialogTitle>比较版本与草稿</DialogTitle>
          <DialogDescription>
            当前文件保留，选择另存副本或重新载入。
          </DialogDescription>
          <div className="grid max-h-[60dvh] gap-4 overflow-auto md:grid-cols-2">
            <section>
              <h3>最新 v{f.version}</h3>
              <pre className="whitespace-pre-wrap break-words bg-muted p-3 text-xs">
                {noteBody(s, f.id, f.version)}
              </pre>
            </section>
            <section>
              <h3>你的未保存草稿</h3>
              <pre className="whitespace-pre-wrap break-words bg-accent p-3 text-xs">
                {n?.draft}
              </pre>
            </section>
          </div>
        </DialogContent>
      </Dialog>
      <Dialog open={reload} onOpenChange={setReload}>
        <DialogContent>
          <DialogTitle>放弃当前草稿并重新载入？</DialogTitle>
          <DialogDescription>
            将使用最新版本。可先另存副本保留你的修改。
          </DialogDescription>
          <Btn
            onClick={() => {
              dispatch({ type: "k-reload", id: f.id });
              onVersion(f.version);
              setReload(false);
            }}
          >
            确认重新载入
          </Btn>
        </DialogContent>
      </Dialog>
      <Dialog open={!!copyName} onOpenChange={(v) => !v && setCopyName("")}>
        <DialogContent>
          <DialogTitle>另存 Markdown 副本</DialogTitle>
          <DialogDescription>
            保存到当前空间知识库，保留原文件。
          </DialogDescription>
          <Input
            aria-label="副本文档名称"
            value={copyName}
            onChange={(e) => setCopyName(e.target.value)}
          />
          <Btn
            className="mt-4"
            onClick={() => {
              dispatch({ type: "k-new", name: copyName, body, copyId: f.id });
              setCopyName("");
            }}
          >
            保存副本
          </Btn>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function minimalDocumentChange(before: string, after: string) {
  let from = 0;
  while (
    from < before.length &&
    from < after.length &&
    before[from] === after[from]
  )
    from++;
  let end = before.length,
    tail = after.length;
  while (end > from && tail > from && before[end - 1] === after[tail - 1]) {
    end--;
    tail--;
  }
  return { from, to: end, insert: after.slice(from, tail) };
}
