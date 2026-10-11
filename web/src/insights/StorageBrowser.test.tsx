import React from "react";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { StorageBrowser } from "./StorageBrowser";
import { InsightsPanel } from "./Insights";

afterEach(cleanup);

const SPACE = { id: "nas", name: "nas", connection_id: "nas", backend: "s3" };
const LIVE_ITEM = {
  ref: "drivebridge://file/aa",
  name: "a.bin",
  dir: "",
  size_bytes: 10,
  mime_type: "text/plain",
  observed_at: "2026-09-14T08:41:20Z",
};
const INDEX_SUBDIR_ITEM = {
  ref: "drivebridge://file/bb",
  name: "b.md",
  dir: "sub",
  size_bytes: 5,
  mime_type: "text/markdown",
  observed_at: "2026-09-13T08:41:20Z",
};

interface Route {
  match: (url: string, method: string) => boolean;
  status?: number;
  body: unknown;
}
function mockRoutes(routes: Route[]) {
  const calls: { url: string; method: string; body?: string }[] = [];
  const spy = vi.spyOn(globalThis, "fetch").mockImplementation(
    async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method || "GET";
      calls.push({ url, method, body: init?.body ? String(init.body) : undefined });
      for (const route of routes) {
        if (route.match(url, method)) {
          return new Response(JSON.stringify(route.body), {
            status: route.status ?? 200,
          });
        }
      }
      return new Response(JSON.stringify({ error: { code: "not_mocked" } }), {
        status: 500,
      });
    },
  );
  return { spy, calls };
}
const liveList = (status: number, data: unknown) => ({
  match: (url: string, method: string) =>
    url.endsWith("/local/list") && method === "POST",
  status,
  body: status === 200 ? { data } : { error: { code: String(data) } },
});
const indexFiles = (items: unknown[], partial = false) => ({
  match: (url: string, method: string) =>
    url.startsWith("/api/v1/usage/files") && method === "GET",
  body: { data: { items, partial, coverage: "observed_index_verify_with_stat" } },
});

it("shows a live directory and aggregates observed subdirectories", async () => {
  mockRoutes([
    liveList(200, { items: [LIVE_ITEM], partial: false, coverage: "live_listing" }),
    indexFiles([INDEX_SUBDIR_ITEM]),
  ]);
  render(<StorageBrowser spaces={[SPACE]} />);
  expect(await screen.findByText("a.bin")).toBeTruthy();
  expect(screen.getByText("sub/")).toBeTruthy();
  expect(screen.getByText(/实时目录 · 列表会更新已观察索引/)).toBeTruthy();
});

it("falls back to the observed index with the honest owner reason", async () => {
  mockRoutes([
    liveList(503, "insights_owner_unavailable"),
    indexFiles([{ ...LIVE_ITEM, name: "old.bin", ref: undefined }]),
  ]);
  render(<StorageBrowser spaces={[SPACE]} />);
  expect(await screen.findByText("old.bin")).toBeTruthy();
  expect(
    screen.getByText(/未检测到运行中的 owner（drivebridge mcp --enabled --insights）/),
  ).toBeTruthy();
  expect(screen.getByText(/已观察索引，非全盘/)).toBeTruthy();
});

it("explains when the owner runs without enabled file operations", async () => {
  mockRoutes([liveList(400, "listing_disabled"), indexFiles([])]);
  render(<StorageBrowser spaces={[SPACE]} />);
  expect(
    await screen.findByText(/实时浏览需 owner 启用文件操作（--enabled）/),
  ).toBeTruthy();
});

it("navigates into an observed subdirectory with a scoped live request", async () => {
  const { calls } = mockRoutes([
    liveList(200, { items: [LIVE_ITEM], partial: false, coverage: "live_listing" }),
    indexFiles([INDEX_SUBDIR_ITEM]),
  ]);
  render(<StorageBrowser spaces={[SPACE]} />);
  fireEvent.click(await screen.findByText("sub/"));
  await waitFor(() => {
    const list = calls.filter((c) => c.url.endsWith("/local/list"));
    expect(list.some((c) => c.body === JSON.stringify({ space_id: "nas", path: "sub" }))).toBe(
      true,
    );
  });
});

it("rejects traversal input before any request", async () => {
  const { calls } = mockRoutes([
    liveList(200, { items: [], partial: false, coverage: "live_listing" }),
    indexFiles([]),
  ]);
  render(<StorageBrowser spaces={[SPACE]} />);
  await screen.findByRole("status");
  const input = screen.getByLabelText("前往目录");
  fireEvent.change(input, { target: { value: "../escape" } });
  fireEvent.keyDown(input, { key: "Enter" });
  expect(screen.getByText(/路径无效/)).toBeTruthy();
  expect(calls.filter((c) => c.url.endsWith("/local/list")).length).toBe(1);
});

it("reaches the storage tab by keyboard wrap-around in the local panel", async () => {
  const summary = {
    metric_version: "1",
    collection: {
      collection_started_at: "2026-09-01T00:00:00Z",
      gap_detected: false,
      last_pruned_at: "0001-01-01T00:00:00Z",
    },
    totals: {
      effective_bytes: 0,
      observed_bytes: 0,
      completed_operations: 0,
      failed_attempts: 0,
    },
    capacities: [],
    scans: [],
    scan_policies: [],
    alerts: [],
    observed_total_bytes: null,
    observed_total_status: "not_available",
    coverage: [],
  };
  mockRoutes([
    {
      match: (url, method) => url.includes("/usage/summary") && method === "GET",
      body: { data: summary },
    },
    {
      match: (url, method) => url.includes("/usage/series") && method === "GET",
      body: { data: [] },
    },
    {
      match: (url, method) =>
        url.startsWith("/api/v1/usage/breakdown") && method === "GET",
      body: { data: [] },
    },
    {
      match: (url, method) => url.endsWith("/spaces") && method === "GET",
      body: { data: [SPACE] },
    },
    liveList(200, { items: [LIVE_ITEM], partial: false, coverage: "live_listing" }),
    indexFiles([]),
  ]);
  render(<InsightsPanel local />);
  const audit = await screen.findByRole("tab", { name: "安全审计" });
  fireEvent.keyDown(audit, { key: "ArrowRight" });
  const storage = await screen.findByRole("tab", { name: "存储浏览" });
  expect(storage.getAttribute("aria-selected")).toBe("true");
  expect(await screen.findByText("a.bin")).toBeTruthy();
});
