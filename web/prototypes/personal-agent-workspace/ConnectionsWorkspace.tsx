import { RecordRow } from "./CompactUI";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "./Dialog";
import { useState } from "react";
import {
  ArrowRight,
  Check,
  ChevronDown,
  Copy,
  Link2,
  Plus,
  ShieldCheck,
} from "lucide-react";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Btn, Choice, Empty, Heading, panel, Pill, useDemo } from "./ui";
import { Agents } from "./TaskPages";
export function ExternalConnections() {
  const { state: s, dispatch, open, go } = useDemo();
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("我的外部助手");
  const [protocol, setProtocol] = useState<"MCP" | "API">("MCP");
  const [directoryId, setDirectoryId] = useState(
    s.storage.nodes.find(
      (n) =>
        n.space === s.space &&
        n.connectionId === "drive" &&
        n.path === "山野计划",
    )?.id || s.storage.activeId,
  );
  const folder = s.storage.nodes.find((n) => n.id === directoryId)?.path || "";
  const [actions, setActions] = useState(["read"]);
  const [hours, setHours] = useState("24");
  const [legacy, setLegacy] = useState(false);
  const [configuration, setConfiguration] = useState("");
  const apps = s.apps.filter((a) => a.space === s.space);
  return (
    <>
      <Heading
        title="外部调用"
        note="让 Agent 与应用访问指定文件。MCP 与 API 使用同一份范围和任务记录。"
      >
        <Btn onClick={() => setCreating(!creating)}>
          <Plus size={16} />
          接入应用
        </Btn>
      </Heading>
      <Dialog open={creating} onOpenChange={setCreating}>
        <DialogContent className="!max-h-[85dvh]">
          <DialogTitle>接入应用</DialogTitle>
          <DialogDescription>指定范围与动作后保存演示配置。</DialogDescription>
          <section className="min-h-0 space-y-5 overflow-y-auto">
            <h2 className="font-semibold">
              1. 调用方式 → 2. 范围与动作 → 3. 有效期
            </h2>
            <div className="grid gap-4 @min-[480px]:grid-cols-2">
              <label className="space-y-2 text-xs">
                <span className="block text-muted-foreground">应用名称</span>
                <Input
                  aria-label="应用名称"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  className="h-11"
                />
              </label>
              <label className="space-y-2 text-xs">
                <span className="block text-muted-foreground">调用协议</span>
                <Choice
                  label="调用协议"
                  value={protocol}
                  onChange={(v) => setProtocol(v as typeof protocol)}
                  options={[
                    { value: "MCP", label: "MCP 客户端" },
                    { value: "API", label: "HTTP API 应用" },
                  ]}
                />
              </label>
              <label className="space-y-2 text-xs">
                <span className="block text-muted-foreground">
                  应用授权目录
                </span>
                <Choice
                  label="应用授权目录"
                  value={directoryId}
                  onChange={setDirectoryId}
                  options={s.storage.nodes
                    .filter((n) => n.space === s.space)
                    .map((n) => ({
                      value: n.id,
                      label: `${s.connections.find((c) => c.id === n.connectionId)?.name} / ${n.path}`,
                    }))}
                />
              </label>
              <label className="space-y-2 text-xs">
                <span className="block text-muted-foreground">
                  应用授权期限
                </span>
                <Choice
                  label="应用授权期限"
                  value={hours}
                  onChange={setHours}
                  options={[
                    { value: "1", label: "1 小时" },
                    { value: "24", label: "24 小时" },
                    { value: "168", label: "7 天" },
                  ]}
                />
              </label>
            </div>
            <div className="flex flex-wrap gap-4">
              {[
                { id: "read", label: "读取和检索" },
                { id: "suggest", label: "提交标签建议" },
                { id: "upload", label: "提交产物到收件箱" },
              ].map((a) => (
                <div className="flex min-h-11 items-center gap-3" key={a.id}>
                  <Checkbox
                    aria-label={a.label}
                    checked={actions.includes(a.id)}
                    onCheckedChange={() =>
                      setActions(
                        actions.includes(a.id)
                          ? actions.filter((x) => x !== a.id)
                          : [...actions, a.id],
                      )
                    }
                  />
                  <span className="text-sm">{a.label}</span>
                </div>
              ))}
            </div>
            <p className="text-xs leading-6 text-muted-foreground">
              不授予自动移动、覆盖、删除或凭据管理权限。标签条件不扩大目录授权。此处仅生成演示配置。
            </p>
            <div className="sticky bottom-0 flex gap-2 border-t border-border bg-card pt-4">
              <Btn
                disabled={!name.trim() || !actions.length}
                onClick={() => {
                  dispatch({
                    type: "app-create",
                    name,
                    protocol,
                    folder,
                    directoryId,
                    actions,
                    hours: Number(hours),
                  });
                  setCreating(false);
                }}
              >
                保存演示配置
              </Btn>
              <Btn variant="outline" onClick={() => setCreating(false)}>
                取消
              </Btn>
            </div>
          </section>
        </DialogContent>
      </Dialog>
      {!apps.length && (
        <Empty
          title="还没有接入应用"
          text="先指定目录与动作，再检查连接。普通云盘功能不依赖应用接入。"
        />
      )}
      <div>
        {apps.map((a) => {
          const snippet =
            a.protocol === "MCP"
              ? `演示说明（不能用于认证）\n协议：MCP\n范围：${a.folder}\n连接后读取 tools/list 和 inputSchema\n读取 capabilities / capability_check\n读取只在授权目录内执行\n文件字节通过已公布的 HTTP 上传合同或上传页传递\n未安装 DriveBridge CLI 也可使用已连接 MCP\n这里不包含地址、凭据或可认证密钥`
              : `演示说明（不能用于认证）\n协议：HTTP API\n范围：${a.folder}\nGET /api/v1/capabilities\nPOST /api/v1/capabilities/check\nPOST /api/v1/file-queries\n上传使用 owner 已公布的 upload-batches 合同\n操作超时先查询原 operation_id，不能重新提交\n此示例不包含服务地址与凭据`;
          return (
            <RecordRow
              key={a.id}
              identity={{ "data-app-id": a.id }}
              name={a.name}
              summary={`${a.protocol} · ${a.folder} · ${a.hours} 小时`}
              status={
                <Pill>
                  {
                    {
                      configured: "待验证",
                      verified: "只读已验证",
                      paused: "已暂停",
                      revoked: "已撤销",
                      expired: "已到期",
                    }[a.state]
                  }
                </Pill>
              }
            >
              <article className={`${panel} p-5`}>
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <h2 className="font-semibold">
                    {a.name}{" "}
                    <small className="ml-2 text-muted-foreground">
                      {a.protocol} · {a.id}
                    </small>
                  </h2>
                  <Pill tone={a.state === "verified" ? "green" : "amber"}>
                    {
                      {
                        configured: "配置已保存 · 尚未验证",
                        verified: "只读连接已验证",
                        paused: "授权暂停",
                        revoked: "授权撤销",
                        expired: "授权到期",
                      }[a.state]
                    }
                  </Pill>
                </div>
                <p className="my-3 text-sm leading-7">
                  {a.folder} · {a.hours} 小时 ·{" "}
                  {a.actions
                    .map(
                      (x) =>
                        (
                          ({
                            read: "读取",
                            suggest: "建议",
                            upload: "回存",
                          }) as Record<string, string>
                        )[x],
                    )
                    .join("、")}
                  <br />
                  最近验证：{a.observedAt || "未知，尚未验证"}
                  <br />
                  写入：
                  {a.writeVerified
                    ? "示例产物回存已验证"
                    : "尚未验证，读取通过不代表写入可用"}
                </p>
                <Btn
                  variant="outline"
                  className="mb-4"
                  onClick={() => setConfiguration(a.id)}
                >
                  配置与调用说明
                </Btn>
                <Dialog
                  open={configuration === a.id}
                  onOpenChange={(v) => !v && setConfiguration("")}
                >
                  <DialogContent>
                    <DialogTitle>配置与调用说明（无需本机 CLI）</DialogTitle>
                    <DialogDescription>
                      仅配置示例，无可认证密钥。
                    </DialogDescription>
                    <pre className="mt-3 whitespace-pre-wrap break-words text-xs leading-7">
                      {snippet}
                    </pre>
                    <Btn
                      variant="outline"
                      onClick={async () => {
                        try {
                          await navigator.clipboard.writeText(snippet);
                          dispatch({
                            type: "notice",
                            text: "已复制无凭据的演示说明。",
                          });
                        } catch {
                          dispatch({
                            type: "notice",
                            text: "浏览器未允许复制，请手动选择说明文字。",
                          });
                        }
                      }}
                    >
                      <Copy size={14} />
                      复制配置说明
                    </Btn>
                  </DialogContent>
                </Dialog>
                <div className="flex flex-wrap gap-2">
                  {a.state === "configured" && (
                    <Btn
                      onClick={() =>
                        dispatch({
                          type: "app-state",
                          id: a.id,
                          state: "verified",
                        })
                      }
                    >
                      <ShieldCheck size={15} />
                      5. 模拟只读验证
                    </Btn>
                  )}
                  {a.state === "verified" ? (
                    <>
                      <Btn
                        variant="outline"
                        onClick={() =>
                          dispatch({
                            type: "app-call",
                            id: a.id,
                            action: "read",
                          })
                        }
                      >
                        模拟检索调用
                      </Btn>
                      <Btn
                        variant="outline"
                        onClick={() =>
                          dispatch({
                            type: "app-call",
                            id: a.id,
                            action: "upload",
                          })
                        }
                      >
                        模拟回存产物
                      </Btn>
                      <Btn
                        variant="outline"
                        onClick={() =>
                          dispatch({
                            type: "app-call",
                            id: a.id,
                            action: "suggest",
                          })
                        }
                      >
                        模拟标签建议
                      </Btn>
                      <Btn
                        variant="ghost"
                        onClick={() =>
                          dispatch({
                            type: "app-call",
                            id: a.id,
                            action: "read",
                            unknown: true,
                          })
                        }
                      >
                        演示结果未知
                      </Btn>
                      <Btn
                        variant="ghost"
                        onClick={() =>
                          dispatch({
                            type: "app-state",
                            id: a.id,
                            state: "paused",
                          })
                        }
                      >
                        暂停授权
                      </Btn>
                    </>
                  ) : (
                    a.state !== "configured" && (
                      <Btn
                        variant="outline"
                        onClick={() =>
                          open({
                            type: "danger",
                            title: "重新授权这个应用？",
                            text: `仍限定「${a.folder}」、${a.actions.join(" / ")}，有效 ${a.hours} 小时。保存后重新验证，不自动重跑旧任务。`,
                            action: {
                              type: "app-state",
                              id: a.id,
                              state: "configured",
                            },
                          })
                        }
                      >
                        重新授权
                      </Btn>
                    )
                  )}
                  <Btn
                    variant="ghost"
                    onClick={() =>
                      dispatch({
                        type: "app-state",
                        id: a.id,
                        state: "expired",
                      })
                    }
                  >
                    演示到期
                  </Btn>
                  <Btn
                    variant="ghost"
                    onClick={() =>
                      open({
                        type: "danger",
                        title: "撤销应用授权？",
                        text: "新的调用停止，已回存产物与原任务记录保留。",
                        action: {
                          type: "app-state",
                          id: a.id,
                          state: "revoked",
                        },
                      })
                    }
                  >
                    撤销授权
                  </Btn>
                  <Btn
                    variant="ghost"
                    onClick={() =>
                      dispatch({ type: "capability-focus", id: "external" })
                    }
                  >
                    能力与限制
                  </Btn>
                </div>
              </article>
            </RecordRow>
          );
        })}
      </div>
      <h2 className="mb-3 mt-7 font-semibold">调用记录</h2>
      <div className="divide-y divide-border">
        {s.calls
          .filter((c) => c.space === s.space)
          .map((c) => (
            <article
              key={c.id}
              data-call-id={c.id}
              data-compact-status
              className="flex flex-wrap items-center gap-2 py-1"
            >
              <div className="min-w-0 flex-1">
                <p className="text-sm">
                  {s.apps.find((a) => a.id === c.appId)?.name} · {c.action} ·{" "}
                  {c.scope}
                </p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {c.operationId} · {c.duration} ·{" "}
                  {c.result === "done"
                    ? "已完成"
                    : c.result === "unknown"
                      ? "结果未知"
                      : "拒绝执行"}
                </p>
              </div>
              {c.result === "unknown" && (
                <Btn
                  variant="outline"
                  onClick={() => dispatch({ type: "call-reconcile", id: c.id })}
                >
                  查询原操作
                </Btn>
              )}
              {c.outputId && (
                <Btn variant="outline" onClick={() => go("inbox")}>
                  验收回存产物
                </Btn>
              )}
            </article>
          ))}
        {!s.calls.some((c) => c.space === s.space) && (
          <p className="text-sm text-muted-foreground">
            暂无调用；保存配置不会自动执行任何文件操作。
          </p>
        )}
      </div>
      <article className="mt-3 flex flex-wrap items-center justify-between gap-2 border-t border-border py-2">
        <div>
          <h2 className="text-sm font-semibold">资料助手</h2>
          <p className="mt-1 text-xs text-muted-foreground">
            既有应用 · {s.agent === "active" ? "已授权" : "等待授权"} ·
            原任务与回执保留
          </p>
        </div>
        <Btn variant="outline" onClick={() => setLegacy(true)}>
          查看资料助手
        </Btn>
      </article>
      <Dialog open={legacy} onOpenChange={setLegacy}>
        <DialogContent kind="detail">
          <DialogTitle>资料助手 · 应用详情</DialogTitle>
          <DialogDescription>
            沿用原应用授权、任务和产物记录。
          </DialogDescription>
          <Agents />
        </DialogContent>
      </Dialog>
    </>
  );
}
const capabilityRows = [
  {
    id: "files",
    name: "文件与多标签",
    what: "浏览、上传、收藏与人工打标",
    device: "Web / 桌面 / 手机",
    limit: "仅当前空间授权文件；标签不扩权",
    next: "查看文件",
    page: "files",
  },
  {
    id: "content",
    name: "内容检索",
    what: "在授权目录的索引中查找片段与固定版本",
    device: "已授权内容服务",
    limit: "语义候选不是完整清单；费用和覆盖需独立确认",
    next: "配置索引",
    page: "settings",
  },
  {
    id: "external",
    name: "MCP / API 外部调用",
    what: "能力发现、只读检索和受限产物回存",
    device: "已连接应用，无需本机 CLI",
    limit: "MCP 客户端路径不是远端文件；字节需 HTTP 或上传页",
    next: "管理应用授权",
    page: "agents",
  },
  {
    id: "storage",
    name: "多存储传输",
    what: "目录浏览、复制与逐项失败恢复",
    device: "取决于当前连接的能力",
    limit: "连接失效保留原任务；支持列表不等于已通过写入验收",
    next: "查看连接与传输",
    page: "connections",
  },
  {
    id: "offline",
    name: "离线与设备",
    what: "下载固定版本，重连后比较新旧副本",
    device: "Web 演示 / 未来原生宿主",
    limit: "原型不证明加密缓存、真实断网持久化和系统后台能力",
    next: "查看离线文件",
    page: "devices",
  },
  {
    id: "native",
    name: "相册、系统分享与后台传输",
    what: "通过原生宿主使用系统能力",
    device: "原生客户端待实现",
    limit: "Web 没有此能力；无法承诺关闭应用后继续本机传输",
    next: "设备与系统说明",
    page: "devices",
  },
] as const;
export function Capabilities() {
  const { state: s, dispatch, go } = useDemo();
  const [connection, setConnection] = useState(s.capabilityConnection);
  const instance = s.connections.find((c) => c.id === connection);
  return (
    <>
      <Heading
        title="能力与连接说明"
        note="配置、连接观测、权限与实际能力分别展示。未知状态不会默认视为可用。"
      />
      <Choice
        label="检查的连接"
        value={connection}
        onChange={setConnection}
        className="mb-5 w-full max-w-sm"
        options={[
          ...s.connections
            .filter((c) => c.id === "drive" || c.space === s.space)
            .map((c) => ({ value: c.id, label: `${c.name} · 演示连接` })),
          { value: "native", label: "原生设备 · 待接入" },
        ]}
      />
      <div className="divide-y divide-border rounded-xl border border-border bg-card">
        {capabilityRows.map((c) => {
          const status =
            s.viewState === "forbidden"
              ? "待授权"
              : !s.online || s.viewState === "disconnected"
                ? "需重连"
                : c.id === "native" || connection === "native"
                  ? "不支持"
                  : instance?.status !== "active" || !instance.authorized
                    ? "需重新授权"
                    : (c.id === "storage" && instance.write === "unknown") ||
                        instance.read !== "passed"
                      ? "未知 · 尚未验证"
                      : c.id === "content" &&
                          !s.indexes.some((i) => i.space === s.space)
                        ? "待配置"
                        : c.id === "external" &&
                            !s.apps.some(
                              (a) =>
                                a.space === s.space && a.state === "verified",
                            )
                          ? "待授权"
                          : "可演示";
          return (
            <article
              key={c.id}
              className={`p-3 ${s.capabilityFocus === c.id ? "bg-accent/40" : ""}`}
            >
              <div className="flex flex-wrap items-center justify-between gap-2">
                <h2 className="font-semibold">{c.name}</h2>
                <Pill tone={status === "可演示" ? "green" : "amber"}>
                  {status}
                </Pill>
              </div>
              <p className="my-1 max-w-[70ch] text-[13px] leading-6">
                {c.what}
              </p>
              <dl className="grid gap-2 text-xs leading-6 min-[1000px]:grid-cols-2">
                <div>
                  <dt className="text-muted-foreground">设备／连接</dt>
                  <dd>
                    {instance?.name || "原生设备"} · {c.device}
                  </dd>
                </div>
                <div>
                  <dt className="text-muted-foreground">最近验证</dt>
                  <dd>
                    {c.id === "external"
                      ? s.apps.find((a) => a.space === s.space)?.observedAt ||
                        "未知"
                      : c.id === "content"
                        ? s.indexes.find((i) => i.space === s.space)
                            ?.observedAt || "未知"
                        : `${instance?.verifiedAt || "未知"}；真实连接尚未验证`}
                  </dd>
                </div>
              </dl>
              <p className="my-2 max-w-[80ch] text-xs leading-6 text-muted-foreground">
                限制：{c.limit}
              </p>
              <Btn
                variant="outline"
                onClick={() =>
                  c.id === "content"
                    ? dispatch({ type: "settings-section", section: "search" })
                    : go(c.page)
                }
              >
                {c.next}
                <ArrowRight size={14} />
              </Btn>
            </article>
          );
        })}
      </div>
      <p className="mt-5 text-xs leading-7 text-muted-foreground">
        实现依据、当前环境是否配置、当前身份是否授权和实际观测分别判断。该页只显示模拟投影，不探测真实连接、不修改服务配置。
      </p>
    </>
  );
}
