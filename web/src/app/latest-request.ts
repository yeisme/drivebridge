// Abort is best effort; identity fencing also rejects responses from transports
// that finish after cancellation. Each mounted consumer owns its own gate.
export function latestRequest() {
  let current: AbortController | undefined;
  return {
    start() {
      current?.abort();
      const controller = new AbortController();
      current = controller;
      return { signal: controller.signal, isCurrent: () => current === controller && !controller.signal.aborted };
    },
    cancel() { current?.abort(); current = undefined; },
  };
}
