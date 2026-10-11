import type { DriveFile } from "./api";

export interface Point {
  x: number;
  y: number;
}
export interface Box {
  left: number;
  top: number;
  width: number;
  height: number;
}
export interface Size {
  width: number;
  height: number;
}
export interface ViewerGeometry {
  rect: Box;
  natural: Size;
  zoom: number;
  rotDeg: number;
  flipH: boolean;
  flipV: boolean;
}
function scaleAxes(g: Pick<ViewerGeometry, "zoom" | "flipH" | "flipV">) {
  return {
    sx: g.zoom * (g.flipH ? -1 : 1),
    sy: g.zoom * (g.flipV ? -1 : 1),
  };
}
function center(box: Box): Point {
  return { x: box.left + box.width / 2, y: box.top + box.height / 2 };
}
export function displayToNatural(
  px: number,
  py: number,
  g: ViewerGeometry,
): Point {
  const c = center(g.rect);
  const dx = px - c.x;
  const dy = py - c.y;
  const a = (-g.rotDeg * Math.PI) / 180;
  const cos = Math.cos(a);
  const sin = Math.sin(a);
  const rx = dx * cos - dy * sin;
  const ry = dx * sin + dy * cos;
  const { sx, sy } = scaleAxes(g);
  return {
    x: g.natural.width / 2 + rx / sx,
    y: g.natural.height / 2 + ry / sy,
  };
}
export function naturalToDisplay(p: Point, g: ViewerGeometry): Point {
  const c = center(g.rect);
  const dx = p.x - g.natural.width / 2;
  const dy = p.y - g.natural.height / 2;
  const { sx, sy } = scaleAxes(g);
  const rx = dx * sx;
  const ry = dy * sy;
  const a = (g.rotDeg * Math.PI) / 180;
  const cos = Math.cos(a);
  const sin = Math.sin(a);
  return {
    x: c.x + rx * cos - ry * sin,
    y: c.y + rx * sin + ry * cos,
  };
}
export function fitScale(natural: Size, box: Size): number {
  if (!natural.width || !natural.height || !box.width || !box.height) return 1;
  return Math.min(box.width / natural.width, box.height / natural.height, 1);
}
export const MIN_ZOOM = 0.2;
export const MAX_ZOOM = 8;
export function clampZoom(value: number): number {
  return Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, value));
}
export function panForZoom(args: {
  rect: Box;
  stage: Box;
  natural: Size;
  zoom: number;
  nextZoom: number;
  rotDeg: number;
  flipH: boolean;
  flipV: boolean;
  cursor: Point;
}): Point {
  const from = displayToNatural(args.cursor.x, args.cursor.y, {
    rect: args.rect,
    natural: args.natural,
    zoom: args.zoom,
    rotDeg: args.rotDeg,
    flipH: args.flipH,
    flipV: args.flipV,
  });
  const stageCenter = center(args.stage);
  const local = {
    x: from.x - args.natural.width / 2,
    y: from.y - args.natural.height / 2,
  };
  const rx = local.x * args.nextZoom * (args.flipH ? -1 : 1);
  const ry = local.y * args.nextZoom * (args.flipV ? -1 : 1);
  const a = (args.rotDeg * Math.PI) / 180;
  const cos = Math.cos(a);
  const sin = Math.sin(a);
  const ex = rx * cos - ry * sin;
  const ey = rx * sin + ry * cos;
  return {
    x: args.cursor.x - ex - stageCenter.x,
    y: args.cursor.y - ey - stageCenter.y,
  };
}
export interface ExifData {
  make?: string;
  model?: string;
  lensModel?: string;
  dateTime?: string;
  dateTimeOriginal?: string;
  orientation?: number;
  exposureTime?: number;
  fNumber?: number;
  iso?: number;
  focalLength?: number;
  gps?: { latitude: number; longitude: number };
}
const EXIF_ASCII = 2;
const EXIF_SHORT = 3;
const EXIF_LONG = 4;
const EXIF_RATIONAL = 5;
function exifValueSize(type: number): number {
  if (type === EXIF_ASCII) return 1;
  if (type === EXIF_SHORT) return 2;
  if (type === EXIF_LONG) return 4;
  if (type === EXIF_RATIONAL) return 8;
  return 1;
}
export function parseJpegExif(buffer: ArrayBuffer): ExifData {
  const out: ExifData = {};
  const view = new DataView(buffer);
  try {
    if (buffer.byteLength < 4 || view.getUint16(0) !== 0xffd8) return out;
    let offset = 2;
    let tiff = -1;
    while (tiff < 0 && offset + 4 <= buffer.byteLength) {
      if (view.getUint8(offset) !== 0xff) break;
      const marker = view.getUint8(offset + 1);
      const length = view.getUint16(offset + 2);
      if (marker === 0xe1 && length > 8) {
        const head = offset + 4;
        if (
          view.getUint32(head) === 0x45786966 &&
          view.getUint16(head + 4) === 0x0000
        )
          tiff = head + 6;
      }
      offset += 2 + length;
    }
    if (tiff < 0 || tiff + 8 > buffer.byteLength) return out;
    const order = view.getUint16(tiff);
    const little = order === 0x4949;
    if (!little && order !== 0x4d4d) return out;
    const u16 = (at: number) => view.getUint16(at, little);
    const u32 = (at: number) => view.getUint32(at, little);
    if (u16(tiff + 2) !== 0x2a) return out;
    const readASCII = (at: number, count: number) => {
      let text = "";
      for (let i = 0; i < count - 1 && at + i < buffer.byteLength; i++)
        text += String.fromCharCode(view.getUint8(at + i));
      return text.replace(/\0+$/, "").trim();
    };
    const readRationals = (at: number, count: number) => {
      const rows: number[] = [];
      for (let i = 0; i < count; i++)
        rows.push(u32(at + i * 8) / (u32(at + i * 8 + 4) || 1));
      return rows;
    };
    type ExifValue = number | string | number[];
    const entryValue = (entry: number): ExifValue => {
      const type = u16(entry + 2);
      const count = u32(entry + 4);
      const size = exifValueSize(type) * count;
      const at = size <= 4 ? entry + 8 : tiff + u32(entry + 8);
      if (type === EXIF_ASCII) return readASCII(at, count);
      if (type === EXIF_SHORT) return u16(at);
      if (type === EXIF_LONG) return u32(at);
      if (type === EXIF_RATIONAL) return readRationals(at, count);
      return u32(at);
    };
    const walk = (
      ifd: number,
      visit: (tag: number, value: ExifValue) => void,
    ) => {
      if (ifd + 2 > buffer.byteLength) return;
      const count = u16(ifd);
      for (let i = 0; i < count; i++) {
        const entry = ifd + 2 + i * 12;
        if (entry + 12 > buffer.byteLength) return;
        visit(u16(entry), entryValue(entry));
      }
    };
    const rationals = (value: ExifValue) =>
      Array.isArray(value) ? value : [];
    const text = (value: ExifValue) =>
      typeof value === "string" ? value : "";
    const number = (value: ExifValue) =>
      typeof value === "number" ? value : undefined;
    const gpsCoordinate = (ref: ExifValue, rows: number[]) => {
      const degrees =
        (rows[0] || 0) + (rows[1] || 0) / 60 + (rows[2] || 0) / 3600;
      return text(ref) === "S" || text(ref) === "W" ? -degrees : degrees;
    };
    let exifIfd = -1;
    let gpsIfd = -1;
    walk(tiff + u32(tiff + 4), (tag, value) => {
      if (tag === 0x010f) out.make = text(value);
      else if (tag === 0x0110) out.model = text(value);
      else if (tag === 0x0112) out.orientation = number(value);
      else if (tag === 0x0132) out.dateTime = text(value);
      else if (tag === 0x8769) exifIfd = number(value) ?? -1;
      else if (tag === 0x8825) gpsIfd = number(value) ?? -1;
    });
    if (exifIfd > 0)
      walk(tiff + exifIfd, (tag, value) => {
        if (tag === 0x829a) {
          const rows = rationals(value);
          if (rows[0]) out.exposureTime = rows[0];
        } else if (tag === 0x829d) {
          const rows = rationals(value);
          if (rows[0]) out.fNumber = rows[0];
        } else if (tag === 0x8827) out.iso = number(value);
        else if (tag === 0x9003) out.dateTimeOriginal = text(value);
        else if (tag === 0x920a) {
          const rows = rationals(value);
          if (rows[0]) out.focalLength = rows[0];
        } else if (tag === 0xa434) out.lensModel = text(value);
      });
    if (gpsIfd > 0) {
      let latitudeRef = "";
      let longitudeRef = "";
      let latitude: number[] = [];
      let longitude: number[] = [];
      walk(tiff + gpsIfd, (tag, value) => {
        if (tag === 0x0001) latitudeRef = text(value);
        else if (tag === 0x0002) latitude = rationals(value);
        else if (tag === 0x0003) longitudeRef = text(value);
        else if (tag === 0x0004) longitude = rationals(value);
      });
      if (latitude.length || longitude.length)
        out.gps = {
          latitude: gpsCoordinate(latitudeRef, latitude),
          longitude: gpsCoordinate(longitudeRef, longitude),
        };
    }
  } catch {
    // 截断或损坏的 EXIF：保留已解析字段即可。
  }
  return out;
}
export interface PngHeader {
  width: number;
  height: number;
  bitDepth: number;
  colorType: number;
}
const PNG_COLORS: Record<number, string> = {
  0: "灰度",
  2: "RGB",
  3: "索引",
  4: "灰度 + 透明",
  6: "RGBA",
};
export function pngColorName(colorType: number): string {
  return PNG_COLORS[colorType] || `类型 ${colorType}`;
}
export function parsePngHeader(buffer: ArrayBuffer): PngHeader | null {
  const view = new DataView(buffer);
  if (buffer.byteLength < 33) return null;
  const signature = [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a];
  for (let i = 0; i < signature.length; i++)
    if (view.getUint8(i) !== signature[i]) return null;
  if (view.getUint32(12) !== 0x49484452) return null;
  return {
    width: view.getUint32(16),
    height: view.getUint32(20),
    bitDepth: view.getUint8(24),
    colorType: view.getUint8(25),
  };
}
export type AnnotationTool = "pen" | "rect" | "arrow" | "ellipse" | "text";
export interface ViewerStroke {
  tool: AnnotationTool;
  color: string;
  width: number;
  points: Point[];
  text?: string;
  fontSize?: number;
}
export function drawStrokes(
  ctx: CanvasRenderingContext2D,
  strokes: ViewerStroke[],
): void {
  ctx.lineCap = "round";
  ctx.lineJoin = "round";
  for (const stroke of strokes) {
    if (stroke.tool === "text" && stroke.points.length && stroke.text) {
      ctx.save();
      ctx.fillStyle = stroke.color;
      ctx.font = `${stroke.fontSize || 24}px system-ui, sans-serif`;
      ctx.textBaseline = "top";
      ctx.fillText(stroke.text, stroke.points[0].x, stroke.points[0].y);
      ctx.restore();
      continue;
    }
    if (stroke.points.length < 2) continue;
    const from = stroke.points[0];
    const to = stroke.points[stroke.points.length - 1];
    ctx.strokeStyle = stroke.color;
    ctx.fillStyle = stroke.color;
    ctx.lineWidth = stroke.width;
    if (stroke.tool === "pen") {
      ctx.beginPath();
      ctx.moveTo(from.x, from.y);
      for (const point of stroke.points.slice(1)) ctx.lineTo(point.x, point.y);
      ctx.stroke();
    } else if (stroke.tool === "rect") {
      ctx.beginPath();
      ctx.rect(
        Math.min(from.x, to.x),
        Math.min(from.y, to.y),
        Math.abs(to.x - from.x),
        Math.abs(to.y - from.y),
      );
      ctx.stroke();
    } else if (stroke.tool === "ellipse") {
      ctx.beginPath();
      ctx.ellipse((from.x + to.x) / 2, (from.y + to.y) / 2,
        Math.abs(to.x - from.x) / 2, Math.abs(to.y - from.y) / 2, 0, 0, Math.PI * 2);
      ctx.stroke();
    } else {
      const angle = Math.atan2(to.y - from.y, to.x - from.x);
      const length = Math.hypot(to.x - from.x, to.y - from.y);
      const head = Math.max(stroke.width * 3.5, 12);
      const bend = Math.min(head, length / 2);
      ctx.beginPath();
      ctx.moveTo(from.x, from.y);
      ctx.lineTo(to.x - bend * Math.cos(angle), to.y - bend * Math.sin(angle));
      ctx.stroke();
      ctx.beginPath();
      ctx.moveTo(to.x, to.y);
      ctx.lineTo(
        to.x - head * Math.cos(angle - Math.PI / 7),
        to.y - head * Math.sin(angle - Math.PI / 7),
      );
      ctx.lineTo(
        to.x - head * Math.cos(angle + Math.PI / 7),
        to.y - head * Math.sin(angle + Math.PI / 7),
      );
      ctx.closePath();
      ctx.fill();
    }
  }
}
export function composeExport(
  image: CanvasImageSource,
  imageWidth: number,
  imageHeight: number,
  annotations: HTMLCanvasElement,
  transform: { rotDeg: number; flipH: boolean; flipV: boolean },
): HTMLCanvasElement | null {
  const swap = Math.abs(transform.rotDeg % 180) === 90;
  const canvas = document.createElement("canvas");
  canvas.width = swap ? imageHeight : imageWidth;
  canvas.height = swap ? imageWidth : imageHeight;
  const ctx = canvas.getContext("2d");
  if (!ctx || !canvas.width || !canvas.height) return null;
  ctx.translate(canvas.width / 2, canvas.height / 2);
  ctx.rotate((transform.rotDeg * Math.PI) / 180);
  ctx.scale(transform.flipH ? -1 : 1, transform.flipV ? -1 : 1);
  ctx.drawImage(
    image,
    -imageWidth / 2,
    -imageHeight / 2,
    imageWidth,
    imageHeight,
  );
  ctx.drawImage(
    annotations,
    -imageWidth / 2,
    -imageHeight / 2,
    imageWidth,
    imageHeight,
  );
  return canvas;
}
export function textStats(text: string): { lines: number; chars: number } {
  return { lines: text ? text.split("\n").length : 0, chars: text.length };
}
export function formatExposure(seconds: number): string {
  if (seconds >= 1) return `${Math.round(seconds * 100) / 100} s`;
  const stop = Math.round(1 / seconds);
  return stop > 0 ? `1/${stop} s` : `${seconds} s`;
}
export function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return "";
  const total = Math.round(seconds);
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const secs = total % 60;
  const pad = (value: number) => String(value).padStart(2, "0");
  return hours
    ? `${hours}:${pad(minutes)}:${pad(secs)}`
    : `${minutes}:${pad(secs)}`;
}
export function formatGps(
  coordinate: number,
  positive: string,
  negative: string,
): string {
  const direction = coordinate >= 0 ? positive : negative;
  const value = Math.abs(coordinate);
  const degrees = Math.floor(value);
  const minutes = Math.floor((value - degrees) * 60);
  const seconds =
    Math.round(((value - degrees) * 60 - minutes) * 60 * 100) / 100;
  return `${degrees}° ${minutes}′ ${seconds}″ ${direction}`;
}
const ORIENTATION_LABELS: Record<number, string> = {
  1: "正常",
  2: "水平镜像",
  3: "旋转 180°",
  4: "垂直镜像",
  5: "镜像 + 旋转",
  6: "顺时针 90°",
  7: "镜像 + 旋转",
  8: "逆时针 90°",
};
export function orientationLabel(orientation?: number): string | undefined {
  return orientation ? ORIENTATION_LABELS[orientation] : undefined;
}
export function isJpegFile(
  file: Pick<DriveFile, "mime_type" | "path">,
): boolean {
  return (
    file.mime_type === "image/jpeg" ||
    /\.jpe?g$/i.test(file.path.split("/").pop() || "")
  );
}
