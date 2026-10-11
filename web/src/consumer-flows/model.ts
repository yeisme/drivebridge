// Consumer flow client model (upload recovery, retrieval, organization,
// fixed delivery and version discussion).
//
// Every shape and rule below is derived from the frozen OpenSpec contracts:
// this package's design.md plus the backend interface tables of
// drivebridge-upload-batch-recovery-v2 (batches, operation lookup, durable
// parts), drivebridge-agent-organization-v1 (plans, policy projection),
// drivebridge-fixed-delivery-v2 (manifests, receipts),
// drivebridge-content-retrieval-v1 (content queries) and
// drivebridge-version-discussion-v1 (threads, private notes). No field, enum
// or status code beyond those contracts is invented; unknown server values
// pass through verbatim so the UI can show the server's own words instead of
// guessing. The client holds only interface temporaries: selections, drafts,
// filters and non-secret recovery identity. Server batch/plan/manifest/thread
// state remains the source of truth.

import { clampLimit, DEFAULT_PAGE_LIMIT, MAX_PAGE_LIMIT, type ViewState } from "../info-workspace/model";

export { clampLimit, DEFAULT_PAGE_LIMIT, MAX_PAGE_LIMIT };
export type { ViewState };

// ---------------------------------------------------------------------------
// Upload contract (drivebridge-upload-batch-recovery-v2).
// ---------------------------------------------------------------------------

// UploadBatch response: batch_id, batch_revision, phase, item counters and
// next_cursor. items_total is null until the batch is sealed.
export interface UploadBatchView {
  batch_id: string;
  batch_revision: number;
  phase: string;
  items_total: number | null;
  items_completed: number;
  items_failed: number;
  items_waiting: number;
  next_cursor: string | null;
}

// Contract item: item_key, source_fingerprint, relative_path, operation_id
// and result_certainty. result_certainty expresses unknown external results
// instead of misreporting them as failed or succeeded.
export interface UploadBatchItem {
  item_key: string;
  source_fingerprint: string;
  relative_path: string;
  operation_id: string;
  result_certainty: string;
}

// POST /operations/lookup result sample: found, batch_id, operation_id,
// result_certainty and recovery_action.
export interface OperationLookupResult {
  found: boolean;
  batch_id?: string;
  operation_id?: string;
  result_certainty?: string;
  recovery_action?: string;
}

// Upload session projection: part_size, received parts / verified ranges,
// durability and source_reselection_required.
export interface UploadSessionView {
  part_size: number;
  received_parts: number[];
  durability: string;
  source_reselection_required: boolean;
}

// Phases displayed by the upload center. "verifying" is a distinct phase:
// 100% network-sent bytes never display as completed (the spec scenario
// "网络发送达到100%但仍验证" must show 验证中, not 上传完成).
export type UploadDisplayPhase =
  | "digesting"
  | "enumerating"
  | "sealed"
  | "transferring"
  | "verifying"
  | "completed"
  | "paused"
  | "partial"
  | "failed";

export const UPLOAD_PHASE_LABELS: Record<UploadDisplayPhase, string> = {
  digesting: "计算摘要",
  enumerating: "枚举文件",
  sealed: "已封存批次",
  transferring: "传输中",
  verifying: "验证中",
  completed: "已完成",
  paused: "已暂停",
  partial: "部分完成",
  failed: "失败",
};

// Separated progress facts: digest computed, network-sent bytes, durably
// ACKed bytes, server-verified bytes and final publication. Only the server
// readback publishes; the client never maps sent bytes to completion.
export interface UploadProgressFact {
  bytes_total: number;
  digest_computed: boolean;
  bytes_sent: number;
  bytes_durable: number;
  bytes_verified: number;
  published: boolean;
}

export function uploadDisplayPhase(fact: UploadProgressFact): UploadDisplayPhase {
  if (fact.published) return "completed";
  // All bytes handed to the network but not yet verified/published: the
  // honest display is 验证中, never 已完成.
  if (fact.bytes_total > 0 && fact.bytes_verified >= fact.bytes_total) return "verifying";
  if (fact.bytes_total > 0 && fact.bytes_sent >= fact.bytes_total) return "verifying";
  if (fact.bytes_durable > 0 || fact.bytes_sent > 0) return "transferring";
  if (fact.digest_computed) return "sealed";
  return "digesting";
}

// ---------------------------------------------------------------------------
// Recovery identity and idempotency (contracts: lookup carries action,
// idempotency_key and original_request; same key + same fingerprint replays,
// same key + different fingerprint is idempotency_conflict; the client must
// never silently re-key).
// ---------------------------------------------------------------------------

export interface RecoveryIdentity {
  instance: string;
  actor: string;
  space: string;
}

export interface RecoveryRecord {
  version: 2;
  identity: RecoveryIdentity;
  action: string;
  idempotency_key: string;
  // Business inputs only — no credentials, tokens or auth material. The
  // server recomputes the fingerprint from these with its own canonical
  // rules; the client never hashes them itself.
  original_request: Record<string, unknown>;
  batch_id?: string;
  item_key?: string;
  saved_at: string;
}

export function sameRecoveryScope(a: RecoveryIdentity, b: RecoveryIdentity): boolean {
  return a.instance === b.instance && a.actor === b.actor && a.space === b.space;
}

// What the UI does after a lookup, per the frozen recovery contract.
export type LookupDecision =
  | { kind: "read_original_operation"; operation_id: string; batch_id?: string }
  | { kind: "replay_same_key"; idempotency_key: string }
  | { kind: "stop"; reason: "idempotency_conflict" };

// idempotency_conflict (same key, different fingerprint) => stop and surface
// the conflict; a new intent would need an explicit new user action.
export function decideAfterLookup(
  lookup: OperationLookupResult,
  error?: unknown,
): LookupDecision {
  const code = error instanceof Error ? error.message : "";
  if (code === "idempotency_conflict") return { kind: "stop", reason: "idempotency_conflict" };
  if (lookup.found && lookup.operation_id)
    return { kind: "read_original_operation", operation_id: lookup.operation_id, batch_id: lookup.batch_id };
  return { kind: "replay_same_key", idempotency_key: "" };
}

// Source reselection check: a same-name file is not proof. Both size and
// SHA-256 must match the persisted fingerprint, otherwise content_mismatch
// and the user must re-select the original file; verified progress is kept.
export function checkReselectedSource(
  expected: { size_bytes?: number; sha256?: string },
  selected: { size: number; sha256: string },
): "match" | "size_mismatch" | "content_mismatch" {
  if (typeof expected.size_bytes === "number" && expected.size_bytes !== selected.size)
    return "size_mismatch";
  if (expected.sha256 && expected.sha256 !== selected.sha256) return "content_mismatch";
  return "match";
}

// Path safety enforced client-side before items are sent: relative paths
// only — absolute paths, ".." traversal and backslash escapes are rejected
// instead of being normalized silently.
export function validRelativePath(path: string): boolean {
  if (!path || path.length > 1024) return false;
  if (path.startsWith("/") || path.includes("\\") || /^[A-Za-z]:/.test(path)) return false;
  const segments = path.split("/");
  return segments.every((segment) => segment === "" || (segment !== "." && segment !== ".."));
}

// Empty directories upload as explicit entries when the browser can
// enumerate them (drag & drop directory entries); a trailing slash marks the
// placeholder.
export function emptyDirectoryEntry(relativePath: string): { relative_path: string; directory: true } {
  return { relative_path: relativePath.replace(/\/+$/, "") + "/", directory: true };
}

export function isEmptyDirectoryEntry(entry: { relative_path: string }): boolean {
  return entry.relative_path.endsWith("/");
}

// ---------------------------------------------------------------------------
// Folder selection: progressive enhancement, no faked support.
// ---------------------------------------------------------------------------

export type DirectorySupport = "directory" | "files-only";

export function directoryInputSupport(): DirectorySupport {
  const probe = document.createElement("input");
  return "webkitdirectory" in probe ? "directory" : "files-only";
}

export interface SelectedEntry {
  file: File;
  // Honest relative path: structured when the browser preserves directories,
  // bare file name otherwise. `structured` is false exactly when the plain
  // multi-file fallback is in play, so the UI can state 不保留目录结构.
  relative_path: string;
  structured: boolean;
  // Filled once the pre-send digest has been computed; never guessed.
  sha256?: string;
}

export function selectedEntries(files: File[], support: DirectorySupport): SelectedEntry[] {
  return files.map((file) => {
    const relative = (file as File & { webkitRelativePath?: string }).webkitRelativePath;
    if (support === "directory" && relative) {
      return { file, relative_path: relative.replace(/^\/+/, ""), structured: true };
    }
    return { file, relative_path: file.name, structured: false };
  });
}

// ---------------------------------------------------------------------------
// Organization contract (drivebridge-agent-organization-v1).
// ---------------------------------------------------------------------------

export interface OrganizationItem {
  item_key: string;
  file_id: string;
  version_id: string;
  action: string;
  before: string;
  after: string;
  expected_target_version?: string;
  state: string;
  reason?: string;
}

export interface OrganizationPlanView {
  plan_id: string;
  revision: number;
  phase: string;
  risk: string;
  approval_required: boolean;
  policy_revision: number;
  items_total: number;
  impact_digest: string;
}

// Confirmation binds the exact plan revision plus impact digest; a phase
// change, revision bump or approval requirement forces a fresh preview.
// Stale plans are never auto-reconfirmed.
export type ConfirmDecision =
  | { kind: "confirm" }
  | { kind: "stale"; current_revision: number }
  | { kind: "approval_required" };

export function confirmDecision(
  plan: OrganizationPlanView,
  expectedRevision: number,
): ConfirmDecision {
  if (plan.approval_required) return { kind: "approval_required" };
  if (plan.revision !== expectedRevision) return { kind: "stale", current_revision: plan.revision };
  if (plan.phase === "stale") return { kind: "stale", current_revision: plan.revision };
  return { kind: "confirm" };
}

// Partial execution keeps per-item results: succeeded items are never redone,
// failed items need re-authorization, and the plan reports both counts.
export function partialOutcome(items: Pick<OrganizationItem, "state">[]): {
  succeeded: number;
  failed: number;
  pending: number;
} {
  let succeeded = 0;
  let failed = 0;
  let pending = 0;
  for (const item of items) {
    if (item.state === "succeeded") succeeded++;
    else if (item.state === "failed") failed++;
    else pending++;
  }
  return { succeeded, failed, pending };
}

// ---------------------------------------------------------------------------
// Fixed delivery contract (drivebridge-fixed-delivery-v2).
// ---------------------------------------------------------------------------

export interface DeliveryManifestView {
  share_id: string;
  manifest_revision: number;
  phase: string;
  required_items: number;
  verified_items: number;
  complete: boolean;
  retention_ready: boolean;
}

// Receipt facts are graded; opened/bytes_served are observations and are
// never displayed as receiver acceptance.
export type ReceiptGrade =
  | "accessible"
  | "opened"
  | "bytes_served"
  | "receiver_verified"
  | "receiver_confirmed";

export const RECEIPT_GRADE_LABELS: Record<ReceiptGrade, string> = {
  accessible: "可访问",
  opened: "已打开",
  bytes_served: "已传出字节",
  receiver_verified: "接收方已验证",
  receiver_confirmed: "接收方已确认",
};

// The publish guard: publishing requires every required member readable,
// digest-verified, manifest committed AND retention protection ready. Missing
// items disable the publish action and surface a recovery entry back to the
// original preparation — an incomplete set is never published.
export function publishGuard(
  manifest: DeliveryManifestView,
): { ok: boolean; blockers: string[] } {
  const blockers: string[] = [];
  if (!manifest.complete) blockers.push("delivery_incomplete");
  if (!manifest.retention_ready) blockers.push("retention_not_ready");
  return { ok: blockers.length === 0, blockers };
}

// ---------------------------------------------------------------------------
// Retrieval contract (drivebridge-content-retrieval-v1 + exact file queries).
// ---------------------------------------------------------------------------

export interface ContentQueryItem {
  file_ref: string;
  asset_revision_ref?: string;
  evidence_ref?: string;
  locator?: { kind: string; start_ms?: number; end_ms?: number };
  truth_level?: string;
  index_generation?: string;
}

export interface ContentQueryView {
  mode: string;
  coverage: string;
  items: ContentQueryItem[];
  missing: string[];
  next_cursor: string | null;
  observed_at?: string;
}

// Exact metadata lists and semantic Top-K candidates are different calibers:
// Top-K is not a full set, so candidate counts are never rendered as totals.
export interface SearchResultGroups {
  exact: { file_id: string; version_id: string }[];
  candidates: ContentQueryItem[];
  topK: boolean;
}

export function groupSearchResults(
  exact: { items: { file_id: string; version_id: string }[] } | null,
  content: ContentQueryView | null,
): SearchResultGroups {
  return {
    exact: exact ? exact.items : [],
    candidates: content ? content.items : [],
    topK: content ? content.mode === "discovery" : false,
  };
}

// Late-response guard for space switches: a response may only render while
// the data-context revision it was observed under is still current. The
// shared api() helper already aborts on revision change; this is the pure
// guard used by tests and by pollers that resolve after the switch.
export function acceptQueryResponse(observedRevision: number, currentRevision: number): boolean {
  return observedRevision === currentRevision;
}

// ---------------------------------------------------------------------------
// Scale fixtures (design acceptance: 10,000-item upload/organization
// pagination, million-entry retrieval catalogue). Rows derive deterministically
// from their index so pages are addressed without materializing the whole
// set in memory; keyset order is the item key itself (ID tiebreak).
// ---------------------------------------------------------------------------

function mulberry32(seed: number) {
  let a = seed >>> 0;
  return () => {
    a |= 0;
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

function hex64(random: () => number): string {
  let out = "";
  for (let i = 0; i < 8; i++) out += Math.floor(random() * 0x100000000).toString(16).padStart(8, "0");
  return out;
}

export function batchItemFixture(total: number, index: number): UploadBatchItem {
  const random = mulberry32(index + 1);
  const dirs = ["素材", "docs", "media/video", "media/audio", "归档"];
  const ext = [".md", ".png", ".mp4", ".wav", ".pdf"][index % 5];
  const zero = index % 977 === 0; // explicit 0-byte entries in the acceptance set.
  return {
    item_key: "item-" + String(index + 1).padStart(6, "0") + "-" + total,
    source_fingerprint: hex64(random),
    relative_path: dirs[index % dirs.length] + "/file-" + String(index + 1).padStart(6, "0") + ext,
    operation_id: "op-fx-" + String(index + 1).padStart(8, "0"),
    result_certainty: index % 991 === 0 ? "unknown" : "confirmed",
    ...(zero ? { size_bytes: 0 } : {}),
  } as UploadBatchItem;
}

export function batchItemPage(total: number, cursorIndex: number, limit: number): UploadBatchItem[] {
  const size = clampLimit(limit);
  const start = Math.max(0, Math.min(cursorIndex, total));
  const end = Math.min(start + size, total);
  const page: UploadBatchItem[] = [];
  for (let i = start; i < end; i++) page.push(batchItemFixture(total, i));
  return page;
}

export function planItemFixture(total: number, index: number): OrganizationItem {
  const random = mulberry32(index + 7919);
  const batch = batchItemFixture(total, index);
  const actions = ["add_label", "move", "rename"];
  const action = actions[index % actions.length];
  return {
    item_key: batch.item_key,
    file_id: "file-fx-" + String(index + 1).padStart(8, "0"),
    version_id: "ver-fx-" + hex64(random).slice(0, 12),
    action,
    before: "/" + batch.relative_path,
    after: action === "move" ? "/整理后/" + batch.relative_path.split("/").pop() : "/" + batch.relative_path,
    expected_target_version: undefined,
    state: index % 13 === 0 ? "failed" : index % 7 === 0 ? "pending" : "succeeded",
    reason: index % 13 === 0 ? "forbidden" : undefined,
  };
}

export function planItemPage(total: number, cursorIndex: number, limit: number): OrganizationItem[] {
  const size = clampLimit(limit);
  const start = Math.max(0, Math.min(cursorIndex, total));
  const end = Math.min(start + size, total);
  const page: OrganizationItem[] = [];
  for (let i = start; i < end; i++) page.push(planItemFixture(total, i));
  return page;
}

// Semantic Top-K candidates against the million-entry catalogue: a bounded
// candidate page (contract limit ≤ 20 for the acceptance run), each carrying
// evidence, locator, truth level and index generation.
export function catalogueCandidate(index: number): ContentQueryItem {
  const random = mulberry32(index + 104729);
  return {
    file_ref: "file-fx-" + String((index * 9973) % 1000000 + 1).padStart(8, "0"),
    asset_revision_ref: "asset-fx-" + hex64(random).slice(0, 12),
    evidence_ref: "evidence-fx-" + hex64(random).slice(0, 12),
    locator: { kind: "time_range", start_ms: index * 100, end_ms: index * 100 + 1500 },
    truth_level: index % 3 === 0 ? "inferred" : "derived",
    index_generation: "gen-fx-0001",
  };
}

export function catalogueCandidatePage(count: number): ContentQueryItem[] {
  const page: ContentQueryItem[] = [];
  for (let i = 0; i < Math.min(count, 20); i++) page.push(catalogueCandidate(i));
  return page;
}
