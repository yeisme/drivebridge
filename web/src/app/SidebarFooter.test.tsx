import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { SidebarFooter } from "./SidebarFooter";

afterEach(cleanup);
it("does not invent usage and routes the usage entry without an implicit action", () => {
  const usage = vi.fn(), settings = vi.fn(), logout = vi.fn();
  render(<SidebarFooter user={{ id: "fixture-user", name: "Fixture" }} tenant={false} onUsage={usage} onSettings={settings} onLogout={logout} />);
  expect(screen.getByText("占用未知 / 配额未提供")).toBeTruthy();
  expect(screen.getByText("未选择空间")).toBeTruthy();
  expect(screen.getByText("个人空间")).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "空间用量" }));
  expect(usage).toHaveBeenCalledTimes(1);
  expect(settings).not.toHaveBeenCalled();
  expect(logout).not.toHaveBeenCalled();
});
