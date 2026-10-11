import { useEffect, useState } from "react";

import { readSidebarPreference as readPreference, TABLET_QUERY } from "./sidebar-preference";

export function sidebarPreferenceKey(instance: string, user: string) {
  return instance && user ? `drivebridge:sidebar:${encodeURIComponent(instance)}:${encodeURIComponent(user)}` : "";
}

export function useSidebarPreference(instance: string, user: string) {
  const key = sidebarPreferenceKey(instance, user);
  const [state, setState] = useState(() => ({ key, ...readPreference(key) }));
  useEffect(() => {
    const media = window.matchMedia(TABLET_QUERY);
    const update = () => setState(previous => previous.explicit || previous.collapsed === media.matches
      ? previous : { ...previous, collapsed: media.matches });
    media.addEventListener("change", update);
    update();
    return () => media.removeEventListener("change", update);
  }, []);
  // Reset before children commit, without persisting another identity's value.
  const current = state.key === key ? state : { key, ...readPreference(key) };
  if (state.key !== key) setState(current);
  function toggle() {
    const collapsed = !current.collapsed;
    setState({ key, collapsed, explicit: true });
    if (key) {
      try { localStorage.setItem(key, String(collapsed)); }
      catch { /* Keep the explicit choice for this mounted session. */ }
    }
  }
  function reset() {
    setState({ key, collapsed: window.matchMedia(TABLET_QUERY).matches, explicit: false });
    if (key) {
      try { localStorage.removeItem(key); }
      catch { /* The automatic mode remains usable for this session. */ }
    }
  }
  return [current.collapsed, toggle, reset, current.explicit] as const;
}
