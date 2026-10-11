import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { claimDocumentSave, clearDocumentSave, documentRecoveryKey } from "./document-client";

const key = documentRecoveryKey("instance:user", { sourceId: "source", objectId: "note", revision: "one" });
const original = Object.getOwnPropertyDescriptor(navigator, "locks");
beforeEach(() => {
  Object.defineProperty(navigator, "locks", { configurable: true, value: {
    request: vi.fn(async (_name: string, options: LockOptions | (() => unknown), callback?: () => unknown) => typeof options === "function" ? options() : callback!()),
  } });
});
afterEach(() => {
  localStorage.clear(); vi.restoreAllMocks();
  if (original) Object.defineProperty(navigator, "locks", original);
  else Reflect.deleteProperty(navigator, "locks");
});
it("claims only one identity for the same actor and object", async () => {
  const signal = new AbortController().signal;
  const [a, b] = await Promise.all([claimDocumentSave(key, signal), claimDocumentSave(key, signal)]);
  expect(a.claimed).toBe(true); expect(b.claimed).toBe(false);
  expect(a.key).toBe(b.key);
  expect(localStorage.getItem(key)).toBe(a.key);
  expect(navigator.locks.request).toHaveBeenCalledWith(key, { signal }, expect.any(Function));
});
it("never clears another tab's newer operation", async () => {
  const original = await claimDocumentSave(key, new AbortController().signal);
  localStorage.setItem(key, "edit-" + "b".repeat(48));
  await clearDocumentSave(key, original.key);
  expect(localStorage.getItem(key)).toBe("edit-" + "b".repeat(48));
});
it("fails closed on corrupt or unavailable storage without replacing the identity", async () => {
  localStorage.setItem(key, "corrupt");
  await expect(claimDocumentSave(key, new AbortController().signal)).rejects.toThrow("recovery_identity_invalid");
  expect(localStorage.getItem(key)).toBe("corrupt");
  localStorage.clear();
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("denied"); });
  await expect(claimDocumentSave(key, new AbortController().signal)).rejects.toThrow("denied");
});
it("does not claim after its document session was revoked", async () => {
  const controller = new AbortController(); controller.abort();
  await expect(claimDocumentSave(key, controller.signal)).rejects.toThrow();
  expect(localStorage.length).toBe(0);
});
