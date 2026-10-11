// @vitest-environment jsdom
import React from "react";
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, render } from "@testing-library/react";
import { CapacityPanel } from "./CapacityView";
const state = vi.hoisted(() => ({ api: vi.fn(), revision: 1 }));
vi.mock("./api", () => ({ api: state.api, bytes: (n: number) => `${n} B`, errorText: () => "无法读取", getDataContext: () => ({ revision: state.revision }) }));
afterEach(() => { cleanup(); vi.useRealTimers(); vi.restoreAllMocks(); state.api.mockReset(); state.revision = 1; });
function setup() {
  HTMLDialogElement.prototype.showModal = vi.fn();
  HTMLDialogElement.prototype.close = vi.fn();
  vi.spyOn(document, "hidden", "get").mockReturnValue(false);
}
it("refreshes only while visible and aborts when closed", async () => {
  setup(); vi.useFakeTimers(); state.api.mockResolvedValue([]);
  const view = render(<CapacityPanel onClose={() => {}} onDetails={() => {}} />);
  await act(async () => {});
  expect(state.api).toHaveBeenCalledTimes(1);
  await act(async () => { vi.advanceTimersByTime(30000); });
  expect(state.api).toHaveBeenCalledTimes(2);
  vi.spyOn(document, "hidden", "get").mockReturnValue(true);
  await act(async () => { document.dispatchEvent(new Event("visibilitychange")); vi.advanceTimersByTime(60000); });
  expect(state.api).toHaveBeenCalledTimes(2);
  const signal = state.api.mock.calls[1][4] as AbortSignal;
  expect(signal.aborted).toBe(true);
  view.unmount();
  await act(async () => { vi.advanceTimersByTime(60000); });
  expect(state.api).toHaveBeenCalledTimes(2);
});
it("does not publish a response from an obsolete identity", async () => {
  setup(); let resolve!: (rows: unknown[]) => void;
  state.api.mockImplementation(() => new Promise(r => { resolve = r; }));
  const refresh = vi.fn();
  render(<CapacityPanel onClose={() => {}} onDetails={() => {}} onRefresh={refresh} />);
  state.revision = 2;
  await act(async () => resolve([{ id: "old", name: "旧空间", used_bytes: 1, reserved_bytes: 0, max_bytes: 10 }]));
  expect(refresh).not.toHaveBeenCalled();
  expect(document.querySelector(".capacity-ring")).toBeNull();
});
