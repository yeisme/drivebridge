import { afterEach, expect, it, vi } from "vitest";
import {
  claimNotebookRecovery,
  clearInvalidNotebookRecovery,
  clearNotebookRecovery,
  encodeNotebookRecovery,
  inspectNotebookRecovery,
  newNotebookKey,
  notebookPageSlot,
  parseNotebookPageRecovery,
} from "./notebook-recovery";

const originalLocks = Object.getOwnPropertyDescriptor(navigator, "locks");
const slot = notebookPageSlot("alice", "tenant", "space", "notes");

function installLocks() {
  Object.defineProperty(navigator, "locks", {
    configurable: true,
    value: { request: async (_name: string, options: LockOptions | (() => unknown), callback?: () => unknown) => typeof options === "function" ? options() : callback!() },
  });
}

afterEach(() => {
  vi.restoreAllMocks();
  localStorage.clear();
  if (originalLocks) Object.defineProperty(navigator, "locks", originalLocks);
  else Reflect.deleteProperty(navigator, "locks");
});

it("refuses inspection and claims when Web Locks is unavailable", async () => {
  Object.defineProperty(navigator, "locks", { configurable: true, value: undefined });
  await expect(inspectNotebookRecovery(slot, parseNotebookPageRecovery)).rejects.toMatchObject({ reason: "lock_unavailable" });
  await expect(claimNotebookRecovery(slot, parseNotebookPageRecovery, { key: newNotebookKey("notebook"), date: "", path: "notes" }, encodeNotebookRecovery, new AbortController().signal)).rejects.toMatchObject({ reason: "lock_unavailable" });
  expect(localStorage.length).toBe(0);
});

it("classifies browser storage failures without replacing a recovery key", async () => {
  installLocks();
  const getItem = vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("storage denied"); });
  await expect(inspectNotebookRecovery(slot, parseNotebookPageRecovery)).rejects.toMatchObject({ reason: "storage_unavailable" });
  expect(getItem).toHaveBeenCalled();
});

it("requires explicit cleanup for malformed records and preserves valid claims", async () => {
  installLocks();
  localStorage.setItem(slot, "broken");
  await expect(inspectNotebookRecovery(slot, parseNotebookPageRecovery)).rejects.toMatchObject({ reason: "invalid_recovery" });
  await clearInvalidNotebookRecovery(slot, parseNotebookPageRecovery);
  expect(localStorage.getItem(slot)).toBeNull();

  const first = { key: newNotebookKey("notebook"), date: "", path: "notes" };
  const claimed = await claimNotebookRecovery(slot, parseNotebookPageRecovery, first, encodeNotebookRecovery, new AbortController().signal);
  const second = await claimNotebookRecovery(slot, parseNotebookPageRecovery, { key: newNotebookKey("notebook"), date: "", path: "other" }, encodeNotebookRecovery, new AbortController().signal);
  expect(claimed).toEqual({ record: first, claimed: true });
  expect(second).toEqual({ record: first, claimed: false });
  await clearNotebookRecovery(slot, parseNotebookPageRecovery, first.key, value => value.key);
  expect(localStorage.getItem(slot)).toBeNull();
});

it("keeps different physical directories in separate recovery slots", () => {
  expect(notebookPageSlot("alice", "tenant", "space", "notes")).not.toBe(notebookPageSlot("alice", "tenant", "space", "other"));
});
