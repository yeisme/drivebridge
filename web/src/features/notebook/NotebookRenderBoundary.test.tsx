import { afterEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { NotebookRenderBoundary } from "./NotebookRenderBoundary";
afterEach(() => { cleanup(); vi.restoreAllMocks(); });

it("keeps source as inert text and allows a successful retry", () => {
  vi.spyOn(console, "error").mockImplementation(() => {});
  let fail = true;
  function Child() { if (fail) throw new Error("Synthetic render failure"); return <p>Rendered</p>; }
  const body = '<script>unsafe()</script>\n# Source';
  const { container } = render(<NotebookRenderBoundary body={body}><Child/></NotebookRenderBoundary>);
  expect(screen.getByLabelText("保留的 Markdown 源码").textContent).toBe(body);
  expect(container.querySelector("script")).toBeNull();
  fail = false;
  fireEvent.click(screen.getByRole("button", { name: "重试渲染" }));
  expect(screen.getByText("Rendered")).toBeTruthy();
  expect(screen.queryByRole("alert")).toBeNull();
});

it("resets a failed render for changed content", () => {
  vi.spyOn(console, "error").mockImplementation(() => {});
  function Broken() { throw new Error("Synthetic render failure"); return null; }
  const view = render(<NotebookRenderBoundary body="old"><Broken/></NotebookRenderBoundary>);
  expect(screen.getByRole("alert")).toBeTruthy();
  view.rerender(<NotebookRenderBoundary body="new"><p>New content</p></NotebookRenderBoundary>);
  expect(screen.getByText("New content")).toBeTruthy();
  expect(screen.queryByLabelText("保留的 Markdown 源码")).toBeNull();
});
