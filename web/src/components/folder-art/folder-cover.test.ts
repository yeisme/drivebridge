import { expect, it } from "vitest";
import type { DriveFile } from "../../api";
import { FOLDER_TONES, folderMeta, folderTone, summarizeFolder, updatedLabel, type FolderEntry } from "./folder-cover";

const file = (path: string, mime_type: string, updated_at: string) =>
  ({ kind: "file", path, file: { path, mime_type, updated_at } as DriveFile }) satisfies FolderEntry;
const dir = (path: string) => ({ kind: "directory", path }) satisfies FolderEntry;

it("keeps one tone per leaf name, independent of the parent path", () => {
  expect(folderTone("设计素材")).toBe(folderTone("2026/项目/设计素材"));
  expect(folderTone("a/设计素材/")).toBe(folderTone("设计素材"));
  for (const name of ["产品资料", "会议录音", "归档", "x", ""]) {
    expect(folderTone(name)).toBeGreaterThanOrEqual(1);
    expect(folderTone(name)).toBeLessThanOrEqual(FOLDER_TONES);
  }
  expect(new Set(["产品资料", "会议录音", "归档", "视频剪辑", "设计素材", "财务报表"].map(folderTone)).size).toBeGreaterThan(2);
});

it("peeks at the most recent files first and fills with subfolders", () => {
  const summary = summarizeFolder([
    dir("a/old"),
    file("a/notes.txt", "text/plain", "2026-10-01T00:00:00Z"),
    file("a/cut.mp4", "video/mp4", "2026-10-03T00:00:00Z"),
    file("a/plan.pdf", "application/pdf", "2026-10-02T00:00:00Z"),
    file("a/cover.png", "image/png", "2026-09-01T00:00:00Z"),
  ]);
  expect(summary).toMatchObject({ files: 4, folders: 1, total: 5, complete: true, updatedAt: "2026-10-03T00:00:00Z" });
  expect(summary.sheets).toEqual(["video", "pdf", "doc"]);
  expect(summarizeFolder([dir("a/x"), file("a/y.csv", "text/csv", "2026-10-01T00:00:00Z")]).sheets).toEqual(["sheet", "folder"]);
  expect(summarizeFolder([dir("a/x"), dir("a/y")]).sheets).toEqual(["folder", "folder"]);
  expect(summarizeFolder([]).sheets).toEqual([]);
});

it("describes exact counts and admits partial pages", () => {
  expect(folderMeta(summarizeFolder([]))).toBe("空文件夹");
  expect(folderMeta(summarizeFolder([file("a/b.txt", "text/plain", "2026-10-01T00:00:00Z")]))).toBe("1 个文件");
  expect(folderMeta(summarizeFolder([dir("a/x"), dir("a/y"), file("a/b.txt", "text/plain", "2026-10-01T00:00:00Z")]))).toBe("1 个文件 · 2 个文件夹");
  expect(folderMeta(summarizeFolder([dir("a/x")], 480, false))).toBe("480 项以上");
});

it("labels recency without inventing future edits", () => {
  const now = Date.parse("2026-10-24T12:00:00Z");
  expect(updatedLabel(undefined, now)).toBeUndefined();
  expect(updatedLabel("not a date", now)).toBeUndefined();
  expect(updatedLabel("2026-10-24T11:59:30Z", now)).toBe("刚刚更新");
  expect(updatedLabel("2026-10-24T13:00:00Z", now)).toBe("刚刚更新");
  expect(updatedLabel("2026-10-24T11:15:00Z", now)).toBe("45分钟前更新");
  expect(updatedLabel("2026-10-24T09:00:00Z", now)).toBe("3小时前更新");
  expect(updatedLabel("2026-10-23T10:00:00Z", now)).toBe("昨天更新");
  expect(updatedLabel("2026-08-01T00:00:00Z", now)).toBe("3个月前更新");
});
