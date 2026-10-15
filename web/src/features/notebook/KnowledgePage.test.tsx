/// <reference lib="es2024.promise" />
import { afterEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { KnowledgePage } from "./KnowledgePage";
import { readKnowledgeFiles } from "./knowledge-files";
import type { Space } from "../../api";
vi.mock("./knowledge-files", () => ({ readKnowledgeFiles: vi.fn() }));
vi.mock("./MarkdownFileDocument", () => ({ MarkdownFileDocument: ({ identity, onRevoked }: { identity: { objectId: string }; onRevoked: () => void }) => <div><p>固定正文 {identity.objectId}</p><button onClick={onRevoked}>模拟正文失权</button></div> }));
const spaces = [{ id: "s", name: "资料", connection_id: "c" }] as Space[];
const entry = { title: "第一篇.md", path: "notes/第一篇.md", ref: { sourceId: "s", objectId: "a", revision: "v1" } };
afterEach(() => { cleanup(); localStorage.clear(); vi.resetAllMocks(); });
it("keeps pagination coverage honest and opens real authorized note identities", async () => {
  vi.mocked(readKnowledgeFiles).mockResolvedValueOnce({ entries: [], nextCursor: "next", scanned: 50, unavailable: 0, stale: 0 })
    .mockResolvedValueOnce({ entries: [entry], nextCursor: null, scanned: 1, unavailable: 0, stale: 0 });
  render(<KnowledgePage actor="a" spaces={spaces}/>);
  await screen.findByText("当前页未找到 Markdown，可继续查找后续文件。");
  const controls = screen.getByText("目录与筛选").closest("details")!;
  expect(controls.open).toBe(false);
  fireEvent.click(screen.getByText("目录与筛选"));
  expect(controls.open).toBe(true);
  expect(readKnowledgeFiles).toHaveBeenCalledTimes(1);
  fireEvent.click(screen.getByRole("button", { name: "继续查找笔记" }));
  await screen.findByText("已检查 51 个文件 · 1 篇 Markdown");
  fireEvent.click(screen.getByRole("button", { name: "快速打开" }));
  fireEvent.click(screen.getByRole("button", { name: "第一篇.md" }));
  await screen.findByText("固定正文 a");
  fireEvent.change(screen.getByRole("textbox", { name: "筛选已加载笔记" }), { target: { value: "不存在" } });
  expect(screen.getByText("固定正文 a")).toBeTruthy();
});
it("clears loaded notes on context revocation and never retries implicitly", async () => {
  vi.mocked(readKnowledgeFiles).mockResolvedValue({ entries: [entry], nextCursor: "next", scanned: 1, unavailable: 0, stale: 0 });
  render(<KnowledgePage actor="a" spaces={spaces}/>);
  await screen.findByText("已检查 1 个文件 · 1 篇 Markdown");
  fireEvent(window, new Event("drivebridge-context"));
  await screen.findByText("访问权限已失效，笔记内容已清除。请重新登录或切换来源。");
  expect(screen.queryByLabelText("笔记工作台")).toBeNull();
  expect(readKnowledgeFiles).toHaveBeenCalledTimes(1);
});
it("unmounts all private panes when a document reports access revocation", async () => {
  vi.mocked(readKnowledgeFiles).mockResolvedValue({ entries: [entry], nextCursor: null, scanned: 1, unavailable: 0, stale: 0 });
  render(<KnowledgePage actor="a" spaces={spaces}/>);
  await screen.findByText("已检查 1 个文件 · 1 篇 Markdown");
  fireEvent.click(screen.getByRole("button", { name: "快速打开" }));
  fireEvent.click(screen.getByRole("button", { name: "第一篇.md" }));
  fireEvent.click(await screen.findByRole("button", { name: "模拟正文失权" }));
  await screen.findByText("访问权限已失效，笔记内容已清除。请重新登录或切换来源。");
  expect(screen.queryByLabelText("笔记工作台")).toBeNull();
  expect(screen.queryByText("固定正文 a")).toBeNull();
  expect(screen.queryByText('页面树与新建父页面')).toBeNull();
  expect(screen.queryByRole('dialog', { name: '新建笔记' })).toBeNull();
  expect(readKnowledgeFiles).toHaveBeenCalledTimes(1);
});
it("does not treat a cursor expiry as an empty library", async () => {
  vi.mocked(readKnowledgeFiles).mockRejectedValue(Object.assign(new Error("cursor_expired"), { status: 410 }));
  render(<KnowledgePage actor="a" spaces={spaces}/>);
  await screen.findByText("文件目录已经变化，请重新查询，不能拼接不同版本的页面。");
  expect(screen.queryByText("当前来源没有可阅读的 Markdown 文件。")).toBeNull();
  vi.mocked(readKnowledgeFiles).mockResolvedValue({ entries: [], nextCursor: null, scanned: 0, unavailable: 0, stale: 0 });
  fireEvent.click(screen.getByRole("button", { name: "重新查询" }));
  await waitFor(() => expect(readKnowledgeFiles).toHaveBeenLastCalledWith("s", null, expect.any(AbortSignal)));
  await screen.findByText("当前来源没有可阅读的 Markdown 文件。");
});
it('keeps offline distinct from an empty library and discards late directory results', async () => {
  const gate = Promise.withResolvers<{ entries: typeof entry[]; nextCursor: null; scanned: number; unavailable: number; stale: number }>();
  vi.mocked(readKnowledgeFiles).mockReturnValueOnce(gate.promise);
  render(<KnowledgePage actor="a" spaces={spaces}/>);
  await waitFor(() => expect(readKnowledgeFiles).toHaveBeenCalled());
  fireEvent(window, new Event('offline'));
  await screen.findByText(/当前离线，目录仅为上次观察/);
  gate.resolve({ entries: [entry], nextCursor: null, scanned: 1, unavailable: 0, stale: 0 });
  await waitFor(() => expect(screen.queryByText('当前来源没有可阅读的 Markdown 文件。')).toBeNull());
  expect(screen.queryByLabelText('笔记工作台')).toBeNull();
});
