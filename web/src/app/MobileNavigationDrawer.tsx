import type { ReactNode } from "react";
import { LogOut } from "lucide-react";
import { AppSheet } from "../components/AppSheet";
import { Button } from "../components/ui/button";
import { ThemeToggle } from "../components/ThemeToggle";
import type { visibleNavGroups } from "../nav-groups";
import type { Page } from "./navigation-state";

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  groups: ReturnType<typeof visibleNavGroups>;
  items: readonly [Page, string, ReactNode][];
  page: Page;
  onNavigate: (page: Page) => void;
  onLogout: () => void;
}

export function MobileNavigationDrawer(p: Props) {
  return <AppSheet title="全部导航" open={p.open} onOpenChange={p.onOpenChange}>
    <nav aria-label="完整工作区导航" className="flex flex-col gap-2">
      {p.groups.map(group => <section key={group.id} aria-labelledby={`mobile-nav-${group.id}`} className="flex flex-col gap-1">
        <h2 id={`mobile-nav-${group.id}`} className="px-3 pt-3 text-xs font-semibold text-muted-foreground">{group.label}</h2>
        {group.items.map(id => {
          const item = p.items.find(([target]) => target === id);
          if (!item) return null;
          const [target, label, icon] = item;
          return <Button key={target} type="button" variant="ghost" className="min-h-11 justify-start" aria-current={p.page === target ? "page" : undefined}
            onClick={() => { p.onNavigate(target); p.onOpenChange(false); }}>{icon}{label}</Button>;
        })}
      </section>)}
      <div className="flex items-center justify-between border-t border-border px-3 pt-2 [&>button]:size-11"><span className="text-sm text-muted-foreground">外观</span><ThemeToggle /></div>
      <Button type="button" variant="ghost" className="min-h-11 justify-start" onClick={p.onLogout}><LogOut size={20} />退出登录</Button>
    </nav>
  </AppSheet>;
}
