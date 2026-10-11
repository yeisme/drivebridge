import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { DocumentConflictMerge } from "./DocumentConflictMerge";

afterEach(cleanup);
const props = { baseVersion: "base-one", base: "original", remoteVersion: "head-two", remote: "shared\nremote\nend", originalDraft: "shared\nlocal\nend", draft: "shared\nmerged\nend", maxBytes: 2097152, onDraft: vi.fn(), onConfirm: vi.fn(), onClose: vi.fn() };
it("requires review of exact draft differences and explicit pinned-base acknowledgement", () => {
  const confirm = vi.fn();
  const view = render(<DocumentConflictMerge {...props} onConfirm={confirm}/>);
  expect(screen.queryByRole("button", { name: "确认新基线并保存新版本" })).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "预览合并差异" }));
  const diff = screen.getByRole("region", { name: "相对新基线的差异" });
  expect(diff).toHaveTextContent("remote"); expect(diff).toHaveTextContent("merged"); expect(diff).not.toHaveTextContent("shared");
  expect(screen.getByRole("button", { name: "确认新基线并保存新版本" })).toBeDisabled();
  fireEvent.click(screen.getByRole("checkbox", { name: /head-two/ }));
  fireEvent.click(screen.getByRole("button", { name: "确认新基线并保存新版本" }));
  expect(confirm).toHaveBeenCalledTimes(1);
  view.rerender(<DocumentConflictMerge {...props} draft="changed after review" onConfirm={confirm}/>);
  expect(screen.queryByRole("button", { name: "确认新基线并保存新版本" })).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "预览合并差异" }));
  expect(screen.getByRole("checkbox", { name: /head-two/ })).not.toBeChecked();
  expect(screen.getByRole("button", { name: "确认新基线并保存新版本" })).toBeDisabled();
});
it("counts UTF-8 bytes but leaves oversize merge text editable", () => {
  const changed = vi.fn();
  render(<DocumentConflictMerge {...props} draft="中文" maxBytes={5} onDraft={changed}/>);
  expect(screen.getByRole("alert")).toHaveTextContent("超过 5 字节");
  expect(screen.getByRole("button", { name: "预览合并差异" })).toBeDisabled();
  expect(screen.getByRole("textbox", { name: "人工合并正文" })).toBeEnabled();
  fireEvent.change(screen.getByRole("textbox", { name: "人工合并正文" }), { target: { value: "中" } });
  expect(changed).toHaveBeenCalledWith("中");
});
