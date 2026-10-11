import "./preview-reader.css";
import { Input } from "./components/ui/input";
import { Button } from "./components/ui/button";
import { SelectAdapter } from "./components/SelectAdapter";
import React, {
  lazy,
  Suspense,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import {
  ArrowUpRight,
  Camera,
  ChevronLeft,
  ChevronRight,
  Download,
  File,
  FileCode2,
  FileText,
  FileSpreadsheet,
  Film,
  FlipHorizontal2,
  FlipVertical2,
  Globe,
  Image as ImageIcon,
  Info,
  LoaderCircle,
  Maximize2,
  Minimize2,
  Music,
  Pencil,
  Play,
  Circle,
  Type,
  Hand,
  Redo2,
  PenLine,
  PictureInPicture2,
  Repeat,
  RotateCcw,
  RotateCw,
  Square,
  Trash2,
  Undo2,
  X,
  ZoomIn,
  ZoomOut,
} from "lucide-react";
import { contextURL, api, bytes, errorText, type DriveFile } from "./api";
import { spreadsheetFormat } from "./spreadsheet-source";
import { HTML_PREVIEW_LIMIT, interactivePreviewURL, readPreviewHTML } from "./html-preview";
import {
  clampZoom,
  composeExport,
  displayToNatural,
  drawStrokes,
  fitScale,
  formatDuration,
  formatExposure,
  formatGps,
  isJpegFile,
  orientationLabel,
  parseJpegExif,
  parsePngHeader,
  panForZoom,
  pngColorName,
  textStats,
  type AnnotationTool,
  type ExifData,
  type PngHeader,
  type Point,
  type ViewerStroke,
} from "./media-viewer";

export interface PreviewResult {
  id: string;
  status: string;
  mime_type: string;
  error?: string;
  kind: string;
}
export interface ViewerMeta {
  width?: number;
  height?: number;
  duration?: number;
  exif?: ExifData;
  png?: PngHeader;
  text?: { lines: number; chars: number };
}
function mediaContextURL(file: DriveFile, path: string) {
  const target = new URL(contextURL(path), location.origin);
  if (file.tenant_context) {
    target.searchParams.set("tenant_context", file.tenant_context);
    target.searchParams.set("space_context", file.space_id);
  }
  return target.pathname + target.search;
}
function mediaHeaders(file: DriveFile): Record<string, string> {
  return file.tenant_context
    ? {
        "X-DriveBridge-Tenant": file.tenant_context,
        "X-DriveBridge-Space": file.space_id,
      }
    : {};
}
export function mediaURL(file: DriveFile) {
  return mediaContextURL(
    file,
    `/api/v1/files/${encodeURIComponent(file.id)}/media?version=${encodeURIComponent(file.version_id)}`,
  );
}
const CONVERTED_EXTENSIONS =
  /^(psd|psb|tif|tiff|dng|cr2|cr3|nef|arw|raf|orf|rw2|docx|pptx)$/;
const IMAGE_EXTENSIONS = /^(png|jpe?g|gif|webp|avif|bmp|svg|ico)$/;
const IMAGE_MIME =
  /^image\/(png|jpeg|gif|webp|avif|bmp|svg\+xml|x-icon|vnd\.microsoft\.icon)$/;
const VIDEO_EXTENSIONS = /^(mp4|webm|m4v|mov|mkv|avi|wmv|flv|mpg|mpeg|ts)$/;
const AUDIO_EXTENSIONS = /^(mp3|wav|ogg|oga|m4a|flac|aac|opus)$/;
const TEXT_EXTENSIONS =
  /^(txt|md|markdown|json|xml|yml|yaml|toml|ini|cfg|conf|csv|tsv|log|env|properties|js|mjs|cjs|ts|tsx|jsx|css|scss|less|go|py|rb|rs|java|kt|swift|c|h|cpp|hpp|cs|php|sh|bash|zsh|sql|graphql|proto|dockerfile|gitignore|makefile)$/;
const TEXT_MIME =
  /^(text\/|application\/(json|xml|javascript|x-yaml|yaml|sql|x-sh|ecmascript))/;
const HTML_EXTENSIONS = /^(html?|xhtml)$/;
const HTML_MIME = /^(text\/html|application\/xhtml\+xml)/;
const VECTOR_EXTENSIONS = /^(svg|ico)$/;
const VECTOR_MIME = /^image\/(svg\+xml|x-icon|vnd\.microsoft\.icon)/;
const SpreadsheetPreview = lazy(() => import("./SpreadsheetPreview"));
const PdfPreview = lazy(() => import("./PdfPreview"));

export function mediaKind(file: Pick<DriveFile, "mime_type" | "path">) {
  const ext = file.path.split(".").pop()?.toLowerCase() || "";
  const generic =
    !file.mime_type || file.mime_type === "application/octet-stream";
  if (spreadsheetFormat(file)) return "spreadsheet";
  if (CONVERTED_EXTENSIONS.test(ext)) return "converted";
  if (IMAGE_EXTENSIONS.test(ext) || IMAGE_MIME.test(file.mime_type))
    return "image";
  if (file.mime_type.startsWith("video/")) return "video";
  if (generic && VIDEO_EXTENSIONS.test(ext)) return "video";
  if (file.mime_type.startsWith("audio/")) return "audio";
  if (generic && AUDIO_EXTENSIONS.test(ext)) return "audio";
  if (file.mime_type === "application/pdf" || ext === "pdf") return "pdf";
  if (HTML_EXTENSIONS.test(ext) || HTML_MIME.test(file.mime_type))
    return "html";
  if (TEXT_MIME.test(file.mime_type) || (generic && TEXT_EXTENSIONS.test(ext)))
    return "text";
  return "unsupported";
}
export function isVectorImageFile(
  file: Pick<DriveFile, "mime_type" | "path">,
): boolean {
  const ext = file.path.split(".").pop()?.toLowerCase() || "";
  return VECTOR_EXTENSIONS.test(ext) || VECTOR_MIME.test(file.mime_type);
}
export function MediaIcon({
  file,
  size = 20,
}: {
  file: Pick<DriveFile, "mime_type" | "path">;
  size?: number;
}) {
  const kind = mediaKind(file);
  const icon =
    kind === "image" || kind === "converted" ? (
      <ImageIcon size={size} />
    ) : kind === "video" ? (
      <Film size={size} />
    ) : kind === "audio" ? (
      <Music size={size} />
    ) : kind === "spreadsheet" ? (
      <FileSpreadsheet size={size} />
    ) : kind === "pdf" ? (
      <FileText size={size} />
    ) : kind === "text" ? (
      <FileCode2 size={size} />
    ) : kind === "html" ? (
      <Globe size={size} />
    ) : (
      <File size={size} />
    );
  return <span className={"media-kind-" + kind}>{icon}</span>;
}
function useDerivative(
  file: DriveFile | undefined,
  kind: "thumbnail" | "compatible",
  enabled: boolean,
  retry: number,
) {
  const [result, setResult] = useState<PreviewResult>();
  const [error, setError] = useState("");
  useEffect(() => {
    setResult(undefined);
    setError("");
    if (!enabled || !file) return;
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout>;
    const run = async () => {
      try {
        const next = await api<PreviewResult>(
          `/files/${encodeURIComponent(file.id)}/previews`,
          "POST",
          { version_id: file.version_id, kind },
          mediaHeaders(file),
          controller.signal,
        );
        if (controller.signal.aborted) return;
        setResult(next);
        if (next.status === "processing") {
          const poll = async () => {
            try {
              const rows = await api<PreviewResult[]>(
                `/files/${encodeURIComponent(file.id)}/previews?version=${encodeURIComponent(file.version_id)}`,
                "GET",
                undefined,
                mediaHeaders(file),
                controller.signal,
              );
              const current = rows.find((row) => row.id === next.id);
              if (controller.signal.aborted) return;
              setResult(current);
              if (current?.status === "processing")
                timer = setTimeout(poll, 1500);
            } catch (e) {
              if (!controller.signal.aborted) setError(errorText(e));
            }
          };
          timer = setTimeout(poll, 1500);
        }
      } catch (e) {
        if (!controller.signal.aborted) setError(errorText(e));
      }
    };
    void run();
    return () => {
      controller.abort();
      clearTimeout(timer);
    };
  }, [
    file?.id,
    file?.version_id,
    file?.tenant_context,
    file?.space_id,
    kind,
    enabled,
    retry,
  ]);
  return { result, error };
}
export function MediaThumbnail({
  file,
  localURL,
}: {
  file: DriveFile;
  localURL?: string;
}) {
  const ref = useRef<HTMLSpanElement>(null);
  const [visible, setVisible] = useState(false);
  const [failed, setFailed] = useState(false);
  const kind = mediaKind(file);
  localURL ||= file.thumbnail_url || file.preview_url;
  useEffect(() => {
    if (!ref.current || typeof IntersectionObserver === "undefined") {
      setVisible(true);
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) {
          setVisible(true);
          observer.disconnect();
        }
      },
      { rootMargin: "120px" },
    );
    observer.observe(ref.current);
    return () => observer.disconnect();
  }, []);
  useEffect(() => setFailed(false), [file.id, file.version_id]);
  const { result } = useDerivative(
    file,
    "thumbnail",
    visible &&
      !file.preview_reason &&
      !localURL &&
      ["image", "video", "converted", "pdf"].includes(kind) &&
      !file.trashed,
    0,
  );
  const src = file.preview_reason
    ? ""
    : localURL ||
      (result?.status === "ready"
        ? mediaContextURL(
            file,
            `/api/v1/previews/${encodeURIComponent(result.id)}/content`,
          )
        : kind === "image" && file.size_bytes <= 5 * 1024 * 1024
          ? mediaURL(file)
          : "");
  return (
    <span ref={ref} className={`media-thumbnail ${kind}`} aria-hidden="true">
      {visible && src && !failed ? (
        <img src={src} alt="" loading="lazy" onError={() => setFailed(true)} />
      ) : (
        <span className="media-fallback">
          <MediaIcon file={file} />
          <span>{kind === "video" ? "视频" : kind === "audio" ? "音频" : kind === "image" ? "图片" : kind === "pdf" ? "PDF" : kind === "converted" ? "设计文档" : kind === "html" ? "网页" : "文档"}</span>
        </span>
      )}
      {kind === "video" && (
        <span className="media-badge">
          <Film size={11} />
        </span>
      )}
    </span>
  );
}
const ANNOTATION_COLORS = [
  "#e5484d",
  "#f76b15",
  "#30a46c",
  "#0090ff",
  "#8e4ec6",
  "#ffffff",
];
const ANNOTATION_WIDTHS = [3, 6, 12];
const PLAYBACK_RATES = [0.25, 0.5, 0.75, 1, 1.25, 1.5, 2];
const EXIF_FETCH_LIMIT = 20 * 1024 * 1024;
function useFullscreenState() {
  const [fullscreen, setFullscreen] = useState(false);
  useEffect(() => {
    const sync = () => setFullscreen(!!document.fullscreenElement);
    document.addEventListener("fullscreenchange", sync);
    return () => document.removeEventListener("fullscreenchange", sync);
  }, []);
  return fullscreen;
}
async function toggleStageFullscreen(stage: HTMLDivElement | null) {
  if (!stage?.requestFullscreen) return;
  if (document.fullscreenElement) await document.exitFullscreen();
  else await stage.requestFullscreen().catch(() => {});
}
function downloadCanvas(canvas: HTMLCanvasElement, name: string) {
  canvas.toBlob((blob) => {
    if (!blob) return;
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = name;
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 4000);
  }, "image/png");
}
function baseName(file: DriveFile) {
  return file.path.split("/").pop() || "file";
}
export function ViewerInfoPanel({
  file,
  meta,
  onClose,
}: {
  file: DriveFile;
  meta: ViewerMeta;
  onClose: () => void;
}) {
  const sections: { title: string; rows: [string, string][] }[] = [];
  const base: [string, string][] = [];
  if (meta.width && meta.height)
    base.push(["尺寸", `${meta.width} × ${meta.height} px`]);
  if (meta.duration) base.push(["时长", formatDuration(meta.duration)]);
  base.push(["类型", file.mime_type || "未知"]);
  base.push(["大小", bytes(file.size_bytes)]);
  sections.push({ title: "基础", rows: base });
  const exif = meta.exif;
  if (exif) {
    const rows: [string, string][] = [];
    const camera = [exif.make, exif.model].filter(Boolean).join(" ");
    if (camera) rows.push(["相机", camera]);
    if (exif.lensModel) rows.push(["镜头", exif.lensModel]);
    const taken = exif.dateTimeOriginal || exif.dateTime;
    if (taken) rows.push(["拍摄时间", taken]);
    if (exif.exposureTime) rows.push(["快门", formatExposure(exif.exposureTime)]);
    if (exif.fNumber) rows.push(["光圈", `f/${Math.round(exif.fNumber * 10) / 10}`]);
    if (exif.iso) rows.push(["ISO", String(exif.iso)]);
    if (exif.focalLength) rows.push(["焦距", `${Math.round(exif.focalLength)} mm`]);
    const direction = orientationLabel(exif.orientation);
    if (direction) rows.push(["方向", direction]);
    if (exif.gps)
      rows.push([
        "位置",
        `${formatGps(exif.gps.latitude, "N", "S")} ${formatGps(exif.gps.longitude, "E", "W")}`,
      ]);
    if (rows.length) sections.push({ title: "拍摄信息", rows });
  }
  if (meta.png)
    sections.push({
      title: "PNG",
      rows: [
        ["位深", `${meta.png.bitDepth} bit`],
        ["颜色", pngColorName(meta.png.colorType)],
      ],
    });
  if (meta.text)
    sections.push({
      title: "文本",
      rows: [
        ["行数", String(meta.text.lines)],
        ["字符数", String(meta.text.chars)],
      ],
    });
  return (
    <aside
      className="viewer-info"
      aria-label="文件信息面板"
      onPointerDown={(event) => event.stopPropagation()}
      onDoubleClick={(event) => event.stopPropagation()}
    >
      <header>
        <strong>信息</strong>
        <Button variant="ghost" size="icon" type="submit" className="icon" aria-label="收起信息" onClick={onClose}>
          <X size={14} />
        </Button>
      </header>
      {sections.map((section) => (
        <section key={section.title}>
          <h4>{section.title}</h4>
          <dl>
            {section.rows.map(([label, value]) => (
              <div key={label}>
                <dt>{label}</dt>
                <dd>{value}</dd>
              </div>
            ))}
          </dl>
        </section>
      ))}
    </aside>
  );
}
function ImageViewer({
  file,
  src,
  meta,
  onMeta,
  onFailed,
}: {
  file: DriveFile;
  src: string;
  meta: ViewerMeta;
  onMeta: (patch: Partial<ViewerMeta>) => void;
  onFailed: () => void;
}) {
  const stageRef = useRef<HTMLDivElement>(null);
  const layerRef = useRef<HTMLDivElement>(null);
  const imgRef = useRef<HTMLImageElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [natural, setNatural] = useState({ width: 0, height: 0 });
  const [fit, setFit] = useState(1);
  const [zoom, setZoom] = useState(1);
  const [pan, setPan] = useState<Point>({ x: 0, y: 0 });
  const [rotDeg, setRotDeg] = useState(0);
  const [flipH, setFlipH] = useState(false);
  const [flipV, setFlipV] = useState(false);
  const [annotate, setAnnotate] = useState(false);
  const [tool, setTool] = useState<AnnotationTool>("pen");
  const [color, setColor] = useState(ANNOTATION_COLORS[0]);
  const [strokeWidth, setStrokeWidth] = useState(ANNOTATION_WIDTHS[1]);
  const [strokes, setStrokes] = useState<ViewerStroke[]>([]);
  const [redo, setRedo] = useState<ViewerStroke[][]>([]);
  const [undo, setUndo] = useState<ViewerStroke[][]>([]);
  const [textLabel, setTextLabel] = useState("");
  const [textSize, setTextSize] = useState(24);
  const [panning, setPanning] = useState(false);
  const commit = (next: ViewerStroke[]) => {
    setUndo(history => [...history, strokes]);
    setStrokes(next);
    setRedo([]);
  };
  const undoStroke = () => {
    if (!undo.length) return;
    setRedo(history => [...history, strokes]);
    setStrokes(undo[undo.length - 1]);
    setUndo(history => history.slice(0, -1));
  };
  const redoStroke = () => {
    if (!redo.length) return;
    setUndo(history => [...history, strokes]);
    setStrokes(redo[redo.length - 1]);
    setRedo(history => history.slice(0, -1));
  };
  const [preview, setPreview] = useState<ViewerStroke | null>(null);
  const [infoOpen, setInfoOpen] = useState(false);
  const [dragging, setDragging] = useState(false);
  const [note, setNote] = useState("");
  const fullscreen = useFullscreenState();
  const view = useRef({ zoom, natural, rotDeg, flipH, flipV });
  view.current = { zoom, natural, rotDeg, flipH, flipV };
  const pointers = useRef(new Map<number, Point>());
  const pinch = useRef<number | null>(null);
  const dragLast = useRef<Point | null>(null);
  const drawing = useRef<ViewerStroke | null>(null);
  const measure = useCallback((size: { width: number; height: number }) => {
    const stage = stageRef.current;
    if (!stage || !size.width) return;
    const box = stage.getBoundingClientRect();
    setFit(fitScale(size, { width: box.width, height: box.height }));
  }, []);
  const stageCenter = () => {
    const box = stageRef.current?.getBoundingClientRect();
    return box
      ? { x: box.left + box.width / 2, y: box.top + box.height / 2 }
      : { x: 0, y: 0 };
  };
  const applyZoomAt = useCallback(
    (nextZoom: number, cursor: Point) => {
      const layer = layerRef.current;
      const stage = stageRef.current;
      const current = view.current;
      if (!layer || !stage || !current.natural.width) return;
      const next = clampZoom(nextZoom);
      const nextPan = panForZoom({
        rect: layer.getBoundingClientRect(),
        stage: stage.getBoundingClientRect(),
        natural: current.natural,
        zoom: current.zoom,
        nextZoom: next,
        rotDeg: current.rotDeg,
        flipH: current.flipH,
        flipV: current.flipV,
        cursor,
      });
      setZoom(next);
      setPan(nextPan);
    },
    [],
  );
  const onImgLoad = () => {
    const img = imgRef.current;
    if (!img || !img.naturalWidth) return;
    const size = { width: img.naturalWidth, height: img.naturalHeight };
    setNatural(size);
    setFit(fitScale(size, stageRef.current?.getBoundingClientRect() || { width: 0, height: 0 }));
    setZoom(1);
    setPan({ x: 0, y: 0 });
    onMeta({ width: size.width, height: size.height });
  };
  useEffect(() => {
    const stage = stageRef.current;
    if (!stage || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(() => measure(natural));
    observer.observe(stage);
    return () => observer.disconnect();
  }, [measure, natural]);
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || !natural.width) return;
    if (canvas.width !== natural.width || canvas.height !== natural.height) {
      canvas.width = natural.width;
      canvas.height = natural.height;
    }
    const ctx = canvas.getContext("2d");
    if (ctx) {
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      drawStrokes(ctx, preview ? [...strokes, preview] : strokes);
    }
  }, [strokes, preview, natural]);
  useEffect(() => {
    if (file.size_bytes > EXIF_FETCH_LIMIT) return;
    const jpeg = isJpegFile(file);
    const png = mediaKind(file) === "image" && /\.png$/i.test(file.path);
    if (!jpeg && !png) return;
    const controller = new AbortController();
    fetch(src, { signal: controller.signal, credentials: "same-origin" })
      .then((r) => {
        if (!r.ok) throw new Error("metadata");
        return r.arrayBuffer();
      })
      .then((buffer) => {
        if (controller.signal.aborted) return;
        if (jpeg) {
          const exif = parseJpegExif(buffer);
          if (Object.keys(exif).length) onMeta({ exif });
        } else {
          const header = parsePngHeader(buffer);
          if (header) onMeta({ png: header });
        }
      })
      .catch(() => {});
    return () => controller.abort();
  }, [src, file.id, file.version_id]);
  useEffect(() => {
    const stage = stageRef.current;
    if (!stage) return;
    const handler = (event: WheelEvent) => {
      if (
        event.target instanceof Element &&
        event.target.closest(".viewer-toolbar, .viewer-info")
      )
        return;
      event.preventDefault();
      const factor = event.deltaY < 0 ? 1.2 : 1 / 1.2;
      applyZoomAt(view.current.zoom * factor, {
        x: event.clientX,
        y: event.clientY,
      });
    };
    stage.addEventListener("wheel", handler, { passive: false });
    return () => stage.removeEventListener("wheel", handler);
  }, [applyZoomAt]);
  const toNatural = (event: React.PointerEvent) => {
    const layer = layerRef.current;
    const current = view.current;
    if (!layer) return { x: 0, y: 0 };
    return displayToNatural(event.clientX, event.clientY, {
      rect: layer.getBoundingClientRect(),
      natural: current.natural,
      zoom: fit * current.zoom,
      rotDeg: current.rotDeg,
      flipH: current.flipH,
      flipV: current.flipV,
    });
  };
  const drawingTool = annotate && !panning;
  const onPointerDown = (event: React.PointerEvent) => {
    if (event.button !== 0) return;
    pointers.current.set(event.pointerId, { x: event.clientX, y: event.clientY });
    if (pointers.current.size === 2) {
      const [first, second] = [...pointers.current.values()];
      pinch.current = Math.hypot(first.x - second.x, first.y - second.y);
      dragLast.current = null;
      drawing.current = null;
      setPreview(null);
      return;
    }
    if (drawingTool) {
      if (tool === "text") {
        if (textLabel.trim()) commit([...strokes, { tool, color, width: strokeWidth, points: [toNatural(event)], text: textLabel.trim(), fontSize: textSize / Math.max(fit * zoom, 0.01) }]);
        else setNote("先输入标注文字，再点击图片放置");
        return;
      }
      stageRef.current?.setPointerCapture(event.pointerId);
      drawing.current = {
        tool,
        color,
        width: strokeWidth / Math.max(fit * zoom, 0.01),
        points: [toNatural(event)],
      };
      return;
    }
    stageRef.current?.setPointerCapture(event.pointerId);
    dragLast.current = { x: event.clientX, y: event.clientY };
    setDragging(true);
  };
  const onPointerMove = (event: React.PointerEvent) => {
    if (!pointers.current.has(event.pointerId)) return;
    pointers.current.set(event.pointerId, { x: event.clientX, y: event.clientY });
    if (pointers.current.size === 2 && pinch.current) {
      const [first, second] = [...pointers.current.values()];
      const dist = Math.hypot(first.x - second.x, first.y - second.y);
      const mid = { x: (first.x + second.x) / 2, y: (first.y + second.y) / 2 };
      applyZoomAt((view.current.zoom * dist) / (pinch.current || dist), mid);
      pinch.current = dist;
      return;
    }
    if (drawing.current) {
      drawing.current.points.push(toNatural(event));
      setPreview({ ...drawing.current, points: [...drawing.current.points] });
      return;
    }
    const last = dragLast.current;
    if (last) {
      setPan((current) => ({
        x: current.x + event.clientX - last.x,
        y: current.y + event.clientY - last.y,
      }));
      dragLast.current = { x: event.clientX, y: event.clientY };
    }
  };
  const onPointerUp = (event: React.PointerEvent) => {
    pointers.current.delete(event.pointerId);
    if (pointers.current.size < 2) pinch.current = null;
    setDragging(false);
    dragLast.current = null;
    const stroke = drawing.current;
    drawing.current = null;
    setPreview(null);
    if (!stroke) return;
    const from = stroke.points[0];
    const to = stroke.points[stroke.points.length - 1];
    const moved =
      stroke.points.length > 1 && Math.hypot(to.x - from.x, to.y - from.y) > 1;
    if (moved) commit([...strokes, stroke]);
  };
  const fitView = () => {
    setZoom(1);
    setPan({ x: 0, y: 0 });
  };
  const actualSize = () => {
    setZoom(clampZoom(fit ? 1 / fit : 1));
    setPan({ x: 0, y: 0 });
  };
  const zoomStep = (factor: number) =>
    applyZoomAt(view.current.zoom * factor, stageCenter());
  const rotate = (delta: number) => {
    setRotDeg((current) => (((current + delta) % 360) + 360) % 360);
    setZoom(1);
    setPan({ x: 0, y: 0 });
  };
  const onKeyDown = (event: React.KeyboardEvent) => {
    if ((event.target as HTMLElement).closest("input,textarea,select")) return;
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "z") {
      event.preventDefault(); event.stopPropagation();
      if (event.shiftKey) redoStroke(); else undoStroke();
      return;
    }
    if (event.key === "+" || event.key === "=") zoomStep(1.25);
    else if (event.key === "-") zoomStep(1 / 1.25);
    else if (event.key === "0") fitView();
    else if (event.key === "1") actualSize();
    else if (event.key === "r") rotate(90);
    else return;
    event.preventDefault();
  };
  const exportAnnotated = () => {
    const img = imgRef.current;
    const canvas = canvasRef.current;
    if (!img || !canvas || !natural.width) return;
    try {
      const out = composeExport(img, natural.width, natural.height, canvas, {
        rotDeg,
        flipH,
        flipV,
      });
      if (!out) throw new Error("export");
      downloadCanvas(out, baseName(file).replace(/\.[^.]+$/, "") + "-标注.png");
      setNote("已导出标注图");
    } catch {
      setNote("导出失败：图片受浏览器安全限制");
    }
    setTimeout(() => setNote(""), 3200);
  };
  const scale = fit * zoom;
  const effectiveScale = scale * (flipH ? -1 : 1);
  const effectiveScaleY = scale * (flipV ? -1 : 1);
  return (
    <div
      ref={stageRef}
      className={
        "viewer-stage image" +
        (fullscreen ? " fullscreen" : "") +
        (dragging ? " dragging" : "")
      }
      tabIndex={0}
      aria-label="图片查看器"
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerUp}
      onDoubleClick={() => {
        if (!drawingTool) (Math.abs(zoom - 1) < 0.01 ? actualSize : fitView)();
      }}
      onKeyDown={onKeyDown}
    >
      <div
        ref={layerRef}
        className="viewer-layer"
        style={{
          width: natural.width || undefined,
          height: natural.height || undefined,
          transform: `translate(${pan.x}px, ${pan.y}px) rotate(${rotDeg}deg) scale(${effectiveScale}, ${effectiveScaleY})`,
        }}
      >
        <img
          ref={imgRef}
          src={src}
          alt={file.path}
          draggable={false}
          onLoad={onImgLoad}
          onError={onFailed}
        />
        <canvas
          ref={canvasRef}
          className={"viewer-canvas" + (drawingTool ? " drawing" : "")}
        />
      </div>
      {!natural.width && (
        <div className="media-placeholder" role="status">
          <LoaderCircle className="spin" />
        </div>
      )}
      {infoOpen && (
        <ViewerInfoPanel
          file={file}
          meta={meta}
          onClose={() => setInfoOpen(false)}
        />
      )}
      {note && <span className="viewer-note">{note}</span>}
      <div
        className="viewer-toolbar"
        onPointerDown={(event) => event.stopPropagation()}
        onDoubleClick={(event) => event.stopPropagation()}
      >
        <Button variant="ghost" size="icon" type="submit"
          className="icon"
          aria-label="缩小"
          title="缩小（-）"
          onClick={() => zoomStep(1 / 1.25)}
        >
          <ZoomOut size={16} />
        </Button>
        <Button variant="ghost" type="submit"
          className="percent"
          aria-label="适应窗口"
          title="适应窗口（0）"
          onClick={fitView}
        >
          {natural.width ? Math.round(scale * 100) + "%" : "—"}
        </Button>
        <Button variant="ghost" size="icon" type="submit"
          className="icon"
          aria-label="放大"
          title="放大（+）"
          onClick={() => zoomStep(1.25)}
        >
          <ZoomIn size={16} />
        </Button>
        <Button variant="ghost" type="submit" className="text" aria-label="实际大小" title="实际大小（1）" onClick={actualSize}>
          1:1
        </Button>
        <span className="divider" />
        <Button variant="ghost" size="icon" type="submit" className="icon" aria-label="向左旋转" title="向左旋转（R）" onClick={() => rotate(-90)}>
          <RotateCcw size={16} />
        </Button>
        <Button variant="ghost" size="icon" type="submit" className="icon" aria-label="向右旋转" title="向右旋转" onClick={() => rotate(90)}>
          <RotateCw size={16} />
        </Button>
        <Button variant="ghost" type="submit"
          className={"icon" + (flipH ? " active" : "")}
          aria-label="水平镜像"
          title="水平镜像"
          onClick={() => setFlipH((value) => !value)}
        >
          <FlipHorizontal2 size={16} />
        </Button>
        <Button variant="ghost" type="submit"
          className={"icon" + (flipV ? " active" : "")}
          aria-label="垂直镜像"
          title="垂直镜像"
          onClick={() => setFlipV((value) => !value)}
        >
          <FlipVertical2 size={16} />
        </Button>
        <span className="divider" />
        <Button variant="ghost" type="submit"
          className={"icon" + (annotate ? " active" : "")}
          aria-label="标注"
          title="标注工具"
          onClick={() => setAnnotate((value) => !value)}
        >
          <PenLine size={16} />
        </Button>
        <Button variant="ghost" type="submit"
          className={"icon" + (infoOpen ? " active" : "")}
          aria-label="文件信息"
          title="文件信息"
          onClick={() => setInfoOpen((value) => !value)}
        >
          <Info size={16} />
        </Button>
        <Button variant="ghost" size="icon" type="submit"
          className="icon"
          aria-label={fullscreen ? "退出全屏" : "全屏"}
          title={fullscreen ? "退出全屏" : "全屏"}
          onClick={() => void toggleStageFullscreen(stageRef.current)}
        >
          {fullscreen ? <Minimize2 size={16} /> : <Maximize2 size={16} />}
        </Button>
        {annotate && (
          <>
            <span className="viewer-tool-break" />
            <span className="viewer-tool-label">标注</span>
            <Button variant="ghost" type="submit" aria-label="拖动画布" aria-pressed={panning} className={panning ? "active" : ""} onClick={() => setPanning(!panning)}><Hand size={18} /></Button>
            {(["pen", "rect", "ellipse", "arrow", "text"] as const).map((name) => (
              <Button variant="ghost" type="submit"
                key={name}
                className={"icon" + (!panning && tool === name ? " active" : "")}
                aria-pressed={!panning && tool === name}
                aria-label={{ pen: "画笔", rect: "矩形", ellipse: "椭圆", arrow: "箭头", text: "文字" }[name]}
                title={{ pen: "画笔", rect: "矩形", ellipse: "椭圆", arrow: "箭头", text: "文字" }[name]}
                onClick={() => { setTool(name); setPanning(false); }}
              >
                {name === "pen" ? (
                  <Pencil size={16} />
                ) : name === "rect" ? (
                  <Square size={16} />
                ) : name === "ellipse" ? <Circle size={18} /> : name === "text" ? <Type size={18} /> : (
                  <ArrowUpRight size={16} />
                )}
              </Button>
            ))}
            <span className="swatches" role="group" aria-label="标注颜色">
              {ANNOTATION_COLORS.map((value) => (
                <Button variant="ghost" type="submit"
                  key={value}
                  className={"swatch" + (color === value ? " active" : "")}
                  style={{ background: value }}
                  aria-label={"颜色 " + value}
                  aria-pressed={color === value}
                  onClick={() => setColor(value)}
                />
              ))}
            </span>
            <span className="widths" role="group" aria-label="标注粗细">
              {ANNOTATION_WIDTHS.map((value) => (
                <Button variant="ghost" type="submit"
                  key={value}
                  className={"width" + (strokeWidth === value ? " active" : "")}
                  aria-label={"粗细 " + value}
                  aria-pressed={strokeWidth === value}
                  onClick={() => setStrokeWidth(value)}
                >
                  <span style={{ width: value + 3, height: value + 3 }} />
                </Button>
              ))}
            </span>
            <Button variant="ghost" size="icon" type="submit"
              className="icon"
              aria-label="撤销笔画"
              title="撤销上一笔"
              disabled={!undo.length}
              onClick={undoStroke}
            >
              <Undo2 size={16} />
            </Button>
            <Button variant="ghost" type="submit" aria-label="重做标注" title="重做（⌘/Ctrl+Shift+Z）" disabled={!redo.length} onClick={redoStroke}><Redo2 size={18} /></Button>
            {tool === "text" && !panning && <label className="viewer-text-input">文字<Input aria-label="标注文字" value={textLabel} maxLength={200} placeholder="输入后点击图片放置" onChange={e => setTextLabel(e.target.value)} /><SelectAdapter aria-label="标注字号" value={textSize} onChange={e => setTextSize(Number(e.target.value))}>{[16,24,36,48].map(size => <option key={size} value={size}>{size} px</option>)}</SelectAdapter></label>}
            <Button variant="ghost" size="icon" type="submit"
              className="icon"
              aria-label="清空标注"
              title="清空标注"
              disabled={!strokes.length}
              onClick={() => commit([])}
            >
              <Trash2 size={16} />
            </Button>
            <Button variant="ghost" size="icon" type="submit"
              className="icon"
              aria-label="导出标注图"
              title="导出标注图 PNG"
              onClick={exportAnnotated}
            >
              <Download size={16} />
            </Button>
          </>
        )}
      </div>
    </div>
  );
}
function VideoViewer({
  file,
  src,
  meta,
  onMeta,
  onFailed,
}: {
  file: DriveFile;
  src: string;
  meta: ViewerMeta;
  onMeta: (patch: Partial<ViewerMeta>) => void;
  onFailed: () => void;
}) {
  const stageRef = useRef<HTMLDivElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const [rate, setRate] = useState(1);
  const [loop, setLoop] = useState(false);
  const [infoOpen, setInfoOpen] = useState(false);
  const fullscreen = useFullscreenState();
  const pipSupported =
    typeof document !== "undefined" && !!document.pictureInPictureEnabled;
  const onLoaded = () => {
    const video = videoRef.current;
    if (!video) return;
    onMeta({
      width: video.videoWidth,
      height: video.videoHeight,
      duration: video.duration,
    });
  };
  const changeRate = (value: number) => {
    setRate(value);
    if (videoRef.current) videoRef.current.playbackRate = value;
  };
  const step = (delta: number) => {
    const video = videoRef.current;
    if (!video) return;
    video.pause();
    video.currentTime = Math.max(
      0,
      Math.min(video.duration || Infinity, video.currentTime + delta),
    );
  };
  const snapshot = () => {
    const video = videoRef.current;
    if (!video?.videoWidth) return;
    const canvas = document.createElement("canvas");
    canvas.width = video.videoWidth;
    canvas.height = video.videoHeight;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    ctx.drawImage(video, 0, 0);
    const stamp = formatDuration(video.currentTime).replace(/:/g, "-");
    downloadCanvas(
      canvas,
      baseName(file).replace(/\.[^.]+$/, "") + "-" + stamp + ".png",
    );
  };
  const togglePip = async () => {
    const video = videoRef.current;
    if (!video?.requestPictureInPicture) return;
    if (document.pictureInPictureElement) await document.exitPictureInPicture();
    else await video.requestPictureInPicture().catch(() => {});
  };
  return (
    <div
      ref={stageRef}
      className={"viewer-stage video" + (fullscreen ? " fullscreen" : "")}
      aria-label="视频查看器"
    >
      <video
        ref={videoRef}
        key={src}
        src={src}
        controls
        playsInline
        preload="metadata"
        loop={loop}
        onLoadedMetadata={onLoaded}
        onError={onFailed}
      />
      {infoOpen && (
        <ViewerInfoPanel
          file={file}
          meta={meta}
          onClose={() => setInfoOpen(false)}
        />
      )}
      <div className="viewer-toolbar">
        <label className="viewer-rate" title="播放速度">
          <SelectAdapter
            value={rate}
            onChange={(event) => changeRate(Number(event.target.value))}
            aria-label="播放速度"
          >
            {PLAYBACK_RATES.map((value) => (
              <option key={value} value={value}>
                {value}×
              </option>
            ))}
          </SelectAdapter>
        </label>
        <span className="divider" />
        <Button variant="ghost" size="icon" type="submit" className="icon" aria-label="逐帧后退" title="逐帧后退" onClick={() => step(-1 / 30)}>
          <ChevronLeft size={16} />
        </Button>
        <Button variant="ghost" size="icon" type="submit" className="icon" aria-label="逐帧前进" title="逐帧前进" onClick={() => step(1 / 30)}>
          <ChevronRight size={16} />
        </Button>
        <Button variant="ghost" type="submit"
          className={"icon" + (loop ? " active" : "")}
          aria-label="循环播放"
          title="循环播放"
          onClick={() => setLoop((value) => !value)}
        >
          <Repeat size={16} />
        </Button>
        <span className="divider" />
        <Button variant="ghost" size="icon" type="submit" className="icon" aria-label="截取当前帧" title="截取当前帧 PNG" onClick={snapshot}>
          <Camera size={16} />
        </Button>
        {pipSupported && (
          <Button variant="ghost" size="icon" type="submit" className="icon" aria-label="画中画" title="画中画" onClick={() => void togglePip()}>
            <PictureInPicture2 size={16} />
          </Button>
        )}
        <Button variant="ghost" type="submit"
          className={"icon" + (infoOpen ? " active" : "")}
          aria-label="文件信息"
          title="文件信息"
          onClick={() => setInfoOpen((value) => !value)}
        >
          <Info size={16} />
        </Button>
        <Button variant="ghost" size="icon" type="submit"
          className="icon"
          aria-label={fullscreen ? "退出全屏" : "全屏"}
          title={fullscreen ? "退出全屏" : "全屏"}
          onClick={() => void toggleStageFullscreen(stageRef.current)}
        >
          {fullscreen ? <Minimize2 size={16} /> : <Maximize2 size={16} />}
        </Button>
      </div>
    </div>
  );
}
export function MediaPreview({
  file,
  sourceURL,
  compact = false,
}: {
  file: DriveFile;
  sourceURL?: string;
  compact?: boolean;
}) {
  const kind = mediaKind(file);
  sourceURL ||= file.preview_url;
  const [convert, setConvert] = useState(kind === "converted");
  const [failed, setFailed] = useState(false);
  const [retry, setRetry] = useState(0);
  const [meta, setMeta] = useState<ViewerMeta>({});
  const [text, setText] = useState("");
  const [textLoaded, setTextLoaded] = useState(false);
  const [error, setError] = useState("");
  const onMeta = useCallback(
    (patch: Partial<ViewerMeta>) => setMeta((current) => ({ ...current, ...patch })),
    [],
  );
  useEffect(() => {
    setConvert(kind === "converted");
    setFailed(false);
    setMeta({});
    setText("");
    setTextLoaded(false);
    setError("");
  }, [file.id, file.version_id, kind]);
  const { result, error: conversionError } = useDerivative(
    file,
    "compatible",
    convert && !sourceURL && !file.preview_reason,
    retry,
  );
  const url =
    convert && file.compatibility_url
      ? file.compatibility_url
      : convert && result?.status === "ready"
        ? mediaContextURL(
            file,
            `/api/v1/previews/${encodeURIComponent(result.id)}/content`,
          )
        : sourceURL || mediaURL(file);
  const displayKind =
    convert && file.compatibility_url && kind === "converted"
      ? /\.(docx|xlsx|pptx)$/i.test(file.path)
        ? "pdf"
        : "image"
      : convert && result?.status === "ready"
        ? result.mime_type === "application/pdf"
          ? "pdf"
          : result.mime_type.startsWith("video")
            ? "video"
            : "image"
        : kind;
  useEffect(() => {
    if (
      file.preview_reason ||
      kind !== "text" ||
      file.size_bytes > 2 * 1024 * 1024
    )
      return;
    const controller = new AbortController();
    fetch(sourceURL || mediaURL(file), {
      signal: controller.signal,
      credentials: "same-origin",
    })
      .then(async (r) => {
        if (!r.ok) throw new Error("preview_load_failed");
        const value = await r.text();
        if (!controller.signal.aborted) {
          setText(value);
          setTextLoaded(true);
          onMeta({ text: textStats(value) });
        }
      })
      .catch((e) => {
        if (!controller.signal.aborted) setError(errorText(e));
      });
    return () => controller.abort();
  }, [file.id, file.version_id, sourceURL, kind, file.size_bytes, onMeta]);
  const pending =
    convert &&
    !sourceURL &&
    (!result || result.status === "processing") &&
    !conversionError;
  const unavailable =
    convert &&
    !file.compatibility_url &&
    (sourceURL ||
      result?.status === "failed" ||
      result?.status === "unsupported" ||
      conversionError);
  if (file.preview_reason)
    return (
      <div className="media-placeholder">
        <MediaIcon file={file} />
        <p>{file.preview_reason}</p>
      </div>
    );
  return (
    <section
      className={`media-preview ${compact ? "compact" : ""}`}
      aria-label="文件预览"
    >
      {pending ? (
        <div className="media-placeholder" role="status">
          <LoaderCircle className="spin" />
          <p>正在准备预览…</p>
          <small>原文件已保存，可稍后回来查看。</small>
        </div>
      ) : unavailable ? (
        <div className="media-placeholder">
          <MediaIcon file={file} />
          <p>
            {errorText(
              new Error(
                conversionError || result?.error || "preview_unsupported",
              ),
            )}
          </p>
          {!sourceURL && (
            <Button variant="outline" size="sm" type="submit"

              onClick={() => setRetry((n) => n + 1)}
            >
              <RotateCcw size={14} />
              重试预览
            </Button>
          )}
        </div>
      ) : failed ? (
        <div className="media-placeholder">
          <MediaIcon file={file} />
          <p>浏览器无法直接预览此格式。</p>
          {(!sourceURL || file.compatibility_url) && (
            <Button variant="outline" type="submit"

              onClick={() => {
                setFailed(false);
                setConvert(true);
                setRetry((n) => n + 1);
              }}
            >
              生成兼容预览
            </Button>
          )}
        </div>
      ) : displayKind === "image" ? (
        isVectorImageFile(file) ? (
          // SVG/ICO often ship viewBox-only exports whose natural size is
          // unreliable; keep them on a fit-to-window stage without the
          // raster-only zoom/annotation viewer.
          <div className="image-stage vector-stage">
            <img
              key={url}
              src={url}
              alt={file.path}
              onError={() => setFailed(true)}
            />
          </div>
        ) : compact ? (
          <div className="image-stage">
            <img
              key={url}
              src={url}
              alt={file.path}
              onError={() => setFailed(true)}
            />
          </div>
        ) : (
          <ImageViewer
            key={url}
            file={file}
            src={url}
            meta={meta}
            onMeta={onMeta}
            onFailed={() => setFailed(true)}
          />
        )
      ) : displayKind === "video" ? (
        compact ? (
          <video
            key={url}
            src={url}
            controls
            playsInline
            preload="metadata"
            onError={() => setFailed(true)}
          />
        ) : (
          <VideoViewer
            key={url}
            file={file}
            src={url}
            meta={meta}
            onMeta={onMeta}
            onFailed={() => setFailed(true)}
          />
        )
      ) : displayKind === "audio" ? (
        <AudioStage url={url} onMeta={onMeta} onFailed={() => setFailed(true)} />
      ) : displayKind === "pdf" ? (
        <Suspense fallback={<p role="status">正在加载 PDF 阅读器…</p>}><PdfPreview key={url} name={file.path} url={url}/></Suspense>
      ) : displayKind === "html" ? (
        <HtmlStage key={url} file={file} url={url} />
      ) : displayKind === "spreadsheet" ? (
        <Suspense fallback={<p role="status">正在加载表格阅读器…</p>}><SpreadsheetPreview key={`${url}:${file.version_id}`} url={url} name={file.path} size={file.size_bytes} format={spreadsheetFormat(file)!} /></Suspense>
      ) : displayKind === "text" ? (
        file.size_bytes > 2 * 1024 * 1024 ? (
          <p>文本较大，请下载查看。</p>
        ) : error ? (
          <p role="alert">{error}</p>
        ) : (
          <>
            {textLoaded && (
              <small className="text-stats">
                {meta.text?.lines ?? 0} 行 · {meta.text?.chars ?? 0} 字符
              </small>
            )}
            <pre className="text-preview">
              {textLoaded ? text || "空文本文件 · 0 B" : "正在读取文本…"}
            </pre>
          </>
        )
      ) : (
        <div className="media-placeholder">
          <MediaIcon file={file} />
          <p>此格式暂不支持预览，原文件可下载。</p>
        </div>
      )}
      {convert && (result?.status === "ready" || file.compatibility_url) && (
        <small className="conversion-note">
          转换预览 · 字体、色彩与原软件可能不同，原文件保持不变。
        </small>
      )}
    </section>
  );
}
function HtmlStage({ file, url }: { file: DriveFile; url: string }) {
  const interactiveURL = interactivePreviewURL(url, window.location.origin);
  const [mode, setMode] = useState<"render" | "interactive" | "source">("render");
  const [loaded, setLoaded] = useState<{ url: string; text: string; state: "loading" | "ready" | "error"; error?: string }>();
  const [retry, setRetry] = useState(0);
  const [restart, setRestart] = useState(0);
  const needsText = true;
  const [actualSize, setActualSize] = useState(false);
  const [frameReady,setFrameReady] = useState("");
  const chooseMode = (next: "render" | "interactive" | "source") => { if (mode !== next) setFrameReady(""); setMode(next); };
  const frameKey = `${url}:${mode}:${restart}`;
  const state = loaded?.url === url ? loaded.state : "loading";
  const text = loaded?.url === url ? loaded.text : "";
  useEffect(() => {
    if (file.size_bytes > HTML_PREVIEW_LIMIT) return;
    if (loaded?.url === url && loaded.state === "ready") return;
    const controller = new AbortController();
    setLoaded({ url, text: "", state: "loading" });
    fetch(url, {
      signal: controller.signal,
      credentials: "same-origin",
    })
      .then(async (r) => {
        const value = await readPreviewHTML(r);
        if (!controller.signal.aborted) {
          setLoaded({ url, text: value, state: "ready" });
        }
      })
      .catch((error: Error) => {
        if (!controller.signal.aborted) setLoaded({ url, text: "", state: "error", error: error.message });
      });
    return () => controller.abort();
  }, [needsText, url, file.size_bytes, retry]);
  const stats = state === "ready" ? textStats(text) : undefined;
  return (
    <div className="html-stage">
      <div className="html-stage-views" role="group" aria-label="HTML 预览视图">
        <Button variant={mode === "render" ? "default" : "ghost"} type="button"
          aria-pressed={mode === "render"}
          className={mode === "render" ? "active" : ""}
          onClick={() => chooseMode("render")}
        >
          安全预览
        </Button>
        <Button variant={mode === "interactive" ? "default" : "ghost"} type="button" disabled={!interactiveURL} title={!interactiveURL ? "此来源暂不支持交互预览" : "运行文件内脚本"} aria-pressed={mode === "interactive"} className={mode === "interactive" ? "active" : ""} onClick={() => chooseMode("interactive")}>
          <Play size={16} />交互预览
        </Button>
        <Button variant={mode === "source" ? "default" : "ghost"} type="button"
          aria-pressed={mode === "source"}
          className={mode === "source" ? "active" : ""}
          onClick={() => chooseMode("source")}
        >
          源码
        </Button>
        <Button variant="ghost" aria-pressed={!actualSize} onClick={() => setActualSize(false)}>适应宽度</Button>
        <Button variant="ghost" aria-pressed={actualSize} onClick={() => setActualSize(true)}>实际大小</Button>
        {mode === "interactive" && state === "ready" && <Button variant="ghost" type="submit" aria-label="重新运行预览" onClick={() => setRestart(n => n + 1)}><RotateCw size={16} />重新运行</Button>}
      </div>
      <div className={`html-viewport ${actualSize ? "actual" : ""}`}>
      {file.size_bytes > HTML_PREVIEW_LIMIT ? <p className="reader-message">HTML 超过 2 MiB，请下载查看。</p> : state === "error" ? <div className="reader-message" role="alert"><p>HTML 加载失败，请检查权限或连接。</p><Button onClick={() => setRetry(n => n + 1)}>重新加载</Button></div> : state !== "ready" ? <p role="status">正在加载 HTML…</p> : mode === "render" ? (
        <iframe
          key={url}
          onLoad={() => setFrameReady(frameKey)}
          style={{visibility: frameReady === frameKey ? "visible" : "hidden"}}
          title={file.path + " HTML 预览"}
          src={url}
          sandbox=""
          referrerPolicy="no-referrer"
        />
      ) : mode === "interactive" ? (
        state === "ready" ? <iframe
          key={`${url}:interactive:${restart}`}
          onLoad={() => setFrameReady(frameKey)}
          style={{visibility: frameReady === frameKey ? "visible" : "hidden"}}
          title={file.path + " HTML 交互预览"}
          src={interactiveURL}
          sandbox="allow-scripts"
          referrerPolicy="no-referrer"
          allow="camera 'none'; microphone 'none'; geolocation 'none'; clipboard-read 'none'; clipboard-write 'none'"
        /> : <p role="status">正在加载交互预览…</p>
      ) : (
        <>
          {stats && (
            <small className="text-stats">
              {stats.lines} 行 · {stats.chars} 字符
            </small>
          )}
          <pre className="text-preview">
            {state === "ready" ? text || "空文本文件 · 0 B" : "正在读取文本…"}
          </pre>
        </>
      )}
      </div>
      {mode === "render" && (
        <small className="sandbox-note">安全预览 · 脚本已禁用，CSS 动画可能运行；按钮交互请切换「交互预览」</small>
      )}
      {mode === "interactive" && <small className="sandbox-note">隔离运行文件内脚本 · 外部依赖与 API 请求受限 · 不保存页面内的操作</small>}
    </div>
  );
}
function AudioStage({
  url,
  onMeta,
  onFailed,
}: {
  url: string;
  onMeta: (patch: Partial<ViewerMeta>) => void;
  onFailed: () => void;
}) {
  const audioRef = useRef<HTMLAudioElement>(null);
  const [rate, setRate] = useState(1);
  return (
    <div className="audio-stage">
      <Music size={48} />
      <audio
        ref={audioRef}
        key={url}
        src={url}
        controls
        preload="metadata"
        onError={onFailed}
        onLoadedMetadata={(event) =>
          onMeta({ duration: event.currentTarget.duration })
        }
      />
      <label className="audio-rate">
        播放速度
        <SelectAdapter
          value={rate}
          onChange={(event) => {
            const value = Number(event.target.value);
            setRate(value);
            if (audioRef.current) audioRef.current.playbackRate = value;
          }}
          aria-label="音频播放速度"
        >
          {PLAYBACK_RATES.map((value) => (
            <option key={value} value={value}>
              {value}×
            </option>
          ))}
        </SelectAdapter>
      </label>
    </div>
  );
}
