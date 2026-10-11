import { afterEach, expect, it, vi } from "vitest";
import { detailPreferenceKey, readDetailWidth, saveDetailWidth } from "./detail-preference";
afterEach(() => { vi.restoreAllMocks(); localStorage.clear(); });
it("isolates width by instance and user and never writes an anonymous key", () => {
  const alice = detailPreferenceKey({ instance: "one", user: "alice" });
  saveDetailWidth(alice, 440);
  expect(readDetailWidth(alice)).toBe(440);
  expect(readDetailWidth(detailPreferenceKey({ instance: "two", user: "alice" }))).toBe(360);
  expect(readDetailWidth(detailPreferenceKey({ instance: "one", user: "bob" }))).toBe(360);
  expect(detailPreferenceKey({ instance: "", user: "alice" })).toBeUndefined();
  saveDetailWidth(undefined, 500);
  expect(localStorage.length).toBe(1);
});
it("does not collide on separators and ignores malformed or out-of-range values", () => {
  expect(detailPreferenceKey({ instance: "a:b", user: "c" })).not.toBe(detailPreferenceKey({ instance: "a", user: "b:c" }));
  for (const value of ["NaN", "null", "319", "641", ""]) {
    localStorage.setItem("fixture", value);
    expect(readDetailWidth("fixture")).toBe(360);
  }
  saveDetailWidth("fixture", Infinity);
  expect(localStorage.getItem("fixture")).toBe("");
});
it("tolerates blocked preference storage", () => {
  vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("denied"); });
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("denied"); });
  expect(readDetailWidth("fixture")).toBe(360);
  expect(() => saveDetailWidth("fixture", 400)).not.toThrow();
});
