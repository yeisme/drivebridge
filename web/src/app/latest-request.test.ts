import { expect, it } from "vitest";
import { latestRequest } from "./latest-request";

it("fences A-B-A responses even when transport ignores abort", async () => {
  const gate = latestRequest();
  let resolveOld!: (value: string) => void;
  const pending = new Promise<string>(resolve => { resolveOld = resolve; });
  const firstA = gate.start();
  let shown = "";
  const oldRead = pending.then(value => { if (firstA.isCurrent()) shown = value; });
  const b = gate.start();
  const latestA = gate.start();
  expect(firstA.signal.aborted).toBe(true);
  expect(b.signal.aborted).toBe(true);
  if (latestA.isCurrent()) shown = "new A";
  resolveOld("old A");
  await oldRead;
  expect(shown).toBe("new A");
  expect(latestA.signal.aborted).toBe(false);
});
it("cancels pending work on teardown and permits a fresh StrictMode setup", () => {
  const gate = latestRequest();
  const first = gate.start();
  gate.cancel();
  gate.cancel();
  expect(first.signal.aborted).toBe(true);
  expect(first.isCurrent()).toBe(false);
  const next = gate.start();
  expect(next.isCurrent()).toBe(true);
});
it("keeps separate mounted consumers independent", () => {
  const a = latestRequest();
  const b = latestRequest();
  const first = a.start();
  const second = b.start();
  a.cancel();
  expect(first.isCurrent()).toBe(false);
  expect(second.isCurrent()).toBe(true);
});
