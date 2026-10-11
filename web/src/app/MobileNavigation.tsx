import { Bot, Files, House, ListTodo, UserRound } from "lucide-react";
import { Button } from "../components/ui/button";
import type { Page } from "./navigation-state";
import styles from "./MobileNavigation.module.css";

const destinations = [
  { page: "overview", label: "首页", icon: House },
  { page: "files", label: "文件", icon: Files },
  { page: "agents", label: "Agent", icon: Bot },
  { page: "transfers", label: "任务", icon: ListTodo },
  { page: "settings", label: "我的", icon: UserRound },
] as const;

interface MobileNavigationProps {
  page: Page;
  onNavigate: (page: Page) => void;
  allowed: readonly Page[];
}

// Visibility is supplied by the owning shell; navigation never grants access.
export function MobileNavigation({ page, onNavigate, allowed }: MobileNavigationProps) {
  return <nav className={styles.navigation} aria-label="手机主导航">
    {destinations.map(({ page: target, label, icon: Icon }) => {
      const enabled = allowed.includes(target);
      const active = page === target || (target === "transfers" && page === "inbox");
      return <Button layout="surface" variant="ghost" key={target} type="button" disabled={!enabled}
        aria-current={active ? "page" : undefined}
        onClick={() => { if (enabled) onNavigate(target); }}>
        <Icon size={20} strokeWidth={1.75} aria-hidden="true" />
        <span>{label}</span>
      </Button>;
    })}
  </nav>;
}
