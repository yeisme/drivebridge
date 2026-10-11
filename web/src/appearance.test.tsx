import React from "react";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { AppearanceProvider, readAppearance, useAppearance } from "./appearance";

const storage = new Map<string, string>();
let listeners: Set<() => void>;
let media: { matches: boolean; addEventListener: ReturnType<typeof vi.fn>; removeEventListener: ReturnType<typeof vi.fn> };
beforeEach(() => {
  storage.clear(); listeners = new Set();
  vi.stubGlobal("localStorage", { getItem: (key: string) => storage.get(key) ?? null, setItem: (key: string, value: string) => storage.set(key, value) });
  media = { matches: false, addEventListener: vi.fn((_event, fn) => listeners.add(fn)), removeEventListener: vi.fn((_event, fn) => listeners.delete(fn)) };
  vi.stubGlobal("matchMedia", vi.fn(() => media));
});
afterEach(() => { cleanup(); vi.unstubAllGlobals(); document.documentElement.classList.remove("dark"); });

function Controls() {
  const { preferences, resolvedTheme, setAppearance, storageError } = useAppearance();
  return <><output>{preferences.theme}/{resolvedTheme}</output>
    <button onClick={() => setAppearance({ theme: "dark", density: "compact", motion: "reduced" })}>Dark</button>
    <button onClick={() => setAppearance({ theme: "system" })}>System</button>
    {storageError && <p role="alert">{storageError}</p>}</>;
}

it("follows the system without overwriting the stored mode and cleans up listeners", () => {
  const view = render(<AppearanceProvider><Controls /></AppearanceProvider>);
  expect(screen.getByText("system/light")).toBeTruthy();
  act(() => { media.matches = true; listeners.forEach(fn => fn()); });
  expect(document.documentElement.dataset.theme).toBe("dark");
  expect(storage.has("drivebridge:theme")).toBe(false);
  view.unmount(); expect(listeners.size).toBe(0);
});

it("persists explicit choices and synchronizes changes from another tab", () => {
  render(<AppearanceProvider><Controls /></AppearanceProvider>);
  fireEvent.click(screen.getByText("Dark"));
  expect(storage.get("drivebridge:theme")).toBe("dark");
  expect(document.documentElement.dataset.density).toBe("compact");
  expect(document.documentElement.dataset.motion).toBe("reduced");
  act(() => { media.matches = false; listeners.forEach(fn => fn()); });
  expect(document.documentElement.dataset.theme).toBe("dark");
  storage.set("drivebridge:theme", "light");
  act(() => { window.dispatchEvent(new StorageEvent("storage", { key: "drivebridge:theme" })); });
  expect(screen.getByText("light/light")).toBeTruthy();
  fireEvent.click(screen.getByText("System"));
  expect(storage.get("drivebridge:theme")).toBe("system");
});

it("keeps settings usable when storage is unavailable and rejects unknown values", () => {
  storage.set("drivebridge:theme", "invalid");
  expect(readAppearance().theme).toBe("system");
  vi.stubGlobal("localStorage", { getItem: () => { throw new Error("denied"); }, setItem: () => { throw new Error("denied"); } });
  render(<AppearanceProvider><Controls /></AppearanceProvider>);
  fireEvent.click(screen.getByText("Dark"));
  expect(document.documentElement.dataset.theme).toBe("dark");
  expect(screen.getByRole("alert").textContent).toContain("浏览器未允许保存");
});
