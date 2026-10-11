import { Input } from "./components/ui/input";
import { Checkbox } from "./components/ui/checkbox";
import { DataTable } from "./components/DataTable";
import { Button } from "./components/ui/button";
import { SelectField } from "./components/SelectField";
import React, { useEffect, useState } from "react";
import {
  ArrowRight,
  KeyRound,
  Pause,
  Play,
  Trash2,
  Undo2,
  UsersRound,
} from "lucide-react";
import { api, errorText } from "./api";
import { AppDialog as WorkspaceDialog } from "./components/AppDialog";
import type { GovernanceState } from "./Governance";

interface Row {
  id: string;
  name: string;
  disabled: boolean;
  archived: boolean;
  revision: number;
  role: string;
  teams: { id: string; name: string }[];
  key_count: number | null;
  active_key_count: number | null;
  can_manage: boolean;
}
interface Key {
  id: string;
  name: string;
  revoked: boolean;
  archived: boolean;
  disabled?: boolean;
  expires_at: string;
}
interface Detail {
  user: Row;
  keys: Key[];
  keys_visible: boolean;
}
interface Impact {
  takeover_allowed?: boolean;
  takeover_digest?: string;
  handoffs?: { kind: string; id: string; name: string }[];
  user: Row;
  action: string;
  allowed: boolean;
  reason: string;
  counts: Record<string, number>;
}
const roles: Record<string, string> = {
  platform_admin: "平台管理员",
  tenant_admin: "租户管理员",
  member: "普通用户",
};
const actions: Record<string, string> = {
  freeze: "冻结账号",
  unfreeze: "解冻账号",
  archive: "删除并归档",
  restore: "恢复归档",
};
const state = (u: Row) =>
  u.archived ? "已归档" : u.disabled ? "已冻结" : "正常";

export function UserDirectory({
  data,
  user,
  query,
  openKeys,
  openTeam,
  fallbackKeys,
}: {
  data: GovernanceState;
  user: { id: string; name: string };
  query: string;
  openKeys: () => void;
  openTeam?: (id: string) => void;
  fallbackKeys: { user: string; name: string; admin?: boolean }[];
}) {
  const [status, setStatus] = useState(""),
    [role, setRole] = useState(""),
    [team, setTeam] = useState("");
  const [page, setPage] = useState(1),
    [revision, refresh] = useState(0);
  const [rows, setRows] = useState<Row[]>([]),
    [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true),
    [busy, setBusy] = useState(false);
  const [error, setError] = useState(""),
    [notice, setNotice] = useState("");
  const [detail, setDetail] = useState<Detail>(),
    [impact, setImpact] = useState<Impact>();
  const [proof, setProof] = useState(""),
    [conflict, setConflict] = useState(false);
  const [legacy, setLegacy] = useState(false);
  const [confirmationMode, setConfirmationMode] = useState("name"),
    [confirmationName, setConfirmationName] = useState(""),
    [takeover, setTakeover] = useState(false);
  const canConfirm =
    !!impact &&
    (impact.allowed || (impact.takeover_allowed && takeover)) &&
    !conflict &&
    (confirmationMode === "name"
      ? confirmationName.trim() === impact.user.name.trim()
      : !!proof);
  useEffect(() => setPage(1), [query, status, role, team]);
  useEffect(() => {
    const changed = () => refresh((n) => n + 1);
    window.addEventListener("drivebridge-governance-changed", changed);
    return () =>
      window.removeEventListener("drivebridge-governance-changed", changed);
  }, []);
  useEffect(() => {
    let alive = true;
    const abort = new AbortController();
    setLoading(true);
    setError("");
    const params = new URLSearchParams({
      q: query,
      status,
      role,
      team,
      page: String(page),
    });
    void api<{ items: Row[]; total: number }>(
      `/governance/users?${params}`,
      "GET",
      undefined,
      {},
      abort.signal,
    )
      .then((result) => {
        if (alive) {
          setRows(result.items);
          setTotal(result.total);
          setLegacy(false);
        }
      })
      .catch((e) => {
        if (!alive || e.name === "AbortError") return;
        setError(errorText(e));
        setRows([]);
        if (e.message === "not_found" || e.name === "SyntaxError") {
          setLegacy(true);
          setError(
            "当前服务尚未提供新版用户管理接口，仅展示基础信息。请使用新版体验服务管理账号。",
          );
        }
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
      abort.abort();
    };
  }, [query, status, role, team, page, revision, data.tenant_id]);
  const legacyRows: Row[] = data.users
    .map((u) => ({
      ...u,
      disabled: !!(u as Row).disabled,
      archived: !!(u as Row).archived,
      revision: 0,
      role:
        data.members.find((m) => m.user_id === u.id)?.role === "admin"
          ? "tenant_admin"
          : "member",
      teams: data.teams.filter((t) =>
        data.team_members.some((m) => m.user_id === u.id && m.team_id === t.id),
      ),
      key_count:
        u.id === user.id
          ? fallbackKeys.filter((k) => k.user === u.id).length
          : null,
      active_key_count: null,
      can_manage: false,
    }))
    .filter(
      (u) =>
        `${u.name} ${u.id}`.toLowerCase().includes(query.toLowerCase()) &&
        (!role || role === u.role) &&
        (!team ||
          (team === "none"
            ? !u.teams.length
            : u.teams.some((t) => t.id === team))) &&
        (status === "archived"
          ? u.archived
          : !u.archived &&
            (!status || (status === "frozen" ? u.disabled : !u.disabled))),
    );
  const display = legacy ? legacyRows : rows;
  async function inspect(u: Row) {
    setBusy(true);
    setError("");
    try {
      setDetail(await api<Detail>(`/governance/users/${u.id}`));
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  }
  async function prepare(u: Row, action: string) {
    setBusy(true);
    setError("");
    setNotice("");
    setProof("");
    setConfirmationName("");
    setConfirmationMode("name");
    setTakeover(false);
    setConflict(false);
    try {
      setImpact(
        await api<Impact>(`/governance/users/${u.id}/impact?action=${action}`),
      );
      setDetail(undefined);
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      <div className="user-filters">
        <SelectField aria-label="筛选账号状态" value={status} onValueChange={setStatus} options={[{value:"",label:"全部未归档"},{value:"active",label:"正常"},{value:"frozen",label:"已冻结"},{value:"archived",label:"已归档"}]} />
        <SelectField aria-label="筛选账号角色" value={role} onValueChange={setRole} options={[{value:"",label:"全部角色"},...Object.entries(roles).map(([value,label])=>({value,label}))]} />
        <SelectField aria-label="筛选所属团队" value={team} onValueChange={setTeam} options={[{value:"",label:"全部团队"},{value:"none",label:"未加入团队"},...data.teams.map(t=>({value:t.id,label:t.name}))]} />
        <span className="text-muted-foreground">
          {legacy ? display.length : total} 位用户
        </span>
        <Button variant="outline" size="sm" type="submit"

          onClick={() => refresh((n) => n + 1)}
          disabled={loading}
        >
          刷新
        </Button>
      </div>
      {error && (
        <p role="alert" className="banner error">
          {error}
        </p>
      )}
      {notice && (
        <p role="status" className="banner success">
          {notice}
        </p>
      )}
      <div className="surface-panel table-scroll" aria-busy={loading}>
        <DataTable pagination={legacy ? "internal" : "external"} label="用户目录" data={display} getRowId={(u)=>String(u.id)} columns={[{id:"c0",header:<> 用户 </>,cell:(u)=><>
                    <div className="identity-name">
                      <span className="avatar">{u.name.slice(0, 1)}</span>
                      <span>
                        <strong>
                          {u.name}
                          {u.id === user.id && "（我）"}
                        </strong>
                        <small title={u.id}>{u.id.slice(0, 12)}…</small>
                      </span>
                    </div>
                  </>},
{id:"c1",header:<> 所属团队 </>,cell:(u)=><>
                    {u.teams.length ? (
                      u.teams.map((t) => (
                        <Button variant="ghost" type="submit"
                          key={t.id}
                          className="pill"
                          onClick={() => openTeam?.(t.id)}
                        >
                          {t.name}
                        </Button>
                      ))
                    ) : (
                      <span className="text-muted-foreground">未加入团队</span>
                    )}
                  </>},
{id:"c2",header:<> 角色 / 状态 </>,cell:(u)=><>
                    {roles[u.role]}
                    <small
                      className={`account-status ${u.disabled ? "frozen" : ""}`}
                    >
                      {state(u)}
                    </small>
                  </>},
{id:"c3",header:<> 登录 Key </>,cell:(u)=><>
                    {u.key_count === null ? (
                      <span
                        className="text-muted-foreground"
                        title="当前身份无权查看，不能据此判断用户是否有 Key"
                      >
                        Key 信息受限
                      </span>
                    ) : (
                      <Button variant="outline" size="sm" type="submit"

                        disabled={busy}
                        onClick={() => (legacy ? openKeys() : void inspect(u))}
                      >
                        <KeyRound size={14} />
                        {u.key_count} 个 Key
                        <ArrowRight size={13} />
                      </Button>
                    )}
                    {u.active_key_count !== null && (
                      <small>{u.active_key_count} 个有效凭据</small>
                    )}
                  </>},
{id:"c4",header:<> 操作 </>,cell:(u)=><>
                    <div className="row-actions">
                      {!legacy && (
                        <Button variant="outline" size="sm" type="submit"

                          disabled={busy}
                          onClick={() => void inspect(u)}
                        >
                          详情
                        </Button>
                      )}
                      {u.can_manage && !u.archived && (
                        <>
                          <Button variant="outline" size="sm" type="submit"

                            disabled={busy}
                            onClick={() =>
                              void prepare(
                                u,
                                u.disabled ? "unfreeze" : "freeze",
                              )
                            }
                          >
                            {u.disabled ? (
                              <Play size={13} />
                            ) : (
                              <Pause size={13} />
                            )}
                            {u.disabled ? "解冻" : "冻结"}
                          </Button>
                          <Button variant="ghost" size="icon" type="submit"
                            className="icon"
                            title="删除并归档"
                            aria-label={`删除并归档 ${u.name}`}
                            disabled={busy}
                            onClick={() => void prepare(u, "archive")}
                          >
                            <Trash2 size={15} />
                          </Button>
                        </>
                      )}
                      {u.can_manage && u.archived && (
                        <Button variant="outline" size="sm" type="submit"

                          disabled={busy}
                          onClick={() => void prepare(u, "restore")}
                        >
                          <Undo2 size={14} />
                          恢复归档
                        </Button>
                      )}
                      {u.id === user.id && <small>当前账号</small>}
                    </div>
                  </>}]} loading={loading} />
        {loading ? (
          <p className="panel-empty" role="status">
            正在加载用户…
          </p>
        ) : (
          !display.length && (
            <div className="workspace-empty">
              <UsersRound size={28} />
              <h3>没有匹配的用户</h3>
              <p>试试其他名称、状态、角色或团队。</p>
            </div>
          )
        )}
      </div>
      {!legacy && total > 50 && (
        <div className="team-pagination">
          <Button variant="outline" size="sm" type="submit"

            disabled={page <= 1}
            onClick={() => setPage((p) => p - 1)}
          >
            上一页
          </Button>
          <span>第 {page} 页</span>
          <Button variant="outline" size="sm" type="submit"

            disabled={page * 50 >= total}
            onClick={() => setPage((p) => p + 1)}
          >
            下一页
          </Button>
        </div>
      )}
      {detail && (
        <WorkspaceDialog
          title="用户与 Key 详情"
          className="user-detail-dialog"
          close={() => setDetail(undefined)}
        >
          <h3>
            {detail.user.name} · {state(detail.user)}
          </h3>
          <p>
            {roles[detail.user.role]} ·{" "}
            {detail.user.teams.map((t) => t.name).join("、") || "未加入团队"}
          </p>
          <p className="info-note">
            这里只显示 Key 名称、标识和状态。密钥明文仅本人可在 Key
            管理中查看与复制。
          </p>
          {detail.keys_visible ? (
            <div className="user-key-list">
              {detail.keys.map((k) => (
                <article key={k.id}>
                  <KeyRound size={17} />
                  <div>
                    <strong>{k.name}</strong>
                    <code>{k.id}</code>
                    <small>
                      {k.archived
                        ? "已归档"
                        : k.revoked
                          ? "已吊销"
                          : k.disabled
                            ? "已禁用"
                          : k.expires_at &&
                              !k.expires_at.startsWith("0001") &&
                              new Date(k.expires_at) <= new Date()
                            ? "已过期"
                            : "有效"}{" "}
                      ·{" "}
                      {k.expires_at && !k.expires_at.startsWith("0001")
                        ? k.expires_at.slice(0, 10)
                        : "长期有效"}
                    </small>
                  </div>
                </article>
              ))}
              {!detail.keys.length && (
                <p className="panel-empty">此用户尚未签发登录 Key。</p>
              )}
            </div>
          ) : (
            <p className="panel-empty">当前身份无权查看此用户的 Key 元数据。</p>
          )}
          {detail.user.id === user.id && (
            <Button variant="outline" type="submit"

              onClick={() => {
                setDetail(undefined);
                openKeys();
              }}
            >
              前往我的 Key 管理
              <ArrowRight size={14} />
            </Button>
          )}
        </WorkspaceDialog>
      )}
      {impact && (
        <WorkspaceDialog
          title={actions[impact.action]}
          close={() => {
            if (!busy) {
              setImpact(undefined);
              setProof("");
            }
          }}
        >
          <form
            onSubmit={async (e) => {
              e.preventDefault();
              if (busy || !canConfirm) return;
              setBusy(true);
              setError("");
              try {
                await api(
                  `/governance/users/${impact.user.id}/action`,
                  "POST",
                  {
                    action: impact.action,
                    expected_revision: impact.user.revision,
                    confirm: true,
                    proof: confirmationMode === "key" ? proof : "",
                    confirmation_name:
                      confirmationMode === "name" ? confirmationName : "",
                    takeover,
                    takeover_digest: takeover
                      ? impact.takeover_digest
                      : undefined,
                  },
                );
                setImpact(undefined);
                setProof("");
                refresh((n) => n + 1);
                window.dispatchEvent(
                  new Event("drivebridge-governance-changed"),
                );
                setNotice(
                  `${impact.user.name}：${actions[impact.action]}已完成。`,
                );
              } catch (e) {
                setError(errorText(e));
                if ((e as Error).message === "version_conflict")
                  setConflict(true);
              } finally {
                setBusy(false);
              }
            }}
          >
            <h3>{impact.user.name}</h3>
            <p className="info-note">
              这是全局账号操作，影响该用户在所有租户中的访问。
            </p>
            <p>
              {impact.action === "archive"
                ? "停止账号访问，吊销登录 Key、Agent 与分享，保留文件及审计记录。归档不等于永久删除数据。"
                : impact.action === "freeze"
                  ? "立即停止此用户的访问并退出已有会话；文件、成员关系与 Key 记录保留，解冻后可重新登录。"
                  : impact.action === "restore"
                    ? "恢复到已冻结状态，不重新启用旧 Key、Agent 或分享。之后可解冻账号，并由本人恢复登录凭据。"
                    : "重新允许账号登录；已失效的会话和临时授权不会恢复。"}
            </p>
            <div className="team-impact">
              {Object.entries(impact.counts).map(([key, n]) => (
                <span key={key}>
                  {{
                    keys: "登录 Key",
                    sessions: "登录会话",
                    agents: "Agent",
                    spaces: "个人网盘",
                    files: "个人文件",
                  }[key] || key}
                  ：{n}
                </span>
              ))}
            </div>
            {!impact.allowed && !impact.takeover_allowed && (
              <p className="banner error" role="alert">
                {errorText(new Error(impact.reason))}
              </p>
            )}
            {impact.takeover_allowed && (
              <div className="account-takeover">
                <strong>由最高管理员接管后继续</strong>
                <p>确认后，你将接任下列职责，原账号按所选操作停用。</p>
                <ul>
                  {impact.handoffs?.map((h) => (
                    <li key={h.kind + h.id}>
                      {h.name} ·{" "}
                      {
                        {
                          tenant_admin: "租户管理员",
                          team_admin: "团队管理员",
                          custodian: "网盘数据负责人",
                        }[h.kind]
                      }
                    </li>
                  ))}
                </ul>
                <label className="checkbox-label">
                  <Checkbox

                    checked={takeover}
                    onCheckedChange={(e) => setTakeover(e)}
                  />
                  由我接管这些职责并继续
                </label>
              </div>
            )}
            {(impact.allowed || impact.takeover_allowed) && (
              <>
                <div className="segmented" aria-label="操作确认方式">
                  <Button variant="ghost"
                    type="button"
                    aria-pressed={confirmationMode === "name"}
                    onClick={() => {
                      setConfirmationMode("name");
                      setProof("");
                    }}
                  >
                    输入用户名
                  </Button>
                  <Button variant="ghost"
                    type="button"
                    aria-pressed={confirmationMode === "key"}
                    onClick={() => {
                      setConfirmationMode("key");
                      setConfirmationName("");
                    }}
                  >
                    验证我的 Key
                  </Button>
                </div>
                {confirmationMode === "name" ? (
                  <label>
                    输入目标用户名确认
                    <Input
                      autoFocus
                      value={confirmationName}
                      onChange={(e) => setConfirmationName(e.target.value)}
                      placeholder={impact.user.name}
                      autoComplete="off"
                    />
                  </label>
                ) : (
                  <label>
                    当前登录 Key（身份确认）
                    <Input
                      type="password"
                      autoComplete="off"
                      value={proof}
                      onChange={(e) => setProof(e.target.value)}
                    />
                  </label>
                )}
              </>
            )}
            {error && (
              <p role="alert" className="banner error">
                {error}
              </p>
            )}
            {conflict && <p>账号状态已变化，请关闭后重新查看操作影响。</p>}
            <div className="dialog-actions">
              <Button variant="outline"
                type="button"

                disabled={busy}
                onClick={() => {
                  setImpact(undefined);
                  setProof("");
                }}
              >
                取消
              </Button>
              <Button type="submit"
                variant={impact.action === "archive" ? "destructive" : "default"}
                disabled={busy || !canConfirm}
              >
                {busy ? "处理中…" : `确认${actions[impact.action]}`}
              </Button>
            </div>
          </form>
        </WorkspaceDialog>
      )}
    </>
  );
}
