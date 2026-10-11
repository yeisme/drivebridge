import { useFloatingNavigation } from "./useFloatingNavigation";
import { FavoritesWorkspace } from "./FavoritesWorkspace";
import { FileSearchWorkspace } from "./FileSearchWorkspace";
import { StorageRuntime } from "./CloudExplorer";
import { seedKnowledge } from "./knowledge-model";
import { KnowledgeRuntime } from "./KnowledgeRuntime";
import {
  KnowledgeWorkspace,
  KnowledgeGraph,
  KnowledgeHome,
} from "./KnowledgeWorkspace";
import { ResourceWorkspace } from "./ResourceWorkspace";
import { HoverPreviewProvider } from "./HoverPreview";
import { Navigation } from "./Navigation";
import { PreviewWorkspace } from "./PreviewWorkspace";
import {
  createPreviewSession,
  parsePreviewFragment,
  previewFragment,
} from "./preview-model";
import type { PreviewSession, ViewerState } from "./preview-model";
import { SurfaceVisible } from "./Dialog";
import { StorageWorkspace } from "./StorageWorkspace";
import { CollaborationWorkspace } from "./CollaborationWorkspace";

import { Tags } from "./TagWorkspace";

import { SettingsWorkspace } from "./CloudSettings";
import { ExternalConnections, Capabilities } from "./ConnectionsWorkspace";
import { preferenceKey, safePreferences } from "./cloud-model";
import { BatchActions } from "./CloudToolbar";
import React, { useEffect, useReducer, useRef, useState } from "react";
import {
  BookOpen,
  Library,
  ArrowLeft,
  ArrowRight,
  Bell,
  Bot,
  ChevronDown,
  CircleHelp,
  Cloud,
  Folder,
  HardDrive,
  Home as HomeIcon,
  Inbox,
  Laptop,
  LayoutList,
  ListTodo,
  LoaderCircle,
  Menu,
  Moon,
  PanelLeftOpen,
  PanelLeftClose,
  PanelRightClose,
  PanelRightOpen,
  Plus,
  RefreshCw,
  Search,
  Settings,
  Share2,
  ShieldCheck,
  Smartphone,
  Sparkles,
  Sun,
  Tag,
  Trash2,
  UserRound,
  Users,
  WifiOff,
  X,
  Zap,
} from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "./Dialog";
import { BrandMark } from "@/BrandMark";
import { cn } from "@/lib/utils";
import { initialState, reducer, scenarioNames } from "./model";
import type { Page, ViewState, Space, State } from "./model";
import { Btn, Choice, Demo, Empty, Heading, IconBtn, panel, Pill } from "./ui";
import type { Modal } from "./ui";
import { InboxPage } from "./WorkspacePages";
import { Files, Home, Collections } from "./CreativeWorkspace";
import { Agents, Automation, Devices, Shares, Tasks } from "./TaskPages";
import { ModalBody } from "./Modals";

const pages: {
  id: Page;
  title: string;
  icon: typeof HomeIcon;
  group: string;
}[] = [
  { id: "home", title: "我的工作台", icon: HomeIcon, group: "工作台" },
  { id: "files", title: "我的云盘", icon: Folder, group: "工作台" },
  { id: "knowledge", title: "知识库", icon: BookOpen, group: "工作台" },
  { id: "resources", title: "资源库", icon: Library, group: "工作台" },
  { id: "knowledge-graph", title: "知识关系", icon: Share2, group: "工作台" },
  { id: "collections", title: "项目集合", icon: LayoutList, group: "工作台" },
  { id: "tags", title: "标签与属性", icon: Tag, group: "工作台" },
  { id: "search", title: "搜索", icon: Search, group: "工作台" },
  { id: "tasks", title: "任务中心", icon: ListTodo, group: "工作台" },
  { id: "inbox", title: "产物与收件", icon: Inbox, group: "协作" },
  { id: "collaboration", title: "共享与协作", icon: Users, group: "协作" },
  { id: "shares", title: "分享与交付", icon: Share2, group: "协作" },
  { id: "agents", title: "外部调用", icon: Bot, group: "协作" },
  { id: "connections", title: "存储管理", icon: Cloud, group: "管理" },
  { id: "devices", title: "设备与离线", icon: Laptop, group: "管理" },
  { id: "automation", title: "备份与整理", icon: Zap, group: "管理" },
  { id: "trash", title: "回收站", icon: Trash2, group: "管理" },
  {
    id: "capabilities",
    title: "能力与连接说明",
    icon: CircleHelp,
    group: "管理",
  },
];
const states: { value: ViewState; label: string }[] = [
  { value: "ready", label: "正常状态" },
  { value: "loading", label: "加载中" },
  { value: "empty", label: "空数据" },
  { value: "forbidden", label: "无权限" },
  { value: "offline", label: "离线" },
  { value: "disconnected", label: "连接失效" },
  { value: "partial", label: "部分完成" },
  { value: "stale", label: "计划过期" },
  { value: "unknown", label: "结果未知" },
  { value: "unsupported", label: "不支持" },
  { value: "error", label: "加载失败" },
];

export function App() {
  const [s, dispatch] = useReducer(reducer, undefined, () => {
    const n = initialState();
    try {
      n.preferences = safePreferences(
        JSON.parse(localStorage.getItem(preferenceKey) || "null"),
      );
    } catch {}
    n.page = n.preferences.startup;
    n.browser.layout = n.preferences.layout;
    n.agentOpen = false;
    const initialLink = parsePreviewFragment(location.hash);
    if (
      initialLink?.ref.id.startsWith("k-") ||
      initialLink?.ref.id.startsWith("kt-")
    ) {
      n.space = initialLink.space;
      seedKnowledge(n);
    }
    return n;
  });
  const [mode, setMode] = useState("auto");
  const [width, setWidth] = useState(window.innerWidth);
  const [systemDark, setSystemDark] = useState(
    matchMedia("(prefers-color-scheme: dark)").matches,
  );
  const dark =
    s.preferences.theme === "dark" ||
    (s.preferences.theme === "system" && systemDark);
  const setDark = (value: boolean) =>
    dispatch({
      type: "preferences",
      patch: { theme: value ? "dark" : "light" },
    });
  useEffect(() => {
    const media = matchMedia("(prefers-color-scheme: dark)");
    const update = () => setSystemDark(media.matches);
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);
  useEffect(() => {
    try {
      localStorage.setItem(preferenceKey, JSON.stringify(s.preferences));
    } catch {}
    document.documentElement.dataset.reducedMotion = String(
      s.preferences.reduceMotion,
    );
  }, [s.preferences]);
  const [scenario, setScenario] = useState("0");
  const [modal, setModal] = useState<Modal | null>(null);
  const [preview, setPreview] = useState<PreviewSession | null>(null);
  const previewReturnFocus = useRef<HTMLElement | null>(null);
  const previewReturnHash = useRef("");

  const previewReaders = useRef<Record<string, ViewerState>>({});
  const previewStack = useRef<PreviewSession[]>([]);
  const navigationDirty = useRef(false);
  const [pendingNavigation, setPendingNavigation] = useState<{
    page?: Page;
    space?: Space;
  } | null>(null);
  const switchSpace = (space: Space) => {
    if (navigationDirty.current) {
      setPendingNavigation({ space });
      return;
    }
    dispatch({ type: "space", space });
  };
  const returnFocus = useRef<HTMLElement | null>(null);
  const viewport = useRef<HTMLDivElement>(null);
  const scroll = useRef<HTMLElement>(null);
  const compact = width < 768;
  const [demoOpen, setDemoOpen] = useState(false);
  const [fileHub, setFileHub] = useState(false);
  const floatingNav = useFloatingNavigation({
    docked: width >= 1280 && !s.preferences.navCollapsed,
    disabled: !!modal || demoOpen || fileHub || !!pendingNavigation,
    width,
    scope: `${s.space}:${s.page}`,
  });
  const [composerActive, setComposerActive] = useState(false);
  useEffect(() => {
    const measure = () =>
      document.documentElement.style.setProperty(
        "--prototype-height",
        `${window.visualViewport?.height || window.innerHeight}px`,
      );
    measure();
    window.visualViewport?.addEventListener("resize", measure);
    window.addEventListener("resize", measure);
    return () => {
      window.visualViewport?.removeEventListener("resize", measure);
      window.removeEventListener("resize", measure);
      document.documentElement.style.removeProperty("--prototype-height");
    };
  }, []);
  const exitPreview = () => {
    const previous = previewStack.current.pop();
    if (previous) {
      if (previous.documentView)
        dispatch({
          type: "k-restore-view",
          id: previous.active.id,
          ...previous.documentView,
        });
      setPreview(previous);
      return;
    }
    const top = preview?.returnScroll || 0;
    setPreview(null);
    history.replaceState(
      null,
      "",
      location.pathname + location.search + previewReturnHash.current,
    );
    requestAnimationFrame(() => {
      if (scroll.current) scroll.current.scrollTop = top;
      previewReturnFocus.current?.focus({ preventScroll: true });
    });
  };
  const open = (value: Modal) => {
    if (value.type === "preview") {
      dispatch({ type: "entry-visit", id: value.id });
      if (!value.edit && s.knowledge.notes[value.id])
        dispatch({ type: "k-inspect", id: value.id });
      if (!preview) {
        previewReturnFocus.current = document.activeElement as HTMLElement;
        previewReturnHash.current = location.hash;
        previewStack.current = [];
      } else {
        const note = s.knowledge.notes[preview.active.id];
        previewStack.current.push({
          ...preview,
          documentView: note
            ? { mode: note.mode, showDraft: note.showDraft }
            : undefined,
        });
      }
      setModal(null);

      setPreview(
        createPreviewSession(
          s,
          value,
          preview?.returnScroll ?? scroll.current?.scrollTop ?? 0,
        ),
      );
      return;
    }
    if (value.type === "help") {
      setModal(null);
      dispatch({ type: "navigate", page: "capabilities" });
      return;
    }
    if (!modal) returnFocus.current = document.activeElement as HTMLElement;
    setModal(value);
  };
  const close = () => setModal(null);
  const go = (page: Page) => {
    const legacyAssistant = page === "assistant";
    if (page === "assistant") {
      page = "search";
      dispatch({
        type: "notice",
        text: "内置助手已移除，请使用检索或外部应用。",
      });
    }
    if (navigationDirty.current && page !== s.page) {
      setPendingNavigation({ page });
      return;
    }
    if (preview) {
      previewStack.current = [];
      setPreview(null);
      history.replaceState(
        null,
        "",
        location.pathname + location.search + previewReturnHash.current,
      );
    }
    if (page === "settings")
      dispatch({ type: "settings-section", section: "" });
    if (page === "search")
      dispatch({
        type: "search-patch",
        patch: {
          folder: "全部文件",
          connections: [
            s.activeConnection === "all" ? "drive" : s.activeConnection,
          ],
          directoryId: s.storage.nodes.find(
            (n) =>
              n.id === s.options.searchFolder &&
              n.space === s.space &&
              n.connectionId === s.activeConnection,
          )?.id,
        },
      });

    if (["knowledge", "resources", "knowledge-graph"].includes(page))
      dispatch({ type: "k-seed" });
    dispatch({ type: "navigate", page: legacyAssistant ? "assistant" : page });
  };
  useEffect(() => {
    const el = viewport.current;
    if (!el) return;
    const observer = new ResizeObserver((entries) =>
      setWidth(entries[0].contentRect.width),
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, []);
  useEffect(() => {
    document.documentElement.classList.toggle("dark", dark);
    return () => document.documentElement.classList.remove("dark");
  }, [dark]);
  useEffect(() => {
    scroll.current?.scrollTo({ top: 0 });
  }, [s.page]);
  useEffect(() => {
    setModal(null);
    previewReaders.current = {};
    if (preview && preview.space !== s.space) {
      previewStack.current = [];
      setPreview(null);
    }
  }, [s.space]);
  useEffect(() => {
    if (preview && s.page !== preview.origin) {
      setPreview(null);
      previewStack.current = [];
      history.replaceState(
        null,
        "",
        location.pathname + location.search + previewReturnHash.current,
      );
    }
  }, [s.page]);
  const latestState = useRef(s);
  latestState.current = s;
  useEffect(() => {
    const read = () => {
      const s = latestState.current;
      return readHash(s);
    };
    const readHash = (s: State) => {
      const parsed = parsePreviewFragment(location.hash);
      if (!parsed) {
        if (location.hash.startsWith("#f/"))
          dispatch({ type: "notice", text: "定位链接无效，未打开其他文件。" });
        return;
      }
      if (parsed.space !== s.space)
        dispatch({ type: "space", space: parsed.space });
      const file = s.files.find((f) => f.id === parsed.ref.id);
      const active = { ...parsed.ref, connectionId: file?.connectionId };
      previewReturnHash.current = "";
      previewStack.current = [];
      setPreview({
        scope: { label: "定位链接", refs: [active] },
        active,
        cursor: 0,
        space: parsed.space,
        origin: s.page,
        returnScroll: 0,
      });
    };
    read();
    window.addEventListener("hashchange", read);
    return () => window.removeEventListener("hashchange", read);
  }, []);
  useEffect(() => {
    if (preview)
      history.replaceState(
        null,
        "",
        location.pathname +
          location.search +
          previewFragment(preview.space, preview.active),
      );
  }, [preview?.active]);
  const indexClock = s.indexes.map((i) => `${i.id}:${i.status}`).join("|");
  useEffect(() => {
    if (!s.online || s.viewState === "forbidden") return;
    const timers = s.indexes
      .filter(
        (i) =>
          i.space === s.space && ["queued", "processing"].includes(i.status),
      )
      .map((i) =>
        setTimeout(
          () =>
            dispatch({
              type: "index-status",
              id: i.id,
              status: i.status === "queued" ? "processing" : "partial",
            }),
          i.status === "queued" ? 500 : 1100,
        ),
      );
    return () => timers.forEach(clearTimeout);
  }, [indexClock, s.space, s.online, s.viewState]);
  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      if (
        e.key === "Escape" &&
        document.querySelector("[data-floating-navigation]")
      )
        return;
      if (
        e.key === "Escape" &&
        document.querySelector('[data-batch-menu-open="true"]')
      )
        return;
      if (
        e.key === "Escape" &&
        e
          .composedPath()
          .some(
            (el) =>
              el instanceof HTMLElement &&
              (el.matches(
                '[role="menu"], [role="dialog"], [data-slot="dropdown-menu-content"]',
              ) ||
                el.closest('[role="menu"], [role="dialog"]')),
          )
      )
        return;
      if (
        !preview &&
        !e.defaultPrevented &&
        !(
          e.target instanceof HTMLElement &&
          e.target.closest("[data-global-agent]")
        ) &&
        e.key === "Escape" &&
        !Array.from(
          document.querySelectorAll('[role="dialog"], [role="menu"]'),
        ).some(
          (el) =>
            el.getClientRects().length > 0 && !el.hasAttribute("data-closed"),
        )
      ) {
        if (s.selected.length) dispatch({ type: "select", ids: [] });
      }
    };
    window.addEventListener("keydown", key, true);
    return () => window.removeEventListener("keydown", key, true);
  }, [s.selected.length, s.agentOpen, !!preview]);
  const taskClock = s.tasks
    .map((t) => `${t.id}:${t.status}:${t.runs}`)
    .join("|");
  useEffect(() => {
    if (!s.online || s.viewState === "forbidden") return;
    const timers = s.tasks
      .filter((t) => t.status === "running" && t.space === s.space)
      .map((t) =>
        setTimeout(() => dispatch({ type: "finish", id: t.id }), 1600),
      );
    return () => timers.forEach(clearTimeout);
  }, [taskClock, s.online, s.space, s.viewState]);
  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      if (
        e.key === "Escape" &&
        document.querySelector('[data-batch-menu-open="true"]')
      )
        return;
      if (
        e.key === "Escape" &&
        e
          .composedPath()
          .some(
            (el) =>
              el instanceof HTMLElement &&
              (el.matches(
                '[role="menu"], [role="dialog"], [data-slot="dropdown-menu-content"]',
              ) ||
                el.closest('[role="menu"], [role="dialog"]')),
          )
      )
        return;
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        returnFocus.current = document.activeElement as HTMLElement;
        setModal({ type: "command" });
      }
    };
    document.addEventListener("keydown", key);
    return () => document.removeEventListener("keydown", key);
  }, []);
  const pending = s.tasks.filter(
    (t) => t.space === s.space && t.status === "approval",
  ).length;
  const inboxCount = s.files.filter(
    (f) => f.space === s.space && f.inbox && !f.trashed,
  ).length;
  const reset = (v: string) => {
    if (
      Object.values(s.knowledge.notes).some((n) => n.dirty) &&
      !window.confirm("当前有未保存草稿，确定重置演示？")
    )
      return;
    setDemoOpen(false);
    setScenario(v);
    previewReaders.current = {};
    history.replaceState(null, "", location.pathname + location.search);
    previewStack.current = [];
    setPreview(null);
    dispatch({ type: "reset", scenario: Number(v) });
    setModal(null);
  };
  const missing = [
    "empty",
    "loading",
    "forbidden",
    "unsupported",
    "disconnected",
    "error",
  ].includes(s.viewState);
  const pageTitle =
    s.page === "favorites"
      ? "本盘收藏"
      : s.page === "settings"
        ? "设置"
        : pages.find((p) => p.id === s.page)?.title;
  const modalTitle =
    modal?.type === "preview"
      ? s.files.find((f) => f.id === modal.id && f.space === s.space)?.name ||
        "文件预览"
      : modal?.type === "danger"
        ? modal.title
        : (
            {
              confirm: "检查这次操作",
              upload: "把新文件带进来",
              consent: "确认本次内容分析",
              agent: "接入一个外部 Agent",
              rule: "配置范围与运行条件",
              command: "快速查找",
              "bookmark-groups": "选择收藏夹",
              "property-batch": "批量修改属性",
              notifications: "待处理事项",
              device: "关联你的设备",
              help: "能力与连接说明",
              label: "标签与集合",
              capacity: "存储用量",
              duplicates: "检查重复文件建议",
              delivery: "交付前，再确认一次",
              compare: "素材与版本比较",
              organize: "预览文件整理",
              collection: "管理项目集合",
              "new-folder": "新建文件夹",
              conversation: "命名这个会话",
              "edit-plan": "调整操作计划",
              conflict: "处理传输冲突",
            } as Record<string, string>
          )[modal?.type || ""];
  return (
    <Demo.Provider
      value={{
        state: s,
        dispatch,
        compact,
        open,
        close,
        go,
        guardNavigation: (dirty) => {
          navigationDirty.current = dirty;
        },
      }}
    >
      <HoverPreviewProvider>
        <KnowledgeRuntime />
        <StorageRuntime />
        <div
          className="flex h-[var(--prototype-height,100dvh)] min-h-0 flex-col overflow-hidden bg-background"
          onFocusCapture={(e) => {
            const target = e.target as HTMLElement;
            if (target.closest("[data-composer]")) {
              if (target.matches("textarea, input")) setComposerActive(true);
            } else setComposerActive(false);
          }}
          onBlurCapture={(e) => {
            if (
              !(e.relatedTarget as HTMLElement | null)?.closest(
                "[data-composer]",
              )
            )
              setComposerActive(false);
          }}
        >
          <Dialog open={fileHub} onOpenChange={setFileHub}>
            <DialogContent>
              <DialogTitle>文件与资料</DialogTitle>
              <DialogDescription>
                选择浏览方式；都使用原有文件。
              </DialogDescription>
              {[
                { id: "files", name: "我的云盘" },
                { id: "knowledge", name: "知识库" },
                { id: "resources", name: "资源库" },
              ].map((x) => (
                <Btn
                  key={x.id}
                  variant="outline"
                  onClick={() => {
                    setFileHub(false);
                    go(x.id as Page);
                  }}
                >
                  {x.name}
                </Btn>
              ))}
            </DialogContent>
          </Dialog>
          {s.notice && !modal && (
            <div
              role="status"
              className="pointer-events-none fixed bottom-20 left-1/2 z-40 max-w-[calc(100%-24px)] -translate-x-1/2 rounded-lg border border-border bg-card px-3 py-2 text-xs shadow-md"
            >
              {s.notice}
              {s.notice.startsWith("已加入本盘未分组") &&
                s.organization.lastBookmark && (
                  <Btn
                    variant="ghost"
                    className="pointer-events-auto !h-11 text-xs"
                    onClick={() =>
                      open({
                        type: "bookmark-groups",
                        refs: [s.organization.lastBookmark!],
                      })
                    }
                  >
                    选择收藏夹
                  </Btn>
                )}
            </div>
          )}
          <Dialog open={demoOpen} onOpenChange={setDemoOpen}>
            <DialogContent>
              <DialogTitle>演示与审阅</DialogTitle>
              <DialogDescription>
                仅影响本页模拟场景，可随时重置。
              </DialogDescription>
              <div
                className="z-20 flex shrink-0 flex-wrap items-center justify-between gap-2 border-b border-border bg-card px-3 py-2 text-xs sm:px-5"
                aria-label="原型演示控制"
              >
                <div className="flex items-center gap-2">
                  <span className="size-1.5 rounded-full bg-primary" />
                  <strong className="text-xs font-medium">交互原型</strong>
                  <span className="hidden text-xs text-muted-foreground sm:inline">
                    仅演示数据 · 刷新即重置
                  </span>
                </div>
                <div className="flex max-w-full flex-wrap items-center gap-1.5">
                  <Choice
                    label="演示场景"
                    value={scenario}
                    onChange={reset}
                    options={scenarioNames.map((label, i) => ({
                      value: String(i),
                      label,
                    }))}
                    className="!min-h-9 !text-xs max-[700px]:!min-h-11"
                  />
                  <Choice
                    label="演示状态"
                    value={s.online ? s.viewState : "offline"}
                    onChange={(v) => {
                      if (
                        v === "offline" &&
                        preview &&
                        s.knowledge.notes[preview.active.id]
                      )
                        dispatch({ type: "network", online: false });
                      else dispatch({ type: "view", state: v as ViewState });
                      setDemoOpen(false);
                    }}
                    options={states}
                    className="!min-h-9 !text-xs max-[700px]:!min-h-11"
                  />
                  <div className="hidden items-center gap-1 rounded-lg bg-muted p-0.5 sm:flex">
                    <IconBtn
                      label="自适应设备"
                      aria-pressed={mode === "auto"}
                      className={mode === "auto" ? "bg-card shadow-sm" : ""}
                      onClick={() => {
                        setMode("auto");
                        setDemoOpen(false);
                      }}
                    >
                      <LayoutList size={14} />
                    </IconBtn>
                    <IconBtn
                      label="模拟桌面"
                      aria-pressed={mode === "desktop"}
                      className={mode === "desktop" ? "bg-card shadow-sm" : ""}
                      onClick={() => {
                        setMode("desktop");
                        setDemoOpen(false);
                      }}
                    >
                      <Laptop size={14} />
                    </IconBtn>
                    <IconBtn
                      label="模拟手机"
                      aria-pressed={mode === "phone"}
                      className={mode === "phone" ? "bg-card shadow-sm" : ""}
                      onClick={() => {
                        setMode("phone");
                        setDemoOpen(false);
                      }}
                    >
                      <Smartphone size={14} />
                    </IconBtn>
                  </div>
                  {s.page === "files" && !preview && (
                    <Choice
                      label="目录加载状态"
                      value={
                        s.storage.nodes.find((n) => n.id === s.storage.activeId)
                          ?.status || "ready"
                      }
                      onChange={(status) => {
                        dispatch({
                          type: "directory-status",
                          id: s.storage.activeId,
                          status: status as
                            | "ready"
                            | "unloaded"
                            | "loading"
                            | "partial"
                            | "failed"
                            | "forbidden",
                        });
                        setDemoOpen(false);
                      }}
                      options={[
                        { value: "ready", label: "正常目录" },
                        { value: "unloaded", label: "尚未加载" },
                        { value: "loading", label: "正在加载" },
                        { value: "partial", label: "部分目录" },
                        { value: "failed", label: "目录失败" },
                        { value: "forbidden", label: "目录无权限" },
                      ]}
                    />
                  )}
                  {preview && s.knowledge.notes[preview.active.id] && (
                    <>
                      <Btn
                        variant="outline"
                        onClick={() => {
                          dispatch({ type: "k-fault", id: preview.active.id });
                          setDemoOpen(false);
                        }}
                      >
                        模拟下次保存失败
                      </Btn>
                      <Btn
                        variant="outline"
                        onClick={() => {
                          dispatch({ type: "k-remote", id: preview.active.id });
                          setDemoOpen(false);
                        }}
                      >
                        模拟文档并发修改
                      </Btn>
                      <Btn
                        variant="outline"
                        onClick={() => {
                          dispatch({
                            type: "k-options",
                            patch: { autosave: !s.knowledge.options.autosave },
                          });
                          setDemoOpen(false);
                        }}
                      >
                        {s.knowledge.options.autosave
                          ? "暂停文档自动保存"
                          : "恢复文档自动保存"}
                      </Btn>
                    </>
                  )}
                  <Btn
                    variant="outline"
                    onClick={() => {
                      dispatch({ type: "large-mode", value: true });
                      setDemoOpen(false);
                    }}
                  >
                    打开万级目录演示
                  </Btn>
                  <Btn
                    variant="outline"
                    onClick={() => {
                      dispatch({ type: "k-list-demo" });
                      setDemoOpen(false);
                    }}
                  >
                    文档与资源分页样本
                  </Btn>
                  <IconBtn label="重置演示" onClick={() => reset(scenario)}>
                    <RefreshCw size={14} />
                  </IconBtn>
                  <IconBtn
                    label={dark ? "切换浅色" : "切换深色"}
                    onClick={() => setDark(!dark)}
                  >
                    {dark ? <Sun size={14} /> : <Moon size={14} />}
                  </IconBtn>
                </div>
              </div>
            </DialogContent>
          </Dialog>
          <div
            ref={viewport}
            data-compact={compact}
            data-device-mode={mode}
            className={cn(
              "group/viewport relative mx-auto flex min-h-0 w-full flex-1 overflow-hidden bg-background",
              mode === "phone" &&
                "max-w-[390px] border-x border-border shadow-lg",
              mode === "desktop" && "max-w-[1600px]",
            )}
          >
            <Navigation
              width={width}
              pages={pages.filter((p) => p.id !== "knowledge-graph")}
              pending={pending}
              inboxCount={inboxCount}
              onSpace={switchSpace}
              floatingNav={floatingNav}
            />

            <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
              <header
                className={cn(
                  "flex shrink-0 items-center justify-between gap-2 border-b border-border bg-card/90",
                  compact ? "h-12 px-2" : "h-12 px-4",
                )}
              >
                <div className="flex min-w-0 items-center gap-2">
                  <IconBtn
                    label={
                      width >= 1280 && !s.preferences.navCollapsed
                        ? "收起主导航"
                        : "显示主导航"
                    }
                    aria-expanded={
                      (width >= 1280 && !s.preferences.navCollapsed) ||
                      floatingNav.open
                    }
                    ref={(node) => {
                      floatingNav.trigger.current = node;
                    }}
                    aria-controls={
                      floatingNav.open && floatingNav.floating
                        ? "floating-main-navigation"
                        : undefined
                    }
                    title={
                      width >= 1280 && !s.preferences.navCollapsed
                        ? "收起主导航"
                        : "悬停显示导航，也可点击打开"
                    }
                    onPointerEnter={floatingNav.enter}
                    onPointerLeave={floatingNav.leave}
                    onClick={(event) => {
                      floatingNav.cancel();
                      if (width >= 1280)
                        dispatch({
                          type: "preferences",
                          patch: { navCollapsed: !s.preferences.navCollapsed },
                        });
                      else floatingNav.toggle(event.detail === 0);
                    }}
                  >
                    {width >= 1280 && !s.preferences.navCollapsed ? (
                      <PanelLeftClose size={18} />
                    ) : (
                      <PanelLeftOpen size={18} />
                    )}
                  </IconBtn>
                  {compact ? (
                    <BrandMark size={28} />
                  ) : (
                    <span className="text-xs text-muted-foreground">
                      {s.space === "personal" ? "个人空间" : "团队空间"}
                    </span>
                  )}
                  {!compact && <span className="text-border">/</span>}
                  <span className="truncate text-[13px] font-medium">
                    {compact && preview
                      ? s.files.find((f) => f.id === preview.active.id)?.name ||
                        pageTitle
                      : pageTitle}
                  </span>
                </div>
                <div className="flex shrink-0 items-center gap-1.5">
                  {!compact && (
                    <Btn
                      variant="ghost"
                      className="mr-2 !min-w-44 !justify-start text-muted-foreground"
                      onClick={() => go("search")}
                    >
                      <Search size={15} />
                      <span className="text-xs">搜索文件与操作</span>
                      <kbd className="ml-auto rounded border border-border px-1.5 text-xs">
                        ⌘ K
                      </kbd>
                    </Btn>
                  )}
                  {compact && (
                    <IconBtn label="搜索" onClick={() => go("search")}>
                      <Search size={18} />
                    </IconBtn>
                  )}
                  <IconBtn
                    label="通知与待确认"
                    onClick={() => open({ type: "notifications" })}
                  >
                    <span className="relative">
                      <Bell size={17} />
                      {pending > 0 && s.options.notifyTasks && (
                        <span className="absolute -top-1 -right-1 size-1.5 rounded-full bg-[var(--warning)]" />
                      )}
                    </span>
                  </IconBtn>
                  <Btn
                    variant="ghost"
                    className="!h-11 text-xs"
                    onClick={() => setDemoOpen(true)}
                    aria-label="演示与审阅"
                  >
                    演示<span className="hidden sm:inline"> · v9.2</span>
                  </Btn>
                </div>
              </header>
              {!s.online && (
                <div
                  role="status"
                  className="flex shrink-0 items-center gap-2 bg-[var(--warning-soft)] px-4 py-2 text-xs text-[var(--warning)]"
                >
                  <WifiOff size={14} />
                  <span className="flex-1">
                    当前离线，已保存的版本仍可阅读。
                  </span>
                  <Btn
                    variant="ghost"
                    onClick={() => dispatch({ type: "network", online: true })}
                  >
                    重新连接
                  </Btn>
                </div>
              )}
              <div className="flex min-h-0 flex-1">
                <SurfaceVisible.Provider value={!preview}>
                  <main
                    ref={scroll}
                    id="main-content"
                    aria-label="工作区内容"
                    data-workspace-page={s.page}
                    className={cn(
                      "min-w-0 flex-1 overflow-y-auto",
                      preview && "hidden",
                      s.preferences.density === "compact" &&
                        "[&_[data-file-id]:not([data-entry-row])]:!py-1",
                      ["files", "knowledge", "resources"].includes(s.page)
                        ? "!overflow-hidden !p-0"
                        : compact
                          ? "p-2"
                          : "p-3",
                    )}
                  >
                    {missing ? (
                      <StateScreen
                        state={s.viewState}
                        onRestore={() =>
                          dispatch({ type: "view", state: "ready" })
                        }
                        onImport={() => {
                          dispatch({ type: "view", state: "ready" });
                          open({ type: "upload" });
                        }}
                      />
                    ) : (
                      <>
                        {s.page === "home" && (
                          <>
                            <KnowledgeHome />
                            <Home />
                          </>
                        )}
                        {s.page === "knowledge" && <KnowledgeWorkspace />}
                        {s.page === "knowledge-graph" && <KnowledgeGraph />}
                        {s.page === "resources" && <ResourceWorkspace />}
                        {s.page === "files" && <Files />}
                        {s.page === "favorites" && <FavoritesWorkspace />}
                        {s.page === "collections" && <Collections />}
                        {s.page === "tags" && <Tags />}
                        {s.page === "search" && <FileSearchWorkspace />}
                        {s.page === "capabilities" && <Capabilities />}
                        {s.page === "tasks" && <Tasks />}
                        {s.page === "inbox" && <InboxPage />}
                        {s.page === "shares" && <Shares />}
                        {s.page === "agents" && <ExternalConnections />}
                        {s.page === "connections" && <StorageWorkspace />}
                        {s.page === "collaboration" && (
                          <CollaborationWorkspace />
                        )}
                        {s.page === "devices" && <Devices />}
                        {s.page === "automation" && <Automation />}
                        {s.page === "trash" && <Files trash />}
                        {s.page === "settings" && <SettingsWorkspace />}
                      </>
                    )}
                  </main>
                </SurfaceVisible.Provider>
                {preview && (
                  <PreviewWorkspace
                    session={preview}
                    readers={previewReaders}
                    onChange={setPreview}
                    onExit={exitPreview}
                  />
                )}
              </div>
              {compact && !preview && !composerActive && (
                <nav
                  aria-label="底部导航"
                  className="grid h-14 shrink-0 grid-cols-5 border-t border-border bg-card pb-[env(safe-area-inset-bottom)]"
                >
                  {s.selected.length > 0 &&
                  [
                    "files",
                    "collections",
                    "search",
                    "favorites",
                    "knowledge",
                    "resources",
                  ].includes(s.page) ? (
                    <div className="col-span-5">
                      <BatchActions mobile />
                    </div>
                  ) : (
                    [
                      { id: "home", label: "首页", icon: HomeIcon },
                      { id: "files", label: "文件", icon: Folder },
                      { id: "search", label: "搜索", icon: Search },
                      { id: "tasks", label: "任务", icon: ListTodo },
                      { id: "settings", label: "我的", icon: UserRound },
                    ].map((p) => (
                      <Btn
                        key={p.id}
                        variant="ghost"
                        className={cn(
                          "!h-full !min-w-0 !flex-col !gap-1 !rounded-none !px-1 !py-1",
                          s.page === p.id
                            ? "text-primary"
                            : "text-muted-foreground",
                        )}
                        aria-current={s.page === p.id ? "page" : undefined}
                        onClick={() =>
                          p.id === "files" ? setFileHub(true) : go(p.id as Page)
                        }
                      >
                        <p.icon size={19} strokeWidth={1.6} />
                        <span className="text-xs">{p.label}</span>
                      </Btn>
                    ))
                  )}
                </nav>
              )}
            </div>
          </div>

          <Dialog
            open={!!pendingNavigation}
            onOpenChange={(v) => !v && setPendingNavigation(null)}
          >
            <DialogContent kind="confirm">
              <DialogTitle>离开未保存的连接配置？</DialogTitle>
              <DialogDescription>
                未保存的表单修改将丢弃，已保存连接与文件不变。
              </DialogDescription>
              <div className="flex gap-2">
                <Btn
                  onClick={() => {
                    const intent = pendingNavigation;
                    navigationDirty.current = false;
                    setPendingNavigation(null);
                    if (intent?.page) go(intent.page);
                    if (intent?.space)
                      dispatch({ type: "space", space: intent.space });
                  }}
                >
                  放弃修改并离开
                </Btn>
                <Btn
                  variant="outline"
                  onClick={() => setPendingNavigation(null)}
                >
                  继续编辑连接
                </Btn>
              </div>
            </DialogContent>
          </Dialog>

          <Dialog open={!!modal} onOpenChange={(isOpen) => !isOpen && close()}>
            <DialogContent
              kind={
                modal?.type === "danger"
                  ? "confirm"
                  : [
                        "upload",
                        "agent",
                        "rule",
                        "delivery",
                        "confirm",
                        "conflict",
                        "organize",
                        "edit-plan",
                      ].includes(modal?.type || "")
                    ? "complex"
                    : "form"
              }
              showCloseButton={false}
              finalFocus={returnFocus}
              className={cn(
                "!flex max-h-[90dvh] !flex-col !gap-0 overflow-hidden !p-0",
                modal?.type === "preview" || modal?.type === "compare"
                  ? "sm:!max-w-5xl"
                  : modal?.type === "danger"
                    ? "sm:!max-w-[400px]"
                    : [
                          "upload",
                          "agent",
                          "rule",
                          "delivery",
                          "confirm",
                          "conflict",
                          "organize",
                          "edit-plan",
                        ].includes(modal?.type || "")
                      ? "sm:!max-w-[640px]"
                      : "sm:!max-w-[520px]",
                compact &&
                  (modal?.type === "preview" || modal?.type === "compare"
                    ? "!left-0 !top-0 !h-[var(--prototype-height,100dvh)] !max-h-none !w-full !max-w-none !translate-x-0 !translate-y-0 !rounded-none"
                    : "!max-w-[min(420px,calc(100%-16px))]"),
              )}
            >
              <div className="flex shrink-0 items-start gap-3 border-b border-border p-5">
                <div className="min-w-0 flex-1">
                  <DialogTitle className="break-words pr-2 text-[15px] leading-7">
                    {modalTitle}
                  </DialogTitle>
                  <DialogDescription className="mt-1 text-xs">
                    DriveBridge · 交互演示，不操作真实文件
                  </DialogDescription>
                </div>
                <IconBtn label="关闭对话框" onClick={close}>
                  <X size={17} />
                </IconBtn>
              </div>
              <div className="min-h-0 overflow-y-auto p-5">
                {s.notice && (
                  <p
                    role="status"
                    className="mb-4 rounded-lg bg-accent p-3 text-xs leading-6"
                  >
                    {s.notice}
                  </p>
                )}
                {modal && (
                  <ModalBody
                    key={`${modal.type}-${"id" in modal ? modal.id : ""}`}
                    modal={modal}
                  />
                )}
              </div>
            </DialogContent>
          </Dialog>
        </div>
      </HoverPreviewProvider>
    </Demo.Provider>
  );
}
function StateScreen({
  state,
  onRestore,
  onImport,
}: {
  state: ViewState;
  onRestore: () => void;
  onImport: () => void;
}) {
  const text: Record<string, [string, string]> = {
    loading: ["正在读取这个空间", "等待文件与权限信息返回。"],
    empty: ["给你的新空间放点东西", "从一份文件开始，也可以接入已有的存储。"],
    forbidden: [
      "你暂时无法访问这个空间",
      "权限可能已被收回，文件名、内容和统计已隐藏。",
    ],
    disconnected: [
      "存储连接已失效",
      "原任务和已验证进度仍然保留。恢复连接后继续。",
    ],
    unsupported: [
      "当前设备暂不支持这项能力",
      "本机目录与后台运行需要客户端集成，可继续使用基本文件功能。",
    ],
    error: ["这次没能加载出来", "未提交新的文件操作。请重试读取当前空间。"],
  };
  return (
    <div className="py-10">
      {state === "loading" && (
        <LoaderCircle size={24} className="mx-auto animate-spin text-primary" />
      )}
      <Empty
        title={text[state]?.[0] || "暂不可用"}
        text={text[state]?.[1] || "请恢复后继续。"}
        action={state === "empty" ? "添加第一份文件" : "恢复正常演示"}
        onAction={state === "empty" ? onImport : onRestore}
      />
    </div>
  );
}
