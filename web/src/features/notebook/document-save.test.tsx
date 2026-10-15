/// <reference lib="es2024.promise" />
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { MarkdownFileDocument } from "./MarkdownFileDocument";
import { documentCapability, documentRecoveryKey, lookupDocumentSave, saveDocument } from "./document-client";
import { readMarkdownFile } from "./notebook-adapters";
vi.mock("./notebook-adapters", async original => ({ ...await original<typeof import("./notebook-adapters")>(), readMarkdownFile: vi.fn() }));
vi.mock("./document-client", async original => ({ ...await original<typeof import("./document-client")>(), documentCapability: vi.fn(), saveDocument: vi.fn(), lookupDocumentSave: vi.fn() }));
const identity = { sourceId: "space", objectId: "note", revision: "one" };
const saved = { file_id: "note", version_id: "two", operation_id: "op", status: "completed", file_ref: "drivebridge://fixture/file/note/version/two" };
const originalLocks = Object.getOwnPropertyDescriptor(navigator, "locks");
beforeEach(() => {
  Object.defineProperty(navigator, "locks", { configurable: true, value: { request: vi.fn(async (_key: string, options: LockOptions | (() => unknown), callback?: () => unknown) => typeof options === "function" ? options() : callback!()) } });
});
afterEach(() => {
  cleanup(); localStorage.clear(); vi.restoreAllMocks(); vi.resetAllMocks();
  if (originalLocks) Object.defineProperty(navigator, "locks", originalLocks);
  else Reflect.deleteProperty(navigator, "locks");
});
function ready() {
  vi.mocked(readMarkdownFile).mockResolvedValue("before");
  vi.mocked(documentCapability).mockResolvedValue({ file_id: "note", version_id: "one", editable: true, reason: "ready", max_bytes: 2097152 });
}
async function confirm() {
  await waitFor(() => expect(screen.getByRole("button", { name: "预览保存" }).hasAttribute("disabled")).toBe(false));
  fireEvent.click(screen.getByRole("button", { name: "预览保存" }));
  fireEvent.click(screen.getByRole("button", { name: "确认保存新版本" }));
}
it("requires preview, persists only recovery identity and accepts the published version", async () => {
  ready(); const complete = vi.fn();
  vi.mocked(saveDocument).mockImplementation(async (_ref, body, key) => {
    expect(localStorage.getItem(documentRecoveryKey("actor", identity))).toBe(key);
    expect(body).toBe("after private text");
    expect(Object.values(localStorage).join()).not.toContain("private text");
    return saved;
  });
  render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="after private text" onDraft={vi.fn()} onSaved={complete}/>);
  expect(saveDocument).not.toHaveBeenCalled();
  await confirm();
  await waitFor(() => expect(complete).toHaveBeenCalledWith({ ...identity, revision: "two" }));
  expect(localStorage.length).toBe(0);
});
it("queries the original operation after a lost response without sending another mutation", async () => {
  ready(); const complete = vi.fn();
  vi.mocked(saveDocument).mockRejectedValue(new TypeError("network"));
  vi.mocked(lookupDocumentSave).mockResolvedValue({ found: true, save: saved });
  render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="after" onDraft={vi.fn()} onSaved={complete}/>);
  await confirm();
  await screen.findByText(/保存结果尚未确认/);
  fireEvent.click(screen.getByRole("button", { name: "查询原保存结果" }));
  await waitFor(() => expect(complete).toHaveBeenCalledWith({ ...identity, revision: "two" }));
  expect(saveDocument).toHaveBeenCalledTimes(1);
});
it("restores pending identity when the same object now has a newer revision", async () => {
  ready();
  const key = "edit-" + "a".repeat(48);
  localStorage.setItem(documentRecoveryKey("actor", identity), key);
  render(<MarkdownFileDocument actor="actor" identity={{ ...identity, revision: "two" }} editablePane onDraft={vi.fn()} onSaved={vi.fn()}/>);
  await screen.findByRole("button", { name: "查询原保存结果" });
  expect(saveDocument).not.toHaveBeenCalled();
  expect(screen.getByRole("button", { name: "预览保存" }).hasAttribute("disabled")).toBe(true);
});
it("drops a late save response after context revocation", async () => {
  ready(); const complete = vi.fn(); let finish!: (value: typeof saved) => void;
  vi.mocked(saveDocument).mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="after" onDraft={vi.fn()} onSaved={complete}/>);
  await confirm();
  await waitFor(() => expect(saveDocument).toHaveBeenCalledTimes(1));
  fireEvent(window, new Event("drivebridge-context"));
  await act(async () => { finish(saved); });
  expect(complete).not.toHaveBeenCalled();
  expect(screen.getByRole("alert").textContent).toContain("访问权限已失效");
});
it("refuses to overwrite another tab's pending recovery identity", async () => {
  ready();
  render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="my draft" onDraft={vi.fn()} onSaved={vi.fn()}/>);
  await waitFor(() => expect(screen.getByRole("button", { name: "预览保存" }).hasAttribute("disabled")).toBe(false));
  const key = "edit-" + "b".repeat(48);
  localStorage.setItem(documentRecoveryKey("actor", identity), key);
  await confirm();
  await screen.findByText(/另一个标签页已有待确认/);
  expect(localStorage.getItem(documentRecoveryKey("actor", identity))).toBe(key);
  expect(saveDocument).not.toHaveBeenCalled();
  expect(screen.queryByRole("button", { name: "继续原保存" })).toBeNull();
});
it("does not discard a local draft when recovering another tab's completed write", async () => {
  ready(); const complete = vi.fn();
  render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="my unsaved draft" onDraft={vi.fn()} onSaved={complete}/>);
  await waitFor(() => expect(screen.getByRole("button", { name: "预览保存" }).hasAttribute("disabled")).toBe(false));
  localStorage.setItem(documentRecoveryKey("actor", identity), "edit-" + "c".repeat(48));
  await confirm();
  await screen.findByText(/另一个标签页已有待确认/);
  vi.mocked(lookupDocumentSave).mockResolvedValue({ found: true, save: saved });
  fireEvent.click(screen.getByRole("button", { name: "查询原保存结果" }));
  await screen.findByText(/原保存已确认，但不是当前草稿/);
  expect(screen.getByText("my unsaved draft", { selector: "p" })).toBeTruthy();
  expect(complete).not.toHaveBeenCalled();
  expect(saveDocument).not.toHaveBeenCalled();
});
it("requires cross-tab locking and valid recovery storage before enabling writes", async () => {
  ready();
  Object.defineProperty(navigator, "locks", { configurable: true, value: undefined });
  render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="my draft" onDraft={vi.fn()} onSaved={vi.fn()}/>);
  await screen.findByText(/无法安全登记保存恢复身份/);
  expect(documentCapability).not.toHaveBeenCalled();
  expect(saveDocument).not.toHaveBeenCalled();
});
it("clears shared drafts through the owner callback when save permission is revoked", async () => {
  ready(); const revoked = vi.fn();
  vi.mocked(saveDocument).mockRejectedValue(Object.assign(new Error("forbidden"), { status: 403 }));
  render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="private draft" onDraft={vi.fn()} onSaved={vi.fn()} onRevoked={revoked}/>);
  await confirm();
  await waitFor(() => expect(revoked).toHaveBeenCalledTimes(1));
  expect(screen.queryByText("private draft")).toBeNull();
});
it("keeps conflict drafts and requires original-result lookup", async () => {
  ready();
  vi.mocked(saveDocument).mockRejectedValue(Object.assign(new Error("version_conflict"), { status: 409 }));
  render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="my draft" onDraft={vi.fn()} onSaved={vi.fn()}/>);
  await confirm();
  await screen.findByText(/文件版本已变化，草稿保留/);
  expect(screen.getByText("my draft", { selector: "p" })).toBeTruthy();
  expect(saveDocument).toHaveBeenCalledTimes(1);
});
it("preserves the in-memory draft but blocks saving after disconnect", async () => {
  ready(); render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="unsaved offline draft" onDraft={vi.fn()} onSaved={vi.fn()}/>);
  await waitFor(() => expect(screen.getByRole("button", { name: "预览保存" }).hasAttribute("disabled")).toBe(false));
  fireEvent(window, new Event("offline"));
  await screen.findByText(/当前离线，草稿保留在内存/);
  expect(screen.getByText("unsaved offline draft", { selector: "p" })).toBeTruthy();
  expect(screen.getByRole("button", { name: "预览保存" }).hasAttribute("disabled")).toBe(true);
  fireEvent(window, new Event("online"));
  expect(saveDocument).not.toHaveBeenCalled();
});
it('reauthenticates explicitly after disconnect without discarding the draft or automatically saving', async () => {
  ready(); render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="kept draft" onDraft={vi.fn()} onSaved={vi.fn()}/>);
  await waitFor(() => expect(screen.getByRole('button', { name: '预览保存' }).hasAttribute('disabled')).toBe(false)); fireEvent(window, new Event('offline')); fireEvent(window, new Event('online'));
  expect(saveDocument).not.toHaveBeenCalled(); fireEvent.click(screen.getByRole('button', { name: '保留草稿并重新鉴权' }));
  await screen.findByText(/已重新鉴权，内存草稿保留/); expect(screen.getByText('kept draft', { selector: 'p' })).toBeTruthy(); expect(screen.getByRole('button', { name: '预览保存' }).hasAttribute('disabled')).toBe(false);
});
it('rechecks the original document explicitly after an in-flight offline transition', async () => {
  ready(); const gate = Promise.withResolvers<string>(); vi.mocked(readMarkdownFile).mockReset().mockResolvedValueOnce('before').mockImplementationOnce(() => gate.promise); render(<MarkdownFileDocument actor="actor" identity={identity} editablePane draft="keep me" onDraft={vi.fn()} onSaved={vi.fn()}/>); await waitFor(() => expect(screen.getByRole('button', { name: '预览保存' }).hasAttribute('disabled')).toBe(false)); fireEvent(window, new Event('offline')); fireEvent.click(screen.getByRole('button', { name: '保留草稿并重新鉴权' })); gate.resolve('before'); await screen.findByText(/已重新鉴权/); expect(screen.getByText('keep me', { selector: 'p' })).toBeTruthy(); expect(saveDocument).not.toHaveBeenCalled();
});
it('recovers an interrupted initial read only after an explicit fresh request', async () => {
  ready();
  const gate = Promise.withResolvers<string>();
  vi.mocked(readMarkdownFile).mockReturnValueOnce(gate.promise);
  render(<MarkdownFileDocument actor="actor" identity={identity} editablePane onDraft={vi.fn()} onSaved={vi.fn()}/>);
  const originalSignal = vi.mocked(readMarkdownFile).mock.calls[0][1];
  fireEvent(window, new Event('offline'));
  expect(originalSignal.aborted).toBe(true);
  expect(screen.getByText(/固定版本读取已暂停/)).toBeTruthy();
  fireEvent(window, new Event('online'));
  await act(async () => { gate.resolve('late stale body'); await gate.promise; });
  expect(readMarkdownFile).toHaveBeenCalledTimes(1);
  expect(screen.queryByText('late stale body')).toBeNull();
  fireEvent.click(screen.getByRole('button', { name: '重新读取固定版本' }));
  await screen.findByText('before', { selector: 'p' });
  expect(readMarkdownFile).toHaveBeenCalledTimes(2);
  const freshSignal = vi.mocked(readMarkdownFile).mock.calls[1][1];
  expect(freshSignal).not.toBe(originalSignal); expect(freshSignal.aborted).toBe(false);
  expect(saveDocument).not.toHaveBeenCalled();
});
