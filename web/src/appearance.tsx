import { createContext, useContext, useEffect, useLayoutEffect, useState, type ReactNode } from "react";

export type Theme = "light" | "dark" | "system";
export type Appearance = { theme: Theme; density: "comfortable" | "compact"; motion: "system" | "reduced" };
export const defaultAppearance: Appearance = { theme: "system", density: "comfortable", motion: "system" };
const keys = { theme: "drivebridge:theme", density: "drivebridge:density", motion: "drivebridge:motion" };

export function readAppearance(): Appearance {
  try {
    const theme = localStorage.getItem(keys.theme), density = localStorage.getItem(keys.density), motion = localStorage.getItem(keys.motion);
    return { theme: theme === "light" || theme === "dark" ? theme : "system", density: density === "compact" ? "compact" : "comfortable", motion: motion === "reduced" ? "reduced" : "system" };
  } catch { return { ...defaultAppearance }; }
}

function systemDark() { return window.matchMedia?.("(prefers-color-scheme: dark)").matches ?? false; }

export function applyAppearance(preferences: Appearance, dark = systemDark()) {
  const resolved = preferences.theme === "system" ? (dark ? "dark" : "light") : preferences.theme;
  const root = document.documentElement;
  root.classList.toggle("dark", resolved === "dark");
  root.dataset.theme = resolved;
  root.dataset.density = preferences.density;
  root.dataset.motion = preferences.motion;
  root.style.colorScheme = resolved;
  return resolved;
}

export function initializeAppearance() { applyAppearance(readAppearance()); }

const AppearanceContext = createContext<{
  preferences: Appearance;
  resolvedTheme: "light" | "dark";
  setAppearance: (patch: Partial<Appearance>) => void;
  storageError: string;
} | null>(null);

export function AppearanceProvider({ children }: { children: ReactNode }) {
  const [preferences, setPreferences] = useState(readAppearance);
  const [dark, setDark] = useState(systemDark);
  const [storageError, setStorageError] = useState("");
  const resolvedTheme = preferences.theme === "system" ? (dark ? "dark" : "light") : preferences.theme;
  useLayoutEffect(() => { applyAppearance(preferences, dark); }, [preferences, dark]);
  useEffect(() => {
    const media = window.matchMedia?.("(prefers-color-scheme: dark)");
    const change = () => setDark(media?.matches ?? false);
    const sync = (event: StorageEvent) => {
      if (event.key === null || Object.values(keys).includes(event.key)) {
        setPreferences(readAppearance());
        setStorageError("");
      }
    };
    media?.addEventListener("change", change);
    window.addEventListener("storage", sync);
    change();
    return () => { media?.removeEventListener("change", change); window.removeEventListener("storage", sync); };
  }, []);
  function setAppearance(patch: Partial<Appearance>) {
    setPreferences(previous => ({ ...previous, ...patch }));
    try {
      for (const key of Object.keys(patch) as (keyof Appearance)[]) localStorage.setItem(keys[key], patch[key]!);
      setStorageError("");
    } catch { setStorageError("设置已在当前页面生效，但浏览器未允许保存；刷新后可能恢复默认。"); }
  }
  return <AppearanceContext.Provider value={{ preferences, resolvedTheme, setAppearance, storageError }}>{children}</AppearanceContext.Provider>;
}

export function useAppearance() {
  const context = useContext(AppearanceContext);
  if (!context) throw new Error("AppearanceProvider is required.");
  return context;
}
