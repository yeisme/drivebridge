import { api } from "./transport";
import type { Operation } from "../api";

// A failed sibling must not let the polling loop overlap an unfinished read.
// Publish the pair only when both reads succeeded in the same scope.
export async function readOperationProjection(signal: AbortSignal): Promise<[Operation[], Operation[]]> {
  const [operations, inbox] = await Promise.allSettled([
    api<Operation[]>("/operations", "GET", undefined, {}, signal),
    api<Operation[]>("/inbox", "GET", undefined, {}, signal),
  ]);
  signal.throwIfAborted();
  // Authorization failures take precedence over a sibling's transient failure.
  for (const result of [operations, inbox]) {
    if (result.status === "rejected" && result.reason instanceof Error && (
      [401, 403].includes((result.reason as Error & { status?: number }).status ?? 0) ||
      ["unauthorized", "forbidden"].includes(result.reason.message)
    )) throw result.reason;
  }
  const failures = [operations, inbox].filter(result => result.status === "rejected");
  const retryAfter = Math.max(0, ...failures.map(result =>
    result.reason instanceof Error ? (result.reason as Error & { retryAfterMs?: number }).retryAfterMs ?? 0 : 0));
  if (failures.length) {
    const error = failures[0].reason;
    if (error instanceof Error && retryAfter > 0) Object.assign(error, { retryAfterMs: retryAfter });
    throw error;
  }
  // Explicit narrowing retains the ordered tuple contract.
  if (operations.status === "rejected") throw operations.reason;
  if (inbox.status === "rejected") throw inbox.reason;
  return [operations.value, inbox.value];
}
