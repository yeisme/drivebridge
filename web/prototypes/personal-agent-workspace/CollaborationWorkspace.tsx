import { RecordRow } from "./CompactUI";
import { useState } from "react";
import { ArrowRight, MessageSquare, Plus, Users } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "./Dialog";
import { Textarea } from "@/components/ui/textarea";
import { Btn, Choice, Empty, FileIcon, Heading, Pill, useDemo } from "./ui";
import type { Member, V4State } from "./v4-model";
import { fileUnavailable } from "./v4-model";
const permissionNames = {
  none: "未获文件权限",
  readonly: "可查看",
  editor: "可编辑",
  custodian: "数据负责人",
};
export function CollaborationWorkspace() {
  const { state: s, dispatch, open, go } = useDemo();
  const [creating, setCreating] = useState(false);
  const [fileId, setFileId] = useState(s.space === "team" ? "f10" : "f1");
  const [recipient, setRecipient] = useState("lin");
  const [scope, setScope] = useState("file");
  const [permission, setPermission] = useState<"readonly" | "editor">(
    "readonly",
  );
  const tab = s.collaborationTab;
  const setTab = (tab: V4State["collaborationTab"]) =>
    dispatch({ type: "collaboration-tab", tab });
  const f = s.files.find((f) => f.id === fileId && f.space === s.space);
  const shares = s.sharedAccess.filter(
    (a) =>
      a.space === s.space &&
      (tab === "received" ? a.recipient === s.actor : true),
  );
  const isOwner = s.actor === "yeisme";
  return (
    <>
      <Heading
        title="共享与协作"
        note="邀请成员、协作资料与外部交付，各有清楚的权限边界。"
      >
        <Btn disabled={!isOwner} onClick={() => setCreating(true)}>
          <Plus size={16} />
          共享文件或目录
        </Btn>
      </Heading>
      <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div className="flex max-w-full gap-1 overflow-x-auto">
          {(
            [
              { id: "received", name: "共享给我" },
              { id: "sent", name: "我发出的共享" },
              { id: "team", name: "团队空间" },
              { id: "activity", name: "邀请与活动" },
            ] as const
          ).map((x) => (
            <Btn
              key={x.id}
              variant={tab === x.id ? "secondary" : "ghost"}
              onClick={() => setTab(x.id)}
            >
              {x.name}
            </Btn>
          ))}
        </div>
        <Choice
          label="模拟协作身份"
          value={s.actor}
          onChange={(id) => dispatch({ type: "actor", id })}
          options={s.members.map((m) => ({
            value: m.id,
            label: `${m.name} · 模拟身份`,
          }))}
        />
      </div>
      {(tab === "received" || tab === "sent") && (
        <>
          <div className="mb-5 rounded-lg bg-accent p-4 text-sm leading-7">
            {tab === "received"
              ? "以当前模拟成员检查共享访问；撤权会立即阻止后续预览。"
              : "共享目录持续更新；固定版本交付请使用“分享与交付”。目录权限继承为探索设计。"}
            <Btn variant="link" onClick={() => go("shares")}>
              查看分享与交付
              <ArrowRight size={14} />
            </Btn>
          </div>
          {shares.map((a) => {
            const file = s.files.find((f) => f.id === a.fileId);
            return (
              <RecordRow
                key={a.id}
                name={
                  a.folder ? `${a.folder} · 共享目录` : file?.name || "共享文件"
                }
                summary={`Yeisme → ${s.members.find((m) => m.id === a.recipient)?.name} · ${permissionNames[a.permission]} · 持续更新`}
                status={<Pill>{a.active ? "共享中" : "已撤销"}</Pill>}
              >
                <article className="mb-3 flex flex-wrap items-center gap-4 rounded-xl border border-border bg-card p-4">
                  {file && <FileIcon file={file} />}
                  <div className="min-w-0 flex-1">
                    <h2 className="break-words text-sm font-semibold">
                      {a.folder ? `${a.folder} · 共享目录` : file?.name}
                    </h2>
                    <p className="mt-1 text-xs leading-6 text-muted-foreground">
                      Yeisme 直接授权 →{" "}
                      {s.members.find((m) => m.id === a.recipient)?.name} ·{" "}
                      {permissionNames[a.permission]}
                      <br />
                      {a.folder
                        ? "目录内最新内容 · 持续更新"
                        : "文件最新版本 · 持续更新"}{" "}
                      · r{a.revision}
                    </p>
                  </div>
                  <Pill tone={a.active ? "green" : "neutral"}>
                    {a.active ? "共享中" : "已撤销"}
                  </Pill>
                  <Btn
                    variant="outline"
                    disabled={!a.active || !!fileUnavailable(s, file)}
                    onClick={() => open({ type: "preview", id: a.fileId })}
                  >
                    预览共享文件
                  </Btn>
                  {isOwner && a.active && (
                    <Btn
                      variant="ghost"
                      onClick={() =>
                        open({
                          type: "danger",
                          title: "撤销成员共享？",
                          text: "后续访问停止，已下载的副本无法收回；历史保留。",
                          action: { type: "shared-revoke", id: a.id },
                        })
                      }
                    >
                      撤销成员共享
                    </Btn>
                  )}
                </article>
              </RecordRow>
            );
          })}
          {!shares.length && (
            <Empty
              title="暂无共享给当前身份的文件"
              text="可以切换到林舟查看演示共享，或由 Yeisme 发起新的指定成员共享。"
            />
          )}
        </>
      )}
      {tab === "team" && (
        <>
          <div className="mb-3 flex flex-wrap items-center gap-2 border-y border-border py-2">
            <Users size={26} className="text-primary" />
            <div className="flex-1">
              <h2 className="font-semibold">山野 · 团队空间</h2>
              <p className="mt-1 text-xs leading-6 text-muted-foreground">
                受管存储：平台 S3 · 演示分配。团队管理员不自动获得数据编辑权。
              </p>
            </div>
            <Btn
              variant="outline"
              onClick={() => {
                dispatch({ type: "space", space: "team" });
                go("files");
              }}
            >
              打开团队文件
            </Btn>
          </div>
          <h2 className="mb-3 font-semibold">成员与文件权限</h2>
          <div className="divide-y divide-border">
            {s.members
              .filter((m) => m.active)
              .map((m) => (
                <div
                  key={m.id}
                  data-compact-status
                  className="flex flex-wrap items-center gap-2 py-1"
                >
                  <div className="flex size-7 items-center justify-center rounded-full bg-accent text-primary">
                    {m.name[0]}
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium">{m.name}</p>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {m.role === "admin" ? "团队管理员" : "团队成员"} ·{" "}
                      {permissionNames[m.permission]}
                    </p>
                  </div>
                  <Choice
                    label={`${m.name} 文件权限`}
                    value={m.permission}
                    onChange={(v) => {
                      if (!isOwner || m.id === s.actor) {
                        dispatch({
                          type: "notice",
                          text: "不能自授权；需要其他有权负责人操作。",
                        });
                        return;
                      }
                      open({
                        type: "danger",
                        title: `变更 ${m.name} 的文件权限？`,
                        text: "新的访问按新权限检查。旧的临时授权不会自动恢复。",
                        action: {
                          type: "member-permission",
                          id: m.id,
                          permission: v as Member["permission"],
                        },
                      });
                    }}
                    options={Object.entries(permissionNames).map(
                      ([value, label]) => ({ value, label }),
                    )}
                  />
                  {isOwner && m.id !== s.actor && (
                    <Btn
                      variant="ghost"
                      onClick={() =>
                        open({
                          type: "danger",
                          title: `移除 ${m.name}？`,
                          text: "团队关系与空间访问停止；文件与历史保留。",
                          action: { type: "member-remove", id: m.id },
                        })
                      }
                    >
                      移除成员
                    </Btn>
                  )}
                </div>
              ))}
          </div>
          <div className="mt-5 flex flex-wrap gap-2">
            <Btn
              disabled={!isOwner}
              onClick={() => {
                dispatch({ type: "invite", userId: "chen" });
                setTab("activity");
              }}
            >
              邀请陈雨加入团队
            </Btn>
            <Btn
              variant="outline"
              onClick={() => {
                dispatch({ type: "space", space: "team" });
                go("connections");
              }}
            >
              查看存储分配与连接
            </Btn>
          </div>
          <p className="mt-4 text-xs leading-6 text-muted-foreground">
            接受邀请仅建立成员关系。个人网盘不会因加入团队自动变为团队共享。
          </p>
        </>
      )}
      {tab === "activity" && (
        <>
          <h2 className="mb-4 font-semibold">团队邀请 · 站内演示</h2>
          {s.invitations.map((i) => (
            <RecordRow
              key={i.id}
              name={`邀请 ${s.members.find((m) => m.id === i.userId)?.name}`}
              summary={`${i.id} · 加入团队不授予文件权限`}
              status={
                <Pill>
                  {
                    {
                      pending: "待接受",
                      accepted: "已接受",
                      declined: "已拒绝",
                      expired: "已过期",
                      revoked: "已撤销",
                    }[i.status]
                  }
                </Pill>
              }
            >
              <article className="border-b border-border/60 py-2">
                <div className="flex flex-wrap justify-between gap-2">
                  <strong className="text-sm">
                    邀请 {s.members.find((m) => m.id === i.userId)?.name}{" "}
                    加入山野团队
                  </strong>
                  <Pill>
                    {
                      {
                        pending: "待接受 · 7 天",
                        accepted: "已接受",
                        declined: "已拒绝",
                        expired: "已过期",
                        revoked: "已撤销",
                      }[i.status]
                    }
                  </Pill>
                </div>
                <p className="my-3 text-xs text-muted-foreground">
                  {i.id} · 加入团队不授予文件权限。仅受邀者可以接受。
                </p>
                {i.status === "pending" && (
                  <div className="flex flex-wrap gap-2">
                    {s.actor === i.userId ? (
                      <>
                        <Btn
                          onClick={() =>
                            dispatch({
                              type: "invitation-state",
                              id: i.id,
                              status: "accepted",
                            })
                          }
                        >
                          接受邀请
                        </Btn>
                        <Btn
                          variant="outline"
                          onClick={() =>
                            dispatch({
                              type: "invitation-state",
                              id: i.id,
                              status: "declined",
                            })
                          }
                        >
                          拒绝邀请
                        </Btn>
                      </>
                    ) : isOwner ? (
                      <>
                        <Btn
                          variant="outline"
                          onClick={() =>
                            dispatch({
                              type: "invitation-state",
                              id: i.id,
                              status: "revoked",
                            })
                          }
                        >
                          撤销邀请
                        </Btn>
                        <Btn
                          variant="ghost"
                          onClick={() =>
                            dispatch({
                              type: "invitation-state",
                              id: i.id,
                              status: "expired",
                            })
                          }
                        >
                          演示到期
                        </Btn>
                      </>
                    ) : null}
                  </div>
                )}
              </article>
            </RecordRow>
          ))}
          {!s.invitations.length && (
            <p className="mb-6 text-sm text-muted-foreground">
              暂无邀请，可从团队空间发起。
            </p>
          )}
          <h2 className="mb-3 mt-6 font-semibold">活动记录</h2>
          <div className="divide-y divide-border">
            {s.activity
              .filter((a) => a.space === s.space)
              .map((a) => (
                <div
                  key={a.id}
                  className="flex items-center justify-between gap-3 py-4"
                >
                  <p className="text-sm leading-7">{a.text}</p>
                  {a.fileId && (
                    <Btn
                      variant="outline"
                      onClick={() =>
                        open({
                          type: "preview",
                          id: a.fileId!,
                          version: a.version,
                        })
                      }
                    >
                      打开文件
                    </Btn>
                  )}
                </div>
              ))}
          </div>
        </>
      )}
      <Dialog open={creating} onOpenChange={setCreating}>
        <DialogContent>
          <DialogTitle>共享给指定成员</DialogTitle>
          <DialogDescription>
            目录共享是持续更新的范围；不会授予外部 Agent 权限。
          </DialogDescription>
          <Choice
            label="共享文件"
            value={fileId}
            onChange={setFileId}
            options={s.files
              .filter((f) => !fileUnavailable(s, f))
              .map((f) => ({ value: f.id, label: f.name }))}
          />
          <Choice
            label="共享范围"
            value={scope}
            onChange={setScope}
            options={[
              { value: "file", label: "这个文件" },
              { value: "folder", label: "文件所在目录（持续更新）" },
            ]}
          />
          <Choice
            label="接收成员"
            value={recipient}
            onChange={setRecipient}
            options={s.members
              .filter((m) => m.id !== s.actor)
              .map((m) => ({ value: m.id, label: m.name }))}
          />
          <Choice
            label="共享权限"
            value={permission}
            onChange={(v) => setPermission(v as typeof permission)}
            options={[
              { value: "readonly", label: "可查看" },
              { value: "editor", label: "可编辑" },
            ]}
          />
          <p className="text-xs leading-6 text-muted-foreground">
            授权来源：Yeisme 直接授权。当前范围：
            {scope === "folder" ? f?.folder : f?.name}。目录继承合同待接入。
          </p>
          <Btn
            disabled={!f}
            onClick={() => {
              dispatch({
                type: "shared-access",
                fileId,
                folder: scope === "folder" ? f!.folder : undefined,
                recipient,
                permission,
              });
              setCreating(false);
              setTab("sent");
            }}
          >
            确认共享范围与权限
          </Btn>
        </DialogContent>
      </Dialog>
    </>
  );
}
export function VersionDiscussion({
  id,
  version,
  draft,
  onDraft,
}: {
  id: string;
  version: number;
  draft?: import("./preview-model").DiscussionDraft;
  onDraft?: (draft: import("./preview-model").DiscussionDraft) => void;
}) {
  const { state: s, dispatch } = useDemo();
  const [local, setLocal] = useState({ body: "", reply: "", replyId: "" });
  const current = draft || local,
    { body, reply, replyId } = current;
  const patch = (value: Partial<typeof current>) =>
    onDraft
      ? onDraft({ ...current, ...value })
      : setLocal((v) => ({ ...v, ...value }));
  const setBody = (body: string) => patch({ body });
  const setReply = (reply: string) => patch({ reply });
  const setReplyId = (replyId: string) => patch({ replyId });
  const threads = s.discussions.filter(
    (t) => t.fileId === id && t.version === version,
  );
  return (
    <div className="space-y-4">
      <p className="text-xs leading-6 text-muted-foreground">
        评论固定在 v{version}。@ 提及仅站内提醒，不增加文件权限。
      </p>
      {s.comments
        .filter((c) => c.fileId === id && c.version === version)
        .map((c, i) => (
          <p key={i} className="rounded-lg border border-border p-4 text-sm">
            {c.body}
          </p>
        ))}
      {threads.map((t) => (
        <article key={t.id} className="rounded-lg border border-border p-4">
          <div className="flex items-center justify-between">
            <strong className="text-xs">
              {t.author === s.actor
                ? "你"
                : s.members.find((m) => m.id === t.author)?.name}{" "}
              · v{version}
            </strong>
            <Pill>{t.resolved ? "已解决" : "讨论中"}</Pill>
          </div>
          <p className="my-3 whitespace-pre-wrap break-words text-sm leading-7">
            {t.body}
          </p>
          {t.replies.map((r, i) => (
            <p
              key={i}
              className="my-2 border-l-2 border-border pl-3 text-sm leading-7"
            >
              {s.members.find((m) => m.id === r.author)?.name}：{r.body}
            </p>
          ))}
          <div className="flex gap-2">
            <Btn variant="ghost" onClick={() => setReplyId(t.id)}>
              回复讨论
            </Btn>
            <Btn
              variant="ghost"
              onClick={() => dispatch({ type: "discussion-resolve", id: t.id })}
            >
              {t.resolved ? "重新打开" : "标记已解决"}
            </Btn>
          </div>
        </article>
      ))}
      <Textarea
        aria-label="版本评论"
        placeholder="针对这个固定版本写下评论…"
        value={body}
        onChange={(e) => setBody(e.target.value)}
      />
      <div className="flex flex-wrap gap-2">
        {s.members
          .filter((m) => m.active)
          .map((m) => (
            <Btn
              key={m.id}
              variant="ghost"
              onClick={() => setBody(`${body}@${m.name} `)}
            >
              @ {m.name}
            </Btn>
          ))}
      </div>
      <Btn
        disabled={!body.trim() || !s.online}
        onClick={() => {
          dispatch({ type: "discussion", fileId: id, version, body });
          setBody("");
        }}
      >
        <MessageSquare size={14} />
        发送评论
      </Btn>
      <Dialog open={!!replyId} onOpenChange={(v) => !v && setReplyId("")}>
        <DialogContent>
          <DialogTitle>回复版本讨论</DialogTitle>
          <DialogDescription>回复保留在原文件与版本线程。</DialogDescription>
          <Textarea
            aria-label="回复内容"
            value={reply}
            onChange={(e) => setReply(e.target.value)}
          />
          <Btn
            disabled={!reply.trim()}
            onClick={() => {
              dispatch({
                type: "discussion",
                fileId: id,
                version,
                body: reply,
                threadId: replyId,
              });
              patch({ reply: "", replyId: "" });
            }}
          >
            发送回复
          </Btn>
        </DialogContent>
      </Dialog>
    </div>
  );
}
