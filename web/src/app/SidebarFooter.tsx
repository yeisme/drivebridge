import { ChevronDown, Cloud, LogOut, Settings } from "lucide-react";
import { bytes, type Space, type UserIdentity } from "../api";
import { capacityOf } from "../capacity";
import { CapacityBar } from "../CapacityView";
import { UserAvatar } from "../components/UserAvatar";
import { Button } from "../components/ui/button";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "../components/ui/dropdown-menu";

export function SidebarFooter({
  space,
  user,
  tenant,
  activeSettings = false,
  onUsage,
  onSettings,
  onLogout,
}: {
  space?: Space;
  user: UserIdentity;
  tenant: boolean;
  activeSettings?: boolean;
  onUsage: () => void;
  onSettings: () => void;
  onLogout: () => void;
}) {
  const meter = space ? capacityOf(space) : null;
  return (
    <>
      <Button
        layout="surface"
        variant="ghost"
        type="button"
        className="storage-meter mx-2 mb-1.5 mt-0 flex w-auto shrink-0 flex-col rounded-lg p-2.5 text-left text-sidebar-foreground hover:bg-[var(--navigation-hover)] group-data-[collapsed=true]/shell:mx-1 group-data-[collapsed=true]/shell:size-9 group-data-[collapsed=true]/shell:items-center group-data-[collapsed=true]/shell:justify-center group-data-[collapsed=true]/shell:self-center group-data-[collapsed=true]/shell:p-0 group-data-[collapsed=true]/shell:[&>small]:hidden group-data-[collapsed=true]/shell:[&>div>span]:hidden group-data-[collapsed=true]/shell:[&>.capacity-meter]:hidden"
        aria-label="空间用量"
        aria-haspopup="dialog"
        onClick={onUsage}
      >
        <div className="mb-1.5 flex items-center gap-2 text-xs font-medium group-data-[collapsed=true]/shell:mb-0">
          <Cloud size={15} className="shrink-0 text-muted-foreground" />
          <span>空间用量</span>
        </div>
        {space && <CapacityBar space={space} />}
        <small className="storage-space mt-1.5 block truncate text-[11px] font-medium" title={space?.name}>
          {space?.name || "未选择空间"}
        </small>
        <small className="storage-value mt-0.5 block text-[11px] tabular-nums">
          {meter?.occupied === null || !meter ? "占用未知" : bytes(meter.occupied)} /{" "}
          {meter?.quota ? bytes(meter.quota) : "配额未提供"}
        </small>
      </Button>

      <Button
        layout="surface"
        variant="ghost"
        type="button"
        aria-label="个人设置"
        aria-current={activeSettings ? "page" : undefined}
        onClick={onSettings}
        className={`nav-item db-nav-item mx-2 mb-1 flex h-9 min-h-9 w-auto shrink-0 items-center justify-start gap-2 rounded-lg px-2 text-left text-[13px] text-sidebar-foreground hover:bg-[var(--navigation-hover)] aria-[current=page]:bg-[var(--navigation-selected)] aria-[current=page]:font-semibold group-data-[collapsed=true]/shell:mx-1 group-data-[collapsed=true]/shell:size-9 group-data-[collapsed=true]/shell:justify-center group-data-[collapsed=true]/shell:self-center group-data-[collapsed=true]/shell:px-0 group-data-[collapsed=true]/shell:[&>.nav-item-label]:hidden${
          activeSettings ? " active" : ""
        }`}
      >
        <span className="nav-glyph-chip" aria-hidden="true">
          <Settings size={18} />
        </span>
        <span className="nav-item-label truncate">设置与管理</span>
      </Button>

      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              layout="surface"
              type="button"
              variant="ghost"
              className="profile mx-2 mb-2 flex h-auto w-auto shrink-0 items-center justify-start gap-2.5 rounded-lg border-0 p-2 text-sidebar-foreground hover:bg-[var(--navigation-hover)] group-data-[collapsed=true]/shell:mx-1 group-data-[collapsed=true]/shell:size-10 group-data-[collapsed=true]/shell:justify-center group-data-[collapsed=true]/shell:self-center group-data-[collapsed=true]/shell:p-0 group-data-[collapsed=true]/shell:[&>span:not(.avatar)]:hidden group-data-[collapsed=true]/shell:[&>svg]:hidden"
              aria-label="账户菜单"
            >
              <UserAvatar user={user} />
              <span className="min-w-0 flex-1 [&>strong]:truncate">
                <strong>{user.display_name || user.name}</strong>
                <small>{tenant ? "当前工作空间" : "个人空间"}</small>
              </span>
              <ChevronDown size={15} />
            </Button>
          }
        />
        <DropdownMenuContent side="top" align="start">
          <DropdownMenuItem onClick={onSettings}>
            <Settings size={16} />
            设置与空间管理
          </DropdownMenuItem>
          <DropdownMenuItem onClick={onLogout}>
            <LogOut size={16} />
            退出登录
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </>
  );
}
