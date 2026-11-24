import { afterEach, expect, it, vi } from "vitest";
import type { BrowserEntry } from "./useDirectoryRead";
import { createFolderSummaryStore } from "./folder-summaries";

afterEach(() => vi.restoreAllMocks());

function controlledReader() {
  const pending: { path: string; signal: AbortSignal; finish: (entries: BrowserEntry[]) => void }[] = [];
  const reader = vi.fn((_space: string, path: string, _query: string, signal: AbortSignal) =>
    new Promise<BrowserEntry[]>((finish) => pending.push({ path, signal, finish })));
  return { reader, pending };
}
const settle = () => new Promise((resolve) => setTimeout(resolve, 0));

it("reads each folder once with at most two reads in flight", async () => {
  const { reader, pending } = controlledReader();
  const store = createFolderSummaryStore("space", reader);
  const listener = vi.fn();
  store.subscribe(listener);
  for (const path of ["a", "b", "c", "a", "b"]) store.want(path);
  expect(reader.mock.calls.map((call) => call[1])).toEqual(["a", "b"]);
  expect(store.get("c").status).toBe("idle");
  pending[0].finish([{ kind: "directory", path: "a/x" }]);
  await settle();
  expect(store.get("a")).toMatchObject({ status: "ready", summary: { folders: 1, sheets: ["folder"] } });
  expect(reader.mock.calls.map((call) => call[1])).toEqual(["a", "b", "c"]);
  expect(listener).toHaveBeenCalled();
  store.want("a");
  expect(reader).toHaveBeenCalledTimes(3);
});

it("aborts in-flight reads on dispose and never publishes late results", async () => {
  const { reader, pending } = controlledReader();
  const store = createFolderSummaryStore("space", reader);
  const listener = vi.fn();
  store.subscribe(listener);
  store.want("a");
  store.want("b");
  store.want("c");
  listener.mockClear();
  store.dispose();
  expect(pending.every((read) => read.signal.aborted)).toBe(true);
  pending[0].finish([]);
  await settle();
  expect(listener).not.toHaveBeenCalled();
  expect(store.get("a").status).toBe("loading");
  expect(reader).toHaveBeenCalledTimes(2);
});

it("marks failed reads as errors without retrying in the same scope", async () => {
  const reader = vi.fn(async () => { throw new Error("forbidden"); });
  const store = createFolderSummaryStore("space", reader);
  store.want("locked");
  await settle();
  expect(store.get("locked").status).toBe("error");
  store.want("locked");
  expect(reader).toHaveBeenCalledTimes(1);
});

it("reads one hosted page by recency and reports partial pages", async () => {
  const fetch = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({
    data: { entries: [{ kind: "directory", path: "big/x" }], next_cursor: "200", coverage: "managed_directory", total: 480 },
  })));
  const store = createFolderSummaryStore("space-1");
  store.want("big");
  await vi.waitFor(() => expect(store.get("big").status).toBe("ready"));
  const url = new URL(String(fetch.mock.calls[0][0]), "http://local");
  expect(url.pathname).toBe("/api/v1/browser");
  expect(Object.fromEntries(url.searchParams)).toEqual({ space_id: "space-1", path: "big", sort: "date", order: "desc" });
  expect(store.get("big")).toMatchObject({ summary: { total: 480, complete: false } });
});
