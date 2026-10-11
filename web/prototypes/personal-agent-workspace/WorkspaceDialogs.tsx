import React, { useState } from "react";
import {
  ArrowRight,
  Check,
  Copy,
  FileText,
  Folder,
  Plus,
  SlidersHorizontal,
} from "lucide-react";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { cn } from "@/lib/utils";
import type { FileRecord, RuleConfig, State, Task } from "./model";
import { versionSnapshot } from "./model";
import {
  Btn,
  Choice,
  Empty,
  FileArt,
  FileIcon,
  panel,
  Pill,
  useDemo,
} from "./ui";
import type { Modal } from "./ui";

export function WorkspaceDialog({ modal: m }: { modal: Modal }) {
  if (m.type === "compare")
    return <Compare ids={m.ids} initialVersion={m.version} />;
  if (m.type === "edit-plan" || m.type === "conflict")
    return <TaskEditor taskId={m.taskId} conflict={m.type === "conflict"} />;
  if (m.type === "organize")
    return <Organize ids={m.ids || []} initialKind={m.kind} />;
  if (m.type === "collection")
    return (
      <CollectionEditor ids={m.ids || []} collectionId={m.id} smart={m.smart} />
    );
  return (
    <SimpleEditor kind={m.type === "new-folder" ? "folder" : "conversation"} />
  );
}
function SimpleEditor({ kind }: { kind: "folder" | "conversation" }) {
  const { state: s, dispatch, close } = useDemo();
  const [name, setName] = useState(
    kind === "conversation" ? s.conversationTitle : "",
  );
  const invalid = !name.trim() || (kind === "folder" && /[\\/]/.test(name));
  return (
    <form
      className="space-y-4"
      onSubmit={(e) => {
        e.preventDefault();
        if (invalid) return;
        dispatch(
          kind === "folder"
            ? { type: "create-folder", name }
            : { type: "conversation-title", title: name },
        );
        close();
      }}
    >
      <p className="text-sm text-muted-foreground">
        {kind === "folder"
          ? `创建位置：${s.browser.folder}`
          : "会话历史、输入草稿和版本引用保持不变。"}
      </p>
      <label className="block space-y-2">
        <span className="text-sm font-medium">
          {kind === "folder" ? "文件夹名称" : "会话名称"}
        </span>
        <Input
          autoFocus
          aria-label={kind === "folder" ? "文件夹名称" : "会话名称"}
          className="h-11"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
      </label>
      {kind === "folder" && /[\\/]/.test(name) && (
        <p role="alert" className="text-sm text-destructive">
          名称不能包含路径分隔符。
        </p>
      )}
      <div className="sticky bottom-0 z-10 flex justify-end gap-2 border-t border-border bg-card pt-4">
        <Btn variant="outline" type="button" onClick={close}>
          取消
        </Btn>
        <Btn disabled={invalid} type="submit">
          {kind === "folder" ? "创建文件夹" : "保存名称"}
        </Btn>
      </div>
    </form>
  );
}
function directoryOptions(s: State) {
  return [
    ...new Set([
      ...s.folders.filter((f) => f.space === s.space).map((f) => f.path),
      "山野计划 / 参考资料",
      "我的资料 / 自动归档",
    ]),
  ].map((v) => ({ value: v, label: v }));
}
function nameAfter(
  f: FileRecord,
  index: number,
  count: number,
  prefix: string,
) {
  return `${prefix.trim() || "素材"}${count > 1 ? `_${String(index + 1).padStart(2, "0")}` : ""}${f.name.match(/\.[^.]+$/)?.[0] || ""}`;
}
function Organize({
  ids,
  initialKind,
}: {
  ids: string[];
  initialKind?: "organize" | "rename" | "copy";
}) {
  const { state: s, dispatch, close } = useDemo();
  const [kind, setKind] = useState(initialKind || "organize");
  const [targetId, setTargetId] = useState(s.storage.activeId);
  const target = s.storage.nodes.find((n) => n.id === targetId)?.path || "";
  const [prefix, setPrefix] = useState("山野素材");
  const files = s.files.filter(
    (f) => f.space === s.space && !f.trashed && ids.includes(f.id),
  );
  return (
    <div className="space-y-5">
      <p className="text-sm leading-7">
        先检查选定的 {files.length} 份文件，再生成需要确认的操作计划。
      </p>
      <Choice
        label="整理动作"
        value={kind}
        onChange={(v) => setKind(v as typeof kind)}
        options={[
          { value: "organize", label: "移动到指定目录" },
          { value: "copy", label: "复制并保留原件" },
          { value: "rename", label: "批量重命名" },
        ]}
        className="w-full"
      />
      {kind === "rename" ? (
        <label className="block space-y-2">
          <span>名称前缀</span>
          <Input
            aria-label="名称前缀"
            className="h-11"
            value={prefix}
            onChange={(e) => setPrefix(e.target.value)}
          />
        </label>
      ) : (
        <Choice
          label="整理目标目录"
          value={targetId}
          onChange={setTargetId}
          options={s.storage.nodes
            .filter((n) => n.space === s.space)
            .map((n) => ({
              value: n.id,
              label: `${s.connections.find((c) => c.id === n.connectionId)?.name} / ${n.path}`,
            }))}
          className="w-full"
        />
      )}
      <div className="max-h-64 divide-y divide-border overflow-y-auto rounded-xl border border-border px-4">
        {files.map((f, i) => (
          <div key={f.id} className="py-3">
            <p className="break-words text-sm">{f.name}</p>
            <p className="mt-2 break-words text-xs text-primary">
              →{" "}
              {kind === "rename"
                ? nameAfter(f, i, files.length, prefix)
                : target}
            </p>
          </div>
        ))}
      </div>
      {!files.length && (
        <p role="alert" className="text-sm text-destructive">
          请先从素材库选择文件。
        </p>
      )}
      <div className="sticky bottom-0 z-10 flex justify-end gap-2 border-t border-border bg-card pt-4">
        <Btn variant="outline" onClick={close}>
          取消
        </Btn>
        <Btn
          disabled={
            !files.length ||
            (kind === "rename" && (!prefix.trim() || /[\\/]/.test(prefix)))
          }
          onClick={() => {
            dispatch({
              type: "plan",
              ids: files.map((f) => f.id),
              kind,
              target,
              prefix,
              targetDirectoryId: kind === "rename" ? undefined : targetId,
            });
            close();
          }}
        >
          生成修改计划
          <ArrowRight size={15} />
        </Btn>
      </div>
    </div>
  );
}
function CollectionEditor({
  ids,
  collectionId,
  smart = false,
}: {
  ids: string[];
  collectionId?: string;
  smart?: boolean;
}) {
  const { state: s, dispatch, close, go } = useDemo();
  const [name, setName] = useState(smart ? "我的智能素材集" : "新的素材集合");
  const [target, setTarget] = useState(collectionId || "new");
  const options = [
    { value: "new", label: "创建新集合" },
    ...s.collections
      .filter((c) => c.space === s.space && !c.query)
      .map((c) => ({ value: c.id, label: c.name })),
  ];
  return (
    <div className="space-y-4">
      <p className="text-sm leading-7">
        {smart
          ? "保存当前名称、类型、来源、日期和标签条件。以后打开时重新计算当前可见结果。"
          : `将选中的 ${ids.length} 份素材作为引用加入集合，原目录与文件内容不变。`}
      </p>
      {!smart && (
        <Choice
          label="目标集合"
          value={target}
          onChange={setTarget}
          options={options}
          className="w-full"
        />
      )}
      {(smart || target === "new") && (
        <label className="block space-y-2">
          <span className="text-sm font-medium">集合名称</span>
          <Input
            aria-label="集合名称"
            className="h-11"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </label>
      )}
      {smart && (
        <div className="rounded-xl bg-background p-4 text-sm leading-7">
          名称：{s.browser.filter.query || "不限"}
          <br />
          类型：{s.browser.filter.kind}
          <br />
          来源：{s.browser.filter.origin}
          <br />
          日期：{s.browser.filter.updated}
        </div>
      )}
      <div className="sticky bottom-0 z-10 flex justify-end gap-2 border-t border-border bg-card pt-4">
        <Btn
          variant="outline"
          onClick={() => {
            go("files");
            close();
          }}
        >
          先去选择素材
        </Btn>
        <Btn
          disabled={
            ((target === "new" || smart) && !name.trim()) ||
            (target !== "new" && !ids.length)
          }
          onClick={() => {
            if (target === "new" || smart)
              dispatch({
                type: "collection-create",
                name,
                ids,
                query: smart ? { ...s.browser.filter } : undefined,
              });
            else dispatch({ type: "collection-add", id: target, ids });
            close();
          }}
        >
          确认保存集合
        </Btn>
      </div>
    </div>
  );
}
function TaskEditor({
  taskId,
  conflict,
}: {
  taskId: string;
  conflict: boolean;
}) {
  const { state: s, dispatch, open, close } = useDemo();
  const t = s.tasks.find((t) => t.id === taskId && t.space === s.space);
  const [revision] = useState(t?.revision || 0);
  const [target, setTarget] = useState(t?.target || "");
  const [prefix, setPrefix] = useState("山野素材");
  const [ids, setIds] = useState(
    t?.items
      .filter(
        (i) =>
          i.status !== "done" && i.status !== "skipped" && i.included !== false,
      )
      .map((i) => i.fileId) || [],
  );
  const [choice, setChoice] = useState("keep-both");
  if (!t)
    return <Empty title="原任务当前不可见" text="请重新检查空间与权限。" />;
  const editable = t.items.filter(
    (i) => i.status !== "done" && i.status !== "skipped",
  );
  if (conflict)
    return (
      <div className="space-y-5">
        <p className="text-sm leading-7">
          {t.id} · {editable.length} 项冲突待处理。已完成项不会重做。
        </p>
        <Choice
          label="冲突处理方式"
          className="w-full"
          value={choice}
          onChange={setChoice}
          options={[
            { value: "keep-both", label: "保留两份 · 副本使用新名称" },
            { value: "skip", label: "跳过冲突项 · 保留原文件" },
            { value: "overwrite", label: "覆盖目标 · 需要单独确认" },
          ]}
        />
        <div className="rounded-xl bg-background p-4 text-sm leading-7">
          {editable.map((i) => (
            <p key={i.fileId}>
              {s.files.find((f) => f.id === i.fileId)?.name}
              <br />
              <span className="text-xs text-muted-foreground">{i.reason}</span>
            </p>
          ))}
        </div>
        <Btn
          variant="outline"
          onClick={() =>
            open({
              type: "compare",
              ids: editable.slice(0, 1).map((i) => i.fileId),
            })
          }
        >
          先检查版本
        </Btn>
        <Btn
          className="ml-2"
          disabled={t.revision !== revision}
          onClick={() => {
            const action = {
              type: "resolve-conflicts" as const,
              id: t.id,
              revision,
              choice: choice as "keep-both" | "skip" | "overwrite",
            };
            if (choice === "overwrite")
              open({
                type: "danger",
                title: "确认覆盖示例目标？",
                text: "仅覆盖本次冲突清单中的目标副本；原文件与已完成项保留。这是单独的覆盖确认。",
                action,
              });
            else {
              dispatch(action);
              close();
            }
          }}
        >
          应用选择并继续
        </Btn>
      </div>
    );
  return (
    <div className="space-y-4">
      <p className="text-sm leading-7">
        {t.id} · r{revision}
        。修改目录、命名规则或包含项会生成新修订，原确认失效。
      </p>
      {t.kind === "rename" ? (
        <Input
          aria-label="调整名称前缀"
          className="h-11"
          value={prefix}
          onChange={(e) => setPrefix(e.target.value)}
        />
      ) : (
        <Choice
          label="调整目标目录"
          value={target}
          onChange={setTarget}
          className="w-full"
          options={[
            ...directoryOptions(s),
            ...(!directoryOptions(s).some((o) => o.value === target)
              ? [{ value: target, label: target }]
              : []),
          ]}
        />
      )}
      <div className="max-h-72 divide-y divide-border overflow-y-auto rounded-xl border border-border px-4">
        {t.items.map((i, n) => (
          <div key={i.fileId} className="flex min-h-16 items-center gap-3 py-3">
            <Checkbox
              aria-label={`包含 ${s.files.find((f) => f.id === i.fileId)?.name}`}
              checked={i.status === "done" || ids.includes(i.fileId)}
              disabled={i.status === "done" || i.status === "skipped"}
              onCheckedChange={() =>
                setIds(
                  ids.includes(i.fileId)
                    ? ids.filter((id) => id !== i.fileId)
                    : [...ids, i.fileId],
                )
              }
            />
            <span className="min-w-0 flex-1">
              <span className="block break-words text-sm">
                {s.files.find((f) => f.id === i.fileId)?.name}
              </span>
              <span className="mt-1 block text-xs text-muted-foreground">
                {i.status === "done"
                  ? "已完成 · 历史结果保留"
                  : t.kind === "rename"
                    ? `→ ${prefix}${t.items.length > 1 ? `_${String(n + 1).padStart(2, "0")}` : ""}${i.beforeName?.match(/\.[^.]+$/)?.[0] || ""}`
                    : `→ ${target}`}
              </span>
            </span>
          </div>
        ))}
      </div>
      <div className="sticky bottom-0 z-10 flex justify-end gap-2 border-t border-border bg-card pt-4">
        <Btn variant="outline" onClick={close}>
          取消
        </Btn>
        <Btn
          disabled={!ids.length || t.revision !== revision}
          onClick={() => {
            dispatch({
              type: "edit-plan",
              id: t.id,
              revision,
              ids,
              target,
              prefix,
            });
            close();
          }}
        >
          保存新修订
        </Btn>
      </div>
    </div>
  );
}

export function Snapshot({
  file,
  version,
}: {
  file: FileRecord;
  version: number;
}) {
  const snap = versionSnapshot(file, version);
  return (
    <div className="overflow-hidden rounded-xl border border-border bg-card">
      {file.kind === "image" || file.kind === "video" ? (
        <FileArt file={file} version={version} large />
      ) : (
        <div className="p-5">
          <FileIcon file={file} />
          <h3 className="mt-4 break-words text-base font-semibold">
            {file.name}
          </h3>
          <p className="mt-3 text-sm leading-8">{snap.content}</p>
          {file.kind === "audio" && (
            <p className="mt-4 text-xs text-muted-foreground">
              音频比较为内容说明与波形示意，不代表实际播放。
            </p>
          )}
        </div>
      )}
      <div className="border-t border-border p-4">
        <p className="text-xs text-muted-foreground">
          内容快照 · v{version} · {file.size}
        </p>
        {(file.kind === "image" || file.kind === "video") && (
          <p className="mt-2 text-sm leading-7">{snap.content}</p>
        )}
      </div>
    </div>
  );
}
function Compare({
  ids,
  initialVersion,
}: {
  ids: string[];
  initialVersion?: number;
}) {
  const { state: s, compact } = useDemo();
  const files = ids
    .slice(0, 2)
    .map((id) =>
      s.files.find((f) => f.id === id && f.space === s.space && !f.trashed),
    );
  const left = files[0],
    right = files[1] || left;
  const [lv, setLv] = useState(
    initialVersion && initialVersion < (left?.version || 1)
      ? initialVersion
      : Math.max(1, (left?.version || 1) - 1),
  );
  const [rv, setRv] = useState(right?.version || 1);
  const [side, setSide] = useState("left");
  if (!left || !right || files.some((f) => !f) || s.viewState === "forbidden")
    return (
      <Empty
        title="比较清单中有不可访问的文件"
        text="重新选择当前有权访问的素材。"
      />
    );
  if (!s.online && (left.offlineVersion !== lv || right.offlineVersion !== rv))
    return (
      <Empty
        title="部分版本没有离线保存"
        text="联网后再比较，或回到已下载的固定版本。"
      />
    );
  const options = (f: FileRecord) =>
    Array.from({ length: f.version }, (_, i) => ({
      value: String(i + 1),
      label: `v${i + 1}${i + 1 === f.version ? " · 最新" : " · 历史"}`,
    }));
  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <Pill tone="green">
          {ids.length > 1 ? "两份素材比较" : "同一文件的两个版本"}
        </Pill>
        <span className="text-xs text-muted-foreground">比较不会修改原件</span>
      </div>
      {compact && (
        <div className="grid grid-cols-2 gap-2">
          <Btn
            variant={side === "left" ? "secondary" : "outline"}
            onClick={() => setSide("left")}
          >
            左侧 / 来源
          </Btn>
          <Btn
            variant={side === "right" ? "secondary" : "outline"}
            onClick={() => setSide("right")}
          >
            右侧 / 结果
          </Btn>
        </div>
      )}
      <div
        className={cn("grid gap-4", compact ? "grid-cols-1" : "grid-cols-2")}
      >
        {[
          {
            id: "left",
            file: left,
            version: lv,
            change: setLv,
            label: "左侧版本",
          },
          {
            id: "right",
            file: right,
            version: rv,
            change: setRv,
            label: "右侧版本",
          },
        ]
          .filter((x) => !compact || x.id === side)
          .map((x) => (
            <section key={x.id} className="min-w-0">
              <div className="mb-3 flex items-center justify-between gap-2">
                <span
                  className="min-w-0 truncate text-sm font-medium"
                  title={x.file.name}
                >
                  {x.file.name}
                </span>
                <Choice
                  label={x.label}
                  value={String(x.version)}
                  onChange={(v) => x.change(Number(v))}
                  options={options(x.file)}
                />
              </div>
              <Snapshot file={x.file} version={x.version} />
            </section>
          ))}
      </div>
      <div className="rounded-xl bg-accent p-4 text-sm leading-7">
        <strong>对比提示</strong>
        <p>
          {versionSnapshot(left, lv).content ===
          versionSnapshot(right, rv).content
            ? "两个快照的示例内容一致。"
            : "两个快照的叙述或画面版本不同，请检查预览后决定采用哪一版。"}
        </p>
        <p className="text-xs text-muted-foreground">
          文件引用与版本固定；此处不自动判定创作质量或覆盖原件。
        </p>
      </div>
    </div>
  );
}

export function RuleEditor() {
  const { state: s, dispatch, close } = useDemo();
  const [mode, setMode] = useState(s.ruleMode);
  const [config, setConfig] = useState<RuleConfig>({ ...s.ruleConfig });
  const valid =
    !!config.target &&
    Number.isInteger(config.maxFiles) &&
    config.maxFiles >= 1 &&
    config.maxFiles <= 500 &&
    Number.isInteger(config.validHours) &&
    config.validHours >= 1 &&
    config.validHours <= 168;
  return (
    <div className="space-y-4">
      <Choice
        label="规则类型"
        value={mode}
        onChange={(v) => {
          setMode(v as typeof mode);
          setConfig({
            ...config,
            source:
              v === "backup"
                ? "手机已选相册"
                : v === "sync"
                  ? "桌面已选目录"
                  : "待整理",
          });
        }}
        options={[
          { value: "backup", label: "单向备份 · 保留副本" },
          { value: "sync", label: "选择性同步 · 需处理冲突" },
          { value: "organize", label: "新收件 · 自动归档" },
        ]}
        className="w-full"
      />
      <label className="block space-y-2">
        <span>来源范围</span>
        <Choice
          label="规则来源"
          value={config.source}
          onChange={(source) => setConfig({ ...config, source })}
          options={[
            "手机已选相册",
            "桌面已选目录",
            ...s.folders.filter((f) => f.space === s.space).map((f) => f.path),
          ]
            .filter((v, i, a) => a.indexOf(v) === i)
            .map((v) => ({ value: v, label: v }))}
          className="w-full"
        />
      </label>
      <label className="block space-y-2">
        <span>目标目录</span>
        <Choice
          label="规则目标"
          value={config.target}
          onChange={(target) => setConfig({ ...config, target })}
          options={directoryOptions(s)}
          className="w-full"
        />
      </label>
      <div className="grid grid-cols-2 gap-3">
        <label className="space-y-2">
          <span className="text-sm">单次文件上限</span>
          <Input
            type="number"
            min="1"
            max="500"
            aria-label="单次文件上限"
            className="h-11"
            value={config.maxFiles}
            onChange={(e) =>
              setConfig({ ...config, maxFiles: Number(e.target.value) })
            }
          />
        </label>
        <label className="space-y-2">
          <span className="text-sm">授权时长（小时）</span>
          <Input
            type="number"
            min="1"
            max="168"
            aria-label="授权时长"
            className="h-11"
            value={config.validHours}
            onChange={(e) =>
              setConfig({ ...config, validHours: Number(e.target.value) })
            }
          />
        </label>
      </div>
      <label className="flex min-h-11 items-center gap-3">
        <Checkbox
          checked={config.wifiOnly}
          onCheckedChange={(wifiOnly) =>
            setConfig({ ...config, wifiOnly: !!wifiOnly })
          }
        />
        仅在 Wi-Fi 下运行
      </label>
      <div className="rounded-xl bg-background p-4 text-sm leading-7">
        删除传播：关闭
        <br />
        覆盖已有内容：关闭
        <br />
        {mode === "organize"
          ? "整理会移动已授权范围内的文件。"
          : "备份和同步只创建新副本，原文件保留。"}
      </div>
      {!valid && (
        <p role="alert" className="text-sm text-destructive">
          文件上限需为 1–500，时长需为 1–168 小时。
        </p>
      )}
      <div className="sticky bottom-0 z-10 flex justify-end gap-2 border-t border-border bg-card pt-4">
        <Btn variant="outline" onClick={close}>
          取消
        </Btn>
        <Btn
          disabled={!valid}
          onClick={() => {
            dispatch({ type: "rule", mode, config });
            close();
          }}
        >
          确认范围并启用
        </Btn>
      </div>
    </div>
  );
}
export function DeliveryEditor({
  modal: m,
}: {
  modal: Extract<
    Modal,
    {
      type:
        | "upload"
        | "consent"
        | "agent"
        | "rule"
        | "command"
        | "notifications"
        | "device"
        | "help"
        | "label"
        | "capacity"
        | "delivery"
        | "duplicates"
        | "organize"
        | "collection"
        | "new-folder"
        | "conversation";
    }
  >;
}) {
  const { state: s, dispatch, close } = useDemo();
  const [members, setMembers] = useState(() => {
    const ids = m.ids?.length ? m.ids : m.id ? [m.id] : s.selected;
    return ids.map((id) => ({
      id,
      version: m.version || s.files.find((f) => f.id === id)?.version || 1,
    }));
  });
  const [days, setDays] = useState(String(s.collaborationDefaults.days));
  const [download, setDownload] = useState(s.collaborationDefaults.download);
  const [mode, setMode] = useState<"fixed" | "live">("fixed");
  const [audience, setAudience] = useState<"link" | "users">("link");
  const [recipient, setRecipient] = useState("lin");
  const [passcode, setPasscode] = useState(false);
  const missing = members.some(
    (r) =>
      !s.files.find(
        (f) =>
          f.id === r.id &&
          f.space === s.space &&
          !f.trashed &&
          f.version >= r.version,
      ),
  );
  return (
    <div className="space-y-4">
      <p className="text-sm leading-7">
        {mode === "fixed"
          ? "检查本次交付清单。每一项都固定在所选版本，不会跟随云端更新。"
          : "动态分享：接收人将看到所选文件的最新版本，不增加新文件或编辑权限。"}
      </p>
      <div className="max-h-64 divide-y divide-border overflow-y-auto rounded-xl border border-border px-4">
        {members.map((r) => {
          const f = s.files.find(
            (f) => f.id === r.id && f.space === s.space && !f.trashed,
          );
          return (
            <div key={r.id} className="flex items-center gap-3 py-3">
              {f && <FileIcon file={f} />}
              <div className="min-w-0 flex-1">
                <p className="break-words text-sm font-medium">
                  {f?.name || "文件当前不可访问"}
                </p>
                <p className="mt-1 text-xs text-muted-foreground">
                  固定版本 v{r.version} · {f ? "已模拟校验" : "清单缺项"}
                </p>
              </div>
              <Btn
                variant="ghost"
                onClick={() => setMembers(members.filter((x) => x.id !== r.id))}
              >
                移除
              </Btn>
            </div>
          );
        })}
      </div>
      <Choice
        label="分享版本策略"
        value={mode}
        onChange={(v) => setMode(v as typeof mode)}
        options={[
          { value: "fixed", label: "固定版本（默认）" },
          { value: "live", label: "持续更新最新版本" },
        ]}
      />
      <Choice
        label="访问对象"
        value={audience}
        onChange={(v) => setAudience(v as typeof audience)}
        options={[
          { value: "link", label: "持有链接的人" },
          { value: "users", label: "指定成员" },
        ]}
      />
      {audience === "users" && (
        <Choice
          label="分享接收人"
          value={recipient}
          onChange={setRecipient}
          options={s.members
            .filter((m) => m.id !== s.actor)
            .map((m) => ({ value: m.id, label: m.name }))}
        />
      )}
      <label className="flex min-h-11 items-center gap-3">
        <Checkbox
          checked={passcode}
          onCheckedChange={(v) => setPasscode(!!v)}
        />
        使用演示访问密码（不生成真实密码）
      </label>
      <Choice
        label="分享有效期"
        value={days}
        onChange={setDays}
        options={[
          { value: "1", label: "1 天" },
          { value: "7", label: "7 天" },
          { value: "30", label: "30 天" },
        ]}
        className="w-full"
      />
      <label className="flex min-h-11 items-center gap-3">
        <Checkbox
          checked={download}
          onCheckedChange={(v) => setDownload(!!v)}
        />
        允许接收人下载
      </label>
      <p className="rounded-lg bg-[var(--warning-soft)] p-3 text-sm leading-7 text-[var(--warning)]">
        这是独立的外发确认。原型不会创建真实公开链接。
      </p>
      {missing && (
        <p role="alert" className="text-sm text-destructive">
          清单有缺项或权限变化，不能发布。
        </p>
      )}
      <div className="sticky bottom-0 z-10 flex justify-end gap-2 border-t border-border bg-card pt-4">
        <Btn variant="outline" onClick={close}>
          取消
        </Btn>
        <Btn
          disabled={!members.length || missing}
          onClick={() => {
            dispatch({
              type: "share",
              id: members[0].id,
              version: members[0].version,
              members,
              days: Number(days),
              download,
              mode,
              audience,
              recipients: audience === "users" ? [recipient] : [],
              passcodeEnabled: passcode,
            });
            close();
          }}
        >
          {mode === "fixed" ? "确认交付固定版本" : "确认分享持续更新内容"}
        </Btn>
      </div>
    </div>
  );
}
