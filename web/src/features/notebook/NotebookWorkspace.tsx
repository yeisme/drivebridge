import { useEffect, useLayoutEffect, useReducer, useRef, useState, type ReactNode, type CSSProperties } from "react";
import { Columns2, Focus, ListTree, PanelRight, Pin, Search, Settings2, Terminal, X } from "lucide-react";
import { Button } from "../../components/ui/button";
import { ConfirmDialog } from "../../components/ConfirmDialog";
import { AppDialog } from "../../components/AppDialog";
import { Popover, PopoverContent, PopoverTitle, PopoverTrigger } from "../../components/ui/popover";
import { NotebookNavigator, notebookShortcut, type NotebookEntry } from "./notebook-navigation";
import { createNotebookState, notebookReducer, noteKey, type NoteIdentity, type Pane } from "./notebook-state";
import { defaultNotebookPreferences, readNotebookPreferences, writeNotebookPreferences, type NotebookScope } from "./notebook-preferences";
import { NotebookSplit } from "./NotebookSplit";
import { NotebookDirectory } from "./NotebookDirectory";
import { NotebookProperties } from "./NotebookProperties";
import { useNotebookLayout } from "./useNotebookLayout";

export interface NotebookDocumentContext {
  ref: NoteIdentity;
  pane: Pane;
  draft?: string;
  onDraft: (body: string) => void;
  onSaved: (ref: NoteIdentity) => void;
  editablePane: boolean;
}
interface Props {
  scope: NotebookScope;
  // The session's complete authorized set, not a paginated search result.
  // Keep confirmed entries during loading; remove them on permission revocation.
  entries: NotebookEntry[];
  initialRef?: NoteIdentity;
  fileRelations?: boolean;
  directoryQuery?: string;
  onSaved?: (previous: NoteIdentity, next: NoteIdentity) => void;
  onDirtyChange?: (dirty: boolean) => void;
  renderDocument: (context: NotebookDocumentContext) => ReactNode;
}

// Identity changes synchronously remount the private session before any child
// can render the previous actor's drafts. Entries are owner-authorized data.
export function NotebookWorkspace(props: Props) {
  return <NotebookSession key={JSON.stringify(props.scope)} {...props} />;
}

function NotebookSession({ scope, entries, initialRef, directoryQuery = "", fileRelations = false, onSaved, onDirtyChange, renderDocument }: Props) {
  const [preferences, setPreferences] = useState(() => {
    try { return readNotebookPreferences(window.localStorage, scope); }
    catch { return defaultNotebookPreferences(); }
  });
  const [preferenceError, setPreferenceError] = useState(false);
  const [state, dispatch] = useReducer(notebookReducer, undefined, () => {
    const initial = { ...createNotebookState(), splitRatio: preferences.splitRatio };
    return initialRef && initialRef.sourceId === scope.source && entries.some(entry => noteKey(entry.ref) === noteKey(initialRef))
      ? notebookReducer(initial, { type: "open", ref: initialRef }) : initial;
  });
  const [navigation, setNavigation] = useState<"commands" | "open">();
  const [closing, setClosing] = useState<string>();
  const [directoryOpen, setDirectoryOpen] = useState(false);
  const [propertiesOpen, setPropertiesOpen] = useState(false);
  const [focused, setFocused] = useState(false);
  const focusControl = useRef<HTMLButtonElement>(null);
  const { container, directoryInline, propertiesInline } = useNotebookLayout();
  const savePreferences = (next: typeof preferences) => {
    setPreferences(next);
    try { setPreferenceError(!writeNotebookPreferences(window.localStorage, scope, next)); }
    catch { setPreferenceError(true); }
  };
  const tabs = useRef(new Map<string, HTMLButtonElement>());
  const focusNext = useRef(false);
  const visible = entries.filter(entry => entry.ref.sourceId === scope.source);
  const directoryEntries = visible.filter(entry => entry.title.toLocaleLowerCase().includes(directoryQuery.trim().toLocaleLowerCase()));
  const allowed = new Set(visible.map(entry => noteKey(entry.ref)));
  const allowedSignature = JSON.stringify([...allowed]);
  useEffect(() => { dispatch({ type: "retain", keys: JSON.parse(allowedSignature) as string[] }); }, [allowedSignature]);
  const open = (ref: NoteIdentity) => {
    if (allowed.has(noteKey(ref))) dispatch({ type: "open", ref });
  };
  useLayoutEffect(() => {
    if (focusNext.current && state.activeTab) {
      tabs.current.get(state.activeTab)?.focus();
      focusNext.current = false;
    }
  }, [state.activeTab]);
  useEffect(() => {
    const beforeUnload = (event: BeforeUnloadEvent) => {
      if (state.tabs.some(tab => tab.dirty)) { event.preventDefault(); event.returnValue = ""; }
    };
    window.addEventListener("beforeunload", beforeUnload);
    return () => window.removeEventListener("beforeunload", beforeUnload);
  }, [state.tabs]);
  const dirty = state.tabs.some(tab => tab.dirty);
  useEffect(() => { onDirtyChange?.(dirty); }, [dirty, onDirtyChange]);
  useEffect(() => () => { onDirtyChange?.(false); }, [onDirtyChange]);
  const active = state.tabs.find(tab => tab.key === state.activeTab && allowed.has(tab.key));
  const title = (key: string) => visible.find(entry => noteKey(entry.ref) === key)?.title ?? "笔记不可用";
  const close = (key: string) => {
    if (state.tabs.find(tab => tab.key === key)?.dirty) setClosing(key);
    else { focusNext.current = true; dispatch({ type: "close", key }); }
  };
  const pane = (which: Pane, key?: string) => {
    const visibleTabs = state.tabs.filter(tab => allowed.has(tab.key));
    // Primary tabs retain their editor transaction history until explicitly
    // closed or revoked. Do not prefetch unopened documents in the second pane.
    const mounted = which === "primary" ? visibleTabs : visibleTabs.filter(tab => tab.key === key);
    return <>
      {!visibleTabs.some(tab => tab.key === key) && <p role="status">请选择可访问的笔记</p>}
      {mounted.map(tab => <div key={tab.key} hidden={tab.key !== key} className="h-full min-h-0">
        <NotebookPane title={title(tab.key)} scrollTop={state.positions[which][tab.key]?.scrollTop ?? 0}
          onScroll={scrollTop => dispatch({ type: "position", pane: which, key: tab.key, scrollTop })}>
          {renderDocument({ ref: tab.ref, pane: which, draft: state.drafts[tab.key], editablePane: which === "primary", onDraft: body => dispatch({ type: "draft", key: tab.key, body }), onSaved: ref => {
            if (ref.sourceId !== tab.ref.sourceId || ref.objectId !== tab.ref.objectId) return;
            onSaved?.(tab.ref, ref);
            dispatch({ type: "saved", key: tab.key, ref });
          } })}
        </NotebookPane>
      </div>)}
    </>;
  };
  return <div ref={container} className="flex h-full min-h-0 min-w-0 flex-col bg-background text-foreground [&_*]:border-border" aria-label="笔记工作台" style={{ "--notebook-font-size": `${preferences.fontSize}px`, "--notebook-line-width": `${{ narrow: 640, standard: 720, wide: 880 }[preferences.readingWidth]}px` } as CSSProperties} onKeyDown={event => {
    const overlayOpen = navigation || closing || (!directoryInline && directoryOpen) || (!propertiesInline && propertiesOpen);
    if (event.key === "Escape" && focused && !overlayOpen && !event.defaultPrevented && !event.nativeEvent.isComposing &&
        !(event.target instanceof Element && event.target.closest('input, textarea, select, [contenteditable], .cm-editor'))) {
      event.preventDefault(); event.stopPropagation(); setFocused(false); focusControl.current?.focus(); return;
    }
    const action = notebookShortcut(event.nativeEvent);
    if (!action || navigation || closing || (!directoryInline && directoryOpen) || (!propertiesInline && propertiesOpen)) return;
    event.preventDefault();
    if (action === "commands" || action === "open") setNavigation(action);
    else { focusNext.current = true; dispatch({ type: "cycle", direction: action === "next-tab" ? 1 : -1 }); }
  }}>
    <div role="tablist" aria-label="已打开笔记" className="flex min-h-11 shrink-0 items-stretch overflow-x-auto border-b bg-card/70 px-1 md:min-h-9 md:px-2">
      {state.tabs.filter(tab => allowed.has(tab.key)).map(tab => <div key={tab.key} className="flex shrink-0 items-center">
        <Button type="button" role="tab" variant="ghost" className="h-11 max-w-56 gap-1.5 rounded-md px-3 text-[13px] aria-selected:bg-background aria-selected:font-semibold aria-selected:text-primary md:h-8" aria-selected={state.activeTab === tab.key} tabIndex={state.activeTab === tab.key ? 0 : -1}
          ref={node => { if (node) tabs.current.set(tab.key, node); else tabs.current.delete(tab.key); }}
          onKeyDown={event => {
            const available = state.tabs.filter(item => allowed.has(item.key));
            const index = available.findIndex(item => item.key === tab.key);
            const next = event.key === "ArrowRight" ? (index + 1) % available.length : event.key === "ArrowLeft" ? (index + available.length - 1) % available.length : event.key === "Home" ? 0 : event.key === "End" ? available.length - 1 : undefined;
            if (next !== undefined) { event.preventDefault(); focusNext.current = true; dispatch({ type: "activate", key: available[next].key }); }
          }}
          onClick={() => dispatch({ type: "activate", key: tab.key })} onDoubleClick={() => dispatch({ type: "pin", key: tab.key, pinned: true })}>
          {tab.pinned && <Pin size={12} aria-label="已固定"/>}<span className="truncate">{title(tab.key)}</span>{tab.dirty && <span aria-label="未保存" className="text-primary">·</span>}
        </Button>
        <Button type="button" variant="ghost" size="icon" className="size-10 rounded-md md:size-8" aria-label={`关闭 ${title(tab.key)}`} onClick={() => close(tab.key)}><X size={14}/></Button>
      </div>)}
      {!state.tabs.length && <p className="flex items-center px-3 text-xs text-muted-foreground">从目录或快速打开选择一篇笔记</p>}
    </div>
    <div className="flex min-h-12 shrink-0 flex-wrap items-center gap-1 border-b bg-background/90 px-2 max-md:[&>button]:size-11 md:min-h-10 md:px-3" role="toolbar" aria-label="笔记工具栏">
      <Button type="button" variant="ghost" size="icon" aria-label="笔记目录" disabled={focused} aria-expanded={!focused && (directoryInline ? preferences.leftOpen : directoryOpen)} onClick={() => { if (!directoryInline) setDirectoryOpen(true); else savePreferences({ ...preferences, leftOpen: !preferences.leftOpen }); }}><ListTree/></Button>
      <Button type="button" variant="ghost" size="icon" aria-label="命令面板" onClick={() => setNavigation("commands")}><Terminal/></Button>
      <Button type="button" variant="ghost" size="icon" aria-label="快速打开" onClick={() => setNavigation("open")}><Search/></Button>
      <Button type="button" variant="ghost" size="icon" aria-label="切换分屏" aria-pressed={state.split} disabled={!state.activeTab || !allowed.has(state.activeTab)} onClick={() => dispatch({ type: "split" })}><Columns2/></Button>
      <Button type="button" variant="ghost" size="icon" aria-label="笔记信息" disabled={focused} aria-expanded={!focused && (propertiesInline ? preferences.rightOpen : propertiesOpen)} onClick={() => { if (!propertiesInline) setPropertiesOpen(true); else savePreferences({ ...preferences, rightOpen: !preferences.rightOpen }); }}><PanelRight/></Button>
      <span className="mx-1 hidden h-5 w-px bg-border md:block" aria-hidden="true"/>
      <span className="hidden text-xs text-muted-foreground md:inline">{dirty ? "有未保存修改" : active ? "已固定版本" : "未选择笔记"}</span>
      <Button ref={focusControl} type="button" variant="ghost" size="icon" className="ml-auto" aria-label={focused ? "退出专注模式" : "进入专注模式"} aria-pressed={focused} onClick={() => setFocused(value => !value)}><Focus/></Button>
      <Popover>
        <PopoverTrigger render={<Button type="button" variant="ghost" size="icon" className="ml-auto max-md:size-11" aria-label="阅读设置"/>}><Settings2/></PopoverTrigger>
        <PopoverContent align="end" className="w-64 gap-4 p-4">
          <PopoverTitle>阅读设置</PopoverTitle>
          <div className="flex items-center justify-between gap-3">
            <span className="text-muted-foreground">字号</span>
            <div className="flex items-center gap-2">
              <Button type="button" variant="outline" size="icon" className="size-11" aria-label="减小阅读字号" disabled={preferences.fontSize <= 14} onClick={() => savePreferences({ ...preferences, fontSize: Math.max(14, preferences.fontSize - 1) })}>A−</Button>
              <output aria-label="当前阅读字号" className="min-w-5 text-center tabular-nums">{preferences.fontSize}</output>
              <Button type="button" variant="outline" size="icon" className="size-11" aria-label="增大阅读字号" disabled={preferences.fontSize >= 24} onClick={() => savePreferences({ ...preferences, fontSize: Math.min(24, preferences.fontSize + 1) })}>A+</Button>
            </div>
          </div>
          <fieldset className="space-y-2">
            <legend className="text-muted-foreground">正文宽度</legend>
            <div className="grid grid-cols-3 gap-1">
              {([['narrow', '窄'], ['standard', '标准'], ['wide', '宽']] as const).map(([width, label]) => <Button key={width} type="button" variant="ghost" className="min-h-11 aria-pressed:bg-accent aria-pressed:font-semibold" aria-label={`阅读行宽：${label}`} aria-pressed={preferences.readingWidth === width} onClick={() => savePreferences({ ...preferences, readingWidth: width })}>{label}</Button>)}
            </div>
          </fieldset>
          <p className="text-xs leading-5 text-muted-foreground">仅调整当前来源的阅读显示，不修改笔记正文。</p>
        </PopoverContent>
      </Popover>
    </div>
    {preferenceError && <p role="status" className="text-sm text-muted-foreground">无法保存布局偏好，本次会话仍可使用。</p>}
    <div className="flex min-h-0 min-w-0 flex-1 overflow-hidden">
    {!focused && directoryInline && preferences.leftOpen && <aside aria-label="笔记目录侧栏" className="shrink-0 overflow-auto border-r" style={{ width: preferences.leftWidth }}>
      <NotebookDirectory entries={directoryEntries} activeKey={state.activeTab} onOpen={open} onPin={ref => dispatch({ type: "open", ref, pinned: true })}/>
    </aside>}
    <NotebookSplit split={state.split} ratio={state.splitRatio} onRatio={value => {
      dispatch({ type: "ratio", value });
      savePreferences({ ...preferences, splitRatio: value });
    }}
      primary={pane("primary", state.activeTab)} secondary={state.split ? pane("secondary", state.secondTab) : null}/>
    {!focused && propertiesInline && preferences.rightOpen && <aside aria-label="笔记信息侧栏" className="shrink-0 overflow-auto border-l" style={{ width: preferences.rightWidth }}><NotebookProperties identity={active?.ref} actor={scope.actor} fileRelations={fileRelations}/></aside>}
    </div>
    {!propertiesInline && propertiesOpen && <AppDialog title="笔记信息" close={() => setPropertiesOpen(false)}><NotebookProperties identity={active?.ref} actor={scope.actor} fileRelations={fileRelations}/></AppDialog>}
    {!directoryInline && directoryOpen && <AppDialog title="当前来源笔记" close={() => setDirectoryOpen(false)}>
      <NotebookDirectory entries={directoryEntries} activeKey={state.activeTab} onOpen={ref => { open(ref); setDirectoryOpen(false); }} onPin={ref => dispatch({ type: "open", ref, pinned: true })}/>
    </AppDialog>}
    {navigation && <NotebookNavigator mode={navigation} entries={visible} sourceId={scope.source} onOpen={open} close={() => setNavigation(undefined)} commands={[
      { id: "font-increase", label: "增大阅读字号", disabled: preferences.fontSize >= 24, run: () => savePreferences({ ...preferences, fontSize: Math.min(24, preferences.fontSize + 1) }) },
      { id: "font-decrease", label: "减小阅读字号", disabled: preferences.fontSize <= 14, run: () => savePreferences({ ...preferences, fontSize: Math.max(14, preferences.fontSize - 1) }) },
      ...([['narrow', '窄'], ['standard', '标准'], ['wide', '宽']] as const).map(([width, label]) => ({ id: `width-${width}`, label: `阅读行宽：${label}`, disabled: preferences.readingWidth === width, run: () => savePreferences({ ...preferences, readingWidth: width }) })),
      { id: "pin", label: active?.pinned ? "取消固定当前笔记" : "固定当前笔记", disabled: !active || active.dirty, run: () => { if (active) dispatch({ type: "pin", key: active.key, pinned: !active.pinned }); } },
      { id: "close", label: "关闭当前笔记", disabled: !active, run: () => { if (active) close(active.key); } },
      { id: "split", label: "切换分屏", disabled: !active, run: () => dispatch({ type: "split" }) },
      { id: "reopen", label: "重新打开已关闭笔记", disabled: !state.closed.some(tab => allowed.has(tab.key)), run: () => dispatch({ type: "reopen" }) },
    ]}/>}
    {closing && <ConfirmDialog open title="放弃未保存草稿？" description="关闭此标签页将丢弃内存草稿，不会修改原文件。" confirmLabel="放弃并关闭" destructive
      onOpenChange={value => { if (!value) setClosing(undefined); }} onConfirm={async () => { focusNext.current = true; dispatch({ type: "close", key: closing, discard: true }); }}/>} 
  </div>;
}

function NotebookPane({ title, scrollTop, onScroll, children }: { title: string; scrollTop: number; onScroll: (value: number) => void; children: ReactNode }) {
  const element = useRef<HTMLDivElement>(null);
  useLayoutEffect(() => { if (element.current) element.current.scrollTop = scrollTop; }, []);
  return <div ref={element} role="region" aria-label={title} className="h-full min-w-0 overflow-auto bg-card/80" onScroll={event => onScroll(event.currentTarget.scrollTop)}>{children}</div>;
}
