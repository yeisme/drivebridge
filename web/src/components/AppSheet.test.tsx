import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { AppSheet } from "./AppSheet";

afterEach(cleanup);
it.each([[360, 360], [440, 440], [100, 320], [900, 640]])("bounds explicit detail width %s to %s", (width, expected) => {
  render(<AppSheet title="详情" open onOpenChange={vi.fn()} detailWidth={width}>内容</AppSheet>);
  const dialog = screen.getByRole("dialog", { name: "详情" });
  expect(dialog.style.width).toBe(`min(100vw, ${expected}px)`);
  expect(dialog.className).toContain("max-md:!w-full");
});
it("resizes the right-hand detail with keyboard limits and reset", () => {
  render(<AppSheet title="详情" open onOpenChange={vi.fn()} detailWidth={360}>内容</AppSheet>);
  const handle = screen.getByRole("separator", { name: "调整详情宽度" });
  const dialog = screen.getByRole("dialog");
  const check = (width: number) => {
    expect(handle.getAttribute("aria-valuenow")).toBe(String(width));
    expect(dialog.style.width).toBe(`min(100vw, ${width}px)`);
  };
  fireEvent.keyDown(handle, { key: "ArrowLeft" }); check(376);
  fireEvent.keyDown(handle, { key: "ArrowRight", shiftKey: true }); check(344);
  fireEvent.keyDown(handle, { key: "Home" }); check(320);
  fireEvent.keyDown(handle, { key: "ArrowRight" }); check(320);
  fireEvent.keyDown(handle, { key: "End" }); check(640);
  fireEvent.keyDown(handle, { key: "ArrowLeft" }); check(640);
  fireEvent.keyDown(handle, { key: "Enter" }); check(360);
  fireEvent.keyDown(handle, { key: "End" });
  fireEvent.doubleClick(handle); check(360);
});

it.each(["close", "unmount"])("releases an in-flight drag on external %s", (mode) => {
  const onOpenChange = vi.fn();
  const view = render(<AppSheet title="详情" open onOpenChange={onOpenChange} detailWidth={360}>内容</AppSheet>);
  const handle = screen.getByRole("separator");
  const release = vi.fn();
  Object.assign(handle, { setPointerCapture: vi.fn(), hasPointerCapture: () => true, releasePointerCapture: release });
  fireEvent.pointerDown(handle, { button: 0, pointerId: 7, clientX: 500 });
  fireEvent.pointerMove(handle, { pointerId: 7, clientX: 420 });
  expect(handle.getAttribute("aria-valuenow")).toBe("440");
  if (mode === "unmount") view.unmount();
  else {
    view.rerender(<AppSheet title="详情" open={false} onOpenChange={onOpenChange} detailWidth={360}>内容</AppSheet>);
    view.rerender(<AppSheet title="详情" open onOpenChange={onOpenChange} detailWidth={360}>内容</AppSheet>);
    expect(screen.getByRole("separator").getAttribute("aria-valuenow")).toBe("360");
  }
  expect(release).toHaveBeenCalledWith(7);
  expect(onOpenChange).not.toHaveBeenCalled();
});

it("cancels on mobile pointer-up even before the media change event arrives", () => {
  const commit = vi.fn();
  const media = window.matchMedia("(max-width: 767px)");
  let mobile = false;
  const mock = vi.spyOn(window, "matchMedia").mockImplementation(() => ({ ...media, matches: mobile, addEventListener: vi.fn(), removeEventListener: vi.fn() }));
  try {
    render(<AppSheet title="详情" open onOpenChange={vi.fn()} detailWidth={360} onDetailWidthCommit={commit}>内容</AppSheet>);
    const handle = screen.getByRole("separator");
    Object.assign(handle, { setPointerCapture: vi.fn(), hasPointerCapture: () => true, releasePointerCapture: vi.fn() });
    fireEvent.pointerDown(handle, { button: 0, pointerId: 7, clientX: 500 });
    fireEvent.pointerMove(handle, { pointerId: 7, clientX: 420 });
    mobile = true;
    fireEvent.pointerUp(handle, { pointerId: 7 });
    expect(handle.getAttribute("aria-valuenow")).toBe("360");
    expect(commit).not.toHaveBeenCalled();
  } finally { cleanup(); mock.mockRestore(); }
});

it("preserves ordinary sheets when no detail width is requested", () => {
  render(<AppSheet title="普通面板" open onOpenChange={vi.fn()}>内容</AppSheet>);
  const dialog = screen.getByRole("dialog", { name: "普通面板" });
  expect(screen.queryByRole("separator")).toBeNull();
  expect(dialog.style.width).toBe("");
  expect(dialog.className).toContain("sm:max-w-[480px]");
});
