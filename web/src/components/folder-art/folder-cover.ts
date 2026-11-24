import type { DriveFile } from "../../api";
import { mediaKind } from "../../Media";

/** Folder identity colours are categorical, not semantic: the same leaf name keeps the same tone. */
export const FOLDER_TONES = 5;

export type FolderSheetKind = "image" | "video" | "audio" | "pdf" | "sheet" | "doc" | "folder";

export interface FolderEntry {
  kind: "file" | "directory";
  path: string;
  file?: DriveFile;
}

/** What one directory page says about a folder; counts are exact only when `complete`. */
export interface FolderSummary {
  files: number;
  folders: number;
  total: number;
  complete: boolean;
  /** Up to three peek sheets, most recently changed files first, then subfolders. */
  sheets: FolderSheetKind[];
  /** Latest modification among the files on this page; nested folders are not included. */
  updatedAt?: string;
}

export function folderTone(path: string): number {
  const name = path.split("/").filter(Boolean).pop() || "";
  let hash = 0;
  for (const char of name) hash = (hash * 31 + char.codePointAt(0)!) >>> 0;
  return (hash % FOLDER_TONES) + 1;
}

export function sheetKind(file: Pick<DriveFile, "mime_type" | "path">): FolderSheetKind {
  switch (mediaKind(file)) {
    case "image":
      return "image";
    case "video":
      return "video";
    case "audio":
      return "audio";
    case "pdf":
      return "pdf";
    case "spreadsheet":
      return "sheet";
    default:
      return "doc";
  }
}

export function summarizeFolder(
  entries: FolderEntry[],
  total = entries.length,
  complete = true,
): FolderSummary {
  const files = entries.flatMap((entry) => (entry.kind === "file" && entry.file ? [entry.file] : []));
  const folders = entries.filter((entry) => entry.kind === "directory").length;
  const recent = [...files].sort((a, b) => Date.parse(b.updated_at) - Date.parse(a.updated_at));
  const sheets = recent.slice(0, 3).map(sheetKind);
  while (sheets.length < 3 && sheets.length < files.length + folders) sheets.push("folder");
  return {
    files: files.length,
    folders,
    total,
    complete,
    sheets,
    updatedAt: recent[0]?.updated_at,
  };
}

export function folderMeta(summary: FolderSummary): string {
  if (!summary.complete) return `${summary.total} 项以上`;
  if (!summary.total) return "空文件夹";
  const parts = [];
  if (summary.files) parts.push(`${summary.files} 个文件`);
  if (summary.folders) parts.push(`${summary.folders} 个文件夹`);
  return parts.join(" · ");
}

const relative = new Intl.RelativeTimeFormat("zh-CN", { numeric: "auto" });
const units: [Intl.RelativeTimeFormatUnit, number][] = [
  ["second", 60],
  ["minute", 60],
  ["hour", 24],
  ["day", 7],
  ["week", 4.35],
  ["month", 12],
  ["year", Infinity],
];

export function updatedLabel(iso: string | undefined, now = Date.now()): string | undefined {
  const time = iso ? Date.parse(iso) : NaN;
  if (!Number.isFinite(time)) return undefined;
  let value = (time - now) / 1000;
  // Future timestamps are clock skew, not scheduled edits.
  if (value > -60) return "刚刚更新";
  for (const [unit, size] of units) {
    if (Math.abs(value) < size) return `${relative.format(Math.round(value), unit)}更新`;
    value /= size;
  }
  return undefined;
}
