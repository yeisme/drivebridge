import { governanceURL } from "./workspace-route";
import { Input } from "./components/ui/input";
import { SearchField } from "./components/SearchField";
import { GridTable } from "./components/DataTable";
import { Button } from "./components/ui/button";
import {CreateIdentity} from "./CreateIdentity";
import React, { lazy, Suspense, useState } from "react";
const UserDirectory = lazy(() => import("./UserDirectory").then(module => ({ default: module.UserDirectory })));
import {
  ArrowRight,
  Folder,
  HardDrive,
  UsersRound,
  Search,
  ShieldCheck,
  ArrowUpDown,
  CheckCircle2,
  AlertCircle,
  KeyRound,
  Plus,
} from "lucide-react";
import {
  bytes,
  type Space,
  type Operation,
  type DriveFile,
  type Agent,
} from "./api";
import { MediaThumbnail } from "./Media";
import { CapacityBar } from "./CapacityView";
import { capacityOf } from "./capacity";
import { type GovernanceState } from "./Governance";

export { DriveHome, filterSpaces } from "./features/files/DriveHome";

export function DashboardOverview({
  onUsage,
  spaces,
  operations,
  agents,
  files,
  spaceName,
  openFiles,
  openTransfers,
  preview,
}: {
  onUsage?: (space?: string) => void;
  spaces: Space[];
  operations: Operation[];
  agents: Agent[];
  files: DriveFile[];
  spaceName?: string;
  openFiles: () => void;
  openTransfers: () => void;
  preview: (f: DriveFile) => void;
}) {
  const used = spaces.length && spaces.every(s => capacityOf(s).occupied !== null) ? spaces.reduce((n, s) => n + capacityOf(s).occupied!, 0) : null,
    quota = spaces.every(s => capacityOf(s).quota !== null) ? spaces.reduce((n, s) => n + s.max_bytes, 0) : null,
    active = operations.filter(
      (o) => !["completed", "cancelled", "failed"].includes(o.status),
    ),
    failed = operations.filter((o) => o.status === "failed"),
    liveAgents = agents.filter(
      (a) => !a.revoked && !a.archived && new Date(a.expires_at) > new Date(),
    );
  return (
    <div className="dashboard-overview">
      <div className="metric-grid">
        {[
          {
            name: "存储用量",
            value: used === null ? "占用未知" : bytes(used),
            detail: quota ? `共 ${bytes(quota)} 空间配额` : "当前可访问空间",
            icon: <HardDrive />,
            onClick: onUsage ? () => onUsage() : openFiles,
          },
          {
            name: "我的网盘",
            value: String(spaces.length),
            detail: `${spaces.filter((s) => s.team_id).length} 个团队网盘`,
            icon: <Folder />,
            onClick: openFiles,
          },
          {
            name: "正在传输",
            value: String(active.length),
            detail: failed.length
              ? `${failed.length} 项需要处理`
              : "当前无失败任务",
            icon: <ArrowUpDown />,
            onClick: openTransfers,
          },
          {
            name: "有效机器人授权",
            value: String(liveAgents.length),
            detail: "仅统计当前可见授权",
            icon: <ShieldCheck />,
          },
        ].map((m) => {
          const body = (
            <>
              <div>
                <span>{m.name}</span>
                {m.icon}
              </div>
              <strong>{m.value}</strong>
              <small>{m.detail}</small>
            </>
          );
          return m.onClick ? (
            <Button layout="surface" variant="ghost" type="button"
              className="metric-card"
              key={m.name}
              onClick={m.onClick}
              aria-label={"查看" + m.name}
            >
              {body}
            </Button>
          ) : (
            <article className="metric-card" key={m.name}>
              {body}
            </article>
          );
        })}
      </div>
      <div className="dashboard-row">
        <section className="surface-panel">
          <header>
            <div>
              <h2>存储分布</h2>
              <p>当前可访问网盘的已用容量</p>
            </div>
            <Button variant="outline" size="sm" type="submit"  onClick={openFiles}>
              查看网盘
              <ArrowRight size={14} />
            </Button>
          </header>
          {spaces.length ? (
            [...spaces].sort((a,b) => (capacityOf(b).occupied || 0) - (capacityOf(a).occupied || 0)).map((s) => (
              <Button layout="surface" variant="ghost" type="button" className="workspace-capacity-row" key={s.id} onClick={() => onUsage?.(s.id)} aria-label={`查看 ${s.name} 空间用量`}>
                <span className="surface-icon">
                  <HardDrive size={18} />
                </span>
                <div>
                  <strong>{s.name}</strong>
                  <CapacityBar space={s} />
                </div>
                <span>
                  {capacityOf(s).occupied === null ? "占用未知" : bytes(capacityOf(s).occupied!)}
                  <small>{capacityOf(s).quota ? ` / ${bytes(s.max_bytes)} 配额` : "配额未提供"}</small>
                </span>
              </Button>
            ))
          ) : (
            <p className="panel-empty">连接存储后显示容量分布。</p>
          )}
        </section>
        <section className="surface-panel">
          <header>
            <div>
              <h2>传输动态</h2>
              <p>最近的传输操作</p>
            </div>
            <Button variant="outline" size="sm" type="submit"  onClick={openTransfers}>
              全部
              <ArrowRight size={14} />
            </Button>
          </header>
          {operations.slice(0, 4).map((o) => (
            <Button layout="surface" variant="ghost" type="button"
              className="operation-summary"
              key={o.id}
              onClick={openTransfers}
            >
              {o.status === "completed" ? (
                <CheckCircle2 size={18} />
              ) : o.status === "failed" ? (
                <AlertCircle size={18} />
              ) : (
                <ArrowUpDown size={18} />
              )}
              <span>
                <strong>{o.path.split("/").pop() || "文件传输"}</strong>
                <small>
                  {bytes(o.size_bytes)} ·{" "}
                  {o.status === "completed"
                    ? "已完成"
                    : o.status === "failed"
                      ? "需要处理"
                      : o.status === "cancelled"
                        ? "已取消"
                        : "传输中"}
                </small>
              </span>
              <ArrowRight size={15} />
            </Button>
          ))}
          {!operations.length && (
            <p className="panel-empty">
              暂无传输记录。<Button type="button" variant="link" onClick={openFiles}>去网盘上传</Button>
            </p>
          )}
        </section>
      </div>
      <section className="surface-panel">
        <header>
          <div>
            <h2>最近文件</h2>
            <p>{spaceName || "当前网盘"} · 点击快速预览</p>
          </div>
          <Button variant="outline" size="sm" type="submit"  onClick={openFiles}>
            打开网盘
            <ArrowRight size={14} />
          </Button>
        </header>
        <div className="recent-media recent-file-list">
          {[...files]
            .filter((f) => !f.trashed)
            .sort((a, b) => b.updated_at.localeCompare(a.updated_at))
            .slice(0, 5)
            .map((f) => (
              <Button layout="surface" variant="ghost" type="button" key={f.id} onClick={() => preview(f)}>
                <MediaThumbnail file={f} />
                <strong>{f.path.split("/").pop()}</strong>
                <small>{bytes(f.size_bytes)}</small><time className="text-xs text-muted-foreground" dateTime={f.updated_at}>{new Date(f.updated_at).toLocaleDateString()}</time>
              </Button>
            ))}
        </div>
        {!files.length && (
          <p className="panel-empty">
            此网盘暂无文件。<Button type="button" variant="link" onClick={openFiles}>打开网盘</Button>
          </p>
        )}
      </section>
    </div>
  );
}

export function IdentityDirectory({
  data,
  user,
  keys,
  openKeys,
  openTeam,
}: {
  data?: GovernanceState;
  user: { id: string; name: string };
  keys: { user: string; name: string; user_name?: string; admin?: boolean }[];
  openKeys: () => void;
  openTeam?: (id: string) => void;
}) {
  const [tab, setTab] = useState("members"),
    [query, setQuery] = useState("");
  const openManagement = () => { location.hash = governanceURL("teams", data?.tenant_id); };
  const [create,setCreate]=useState<"user"|"team">();
  const users = data?.enabled
    ? data.users
    : [
        ...new Map(
          [
            user,
            ...keys.map((k) => ({ id: k.user, name: k.user_name || k.name })),
          ].map((u) => [u.id, u]),
        ).values(),
      ];
  return (
    <>
      <div className="workspace-toolbar">
        <div className="segmented">
          {[
            ["members", "用户"],
            ["teams", "团队"],
          ].map(([id, label]) => (
            <Button layout="surface" variant="ghost" type="button"
              key={id}
              aria-pressed={tab === id}
              onClick={() => setTab(id)}
            >
              {label}
            </Button>
          ))}
        </div>
        <SearchField
            aria-label="搜索用户或团队"
            placeholder={tab === "members" ? "搜索用户、Key 名称或标识…" : "搜索团队…"}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        {data?.platform_admin && <Button variant="default" type="submit"  onClick={()=>setCreate("user")}><Plus size={16}/>新建用户</Button>}
        {(data?.platform_admin || data?.tenant_admin) && <Button variant="outline" type="submit"  onClick={()=>setCreate("team")}><Plus size={16}/>新建团队</Button>}
        {data?.enabled && (
          <Button variant="outline" size="sm" type="submit"  onClick={() => openManagement()}>
            <Plus size={16} />
            租户设置
          </Button>
        )}
      </div>
      {create && <CreateIdentity kind={create} user={user} close={()=>setCreate(undefined)}/>}
      <div className="relationship-strip">
        <span>
          <KeyRound size={16} />
          登录 Key
        </span>
        <ArrowRight size={14} />
        <span>所属用户</span>
        <ArrowRight size={14} />
        <span>团队成员关系</span>
        <ArrowRight size={14} />
        <span>网盘访问权限</span>
      </div>
      {tab === "members" && data?.enabled ? (
        <Suspense fallback={<p role="status">正在加载用户目录…</p>}><UserDirectory key={`${user.id}:${data.tenant_id}`} data={data} user={user} query={query} openKeys={openKeys} openTeam={openTeam} fallbackKeys={keys} /></Suspense>
      ) : tab === "members" ? (
        <div className="surface-panel table-scroll">
          <GridTable headers={[<> 用户 </>,<> 所属团队 </>,<> 账号角色 </>,<> 登录 Key </>]} rows={[...(users
    .filter((u) => `${u.name} ${u.id}`
    .toLowerCase()
    .includes(query.toLowerCase()))
    .map((u) => {
    const teams = data?.team_members.filter((m) => m.user_id === u.id) || [];
    return (({id:String(u.id),cells:[<>
                        <div className="identity-name">
                          <span className="avatar">{u.name.slice(0, 1)}</span>
                          <span>
                            <strong>
                              {u.name}
                              {u.id === user.id ? "（我）" : ""}
                            </strong>
                            <small title={u.id}>{u.id.slice(0, 12)}…</small>
                          </span>
                        </div>
                       </>,<>
                        {teams.length ? (
                          teams.map((m) => (
                            <Button layout="surface" variant="ghost" type="button" className="pill" key={m.team_id} onClick={()=>openTeam?.(m.team_id)}>
                              {data?.teams.find((t) => t.id === m.team_id)
                                ?.name || m.team_id.slice(0, 8)}
                            </Button>
                          ))
                        ) : (
                          <span className="text-muted-foreground">未加入团队</span>
                        )}
                       </>,<>
                        {data?.members.find((m) => m.user_id === u.id)?.role ===
                        "admin"
                          ? "租户管理员"
                          : keys.some((k) => k.user === u.id && k.admin)
                            ? "管理员"
                            : "用户"}
                       </>,<>
                        <Button variant="outline" size="sm" type="submit"  onClick={openKeys}>
                          {keys.filter((k) => k.user === u.id).length} 个可见
                          Key
                          <ArrowRight size={14} />
                        </Button>
                       </>]}));
}))]}/>
        </div>
      ) : (
        <div className="drive-cards">
          {data?.teams
            .filter((t) => t.name.toLowerCase().includes(query.toLowerCase()))
            .map((t) => (
              <article className="drive-card team-card" key={t.id}>
                <span className="surface-icon">
                  <UsersRound />
                </span>
                <h2>{t.name}</h2>
                <p>
                  {data.team_members.filter((m) => m.team_id === t.id).length}{" "}
                  位成员 ·{" "}
                  {data.spaces.filter((s) => s.team_id === t.id).length} 个网盘
                </p>
                <Button variant="outline" type="submit"  onClick={() => openTeam ? openTeam(t.id) : openManagement()}>
                  管理团队
                  <ArrowRight size={15} />
                </Button>
              </article>
            ))}
        </div>
      )}
      {tab === "teams" && !data?.teams.length && (
        <div className="workspace-empty">
          <UsersRound size={36} />
          <h3>尚无团队</h3>
          <p>
            {data?.enabled
              ? "创建团队、添加成员，再分配网盘的访问权限。"
              : "当前为个人空间模式。启用团队服务后，成员和权限会在这里显示。"}
          </p>
          {data?.tenant_admin && (
            <Button variant="outline" type="submit"  onClick={() => openManagement()}>
              创建团队
            </Button>
          )}
        </div>
      )}

    </>
  );
}
