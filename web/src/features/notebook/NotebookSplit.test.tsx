import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { NotebookSplit, notebookSplitRatio } from "./NotebookSplit";
afterEach(() => { cleanup(); vi.restoreAllMocks(); });

it("keeps each wide pane at least 480px", () => {
  expect(notebookSplitRatio(972, 0.25)).toBe(0.5);
  expect(notebookSplitRatio(1212, 0.1)).toBe(0.4);
  expect(notebookSplitRatio(1212, 0.9)).toBe(0.6);
  expect(notebookSplitRatio(390, 0.7)).toBe(0.5);
  expect(notebookSplitRatio(1212, NaN)).toBe(0.5);
});

it("switches narrow panes without unmounting their content or changing the saved ratio", () => {
  let width = 390;
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(() => ({ width, left: 0 } as DOMRect));
  const onRatio = vi.fn();
  render(<NotebookSplit split ratio={0.6} onRatio={onRatio} primary={<textarea aria-label="primary" defaultValue="draft"/>} secondary={<p>secondary</p>}/>);
  const editor = screen.getByLabelText("primary");
  fireEvent.click(screen.getByRole("button", { name: "副窗格" }));
  expect(editor.closest("[hidden]")).not.toBeNull();
  expect(onRatio).not.toHaveBeenCalled();
  width = 1212;
  act(() => { window.dispatchEvent(new Event("resize")); });
  expect(screen.getByLabelText("primary")).toBe(editor);
  expect(editor.closest("[hidden]")).toBeNull();
  expect(screen.getByRole("separator").getAttribute("aria-valuenow")).toBe("60");
  fireEvent.keyDown(screen.getByRole("separator"), { key: "Home" });
  expect(onRatio).toHaveBeenCalledWith(0.4);
});
