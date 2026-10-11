import { describe, expect, it } from "vitest";
import { pendingNavigationTasks, visibleNavGroups } from "./nav-groups";

describe("visibleNavGroups", () => {
  it("counts unique unfinished loaded tasks without inventing completion", () => {
    expect(pendingNavigationTasks([])).toBe(0);
    expect(pendingNavigationTasks([
      { id: "active", status: "verifying" }, { id: "active", status: "verifying" },
      { id: "failed", status: "failed" }, { id: "unknown", status: "future_state" },
      { id: "done", status: "completed" }, { id: "cancelled", status: "cancelled" },
    ])).toBe(3);
  });
  it("drops groups that have no remaining items after permission filters", () => {
    const groups = visibleNavGroups(["files", "transfers"], {});
    expect(groups.map((g) => g.id)).toEqual(["workbench"]);
    expect(groups.some((g) => g.label === "管理")).toBe(false);
  });
  it("treats missing collapse flags as expanded", () => {
    const groups = visibleNavGroups(["files", "shares", "members"], {});
    expect(groups.every((g) => g.open)).toBe(true);
  });
  it("separates Agent work while retaining existing management entry points", () => {
    const groups = visibleNavGroups(["files", "agents", "inbox", "shares", "teams", "members", "governance"], { system: false, collab: false });
    expect(groups.map(group => group.label)).toEqual(["工作台", "Agent", "管理"]);
    expect(groups.find(group => group.id === "agents")).toMatchObject({ items: ["inbox", "agents"], open: true });
    expect(groups.find(group => group.id === "system")).toMatchObject({ items: ["shares", "teams", "members", "governance"], open: false });
  });
  it("exposes the missing management pages only when the shell supplies them", () => {
    expect(visibleNavGroups(["devices", "automation", "capabilities"], {})[0]).toMatchObject({
      id: "system", items: ["devices", "automation", "capabilities"],
    });
    expect(visibleNavGroups(["files"], {}).flatMap(group => group.items)).not.toContain("devices");
  });
  it("honors an explicit collapsed group", () => {
    const groups = visibleNavGroups(["files", "agents", "members"], {
      system: false,
    });
    expect(groups.find((g) => g.id === "system")?.open).toBe(false);
    expect(groups.find((g) => g.id === "workbench")?.open).toBe(true);
  });
});
