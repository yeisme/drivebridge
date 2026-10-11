import { useSyncExternalStore } from "react";

const query = "(max-width: 767px)";
function subscribe(notify: () => void) {
  const media = window.matchMedia(query);
  media.addEventListener("change", notify);
  return () => media.removeEventListener("change", notify);
}
const snapshot = () => window.matchMedia(query).matches;
export function useMobileViewport() {
  return useSyncExternalStore(subscribe, snapshot, () => false);
}
