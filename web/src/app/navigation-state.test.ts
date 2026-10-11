import { expect, it } from "vitest";
import { navigationReducer, type NavigationState } from "./navigation-state";

it("updates page and tab atomically without mutating the prior route", () => {
  const before: NavigationState = Object.freeze({ page: "files", tab: "" });
  expect(navigationReducer(before, { type: "route", page: "members", tab: "keys" })).toEqual({ page: "members", tab: "keys" });
  expect(before).toEqual({ page: "files", tab: "" });
});

it("keeps legacy single-field transitions for existing page callers", () => {
  const state: NavigationState = { page: "insights", tab: "usage" };
  expect(navigationReducer(state, { type: "page", page: "transfers" })).toEqual({ page: "transfers", tab: "usage" });
  expect(navigationReducer(state, { type: "tab", tab: "overview" })).toEqual({ page: "insights", tab: "overview" });
});
