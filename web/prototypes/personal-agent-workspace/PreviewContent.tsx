import { SheetPreview } from "./SheetPreview";
import { PreviewReadingTools } from "./PreviewReadingTools";
import { Input } from "@/components/ui/input";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { Pause, Play, ZoomIn, ZoomOut, Maximize2 } from "lucide-react";
import { Btn, FileArt, IconBtn } from "./ui";
import type { FileRecord } from "./model";
import { versionSnapshot } from "./model";
import type { ViewerState } from "./preview-model";
interface ContentProps {
  file: FileRecord;
  version: number;
  view: ViewerState;
  change: (patch: Partial<ViewerState>) => void;
  toolbar: HTMLElement | null;
}
export function PreviewContent(props: ContentProps) {
  if (props.file.kind === "sheet")
    return (
      <div
        className="h-full min-h-0 min-w-0"
        data-renderer={`${props.file.id}@${props.version}`}
      >
        <SheetPreview
          version={props.version}
          reading={props.view.sheet}
          change={(sheet) => props.change({ sheet })}
          toolbar={props.toolbar}
        />
      </div>
    );
  return <MediaPreviewContent {...props} />;
}
function MediaPreviewContent({
  file: f,
  version,
  view,
  change,
  toolbar,
}: ContentProps) {
  const reader = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState({ width: 500, height: 450 });
  const [playing, setPlaying] = useState(false);
  useLayoutEffect(() => {
    const el = reader.current!;
    el.scrollTop = view.scrollTop;
    el.scrollLeft = view.scrollLeft;
    const observer = new ResizeObserver((e) =>
      setSize({
        width: e[0].contentRect.width,
        height: e[0].contentRect.height,
      }),
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, []);
  useEffect(() => {
    if (!playing) return;
    const timer = setInterval(
      () => change({ time: view.time >= 42 ? 0 : view.time + 1 }),
      1000,
    );
    return () => clearInterval(timer);
  }, [playing, view.time]);
  const visual = f.kind === "image" || f.kind === "video";
  const mediaWidth =
    Math.max(120, Math.min(size.width - 32, ((size.height - 32) * 4) / 3)) *
    view.zoom;
  const zoomControls = (
    <div className="flex items-center gap-1" role="group" aria-label="预览缩放">
      <IconBtn
        label="缩小预览"
        onClick={() => change({ zoom: Math.max(0.5, view.zoom - 0.25) })}
      >
        <ZoomOut size={16} />
      </IconBtn>
      <Btn variant="ghost" onClick={() => change({ zoom: 1 })}>
        {Math.round(view.zoom * 100)}%
      </Btn>
      <IconBtn
        label="放大预览"
        onClick={() => change({ zoom: Math.min(3, view.zoom + 0.25) })}
      >
        <ZoomIn size={16} />
      </IconBtn>
      <IconBtn
        label={f.kind === "document" ? "适应宽度" : "适应窗口"}
        onClick={() => change({ zoom: 1 })}
      >
        <Maximize2 size={16} />
      </IconBtn>
    </div>
  );
  const pageTo = (page: number) => {
    reader.current?.scrollTo({ top: 0 });
    change({ page: Math.max(1, Math.min(8, page)), scrollTop: 0 });
  };
  const media = ["audio", "video"].includes(f.kind);
  const mediaControls = (
    <div
      role="group"
      aria-label="媒体播放控制"
      tabIndex={0}
      className="absolute bottom-4 left-3 right-3 mx-auto flex h-11 max-w-md items-center gap-2 rounded-lg border bg-card/95 px-2 shadow-sm outline-none focus-visible:ring-2 focus-visible:ring-primary"
      onKeyDown={(e) => {
        if (e.target !== e.currentTarget) return;
        if (e.key === " " || e.key === "k") {
          e.preventDefault();
          setPlaying((p) => !p);
        }
        if (["ArrowLeft", "ArrowRight"].includes(e.key)) {
          e.preventDefault();
          change({
            time: Math.max(
              0,
              Math.min(42, view.time + (e.key === "ArrowRight" ? 5 : -5)),
            ),
          });
        }
      }}
    >
      <IconBtn
        label={playing ? "暂停模拟播放" : "播放模拟片段"}
        onClick={() => setPlaying(!playing)}
      >
        {playing ? <Pause size={17} /> : <Play size={17} />}
      </IconBtn>
      <input
        type="range"
        aria-label="播放位置"
        min={0}
        max={42}
        value={view.time}
        className="h-11 min-w-0 flex-1 accent-[var(--ui-primary)]"
        onChange={(e) => change({ time: Number(e.target.value) })}
      />
      <span className="shrink-0 text-[10px] tabular-nums">
        00:{String(view.time).padStart(2, "0")} / 00:42
      </span>
    </div>
  );
  return (
    <div
      className="relative flex h-full min-h-0 flex-col"
      data-renderer={`${f.id}@${version}`}
    >
      {!media && (
        <PreviewReadingTools
          toolbar={toolbar}
          summary={
            f.kind === "document"
              ? `${view.page}/8 页 · ${Math.round(view.zoom * 100)}%`
              : `${Math.round(view.zoom * 100)}%`
          }
          inline={zoomControls}
        >
          {zoomControls}
          {f.kind === "document" && (
            <>
              <div className="flex items-center justify-between">
                <Btn
                  variant="outline"
                  disabled={view.page <= 1}
                  onClick={() => pageTo(view.page - 1)}
                >
                  上一页
                </Btn>
                <span className="text-xs">第 {view.page} / 8 页</span>
                <Btn
                  variant="outline"
                  disabled={view.page >= 8}
                  onClick={() => pageTo(view.page + 1)}
                >
                  下一页
                </Btn>
              </div>
              <label className="block space-y-1 text-xs">
                <span>跳转文档页码</span>
                <Input
                  aria-label="文档页码"
                  type="number"
                  min={1}
                  max={8}
                  value={view.page}
                  onChange={(e) => pageTo(Number(e.target.value) || 1)}
                />
              </label>
            </>
          )}
        </PreviewReadingTools>
      )}
      <div
        ref={reader}
        data-reader
        className="min-h-0 flex-1 overflow-auto overscroll-contain bg-muted/40 p-4"
        onScroll={(e) =>
          change({
            scrollTop: e.currentTarget.scrollTop,
            scrollLeft: e.currentTarget.scrollLeft,
          })
        }
      >
        {visual && (
          <div
            className="mx-auto flex min-h-full items-center justify-center"
            style={{ width: Math.max(mediaWidth, size.width - 32) }}
          >
            <div
              style={{ width: mediaWidth }}
              data-media-frame={f.kind === "video" ? "video" : undefined}
              className="relative shrink-0 overflow-hidden rounded-lg border border-border bg-card"
            >
              <FileArt file={f} version={version} large />
              {f.kind === "video" && mediaControls}
            </div>
          </div>
        )}
        {f.kind === "document" && (
          <article
            className="mx-auto min-h-full max-w-3xl rounded-lg border border-border bg-card p-5 sm:p-10"
            style={{ zoom: view.zoom }}
          >
            <p className="text-xs text-primary">文档预览 · 第 {view.page} 页</p>
            <h2 className="mt-5 break-words text-2xl font-semibold">
              {f.name}
            </h2>
            <p className="mt-2 text-xs text-muted-foreground">
              {f.externalRevision
                ? `外部修订 ${f.externalRevision}`
                : `版本 ${version}`}{" "}
              · 示例内容
            </p>
            <h3 className="mt-8 font-semibold">01 / 回到自然的节奏</h3>
            <p className="mt-4 text-sm leading-8">
              我们想记录一次没有匆忙赶路的山行。让自然光成为画面的线索，让风和溪流留在叙述之间。
            </p>
            <blockquote className="my-6 border-l-2 border-primary pl-4 text-sm leading-8">
              {versionSnapshot(f, version).content}
            </blockquote>
            <h3 className="mt-8 font-semibold">02 / 内容与素材</h3>
            <p className="mt-4 text-sm leading-8">
              开场：抵达山脚，交代环境。
              <br />
              中段：停留、观察，记录人与自然的关系。
              <br />
              结尾：沿着溪流归途，让环境声延续。
            </p>
            <div className="my-8 h-px bg-border" />
            <p className="text-xs leading-7 text-muted-foreground">
              示例预览，不读取真实文件。滚动和缩放按文件与版本保留；详情切换不会重新加载正文。
            </p>
          </article>
        )}
        {f.kind === "audio" && (
          <div
            data-media-frame="audio"
            className="relative flex min-h-full flex-col items-center justify-center gap-8 pb-16"
          >
            <h2 className="text-center text-lg font-semibold">{f.name}</h2>
            <div
              className="flex items-center gap-1.5"
              aria-label="示例音频波形"
            >
              {Array.from({ length: 26 }, (_, i) => (
                <span
                  key={i}
                  className={`w-1.5 rounded-full bg-primary/60 ${i % 4 === 0 ? "h-20" : i % 3 === 0 ? "h-10" : "h-14"}`}
                />
              ))}
            </div>
            <p className="text-xs text-muted-foreground">
              模拟波形 · 无真实音频
            </p>
            {mediaControls}
          </div>
        )}
      </div>
    </div>
  );
}
