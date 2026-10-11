import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { DesktopNavigation } from "./DesktopNavigation";
import { visibleNavGroups } from "../nav-groups";

afterEach(cleanup);
it("renders only admitted entries and preserves navigation callbacks", () => {
  const navigate = vi.fn();
  const toggle = vi.fn();
  render(<DesktopNavigation groups={visibleNavGroups(["files", "transfers"], { workbench: false })} items={[["files", "我的网盘", null], ["transfers", "交付中心", null], ["governance", "管理与授权", null]]} page="files" title="我的网盘" pendingCount={3} inviteCount={0} onNavigate={navigate} onToggle={toggle} />);
  expect(screen.queryByRole("button", { name: "管理与授权" })).toBeNull();
  expect(screen.getByLabelText("当前已加载列表有 3 个未结束任务").textContent).toBe("3");
  const group = screen.getByRole("button", { name: "工作台" });
  expect(group.getAttribute("aria-expanded")).toBe("false");
  fireEvent.click(group);
  expect(toggle).toHaveBeenCalledWith("workbench", true);
  const files = screen.getByRole("button", { name: "我的网盘" });
  expect(files.getAttribute("aria-current")).toBe("page");
  fireEvent.click(files);
  expect(navigate).toHaveBeenCalledWith("files");
});
