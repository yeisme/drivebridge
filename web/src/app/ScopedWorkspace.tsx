import { type ComponentType, useEffect, useState } from "react";
import { getDataContext, setDataContext } from "./data-context";
import { workspaceRoute } from "../workspace-route";
import { WorkspaceScopeContext } from "./WorkspaceScope";
import { confirmRouteLeave } from "./route-leave";

// Remount the page subtree when scope changes so view state never crosses
// identities. The page owns its data; this boundary holds no file cache.
export function ScopedWorkspace({ component: Page }: { component: ComponentType }) {
  const [epoch, setEpoch] = useState(0);
  const [ready, setReady] = useState(false);
  useEffect(() => {
    let acceptedHash = location.hash;
    const change = () => setEpoch((value) => value + 1);
    const route = (event?: HashChangeEvent) => {
      if (event && location.hash !== acceptedHash && !confirmRouteLeave()) {
        // Capture phase runs before page listeners and scope mutation. Keep
        // history.state, and never remount or fetch the rejected scope.
        history.replaceState(history.state, "", location.pathname + location.search + acceptedHash);
        event.stopImmediatePropagation();
        return;
      }
      acceptedHash = location.hash;
      const next = workspaceRoute(location.hash);
      if (!next.page) return;
      const current = getDataContext();
      const tenant = next.tenant || current.tenant;
      if (current.tenant !== tenant || current.space !== next.scope) {
        setDataContext(tenant, next.scope);
      }
    };
    window.addEventListener("drivebridge-context", change);
    window.addEventListener("hashchange", route, true);
    route();
    setReady(true);
    return () => {
      window.removeEventListener("drivebridge-context", change);
      window.removeEventListener("hashchange", route, true);
    };
  }, []);
  // Child effects may fetch immediately; do not mount under the previous scope.
  if (!ready) return null;
  const scope = getDataContext();
  return <WorkspaceScopeContext.Provider value={scope}><Page key={epoch} /></WorkspaceScopeContext.Provider>;
}
