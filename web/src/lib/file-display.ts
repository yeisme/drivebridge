export type FileVisualTone =
  | "doc"
  | "sheet"
  | "pdf"
  | "image"
  | "video"
  | "audio"
  | "archive"
  | "code";

const EXT_LABELS: Record<string, { label: string; badge: string; tone: FileVisualTone }> = {
  md: { label: "Markdown 笔记", badge: "MD", tone: "doc" },
  markdown: { label: "Markdown 笔记", badge: "MD", tone: "doc" },
  txt: { label: "文本文档", badge: "TXT", tone: "doc" },
  rtf: { label: "富文本文档", badge: "RTF", tone: "doc" },
  doc: { label: "Word 文档", badge: "DOC", tone: "doc" },
  docx: { label: "Word 文档", badge: "DOCX", tone: "doc" },
  ppt: { label: "演示文稿", badge: "PPT", tone: "archive" },
  pptx: { label: "演示文稿", badge: "PPTX", tone: "archive" },
  pdf: { label: "PDF 文档", badge: "PDF", tone: "pdf" },
  xls: { label: "表格文档", badge: "XLS", tone: "sheet" },
  xlsx: { label: "表格文档", badge: "XLSX", tone: "sheet" },
  csv: { label: "CSV 表格", badge: "CSV", tone: "sheet" },
  tsv: { label: "数据表格", badge: "TSV", tone: "sheet" },
  png: { label: "PNG 图片", badge: "PNG", tone: "image" },
  jpg: { label: "JPG 图片", badge: "JPG", tone: "image" },
  jpeg: { label: "JPG 图片", badge: "JPG", tone: "image" },
  gif: { label: "GIF 动图", badge: "GIF", tone: "image" },
  webp: { label: "WebP 图片", badge: "WEBP", tone: "image" },
  avif: { label: "AVIF 图片", badge: "AVIF", tone: "image" },
  svg: { label: "SVG 矢量图", badge: "SVG", tone: "image" },
  bmp: { label: "BMP 图片", badge: "BMP", tone: "image" },
  ico: { label: "图标文件", badge: "ICO", tone: "image" },
  psd: { label: "PSD 设计稿", badge: "PSD", tone: "image" },
  psb: { label: "PSB 设计稿", badge: "PSB", tone: "image" },
  tif: { label: "TIFF 图片", badge: "TIFF", tone: "image" },
  tiff: { label: "TIFF 图片", badge: "TIFF", tone: "image" },
  jxr: { label: "HDR 截图 (JXR)", badge: "JXR", tone: "image" },
  wdp: { label: "HD Photo (WDP)", badge: "WDP", tone: "image" },
  hdp: { label: "HD Photo (HDP)", badge: "HDP", tone: "image" },
  heic: { label: "HEIC 高效图片", badge: "HEIC", tone: "image" },
  heif: { label: "HEIF 高效图片", badge: "HEIF", tone: "image" },
  hif: { label: "HIF 高效图片", badge: "HIF", tone: "image" },
  jxl: { label: "JPEG XL 图片", badge: "JXL", tone: "image" },
  jp2: { label: "JPEG 2000 图片", badge: "JP2", tone: "image" },
  j2k: { label: "JPEG 2000 图片", badge: "J2K", tone: "image" },
  exr: { label: "OpenEXR 渲染图", badge: "EXR", tone: "image" },
  hdr: { label: "HDR 光照贴图", badge: "HDR", tone: "image" },
  tga: { label: "TGA 纹理图片", badge: "TGA", tone: "image" },
  dds: { label: "DDS 纹理贴图", badge: "DDS", tone: "image" },
  pcx: { label: "PCX 图像", badge: "PCX", tone: "image" },
  ppm: { label: "PPM 图像", badge: "PPM", tone: "image" },
  pgm: { label: "PGM 灰度图", badge: "PGM", tone: "image" },
  pbm: { label: "PBM 位图", badge: "PBM", tone: "image" },
  pnm: { label: "PNM 图像", badge: "PNM", tone: "image" },
  dng: { label: "RAW 照片", badge: "RAW", tone: "image" },
  cr2: { label: "RAW 照片", badge: "RAW", tone: "image" },
  cr3: { label: "RAW 照片", badge: "RAW", tone: "image" },
  nef: { label: "RAW 照片", badge: "RAW", tone: "image" },
  arw: { label: "RAW 照片", badge: "RAW", tone: "image" },
  raf: { label: "RAW 照片", badge: "RAW", tone: "image" },
  orf: { label: "RAW 照片", badge: "RAW", tone: "image" },
  rw2: { label: "RAW 照片", badge: "RAW", tone: "image" },
  pef: { label: "RAW 照片", badge: "RAW", tone: "image" },
  srw: { label: "RAW 照片", badge: "RAW", tone: "image" },
  rwl: { label: "RAW 照片", badge: "RAW", tone: "image" },
  "3fr": { label: "RAW 照片", badge: "RAW", tone: "image" },
  iiq: { label: "RAW 照片", badge: "RAW", tone: "image" },
  mp4: { label: "MP4 视频", badge: "MP4", tone: "video" },
  mov: { label: "MOV 视频", badge: "MOV", tone: "video" },
  webm: { label: "WebM 视频", badge: "WEBM", tone: "video" },
  mkv: { label: "MKV 视频", badge: "MKV", tone: "video" },
  m4v: { label: "视频文件", badge: "M4V", tone: "video" },
  avi: { label: "AVI 视频", badge: "AVI", tone: "video" },
  mp3: { label: "MP3 音频", badge: "MP3", tone: "audio" },
  wav: { label: "WAV 音频", badge: "WAV", tone: "audio" },
  flac: { label: "无损音频", badge: "FLAC", tone: "audio" },
  m4a: { label: "音频文件", badge: "M4A", tone: "audio" },
  aac: { label: "AAC 音频", badge: "AAC", tone: "audio" },
  ogg: { label: "OGG 音频", badge: "OGG", tone: "audio" },
  opus: { label: "语音音频", badge: "OPUS", tone: "audio" },
  zip: { label: "ZIP 压缩包", badge: "ZIP", tone: "archive" },
  tar: { label: "TAR 归档包", badge: "TAR", tone: "archive" },
  gz: { label: "GZ 压缩包", badge: "GZ", tone: "archive" },
  tgz: { label: "压缩归档包", badge: "TGZ", tone: "archive" },
  "7z": { label: "7Z 压缩包", badge: "7Z", tone: "archive" },
  rar: { label: "RAR 压缩包", badge: "RAR", tone: "archive" },
  html: { label: "网页文档", badge: "HTML", tone: "code" },
  htm: { label: "网页文档", badge: "HTML", tone: "code" },
  json: { label: "JSON 配置", badge: "JSON", tone: "code" },
  yaml: { label: "YAML 配置", badge: "YAML", tone: "code" },
  yml: { label: "YAML 配置", badge: "YML", tone: "code" },
  toml: { label: "TOML 配置", badge: "TOML", tone: "code" },
  xml: { label: "XML 文档", badge: "XML", tone: "code" },
  sql: { label: "SQL 脚本", badge: "SQL", tone: "code" },
  js: { label: "脚本文件", badge: "JS", tone: "code" },
  mjs: { label: "脚本文件", badge: "MJS", tone: "code" },
  ts: { label: "TypeScript", badge: "TS", tone: "code" },
  tsx: { label: "TypeScript", badge: "TSX", tone: "code" },
  jsx: { label: "React 组件", badge: "JSX", tone: "code" },
  go: { label: "Go 源码", badge: "GO", tone: "code" },
  py: { label: "Python 脚本", badge: "PY", tone: "code" },
  rs: { label: "Rust 源码", badge: "RS", tone: "code" },
  sh: { label: "Shell 脚本", badge: "SH", tone: "code" },
  log: { label: "日志文件", badge: "LOG", tone: "doc" },
};

export function fileExtension(path: string): string {
  const base = path.split("/").pop() || "";
  const dot = base.lastIndexOf(".");
  if (dot <= 0 || dot === base.length - 1) return "";
  return base.slice(dot + 1).toLowerCase();
}

export function fileVisualMeta(file: { path: string; mime_type?: string }): {
  label: string;
  badge: string;
  tone: FileVisualTone;
} {
  const ext = fileExtension(file.path);
  if (ext && EXT_LABELS[ext]) return EXT_LABELS[ext];
  const mime = (file.mime_type || "").split(";")[0].trim().toLowerCase();
  if (mime === "application/pdf") return { label: "PDF 文档", badge: "PDF", tone: "pdf" };
  if (mime.startsWith("image/")) return { label: "图片文件", badge: "IMG", tone: "image" };
  if (mime.startsWith("video/")) return { label: "视频文件", badge: "VID", tone: "video" };
  if (mime.startsWith("audio/")) return { label: "音频文件", badge: "AUD", tone: "audio" };
  if (mime.includes("spreadsheet") || mime.includes("excel") || mime === "text/csv")
    return { label: "表格文档", badge: "SHEET", tone: "sheet" };
  if (mime === "text/markdown") return { label: "Markdown 笔记", badge: "MD", tone: "doc" };
  if (mime === "text/html") return { label: "网页文档", badge: "HTML", tone: "code" };
  if (mime.startsWith("text/")) return { label: "文本文档", badge: ext ? ext.slice(0, 4).toUpperCase() : "TXT", tone: "doc" };
  if (mime.includes("zip") || mime.includes("compressed") || mime.includes("tar"))
    return { label: "压缩归档", badge: "ZIP", tone: "archive" };
  if (ext && /^[a-z0-9]{1,6}$/.test(ext))
    return { label: `${ext.toUpperCase()} 文件`, badge: ext.slice(0, 4).toUpperCase(), tone: "doc" };
  return { label: "云盘文件", badge: "FILE", tone: "doc" };
}

export function humanFileKind(file: { path: string; mime_type?: string }): string {
  return fileVisualMeta(file).label;
}

export function humanModifiedDate(iso?: string, nowMs = Date.now()): string {
  if (!iso) return "";
  const parsed = Date.parse(iso);
  if (!Number.isFinite(parsed)) return "";
  const diff = nowMs - parsed;
  if (diff >= 0 && diff < 60_000) return "刚刚";
  if (diff >= 60_000 && diff < 3_600_000) return `${Math.floor(diff / 60_000)} 分钟前`;
  const target = new Date(parsed);
  const current = new Date(nowMs);
  if (
    target.getFullYear() === current.getFullYear() &&
    target.getMonth() === current.getMonth() &&
    target.getDate() === current.getDate()
  ) {
    return `今天 ${String(target.getHours()).padStart(2, "0")}:${String(target.getMinutes()).padStart(2, "0")}`;
  }
  if (target.getFullYear() === current.getFullYear()) {
    return `${target.getMonth() + 1}月${target.getDate()}日`;
  }
  return `${target.getFullYear()}/${String(target.getMonth() + 1).padStart(2, "0")}/${String(target.getDate()).padStart(2, "0")}`;
}
