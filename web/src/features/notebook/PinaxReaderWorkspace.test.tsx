import { afterEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { PinaxReaderWorkspace } from "./PinaxReaderWorkspace";
import type { DesktopBridge } from "../../generated/desktop-ipc";

const note = { object_id: "note", revision: "r1", title: "旅记", title_state: "present", status: "active", tags: [], updated_at: "2026-10-11T00:00:00Z", uri: "pinax://note/note?v=r1" } as const;
const bridge: Pick<DesktopBridge, "listNotes" | "searchNotes" | "readDocument" | "readLinks"> = {
  listNotes: vi.fn().mockResolvedValue({ data: { items: [note], index_state: { status: "ready" }, index_watermark: "w", observed_at: "now" } }),
  searchNotes: vi.fn().mockResolvedValue({ data: { items: [note], index_state: { status: "ready" }, index_watermark: "w", observed_at: "now", snippets: [] } }),
  readDocument: vi.fn().mockResolvedValue({ data: { schema_version: "pinax.reader.document.v1alpha1", object_id: "note", revision: "r1", current_revision: "r1", uri: note.uri, observed_at: "now", complete: true, content_bytes: 5, nodes: [{ type: "paragraph", anchor: "a", text: "正文" }, { type: "image", anchor: "img" }, { type: "unsupported", anchor: "raw" }], outline: [] } }),
  readLinks: vi.fn().mockResolvedValue({ data: { direction: "backlinks", object_id: "note", revision: "r1", observed_at: "now", relations_watermark: "w", items: [{ object_id: "other", anchor: "a" }] } }),
};
afterEach(() => { cleanup(); vi.clearAllMocks(); });

it("reads fixed Pinax revisions, backlinks, and keeps the UI read-only", async () => {
  render(<PinaxReaderWorkspace bridge={bridge} sourceId="source"/>);
  fireEvent.click(await screen.findByRole("button", { name: "旅记" }));
  await screen.findByText("正文");
  expect(screen.getByText("other#a")).toBeTruthy();
  expect(screen.getByLabelText("Pinax 来源绑定").textContent).toContain("source");
  expect(screen.getByRole("region", { name: "Pinax 附件状态" }).textContent).toContain("owner attachment resolution required");
  expect(screen.getByText(/只读 Pinax reader/)).toBeTruthy();
  expect(bridge.readDocument).toHaveBeenCalledWith({ input: { object_id: "note", revision: "r1" }, source_id: "source" });
});

it("rejects a late document identity instead of showing it under another note", async () => {
  let finish!: (value: unknown) => void;
  vi.mocked(bridge.readDocument).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }) as never);
  render(<PinaxReaderWorkspace bridge={bridge} sourceId="source"/>);
  fireEvent.click(await screen.findByRole("button", { name: "旅记" }));
  finish({ data: { schema_version: "pinax.reader.document.v1alpha1", object_id: "other", revision: "bad", current_revision: "bad", uri: "pinax://note/other?v=bad", observed_at: "now", complete: true, content_bytes: 0, nodes: [], outline: [] } });
  expect((await screen.findAllByText("pinax_reader_identity_mismatch")).length).toBeGreaterThan(0);
  expect(screen.queryByLabelText("Pinax 固定正文")).toBeNull();
});
