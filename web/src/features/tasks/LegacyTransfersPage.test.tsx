import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { LegacyTransfersPage } from "./LegacyTransfersPage";
import type { Operation } from "../../api";

const op: Operation = { id: "original-operation", kind: "upload", space_id: "space-a", path: "fixture.txt", size_bytes: 100, bytes_transferred: 100, sha256: "fixture", status: "running" };
const props = { scope: "fixture:user:transfers", busy: false, cancelId: "", onNavigate: vi.fn(), onOpen: vi.fn(), onCopy: vi.fn(), onResume: vi.fn(), onCancelRequest: vi.fn(), onCancelConfirm: vi.fn() };
afterEach(() => { cleanup(); localStorage.clear(); vi.clearAllMocks(); });
it("does not treat sent bytes as publication and retains the original recovery identity", () => {
  const view = render(<LegacyTransfersPage {...props} operations={[op]} />);
  expect(screen.queryByRole("button", { name: "查看结果" })).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "恢复" }));
  expect(props.onResume).toHaveBeenCalledWith(op);
  fireEvent.click(screen.getByRole("button", { name: "取消 fixture.txt" }));
  expect(props.onCancelRequest).toHaveBeenCalledWith(op.id);
  expect(props.onCancelConfirm).not.toHaveBeenCalled();
  view.rerender(<LegacyTransfersPage {...props} cancelId={op.id} operations={[op]} />);
  const confirmation = screen.getByRole("group", { name: "确认取消传输" });
  fireEvent.click(within(confirmation).getByRole("button", { name: "返回" }));
  expect(props.onCancelConfirm).not.toHaveBeenCalled();
  fireEvent.click(within(confirmation).getByRole("button", { name: "确认取消" }));
  expect(props.onCancelConfirm).toHaveBeenCalledWith(op);
});
it("opens only a completed fixed-version result and preserves its reference", () => {
  const completed = { ...op, status: "completed", file_id: "file-a", version_id: "version-a" };
  const view = render(<LegacyTransfersPage {...props} operations={[completed]} />);
  fireEvent.click(screen.getByRole("button", { name: "查看结果" }));
  expect(props.onOpen).toHaveBeenCalledWith(completed);
  fireEvent.click(screen.getByRole("button", { name: "复制引用" }));
  expect(props.onCopy).toHaveBeenCalledWith(completed);
  expect(screen.queryByRole("button", { name: "恢复" })).toBeNull();
  view.rerender(<LegacyTransfersPage {...props} operations={[{ ...op, status: "cancelled" }]} />);
  expect(screen.queryByRole("button", { name: "查看结果" })).toBeNull();
  expect(screen.queryByRole("button", { name: "取消 fixture.txt" })).toBeNull();
});
