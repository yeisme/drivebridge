import { DirectoryLayout, useDirectoryQuery } from "./components/DirectoryLayout";
import { Checkbox } from "./components/ui/checkbox";
import { Textarea } from "./components/ui/textarea";
import { Progress } from "./components/ui/progress";
import { Input } from "./components/ui/input";
import { SearchField } from "./components/SearchField";
import { GridTable } from "./components/DataTable";
import { Button } from "./components/ui/button";
import { SelectAdapter } from "./components/SelectAdapter";
import React, { useEffect, useState } from "react";
import {
  ArrowLeft,
  ArrowRight,
  UsersRound,
  Folder,
  Settings,
  ShieldCheck,
  Activity,
  LayoutGrid,
  Plus,
  PanelLeft,
  Archive,
  RefreshCw,
  Search,
  KeyRound,
  Copy,
} from "lucide-react";
import {
  api,
  bytes,
  errorText,
  getDataContext,
  setDataContext,
  type Space,
  type DriveFile,
  type Connection,
  type Agent,
} from "./api";
import { AppDialog as WorkspaceDialog } from "./components/AppDialog";
import { MediaThumbnail, MediaPreview } from "./Media";
import type { GovernanceState } from "./Governance";
import "./teams.css";

type Team = {
  id: string;
  tenant_id: string;
  name: string;
  description: string;
  archived: boolean;
  revision: number;
  updated_at: string;
};
type Action = { allowed: boolean; reason?: string };
type Summary = {
  team: Team;
  role: string;
  member_count: number;
  space_count: number;
  used_bytes: number;
  quota_bytes: number;
  pending_invitations: number;
  actions: Record<string, Action>;
};
type Member = { user_id: string; name: string; role: string; status: string };
type Invitation = {
  id: string;
  team_id: string;
  tenant_id?: string;
  user_id: string;
  inviter?: string;
  status: string;
  expires_at: string;
  user_name?: string;
  inviter_name?: string;
  membership_status?: string;
  next_step?: string;
};
type Detail = Summary & {
  space_roles: Record<string, string>;
  can_grant: Record<string, boolean>;
  permissions: { space_id: string; user_id: string; role: string }[];
  recent_files: DriveFile[];
  agents: Agent[];
  pending_requests: {
    id: string;
    requester: string;
    space_id: string;
    reason: string;
    kind: string;
    action: string;
  }[];
  connections: Connection[];
  storage_candidates: Connection[];
};
type Paged<T> = { items: T[]; next_page: number };
type Impact = {
  revision: number;
  members: number;
  spaces: Space[];
  shares: number;
  agents: number;
  operations: number;
  blockers: string[];
};
const tabs = [
  ["overview", "概览", LayoutGrid],
  ["members", "成员", UsersRound],
  ["drives", "网盘", Folder],
  ["permissions", "权限", ShieldCheck],
  ["activity", "活动", Activity],
  ["settings", "设置", Settings],
] as const;
const roleNames: Record<string, string> = {
  admin: "团队管理员",
  tenant_admin: "租户管理员",
  member: "成员",
  viewer: "只读",
  editor: "编辑",
  custodian: "数据负责人",
  remove: "撤销权限",
};
const actionNames: Record<string, string> = {
  "team.created": "创建团队",
  "team.member": "调整团队成员",
  "space.created": "创建网盘",
  "space.permission": "调整网盘权限",
  "team-update": "更新团队资料",
  "team-archive": "归档团队",
  "team-restore": "恢复团队",
  "team-leave": "退出团队",
  "team-invite": "邀请成员",
  "team-invite-accept": "接受团队邀请",
  "team-invite-cancel": "取消邀请",
  "team-invite-decline": "拒绝团队邀请",
  "team-invite-resend": "续期团队邀请",
  "team-storage-assign": "分配存储",
  "team-storage-unassign": "取消存储分配",
  "data.view": "查看团队资源",
  "data.download": "下载团队文件",
};
function route() {
  const [path] = location.hash.slice(1).split("?");
  const p = path.split("/");
  return {
    id: p[2] || "",
    tab: tabs.some((t) => t[0] === p[3]) ? p[3] : "overview",
  };
}
import { teamURL } from "./app/team-route";
export { teamURL } from "./app/team-route";
function deliveryText(name: string, code: string) {
  return [
    "DriveBridge 账号已开通。请本人保存激活码。",
    "账号：" + name,
    "激活码：" + code,
    "激活码 24 小时有效，只显示这一次。",
    "登录：" + location.origin,
    "登录后打开「团队」，按卡片先接受租户邀请，再接受团队邀请。",
    "加入后不会自动看到网盘文件，需要负责人在「权限」里授权。",
  ].join("\n");
}
function inviteHash() {
  return new URLSearchParams(location.hash.split("?")[1] || "").get("invite") || "";
}
function Pages({
  page,
  next,
  change,
  label = "记录分页",
}: {
  label?: string;
  page: number;
  next: number;
  change: (n: number) => void;
}) {
  if (page <= 1 && !next) return null;
  return (
    <div className="team-pagination" aria-label={label}>
      <span>{label}</span>
      <Button variant="outline" size="sm" type="submit"

        disabled={page <= 1}
        onClick={() => change(page - 1)}
      >
        上一页
      </Button>
      <span>第 {page} 页</span>
      <Button variant="outline" size="sm" type="submit"

        disabled={!next}
        onClick={() => change(next)}
      >
        下一页
      </Button>
    </div>
  );
}
export function TeamWorkspace({
  enabled = true,
  governance,
  user,
  openDrive,
  openKeys,
}: {
  enabled?: boolean;
  governance?: GovernanceState;
  user: { id: string; name: string };
  openDrive: (s: Space) => void;
  openKeys: () => void;
}) {
  const [locationState, setLocationState] = useState(route),
    [revision, refresh] = useState(0),
    [page, setPage] = useState(1),
    [detail, setDetail] = useState<Detail>(),
    [list, setList] = useState<
      Paged<Summary> & { enabled: boolean; can_create: boolean }
    >(),
    [members, setMembers] = useState<Paged<Member>>({
      items: [],
      next_page: 0,
    }),
    [spaces, setSpaces] = useState<Paged<Space>>({ items: [], next_page: 0 }),
    [events, setEvents] = useState<
      Paged<{
        id: string;
        action: string;
        actor: string;
        outcome: string;
        at: string;
      }>
    >({ items: [], next_page: 0 }),
    [invitations, setInvitations] = useState<Paged<Invitation>>({
      items: [],
      next_page: 0,
    }),
    [mine, setMine] = useState<
      {
        invitation: Invitation;
        status: string;
        next_step?: string;
        team: Team;
        membership: { id: string; status: string };
      }[]
    >([]),
    [loading, setLoading] = useState(true),
    [error, setError] = useState(""),
    [notice, setNotice] = useState(""),
    [busy, setBusy] = useState(false),
    [collapsed, setCollapsed] = useState(false);
  const [filter, setFilter] = useDirectoryQuery("group", "all");
  const [search, setSearch] = useDirectoryQuery("q");
  const [previewFile, setPreviewFile] = useState<DriveFile>();
  const [invitePage, setInvitePage] = useState(1),
    [spacePage, setSpacePage] = useState(1);
  const [form, setForm] = useState(""),
    [target, setTarget] = useState<Record<string, unknown>>({}),
    [impact, setImpact] = useState<Impact>(),
    [secret, setSecret] = useState<{
      user_id: string;
      activation_code: string;
    }>(),
    [impactAction, setImpactAction] = useState(""),
    [peopleQuery, setPeopleQuery] = useState(""),
    [peopleHits, setPeopleHits] = useState<
      { id: string; name: string; teams: { id: string }[] }[]
    >([]),
    [picked, setPicked] = useState(""),
    [askConfirm, setAskConfirm] = useState(false);
  const { id, tab } = locationState;
  const focusInvite = inviteHash();
  useEffect(() => {
    const changed = () => {
      setLocationState(route());
      setPage(1);
      setInvitePage(1);
      setSpacePage(1);
      setForm("");
      setSecret(undefined);
      setImpact(undefined);
      setNotice("");
      setError("");
      setDetail((current) =>
        current?.team.id === route().id ? current : undefined,
      );
      setPreviewFile(undefined);
    };
    window.addEventListener("hashchange", changed);
    return () => window.removeEventListener("hashchange", changed);
  }, []);
  useEffect(() => {
    if (!enabled) {
      setList({ enabled: false, items: [], next_page: 0, can_create: false });
      setLoading(false);
      return;
    }
    let active = true;
    const abort = new AbortController();
    const load = async () => {
      try {
        const own = await api<{ items: typeof mine }>(
          "/governance/team-invitations",
          "GET",
          undefined,
          {},
          abort.signal,
        ).catch((e) => {
          if (e.name === "AbortError") throw e;
          return { items: [] };
        });
        if (!id) {
          const result = await api<NonNullable<typeof list>>(
            `/governance/teams?filter=${filter}&q=${encodeURIComponent(search)}&page=${page}`,
            "GET",
            undefined,
            {},
            abort.signal,
          );
          if (active) {
            setList(result);
            setMine(own.items);
          }
        } else {
          const base = "/governance/teams/" + encodeURIComponent(id);
          const [d, m, s, e, i] = await Promise.all([
            api<Detail>(base, "GET", undefined, {}, abort.signal),
            api<Paged<Member>>(
              base +
                "/members?page=" +
                (tab === "members" || tab === "permissions" ? page : 1),
              "GET",
              undefined,
              {},
              abort.signal,
            ),
            api<Paged<Space>>(
              base + "/spaces?page=" + spacePage,
              "GET",
              undefined,
              {},
              abort.signal,
            ),
            api<typeof events>(
              base + "/activity?page=" + (tab === "activity" ? page : 1),
              "GET",
              undefined,
              {},
              abort.signal,
            ),
            api<Paged<Invitation>>(
              base + "/invitations?page=" + invitePage,
              "GET",
              undefined,
              {},
              abort.signal,
            ),
          ]);
          if (active) {
            setDetail(d);
            setMembers(m);
            setSpaces(s);
            setEvents(e);
            setInvitations(i);
            setMine(own.items);
          }
        }
        if (active) setError("");
      } catch (e) {
        if (active && (e as Error).name !== "AbortError") {
          setError(errorText(e));
          if (["forbidden", "not_found"].includes((e as Error).message))
            setDetail(undefined);
        }
      } finally {
        if (active) setLoading(false);
      }
    };
    setLoading(true);
    void load();
    const timer = setInterval(() => {
      if (!document.hidden) void load();
    }, 15000);
    return () => {
      active = false;
      abort.abort();
      clearInterval(timer);
    };
  }, [enabled, id, tab, page, invitePage, spacePage, filter, search, revision]);
  const changed = () => {
    refresh((n) => n + 1);
    window.dispatchEvent(new Event("drivebridge-governance-changed"));
  };
  const allow = (a: string) => !!detail?.actions[a]?.allowed;
  const name = (id: string) =>
    governance?.users.find((u) => u.id === id)?.name ||
    members.items.find((u) => u.user_id === id)?.name ||
    id.slice(0, 10);
  const candidates = (governance?.users || []).filter((u) =>
    governance?.members.some(
      (m) => m.user_id === u.id && m.status === "active",
    ),
  );
  async function perform(
    action: string,
    body: Record<string, unknown>,
    team = detail?.team,
  ) {
    setBusy(true);
    setError("");
    try {
      const result = await api<any>("/governance/" + action, "POST", {
        ...(team ? { team_id: team.id, expected_revision: team.revision } : {}),
        ...body,
      });
      changed();
      setNotice("操作已完成");
      return result;
    } catch (e) {
      setError(errorText(e));
      throw e;
    } finally {
      setBusy(false);
    }
  }
  async function showImpact(
    action: string,
    body: Record<string, unknown> = {},
  ) {
    if (!detail) return;
    setError("");
    setBusy(true);
    try {
      const query = new URLSearchParams();
      for (const k of ["user_id", "space_id", "role", "connection_id"])
        if (body[k]) query.set(k, String(body[k]));
      const data = await api<Impact>(
        "/governance/teams/" + id + "/impact?" + query,
      );
      setTarget(body);
      setImpact(data);
      setImpactAction(action);
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  }
  const choose = (action: string, body: Record<string, unknown> = {}) => {
    setForm(action);
    setTarget({ ...body, expected_revision: detail?.team.revision });
    setError("");
    setSecret(undefined);
    setPeopleQuery("");
    setPeopleHits([]);
    setPicked("");
    setAskConfirm(false);
  };
  useEffect(() => {
    if (form !== "invite-join" || !enabled) return;
    const timer = setTimeout(() => {
      void api<{ items: { id: string; name: string; teams: { id: string }[] }[] }>(
        "/governance/users?q=" + encodeURIComponent(peopleQuery),
      )
        .then((data) => setPeopleHits(data.items || []))
        .catch(() => setPeopleHits([]));
    }, 200);
    return () => clearTimeout(timer);
  }, [form, peopleQuery, enabled]);
  async function finishInvitation(account: { user_id: string }) {
    if (!detail) return;
    setBusy(true);
    setError("");
    try {
      const current = await api<GovernanceState>("/governance");
      if (
        !current.members.some(
          (m) =>
            m.user_id === account.user_id &&
            (m.status === "active" || m.status === "invited"),
        )
      )
        await api("/governance/invite", "POST", {
          user_id: account.user_id,
          role: "member",
        });
      const currentTeam = await api<Detail>("/governance/teams/" + id);
      try {
        await api("/governance/team-invite", "POST", {
          team_id: id,
          user_id: account.user_id,
          expected_revision: currentTeam.team.revision,
        });
      } catch (e) {
        if ((e as Error).message !== "invitation_exists") throw e;
      }
      changed();
      setNotice("账号及邀请已准备好，请向本人交付激活信息");
    } catch (e) {
      setError("账号已开通，邀请尚未完成，请继续发送邀请。" + errorText(e));
    } finally {
      setBusy(false);
    }
  }
  async function copy(value: string) {
    try {
      await navigator.clipboard.writeText(value);
      setNotice("已复制");
    } catch {
      setError("复制失败，请手动选择文本复制");
    }
  }
  async function accept(entry: (typeof mine)[number]) {
    const tenant = getDataContext().tenant || governance?.tenant_id;
    const step =
      entry.next_step ||
      (tenant !== entry.team.tenant_id
        ? "switch_tenant"
        : entry.membership.status === "active"
          ? "accept_team"
          : "accept_tenant");
    if (step === "switch_tenant") {
      setDataContext(entry.team.tenant_id);
      return;
    }
    try {
      if (step === "accept_tenant") {
        await perform("accept", { id: entry.membership.id }, undefined);
        setNotice("租户邀请已接受，请确认加入团队");
        return;
      }
      await perform(
        "team-invite-accept",
        { id: entry.invitation.id },
        entry.team,
      );
      setNotice("已加入团队。网盘权限需要负责人另行授予。");
    } catch {
      /* perform already surfaced the error */
    }
  }
  const invitationInbox = mine.length > 0 && (
    <section className="team-invitations surface-panel">
      <header>
        <h2>我的邀请</h2>
      </header>
      {mine.map((i) => {
        const step =
          i.next_step ||
          (i.status === "expired"
            ? "expired"
            : (getDataContext().tenant || governance?.tenant_id) !==
                i.team.tenant_id
              ? "switch_tenant"
              : i.membership.status === "active"
                ? "accept_team"
                : "accept_tenant");
        const primary =
          step === "switch_tenant"
            ? "切换到邀请租户"
            : step === "accept_team"
              ? "接受团队邀请"
              : step === "accept_tenant"
                ? "接受租户邀请"
                : "";
        return (
          <article
            key={i.invitation.id}
            data-invite={i.invitation.id}
            className={
              focusInvite === i.invitation.id ? "invite-focus" : undefined
            }
          >
            <div>
              <strong>{i.team.name}</strong>
              <small>
                {step === "expired"
                  ? "邀请已过期，请让管理员重新发送"
                  : step === "accept_tenant"
                    ? "步骤 1/2  先加入租户"
                    : step === "switch_tenant"
                      ? "步骤 1/2  先切换到邀请所在的租户"
                      : "步骤 2/2  加入团队 · 加入后不会自动看到网盘文件"}
              </small>
            </div>
            {(step === "accept_tenant" || step === "accept_team") && (
              <Button variant="outline" size="sm" type="submit"

                disabled={busy}
                onClick={() =>
                  void perform("team-invite-decline", {
                    id: i.invitation.id,
                  }, i.team).catch(() => {})
                }
              >
                忽略
              </Button>
            )}
            {primary && (
              <Button variant="default" size="sm" type="submit"

                disabled={busy}
                onClick={() => void accept(i)}
              >
                {primary}
              </Button>
            )}
          </article>
        );
      })}
    </section>
  );
  return (
    <section className="teams-workspace">
      {error && (
        <div className="banner error" role="alert">
          {error}
          <Button variant="outline" size="sm" type="submit"  onClick={changed}>
            刷新重试
          </Button>
        </div>
      )}
      {notice && (
        <p className="banner success" role="status">
          {notice}
        </p>
      )}
      {!enabled ? (
        <div className="workspace-empty">
          <UsersRound />
          <h3>团队服务尚未启用</h3>
          <p>个人网盘仍可使用；团队功能需要团队服务或隔离团队体验环境。</p>
        </div>
      ) : !id ? (
        <>
          <div className="workspace-toolbar">
            <div className="segmented">
              {[
                ["all", "全部团队"],
                ["joined", "我加入的"],
                ["managed", "我管理的"],
                ["archived", "已归档"],
              ].map(([k, n]) => (
                <Button variant="ghost" type="submit"
                  key={k}
                  aria-pressed={filter === k}
                  onClick={() => {
                    setFilter(k);
                    setPage(1);
                  }}
                >
                  {n}
                </Button>
              ))}
            </div>
            <SearchField
                placeholder="搜索团队…"
                aria-label="搜索团队"
                value={search}
                onChange={(e) => {
                  setSearch(e.target.value);
                  setPage(1);
                }}
              />
            {list?.can_create && (
              <Button variant="default" type="submit"

                onClick={() => choose("teams")}
              >
                <Plus size={16} />
                创建团队
              </Button>
            )}
          </div>
          {invitationInbox}
          {loading && !list ? (
            <p role="status">正在加载团队…</p>
          ) : list?.enabled === false ? (
            <div className="workspace-empty">
              <UsersRound />
              <h3>团队服务尚未启用</h3>
              <p>当前个人网盘仍可使用。团队体验需要显式启动隔离的团队环境。</p>
            </div>
          ) : (
            <>
              <DirectoryLayout label="团队" items={(list?.items || []).map(row=>({id:row.team.id,title:row.team.name,subtitle:row.team.description || "团队协作空间",icon:<UsersRound size={24}/>,meta:<>{row.member_count} 位成员 · {row.space_count} 个网盘</>,openLabel:`打开团队 ${row.team.name}`,open:()=>{location.hash=teamURL(row.team.id,"overview",row.team.tenant_id);},detail:<dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-3 text-sm"><dt>状态</dt><dd>{row.team.archived ? "只读封存" : "正常协作"}</dd><dt>我的角色</dt><dd>{roleNames[row.role] || row.role}</dd><dt>成员</dt><dd>{row.member_count}</dd><dt>可见网盘</dt><dd>{row.space_count}</dd><dt>已用容量</dt><dd>{bytes(row.used_bytes)}</dd></dl>}))} cards={
<div className="drive-cards">
                {list?.items.map((row) => (
                  <Button layout="surface" variant="ghost" type="button"
                    className="drive-card"
                    key={row.team.id}
                    onClick={() => {
                      location.hash = teamURL(
                        row.team.id,
                        "overview",
                        row.team.tenant_id,
                      );
                    }}
                  >
                    <div className="drive-card-top">
                      <span className="surface-icon">
                        <UsersRound />
                      </span>
                      <span className="pill">
                        {row.team.archived
                          ? "只读封存"
                          : roleNames[row.role] || "成员"}
                      </span>
                    </div>
                    <h2>{row.team.name}</h2>
                    <p>{row.team.description || "团队协作空间"}</p>
                    <div className="team-card-stats">
                      <span>{row.member_count} 位成员</span>
                      <span>{row.space_count} 个可见网盘</span>
                    </div>
                    <div className="drive-card-footer">
                      <span>{bytes(row.used_bytes)} 已用</span>
                      <ArrowRight size={16} />
                    </div>
                  </Button>
                ))}
              </div>              }/>
              {!list?.items.length && (
                <div className="workspace-empty">
                  <UsersRound />
                  <h3>没有匹配的团队</h3>
                  <p>创建团队，或在收到邀请后加入。</p>
                </div>
              )}
              <Pages page={page} next={list?.next_page || 0} change={setPage} />
            </>
          )}
        </>
      ) : !detail ? (
        <p role="status">
          {loading ? "正在加载团队详情…" : "当前团队不可访问，请返回团队列表。"}
          <Button variant="outline" type="submit"

            onClick={() => {
              location.hash = teamURL();
            }}
          >
            返回团队列表
          </Button>
        </p>
      ) : (
        <>
          <header className="team-header">
            <Button variant="ghost" size="icon" type="submit"
              className="icon"
              aria-label="返回团队列表"
              onClick={() => {
                location.hash = teamURL("", "overview", detail.team.tenant_id);
              }}
            >
              <ArrowLeft size={18} />
            </Button>
            <span className="surface-icon">
              <UsersRound />
            </span>
            <div>
              <h1>{detail.team.name}</h1>
              <p title={detail.team.description}>
                {detail.team.description || "团队工作区"} ·{" "}
                {roleNames[detail.role] || "成员"}
              </p>
            </div>
            <span className="pill">
              {detail.team.archived ? "已归档 · 只读" : "正常协作"}
            </span>
            <Button variant="ghost" size="icon" type="submit" className="icon" aria-label="刷新团队" onClick={changed}>
              <RefreshCw size={17} />
            </Button>
          </header>
          {detail.team.archived && (
            <p className="info-note">
              团队已只读封存。保留原读取权限，分享、临时授权和 Agent
              不会自动恢复。
            </p>
          )}
          <div
            className={"team-layout " + (collapsed ? "team-nav-collapsed" : "")}
          >
            <aside className="team-nav" aria-label="团队页面">
              <Button variant="outline" size="sm" type="submit"

                aria-label={collapsed ? "展开团队导航" : "折叠团队导航"}
                aria-expanded={!collapsed}
                onClick={() => setCollapsed(!collapsed)}
              >
                <PanelLeft size={16} />
              </Button>
              {tabs.map(([k, n, Icon]) => (
                <a
                  key={k}
                  href={teamURL(id, k, detail.team.tenant_id)}
                  aria-current={tab === k ? "page" : undefined}
                  title={n}
                  aria-label={n}
                >
                  <Icon size={17} />
                  {!collapsed && <span>{n}</span>}
                </a>
              ))}
            </aside>
            <div className="team-body" aria-busy={busy}>
              {tab === "overview" && (
                <>
                  <div className="metric-grid">
                    {[
                      ["成员", detail.member_count],
                      ["可见网盘", detail.space_count],
                      ["已用空间", bytes(detail.used_bytes)],
                      ["待处理邀请", detail.pending_invitations],
                    ].map(([label, value]) => (
                      <article className="metric-card" key={label}>
                        <div>{label}</div>
                        <strong>{value}</strong>
                      </article>
                    ))}
                  </div>
                  <section className="surface-panel">
                    <header>
                      <h3>近期文件</h3>
                      <a href={teamURL(id, "drives", detail.team.tenant_id)}>
                        查看团队网盘
                      </a>
                    </header>
                    <div className="recent-media">
                      {detail.recent_files.map((f) => (
                        <Button variant="ghost" type="submit"
                          key={f.id}
                          onClick={() =>
                            setPreviewFile({
                              ...f,
                              tenant_context: detail.team.tenant_id,
                            })
                          }
                        >
                          <MediaThumbnail
                            file={{
                              ...f,
                              tenant_context: detail.team.tenant_id,
                            }}
                          />
                          <strong>{f.path.split("/").pop()}</strong>
                          <small>{bytes(f.size_bytes)}</small>
                        </Button>
                      ))}
                    </div>
                    {!detail.recent_files.length && (
                      <p className="panel-empty">暂时没有可预览的团队文件。</p>
                    )}
                  </section>
                  <section className="surface-panel">
                    <header>
                      <h3>待处理授权申请</h3>
                      <a
                        href={teamURL(id, "permissions", detail.team.tenant_id)}
                      >
                        查看权限
                      </a>
                    </header>
                    <p className="panel-empty">
                      {detail.pending_requests.length}{" "}
                      项当前可见的待审批申请；管理身份不自动获得编辑权限。
                    </p>
                  </section>
                </>
              )}
              {tab === "members" && (
                <>
                  <div className="workspace-toolbar">
                    <h3>团队成员</h3>
                    {(allow("manage_members") ||
                      allow("invite") ||
                      allow("provision")) && (
                      <Button variant="default" type="submit"

                        onClick={() => choose("invite-join")}
                      >
                        <Plus size={15} />
                        邀请加入
                      </Button>
                    )}
                  </div>
                  <div className="surface-panel table-scroll">
                    <GridTable pagination="external" headers={[<> 用户 </>,<> 团队角色 </>,<> 状态 </>,<> 网盘权限 </>,<> 操作 </>]} rows={[...(members.items.map((m) => (({id:String(m.user_id),cells:[<>
                              <strong>
                                {m.name}
                                {m.user_id === user.id ? "（我）" : ""}
                              </strong>
                              <small>{m.user_id.slice(0, 12)}</small>
                             </>,<> {roleNames[m.role]} </>,<>
                              {m.status === "active" ? "已加入" : "不可用"}
                             </>,<>
                              {detail.permissions
                                .filter((v) => v.user_id === m.user_id)
                                .map((v) => (
                                  <span className="pill" key={v.space_id}>
                                    {spaces.items.find(
                                      (s) => s.id === v.space_id,
                                    )?.name || v.space_id.slice(0, 8)}{" "}
                                    · {roleNames[v.role]}
                                  </span>
                                ))}
                             </>,<>
                              <div className="row-actions">
                                {allow("promote_admin") &&
                                  m.user_id !== user.id && (
                                    <Button variant="outline" size="sm" type="submit"

                                      onClick={() =>
                                        void showImpact("team-member", {
                                          user_id: m.user_id,
                                          role:
                                            m.role === "admin"
                                              ? "member"
                                              : "admin",
                                        })
                                      }
                                    >
                                      {m.role === "admin"
                                        ? "降为成员"
                                        : "设为管理员"}
                                    </Button>
                                  )}
                                {allow("manage_members") &&
                                  m.user_id !== user.id &&
                                  (m.role !== "admin" ||
                                    allow("promote_admin")) && (
                                    <Button variant="destructive" size="sm" type="submit"

                                      onClick={() =>
                                        void showImpact("team-member", {
                                          user_id: m.user_id,
                                          role: "remove",
                                        })
                                      }
                                    >
                                      移除
                                    </Button>
                                  )}
                                {m.user_id === user.id && allow("leave") && (
                                  <Button variant="outline" size="sm" type="submit"

                                    onClick={() =>
                                      void showImpact("team-leave", {
                                        user_id: user.id,
                                        role: "remove",
                                      })
                                    }
                                  >
                                    退出团队
                                  </Button>
                                )}
                              </div>
                             </>]})))),...(invitations.items
    .filter((i) => (i.status === "pending" ||
    i.status === "expired") &&
    !members.items.some((m) => m.user_id === i.user_id))
    .map((i) => (({id:String("invite-" + i.id),cells:[<>
                                <strong>
                                  {i.user_name || name(i.user_id)}
                                </strong>
                                <small>{i.user_id.slice(0, 12)}</small>
                               </>,<> 成员 </>,<>
                                {i.status === "expired" ? "已过期" : "待接受"}
                               </>,<>
                                <span className="pill">尚未授权网盘</span>
                               </>,<>
                                <div className="row-actions">
                                  {allow("invite") && i.status === "expired" && (
                                    <Button variant="outline" size="sm" type="submit"

                                      disabled={busy}
                                      onClick={() =>
                                        void perform("team-invite-resend", {
                                          id: i.id,
                                        }).catch(() => {})
                                      }
                                    >
                                      续期七天
                                    </Button>
                                  )}
                                  {allow("invite") && (
                                    <Button variant="destructive" size="sm" type="submit"

                                      disabled={busy}
                                      onClick={() =>
                                        void perform("team-invite-cancel", {
                                          id: i.id,
                                        }).catch(() => {})
                                      }
                                    >
                                      取消邀请
                                    </Button>
                                  )}
                                </div>
                               </>]}))))]}/>
                  </div>
                  <Pages
                    page={page}
                    next={members.next_page}
                    label="成员分页"
                    change={setPage}
                  />
                  <section className="surface-panel">
                    <header>
                      <h3>邀请记录</h3>
                    </header>
                    {invitations.items.map((i) => (
                      <article className="team-invitation-row" key={i.id}>
                        <div>
                          <strong>{i.user_name || name(i.user_id)}</strong>
                          <small>
                            {{
                              pending: "待接受",
                              expired: "已过期",
                              accepted: "已接受",
                              cancelled: "已取消",
                              declined: "已拒绝",
                            }[i.status] || i.status}
                            {i.inviter_name ? " · 由 " + i.inviter_name : ""}
                            {" · "}
                            {i.expires_at.slice(0, 10)}
                          </small>
                        </div>
                        {allow("invite") &&
                          (i.status === "pending" || i.status === "expired") && (
                            <>
                              <Button variant="outline" size="sm" type="submit"

                                disabled={busy}
                                onClick={() =>
                                  void perform("team-invite-resend", {
                                    id: i.id,
                                  }).catch(() => {})
                                }
                              >
                                续期七天
                              </Button>
                              {i.status === "pending" && (
                                <Button variant="outline" size="sm" type="submit"

                                  disabled={busy}
                                  onClick={() =>
                                    void perform("team-invite-cancel", {
                                      id: i.id,
                                    }).catch(() => {})
                                  }
                                >
                                  取消邀请
                                </Button>
                              )}
                            </>
                          )}
                      </article>
                    ))}
                    <Pages
                      page={invitePage}
                      next={invitations.next_page}
                      change={setInvitePage}
                    />
                    {!invitations.items.length && (
                      <p className="panel-empty">暂无邀请记录。</p>
                    )}
                  </section>
                </>
              )}
              {tab === "drives" && (
                <>
                  <div className="workspace-toolbar">
                    <h3>团队网盘</h3>
                    {allow("create_space") && (
                      <Button variant="default" type="submit"

                        onClick={() => choose("spaces")}
                      >
                        <Plus size={16} />
                        创建团队网盘
                      </Button>
                    )}
                    <a href={teamURL(id, "settings", detail.team.tenant_id)}>
                      管理分配的存储
                    </a>
                  </div>
                  <div className="drive-cards">
                    {spaces.items.map((s) => (
                      <Button layout="surface" variant="ghost" type="button"
                        className="drive-card"
                        key={s.id}
                        onClick={() => openDrive(s)}
                      >
                        <span className="surface-icon">
                          <Folder />
                        </span>
                        <h2>{s.name}</h2>
                        <p>
                          {detail.connections.find(
                            (c) => c.id === s.connection_id,
                          )?.name || "已分配存储"}{" "}
                          · {roleNames[detail.space_roles[s.id]] || "管理只读"}
                        </p>
                        <Progress
                          value={s.used_bytes || 0}
                          max={s.max_bytes || 1}
                        />
                        <div className="drive-card-footer">
                          <span>
                            {bytes(s.used_bytes)} / {bytes(s.max_bytes)}
                          </span>
                          <ArrowRight size={16} />
                        </div>
                      </Button>
                    ))}
                  </div>
                  {allow("set_quota") && spaces.items.map(s => <div key={s.id} className="flex items-center justify-between gap-2 py-2"><span>{s.name}</span><Button variant="outline" onClick={() => {setError("");setTarget({space_id:s.id,max_bytes:s.max_bytes});setForm("space-quota");}}>调整空间配额</Button></div>)}
                  {!spaces.items.length && (
                    <p className="workspace-empty">
                      暂无可访问的团队网盘。加入团队不会自动获得文件权限。
                    </p>
                  )}
                  <Pages
                    page={spacePage}
                    next={spaces.next_page}
                    change={setSpacePage}
                  />
                </>
              )}
              {tab === "permissions" && (
                <>
                  <div className="relationship-strip">
                    <KeyRound size={16} />
                    登录 Key → 用户 → 团队成员 → 网盘权限
                    <Button variant="outline" size="sm" type="submit"  onClick={openKeys}>
                      我的 Key
                    </Button>
                  </div>
                  <p className="text-muted-foreground">
                    管理角色和数据角色独立。交接前先授予新的负责人，再移除原负责人。
                  </p>
                  <div className="surface-panel table-scroll">
                    <GridTable pagination="external" headers={[<> 成员 </>,...(spaces.items.map((s) => (<>
                              {s.name}
                             </>)))]} rows={[...(members.items.map((m) => (({id:String(m.user_id),cells:[<> {m.name} </>,...(spaces.items.map((s) => {
    const role = detail.permissions.find((p) => p.space_id === s.id &&
        p.user_id === m.user_id)?.role || "none";
    return (<>
                                  {detail.can_grant[s.id] &&
                                  m.user_id !== user.id ? (
                                    <SelectAdapter
                                      aria-label={`${m.name} 在 ${s.name} 的权限`}
                                      value={role}
                                      disabled={busy}
                                      onChange={(e) =>
                                        void showImpact("permission", {
                                          space_id: s.id,
                                          user_id: m.user_id,
                                          role:
                                            e.target.value === "none"
                                              ? "remove"
                                              : e.target.value,
                                        })
                                      }
                                    >
                                      {[
                                        ["none", "未授权"],
                                        ["viewer", "只读"],
                                        ["editor", "编辑"],
                                        ["custodian", "数据负责人"],
                                      ].map(([k, n]) => (
                                        <option key={k} value={k}>
                                          {n}
                                        </option>
                                      ))}
                                    </SelectAdapter>
                                  ) : (
                                    roleNames[role] || "未授权"
                                  )}
                                 </>);
}))]}))))]}/>
                  </div>
                  <Pages
                    page={page}
                    next={members.next_page}
                    label="成员分页"
                    change={setPage}
                  />
                  <Pages label="网盘列分页"
                    page={spacePage}
                    next={spaces.next_page}
                    change={setSpacePage}
                  />
                  <h3>机器人授权</h3>
                  {detail.agents.map((a) => (
                    <div className="team-invitation-row" key={a.id}>
                      <strong>{a.name}</strong>
                      <span>
                        {a.prefix || "根目录"} · {a.scopes}
                      </span>
                      <span className="pill">
                        {a.revoked ? "已撤销／暂停" : "有效"}
                      </span>
                    </div>
                  ))}
                  {!detail.agents.length && (
                    <p className="panel-empty">暂无可见的团队 机器人授权。</p>
                  )}
                  <h3>待审批申请</h3>
                  {detail.pending_requests.map((q) => (
                    <div className="team-invitation-row" key={q.id}>
                      <div>
                        <strong>
                          {name(q.requester)} ·{" "}
                          {q.kind === "exact" ? "固定清单" : "范围授权"}
                        </strong>
                        <p>{q.reason}</p>
                      </div>
                      {detail.can_grant[q.space_id] &&
                        q.requester !== user.id && (
                          <>
                            <Button variant="outline" type="submit"

                              onClick={() =>
                                choose("approval", {
                                  id: q.id,
                                  reason: q.reason,
                                })
                              }
                            >
                              查看并处理
                            </Button>
                          </>
                        )}
                    </div>
                  ))}
                </>
              )}
              {tab === "activity" && (
                <>
                  <div className="surface-panel">
                    {events.items.map((e) => (
                      <article className="team-invitation-row" key={e.id}>
                        <span className="surface-icon">
                          <Activity size={17} />
                        </span>
                        <div>
                          <strong>{actionNames[e.action] || e.action}</strong>
                          <small>
                            {name(e.actor)} ·{" "}
                            {e.outcome === "success" ? "成功" : e.outcome}
                          </small>
                        </div>
                        <time>{new Date(e.at).toLocaleString()}</time>
                      </article>
                    ))}
                    {!events.items.length && (
                      <p className="panel-empty">暂无活动记录。</p>
                    )}
                  </div>
                  <Pages page={page} next={events.next_page} change={setPage} />
                </>
              )}
              {tab === "settings" && (
                <>
                  <section className="surface-panel">
                    <header>
                      <h3>团队资料</h3>
                      {allow("update") && (
                        <Button variant="outline" type="submit"

                          onClick={() => choose("team-update")}
                        >
                          编辑资料
                        </Button>
                      )}
                    </header>
                    <div className="team-settings-body">
                      <strong>{detail.team.name}</strong>
                      <p>{detail.team.description || "暂无说明"}</p>
                      <small>
                        版本 {detail.team.revision} · 更新于{" "}
                        {new Date(detail.team.updated_at).toLocaleString()}
                      </small>
                    </div>
                  </section>
                  <section className="surface-panel">
                    <header>
                      <div>
                        <h3>已分配存储</h3>
                        <p>
                          当前支持 S3／COS 兼容连接。OneDrive
                          保留导入与导出入口。
                        </p>
                      </div>
                      {allow("manage_storage") && (
                        <Button variant="outline" type="submit"

                          onClick={() => choose("team-storage-assign")}
                        >
                          分配已有存储
                        </Button>
                      )}
                    </header>
                    {detail.connections.map((c) => (
                      <div className="team-invitation-row" key={c.id}>
                        <div>
                          <strong>{c.name}</strong>
                          <small>
                            {c.kind} · {c.disabled ? "已停用" : c.status}
                          </small>
                        </div>
                        {allow("manage_storage") && (
                          <Button variant="outline" size="sm" type="submit"

                            onClick={() =>
                              void showImpact("team-storage-unassign", {
                                connection_id: c.id,
                              })
                            }
                          >
                            取消分配
                          </Button>
                        )}
                      </div>
                    ))}
                  </section>
                  <section className="surface-panel">
                    <header>
                      <h3>团队生命周期</h3>
                    </header>
                    <div className="team-settings-body">
                      <p>
                        归档后保留原读取权限和文件，停止写入、分享与
                        Agent。旧临时授权不随恢复重新生效。
                      </p>
                      {allow("archive") && (
                        <Button variant="destructive" type="submit"

                          onClick={() => void showImpact("team-archive")}
                        >
                          <Archive size={15} />
                          归档团队
                        </Button>
                      )}
                      {allow("restore") && (
                        <Button variant="outline" type="submit"

                          onClick={() => void showImpact("team-restore")}
                        >
                          恢复团队
                        </Button>
                      )}
                    </div>
                  </section>
                </>
              )}
            </div>
          </div>
        </>
      )}
      {previewFile && (
        <WorkspaceDialog
          title={previewFile.path.split("/").pop() || "团队文件预览"}
          close={() => setPreviewFile(undefined)}
        >
          <MediaPreview file={previewFile} />
        </WorkspaceDialog>
      )}
      {impact && detail && (
        <WorkspaceDialog
          title="确认团队变更"
          close={() => {
            if (!busy) setImpact(undefined);
          }}
        >
          <p>
            {actionNames[impactAction] || "调整团队"}
            {target.user_id ? " · " + name(String(target.user_id)) : ""}
          </p>
          <div className="team-impact">
            <span>{impact.members} 位成员</span>
            <span>{impact.spaces.length} 个网盘</span>
            <span>{impact.shares} 个活动分享</span>
            <span>{impact.agents} 个 Agent</span>
            <span>{impact.operations} 项进行中操作</span>
          </div>
          {impact.blockers.map((b) => (
            <p className="error" key={b}>
              {errorText(new Error(b))}
            </p>
          ))}
          {impactAction === "team-storage-unassign" &&
            impact.spaces
              .filter((s) => s.connection_id === target.connection_id)
              .map((s) => (
                <p key={s.id}>仍由「{s.name}」使用，不能取消分配。</p>
              ))}
          {impactAction === "team-archive" && (
            <p className="info-note">
              归档不删除文件，但会撤销活动分享和临时授权、暂停
              Agent。已签发的存储直链在原到期时间前可能继续有效。
            </p>
          )}
          {error && (
            <p className="error" role="alert">
              {error}
            </p>
          )}
          <div className="dialog-actions">
            <Button variant="outline" type="submit"

              disabled={busy}
              onClick={() => setImpact(undefined)}
            >
              取消
            </Button>
            <Button variant="default" type="submit"

              disabled={
                busy ||
                !!impact.blockers.length ||
                (impactAction === "team-storage-unassign" &&
                  impact.spaces.some(
                    (s) => s.connection_id === target.connection_id,
                  ))
              }
              onClick={async () => {
                try {
                  await perform(impactAction, target, {
                    ...detail.team,
                    revision: impact.revision,
                  });
                  setImpact(undefined);
                  if (impactAction === "team-leave") location.hash = teamURL();
                } catch {}
              }}
            >
              {busy ? "提交中…" : "确认变更"}
            </Button>
          </div>
        </WorkspaceDialog>
      )}
      {form && (
        <WorkspaceDialog
          className="team-drawer"
          title={
            (
              {
                teams: "创建团队",
                "team-update": "编辑团队资料",
                "team-member": "添加已有成员",
                "team-invite": "邀请用户",
                "invite-join": "邀请加入",
                provision: "开通账号并邀请",
                spaces: "创建团队网盘",
                "space-quota": "调整空间配额",
                "team-storage-assign": "分配已有存储",
                approval: "处理授权申请",
              } as Record<string, string>
            )[form] || "团队操作"
          }
          close={() => {
            if (!busy) {
              setForm("");
              setSecret(undefined);
            }
          }}
        >
          {secret ? (
            <>
              <p>
                新账号已开通。把下面整段交给本人。关闭后管理员再也看不到激活码，也不会取得登录
                Key。
              </p>
              <label>
                激活码
                <Textarea
                  readOnly
                  className="token-value"
                  value={secret.activation_code}
                />
              </label>
              <pre className="invite-packet">
                {deliveryText(
                  peopleQuery || "新成员",
                  secret.activation_code,
                )}
              </pre>
              <Button variant="outline" type="submit"

                onClick={() =>
                  void copy(
                    deliveryText(
                      peopleQuery || "新成员",
                      secret.activation_code,
                    ),
                  )
                }
              >
                <Copy size={15} />
                复制说明
              </Button>
              {error && (
                <Button variant="outline" type="submit"

                  disabled={busy}
                  onClick={() => void finishInvitation(secret)}
                >
                  继续发送邀请
                </Button>
              )}
              {notice && <p role="status">{notice}</p>}
              {error && (
                <p className="error" role="alert">
                  {error}
                </p>
              )}
              <div className="dialog-actions">
                <Button variant="default" type="submit"

                  onClick={() => {
                    setSecret(undefined);
                    setForm("");
                  }}
                >
                  我已交给本人
                </Button>
              </div>
            </>
          ) : form === "approval" ? (
            <>
              <p>{String(target.reason || "")}</p>
              <p>批准后由申请人确认执行，不会自动修改文件。</p>
              <div className="dialog-actions">
                <Button variant="outline" type="submit"

                  disabled={busy}
                  onClick={() =>
                    void perform("reject", { id: target.id })
                      .then(() => setForm(""))
                      .catch(() => {})
                  }
                >
                  拒绝
                </Button>
                <Button variant="default" type="submit"

                  disabled={busy}
                  onClick={() =>
                    void perform("approve", { id: target.id })
                      .then(() => setForm(""))
                      .catch(() => {})
                  }
                >
                  批准申请
                </Button>
              </div>
            </>
          ) : (
            <form
              onSubmit={async (e) => {
                e.preventDefault();
                const data: Record<string, FormDataEntryValue> = {};
                new FormData(e.currentTarget).forEach((value, key) => {
                  data[key] = value;
                });
                try {
                  if (form === "invite-join") {
                    const selected = peopleHits.find((u) => u.id === picked);
                    if (selected) {
                      const pending = invitations.items.find(
                        (i) =>
                          i.user_id === selected.id &&
                          (i.status === "pending" || i.status === "expired"),
                      );
                      if (members.items.some((m) => m.user_id === selected.id)) {
                        setError("已经在团队里");
                        return;
                      }
                      if (pending) {
                        await perform("team-invite-resend", { id: pending.id });
                        setForm("");
                        return;
                      }
                      const tenantMember = governance?.members.find(
                        (m) => m.user_id === selected.id,
                      );
                      if (tenantMember?.status === "active" && !askConfirm) {
                        await perform("team-member", {
                          ...target,
                          user_id: selected.id,
                          role: "member",
                        });
                        setForm("");
                        return;
                      }
                      if (tenantMember?.status !== "active")
                        await perform("invite", {
                          user_id: selected.id,
                          role: "member",
                        });
                      await perform("team-invite", {
                        ...target,
                        user_id: selected.id,
                      });
                      setForm("");
                      return;
                    }
                    if (!allow("provision") || !String(data.name || "").trim()) {
                      setError("请选择用户，或填写名称开通新账号。");
                      return;
                    }
                    const account = await perform("provision", {
                      name: data.name,
                    });
                    setPeopleQuery(String(data.name));
                    setSecret(account);
                    await finishInvitation(account);
                    return;
                  }
                  if (form === "provision") {
                    const account = await perform("provision", {
                      name: data.name,
                    });
                    setSecret(account);
                    await finishInvitation(account);
                  } else if (form === "team-invite") {
                    const member = governance?.members.find(
                      (m) =>
                        m.user_id === data.user_id && m.status === "active",
                    );
                    if (!member)
                      await perform("invite", {
                        user_id: data.user_id,
                        role: "member",
                      });
                    await perform(form, { ...target, ...data });
                    setForm("");
                  } else {
                    const quota = ["spaces", "space-quota"].includes(form) ? Math.round(Number(data.quota_gib) * 1024 ** 3) : undefined;
                    if (quota !== undefined && (!Number.isSafeInteger(quota) || quota <= 0)) { setError("请输入有效的空间配额"); return; }
                    delete data.quota_gib;
                    const result = await perform(form, {
                      ...target,
                      ...data,
                      ...(quota !== undefined ? {max_bytes: quota} : {}),
                      ...(form === "team-member" ? { role: "member" } : {}),
                    });
                    setForm("");
                    if (form === "teams")
                      location.hash = teamURL(
                        result.id,
                        "overview",
                        result.tenant_id,
                      );
                  }
                } catch {}
              }}
            >
              {form === "invite-join" && (
                <>
                  <label>
                    搜索已有账号
                    <Input
                      value={peopleQuery}
                      onChange={(e) => {
                        setPeopleQuery(e.target.value);
                        setPicked("");
                      }}
                      placeholder="姓名或用户 ID"
                      aria-label="搜索已有账号"
                    />
                  </label>
                  <div className="invite-hits" role="listbox" aria-label="匹配的用户">
                    {peopleHits
                      .filter((u) => u.id !== user.id)
                      .map((u) => {
                        const inTeam =
                          members.items.some((m) => m.user_id === u.id) ||
                          u.teams?.some((t) => t.id === id);
                        const pending = invitations.items.find(
                          (i) =>
                            i.user_id === u.id &&
                            (i.status === "pending" || i.status === "expired"),
                        );
                        const tenantMember = governance?.members.find(
                          (m) => m.user_id === u.id,
                        );
                        return (
                          <Button variant="ghost"
                            type="button"
                            role="option"
                            aria-label={u.name}
                            aria-selected={picked === u.id}
                            className={
                              "invite-hit" + (picked === u.id ? " selected" : "")
                            }
                            key={u.id}
                            onClick={() => setPicked(u.id)}
                          >
                            <strong>{u.name}</strong>
                            <small>
                              {u.id.slice(0, 8)}
                              {inTeam
                                ? " · 已在本团队"
                                : pending
                                  ? " · 已有待接受邀请"
                                  : tenantMember?.status === "active"
                                    ? " · 本租户成员"
                                    : tenantMember
                                      ? " · 待接受租户邀请"
                                      : " · 未加入本租户"}
                            </small>
                          </Button>
                        );
                      })}
                    {!peopleHits.length && (
                      <p className="panel-empty">
                        {allow("provision")
                          ? "没有匹配的人。可以在下方开通新账号。"
                          : "没有匹配的人。只能邀请本租户已有成员。"}
                      </p>
                    )}
                  </div>
                  {picked &&
                    governance?.members.some(
                      (m) =>
                        m.user_id === picked && m.status === "active",
                    ) &&
                    !invitations.items.some(
                      (i) =>
                        i.user_id === picked &&
                        (i.status === "pending" || i.status === "expired"),
                    ) && (
                      <label className="invite-confirm">
                        <Checkbox

                          checked={askConfirm}
                          onCheckedChange={(e) => setAskConfirm(e)}
                        />
                        改为发送邀请，让对方确认
                      </label>
                    )}
                  {allow("provision") && (
                    <p>开通空账号。激活码只显示一次，不会生成他人的登录 Key。</p>
                  )}
                </>
              )}
              {["teams", "team-update", "spaces", "provision", "invite-join"].includes(
                form,
              ) && (form !== "invite-join" || allow("provision")) && (
                <label>
                  {form === "provision" || form === "invite-join"
                    ? "用户名称"
                    : form === "spaces"
                      ? "网盘名称"
                      : "团队名称"}
                  <Input
                    name="name"
                    required={form !== "invite-join" || !picked}
                    maxLength={120}
                    defaultValue={
                      form === "team-update" ? detail?.team.name : ""
                    }
                  />
                </label>
              )}
              {["teams", "team-update"].includes(form) && (
                <label>
                  团队说明
                  <Textarea
                    name="description"
                    maxLength={2000}
                    rows={4}
                    defaultValue={
                      form === "team-update" ? detail?.team.description : ""
                    }
                  />
                </label>
              )}
              {["teams", "team-member", "team-invite", "spaces"].includes(
                form,
              ) && (
                <label>
                  {form === "teams"
                    ? "团队管理员"
                    : form === "spaces"
                      ? "数据负责人"
                      : "选择用户"}
                  <SelectAdapter aria-label={form === "teams" ? "团队管理员" : form === "spaces" ? "数据负责人" : "选择用户"} name="user_id" required defaultValue="">
                    <option value="" disabled>
                      选择用户…
                    </option>
                    {(form === "team-invite" && allow("create_space")
                      ? governance?.users || []
                      : candidates
                    )
                      .filter((u) => form === "teams" || form === "spaces" || u.id !== user.id)
                      .filter(
                        (u) =>
                          form !== "spaces" ||
                          governance?.team_members.some(
                            (m) => m.team_id === id && m.user_id === u.id,
                          ) ||
                          members.items.some((m) => m.user_id === u.id),
                      )
                      .map((u) => (
                        <option key={u.id} value={u.id}>
                          {u.name} · {u.id.slice(0, 8)}
                        </option>
                      ))}
                  </SelectAdapter>
                </label>
              )}
              {["spaces", "team-storage-assign"].includes(form) && (
                <label>
                  存储连接
                  <SelectAdapter aria-label="存储连接" name="connection_id" required defaultValue="">
                    <option value="" disabled>
                      选择 S3 兼容连接…
                    </option>
                    {(form === "spaces"
                      ? detail?.connections
                      : detail?.storage_candidates
                    )?.map((c) => (
                      <option
                        key={c.id}
                        value={c.id}
                        disabled={
                          c.disabled ||
                          c.kind !== "s3" ||
                          (form === "team-storage-assign" &&
                            detail?.connections.some((v) => v.id === c.id))
                        }
                      >
                        {c.name} · {c.kind}
                        {c.disabled
                          ? "（不可用）"
                          : c.kind !== "s3"
                            ? "（仅导入／导出）"
                            : ""}
                      </option>
                    ))}
                  </SelectAdapter>
                </label>
              )}
              {["spaces", "space-quota"].includes(form) && <label>空间配额（GiB）<Input aria-label="空间配额（GiB）" name="quota_gib" type="number" min="0.001" step="any" defaultValue={form === "space-quota" ? Number(target.max_bytes) / 1024 ** 3 : 1} required/><small>已用与上传预留占用空间配额，不能将配额调低到占用量以下。</small></label>}
              {form === "team-member" && (
                <p>加入团队不自动获得网盘编辑权限。加入后在“权限”页分配。</p>
              )}
              {(form === "team-invite" || form === "invite-join") && (
                <p>
                  团队邀请七天有效，只邀请指定账号，不提供开放加入链接。加入后不会自动获得网盘权限。
                </p>
              )}
              {form === "provision" && (
                <p>
                  复用本人激活流程，激活码一次显示；不会生成或展示他人的登录
                  Key。
                </p>
              )}
              {error && (
                <p className="error" role="alert">
                  {error}
                </p>
              )}
              <div className="dialog-actions">
                <Button variant="outline"
                  type="button"

                  disabled={busy}
                  onClick={() => setForm("")}
                >
                  取消
                </Button>
                <Button variant="default" type="submit"  disabled={busy}>
                  {busy ? "提交中…" : "保存并继续"}
                </Button>
              </div>
            </form>
          )}
        </WorkspaceDialog>
      )}
    </section>
  );
}
