import { describe, expect, it } from "vitest";
import { mediaKind, isVectorImageFile } from "./Media";

describe("mediaKind", () => {
  it("distinguishes audio pdf text and other files", () => {
    expect(mediaKind({ mime_type: "audio/mpeg", path: "a.mp3" })).toBe("audio");
    expect(mediaKind({ mime_type: "application/pdf", path: "a.pdf" })).toBe(
      "pdf",
    );
    expect(mediaKind({ mime_type: "text/plain", path: "a.txt" })).toBe("text");
    expect(
      mediaKind({ mime_type: "application/zip", path: "a.zip" }),
    ).toBe("unsupported");
  });
  it("routes spreadsheet names and MIME types to the table reader", () => {
    for (const file of [
      { path: "report.XLSX", mime_type: "application/zip" },
      { path: "report.csv", mime_type: "text/plain" },
      { path: "report.tsv", mime_type: "application/octet-stream" },
      { path: "report", mime_type: "text/csv; charset=utf-8" },
      { path: "report", mime_type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" },
    ]) expect(mediaKind(file)).toBe("spreadsheet");
    expect(mediaKind({ path: "report.docx", mime_type: "application/zip" })).toBe("converted");
  });
  it("recognises html pages over text fallback", () => {
    expect(
      mediaKind({ mime_type: "text/html; charset=utf-8", path: "报告.html" }),
    ).toBe("html");
    expect(
      mediaKind({ mime_type: "application/octet-stream", path: "page.htm" }),
    ).toBe("html");
    expect(
      mediaKind({ mime_type: "application/xhtml+xml", path: "doc" }),
    ).toBe("html");
    expect(mediaKind({ mime_type: "text/plain", path: "a.txt" })).toBe("text");
  });
  it("recognises svg and ico images", () => {
    expect(mediaKind({ mime_type: "image/svg+xml", path: "logo.svg" })).toBe(
      "image",
    );
    expect(
      mediaKind({ mime_type: "application/octet-stream", path: "favicon.ico" }),
    ).toBe("image");
  });
  it("marks svg and ico as vector stage files", () => {
    expect(isVectorImageFile({ mime_type: "text/xml", path: "矢量.svg" })).toBe(
      true,
    );
    expect(
      isVectorImageFile({ mime_type: "image/svg+xml", path: "logo" }),
    ).toBe(true);
    expect(isVectorImageFile({ mime_type: "image/png", path: "a.png" })).toBe(
      false,
    );
  });
  it("falls back to extensions when the mime type is generic", () => {
    expect(
      mediaKind({ mime_type: "application/octet-stream", path: "notes.md" }),
    ).toBe("text");
    expect(
      mediaKind({ mime_type: "application/octet-stream", path: "clip.mp4" }),
    ).toBe("video");
    expect(
      mediaKind({ mime_type: "application/octet-stream", path: "song.flac" }),
    ).toBe("audio");
    expect(
      mediaKind({ mime_type: "application/octet-stream", path: "archive.bin" }),
    ).toBe("unsupported");
  });
});
