import type { ReactNode } from "react";
import { ChevronDown } from "lucide-react";
import { Button } from "../components/ui/button";
import type { Page } from "./navigation-state";
import type { visibleNavGroups } from "../nav-groups";

interface Props {
  groups: ReturnType<typeof visibleNavGroups>;
  items: readonly [Page, string, ReactNode][];
  page: Page;
  title: string;
  pendingCount: number;
  inviteCount: number;
  onToggle: (id: string, open: boolean) => void;
  onNavigate: (page: Page) => void;
}

// Render only the already-authorized groups supplied by the application.
export function DesktopNavigation(p: Props) {
  return <nav aria-label="工作区导航" style={{ scrollbarWidth: "none" }} className="no-scrollbar mt-1 min-h-0 w-full flex-1 gap-1 overflow-y-auto overflow-x-hidden px-2 pb-2 group-data-[collapsed=true]/shell:px-1">{p.groups.map(group => <div className="nav-group mb-3 flex flex-col gap-0.5" key={group.id} data-open={group.open ? "true" : "false"}>
    <Button layout="surface" variant="ghost" type="button" className="nav-group-toggle flex min-h-9 w-full items-center gap-1.5 rounded-md px-2 text-xs font-medium text-[var(--navigation-muted)] hover:bg-[var(--navigation-hover)] group-data-[collapsed=true]/shell:hidden" aria-expanded={group.open} aria-label={group.label} onClick={() => p.onToggle(group.id, !group.open)}>
      <span>{group.label}</span>
      {!group.open && group.items.includes("transfers") && p.pendingCount > 0 && <span className="nav-count" title="当前已加载列表中的未结束任务" aria-label={`当前已加载列表有 ${p.pendingCount} 个未结束任务`}>{p.pendingCount}</span>}
      {!group.open && group.items.includes(p.page) && <span className="nav-group-current" title={p.title}>{p.title}</span>}
      <ChevronDown size={14} aria-hidden="true" />
    </Button>
    {group.items.map(id => {
      const item = p.items.find(([target]) => target === id);
      if (!item) return null;
      const [target, name, icon] = item;
      return <Button layout="surface" variant="ghost" type="button" key={id} className={"nav-item flex h-9 min-h-9 w-full shrink-0 items-center justify-start gap-2 rounded-md px-2 text-left text-[13px] font-normal text-sidebar-foreground transition-colors hover:bg-[var(--navigation-hover)] aria-[current=page]:bg-[var(--navigation-selected)] aria-[current=page]:font-semibold aria-[current=page]:text-sidebar-foreground [&>svg]:size-[18px] group-data-[collapsed=true]/shell:justify-center group-data-[collapsed=true]/shell:px-0 group-data-[collapsed=true]/shell:[&>span]:hidden" + (p.page === id ? " active" : "")} title={name} aria-label={name} aria-current={p.page === id ? "page" : undefined} onClick={() => p.onNavigate(target)}>
        {icon}<span>{name}</span>
        {id === "transfers" && p.pendingCount > 0 && <i />}
        {id === "teams" && p.inviteCount > 0 && <span className="nav-count">{p.inviteCount}</span>}
      </Button>;
    })}
  </div>)}</nav>;
}
