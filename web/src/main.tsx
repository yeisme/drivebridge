import { createRoot } from "react-dom/client";
import { AppearanceProvider, initializeAppearance } from "./appearance";
import { SharedView } from "./Sharing";
import { ScopedWorkspace } from "./app/ScopedWorkspace";
import App from "./WorkspaceApp";
import { DesktopWorkspace } from "./features/notebook/DesktopWorkspace";
import { isDesktopEntry } from "./app/desktop-entry";

// Keep the legacy default export while the production entry only bootstraps.
export default App;
const root = document.getElementById("root");
if (root) {
  initializeAppearance();
  createRoot(root).render(
    <AppearanceProvider>
      {isDesktopEntry(location)
        ? <DesktopWorkspace bridge={window.drivebridgeDesktop} />
        : location.hash.startsWith("#share/")
        ? <SharedView />
        : <ScopedWorkspace component={App} />}
    </AppearanceProvider>,
  );
}
