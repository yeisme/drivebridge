import { expect, it } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { notebookLayout, useNotebookLayout } from "./useNotebookLayout";

it("uses workspace width thresholds rather than viewport width", () => {
  expect(notebookLayout(767)).toEqual({ directoryInline: false, propertiesInline: false });
  expect(notebookLayout(1023)).toEqual({ directoryInline: false, propertiesInline: false });
  expect(notebookLayout(1024)).toEqual({ directoryInline: true, propertiesInline: false });
  expect(notebookLayout(1279)).toEqual({ directoryInline: true, propertiesInline: false });
  expect(notebookLayout(1280)).toEqual({ directoryInline: true, propertiesInline: true });
});

it("starts in safe overlay mode before a container is attached", () => {
  const { result, unmount } = renderHook(() => useNotebookLayout());
  act(() => { window.dispatchEvent(new Event("resize")); });
  expect(result.current.directoryInline).toBe(false);
  expect(result.current.propertiesInline).toBe(false);
  unmount();
});
