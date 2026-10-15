// Typed consumption surface for the consumer flows (upload recovery,
// retrieval, organization, fixed delivery and discussion writes).
//
// All requests go through the shared `api` envelope helper, so CSRF headers,
// data-context headers and the context-revision guard (a response arriving
// after a tenant/space switch is rejected) apply uniformly. Write requests
// carry the Idempotency-Key header scoped to instance+tenant+actor+action;
// the key is minted once per user intent and persisted to the recovery store
// BEFORE the request leaves, so a lost create response can be resolved by
// POST /operations/lookup with the original key and original_request.

import { api } from "../api";
import type { FileQueryRequest } from "../info-workspace/model";
import {
  normalizeContentQuery,
  normalizeDeliveryManifest,
  normalizeLookupResult,
  normalizeOrganizationItems,
  normalizeOrganizationPlan,
  normalizeUploadBatch,
  normalizeUploadBatchItems,
} from "./compat";
import {
  clampLimit,
  type ContentQueryView,
  type DeliveryManifestView,
  type OperationLookupResult,
  type OrganizationItem,
  type OrganizationPlanView,
  type UploadBatchItem,
  type UploadBatchView,
} from "./model";

// Add-items pages are capped at 500 by the upload contract; reads default to
// 50 and cap at 200 like every new DriveBridge surface.
export const ADD_ITEMS_PAGE = 500;

// ---------------------------------------------------------------------------
// Upload (drivebridge-upload-batch-recovery-v2).
// ---------------------------------------------------------------------------

// Read-only recovery query: the server recomputes the fingerprint from
// original_request with the same canonical rules as the create call, so the
// client sends the business inputs verbatim and never hashes them itself.
export async function lookupOperation(
  action: string,
  idempotencyKey: string,
  originalRequest: Record<string, unknown>,
  signal?: AbortSignal,
): Promise<OperationLookupResult> {
  const raw = await api<unknown>(
    "/operations/lookup",
    "POST",
    { action, idempotency_key: idempotencyKey, original_request: originalRequest },
    {},
    signal,
  );
  return normalizeLookupResult(raw);
}

export async function createUploadBatch(
  input: { space_id: string; conflict_policy: string },
  idempotencyKey: string,
  signal?: AbortSignal,
): Promise<UploadBatchView> {
  const raw = await api<unknown>("/upload-batches", "POST", input, { "Idempotency-Key": idempotencyKey }, signal);
  const view = normalizeUploadBatch(raw);
  if (!view) throw new Error("request_failed");
  return view;
}

export interface BatchItemInput {
  relative_path: string;
  size_bytes: number;
  sha256: string;
  item_key: string;
  expected_version?: string;
}

export async function addUploadBatchItems(
  batchId: string,
  items: BatchItemInput[],
  signal?: AbortSignal,
): Promise<void> {
  // The contract allows at most 500 items per call; larger selections are
  // chunked by the caller (sendBatchItems below).
  for (let offset = 0; offset < items.length; offset += ADD_ITEMS_PAGE) {
    await api<unknown>(
      "/upload-batches/" + encodeURIComponent(batchId) + "/items",
      "POST",
      { items: items.slice(offset, offset + ADD_ITEMS_PAGE) },
      {},
      signal,
    );
  }
}

export async function sealUploadBatch(batchId: string, signal?: AbortSignal): Promise<UploadBatchView> {
  const raw = await api<unknown>("/upload-batches/" + encodeURIComponent(batchId) + "/seal", "POST", {}, {}, signal);
  const view = normalizeUploadBatch(raw);
  if (!view) throw new Error("request_failed");
  return view;
}

export interface BatchPage {
  batch: UploadBatchView;
  items: UploadBatchItem[];
}

export async function getUploadBatch(
  batchId: string,
  page: { cursor?: string; limit?: number } = {},
  signal?: AbortSignal,
): Promise<BatchPage> {
  const params = new URLSearchParams();
  if (page.cursor) params.set("cursor", page.cursor);
  params.set("limit", String(clampLimit(page.limit)));
  const raw = await api<unknown>(
    "/upload-batches/" + encodeURIComponent(batchId) + "/items" + (params.size ? "?" + params : ""),
    "GET",
    undefined,
    {},
    signal,
  );
  const body = (raw && typeof raw === "object" ? raw : {}) as Record<string, unknown>;
  const view = normalizeUploadBatch(raw);
  if (!view) throw new Error("request_failed");
  return { batch: view, items: normalizeUploadBatchItems(body.items) };
}

export type BatchControl = "pause" | "resume" | "cancel";

export async function controlUploadBatch(
  batchId: string,
  action: BatchControl,
  signal?: AbortSignal,
): Promise<UploadBatchView> {
  const raw = await api<unknown>("/upload-batches/" + encodeURIComponent(batchId) + "/" + action, "POST", {}, {}, signal);
  const view = normalizeUploadBatch(raw);
  if (!view) throw new Error("request_failed");
  return view;
}

// ---------------------------------------------------------------------------
// Organization (drivebridge-agent-organization-v1).
// ---------------------------------------------------------------------------

export interface OrganizationPlanInput {
  source_snapshot_id?: string;
  refs?: string[];
  actions: { kind: string; namespace?: string; value?: string; target_prefix?: string }[];
  policy_id: string;
}

export async function createOrganizationPlan(
  input: OrganizationPlanInput,
  idempotencyKey: string,
  signal?: AbortSignal,
): Promise<OrganizationPlanView> {
  const raw = await api<unknown>("/organization-plans", "POST", input, { "Idempotency-Key": idempotencyKey }, signal);
  const view = normalizeOrganizationPlan(raw);
  if (!view) throw new Error("request_failed");
  return view;
}

export interface PlanPage {
  plan: OrganizationPlanView;
  items: OrganizationItem[];
}

export async function getOrganizationPlan(
  planId: string,
  page: { cursor?: string; limit?: number } = {},
  signal?: AbortSignal,
): Promise<PlanPage> {
  const params = new URLSearchParams();
  if (page.cursor) params.set("cursor", page.cursor);
  params.set("limit", String(clampLimit(page.limit)));
  const raw = await api<unknown>(
    "/organization-plans/" + encodeURIComponent(planId) + (params.size ? "?" + params : ""),
    "GET",
    undefined,
    {},
    signal,
  );
  const body = (raw && typeof raw === "object" ? raw : {}) as Record<string, unknown>;
  const view = normalizeOrganizationPlan(raw);
  if (!view) throw new Error("request_failed");
  return { plan: view, items: normalizeOrganizationItems(body.items) };
}

// Human confirmation entry: binds expected_revision and the impact digest.
export async function confirmOrganizationPlan(
  planId: string,
  input: { expected_revision: number; impact_digest: string },
  signal?: AbortSignal,
): Promise<OrganizationPlanView> {
  const raw = await api<unknown>("/organization-plans/" + encodeURIComponent(planId) + "/confirm", "POST", input, {}, signal);
  const view = normalizeOrganizationPlan(raw);
  if (!view) throw new Error("request_failed");
  return view;
}

export async function executeOrganizationPlan(
  planId: string,
  input: { expected_revision: number },
  idempotencyKey: string,
  signal?: AbortSignal,
): Promise<OrganizationPlanView> {
  const raw = await api<unknown>(
    "/organization-plans/" + encodeURIComponent(planId) + "/execute",
    "POST",
    input,
    { "Idempotency-Key": idempotencyKey },
    signal,
  );
  const view = normalizeOrganizationPlan(raw);
  if (!view) throw new Error("request_failed");
  return view;
}

// Compensation is a NEW reverse plan referencing the original result version;
// it is never an implicit undo.
export async function createCompensationPlan(
  planId: string,
  idempotencyKey: string,
  signal?: AbortSignal,
): Promise<OrganizationPlanView> {
  const raw = await api<unknown>(
    "/organization-plans/" + encodeURIComponent(planId) + "/compensations",
    "POST",
    {},
    { "Idempotency-Key": idempotencyKey },
    signal,
  );
  const view = normalizeOrganizationPlan(raw);
  if (!view) throw new Error("request_failed");
  return view;
}

// Read-only policy projection used by the organization center preview. The
// contract field is `revision`; a space without a policy yet answers 404
// not_found — the normal first-use state — and is surfaced as null so the UI
// guides creation instead of raising an error.
export interface OrganizationPolicyView {
  policy_id: string;
  revision: number;
  enabled: boolean;
}

export async function getOrganizationPolicy(
  spaceId: string,
  signal?: AbortSignal,
): Promise<OrganizationPolicyView | null> {
  let raw: unknown;
  try {
    raw = await api<unknown>("/spaces/" + encodeURIComponent(spaceId) + "/organization-policy", "GET", undefined, {}, signal);
  } catch (error) {
    if (error instanceof Error && error.message === "not_found") return null;
    throw error;
  }
  const body = (raw && typeof raw === "object" ? raw : {}) as Record<string, unknown>;
  return {
    policy_id: typeof body.policy_id === "string" ? body.policy_id : "",
    revision: typeof body.revision === "number" ? body.revision : 0,
    enabled: body.enabled === true,
  };
}

// ---------------------------------------------------------------------------
// Fixed delivery (drivebridge-fixed-delivery-v2).
// ---------------------------------------------------------------------------

export async function prepareDeliveryManifest(
  shareId: string,
  input: { snapshot_id: string; expected_share_revision: number },
  idempotencyKey: string,
  signal?: AbortSignal,
): Promise<DeliveryManifestView> {
  const raw = await api<unknown>(
    "/shares/" + encodeURIComponent(shareId) + "/delivery-manifests",
    "POST",
    input,
    { "Idempotency-Key": idempotencyKey },
    signal,
  );
  const view = normalizeDeliveryManifest(raw);
  if (!view) throw new Error("request_failed");
  return view;
}

export async function getDeliveryManifest(
  shareId: string,
  revision: number,
  signal?: AbortSignal,
): Promise<DeliveryManifestView> {
  const raw = await api<unknown>(
    "/shares/" + encodeURIComponent(shareId) + "/delivery-manifests/" + encodeURIComponent(String(revision)),
    "GET",
    undefined,
    {},
    signal,
  );
  const view = normalizeDeliveryManifest(raw);
  if (!view) throw new Error("request_failed");
  return view;
}

// Publish is the human-authorized completion gate: it succeeds only when every
// required member is readable, digest-verified, the manifest is committed and
// retention protection holds.
export async function publishDeliveryManifest(
  shareId: string,
  revision: number,
  signal?: AbortSignal,
): Promise<DeliveryManifestView> {
  const raw = await api<unknown>(
    "/shares/" + encodeURIComponent(shareId) + "/delivery-manifests/" + encodeURIComponent(String(revision)) + "/publish",
    "POST",
    {},
    {},
    signal,
  );
  const view = normalizeDeliveryManifest(raw);
  if (!view) throw new Error("request_failed");
  return view;
}

export type ReceiptKind = "opened" | "bytes_served" | "receiver_verified" | "receiver_confirmed";

export async function recordDeliveryReceipt(
  input: { share_id: string; manifest_revision: number; kind: ReceiptKind; sha256?: string },
  idempotencyKey: string,
  signal?: AbortSignal,
): Promise<void> {
  await api<unknown>("/delivery-receipts", "POST", input, { "Idempotency-Key": idempotencyKey }, signal);
}

// ---------------------------------------------------------------------------
// Retrieval (drivebridge-content-retrieval-v1).
// ---------------------------------------------------------------------------

export interface ContentQueryRequest {
  mode: "discovery" | "exact";
  query: { text?: string; image_ref?: string };
  space_ids: string[];
  modalities?: string[];
  evidence_required?: boolean;
  limit?: number;
  cursor?: string;
}


export interface FileCollectionView {
  collection_id: string;
  revision: number;
}

export async function createFileCollection(
  input: { scope: string; space_id?: string; members: string[] },
  idempotencyKey: string,
  signal?: AbortSignal,
): Promise<FileCollectionView> {
  const raw = await api<unknown>("/file-collections", "POST", input, { "Idempotency-Key": idempotencyKey }, signal);
  const body = (raw && typeof raw === "object" ? raw : {}) as Record<string, unknown>;
  if (typeof body.collection_id !== "string" || !body.collection_id) throw new Error("request_failed");
  return { collection_id: body.collection_id, revision: typeof body.revision === "number" ? body.revision : 1 };
}

export interface FileSnapshotView {
  snapshot_id: string;
  item_count: number;
}

export async function createCollectionSnapshot(
  collectionId: string,
  signal?: AbortSignal,
): Promise<FileSnapshotView> {
  const raw = await api<unknown>("/file-collections/" + encodeURIComponent(collectionId) + "/snapshots", "POST", {}, {}, signal);
  const body = (raw && typeof raw === "object" ? raw : {}) as Record<string, unknown>;
  if (typeof body.snapshot_id !== "string" || !body.snapshot_id) throw new Error("request_failed");
  return { snapshot_id: body.snapshot_id, item_count: typeof body.item_count === "number" ? body.item_count : 0 };
}

// Saved searches persist the query (not permissions); every read
// re-authorizes on the server.
export async function saveSearch(
  input: { name: string; query: FileQueryRequest },
  signal?: AbortSignal,
): Promise<void> {
  await api<unknown>("/saved-searches", "POST", input, {}, signal);
}


export async function queryContent(request: ContentQueryRequest, signal?: AbortSignal): Promise<ContentQueryView> {
  const raw = await api<unknown>("/content-queries", "POST", request, {}, signal);
  const view = normalizeContentQuery(raw);
  if (!view) throw new Error("request_failed");
  return view;
}

// ---------------------------------------------------------------------------
// Discussion writes (drivebridge-version-discussion-v1); reads already exist
// in the information workspace client (listFileThreads).
// ---------------------------------------------------------------------------

export interface FileThreadView {
  thread_id: string;
  file_id: string;
  version_id: string;
  revision: number;
  state: string;
  author_kind: string;
}

export async function createFileThread(
  fileId: string,
  input: { version_id?: string; body: string },
  idempotencyKey: string,
  signal?: AbortSignal,
): Promise<FileThreadView> {
  const raw = await api<unknown>(
    "/files/" + encodeURIComponent(fileId) + "/threads",
    "POST",
    { ...input, idempotency_key: idempotencyKey },
    {},
    signal,
  );
  const body = (raw && typeof raw === "object" ? raw : {}) as Record<string, unknown>;
  if (typeof body.thread_id !== "string" || !body.thread_id) throw new Error("request_failed");
  return {
    thread_id: body.thread_id,
    file_id: typeof body.file_id === "string" ? body.file_id : fileId,
    version_id: typeof body.version_id === "string" ? body.version_id : "",
    revision: typeof body.revision === "number" ? body.revision : 1,
    state: typeof body.state === "string" ? body.state : "open",
    author_kind: typeof body.author_kind === "string" ? body.author_kind : "user",
  };
}

export async function replyFileThread(
  threadId: string,
  input: { body: string },
  idempotencyKey: string,
  signal?: AbortSignal,
): Promise<void> {
  await api<unknown>("/threads/" + encodeURIComponent(threadId) + "/replies", "POST", { ...input, idempotency_key: idempotencyKey }, {}, signal);
}

export interface PrivateNoteView {
  body: string;
  revision: number;
}

export async function getPrivateNote(fileId: string, signal?: AbortSignal): Promise<PrivateNoteView | null> {
  const raw = await api<unknown>("/files/" + encodeURIComponent(fileId) + "/private-note", "GET", undefined, {}, signal);
  const body = (raw && typeof raw === "object" ? raw : {}) as Record<string, unknown>;
  if (typeof body.body !== "string") return null;
  return { body: body.body, revision: typeof body.revision === "number" ? body.revision : 1 };
}

export async function setPrivateNote(
  fileId: string,
  input: { body: string; expected_revision?: number },
  signal?: AbortSignal,
): Promise<void> {
  await api<unknown>("/files/" + encodeURIComponent(fileId) + "/private-note", "PUT", input, {}, signal);
}

// Publication preview shows what publishing the private note would create —
// the implemented shape answers {file_id,target,body_preview,note_revision,
// note}. It writes nothing: actual publication is an explicit thread
// creation, and the private note stays private.
export interface NotePublicationPreview {
  file_id: string;
  target: string;
  body_preview: string;
  note_revision: number;
  note: string;
}

export async function previewNotePublication(fileId: string, signal?: AbortSignal): Promise<NotePublicationPreview> {
  const raw = await api<unknown>("/files/" + encodeURIComponent(fileId) + "/private-note/publish-preview", "POST", {}, {}, signal);
  const body = (raw && typeof raw === "object" ? raw : {}) as Record<string, unknown>;
  return {
    file_id: typeof body.file_id === "string" ? body.file_id : "",
    target: typeof body.target === "string" ? body.target : "",
    body_preview: typeof body.body_preview === "string" ? body.body_preview : "",
    note_revision: typeof body.note_revision === "number" ? body.note_revision : 0,
    note: typeof body.note === "string" ? body.note : "",
  };
}

// ---------------------------------------------------------------------------
// Capability gates.
// ---------------------------------------------------------------------------

export type FlowCapability = "checking" | "available" | "unsupported";

// These read-only requests use LookupOperation's exact action-specific
// normalization (uploadbatch.go). A miss proves route support only, never
// authorization to create, confirm, or execute a user's actual intent.
export async function probeFlowCapability(action: string, signal?: AbortSignal): Promise<FlowCapability> {
  const originalRequests: Record<string, Record<string, unknown>> = {
    upload_batch_create: { space_id: "", conflict_policy: "fail" },
    content_query: { mode: "exact", space_ids: [] },
    organize_plan: { refs: [], actions: [], source_snapshot_id: "", policy_id: "" },
    delivery_prepare: { share_id: "", snapshot_id: "", expected_share_revision: 0 },
  };
  const original = originalRequests[action];
  if (!original) throw new Error("invalid_request");
  try {
    const lookup = await lookupOperation(action, "drivebridge-web-readonly-probe-" + crypto.randomUUID(), original, signal);
    return lookup.found === false || lookup.found === true ? "available" : "unsupported";
  } catch (error) {
    // A context-switch abort means the observation is void, not a verdict:
    // the caller re-probes after the tenant/space context settles.
    if (error instanceof DOMException && error.name === "AbortError") return "checking";
    const code = error instanceof Error ? error.message : "";
    const status = error instanceof Error ? (error as Error & { status?: number }).status : undefined;
    // Authentication and infrastructure failures cannot prove absent capability.
    if (status === 401 || status === 403 || (status !== undefined && status >= 500)) throw error;
    // Invalid request is a client/server contract error, not absent capability.
    if (code === "not_found" || (code === "request_failed" && (status === 404 || status === 405)) || code === "unsupported" || code === "unsupported_action")
      return "unsupported";
    throw error;
  }
}
