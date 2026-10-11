import { contextHeaders, getDataContext } from "../../app/data-context";
import { statFile } from "../../info-workspace/client";
import type { NoteIdentity } from "./notebook-state";

export const NOTEBOOK_READ_LIMIT = 2 * 1024 * 1024;
export class NotebookReadError extends Error {
  constructor(public readonly state: "forbidden" | "stale" | "unsupported" | "error", message: string) { super(message); }
}

function readFailure(status: number): NotebookReadError {
  return new NotebookReadError(status === 401 || status === 403 ? "forbidden" : status === 404 || status === 409 ? "stale" : "error", "Notebook content is unavailable");
}

// Consumes the existing file-content contract; never follows a provider URL or
// treats a Pinax source as an ordinary writable file.
export async function readMarkdownFile(ref: NoteIdentity, signal: AbortSignal): Promise<string> {
  signal.throwIfAborted();
  const context = getDataContext();
  if (!ref.objectId || !ref.revision || !ref.sourceId || (context.space && ref.sourceId !== context.space)) throw new NotebookReadError("unsupported", "Notebook source does not match the active space");
  const check = () => {
    signal.throwIfAborted();
    if (getDataContext().revision !== context.revision) throw new DOMException("Context changed", "AbortError");
  };
  // Personal routes do not set a managed-space header. Resolve membership
  // through the authorized owner metadata instead of inventing that header.
  if (!context.space) {
    const file = await statFile(ref.objectId, signal).catch(error => {
      check();
      if (error instanceof Error && "status" in error && typeof error.status === "number") throw readFailure(error.status);
      throw error;
    });
    check();
    if (file.space_id !== ref.sourceId || file.trashed) throw new NotebookReadError("forbidden", "Notebook source is unavailable");
  }
  const response = await fetch(`/api/v1/files/${encodeURIComponent(ref.objectId)}/content?version=${encodeURIComponent(ref.revision)}`, {
    credentials: "same-origin", redirect: "error", headers: contextHeaders(), signal,
  });
  try { check(); }
  catch (error) { await response.body?.cancel().catch(() => {}); throw error; }
  if (!response.ok) {
    await response.body?.cancel();
    throw readFailure(response.status);
  }
  if (Number(response.headers.get("Content-Length")) > NOTEBOOK_READ_LIMIT) {
    await response.body?.cancel();
    throw new NotebookReadError("unsupported", "Notebook document exceeds the reading budget");
  }
  if (!response.body) throw new NotebookReadError("error", "Notebook content stream is missing");
  const reader = response.body.getReader();
  const decoder = new TextDecoder("utf-8", { fatal: true });
  let bytes = 0, body = "";
  try {
    while (true) {
      const next = await reader.read();
      check();
      if (next.done) break;
      bytes += next.value.byteLength;
      if (bytes > NOTEBOOK_READ_LIMIT) throw new NotebookReadError("unsupported", "Notebook document exceeds the reading budget");
      body += decoder.decode(next.value, { stream: true });
    }
    body += decoder.decode();
    check();
    return body;
  } catch (error) {
    await reader.cancel().catch(() => {});
    throw error;
  } finally { reader.releaseLock(); }
}
