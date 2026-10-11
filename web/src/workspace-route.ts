import type { Page } from "./app/navigation-state";

export function workspacePageURL(page: Page, tenant = "") {
  const canonical: Partial<Record<Page, string>> = { overview: "home", files: "files", transfers: "tasks", connections: "storage" };
  const query = new URLSearchParams();
  if (tenant) query.set("tenant", tenant);
  return `#/${canonical[page] || page}${query.size ? "?" + query : ""}`;
}

export const governanceTabs = ["requests", "notifications", "events", "teams", "accounts"] as const;
export function governanceURL(tab = "requests", tenant = "", space = "") {
  const query = new URLSearchParams({ tab: governanceTabs.includes(tab as typeof governanceTabs[number]) ? tab : "requests" });
  if (tenant) query.set("tenant", tenant);
  if (space) query.set("space", space);
  return "#/governance?" + query;
}

// Legacy aliases stay explicit so shell navigation can change without breaking bookmarks.
export const workspaceAliases = {
  home: { page: "overview", tab: "" },
  tasks: { page: "transfers", tab: "" },
  storage: { page: "connections", tab: "" },
  overview: { page: "overview", tab: "" },
  analytics: { page: "insights", tab: "usage" },
  insights: { page: "insights", tab: "overview" },
  users: { page: "members", tab: "keys" },
  members: { page: "members", tab: "people" },
  drives: { page: "files", tab: "" },
} as const;
export type WorkspaceAlias = keyof typeof workspaceAliases;
export interface WorkspaceRoute {
  page: string;
  tenant: string;
  scope: string;
  space: string;
  team: string;
  tab: string;
}

export function workspaceRoute(hash: string): WorkspaceRoute {
  const empty = {
    page: "",
    tenant: "",
    scope: "",
    space: "",
    team: "",
    tab: "",
  };
  if (!hash.startsWith("#/")) return empty;
  try {
    const url = new URL(hash.slice(1), "http://workspace.invalid");
    const parts = url.pathname.split("/");
    const name = parts[1];
    const tenant = url.searchParams.get("tenant") || "";
    if (name === "drive")
      return {
        page: "files",
        tenant,
        space: decodeURIComponent(parts[2] || ""),
        scope:
          url.searchParams.get("team") ||
          url.searchParams.get("scope") === "managed"
            ? decodeURIComponent(parts[2] || "")
            : "",
        team: url.searchParams.get("team") || "",
        tab: "",
      };
    if (name === "inbox") return { ...empty, page: "inbox", tenant, scope: url.searchParams.get("scope") || "" };
    if (name === "governance") {
      const tab = url.searchParams.get("tab") || "requests";
      return { ...empty, page: "governance", tenant, scope: url.searchParams.get("space") || "", tab: governanceTabs.includes(tab as typeof governanceTabs[number]) ? tab : "requests" };
    }
    if (name === "flows")
      return {
        ...empty,
        page: "flows",
        tenant,
        tab: url.searchParams.get("center") || "",
      };
    if (Object.prototype.hasOwnProperty.call(workspaceAliases, name)) {
      return { ...empty, ...workspaceAliases[name as WorkspaceAlias], tenant };
    }
    return { ...empty, page: name, tenant };
  } catch {
    return empty;
  }
}

export function driveFocusURL(hash: string, focused: boolean) {
  const [route, search = ""] = hash.split("?");
  const query = new URLSearchParams(search);
  if (focused) query.set("view", "drive"); else query.delete("view");
  return route + (query.size ? "?" + query : "");
}
