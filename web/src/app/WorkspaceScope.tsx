import { createContext, useContext } from "react";

export interface WorkspaceScope {
  readonly tenant: string;
  readonly space: string;
  readonly revision: number;
}

// A scope projection, never an authorization decision or credential store.
export const WorkspaceScopeContext = createContext<WorkspaceScope | null>(null);

export function useWorkspaceScope(): WorkspaceScope {
  const scope = useContext(WorkspaceScopeContext);
  if (!scope) throw new Error("workspace_scope_provider_required");
  return scope;
}
