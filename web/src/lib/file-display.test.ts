import { describe, expect, it } from "vitest";
import { fileExtension, fileVisualMeta, humanFileKind, humanModifiedDate } from "./file-display";

describe("file-display", () => {
  it("maps raw MIME types and extensions to friendly C-end labels without exposing charset strings", () => {
    expect(humanFileKind({ path: "notes/plan.md", mime_type: "text/plain; charset=utf-8" })).toBe("Markdown 笔记");
    expect(humanFileKind({ path: "drag-123.txt", mime_type: "text/plain; charset=utf-8" })).toBe("文本文档");
    expect(humanFileKind({ path: "budget.xlsx", mime_type: "application/octet-stream" })).toBe("表格文档");
    expect(humanFileKind({ path: "scan.pdf", mime_type: "application/pdf" })).toBe("PDF 文档");
    expect(humanFileKind({ path: "clip.mp4", mime_type: "video/mp4" })).toBe("MP4 视频");
    expect(humanFileKind({ path: "archive.zip", mime_type: "application/zip" })).toBe("ZIP 压缩包");
    expect(humanFileKind({ path: "README", mime_type: "text/plain; charset=utf-8" })).toBe("文本文档");
    expect(fileExtension(".gitignore")).toBe("");
    expect(fileVisualMeta({ path: "design.psd" })).toEqual({ label: "PSD 设计稿", badge: "PSD", tone: "image" });
  });

  it("formats timestamps cleanly across recent and historical dates", () => {
    const now = Date.parse("2026-10-20T14:30:00Z");
    expect(humanModifiedDate("2026-10-20T14:29:40Z", now)).toBe("刚刚");
    expect(humanModifiedDate("2026-10-20T14:10:00Z", now)).toBe("20 分钟前");
    expect(humanModifiedDate("2025-05-03T08:00:00Z", now)).toBe("2025/05/03");
    expect(humanModifiedDate("invalid", now)).toBe("");
  });
});
