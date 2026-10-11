import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { RouteBoundary } from "./RouteBoundary";

function Broken(): never { throw new Error("private diagnostic fixture"); }
afterEach(() => { cleanup(); vi.restoreAllMocks(); });
it("isolates page render errors without disclosing the exception or removing navigation", () => {
  vi.spyOn(console, "error").mockImplementation(() => {});
  render(<><nav>工作区导航</nav><RouteBoundary scope="files"><Broken /></RouteBoundary></>);
  expect(screen.getByRole("navigation")).toBeTruthy();
  expect(screen.getByRole("alert").textContent).toContain("此页面暂时无法显示");
  expect(document.body.textContent).not.toContain("private diagnostic fixture");
});
it("recovers on explicit retry and resets a failed page when scope changes", () => {
  vi.spyOn(console, "error").mockImplementation(() => {});
  const view = render(<RouteBoundary scope="files"><Broken /></RouteBoundary>);
  view.rerender(<RouteBoundary scope="files"><p>读取结果</p></RouteBoundary>);
  expect(screen.queryByText("读取结果")).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "重试显示页面" }));
  expect(screen.getByText("读取结果")).toBeTruthy();
  view.rerender(<RouteBoundary scope="files"><Broken /></RouteBoundary>);
  expect(screen.getByRole("alert")).toBeTruthy();
  view.rerender(<RouteBoundary scope="other-user:tasks"><p>新范围</p></RouteBoundary>);
  expect(screen.getByText("新范围")).toBeTruthy();
  expect(screen.queryByRole("alert")).toBeNull();
});
