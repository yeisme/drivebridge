// Typed consumption surface for the information workspace.
//
// All requests go through the shared `api` envelope helper, so CSRF headers,
// data-context headers, and the context-revision guard (a response that
// arrives after a tenant/space switch is rejected) apply uniformly. Read
// paths only: this package creates no server-side state, so no
// Idempotency-Key is issued here; write operations remain with their owners.

import { api, type DriveFile, type Operation, type Version } from "../api";
import { clampLimit } from "./model";
import type {
  AnalyticsQueryRequest,
  AnalyticsQueryResult,
  ContentBinding,
  FileQueryRequest,
  FileQueryResult,
  FileThread,
} from "./model";
import {
  normalizeAnalyticsResult,
  normalizeBindingList,
  normalizeFileQueryResult,
  normalizeThreadList,
} from "./compat";

export async function queryAnalytics(
  request: AnalyticsQueryRequest,
  signal?: AbortSignal,
): Promise<AnalyticsQueryResult> {
  const raw = await api<unknown>(
    "/analytics/queries",
    "POST",
    { ...request, limit: clampLimit(request.limit) },
    {},
    signal,
  );
  const result = normalizeAnalyticsResult(raw);
  if (!result) throw new Error("invalid_response");
  return result;
}

// Read back an analytics query by its original ID (export jobs for
// over-budget queries); the ID grants result readback, never file access.
export async function getAnalyticsQuery(id: string, signal?: AbortSignal) {
  return api<unknown>("/analytics/queries/" + encodeURIComponent(id), "GET", undefined, {}, signal);
}

export async function queryFiles(
  request: FileQueryRequest,
  signal?: AbortSignal,
): Promise<FileQueryResult> {
  const raw = await api<unknown>(
    "/file-queries",
    "POST",
    { ...request, limit: clampLimit(request.limit) },
    {},
    signal,
  );
  const result = normalizeFileQueryResult(raw);
  if (!result) throw new Error("invalid_response");
  return result;
}

export async function listFileThreads(fileId: string, signal?: AbortSignal): Promise<FileThread[]> {
  const raw = await api<unknown>(
    "/files/" + encodeURIComponent(fileId) + "/threads",
    "GET",
    undefined,
    {},
    signal,
  );
  return normalizeThreadList(raw);
}

// Bindings are listed per space (the contract route scopes by space_id and
// re-authorizes every row); callers narrow to one file by its fixed file_ref,
// which embeds the file id.
export async function listContentBindings(
  spaceId: string,
  signal?: AbortSignal,
): Promise<ContentBinding[]> {
  const raw = await api<unknown>(
    "/content-bindings?space_id=" + encodeURIComponent(spaceId),
    "GET",
    undefined,
    {},
    signal,
  );
  return normalizeBindingList(raw);
}

// Original-ID readback for operations: recovery never re-keys, it re-reads
// the operation the evidence points at.
export async function readOperation(id: string, signal?: AbortSignal): Promise<Operation> {
  // operations.get responds with the operation nested under `data.operation`.
  const out = await api<{ operation?: Operation } & Operation>(
    "/operations/" + encodeURIComponent(id),
    "GET",
    undefined,
    {},
    signal,
  );
  return out.operation || out;
}

export async function statFile(fileId: string, signal?: AbortSignal): Promise<DriveFile> {
  // files.stat responds with the file nested under `data.file` alongside the
  // top-level ref, matching what the existing pinnedFile consumer reads.
  const out = await api<{ file?: DriveFile } & DriveFile>(
    "/files/" + encodeURIComponent(fileId),
    "GET",
    undefined,
    {},
    signal,
  );
  return out.file || out;
}

export async function fileVersions(fileId: string, signal?: AbortSignal): Promise<Version[]> {
  return api<Version[]>("/files/" + encodeURIComponent(fileId) + "/versions", "GET", undefined, {}, signal);
}

// Source operations for one file. The preserved /operations endpoint has no
// file_id filter and no cursor: it returns the most recent operations of the
// caller (limit<=200, ordered updated DESC) after per-row authorization. The
// client narrows by space_id server-side (a filter the endpoint does support)
// and filters by file_id locally; the honest consequence is that a file whose
// operations fall outside the recent window shows no source row rather than
// a wrong one. A server-side file filter or cursor would belong to the
// backend owner's contract, not to this client.
export async function operationsForFile(
  fileId: string,
  signal?: AbortSignal,
  spaceId?: string,
): Promise<Operation[]> {
  const query = spaceId ? "?space_id=" + encodeURIComponent(spaceId) + "&limit=200" : "?limit=200";
  const ops = await api<Operation[]>("/operations" + query, "GET", undefined, {}, signal);
  return ops.filter((op) => op.file_id === fileId);
}

export type AnalyticsCapability = "checking" | "available" | "unsupported" | "error";

// Capability gate. New capabilities ship disabled; when the deployment has
// not enabled the analytics surface the typed panels must say so instead of
// failing the whole overview. Besides the contract's `unsupported` code, an
// older server answers unknown routes with a plain-text 404/405 body; the
// shared envelope helper reports those as `request_failed`, and a `not_found`
// on a dataset probe can only mean the surface is absent. The observed path
// also reports its own dependency honestly: 503 analytics_backend_unavailable
// means no analytics backend is deployed (the live path stays a plain typed
// read), so the gate closes as unsupported instead of surfacing a retryable
// error for a capability fact. Any other error (network, auth, budget) is
// reported as a real error by the caller.
export async function probeAnalytics(signal?: AbortSignal): Promise<AnalyticsCapability> {
  try {
    // Probe with the exact shape the panel needs: a minimal live query with
    // the dataset's fixed vocabulary. Omitted consistency would default to
    // observed, and empty dimensions+metrics is an invalid_query.
    await queryAnalytics(
      {
        dataset: "capacity_daily",
        filters: [],
        dimensions: ["day"],
        metrics: ["file_count"],
        limit: 1,
        consistency: "live",
      },
      signal,
    );
    return "available";
  } catch (error) {
    if (error instanceof Error) {
      const code = error.message;
      if (
        code === "unsupported" ||
        code === "request_failed" ||
        code === "not_found" ||
        code === "analytics_backend_unavailable"
      )
        return "unsupported";
    }
    throw error;
  }
}

