import { Moon, Sun } from "lucide-react";
import { Button } from "./ui/button";
import { useAppearance } from "../appearance";

export function ThemeToggle() {
  const { resolvedTheme, setAppearance } = useAppearance();
  const dark = resolvedTheme === "dark";
  return <Button variant="ghost" size="icon" aria-label={dark ? "切换浅色主题" : "切换深色主题"} title="外观设置可在个人设置中调整" onClick={() => setAppearance({ theme: dark ? "light" : "dark" })}>{dark ? <Sun /> : <Moon />}</Button>;
}
