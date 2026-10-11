import { afterEach, expect, it, vi } from "vitest";
import { contextHeaders, contextURL, getDataContext, setDataContext } from "./data-context";
import { getDataContext as legacyContext, setDataContext as legacySet } from "../api";

afterEach(() => { setDataContext(""); vi.restoreAllMocks(); });

it("keeps old API imports on the same tab-local state and revision", () => {
  const before = getDataContext().revision;
  const listener = vi.fn();
  window.addEventListener("drivebridge-context", listener);
  try {
    legacySet("tenant", "space");
    expect(legacyContext()).toBe(getDataContext());
    expect(getDataContext()).toEqual({ tenant: "tenant", space: "space", revision: before + 1 });
    expect(listener).toHaveBeenCalledTimes(1);
    setDataContext("tenant", "space");
    expect(getDataContext().revision).toBe(before + 2);
  } finally { window.removeEventListener("drivebridge-context", listener); }
});

it("prevents callers from bypassing revision changes by mutating a snapshot", () => {
  setDataContext("tenant-a", "space-a");
  const before = getDataContext();
  expect(Object.isFrozen(before)).toBe(true);
  expect(Reflect.set(before, "tenant", "tenant-b")).toBe(false);
  expect(Reflect.set(before, "space", "space-b")).toBe(false);
  expect(Reflect.set(before, "revision", -1)).toBe(false);
  expect(contextHeaders()).toEqual({ "X-DriveBridge-Tenant": "tenant-a", "X-DriveBridge-Space": "space-a" });
  setDataContext("tenant-b", "space-b");
  expect(before.tenant).toBe("tenant-a");
  expect(getDataContext().revision).toBe(before.revision + 1);
  expect(Object.isFrozen(getDataContext())).toBe(true);
});

it("omits space for identity requests and keeps deep-link parameters", () => {
  setDataContext("tenant / one", "space-two");
  expect(contextHeaders(false)).toEqual({ "X-DriveBridge-Tenant": "tenant / one" });
  expect(contextHeaders()).toEqual({ "X-DriveBridge-Tenant": "tenant / one", "X-DriveBridge-Space": "space-two" });
  const url = new URL(contextURL("/api/v1/files/file/content?version=fixed#anchor"), location.origin);
  expect(url.searchParams.get("version")).toBe("fixed");
  expect(url.searchParams.get("tenant_context")).toBe("tenant / one");
  expect(url.searchParams.get("space_context")).toBe("space-two");
  expect(url.hash).toBe("#anchor");
  setDataContext("");
  expect(contextHeaders()).toEqual({});
});
