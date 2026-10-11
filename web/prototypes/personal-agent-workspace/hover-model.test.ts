import { afterEach, describe, expect, it, vi } from "vitest";
import { createHoverController } from "./hover-model";
afterEach(() => vi.useRealTimers());
describe("400ms file hover preview", () => {
  it("opens at 400ms, never at 399ms", () => {
    vi.useFakeTimers();
    const show = vi.fn();
    const c = createHoverController<string>(show);
    c.enter("a@v1");
    vi.advanceTimersByTime(399);
    expect(show).not.toHaveBeenCalledWith("a@v1");
    vi.advanceTimersByTime(1);
    expect(show).toHaveBeenLastCalledWith("a@v1");
  });
  it("invalidates old candidates when crossing links or clicking before 400ms", () => {
    vi.useFakeTimers();
    const show = vi.fn();
    const c = createHoverController<string>(show);
    c.enter("a");
    vi.advanceTimersByTime(399);
    c.enter("b");
    vi.advanceTimersByTime(1);
    expect(show).not.toHaveBeenCalledWith("a");
    vi.advanceTimersByTime(399);
    expect(show).toHaveBeenLastCalledWith("b");
    c.enter("c");
    c.cancel();
    vi.runAllTimers();
    expect(show).not.toHaveBeenCalledWith("c");
    expect(c.active()).toBe(false);
  });
  it("retains the visible card while entered and closes after 150ms outside", () => {
    vi.useFakeTimers();
    const show = vi.fn();
    const c = createHoverController<string>(show);
    c.enter("a");
    vi.advanceTimersByTime(400);
    c.leave();
    vi.advanceTimersByTime(149);
    expect(show).toHaveBeenLastCalledWith("a");
    c.retain();
    vi.advanceTimersByTime(100);
    expect(show).toHaveBeenLastCalledWith("a");
    c.leave();
    vi.advanceTimersByTime(150);
    expect(show).toHaveBeenLastCalledWith(null);
  });
  it("does not expose a candidate after permission loss or cancellation during delay", () => {
    vi.useFakeTimers();
    let valid = true;
    const show = vi.fn();
    const c = createHoverController<string>(show, () => valid);
    c.enter("a");
    valid = false;
    vi.advanceTimersByTime(400);
    expect(show).not.toHaveBeenCalledWith("a");
    c.cancel();
    valid = true;
    vi.runAllTimers();
    expect(show).not.toHaveBeenCalledWith("a");
  });
  it("keeps the same visible reference when returning from card to its trigger", () => {
    vi.useFakeTimers();
    const show = vi.fn();
    const c = createHoverController<{ id: string }>(
      show,
      () => true,
      (a, b) => a.id === b.id,
    );
    c.enter({ id: "a" });
    vi.advanceTimersByTime(400);
    c.leave();
    vi.advanceTimersByTime(100);
    show.mockClear();
    c.enter({ id: "a" });
    expect(show).not.toHaveBeenCalledWith(null);
    vi.advanceTimersByTime(150);
    expect(show).toHaveBeenLastCalledWith({ id: "a" });
  });
});
