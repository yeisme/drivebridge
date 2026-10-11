import { it, expect } from "vitest";
import { workspaceRoute, workspacePageURL, governanceURL, driveFocusURL } from "./workspace-route";
import { mediaURL } from "./Media";
import { setDataContext, type DriveFile } from "./api";
it("generates canonical routes while round-tripping tenant scope", () => {
  for (const [page, route] of [["overview", "home"], ["files", "files"], ["transfers", "tasks"], ["connections", "storage"]] as const) {
    const url = workspacePageURL(page, "tenant / one");
    expect(url.startsWith(`#/${route}?`)).toBe(true);
    expect(workspaceRoute(url)).toMatchObject({ page, tenant: "tenant / one" });
  }
  expect(workspacePageURL("settings")).toBe("#/settings");
});

it("round trips management tabs and managed space without changing legacy routes", () => {
  expect(workspaceRoute(governanceURL("events", "tenant a", "managed/space"))).toMatchObject({
    page: "governance", tab: "events", tenant: "tenant a", scope: "managed/space", space: "",
  });
  expect(workspaceRoute("#/governance?tab=unknown").tab).toBe("requests");
  const original = "#/drive/test?tenant=t&team=g&scope=managed";
  expect(driveFocusURL(driveFocusURL(original, true), false)).toBe(original);
  expect(workspaceRoute(driveFocusURL(original, true)).scope).toBe("test");
});

it("maps the flows hash with and without a center parameter", () => {
  // Positive: the center lands in the tab field the shell filters against
  // its known centers.
  expect(workspaceRoute("#/flows?center=search")).toMatchObject({ page: "flows", tab: "search" });
  expect(workspaceRoute("#/flows?center=deliver&tenant=t-a")).toMatchObject({ page: "flows", tab: "deliver", tenant: "t-a" });
  // Negative: no center defaults to empty (the shell falls back to upload);
  // an unknown center is passed through verbatim for the shell to reject —
  // the route layer does not invent a center.
  expect(workspaceRoute("#/flows")).toMatchObject({ page: "flows", tab: "" });
  expect(workspaceRoute("#/flows?center=bogus").tab).toBe("bogus");
  expect(workspaceRoute("#/flows").space).toBe("");
});
it("keeps a team drive context on reload without reinterpreting existing share hashes", () => {
  expect(
    workspaceRoute("#/drive/file-space?tenant=tenant-a&team=team-a"),
  ).toEqual({
    page: "files",
    tenant: "tenant-a",
    team: "team-a",
    space: "file-space",
    scope: "file-space",
    tab: "",
  });
  expect(workspaceRoute("#/teams/t/members?tenant=a")).toMatchObject({
    page: "teams",
    tenant: "a",
    scope: "",
  });
  expect(workspaceRoute("#share/id/grant").page).toBe("");
  expect(workspaceRoute("#/drive/%E0%A4%A").page).toBe("");
});
it("previews the selected team file instead of leaking the active personal context", () => {
  setDataContext("personal-tenant", "personal-space");
  const url = mediaURL({
    id: "file",
    version_id: "version",
    space_id: "team-space",
    tenant_context: "team-tenant",
  } as DriveFile);
  const q = new URL(url, "http://localhost").searchParams;
  expect(q.get("tenant_context")).toBe("team-tenant");
  expect(q.get("space_context")).toBe("team-space");
  expect(q.get("version")).toBe("version");
  setDataContext("");
});

it("preserves explicit managed access for personal drives without changing ordinary links",()=>{expect(workspaceRoute("#/drive/private?tenant=t&scope=managed").scope).toBe("private");expect(workspaceRoute("#/drive/private?tenant=t").scope).toBe("");});

it.each([
  ["home", "overview", ""],
  ["tasks", "transfers", ""],
  ["storage", "connections", ""],
  ["overview", "overview", ""],
  ["analytics", "insights", "usage"],
  ["insights", "insights", "overview"],
  ["users", "members", "keys"],
  ["members", "members", "people"],
  ["drives", "files", ""],
])("preserves the %s legacy alias and tenant", (alias, page, tab) => {
  expect(workspaceRoute(`#/${alias}?tenant=tenant%20one`)).toEqual({
    page, tab, tenant: "tenant one", scope: "", space: "", team: "",
  });
});

it("does not interpret inherited object properties as aliases", () => {
  for (const page of ["constructor", "toString", "__proto__", "future-page"]) {
    expect(workspaceRoute(`#/${page}?tenant=t`)).toMatchObject({ page, tenant: "t", tab: "" });
  }
});

it("maps legacy analytics and users hashes onto merged tabs", () => {
  expect(workspaceRoute("#/analytics")).toMatchObject({
    page: "insights",
    tab: "usage",
  });
  expect(workspaceRoute("#/insights")).toMatchObject({
    page: "insights",
    tab: "overview",
  });
  expect(workspaceRoute("#/users")).toMatchObject({
    page: "members",
    tab: "keys",
  });
  expect(workspaceRoute("#/members")).toMatchObject({
    page: "members",
    tab: "people",
  });
});
