import { AppSheet } from "./components/AppSheet";
import { DataTable } from "./components/DataTable";
import { Identifier } from "./components/Identifier";
import { Badge } from "./components/ui/badge";
import { governanceURL } from "./workspace-route";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "./components/ui/tabs";
import { Popover, PopoverTrigger, PopoverContent } from "./components/ui/popover";
import { Checkbox } from "./components/ui/checkbox";
import { Input } from "./components/ui/input";
import { Button, buttonVariants } from "./components/ui/button";
import { SelectAdapter } from "./components/SelectAdapter";
import React, { useEffect, useRef, useState } from "react";
import {
  api,
  contextURL,
  download,
  logout,
  errorText,
  getDataContext,
  setDataContext,
  type DriveFile,
  type Space,
} from "./api";
import { purgeLocalFlowState } from "./consumer-flows/compat";
import { AppDialog as WorkspaceDialog } from "./components/AppDialog";
import "./governance.css";
import { SlidersHorizontal, ShieldCheck, LogOut, KeyRound, Plug, Bot } from "lucide-react";

import { GovernanceBar, useGovernance, governanceMode } from "./governance-context";
export { GovernanceBar, useGovernance, governanceMode };

import type { GovernanceState } from "./governance-context";
export type { GovernanceState } from "./governance-context";
const roleLabels: Record<string, string> = {
  owner: "本人空间",
  admin: "管理只读",
  custodian: "数据负责人",
  editor: "已授权编辑",
  viewer: "只读协作",
};
const states: Record<string, string> = {
  pending: "待审批",
  approved: "已授权",
  rejected: "已拒绝",
  revoked: "已撤销",
  expired: "已过期",
  executing: "执行中",
  completed: "已完成",
  partial: "部分失败",
  interrupted: "执行中断",
  succeeded: "成功",
  failed: "失败",
};
const actions: Record<string, string> = {
  "account.provisioned": "新账号等待激活",
  "account.activated": "账号已激活",
  "account.platform_role": "平台管理员角色变化",
  "member.role": "租户管理员角色变化",
  "governance.bootstrap": "部署侧治理引导",
  "data.view": "管理员查看",
  "data.download": "文件下载或导出",
  "request.created": "新的授权申请",
  "request.approved": "申请已批准",
  "request.rejected": "申请被拒绝",
  "request.revoked": "授权已撤销",
  "request.expired": "授权已过期",
  "request.result": "操作执行结果",
  "request.item": "清单操作结果",
  "request.executing": "操作开始执行",
  "emergency.started": "应急访问已启动",
  "member.invited": "租户邀请",
  "member.accepted": "成员已加入",
  "member.removed": "成员已移除",
  "team.created": "团队已创建",
  "team.member": "团队成员变化",
  "space.created": "团队空间已创建",
  "space.permission": "空间权限变化",
  "tenant.invited": "租户管理员邀请",
  "tenant.status": "租户状态变化",
  "account.recovery_requested": "账号恢复申请",
  "account.recovery_verified": "人工核验通过",
  "account.recovered": "账号已恢复",
  "account.privileges_restored": "特权已重新确认",
  "account.status": "账号状态变化",
  "account.recovery_code": "恢复码已更新",
};
const FormFeedback = React.createContext({ error: "", clear: () => {} });
function Form({ title, submit, children, busy, standalone = false }: {
  standalone?: boolean; title: string; submit: (data: FormData) => Promise<void | boolean>;
  children: React.ReactNode; busy: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [pending, setPending] = useState(false);
  const [localError, setLocalError] = useState("");
  const feedback = React.useContext(FormFeedback);
  const submitting = busy || pending;
  const form = <form className="governance-form" onSubmit={async e => {
    e.preventDefault();
    if (submitting) return;
    const values = new FormData(e.currentTarget);
    setPending(true); setLocalError("");
    try { if (await submit(values) !== false) setOpen(false); }
    catch (error) { setLocalError(errorText(error)); }
    finally { setPending(false); }
  }}>
    <div className="grid min-w-0 gap-4 [&>label]:grid [&>label]:gap-2 [&>label]:min-w-0 [&_input]:w-full [&_[data-slot=select-trigger]]:w-full">{children}</div>
    {(localError || feedback.error) && <p role="alert" className="text-sm text-destructive">{localError || feedback.error}</p>}
    <footer className="sticky bottom-0 -mx-4 -mb-4 mt-6 flex justify-end gap-2 border-t border-border bg-popover p-4">
      {!standalone && <Button type="button" variant="outline" disabled={submitting} onClick={() => setOpen(false)}>取消</Button>}
      <Button type="submit" disabled={submitting}>{submitting ? "正在处理…" : title}</Button>
    </footer>
  </form>;
  return standalone ? form : <AppSheet title={title} open={open} onOpenChange={next => { if (!submitting) { setOpen(next); setLocalError(""); feedback.clear(); } }} trigger={<Button type="button" variant="outline" className="mr-2 mb-2" disabled={busy}>{title}</Button>}>{form}</AppSheet>;
}

function Field({
  name,
  label,
  type = "text",
  required = true,
}: {
  name: string;
  label: string;
  type?: string;
  required?: boolean;
}) {
  return (
    <label>
      {label}
      <Input name={name} type={type} required={required} autoComplete="off" />
    </label>
  );
}
function UserField({
  name,
  label,
  users,
  actor,
}: {
  name: string;
  label: string;
  users: { id: string; name: string }[];
  actor: string;
}) {
  const excluded = label.includes("不能为本人");
  return (
    <label>
      {label.replace("账号 ID", "用户")}
      <SelectAdapter aria-label={label.replace("账号 ID", "用户")} name={name} required defaultValue="">
        <option value="" disabled>
          选择用户…
        </option>
        {users
          .filter((u) => !excluded || u.id !== actor)
          .map((u) => (
            <option key={u.id} value={u.id}>
              {u.name} · {u.id.slice(0, 8)}
            </option>
          ))}
      </SelectAdapter>
    </label>
  );
}

export function GovernancePanel({
  initial,
  initialTab = "requests",
  user,
  close,
}: {
  initial: GovernanceState;
  initialTab?: string;
  user: { id: string; name: string };
  close: () => void;
}) {
  const [data, setData] = useState(initial),
    [tab, setTab] = useState(initialTab),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [notice, setNotice] = useState(""),
    [secret, setSecret] = useState(""),
    [secretLabel, setSecretLabel] = useState("恢复码");
  const [requestSpace, setRequestSpace] = useState(
      getDataContext().space || initial.spaces[0]?.id || "",
    ),
    [files, setFiles] = useState<DriveFile[]>([]),
    [selection, setSelection] = useState<string[]>([]);
  const [kind, setKind] = useState("range"),
    [action, setAction] = useState("trash"),
    [target, setTarget] = useState(""),
    [source, setSource] = useState("");
  const mounted = useRef(true);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  const [detail, setDetail] = useState("");
  useEffect(() => { setTab(initialTab); setDetail(""); setError(""); }, [initialTab]);
  const changeTab = (next: string) => { const c = getDataContext(); location.hash = governanceURL(next, c.tenant || data.tenant_id, c.space); };
  const refresh = async () =>
    { const next = await api<GovernanceState>("/governance"); if (mounted.current) setData(next); };
  const perform = async (fn: () => Promise<void>) => {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await fn();
      try { await refresh(); }
      catch { if (mounted.current) setError("操作已完成，列表刷新失败。请重新加载页面查看结果。"); }
      if (mounted.current) window.dispatchEvent(new Event("drivebridge-governance-changed"));
      return true;
    } catch (e) {
      if (mounted.current) setError(errorText(e));
      return false;
    } finally {
      if (mounted.current) setBusy(false);
    }
  };
  const change = async (action: string, body: unknown) => {
    await api("/governance/" + action, "POST", body);
  };
  const name = (id: string) => data.users.find((u) => u.id === id)?.name || id;
  useEffect(() => {
    let alive = true;
    const timer = setInterval(
      () =>
        void api<GovernanceState>("/governance")
          .then((v) => {
            if (alive) setData(v);
          })
          .catch(() => {}),
      10000,
    );
    return () => {
      alive = false;
      clearInterval(timer);
    };
  }, []);
  useEffect(() => {
    if (!requestSpace) return;
    const controller = new AbortController();
    setFiles([]);
    setSelection([]);
    void api<DriveFile[]>(
      `/files?space_id=${encodeURIComponent(requestSpace)}&trash=${action === "purge" || action === "restore"}`,
      "GET",
      undefined,
      { "X-DriveBridge-Space": requestSpace },
      controller.signal,
    )
      .then((v) => {
        if (!controller.signal.aborted) setFiles(v);
      })
      .catch((e) => {
        if (!controller.signal.aborted) setError(errorText(e));
      });
    return () => controller.abort();
  }, [requestSpace, action]);
  const values = (form: FormData) => {
    const out: Record<string, FormDataEntryValue> = {};
    form.forEach((value, key) => (out[key] = value));
    return out;
  };
  const scopeSelect = (
    <label>
      空间
      <SelectAdapter
        aria-label="申请空间"
        value={requestSpace}
        onChange={(e) => setRequestSpace(e.target.value)}
      >
        {data.spaces.map((s) => (
          <option key={s.id} value={s.id}>
            {s.name}
          </option>
        ))}
      </SelectAdapter>
    </label>
  );
  return (
    <section className="governance-page min-w-0" aria-label="管理与授权中心">
      <FormFeedback.Provider value={{error, clear: () => setError("")}}>
      <header className="mb-6 flex flex-wrap items-center gap-x-4 gap-y-2">
        <h2 className="text-xl font-semibold">管理与授权</h2>
        <span className="text-sm text-muted-foreground">{data.tenants.find(t => t.id === data.tenant_id)?.name} · {user.name}</span>
        <Badge variant="secondary">{data.platform_admin ? "平台管理员" : data.tenant_admin ? "租户管理员" : "成员"}</Badge>
      </header>
      <Tabs value={tab} onValueChange={value => changeTab(String(value))}>
        <div className="max-w-full overflow-x-auto border-b border-border pb-1">
          <TabsList variant="line" aria-label="管理页面">
            {[["requests","申请与审批"],["notifications","通知"],["events","访问记录"],["teams", data.tenant_admin || data.platform_admin ? "租户与权限" : "我的邀请"],["accounts",data.platform_admin ? "账号与恢复" : "本人账号恢复"]].map(([id,label]) => <TabsTrigger key={id} value={id}>{label}</TabsTrigger>)}
          </TabsList>
        </div>
        <TabsContent value={tab} key={tab} className="governance-content min-w-0 space-y-4 pt-4">
        {tab === "teams" && <div className="flex flex-wrap gap-2"><a className={buttonVariants({variant:"outline"})} href={`#/teams?tenant=${encodeURIComponent(data.tenant_id)}`}>打开团队工作区</a>{(data.platform_admin || data.tenant_admin) && <a className={buttonVariants({variant:"outline"})} href={`#/members?tenant=${encodeURIComponent(data.tenant_id)}`}>打开用户目录</a>}</div>}
        {error && (
          <p role="alert" className="banner error">
            {error}
          </p>
        )}
        {notice && (
          <p role="status" className="banner">
            {notice}
          </p>
        )}
        {tab === "requests" && (
          <>
            <Form
              title="提交修改申请"
              busy={busy}
              submit={(form) =>
                perform(async () => {
                  const chosen = files.filter((f) => selection.includes(f.id));
                  if (
                    kind === "exact" &&
                    (!chosen.length ||
                      (action === "move" && chosen.length !== 1))
                  )
                    throw new Error("invalid_selection");
                  const sourceFile = files.find((f) => f.id === source);
                  await change("requests", {
                    ...values(form),
                    space_id: requestSpace,
                    kind,
                    action: "write",
                    hours: Number(form.get("hours") || 1),
                    items: chosen.map((f) => ({
                      file_id: f.id,
                      version_id: f.version_id,
                      action,
                      path: target,
                      source_file_id: sourceFile?.id,
                      source_version_id: sourceFile?.version_id,
                    })),
                  });
                  setNotice("申请已提交，批准后仍需由你确认执行。");
                })
              }
            >
              {scopeSelect}
              <label>
                授权方式
                <SelectAdapter value={kind} onChange={(e) => setKind(e.target.value)}>
                  <option value="range">短期范围授权</option>
                  <option value="exact">具体操作清单</option>
                </SelectAdapter>
              </label>
              <Field name="reason" label="申请原因（勿填写凭据）" />
              {kind === "range" ? (
                <>
                  <Field
                    name="prefix"
                    label="限定目录（留空为整个空间）"
                    required={false}
                  />
                  <label>
                    有效小时
                    <Input
                      name="hours"
                      type="number"
                      min="1"
                      max="8"
                      defaultValue="1"
                    />
                  </label>
                  <small>
                    允许新建、上传新文件和范围内移动；覆盖、删除及分享需操作清单。
                  </small>
                </>
              ) : (
                <>
                  <label>
                    操作
                    <SelectAdapter
                      value={action}
                      onChange={(e) => setAction(e.target.value)}
                    >
                      <option value="trash">移入回收站</option>
                      <option value="restore">恢复</option>
                      <option value="purge">永久删除</option>
                      <option value="move">移动或重命名（单文件）</option>
                      <option value="overwrite">用空间内固定版本覆盖</option>
                      <option value="share">创建一天有效的固定版本分享</option>
                    </SelectAdapter>
                  </label>
                  {action === "move" && (
                    <label>
                      目标完整路径
                      <Input
                        value={target}
                        onChange={(e) => setTarget(e.target.value)}
                        required
                      />
                    </label>
                  )}
                  {action === "overwrite" && (
                    <label>
                      来源文件
                      <SelectAdapter
                        value={source}
                        onChange={(e) => setSource(e.target.value)}
                        required
                      >
                        <option value="">选择固定版本来源</option>
                        {files.map((f) => (
                          <option value={f.id} key={f.id}>
                            {f.path}
                          </option>
                        ))}
                      </SelectAdapter>
                    </label>
                  )}
                  <fieldset>
                    <legend>固定版本操作清单</legend>
                    {files.map((f) => (
                      <label key={f.id}>
                        <Checkbox

                          checked={selection.includes(f.id)}
                          onCheckedChange={(e) =>
                            setSelection(
                              e
                                ? [...selection, f.id]
                                : selection.filter((id) => id !== f.id),
                            )}
                        />
                        {f.path}
                        <small>版本 {f.version_id.slice(0, 10)}</small>
                      </label>
                    ))}
                  </fieldset>
                </>
              )}
            </Form>
            <h3>我的申请与待我审批</h3>
            <p className="text-xs text-muted-foreground">管理身份默认只读；批准申请后，操作清单仍需申请人确认执行。</p>
            <DataTable label="申请与审批" data={data.requests} getRowId={r => r.id} columns={[
              {id:"space",header:"空间 / 申请人",cell:r=><><strong>{data.spaces.find(s=>s.id===r.space_id)?.name || r.space_id.slice(0,10)}</strong><small>{name(r.requester)}</small></>},
              {id:"reason",header:"原因",cell:r=><span className="block max-w-72 truncate" title={r.reason}>{r.reason}</span>},
              {id:"status",header:"状态",cell:r=><Badge variant="secondary">{states[r.status] || r.status}</Badge>},
              {id:"action",header:"操作",sortable:false,cell:r=><Button type="button" variant="ghost" onClick={()=>setDetail(r.id)}>查看申请</Button>}
            ]}/>
            <AppSheet title="申请详情" open={data.requests.some(r=>r.id===detail)} onOpenChange={open=>{if(!open)setDetail("");}}>
              {error && <p role="alert" className="mb-4 text-sm text-destructive">{error}</p>}
            {data.requests.filter(r => r.id === detail).map((r) => (
              <article className="governance-card" key={r.id}>
                <strong>
                  {data.spaces.find((s) => s.id === r.space_id)?.name ||
                    r.space_id}{" "}
                  · {states[r.status]}
                </strong>
                <p>
                  {name(r.requester)}：{r.reason}
                </p>
                <small>
                  {r.kind === "range"
                    ? `范围：${r.prefix || "整个空间"}`
                    : r.kind === "emergency"
                      ? "应急访问"
                      : "一次性操作清单"}{" "}
                  · 到期{" "}
                  {new Date(
                    r.status === "pending" ? r.expires_at : r.grant_until,
                  ).toLocaleString()}
                </small>
                {r.items?.map((i) => (
                  <p key={i.file_id}>
                    {i.action} · {i.file_id.slice(0, 10)} · 版本{" "}
                    {i.version_id.slice(0, 10)} {i.path || ""}{" "}
                    {i.status ? states[i.status] || i.status : ""}
                    {i.error && `：${errorText(new Error(i.error))}`}
                    {i.result_id && (
                      <Button variant="ghost" type="submit"
                        onClick={() =>
                          void perform(async () => {
                            const v = await api<{ link: string }>(
                              `/shares/${i.result_id}/link`,
                              "POST",
                              {},
                              { "X-DriveBridge-Space": r.space_id },
                            );
                            await navigator.clipboard.writeText(v.link);
                            setNotice("已复制获批分享链接");
                          })
                        }
                      >
                        复制获批分享链接
                      </Button>
                    )}
                  </p>
                ))}

                {["partial", "interrupted"].includes(r.status) && <p role="status" className="rounded-lg bg-muted p-3 text-sm">清单仅执行一次，已完成项会保留。请核对失败项及最新版本，再从“提交修改申请”创建新的清单；不要重复执行原清单。</p>}
                <div className="governance-actions">
                  {r.status === "pending" && data.approvable[r.id] && (
                    <>
                      <Button variant="outline" type="submit"

                        disabled={busy}
                        onClick={() =>
                          void perform(() => change("approve", { id: r.id }))
                        }
                      >
                        批准申请
                      </Button>
                      <Button variant="outline" type="submit"

                        disabled={busy}
                        onClick={() =>
                          void perform(() => change("reject", { id: r.id }))
                        }
                      >
                        拒绝
                      </Button>
                    </>
                  )}
                  {r.status === "approved" &&
                    r.requester === user.id &&
                    r.kind === "exact" && (
                      <Button variant="default" type="submit"

                        disabled={busy}
                        onClick={() =>
                          void perform(() => change("execute", { id: r.id }))
                        }
                      >
                        确认执行清单
                      </Button>
                    )}
                  {["pending", "approved", "executing"].includes(r.status) &&
                    (r.requester === user.id || data.approvable[r.id]) && (
                      <Button variant="outline" type="submit"

                        disabled={busy}
                        onClick={() =>
                          void perform(() => change("revoke", { id: r.id }))
                        }
                      >
                        撤销
                      </Button>
                    )}
                </div>
              </article>
            ))}
            </AppSheet>
            {data.platform_admin && (
              <Form
                title="启动 15 分钟应急授权"
                busy={busy}
                submit={(form) =>
                  perform(async () => {
                    await change("emergency", {
                      ...values(form),
                      space_id: requestSpace,
                      action: "all",
                    });
                    setNotice("应急已启动，相关人员已产生站内通知。");
                  })
                }
              >
                {scopeSelect}
                <Field name="reason" label="应急原因" />
                <Field name="prefix" label="限定目录" required={false} />
                <Field
                  name="proof"
                  label="重新输入登录密钥（近期 OIDC 登录可留空）"
                  type="password"
                  required={false}
                />
                <p>可在限定范围修改数据，不替代永久角色。15 分钟后自动失效。</p>
              </Form>
            )}
          </>
        )}
        {tab === "notifications" && (
          <>
            <h3>站内通知</h3>
            <p>离线通知会保留。已生成通知不代表收件人已经阅读。</p>
            {data.notifications.length === 0 && <p>暂无通知。</p>}
            <DataTable label="通知" data={data.notifications} getRowId={n=>n.id} columns={[
              {id:"title",header:"通知",cell:n=>actions[n.action] || n.action},
              {id:"actor",header:"操作人",cell:n=>name(n.actor)},
              {id:"time",header:"时间",cell:n=>new Date(n.created_at).toLocaleString()},
              {id:"status",header:"状态",cell:n=><Badge variant={n.read ? "outline" : "secondary"}>{n.read ? "已读" : "未读"}</Badge>},
              {id:"action",header:"操作",sortable:false,cell:n=><Button type="button" variant="ghost" onClick={()=>setDetail(n.id)}>查看通知</Button>}
            ]}/>
            <AppSheet title="通知详情" open={data.notifications.some(n=>n.id===detail)} onOpenChange={open=>{if(!open)setDetail("");}}>
              {error && <p role="alert" className="mb-4 text-sm text-destructive">{error}</p>}
            {data.notifications.filter(n=>n.id===detail).map((n) => (
              <article className="governance-card" key={n.id}>
                <strong>{actions[n.action] || n.action}</strong>
                <p>
                  {name(n.actor)} · {new Date(n.created_at).toLocaleString()}
                </p>
                <small>记录对象：<Identifier value={n.resource_id} /></small>
                {!n.read && (
                  <Button variant="outline" type="submit"

                    onClick={() =>
                      void perform(() =>
                        change("read-notification", { id: n.id }),
                      )
                    }
                  >
                    标为已读
                  </Button>
                )}
              </article>
            ))}

            </AppSheet>          </>
        )}
        {tab === "events" && (
          <>
            <h3>访问与执行记录</h3>
            <p>管理员下载后保存的副本无法撤回。这里保留实际操作人及结果。</p>
            <DataTable label="访问记录" data={data.events} getRowId={e=>e.id} columns={[
              {id:"action",header:"操作",cell:e=><Button type="button" variant="link" onClick={()=>setDetail(e.id)}>{actions[e.action] || e.action}</Button>},
              {id:"actor",header:"操作人",cell:e=>name(e.actor)},
              {id:"outcome",header:"结果",cell:e=><Badge variant="outline">{states[e.outcome] || e.outcome}</Badge>},
              {id:"time",header:"时间",cell:e=>new Date(e.at).toLocaleString()}
            ]}/>
            <AppSheet title="访问详情" open={data.events.some(e=>e.id===detail)} onOpenChange={open=>{if(!open)setDetail("");}}>
              {error && <p role="alert" className="mb-4 text-sm text-destructive">{error}</p>}
            {data.events.filter(e=>e.id===detail).map((e) => (
              <article className="governance-card" key={e.id}>
                <strong>{actions[e.action] || e.action}</strong>
                <p>
                  {name(e.actor)} · {e.outcome} ·{" "}
                  {new Date(e.at).toLocaleString()}
                </p>
                <small>空间：{e.space_id ? <Identifier value={e.space_id} /> : "账号或租户管理"}</small>
              </article>
            ))}

            </AppSheet>          </>
        )}
        {tab === "teams" && (
          <>
            {data.memberships
              .filter((m) => m.status === "invited")
              .map((m) => (
                <article key={m.id} className="governance-card">
                  邀请加入{" "}
                  {data.tenants.find((t) => t.id === m.tenant_id)?.name}
                  <a className={buttonVariants({variant:"outline",size:"default",className:""})} href="#/teams">
                    去团队查看邀请
                  </a>
                  <Button variant="outline" type="submit"

                    onClick={() =>
                      void perform(async () => {
                        await change("accept", { id: m.id });
                        setDataContext(m.tenant_id);
                      })
                    }
                  >
                    接受租户邀请
                  </Button>
                </article>
              ))}
            {data.platform_admin && (
              <Form
                title="创建租户"
                busy={busy}
                submit={(f) => perform(() => change("tenants", values(f)))}
              >
                <Field name="name" label="租户名称" />
                <UserField
                  users={data.users}
                  actor={user.id}
                  name="user_id"
                  label="首位租户管理员账号 ID"
                />
              </Form>
            )}
            {data.tenant_admin && (
              <>
                <Form
                  title="邀请租户成员"
                  busy={busy}
                  submit={(f) => perform(() => change("invite", values(f)))}
                >
                  <UserField
                    users={data.users}
                    actor={user.id}
                    name="user_id"
                    label="已有账号 ID"
                  />
                  <label>
                    角色
                    <SelectAdapter name="role">
                      <option value="member">成员</option>
                      {data.platform_admin && (
                        <option value="admin">租户管理员</option>
                      )}
                    </SelectAdapter>
                  </label>
                </Form>
                <Form
                  title="创建团队"
                  busy={busy}
                  submit={(f) => perform(() => change("teams", values(f)))}
                >
                  <Field name="name" label="团队名称" />
                  <UserField
                    users={data.users}
                    actor={user.id}
                    name="user_id"
                    label="团队管理员"
                  />
                </Form>
              </>
            )}
            <DataTable label="团队" data={data.teams} getRowId={t=>t.id} columns={[
              {id:"name",header:"团队",cell:t=><strong>{t.name}</strong>},
              {id:"members",header:"成员",align:"right",value:t=>data.team_members.filter(m=>m.team_id===t.id).length,cell:t=>data.team_members.filter(m=>m.team_id===t.id).length},
              {id:"action",header:"操作",sortable:false,cell:t=><a className={buttonVariants({variant:"ghost"})} href={`#/teams/${encodeURIComponent(t.id)}/members?tenant=${encodeURIComponent(data.tenant_id)}`}>管理成员与网盘</a>}
            ]}/>
            <DataTable label="空间协作权限" data={data.spaces.filter(s=>s.team_id && (data.tenant_admin || data.space_roles[s.id]==="custodian"))} getRowId={s=>s.id} columns={[
              {id:"name",header:"网盘",cell:s=><strong>{s.name}</strong>},
              {id:"role",header:"我的访问",cell:s=><Badge variant="outline">{roleLabels[data.space_roles[s.id]] || "未授权"}</Badge>},
              {id:"action",header:"操作",sortable:false,cell:s=>(
                <Form
                  title={`设置 ${s.name} 协作权限`}
                  busy={busy}
                  submit={(f) =>
                    perform(() =>
                      change("permission", { ...values(f), space_id: s.id }),
                    )
                  }
                >
                  <UserField
                    users={data.users}
                    actor={user.id}
                    name="user_id"
                    label="团队成员账号 ID（不能为本人）"
                  />
                  <label>
                    角色
                    <SelectAdapter name="role">
                      <option value="viewer">只读协作</option>
                      <option value="editor">编辑</option>
                      <option value="custodian">数据负责人／备份审批人</option>
                      <option value="remove">撤销权限</option>
                    </SelectAdapter>
                  </label>
                </Form>
              )}
            ]}/>

            {data.tenant_admin && (
              <>
                <h3>租户成员</h3>
                <DataTable label="租户成员" data={data.members} getRowId={m=>m.id} columns={[
                  {id:"name",header:"成员",cell:m=>name(m.user_id)},
                  {id:"role",header:"角色",cell:m=>m.role==="admin" ? "租户管理员" : "成员"},
                  {id:"status",header:"状态",cell:m=><Badge variant="outline">{m.status==="active" ? "已加入" : m.status==="invited" ? "待接受" : m.status}</Badge>},
                  {id:"action",header:"操作",sortable:false,cell:m=><>                    {m.user_id !== user.id && m.status === "active" && (
                      <Button variant="outline" type="submit"

                        onClick={() =>
                          void perform(() =>
                            change("remove-member", { user_id: m.user_id }),
                          )
                        }
                      >
                        移出租户，保留数据
                      </Button>
                    )}</>}
                ]}/>              </>
            )}
            {data.platform_admin && (
              <Button variant="outline" type="submit"

                onClick={() =>
                  void perform(() =>
                    change("tenant-status", {
                      id: data.tenant_id,
                      disabled: !data.tenants.find(
                        (t) => t.id === data.tenant_id,
                      )?.disabled,
                    }),
                  )
                }
              >
                {data.tenants.find((t) => t.id === data.tenant_id)?.disabled
                  ? "启用当前租户"
                  : "停用当前租户（保留数据）"}
              </Button>
            )}
          </>
        )}
        {tab === "accounts" && (
          <>
            <p>
              本人账号 ID：<Identifier value={user.id} />
            </p>
            <p>
              租户管理员只能管理成员关系。全局账号恢复和冻结由平台最高管理员处理，恢复凭据仅由本人持有。
            </p>
            <Form
              title="生成本人恢复码"
              busy={busy}
              submit={(f) =>
                perform(async () => {
                  const out = await api<{ recovery_code: string }>(
                    "/governance/recovery-code",
                    "POST",
                    values(f),
                  );
                  setSecretLabel("恢复码");
                  setSecret(out.recovery_code);
                })
              }
            >
              <Field
                name="proof"
                label="重新输入登录密钥（近期 OIDC 登录可留空）"
                type="password"
                required={false}
              />
            </Form>
            {data.platform_admin && (
              <>
                <Form
                  title="开通新账号"
                  busy={busy}
                  submit={(f) =>
                    perform(async () => {
                      const out = await api<{
                        user_id: string;
                        activation_code: string;
                      }>("/governance/provision", "POST", values(f));
                      setSecretLabel("激活码");
                      setSecret(out.activation_code);
                      setNotice(
                        "新账号 ID：" +
                          out.user_id +
                          "。本人需在登录页激活，再接受租户邀请。",
                      );
                    })
                  }
                >
                  <Field name="name" label="账号名称" />
                  <p>
                    激活码仅用于空账号首次领取，24
                    小时有效；管理员不会取得用户的登录密钥。
                  </p>
                </Form>
                <h3>全局账号目录</h3>
                <DataTable label="全局账号" data={data.users} getRowId={u=>u.id} columns={[{id:"name",header:"账号",cell:u=>u.name},{id:"id",header:"标识",cell:u=><Identifier value={u.id}/>}]}/>
                <Form
                  title="设置平台最高管理员"
                  busy={busy}
                  submit={(f) =>
                    perform(() => change("platform-role", values(f)))
                  }
                >
                  <UserField
                    users={data.users}
                    actor={user.id}
                    name="user_id"
                    label="目标账号 ID（不能为本人）"
                  />
                  <label>
                    操作
                    <SelectAdapter name="role">
                      <option value="admin">任命</option>
                      <option value="remove">撤销</option>
                    </SelectAdapter>
                  </label>
                  <Field
                    name="proof"
                    label="重新输入当前登录密钥"
                    type="password"
                    required={false}
                  />
                </Form>
                <Form
                  title="设置当前租户管理员"
                  busy={busy}
                  submit={(f) =>
                    perform(() => change("member-role", values(f)))
                  }
                >
                  <UserField
                    users={data.users}
                    actor={user.id}
                    name="user_id"
                    label="租户成员账号 ID"
                  />
                  <label>
                    角色
                    <SelectAdapter name="role">
                      <option value="admin">租户管理员</option>
                      <option value="member">普通成员</option>
                    </SelectAdapter>
                  </label>
                </Form>
                {data.recoveries
                  .filter((c) => c.status === "pending")
                  .map((c) => (
                    <Form
                      title={`人工核验恢复 ${name(c.user_id)}`}
                      key={c.id}
                      busy={busy}
                      submit={(f) =>
                        perform(() =>
                          change("recovery-approve", {
                            ...values(f),
                            claim_id: c.id,
                          }),
                        )
                      }
                    >
                      <p>
                        请求 {c.id}。仅在已通过独立渠道核验本人及此请求 ID
                        后批准；不要索取新密钥。
                      </p>
                      <Field
                        name="reason"
                        label="核验结论（不要填写证件或凭据）"
                      />
                      <Field
                        name="proof"
                        label="重新输入当前登录密钥"
                        type="password"
                        required={false}
                      />
                    </Form>
                  ))}
                <Form
                  title="重新确认人工恢复后的特权"
                  busy={busy}
                  submit={(f) =>
                    perform(() => change("restore-privileges", values(f)))
                  }
                >
                  <UserField
                    users={data.users}
                    actor={user.id}
                    name="user_id"
                    label="账号 ID（不能为本人）"
                  />
                  <Field
                    name="proof"
                    label="重新输入当前登录密钥"
                    type="password"
                    required={false}
                  />
                </Form>
                <Form
                  title="更新全局账号状态"
                  busy={busy}
                  submit={(f) =>
                    perform(() =>
                      change("account-status", {
                        ...values(f),
                        disabled: f.get("disabled") === "true",
                      }),
                    )
                  }
                >
                  <UserField
                    users={data.users}
                    actor={user.id}
                    name="user_id"
                    label="账号 ID（不能为本人）"
                  />
                  <label>
                    状态
                    <SelectAdapter name="disabled">
                      <option value="true">冻结</option>
                      <option value="false">启用</option>
                    </SelectAdapter>
                  </label>
                  <Field
                    name="proof"
                    label="重新输入当前登录密钥"
                    type="password"
                    required={false}
                  />
                </Form>
              </>
            )}
          </>
        )}
        </TabsContent>
      </Tabs>
      {secret && (
        <WorkspaceDialog
          title={secretLabel + "仅显示一次"}
          close={() => setSecret("")}
        >
          <p>请保存到个人密码管理器，关闭后无法再次查看。</p>
          <code className="one-time-secret">{secret}</code>
          <Button variant="outline" type="submit"

            onClick={() => void navigator.clipboard.writeText(secret)}
          >
            复制{secretLabel}
          </Button>
          <Button variant="outline" type="submit"  onClick={() => setSecret("")}>
            已保存，关闭
          </Button>
        </WorkspaceDialog>
      )}
      </FormFeedback.Provider>
    </section>
  );
}

export function AccountRecovery() {
  const [open, setOpen] = useState(false),
    [claim, setClaim] = useState<{ id: string; claim_proof: string }>(),
    [newKey, setNewKey] = useState(""),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [done, setDone] = useState(false),
    [enrollment, setEnrollment] = useState(true),
    [copied, setCopied] = useState(false);
  const post = async (path: string, body: unknown) => {
    const r = await fetch(path, {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const out = await r.json();
    if (!r.ok) throw new Error(out.error?.code || "request_failed");
    return out.data;
  };
  const run = async (fn: () => Promise<void>) => {
    setBusy(true);
    setError("");
    try {
      await fn();
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <>
      <Button variant="ghost" className="mt-3 w-full" type="button" onClick={() => { setError(""); setCopied(false); setOpen(true); }}>
        激活或恢复账号
      </Button>
      {open && (
        <WorkspaceDialog
          className="account-access-dialog sm:max-w-lg"
          title={claim ? (done ? "密钥已准备好" : "等待恢复确认") : "欢迎使用 DriveBridge"}
          close={() => {
            setOpen(false);
            setClaim(undefined);
            setNewKey("");
            setDone(false);
          }}
        >
          <p className="text-sm leading-6 text-muted-foreground">
            {claim ? "请妥善保存你的登录凭据，关闭后将不再显示。" : "首次使用需要管理员提供激活码。已有账号可在这里找回访问权限。"}
          </p>
          <Tabs className="flex-col" value={enrollment ? "activate" : "recover"} onValueChange={value => { setEnrollment(value === "activate"); setError(""); }}>
          {!claim && <TabsList className="mb-4 h-11 w-full shrink-0" aria-label="账号操作">
            <TabsTrigger value="activate" className="aria-selected:bg-background aria-selected:text-foreground" disabled={busy}>激活新账号</TabsTrigger>
            <TabsTrigger value="recover" className="aria-selected:bg-background aria-selected:text-foreground" disabled={busy}>恢复已有账号</TabsTrigger>
          </TabsList>}
          <TabsContent value={enrollment ? "activate" : "recover"}>
          {error && <p role="alert" className="mb-4 rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">{error}</p>}
          {!claim ? (
            <Form
              standalone
              title={enrollment ? "激活新账号" : "提交恢复申请"}
              busy={busy}
              submit={(f) =>
                run(async () => {
                  const bytes = crypto.getRandomValues(new Uint8Array(48));
                  const key =
                    "dbu_" +
                    Array.from(bytes, (b) =>
                      b.toString(16).padStart(2, "0"),
                    ).join("");
                  if (enrollment) {
                    const out = await post("/auth/enroll", {
                      activation_code: f.get("activation_code"),
                      new_key: key,
                    });
                    setNewKey(key);
                    setClaim({ id: out.user_id, claim_proof: "" });
                    setDone(true);
                  } else {
                    const c = await post("/auth/recovery/request", {
                      user_id: f.get("user_id"),
                      recovery_code: f.get("recovery_code"),
                      new_key: key,
                    });
                    setNewKey(key);
                    setClaim(c);
                  }
                })
              }
            >
              {enrollment ? (
                <Field
                  name="activation_code"
                  label="一次性激活码"
                  type="password"
                />
              ) : (
                <>
                  <Field name="user_id" label="本人账号 ID" />
                  <Field
                    name="recovery_code"
                    label="恢复码（没有则申请人工核验）"
                    type="password"
                    required={false}
                  />
                </>
              )}
              <div className="flex gap-3 rounded-lg bg-muted p-4 text-sm leading-6 text-muted-foreground">
                <ShieldCheck className="mt-1 size-4 shrink-0" />
                <p>{enrollment ? "激活后会生成仅由你保管的登录密钥。请立即保存，管理员无法替你查看。" : "有恢复码可直接验证；没有恢复码时，需要管理员通过独立渠道核对本人和请求 ID。请勿向管理员发送新密钥。"}</p>
              </div>
            </Form>
          ) : (
            <>
              <p>
                {enrollment ? "账号 ID：" : "恢复请求 ID："}<code>{claim.id}</code>
              </p>
              <p>
                请保存新密钥；刷新或关闭页面会丢失本次领取证明。不要把新密钥或领取证明发送给管理员。
              </p>
              <code className="one-time-secret">{newKey}</code>
              <Button variant="outline" type="submit"

                onClick={() => void run(async () => { await navigator.clipboard.writeText(newKey); setCopied(true); })}
              >
                {copied ? "已复制，请妥善保存" : "复制新密钥"}
              </Button>
              {done ? (
                <p role="status">
                  {enrollment ? "账号激活完成。请保存新密钥，关闭窗口后登录并接受团队邀请。" : "恢复完成。请关闭窗口并使用新密钥登录；人工恢复后的特权需另行确认。"}
                </p>
              ) : (
                <Button variant="default" type="submit"

                  disabled={busy}
                  onClick={() =>
                    void run(async () => {
                      await post("/auth/recovery/complete", claim);
                      setDone(true);
                    })
                  }
                >
                  查询批准并领取
                </Button>
              )}
            </>
          )}
          </TabsContent>
          </Tabs>
        </WorkspaceDialog>
      )}
    </>
  );
}
