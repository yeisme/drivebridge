import { act, cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { InfoWorkspace } from "./InfoWorkspace";
import { setDataContext } from "../api";

const envelope = (data: unknown) =>
  new Response(JSON.stringify({ data }), { status: 200, headers: { "Content-Type": "application/json" } });
const failure = (code: string, status = 400) =>
  new Response(JSON.stringify({ error: { code } }), { status, headers: { "Content-Type": "application/json" } });
const plainText = new Response("404 page not found", { status: 404 });

const summaryFixture = (coverage: string[] = ["complete"]) => ({
  collection: { collection_started_at: "2026-09-25T00:00:00Z" },
  totals: { effective_bytes: 10, observed_bytes: 20, completed_operations: 1, failed_attempts: 0 },
  capacities: [
    {
      space_id: "sp-a",
      connection_id: "platform",
      current_bytes: 4096,
      history_bytes: null,
      trash_bytes: 0,
      reserved_bytes: 1024,
      quota_bytes: 1 << 20,
      coverage: "complete",
      observed_at: "2026-09-25T00:00:00Z",
      day: "2026-09-25",
    },
  ],
  coverage,
  observed_total_bytes: 20,
  observed_total_status: "observed",
});
const seriesFixture = [
  { day: "2026-09-24", effective_bytes: 8, observed_bytes: 10 },
  { day: "2026-09-25", effective_bytes: 10, observed_bytes: 20 },
];
const groupFixture = (key: string, kind: string) => ({
  key,
  kind,
  effective_bytes: 10,
  observed_bytes: 20,
  completed_operations: 1,
  failed_attempts: 0,
});
// Implemented live-backend shape (analytics.go AnalyticsQueryView): the
// transactional live path answers query_id/backend:"transactional" and omits
// the analytics-only watermark fields.
const analyticsSample = {
  query_id: "query-fixture-1",
  backend: "transactional",
  observed_at: "2026-09-25T00:00:00Z",
  coverage: "partial",
  partial: true,
  rows: [{ mime_type: "image/png", logical_bytes: 4096, file_count: 1 }],
};

type Handler = (url: string, init?: RequestInit) => Response | Promise<Response>;

function mockFetch(handler: Handler) {
  return vi.spyOn(globalThis, "fetch").mockImplementation(((input: RequestInfo | URL, init?: RequestInit) => {
    const url =
      typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
    return Promise.resolve(handler(url, init));
  }) as typeof fetch);
}

// Usage-backed panels plus an analytics surface that is either absent
// (plain-text 404 from an older deployment) or enabled (contract sample).
function defaultHandler(options?: {
  analytics?: "absent" | "enabled" | Response;
  usage?: (url: string) => Response | undefined;
  fileQuery?: Response;
}) {
  return (url: string, init?: RequestInit): Response | Promise<Response> => {
    if (url.includes("/api/v1/analytics/queries")) {
      if (options?.analytics === "enabled") return envelope(analyticsSample);
      if (options?.analytics && options.analytics !== "absent") return options.analytics;
      return plainText;
    }
    if (url.includes("/api/v1/file-queries")) {
      return options?.fileQuery ?? envelope({ mode: "exact", items: [], next_cursor: null, coverage: "complete" });
    }
    const overridden = options?.usage?.(url);
    if (overridden) return overridden;
    if (url.includes("/usage/summary")) return envelope(summaryFixture());
    if (url.includes("/usage/series")) return envelope(seriesFixture);
    if (url.includes("/usage/breakdown") && url.includes("group=direction"))
      return envelope([groupFixture("client_to_server", "direction")]);
    if (url.includes("/usage/breakdown") && url.includes("group=actor"))
      return envelope([groupFixture("agent-1", "agent")]);
    if (url.includes("/usage/files"))
      return envelope({
        items: [{ resource_id: "f-1", version_id: "v-1", name: "镜头-1.png", size_bytes: 4096 }],
        partial: false,
        coverage: "complete",
      });
    if (url.includes("/usage/events"))
      return envelope({
        events: [
          {
            sequence: 1,
            at: "2026-09-25T00:00:00Z",
            purpose: "upload",
            metric: "effective_bytes",
            bytes: 4096,
            operation_id: "op-1",
            resource_id: "f-1",
          },
        ],
        next_cursor: 0,
      });
    if (url.includes("/api/v1/files/") && url.includes("/versions"))
      return envelope([{ id: "v-1", sha256: "a".repeat(64), size_bytes: 4096, mime_type: "image/png", created_at: "2026-09-25T00:00:00Z" }]);
    if (url.includes("/api/v1/files/"))
      return envelope({
        id: "f-1",
        space_id: "sp-a",
        path: "/镜头-1.png",
        version_id: "v-1",
        size_bytes: 4096,
        mime_type: "image/png",
        trashed: false,
        updated_at: "2026-09-25T00:00:00Z",
        ref: "drivebridge://instance/file/f-1/version/v-1",
      });
    if (url.includes("/api/v1/connections")) return envelope([]);
    if (url.includes("/api/v1/spaces"))
      return envelope([{ id: "sp-a", name: "体验空间", connection_id: "platform", used_bytes: 1, reserved_bytes: 0, max_bytes: 2 }]);
    return failure("not_found", 404);
  };
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  window.location.hash = "";
});

describe("TestFPInformationWorkspace", () => {
  it("renders the six overview panels with scope and observation time", async () => {
    mockFetch(defaultHandler());
    render(<InfoWorkspace />);
    for (const label of ["容量", "增长", "传输", "连接", "整理", "Agent 活动"])
      expect(await screen.findByRole("heading", { name: label, hidden: true })).toBeTruthy();
    // Scope and observation time are stated per panel, not implied.
    expect(screen.getAllByText("当前授权空间").length).toBeGreaterThan(0);
    expect(screen.getAllByText(/观测于 2026-09-25 00:00:00 UTC/).length).toBeGreaterThan(0);
  });

  it("labels partial coverage and never presents it as a full total", async () => {
    mockFetch(defaultHandler({ usage: (url) => (url.includes("/usage/summary") ? envelope(summaryFixture(["partial"])) : undefined) }));
    render(<InfoWorkspace />);
    const capacity = await screen.findByRole("heading", { name: "容量", hidden: true });
    const panel = capacity.closest("[data-panel]")!;
    expect(panel.getAttribute("data-state")).toBe("partial");
    expect(panel.textContent).toContain("部分覆盖");
    expect(panel.textContent).toContain("不代表全盘总量");
    // Unknown history bytes stay unknown instead of being replaced by 0.
    expect(panel.textContent).toContain("未取得");
  });

  it("keeps old deployments usable: analytics absent degrades only the typed panel", async () => {
    mockFetch(defaultHandler({ analytics: "absent" }));
    render(<InfoWorkspace />);
    const organization = (await screen.findByRole("heading", { name: "整理", hidden: true })).closest("[data-panel]")!;
    await waitFor(() => expect(organization.getAttribute("data-state")).toBe("unsupported"));
    expect(organization.textContent).toContain("此部署未启用对应分析能力");
    // Drilldown into the missing surface is disabled with a reason, not faked.
    const drillButton = organization.querySelector<HTMLButtonElement>(".iw-drill-btn")!;
    expect(drillButton.disabled).toBe(true);
    expect(organization.textContent).toContain("后端未提供明细，钻取已禁用");
    // Usage-backed panels keep their data and their drilldown.
    const capacity = (await screen.findByRole("heading", { name: "容量", hidden: true })).closest("[data-panel]")!;
    expect(capacity.getAttribute("data-state")).toBe("ready");
    expect((capacity.querySelector<HTMLButtonElement>(".iw-drill-btn"))!.disabled).toBe(false);
  });

  it("queries live consistency with explicit dimensions and metrics for the precise panel", async () => {
    // Drift D1/D2: omitted consistency defaults to observed (503 analytics_
    // backend_unavailable in this deployment) and empty dimensions+metrics
    // answers 400 invalid_query; the panel and its capability probe must both
    // send the typed vocabulary with consistency:"live".
    const fetcher = mockFetch(defaultHandler({ analytics: "enabled" }));
    render(<InfoWorkspace />);
    const organization = (await screen.findByRole("heading", { name: "整理", hidden: true })).closest("[data-panel]")!;
    await waitFor(() => expect(organization.getAttribute("data-state")).toBe("partial"));
    const bodies = fetcher.mock.calls
      .filter(([url]) => String(url).includes("/api/v1/analytics/queries"))
      .map(([, init]) => JSON.parse(String(init?.body)) as Record<string, unknown>);
    expect(bodies.length).toBeGreaterThanOrEqual(2); // capability probe + panel query
    for (const body of bodies) expect(body.consistency).toBe("live");
    const panel = bodies.find((body) => body.dataset === "file_current")!;
    expect(panel.dimensions).toEqual(["mime_type"]);
    expect(panel.metrics).toEqual(["logical_bytes", "file_count"]);
  });

  it("degrades the analytics gate honestly when the analytics backend is unavailable", async () => {
    // Drift D1: a 503 analytics_backend_unavailable on the observed path means
    // "no analytics backend deployed" — the gate closes as unsupported
    // instead of rendering a retryable error for a capability fact.
    mockFetch(defaultHandler({ analytics: failure("analytics_backend_unavailable", 503) }));
    render(<InfoWorkspace />);
    const organization = (await screen.findByRole("heading", { name: "整理", hidden: true })).closest("[data-panel]")!;
    await waitFor(() => expect(organization.getAttribute("data-state")).toBe("unsupported"));
    expect(organization.textContent).toContain("此部署未启用对应分析能力");
  });

  it("renders typed analytics rows with generation metadata when enabled", async () => {
    mockFetch(defaultHandler({ analytics: "enabled" }));
    render(<InfoWorkspace />);
    const organization = (await screen.findByRole("heading", { name: "整理", hidden: true })).closest("[data-panel]")!;
    await waitFor(() => expect(organization.getAttribute("data-state")).toBe("partial"));
    expect(organization.textContent).toContain("image/png");
    expect(organization.textContent).toContain("部分覆盖");
  });

  it("reports a transient probe failure as a retryable error, not as a disabled capability", async () => {
    // Regression: any non-abort probe error used to latch "unsupported"
    // forever, so a momentary 500 read as "capability not enabled".
    let probeHealthy = false;
    mockFetch(
      defaultHandler({
        analytics: failure("internal_error", 500),
      }),
    ).mockImplementation(((input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
      if (url.includes("/api/v1/analytics/queries")) {
        return Promise.resolve(probeHealthy ? envelope(analyticsSample) : failure("internal_error", 500));
      }
      return Promise.resolve(defaultHandler()(url, init));
    }) as typeof fetch);
    render(<InfoWorkspace />);
    const organization = (await screen.findByRole("heading", { name: "整理", hidden: true })).closest<HTMLElement>("[data-panel]")!;
    await waitFor(() => expect(organization.getAttribute("data-state")).toBe("error"));
    expect(organization.textContent).not.toContain("此部署未启用对应分析能力");
    expect(organization.textContent).toContain("重试");
    // Retry re-runs the probe; once the transient failure clears the panel
    // recovers instead of staying latched.
    probeHealthy = true;
    await userEvent.click(within(organization).getByRole("button", { name: "重试" }));
    await waitFor(() => expect(organization.getAttribute("data-state")).toBe("partial"));
  });

  it("isolates panel failures: a forbidden panel does not fail the page", async () => {
    mockFetch(
      defaultHandler({
        usage: (url) => (url.includes("group=actor") ? failure("forbidden", 403) : undefined),
      }),
    );
    render(<InfoWorkspace />);
    const agent = (await screen.findByRole("heading", { name: "Agent 活动", hidden: true })).closest("[data-panel]")!;
    await waitFor(() => expect(agent.getAttribute("data-state")).toBe("forbidden"));
    expect(agent.textContent).toContain("当前授权不允许查看此范围");
    const capacity = screen.getByRole("heading", { name: "容量", hidden: true }).closest("[data-panel]")!;
    expect(capacity.getAttribute("data-state")).toBe("ready");
  });

  it("drills down with the same filters and opens the pinned file detail", async () => {
    window.location.hash = "#/overview?space=sp-a&from=2026-09-24&to=2026-09-25";
    const fetcher = mockFetch(defaultHandler());
    render(<InfoWorkspace />);
    const capacity = (await screen.findByRole("heading", { name: "容量", hidden: true })).closest("[data-panel]")!;
    await userEvent.click(capacity.querySelector<HTMLButtonElement>(".iw-drill-btn")!);
    const drill = await screen.findByRole("region", { name: "筛选明细" });
    // Same caliber: the drill query carries the panel's space and window.
    const filesCall = fetcher.mock.calls
      .map(([input]) => (typeof input === "string" ? input : ""))
      .find((url) => url.includes("/usage/files"))!;
    expect(filesCall).toContain("space_id=sp-a");
    expect(filesCall).toContain("from=2026-09-24");
    expect(filesCall).toContain("to=2026-09-25");
    expect(drill.textContent).toContain("镜头-1.png");
    const open = await screen.findByRole("button", { name: "打开详情" });
    await userEvent.click(open);
    expect(await screen.findByText(/文件详情：镜头-1\.png/)).toBeTruthy();
  });

  it("keeps a dependency outage as an explicitly stale last observation", async () => {
    let analyticsAvailable = true;
    mockFetch(
      defaultHandler({
        analytics: "enabled",
        usage: () => undefined,
      }),
    ).mockImplementation(((input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
      if (url.includes("/api/v1/analytics/queries")) {
        return Promise.resolve(analyticsAvailable ? envelope(analyticsSample) : failure("dependency_unavailable", 503));
      }
      return Promise.resolve(defaultHandler()(url, init));
    }) as typeof fetch);
    render(<InfoWorkspace />);
    const organization = (await screen.findByRole("heading", { name: "整理", hidden: true })).closest("[data-panel]")!;
    await waitFor(() => expect(organization.getAttribute("data-state")).toBe("partial"));
    analyticsAvailable = false;
    await userEvent.click(screen.getByRole("button", { name: "刷新" }));
    await waitFor(() => expect(organization.getAttribute("data-state")).toBe("stale"));
    expect(organization.textContent).toContain("上次观测");
    expect(organization.textContent).toContain("非实时数据");
  });

  it("preserves user filters when a refresh fails", async () => {
    window.location.hash = "#/overview?space=sp-a&from=2026-09-24&to=2026-09-25";
    let healthy = true;
    const fetcher = mockFetch(((input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
      if (!healthy && url.includes("/usage/")) return Promise.resolve(failure("request_failed", 500));
      return Promise.resolve(defaultHandler()(url, init));
    }) as typeof fetch);
    render(<InfoWorkspace />);
    await waitFor(() => expect(document.querySelector('[data-panel="capacity"]')?.textContent).toContain("观测于"));
    healthy = false;
    await userEvent.click(screen.getByRole("button", { name: "刷新" }));
    await waitFor(() => expect(fetcher.mock.calls.length).toBeGreaterThan(6));
    // The unsent window and space selection survive the failed refresh.
    expect(screen.getByText(/当前口径：体验空间/)).toBeTruthy();
    expect((screen.getByLabelText("观测开始日期") as HTMLInputElement).value).toBe("2026-09-24");
    expect((screen.getByLabelText("观测结束日期") as HTMLInputElement).value).toBe("2026-09-25");
  });

  it("drops late responses from a previous context instead of rendering them", async () => {
    // Deferred batches: every fetch of one load cycle is held until the test
    // releases it, so old-context responses can arrive after a tenant switch.
    const batches: { url: string; resolve: (response: Response) => void }[][] = [];
    let current = -1;
    mockFetch(((input: RequestInfo | URL) => {
      const url = typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
      return new Promise<Response>((resolve) => {
        batches[current].push({ url, resolve });
      });
    }) as typeof fetch);
    const startBatch = () => {
      current += 1;
      batches.push([]);
    };
    const respond = (index: number, spaceId: string) => {
      for (const entry of batches[index]) {
        if (entry.url.includes("/usage/summary"))
          entry.resolve(
            envelope({
              ...summaryFixture(),
              capacities: [{ ...summaryFixture().capacities[0], space_id: spaceId }],
            }),
          );
        else if (entry.url.includes("/usage/series")) entry.resolve(envelope(seriesFixture));
        else if (entry.url.includes("/usage/breakdown")) entry.resolve(envelope([groupFixture("client_to_server", "direction")]));
        else if (entry.url.includes("/api/v1/connections")) entry.resolve(envelope([]));
        else if (entry.url.includes("/api/v1/analytics/queries")) entry.resolve(plainText);
        else entry.resolve(envelope([{ id: spaceId, name: "空间 " + spaceId, connection_id: "platform", used_bytes: 1, reserved_bytes: 0, max_bytes: 2 }]));
      }
    };
    startBatch();
    render(<InfoWorkspace />);
    expect(batches[0].length).toBeGreaterThanOrEqual(5);
    // Open the next batch before the switch so the post-switch load lands in
    // batch 1; the initial load cycle keeps racing the switch in batch 0.
    startBatch();
    act(() => setDataContext("tenant-b", "sp-b"));
    // Old-context responses (space sp-a) arrive after the switch.
    respond(0, "sp-a");
    await act(async () => {
      await Promise.resolve();
    });
    const panels = document.querySelectorAll("[data-panel]");
    expect(panels.length).toBe(6);
    panels.forEach((panel) => expect(panel.textContent).not.toContain("sp-a"));
    // The new context loads and renders its own space only.
    respond(1, "sp-b");
    await waitFor(() => {
      expect(document.querySelector('[data-panel="capacity"]')?.textContent).toContain("sp-b");
    });
    panels.forEach((panel) => expect(panel.textContent).not.toContain("sp-a"));
  });

  it("returns focus to the invoking row after closing the detail sheet", async () => {
    mockFetch(defaultHandler());
    render(<InfoWorkspace />);
    const capacity = (await screen.findByRole("heading", { name: "容量", hidden: true })).closest("[data-panel]")!;
    await userEvent.click(capacity.querySelector<HTMLButtonElement>(".iw-drill-btn")!);
    const open = await screen.findByRole("button", { name: "打开详情" });
    open.focus();
    await userEvent.click(open);
    expect(await screen.findByText(/文件详情：镜头-1\.png/)).toBeTruthy();
    // Close via the sheet's accessible close control; focus returns to the
    // row action that opened the detail, per the keyboard scenario.
    await userEvent.click(await screen.findByRole("button", { name: "关闭" }));
    await waitFor(() => expect(screen.queryByText(/文件详情：/)).toBeNull());
    await waitFor(() => expect(document.activeElement).toBe(open));
  });

  it("mounts at most one page of a ten-thousand-row batch", async () => {
    const items = Array.from({ length: 10000 }, (_, index) => ({
      file_id: "f" + String(index).padStart(6, "0"),
      version_id: "v" + index,
    }));
    mockFetch(
      defaultHandler({
        analytics: "enabled",
        fileQuery: envelope({
          mode: "exact",
          items,
          catalogue_revision: "rev-1",
          next_cursor: null,
          coverage: "complete",
          total_count: 10000,
        }),
      }),
    );
    window.location.hash = "#/overview?space=sp-a";
    render(<InfoWorkspace />);
    const organization = (await screen.findByRole("heading", { name: "整理", hidden: true })).closest("[data-panel]")!;
    await waitFor(() => expect(organization.querySelector<HTMLButtonElement>(".iw-drill-btn")!.disabled).toBe(false));
    await userEvent.click(organization.querySelector<HTMLButtonElement>(".iw-drill-btn")!);
    await screen.findByRole("region", { name: "筛选明细" });
    const table = await screen.findByRole("region", { name: "筛选明细" });
    await waitFor(() => {
      const mounted = table.querySelectorAll("tbody tr").length;
      expect(mounted).toBeGreaterThan(0);
      expect(mounted).toBeLessThanOrEqual(200);
    });
  });

  it("shares only safe filter values through the URL", async () => {
    mockFetch(defaultHandler());
    window.location.hash = "#/overview?space=sp-a&from=2026-09-24";
    render(<InfoWorkspace />);
    await waitFor(() => expect(document.querySelector('[data-panel="capacity"]')?.textContent).toContain("观测于"));
    const query = new URLSearchParams(window.location.hash.split("?")[1] || "");
    expect(query.get("space")).toBe("sp-a");
    expect(window.location.hash).not.toContain("token");
    expect(window.location.hash).toContain("from=2026-09-24");
  });
});
