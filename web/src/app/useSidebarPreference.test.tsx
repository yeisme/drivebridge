import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { sidebarPreferenceKey, useSidebarPreference } from "./useSidebarPreference";

function Fixture({ instance = "instance-a", user = "user-a" }) {
  const [collapsed, toggle, reset, explicit] = useSidebarPreference(instance, user);
  return <><button onClick={toggle} aria-pressed={collapsed}>toggle</button><button onClick={reset} disabled={!explicit}>auto</button></>;
}
afterEach(() => { cleanup(); localStorage.clear(); vi.restoreAllMocks(); });
it("isolates explicit choices by instance and user without copying the old global value", () => {
  localStorage.setItem("drivebridge:sidebar-collapsed", "true");
  const view = render(<Fixture />);
  const button = screen.getByRole("button", { name: "toggle" });
  expect(button.getAttribute("aria-pressed")).toBe("false");
  fireEvent.click(button);
  expect(localStorage.getItem(sidebarPreferenceKey("instance-a", "user-a"))).toBe("true");
  view.rerender(<Fixture user="user-b" />);
  expect(button.getAttribute("aria-pressed")).toBe("false");
  view.rerender(<Fixture instance="instance-b" />);
  expect(button.getAttribute("aria-pressed")).toBe("false");
  view.rerender(<Fixture />);
  expect(button.getAttribute("aria-pressed")).toBe("true");
});
it("does not persist anonymous or automatic defaults", () => {
  const save = vi.spyOn(Storage.prototype, "setItem");
  const view = render(<Fixture user="" />);
  fireEvent.click(screen.getByRole("button", { name: "toggle" }));
  expect(save).not.toHaveBeenCalled();
  view.rerender(<Fixture />);
  expect(screen.getByRole("button", { name: "toggle" }).getAttribute("aria-pressed")).toBe("false");
  expect(save).not.toHaveBeenCalled();
});
it("tracks responsive defaults without overwriting explicit choices or saving media changes", () => {
  const media = Object.assign(new EventTarget(), { matches: false });
  vi.spyOn(window, "matchMedia").mockReturnValue(media as MediaQueryList);
  const save = vi.spyOn(Storage.prototype, "setItem");
  const view = render(<Fixture />);
  const button = screen.getByRole("button", { name: "toggle" });
  act(() => { media.matches = true; media.dispatchEvent(new Event("change")); });
  expect(button.getAttribute("aria-pressed")).toBe("true");
  expect(save).not.toHaveBeenCalled();
  fireEvent.click(button);
  expect(button.getAttribute("aria-pressed")).toBe("false");
  act(() => { media.matches = false; media.dispatchEvent(new Event("change")); media.matches = true; media.dispatchEvent(new Event("change")); });
  expect(button.getAttribute("aria-pressed")).toBe("false");
  expect(save).toHaveBeenCalledTimes(1);
  view.rerender(<Fixture user="user-b" />);
  expect(button.getAttribute("aria-pressed")).toBe("true");
  act(() => { media.matches = false; media.dispatchEvent(new Event("change")); });
  expect(button.getAttribute("aria-pressed")).toBe("false");
  expect(save).toHaveBeenCalledTimes(1);
});

it("restores responsive mode without deleting other users' preferences", () => {
  const media = Object.assign(new EventTarget(), { matches: true });
  vi.spyOn(window, "matchMedia").mockReturnValue(media as MediaQueryList);
  const otherKey = sidebarPreferenceKey("instance-a", "other");
  localStorage.setItem(otherKey, "false");
  render(<Fixture />);
  fireEvent.click(screen.getByRole("button", { name: "toggle" }));
  expect(screen.getByRole("button", { name: "toggle" }).getAttribute("aria-pressed")).toBe("false");
  fireEvent.click(screen.getByRole("button", { name: "auto" }));
  expect(screen.getByRole("button", { name: "toggle" }).getAttribute("aria-pressed")).toBe("true");
  expect(localStorage.getItem(sidebarPreferenceKey("instance-a", "user-a"))).toBeNull();
  expect(localStorage.getItem(otherKey)).toBe("false");
  act(() => { media.matches = false; media.dispatchEvent(new Event("change")); });
  expect(screen.getByRole("button", { name: "toggle" }).getAttribute("aria-pressed")).toBe("false");
});

it("keeps navigation usable when storage is denied", () => {
  vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("denied"); });
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("denied"); });
  render(<Fixture />);
  fireEvent.click(screen.getByRole("button", { name: "toggle" }));
  expect(screen.getByRole("button", { name: "toggle" }).getAttribute("aria-pressed")).toBe("true");
});
