import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { MobileNavigationDrawer } from "./MobileNavigationDrawer";
import { visibleNavGroups } from "../nav-groups";
import { AppearanceProvider } from "../appearance";

afterEach(cleanup);
it("uses the real sheet, filtered entries and close callback without implicit logout", () => {
  const navigate = vi.fn(), close = vi.fn(), logout = vi.fn();
  render(<AppearanceProvider><MobileNavigationDrawer open onOpenChange={close} groups={visibleNavGroups(["files"], {})} items={[["files", "我的网盘", null], ["governance", "管理与授权", null]]} page="files" onNavigate={navigate} onLogout={logout} /></AppearanceProvider>);
  expect(screen.getByRole("dialog", { name: "全部导航" })).toBeTruthy();
  expect(screen.queryByRole("button", { name: "管理与授权" })).toBeNull();
  const files = screen.getByRole("button", { name: "我的网盘" });
  expect(files.getAttribute("aria-current")).toBe("page");
  fireEvent.click(files);
  expect(navigate).toHaveBeenCalledWith("files");
  expect(close).toHaveBeenCalledWith(false);
  expect(logout).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "退出登录" }));
  expect(logout).toHaveBeenCalledTimes(1);
});
