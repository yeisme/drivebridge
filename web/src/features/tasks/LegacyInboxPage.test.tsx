import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { Operation } from "../../api";
import { LegacyInboxPage } from "./LegacyInboxPage";

const op: Operation = { id: "original-op", kind: "upload", space_id: "fixture-space", path: "fixture.txt", status: "completed", size_bytes: 10, bytes_transferred: 10, sha256: "fixture", file_id: "fixed-file", version_id: "fixed-version" };
const props = { scope: "fixture:inbox", busy: false, onOpen: vi.fn(), onCopy: vi.fn(), onTransfers: vi.fn() };
afterEach(() => { cleanup(); localStorage.clear(); vi.clearAllMocks(); });
it("passes original fixed-version results to preview and copy", () => {
  render(<LegacyInboxPage {...props} operations={[op]} />);
  fireEvent.click(screen.getByRole("button", { name: "预览并选版本" }));
  fireEvent.click(screen.getByRole("button", { name: "复制引用" }));
  expect(props.onOpen).toHaveBeenCalledWith(op);
  expect(props.onCopy).toHaveBeenCalledWith(op);
  fireEvent.click(screen.getByRole("button", { name: "查看传输记录" }));
  expect(props.onTransfers).toHaveBeenCalledTimes(1);
  expect(screen.queryByRole("button", { name: "查看恢复" })).toBeNull();
});
it("does not offer a fixed reference for incomplete or reference-less results", () => {
  const view = render(<LegacyInboxPage {...props} operations={[{ ...op, status: "running" }]} />);
  expect(screen.queryByRole("button", { name: "复制引用" })).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "查看恢复" }));
  expect(props.onTransfers).toHaveBeenCalledTimes(1);
  view.rerender(<LegacyInboxPage {...props} operations={[{ ...op, version_id: undefined }]} />);
  expect(screen.queryByRole("button", { name: "复制引用" })).toBeNull();
  expect(screen.queryByRole("button", { name: "预览并选版本" })).toBeNull();
});
it("keeps the empty result recoverable through the original task page", () => {
  render(<LegacyInboxPage {...props} operations={[]} />);
  expect(screen.getByText("最近没有产物")).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "查看传输记录" }));
  expect(props.onTransfers).toHaveBeenCalledTimes(1);
});
