import { RecordRow } from "./CompactUI";
import { StorageIndexes, StorageUsage } from "./StorageIndexes";
import { rootNode } from "./storage-model";
import { useEffect, useLayoutEffect, useState } from "react";
import {
  ArrowLeft,
  ArrowRight,
  Check,
  Cloud,
  Copy,
  Database,
  Folder,
  Plus,
  Settings2,
} from "lucide-react";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "./Dialog";
import { Btn, Choice, Empty, Heading, Pill, useDemo } from "./ui";
import { providerNames } from "./v4-model";
import type { ConnectionInstance, ProviderKind } from "./v4-model";
import { Tasks } from "./TaskPages";

export function ProviderIcon({ kind }: { kind: ProviderKind }) {
  const Icon = kind === "s3" ? Database : kind === "gdrive" ? Folder : Cloud;
  return <Icon size={20} className="shrink-0 text-primary" />;
}
const statusLabel = {
  active: "已授权",
  expired: "授权过期",
  paused: "已停用",
  disconnected: "已断开",
};
export function StorageWorkspace() {
  const { state: s, dispatch, go } = useDemo();
  const tab = s.storage.tab;
  const setTab = (tab: typeof s.storage.tab) =>
    dispatch({ type: "storage-page", tab });
  const [editing, setEditing] = useState<ConnectionInstance | null>(null);
  const [source, setSource] = useState("drive");
  const [target, setTarget] = useState("s3");
  const available = s.connections.filter(
    (c) => c.id === "drive" || c.space === s.space,
  );
  const targetConnection = available.find((c) => c.id === target);
  if (editing)
    return (
      <ConnectionEditor
        key={editing.id}
        connection={editing}
        onClose={() => setEditing(null)}
      />
    );
  return (
    <>
      <Heading
        title="存储管理"
        note="连接多个账号，在不同存储之间继续工作。所有连接均为演示。"
      >
        <Btn
          onClick={() =>
            setEditing({
              id: `connection-${s.nextConnection}`,
              kind: s.space === "team" ? "s3" : "onedrive",
              name: "",
              root: "资料",
              space: s.space,
              status: "active",
              revision: 0,
              configured: false,
              authorized: false,
              read: "unknown",
              write: "unknown",
              verifiedAt: null,
            })
          }
        >
          <Plus size={16} />
          添加存储连接
        </Btn>
      </Heading>
      <div className="mb-6 flex gap-2 border-b border-border pb-3">
        {(
          [
            { id: "connections", label: "连接" },
            { id: "indexes", label: "索引" },
            { id: "usage", label: "用量" },
          ] as const
        ).map((x) => (
          <Btn
            key={x.id}
            variant={tab === x.id ? "secondary" : "ghost"}
            aria-pressed={tab === x.id}
            onClick={() => setTab(x.id)}
          >
            {x.label}
          </Btn>
        ))}
      </div>
      {tab === "connections" ? (
        <>
          <div className="mb-4">
            {available.map((c) => (
              <RecordRow
                key={c.id}
                identity={{ "data-connection-id": c.id }}
                name={c.name}
                icon={<ProviderIcon kind={c.kind} />}
                summary={`${providerNames[c.kind]} · ${c.root} · 只读${c.read === "passed" ? "已验证" : "未验证"} · 写入${c.write === "passed" ? "已验证" : "未验证"}`}
                status={
                  <Pill tone={c.status === "active" ? "green" : "amber"}>
                    {statusLabel[c.status]}
                  </Pill>
                }
                actions={
                  <Btn
                    variant="ghost"
                    onClick={() => {
                      dispatch({
                        type: "connection-select",
                        id: c.id,
                        scroll: 0,
                      });
                      go("files");
                    }}
                  >
                    浏览文件
                  </Btn>
                }
              >
                <article className="rounded-xl border border-border bg-card p-4">
                  <div className="mb-4 flex items-center justify-between gap-2">
                    <ProviderIcon kind={c.kind} />
                    <Pill tone={c.status === "active" ? "green" : "amber"}>
                      {statusLabel[c.status]}
                    </Pill>
                  </div>
                  <h2 className="break-words font-semibold">{c.name}</h2>
                  <p className="my-2 break-words text-xs text-muted-foreground">
                    {providerNames[c.kind]} · {c.root}
                  </p>
                  <dl className="space-y-2 text-xs">
                    <div className="flex justify-between gap-2">
                      <dt>配置</dt>
                      <dd>已保存</dd>
                    </div>
                    <div className="flex justify-between gap-2">
                      <dt>只读验证</dt>
                      <dd>
                        {c.read === "passed"
                          ? "通过（演示）"
                          : c.read === "failed"
                            ? "失败"
                            : "尚未验证"}
                      </dd>
                    </div>
                    <div className="flex justify-between gap-2">
                      <dt>写入验证</dt>
                      <dd>
                        {c.write === "passed" ? "通过（演示）" : "尚未验证"}
                      </dd>
                    </div>
                    <div className="flex justify-between gap-2">
                      <dt>容量</dt>
                      <dd>{c.kind === "drive" ? "24 / 100 GB" : "未知"}</dd>
                    </div>
                  </dl>
                  <div className="mt-4 flex flex-wrap gap-2">
                    <Btn
                      variant="outline"
                      onClick={() => {
                        dispatch({
                          type: "connection-select",
                          id: c.id,
                          scroll: 0,
                        });
                        go("files");
                      }}
                    >
                      浏览文件
                    </Btn>
                    <Btn
                      variant="ghost"
                      onClick={() =>
                        dispatch({
                          type: "storage-page",
                          tab: "indexes",
                          directoryId: rootNode(s, c.id)?.id,
                        })
                      }
                    >
                      管理索引
                    </Btn>
                    <Btn
                      variant="ghost"
                      onClick={() =>
                        dispatch({
                          type: "storage-transfer",
                          connectionId: c.id,
                        })
                      }
                    >
                      传输任务
                    </Btn>
                    {c.kind !== "drive" && (
                      <Btn
                        variant="ghost"
                        aria-label={`配置 ${c.name}`}
                        onClick={() => setEditing(c)}
                      >
                        <Settings2 size={16} />
                        配置
                      </Btn>
                    )}
                    <Btn
                      variant="ghost"
                      className="text-xs"
                      onClick={() =>
                        dispatch({ type: "connection-inspect", id: c.id })
                      }
                    >
                      能力说明
                    </Btn>
                  </div>
                </article>
              </RecordRow>
            ))}
          </div>
          <h2 className="mb-3 font-semibold">跨存储复制</h2>
          <div className="rounded-xl border border-border bg-card p-4">
            <p className="mb-4 text-sm leading-7 text-muted-foreground">
              从来源根目录复制可访问的文件，保留相对目录层级；先固定来源版本，再检查目标位置。原文件保留，覆盖同名文件需单独确认。
            </p>
            <div className="mb-4 flex flex-wrap items-center gap-3">
              <Choice
                label="来源存储"
                value={source}
                onChange={setSource}
                options={available.map((c) => ({ value: c.id, label: c.name }))}
              />
              <ArrowRight size={16} />
              <Choice
                label="目标存储"
                value={target}
                onChange={setTarget}
                options={available.map((c) => ({ value: c.id, label: c.name }))}
              />
            </div>
            <Btn
              disabled={
                source === target ||
                !targetConnection ||
                targetConnection.status !== "active"
              }
              onClick={() => {
                const ids = s.files
                  .filter(
                    (f) =>
                      f.space === s.space &&
                      !f.trashed &&
                      !f.inbox &&
                      (f.connectionId || "drive") === source,
                  )
                  .map((f) => f.id);
                if (!ids.length) {
                  dispatch({ type: "notice", text: "来源没有可复制文件。" });
                  return;
                }
                dispatch({
                  type: "plan",
                  ids,
                  kind: "transfer",
                  target: targetConnection!.root,
                  targetConnectionId: target,
                  sourceDirectoryId: rootNode(s, source)?.id,
                  targetDirectoryId: rootNode(s, target)?.id,
                });
              }}
            >
              <Copy size={15} />
              预览复制计划
            </Btn>
          </div>
          <p className="mt-4 text-xs leading-6 text-muted-foreground">
            团队受管存储仅展示平台或显式分配的 S3
            兼容能力。连接并不自动授权给团队或 Agent。
          </p>
        </>
      ) : tab === "indexes" ? (
        <StorageIndexes />
      ) : (
        <StorageUsage />
      )}
    </>
  );
}
function ConnectionEditor({
  connection: c,
  onClose,
}: {
  connection: ConnectionInstance;
  onClose: () => void;
}) {
  const { state: s, dispatch, go, guardNavigation } = useDemo();
  const [draft, setDraft] = useState({ ...c });
  const [leaving, setLeaving] = useState(false);
  const [confirmRoot, setConfirmRoot] = useState(false);
  const [scanConfirmed, setScanConfirmed] = useState(false);
  const [scanExclusions, setScanExclusions] = useState("私密,凭据");
  const dirty = JSON.stringify(draft) !== JSON.stringify(c);
  useEffect(() => {
    guardNavigation(dirty);
    return () => guardNavigation(false);
  }, [dirty]);
  const update = (patch: Partial<ConnectionInstance>) => {
    if (patch.root !== undefined || patch.kind !== undefined)
      setScanConfirmed(false);
    setDraft({ ...draft, ...patch });
  };
  const save = () => {
    dispatch({
      type: "connection-save",
      connection: { ...draft, configured: true },
      expectedRevision: c.revision,
      scanExclusions: scanConfirmed
        ? scanExclusions
            .split(/[,，]/)
            .map((x) => x.trim())
            .filter(Boolean)
        : undefined,
    });
    onClose();
  };
  const canSave =
    !!draft.name.trim() &&
    !!draft.root.trim() &&
    draft.authorized &&
    (draft.kind !== "s3" || (!!draft.endpoint && !!draft.bucket));
  const affected = s.tasks.filter(
    (t) =>
      t.status !== "done" &&
      (t.targetConnectionId === c.id ||
        t.items.some(
          (i) => s.files.find((f) => f.id === i.fileId)?.connectionId === c.id,
        )),
  ).length;
  return (
    <section className="mx-auto max-w-3xl">
      <Btn
        variant="ghost"
        className="mb-4"
        onClick={() => (dirty ? setLeaving(true) : onClose())}
      >
        <ArrowLeft size={16} />
        返回存储连接
      </Btn>
      <Heading
        title={c.configured ? "配置存储连接" : "添加存储连接"}
        note="页面内配置 · 无真实授权、凭据或网络调用。"
      />
      <div className="space-y-6">
        <section className="space-y-3">
          <h2 className="font-semibold">1. 选择 Provider 与实例名称</h2>
          {s.space === "team" && (
            <p className="text-xs leading-6 text-muted-foreground">
              团队受管存储仅演示平台分配的 S3 兼容连接。OneDrive / Google Drive
              请在个人空间配置导入与导出连接。
            </p>
          )}
          <div className="flex flex-wrap gap-2">
            {(["onedrive", "s3", "gdrive"] as const).map((kind) => (
              <Btn
                key={kind}
                variant={draft.kind === kind ? "secondary" : "outline"}
                disabled={c.configured || (s.space === "team" && kind !== "s3")}
                aria-pressed={draft.kind === kind}
                onClick={() =>
                  update({
                    kind,
                    authorized: false,
                    read: "unknown",
                    write: "unknown",
                  })
                }
              >
                <ProviderIcon kind={kind} />
                {providerNames[kind]}
              </Btn>
            ))}
          </div>
          <label className="block text-sm">
            连接名称
            <Input
              aria-label="连接名称"
              className="mt-2 h-11"
              value={draft.name}
              placeholder="例如：OneDrive · 工作"
              onChange={(e) => update({ name: e.target.value })}
            />
          </label>
        </section>
        <section className="space-y-3 border-t border-border pt-5">
          <h2 className="font-semibold">2. 授权与连接参数</h2>
          {draft.kind === "s3" ? (
            <>
              <div className="grid gap-3 sm:grid-cols-2">
                {(["endpoint", "bucket", "region", "prefix"] as const).map(
                  (k) => (
                    <label key={k} className="text-xs">
                      {k}
                      <Input
                        aria-label={k}
                        className="mt-2 h-11"
                        value={draft[k] || ""}
                        placeholder={
                          k === "endpoint"
                            ? "https://storage.example.invalid"
                            : k === "bucket"
                              ? "demo-archive"
                              : "可选"
                        }
                        onChange={(e) =>
                          update({ [k]: e.target.value, read: "unknown" })
                        }
                      />
                    </label>
                  ),
                )}
              </div>
              <p className="text-xs text-muted-foreground">
                凭据由连接 owner 持有；本原型仅使用演示引用，不输入真实密钥。
              </p>
            </>
          ) : (
            <p className="text-sm leading-7 text-muted-foreground">
              模拟选择 {providerNames[draft.kind]} 账号；不会打开真实 OAuth
              或请求账号权限。
            </p>
          )}
          <Btn
            variant="outline"
            onClick={() =>
              update({
                authorized: true,
                status: "active",
                read: "unknown",
                write: "unknown",
              })
            }
          >
            <Check size={15} />
            {draft.kind === "s3" ? "使用演示凭据" : "模拟授权账号"}
          </Btn>
          <Pill tone={draft.authorized ? "green" : "amber"}>
            {draft.authorized ? "当前已授权 · 演示" : "尚未授权"}
          </Pill>
        </section>
        <section className="space-y-3 border-t border-border pt-5">
          <h2 className="font-semibold">3. 选择根目录与验证</h2>
          <label className="block text-sm">
            授权根目录
            <Input
              aria-label="授权根目录"
              className="mt-2 h-11"
              value={draft.root}
              onChange={(e) =>
                update({
                  root: e.target.value,
                  read: "unknown",
                  write: "unknown",
                  verifiedAt: null,
                })
              }
            />
          </label>
          <div className="flex flex-wrap gap-2">
            <Btn
              variant="outline"
              disabled={!draft.authorized || !draft.root.trim()}
              onClick={() =>
                update({ read: "passed", verifiedAt: "演示检查 · 本次会话" })
              }
            >
              模拟只读验证
            </Btn>
            <Btn
              variant="ghost"
              onClick={() =>
                update({ read: "failed", verifiedAt: "演示检查 · 本次会话" })
              }
            >
              演示验证失败
            </Btn>
          </div>
          <p role="status" className="text-sm">
            只读：
            {draft.read === "passed"
              ? "验证通过"
              : draft.read === "failed"
                ? "验证失败，请检查目录"
                : "尚未验证"}{" "}
            · 写入：{draft.write === "passed" ? "通过（演示）" : "尚未验证"}
          </p>
          <p className="text-xs text-muted-foreground">
            最近检查：{draft.verifiedAt || "未知"}
            。读取通过不代表上传、覆盖或历史版本可用。
          </p>
        </section>
        <section className="space-y-3 border-t pt-4">
          <h2 className="font-semibold">4. 确认目录索引范围</h2>
          <p className="text-xs leading-6">
            {draft.name || "此连接"} / {draft.root} 及子目录 · 仅元数据 · 10,000
            条预算 · 手动更新。正文分析未开启。
          </p>
          <label className="block text-xs">
            排除路径或名称
            <Input
              aria-label="连接扫描排除项"
              value={scanExclusions}
              onChange={(e) => setScanExclusions(e.target.value)}
            />
          </label>
          <Btn
            variant={scanConfirmed ? "secondary" : "outline"}
            aria-pressed={scanConfirmed}
            onClick={() => setScanConfirmed(!scanConfirmed)}
          >
            {scanConfirmed ? "已确认扫描范围" : "确认以上范围并在保存后扫描"}
          </Btn>
          <p className="text-xs text-muted-foreground">
            也可以只保存连接，之后在索引中心确认；未验证读取时不能扫描。
          </p>
        </section>
        <div className="flex flex-wrap gap-2 border-t border-border pt-5">
          <Btn
            disabled={!canSave || (scanConfirmed && draft.read !== "passed")}
            onClick={() =>
              c.configured && draft.root !== c.root
                ? setConfirmRoot(true)
                : save()
            }
          >
            保存连接
          </Btn>
          <Btn
            variant="outline"
            onClick={() => (dirty ? setLeaving(true) : onClose())}
          >
            取消
          </Btn>
          <Btn
            variant="ghost"
            onClick={() => {
              if (dirty) setLeaving(true);
              else {
                dispatch({ type: "connection-inspect", id: c.id });
              }
            }}
          >
            能力与连接说明
          </Btn>
        </div>
        {c.configured && (
          <div className="flex flex-wrap gap-2 border-t border-border pt-5">
            {(["paused", "expired", "disconnected", "active"] as const).map(
              (status) => (
                <Btn
                  key={status}
                  variant="outline"
                  disabled={dirty}
                  onClick={() => {
                    dispatch({ type: "connection-state", id: c.id, status });
                    onClose();
                  }}
                >
                  {status === "active"
                    ? "模拟重新授权"
                    : status === "paused"
                      ? "停用连接"
                      : status === "expired"
                        ? "演示授权到期"
                        : "断开连接"}
                </Btn>
              ),
            )}
          </div>
        )}
      </div>
      <Dialog
        open={leaving || confirmRoot}
        onOpenChange={(v) => {
          if (!v) {
            setLeaving(false);
            setConfirmRoot(false);
          }
        }}
      >
        <DialogContent>
          <DialogTitle>
            {confirmRoot ? "更改授权根目录？" : "放弃未保存的配置？"}
          </DialogTitle>
          <DialogDescription>
            {confirmRoot
              ? `涉及 ${affected} 个未完成任务。旧范围的确认将失效，需要重新预览；已有文件和历史保留。`
              : "离开后未保存的表单修改将丢弃。"}
          </DialogDescription>
          <div className="flex gap-2">
            <Btn onClick={() => (confirmRoot ? save() : onClose())}>
              {confirmRoot ? "确认更改范围" : "放弃修改"}
            </Btn>
            <Btn
              variant="outline"
              onClick={() => {
                setLeaving(false);
                setConfirmRoot(false);
              }}
            >
              继续编辑
            </Btn>
          </div>
        </DialogContent>
      </Dialog>
    </section>
  );
}
