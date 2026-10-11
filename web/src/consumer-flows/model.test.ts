import { describe, expect, it } from "vitest";
import {
  acceptQueryResponse,
  batchItemFixture,
  batchItemPage,
  catalogueCandidatePage,
  checkReselectedSource,
  confirmDecision,
  decideAfterLookup,
  directoryInputSupport,
  groupSearchResults,
  partialOutcome,
  planItemFixture,
  planItemPage,
  publishGuard,
  sameRecoveryScope,
  selectedEntries,
  uploadDisplayPhase,
  validRelativePath,
  type OrganizationItem,
  type OrganizationPlanView,
  type RecoveryIdentity,
} from "./model";

const identity = (over: Partial<RecoveryIdentity> = {}): RecoveryIdentity => ({
  instance: "https://instance.example",
  actor: "user-a",
  space: "sp-1",
  ...over,
});


describe("TestFPPlatformConsumers/model", () => {
  it("scopes recovery identity to instance+actor+space", () => {
    const base = identity();
    expect(sameRecoveryScope(base, identity())).toBe(true);
    expect(sameRecoveryScope(base, identity({ actor: "user-b" }))).toBe(false);
    expect(sameRecoveryScope(base, identity({ space: "sp-2" }))).toBe(false);
    expect(sameRecoveryScope(base, identity({ instance: "https://other.example" }))).toBe(false);
  });

  it("reads the original operation by its original ID when the lookup finds the key", () => {
    const decision = decideAfterLookup({
      found: true,
      batch_id: "batch-1",
      operation_id: "op-77",
      result_certainty: "confirmed",
      recovery_action: "read_original_operation",
    });
    expect(decision).toEqual({ kind: "read_original_operation", operation_id: "op-77", batch_id: "batch-1" });
  });

  it("never re-keys: found=false may only replay the same key, conflict stops", () => {
    // Counter-example discipline: the only legal retry after found=false is
    // replaying the SAME idempotency key with the SAME original_request.
    const replay = decideAfterLookup({ found: false });
    expect(replay.kind).toBe("replay_same_key");
    // Same key with a different fingerprint is a hard stop — the client must
    // surface idempotency_conflict, never mint a replacement key.
    const conflict = decideAfterLookup({ found: false }, new Error("idempotency_conflict"));
    expect(conflict).toEqual({ kind: "stop", reason: "idempotency_conflict" });
  });

  it("displays 100% network-sent bytes as verifying, never as completed", () => {
    // Frozen scenario: 网络发送达到100%但仍验证 → 验证中而非上传完成.
    const sent = uploadDisplayPhase({
      bytes_total: 100,
      digest_computed: true,
      bytes_sent: 100,
      bytes_durable: 100,
      bytes_verified: 0,
      published: false,
    });
    expect(sent).toBe("verifying");
    expect(uploadDisplayPhase({
      bytes_total: 100,
      digest_computed: true,
      bytes_sent: 100,
      bytes_durable: 100,
      bytes_verified: 100,
      published: false,
    })).toBe("verifying");
    // Only server readback + publication completes the upload.
    expect(uploadDisplayPhase({
      bytes_total: 100,
      digest_computed: true,
      bytes_sent: 100,
      bytes_durable: 100,
      bytes_verified: 100,
      published: true,
    })).toBe("completed");
    expect(uploadDisplayPhase({
      bytes_total: 100,
      digest_computed: true,
      bytes_sent: 40,
      bytes_durable: 20,
      bytes_verified: 0,
      published: false,
    })).toBe("transferring");
  });

  it("verifies re-selected sources by size and digest, not by name", () => {
    const expected = { size_bytes: 10, sha256: "a".repeat(64) };
    expect(checkReselectedSource(expected, { size: 10, sha256: "a".repeat(64) })).toBe("match");
    expect(checkReselectedSource(expected, { size: 11, sha256: "a".repeat(64) })).toBe("size_mismatch");
    expect(checkReselectedSource(expected, { size: 10, sha256: "b".repeat(64) })).toBe("content_mismatch");
    // Without a persisted expectation nothing is claimed.
    expect(checkReselectedSource({}, { size: 10, sha256: "b".repeat(64) })).toBe("match");
  });

  it("rejects unsafe relative paths before items are sent", () => {
    expect(validRelativePath("docs/a.txt")).toBe(true);
    expect(validRelativePath("素材/媒体/视频.mp4")).toBe(true);
    expect(validRelativePath("/abs/path.txt")).toBe(false);
    expect(validRelativePath("a/../../etc/passwd")).toBe(false);
    expect(validRelativePath("a\\b.txt")).toBe(false);
    expect(validRelativePath("C:\\x")).toBe(false);
  });

  it("reports directory support honestly and preserves structure only when the browser does", () => {
    // jsdom has no webkitdirectory: the honest answer is files-only.
    expect(directoryInputSupport()).toBe("files-only");
    const plain = selectedEntries(
      [new File(["x"], "a.txt"), new File(["y"], "b.txt")],
      "files-only",
    );
    expect(plain.map((entry) => entry.relative_path)).toEqual(["a.txt", "b.txt"]);
    expect(plain.every((entry) => !entry.structured)).toBe(true);
    const structured = selectedEntries(
      [Object.defineProperty(new File(["x"], "a.txt"), "webkitRelativePath", { value: "root/docs/a.txt", configurable: true })],
      "directory",
    );
    expect(structured[0].relative_path).toBe("root/docs/a.txt");
    expect(structured[0].structured).toBe(true);
  });

  const plan = (over: Partial<OrganizationPlanView> = {}): OrganizationPlanView => ({
    plan_id: "plan-1",
    revision: 3,
    phase: "previewed",
    risk: "policy_scoped",
    approval_required: false,
    policy_revision: 4,
    items_total: 10,
    impact_digest: "d".repeat(64),
    ...over,
  });

  it("binds organization confirmation to the exact plan revision and never auto-reconfirms stale plans", () => {
    expect(confirmDecision(plan(), 3)).toEqual({ kind: "confirm" });
    // Frozen scenario: 确认前计划变化 → stale, must re-preview.
    expect(confirmDecision(plan({ revision: 4 }), 3)).toEqual({ kind: "stale", current_revision: 4 });
    expect(confirmDecision(plan({ phase: "stale" }), 3)).toEqual({ kind: "stale", current_revision: 3 });
    // Agent cannot self-approve risk beyond the rule scope.
    expect(confirmDecision(plan({ approval_required: true }), 3)).toEqual({ kind: "approval_required" });
  });

  it("keeps per-item results for partial organization execution", () => {
    const items: OrganizationItem[] = [
      { item_key: "1", file_id: "f1", version_id: "v", action: "add_label", before: "/a", after: "/a", state: "succeeded" },
      { item_key: "2", file_id: "f2", version_id: "v", action: "add_label", before: "/b", after: "/b", state: "failed", reason: "forbidden" },
      { item_key: "3", file_id: "f3", version_id: "v", action: "move", before: "/c", after: "/d", state: "pending" },
    ];
    expect(partialOutcome(items)).toEqual({ succeeded: 1, failed: 1, pending: 1 });
  });

  it("blocks publishing incomplete deliveries and names the blockers", () => {
    // Frozen scenario: 交付缺一项 → 缺项及恢复原任务入口，不发布不完整集合.
    expect(publishGuard({ share_id: "s", manifest_revision: 1, phase: "verifying", required_items: 10, verified_items: 9, complete: false, retention_ready: true })).toEqual({
      ok: false,
      blockers: ["delivery_incomplete"],
    });
    expect(publishGuard({ share_id: "s", manifest_revision: 1, phase: "ready", required_items: 10, verified_items: 10, complete: true, retention_ready: false })).toEqual({
      ok: false,
      blockers: ["retention_not_ready"],
    });
    expect(publishGuard({ share_id: "s", manifest_revision: 1, phase: "ready", required_items: 10, verified_items: 10, complete: true, retention_ready: true }).ok).toBe(true);
  });

  it("groups exact lists separately from Top-K candidates and drops late context responses", () => {
    const grouped = groupSearchResults(
      { items: [{ file_id: "f1", version_id: "v1" }] },
      { mode: "discovery", coverage: "partial", items: [{ file_ref: "f2" }, { file_ref: "f3" }], missing: ["audio"], next_cursor: null },
    );
    expect(grouped.exact).toEqual([{ file_id: "f1", version_id: "v1" }]);
    expect(grouped.candidates.map((item) => item.file_ref)).toEqual(["f2", "f3"]);
    expect(grouped.topK).toBe(true);
    expect(groupSearchResults(null, null).topK).toBe(false);
    // Frozen scenario: 空间切换时旧查询返回 → 丢弃旧响应，保留新空间上下文.
    expect(acceptQueryResponse(4, 4)).toBe(true);
    expect(acceptQueryResponse(4, 5)).toBe(false);
  });

  it("addresses 10,000-item batch and plan pages without materializing the whole set", () => {
    // Deterministic rows: the same index always yields the same item.
    expect(batchItemFixture(10000, 0)).toEqual(batchItemFixture(10000, 0));
    // Pages are bounded by the clamp (50 default, 200 cap).
    expect(batchItemPage(10000, 0, 50)).toHaveLength(50);
    expect(batchItemPage(10000, 0, 500)).toHaveLength(200);
    expect(batchItemPage(10000, 9980, 50)).toHaveLength(20);
    // The acceptance set embeds explicit 0-byte and unknown-certainty entries.
    const zeroByte = batchItemFixture(10000, 1954);
    expect((zeroByte as typeof zeroByte & { size_bytes?: number }).size_bytes).toBe(0);
    expect(batchItemFixture(10000, 991).result_certainty).toBe("unknown");
    // Organization plan pages read at the contract's 200 cap with per-item
    // before/after and failure reasons preserved.
    const planPage = planItemPage(10000, 0, 200);
    expect(planPage).toHaveLength(200);
    expect(planItemFixture(10000, 0)).toEqual(planItemFixture(10000, 0));
    expect(planPage.some((item) => item.state === "failed")).toBe(true);
    // Million-catalogue retrieval: Top-K candidates stay bounded at 20.
    expect(catalogueCandidatePage(50)).toHaveLength(20);
    expect(catalogueCandidatePage(20)[0].locator?.kind).toBe("time_range");
  });
});
