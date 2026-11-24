import { afterEach, describe, expect, it } from "vitest";
import { createLanLockManager, ensureLanBrowserCompat, sha256Hex } from "./tab-lock";
import {
  claimNotebookRecovery,
  encodeNotebookRecovery,
  inspectNotebookRecovery,
  newNotebookKey,
  notebookPageSlot,
  parseNotebookPageRecovery,
} from "../features/notebook/notebook-recovery";

const originalLocks = Object.getOwnPropertyDescriptor(navigator, "locks");
const originalSubtle = Object.getOwnPropertyDescriptor(globalThis.crypto, "subtle");
const originalSecureContext = Object.getOwnPropertyDescriptor(window, "isSecureContext");

afterEach(() => {
  localStorage.clear();
  if (originalLocks) Object.defineProperty(navigator, "locks", originalLocks);
  else Reflect.deleteProperty(navigator, "locks");
  if (originalSubtle) Object.defineProperty(globalThis.crypto, "subtle", originalSubtle);
  if (originalSecureContext) Object.defineProperty(window, "isSecureContext", originalSecureContext);
});

describe("LAN HTTP browser compatibility (tab-lock & sha256)", () => {
  it("computes identical SHA-256 digests when crypto.subtle is unavailable on plain HTTP", async () => {
    const withSubtle = await sha256Hex("drivebridge-lan-test");
    Object.defineProperty(globalThis.crypto, "subtle", { configurable: true, value: undefined });
    const withoutSubtle = await sha256Hex("drivebridge-lan-test");
    expect(withoutSubtle).toBe(withSubtle);
    expect(withoutSubtle).toMatch(/^[a-f0-9]{64}$/);
  });

  it("serializes concurrent lock claims and releases lease in storage", async () => {
    const manager = createLanLockManager();
    const order: string[] = [];
    const p1 = manager.request("slot-a", async () => {
      order.push("start-1");
      await new Promise((r) => setTimeout(r, 20));
      order.push("end-1");
      return "one";
    });
    const p2 = manager.request("slot-a", async () => {
      order.push("start-2");
      return "two";
    });
    const [r1, r2] = await Promise.all([p1, p2]);
    expect(r1).toBe("one");
    expect(r2).toBe("two");
    expect(order).toEqual(["start-1", "end-1", "start-2"]);
    expect(localStorage.getItem("drivebridge:tab-lock:slot-a")).toBeNull();
  });

  it("unlocks notebook creation on non-secure LAN HTTP origins where navigator.locks is absent", async () => {
    Object.defineProperty(window, "isSecureContext", { configurable: true, value: false });
    Object.defineProperty(navigator, "locks", { configurable: true, value: undefined });
    Object.defineProperty(globalThis.crypto, "subtle", { configurable: true, value: undefined });
    ensureLanBrowserCompat();

    const slot = notebookPageSlot("alice", "", "space-1", "");
    expect(await inspectNotebookRecovery(slot, parseNotebookPageRecovery)).toBeNull();
    const record = { key: newNotebookKey("notebook"), date: "", path: "" };
    const claimed = await claimNotebookRecovery(
      slot,
      parseNotebookPageRecovery,
      record,
      encodeNotebookRecovery,
      new AbortController().signal,
    );
    expect(claimed).toEqual({ record, claimed: true });
    expect(await inspectNotebookRecovery(slot, parseNotebookPageRecovery)).toEqual(record);
  });
});
