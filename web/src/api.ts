import {localizedError as errorText} from "./i18n";
import { sha256 } from "@noble/hashes/sha2.js";
import { getDataContext, contextHeaders, contextURL } from "./app/data-context";
import { api, csrfHeaders } from "./app/transport";
import { endSession } from "./app/logout";
export { api, setCSRF } from "./app/transport";
export { getDataContext, setDataContext, contextHeaders, contextURL } from "./app/data-context";
export interface Space {
 tenant_id?: string; team_id?: string;
  id: string;
  name: string;
  connection_id: string;
  used_bytes: number;
  reserved_bytes: number;
  max_bytes: number;
  trash_retention_days?: number;
  backend?: string;
}
export interface UserIdentity {
  id: string;
  name: string;
  display_name?: string;
  avatar_ref?: string;
}
export interface DriveFile {
 tenant_context?: string;
  preview_reason?: string;
  ref?: string;
  preview_url?: string;
  thumbnail_url?: string;
  compatibility_url?: string;
  review_pending?: boolean;
  caller?: string;
  task_ref?: string;
  id: string;
  space_id: string;
  path: string;
  version_id: string;
  size_bytes: number;
  mime_type: string;
  trashed: boolean;
  trashed_at?: string;
  purge_after?: string;
  updated_at: string;
}
export interface Operation {
  mime_type?: string;
  caller?: string;
  task_ref?: string;
  id: string;
  kind: string;
  space_id: string;
  path: string;
  size_bytes: number;
  sha256: string;
  status: string;
  bytes_transferred: number;
  blob_reused?: boolean;
  file_id?: string;
  version_id?: string;
  error?: string;
  updated_at?: string;
  source_id?: string;
  source_version?: string;
  source_space_id?: string;
  source_path?: string;
  progress_files?: number;
  total_files?: number;
}
export interface Connection {
  id: string;
  kind: string;
  name: string;
  disabled: boolean;
  status: string;
}
export interface Agent {
  archived?: boolean;
  id: string;
  name: string;
  space_id: string;
  prefix: string;
  scopes: string;
  expires_at: string;
  revoked: boolean;
}
export interface Version {
  caller?: string;
  task_ref?: string;
  id: string;
  sha256: string;
  size_bytes: number;
  mime_type: string;
  created_at: string;
}
export interface Audit {
  id: number;
  agent_id: string;
  action: string;
  resource: string;
  outcome: string;
  at: string;
}
export {errors as labels} from "./i18n";
export {localizedError as errorText} from "./i18n";
export function bytes(n: number) {
  if (n < 1024) return n + " B";
  if (n < 1048576) return (n / 1024).toFixed(1) + " KB";
  if (n < 1073741824) return (n / 1048576).toFixed(1) + " MB";
  return (n / 1073741824).toFixed(1) + " GB";
}
export async function upload(
  file: File,
  space: string,
  dest: string,
  onProgress: (op: Operation) => void,
  signal: AbortSignal,
  resume?: Operation,
  version?: DriveFile,
) {
  const uploadRevision = getDataContext().revision;
  const checkContext = () => { if (signal.aborted || uploadRevision !== getDataContext().revision) throw new DOMException("Upload context changed", "AbortError"); };
  checkContext();
  const hash = sha256.create();
  for (let offset = 0; offset < file.size; offset += 8 << 20) {
    checkContext();
    hash.update(
      new Uint8Array(
        await file.slice(offset, offset + (8 << 20)).arrayBuffer(),
      ),
    );
  }
  checkContext();
  const digest = Array.from(hash.digest(), (b) =>
    b.toString(16).padStart(2, "0"),
  ).join("");
  if (resume && (resume.size_bytes !== file.size || resume.sha256 !== digest))
    throw new Error("content_mismatch");
  let op =
    resume ||
    (await api<Operation>(
      "/uploads",
      "POST",
      {
        space_id: space,
        caller: "drivebridge-client",
        path: dest,
        size_bytes: file.size,
        sha256: digest,
        ...(version
          ? { file_id: version.id, expected_version: version.version_id }
          : {}),
      },
      // getRandomValues remains available on LAN HTTP; randomUUID does not.
      // Keep a cryptographic identity, never a timestamp/Math.random fallback.
      { "Idempotency-Key": `upload-${Array.from(crypto.getRandomValues(new Uint8Array(24)), byte => byte.toString(16).padStart(2, "0")).join("")}` },
      signal,
    ));
  onProgress(op);
  if (op.status === "completed") return op;
  const state = await api<{ received_parts: number[] }>("/operations/" + op.id);
  const received = new Set(state.received_parts);
  for (let index = 0; index * (8 << 20) < file.size; index++) {
    checkContext();
    if (received.has(index)) continue;
    const response = await fetch(
      "/api/v1/operations/" + op.id + "/parts/" + index,
      {
        method: "PUT",
        credentials: "same-origin",
        headers: {
          ...contextHeaders(),
          "Content-Type": "application/octet-stream",
          ...csrfHeaders(),
        },
        body: file.slice(index * (8 << 20), (index + 1) * (8 << 20)),
        signal,
      },
    );
    const envelope = await response.json();
    checkContext();
    if (!response.ok) throw new Error(envelope.error?.code || "request_failed");
    op = envelope.data;
    onProgress(op);
  }
  checkContext();
  onProgress({ ...op, status: "verifying" });
  op = await api<Operation>(
    "/operations/" + op.id + "/complete",
    "POST",
    {},
    {},
    signal,
  );
  onProgress(op);
  return op;
}
export async function download(file: DriveFile, version?: string) {
  // Validate the session before handing the byte stream to the browser's native
  // download manager. Large downloads must not accumulate in JS Blob memory.
  await api("/files/" + encodeURIComponent(file.id));
  const a = document.createElement("a");
  a.href =
    "/api/v1/files/" +
    encodeURIComponent(file.id) +
    "/content" +
    (version ? "?version=" + encodeURIComponent(version) : "");
  a.href = contextURL(a.href);
  a.download = file.path.split("/").pop() || "file";
  a.click();
}
export async function logout() {
  await endSession();
  location.reload();
}

// Resolve the selected version, even if a newer version was uploaded meanwhile.
export async function pinnedFile(file: DriveFile, version = file.version_id, headers: Record<string,string> = {}, signal?: AbortSignal) {
  const current = await api<{ ref: string }>(
    "/files/" + encodeURIComponent(file.id), "GET", undefined, headers, signal,
  );
  const ref = current.ref.replace(/\/version\/[^/]+$/, "/version/" + version);
  return api<{ file: DriveFile; ref: string; sha256: string }>(
    "/references?ref=" + encodeURIComponent(ref), "GET", undefined, headers, signal,
  );
}
export async function copyReference(
  file: DriveFile,
  version = file.version_id,
) {
  const pinned = await pinnedFile(file, version);
  await navigator.clipboard.writeText(pinned.ref);
}
export interface AuthMode {
 current_persona?: string;
 team_demo?: boolean; personas?: {id:string;name:string}[];
  demo?: boolean;
  governance?: boolean;
  oidc: boolean;
  key: boolean;
}
export interface IssuedUserKey {
  user_id: string;
  name: string;
  key: string;
  admin: boolean;
  expires_at: string;
}
export interface UserKeyRow {
  can_copy?: boolean; can_rotate?: boolean; can_revoke?: boolean; can_delete?: boolean; can_disable?: boolean; can_enable?: boolean; copy_unavailable_reason?: string;
  user_name?: string; current?: boolean; created_at?: string;
  archived?: boolean;
  disabled?: boolean;
  id: string;
  user: string;
  name: string;
  admin: boolean;
  expires_at: string;
  revoked: boolean;
}
// authMode reports which login paths this deployment honestly supports; it
// needs no session.
export async function authMode(): Promise<AuthMode> {
  const r = await fetch("/auth/mode", { credentials: "same-origin" });
  if (!r.ok) throw new Error("request_failed");
  const envelope = await r.json();
  return envelope.data as AuthMode;
}
// keyLogin exchanges a user key for the same cookie session OIDC mints; the
// key itself is never stored anywhere client-side.
export async function keyLogin(key: string): Promise<void> {
  const r = await fetch("/auth/key", {
    method: "POST",
    credentials: "same-origin",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ key }),
  });
  const envelope = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(envelope.error?.code === "csrf_rejected" ? "login_origin_mismatch" : envelope.error?.code || "request_failed");
}

// Only the explicitly enabled disposable fixture exposes this endpoint.
export async function demoLogin(persona = "admin"): Promise<void> {
  const response = await fetch("/auth/demo", { method: "POST",
    headers: {"Content-Type":"application/json"},
    body: JSON.stringify({persona}), credentials: "same-origin" });
  const envelope = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(envelope.error?.code === "csrf_rejected" ? "login_origin_mismatch" : envelope.error?.code || "request_failed");
}
