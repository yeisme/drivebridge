// Old-service compatibility, persistent recovery metadata and drafts for the
// consumer flow surfaces.
//
// The client consumes whatever the current deployment actually returns:
// responses may predate new optional fields, come from routes that do not
// exist yet, or carry unrecognized enum values. Normalizers keep only what is
// present, never invent values, and return null for payloads that cannot be
// interpreted so callers render an honest error state.
//
// Persistent recovery metadata is limited to non-secret identity: instance,
// actor, space, action, idempotency key, original_request and batch/item
// references. No file bodies, private download URLs or auth tokens are ever
// stored (contract constraint). Records are isolated per identity scope and

import { getDataContext } from "../api";
import type {
  ContentQueryView,
  DeliveryManifestView,
  OperationLookupResult,
  OrganizationItem,
  OrganizationPlanView,
  RecoveryIdentity,
  RecoveryRecord,
  UploadBatchItem,
  UploadBatchView,
} from "./model";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function asString(value: unknown): string | undefined {
  return typeof value === "string" && value.length > 0 ? value : undefined;
}

function asNumber(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function asBoolean(value: unknown): boolean | undefined {
  return typeof value === "boolean" ? value : undefined;
}

// ---------------------------------------------------------------------------
// Normalizers. Every new-surface field is optional from the client's point of
// view: counters fall back to 0 only when the whole block is absent (an old
// server), while a present-but-unrecognized value passes through verbatim.
// ---------------------------------------------------------------------------

export function normalizeUploadBatch(raw: unknown): UploadBatchView | null {
  if (!isRecord(raw)) return null;
  const batchId = asString(raw.batch_id);
  if (!batchId) return null;
  return {
    batch_id: batchId,
    batch_revision: asNumber(raw.batch_revision) ?? 1,
    phase: asString(raw.phase) ?? "enumerating",
    items_total: raw.items_total === null ? null : asNumber(raw.items_total) ?? null,
    items_completed: asNumber(raw.items_completed) ?? 0,
    items_failed: asNumber(raw.items_failed) ?? 0,
    items_waiting: asNumber(raw.items_waiting) ?? 0,
    next_cursor: asString(raw.next_cursor) ?? null,
  };
}

export function normalizeUploadBatchItems(raw: unknown): UploadBatchItem[] {
  if (!Array.isArray(raw)) return [];
  const items: UploadBatchItem[] = [];
  for (const entry of raw) {
    if (!isRecord(entry)) continue;
    const itemKey = asString(entry.item_key);
    const operationId = asString(entry.operation_id);
    if (!itemKey || !operationId) continue;
    items.push({
      item_key: itemKey,
      source_fingerprint: asString(entry.source_fingerprint) ?? "",
      relative_path: asString(entry.relative_path) ?? "",
      operation_id: operationId,
      result_certainty: asString(entry.result_certainty) ?? "unknown",
    });
  }
  return items;
}

export function normalizeLookupResult(raw: unknown): OperationLookupResult {
  if (!isRecord(raw)) return { found: false };
  return {
    found: asBoolean(raw.found) ?? false,
    batch_id: asString(raw.batch_id),
    operation_id: asString(raw.operation_id),
    ...(asString(raw.resource_id) ? { resource_id: asString(raw.resource_id) } : {}),
    ...(asString(raw.resource_type) ? { resource_type: asString(raw.resource_type) } : {}),
    ...(asString(raw.status) ? { status: asString(raw.status) } : {}),
    result_certainty: asString(raw.result_certainty) ?? "unknown",
    recovery_action: asString(raw.recovery_action),
  };
}

export function normalizeOrganizationPlan(raw: unknown): OrganizationPlanView | null {
  if (!isRecord(raw)) return null;
  const planId = asString(raw.plan_id);
  if (!planId) return null;
  return {
    plan_id: planId,
    revision: asNumber(raw.revision) ?? 1,
    phase: asString(raw.phase) ?? "draft",
    risk: asString(raw.risk) ?? "unknown",
    approval_required: asBoolean(raw.approval_required) ?? true,
    policy_revision: asNumber(raw.policy_revision) ?? 0,
    items_total: asNumber(raw.items_total) ?? 0,
    impact_digest: asString(raw.impact_digest) ?? "",
  };
}

export function normalizeOrganizationItems(raw: unknown): OrganizationItem[] {
  if (!Array.isArray(raw)) return [];
  const items: OrganizationItem[] = [];
  for (const entry of raw) {
    if (!isRecord(entry)) continue;
    const itemKey = asString(entry.item_key);
    const fileId = asString(entry.file_id);
    if (!itemKey || !fileId) continue;
    items.push({
      item_key: itemKey,
      file_id: fileId,
      version_id: asString(entry.version_id) ?? "",
      action: asString(entry.action) ?? "",
      before: asString(entry.before) ?? "",
      after: asString(entry.after) ?? "",
      expected_target_version: asString(entry.expected_target_version),
      state: asString(entry.state) ?? "pending",
      reason: asString(entry.reason),
    });
  }
  return items;
}

export function normalizeDeliveryManifest(raw: unknown): DeliveryManifestView | null {
  if (!isRecord(raw)) return null;
  const shareId = asString(raw.share_id);
  if (!shareId) return null;
  return {
    share_id: shareId,
    manifest_revision: asNumber(raw.manifest_revision) ?? 1,
    phase: asString(raw.phase) ?? "preparing",
    required_items: asNumber(raw.required_items) ?? 0,
    verified_items: asNumber(raw.verified_items) ?? 0,
    complete: asBoolean(raw.complete) ?? false,
    retention_ready: asBoolean(raw.retention_ready) ?? false,
  };
}

export function normalizeContentQuery(raw: unknown): ContentQueryView | null {
  if (!isRecord(raw)) return null;
  const items: ContentQueryView["items"] = [];
  for (const entry of Array.isArray(raw.items) ? raw.items : []) {
    if (!isRecord(entry)) continue;
    const fileRef = asString(entry.file_ref);
    if (!fileRef) continue;
    const locatorRaw = isRecord(entry.locator) ? entry.locator : undefined;
    items.push({
      file_ref: fileRef,
      asset_revision_ref: asString(entry.asset_revision_ref),
      evidence_ref: asString(entry.evidence_ref),
      locator: locatorRaw
        ? {
            kind: asString(locatorRaw.kind) ?? "unknown",
            start_ms: asNumber(locatorRaw.start_ms),
            end_ms: asNumber(locatorRaw.end_ms),
          }
        : undefined,
      truth_level: asString(entry.truth_level),
      index_generation: asString(entry.index_generation),
    });
  }
  return {
    mode: asString(raw.mode) ?? "discovery",
    coverage: asString(raw.coverage) ?? "unknown",
    items,
    missing: Array.isArray(raw.missing) ? raw.missing.filter((m): m is string => typeof m === "string") : [],
    next_cursor: asString(raw.next_cursor) ?? null,
    observed_at: asString(raw.observed_at),
  };
}

// ---------------------------------------------------------------------------
// Recovery metadata store.
//
// IndexedDB is the contract home for upload recovery identity. The store is
// keyed by the full identity scope (instance+actor+space+action+key) so
// records from another user or space are unreachable by construction; listing
// filters again as the authority. When IndexedDB is unavailable (old browser,
// test environment) the fallback keeps records in memory only and reports
// durable:false so the UI can say recovery across refresh is unavailable
// instead of pretending the identity was saved.
// ---------------------------------------------------------------------------

export const RECOVERY_DB = "drivebridge-consumer-flows";
export const RECOVERY_STORE = "recovery";

export interface RecoveryStore {
  readonly durable: boolean;
  put(record: RecoveryRecord): Promise<void>;
  list(identity: RecoveryIdentity): Promise<RecoveryRecord[]>;
  remove(identity: RecoveryIdentity, action: string, idempotencyKey: string): Promise<void>;
  clearIdentity(identity: RecoveryIdentity): Promise<void>;
}

function scopeKey(identity: RecoveryIdentity, action: string, idempotencyKey: string): string {
  return [identity.instance, identity.actor, identity.space, action, idempotencyKey].join("|");
}

// Historical v1 shape (early local drafts): a plain record without a version
// key and without instance scoping. Migration keeps every reference —
// idempotency key, original_request, batch/item ids — and only adds the
// version marker and the instance scope. Records from an unknown future
// version are discarded rather than guessed at.
export function migrateRecoveryRecord(raw: unknown, instance: string): RecoveryRecord | null {
  if (!isRecord(raw)) return null;
  const action = asString(raw.action);
  const idempotencyKey = asString(raw.idempotency_key);
  if (!action || !idempotencyKey) return null;
  if (raw.version === 2) {
    const identity = isRecord(raw.identity) ? raw.identity : undefined;
    const actor = asString(identity?.actor);
    const space = asString(identity?.space);
    if (!actor) return null;
    return {
      version: 2,
      identity: { instance: asString(identity?.instance) ?? instance, actor, space: space ?? "" },
      action,
      idempotency_key: idempotencyKey,
      original_request: isRecord(raw.original_request) ? raw.original_request : {},
      batch_id: asString(raw.batch_id),
      item_key: asString(raw.item_key),
      saved_at: asString(raw.saved_at) ?? "",
    };
  }
  if (raw.version !== undefined) return null; // unknown future version
  // v1: top-level actor/space without version or instance.
  const actor = asString(raw.actor);
  if (!actor) return null;
  return {
    version: 2,
    identity: { instance, actor, space: asString(raw.space) ?? "" },
    action,
    idempotency_key: idempotencyKey,
    original_request: isRecord(raw.original_request) ? raw.original_request : {},
    batch_id: asString(raw.batch_id),
    item_key: asString(raw.item_key),
    saved_at: asString(raw.saved_at) ?? "",
  };
}

export function memoryRecoveryStore(): RecoveryStore {
  const records = new Map<string, RecoveryRecord>();
  return {
    durable: false,
    async put(record) {
      records.set(scopeKey(record.identity, record.action, record.idempotency_key), record);
    },
    async list(identity) {
      return [...records.values()].filter((record) =>
        [record.identity.instance, record.identity.actor, record.identity.space].join("|") ===
        [identity.instance, identity.actor, identity.space].join("|"),
      );
    },
    async remove(identity, action, idempotencyKey) {
      records.delete(scopeKey(identity, action, idempotencyKey));
    },
    async clearIdentity(identity) {
      for (const [key, record] of records) {
        if (sameScopeKey(record.identity, identity)) records.delete(key);
      }
    },
  };
}

export function openRecoveryStore(instance: string): RecoveryStore {
  const idb = (globalThis as { indexedDB?: IDBFactory }).indexedDB;
  if (!idb) return memoryRecoveryStore();
  // The connection is opened lazily per operation; failures degrade to the
  // in-memory store so a broken/private-mode IndexedDB never blocks an upload
  // intent — it only loses cross-refresh recovery, which the UI states.
  const memory = memoryRecoveryStore();
  return {
    durable: true,
    async put(record) {
      try {
        const db = await openRecoveryDB(idb);
        await withStore(db, "readwrite", (store) =>
          store.put(record, scopeKey(record.identity, record.action, record.idempotency_key)),
        );
      } catch {
        await memory.put(record);
      }
    },
    async list(identity) {
      try {
        const db = await openRecoveryDB(idb);
        const records = await withStore(db, "readonly", (store) => store.getAll() as IDBRequest<RecoveryRecord[]>);
        return (records ?? [])
          .map((raw) => migrateRecoveryRecord(raw, instance))
          .filter((record): record is RecoveryRecord => record !== null)
          .filter((record) => sameScopeKey(record.identity, identity));
      } catch {
        return memory.list(identity);
      }
    },
    async remove(identity, action, idempotencyKey) {
      try {
        const db = await openRecoveryDB(idb);
        await withStore(db, "readwrite", (store) => store.delete(scopeKey(identity, action, idempotencyKey)));
      } catch {
        await memory.remove(identity, action, idempotencyKey);
      }
    },
    async clearIdentity(identity) {
      try {
        const db = await openRecoveryDB(idb);
        const records = await withStore(db, "readonly", (store) => store.getAll() as IDBRequest<RecoveryRecord[]>);
        await withStore(db, "readwrite", (store) => {
          for (const raw of records ?? []) {
            const record = migrateRecoveryRecord(raw, instance);
            if (record && sameScopeKey(record.identity, identity))
              store.delete(scopeKey(record.identity, record.action, record.idempotency_key));
          }
        });
      } catch {
        await memory.clearIdentity(identity);
      }
    },
  };
}

// Scope comparison shared by every store implementation so the IndexedDB and
function sameScopeKey(record: RecoveryIdentity, identity: RecoveryIdentity): boolean {
  return [record.instance, record.actor, record.space].join("|") === [identity.instance, identity.actor, identity.space].join("|");
}

function openRecoveryDB(idb: IDBFactory): Promise<IDBDatabase> {
  // Executor form: the project targets ES2022, before Promise.withResolvers.
  return new Promise((resolve, reject) => {
    const request = idb.open(RECOVERY_DB, 1);
    request.onupgradeneeded = () => {
      const db = request.result;
      if (!db.objectStoreNames.contains(RECOVERY_STORE)) db.createObjectStore(RECOVERY_STORE);
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error ?? new Error("indexeddb_unavailable"));
  });
}

function withStore<T>(
  db: IDBDatabase,
  mode: IDBTransactionMode,
  run: (store: IDBObjectStore) => IDBRequest<T> | void,
): Promise<T | undefined> {
  // Executor form: same ES2022 target constraint as openRecoveryDB.
  return new Promise((resolve, reject) => {
    const tx = db.transaction(RECOVERY_STORE, mode);
    const request = run(tx.objectStore(RECOVERY_STORE));
    if (request) {
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => reject(request.error ?? new Error("indexeddb_unavailable"));
    } else {
      resolve(undefined);
    }
    tx.oncomplete = () => db.close();
  });
}


// Called on the logout success path, BEFORE the page reloads: the recovery
// records and drafts scoped to the current identity are removed so the next
// login (possibly a different user on a shared browser) cannot see this
// session's unsent state. Failure is non-fatal — logout must proceed.
export async function purgeLocalFlowState(actor: string): Promise<void> {
  try {
    const store = openRecoveryStore(location.origin);
    await store.clearIdentity({ instance: location.origin, actor, space: getDataContext().space });
  } catch {
    /* Logout proceeds even if local cleanup fails. */
  }
}
