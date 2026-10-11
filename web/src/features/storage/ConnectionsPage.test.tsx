import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { ConnectionsPage } from "./ConnectionsPage";

const base = { id: "fixture-s3", kind: "s3", name: "本地存储", disabled: false, status: "verified" };
const callbacks = { onCheck: vi.fn(), onBrowse: vi.fn(), onUse: vi.fn(), onDisconnect: vi.fn(), onAdd: vi.fn() };
afterEach(() => { cleanup(); vi.clearAllMocks(); });
it("routes each action with its original connection without requesting a provider", () => {
  const cloud = { ...base, id: "fixture-drive", kind: "onedrive", name: "个人网盘" };
  const fetcher = vi.spyOn(globalThis, "fetch");
  try {
    render(<ConnectionsPage {...callbacks} connections={[base, cloud]} />);
    const card = screen.getByRole("heading", { name: base.name }).closest("article")!;
    fireEvent.click(within(card).getByRole("button", { name: "验证连接" }));
    fireEvent.click(within(card).getByRole("button", { name: "用于空间" }));
    fireEvent.click(within(card).getByRole("button", { name: "断开连接" }));
    fireEvent.click(screen.getByRole("button", { name: "浏览文件" }));
    expect(callbacks.onCheck).toHaveBeenCalledWith(base);
    expect(callbacks.onUse).toHaveBeenCalledWith(base);
    expect(callbacks.onDisconnect).toHaveBeenCalledWith(base);
    expect(callbacks.onBrowse).toHaveBeenCalledWith(cloud);
    expect(fetcher).not.toHaveBeenCalled();
  } finally { fetcher.mockRestore(); }
});
it("prevents disconnected actions and keeps platform storage non-disconnectable", () => {
  render(<ConnectionsPage {...callbacks} connections={[{ ...base, id: "platform" }, { ...base, disabled: true, name: "断开的存储" }]} />);
  expect(screen.queryByRole("button", { name: "断开连接" })).toBeNull();
  const card = screen.getByRole("heading", { name: "断开的存储" }).closest("article")!;
  for (const button of within(card).getAllByRole("button")) {
    expect((button as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(button);
  }
  expect(callbacks.onCheck).not.toHaveBeenCalled();
  expect(callbacks.onUse).not.toHaveBeenCalled();
});
it("does not label stored configuration or an old observation as current connectivity", () => {
  render(<ConnectionsPage {...callbacks} connections={[{ ...base, status: "connected" }, { ...base, id: "checked", name: "先前验证", status: "verified" }]} />);
  expect(screen.queryByText("已连接")).toBeNull();
  expect(screen.getByText("已配置 · 未验证")).toBeTruthy();
  expect(screen.getByText("验证记录：通过")).toBeTruthy();
});
it("keeps the add entry reachable without any existing connection", () => {
  render(<ConnectionsPage {...callbacks} connections={[]} />);
  fireEvent.click(screen.getByRole("button", { name: /连接你已有的存储/ }));
  expect(callbacks.onAdd).toHaveBeenCalledTimes(1);
});
