import { useEffect, useLayoutEffect, useRef, useState, useSyncExternalStore, type ReactNode } from 'react';
import { Maximize2, Minimize2, PanelRight, X } from 'lucide-react';
import { Dialog, DialogClose, DialogContent, DialogTitle } from '../../components/ui/dialog';
import { Button } from '../../components/ui/button';

const inlineQuery = '(min-width: 1280px)';
function subscribeInline(notify: () => void) {
  const media = window.matchMedia(inlineQuery);
  media.addEventListener('change', notify);
  return () => media.removeEventListener('change', notify);
}
const inlineSnapshot = () => window.matchMedia(inlineQuery).matches;

// Only the shell owns presentation state. Readers and information remain at
// stable positions so resizing and panel toggles cannot reset their drafts.
export function FilePreviewDialog({ title, close, notebookAction, information, children }: {
  title: string;
  close: () => void;
  notebookAction?: ReactNode;
  information: ReactNode;
  children: ReactNode;
}) {
  const inline = useSyncExternalStore(subscribeInline, inlineSnapshot, () => false);
  const [desktopOpen, setDesktopOpen] = useState(true);
  const [overlayOpen, setOverlayOpen] = useState(false);
  const [expanded, setExpanded] = useState(false);
  const toggle = useRef<HTMLButtonElement>(null);
  const panelClose = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLElement>(null);
  const open = inline ? desktopOpen : overlayOpen;
  useLayoutEffect(() => {
    if (!open && panel.current?.contains(document.activeElement)) toggle.current?.focus();
  }, [open]);
  useEffect(() => { setOverlayOpen(false); }, [inline]);
  useEffect(() => {
    if (!inline && overlayOpen) panelClose.current?.focus();
  }, [inline, overlayOpen]);
  const hideInformation = () => {
    if (inline) setDesktopOpen(false); else setOverlayOpen(false);
    toggle.current?.focus();
  };
  return <Dialog open onOpenChange={(next, details) => {
    if (next) return;
    if (details.reason === 'escape-key' && !inline && overlayOpen) {
      details.cancel();
      hideInformation();
    } else close();
  }}>
    <DialogContent aria-label={title} aria-describedby={undefined} showCloseButton={false}
      className={`app-dialog quicklook-dialog flex flex-col gap-0 overflow-hidden p-0 max-md:h-dvh max-md:max-h-dvh max-md:w-screen max-md:max-w-none max-md:rounded-none ${expanded ? 'quicklook-expanded md:w-[98vw] md:max-w-[98vw] md:h-[96dvh] md:max-h-[96dvh]' : 'md:w-[min(1600px,96vw)] md:max-w-[96vw] md:h-[92dvh] md:max-h-[92dvh]'}`}>
      <header className="grid shrink-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-x-2 gap-y-1 border-b border-border px-3 py-1 md:flex md:min-h-12 md:px-4">
        <DialogTitle className="min-w-0 flex-1 truncate leading-normal" title={title}>{title}</DialogTitle>
        <DialogClose render={<Button type="button" variant="ghost" size="icon" className="size-11 md:order-3 md:size-8" aria-label="关闭" />}><X /></DialogClose>
        <div className="quicklook-header-actions col-span-2 flex min-w-0 flex-wrap items-center justify-end gap-2 md:order-2 max-md:[&_button]:min-h-11 max-md:[&_button]:min-w-11">
          {notebookAction}
          <Button type="button" variant="outline" size="icon" aria-label={expanded ? '还原预览窗口' : '最大化预览'} onClick={() => setExpanded(value => !value)}>
            {expanded ? <Minimize2 /> : <Maximize2 />}
          </Button>
          <Button ref={toggle} type="button" variant="outline" aria-label={open ? '收起信息栏' : '展开信息栏'} aria-expanded={open} aria-controls="quicklook-sidebar"
            onClick={() => { if (open) hideInformation(); else if (inline) setDesktopOpen(true); else setOverlayOpen(true); }}>
            <PanelRight /><span className="max-md:hidden">{open ? '收起信息栏' : '展开信息栏'}</span><span className="md:hidden">信息</span>
          </Button>
        </div>
      </header>
      <div className={`quicklook-layout relative grid min-h-0 flex-1 ${inline && open ? 'grid-cols-[minmax(0,1fr)_360px]' : 'grid-cols-[minmax(0,1fr)]'}`}>
        <section className="quicklook-stage" aria-label="文件预览" inert={!inline && overlayOpen}>
          {children}
        </section>
        <aside ref={panel} id="quicklook-sidebar" className={`quicklook-sidebar flex min-h-0 min-w-0 flex-col gap-4 overflow-y-auto overscroll-contain border-l border-border bg-card p-4 max-md:[&_button]:min-h-11 ${inline ? '' : 'absolute inset-y-0 right-0 z-20 w-full shadow-lg md:w-[360px]'}`}
          hidden={!open} aria-label="预览文件列表与信息">
          <header className="flex items-center justify-between gap-2">
            <h3 className="text-sm font-semibold">文件与版本信息</h3>
            <Button ref={panelClose} type="button" variant="ghost" size="icon" className="max-md:size-11" aria-label="关闭信息栏" onClick={hideInformation}><X /></Button>
          </header>
          {information}
        </aside>
      </div>
    </DialogContent>
  </Dialog>;
}
