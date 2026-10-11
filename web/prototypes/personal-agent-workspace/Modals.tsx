import { QuickFind } from "./UnifiedFind";
import { PropertyBatchEditor } from "./PropertyWorkspace";
import { BookmarkGroups } from "./FavoritesWorkspace";
import { entryRef } from "./organization-model";
import { ResourcePlan } from "./ResourceWorkspace";
import { VersionDiscussion } from "./CollaborationWorkspace";
import { fileUnavailable } from "./v4-model";
import { TagBatch, FileTags } from "./TagWorkspace";
import React, { useEffect, useState } from "react";
import {
  ArrowDownToLine,
  ArrowRight,
  Camera,
  Check,
  Cloud,
  Copy,
  FileText,
  Folder,
  Image,
  Laptop,
  Link2,
  LockKeyhole,
  MessageSquare,
  Pause,
  Play,
  Plus,
  RotateCcw,
  Search,
  Share2,
  ShieldCheck,
  Smartphone,
  Sparkles,
  Upload,
  X,
  ZoomIn,
  ZoomOut,
} from "lucide-react";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import {
  Btn,
  Choice,
  Empty,
  FileArt,
  FileIcon,
  IconBtn,
  panel,
  Pill,
  useDemo,
} from "./ui";
import type { Modal } from "./ui";
import type { FileRecord, State } from "./model";
import { versionSnapshot } from "./model";
import {
  WorkspaceDialog,
  RuleEditor,
  DeliveryEditor,
} from "./WorkspaceDialogs";

export function ModalBody({ modal: m }: { modal: Modal }) {
  const { state: s, dispatch, close, go, open } = useDemo();
  const uploadDirectory =
    s.page === "files"
      ? s.storage.nodes.find((n) => n.id === s.storage.activeId)
      : s.storage.nodes.find(
          (n) =>
            n.path === s.options.uploadTarget &&
            n.space === s.space &&
            n.connectionId === "drive",
        );
  const [query, setQuery] = useState("");
  const [scope, setScope] = useState("山野计划");
  const [step, setStep] = useState(1);
  const selected = s.files.filter(
    (f) => s.selected.includes(f.id) && f.space === s.space && !f.trashed,
  );
  const task =
    m.type === "confirm"
      ? s.tasks.find((t) => t.id === m.taskId && t.space === s.space)
      : undefined;
  if (
    [
      "compare",
      "organize",
      "collection",
      "new-folder",
      "conversation",
      "edit-plan",
      "conflict",
    ].includes(m.type)
  )
    return <WorkspaceDialog modal={m} />;
  if (m.type === "preview") return null;
  if (m.type === "duplicates") {
    const candidates = s.files.filter(
      (f) => ["f2", "f12"].includes(f.id) && f.space === s.space && !f.trashed,
    );
    return (
      <div className="space-y-4">
        <p className="text-sm leading-7">
          这组演示文件代表摘要相同的精确重复项。相似图片或同名文件不会作为删除依据。
        </p>
        {candidates.map((f) => (
          <div
            key={f.id}
            className="flex gap-3 rounded-lg border border-border p-3"
          >
            <FileIcon file={f} />
            <div>
              <p className="text-xs font-medium">{f.name}</p>
              <p className="mt-1 text-xs text-muted-foreground">
                {f.folder} · {f.size} · v{f.version}
              </p>
            </div>
          </div>
        ))}
        <Pill tone="green">演示：已验证摘要一致</Pill>
        <p className="text-xs leading-6 text-muted-foreground">
          保留主资料中的原件，仅为临时目录中的副本创建回收站计划。原型未执行真实摘要扫描。
        </p>
        <Btn
          disabled={candidates.length < 2}
          onClick={() => {
            dispatch({
              type: "plan",
              ids: ["f12"],
              kind: "trash",
              target: "回收站",
            });
            close();
          }}
        >
          预览副本回收计划
        </Btn>
      </div>
    );
  }
  if (m.type === "confirm" && s.knowledge.taskPlans[m.taskId])
    return <ResourcePlan taskId={m.taskId} />;
  if (m.type === "confirm")
    return task ? (
      <div className="space-y-4">
        <p className="text-sm leading-7">
          {task.title}。请检查修改前后与目标位置，确认只对本次固定清单有效。
        </p>
        <div className="flex flex-wrap gap-2">
          <Pill tone="amber">
            {task.kind === "transfer"
              ? "跨存储发送 · 单独确认"
              : task.kind === "trash"
                ? "移入回收站 · 可恢复"
                : "文件修改 · 需要确认"}
          </Pill>
          <Pill>{task.items.length} 个文件</Pill>
          <Pill>
            {task.id} · r{m.revision}
          </Pill>
        </div>
        <div className="divide-y divide-border rounded-xl border border-border px-4">
          {task.items.map((i) => (
            <div key={i.fileId} className="py-3">
              <p className="text-xs font-medium">
                {s.files.find((f) => f.id === i.fileId)?.name} · v{i.version}
              </p>
              <p className="mt-2 break-words text-xs text-muted-foreground">
                {i.beforeName || i.before} → {i.afterName || i.after}
                {i.included === false
                  ? " · 本次跳过"
                  : i.status === "done"
                    ? " · 已完成保留"
                    : ""}
              </p>
            </div>
          ))}
        </div>
        <p className="rounded-lg bg-accent p-3 text-xs leading-6 text-primary">
          原版本和操作记录保留。部分失败只恢复未完成项，取消不会回退已经发生的修改。
        </p>
        {task.status !== "approval" && (
          <p role="alert" className="text-sm text-destructive">
            计划已变化或已经处理，请关闭后重新预览。
          </p>
        )}
        <div className="sticky bottom-0 z-10 flex flex-wrap justify-end gap-2 border-t border-border bg-card pt-4">
          <Btn variant="outline" onClick={close}>
            暂不执行
          </Btn>
          <Btn
            disabled={task.status !== "approval"}
            onClick={() => {
              dispatch({
                type: "confirm",
                id: task.id,
                revision: m.revision,
                contextRevision: m.contextRevision,
              });
              close();
            }}
          >
            确认执行这份计划
          </Btn>
        </div>
        <div className="border-t border-dashed border-border pt-3">
          <Btn
            variant="ghost"
            className="text-xs text-muted-foreground"
            onClick={() => dispatch({ type: "invalidate", id: task.id })}
          >
            演示异常：确认前计划发生变化
          </Btn>
        </div>
      </div>
    ) : (
      <Empty
        title="原计划当前不可见"
        text="空间或权限已变化，请返回原空间查看。"
      />
    );
  if (m.type === "danger")
    return (
      <div className="space-y-5">
        <p className="text-sm leading-7">{m.text}</p>
        <div className="sticky bottom-0 z-10 flex justify-end gap-2 border-t border-border bg-card pt-4">
          <Btn variant="outline" onClick={close}>
            取消
          </Btn>
          <Btn
            variant={m.action.type === "select" ? "default" : "destructive"}
            onClick={() => {
              dispatch(m.action);
              close();
            }}
          >
            确认操作
          </Btn>
        </div>
      </div>
    );
  if (m.type === "upload")
    return (
      <div className="space-y-4">
        <p className="text-sm leading-7 text-muted-foreground">
          文件保存到「
          {
            s.connections.find(
              (c) => c.id === (uploadDirectory?.connectionId || "drive"),
            )?.name
          }{" "}
          / {uploadDirectory?.path || s.options.uploadTarget}
          」，并进入待验收收件箱。以下入口演示客户端采集，不读取真实文件或相册。
        </p>
        <div className="grid grid-cols-2 gap-3">
          {[
            { title: "选择手机照片", icon: Image, source: "phone" as const },
            { title: "拍照上传", icon: Camera, source: "phone" as const },
            {
              title: "桌面文件 / 目录",
              icon: Laptop,
              source: "local" as const,
            },
            { title: "系统分享导入", icon: Share2, source: "phone" as const },
          ].map((x) => (
            <Btn
              layout="surface"
              key={x.title}
              className={cn(
                panel,
                "!h-auto !flex-col !whitespace-normal !p-5 text-xs",
              )}
              onClick={() => {
                dispatch({
                  type: "import",
                  source: x.source,
                  directoryId: uploadDirectory?.id,
                });
                close();
              }}
            >
              <x.icon size={25} className="mb-2 text-primary" />
              {x.title}
            </Btn>
          ))}
        </div>
        <p className="text-xs leading-6 text-muted-foreground">
          大文件显示发送、验证、保存三个阶段。权限拒绝、磁盘不足和系统暂停均保留原任务。
        </p>
      </div>
    );
  if (m.type === "agent")
    return (
      <div className="space-y-5">
        <div className="flex flex-wrap gap-2">
          {["限定范围", "复制接入说明", "验证连接"].map((v, i) => (
            <Pill key={v} tone={step === i + 1 ? "green" : "neutral"}>
              {i + 1}. {v}
            </Pill>
          ))}
        </div>
        {step === 1 && (
          <>
            <label className="block text-sm font-medium">允许访问的目录</label>
            <Choice
              label="Agent 授权目录"
              value={scope}
              onChange={setScope}
              options={["山野计划", "我的资料", "生活记录", "共同创作"].map(
                (v) => ({ label: v, value: v }),
              )}
              className="w-full"
            />
            <p className="rounded-lg bg-background p-3 text-xs leading-6">
              读取选定目录，提交产物到收件箱。
              <br />
              移动、删除、覆盖和外发仍需独立确认。
              <br />
              有效期 24 小时，不授予凭据管理能力。
            </p>
            <Btn className="w-full" onClick={() => setStep(2)}>
              下一步：接入说明
              <ArrowRight size={14} />
            </Btn>
          </>
        )}
        {step === 2 && (
          <>
            <p className="text-sm leading-7">
              在外部客户端选择远程
              MCP，使用由服务端签发的目录授权。原型不签发密钥，也不修改本机配置。
            </p>
            <div className="rounded-xl border border-border bg-background p-4 text-xs leading-7">
              接入方式：远程 MCP / Streamable HTTP
              <br />
              授权范围：{scope}
              <br />
              连接后：读取能力与参数说明，再检查目录访问
              <br />
              不需要在 MCP 客户端安装 DriveBridge CLI
            </div>
            <Btn
              variant="outline"
              onClick={async () => {
                const text = `DriveBridge prototype setup\nMode: remote MCP\nScope: ${scope}\nNo credential or endpoint is included. Discover tools and capabilities after connecting.`;
                try {
                  await navigator.clipboard.writeText(text);
                  dispatch({
                    type: "notice",
                    text: "已复制不含凭据的接入说明，不代表已经连接。",
                  });
                } catch {
                  dispatch({
                    type: "notice",
                    text: "当前浏览器不允许复制，请使用上方说明。",
                  });
                }
              }}
            >
              <Copy size={14} />
              复制说明
            </Btn>
            <Btn className="ml-2" onClick={() => setStep(3)}>
              下一步：验证连接
            </Btn>
          </>
        )}
        {step === 3 && (
          <>
            <div className="rounded-xl bg-accent p-4 text-sm leading-8 text-primary">
              <Check size={16} className="mr-2 inline" />
              模拟身份验证通过
              <br />
              <Check size={16} className="mr-2 inline" />
              限定目录可读，目录外访问拒绝
              <br />
              <Check size={16} className="mr-2 inline" />
              产物写入限定为收件箱
            </div>
            <p className="text-xs text-muted-foreground">
              这是演示结果，不代表真实客户端已安装或服务已连接。
            </p>
            <Btn
              className="w-full"
              onClick={() => {
                dispatch({ type: "agent", value: "active", scope });
                close();
              }}
            >
              完成演示接入
            </Btn>
          </>
        )}
      </div>
    );
  if (m.type === "property-batch") return <PropertyBatchEditor ids={m.ids} />;
  if (m.type === "bookmark-groups")
    return (
      <BookmarkGroups
        refs={
          m.refs || (m.ids || []).map((id) => entryRef(s, id)!).filter(Boolean)
        }
      />
    );
  if (m.type === "rule") return <RuleEditor />;
  if (m.type === "command") return <QuickFind />;
  if (m.type === "notifications")
    return (
      <div className="space-y-3">
        {s.tasks
          .filter(
            (t) =>
              t.space === s.space &&
              t.status !== "done" &&
              (["upload", "transfer"].includes(t.kind)
                ? s.options.notifyTransfers
                : s.options.notifyTasks),
          )
          .map((t) => (
            <Btn
              key={t.id}
              layout="surface"
              className={cn(
                panel,
                "!h-auto !w-full !justify-start !whitespace-normal !p-4 text-left",
              )}
              onClick={() => {
                go("tasks");
                close();
              }}
            >
              <ShieldCheck size={20} className="shrink-0 text-primary" />
              <span>
                <span className="block text-sm">{t.title}</span>
                <span className="mt-1 block text-xs text-muted-foreground">
                  返回原任务 {t.id} 查看与处理
                </span>
              </span>
            </Btn>
          ))}
        <p className="text-xs leading-6 text-muted-foreground">
          通知只携带安全任务引用。打开后重新检查身份、权限和计划版本。
        </p>
      </div>
    );
  if (m.type === "device")
    return (
      <div className="space-y-4">
        <div className="flex justify-center gap-6 rounded-xl bg-accent p-8 text-primary">
          <Laptop size={40} />
          <Link2 size={28} />
          <Smartphone size={40} />
        </div>
        <p className="text-sm leading-7">
          关联设备用于打开同一文件或任务。下一步再由你选择目录、相册或离线范围，Agent
          不会自动获得设备访问权限。
        </p>
        <p className="text-xs text-muted-foreground">
          演示配对，无真实设备发现、后台服务或系统通知。
        </p>
        <Btn
          className="w-full"
          onClick={() => {
            dispatch({ type: "link-device" });
            close();
          }}
        >
          模拟关联设备
        </Btn>
      </div>
    );
  if (m.type === "label") return <TagBatch ids={m.ids || s.selected} />;
  if (m.type === "capacity")
    return (
      <div className="space-y-4">
        <p className="text-sm">个人空间 · 示例快照</p>
        {[
          { name: "当前文件", value: "18.4 GB", width: "w-[62%]" },
          { name: "历史版本", value: "4.2 GB", width: "w-[24%]" },
          { name: "回收站", value: "1.1 GB", width: "w-[10%]" },
        ].map((x) => (
          <div key={x.name}>
            <div className="mb-2 flex justify-between text-xs">
              <span>{x.name}</span>
              <span>{x.value}</span>
            </div>
            <div className="h-2 rounded-full bg-muted">
              <div
                className={cn("h-full rounded-full bg-primary/60", x.width)}
              />
            </div>
          </div>
        ))}
        <p className="text-xs leading-6 text-muted-foreground">
          当前文件、版本、缓存与回收站分别统计。这里是演示容量，不代表真实配额或云商账单。
        </p>
        <Btn
          variant="outline"
          onClick={() => {
            go("files");
            close();
          }}
        >
          查看文件明细
        </Btn>
      </div>
    );
  if (m.type === "delivery") return <DeliveryEditor modal={m} />;
  return (
    <div className="space-y-4">
      <p className="text-sm leading-7">
        DriveBridge
        通过存储连接提供统一文件体验。连接、目录权限与数据处理均由服务端负责，客户端根据实际能力显示可用动作。
      </p>
      <div className="rounded-xl bg-background p-4 text-xs leading-7">
        已有实现依据：文件、版本、上传、整理计划、分享与授权。
        <br />
        待接入：索引运行投影、外部任务回执、跨设备续接。
        <br />
        探索设计：原生相册、系统分享、后台传输与离线缓存。
      </div>
      <p className="text-xs leading-6 text-muted-foreground">
        所有原型数据仅在内存中保存，刷新或重置会恢复初始示例。没有真实云存储或模型调用。
      </p>
    </div>
  );
}
