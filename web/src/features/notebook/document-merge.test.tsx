import "@testing-library/jest-dom/vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { MarkdownFileDocument } from "./MarkdownFileDocument";
import { documentCapability, documentRecoveryKey, lookupDocumentSave, saveDocument } from "./document-client";
import { readMarkdownFile } from "./notebook-adapters";
import type * as DocumentClient from "./document-client";
import type * as NotebookAdapters from "./notebook-adapters";

vi.mock("./notebook-adapters", async original => ({ ...await original<typeof NotebookAdapters>(), readMarkdownFile: vi.fn() }));
vi.mock("./document-client", async original => ({ ...await original<typeof DocumentClient>(), documentCapability: vi.fn(), saveDocument: vi.fn(), lookupDocumentSave: vi.fn() }));
const identity = { sourceId: "space", objectId: "note", revision: "one" };
const receipt = { file_id: "note", version_id: "three", operation_id: "op", status: "completed", file_ref: "drivebridge://fixture/file/note/version/three" };
const conflict = Object.assign(new Error("version_conflict"), { status: 409 });
const locks = Object.getOwnPropertyDescriptor(navigator, "locks");
beforeEach(() => {
  Object.defineProperty(navigator, "locks", { configurable: true, value: { request: vi.fn(async (_key: string, options: LockOptions | (() => unknown), callback?: () => unknown) => typeof options === "function" ? options() : callback!()) } });
  vi.mocked(readMarkdownFile).mockImplementation(async ref => ref.revision === "one" ? "original base" : "remote body");
  vi.mocked(documentCapability).mockResolvedValue({ file_id: "note", version_id: "one", editable: true, reason: "ready", max_bytes: 2097152 });
});
afterEach(() => {
  cleanup(); localStorage.clear(); vi.restoreAllMocks(); vi.resetAllMocks();
  if (locks) Object.defineProperty(navigator, "locks", locks); else Reflect.deleteProperty(navigator, "locks");
});
async function submit() {
  await waitFor(() => expect(screen.getByRole("button", { name: "预览保存" })).toBeEnabled());
  fireEvent.click(screen.getByRole("button", { name: "预览保存" }));
  fireEvent.click(screen.getByRole("button", { name: "确认保存新版本" }));
  await waitFor(() => expect(saveDocument).toHaveBeenCalled());
}
async function openMerge() {
  vi.mocked(documentCapability).mockResolvedValue({ file_id: "note", version_id: "two", editable: true, reason: "ready", max_bytes: 2097152 });
  fireEvent.click(screen.getByRole("button", { name: "查询原保存结果" }));
  fireEvent.click(await screen.findByRole("button", { name: "读取当前版本并人工合并" }));
  return screen.findByRole("dialog", { name: "人工合并冲突" });
}
async function confirmMerge(text = "merged private body") {
  fireEvent.change(screen.getByRole("textbox", { name: "人工合并正文" }), { target: { value: text } });
  fireEvent.click(screen.getByRole("button", { name: "预览合并差异" }));
  expect(screen.getByRole("button", { name: "确认新基线并保存新版本" })).toBeDisabled();
  fireEvent.click(screen.getByRole("checkbox", { name: /我已审阅差异/ }));
  fireEvent.click(screen.getByRole("button", { name: "确认新基线并保存新版本" }));
}
it("proves rejection by original key before pinning the head and confirming a new intent", async () => {
  const complete = vi.fn();
  vi.mocked(saveDocument).mockRejectedValueOnce(conflict).mockResolvedValueOnce(receipt);
  vi.mocked(lookupDocumentSave).mockResolvedValue({ found: false, save: null });
  render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="local private body" onDraft={vi.fn()} onSaved={complete}/>);
  await submit();
  await screen.findByText(/文件版本已变化/);
  expect(screen.queryByRole("button", { name: "读取当前版本并人工合并" })).toBeNull();
  const originalKey = vi.mocked(saveDocument).mock.calls[0][2];
  const dialog = await openMerge();
  expect(lookupDocumentSave).toHaveBeenCalledWith(identity, originalKey, expect.any(AbortSignal));
  expect(readMarkdownFile).toHaveBeenLastCalledWith({ ...identity, revision: "two" }, expect.any(AbortSignal));
  expect(within(dialog).getByText("original base")).toBeTruthy();
  expect(within(dialog).getByText("remote body")).toBeTruthy();
  expect(within(dialog).getByRole("textbox", { name: "人工合并正文" })).toHaveValue("local private body");
  expect(complete).not.toHaveBeenCalled();
  await confirmMerge();
  await waitFor(() => expect(complete).toHaveBeenCalledWith({ ...identity, revision: "three" }));
  const next = vi.mocked(saveDocument).mock.calls[1];
  expect(next[0].revision).toBe("two"); expect(next[1]).toBe("merged private body"); expect(next[2]).not.toBe(originalKey);
  expect(Object.values(localStorage).join()).not.toContain("private");
});
it.each(["network", "idempotency_conflict"])("does not treat %s plus not-found as proof of rejection", async failure => {
  vi.mocked(saveDocument).mockRejectedValue(failure === "network" ? new TypeError("network") : Object.assign(new Error(failure), { status: 409 }));
  vi.mocked(lookupDocumentSave).mockResolvedValue({ found: false, save: null });
  render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="draft" onDraft={vi.fn()} onSaved={vi.fn()}/>);
  await submit();
  fireEvent.click(await screen.findByRole("button", { name: "查询原保存结果" }));
  await screen.findByText(/尚未找到原保存记录/);
  expect(screen.queryByRole("button", { name: "读取当前版本并人工合并" })).toBeNull();
  const key = vi.mocked(saveDocument).mock.calls[0][2];
  expect(localStorage.getItem(documentRecoveryKey("actor", identity))).toBe(key);
  expect(saveDocument).toHaveBeenCalledTimes(1);
  expect(documentCapability).toHaveBeenCalledTimes(1);
});
it("keeps the same body and key when resuming an unknown rebased write", async () => {
  vi.mocked(saveDocument).mockRejectedValueOnce(conflict).mockRejectedValueOnce(new TypeError("network")).mockResolvedValueOnce(receipt);
  vi.mocked(lookupDocumentSave).mockResolvedValue({ found: false, save: null });
  render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="draft" onDraft={vi.fn()} onSaved={vi.fn()}/>);
  await submit(); await screen.findByText(/文件版本已变化/); await openMerge(); await confirmMerge();
  await screen.findByText(/保存结果尚未确认/);
  fireEvent.click(screen.getByRole("button", { name: "查询原保存结果" }));
  fireEvent.click(await screen.findByRole("button", { name: "继续原保存" }));
  await waitFor(() => expect(saveDocument).toHaveBeenCalledTimes(3));
  expect(vi.mocked(saveDocument).mock.calls[2].slice(0, 3)).toEqual(vi.mocked(saveDocument).mock.calls[1].slice(0, 3));
});
it("retains a merged draft and requires another lookup after a second conflict", async () => {
  vi.mocked(saveDocument).mockRejectedValue(conflict);
  vi.mocked(lookupDocumentSave).mockResolvedValue({ found: false, save: null });
  const complete = vi.fn();
  render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="draft" onDraft={vi.fn()} onSaved={complete}/>);
  await submit(); await screen.findByText(/文件版本已变化/); await openMerge(); await confirmMerge();
  await screen.findByText(/文件版本已变化/);
  expect(screen.getByText("merged private body", { selector: "p" })).toBeTruthy();
  expect(screen.queryByRole("button", { name: "读取当前版本并人工合并" })).toBeNull();
  const secondKey = vi.mocked(saveDocument).mock.calls[1][2];
  expect(localStorage.getItem(documentRecoveryKey("actor", identity))).toBe(secondKey);
  const dialog = await openMerge();
  expect(within(dialog).getByRole("textbox", { name: "人工合并正文" })).toHaveValue("merged private body");
  expect(complete).not.toHaveBeenCalled();
});
it("permits rebase after a cancelled original operation, but not while it is transferring", async () => {
  const key = "edit-" + "a".repeat(48);
  localStorage.setItem(documentRecoveryKey("actor", identity), key);
  vi.mocked(lookupDocumentSave).mockResolvedValueOnce({ found: true, save: { ...receipt, version_id: "", status: "transferring" } }).mockResolvedValueOnce({ found: true, save: { ...receipt, version_id: "", status: "cancelled" } });
  render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="draft" onDraft={vi.fn()} onSaved={vi.fn()}/>);
  fireEvent.click(await screen.findByRole("button", { name: "查询原保存结果" }));
  await screen.findByText(/原保存尚未完成/);
  expect(screen.queryByRole("button", { name: "读取当前版本并人工合并" })).toBeNull();
  await openMerge(); expect(saveDocument).not.toHaveBeenCalled();
});
it("never marks a newer local draft saved by a late successful receipt", async () => {
  const { promise, resolve: finish } = Promise.withResolvers<typeof receipt>();
  const complete = vi.fn();
  vi.mocked(saveDocument).mockReturnValue(promise);
  const view = render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="submitted" onDraft={vi.fn()} onSaved={complete}/>);
  await submit();
  view.rerender(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="newer unsaved" onDraft={vi.fn()} onSaved={complete}/>);
  await act(async () => { finish(receipt); });
  await screen.findByRole("button", { name: "读取当前版本并人工合并" });
  expect(complete).not.toHaveBeenCalled();
  expect(screen.getByText("newer unsaved", { selector: "p" })).toBeTruthy();
});
it("clears all merge bodies on revocation and drops a late pinned-head read", async () => {
  vi.mocked(saveDocument).mockRejectedValue(conflict);
  vi.mocked(lookupDocumentSave).mockResolvedValue({ found: false, save: null });
  const revoked = vi.fn();
  render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="private local" onDraft={vi.fn()} onSaved={vi.fn()} onRevoked={revoked}/>);
  await submit(); await screen.findByText(/文件版本已变化/); await openMerge();
  fireEvent.change(screen.getByRole("textbox", { name: "人工合并正文" }), { target: { value: "private merge" } });
  fireEvent.click(screen.getByRole("button", { name: "关闭" }));
  const { promise, resolve: finish } = Promise.withResolvers<string>();
  vi.mocked(readMarkdownFile).mockReturnValue(promise);
  fireEvent.click(screen.getByRole("button", { name: "读取当前版本并人工合并" }));
  await waitFor(() => expect(readMarkdownFile).toHaveBeenCalledTimes(3));
  fireEvent(window, new Event("drivebridge-context"));
  await act(async () => { finish("late private remote"); });
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(screen.queryByText(/private/)).toBeNull();
  expect(revoked).toHaveBeenCalledTimes(1);
});
it("keeps another tab's completed receipt separate from the current draft and exposes manual rebase", async () => {
  const key = "edit-" + "b".repeat(48);
  localStorage.setItem(documentRecoveryKey("actor", identity), key);
  vi.mocked(lookupDocumentSave).mockResolvedValue({ found: true, save: receipt });
  const complete = vi.fn();
  render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="my unsaved draft" onDraft={vi.fn()} onSaved={complete}/>);
  await screen.findByRole("button", { name: "查询原保存结果" });
  await waitFor(() => expect(documentCapability).toHaveBeenCalledTimes(1));
  const dialog = await openMerge();
  expect(within(dialog).getByRole("textbox", { name: "人工合并正文" })).toHaveValue("my unsaved draft");
  expect(complete).not.toHaveBeenCalled();
  expect(saveDocument).not.toHaveBeenCalled();
});
it("does not replace the key when lookup itself fails after a deterministic conflict", async () => {
  vi.mocked(saveDocument).mockRejectedValue(conflict);
  vi.mocked(lookupDocumentSave).mockRejectedValue(new TypeError("offline"));
  render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="draft" onDraft={vi.fn()} onSaved={vi.fn()}/>);
  await submit(); await screen.findByText(/文件版本已变化/);
  const key = vi.mocked(saveDocument).mock.calls[0][2];
  fireEvent.click(screen.getByRole("button", { name: "查询原保存结果" }));
  await screen.findByText(/保存结果尚未确认/);
  expect(screen.queryByRole("button", { name: "读取当前版本并人工合并" })).toBeNull();
  expect(localStorage.getItem(documentRecoveryKey("actor", identity))).toBe(key);
});
it("clears the prior merge session when switching source identity", async () => {
  vi.mocked(saveDocument).mockRejectedValue(conflict);
  vi.mocked(lookupDocumentSave).mockResolvedValue({ found: false, save: null });
  const view = render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="private source draft" onDraft={vi.fn()} onSaved={vi.fn()}/>);
  await submit(); await screen.findByText(/文件版本已变化/); await openMerge();
  fireEvent.change(screen.getByRole("textbox", { name: "人工合并正文" }), { target: { value: "private source merge" } });
  vi.mocked(readMarkdownFile).mockResolvedValue("different source body");
  view.rerender(<MarkdownFileDocument actor="actor" identity={{ ...identity, sourceId: "other-space", objectId: "other-note" }} editablePane onDraft={vi.fn()} onSaved={vi.fn()}/>);
  await screen.findByText("different source body", { selector: "p" });
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(screen.queryByText(/private source/)).toBeNull();
  expect(Object.values(localStorage).join()).not.toContain("private source");
});
it("clears draft bodies when refreshed head capability reports permission revocation", async () => {
  vi.mocked(saveDocument).mockRejectedValue(conflict);
  vi.mocked(lookupDocumentSave).mockResolvedValue({ found: false, save: null });
  const revoked = vi.fn();
  render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="private body" onDraft={vi.fn()} onSaved={vi.fn()} onRevoked={revoked}/>);
  await submit(); await screen.findByText(/文件版本已变化/);
  fireEvent.click(screen.getByRole("button", { name: "查询原保存结果" }));
  const rebase = await screen.findByRole("button", { name: "读取当前版本并人工合并" });
  vi.mocked(documentCapability).mockResolvedValue({ file_id: "note", version_id: "two", editable: false, reason: "forbidden", max_bytes: 2097152 });
  fireEvent.click(rebase);
  await screen.findByText(/访问权限已失效/);
  expect(screen.queryByText(/private body/)).toBeNull();
  expect(revoked).toHaveBeenCalledTimes(1);
  expect(readMarkdownFile).toHaveBeenCalledTimes(1);
});
it("refuses a terminal receipt for a different document", async () => {
  const key = "edit-" + "c".repeat(48);
  localStorage.setItem(documentRecoveryKey("actor", identity), key);
  vi.mocked(lookupDocumentSave).mockResolvedValue({ found: true, save: { ...receipt, file_id: "other-note", status: "cancelled" } });
  const complete = vi.fn();
  render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="draft" onDraft={vi.fn()} onSaved={complete}/>);
  fireEvent.click(await screen.findByRole("button", { name: "查询原保存结果" }));
  await screen.findByText(/保存结果尚未确认/);
  expect(screen.queryByRole("button", { name: "读取当前版本并人工合并" })).toBeNull();
  expect(localStorage.getItem(documentRecoveryKey("actor", identity))).toBe(key);
  expect(complete).not.toHaveBeenCalled();
});
it("preserves local merge edits on close and mirrors them to the owner's dirty draft", async () => {
  vi.mocked(saveDocument).mockRejectedValue(conflict);
  vi.mocked(lookupDocumentSave).mockResolvedValue({ found: false, save: null });
  const onDraft = vi.fn(); const complete = vi.fn();
  render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="draft" onDraft={onDraft} onSaved={complete}/>);
  await submit(); await screen.findByText(/文件版本已变化/); await openMerge();
  fireEvent.change(screen.getByRole("textbox", { name: "人工合并正文" }), { target: { value: "unpublished merge" } });
  expect(onDraft).toHaveBeenLastCalledWith("unpublished merge");
  fireEvent.click(screen.getByRole("button", { name: "返回并保留草稿" }));
  expect(screen.getByText("unpublished merge", { selector: "p" })).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "读取当前版本并人工合并" }));
  await screen.findByRole("dialog", { name: "人工合并冲突" });
  expect(screen.getByRole("textbox", { name: "人工合并正文" })).toHaveValue("unpublished merge");
  expect(complete).not.toHaveBeenCalled();
  expect(saveDocument).toHaveBeenCalledTimes(1);
});
