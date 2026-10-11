import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { MobileNavigation } from "./MobileNavigation";
import type { Page } from "./navigation-state";

const allowed: Page[] = ["overview", "files", "agents", "transfers", "settings"];
afterEach(cleanup);
it("uses five stable destinations and delegates without owning route state", () => {
  const navigate = vi.fn();
  render(<MobileNavigation page="files" allowed={allowed} onNavigate={navigate} />);
  expect(screen.getByRole("navigation", { name: "手机主导航" })).toBeTruthy();
  expect(screen.getAllByRole("button").map(button => button.textContent)).toEqual(["首页", "文件", "Agent", "任务", "我的"]);
  expect(screen.getByRole("button", { name: "文件" }).getAttribute("aria-current")).toBe("page");
  fireEvent.click(screen.getByRole("button", { name: "Agent" }));
  expect(navigate).toHaveBeenCalledWith("agents");
  expect(screen.getByRole("button", { name: "文件" }).getAttribute("aria-current")).toBe("page");
});
it("does not navigate to a disallowed surface", () => {
  const navigate = vi.fn();
  render(<MobileNavigation page="inbox" allowed={allowed.filter(page => page !== "agents")} onNavigate={navigate} />);
  const agent = screen.getByRole("button", { name: "Agent" }) as HTMLButtonElement;
  expect(agent.disabled).toBe(true);
  fireEvent.click(agent);
  expect(navigate).not.toHaveBeenCalled();
  expect(screen.getByRole("button", { name: "任务" }).getAttribute("aria-current")).toBe("page");
});
