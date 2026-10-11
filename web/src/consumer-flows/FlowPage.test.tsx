import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { FlowPage, flowPageRoute } from "./FlowPage";
import { workspaceRoute } from "../workspace-route";

vi.mock("./ConsumerFlows", () => ({
  CENTERS: ["upload", "search", "organize", "deliver"].map(id => ({ id })),
  ConsumerFlows: ({ initialCenter, onCenterChange }: { initialCenter: string; onCenterChange: (center: string) => void }) =>
    <div><output>{initialCenter}</output><button onClick={() => onCenterChange("upload")}>upload</button></div>,
}));
vi.mock("./SearchWorkspace", () => ({ SearchWorkspace: () => <section aria-label="search-only">Search</section> }));
afterEach(() => { cleanup(); history.replaceState(null, "", "/"); });

it("round-trips both search routes without losing encoded tenant context", () => {
  for (const page of ["flows", "search"] as const) {
    const route = flowPageRoute(page, "search", "tenant / one");
    expect(workspaceRoute(route.hash)).toMatchObject({ page, tenant: "tenant / one" });
    expect(route.hash).not.toContain("undefined");
  }
});
it("separates canonical search from legacy flows without losing the surrounding URL", () => {
  history.replaceState(null, "", "/workspace?entry=fixture#/search");
  const onNavigate = vi.fn();
  const props = { tenant: "tenant-a", user: { id: "user-a", name: "Fixture" }, spaces: [], onNavigate };
  const view = render(<FlowPage {...props} page="search" tab="unknown" />);
  expect(screen.getByRole('region', { name: 'search-only' })).toBeTruthy();
  expect(screen.queryByRole('button', { name: 'upload' })).toBeNull();
  fireEvent.click(screen.getByRole('button', { name: '打开文件流程' }));
  expect(location.pathname + location.search).toBe("/workspace?entry=fixture");
  expect(workspaceRoute(location.hash)).toMatchObject({ page: "flows", tab: "upload", tenant: "tenant-a" });
  expect(onNavigate).toHaveBeenCalledWith("flows", "upload");
  view.rerender(<FlowPage {...props} page="flows" tab="unknown" />);
  expect(screen.getByRole("status").textContent).toBe("upload");
  view.rerender(<FlowPage {...props} page="flows" tab="search" />);
  expect(screen.getByRole("status").textContent).toBe("search");
});
