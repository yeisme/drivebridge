export type Page =
  | "search" | "flows" | "overview" | "teams" | "members" | "shares" | "insights"
  | "analytics" | "files" | "knowledge" | "resources" | "collections" | "properties" | "transfers" | "connections" | "agents"
  | "users" | "governance" | "settings" | "trash" | "inbox"
  | "devices" | "automation" | "capabilities";

export interface NavigationState { page: Page; tab: string }
export type NavigationAction =
  | { type: "route"; page: Page; tab: string }
  | { type: "page"; page: Page }
  | { type: "tab"; tab: string };

export function navigationReducer(state: NavigationState, action: NavigationAction): NavigationState {
  switch (action.type) {
    case "route": return { page: action.page, tab: action.tab };
    case "page": return { ...state, page: action.page };
    case "tab": return { ...state, tab: action.tab };
  }
}
