import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { Agent } from "../../api";
import { LegacyAgentPage } from "./LegacyAgentPage";

const agent: Agent = { id: "fixture-agent", name: "旧授权", space_id: "space-a", prefix: "docs/", scopes: "read", expires_at: "2099-01-01T00:00:00Z", revoked: false };
const defaults = { agents: [agent], audits: [], spaces: [], ownerLabel: "当前空间", query: "", filter: "all", archived: false, onQuery: vi.fn(), onFilter: vi.fn(), onArchived: vi.fn(), onRevoke: vi.fn(), onArchive: vi.fn() };
afterEach(() => { cleanup(); vi.clearAllMocks(); });

it("delegates actions using the original agent without issuing requests", () => {
  const fetcher = vi.spyOn(globalThis, "fetch");
  try {
    render(<LegacyAgentPage {...defaults} />);
    const row = screen.getByRole("listitem");
    expect(row.textContent).toContain("docs/");
    expect(row.textContent).toContain("当前空间");
    fireEvent.click(within(row).getByRole("button", { name: "撤销授权" }));
    expect(defaults.onRevoke).toHaveBeenCalledWith(agent);
    fireEvent.click(within(row).getByRole("button", { name: "删除并归档" }));
    expect(defaults.onArchive).toHaveBeenCalledWith(agent);
    expect(fetcher).not.toHaveBeenCalled();
  } finally { fetcher.mockRestore(); }
});
it("keeps revoked and archived records non-actionable and filters hidden records", () => {
  const archived = { ...agent, revoked: true, archived: true };
  const view = render(<LegacyAgentPage {...defaults} agents={[archived]} />);
  expect(screen.queryByRole("listitem")).toBeNull();
  view.rerender(<LegacyAgentPage {...defaults} agents={[archived]} archived />);
  const row = screen.getByRole("listitem");
  expect(within(row).queryByRole("button")).toBeNull();
  fireEvent.keyDown(row, { key: "Enter" });
  expect(defaults.onRevoke).not.toHaveBeenCalled();
});
it("preserves the empty page and controlled search callback", () => {
  render(<LegacyAgentPage {...defaults} agents={[]} />);
  expect(screen.getByText("还没有获准的机器人")).toBeTruthy();
  expect(screen.getByText("还没有可展示的活动。")).toBeTruthy();
  fireEvent.change(screen.getByRole("searchbox", { name: "搜索机器人" }), { target: { value: "fixture" } });
  expect(defaults.onQuery).toHaveBeenCalledWith("fixture");
});
