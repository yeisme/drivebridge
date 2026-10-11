import type { ComponentProps } from "react";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { TrashPage } from "./TrashPage";

const file = { id: "fixture-file", version_id: "fixed-version", space_id: "fixture-space", path: "fixture.txt", mime_type: "text/plain", size_bytes: 10, trashed: true, updated_at: "2026-01-01T00:00:00Z" };
const props: ComponentProps<typeof TrashPage> = { readState: 'ready', onReload: vi.fn(), spaces: [], space: "fixture-space", retention: 30, search: "", prefix: "", grid: false, busy: false, restricted: false, restorableCount: 1, count: 1, folders: [], visible: [file], picked: { "fixture-file": true }, setPicked: vi.fn(), onSpace: vi.fn(), onSearch: vi.fn(), onPrefix: vi.fn(), onGrid: vi.fn(), onRetention: vi.fn(), onRestoreSelected: vi.fn(), onRestore: vi.fn(), onPreview: vi.fn() };
afterEach(() => { cleanup(); vi.clearAllMocks(); });
it("retains exact restore identity while permanent deletion stays unavailable", () => {
  render(<TrashPage {...props} />);
  fireEvent.click(screen.getByRole("button", { name: "恢复 fixture.txt" }));
  expect(props.onRestore).toHaveBeenCalledWith(file);
  fireEvent.click(screen.getByRole("button", { name: "恢复所选" }));
  expect(props.onRestoreSelected).toHaveBeenCalledTimes(1);
  for (const button of screen.getAllByRole("button", { name: /永久删除/ })) expect((button as HTMLButtonElement).disabled).toBe(true);
});
it("does not expose enabled restore actions to a restricted identity", () => {
  render(<TrashPage {...props} restricted />);
  for (const button of screen.getAllByRole("button", { name: /恢复/ })) {
    expect((button as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(button);
  }
  expect(props.onRestore).not.toHaveBeenCalled();
  expect(props.onRestoreSelected).not.toHaveBeenCalled();
  expect((screen.getByRole("combobox", { name: "回收站自动清理天数" }) as HTMLButtonElement).disabled).toBe(true);
});
it("disables stale selections and duplicate restore actions while busy", () => {
  const view = render(<TrashPage {...props} restorableCount={0} />);
  const batch = screen.getByRole("button", { name: "恢复所选" });
  expect((batch as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(batch);
  expect(props.onRestoreSelected).not.toHaveBeenCalled();
  view.rerender(<TrashPage {...props} busy />);
  for (const button of screen.getAllByRole("button", { name: /恢复/ })) {
    expect((button as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(button);
  }
  expect(props.onRestore).not.toHaveBeenCalled();
});
it.each(['loading', 'error', 'unavailable'] as const)('never displays stale rows or empty success while %s', readState => {
  render(<TrashPage {...props} readState={readState} />);
  expect(screen.queryByText('回收站是空的')).toBeNull();
  expect(screen.queryByRole('button', { name: '恢复 fixture.txt' })).toBeNull();
  expect(screen.getByRole('button', { name: '恢复所选' })).toHaveProperty('disabled', true);
  const reload = screen.getByRole('button', { name: '重读回收站' });
  expect(reload).toHaveProperty('disabled', readState !== 'error');
  if (readState === 'error') { fireEvent.click(reload); expect(props.onReload).toHaveBeenCalledOnce(); expect(screen.getByText(/无法读取回收站/)).toBeTruthy(); }
});
it("distinguishes empty trash from an empty search", () => {
  const view = render(<TrashPage {...props} visible={[]} count={0} />);
  expect(screen.getByText("回收站是空的")).toBeTruthy();
  view.rerender(<TrashPage {...props} visible={[]} count={0} search="missing" />);
  expect(screen.getByText("没有匹配的文件")).toBeTruthy();
});
