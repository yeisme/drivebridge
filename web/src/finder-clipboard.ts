import type { DriveFile } from "./api";

export type ClipboardMode = "copy" | "cut";

export interface ClipboardItem {
  kind: "file" | "directory";
  id: string;
  version_id: string;
  path: string;
  name: string;
}

export interface FileClipboard {
  mode: ClipboardMode;
  space_id: string;
  items: ClipboardItem[];
}

export function fileName(path: string) {
  return path.split("/").pop() || path;
}

export function joinPath(dir: string, name: string) {
  return [dir, name].filter(Boolean).join("/");
}

export function clipboardFromFiles(
  mode: ClipboardMode,
  space_id: string,
  files: DriveFile[],
  folders: string[] = [],
): FileClipboard | undefined {
  const items: ClipboardItem[] = [
    ...folders.map((path) => ({
      kind: "directory" as const,
      id: "dir:" + path,
      version_id: "",
      path,
      name: fileName(path),
    })),
    ...files.map((file) => ({
      kind: "file" as const,
      id: file.id,
      version_id: file.version_id,
      path: file.path,
      name: fileName(file.path),
    })),
  ];
  if (!items.length) return undefined;
  return { mode, space_id, items };
}

export function clipboardNotice(board: FileClipboard) {
  const verb = board.mode === "cut" ? "已剪切" : "已复制";
  return `${verb} ${board.items.length} 项，可粘贴到当前目录`;
}

export function copyDisabledReason(
  readOnly: boolean,
  canCopy: boolean,
  hasFolders = false,
  canCopyFolders = true,
) {
  if (readOnly) return "当前网盘仅供浏览";
  if (hasFolders && !canCopyFolders) return "当前连接不支持复制文件夹";
  if (!canCopy && !hasFolders) return "当前连接不支持复制文件";
  return "";
}

export function cutDisabledReason(readOnly: boolean) {
  return readOnly ? "当前网盘仅供浏览" : "";
}

export function pasteDisabledReason(
  board: FileClipboard | undefined,
  space: string,
  readOnly: boolean,
  canCopy: boolean,
  canCopyFolders = true,
  canMoveFolders = true,
) {
  if (!board?.items.length) return "剪贴板为空";
  if (readOnly) return "当前网盘仅供浏览";
  if (board.mode === "cut" && board.space_id !== space)
    return "不能跨网盘剪切，请改为复制后再粘贴";
  const hasFolders = board.items.some((item) => item.kind === "directory");
  const hasFiles = board.items.some((item) => item.kind === "file");
  if (board.mode === "copy" && hasFiles && !canCopy)
    return "当前连接不支持复制文件";
  if (board.mode === "copy" && hasFolders && !canCopyFolders)
    return "当前连接不支持复制文件夹";
  if (board.mode === "cut" && hasFolders && !canMoveFolders)
    return "当前连接不支持移动文件夹";
  return "";
}

export function needsPasteConfirm(count: number) {
  return count >= 10;
}

export function listedNames(files: { path: string }[], limit = 8) {
  const names = files.map((file) => fileName(file.path));
  if (names.length <= limit) return names.join("、");
  return `${names.slice(0, limit).join("、")} 等 ${names.length} 项`;
}

export function isCutItem(
  board: FileClipboard | undefined,
  id: string,
  path?: string,
) {
  if (board?.mode !== "cut") return false;
  return board.items.some(
    (item) => item.id === id || (path && item.path === path),
  );
}
