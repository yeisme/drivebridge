import { afterEach, expect, it, vi } from "vitest";
import { readSidebarPreference } from "./sidebar-preference";

const key = "drivebridge:sidebar:fixture-instance:fixture-user";
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); localStorage.clear(); });
function viewport(tablet: boolean) {
  vi.stubGlobal("matchMedia", vi.fn().mockReturnValue({ matches: tablet }));
}
it("defaults to rail only on tablet without persisting a preference", () => {
  viewport(true);
  expect(readSidebarPreference(key)).toEqual({ collapsed: true, explicit: false });
  viewport(false);
  expect(readSidebarPreference(key)).toEqual({ collapsed: false, explicit: false });
  expect(localStorage.getItem(key)).toBeNull();
});
it.each(["true", "false"])("honors the explicit %s scoped preference over viewport", saved => {
  viewport(saved !== "true");
  localStorage.setItem(key, saved);
  expect(readSidebarPreference(key)).toEqual({ collapsed: saved === "true", explicit: true });
});
it("ignores legacy and invalid values and tolerates unavailable storage", () => {
  viewport(true);
  localStorage.setItem("drivebridge:sidebar-collapsed", "false");
  localStorage.setItem(key, "invalid");
  expect(readSidebarPreference(key)).toEqual({ collapsed: true, explicit: false });
  vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("denied"); });
  expect(readSidebarPreference(key)).toEqual({ collapsed: true, explicit: false });
});
it("does not read storage without an identity key", () => {
  viewport(false);
  const get = vi.spyOn(Storage.prototype, "getItem");
  expect(readSidebarPreference("")).toEqual({ collapsed: false, explicit: false });
  expect(get).not.toHaveBeenCalled();
});
