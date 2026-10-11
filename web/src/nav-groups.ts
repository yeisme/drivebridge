export type NavGroupId = "workbench" | "agents" | "system";

export const NAV_GROUPS: {
  id: NavGroupId;
  label: string;
  items: string[];
}[] = [
  {
    id: "workbench",
    label: "工作台",
    items: ["overview", "insights", "files", "knowledge", "resources", "collections", "properties", "search", "flows", "transfers"],
  },
  { id: "agents", label: "Agent", items: ["inbox", "agents"] },
  {
    id: "system",
    label: "管理",
    items: ["connections", "devices", "automation", "trash", "capabilities", "shares", "teams", "members", "governance", "settings"],
  },
];

// Count only the loaded projection, not a server-wide total. Repeated rows
// from recovery refreshes must not inflate the navigation indicator.
export function pendingNavigationTasks(operations: readonly { id: string; status: string }[]): number {
  return new Set(operations.filter(operation => !["completed", "cancelled"].includes(operation.status)).map(operation => operation.id)).size;
}

export function navGroupsKey(instance: string, user: string) {
  return instance && user ? `drivebridge:nav-groups:${encodeURIComponent(instance)}:${encodeURIComponent(user)}` : "";
}

export function readNavGroups(key: string): Record<string, boolean> {
  if (!key) return {};
  try {
    const raw: unknown = JSON.parse(localStorage.getItem(key) || "");
    if (!raw || typeof raw !== "object") return {};
    return Object.fromEntries(
      Object.entries(raw as Record<string, unknown>).filter(
        ([id, v]) => NAV_GROUPS.some(group => group.id === id) && typeof v === "boolean",
      ),
    ) as Record<string, boolean>;
  } catch {
    return {};
  }
}

export function writeNavGroups(key: string, value: Record<string, boolean>) {
  if (!key) return;
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {
    /* Preferences are optional. */
  }
}

export function visibleNavGroups(
  itemIds: string[],
  expanded: Record<string, boolean>,
) {
  return NAV_GROUPS.map((group) => ({
    ...group,
    items: group.items.filter((id) => itemIds.includes(id)),
    open: expanded[group.id] !== false,
  })).filter((group) => group.items.length > 0);
}
