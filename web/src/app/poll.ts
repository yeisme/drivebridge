import { readRetryDelay } from "./retry-delay";

// Sequential polling: one consumer owns the timer and in-flight request.
// Initial loading remains with the caller. Stop fences even abort-ignoring work.
export function poll<T>(read: (signal: AbortSignal) => Promise<T>, accept: (value: T) => void, delay: number | (() => number), onAuthorizationFailure?: (error: Error) => void, beforeResume?: (signal: AbortSignal) => Promise<void>) {
  let controller = new AbortController();
  let stopped = false;
  let failures = 0;
  let needsResumeCheck = document.hidden;
  const normalDelay = () => typeof delay === "function" ? delay() : delay;
  let nextDelay = normalDelay();
  let timer: ReturnType<typeof setTimeout> | undefined;
  let notBefore = 0;
  function schedule() {
    timer = setTimeout(tick, Math.min(2147483647, Math.max(0, notBefore - Date.now())));
  }
  async function tick() {
    if (stopped || document.hidden) return;
    if (Date.now() < notBefore) { schedule(); return; }
    const request = controller;
    try {
      if (needsResumeCheck && beforeResume) {
        await beforeResume(request.signal);
        if (stopped || request.signal.aborted || document.hidden) return;
        needsResumeCheck = false;
      }
      const value = await read(request.signal);
      if (!stopped && !request.signal.aborted && !document.hidden) {
        failures = 0;
        accept(value);
        nextDelay = normalDelay();
      }
    } catch (error) {
      // A stale generation cannot stop a newer visible session.
      if (!request.signal.aborted && error instanceof Error && (
        [401, 403].includes((error as Error & { status?: number }).status ?? 0) ||
        ["unauthorized", "forbidden"].includes(error.message)
      )) {
        stopped = true;
        controller.abort();
        onAuthorizationFailure?.(error);
      }
      if (!stopped && !request.signal.aborted) {
        const retryAfter = error instanceof Error ? (error as Error & { retryAfterMs?: number }).retryAfterMs : undefined;
        nextDelay = readRetryDelay(++failures, retryAfter);
      }
      // Retain the last projection on other read failures; never retry a mutation.
    } finally {
      if (!stopped && !request.signal.aborted) {
        notBefore = Date.now() + nextDelay;
        if (!document.hidden) schedule();
      }
    }
  }
  function visibilityChanged() {
    needsResumeCheck = true;
    clearTimeout(timer);
    controller.abort();
    controller = new AbortController();
    if (!stopped && !document.hidden) {
      notBefore = Math.max(notBefore, Date.now() + normalDelay());
      schedule();
    }
  }
  document.addEventListener("visibilitychange", visibilityChanged);
  notBefore = Date.now() + normalDelay();
  if (!document.hidden) schedule();
  return () => {
    stopped = true;
    controller.abort();
    clearTimeout(timer);
    document.removeEventListener("visibilitychange", visibilityChanged);
  };
}
