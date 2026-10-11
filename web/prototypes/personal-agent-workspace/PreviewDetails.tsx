import { PropertyPanel } from "./PropertyWorkspace";
import { FavoriteButton } from "./FavoritesWorkspace";
import { entryRef } from "./organization-model";
import { NoteRelations } from "./KnowledgeWorkspace";
import { FileButton } from "./HoverPreview";
import { useState } from "react";
import { Check, Copy, ArrowUpRight } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Btn, Choice, Empty, Pill, useDemo } from "./ui";
import { FileTags } from "./TagWorkspace";
import { canEditFile, fileUnavailable } from "./v4-model";
import { collectionFiles } from "./model";
import { VersionDiscussion } from "./CollaborationWorkspace";
import {
  agentContext,
  agentReference,
  previewFragment,
  previewUnavailable,
} from "./preview-model";
import type { PreviewRef } from "./preview-model";
export type DetailTab = "info" | "comments" | "history" | "activity" | "links";
export function PreviewDetails({
  reference: r,
  tab,
  onVersion,
  discussionDraft,
  onDiscussionDraft,
}: {
  reference: PreviewRef;
  tab: DetailTab;
  onVersion: (v: number) => void;
  discussionDraft?: import("./preview-model").DiscussionDraft;
  onDiscussionDraft?: (
    draft: import("./preview-model").DiscussionDraft,
  ) => void;
}) {
  const { state: s, dispatch, open, go } = useDemo();
  const f = s.files.find((f) => f.id === r.id);
  const [copied, setCopied] = useState("");
  const [manual, setManual] = useState("");
  const copy = async (text: string, message: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(message);
      setManual("");
    } catch {
      setCopied("自动复制不可用，请选择下方文本手动复制。");
      setManual(text);
    }
  };
  const unavailable = previewUnavailable(s, r);
  if (!f || unavailable)
    return (
      <Empty
        title="详情当前不可访问"
        text={unavailable || "未取得文件信息。"}
      />
    );
  const connection = s.connections.find(
    (c) => c.id === (r.connectionId || f.connectionId || "drive"),
  );
  const tasks = s.tasks.filter(
    (t) =>
      t.space === s.space &&
      (t.items.some((i) => i.fileId === f.id || i.outputId === f.id) ||
        f.id === `${t.id}-output`),
  );
  const fragment = previewFragment(s.space, r);
  const link = new URL(location.href);
  link.hash = fragment;
  return (
    <div className="space-y-5 p-4 text-sm" data-details-content={tab}>
      {copied && (
        <p role="status" className="rounded-lg bg-accent p-3 text-xs leading-6">
          {copied}
        </p>
      )}
      {manual && (
        <div
          aria-label="手动复制内容"
          tabIndex={0}
          className="max-h-60 overflow-auto whitespace-pre-wrap break-words select-text text-xs leading-6"
        >
          {manual}
        </div>
      )}
      {tab === "links" && <NoteRelations file={f} version={r.version} />}
      {tab === "info" && (
        <>
          <div className="flex items-center text-xs">
            <span className="mr-auto">个人收藏 · 当前连接</span>
            <FavoriteButton reference={entryRef(s, f.id)!} label={f.name} />
          </div>
          <PropertyPanel file={f} version={r.version} />
          <section className="space-y-3">
            <h3 className="font-semibold">
              链接与引用 <Pill>演示</Pill>
            </h3>
            <p className="text-xs leading-6 text-muted-foreground">
              定位到当前版本。复制不会创建分享或扩大 Agent 权限。
            </p>
            <div className="rounded-lg border border-border bg-background p-3">
              <p className="text-xs text-muted-foreground">内部定位</p>
              <code className="mt-2 block break-all text-xs">{fragment}</code>
              <Btn
                variant="ghost"
                className="mt-2 !px-0 text-xs"
                onClick={() =>
                  copy(link.href, "已复制定位链接；仍需原有文件权限。")
                }
              >
                <Copy size={14} />
                复制定位链接
              </Btn>
            </div>
            <div className="rounded-lg border border-border bg-background p-3">
              <p className="text-xs text-muted-foreground">
                {r.externalRevision
                  ? "外部条目与观测修订"
                  : "Agent 固定版本引用"}
              </p>
              <code className="mt-2 block whitespace-pre-wrap break-all text-xs">
                {agentReference(s, r)}
              </code>
              <Btn
                variant="ghost"
                className="mt-2 !px-0 text-xs"
                onClick={() =>
                  copy(
                    agentReference(s, r),
                    "已复制演示引用，不可调用真实服务。",
                  )
                }
              >
                <Copy size={14} />
                复制 Agent 引用
              </Btn>
            </div>
            <Btn
              variant="outline"
              className="w-full"
              onClick={() =>
                copy(
                  agentContext(s, r),
                  "已复制演示 Agent 上下文；未附带凭据。",
                )
              }
            >
              复制 Agent 上下文
            </Btn>
          </section>
          <section className="space-y-3 border-t border-border pt-4">
            <h3 className="font-semibold">文件概况</h3>
            <dl className="grid grid-cols-[72px_minmax(0,1fr)] gap-x-3 gap-y-3 text-xs leading-6">
              <dt className="text-muted-foreground">名称</dt>
              <dd className="break-words">{f.name}</dd>
              <dt className="text-muted-foreground">格式 / 大小</dt>
              <dd>
                {f.name.split(".").at(-1)?.toUpperCase()} · {f.size}
              </dd>
              <dt className="text-muted-foreground">更新</dt>
              <dd>{f.updatedAt || "尚未取得"}</dd>
              <dt className="text-muted-foreground">固定版本</dt>
              <dd className="break-all">
                {r.externalRevision || `v${r.version}`}
              </dd>
              <dt className="text-muted-foreground">文件标识</dt>
              <dd className="break-all font-mono">{f.id}</dd>
              <dt className="text-muted-foreground">摘要</dt>
              <dd>尚未取得</dd>
            </dl>
          </section>
          <section className="space-y-3 border-t border-border pt-4">
            <h3 className="font-semibold">位置与权限</h3>
            <p className="break-words text-xs leading-7">
              {s.space === "personal" ? "个人空间" : "团队空间"} /{" "}
              {connection?.name || "来源未知"}
              <br />
              Provider：
              {connection
                ? {
                    drive: "DriveBridge",
                    onedrive: "OneDrive",
                    s3: "S3 兼容",
                    gdrive: "Google Drive",
                  }[connection.kind]
                : "未知"}
              <br />
              根目录：{connection?.root || "尚未取得"}
              <br />
              {f.folder}
              <br />
              访问：
              {s.actor === "yeisme"
                ? "文件负责人"
                : canEditFile(s, f)
                  ? "可编辑"
                  : "可查看"}{" "}
              · 演示投影
            </p>
          </section>
          <section className="space-y-3 border-t border-border pt-4">
            <h3 className="font-semibold">标签与集合</h3>
            <FileTags file={f} full />
            <p className="text-xs text-muted-foreground">
              {s.collections
                .filter(
                  (c) =>
                    c.space === s.space &&
                    collectionFiles(s, c).some((x) => x.id === f.id),
                )
                .map((c) => c.name)
                .join("、") || "未加入集合"}
            </p>
            <Btn
              variant="ghost"
              onClick={() => open({ type: "label", ids: [f.id] })}
            >
              管理文件标签
            </Btn>
          </section>
          <section className="space-y-3 border-t border-border pt-4">
            <h3 className="font-semibold">来源与处理</h3>
            <p className="text-xs leading-6 text-muted-foreground">
              离线副本：{f.offlineVersion ? `v${f.offlineVersion}` : "未保存"}
              <br />
              来源：
              {
                {
                  agent: "Agent 产物",
                  phone: "手机导入",
                  local: "本机文件",
                  cloud: "云端存储",
                }[f.origin]
              }
            </p>
            {f.inputRefs?.map((ref) => (
              <FileButton
                reference={ref}
                key={ref.id}
                variant="link"
                className="!h-auto !justify-start !whitespace-normal !px-0 text-left text-xs"
                onOpen={() =>
                  open({
                    type: "preview",
                    id: ref.id,
                    version: ref.version,
                    single: true,
                  })
                }
              >
                输入：{s.files.find((x) => x.id === ref.id)?.name || ref.id} · v
                {ref.version}
                <ArrowUpRight size={13} />
              </FileButton>
            ))}
            {tasks.map((t) => (
              <Btn
                key={t.id}
                variant="outline"
                className="!h-auto !min-h-11 !w-full !justify-start !whitespace-normal text-xs"
                onClick={() => go("tasks")}
              >
                {t.id} · {t.title}
              </Btn>
            ))}
            <p className="text-xs text-muted-foreground">相关产物</p>
            {s.files
              .filter(
                (x) =>
                  !x.trashed &&
                  !fileUnavailable(s, x) &&
                  x.inputRefs?.some(
                    (ref) => ref.id === f.id && ref.version === r.version,
                  ),
              )
              .map((x) => (
                <FileButton
                  reference={{
                    id: x.id,
                    version: x.version,
                    connectionId: x.connectionId,
                    externalRevision: x.externalRevision,
                  }}
                  key={x.id}
                  variant="link"
                  className="!h-auto !min-h-11 !justify-start !whitespace-normal !px-0 text-left text-xs"
                  onOpen={() =>
                    open({
                      type: "preview",
                      id: x.id,
                      version: x.version,
                      single: true,
                    })
                  }
                >
                  {x.name} · v{x.version}
                </FileButton>
              ))}
            {!s.files.some(
              (x) =>
                !x.trashed &&
                !fileUnavailable(s, x) &&
                x.inputRefs?.some(
                  (ref) => ref.id === f.id && ref.version === r.version,
                ),
            ) && (
              <p className="text-xs text-muted-foreground">
                当前版本暂无可访问产物
              </p>
            )}
            {!tasks.length && (
              <p className="text-xs text-muted-foreground">无关联任务</p>
            )}
          </section>
          <section className="space-y-3 border-t border-border pt-4">
            <h3 className="font-semibold">分享</h3>
            {s.shares
              .filter((x) =>
                (x.members || [{ id: x.fileId }]).some((m) => m.id === f.id),
              )
              .map((x) => (
                <div
                  key={x.id}
                  className="space-y-2 border-b border-border pb-3 text-xs"
                >
                  <p>
                    {x.mode === "live"
                      ? "持续更新"
                      : `固定版本 v${x.members?.find((m) => m.id === f.id)?.version || x.version}`}{" "}
                    · {x.audience === "users" ? "指定成员" : "链接访问"} ·{" "}
                    {x.days || 7} 天 ·{" "}
                    {x.state === "active" ? "有效" : "已撤销"}
                  </p>
                  <code className="block break-all text-xs">
                    /demo-share/{x.id} · 演示地址
                  </code>
                  <Btn
                    variant="outline"
                    disabled={x.state !== "active"}
                    onClick={() =>
                      copy(
                        `https://drivebridge.example.invalid/demo-share/${x.id}`,
                        "已复制演示分享地址，不是真实可访问的外链。",
                      )
                    }
                  >
                    复制演示分享链接
                  </Btn>
                </div>
              ))}
            <Btn
              variant="outline"
              onClick={() =>
                open({ type: "delivery", id: f.id, version: r.version })
              }
            >
              创建分享 / 交付此版本
            </Btn>
          </section>
        </>
      )}
      {tab === "comments" && (
        <VersionDiscussion
          key={`${r.id}-${r.version}`}
          id={r.id}
          version={r.version}
          draft={discussionDraft}
          onDraft={onDiscussionDraft}
        />
      )}
      {tab === "history" &&
        (r.externalRevision ? (
          <p className="text-sm leading-7">
            外部历史读取能力尚未验证。仅显示本次观测修订，不生成受管版本。
          </p>
        ) : (
          <>
            <h3 className="font-semibold">版本记录</h3>
            <p className="text-xs leading-6 text-muted-foreground">
              在这里选版本，左侧内容随之更新；版本列表保持原位。
            </p>
            {Array.from({ length: f.version }, (_, i) => f.version - i).map(
              (v) => (
                <div
                  key={v}
                  className="flex flex-wrap items-center gap-2 border-b border-border py-3"
                >
                  <div className="flex-1">
                    <p className="font-medium">
                      版本 {v} {v === f.version ? "· 最新" : ""}
                    </p>
                    <small className="text-muted-foreground">
                      {v === 1 ? "初次导入" : "内容更新"} · 示例
                    </small>
                  </div>
                  <Btn
                    variant={r.version === v ? "secondary" : "outline"}
                    aria-pressed={r.version === v}
                    onClick={() => onVersion(v)}
                  >
                    {r.version === v ? "正在查看" : "查看"}
                  </Btn>
                  {v !== f.version && (
                    <Btn
                      variant="ghost"
                      onClick={() =>
                        open({
                          type: "danger",
                          title: `将 v${v} 恢复为新版本？`,
                          text: "不会删除现有版本。恢复后产生可追溯的新版本，当前阅读仍保持固定。",
                          action: {
                            type: "restore-version",
                            id: f.id,
                            version: v,
                          },
                        })
                      }
                    >
                      恢复
                    </Btn>
                  )}
                </div>
              ),
            )}
            <Btn
              variant="outline"
              onClick={() => dispatch({ type: "remote-version", id: f.id })}
            >
              模拟他人提交新版本
            </Btn>
            <Btn
              variant="outline"
              onClick={() => {
                dispatch({
                  type: "collaboration-conflict",
                  fileId: f.id,
                  expectedVersion: r.version,
                  choice: "compare",
                });
                if (r.version !== f.version)
                  open({ type: "compare", ids: [f.id], version: r.version });
              }}
            >
              检查并发修改
            </Btn>
            <Btn
              variant="outline"
              disabled={r.version === f.version}
              onClick={() =>
                dispatch({
                  type: "collaboration-conflict",
                  fileId: f.id,
                  expectedVersion: r.version,
                  choice: "copy",
                })
              }
            >
              保留当前并另存副本
            </Btn>
          </>
        ))}
      {tab === "activity" && (
        <>
          <h3 className="font-semibold">文件活动</h3>
          {s.activity
            .filter((a) => a.fileId === f.id && a.space === s.space)
            .map((a) => (
              <p
                key={a.id}
                className="border-b border-border py-3 text-xs leading-7"
              >
                {a.text} · v{a.version || "未知"}
              </p>
            ))}
          {!s.activity.some((a) => a.fileId === f.id) && (
            <p className="text-sm text-muted-foreground">暂无活动记录。</p>
          )}
          {tasks.map((t) => (
            <Btn key={t.id} variant="outline" onClick={() => go("tasks")}>
              查看任务 {t.id}
            </Btn>
          ))}
        </>
      )}
    </div>
  );
}
