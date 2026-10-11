import { useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { Button } from "../../components/ui/button";

const MIN_PANE = 480;
const SEPARATOR = 12;
export function notebookSplitRatio(width: number, ratio: number): number {
  const available = width - SEPARATOR;
  if (available < MIN_PANE * 2 || !Number.isFinite(available)) return 0.5;
  const min = MIN_PANE / available;
  return Math.max(min, Math.min(1 - min, Number.isFinite(ratio) ? ratio : 0.5));
}

export function NotebookSplit({ split, ratio, onRatio, primary, secondary }: {
  split: boolean; ratio: number; onRatio: (value: number) => void;
  primary: ReactNode; secondary: ReactNode;
}) {
  const container = useRef<HTMLDivElement>(null);
  const drag = useRef<number | undefined>(undefined);
  const [width, setWidth] = useState(0);
  const [mobilePane, setMobilePane] = useState<"primary" | "secondary">("primary");
  useLayoutEffect(() => {
    const element = container.current;
    if (!element) return;
    const measure = () => setWidth(element.getBoundingClientRect().width);
    measure();
    if (typeof ResizeObserver === "undefined") {
      window.addEventListener("resize", measure);
      return () => window.removeEventListener("resize", measure);
    }
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, []);
  const wide = split && width >= MIN_PANE * 2 + SEPARATOR;
  const effective = notebookSplitRatio(width, ratio);
  const update = (clientX: number) => {
    const bounds = container.current?.getBoundingClientRect();
    if (bounds) onRatio(notebookSplitRatio(bounds.width, (clientX - bounds.left - SEPARATOR / 2) / (bounds.width - SEPARATOR)));
  };
  return <div ref={container} className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
    {split && !wide && <div className="flex shrink-0 gap-1 border-b" aria-label="窗格切换">
      <Button type="button" variant="ghost" aria-pressed={mobilePane === "primary"} onClick={() => setMobilePane("primary")}>主窗格</Button>
      <Button type="button" variant="ghost" aria-pressed={mobilePane === "secondary"} onClick={() => setMobilePane("secondary")}>副窗格</Button>
    </div>}
    <div className="flex min-h-0 min-w-0 flex-1">
      <div hidden={split && !wide && mobilePane !== "primary"} className="min-w-0" style={{ flex: wide ? `0 0 ${(width - SEPARATOR) * effective}px` : "1 1 0%" }}>{primary}</div>
      {wide && <div role="separator" tabIndex={0} aria-label="调整笔记窗格宽度" aria-orientation="vertical" aria-valuemin={Math.round(MIN_PANE / (width - SEPARATOR) * 100)} aria-valuemax={Math.round((1 - MIN_PANE / (width - SEPARATOR)) * 100)} aria-valuenow={Math.round(effective * 100)}
        className="w-3 shrink-0 touch-none cursor-col-resize border-x bg-muted focus-visible:outline-2 focus-visible:outline-ring"
        onPointerDown={event => { if (event.button !== 0) return; drag.current = event.pointerId; event.currentTarget.setPointerCapture(event.pointerId); }}
        onPointerMove={event => { if (drag.current === event.pointerId) update(event.clientX); }}
        onPointerUp={event => { if (drag.current === event.pointerId) { update(event.clientX); drag.current = undefined; event.currentTarget.releasePointerCapture(event.pointerId); } }}
        onPointerCancel={() => { drag.current = undefined; }} onLostPointerCapture={() => { drag.current = undefined; }}
        onKeyDown={event => {
          const next = event.key === "ArrowLeft" ? effective - 0.02 : event.key === "ArrowRight" ? effective + 0.02 : event.key === "Home" ? 0 : event.key === "End" ? 1 : undefined;
          if (next !== undefined) { event.preventDefault(); onRatio(notebookSplitRatio(width, next)); }
        }}/>} 
      <div hidden={!split || (!wide && mobilePane !== "secondary")} className="min-w-0 flex-1">{secondary}</div>
    </div>
  </div>;
}
