import { api } from "../../api";
import type { components } from "../../generated/personal-workspace";
import type { NoteIdentity } from "./notebook-state";

type Schemas = components["schemas"];
export type NoteLinks = Schemas["FileLinkListEnvelope"]["data"];
export type NoteLinkReceipt = Schemas["FileLinkReceiptEnvelope"]["data"];
export async function readNoteLinks(identity: NoteIdentity, backlinks: boolean, signal: AbortSignal, cursor = "") {
  const query = new URLSearchParams({ version_id: identity.revision, limit: "50" });
  if (cursor) query.set("cursor", cursor);
  const url = `/files/${encodeURIComponent(identity.objectId)}/${backlinks ? "backlinks" : "links"}?${query}`;
  const result = await api<NoteLinks>(url, "GET", undefined, {}, signal);
  if (!result || result.file_id !== identity.objectId || result.version_id !== identity.revision || typeof result.indexed !== "boolean" || typeof result.stale !== "boolean" || !Array.isArray(result.items) || result.items.length > 1000 || result.items.some(link => !link || typeof link.file_ref !== "string" || typeof link.target_ref !== "string" || typeof link.source_path !== "string" || typeof link.target_path !== "string" || typeof link.anchor !== "string" || typeof link.stale !== "boolean")) throw new Error("relation_identity_mismatch");
  return result;
}
export const rebuildNoteLinks = (identity: NoteIdentity, key: string, signal: AbortSignal) => api<NoteLinkReceipt>(`/files/${encodeURIComponent(identity.objectId)}/links/rebuild`, "POST", { version_id: identity.revision } satisfies Schemas["FileLinkRebuild"], { "Idempotency-Key": key }, signal);
export const lookupNoteLinks = (identity: NoteIdentity, key: string, signal: AbortSignal) => api<NoteLinkReceipt>(`/files/${encodeURIComponent(identity.objectId)}/links/lookup`, "POST", { version_id: identity.revision, idempotency_key: key } satisfies Schemas["FileLinkLookup"], {}, signal);
