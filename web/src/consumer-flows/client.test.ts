import { afterEach, describe, expect, it, vi } from "vitest";
import {
  addUploadBatchItems,
  createFileThread,
  createOrganizationPlan,
  createUploadBatch,
  getDeliveryManifest,
  getOrganizationPlan,
  getOrganizationPolicy,
  getUploadBatch,
  lookupOperation,
  prepareDeliveryManifest,
  probeFlowCapability,
  previewNotePublication,
  publishDeliveryManifest,
  queryContent,
  sealUploadBatch,
  saveSearch,
} from "./client";

// TestFPPlatformConsumers: typed client suite. Requests are asserted at the
// network boundary — envelope paths, Idempotency-Key headers, contract body
// shapes and pagination chunking — against fixtures derived from the frozen
// backend contract samples.

const envelope = (data: unknown) =>
  new Response(JSON.stringify({ data }), { status: 200, headers: { "Content-Type": "application/json" } });
const failure = (code: string, status = 400) =>
  new Response(JSON.stringify({ error: { code } }), { status, headers: { "Content-Type": "application/json" } });
const plainText404 = new Response("404 page not found", { status: 404 });

function mockFetch(handler: (url: string, init?: RequestInit) => Response | Promise<Response>) {
  return vi.spyOn(globalThis, "fetch").mockImplementation(((input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
    return Promise.resolve(handler(url, init));
  }) as typeof fetch);
}

const batchSample = {
  batch_id: "BATCH_ID",
  batch_revision: 2,
  phase: "transferring",
  items_total: 3,
  items_completed: 1,
  items_failed: 0,
  items_waiting: 2,
  next_cursor: null,
};
const lookupSample = { found: true, batch_id: "BATCH_ID", operation_id: "OP_ID", result_certainty: "confirmed", recovery_action: "read_original_operation" };
const planSample = {
  plan_id: "PLAN_ID",
  revision: 1,
  phase: "previewed",
  risk: "policy_scoped",
  approval_required: false,
  policy_revision: 4,
  items_total: 100,
  impact_digest: "IMPACT_SHA256",
  items: [{ item_key: "k-1", file_id: "f-1", version_id: "v-1", action: "add_label", before: "/a", after: "/a", state: "pending" }],
};
const manifestSample = {
  share_id: "SHARE_ID",
  manifest_revision: 1,
  phase: "verifying",
  required_items: 10,
  verified_items: 9,
  complete: false,
  retention_ready: false,
};
const contentSample = {
  mode: "discovery",
  coverage: "partial",
  items: [{ file_ref: "FIXED_FILE_REF", evidence_ref: "EVIDENCE_REF", locator: { kind: "time_range", start_ms: 1000, end_ms: 2500 }, truth_level: "inferred", index_generation: "GEN_ID" }],
  missing: ["audio"],
  next_cursor: null,
};

it("sends saved exact conditions in the owner query envelope", async () => {
  const query = { mode: "exact" as const, space_ids: ["space"], filters: [{ field: "path_prefix", op: "contains" as const, value: "report%" }], limit: 50 };
  const fetch = mockFetch(() => envelope({ query_id: "saved", query }));
  await saveSearch({ name: "Report", query });
  expect(fetch.mock.calls[0][0]).toBe("/api/v1/saved-searches");
  expect(JSON.parse(fetch.mock.calls[0][1]?.body as string)).toEqual({ name: "Report", query });
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("TestFPPlatformConsumers/client", () => {
  it("sends the lookup recovery query with the original key and business inputs only", async () => {
    const fetcher = mockFetch((url) => (url.includes("/api/v1/operations/lookup") ? envelope(lookupSample) : failure("not_found", 404)));
    const result = await lookupOperation("upload_batch_create", "upload-batch-example", { space_id: "SPACE_ID", conflict_policy: "fail" });
    expect(result).toEqual(expect.objectContaining({ found: true, operation_id: "OP_ID" }));
    const [, init] = fetcher.mock.calls[0];
    expect(JSON.parse(String(init?.body))).toEqual({
      action: "upload_batch_create",
      idempotency_key: "upload-batch-example",
      original_request: { space_id: "SPACE_ID", conflict_policy: "fail" },
    });
  });

  it("creates upload batches with the Idempotency-Key header and seals them", async () => {
    const fetcher = mockFetch((url) => (url.includes("/seal") ? envelope({ ...batchSample, phase: "sealed" }) : envelope(batchSample)));
    const batch = await createUploadBatch({ space_id: "sp-1", conflict_policy: "fail" }, "key-1");
    expect(batch).toEqual(expect.objectContaining({ batch_id: "BATCH_ID", phase: "transferring", items_waiting: 2 }));
    const [, init] = fetcher.mock.calls[0];
    expect(new Headers(init?.headers).get("Idempotency-Key")).toBe("key-1");
    expect(JSON.parse(String(init?.body))).toEqual({ space_id: "sp-1", conflict_policy: "fail" });
    const sealed = await sealUploadBatch("BATCH_ID");
    expect(sealed.phase).toBe("sealed");
    expect(fetcher.mock.calls[1][0]).toContain("/api/v1/upload-batches/BATCH_ID/seal");
  });

  it("chunks batch items into contract pages of at most 500", async () => {
    const fetcher = mockFetch(() => envelope({}));
    const items = Array.from({ length: 1201 }, (_, index) => ({
      relative_path: `dir/f-${index}.txt`,
      size_bytes: 1,
      sha256: "a".repeat(64),
      item_key: `k-${index}`,
    }));
    await addUploadBatchItems("BATCH_ID", items);
    const itemCalls = fetcher.mock.calls.filter(([url]) => String(url).includes("/items"));
    expect(itemCalls).toHaveLength(3);
    expect(JSON.parse(String(itemCalls[0][1]?.body)).items).toHaveLength(500);
    expect(JSON.parse(String(itemCalls[2][1]?.body)).items).toHaveLength(201);
  });

  it("reads batch pages with the clamped limit and cursor", async () => {
    const fetcher = mockFetch(() => envelope({ ...batchSample, items: [{ item_key: "k-1", relative_path: "a.txt", operation_id: "op-1" }] }));
    const page = await getUploadBatch("BATCH_ID", { cursor: "c-1", limit: 500 });
    expect(page.batch.batch_id).toBe("BATCH_ID");
    expect(page.items[0]).toEqual(expect.objectContaining({ item_key: "k-1", result_certainty: "unknown" }));
    expect(fetcher.mock.calls[0][0]).toContain("cursor=c-1");
    expect(fetcher.mock.calls[0][0]).toContain("limit=200");
  });

  it("runs the organization plan chain with expected revisions bound", async () => {
    const fetcher = mockFetch((url) => (url.includes("/organization-plans") ? envelope(planSample) : failure("not_found", 404)));
    const plan = await createOrganizationPlan(
      { source_snapshot_id: "SNAPSHOT_ID", actions: [{ kind: "add_label", namespace: "rule", value: "reference" }], policy_id: "POLICY_ID" },
      "organize-example",
    );
    expect(plan).toEqual(expect.objectContaining({ plan_id: "PLAN_ID", risk: "policy_scoped", impact_digest: "IMPACT_SHA256" }));
    const [, createInit] = fetcher.mock.calls[0];
    expect(new Headers(createInit?.headers).get("Idempotency-Key")).toBe("organize-example");
    const read = await getOrganizationPlan("PLAN_ID", { limit: 200 });
    expect(read.items[0].action).toBe("add_label");
    expect(fetcher.mock.calls[1][0]).toContain("/api/v1/organization-plans/PLAN_ID");
  });

  it("prepares, reads and publishes delivery manifests on the share routes", async () => {
    const fetcher = mockFetch((url) =>
      url.includes("/publish") ? envelope({ ...manifestSample, phase: "published", complete: true, retention_ready: true, verified_items: 10 }) : envelope(manifestSample),
    );
    const manifest = await prepareDeliveryManifest("SHARE_ID", { snapshot_id: "SNAPSHOT_ID", expected_share_revision: 3 }, "delivery-example");
    expect(manifest).toEqual(expect.objectContaining({ manifest_revision: 1, complete: false, retention_ready: false }));
    expect(new Headers(fetcher.mock.calls[0][1]?.headers).get("Idempotency-Key")).toBe("delivery-example");
    expect(JSON.parse(String(fetcher.mock.calls[0][1]?.body))).toEqual({ snapshot_id: "SNAPSHOT_ID", expected_share_revision: 3 });
    const read = await getDeliveryManifest("SHARE_ID", 1);
    expect(read.required_items).toBe(10);
    expect(fetcher.mock.calls[1][0]).toContain("/api/v1/shares/SHARE_ID/delivery-manifests/1");
    const published = await publishDeliveryManifest("SHARE_ID", 1);
    expect(published.phase).toBe("published");
  });

  it("queries content with the contract body and propagates envelope error codes", async () => {
    const fetcher = mockFetch((url) => (url.includes("/api/v1/content-queries") ? envelope(contentSample) : failure("not_found", 404)));
    const view = await queryContent({ mode: "discovery", query: { text: "雨天街道的镜头" }, space_ids: ["SPACE_ID"], modalities: ["video"], evidence_required: true, limit: 20 });
    expect(view.coverage).toBe("partial");
    expect(view.missing).toEqual(["audio"]);
    expect(view.items[0].locator?.end_ms).toBe(2500);
    expect(JSON.parse(String(fetcher.mock.calls[0][1]?.body))).toEqual({
      mode: "discovery",
      query: { text: "雨天街道的镜头" },
      space_ids: ["SPACE_ID"],
      modalities: ["video"],
      evidence_required: true,
      limit: 20,
    });
    // Error envelopes surface as their code so the UI can show the frozen
    // next-step (e.g. content_index_stale → 旧版本标记).
    mockFetch(() => failure("content_index_stale", 409));
    await expect(queryContent({ mode: "discovery", query: { text: "x" }, space_ids: [] })).rejects.toThrow("content_index_stale");
  });

  it("creates version-anchored threads with an idempotency key in the body", async () => {
    const fetcher = mockFetch(() => envelope({ thread_id: "THREAD_ID", file_id: "FILE_ID", version_id: "VERSION_ID", revision: 1, state: "open", author_kind: "user" }));
    const thread = await createFileThread("FILE_ID", { version_id: "VERSION_ID", body: "请确认这个版本的标题。" }, "thread-example");
    expect(thread).toEqual(expect.objectContaining({ thread_id: "THREAD_ID", state: "open" }));
    expect(JSON.parse(String(fetcher.mock.calls[0][1]?.body))).toEqual({
      version_id: "VERSION_ID",
      body: "请确认这个版本的标题。",
      idempotency_key: "thread-example",
    });
    expect(fetcher.mock.calls[0][0]).toContain("/api/v1/files/FILE_ID/threads");
  });

  it.each([401, 403, 429, 500, 503])("does not turn plain-text HTTP %s into an unsupported capability", async status => {
    mockFetch(() => new Response("upstream failure", { status }));
    await expect(probeFlowCapability("content_query")).rejects.toMatchObject({ message: "request_failed", status });
  });

  it("probes capabilities read-only: absent surfaces close the gate, real errors throw", async () => {
    // Available: the lookup route answers found:false for the probe key.
    mockFetch((url) => (url.includes("/api/v1/operations/lookup") ? envelope({ found: false }) : failure("not_found", 404)));
    await expect(probeFlowCapability("upload_batch_create")).resolves.toBe("available");
    // Older deployment: unknown route answers plain text 404 → request_failed.
    mockFetch(() => plainText404);
    await expect(probeFlowCapability("organize_plan")).resolves.toBe("unsupported");
    mockFetch(() => failure("unsupported", 400));
    await expect(probeFlowCapability("delivery_prepare")).resolves.toBe("unsupported");
    // A transient 500 is a real error, not a capability verdict.
    mockFetch(() => failure("internal_error", 500));
    await expect(probeFlowCapability("content_query")).rejects.toThrow("internal_error");
  });

  it.each([
    ['upload_batch_create', { space_id: '', conflict_policy: 'fail' }],
    ['content_query', { mode: 'exact', space_ids: [] }],
    ['organize_plan', { refs: [], actions: [], source_snapshot_id: '', policy_id: '' }],
    ['delivery_prepare', { share_id: '', snapshot_id: '', expected_share_revision: 0 }],
  ])('uses the exact read-only lookup shape for %s', async (action, original) => {
    const fetcher = mockFetch(() => envelope({ found: false }));
    await expect(probeFlowCapability(action as string)).resolves.toBe('available');
    expect(JSON.parse(String(fetcher.mock.calls[0][1]?.body))).toEqual({ action, idempotency_key: expect.stringMatching(/^drivebridge-web-readonly-probe-/), original_request: original });
    expect(fetcher.mock.calls[0][0]).toContain('/operations/lookup');
  });
  it('reports an invalid lookup contract instead of declaring the surface unsupported', async () => {
    mockFetch(() => failure('invalid_request', 400));
    await expect(probeFlowCapability('organize_plan')).rejects.toThrow('invalid_request');
  });

  it("reads the organization policy by its revision field and treats an absent policy as null", async () => {
    // Drift D8: the contract view carries `revision` (the client used to read
    // policy_revision and render 0), and a space without a policy answers
    // 404 not_found — the normal first-use state, surfaced as null so the UI
    // can guide creation instead of showing an error.
    const fetcher = mockFetch((url) =>
      url.includes("/organization-policy")
        ? envelope({ policy_id: "POLICY_ID", space_id: "sp-1", actions: ["add_label"], revision: 4, enabled: true })
        : failure("not_found", 404),
    );
    await expect(getOrganizationPolicy("sp-1")).resolves.toEqual({ policy_id: "POLICY_ID", revision: 4, enabled: true });
    expect(fetcher.mock.calls[0][0]).toContain("/api/v1/spaces/sp-1/organization-policy");
    mockFetch(() => failure("not_found", 404));
    await expect(getOrganizationPolicy("sp-1")).resolves.toBeNull();
    // Any other failure stays an error.
    mockFetch((url) => (url.includes("/organization-policy") ? failure("forbidden", 403) : failure("not_found", 404)));
    await expect(getOrganizationPolicy("sp-1")).rejects.toThrow("forbidden");
  });

  it("reads the publish preview from body_preview/note_revision/note", async () => {
    // Drift D9: the implemented preview answers
    // {file_id,target,body_preview,note_revision,note}; target_teams/body
    // never existed, so the old reader rendered an empty preview.
    mockFetch((url) =>
      url.includes("/publish-preview")
        ? envelope({
            file_id: "FILE_ID",
            target: "team_thread",
            body_preview: "内部确认过的备注",
            note_revision: 2,
            note: "preview only; publish explicitly with file_thread_create; the private note stays private",
          })
        : failure("not_found", 404),
    );
    await expect(previewNotePublication("FILE_ID")).resolves.toEqual({
      file_id: "FILE_ID",
      target: "team_thread",
      body_preview: "内部确认过的备注",
      note_revision: 2,
      note: "preview only; publish explicitly with file_thread_create; the private note stays private",
    });
  });
});
