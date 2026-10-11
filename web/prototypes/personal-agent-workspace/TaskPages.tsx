import { RecordRow } from "./CompactUI";
import { FileButton } from "./HoverPreview";
import { taskPreviewScope } from "./preview-model";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "./Dialog";
import { fileUnavailable } from "./v4-model";
import React, { useState } from "react";
import {
  ArrowRight,
  ArrowUpRight,
  Bell,
  Bot,
  Check,
  ChevronDown,
  CircleAlert,
  Cloud,
  Copy,
  Download,
  Folder,
  HardDrive,
  History,
  Laptop,
  Link2,
  LockKeyhole,
  Pause,
  Play,
  Plus,
  RefreshCw,
  RotateCcw,
  ShieldCheck,
  Smartphone,
  Sparkles,
  Unplug,
  Wifi,
  X,
} from "lucide-react";
import { cn } from "@/lib/utils";
import type { Task } from "./model";
import {
  Btn,
  Choice,
  Empty,
  FileIcon,
  Heading,
  IconBtn,
  panel,
  Pill,
  SectionTitle,
  TaskPill,
  useDemo,
} from "./ui";

export function Tasks({ transferOnly = false }: { transferOnly?: boolean }) {
  const { state: s, dispatch } = useDemo();
  const [filter, setFilter] = useState("all");
  const [kind, setKind] = useState("all");
  const [expanded, setExpanded] = useState("");
  const tasks = s.tasks.filter(
    (t) =>
      t.space === s.space &&
      (s.actor === "yeisme" ||
        t.items.every(
          (i) =>
            !fileUnavailable(
              s,
              s.files.find((f) => f.id === i.fileId),
              i.version,
            ),
        )) &&
      (!s.storage.taskConnection ||
        t.targetConnectionId === s.storage.taskConnection ||
        t.items.some(
          (i) =>
            s.files.find((f) => f.id === i.fileId)?.connectionId ===
            s.storage.taskConnection,
        )) &&
      (!transferOnly || ["upload", "transfer"].includes(t.kind)) &&
      (filter === "all" ||
        (filter === "recover" &&
          ["partial", "paused", "stale", "blocked", "unknown"].includes(
            t.status,
          )) ||
        t.status === filter) &&
      (kind === "all" ||
        (kind === "agent" && !["upload", "transfer"].includes(t.kind)) ||
        (kind === "transfer" && ["upload", "transfer"].includes(t.kind))),
  );
  return (
    <>
      <Heading
        title="任务中心"
        note="查看进度、确认影响，或从中断的地方继续。"
      />
      {s.storage.taskConnection && (
        <Btn
          variant="ghost"
          onClick={() =>
            dispatch({ type: "storage-transfer", connectionId: "" })
          }
        >
          清除连接筛选
        </Btn>
      )}
      <div className="mb-5 flex flex-wrap items-center gap-2">
        {[
          { value: "all", label: "全部" },
          { value: "approval", label: "待确认" },
          { value: "running", label: "进行中" },
          { value: "recover", label: "待恢复" },
          { value: "done", label: "已完成" },
        ].map((x) => (
          <Btn
            key={x.value}
            variant={filter === x.value ? "secondary" : "ghost"}
            onClick={() => setFilter(x.value)}
            aria-pressed={filter === x.value}
          >
            {x.label}
          </Btn>
        ))}
        <Choice
          label="任务类别"
          value={kind}
          onChange={setKind}
          options={[
            { value: "all", label: "所有类别" },
            { value: "agent", label: "外部应用与整理" },
            { value: "transfer", label: "文件传输" },
          ]}
        />
      </div>
      <div>
        {tasks.map((task) => (
          <TaskCard
            key={task.id}
            task={task}
            details={false}
            onDetails={() => setExpanded(task.id)}
          />
        ))}
      </div>
      <Dialog open={!!expanded} onOpenChange={(v) => !v && setExpanded("")}>
        <DialogContent kind="detail">
          <DialogTitle>任务记录与关联文件</DialogTitle>
          <DialogDescription>
            输入与产物分别引用固定版本，恢复沿用原操作。
          </DialogDescription>
          {s.tasks
            .filter((t) => t.id === expanded && t.space === s.space)
            .map((t) => (
              <TaskCard
                key={t.id}
                task={t}
                details
                onDetails={() => setExpanded("")}
              />
            ))}
        </DialogContent>
      </Dialog>
      {!tasks.length && (
        <Empty
          title="这里没有待处理任务"
          text="调整筛选，或从文件页创建操作。"
          action="查看全部任务"
          onAction={() => {
            setFilter("all");
            setKind("all");
          }}
        />
      )}
    </>
  );
}
function TaskCard({
  task: t,
  details,
  onDetails,
}: {
  task: Task;
  details: boolean;
  onDetails: () => void;
}) {
  const { state: s, dispatch, open } = useDemo();
  const complete = t.items.filter((i) => i.status === "done").length;
  if (!details)
    return (
      <RecordRow
        name={t.title}
        identity={{ "data-task-id": t.id }}
        icon={<Cloud size={16} />}
        status={<TaskPill status={t.status} />}
        summary={`${t.id} · ${complete}/${t.items.length} 项 · 计划 r${t.revision}`}
        title={
          <FileButton
            reference={
              t.items.length === 1
                ? { id: t.items[0].fileId, version: t.items[0].version }
                : undefined
            }
            layout="surface"
            className="!h-6 !min-h-0 !max-w-full !justify-start !px-0 !font-medium"
            onOpen={() =>
              t.items.length === 1
                ? open({
                    type: "preview",
                    id: t.items[0].fileId,
                    version: t.items[0].version,
                    scope: taskPreviewScope(s, t.id),
                  })
                : onDetails()
            }
          >
            <span className="truncate">{t.title}</span>
          </FileButton>
        }
        actions={
          <Btn variant="ghost" onClick={onDetails}>
            {t.status === "approval" ? "查看计划" : "任务记录"}
          </Btn>
        }
      >
        <TaskCard task={t} details onDetails={onDetails} />
      </RecordRow>
    );

  return (
    <article data-task-detail={t.id} className={cn(panel, "overflow-hidden")}>
      <div className="p-5">
        <div className="flex items-start gap-3">
          <span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-accent text-primary">
            {["upload", "transfer"].includes(t.kind) ? (
              <Cloud size={20} />
            ) : (
              <Sparkles size={20} />
            )}
          </span>
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center justify-between gap-2">
              {details && t.items.length !== 1 ? (
                <h2 className="font-semibold">{t.title}</h2>
              ) : (
                <FileButton
                  reference={
                    t.items.length === 1
                      ? { id: t.items[0].fileId, version: t.items[0].version }
                      : undefined
                  }
                  layout="surface"
                  className="!h-auto !justify-start !p-0 text-left font-semibold"
                  onOpen={() =>
                    t.items.length === 1
                      ? open({
                          type: "preview",
                          id: t.items[0].fileId,
                          version: t.items[0].version,
                          scope: taskPreviewScope(s, t.id),
                        })
                      : onDetails()
                  }
                >
                  {t.title}
                </FileButton>
              )}
              <TaskPill status={t.status} />
            </div>
            <p className="mt-1.5 font-mono text-xs text-muted-foreground">
              {t.id} · 计划 r{t.revision} ·{" "}
              {t.agent ? "外部 Agent" : "你的文件操作"}
            </p>
          </div>
        </div>
        {!details && (
          <div className="mt-3 flex flex-wrap gap-2">
            {t.items.slice(0, 2).map((i) => (
              <TaskFileLink
                key={i.fileId}
                id={i.fileId}
                version={i.version}
                label="来源"
                taskId={t.id}
              />
            ))}
            {t.items.length > 2 && (
              <Btn variant="ghost" onClick={onDetails}>
                查看全部 {t.items.length} 个文件
              </Btn>
            )}
          </div>
        )}
        {details && (
          <>
            <div className="mt-4 flex flex-wrap items-center gap-2 text-xs">
              <span className="rounded-md bg-muted px-2 py-1.5">
                {t.source}
              </span>
              <ArrowRight size={14} className="text-muted-foreground" />
              <span className="break-words rounded-md bg-accent px-2 py-1.5 text-primary">
                {t.target}
              </span>
            </div>
            <p className="mt-3 text-xs leading-6 text-muted-foreground">
              {t.note}
            </p>
          </>
        )}
        {details && t.status === "approval" && (
          <div className="mt-3 flex items-start gap-2 rounded-lg bg-[var(--warning-soft)] p-3 text-xs leading-6 text-[var(--warning)]">
            <ShieldCheck size={16} className="mt-1 shrink-0" />
            <span>
              {t.kind === "transfer"
                ? "即将跨存储发送文件，需单独确认目标。"
                : t.kind === "trash"
                  ? "将移入回收站，不会永久删除。"
                  : "等待你的确认，文件尚未修改。"}
            </span>
          </div>
        )}
        {t.status === "running" && (
          <div className="mt-4">
            <div className="mb-2 flex justify-between text-xs text-muted-foreground">
              <span>
                {["upload", "transfer"].includes(t.kind)
                  ? "已发送，正在校验与保存"
                  : "按计划处理文件"}
              </span>
              <span>
                {complete} / {t.items.length} 已确认完成
              </span>
            </div>
            <div
              className="h-1.5 overflow-hidden rounded-full bg-muted"
              role="progressbar"
              aria-label="任务进度"
              aria-valuemin={0}
              aria-valuemax={100}
              aria-valuenow={Math.round(
                (complete / Math.max(1, t.items.length)) * 100,
              )}
            >
              <div
                className="h-full rounded-full bg-primary"
                style={{
                  width: `${(complete / Math.max(1, t.items.length)) * 100}%`,
                }}
              />
            </div>
          </div>
        )}
        {t.status === "partial" && (
          <p className="mt-3 rounded-lg bg-[var(--warning-soft)] p-3 text-xs text-[var(--warning)]">
            {complete} 项已完成，
            {
              t.items.filter(
                (i) =>
                  i.status !== "done" &&
                  i.status !== "skipped" &&
                  i.included !== false,
              ).length
            }{" "}
            项待恢复。
            {t.kind === "transfer"
              ? "同名项尚未覆盖，可以保留两份、跳过或检查版本。"
              : "恢复不会重复处理已完成项。"}
          </p>
        )}
        {t.status === "stale" && (
          <p className="mt-3 text-xs text-destructive">
            文件或计划发生变化。旧确认已失效，请重新预览。
          </p>
        )}
        {t.status === "blocked" && (
          <p className="mt-3 text-xs text-destructive">
            授权已撤销或过期。任务已停止，需要重新授权并预览。
          </p>
        )}
        <div className="mt-4 flex flex-wrap items-center justify-between gap-2">
          {!details && (
            <Btn variant="ghost" className="text-xs" onClick={onDetails}>
              任务记录
            </Btn>
          )}
          <div className="flex flex-wrap gap-2">
            {["approval", "partial", "stale"].includes(t.status) &&
              ["organize", "rename", "copy", "transfer"].includes(t.kind) && (
                <Btn
                  variant="outline"
                  onClick={() => open({ type: "edit-plan", taskId: t.id })}
                >
                  调整计划
                </Btn>
              )}
            {t.status === "partial" &&
              ["transfer", "copy"].includes(t.kind) && (
                <Btn
                  variant="outline"
                  onClick={() => open({ type: "conflict", taskId: t.id })}
                >
                  处理冲突
                </Btn>
              )}
            {t.status === "approval" && (
              <Btn
                onClick={() =>
                  open({
                    type: "confirm",
                    taskId: t.id,
                    revision: t.revision,
                    contextRevision: s.contextRevision,
                  })
                }
              >
                检查并确认
                <ArrowRight size={14} />
              </Btn>
            )}
            {t.status === "running" && (
              <Btn
                variant="outline"
                onClick={() => dispatch({ type: "pause", id: t.id })}
              >
                <Pause size={14} />
                暂停
              </Btn>
            )}
            {["partial", "paused"].includes(t.status) && (
              <Btn onClick={() => dispatch({ type: "recover", id: t.id })}>
                <Play size={14} />
                {t.kind === "transfer" && t.status === "partial"
                  ? "保留两份并继续"
                  : "恢复未完成项"}
              </Btn>
            )}
            {t.status === "unknown" && (
              <Btn onClick={() => dispatch({ type: "reconcile", id: t.id })}>
                <RefreshCw size={14} />
                查询原操作
              </Btn>
            )}
            {t.status === "stale" && (
              <Btn onClick={() => dispatch({ type: "refresh-plan", id: t.id })}>
                <RefreshCw size={14} />
                重新预览
              </Btn>
            )}
            {t.status === "blocked" && (
              <Btn variant="outline" onClick={() => open({ type: "agent" })}>
                检查授权
              </Btn>
            )}
            {t.status === "done" &&
              (t.kind === "organize" ||
                (t.kind === "automation" &&
                  t.automationMode === "organize")) && (
                <Btn
                  variant="outline"
                  onClick={() => dispatch({ type: "compensate", id: t.id })}
                >
                  <RotateCcw size={14} />
                  生成恢复计划
                </Btn>
              )}
            {!["done", "cancelled", "running"].includes(t.status) && (
              <Btn
                variant="ghost"
                onClick={() =>
                  open({
                    type: "danger",
                    title: "取消剩余操作？",
                    text: "不会回退已经完成的修改。原记录会保留。",
                    action: { type: "cancel", id: t.id },
                  })
                }
              >
                取消
              </Btn>
            )}
          </div>
        </div>
      </div>
      {details && (
        <div className="border-t border-border bg-background/50 px-5 py-2">
          {t.revisions?.length ? (
            <p className="py-3 text-xs text-muted-foreground">
              历史修订：
              {t.revisions
                .map((r) => `r${r.revision} · ${r.summary}`)
                .join("；")}
              。原操作编号保持不变。
            </p>
          ) : null}
          {s.files
            .filter((f) => f.id === `${t.id}-output`)
            .map((f) => (
              <TaskFileLink
                key={f.id}
                id={f.id}
                version={f.version}
                label="产物"
                taskId={t.id}
              />
            ))}
          {t.items.map((i) => (
            <div
              key={i.fileId}
              className="flex flex-wrap items-center justify-between gap-2 border-b border-border/60 py-3 last:border-0"
            >
              <div className="min-w-0 flex-1">
                <TaskFileLink
                  id={i.fileId}
                  version={i.version}
                  label="来源"
                  taskId={t.id}
                />
                {i.outputId && (
                  <TaskFileLink
                    id={i.outputId}
                    version={i.outputVersion || 1}
                    label="目标"
                    taskId={t.id}
                  />
                )}
                <p className="mt-1 break-words text-xs text-muted-foreground">
                  {i.beforeName || i.before} → {i.afterName || i.after}
                </p>
                {i.reason && (
                  <p className="mt-1 text-xs text-destructive">{i.reason}</p>
                )}
              </div>
              <Pill
                tone={
                  i.status === "done"
                    ? "green"
                    : i.status === "pending"
                      ? "neutral"
                      : "amber"
                }
              >
                {i.status === "done"
                  ? "已完成"
                  : i.status === "failed"
                    ? "待恢复"
                    : i.status === "stale"
                      ? "已变化"
                      : i.status === "skipped" || i.included === false
                        ? "跳过"
                        : "待处理"}
              </Pill>
            </div>
          ))}
        </div>
      )}
    </article>
  );
}

export function Agents() {
  const { state: s, dispatch, open, go } = useDemo();
  const active = s.agent === "active";
  return (
    <>
      <Heading
        title="外部 Agent"
        note="让熟悉的工具参与工作，文件权限始终由你决定。"
      >
        <Btn onClick={() => open({ type: "agent" })}>
          <Plus size={15} />
          接入 Agent
        </Btn>
      </Heading>
      <div className={cn(panel, "mb-6 p-5")}>
        <div className="flex flex-wrap items-start gap-4">
          <span className="flex size-13 items-center justify-center rounded-2xl bg-accent text-primary">
            <Bot size={27} strokeWidth={1.4} />
          </span>
          <div className="flex-1">
            <h2 className="text-base font-semibold">资料助手</h2>
            <p className="mt-1 text-xs text-muted-foreground">
              外部 MCP 客户端 · 独立身份
            </p>
          </div>
          <Pill tone={active ? "green" : "amber"}>
            {
              {
                active: "连接已验证 · 演示",
                unconnected: "未接入",
                paused: "授权已暂停",
                revoked: "授权已撤销",
                expired: "授权已到期",
              }[s.agent]
            }
          </Pill>
        </div>
        <div className="my-5 grid grid-cols-2 gap-4 rounded-lg bg-background p-4 text-xs">
          <div>
            <p className="text-muted-foreground">允许访问</p>
            <p className="mt-2 font-medium">我的网盘 / {s.agentScope}</p>
          </div>
          <div>
            <p className="text-muted-foreground">有效期</p>
            <p className="mt-2 font-medium">24 小时 · 到期停止</p>
          </div>
          <div>
            <p className="text-muted-foreground">读写范围</p>
            <p className="mt-2">读取资料、提交产物</p>
          </div>
          <div>
            <p className="text-muted-foreground">修改文件</p>
            <p className="mt-2">需要独立计划与确认</p>
          </div>
        </div>
        <div className="flex flex-wrap justify-end gap-2">
          {active ? (
            <>
              <Btn
                variant="ghost"
                onClick={() =>
                  open({
                    type: "danger",
                    title: "撤销资料助手的授权？",
                    text: "立即停止新的访问与未完成任务，已交付的文件保留。",
                    action: { type: "agent", value: "revoked" },
                  })
                }
              >
                撤销授权
              </Btn>
              <Btn
                variant="outline"
                onClick={() => dispatch({ type: "agent", value: "paused" })}
              >
                <Pause size={14} />
                暂停
              </Btn>
              <Btn onClick={() => dispatch({ type: "agent-run" })}>
                <Play size={14} />
                模拟处理资料
              </Btn>
            </>
          ) : (
            <Btn onClick={() => open({ type: "agent" })}>
              <Link2 size={14} />
              {s.agent === "unconnected" ? "设置接入" : "重新设置授权"}
            </Btn>
          )}
        </div>
      </div>
      <div className="mb-5 flex gap-2">
        <Btn
          variant="ghost"
          onClick={() => dispatch({ type: "agent", value: "expired" })}
        >
          模拟授权到期
        </Btn>
      </div>
      <SectionTitle title="每次合作，都有清楚的记录" />
      <div className="grid gap-3">
        {[
          {
            icon: ShieldCheck,
            title: "权限小一点，放心多一点",
            text: "授权具体目录、动作和期限。连接设备不会自动授权整个文件系统。",
          },
          {
            icon: InboxIcon,
            title: "产物先进入收件箱",
            text: "查看输入版本、任务来源和结果后，再接收归档。",
          },
          {
            icon: History,
            title: "执行中也可以收回授权",
            text: "未完成任务停止，已经发生的操作保留可追溯记录。",
          },
        ].map((x) => (
          <div
            key={x.title}
            className="flex gap-3 rounded-lg border border-border p-4"
          >
            <x.icon size={19} className="mt-1 shrink-0 text-primary" />
            <div>
              <h3 className="text-sm font-medium">{x.title}</h3>
              <p className="mt-1 text-xs leading-6 text-muted-foreground">
                {x.text}
              </p>
            </div>
          </div>
        ))}
      </div>
      <Btn variant="link" className="mt-4" onClick={() => go("inbox")}>
        查看产物收件箱
        <ArrowUpRight size={14} />
      </Btn>
    </>
  );
}
function InboxIcon(props: React.ComponentProps<typeof Download>) {
  return <Download {...props} />;
}

export function Devices() {
  const { state: s, dispatch, open, compact } = useDemo();
  const files = s.files.filter((f) => f.space === s.space && !f.trashed);
  return (
    <>
      <Heading
        title="设备与离线"
        note="在合适的设备继续工作，也为没有网络的时候留一份。"
      >
        <Btn onClick={() => open({ type: "device" })}>
          <Plus size={15} />
          关联设备
        </Btn>
      </Heading>
      <div className={cn("mb-4 divide-y divide-border/50")}>
        {[
          { icon: Laptop, name: "我的电脑", text: "本机目录 · 手动选择范围" },
          {
            icon: Smartphone,
            name: "我的手机",
            text: "相册与分享入口 · 按需授权",
          },
        ].map((x) => (
          <div
            key={x.name}
            data-compact-status
            className="flex h-12 items-center gap-3 px-2 max-[767px]:h-14"
          >
            <x.icon size={16} strokeWidth={1.4} className="text-primary" />
            <div className="flex-1">
              <p className="font-medium">{x.name}</p>
              <p className="mt-1 text-xs text-muted-foreground">{x.text}</p>
            </div>
            <Pill tone="green">{s.deviceLinked ? "已关联" : "演示设备"}</Pill>
          </div>
        ))}
      </div>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2 border-y py-2">
        <div>
          <h2 className="text-sm font-semibold">离线缓存</h2>
          <p className="mt-1 text-xs text-muted-foreground">
            {files.filter((f) => f.offlineVersion).length} 个已下载版本 ·
            仅本设备 · 示例容量
          </p>
        </div>
        <Btn
          variant="outline"
          onClick={() => dispatch({ type: "network", online: !s.online })}
        >
          <Wifi size={14} />
          {s.online ? "模拟断网" : "重新连接"}
        </Btn>
      </div>
      <SectionTitle title="选择离线文件" />
      <div className={cn(panel, "divide-y divide-border")}>
        {files.slice(0, 5).map((f) => (
          <div key={f.id} className="flex flex-wrap items-center gap-3 p-4">
            <FileIcon file={f} small />
            <div className="min-w-0 flex-1">
              <p className="truncate text-xs font-medium">{f.name}</p>
              <p className="mt-1 text-xs text-muted-foreground">
                云端 v{f.version} ·{" "}
                {f.offlineVersion ? `本设备 v${f.offlineVersion}` : "尚未下载"}
              </p>
              {f.offlineVersion && f.offlineVersion !== f.version && (
                <p className="mt-1 text-xs text-[var(--warning)]">
                  有新版本，当前副本仍可阅读
                </p>
              )}
            </div>
            <div className="flex flex-wrap gap-1">
              {f.offlineVersion && (
                <Btn
                  variant="ghost"
                  onClick={() =>
                    open({
                      type: "preview",
                      id: f.id,
                      version: f.offlineVersion,
                    })
                  }
                >
                  离线阅读
                </Btn>
              )}
              <Btn
                variant="outline"
                disabled={!s.online}
                onClick={() => dispatch({ type: "offline", id: f.id })}
              >
                <Download size={13} />
                {f.offlineVersion ? "更新副本" : "下载"}
              </Btn>
            </div>
          </div>
        ))}
      </div>
      <div className="mt-5 flex flex-wrap gap-2">
        <Btn
          variant="ghost"
          onClick={() =>
            dispatch({ type: "remote-version", id: files[0]?.id || "f1" })
          }
        >
          模拟云端出现新版本
        </Btn>
        <Btn
          variant="ghost"
          onClick={() =>
            dispatch({
              type: "notice",
              text: "设备空间不足：请清理缓存或减少离线范围。云端文件不会被删除。",
            })
          }
        >
          模拟空间不足
        </Btn>
      </div>
      <p className="mt-4 text-xs leading-6 text-muted-foreground">
        关联设备不等于授权整个设备。此原型的离线阅读、跨端续接均为内存模拟，刷新会重置。
      </p>
    </>
  );
}

export function Shares() {
  const { state: s, dispatch, open } = useDemo();
  const shares = s.shares.filter(
    (x) => s.files.find((f) => f.id === x.fileId)?.space === s.space,
  );
  return (
    <>
      <Heading title="分享与交付" note="固定的版本，清楚的清单。">
        <Btn
          onClick={() =>
            open({
              type: "delivery",
              ids: s.selected.length
                ? s.selected
                : [s.files.find((f) => f.space === s.space && !f.trashed)!.id],
            })
          }
        >
          <Plus size={15} />
          创建交付包
        </Btn>
      </Heading>
      {shares.map((x) => {
        const members = (
          x.members || [{ id: x.fileId, version: x.version }]
        ).map((r) => ({
          ...r,
          version:
            x.mode === "live"
              ? s.files.find((f) => f.id === r.id)?.version || r.version
              : r.version,
        }));
        return (
          <RecordRow
            key={x.id}
            name={
              members.length === 1
                ? s.files.find((f) => f.id === x.fileId)?.name || "分享文件"
                : `交付包 · ${members.length} 项`
            }
            icon={<Link2 size={16} />}
            summary={`${x.mode === "live" ? "持续更新" : "固定版本"} · ${x.audience === "users" ? "指定成员" : "持有链接的人"} · ${x.days || 7} 天`}
            status={<Pill>{x.state === "active" ? "分享中" : "已撤销"}</Pill>}
          >
            <article className={cn(panel, "mb-4 p-5")}>
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <h2 className="text-base font-semibold">
                    {members.length === 1
                      ? `${s.files.find((f) => f.id === x.fileId)?.name} · v${x.version}`
                      : `素材交付包 · ${members.length} 个固定版本`}
                  </h2>
                  <p className="mt-2 text-sm text-muted-foreground">
                    {x.mode === "live" ? "持续更新最新版本" : "固定版本"} ·{" "}
                    {x.audience === "users" ? "指定成员" : "持有链接的人"} ·{" "}
                    {x.passcodeEnabled ? "演示密码保护" : "未设密码"}
                    <br />
                    有效期 {x.days || 7} 天 ·{" "}
                    {x.download === false ? "仅预览" : "允许下载"}
                  </p>
                </div>
                <Pill tone={x.state === "active" ? "green" : "neutral"}>
                  {x.state === "active" ? "分享中 · 演示" : "已撤销"}
                </Pill>
              </div>
              <div className="my-4 divide-y divide-border rounded-xl bg-background px-4">
                {members.map((r) => {
                  const f = s.files.find(
                    (f) => f.id === r.id && f.space === s.space && !f.trashed,
                  );
                  return (
                    <div key={r.id} className="flex items-center gap-3 py-3">
                      {f && <FileIcon file={f} small />}
                      <p className="min-w-0 flex-1 break-words text-sm">
                        {f?.name || "文件当前不可访问"} · v{r.version}
                      </p>
                      <Btn
                        variant="ghost"
                        disabled={!f}
                        onClick={() =>
                          open({
                            type: "preview",
                            id: r.id,
                            version: r.version,
                          })
                        }
                      >
                        {x.mode === "live" ? "查看当前版本" : "查看固定版本"}
                      </Btn>
                    </div>
                  );
                })}
              </div>
              <p className="text-sm leading-7">
                接收事实：
                {x.received ? "已确认接收；未声明领域采纳" : "等待对方接收"}
              </p>
              <p className="mt-1 text-xs text-muted-foreground">
                {x.mode === "live"
                  ? "此分享随所选文件版本更新，不授予编辑权限。"
                  : "交付内容不会跟随原文件更新。"}
                此处没有真实公开链接。
              </p>
              <div className="mt-4 flex flex-wrap justify-end gap-2">
                {x.state === "active" && (
                  <>
                    <Btn
                      variant="outline"
                      onClick={() => dispatch({ type: "receipt", id: x.id })}
                    >
                      模拟对方接收
                    </Btn>
                    <Btn
                      variant="ghost"
                      onClick={() =>
                        open({
                          type: "danger",
                          title: "撤销这份分享？",
                          text: "后续访问将停止，已经下载的副本无法召回。",
                          action: { type: "revoke-share", id: x.id },
                        })
                      }
                    >
                      撤销分享
                    </Btn>
                  </>
                )}
              </div>
            </article>
          </RecordRow>
        );
      })}
      {!shares.length && (
        <Empty
          title="把整理好的资料交出去"
          text="从素材库多选文件，再检查固定版本清单。"
          action="选择交付文件"
          onAction={() =>
            open({
              type: "delivery",
              ids: s.files
                .filter((f) => f.space === s.space && !f.trashed)
                .slice(0, 2)
                .map((f) => f.id),
            })
          }
        />
      )}
    </>
  );
}

export function Automation() {
  const { state: s, dispatch, open } = useDemo();
  return (
    <>
      <Heading
        title="备份与自动整理"
        note="让重复的小事按规则完成，保留随时停止的权利。"
      >
        <Btn onClick={() => open({ type: "rule" })}>
          <Plus size={15} />
          配置规则
        </Btn>
      </Heading>
      <RecordRow
        name={
          s.ruleMode === "backup"
            ? "手机相册 · 单向备份"
            : s.ruleMode === "sync"
              ? "指定目录 · 选择性同步"
              : "新收件 · 自动整理"
        }
        summary={`${s.ruleConfig.source} → ${s.ruleConfig.target}`}
        status={
          <Pill>
            {s.rule === "off"
              ? "未启用"
              : s.rule === "active"
                ? "已启用"
                : "已暂停"}
          </Pill>
        }
      >
        <div className={cn(panel, "mb-6 p-5")}>
          <div className="flex items-start justify-between gap-3">
            <div>
              <h2 className="font-semibold">
                {s.ruleMode === "backup"
                  ? "手机相册 · 单向备份"
                  : s.ruleMode === "sync"
                    ? "指定目录 · 选择性同步"
                    : "新收件 · 自动整理"}
              </h2>
              <p className="mt-2 text-xs leading-6 text-muted-foreground">
                范围：{s.ruleConfig.source}
                <br />
                目标：{s.ruleConfig.target}
                <br />
                条件：{s.ruleConfig.wifiOnly ? "仅 Wi-Fi" : "允许当前网络"} ·
                授权 {s.ruleConfig.validHours} 小时 · 最多{" "}
                {s.ruleConfig.maxFiles} 个文件
              </p>
            </div>
            <Pill tone={s.rule === "active" ? "green" : "amber"}>
              {s.rule === "off"
                ? "未启用"
                : s.rule === "active"
                  ? "已启用 · 演示"
                  : "已暂停"}
            </Pill>
          </div>
          <div className="my-5 rounded-lg bg-background p-4 text-xs leading-6">
            <p>备份：单向保存副本，删除源文件不删除备份。</p>
            <p>同步：需要单独配置方向、冲突和删除传播。</p>
            <p>自动整理：只执行已预授权的目录与动作。</p>
          </div>
          <div className="flex flex-wrap justify-end gap-2">
            <Btn variant="outline" onClick={() => open({ type: "rule" })}>
              查看规则范围
            </Btn>
            {s.rule === "active" && (
              <Btn
                variant="ghost"
                onClick={() => dispatch({ type: "rule-pause" })}
              >
                暂停规则
              </Btn>
            )}
            <Btn
              disabled={s.rule !== "active"}
              onClick={() => dispatch({ type: "rule-run" })}
            >
              <Play size={14} />
              模拟条件受限的运行
            </Btn>
          </div>
        </div>
      </RecordRow>
      <div className="rounded-xl border border-border p-5">
        <div className="flex gap-3">
          <ShieldCheck className="shrink-0 text-primary" size={22} />
          <div>
            <h3 className="text-sm font-semibold">规则不会替你扩大权限</h3>
            <p className="mt-2 text-xs leading-6 text-muted-foreground">
              覆盖、删除、跨空间发送和付费分析仍需独立确认。系统暂停、网络变化或授权到期时，保留原任务与已经完成的部分。
            </p>
          </div>
        </div>
      </div>
    </>
  );
}

function TaskFileLink({
  id,
  version,
  label,
  taskId,
}: {
  taskId?: string;
  id: string;
  version: number;
  label: string;
}) {
  const { state: s, open } = useDemo();
  const f = s.files.find((f) => f.id === id);
  const error = fileUnavailable(s, f, version);
  return (
    <div className="min-w-0">
      <FileButton
        reference={{
          id,
          version,
          connectionId: f?.connectionId,
          externalRevision: f?.externalRevision,
        }}
        variant="link"
        className="!h-auto !min-h-11 !max-w-full !justify-start !whitespace-normal !px-0 text-left text-xs"
        disabled={!!error}
        onOpen={() =>
          open({
            type: "preview",
            id,
            version,
            scope: taskId ? taskPreviewScope(s, taskId) : undefined,
            single: !taskId,
          })
        }
      >
        {label} · {f?.name || "文件尚未生成"} ·{" "}
        {f?.externalRevision ? "固定修订" : `v${version}`}
      </FileButton>
      {error && <p className="text-xs text-muted-foreground">{error}</p>}
    </div>
  );
}
