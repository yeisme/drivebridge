import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { MobileNavigation } from "./MobileNavigation";
import type { Page } from "./navigation-state";

const allowed: Page[] = ["overview", "files", "inbox", "agents", "transfers", "settings"];
afterEach(cleanup);

it("uses five C-end cloud-drive destinations and delegates without owning route state", () => {
  const navigate = vi.fn();
  render(<MobileNavigation page="files" allowed={allowed} onNavigate={navigate} />);
  expect(screen.getByRole("navigation", { name: "手机主导航" })).toBeTruthy();
  expect(screen.getAllByRole("button").map(button => button.textContent)).toEqual([
    "文件",
    "最近",
    "新建",
    "AI 投递",
    "我的",
  ]);
  expect(screen.getByRole("button", { name: "文件" }).getAttribute("aria-current")).toBe("page");
  fireEvent.click(screen.getByRole("button", { name: "AI 投递" }));
  expect(navigate).toHaveBeenCalledWith("inbox");
  expect(screen.getByRole("button", { name: "文件" }).getAttribute("aria-current")).toBe("page");
});

it("does not navigate to a disallowed surface", () => {
  const navigate = vi.fn();
  render(<MobileNavigation page="agents" allowed={allowed.filter(page => page !== "inbox")} onNavigate={navigate} />);
  const inbox = screen.getByRole("button", { name: "AI 投递" }) as HTMLButtonElement;
  expect(inbox.disabled).toBe(true);
  fireEvent.click(inbox);
  expect(navigate).not.toHaveBeenCalled();
  expect(inbox.getAttribute("aria-current")).toBe("page");
});
