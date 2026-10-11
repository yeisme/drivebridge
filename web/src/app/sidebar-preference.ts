export const TABLET_QUERY = "(min-width: 768px) and (max-width: 1279px)";

export function readSidebarPreference(key: string) {
  try {
    const value = key ? localStorage.getItem(key) : null;
    if (value === "true" || value === "false") return { collapsed: value === "true", explicit: true };
  } catch { /* Restricted storage does not block navigation. */ }
  return { collapsed: window.matchMedia(TABLET_QUERY).matches, explicit: false };
}
