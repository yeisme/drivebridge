import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ConsumerFlows, type CenterId } from "./ConsumerFlows";
import { memoryRecoveryStore, type RecoveryStore } from "./compat";
import { setDataContext, type Space, type UserIdentity } from "../api";
const locks = Object.getOwnPropertyDescriptor(navigator, 'locks');
// TestFPPlatformConsumers: page entry suite. The human entries are wired to
// the real typed client behind per-center capability gates; old routes are
// untouched (covered by the workspace suites).

const envelope = (data: unknown) =>
  new Response(JSON.stringify({ data }), { status: 200, headers: { "Content-Type": "application/json" } });
const failure = (code: string, status = 400) =>
  new Response(JSON.stringify({ error: { code } }), { status, headers: { "Content-Type": "application/json" } });
const plainText404 = new Response("404 page not found", { status: 404 });

const user: UserIdentity = { id: "user-a", name: "Alice" };
const spaces: Space[] = [{ id: "sp-1", name: "空间一", connection_id: "platform", used_bytes: 0, reserved_bytes: 0, max_bytes: 100 } as Space];

function mockFetch(handler: (url: string, init?: RequestInit) => Response | Promise<Response>) {
  return vi.spyOn(globalThis, "fetch").mockImplementation(((input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
    return Promise.resolve(handler(url, init));
  }) as typeof fetch);
}

// Lookup answers found:false for every probe key: all four centers available.
function allAvailableHandler(extra?: (url: string, init?: RequestInit) => Response | Promise<Response> | undefined) {
  return (url: string, init?: RequestInit): Response | Promise<Response> => {
    const override = extra?.(url, init);
    if (override) return override;
    if (url.includes("/api/v1/operations/lookup")) return envelope({ found: false });
    if (url.includes('/files/f-1')) return envelope({ ref: 'drivebridge://fixture/file/f-1/version/v-1', file: { id: 'f-1', space_id: 'sp-1', version_id: 'v-1' } });
    if (url.includes('/references?')) return envelope({ ref: 'drivebridge://fixture/file/f-1/version/v-1', file: { id: 'f-1', space_id: 'sp-1', version_id: 'v-1' } });
    if (url.includes("/api/v1/spaces")) return envelope(spaces);
    return failure("not_found", 404);
  };
}

function renderFlows(over?: { initialCenter?: CenterId; store?: RecoveryStore }) {
  Object.defineProperty(navigator, 'locks', { configurable: true, value: { request: async (_name: string, options: LockOptions | (() => unknown), callback?: () => unknown) => typeof options === 'function' ? options() : callback!() } });
  return render(<ConsumerFlows user={user} spaces={spaces} initialCenter={over?.initialCenter} store={over?.store ?? memoryRecoveryStore()}/>);
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  localStorage.clear();
  setDataContext("", "");
  if (locks) Object.defineProperty(navigator, 'locks', locks); else Reflect.deleteProperty(navigator, 'locks');
});

describe("TestFPPlatformConsumers/page", () => {
  it("synchronizes route-driven center changes without reprobing or issuing business writes", async () => {
    const fetcher = mockFetch(allAvailableHandler());
    const store = memoryRecoveryStore();
    const changed = vi.fn();
    const view = render(<ConsumerFlows user={user} spaces={spaces} initialCenter="upload" store={store} onCenterChange={changed} />);
    await waitFor(() => expect(fetcher.mock.calls.filter(([url]) => String(url).includes("/operations/lookup"))).toHaveLength(4));
    view.rerender(<ConsumerFlows user={user} spaces={spaces} initialCenter="search" store={store} onCenterChange={changed} />);
    await waitFor(() => expect(screen.getByRole("tab", { name: "云端检索" }).getAttribute("aria-selected")).toBe("true"));
    view.rerender(<ConsumerFlows user={user} spaces={spaces} initialCenter="upload" store={store} onCenterChange={changed} />);
    await waitFor(() => expect(screen.getByRole("tab", { name: "上传中心" }).getAttribute("aria-selected")).toBe("true"));
    expect(changed).not.toHaveBeenCalled();
    expect(fetcher.mock.calls.filter(([url]) => String(url).includes("/operations/lookup"))).toHaveLength(4);
    expect(fetcher.mock.calls.filter(([url, init]) => init?.method === "POST" && !String(url).includes("/operations/lookup"))).toHaveLength(0);
  });
  it("offers the four centers with the upload center first", async () => {
    mockFetch(allAvailableHandler());
    renderFlows();
    for (const label of ["上传中心", "云端检索", "整理中心", "交付与讨论"]) {
      expect(screen.getByRole("tab", { name: label })).toBeTruthy();
    }
    // The upload center is the default entry and renders its honest
    // no-IndexedDB note in the test environment.
    await waitFor(() => expect(screen.getByRole("tabpanel", { name: "上传中心" })).toBeTruthy());
  });

  it("degrades each center honestly when the deployment has not enabled the surface", async () => {
    // Older deployment: the lookup route itself is absent (plain-text 404).
    mockFetch((url) => (url.includes("/api/v1/operations/lookup") ? plainText404 : envelope(spaces)));
    renderFlows();
    await waitFor(() => expect(screen.getByText("此部署未启用对应能力")).toBeTruthy());
    expect(screen.getByText("upload_batch_create")).toBeTruthy();
    await userEvent.click(screen.getByRole("tab", { name: "云端检索" }));
    expect(screen.getByRole("searchbox", { name: "检索条件" })).toBeTruthy();
    expect((screen.getByRole("button", { name: "检索" }) as HTMLButtonElement).disabled).toBe(false);
    await userEvent.click(screen.getByRole("radio", { name: "内容发现（Top-K）" }));
    await waitFor(() => expect(screen.getByText("content_query")).toBeTruthy());
    // The old upload path stays available: the message says so.
    expect(screen.getByText(/既有上传、检索与分享入口不受影响/)).toBeTruthy();
  });

  it("recovers an upload by the original key: reselect mismatch keeps verified progress", async () => {
    const store = memoryRecoveryStore();
    await store.put({
      version: 2,
      identity: { instance: location.origin, actor: "user-a", space: "sp-1" },
      action: "upload_batch_create",
      idempotency_key: "key-recover",
      original_request: { space_id: "sp-1", conflict_policy: "fail" },
      batch_id: "batch-9",
      saved_at: "2026-09-25T00:00:00Z",
    });
    setDataContext("", "sp-1");
    mockFetch(
      allAvailableHandler((url) =>
        // The lookup finds the original key → read the original operation.
        url.includes("/api/v1/operations/lookup")
          ? envelope({ found: true, batch_id: "batch-9", operation_id: "op-77", result_certainty: "confirmed", recovery_action: "read_original_operation" })
          : url.includes("/api/v1/upload-batches/batch-9/items")
            ? envelope({ batch_id: "batch-9", batch_revision: 2, phase: "transferring", items_total: 1, items_completed: 0, items_failed: 0, items_waiting: 1, next_cursor: null, items: [{ item_key: "k", relative_path: "a.txt", operation_id: "op-77", result_certainty: "confirmed", source_fingerprint: "f".repeat(64) }] })
            : undefined,
      ),
    );
    renderFlows({ store });
    await userEvent.click(await screen.findByRole("button", { name: "按原键恢复" }));
    const reselect = await screen.findByRole("region", { name: "上传中心" });
    await waitFor(() => expect(reselect.textContent).toContain("重新选择原文件"));
    // A re-selected file whose digest differs is content_mismatch: verified
    // progress is kept and the original task is never re-keyed.
    const file = new File(["different"], "a.txt", { type: "text/plain" });
    Object.defineProperty(file, "webkitRelativePath", { value: "docs/a.txt", configurable: true });
    await userEvent.upload(screen.getByLabelText("重新选择原文件"), [file]);
    await waitFor(() => expect(screen.getByText("已重新计算摘要，请校验并继续原任务。")).toBeTruthy());
    await userEvent.click(screen.getByRole("button", { name: "校验并继续原任务" }));
    // Digest check runs client-side; the mismatch is reported, not uploaded.
    await waitFor(() => expect(screen.getByText(/已验证的进度保留/)).toBeTruthy());
  });

  it("groups semantic Top-K candidates with caliber chips and drops late context responses", async () => {
    setDataContext("", "sp-1");
    // The response resolves only after the context revision moved: the late
    // payload must never render under the new space context.
    let releaseContent: ((value: Response) => void) | undefined;
    mockFetch(
      allAvailableHandler((url) =>
        url.includes("/api/v1/content-queries")
          ? new Promise<Response>((resolve) => {
              releaseContent = resolve;
            })
          : undefined,
      ),
    );
    renderFlows({ initialCenter: "search" });
    await userEvent.type(await screen.findByLabelText("检索条件"), "雨天街道的镜头");
    await userEvent.click(screen.getByRole("radio", { name: "内容发现（Top-K）" }));
    const textModality = screen.getByRole("checkbox", { name: "text" });
    await userEvent.click(textModality);
    expect(textModality.getAttribute("aria-checked")).toBe("true");
    await userEvent.click(textModality);
    expect(textModality.getAttribute("aria-checked")).toBe("false");
    await userEvent.click(screen.getByRole("button", { name: "检索" }));
    setDataContext("", "sp-2"); // Space switch: revision bumps.
    releaseContent?.(
      envelope({
        mode: "discovery",
        coverage: "partial",
        items: [{ file_ref: "FIXED_FILE_REF", truth_level: "inferred", index_generation: "GEN_ID" }],
        missing: ["audio"],
        next_cursor: null,
      }),
    );
    const panel = await screen.findByRole("tabpanel", { name: "云端检索" });
    await waitFor(() => {
      expect(panel.textContent).not.toContain("FIXED_FILE_REF");
      expect(panel.textContent).not.toContain("内容候选");
    });
  });

  it("requires a fresh preview when the plan went stale and never auto-reconfirms", async () => {
    setDataContext("", "sp-1");
    mockFetch(
      allAvailableHandler((url) => {
        if (url.includes("/api/v1/organization-plans") && url.includes("/execute"))
          return failure("plan_stale", 409);
        if (url.includes("/api/v1/organization-plans"))
          return envelope({
            plan_id: "PLAN_ID",
            revision: 2,
            phase: "previewed",
            risk: "policy_scoped",
            approval_required: false,
            policy_revision: 4,
            items_total: 1,
            impact_digest: "IMPACT_SHA256",
            items: [{ item_key: "k", file_id: "f-1", version_id: "v-1", action: "add_label", before: "/a", after: "/a", state: "pending" }],
          });
        if (url.includes("/api/v1/file-collections"))
          return url.includes("/snapshots") ? envelope({ snapshot_id: "SNAP_ID", item_count: 1 }) : envelope({ collection_id: "COLL_ID", revision: 1 });
        if (url.includes("/api/v1/file-queries"))
          return envelope({ mode: "exact", items: [{ file_id: "f-1", version_id: "v-1" }], coverage: "complete", total_count: 1, next_cursor: null });
        if (url.includes("/api/v1/spaces/sp-1/organization-policy"))
          return envelope({ policy_id: "POLICY_ID", space_id: "sp-1", actions: ["add_label"], revision: 4, enabled: true });
        return undefined;
      }),
    );
    renderFlows({ initialCenter: "organize" });
    await userEvent.type(await screen.findByLabelText("目录前缀"), "docs/");
    await userEvent.click(screen.getByRole("button", { name: "固定所选文件" }));
    await waitFor(() => expect(screen.getByText(/已固定 1 个文件版本/)).toBeTruthy());
    await userEvent.click(screen.getByRole("button", { name: "生成标签整理计划" }));
    await waitFor(() => expect(screen.getByText(/PLAN_ID/)).toBeTruthy());
    await userEvent.click(screen.getByRole("button", { name: "确认并执行" }));
    // plan_stale: the banner demands a fresh preview; no automatic retry of
    // the confirmed revision happens.
    await waitFor(() => expect(screen.getByText(/计划在预览后已发生变化/)).toBeTruthy());
    const confirmCalls = vi
      .spyOn(globalThis, "fetch")
      .mock.calls.filter(([input]) => String(input).includes("/confirm"));
    expect(confirmCalls).toHaveLength(1);
    await userEvent.click(screen.getByRole('button', { name: '释放已拒绝请求并重新预览' }));
    await screen.findByText(/原请求已明确拒绝/);
    expect(screen.getByRole('button', { name: '固定所选文件' })).toHaveProperty('disabled', false);
    expect(Object.values(localStorage).some(value => value.includes('organize_apply'))).toBe(false);
  });

  it("disables publishing incomplete deliveries and keeps the discussion draft on failure", async () => {
    setDataContext("", "sp-1");
    let threadCalls = 0;
    mockFetch(
      allAvailableHandler((url, init) => {
        if (url.includes("/delivery-manifests") && url.includes("/publish"))
          return failure("delivery_incomplete", 409);
        if (url.includes("/delivery-manifests"))
          return envelope({ share_id: "SHARE_ID", manifest_revision: 1, phase: "verifying", required_items: 10, verified_items: 9, complete: false, retention_ready: true });
        if (url.includes("/api/v1/shares"))
          return envelope([{ id: "SHARE_ID", name: "交付一批", revision: 3, revoked: false }]);
        if (url.includes('/file-collections?')) return envelope([{ collection_id: 'COLLECTION_ID', name: 'Collection', space_id: 'sp-1' }]);
        if (url.includes('/file-collections/COLLECTION_ID/snapshots')) return envelope({ collection_id: 'COLLECTION_ID', observed_at: '2026-01-01T00:00:00Z', items: [{ snapshot_id: 'a'.repeat(48), collection_revision: 1, created_at: '2026-01-01T00:00:00Z' }] });
        if (url.includes("/files/f-1/threads") && init?.method === "POST") {
          threadCalls++;
          return failure("revision_conflict", 409);
        }
        if (url.includes("/files/f-1/threads")) return envelope([]);
        if (url.includes("/private-note")) return failure("not_found", 404);
        return undefined;
      }),
    );
    renderFlows({ initialCenter: "deliver" });
    const deliver = await screen.findByRole("tabpanel", { name: "交付与讨论" });
    await userEvent.click(screen.getByRole('button', { name: '读取分享与集合来源' }));
    await userEvent.click(await screen.findByLabelText('选择分享'));
    await userEvent.click(await screen.findByRole('option', { name: '交付一批 · 修订 3' }));
    await userEvent.click(screen.getByLabelText('交付来源集合')); await userEvent.click(await screen.findByRole('option', { name: 'Collection' }));
    await userEvent.click(await screen.findByLabelText('交付固定快照')); await userEvent.click(await screen.findByRole('option', { name: /修订 1/ }));
    await userEvent.click(screen.getByRole('button', { name: '准备清单' }));
    await waitFor(() => expect(screen.getByText(/交付缺 1 项/)).toBeTruthy());
    expect((screen.getByRole("button", { name: "发布完整清单" }) as HTMLButtonElement).disabled).toBe(true);
    expect(screen.getAllByText(/不能发布不完整集合/).length).toBeGreaterThan(1);
    // A failed discussion post keeps the draft.
    await userEvent.type(within(deliver).getByLabelText("文件 ID"), "f-1");
    await userEvent.type(within(deliver).getByLabelText("讨论内容"), "请确认这个版本的标题。");
    await userEvent.click(screen.getByRole("button", { name: "发布讨论" }));
    await waitFor(() =>
      expect(within(deliver).getAllByRole("alert").some((node) => node.textContent?.includes("草稿已保留"))).toBe(true),
    );
    expect((within(deliver).getByLabelText("讨论内容") as HTMLTextAreaElement).value).toBe("请确认这个版本的标题。");
    expect(threadCalls).toBe(1);
  });

  it("re-previews a stale move plan as the same move action, never silently relabeled", async () => {
    // Regression: the stale re-preview used a constant action kind, so a
    // user-approved move came back as add_label.
    setDataContext("", "sp-1");
    const planPosts: Record<string, unknown>[] = [];
    mockFetch(
      allAvailableHandler((url, init) => {
        if (url.includes("/api/v1/organization-plans") && url.includes("/execute")) return failure("plan_stale", 409);
        if (url.includes("/api/v1/organization-plans")) {
          if (init?.method === "POST" && !url.includes("/confirm") && !url.includes("/execute") && !url.includes("/compensations")) planPosts.push(JSON.parse(String(init?.body)));
          return envelope({
            plan_id: "PLAN_ID",
            revision: 2,
            phase: "previewed",
            risk: "policy_scoped",
            approval_required: false,
            policy_revision: 4,
            items_total: 1,
            impact_digest: "IMPACT_SHA256",
            items: [{ item_key: "k", file_id: "f-1", version_id: "v-1", action: "move", before: "/a", after: "/整理后/a", state: "pending" }],
          });
        }
        if (url.includes("/api/v1/file-collections"))
          return url.includes("/snapshots") ? envelope({ snapshot_id: "SNAP_ID", item_count: 1 }) : envelope({ collection_id: "COLL_ID", revision: 1 });
        if (url.includes("/api/v1/file-queries"))
          return envelope({ mode: "exact", items: [{ file_id: "f-1", version_id: "v-1" }], coverage: "complete", total_count: 1, next_cursor: null });
        if (url.includes("/api/v1/spaces/sp-1/organization-policy"))
          return envelope({ policy_id: "POLICY_ID", space_id: "sp-1", actions: ["add_label"], revision: 4, enabled: true });
      }),
    );
    renderFlows({ initialCenter: "organize" });
    await userEvent.type(await screen.findByLabelText("目录前缀"), "docs/");
    await userEvent.click(screen.getByRole("button", { name: "固定所选文件" }));
    await waitFor(() => expect(screen.getByText(/已固定 1 个文件版本/)).toBeTruthy());
    await userEvent.click(screen.getByRole("button", { name: "生成移动计划" }));
    await waitFor(() => expect(screen.getByText(/PLAN_ID/)).toBeTruthy());
    await userEvent.click(screen.getByRole("button", { name: "确认并执行" }));
    await waitFor(() => expect(screen.getByText(/计划在预览后已发生变化/)).toBeTruthy());
    await userEvent.click(screen.getByRole("button", { name: "查询原整理请求" }));
    expect(planPosts).toHaveLength(1);
    expect((planPosts[0].actions as { kind: string }[])[0].kind).toBe("move");
  });

  it("retries the same draft with the same idempotency key and blocks double submits", async () => {
    // Regression: every submit minted a fresh key, so a retry after a lost
    // response would duplicate the thread under a different key.
    setDataContext("", "sp-1");
    const threadPosts: { idempotency_key: string }[] = [];
    let threadOk = false;
    mockFetch(
      allAvailableHandler((url, init) => {
        if (url.includes("/api/v1/shares")) return envelope([]);
        if (url.includes("/files/f-1/threads") && init?.method === "POST") {
          threadPosts.push(JSON.parse(String(init?.body)));
          return threadOk ? envelope({ thread_id: "THREAD_ID", file_id: "f-1", version_id: "v-1", revision: 1, state: "open", author_kind: "user" }) : failure("revision_conflict", 409);
        }
        if (url.includes("/files/f-1/threads")) return envelope([]);
        if (url.includes("/private-note")) return failure("not_found", 404);
        return undefined;
      }),
    );
    renderFlows({ initialCenter: "deliver" });
    const deliver = await screen.findByRole("tabpanel", { name: "交付与讨论" });
    await userEvent.type(within(deliver).getByLabelText("文件 ID"), "f-1");
    await userEvent.type(within(deliver).getByLabelText("讨论内容"), "请确认这个版本的标题。");
    // First submit fails: the draft AND its idempotency key survive.
    await userEvent.click(screen.getByRole("button", { name: "发布讨论" }));
    await waitFor(() => expect(threadPosts).toHaveLength(1));
    await waitFor(() =>
      expect(within(deliver).getAllByRole("alert").some((node) => node.textContent?.includes("草稿已保留"))).toBe(true),
    );
    // The retry of the SAME draft must reuse the SAME key so the server can
    // dedupe by key+fingerprint instead of creating a second thread.
    threadOk = true;
    await userEvent.click(screen.getByRole("button", { name: "发布讨论" }));
    await waitFor(() => expect(threadPosts).toHaveLength(2));
    expect(threadPosts[1].idempotency_key).toBe(threadPosts[0].idempotency_key);
    expect((threadPosts[0] as unknown as { body: string }).body).toBe("请确认这个版本的标题。");
    // A successful post invalidates the key: a NEW draft mints a NEW key.
    await userEvent.type(within(deliver).getByLabelText("讨论内容"), "第二条。");
    await userEvent.click(screen.getByRole("button", { name: "发布讨论" }));
    await waitFor(() => expect(threadPosts).toHaveLength(3));
    expect(threadPosts[2].idempotency_key).not.toBe(threadPosts[1].idempotency_key);
  });

  it("treats an unconfigured organization policy as a normal first-use state, not an error", async () => {
    // Drift D8: a space without a policy answers 404 not_found; the organize
    // center must not raise an error banner right after a successful
    // materialize — it guides policy creation instead.
    setDataContext("", "sp-1");
    mockFetch(
      allAvailableHandler((url) => {
        if (url.includes("/api/v1/file-collections"))
          return url.includes("/snapshots") ? envelope({ snapshot_id: "SNAP_ID", item_count: 1 }) : envelope({ collection_id: "COLL_ID", revision: 1 });
        if (url.includes("/api/v1/file-queries"))
          return envelope({ mode: "exact", items: [{ file_id: "f-1", version_id: "v-1" }], coverage: "complete", total_count: 1, next_cursor: null });
        if (url.includes("/api/v1/spaces/sp-1/organization-policy"))
          return failure("not_found", 404);
        return undefined;
      }),
    );
    const { container } = renderFlows({ initialCenter: "organize" });
    await userEvent.type(await screen.findByLabelText("目录前缀"), "docs/");
    await userEvent.click(screen.getByRole("button", { name: "固定所选文件" }));
    await waitFor(() => expect(screen.getByText(/已固定 1 个文件版本/)).toBeTruthy());
    // Guidance, no error banner.
    expect(container.querySelector(".banner.error")).toBeNull();
  });
});

it('keeps an independent center usable when another capability check fails', async () => {
  mockFetch(allAvailableHandler((url, init) => {
    if (url.includes('/operations/lookup') && JSON.parse(String(init?.body)).action === 'delivery_prepare') return failure('invalid_request', 400);
    return undefined;
  }));
  const actor = userEvent.setup(); renderFlows({ initialCenter: 'deliver' });
  await screen.findByText('无法确认流程能力');
  await actor.click(screen.getByRole('tab', { name: '整理中心' }));
  await screen.findByRole('button', { name: '固定所选文件' });
  expect(screen.queryByText('无法确认流程能力')).toBeNull();
});

it('manually rereads the exact directory prefix after reconnect and denies stale observations', async () => {
  setDataContext('', 'sp-1'); let forbidden = false;
  const fetch = mockFetch(allAvailableHandler(url => {
    if (url.includes('/file-queries')) return forbidden ? failure('forbidden', 403) : envelope({ mode: 'exact', items: [{ file_id: 'f-1', version_id: 'v-1' }], coverage: 'complete', next_cursor: null });
    if (url.includes('/organization-policy')) return failure('not_found', 404);
    return undefined;
  }));
  renderFlows({ initialCenter: 'organize' });
  await userEvent.type(await screen.findByLabelText('目录前缀'), 'docs/'); fireEvent(window, new Event('offline')); fireEvent(window, new Event('online'));
  const before = fetch.mock.calls.length; expect(screen.getByRole('button', { name: '固定所选文件' })).toHaveProperty('disabled', true); expect(fetch).toHaveBeenCalledTimes(before);
  await userEvent.click(screen.getByRole('button', { name: '联网后重新读取固定文件' }));
  await screen.findByText(/已固定 1 个文件版本/);
  const query = fetch.mock.calls.find(([url]) => String(url).includes('/file-queries'))!;
  expect(JSON.parse(String(query[1]?.body)).filters).toEqual([{ field: 'path_prefix', op: 'prefix', value: 'docs/' }]);
  expect(screen.getByRole('button', { name: '生成标签整理计划' })).toHaveProperty('disabled', false);
  forbidden = true; await userEvent.click(screen.getByRole('button', { name: '固定所选文件' }));
  await screen.findByText(/权限或范围已变化，整理信息已清除/);
  expect(screen.queryByRole('button', { name: '生成标签整理计划' })).toBeNull();
});
it('retains only a deterministic rejected intent marker across a center remount', async () => {
  const slot = 'drivebridge:organize:' + JSON.stringify([location.origin, user.id, '', '', 'sp-1']);
  const key = 'organize-' + 'a'.repeat(48);
  localStorage.setItem(slot, JSON.stringify({ key, action: 'organize_apply', plan: 'plan', revision: 2 })); localStorage.setItem(slot + ':rejected', key);
  mockFetch(allAvailableHandler()); renderFlows({ initialCenter: 'organize' });
  await userEvent.click(await screen.findByRole('button', { name: '释放已拒绝请求并重新预览' }));
  await screen.findByText(/原请求已明确拒绝/);
  expect(localStorage.getItem(slot)).toBeNull(); expect(localStorage.getItem(slot + ':rejected')).toBeNull();
  expect(screen.getByRole('button', { name: '固定所选文件' })).toHaveProperty('disabled', false);
});
