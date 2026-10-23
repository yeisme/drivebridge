import { afterEach, expect, it, vi } from "vitest";
import { claimPinaxRecovery, clearPinaxRecovery, inspectPinaxRecovery, pinaxRecoverySlot } from "./pinax-workbench-recovery";

const scope = { actor: "actor", sourceId: "source", bindingId: "binding" };
const originalLocks = Object.getOwnPropertyDescriptor(navigator, "locks");
function installLocks() {
  Object.defineProperty(navigator, "locks", { configurable: true, value: { request: async (_name: string, options: LockOptions | (() => unknown), callback?: () => unknown) => typeof options === "function" ? options() : callback!() } });
}
afterEach(() => {
  vi.restoreAllMocks(); localStorage.clear();
  if (originalLocks) Object.defineProperty(navigator, "locks", originalLocks);
  else Reflect.deleteProperty(navigator, "locks");
});

it("persists only the original opaque key and supports readback without a draft", async () => {
  installLocks();
  const signal = new AbortController().signal;
  expect(await claimPinaxRecovery(scope, "original", signal)).toEqual({ record: "original", claimed: true });
  expect(await claimPinaxRecovery(scope, "replacement", signal)).toEqual({ record: "original", claimed: false });
  expect(localStorage.getItem(pinaxRecoverySlot(scope))).toBe("original");
  expect(await inspectPinaxRecovery(scope)).toBe("original");
  await expect(clearPinaxRecovery(scope, "replacement")).rejects.toMatchObject({ reason: "recovery_changed" });
  expect(await inspectPinaxRecovery(scope)).toBe("original");
  await clearPinaxRecovery(scope, "original");
  expect(await inspectPinaxRecovery(scope)).toBeNull();
});

it("isolates actor, source and binding identities", async () => {
  installLocks();
  await claimPinaxRecovery(scope, "original", new AbortController().signal);
  for (const field of ["actor", "sourceId", "bindingId"]) expect(await inspectPinaxRecovery({ ...scope, [field]: "other" })).toBeNull();
});

it("refuses missing locks, denied storage and malformed records without deleting the original", async () => {
  Object.defineProperty(navigator, "locks", { configurable: true, value: undefined });
  await expect(claimPinaxRecovery(scope, "original", new AbortController().signal)).rejects.toMatchObject({ reason: "lock_unavailable" });
  installLocks();
  localStorage.setItem(pinaxRecoverySlot(scope), '{"body":"private draft"}');
  await expect(claimPinaxRecovery(scope, "replacement", new AbortController().signal)).rejects.toMatchObject({ reason: "invalid_recovery" });
  expect(localStorage.getItem(pinaxRecoverySlot(scope))).toBe('{"body":"private draft"}');
  const read = vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("denied"); });
  await expect(inspectPinaxRecovery(scope)).rejects.toMatchObject({ reason: "storage_unavailable" });
  read.mockRestore();
});

it.each(["", "../path", '{"body":"text"}', "a".repeat(129)])("rejects invalid key %s before storage", operationKey => {
  installLocks();
  expect(() => claimPinaxRecovery(scope, operationKey, new AbortController().signal)).toThrow();
  expect(localStorage.length).toBe(0);
});
