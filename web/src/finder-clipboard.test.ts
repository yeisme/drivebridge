import { describe, expect, it } from "vitest";
import {
  clipboardFromFiles,
  copyDisabledReason,
  isCutItem,
  joinPath,
  listedNames,
  needsPasteConfirm,
  pasteDisabledReason,
} from "./finder-clipboard";
import type { DriveFile } from "./api";

const file = (path: string, id = path): DriveFile => ({
  id,
  space_id: "s",
  path,
  version_id: "v1",
  size_bytes: 1,
  mime_type: "text/plain",
  trashed: false,
  updated_at: "2026-09-19T00:00:00Z",
});

describe("file clipboard", () => {
  it("builds copy items and destination paths", () => {
    const board = clipboardFromFiles("copy", "s", [file("docs/a.txt", "1")]);
    expect(board?.items[0].name).toBe("a.txt");
    expect(joinPath("backup", board!.items[0].name)).toBe("backup/a.txt");
  });
  it("includes folders in the clipboard", () => {
    const board = clipboardFromFiles("copy", "s", [], ["项目/资料"]);
    expect(board?.items[0]).toMatchObject({
      kind: "directory",
      path: "项目/资料",
      name: "资料",
    });
  });
  it("blocks cross-space cut paste and missing copy", () => {
    const cut = clipboardFromFiles("cut", "other", [file("a.txt", "1")]);
    expect(pasteDisabledReason(cut, "s", false, true)).toContain("跨网盘");
    expect(copyDisabledReason(false, false)).toContain("不支持复制");
    expect(isCutItem(cut, "1")).toBe(true);
    expect(copyDisabledReason(false, true, true, false)).toContain("文件夹");
  });
  it("confirms large pastes and lists names", () => {
    expect(needsPasteConfirm(10)).toBe(true);
    expect(listedNames([file("a.txt"), file("b.txt")])).toBe("a.txt、b.txt");
  });
});
