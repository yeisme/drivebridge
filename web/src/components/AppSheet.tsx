import { useEffect, useRef, useState, type ReactElement, type ReactNode } from "react";
import { Sheet, SheetClose, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from "./ui/sheet";
import { Button } from "./ui/button";
import { X } from "lucide-react";

export function AppSheet({ title, trigger, open, onOpenChange, children, detailWidth, initialDetailWidth, onDetailWidthCommit }: {
  title: string;
  detailWidth?: number;
  initialDetailWidth?: number;
  onDetailWidthCommit?: (width: number) => void;
  trigger?: ReactElement;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  children: ReactNode;
}) {
  const [adjustment, setAdjustment] = useState(() =>
    Math.min(640, Math.max(320, initialDetailWidth ?? detailWidth ?? 480)) - Math.min(640, Math.max(320, detailWidth ?? 480)));
  const initialWidth = Math.min(640, Math.max(320, detailWidth ?? 480));
  const width = Math.min(640, Math.max(320, initialWidth + adjustment));
  const drag = useRef<{ id: number; x: number; width: number; element: HTMLDivElement } | null>(null);
  const finishDrag = (cancel: boolean) => {
    const current = drag.current;
    if (!current) return;
    drag.current = null;
    if (cancel) setAdjustment(current.width - initialWidth);
    else onDetailWidthCommit?.(width);
    if (current.element.hasPointerCapture(current.id)) current.element.releasePointerCapture(current.id);
  };
  useEffect(() => {
    if (open) return;
    const current = drag.current;
    if (!current) return;
    drag.current = null;
    setAdjustment(current.width - initialWidth);
    if (current.element.hasPointerCapture(current.id)) current.element.releasePointerCapture(current.id);
  }, [open, initialWidth]);
  useEffect(() => () => {
    const current = drag.current;
    drag.current = null;
    if (current?.element.hasPointerCapture(current.id)) current.element.releasePointerCapture(current.id);
  }, []);
  useEffect(() => {
    const media = window.matchMedia("(max-width: 767px)");
    const cancelOnMobile = () => {
      const current = drag.current;
      if (!media.matches || !current) return;
      drag.current = null;
      setAdjustment(current.width - initialWidth);
      if (current.element.hasPointerCapture(current.id)) current.element.releasePointerCapture(current.id);
    };
    media.addEventListener("change", cancelOnMobile);
    return () => media.removeEventListener("change", cancelOnMobile);
  }, [initialWidth]);
  return <Sheet open={open} onOpenChange={(next, details) => {
    if (!next && details.reason === "escape-key" && drag.current) {
      details.cancel();
      finishDrag(true);
      return;
    }
    finishDrag(true);
    onOpenChange(next);
  }}>
    {trigger && <SheetTrigger render={trigger} />}
    <SheetContent aria-label={title} aria-describedby={undefined} showCloseButton={false}
      style={detailWidth === undefined ? undefined : { width: `min(100vw, ${width}px)` }}
      className={detailWidth === undefined ? "gap-0 data-[side=right]:w-full data-[side=right]:sm:max-w-[480px]" : "gap-0 data-[side=right]:max-w-none data-[side=right]:sm:max-w-none max-md:!w-full"}>
      {detailWidth !== undefined && <div role="separator" tabIndex={0} aria-label="调整详情宽度"
        aria-orientation="vertical" aria-valuemin={320} aria-valuemax={640} aria-valuenow={width}
        className="absolute inset-y-0 left-0 z-10 hidden w-2 cursor-col-resize focus-visible:bg-ring md:block"
        style={{ touchAction: "none" }}
        onPointerDown={event => {
          if (event.button !== 0 || drag.current) return;
          event.preventDefault();
          event.currentTarget.focus();
          event.currentTarget.setPointerCapture(event.pointerId);
          drag.current = { id: event.pointerId, x: event.clientX, width, element: event.currentTarget };
        }}
        onPointerMove={event => {
          const current = drag.current;
          if (!current || current.id !== event.pointerId) return;
          setAdjustment(Math.min(640, Math.max(320, current.width + current.x - event.clientX)) - initialWidth);
        }}
        onPointerUp={event => { if (drag.current?.id === event.pointerId) finishDrag(window.matchMedia("(max-width: 767px)").matches); }}
        onPointerCancel={() => finishDrag(true)}
        onLostPointerCapture={() => finishDrag(true)}
        onDoubleClick={() => { finishDrag(true); setAdjustment(0); onDetailWidthCommit?.(initialWidth); }}
        onKeyDown={event => {
          if (event.key === "Escape" && drag.current) {
            event.preventDefault(); event.stopPropagation(); finishDrag(true); return;
          }
          if (drag.current) return;
          const step = event.shiftKey ? 32 : 16;
          let next: number;
          switch (event.key) {
            case "ArrowLeft": next = width + step; break;
            case "ArrowRight": next = width - step; break;
            case "Home": next = 320; break;
            case "End": next = 640; break;
            case "Enter": next = initialWidth; break;
            default: return;
          }
          event.preventDefault();
          event.stopPropagation();
          const committed = Math.min(640, Math.max(320, next));
          setAdjustment(committed - initialWidth);
          onDetailWidthCommit?.(committed);
        }} />}
      <SheetHeader className="flex-row items-center gap-2 border-b border-border sm:p-5">
        <SheetTitle className="min-w-0 flex-1">{title}</SheetTitle>
        <SheetClose render={<Button type="button" variant="ghost" size="icon" className="max-md:size-11" aria-label="关闭" />}><X /></SheetClose>
      </SheetHeader>
      <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain p-4 sm:p-5">{children}</div>
    </SheetContent>
  </Sheet>;
}
