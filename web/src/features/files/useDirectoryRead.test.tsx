import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { useDirectoryRead, type BrowserEntry } from "./useDirectoryRead";

afterEach(() => { cleanup(); vi.useRealTimers(); vi.restoreAllMocks(); });
it("aborts an old scope and ignores its late directory result", async () => {
  vi.useFakeTimers();
  const pending: { signal: AbortSignal; finish: (entries: BrowserEntry[]) => void }[] = [];
  const reader = vi.fn((_space: string, _path: string, _query: string, signal: AbortSignal) => new Promise<BrowserEntry[]>(finish => pending.push({ signal, finish })));
  const onPage = vi.fn(), onFinish = vi.fn();
  const callbacks = { onStart: vi.fn(), onPage, onError: vi.fn(), onFinish };
  const view = renderHook(({ space }) => useDirectoryRead({ space, prefix: "", query: "", cursor: "", sort: "name", order: "asc", revision: 0, reader, ...callbacks }), { initialProps: { space: "a" } });
  await act(async () => { await vi.advanceTimersByTimeAsync(0); });
  view.rerender({ space: "b" });
  await act(async () => { await vi.advanceTimersByTimeAsync(0); });
  expect(pending[0].signal.aborted).toBe(true);
  await act(async () => { pending[0].finish([{ kind: "directory", path: "old" }]); });
  expect(onPage).not.toHaveBeenCalled();
  expect(onFinish).not.toHaveBeenCalled();
  await act(async () => { pending[1].finish([{ kind: "directory", path: "current" }]); });
  expect(onPage).toHaveBeenCalledWith({ entries: [{ kind: "directory", path: "current" }], next_cursor: "", total: 1 });
  view.unmount();
  expect(pending[1].signal.aborted).toBe(true);
});
it("clears the page when scope disappears without reading an empty scope", async () => {
  vi.useFakeTimers();
  let finish!: (entries: BrowserEntry[]) => void;
  let signal!: AbortSignal;
  const reader = vi.fn((_space: string, _path: string, _query: string, value: AbortSignal) => {
    signal = value; return new Promise<BrowserEntry[]>(resolve => { finish = resolve; });
  });
  const onPage = vi.fn(), onFinish = vi.fn();
  const view = renderHook(({ space }) => useDirectoryRead({ space, prefix: "", query: "", cursor: "", sort: "name", order: "asc", revision: 0, reader, onPage, onFinish, onStart: vi.fn(), onError: vi.fn() }), { initialProps: { space: "original" } });
  await act(async () => { await vi.advanceTimersByTimeAsync(0); });
  view.rerender({ space: "" });
  expect(signal.aborted).toBe(true);
  expect(onPage).toHaveBeenCalledWith({ entries: [], next_cursor: "", total: 0 });
  expect(onFinish).toHaveBeenCalledTimes(1);
  await act(async () => { finish([{ kind: "directory", path: "stale" }]); await vi.advanceTimersByTimeAsync(200); });
  expect(reader).toHaveBeenCalledTimes(1);
  expect(onPage).toHaveBeenCalledTimes(1);
});
it("sends the original directory paging and sorting contract", async () => {
  vi.useFakeTimers();
  const fetcher = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({ data: { entries: [], total: 0, next_cursor: "next", coverage: "complete" } })));
  const onPage = vi.fn();
  renderHook(() => useDirectoryRead({ space: "a", prefix: "docs/one", query: "needle", cursor: "opaque", sort: "date", order: "desc", revision: 0, onPage, onStart: vi.fn(), onFinish: vi.fn(), onError: vi.fn() }));
  await act(async () => { await vi.advanceTimersByTimeAsync(199); });
  expect(fetcher).not.toHaveBeenCalled();
  await act(async () => { await vi.advanceTimersByTimeAsync(1); });
  const url = new URL(String(fetcher.mock.calls[0][0]), location.origin);
  expect(Object.fromEntries(url.searchParams)).toEqual({ space_id: "a", path: "docs/one", q: "needle", cursor: "opaque", sort: "date", order: "desc" });
  expect(onPage).toHaveBeenCalledWith(expect.objectContaining({ next_cursor: "next" }));
});
