import { ChevronDown, Cloud, LogOut, Settings } from "lucide-react";
import { bytes, type Space, type UserIdentity } from "../api";
import { capacityOf } from "../capacity";
import { CapacityBar } from "../CapacityView";
import { UserAvatar } from "../components/UserAvatar";
import { Button } from "../components/ui/button";
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem } from "../components/ui/dropdown-menu";

export function SidebarFooter({ space, user, tenant, onUsage, onSettings, onLogout }: {
  space?: Space;
  user: UserIdentity;
  tenant: boolean;
  onUsage: () => void;
  onSettings: () => void;
  onLogout: () => void;
}) {
  const meter = space ? capacityOf(space) : null;
  return <>
    <Button layout="surface" variant="ghost" type="button" className="storage-meter mx-2 mb-2 mt-0 shrink-0 rounded-md border-0 p-2 text-left text-sidebar-foreground hover:bg-[var(--navigation-hover)] group-data-[collapsed=true]/shell:mx-0 group-data-[collapsed=true]/shell:[&>small]:hidden group-data-[collapsed=true]/shell:[&>div>span]:hidden group-data-[collapsed=true]/shell:[&>.capacity-meter]:hidden" aria-label="空间用量" aria-haspopup="dialog" onClick={onUsage}>
      <div><Cloud size={18} /><span>空间用量</span></div>
      {space && <CapacityBar space={space} />}
      <small className="storage-space" title={space?.name}>{space?.name || "未选择空间"}</small>
      <small className="storage-value">{meter?.occupied === null || !meter ? "占用未知" : bytes(meter.occupied)} / {meter?.quota ? bytes(meter.quota) : "配额未提供"}</small>
    </Button>
    <DropdownMenu>
      <DropdownMenuTrigger render={<Button layout="surface" type="button" variant="ghost" className="profile mx-2 mb-2 flex h-auto w-auto shrink-0 items-center justify-start gap-2 border-0 p-2 text-sidebar-foreground hover:bg-[var(--navigation-hover)] group-data-[collapsed=true]/shell:mx-0 group-data-[collapsed=true]/shell:justify-center group-data-[collapsed=true]/shell:[&>span:not(.avatar)]:hidden group-data-[collapsed=true]/shell:[&>svg]:hidden" aria-label="账户菜单"><UserAvatar user={user} /><span className="min-w-0 flex-1 [&>strong]:truncate"><strong>{user.display_name || user.name}</strong><small>{tenant ? "当前工作空间" : "个人空间"}</small></span><ChevronDown size={16} /></Button>} />
      <DropdownMenuContent side="top" align="start">
        <DropdownMenuItem onClick={onSettings}><Settings size={16} />个人设置</DropdownMenuItem>
        <DropdownMenuItem onClick={onLogout}><LogOut size={16} />退出登录</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  </>;
}
