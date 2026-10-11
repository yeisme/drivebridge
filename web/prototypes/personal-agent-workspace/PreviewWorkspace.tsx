import { PreviewFileList } from "./PreviewFileList";
import { MarkdownEditor } from "./MarkdownEditor";
import { isMarkdown } from "./knowledge-model";
import type { RefObject } from "react";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import {
  ArrowLeft,
  ArrowRight,
  Check,
  Download,
  List,
  Maximize2,
  Minimize2,
  PanelRight,
  Pin,
  PinOff,
  Share2,
  X,
} from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "./Dialog";
import { cn } from "@/lib/utils";
import { Btn, Choice, Empty, IconBtn, Pill, useDemo } from "./ui";
import { PreviewContent } from "./PreviewContent";
import { PreviewDetails } from "./PreviewDetails";
import type { DetailTab } from "./PreviewDetails";
import { defaultViewer, previewUnavailable, refKey } from "./preview-model";
import type { PreviewRef, PreviewSession, ViewerState } from "./preview-model";

export function PreviewWorkspace({
  session,
  readers,
  onChange,
  onExit,
}: {
  session: PreviewSession;
  readers: RefObject<Record<string, ViewerState>>;
  onChange: (s: PreviewSession) => void;
  onExit: () => void;
}) {
  const { state: s, dispatch, open } = useDemo();
  const root = useRef<HTMLElement>(null);
  const list = useRef<HTMLDivElement>(null);
  const listScroll = useRef(0);
  const resize = useRef(false);
  const [width, setWidth] = useState(0);
  const [listWidth, setListWidth] = useState(s.preferences.previewListWidth);
  const [details, setDetails] = useState(false);
  const [listOpen, setListOpen] = useState(false);
  const [listVisible, setListVisible] = useState(true);

  const [tabs, setTabs] = useState<Record<string, DetailTab>>({});
  const scopedFile = `${s.actor}:${s.space}:${session.active.id}`;
  const tab = tabs[scopedFile] || "info";
  const setTab = (value: DetailTab) =>
    setTabs((t) => ({ ...t, [scopedFile]: value }));
  const [focus, setFocus] = useState(false);
  const [query, setQuery] = useState("");
  const [operations, setOperations] = useState(false);
  const [editorTools, setEditorTools] = useState<HTMLDivElement | null>(null);
  const inspectorScroll = useRef<Record<string, number>>({});
  const inspectorKey = `${scopedFile}-${tab}`;
  const [archiveFolder, setArchiveFolder] = useState(
    session.archiveFolder || "山野计划",
  );
  const [viewer, setViewer] = useState<Record<string, ViewerState>>(
    () => readers.current,
  );
  useLayoutEffect(() => {
    readers.current = viewer;
  }, [viewer]);
  const current = session.active;
  const key = `${s.actor}:${s.space}:${refKey(current)}`;
  const f = s.files.find((f) => f.id === current.id);
  const error =
    previewUnavailable(s, current) ||
    (!s.online &&
    f?.offlineVersion !== current.version &&
    !(f && isMarkdown(f) && s.knowledge.notes[f.id]?.versions[current.version])
      ? "该版本尚未离线保存，连接网络后再下载。"
      : "");
  const canPin = width >= 1100;
  const right = canPin && s.preferences.inspectorPosition === "right";
  const inlineList = width >= 760 && listVisible && !focus;
  const overlayDetails = width > 0 && width < 760;
  const maxList = Math.max(
    200,
    Math.min(320, width - 440 - (right && details && !focus ? 320 : 0) - 6),
  );
  const actualList = Math.min(listWidth, maxList);
  const detailsVisible = details && !focus;
  const contentColumn = inlineList ? 3 : 1;

  const closeDetails = () => {
    setDetails(false);

    requestAnimationFrame(() => {
      const button = root.current?.querySelector<HTMLButtonElement>(
        '[aria-label="显示文件详情"]',
      );
      (button?.getClientRects().length
        ? button
        : root.current?.querySelector<HTMLButtonElement>(
            '[aria-label="文件操作"]',
          )
      )?.focus({ preventScroll: true });
    });
  };
  const index = session.cursor;
  const view = viewer[key] || { ...defaultViewer };
  const updateView = (patch: Partial<ViewerState>) =>
    setViewer((v) => ({
      ...v,
      [key]: { ...(v[key] || defaultViewer), ...patch },
    }));
  const select = (r: PreviewRef) => {
    onChange({ ...session, active: r, cursor: session.scope.refs.indexOf(r) });
    setListOpen(false);
  };
  const version = (v: number) => {
    onChange({ ...session, active: { ...current, version: v } });
  };
  const inspectVersion = (v: number) => {
    if (f && isMarkdown(f)) dispatch({ type: "k-inspect", id: f.id });
    version(v);
  };
  useLayoutEffect(() => {
    const el = root.current!;
    const o = new ResizeObserver((entries) =>
      setWidth(entries[0].contentRect.width),
    );
    o.observe(el);
    return () => o.disconnect();
  }, []);

  useEffect(() => {
    root.current
      ?.querySelector<HTMLButtonElement>('[aria-label="退出预览"]')
      ?.focus({ preventScroll: true });
  }, []);
  useEffect(() => {
    list.current
      ?.querySelector('[aria-current="true"]')
      ?.scrollIntoView({ block: "nearest" });
  }, [key]);
  useEffect(() => {
    const listener = (e: KeyboardEvent) => {
      if (
        e.defaultPrevented ||
        !!document.querySelector("[data-floating-navigation]") ||
        (e.target instanceof HTMLElement &&
          e.target.closest("[data-global-agent]")) ||
        Array.from(
          document.querySelectorAll(
            '[role="dialog"],[role="menu"],[role="listbox"]',
          ),
        ).some(
          (el) =>
            el.getClientRects().length > 0 && !el.hasAttribute("data-closed"),
        )
      )
        return;
      if (e.key === "Escape") {
        e.preventDefault();
        e.stopPropagation();
        if (focus) setFocus(false);
        else if (detailsVisible) closeDetails();
        else onExit();
      }
      if (
        e.target instanceof HTMLElement &&
        e.target.closest("input,textarea,select,[contenteditable]")
      )
        return;
      if (e.key === "ArrowDown" || e.key === "ArrowUp") {
        if (
          !(e.target instanceof HTMLElement) ||
          !e.target.closest("[data-preview-list]")
        )
          return;
        const matching = session.scope.refs.filter(
          (r) =>
            !query ||
            (s.files.find((f) => f.id === r.id)?.name || "")
              .toLocaleLowerCase()
              .includes(query.toLocaleLowerCase()),
        );
        const at = matching.indexOf(session.scope.refs[index]);
        const next = matching[at + (e.key === "ArrowDown" ? 1 : -1)];
        if (next) {
          e.preventDefault();
          select(next);
          requestAnimationFrame(() =>
            list.current
              ?.querySelector<HTMLButtonElement>('[aria-current="true"]')
              ?.focus({ preventScroll: true }),
          );
        }
      }
    };
    window.addEventListener("keydown", listener, true);
    return () => window.removeEventListener("keydown", listener, true);
  }, [focus, detailsVisible, overlayDetails, session, query]);
  const restoreList = () => {
    setListWidth(220);
    dispatch({ type: "preferences", patch: { previewListWidth: 220 } });
  };
  const listBody = () => (
    <PreviewFileList
      session={session}
      query={query}
      onQuery={setQuery}
      onSelect={select}
      listRef={list}
      scroll={listScroll}
    />
  );
  const inspector = (
    <div className="flex h-full min-h-0 flex-col bg-card" aria-label="预览详情">
      <div className="flex h-11 shrink-0 items-center gap-1 border-b px-2">
        <span
          className="min-w-0 flex-1 truncate text-xs font-medium"
          title={f?.name}
        >
          {error ? "文件详情" : f?.name}
        </span>
        <IconBtn
          label={
            s.preferences.inspectorPosition === "right"
              ? "取消固定详情"
              : "固定到右侧"
          }
          disabled={!canPin && s.preferences.inspectorPosition !== "right"}
          title={
            !canPin
              ? "页面宽度不足 1100px，使用覆盖详情以保留正文空间"
              : "固定或取消固定"
          }
          onClick={() =>
            dispatch({
              type: "preferences",
              patch: {
                inspectorPosition:
                  s.preferences.inspectorPosition === "right"
                    ? "auto"
                    : "right",
              },
            })
          }
        >
          {s.preferences.inspectorPosition === "right" ? (
            <PinOff size={15} />
          ) : (
            <Pin size={15} />
          )}
        </IconBtn>
        <IconBtn label="收起详情" onClick={closeDetails}>
          <X size={16} />
        </IconBtn>
      </div>
      <div className="flex h-11 shrink-0 items-center gap-1 border-b border-border px-2">
        {(
          [
            { id: "info", label: "详情" },
            { id: "comments", label: "讨论" },
            { id: "history", label: "版本" },
            { id: "activity", label: "活动" },
            { id: "links", label: "关联" },
          ] as const
        ).map((x) => (
          <Btn
            key={x.id}
            variant={tab === x.id ? "secondary" : "ghost"}
            className="min-w-0 flex-1 !px-1 text-xs"
            aria-pressed={tab === x.id}
            onClick={() => setTab(x.id)}
          >
            {x.label}
          </Btn>
        ))}
      </div>
      <div
        key={inspectorKey}
        ref={(el) => {
          if (el) el.scrollTop = inspectorScroll.current[inspectorKey] || 0;
        }}
        onScroll={(e) => {
          inspectorScroll.current[inspectorKey] = e.currentTarget.scrollTop;
        }}
        className="min-h-0 flex-1 overflow-y-auto overscroll-contain"
        data-inspector-scroll
      >
        <PreviewDetails
          key={current.id}
          reference={current}
          tab={tab}
          onVersion={inspectVersion}
          discussionDraft={view.discussion}
          onDiscussionDraft={(discussion) => updateView({ discussion })}
        />
      </div>
    </div>
  );
  return (
    <section
      ref={root}
      role="region"
      aria-label="文件预览工作台"
      className="relative flex h-full min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-background"
      data-preview-workspace
    >
      <header
        className="flex h-11 shrink-0 items-center gap-1 overflow-hidden border-b border-border bg-card px-2"
        data-preview-toolbar
      >
        <IconBtn label="退出预览" onClick={onExit}>
          <ArrowLeft size={18} />
        </IconBtn>
        <div className="hidden min-w-0 flex-1 sm:block">
          <h1
            className="truncate text-sm font-semibold"
            title={!error ? f?.name : undefined}
          >
            {error ? "文件预览不可用" : f?.name}
          </h1>
          <p className="truncate text-[10px] text-muted-foreground">
            {session.scope.label} · {Math.max(index + 1, 1)} /{" "}
            {session.scope.refs.length} ·{" "}
            {current.externalRevision ? "固定外部修订" : `v${current.version}`}
          </p>
        </div>
        <div className="hidden shrink-0 items-center gap-1 min-[1024px]:flex">
          <IconBtn
            label="上一份"
            disabled={index <= 0}
            onClick={() => select(session.scope.refs[index - 1])}
          >
            <ArrowLeft size={15} />
          </IconBtn>
          <IconBtn
            label="下一份"
            disabled={index < 0 || index >= session.scope.refs.length - 1}
            onClick={() => select(session.scope.refs[index + 1])}
          >
            <ArrowRight size={15} />
          </IconBtn>
          {!error && f && !current.externalRevision ? (
            <Choice
              label="查看版本"
              className="!w-auto shrink-0 text-xs"
              value={String(current.version)}
              options={Array.from({ length: f.version }, (_, i) => ({
                value: String(f.version - i),
                label: `v${f.version - i}${i === 0 ? " · 最新" : " · 历史"}`,
              }))}
              onChange={(v) => inspectVersion(Number(v))}
            />
          ) : (
            <Pill>固定修订</Pill>
          )}
        </div>
        <div
          ref={setEditorTools}
          className="flex min-w-0 flex-1 items-center overflow-x-auto empty:hidden"
        />
        <IconBtn
          className={f && isMarkdown(f) && width < 760 ? "hidden" : ""}
          label={focus ? "退出专注预览" : "专注预览"}
          aria-pressed={focus}
          onClick={() => setFocus(!focus)}
        >
          {focus ? <Minimize2 size={16} /> : <Maximize2 size={16} />}
        </IconBtn>
        <Btn
          variant="ghost"
          className={f && isMarkdown(f) && width < 760 ? "hidden" : ""}
          aria-label="显示文件列表"
          aria-pressed={width >= 760 ? listVisible : listOpen}
          onClick={() =>
            width >= 760 ? setListVisible(!listVisible) : setListOpen(true)
          }
        >
          <List size={17} />
          <span className="hidden min-[1024px]:inline">文件列表</span>
        </Btn>
        <Btn
          variant="ghost"
          className={f && isMarkdown(f) && width < 760 ? "hidden" : ""}
          aria-label="显示文件详情"
          aria-pressed={details}
          onClick={() => {
            detailsVisible ? closeDetails() : setDetails(true);
          }}
        >
          <PanelRight size={17} />
          <span className="hidden min-[1024px]:inline">文件详情</span>
        </Btn>
        <IconBtn label="文件操作" onClick={() => setOperations(true)}>
          <span aria-hidden="true">•••</span>
        </IconBtn>
      </header>
      <div
        data-page-preview-body
        className="relative grid min-h-0 flex-1 overflow-hidden"
        style={{
          gridTemplateColumns: `${inlineList ? `${actualList}px 6px ` : ""}minmax(0,1fr)${right && detailsVisible ? " 320px" : ""}`,
          gridTemplateRows: "minmax(0,1fr)",
        }}
      >
        {inlineList && (
          <>
            <aside
              style={{ width: actualList, gridRow: "1 / -1" }}
              className="min-h-0 shrink-0 overflow-hidden border-r border-border"
              aria-label="预览文件导航"
            >
              {listBody()}
            </aside>
            <div
              style={{ gridRow: "1 / -1" }}
              role="separator"
              tabIndex={0}
              aria-label="调整文件列表宽度"
              aria-orientation="vertical"
              aria-valuemin={200}
              aria-valuemax={maxList}
              aria-valuenow={Math.round(actualList)}
              className="z-10 w-1.5 shrink-0 touch-none cursor-col-resize bg-border/40 hover:bg-primary/40 focus:bg-primary/40"
              onDoubleClick={restoreList}
              onKeyDown={(e) => {
                const w =
                  e.key === "ArrowLeft"
                    ? actualList - 16
                    : e.key === "ArrowRight"
                      ? actualList + 16
                      : e.key === "Home"
                        ? 200
                        : e.key === "End"
                          ? maxList
                          : null;
                if (w !== null) {
                  e.preventDefault();
                  setListWidth(Math.max(200, Math.min(maxList, w)));
                  dispatch({
                    type: "preferences",
                    patch: {
                      previewListWidth: Math.max(200, Math.min(maxList, w)),
                    },
                  });
                }
              }}
              onPointerDown={(e) => {
                resize.current = true;
                e.currentTarget.setPointerCapture(e.pointerId);
              }}
              onPointerMove={(e) => {
                if (resize.current && root.current)
                  setListWidth(
                    Math.max(
                      200,
                      Math.min(
                        maxList,
                        e.clientX - root.current.getBoundingClientRect().left,
                      ),
                    ),
                  );
              }}
              onPointerUp={() => {
                resize.current = false;
                dispatch({
                  type: "preferences",
                  patch: { previewListWidth: Math.round(actualList) },
                });
              }}
              onPointerCancel={() => {
                resize.current = false;
              }}
            />
          </>
        )}
        <div
          style={{ gridColumn: contentColumn, gridRow: 1 }}
          className="flex min-h-0 min-w-0 flex-col"
        >
          <div
            data-preview-stage
            className="min-h-0 min-w-0 flex-1 overflow-hidden"
          >
            {error || !f ? (
              <Empty
                title="此文件当前不可见"
                text={error || "演示会话已重置，文件不存在。"}
              />
            ) : isMarkdown(f) ? (
              <MarkdownEditor
                toolbar={editorTools}
                key={f.id}
                file={f}
                version={current.version}
                onVersion={version}
                anchor={session.anchor}
              />
            ) : s.knowledge.notes[f.id] ? (
              <div data-reader className="h-full overflow-auto bg-card p-5">
                <p className="mb-4 text-xs text-muted-foreground">
                  源码只读 · 演示资源 · 不执行脚本
                </p>
                <pre className="whitespace-pre-wrap break-words font-mono text-sm leading-7">
                  {s.knowledge.notes[f.id].versions[current.version]}
                </pre>
              </div>
            ) : (
              <PreviewContent
                key={key}
                file={f}
                version={current.version}
                view={view}
                change={updateView}
                toolbar={editorTools}
              />
            )}
          </div>
        </div>
        {!overlayDetails && (
          <aside
            hidden={!detailsVisible}
            aria-label="页面文件详情"
            data-inspector-dock={right ? "right" : "overlay"}
            style={
              right
                ? { gridColumn: contentColumn + 1, gridRow: 1 }
                : {
                    position: "absolute",
                    top: 8,
                    bottom: 8,
                    right: 8,
                    width: 320,
                    maxWidth: "calc(100% - 16px)",
                    zIndex: 30,
                  }
            }
            className={cn(
              "min-h-0 min-w-0 overflow-hidden border-border bg-card",
              right ? "border-l" : "rounded-lg border shadow-lg",
            )}
          >
            {inspector}
          </aside>
        )}
        <Dialog
          open={overlayDetails && detailsVisible}
          onOpenChange={(value) => !value && closeDetails()}
        >
          <DialogContent
            portalContainer={root}
            backdropClassName="!absolute !top-11 !bg-transparent"
            showCloseButton={false}
            className="!absolute !bottom-0 !left-auto !right-0 !top-11 !h-[calc(100%-44px)] !max-h-none !w-full !max-w-none !translate-x-0 !translate-y-0 !gap-0 !rounded-none !p-0"
            data-inspector-dock="full"
          >
            <DialogTitle className="sr-only">页面文件详情</DialogTitle>
            <DialogDescription className="sr-only">
              固定版本详情，从右侧覆盖显示。
            </DialogDescription>
            {overlayDetails && inspector}
          </DialogContent>
        </Dialog>
      </div>
      <Dialog open={operations} onOpenChange={setOperations}>
        <DialogContent>
          <DialogTitle>文件操作</DialogTitle>
          <DialogDescription>{f?.name} · 当前固定版本</DialogDescription>
          <div
            className="flex flex-wrap gap-2"
            onClick={(e) => {
              const button = (e.target as HTMLElement).closest("button");
              if (button && button.getAttribute("role") !== "combobox")
                setOperations(false);
            }}
          >
            <Btn
              variant="outline"
              onClick={() => {
                setDetails(true);
              }}
            >
              显示文件详情
            </Btn>
            <Btn
              variant="outline"
              onClick={() =>
                width < 760 ? setListOpen(true) : setListVisible(!listVisible)
              }
            >
              显示文件列表
            </Btn>
            <Btn variant="outline" onClick={() => setFocus(!focus)}>
              {focus ? "退出专注预览" : "专注预览"}
            </Btn>
            {f?.inbox ? (
              <>
                <Choice
                  label="验收归档目录"
                  value={archiveFolder}
                  onChange={setArchiveFolder}
                  options={s.folders
                    .filter(
                      (x) => x.space === s.space && x.connectionId === "drive",
                    )
                    .map((x) => ({ value: x.path, label: x.path }))}
                />
                <Btn
                  variant="outline"
                  disabled={!!error}
                  onClick={() =>
                    open({
                      type: "danger",
                      title: "退回这份产物？",
                      text: "移入回收站，可恢复。",
                      action: { type: "reject", id: f.id },
                    })
                  }
                >
                  退回
                </Btn>
                <Btn
                  disabled={
                    !!error ||
                    s.tasks.some(
                      (t) =>
                        t.kind === "upload" &&
                        t.items.some((i) => i.fileId === f.id) &&
                        t.status !== "done",
                    )
                  }
                  onClick={() =>
                    dispatch({
                      type: "accept",
                      id: f.id,
                      folder: archiveFolder,
                    })
                  }
                >
                  <Check size={15} />
                  接收并归档
                </Btn>
              </>
            ) : (
              <Btn
                variant="outline"
                disabled={!!error || !s.online}
                onClick={() =>
                  dispatch({
                    type: "offline",
                    id: current.id,
                    version: current.version,
                  })
                }
              >
                <Download size={15} />
                离线保存
              </Btn>
            )}
            <Btn
              variant="outline"
              disabled={!!error}
              onClick={() =>
                open({
                  type: "compare",
                  ids: [current.id],
                  version: current.version,
                })
              }
            >
              比较版本
            </Btn>
            <Btn
              disabled={!!error}
              onClick={() =>
                open({
                  type: "delivery",
                  id: current.id,
                  version: current.version,
                })
              }
            >
              <Share2 size={15} />
              交付此版本
            </Btn>
            <Btn
              variant="ghost"
              disabled={!!error}
              onClick={() => {
                setDetails(true);
                setTab("info");
              }}
            >
              链接与引用
            </Btn>
            <Btn
              variant="ghost"
              onClick={() => {
                setOperations(false);
                setTab("info");
                setDetails(true);

                requestAnimationFrame(() =>
                  document
                    .querySelector("[data-file-properties]")
                    ?.scrollIntoView({ block: "start" }),
                );
              }}
            >
              文件属性
            </Btn>
          </div>
        </DialogContent>
      </Dialog>
      <Dialog modal={false} open={listOpen} onOpenChange={setListOpen}>
        <DialogContent
          portalContainer={root}
          backdropClassName="!absolute"
          className="!absolute !bottom-0 !left-0 !top-11 !h-[calc(100%-44px)] !max-h-full !w-full !max-w-[320px] !translate-x-0 !translate-y-0 !rounded-none !p-0"
        >
          <div className="p-4">
            <DialogTitle>本次预览文件列表</DialogTitle>
            <DialogDescription>
              {session.scope.label} · {session.scope.refs.length} 项固定引用
            </DialogDescription>
          </div>
          <div className="min-h-0 flex-1">{listBody()}</div>
        </DialogContent>
      </Dialog>
    </section>
  );
}
