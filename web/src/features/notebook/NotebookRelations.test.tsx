import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { NotebookRelations } from "./NotebookRelations";
import { readNoteLinks, rebuildNoteLinks, lookupNoteLinks } from "./notebook-relations";
import { claimDocumentSave, clearDocumentSave } from "./document-client";
vi.mock("./notebook-relations", () => ({ readNoteLinks: vi.fn(), rebuildNoteLinks: vi.fn(), lookupNoteLinks: vi.fn() }));
vi.mock("./document-client", () => ({ claimDocumentSave: vi.fn(), clearDocumentSave: vi.fn() }));
const identity = { sourceId: "space", objectId: "file", revision: "v1" };
const empty = { file_id: "file", version_id: "v1", indexed: true, stale: false, items: [], next_cursor: "", observed_at: "2026-10-09T00:00:00Z", total: null, total_kind: "unknown" as const };
afterEach(() => { cleanup(); localStorage.clear(); vi.resetAllMocks(); });

it("reads only on entry and keeps display preference rerenders from refetching", async () => {
  vi.mocked(readNoteLinks).mockResolvedValue(empty);
  const view = render(<NotebookRelations identity={identity} actor="alice"/>);
  await screen.findByText(/当前可见索引中没有反向链接/);
  view.rerender(<NotebookRelations identity={{ ...identity }} actor="alice"/>);
  expect(readNoteLinks).toHaveBeenCalledTimes(2);
  expect(rebuildNoteLinks).not.toHaveBeenCalled();
  expect(lookupNoteLinks).not.toHaveBeenCalled();
});

it("does not allow a late old revision to repopulate revoked relationships", async () => {
  let finish = (_value: typeof empty) => {};
  // The project targets ES2022, which has no Promise.withResolvers type.
  const promise = new Promise<typeof empty>(resolve => { finish = resolve; });
  vi.mocked(readNoteLinks).mockReturnValue(promise);
  render(<NotebookRelations identity={identity} actor="alice"/>);
  fireEvent(window, new Event("drivebridge-context"));
  await act(async () => finish(empty));
  expect(screen.getByRole("alert").textContent).toContain("已清除");
  expect(screen.queryByText(/当前可见索引中没有反向链接/)).toBeNull();
  expect(screen.queryByRole("button", { name: "重建此版本关系索引" })).toBeNull();
});

it("queries the same key after a lost rebuild response instead of rebuilding again", async () => {
  const key = "edit-" + "a".repeat(48);
  vi.mocked(readNoteLinks).mockResolvedValue(empty);
  vi.mocked(claimDocumentSave).mockResolvedValue({ key, claimed: true });
  vi.mocked(rebuildNoteLinks).mockRejectedValue(new Error("lost_response"));
  vi.mocked(lookupNoteLinks).mockResolvedValue({ file_id: "file", version_id: "v1", request_digest: "digest", committed_at: "2026-10-09T00:00:00Z" });
  render(<NotebookRelations identity={identity} actor="alice"/>);
  fireEvent.click(await screen.findByRole("button", { name: "重建此版本关系索引" }));
  fireEvent.click(await screen.findByRole("button", { name: "查询原索引结果" }));
  await screen.findByText(/已确认固定版本索引/);
  expect(rebuildNoteLinks).toHaveBeenCalledTimes(1);
  expect(lookupNoteLinks).toHaveBeenCalledWith(identity, key, expect.any(AbortSignal));
  expect(clearDocumentSave).toHaveBeenCalledWith(expect.stringContaining("alice"), key);
});

it("keeps refresh and rebuild from aborting an in-flight next page", async () => {
  let finish = (_value: typeof empty) => {};
  const next = new Promise<typeof empty>(resolve => { finish = resolve; });
  vi.mocked(readNoteLinks).mockResolvedValueOnce({ ...empty, next_cursor: 'cursor' }).mockResolvedValueOnce(empty).mockReturnValueOnce(next);
  render(<NotebookRelations identity={identity} actor="alice"/>);
  fireEvent.click(await screen.findByRole('button', { name: '更多文内链接' }));
  expect(screen.getByRole('button', { name: '刷新关系' }).hasAttribute('disabled')).toBe(true);
  expect(screen.getByRole('button', { name: '重建此版本关系索引' }).hasAttribute('disabled')).toBe(true);
  await act(async () => finish(empty));
  expect(screen.getByRole('button', { name: '刷新关系' }).hasAttribute('disabled')).toBe(false);
});
