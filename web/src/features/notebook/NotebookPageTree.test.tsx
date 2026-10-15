/// <reference lib="es2024.promise" />
import { afterEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NotebookPageTree } from "./NotebookPageTree";

afterEach(() => { cleanup(); vi.restoreAllMocks(); });

it("reads the owner page metadata and exposes a real parent selection", async () => {
  vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({ data: { items: [
    { file_id: "root", space_id: "space", version_id: "v1", path: "Root.md", parent_file_id: "", icon: "book", revision: 1 },
    { file_id: "child", space_id: "space", version_id: "v2", path: "Root/Child.md", parent_file_id: "root", icon: "file", revision: 1 },
  ] } }), { status: 200, headers: { "Content-Type": "application/json" } }));
  const select = vi.fn();
  render(<NotebookPageTree space="space" onSelect={select}/>);
  fireEvent.click(screen.getByText("页面树与新建父页面"));
  await screen.findByRole("button", { name: /Root/ });
  fireEvent.click(screen.getByRole("button", { name: /Root/ }));
  expect(select).toHaveBeenCalledWith("root");
  expect(fetch).toHaveBeenCalledWith("/api/v1/notebook/pages?space_id=space&limit=100", expect.objectContaining({ method: "GET" }));
});

it("keeps forbidden metadata separate from an empty page tree", async () => {
  vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({ error: { code: "forbidden" } }), { status: 403, headers: { "Content-Type": "application/json" } }));
  render(<NotebookPageTree space="space" onSelect={vi.fn()}/>);
  fireEvent.click(screen.getByText("页面树与新建父页面"));
  await screen.findByText("页面树权限已失效。");
  expect(screen.queryByText("当前来源还没有服务端页面元数据。")).toBeNull();
});

it("does not abort a slow page tree when loading state rerenders", async () => {
  const gate = Promise.withResolvers<Response>();
  const fetch = vi.spyOn(globalThis, "fetch").mockImplementationOnce(() => gate.promise);
  render(<NotebookPageTree space="space" onSelect={vi.fn()}/>);
  fireEvent.click(screen.getByText("页面树与新建父页面"));
  await screen.findByText("正在读取页面树…");
  const request = fetch.mock.calls[0][1];
  expect(request?.signal?.aborted).toBe(false);
  gate.resolve(new Response(JSON.stringify({ data: { items: [{ file_id: "root", space_id: "space", version_id: "v1", path: "Root.md", parent_file_id: "", icon: "book", revision: 1 }] } }), { status: 200 }));
  await screen.findByRole("button", { name: /Root/ });
  expect(fetch).toHaveBeenCalledTimes(1);
});

it("retries errors only on explicit reload and clears offline metadata", async () => {
  const fetch = vi.spyOn(globalThis, "fetch").mockRejectedValueOnce(new Error("unavailable")).mockResolvedValueOnce(new Response(JSON.stringify({ data: { items: [] } }), { status: 200 }));
  render(<NotebookPageTree space="space" onSelect={vi.fn()}/>);
  fireEvent.click(screen.getByText("页面树与新建父页面"));
  await screen.findByText("页面树暂不可用，请稍后重试。");
  expect(fetch).toHaveBeenCalledTimes(1);
  fireEvent.click(screen.getByRole("button", { name: "重读页面树" }));
  await screen.findByText("当前来源还没有服务端页面元数据。");
  fireEvent(window, new Event("offline"));
  await waitFor(() => expect(screen.queryByText("当前来源还没有服务端页面元数据。")).toBeNull());
  await screen.findByText("当前离线，页面树已清除；联网后请手动重读。");
});
it('clears a selected parent absent from the successful authorized tree', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({ data: { items: [] } }), { status: 200 })); const select = vi.fn(); render(<NotebookPageTree space="space" selected="old-parent" onSelect={select}/>); fireEvent.click(screen.getByText('页面树与新建父页面')); await screen.findByText('当前来源还没有服务端页面元数据。'); expect(select).toHaveBeenCalledWith(undefined);
});
it('clears the selected parent before a slow explicit reload can finish', async () => {
  const gate = Promise.withResolvers<Response>();
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(new Response(JSON.stringify({ data: { items: [{ file_id: 'root', space_id: 'space', version_id: 'v1', path: 'Root.md', parent_file_id: '', icon: 'book', revision: 1 }] } }), { status: 200 })).mockReturnValueOnce(gate.promise);
  const select = vi.fn(); render(<NotebookPageTree space="space" selected="root" onSelect={select}/>);
  fireEvent.click(screen.getByText('页面树与新建父页面'));
  await screen.findByRole('button', { name: 'Root' }); select.mockClear();
  fireEvent.click(screen.getByRole('button', { name: '重读页面树' }));
  expect(select).toHaveBeenCalledWith(undefined);
  expect(screen.getByText('正在读取页面树…')).toBeTruthy();
  gate.resolve(new Response(JSON.stringify({ data: { items: [] } }), { status: 200 }));
  await screen.findByText('当前来源还没有服务端页面元数据。');
});
