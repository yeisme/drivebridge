import { getDataContext } from "./data-context";

// Shared by navigation and the lazy compatibility page; no UI dependency.
export function teamURL(id = "", tab = "overview", tenant = getDataContext().tenant) {
  return "#/teams" + (id ? "/" + encodeURIComponent(id) + "/" + tab : "") +
    (tenant ? "?tenant=" + encodeURIComponent(tenant) : "");
}
