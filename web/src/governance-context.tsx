import React, { useEffect, useState } from "react";
import { api, getDataContext, setDataContext, logout, type Space } from "./api";
import { governanceURL } from "./workspace-route";
import { useMobileViewport } from "./app/useMobileViewport";
import { purgeLocalFlowState } from "./consumer-flows/compat";
import { SelectAdapter } from "./components/SelectAdapter";
import { Button, buttonVariants } from "./components/ui/button";
import { Popover, PopoverTrigger, PopoverContent } from "./components/ui/popover";
import { SlidersHorizontal, ShieldCheck, LogOut, KeyRound, Plug, Bot } from "lucide-react";

interface Tenant {
  id: string;
  name: string;
  disabled: boolean;
}
interface Member {
  id: string;
  user_id: string;
  tenant_id: string;
  role: string;
  status: string;
}
interface Team {
  id: string;
  tenant_id: string;
  name: string;
}
interface RequestItem {
  file_id: string;
  version_id: string;
  action: string;
  path?: string;
  source_file_id?: string;
  source_version_id?: string;
  status?: string;
  error?: string;
  result_id?: string;
}
interface Request {
  id: string;
  requester: string;
  approver: string;
  space_id: string;
  reason: string;
  kind: string;
  action: string;
  prefix: string;
  status: string;
  grant_until: string;
  expires_at: string;
  items: RequestItem[];
}
interface Notice {
  id: string;
  action: string;
  actor: string;
  resource_id: string;
  read: boolean;
  created_at: string;
}
export interface GovernanceState {
  enabled: boolean;
  actor_id: string;
  tenant_id: string;
  platform_admin: boolean;
  tenant_admin: boolean;
  tenants: Tenant[];
  memberships: Member[];
  members: Member[];
  users: { id: string; name: string }[];
  teams: Team[];
  team_members: { team_id: string; user_id: string; role: string }[];
  spaces: Space[];
  space_roles: Record<string, string>;
  permissions: { space_id: string; user_id: string; role: string }[];
  requests: Request[];
  approvable: Record<string, boolean>;
  notifications: Notice[];
  events: {
    id: string;
    actor: string;
    action: string;
    space_id: string;
    outcome: string;
    at: string;
  }[];
  recoveries: { id: string; user_id: string; status: string; kind: string }[];
  platform_roles: { user_id: string; suspended: boolean }[];
}
const roleLabels: Record<string, string> = {
  owner: "本人空间",
  admin: "管理只读",
  custodian: "数据负责人",
  editor: "已授权编辑",
  viewer: "只读协作",
};
export function useGovernance(user?: string) {
  const [data, setData] = useState<GovernanceState>();
  useEffect(() => {
    if (!user) return;
    let alive = true;
    const load = () =>
      api<GovernanceState>("/governance")
        .then((d) => {
          if (alive) setData(d.enabled ? d : undefined);
        })
        .catch(() => {
          if (alive) setData(undefined);
        });
    void load();
    const changed = () => void load();
    window.addEventListener("drivebridge-governance-changed", changed);
    const timer = setInterval(() => void load(), 15000);
    return () => {
      alive = false;
      window.removeEventListener("drivebridge-governance-changed", changed);
      clearInterval(timer);
    };
  }, [user]);
  return data;
}
export function governanceMode(data?: GovernanceState) {
  const space = getDataContext().space;
  if (!data?.enabled || !space) return "owner";
  const role = data.space_roles[space];
  if (["owner", "editor", "custodian"].includes(role)) return role;
  const grant = data.requests?.find(
    (r) =>
      r.requester === data.actor_id &&
      r.space_id === space &&
      r.status === "approved" &&
      new Date(r.grant_until).getTime() > Date.now() &&
      (r.kind === "emergency" || r.kind === "range"),
  );
  return grant?.kind === "emergency"
    ? "emergency"
    : grant
      ? "granted"
      : role || "viewer";
}
export function GovernanceBar({
  data,
  children,
  user,
}: {
  data: GovernanceState;
  children?: React.ReactNode;
  user: { id: string; name: string };
}) {
  const [menuOpen, setMenuOpen] = useState(false);
  const mobile = useMobileViewport();
  useEffect(() => {
    const current = getDataContext();
    if (current.space && !data.space_roles[current.space])
      setDataContext(current.tenant);
  }, [data]);
  const tenant = getDataContext().tenant || data.tenant_id;
  const scope = getDataContext().space;
  const mode = governanceMode(data);
  const accessLabel = mode === "emergency" ? "应急 · 15 分钟" : mode === "granted" ? "已授权 · 限定范围" : roleLabels[mode] || "只读";
  const tenantPicker = <label className="tenant-picker">
          <span className="sr-only">租户</span>
          <SelectAdapter
            aria-label="切换租户"
            value={tenant}
            onChange={(e) => {
              const tenant=e.target.value;
              if(location.hash.startsWith("#/governance")) location.hash=governanceURL(new URLSearchParams(location.hash.split("?")[1] || "").get("tab") || "requests",tenant);
              setDataContext(tenant);
            }}
          >
            {data.tenants.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name}
                {t.disabled ? "（停用）" : ""}
              </option>
            ))}
          </SelectAdapter>
        </label>;
  return (
    <>
      <div className="topbar-governance" aria-label="租户与访问上下文">
        {!mobile && tenantPicker}
        <Popover open={menuOpen} onOpenChange={setMenuOpen}>
          <PopoverTrigger render={<Button type="button" variant="ghost"/>} aria-label="工作区选项" title="工作区、管理与体验身份">{mobile ? <><SlidersHorizontal size={17}/><span className="sr-only">{accessLabel}</span></> : scope ? <span className="context-access-state" role="status">{accessLabel}</span> : <><SlidersHorizontal size={17} /><span>工作区</span></>}{data.notifications.some(n => !n.read) && <i aria-label="有未读通知" />}</PopoverTrigger>
          <PopoverContent align="end" aria-label="工作区菜单" className="workspace-menu w-80 max-w-[calc(100vw-2rem)] max-h-[var(--available-height)] overflow-y-auto gap-4 p-4">
        <div className="context-identity [&_strong]:text-sm [&_small]:text-xs"><span className="avatar" aria-hidden="true">{user.name.slice(0,1)}</span><div><strong>{user.name}</strong><small>{accessLabel}</small></div></div>
        {mobile && <div className="grid gap-2 text-xs font-medium text-muted-foreground">当前租户{tenantPicker}</div>}
        <label className="grid gap-2 text-xs font-medium text-muted-foreground">
          工作空间
          <SelectAdapter
            aria-label="切换管理空间"
            value={scope}
            onChange={(e) => setDataContext(tenant, e.target.value)}
          >
            <option value="">我的个人空间</option>
            {data.spaces.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name} · {roleLabels[data.space_roles[s.id]]}
              </option>
            ))}
          </SelectAdapter>
        </label>
        <Button variant="ghost" type="submit" className="w-full justify-start" onClick={() => {setMenuOpen(false); location.hash = governanceURL("requests", tenant, scope);}}>
          <ShieldCheck size={17} />管理与授权中心
          {data.notifications.some((n) => !n.read) ? " · 有未读" : ""}
        </Button>
        <nav className="grid gap-1" aria-label="工作区快捷入口">
          {[["users","我的登录 Key",KeyRound],["connections","我的存储连接",Plug],["agents","我的机器人授权",Bot]].map(([route,title,Icon])=>{const Glyph=Icon as typeof KeyRound;return <a key={String(route)} className={buttonVariants({variant:"ghost",className:"w-full justify-start"})} href={`#/${route}?tenant=${encodeURIComponent(tenant)}`} onClick={()=>setMenuOpen(false)}><Glyph/>{String(title)}</a>;})}
        </nav>
        {children}
        <div className="border-t border-border pt-3"><Button variant="ghost" type="button"
          className="w-full justify-start text-muted-foreground"
          onClick={() => void (async () => { await purgeLocalFlowState(user.id); await logout(); })()}
        >
          <LogOut size={17} />退出当前账号
        </Button></div>
          </PopoverContent>
        </Popover>
      </div>
    </>
  );
}
