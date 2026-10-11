import { api } from "../../api";
import type { components } from "../../generated/personal-workspace";
import type { NoteIdentity } from "./notebook-state";

type Schemas = components["schemas"];
export type DocumentCapability = Schemas["DocumentCapabilityEnvelope"]["data"];
export type DocumentSave = Schemas["DocumentSaveEnvelope"]["data"];
export type DocumentLookup = Schemas["DocumentLookupEnvelope"]["data"];
export type DocumentRequest = Schemas["SaveDocumentRequest"];
const path = (ref: NoteIdentity) => `/files/${encodeURIComponent(ref.objectId)}`;
export const documentCapability = (ref: NoteIdentity, signal: AbortSignal) => api<DocumentCapability>(path(ref) + "/document-capability", "GET", undefined, {}, signal);
export const saveDocument = (ref: NoteIdentity, text: string, key: string, signal: AbortSignal) => api<DocumentSave>(path(ref) + "/document", "PUT", { base_version_id: ref.revision, text } satisfies DocumentRequest, { "Idempotency-Key": key }, signal);
export const lookupDocumentSave = (ref: NoteIdentity, key: string, signal: AbortSignal) => api<DocumentLookup>(path(ref) + "/document/lookup", "POST", { idempotency_key: key }, {}, signal);

// Only non-secret recovery identity is persisted, never body or credentials.
export function documentRecoveryKey(actor: string, ref: NoteIdentity) {
  return "drivebridge:document-save:" + JSON.stringify([actor, ref.sourceId, ref.objectId]);
}
// A Web Lock makes the recovery slot claim atomic across browser tabs. Without
// it, stay read-only rather than overwriting another tab's unresolved identity.
export async function claimDocumentSave(recoveryKey: string, signal: AbortSignal) {
  if (!navigator.locks?.request) throw new Error("recovery_lock_unavailable");
  return navigator.locks.request(recoveryKey, { signal }, () => {
    signal.throwIfAborted();
    const existing = localStorage.getItem(recoveryKey);
    if (existing !== null) {
      if (!/^edit-[a-f0-9]{48}$/.test(existing)) throw new Error("recovery_identity_invalid");
      return { key: existing, claimed: false };
    }
    const key = newDocumentSaveKey();
    localStorage.setItem(recoveryKey, key);
    return { key, claimed: true };
  });
}
export async function clearDocumentSave(recoveryKey: string, key: string) {
  if (!navigator.locks?.request) throw new Error("recovery_lock_unavailable");
  return navigator.locks.request(recoveryKey, () => {
    // Never remove a newer operation belonging to a different tab.
    if (localStorage.getItem(recoveryKey) === key) localStorage.removeItem(recoveryKey);
  });
}
export function newDocumentSaveKey() {
  const bytes = crypto.getRandomValues(new Uint8Array(24));
  return "edit-" + Array.from(bytes, value => value.toString(16).padStart(2, "0")).join("");
}
