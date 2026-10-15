import { describe, expect, it } from "vitest";
import {
  memoryRecoveryStore,
  migrateRecoveryRecord,
  normalizeContentQuery,
  normalizeDeliveryManifest,
  normalizeLookupResult,
  normalizeOrganizationItems,
  normalizeOrganizationPlan,
  normalizeUploadBatch,
  normalizeUploadBatchItems,
} from "./compat";
import type { RecoveryIdentity, RecoveryRecord } from "./model";

// TestFPPlatformConsumers: old-service compatibility and recovery metadata
// suite. Old fixtures keep reading; migrations never lose references.

const identity = (over: Partial<RecoveryIdentity> = {}): RecoveryIdentity => ({
  instance: "https://instance.example",
  actor: "user-a",
  space: "sp-1",
  ...over,
});

const record = (over: Partial<RecoveryRecord> = {}): RecoveryRecord => ({
  version: 2,
  identity: identity(),
  action: "upload_batch_create",
  idempotency_key: "key-1",
  original_request: { space_id: "sp-1", conflict_policy: "fail" },
  batch_id: "batch-9",
  saved_at: "2026-09-25T00:00:00Z",
  ...over,
});

it('preserves organization plan lookup resource identity', () => {
  expect(normalizeLookupResult({ found: true, resource_id: 'plan-original', resource_type: 'organization_plan', status: 'previewed' })).toMatchObject({ found: true, resource_id: 'plan-original', resource_type: 'organization_plan', status: 'previewed' });
});

describe("TestFPPlatformConsumers/compat", () => {
  it("reads old-server upload batches with missing optional fields honestly", () => {
    // An old deployment answers with the preserved shape only; counters the
    // client cannot see are surfaced as defaults, never invented.
    const old = normalizeUploadBatch({ batch_id: "b-1" });
    expect(old).toMatchObject({ batch_id: "b-1", batch_revision: 1, phase: "enumerating", items_total: null, items_completed: 0 });
    expect(normalizeUploadBatch({ batch_id: "b-2", items_total: null, phase: "weird-phase" })).toMatchObject({ items_total: null, phase: "weird-phase" });
    expect(normalizeUploadBatch({})).toBeNull();
    expect(normalizeUploadBatch("junk")).toBeNull();
  });

  it("drops malformed batch items instead of guessing and keeps unknown certainty verbatim", () => {
    const items = normalizeUploadBatchItems([
      { item_key: "k-1", source_fingerprint: "f".repeat(64), relative_path: "a.txt", operation_id: "op-1", result_certainty: "unknown" },
      { item_key: "", operation_id: "op-2" },
      "junk",
    ]);
    expect(items).toHaveLength(1);
    expect(items[0].result_certainty).toBe("unknown");
  });

  it("normalizes the lookup result and organization/delivery/content projections", () => {
    expect(normalizeLookupResult({ found: true, batch_id: "b", operation_id: "o", result_certainty: "confirmed", recovery_action: "read_original_operation" })).toEqual({
      found: true, batch_id: "b", operation_id: "o", result_certainty: "confirmed", recovery_action: "read_original_operation",
    });
    expect(normalizeLookupResult(undefined)).toEqual({ found: false });
    // approval_required defaults to true: an old server that omits the flag
    // must not silently grant agent self-approval.
    expect(normalizeOrganizationPlan({ plan_id: "p-1" })).toMatchObject({ approval_required: true, phase: "draft", risk: "unknown" });
    expect(normalizeOrganizationPlan({ plan_id: "p-2", revision: 4, approval_required: false, impact_digest: "d" })).toMatchObject({ revision: 4, approval_required: false });
    expect(normalizeOrganizationPlan({ revision: 1 })).toBeNull();
    expect(normalizeOrganizationItems([{ item_key: "k", file_id: "f", state: "stale" }, { file_id: "f2" }])).toEqual([
      expect.objectContaining({ item_key: "k", file_id: "f", state: "stale" }),
    ]);
    expect(normalizeDeliveryManifest({ share_id: "s", manifest_revision: 1, required_items: 10, verified_items: 9 })).toMatchObject({ complete: false, retention_ready: false });
    const content = normalizeContentQuery({
      mode: "discovery",
      coverage: "partial",
      items: [{ file_ref: "f", locator: { kind: "time_range", start_ms: 1000, end_ms: 2500 }, truth_level: "inferred" }, { file_ref: "" }],
      missing: ["audio", 7],
      next_cursor: null,
    });
    expect(content).toMatchObject({ mode: "discovery", coverage: "partial", missing: ["audio"] });
    expect(content?.items).toHaveLength(1);
    expect(content?.items[0].locator).toEqual({ kind: "time_range", start_ms: 1000, end_ms: 2500 });
  });

  it("migrates v1 recovery metadata without losing references and discards unknown versions", () => {
    // v1 shape: top-level actor/space, no version, no instance scope.
    const v1 = {
      actor: "user-a",
      space: "sp-1",
      action: "upload_batch_create",
      idempotency_key: "key-1",
      original_request: { space_id: "sp-1", conflict_policy: "fail" },
      batch_id: "batch-9",
      saved_at: "2026-09-24T00:00:00Z",
    };
    const migrated = migrateRecoveryRecord(v1, "https://instance.example");
    expect(migrated).toEqual(record({ saved_at: "2026-09-24T00:00:00Z" }));
    // v2 passes through with the instance backfilled only when absent.
    expect(migrateRecoveryRecord(record({ identity: identity({ instance: "" }) }), "https://instance.example")?.identity.instance).toBe("https://instance.example");
    expect(migrateRecoveryRecord({ version: 99, actor: "user-a", action: "a", idempotency_key: "k" }, "i")).toBeNull();
    expect(migrateRecoveryRecord({ actor: "", action: "a", idempotency_key: "k" }, "i")).toBeNull();
  });


  it("isolates the recovery store by identity scope and clears on identity switch", async () => {
    const store = memoryRecoveryStore();
    expect(store.durable).toBe(false);
    const mine = record();
    const otherActor = record({ identity: identity({ actor: "user-b" }) });
    const otherSpace = record({ identity: identity({ space: "sp-2" }) });
    await store.put(mine);
    await store.put(otherActor);
    await store.put(otherSpace);
    const visible = await store.list(identity());
    expect(visible).toEqual([mine]);
    // Logout/identity switch clears this scope's display cache only.
    await store.clearIdentity(identity());
    expect(await store.list(identity())).toEqual([]);
    expect(await store.list(identity({ actor: "user-b" }))).toEqual([otherActor]);
    // Removal is scoped to the exact key.
    await store.put(mine);
    await store.remove(identity(), "upload_batch_create", "other-key");
    expect(await store.list(identity())).toHaveLength(1);
    await store.remove(identity(), "upload_batch_create", "key-1");
    expect(await store.list(identity())).toEqual([]);
  });

});
