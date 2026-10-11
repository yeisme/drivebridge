import type { Action, FileRecord, State, Space } from "./model";

// Prototype projections only. Production authorization remains with DriveBridge.
export type ProviderKind = "drive" | "onedrive" | "s3" | "gdrive";
export interface ConnectionInstance {
  id: string;
  kind: ProviderKind;
  name: string;
  space: Space;
  root: string;
  status: "active" | "expired" | "paused" | "disconnected";
  revision: number;
  configured: boolean;
  authorized: boolean;
  read: "unknown" | "passed" | "failed";
  write: "unknown" | "passed";
  verifiedAt: string | null;
  endpoint?: string;
  bucket?: string;
  region?: string;
  prefix?: string;
}
export interface ContextReference {
  excerpt?: string;
  anchor?: string;
  key: string;
  kind: "file" | "folder" | "collection";
  name: string;
  connectionId: string;
  refs: { id: string; version: number; externalRevision?: string }[];
}
export interface Member {
  id: string;
  name: string;
  role: "admin" | "member";
  permission: "none" | "readonly" | "editor" | "custodian";
  active: boolean;
}
export interface Invitation {
  id: string;
  userId: string;
  status: "pending" | "accepted" | "declined" | "expired" | "revoked";
}
export interface SharedAccess {
  id: string;
  space: Space;
  fileId: string;
  folder?: string;
  recipient: string;
  permission: "readonly" | "editor";
  active: boolean;
  revision: number;
}
export interface Discussion {
  id: string;
  fileId: string;
  version: number;
  author: string;
  body: string;
  resolved: boolean;
  replies: { author: string; body: string }[];
}
export interface V4State {
  connections: ConnectionInstance[];
  activeConnection: string;
  nextConnection: number;
  capabilityConnection: string;
  connectionBrowsers: Record<
    string,
    { browser: State["browser"]; scroll: number }
  >;
  contextReferences: Record<string, ContextReference[]>;
  actor: string;
  members: Member[];
  invitations: Invitation[];
  sharedAccess: SharedAccess[];
  discussions: Discussion[];
  activity: {
    id: string;
    text: string;
    fileId?: string;
    version?: number;
    space: Space;
  }[];
  collaborationTab: "received" | "sent" | "team" | "activity";
  collaborationDefaults: { days: number; download: boolean; mentions: boolean };
}
export const providerNames: Record<ProviderKind, string> = {
  drive: "DriveBridge",
  onedrive: "OneDrive",
  s3: "S3 兼容",
  gdrive: "Google Drive",
};
export function v4Initial(files: FileRecord[]): V4State {
  files.forEach((f) => {
    f.connectionId = f.id === "f13" ? "s3" : "drive";
    f.rootId = "root";
    if (f.id === "f13") f.externalRevision = "s3-etag-demo-1";
  });
  for (const [id, connectionId, name, kind] of [
    ["od-personal-file", "od-personal", "旅行清单.pdf", "document"],
    ["od-work-file", "od-work", "工作项目清单.pdf", "document"],
    ["gd-file", "gdrive", "山野参考笔记.pdf", "document"],
  ] as const)
    files.push({
      id,
      connectionId,
      rootId: "root",
      externalRevision: `${connectionId}-revision-1`,
      name,
      kind,
      size: "1.2 MB",
      folder: "资料",
      version: 1,
      space: "personal",
      origin: "cloud",
      favorite: false,
    });
  const connections: ConnectionInstance[] = [
    ["drive", "drive", "DriveBridge 云盘", "全部文件"],
    ["od-personal", "onedrive", "OneDrive · 个人", "资料"],
    ["od-work", "onedrive", "OneDrive · 工作", "资料"],
    ["s3", "s3", "S3 · 归档", "S3 / 创作归档"],
    ["gdrive", "gdrive", "Google Drive · 参考", "资料"],
  ].map(([id, kind, name, root]) => ({
    id,
    kind: kind as ProviderKind,
    name,
    root,
    space: "personal",
    status: "active",
    revision: 1,
    configured: true,
    authorized: true,
    read: "passed",
    write: id === "drive" ? "passed" : "unknown",
    verifiedAt: "演示检查 · 本次会话",
  }));
  return {
    connections,
    activeConnection: "drive",
    nextConnection: 1,
    capabilityConnection: "drive",
    connectionBrowsers: {},
    contextReferences: {
      "conversation-1": ["f1", "f3"].map((id) => {
        const f = files.find((f) => f.id === id)!;
        return {
          key: id,
          kind: "file",
          name: f.name,
          connectionId: "drive",
          refs: [{ id, version: f.version }],
        };
      }),
    },
    actor: "yeisme",
    members: [
      {
        id: "yeisme",
        name: "Yeisme",
        role: "admin",
        permission: "custodian",
        active: true,
      },
      {
        id: "lin",
        name: "林舟",
        role: "member",
        permission: "readonly",
        active: true,
      },
      {
        id: "chen",
        name: "陈雨",
        role: "member",
        permission: "none",
        active: false,
      },
    ],
    invitations: [],
    sharedAccess: [
      {
        id: "shared-1",
        space: "personal",
        fileId: "f1",
        folder: "山野计划",
        recipient: "lin",
        permission: "readonly",
        active: true,
        revision: 1,
      },
    ],
    discussions: [],
    activity: [],
    collaborationTab: "received",
    collaborationDefaults: { days: 7, download: true, mentions: true },
  };
}
export function connectionFor(s: State, f: FileRecord) {
  return s.connections.find((c) => c.id === (f.connectionId || "drive"));
}
export function fileUnavailable(
  s: State,
  f?: FileRecord,
  version?: number,
): string {
  if (!f || f.trashed || f.space !== s.space || s.viewState === "forbidden")
    return "文件不存在或当前无权访问";
  const c = connectionFor(s, f);
  if (!c || c.status !== "active" || !c.authorized)
    return "来源连接已失效，请重新授权";
  if (c.kind !== "drive" && c.read !== "passed")
    return c.read === "failed"
      ? "读取检查失败，请检查连接根目录"
      : "来源尚未通过只读验证";
  if (
    c.kind !== "drive" &&
    c.root !== "全部文件" &&
    !(f.folder === c.root || f.folder.startsWith(c.root + " / "))
  )
    return "文件已离开当前授权根目录";
  if (
    version &&
    (version > f.version ||
      version < 1 ||
      (f.externalRevision && version !== f.version))
  )
    return "该固定版本不可用，未切换到最新版本";
  if (s.actor !== "yeisme") {
    const member = s.members.find((m) => m.id === s.actor);
    const shared = s.sharedAccess.some(
      (a) =>
        a.active &&
        a.recipient === s.actor &&
        a.space === f.space &&
        (a.folder
          ? f.folder === a.folder || f.folder.startsWith(a.folder + " / ")
          : a.fileId === f.id),
    );
    if (
      !shared &&
      !(f.space === "team" && member?.active && member.permission !== "none")
    )
      return "当前成员没有文件权限";
  }
  return "";
}
export function canEditFile(s: State, f: FileRecord) {
  if (fileUnavailable(s, f)) return false;
  if (s.actor === "yeisme") return true;
  const m = s.members.find((m) => m.id === s.actor);
  return (
    (f.space === "team" &&
      m?.active &&
      ["editor", "custodian"].includes(m.permission)) ||
    s.sharedAccess.some(
      (a) =>
        a.active &&
        a.recipient === s.actor &&
        a.permission === "editor" &&
        a.space === f.space &&
        (a.folder
          ? f.folder === a.folder || f.folder.startsWith(a.folder + " / ")
          : a.fileId === f.id),
    )
  );
}
export function contextRefs(s: State): ContextReference[] {
  const saved = s.contextReferences[s.activeConversation];
  return (
    saved ||
    s.context
      .map((id) => s.files.find((f) => f.id === id))
      .filter((f): f is FileRecord => !!f)
      .map((f) => ({
        key: f.id,
        kind: "file",
        name: f.name,
        connectionId: f.connectionId || "drive",
        refs: [
          {
            id: f.id,
            version: f.version,
            externalRevision: f.externalRevision,
          },
        ],
      }))
  );
}
export function contextUnavailable(s: State) {
  return contextRefs(s).some((c) =>
    c.refs.some((r) => {
      const f = s.files.find((f) => f.id === r.id);
      return (
        !!fileUnavailable(s, f, r.version) ||
        f?.version !== r.version ||
        (r.externalRevision && r.externalRevision !== f?.externalRevision)
      );
    }),
  );
}
export type V4Action =
  | { type: "connection-select"; id: string; scroll: number }
  | {
      type: "connection-save";
      scanExclusions?: string[];
      connection: ConnectionInstance;
      expectedRevision?: number;
    }
  | {
      type: "connection-state";
      id: string;
      status: ConnectionInstance["status"];
    }
  | { type: "connection-inspect"; id: string }
  | { type: "connection-check"; id: string; failed?: boolean }
  | { type: "context-reference"; reference?: ContextReference; remove?: string }
  | { type: "collaboration-tab"; tab: V4State["collaborationTab"] }
  | {
      type: "collaboration-defaults";
      patch: Partial<V4State["collaborationDefaults"]>;
    }
  | { type: "actor"; id: string }
  | { type: "invite"; userId: string }
  | { type: "invitation-state"; id: string; status: Invitation["status"] }
  | { type: "member-permission"; id: string; permission: Member["permission"] }
  | { type: "member-remove"; id: string }
  | {
      type: "shared-access";
      fileId: string;
      folder?: string;
      recipient: string;
      permission: SharedAccess["permission"];
    }
  | { type: "shared-revoke"; id: string }
  | {
      type: "discussion";
      fileId: string;
      version: number;
      body: string;
      threadId?: string;
    }
  | { type: "discussion-resolve"; id: string }
  | {
      type: "collaboration-conflict";
      fileId: string;
      expectedVersion: number;
      choice: "compare" | "copy";
    };
export function v4Reduce(s: State, a: V4Action): State | undefined {
  const note = (text: string, fileId?: string, version?: number) => {
    s.notice = text;
    s.activity.unshift({
      id: `activity-${s.activity.length + 1}`,
      text,
      fileId,
      version,
      space: s.space,
    });
    return s;
  };
  const admin = () => s.actor === "yeisme";
  if (
    !s.online &&
    [
      "connection-save",
      "shared-access",
      "shared-revoke",
      "invite",
      "invitation-state",
      "member-permission",
      "member-remove",
      "discussion",
      "discussion-resolve",
    ].includes(a.type)
  )
    return note("当前离线，尚未提交修改。");
  switch (a.type) {
    case "connection-select": {
      if (a.id !== "all" && !s.connections.some((c) => c.id === a.id)) return s;
      s.connectionBrowsers[`${s.space}:${s.activeConnection}`] = {
        browser: structuredClone(s.browser),
        scroll: a.scroll,
      };
      s.activeConnection = a.id;
      const old = s.connectionBrowsers[`${s.space}:${a.id}`];
      s.browser = old
        ? structuredClone(old.browser)
        : {
            ...s.browser,
            folder: "全部文件",
            back: [],
            forward: [],
            page: 1,
            filter: {
              query: "",
              kind: "all",
              origin: "all",
              tag: "all",
              updated: "all",
            },
          };
      s.selected = [];
      s.notice = "已切换存储，清除批量选择；Agent 上下文保持原范围。";
      return s;
    }
    case "connection-save": {
      const old = s.connections.find((c) => c.id === a.connection.id);
      if (!admin()) return note("当前身份不能配置此连接。");
      if (s.space === "team" && a.connection.kind !== "s3")
        return note("当前团队受管存储仅支持 S3 兼容连接分配。");
      if (old && old.revision !== a.expectedRevision)
        return note("连接配置已变化，请重新读取。");
      if (!a.connection.name.trim() || !a.connection.root.trim())
        return note("请填写连接名称和根目录。");
      if (old) {
        const changed = old.root !== a.connection.root;
        Object.assign(old, a.connection, { revision: old.revision + 1 });
        if (changed) {
          old.read = "unknown";
          old.write = "unknown";
          old.verifiedAt = null;
          invalidateConnection(s, old.id);
        }
      } else {
        s.connections.push({ ...a.connection, revision: 1 });
        s.nextConnection++;
      }
      return note("连接配置已保存 · 演示。读取验证与写入验证分别记录。");
    }
    case "connection-state": {
      const c = s.connections.find((c) => c.id === a.id);
      if (!c || c.kind === "drive" || !admin()) return s;
      c.status = a.status;
      c.authorized = a.status === "active";
      c.revision++;
      c.read = "unknown";
      c.write = "unknown";
      c.verifiedAt = null;
      if (a.status !== "active") invalidateConnection(s, c.id);
      return note(
        a.status === "active"
          ? "已模拟重新授权，请重新验证连接。"
          : "连接已停用，历史与文件保留。",
      );
    }
    case "connection-inspect":
      s.capabilityConnection = a.id;
      s.page = "capabilities";
      return s;
    case "connection-check": {
      const c = s.connections.find((c) => c.id === a.id);
      if (!c || c.status !== "active" || !c.authorized)
        return note("先恢复授权，再检查连接。");
      c.read = a.failed ? "failed" : "passed";
      c.verifiedAt = "演示检查 · 本次会话";
      return note(
        a.failed
          ? "读取检查失败，请检查根目录。"
          : "只读验证通过；写入尚未验证。",
      );
    }
    case "context-reference": {
      let refs = contextRefs(s);
      if (a.remove) refs = refs.filter((r) => r.key !== a.remove);
      if (a.reference) {
        if (
          a.reference.refs.some((r) =>
            fileUnavailable(
              s,
              s.files.find((f) => f.id === r.id),
              r.version,
            ),
          )
        )
          return note("选定引用当前不可访问。");
        refs = [...refs.filter((r) => r.key !== a.reference!.key), a.reference];
      }
      s.contextReferences[s.activeConversation] = refs;
      s.context = [...new Set(refs.flatMap((r) => r.refs.map((x) => x.id)))];
      s.contextRevision++;
      s.answersPending = false;
      s.analysisRefs = [];
      s.notice = "上下文已固定；目录后续新增文件不会自动加入。";
      return s;
    }
    case "collaboration-tab":
      s.collaborationTab = a.tab;
      return s;
    case "collaboration-defaults":
      s.collaborationDefaults = { ...s.collaborationDefaults, ...a.patch };
      s.settingsNotice = "共享默认值已保存 · 当前演示空间";
      return s;
    case "actor":
      if (s.members.some((m) => m.id === a.id)) {
        s.actor = a.id;
        s.selected = [];
        s.answersPending = false;
        s.contextRevision++;
      }
      return note("已切换模拟身份，现有授权会重新检查。");
    case "invite": {
      if (!admin()) return note("仅团队管理员可以邀请。");
      if (
        s.invitations.some(
          (i) => i.userId === a.userId && i.status === "pending",
        )
      )
        return note("该成员已有待接受邀请。");
      s.invitations.unshift({
        id: `invite-${s.invitations.length + 1}`,
        userId: a.userId,
        status: "pending",
      });
      return note("站内邀请已创建 · 7 天有效；接受不自动获得网盘权限。");
    }
    case "invitation-state": {
      const i = s.invitations.find((i) => i.id === a.id);
      if (!i || i.status !== "pending")
        return note("邀请已结束，不能重复处理。");
      if (["accepted", "declined"].includes(a.status) && s.actor !== i.userId)
        return note("请以被邀请人的模拟身份处理。");
      if (["revoked", "expired"].includes(a.status) && !admin())
        return note("仅管理员可撤销或演示到期。");
      i.status = a.status;
      if (a.status === "accepted") {
        const m = s.members.find((m) => m.id === i.userId)!;
        m.active = true;
        m.permission = "none";
      }
      return note(
        a.status === "accepted"
          ? "已加入团队，尚无文件权限。"
          : "邀请状态已更新。",
      );
    }
    case "member-permission":
    case "member-remove": {
      if (!admin() || a.id === s.actor)
        return note("不能自授权或移除最后的数据负责人。");
      const m = s.members.find((m) => m.id === a.id);
      if (!m?.active) return note("先接受团队邀请。");
      if (a.type === "member-remove") {
        m.active = false;
        m.permission = "none";
      } else m.permission = a.permission;
      s.contextRevision++;
      s.answersPending = false;
      return note("成员权限已更新，未完成请求重新检查权限；历史保留。");
    }
    case "shared-access": {
      if (!admin()) return note("需要文件负责人授权。");
      const f = s.files.find((f) => f.id === a.fileId);
      if (fileUnavailable(s, f)) return note("文件当前不可访问。");
      s.sharedAccess.unshift({
        id: `shared-${s.sharedAccess.length + 1}`,
        space: s.space,
        fileId: a.fileId,
        folder: a.folder,
        recipient: a.recipient,
        permission: a.permission,
        active: true,
        revision: 1,
      });
      return note(
        "指定成员共享已创建 · 演示。目录内容持续更新。",
        f!.id,
        f!.version,
      );
    }
    case "shared-revoke": {
      if (!admin()) return note("需要共享负责人撤销。");
      const x = s.sharedAccess.find((x) => x.id === a.id);
      if (x) {
        x.active = false;
        x.revision++;
        s.contextRevision++;
        s.answersPending = false;
      }
      return note("共享已撤销，后续访问停止；已下载副本无法收回。");
    }
    case "discussion": {
      const f = s.files.find((f) => f.id === a.fileId);
      if (fileUnavailable(s, f, a.version) || !a.body.trim())
        return note("当前不能发表此版本评论。");
      const t = s.discussions.find(
        (t) =>
          t.id === a.threadId &&
          t.fileId === a.fileId &&
          t.version === a.version,
      );
      if (t) t.replies.push({ author: s.actor, body: a.body.trim() });
      else
        s.discussions.push({
          id: `thread-${s.discussions.length + 1}`,
          fileId: a.fileId,
          version: a.version,
          author: s.actor,
          body: a.body.trim(),
          resolved: false,
          replies: [],
        });
      return note(
        "评论已保存 · 站内模拟。提及成员不会增加文件权限。",
        a.fileId,
        a.version,
      );
    }
    case "discussion-resolve": {
      const t = s.discussions.find((t) => t.id === a.id);
      if (
        t &&
        !fileUnavailable(
          s,
          s.files.find((f) => f.id === t.fileId),
          t.version,
        )
      )
        t.resolved = !t.resolved;
      return note("讨论状态已更新。");
    }
    case "collaboration-conflict": {
      const f = s.files.find((f) => f.id === a.fileId);
      if (!f || !canEditFile(s, f)) return note("文件当前不可写。");
      const member = s.members.find((m) => m.id === s.actor);
      if (
        s.actor !== "yeisme" &&
        !(
          f.space === "team" &&
          member?.active &&
          ["editor", "custodian"].includes(member.permission)
        )
      )
        return note("当前没有编辑权限。");
      if (a.expectedVersion === f.version) return note("版本没有冲突。");
      if (a.choice === "copy") {
        s.files.push({
          ...structuredClone(f),
          id: `conflict-${s.files.length}`,
          name: f.name.replace(/(\.[^.]+)$/, "（我的副本）$1"),
        });
        return note("已另存副本，未覆盖当前版本。", f.id, f.version);
      }
      return note(
        "发现版本冲突，保留当前版本，请先查看差异。",
        f.id,
        f.version,
      );
    }
  }
}
function invalidateConnection(s: State, id: string) {
  s.tasks
    .filter(
      (t) =>
        (t.status !== "done" &&
          t.items.some(
            (i) => s.files.find((f) => f.id === i.fileId)?.connectionId === id,
          )) ||
        (t.targetConnectionId === id && t.status !== "done"),
    )
    .forEach((t) => {
      t.status = "stale";
      t.revision++;
      t.confirmed = undefined;
    });
  s.contextRevision++;
  s.answersPending = false;
}
export function v4After(prev: State, next: State, a: Action): State {
  if (a.type === "space") {
    next.activeConnection = "drive";
  }
  if (a.type === "context") {
    delete next.contextReferences[next.activeConversation];
    next.contextReferences[next.activeConversation] = contextRefs(next);
  }
  if (a.type === "finish") {
    const task = next.tasks.find((t) => t.id === a.id);
    if (
      task?.targetConnectionId &&
      task.items.some((i) => i.status === "done")
    ) {
      const c = next.connections.find((c) => c.id === task.targetConnectionId);
      if (c && c.status === "active") c.write = "passed";
    }
  }
  if (a.type === "ask" && next.answersPending) {
    const message = next.messages.at(-1);
    if (message) message.refs = contextRefs(prev).flatMap((c) => c.refs);
  }
  return next;
}
