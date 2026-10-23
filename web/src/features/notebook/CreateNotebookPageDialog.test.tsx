/// <reference lib="es2024.promise" />
import { afterEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { CreateNotebookPageDialog } from "./CreateNotebookPageDialog";
import { createNotebookDaily, createNotebookPage, lookupNotebookCreation, resumeNotebookCreation } from "./notebook-client";
import type { NotebookCreation } from "./notebook-client";
import { notebookPageSlot } from "./notebook-recovery";

vi.mock("./notebook-client", () => ({ createNotebookDaily: vi.fn(), createNotebookPage: vi.fn(), lookupNotebookCreation: vi.fn(), resumeNotebookCreation: vi.fn() }));
vi.mock("../../components/AppDialog", () => ({ AppDialog: ({ title, children }: { title: string; children: React.ReactNode }) => <section role="dialog" aria-label={title}>{children}</section> }));
const complete = (path = "Notes/Child.md"): NotebookCreation => ({ found: true, status: "completed", operation_id: "op-note", page: { file_id: "file-note", space_id: "space", version_id: "v1", path, parent_file_id: "parent", icon: "file", revision: 1 } });
const props = { actor: "alice", space: "space", parent: "parent", daily: false, close: vi.fn(), onCreated: vi.fn(), onDenied: vi.fn() };
const originalLocks = Object.getOwnPropertyDescriptor(navigator, "locks");
afterEach(() => { cleanup(); localStorage.clear(); vi.clearAllMocks(); vi.restoreAllMocks(); if (originalLocks) Object.defineProperty(navigator, "locks", originalLocks); else Reflect.deleteProperty(navigator, "locks"); });
function installLocks() { Object.defineProperty(navigator, "locks", { configurable: true, value: { request: async (_name: string, options: LockOptions | (() => unknown), callback?: () => unknown) => typeof options === "function" ? options() : callback!() } }); }
it("creates a child with the selected parent and clears the recovery identity after a confirmed response", async () => {
  installLocks(); vi.mocked(createNotebookPage).mockResolvedValue(complete());
  render(<CreateNotebookPageDialog {...props}/>);
  await screen.findByRole("textbox", { name: "笔记标题" }); fireEvent.change(screen.getByRole("textbox", { name: "笔记标题" }), { target: { value: "Child" } }); fireEvent.click(screen.getByRole("button", { name: "确认创建" }));
  await waitFor(() => expect(createNotebookPage).toHaveBeenCalledWith("space", "Child", "parent", "file", expect.stringMatching(/^notebook-[a-f0-9]{32}$/), expect.any(AbortSignal)));
  expect(props.onCreated).toHaveBeenCalled(); expect(localStorage.length).toBe(0);
});
it("passes the selected physical directory to the notebook contract", async () => {
  installLocks(); vi.mocked(createNotebookPage).mockResolvedValue(complete("notes/Child.md"));
  render(<CreateNotebookPageDialog {...props} directoryPath="notes" parent={undefined}/>);
  await screen.findByRole("textbox", { name: "笔记标题" }); fireEvent.change(screen.getByRole("textbox", { name: "笔记标题" }), { target: { value: "Child" } }); fireEvent.click(screen.getByRole("button", { name: "确认创建" }));
  await waitFor(() => expect(createNotebookPage).toHaveBeenCalledWith("space", "Child", "", "file", expect.stringMatching(/^notebook-[a-f0-9]{32}$/), expect.any(AbortSignal), "notes"));
});
it("does not issue a second POST after an unknown response and recovers through the original key", async () => {
  installLocks(); const gate = Promise.withResolvers<NotebookCreation>(); vi.mocked(createNotebookPage).mockReturnValueOnce(gate.promise); vi.mocked(lookupNotebookCreation).mockResolvedValue(complete());
  render(<CreateNotebookPageDialog {...props}/>); await screen.findByRole("textbox", { name: "笔记标题" }); fireEvent.change(screen.getByRole("textbox", { name: "笔记标题" }), { target: { value: "Draft" } }); fireEvent.click(screen.getByRole("button", { name: "确认创建" }));
  await waitFor(() => expect(createNotebookPage).toHaveBeenCalledTimes(1)); gate.reject(new Error("network_lost")); await screen.findByText(/结果待回查/); fireEvent.click(screen.getByRole("button", { name: "查询原创建" }));
  await waitFor(() => expect(lookupNotebookCreation).toHaveBeenCalledTimes(1)); expect(createNotebookPage).toHaveBeenCalledTimes(1); expect(props.onCreated).toHaveBeenCalledTimes(1);
});
it("creates a daily page with the server daily contract", async () => {
  installLocks(); vi.mocked(createNotebookDaily).mockResolvedValue(complete("2026-10-12.md"));
  render(<CreateNotebookPageDialog {...props} daily/>); await screen.findByText(/日记日期/); fireEvent.click(screen.getByRole("button", { name: "确认创建" }));
  await waitFor(() => expect(createNotebookDaily).toHaveBeenCalledWith("space", expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/), expect.stringMatching(/^notebook-[a-f0-9]{32}$/), expect.any(AbortSignal)));
});
it("does not create while offline", async () => {
  installLocks(); Object.defineProperty(navigator, "onLine", { configurable: true, value: false });
  render(<CreateNotebookPageDialog {...props}/>); await screen.findByText(/当前离线/); expect(screen.queryByRole("button", { name: "确认创建" })).toBeNull(); expect(createNotebookPage).not.toHaveBeenCalled();
  Object.defineProperty(navigator, "onLine", { configurable: true, value: true });
});

it("does not auto-create after coming back online", async () => {
  installLocks(); Object.defineProperty(navigator, "onLine", { configurable: true, value: false });
  render(<CreateNotebookPageDialog {...props}/>); await screen.findByText(/当前离线/); fireEvent(window, new Event("online"));
  expect(createNotebookPage).not.toHaveBeenCalled();
  Object.defineProperty(navigator, "onLine", { configurable: true, value: true });
});
it("restores a persisted original identity without creating and resumes only the found operation", async () => {
  installLocks(); vi.mocked(createNotebookPage).mockRejectedValueOnce(new Error("lost_response"));
  const first = render(<CreateNotebookPageDialog {...props}/>); await screen.findByRole("textbox", { name: "笔记标题" }); fireEvent.change(screen.getByRole("textbox", { name: "笔记标题" }), { target: { value: "Private title" } }); fireEvent.click(screen.getByRole("button", { name: "确认创建" }));
  await screen.findByText(/结果待回查/); const key = vi.mocked(createNotebookPage).mock.calls[0][4];
  expect(localStorage.getItem(localStorage.key(0)!)).not.toContain("Private title"); first.unmount();
  vi.mocked(lookupNotebookCreation).mockResolvedValueOnce({ found: true, status: "verifying", operation_id: "op-note", page: null }); vi.mocked(resumeNotebookCreation).mockResolvedValueOnce(complete());
  render(<CreateNotebookPageDialog {...props}/>); await screen.findByText(/结果待回查/); expect(screen.queryByRole("button", { name: "确认创建" })).toBeNull(); expect(createNotebookPage).toHaveBeenCalledTimes(1);
  fireEvent(window, new Event("online")); expect(createNotebookPage).toHaveBeenCalledTimes(1); expect(lookupNotebookCreation).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "查询原创建" })); await screen.findByRole("button", { name: "恢复原创建" });
  expect(lookupNotebookCreation).toHaveBeenCalledWith("space", key, "", expect.any(AbortSignal)); fireEvent.click(screen.getByRole("button", { name: "恢复原创建" }));
  await waitFor(() => expect(resumeNotebookCreation).toHaveBeenCalledWith("space", key, "", expect.any(AbortSignal))); await waitFor(() => expect(props.onCreated).toHaveBeenCalledTimes(1)); expect(createNotebookPage).toHaveBeenCalledTimes(1);
});
it.each([[400, 'invalid_request'], [422, 'unsupported_action']])('releases only its rejected pre-admission key %s and preserves title', async (status, code) => {
  installLocks(); vi.mocked(createNotebookPage).mockRejectedValueOnce(Object.assign(new Error(String(code)), { status }));
  render(<CreateNotebookPageDialog {...props}/>); await screen.findByRole('textbox', { name: '笔记标题' }); fireEvent.change(screen.getByRole('textbox', { name: '笔记标题' }), { target: { value: 'Correctable title' } }); fireEvent.click(screen.getByRole('button', { name: '确认创建' }));
  await screen.findByText(/创建在受理前被拒绝/); expect(screen.getByRole('textbox', { name: '笔记标题' })).toHaveProperty('value', 'Correctable title'); expect(localStorage.length).toBe(0);
});

it("explains missing Web Locks without sending a create request", async () => {
  Object.defineProperty(navigator, "locks", { configurable: true, value: undefined });
  render(<CreateNotebookPageDialog {...props}/>);
  await screen.findByText(/Web Locks/);
  expect(createNotebookPage).not.toHaveBeenCalled();
});

it("offers explicit cleanup for a malformed recovery record", async () => {
  installLocks(); localStorage.setItem(notebookPageSlot("alice", "", "space", ""), "broken");
  render(<CreateNotebookPageDialog {...props}/>);
  await screen.findByText(/无法解析的旧恢复记录/);
  fireEvent.click(screen.getByRole("button", { name: "清除损坏恢复记录并重新开始" }));
  await screen.findByRole("textbox", { name: "笔记标题" });
  expect(localStorage.length).toBe(0);
});

it("opens a confirmed page even when recovery cleanup loses the lock", async () => {
  installLocks();
  vi.mocked(createNotebookPage).mockImplementationOnce(async () => {
    Object.defineProperty(navigator, "locks", { configurable: true, value: undefined });
    return complete();
  });
  render(<CreateNotebookPageDialog {...props}/>);
  await screen.findByRole("textbox", { name: "笔记标题" }); fireEvent.change(screen.getByRole("textbox", { name: "笔记标题" }), { target: { value: "Child" } }); fireEvent.click(screen.getByRole("button", { name: "确认创建" }));
  await screen.findByText(/已创建固定版本/);
  expect(props.onCreated).toHaveBeenCalledOnce(); expect(localStorage.length).toBe(1);
});
