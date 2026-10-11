import { contextHeaders, getDataContext } from "./data-context";
import { retryAfterMs } from "./retry-delay";

let csrf = "";
export function setCSRF(value: string) { csrf = value; }
export function csrfHeaders(): Record<string, string> { return { "X-CSRF-Token": csrf }; }

export async function api<T>(
  path: string,
  method = "GET",
  body?: unknown,
  headers: Record<string, string> = {},
  signal?: AbortSignal,
): Promise<T> {
  signal?.throwIfAborted();
  const revision = getDataContext().revision;
  const response = await fetch("/api/v1" + path, {
    method,
    credentials: "same-origin",
    headers: {
      ...(body === undefined ? {} : { "Content-Type": "application/json" }),
      ...(method === "GET" ? {} : csrfHeaders()),
      ...contextHeaders(!path.startsWith("/governance") && path !== "/me" && !path.startsWith("/me/")),
      ...headers,
    },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal,
  });
  // Preserve compatibility with older plain-text 404/405 responses.
  const envelope = (await response.json().catch(() => ({}))) as {
    data?: unknown;
    error?: { code?: string; message?: string };
  };
  signal?.throwIfAborted();
  if (revision !== getDataContext().revision) throw new DOMException("Context changed", "AbortError");
  if (!response.ok) {
    const error = new Error(envelope.error?.code || "request_failed") as Error & { serverMessage?: string; status: number; retryAfterMs?: number };
    error.status = response.status;
    error.retryAfterMs = retryAfterMs(response.headers.get("Retry-After"));
    error.serverMessage = envelope.error?.message;
    throw error;
  }
  return envelope.data as T;
}
