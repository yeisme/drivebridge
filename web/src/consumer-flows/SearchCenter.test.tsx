import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { SearchCenter } from "./SearchCenter";
import { queryFiles } from "../info-workspace/client";
import { setDataContext } from "../api";
import { saveSearch } from "./client";

vi.mock("./client", async importOriginal => ({ ...await importOriginal<typeof import("./client")>(), saveSearch: vi.fn() }));

vi.mock("../info-workspace/client", () => ({ queryFiles: vi.fn() }));
afterEach(() => { cleanup(); vi.resetAllMocks(); vi.restoreAllMocks(); setDataContext(""); });
it("loads saved conditions without executing and preserves their spaces across a retry", async () => {
  const query = { mode: "exact", space_ids: ["saved-space"], filters: [{ field: "path_prefix", op: "contains", value: "report" }], limit: 50 };
  const fetch = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(new Response(JSON.stringify({ data: [{ query_id: "saved", name: "Saved", query, created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-01T00:00:00Z" }] }), { status: 200 }));
  vi.mocked(queryFiles).mockRejectedValueOnce(new Error("unavailable")).mockResolvedValueOnce({ mode: "exact", items: [], next_cursor: null, coverage: "complete" });
  render(<SearchCenter spaces={[]} capability="unsupported"/>);
  expect(fetch).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "查看已保存条件" }));
  fireEvent.click(await screen.findByRole("button", { name: "选用条件 Saved" }));
  expect(queryFiles).not.toHaveBeenCalled(); expect(saveSearch).not.toHaveBeenCalled();
  expect((screen.getByRole("searchbox") as HTMLInputElement).value).toBe("report");
  expect(document.activeElement).toBe(screen.getByRole("searchbox"));
  await act(async () => { fireEvent.click(screen.getByRole("button", { name: "检索" })); });
  await act(async () => { fireEvent.click(screen.getByRole("button", { name: "检索" })); });
  expect(vi.mocked(queryFiles).mock.calls.map(([request]) => request.space_ids)).toEqual([["saved-space"], ["saved-space"]]);
});
it.each([new Error("cursor_expired"), Object.assign(new Error("request_failed"), { status: 410 })])("discards an expired cursor and explicitly restarts with preserved criteria", async error => {
  vi.mocked(queryFiles)
    .mockResolvedValueOnce({ mode: "exact", items: [{ file_id: "old-page", version_id: "v1" }], next_cursor: "expired-cursor", coverage: "complete" })
    .mockRejectedValueOnce(error)
    .mockResolvedValueOnce({ mode: "exact", items: [], next_cursor: null, coverage: "complete" });
  render(<SearchCenter spaces={[]} capability="available" />);
  const input = screen.getByRole("searchbox", { name: "检索条件" }) as HTMLInputElement;
  fireEvent.change(input, { target: { value: "retained-query" } });
  await act(async () => { fireEvent.click(screen.getByRole("button", { name: "检索" })); });
  await act(async () => { fireEvent.click(screen.getByRole("button", { name: "下一页" })); });
  expect(screen.getByRole("alert").textContent).toContain("搜索分页已失效");
  expect(screen.queryByText("old-page")).toBeNull();
  expect(input.value).toBe("retained-query");
  expect(queryFiles).toHaveBeenCalledTimes(2);
  await act(async () => { fireEvent.click(screen.getByRole("button", { name: "检索" })); });
  expect(vi.mocked(queryFiles).mock.calls[2][0]).not.toHaveProperty("cursor");
  expect(vi.mocked(queryFiles).mock.calls[2][0].filters?.[0].value).toBe("retained-query");
});
it("uses the server cursor for the next page and resets it on a new query", async () => {
  vi.mocked(queryFiles)
    .mockResolvedValueOnce({ mode: "exact", items: [{ file_id: "first-page", version_id: "v1" }], next_cursor: "opaque-next", coverage: "complete" })
    .mockResolvedValueOnce({ mode: "exact", items: [{ file_id: "second-page", version_id: "v2" }], next_cursor: null, coverage: "complete" })
    .mockResolvedValueOnce({ mode: "exact", items: [], next_cursor: null, coverage: "complete" });
  render(<SearchCenter spaces={[]} capability="available" />);
  const input = screen.getByRole("searchbox", { name: "检索条件" });
  fireEvent.change(input, { target: { value: "fixture" } });
  await act(async () => { fireEvent.click(screen.getByRole("button", { name: "检索" })); });
  expect(screen.getByText("first-page")).toBeTruthy();
  await act(async () => { fireEvent.click(screen.getByRole("button", { name: "下一页" })); });
  expect(vi.mocked(queryFiles).mock.calls[1][0]).toMatchObject({ cursor: "opaque-next", limit: 50 });
  expect(screen.queryByText("first-page")).toBeNull();
  expect(screen.getByText("second-page")).toBeTruthy();
  expect(screen.queryByRole("button", { name: "下一页" })).toBeNull();
  fireEvent.change(input, { target: { value: "new-query" } });
  await act(async () => { fireEvent.click(screen.getByRole("button", { name: "检索" })); });
  expect(vi.mocked(queryFiles).mock.calls[2][0]).not.toHaveProperty("cursor");
});
it("blocks duplicate saves and suppresses a receipt for changed criteria", async () => {
  let finish!: () => void;
  vi.mocked(saveSearch).mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  render(<SearchCenter spaces={[]} capability="available" />);
  const input = screen.getByRole("searchbox", { name: "检索条件" });
  fireEvent.change(input, { target: { value: "original" } });
  fireEvent.click(screen.getByRole("button", { name: "保存条件" }));
  const saving = screen.getByRole("button", { name: "正在保存…" });
  expect((saving as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(saving);
  expect(saveSearch).toHaveBeenCalledTimes(1);
  fireEvent.change(input, { target: { value: "changed" } });
  await act(async () => { finish(); });
  expect(screen.queryByText(/检索条件已保存/)).toBeNull();
  expect((screen.getByRole("button", { name: "保存条件" }) as HTMLButtonElement).disabled).toBe(false);
  expect(saveSearch).toHaveBeenCalledWith(expect.objectContaining({ query: expect.objectContaining({ mode: "exact", filters: [{ field: "path_prefix", op: "contains", value: "original" }] }) }));
});
it("clears displayed results and criteria when the scope changes without issuing a query", async () => {
  vi.mocked(queryFiles).mockResolvedValue({ mode: "exact", items: [{ file_id: "prior-scope-file", version_id: "version" }], next_cursor: null, coverage: "complete" });
  render(<SearchCenter spaces={[]} capability="available" />);
  const input = screen.getByRole("searchbox", { name: "检索条件" }) as HTMLInputElement;
  fireEvent.change(input, { target: { value: "previous-scope-query" } });
  await act(async () => { fireEvent.click(screen.getByRole("button", { name: "检索" })); });
  expect(screen.getByText("prior-scope-file")).toBeTruthy();
  act(() => setDataContext("other-tenant", "other-space"));
  expect(screen.queryByText("prior-scope-file")).toBeNull();
  expect(input.value).toBe("");
  expect(screen.getByRole("status").textContent).toContain("尚未读取搜索结果");
  expect(queryFiles).toHaveBeenCalledTimes(1);
});
it.each(["text", "mode"])("invalidates a pending query when %s changes", async change => {
  let resolve!: (value: Awaited<ReturnType<typeof queryFiles>>) => void;
  vi.mocked(queryFiles).mockImplementationOnce(() => new Promise(done => { resolve = done; }));
  render(<SearchCenter spaces={[]} capability="available" />);
  const input = screen.getByRole("searchbox", { name: "检索条件" });
  fireEvent.change(input, { target: { value: "original" } });
  fireEvent.click(screen.getByRole("button", { name: "检索" }));
  const signal = vi.mocked(queryFiles).mock.calls[0][1]!;
  if (change === "text") fireEvent.change(input, { target: { value: "changed" } });
  else fireEvent.click(screen.getByRole("radio", { name: "内容发现（Top-K）" }));
  expect(signal.aborted).toBe(true);
  await act(async () => { resolve({ mode: "exact", items: [{ file_id: "stale-result", version_id: "old-version" }], next_cursor: null }); });
  expect(screen.queryByText("stale-result")).toBeNull();
  expect(screen.getByRole("status").textContent).toContain("尚未读取搜索结果");
  expect(queryFiles).toHaveBeenCalledTimes(1);
});
it("shows empty only after a completed query and retains input after failure", async () => {
  let resolve!: (value: Awaited<ReturnType<typeof queryFiles>>) => void;
  vi.mocked(queryFiles).mockImplementationOnce(() => new Promise(done => { resolve = done; }));
  render(<SearchCenter spaces={[]} capability="available" />);
  const input = screen.getByRole("searchbox", { name: "检索条件" }) as HTMLInputElement;
  fireEvent.change(input, { target: { value: "fixture-query" } });
  expect(queryFiles).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "检索" }));
  expect(screen.getByRole("status").textContent).toContain("正在检索");
  await act(async () => { resolve({ mode: "exact", items: [], next_cursor: null, coverage: "complete" }); });
  expect(screen.getByRole("status").textContent).toContain("没有符合条件的结果");
  expect(screen.getByRole("region", { name: "云端检索" }).getAttribute("aria-busy")).toBe("false");
  vi.mocked(queryFiles).mockRejectedValueOnce(new Error("unavailable"));
  await act(async () => { fireEvent.click(screen.getByRole("button", { name: "检索" })); });
  expect(screen.getByRole("alert")).toBeTruthy();
  expect(input.value).toBe("fixture-query");
  expect(screen.queryByText("没有符合条件的结果。")).toBeNull();
});
it.each(["partial", undefined])("does not report complete emptiness with coverage %s", async coverage => {
  vi.mocked(queryFiles).mockResolvedValue({ mode: "exact", items: [], next_cursor: null, coverage });
  render(<SearchCenter spaces={[]} capability="available" />);
  fireEvent.change(screen.getByRole("searchbox", { name: "检索条件" }), { target: { value: "fixture" } });
  await act(async () => { fireEvent.click(screen.getByRole("button", { name: "检索" })); });
  expect(screen.queryByText("没有符合条件的结果。")).toBeNull();
  expect(screen.getByRole("status").textContent).toContain("当前已覆盖范围未返回结果");
  expect(screen.getByText(/不能据此断言没有文件/)).toBeTruthy();
});
it.each(["unsupported", "checking"] as const)("keeps exact metadata independent of content capability %s", async capability => {
  vi.mocked(queryFiles).mockResolvedValue({ mode: "exact", items: [], next_cursor: null, coverage: "complete" });
  render(<SearchCenter spaces={[]} capability={capability}/>);
  fireEvent.change(screen.getByRole("searchbox"), { target: { value: "100%_" } });
  await act(async () => { fireEvent.click(screen.getByRole("button", { name: "检索" })); });
  expect(queryFiles).toHaveBeenCalledWith(expect.objectContaining({ filters: [{ field: "path_prefix", op: "contains", value: "100%_" }] }), expect.any(AbortSignal));
  fireEvent.click(screen.getByRole("radio", { name: "内容发现（Top-K）" }));
  expect((screen.getByRole("button", { name: "检索" }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole("button", { name: "保存条件" }) as HTMLButtonElement).disabled).toBe(true);
  expect(saveSearch).not.toHaveBeenCalled();
});
it("clears revoked results and blocks further search until context revalidation", async () => {
  vi.mocked(queryFiles).mockResolvedValueOnce({ mode: "exact", items: [{ file_id: "private-file", version_id: "v" }], next_cursor: null, coverage: "complete" }).mockRejectedValueOnce(Object.assign(new Error("forbidden"), { status: 403 }));
  render(<SearchCenter spaces={[]} capability="available"/>);
  fireEvent.change(screen.getByRole("searchbox"), { target: { value: "private" } });
  await act(async () => { fireEvent.click(screen.getByRole("button", { name: "检索" })); });
  expect(screen.getByText("private-file")).toBeTruthy();
  await act(async () => { fireEvent.click(screen.getByRole("button", { name: "检索" })); });
  expect(screen.queryByText("private-file")).toBeNull();
  expect(screen.getByRole("alert").textContent).toContain("权限已失效");
  expect((screen.getByRole("button", { name: "检索" }) as HTMLButtonElement).disabled).toBe(true);
});
it("a denied save cancels a concurrent query so its late result cannot restore access", async () => {
  let finish!: (value: Awaited<ReturnType<typeof queryFiles>>) => void;
  vi.mocked(queryFiles).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  vi.mocked(saveSearch).mockRejectedValueOnce(Object.assign(new Error("forbidden"), { status: 403 }));
  render(<SearchCenter spaces={[]} capability="available"/>);
  fireEvent.change(screen.getByRole("searchbox"), { target: { value: "old" } });
  fireEvent.click(screen.getByRole("button", { name: "检索" }));
  await act(async () => { fireEvent.click(screen.getByRole("button", { name: "保存条件" })); });
  await act(async () => { finish({ mode: "exact", items: [{ file_id: "late-private", version_id: "v" }], next_cursor: null }); });
  expect(screen.queryByText("late-private")).toBeNull();
  expect(screen.getByRole("alert").textContent).toContain("权限已失效");
});
it("offline discards a late query and online never retries automatically", async () => {
  let finish!: (value: Awaited<ReturnType<typeof queryFiles>>) => void;
  vi.mocked(queryFiles).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  render(<SearchCenter spaces={[]} capability="available"/>);
  fireEvent.change(screen.getByRole("searchbox"), { target: { value: "old" } });
  fireEvent.click(screen.getByRole("button", { name: "检索" }));
  fireEvent(window, new Event("offline"));
  await act(async () => { finish({ mode: "exact", items: [{ file_id: "late-file", version_id: "v" }], next_cursor: null }); });
  fireEvent(window, new Event("online"));
  expect(screen.queryByText("late-file")).toBeNull(); expect(queryFiles).toHaveBeenCalledTimes(1);
});
it("aborts the active query on unmount even when the client settles late", async () => {
  let reject!: (error: Error) => void;
  vi.mocked(queryFiles).mockImplementation(() => new Promise((_, fail) => { reject = fail; }));
  const view = render(<SearchCenter spaces={[]} capability="available" />);
  expect(screen.getByRole("status").textContent).toContain("尚未读取搜索结果");
  expect(queryFiles).not.toHaveBeenCalled();
  fireEvent.change(screen.getByRole("searchbox", { name: "检索条件" }), { target: { value: "fixture" } });
  fireEvent.click(screen.getByRole("button", { name: "检索" }));
  expect(screen.getByRole("status").textContent).toContain("正在检索");
  expect(screen.getByRole("region", { name: "云端检索" }).getAttribute("aria-busy")).toBe("true");
  expect(screen.queryByText("没有符合条件的结果。")).toBeNull();
  const signal = vi.mocked(queryFiles).mock.calls[0][1]!;
  expect(signal.aborted).toBe(false);
  view.unmount();
  expect(signal.aborted).toBe(true);
  await act(async () => { reject(new Error("late failure")); });
  expect(screen.queryByRole("alert")).toBeNull();
});
