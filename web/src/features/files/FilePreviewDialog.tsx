import { useEffect, useLayoutEffect, useRef, useState, useSyncExternalStore, type CSSProperties, type ReactNode } from 'react';
import { Info, Maximize2, Minimize2, MoreHorizontal, PanelRight, Pin, PinOff, X } from 'lucide-react';
import { Dialog, DialogClose, DialogContent, DialogTitle } from '../../components/ui/dialog';
import { Button } from '../../components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '../../components/ui/dropdown-menu';

const inlineQuery = '(min-width: 1280px)';
function subscribeInline(notify: () => void) {
  const media = window.matchMedia(inlineQuery);
  media.addEventListener('change', notify);
  return () => media.removeEventListener('change', notify);
}
const inlineSnapshot = () => window.matchMedia(inlineQuery).matches;

// Only the shell owns presentation state. Readers and information remain at
// stable positions so resizing and panel toggles cannot reset their drafts.
export function FilePreviewDialog({ title, close, notebookAction, information, directory, tabs = [], activeTab, onActivateTab, onCloseTab, onPinTab, onCloseOthers, onCloseToRight, onCloseAll, panelMode = 'directory', onPanelModeChange, panelOpen, onPanelOpenChange, navigationWidth = 0, children }: {
  title: string;
  close: () => void;
  notebookAction?: ReactNode;
  information: ReactNode;
  children: ReactNode;
  directory?: ReactNode;
  tabs?: { key: string; title: string; path: string; pinned: boolean }[];
  activeTab?: string;
  onActivateTab?: (key: string) => void;
  onCloseTab?: (key: string) => void;
  onPinTab?: (key: string, pinned: boolean) => void;
  onCloseOthers?: (key: string) => void;
  onCloseToRight?: (key: string) => void;
  onCloseAll?: () => void;
  panelMode?: 'directory' | 'details';
  onPanelModeChange?: (mode: 'directory' | 'details') => void;
  panelOpen?: boolean;
  onPanelOpenChange?: (open: boolean) => void;
  navigationWidth?: number;
}) {
  const inline = useSyncExternalStore(subscribeInline, inlineSnapshot, () => false);
  const [localDesktopOpen, setLocalDesktopOpen] = useState(true);
  const [overlayOpen, setOverlayOpen] = useState(false);
  const [expanded, setExpanded] = useState(false);
  const toggle = useRef<HTMLButtonElement>(null);
  const panelClose = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLElement>(null);
  const desktopOpen = panelOpen ?? localDesktopOpen;
  const setDesktopOpen = (open: boolean) => { onPanelOpenChange?.(open); if (panelOpen === undefined) setLocalDesktopOpen(open); };
  const open = inline ? desktopOpen : overlayOpen;
  useLayoutEffect(() => {
    if (!open && panel.current?.contains(document.activeElement)) toggle.current?.focus();
  }, [open]);
  useEffect(() => { setOverlayOpen(false); }, [inline]);
  useEffect(() => {
    if (!inline && overlayOpen) panelClose.current?.focus();
  }, [inline, overlayOpen]);
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.isComposing || !(event.metaKey || event.ctrlKey) || (event.target instanceof HTMLElement && event.target.closest('input, textarea, select, [contenteditable="true"]'))) return;
      if (event.key === 'Tab' && tabs.length > 1) {
        event.preventDefault();
        const index = Math.max(0, tabs.findIndex(tab => tab.key === activeTab));
        const next = tabs[(index + (event.shiftKey ? -1 : 1) + tabs.length) % tabs.length];
        if (next) onActivateTab?.(next.key);
      } else if (event.key.toLowerCase() === 'w' && activeTab) {
        event.preventDefault();
        onCloseTab?.(activeTab);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [activeTab, onActivateTab, onCloseTab, tabs]);
  const hideInformation = () => {
    if (inline) setDesktopOpen(false); else setOverlayOpen(false);
    toggle.current?.focus();
  };
  return <Dialog open modal={inline ? false : true} disablePointerDismissal={inline} onOpenChange={(next, details) => {
    if (next) return;
    if (details.reason === 'escape-key' && !inline && overlayOpen) {
      details.cancel();
      hideInformation();
    } else close();
  }}>
    <DialogContent aria-label={title} aria-describedby={undefined} showCloseButton={false}
      overlayClassName="quicklook-overlay-offset"
      overlayStyle={{ '--quicklook-nav-width': `${navigationWidth}px` } as CSSProperties}
      style={{ '--quicklook-nav-width': `${navigationWidth}px` } as CSSProperties}
      className={`app-dialog quicklook-dialog flex flex-col gap-0 overflow-hidden p-0 max-md:h-dvh max-md:max-h-dvh max-md:w-screen max-md:max-w-none max-md:rounded-none ${expanded ? 'quicklook-expanded' : ''}`}>
      <header className="grid shrink-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-x-2 gap-y-1 border-b border-border px-3 py-1 md:flex md:min-h-12 md:px-4">
        <DialogTitle className="min-w-0 flex-1 truncate leading-normal" title={title}>{title}</DialogTitle>
        <DialogClose render={<Button type="button" variant="ghost" size="icon" className="size-11 md:order-3 md:size-8" aria-label="关闭" />}><X /></DialogClose>
        <div className="quicklook-header-actions col-span-2 flex min-w-0 flex-wrap items-center justify-end gap-2 md:order-2 max-md:[&_button]:min-h-11 max-md:[&_button]:min-w-11">
          {notebookAction}
          {tabs.length > 1 && activeTab && <DropdownMenu>
            <DropdownMenuTrigger render={<Button type="button" variant="ghost" size="icon" className="size-9" aria-label="标签操作" />}><MoreHorizontal /></DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={() => onPinTab?.(activeTab, !tabs.find(tab => tab.key === activeTab)?.pinned)}>{tabs.find(tab => tab.key === activeTab)?.pinned ? '取消固定当前标签' : '固定当前标签'}</DropdownMenuItem>
              <DropdownMenuItem onClick={() => onCloseOthers?.(activeTab)}>关闭其他标签</DropdownMenuItem>
              <DropdownMenuItem onClick={() => onCloseToRight?.(activeTab)}>关闭右侧标签</DropdownMenuItem>
              <DropdownMenuItem onClick={() => onCloseAll?.()}>关闭全部可关闭标签</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>}
          <Button type="button" variant="outline" size="icon" aria-label={expanded ? '还原预览窗口' : '最大化预览'} onClick={() => setExpanded(value => !value)}>
            {expanded ? <Minimize2 /> : <Maximize2 />}
          </Button>
          <Button ref={toggle} type="button" variant="outline" aria-label={open ? '收起信息栏' : '展开信息栏'} aria-expanded={open} aria-controls="quicklook-sidebar"
            onClick={() => { if (open) hideInformation(); else if (inline) setDesktopOpen(true); else setOverlayOpen(true); }}>
            <PanelRight /><span className="max-md:hidden">{open ? '收起信息栏' : '展开信息栏'}</span><span className="md:hidden">信息</span>
          </Button>
        </div>
      </header>
      {tabs.length > 0 && <div role="tablist" aria-label="已打开文件" className="quicklook-tabs flex min-h-10 shrink-0 items-stretch gap-1 overflow-x-auto border-b border-border bg-card/80 px-2">
        {tabs.map(tab => <div key={tab.key} className="quicklook-tab flex shrink-0 items-center">
          <Button type="button" role="tab" variant="ghost" aria-selected={activeTab === tab.key} tabIndex={activeTab === tab.key ? 0 : -1} className="h-9 max-w-56 justify-start gap-1.5 rounded-md px-2.5 text-xs aria-selected:bg-background aria-selected:font-semibold" title={tab.path} onKeyDown={event => {
            const index = tabs.findIndex(item => item.key === tab.key);
            const next = event.key === 'ArrowRight' ? (index + 1) % tabs.length : event.key === 'ArrowLeft' ? (index + tabs.length - 1) % tabs.length : event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1 : -1;
            if (next >= 0) { event.preventDefault(); const target = tabs[next]; if (target) onActivateTab?.(target.key); }
          }} onClick={() => onActivateTab?.(tab.key)} onDoubleClick={() => onPinTab?.(tab.key, !tab.pinned)}>
            {tab.pinned ? <Pin size={12} aria-label="已固定"/> : <PinOff size={12} aria-label="未固定"/>}<span className="truncate">{tab.title}</span>
          </Button>
          <Button type="button" variant="ghost" size="icon" className="size-8" aria-label={`关闭 ${tab.title}`} onClick={() => onCloseTab?.(tab.key)}><X size={13}/></Button>
        </div>)}
      </div>}
      <div className={`quicklook-layout relative grid min-h-0 flex-1 ${inline && open ? 'grid-cols-[minmax(0,1fr)_288px]' : inline ? 'grid-cols-[minmax(0,1fr)_40px]' : 'grid-cols-[minmax(0,1fr)]'}`}>
        <section className="quicklook-stage" aria-label="文件预览" inert={!inline && overlayOpen}>
          {children}
        </section>
        <aside ref={panel} id="quicklook-sidebar" className={`quicklook-sidebar flex min-h-0 min-w-0 flex-col gap-3 overflow-y-auto overscroll-contain border-l border-border bg-card p-3 max-md:[&_button]:min-h-11 ${inline ? '' : 'absolute inset-y-0 right-0 z-20 w-[min(360px,92vw)] shadow-lg'}`}
          hidden={!open} aria-label="预览文件列表与信息">
          <header className="flex items-center justify-between gap-2">
            <div className="flex min-w-0 items-center gap-1 rounded-md bg-muted/50 p-1" role="tablist" aria-label="预览侧栏">
              <Button type="button" variant="ghost" size="sm" className="h-8 gap-1 px-2 text-xs" aria-pressed={panelMode === 'directory'} onClick={() => onPanelModeChange?.('directory')}><PanelRight size={13}/>目录</Button>
              <Button type="button" variant="ghost" size="sm" className="h-8 gap-1 px-2 text-xs" aria-pressed={panelMode === 'details'} onClick={() => onPanelModeChange?.('details')}><Info size={13}/>信息</Button>
            </div>
            <Button ref={panelClose} type="button" variant="ghost" size="icon" className="max-md:size-11" aria-label="关闭信息栏" onClick={hideInformation}><X /></Button>
          </header>
          {panelMode === 'directory' ? (directory || information) : information}
        </aside>
        {inline && !open && <aside className="quicklook-sidebar-rail flex min-h-0 items-start justify-center border-l border-border bg-card pt-3" aria-label="预览侧栏已收起">
          <Button type="button" variant="ghost" size="icon" className="size-9" aria-label="恢复预览侧栏" onClick={() => setDesktopOpen(true)}><PanelRight /></Button>
        </aside>}
      </div>
    </DialogContent>
  </Dialog>;
}
