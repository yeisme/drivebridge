import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { AuthGate } from "./AuthGate";

afterEach(cleanup);
const props = () => ({ loading: false, mode: null, busy: false, value: "fixture", error: "", onChange: vi.fn(), onKeyLogin: vi.fn(), onDemoLogin: vi.fn() });

it("renders only the loading state until session resolution", () => {
  render(<AuthGate {...props()} loading mode={{ oidc: true, key: true, demo: true }} />);
  expect(screen.getByText("正在打开你的空间…")).toBeTruthy();
  expect(screen.queryByLabelText("使用密钥登录")).toBeNull();
  expect(screen.queryByRole("link")).toBeNull();
});

it("does not invent login capabilities before discovery", () => {
  render(<AuthGate {...props()} error="身份服务暂不可用" />);
  expect(screen.queryByLabelText("使用密钥登录")).toBeNull();
  expect(screen.queryByRole("link")).toBeNull();
  expect(screen.getByRole("alert").textContent).toBe("身份服务暂不可用");
});

it("keeps legacy key login available and delegates submission", () => {
  const state = props();
  render(<AuthGate {...state} mode={{ oidc: false, key: true }} />);
  fireEvent.submit(screen.getByLabelText("使用密钥登录").closest("form")!);
  expect(state.onKeyLogin).toHaveBeenCalledTimes(1);
  expect(screen.queryByRole("link")).toBeNull();
  expect(screen.queryByText("无需账号，进入本地体验")).toBeNull();
});

it("shows only advertised OIDC and disposable demo entries", () => {
  const state = props();
  render(<AuthGate {...state} mode={{ oidc: true, key: true, demo: true }} />);
  expect(screen.getByRole("link", { name: "使用账号登录" }).getAttribute("href")).toBe("/auth/login");
  fireEvent.click(screen.getByRole("button", { name: "无需账号，进入本地体验" }));
  expect(state.onDemoLogin).toHaveBeenCalledTimes(1);
});
