import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it } from "vitest";
import { WorkspaceShell, WorkspaceMain } from "./WorkspaceShell";

afterEach(cleanup);
it("preserves the direct layout hierarchy and responsive state classes", () => {
  const view = render(<WorkspaceShell mobile={false} collapsed focused={false}>
    <aside aria-label="导航" />
    <WorkspaceMain><h1>工作区</h1></WorkspaceMain>
  </WorkspaceShell>);
  const main = screen.getByRole("main");
  expect(main.parentElement?.className).toContain("sidebar-collapsed");
  expect(main.parentElement?.dataset.collapsed).toBe("true");
  expect(main.previousElementSibling).toBe(screen.getByRole("complementary"));
  view.rerender(<WorkspaceShell mobile collapsed={false} focused>
    <WorkspaceMain><h1>工作区</h1></WorkspaceMain>
  </WorkspaceShell>);
  expect(screen.getByRole("main").parentElement?.className).toContain("mobile-shell drive-focused");
  expect(screen.getAllByRole("main")).toHaveLength(1);
});
