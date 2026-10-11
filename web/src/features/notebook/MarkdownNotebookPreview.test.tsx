import { afterEach, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import type { DriveFile } from "../../api";
import { setDataContext } from "../../app/data-context";
import { MarkdownNotebookPreview, canOpenMarkdownNotebook } from "./MarkdownNotebookPreview";
const file: DriveFile = { id: "note", space_id: "space", path: "notes/a.md", version_id: "one", size_bytes: 8, mime_type: "text/markdown", trashed: false, updated_at: "2026-10-04" };
afterEach(() => { cleanup(); vi.unstubAllGlobals(); setDataContext(""); });

it("only admits ordinary Markdown files in the active space", () => {
  expect(canOpenMarkdownNotebook(file, "space")).toBe(true);
  expect(canOpenMarkdownNotebook(file, "other")).toBe(false);
  expect(canOpenMarkdownNotebook({ ...file, preview_url: "/scoped-preview" }, "space")).toBe(false);
  expect(canOpenMarkdownNotebook({ ...file, preview_reason: "forbidden" }, "space")).toBe(false);
  expect(canOpenMarkdownNotebook({ ...file, trashed: true }, "space")).toBe(false);
});

it("assembles the real file adapter and workspace with the selected fixed note open", async () => {
  setDataContext("tenant", "space");
  const fetcher = vi.fn(async () => new Response("# Selected note"));
  vi.stubGlobal("fetch", fetcher);
  render(<MarkdownNotebookPreview actor="actor" file={file} siblings={[file]}/>);
  expect(screen.getByRole("tab", { name: "a.md" })).toBeTruthy();
  expect(await screen.findByRole("heading", { name: "Selected note" })).toBeTruthy();
  expect(fetcher).toHaveBeenCalledWith("/api/v1/files/note/content?version=one", expect.anything());
});
