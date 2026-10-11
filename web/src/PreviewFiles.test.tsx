// @vitest-environment jsdom
import React from "react";
import { afterEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { PreviewFiles } from "./PreviewFiles";
import type { DriveFile } from "./api";
vi.mock("./Media", () => ({ MediaThumbnail: () => <span />, mediaKind: () => "image" }));
afterEach(cleanup);
it("bounds a large file list and locates selection across pages and filters", () => {
  Element.prototype.scrollIntoView = vi.fn();
  const files = Array.from({ length: 1000 }, (_, i) => ({ id: String(i), path: `image-${i}.png`, size_bytes: i, mime_type: "image/png" }) as DriveFile);
  const onSelect = vi.fn();
  render(<PreviewFiles files={files} selected={files[900]} onSelect={onSelect} />);
  const rows = () => document.querySelectorAll(".quicklook-files > button");
  expect(rows().length).toBe(40);
  expect(document.querySelector('[aria-current="true"]')?.getAttribute("title")).toBe("image-900.png");
  fireEvent.click(screen.getByRole("button", { name: "上一组" }));
  expect(document.querySelector('[aria-current="true"]')).toBeNull();
  fireEvent.change(screen.getByRole("textbox", { name: "搜索预览文件" }), { target: { value: "image-999.png" } });
  expect(rows().length).toBe(1);
  fireEvent.click(rows()[0]);
  expect(onSelect).toHaveBeenCalledWith(files[999]);
  fireEvent.click(screen.getByRole("button", { name: "定位当前文件" }));
  expect(rows().length).toBe(40);
  expect(document.querySelector('[aria-current="true"]')?.getAttribute("title")).toBe("image-900.png");
});
