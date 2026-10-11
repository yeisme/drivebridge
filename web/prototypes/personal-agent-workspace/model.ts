import {
  emptyBrowse,
  browseReduce,
  browseAfter,
  browseLocation,
} from "./browse-model";
import type { BrowseState, BrowseAction } from "./browse-model";
import {
  emptyOrganization,
  migrateOrganization,
  organizationReduce,
  syncOrganization,
} from "./organization-model";
import type {
  OrganizationState,
  OrganizationAction,
} from "./organization-model";
import {
  directoryAvailable,
  emptyStorage,
  initStorage,
  ensureDirectories,
  storageReduce,
  rootNode,
} from "./storage-model";
import type { StorageState, StorageAction } from "./storage-model";
import { knowledgeInitial, knowledgeReduce } from "./knowledge-model";
import type { KnowledgeState, KnowledgeAction } from "./knowledge-model";
import {
  v4Initial,
  v4Reduce,
  v4After,
  fileUnavailable,
  contextUnavailable,
  canEditFile,
} from "./v4-model";
import type { V4State, V4Action } from "./v4-model";
import {
  cloudInitial,
  cloudReduce,
  matchesTags,
  emptySearch,
} from "./cloud-model";
import type { CloudState, CloudAction, TagAssignment } from "./cloud-model";
// In-memory prototype only. These types are interaction contracts, not production wire types.
export type FileKind = "document" | "image" | "video" | "audio" | "sheet";
export type Space = "personal" | "team";
export type Page =
  | "favorites"
  | "knowledge"
  | "resources"
  | "knowledge-graph"
  | "home"
  | "files"
  | "collections"
  | "assistant"
  | "tasks"
  | "inbox"
  | "shares"
  | "agents"
  | "connections"
  | "devices"
  | "automation"
  | "trash"
  | "settings"
  | "tags"
  | "search"
  | "capabilities"
  | "collaboration";
export type ViewState =
  | "ready"
  | "loading"
  | "empty"
  | "forbidden"
  | "offline"
  | "disconnected"
  | "partial"
  | "stale"
  | "unknown"
  | "unsupported"
  | "error";
export type TaskStatus =
  | "approval"
  | "running"
  | "partial"
  | "paused"
  | "done"
  | "blocked"
  | "stale"
  | "unknown"
  | "cancelled";
export type ItemStatus = "pending" | "done" | "failed" | "stale" | "skipped";
export interface VersionSnapshot {
  version: number;
  content: string;
  accent: number;
}
export interface FileFilter {
  query: string;
  kind: string;
  origin: string;
  tag: string;
  tagIds?: string[];
  tagMode?: "all" | "any" | "exclude";
  updated: string;
}
export interface Collection {
  id: string;
  name: string;
  space: Space;
  ids: string[];
  query?: FileFilter;
  groups: Record<string, string>;
}
export interface Conversation {
  id: string;
  title: string;
  space: Space;
  context: string[];
  messages: Message[];
  draft: string;
  intent: "summarize" | "search" | "organize";
}
export interface RuleConfig {
  source: string;
  target: string;
  wifiOnly: boolean;
  maxFiles: number;
  validHours: number;
}
export interface FileRecord {
  directoryId?: string;
  connectionId?: string;
  rootId?: string;
  externalRevision?: string;
  id: string;
  name: string;
  kind: FileKind;
  size: string;
  folder: string;
  version: number;
  space: Space;
  origin: "cloud" | "local" | "agent" | "phone";
  favorite: boolean;
  offlineVersion?: number;
  label?: string;
  tags?: TagAssignment[];
  trashed?: boolean;
  inbox?: boolean;
  picked?: boolean;
  updatedAt?: string;
  metadata?: string;
  snapshots?: VersionSnapshot[];
  inputRefs?: { id: string; version: number }[];
}
export interface TaskItem {
  outputId?: string;
  outputVersion?: number;
  fileId: string;
  version: number;
  before: string;
  after: string;
  status: ItemStatus;
  reason?: string;
  beforeName?: string;
  afterName?: string;
  included?: boolean;
  decision?: "keep-both" | "skip" | "overwrite";
}
export interface Task {
  targetConnectionId?: string;
  id: string;
  title: string;
  kind:
    | "organize"
    | "upload"
    | "transfer"
    | "agent"
    | "automation"
    | "restore"
    | "trash"
    | "rename"
    | "copy";
  space: Space;
  status: TaskStatus;
  revision: number;
  confirmed?: number;
  items: TaskItem[];
  source: string;
  target: string;
  note: string;
  runs: number;
  injectPartial: boolean;
  agent?: boolean;
  automationMode?: "backup" | "sync" | "organize";
  revisions?: { revision: number; summary: string }[];
}
export interface Message {
  id: number;
  role: "user" | "assistant";
  text: string;
  refs?: { id: string; version: number }[];
  taskId?: string;
  kind?: "answer" | "candidates";
}
export interface Share {
  mode?: "fixed" | "live";
  audience?: "link" | "users";
  recipients?: string[];
  passcodeEnabled?: boolean;
  id: string;
  fileId: string;
  version: number;
  state: "active" | "revoked";
  received: boolean;
  members?: { id: string; version: number }[];
  days?: number;
  download?: boolean;
}
export interface State extends CloudState, V4State {
  browse: BrowseState;
  organization: OrganizationState;
  storage: StorageState;
  knowledge: KnowledgeState;
  page: Page;
  space: Space;
  contextRevision: number;
  files: FileRecord[];
  selected: string[];
  context: string[];
  tasks: Task[];
  messages: Message[];
  nextTask: number;
  shares: Share[];
  agent: "unconnected" | "active" | "paused" | "revoked" | "expired";
  agentScope: string;
  online: boolean;
  rule: "off" | "active" | "paused";
  ruleMode: "backup" | "sync" | "organize";
  viewState: ViewState;
  notice: string;
  analysisRefs: string[];
  answersPending: boolean;
  audit: string[];
  comments: { fileId: string; version: number; body: string }[];
  deviceLinked: boolean;
  browser: {
    folder: string;
    back: string[];
    forward: string[];
    filter: FileFilter;
    sort: string;
    layout: "list" | "grid" | "gallery";
    collectionId: string;
    focusedId: string;
    page: number;
  };
  folders: { path: string; space: Space; connectionId?: string }[];
  collections: Collection[];
  activeConversation: string;
  conversationTitle: string;
  otherConversations: Conversation[];
  nextConversation: number;
  draft: string;
  intent: Conversation["intent"];
  ruleConfig: RuleConfig;
}

export const emptyFilter = (): FileFilter => ({
  query: "",
  kind: "all",
  origin: "all",
  tag: "all",
  updated: "all",
});
export function versionSnapshot(
  file: FileRecord,
  version: number,
): VersionSnapshot {
  return (
    file.snapshots?.find((v) => v.version === version) || {
      version,
      accent: version,
      content:
        version === 1
          ? "最初的想法：去寻找一处安静的地方。以远景交代环境，保留初次探索的留白。"
          : version === 2
            ? "第二版加入人物行走与环境声。镜头贴近脚步和植被，让停留成为叙述的中心。"
            : "定稿强调自然光、真实的行走感与克制的叙述。开场抵达，中段停留，沿着溪流归途。",
    }
  );
}
export function matchesFilter(file: FileRecord, filter: FileFilter): boolean {
  return (
    file.name.toLowerCase().includes(filter.query.toLowerCase()) &&
    (filter.kind === "all" ||
      (filter.kind === "favorite" && file.favorite) ||
      (filter.kind === "offline" && !!file.offlineVersion) ||
      (filter.kind === "picked" && file.picked) ||
      file.kind === filter.kind) &&
    (filter.origin === "all" || file.origin === filter.origin) &&
    (filter.tag === "all" ||
      file.label === filter.tag ||
      !!file.tags?.some(
        (t) => t.tagId === filter.tag && t.source !== "suggested",
      )) &&
    matchesTags(file, filter.tagIds, filter.tagMode) &&
    (filter.updated === "all" ||
      (file.updatedAt || "2026-09-27") >=
        (filter.updated === "today" ? "2026-09-27" : "2026-09-20"))
  );
}
export function collectionFiles(
  state: State,
  collection: Collection,
): FileRecord[] {
  return state.files.filter(
    (f) =>
      f.space === state.space &&
      f.space === collection.space &&
      !f.trashed &&
      !fileUnavailable(state, f) &&
      (collection.query
        ? matchesFilter(f, collection.query)
        : collection.ids.includes(f.id)),
  );
}
export function currentConversation(state: State): Conversation {
  return {
    id: state.activeConversation,
    title: state.conversationTitle,
    space: state.space,
    context: [...state.context],
    messages: structuredClone(state.messages),
    draft: state.draft,
    intent: state.intent,
  };
}
function activateConversation(s: State, next: Conversation) {
  const previous = currentConversation(s);
  s.otherConversations = [
    ...s.otherConversations.filter(
      (c) => c.id !== next.id && c.id !== previous.id,
    ),
    previous,
  ];
  s.activeConversation = next.id;
  s.conversationTitle = next.title;
  s.context = next.context;
  s.messages = next.messages;
  s.draft = next.draft;
  s.intent = next.intent;
  s.analysisRefs = [];
  s.answersPending = false;
  s.contextRevision++;
}

export const initialFiles: FileRecord[] = [
  {
    id: "f1",
    name: "山野计划 · 创作简报.pdf",
    kind: "document",
    size: "2.4 MB",
    folder: "山野计划",
    version: 3,
    space: "personal",
    origin: "cloud",
    favorite: true,
    label: "项目资料",
  },
  {
    id: "f2",
    name: "秋日山行 · 视觉参考.jpg",
    kind: "image",
    size: "8.6 MB",
    folder: "山野计划",
    version: 2,
    space: "personal",
    origin: "cloud",
    favorite: true,
  },
  {
    id: "f3",
    name: "山间来信 · 分镜脚本.docx",
    kind: "document",
    size: "840 KB",
    folder: "山野计划",
    version: 2,
    space: "personal",
    origin: "cloud",
    favorite: false,
    offlineVersion: 2,
  },
  {
    id: "f4",
    name: "林间片段 · 初剪.mp4",
    kind: "video",
    size: "426 MB",
    folder: "山野计划",
    version: 1,
    space: "personal",
    origin: "local",
    favorite: false,
  },
  {
    id: "f5",
    name: "九月阅读摘录.md",
    kind: "document",
    size: "24 KB",
    folder: "我的资料",
    version: 1,
    space: "personal",
    origin: "cloud",
    favorite: false,
  },
  {
    id: "f6",
    name: "旅行收支.xlsx",
    kind: "sheet",
    size: "128 KB",
    folder: "生活记录",
    version: 2,
    space: "personal",
    origin: "cloud",
    favorite: false,
  },
  {
    id: "f7",
    name: "山风与溪流.wav",
    kind: "audio",
    size: "34 MB",
    folder: "山野计划",
    version: 1,
    space: "personal",
    origin: "local",
    favorite: false,
  },
  {
    id: "f8",
    name: "灵感整理 · 摘要.md",
    kind: "document",
    size: "36 KB",
    folder: "待整理",
    version: 1,
    space: "personal",
    origin: "agent",
    favorite: false,
    inbox: true,
  },
  {
    id: "f9",
    name: "IMG_0927.jpg",
    kind: "image",
    size: "6.2 MB",
    folder: "待整理",
    version: 1,
    space: "personal",
    origin: "phone",
    favorite: false,
    inbox: true,
  },
  {
    id: "f10",
    name: "团队拍摄安排.pdf",
    kind: "document",
    size: "1.2 MB",
    folder: "共同创作",
    version: 1,
    space: "team",
    origin: "cloud",
    favorite: false,
  },
  {
    id: "f11",
    name: "旧版提案.pdf",
    kind: "document",
    size: "720 KB",
    folder: "山野计划",
    version: 1,
    space: "personal",
    origin: "cloud",
    favorite: false,
    trashed: true,
  },
  {
    id: "f12",
    name: "秋日山行 · 视觉参考（备份）.jpg",
    kind: "image",
    size: "8.6 MB",
    folder: "临时收集",
    version: 2,
    space: "personal",
    origin: "cloud",
    favorite: false,
  },
  {
    id: "f13",
    name: "山间来信 · 分镜脚本.docx",
    kind: "document",
    size: "790 KB",
    folder: "S3 / 创作归档",
    version: 1,
    space: "personal",
    origin: "cloud",
    favorite: false,
  },
];
export const taskStatusText: Record<TaskStatus, string> = {
  approval: "待确认",
  running: "进行中",
  partial: "部分完成",
  paused: "已暂停",
  done: "已完成",
  blocked: "等待授权",
  stale: "计划已过期",
  unknown: "结果待确认",
  cancelled: "已取消",
};
export const scenarioNames = [
  "自由探索",
  "01 手机采集与归档",
  "02 跨端确认与恢复",
  "03 文档检索与引用",
  "04 外部 Agent 产物",
  "05 跨存储复制",
  "06 离线与版本冲突",
  "07 版本讨论与交付",
  "08 备份与自动整理",
];

function itemsFor(
  files: FileRecord[],
  ids: string[],
  after: string,
): TaskItem[] {
  return files
    .filter((f) => ids.includes(f.id) && !f.trashed)
    .map((f) => ({
      fileId: f.id,
      version: f.version,
      before: f.folder,
      after,
      status: "pending",
    }));
}
export function initialState(): State {
  const files = initialFiles.map((f, index) => ({
    ...f,
    inputRefs: f.origin === "agent" ? [{ id: "f1", version: 3 }] : undefined,
    updatedAt: index < 7 ? "2026-09-27" : "2026-09-22",
    metadata:
      f.kind === "image"
        ? "3840 × 2160"
        : f.kind === "video"
          ? "00:42 · 4K"
          : f.kind === "audio"
            ? "00:42 · WAV"
            : f.kind === "sheet"
              ? "3 个工作表"
              : "8 页",
    snapshots: Array.from({ length: f.version }, (_, i) =>
      versionSnapshot(f, i + 1),
    ),
  }));
  const v4 = v4Initial(files);
  const state: State = {
    browse: emptyBrowse(),
    organization: emptyOrganization(),
    storage: emptyStorage(),
    knowledge: knowledgeInitial(),
    ...v4,
    ...cloudInitial(files),
    page: "files",
    space: "personal",
    contextRevision: 1,
    files,
    selected: [],
    context: ["f1", "f3"],
    tasks: [
      {
        id: "DB-024",
        title: "整理山野计划的参考资料",
        kind: "organize",
        space: "personal",
        status: "approval",
        revision: 1,
        items: itemsFor(files, ["f1", "f2", "f3"], "山野计划 / 参考资料"),
        source: "我的网盘",
        target: "山野计划 / 参考资料",
        note: "移动 3 个文件，保留文件名和版本。",
        runs: 0,
        injectPartial: true,
      },
    ],
    nextTask: 25,
    messages: [],
    shares: [],
    agent: "unconnected",
    agentScope: "山野计划",
    online: true,
    rule: "off",
    ruleMode: "backup",
    viewState: "ready",
    notice: "",
    analysisRefs: [],
    answersPending: false,
    audit: [],
    comments: [],
    deviceLinked: false,
    browser: {
      folder: "全部文件",
      back: [],
      forward: [],
      filter: emptyFilter(),
      sort: "recent",
      layout: "list",
      collectionId: "",
      focusedId: "f2",
      page: 1,
    },
    folders: [...new Set(files.map((f) => `${f.space}|${f.folder}`))].map(
      (v) => {
        const [space, path] = v.split("|");
        return { path, space: space as Space };
      },
    ),
    collections: [
      {
        id: "c1",
        name: "山野计划",
        space: "personal",
        ids: ["f1", "f2", "f3", "f4", "f7"],
        groups: {
          f1: "参考",
          f2: "参考",
          f3: "工作稿",
          f4: "工作稿",
          f7: "参考",
        },
      },
      {
        id: "c2",
        name: "日常与灵感",
        space: "personal",
        ids: ["f5", "f6", "f9"],
        groups: {},
      },
    ],
    activeConversation: "conversation-1",
    conversationTitle: "山野计划 · 素材梳理",
    otherConversations: [],
    nextConversation: 2,
    draft: "",
    intent: "summarize",
    ruleConfig: {
      source: "手机已选相册",
      target: "我的资料 / 自动归档",
      wifiOnly: true,
      maxFiles: 20,
      validHours: 24,
    },
  };
  return migrateOrganization(initStorage(state));
}
type BaseAction =
  | { type: "navigate"; page: Page }
  | { type: "space"; space: Space }
  | { type: "select"; ids: string[] }
  | { type: "context"; ids: string[] }
  | { type: "notice"; text: string }
  | { type: "view"; state: ViewState }
  | { type: "favorite"; id: string }
  | { type: "offline"; id: string; version?: number }
  | { type: "network"; online: boolean }
  | { type: "remote-version"; id: string }
  | { type: "import"; source: "phone" | "local"; directoryId?: string }
  | {
      type: "plan";
      sourceDirectoryId?: string;
      targetDirectoryId?: string;
      ids: string[];
      kind?: Task["kind"];
      target?: string;
      prefix?: string;
      targetConnectionId?: string;
    }
  | { type: "confirm"; id: string; revision: number; contextRevision: number }
  | { type: "finish"; id: string }
  | { type: "recover"; id: string }
  | { type: "pause"; id: string }
  | { type: "reconcile"; id: string }
  | { type: "invalidate"; id: string }
  | { type: "refresh-plan"; id: string }
  | { type: "compensate"; id: string }
  | { type: "cancel"; id: string }
  | { type: "analysis-consent" }
  | { type: "ask"; question: string; contextRevision: number }
  | {
      type: "answer";
      contextRevision: number;
      refs: { id: string; version: number }[];
    }
  | { type: "save-answer" }
  | { type: "agent"; value: State["agent"]; scope?: string }
  | { type: "agent-run" }
  | { type: "accept"; id: string; folder: string }
  | { type: "reject"; id: string }
  | {
      type: "share";
      id: string;
      version: number;
      members?: { id: string; version: number }[];
      days?: number;
      download?: boolean;
      mode?: "fixed" | "live";
      audience?: "link" | "users";
      recipients?: string[];
      passcodeEnabled?: boolean;
    }
  | { type: "revoke-share"; id: string }
  | { type: "receipt"; id: string }
  | { type: "comment"; id: string; version: number; body: string }
  | { type: "restore-version"; id: string; version: number }
  | { type: "rule"; mode: State["ruleMode"]; config?: RuleConfig }
  | { type: "rule-run" }
  | { type: "rule-pause" }
  | { type: "label"; ids: string[]; label: string }
  | { type: "link-device" }
  | { type: "reset"; scenario?: number }
  | { type: "folder"; path: string }
  | { type: "folder-history"; direction: "back" | "forward" }
  | { type: "create-folder"; name: string }
  | { type: "browser"; patch: Partial<State["browser"]> }
  | { type: "filter"; patch: Partial<FileFilter> }
  | {
      type: "collection-create";
      name: string;
      ids: string[];
      query?: FileFilter;
    }
  | { type: "collection-open"; id: string }
  | { type: "collection-add"; id: string; ids: string[] }
  | { type: "collection-group"; id: string; fileId: string; group: string }
  | { type: "pick"; id: string }
  | { type: "conversation-new" }
  | { type: "conversation-open"; id: string }
  | { type: "conversation-title"; title: string }
  | { type: "draft"; text: string }
  | { type: "intent"; value: Conversation["intent"] }
  | {
      type: "edit-plan";
      id: string;
      revision: number;
      ids: string[];
      target: string;
      prefix: string;
    }
  | {
      type: "resolve-conflicts";
      id: string;
      revision: number;
      choice: "keep-both" | "skip" | "overwrite";
    };

function baseReducer(state: State, action: BaseAction): State {
  const s = structuredClone(state);
  s.notice = "";
  const accessible = (id: string) =>
    s.files.find(
      (f) =>
        f.id === id &&
        f.space === s.space &&
        !f.trashed &&
        !fileUnavailable(s, f),
    );
  const changed = (text: string) => {
    s.notice = text;
    s.audit.unshift(text);
    return s;
  };
  const writes = [
    "plan",
    "confirm",
    "finish",
    "recover",
    "refresh-plan",
    "compensate",
    "analysis-consent",
    "ask",
    "save-answer",
    "agent-run",
    "accept",
    "reject",
    "share",
    "comment",
    "restore-version",
    "rule",
    "rule-run",
    "label",
    "create-folder",
    "collection-create",
    "collection-add",
    "collection-group",
    "edit-plan",
    "resolve-conflicts",
  ];
  if (
    writes.includes(action.type) &&
    (!s.online || s.viewState === "forbidden" || s.viewState === "disconnected")
  )
    return changed("当前不可执行：请恢复连接与权限，原任务和进度保留。");
  const task =
    "id" in action
      ? s.tasks.find((t) => t.id === action.id && t.space === s.space)
      : undefined;
  const file =
    "id" in action
      ? s.files.find((f) => f.id === action.id && f.space === s.space)
      : undefined;
  const agentCanContinue = (t: Task) =>
    s.agent === "active" &&
    t.items.every(
      (i) =>
        i.status === "done" ||
        i.status === "skipped" ||
        i.included === false ||
        accessible(i.fileId)?.folder === s.agentScope,
    );
  const addTask = (
    kind: Task["kind"],
    ids: string[],
    target: string,
    status: TaskStatus = "approval",
    agent = false,
  ) => {
    const t: Task = {
      id: `DB-${String(s.nextTask++).padStart(3, "0")}`,
      title: {
        organize: "整理选中的文件",
        upload: "导入新文件",
        transfer: "跨存储复制",
        agent: "生成资料摘要",
        automation: "整理新收件",
        restore: "恢复文件位置",
        trash: "移入回收站",
        rename: "批量重命名",
        copy: "复制选定文件",
      }[kind],
      kind,
      space: s.space,
      status,
      revision: 1,
      items: itemsFor(
        s.files,
        ids.filter((id) => accessible(id)),
        target,
      ),
      source: kind === "upload" ? "本机 / 手机" : "我的网盘",
      target,
      note:
        kind === "transfer"
          ? "复制到其他存储，原文件保留；同名项需确认。"
          : "仅处理已选文件，保留历史版本。",
      runs: status === "running" ? 1 : 0,
      injectPartial: kind === "organize" || kind === "transfer",
      agent,
    };
    s.tasks.unshift(t);
    return t;
  };
  switch (action.type) {
    case "folder": {
      s.browser.back.push(s.browser.folder);
      s.browser.forward = [];
      s.browser.folder = action.path;
      s.browser.collectionId = "";
      s.browser.page = 1;
      s.selected = [];
      s.page = "files";
      return s;
    }
    case "folder-history": {
      const from =
        action.direction === "back" ? s.browser.back : s.browser.forward;
      const to =
        action.direction === "back" ? s.browser.forward : s.browser.back;
      const next = from.pop();
      if (next !== undefined) {
        to.push(s.browser.folder);
        s.browser.folder = next;
        s.browser.collectionId = "";
        s.selected = [];
        s.browser.page = 1;
      }
      return s;
    }
    case "create-folder": {
      const name = action.name.trim();
      if (!name || /[\\/]/.test(name))
        return changed("请输入不含路径分隔符的文件夹名称。");
      const path =
        s.browser.folder === "全部文件"
          ? name
          : `${s.browser.folder} / ${name}`;
      if (s.folders.some((f) => f.space === s.space && f.path === path))
        return changed("这个目录下已有同名文件夹。");
      s.folders.push({
        path,
        space: s.space,
        connectionId:
          s.activeConnection === "all" ? "drive" : s.activeConnection,
      });
      return changed(`已创建文件夹「${name}」。`);
    }
    case "browser":
      s.browser = { ...s.browser, ...action.patch };
      return s;
    case "filter":
      s.selected = [];
      s.browser.filter = { ...s.browser.filter, ...action.patch };
      s.browser.page = 1;
      return s;
    case "collection-create": {
      if (!action.name.trim()) return changed("请填写集合名称。");
      const id = `c${s.collections.length + 1}`;
      s.collections.push({
        id,
        name: action.name.trim(),
        space: s.space,
        ids: action.ids.filter((id) => accessible(id)),
        query: action.query,
        groups: {},
      });
      s.browser.collectionId = id;
      s.page = "collections";
      return changed("集合已创建，原文件位置保持不变。");
    }
    case "collection-open":
      if (
        s.collections.some((c) => c.id === action.id && c.space === s.space)
      ) {
        s.browser.collectionId = action.id;
        s.page = "collections";
        s.selected = [];
      }
      return s;
    case "collection-add": {
      const c = s.collections.find(
        (c) => c.id === action.id && c.space === s.space && !c.query,
      );
      if (c)
        c.ids = [
          ...new Set([...c.ids, ...action.ids.filter((id) => accessible(id))]),
        ];
      return changed("已加入集合，未移动或复制原文件。");
    }
    case "collection-group": {
      const c = s.collections.find(
        (c) => c.id === action.id && c.space === s.space,
      );
      if (c && accessible(action.fileId))
        c.groups[action.fileId] = action.group;
      return changed("集合内分组已更新，文件目录不变。");
    }
    case "pick":
      if (file) file.picked = !file.picked;
      return s;
    case "draft":
      s.draft = action.text;
      return s;
    case "intent":
      s.intent = action.value;
      return s;
    case "conversation-title":
      if (action.title.trim()) s.conversationTitle = action.title.trim();
      return s;
    case "conversation-new":
      activateConversation(s, {
        id: `conversation-${s.nextConversation++}`,
        title: "新会话",
        space: s.space,
        context: [],
        messages: [],
        draft: "",
        intent: "summarize",
      });
      s.page = "assistant";
      return s;
    case "conversation-open": {
      const next = s.otherConversations.find(
        (c) => c.id === action.id && c.space === s.space,
      );
      if (next) activateConversation(s, next);
      return s;
    }
    case "edit-plan": {
      if (
        !task ||
        task.revision !== action.revision ||
        !["approval", "stale", "partial"].includes(task.status)
      )
        return changed("计划已变化，请重新打开编辑。");
      if (!action.ids.length) return changed("请至少保留一个待处理文件。");
      task.revisions ??= [];
      task.revisions.push({
        revision: task.revision,
        summary: `${task.target} · ${task.items.filter((i) => i.included !== false).length} 项`,
      });
      task.revision++;
      task.confirmed = undefined;
      task.target = action.target || task.target;
      task.items.forEach((i, index) => {
        if (i.status === "done" || i.status === "skipped") return;
        i.included = action.ids.includes(i.fileId);
        i.after = task.target;
        if (task.kind === "rename")
          i.afterName = `${action.prefix.trim() || "素材"}${task.items.length > 1 ? `_${String(index + 1).padStart(2, "0")}` : ""}${i.beforeName?.match(/\.[^.]+$/)?.[0] || ""}`;
      });
      task.status = "approval";
      return changed("计划已更新，旧确认失效。请检查新的修改前后。");
    }
    case "resolve-conflicts": {
      if (
        !task ||
        task.revision !== action.revision ||
        task.status !== "partial"
      )
        return changed("冲突清单已变化，请重新查看。");
      task.items.forEach((i) => {
        if (i.status !== "done" && i.status !== "skipped") {
          i.decision = action.choice;
          if (action.choice === "skip") {
            i.status = "skipped";
            i.reason = "用户选择跳过";
          }
        }
      });
      if (action.choice === "skip") {
        task.status = "done";
        return changed("已跳过冲突项，原文件保持不变。");
      }
      task.status = "running";
      task.runs++;
      return changed(
        action.choice === "overwrite"
          ? "已单独确认覆盖示例目标，继续原任务。"
          : "将为冲突项保留两份，继续原任务。",
      );
    }
    case "reset": {
      const n = initialState();
      const pages: Page[] = [
        "home",
        "inbox",
        "tasks",
        "assistant",
        "agents",
        "connections",
        "devices",
        "files",
        "automation",
      ];
      n.page = pages[action.scenario ?? 0] || "home";
      if (action.scenario === 6) n.files[0].offlineVersion = 2;
      return n;
    }
    case "navigate":
      s.page = action.page;
      return s;
    case "space": {
      if (s.space === action.space) return s;
      const next = s.otherConversations.find(
        (c) => c.space === action.space,
      ) || {
        id: `conversation-${s.nextConversation++}`,
        title: "新会话",
        space: action.space,
        context: [],
        messages: [],
        draft: "",
        intent: "summarize" as const,
      };
      activateConversation(s, next);
      s.space = action.space;
      s.selected = [];
      s.browser.folder = "全部文件";
      s.browser.collectionId = "";
      s.browser.filter = emptyFilter();
      s.browser.back = [];
      s.browser.forward = [];
      return changed("已切换空间，只显示当前空间的文件与会话。");
    }
    case "select":
      s.selected = action.ids.filter((id) => accessible(id));
      return s;
    case "context":
      s.context = action.ids.filter((id) => accessible(id));
      s.contextRevision++;
      s.analysisRefs = [];
      s.answersPending = false;
      return changed("已更新后续提问的范围，历史回答与引用保持原版本。");
    case "notice":
      s.notice = action.text;
      return s;
    case "view": {
      s.viewState = action.state;
      s.online = action.state !== "offline";
      if (action.state === "offline") s.page = "devices";
      const t = s.tasks.find((x) => x.space === s.space && x.status !== "done");
      if (t && action.state === "unknown") {
        t.status = "unknown";
        s.page = "tasks";
      }
      if (t && action.state === "stale") {
        t.status = "stale";
        t.revision++;
        s.page = "tasks";
      }
      if (t && action.state === "partial" && s.knowledge.taskPlans[t.id]) {
        t.status = "unknown";
        s.page = "tasks";
        s.notice = "资源任务结果未知，请查询原操作。";
        return s;
      }
      if (t && action.state === "partial") {
        t.status = "partial";
        t.runs = 1;
        t.items.forEach((i, index) => {
          i.status = index ? "failed" : "done";
          i.reason = index ? "连接中断，等待恢复" : undefined;
          if (!index) {
            const f = accessible(i.fileId);
            if (f) f.folder = i.after;
          }
        });
        s.page = "tasks";
      }
      if (action.state === "forbidden") {
        s.messages = [];
        s.analysisRefs = [];
        s.answersPending = false;
        s.contextRevision++;
      }
      return s;
    }
    case "favorite":
      if (file) file.favorite = !file.favorite;
      return s;
    case "offline":
      if (!file) return s;
      if (!s.online) return changed("当前离线，连接网络后可更新副本。");
      if (
        action.version !== undefined &&
        (action.version < 1 || action.version > file.version)
      )
        return changed("固定版本不可用，未下载其他版本。");
      file.offlineVersion = action.version ?? file.version;
      return changed("已保存此版本的离线副本（演示）。");
    case "network":
      s.online = action.online;
      return changed(
        action.online
          ? "已重新连接，离线副本需要检查版本。"
          : "当前离线，可阅读已下载的版本；修改操作等待联网。",
      );
    case "remote-version":
      if (file) {
        file.snapshots = Array.from({ length: file.version }, (_, i) =>
          versionSnapshot(file, i + 1),
        );
        file.version++;
        file.snapshots.push({
          version: file.version,
          content: "新的云端版本补充了收尾镜头与录音说明。",
          accent: file.version,
        });
      }
      return changed("云端出现新版本，离线副本保持原版本。");
    case "import": {
      if (!s.online) return changed("当前离线，请联网后提交上传。");
      const id = `import-${s.nextTask}`;
      s.files.unshift({
        id,
        name:
          action.source === "phone"
            ? "山行随拍 · 手机导入.jpg"
            : "新灵感 · 桌面导入.pdf",
        kind: action.source === "phone" ? "image" : "document",
        size: "4.8 MB",
        folder:
          s.storage.nodes.find((n) => n.id === action.directoryId)?.path ||
          s.options.uploadTarget,
        connectionId:
          s.storage.nodes.find((n) => n.id === action.directoryId)
            ?.connectionId || "drive",
        rootId: "root",
        version: 1,
        space: s.space,
        origin: action.source,
        favorite: false,
        inbox: true,
      });
      addTask("upload", [id], s.files[0].folder, "running");
      s.page = "tasks";
      return changed("模拟上传已开始：发送、验证、保存分开显示。");
    }
    case "plan": {
      if (!s.online || s.viewState === "forbidden")
        return changed("当前无法修改文件，请恢复连接与权限。");
      const ids = action.ids.filter((id) => accessible(id));
      if (!ids.length) return changed("请先选择当前空间内的文件。");
      const t = addTask(
        action.kind || "organize",
        ids,
        action.target || "山野计划 / 参考资料",
      );
      if (action.kind === "rename")
        t.items.forEach((i, n) => {
          const f = accessible(i.fileId)!;
          i.beforeName = f.name;
          i.afterName = `${action.prefix?.trim() || "素材"}${t.items.length > 1 ? `_${String(n + 1).padStart(2, "0")}` : ""}${f.name.match(/\.[^.]+$/)?.[0] || ""}`;
        });

      s.page = "tasks";
      return changed("计划已生成，请检查修改前后与影响范围。");
    }
    case "confirm": {
      if (!task || action.contextRevision !== s.contextRevision)
        return changed("空间已变化，请重新打开原计划。");
      if (!s.online || s.viewState === "forbidden")
        return changed("无法确认：请恢复网络或权限。");
      if (task.status !== "approval")
        return changed("该计划已处理或已过期，没有重复执行。");
      if (
        task.revision !== action.revision ||
        task.items.some(
          (i) =>
            i.status !== "done" &&
            i.status !== "skipped" &&
            i.included !== false &&
            (!accessible(i.fileId) ||
              accessible(i.fileId)!.version !== i.version ||
              (i.beforeName && accessible(i.fileId)!.name !== i.beforeName)),
        )
      ) {
        task.status = "stale";
        return changed("计划已过期，请重新预览后确认。");
      }
      if (task.agent && !agentCanContinue(task)) {
        task.status = "blocked";
        return changed("Agent 授权已失效或范围变化，未执行任何新操作。");
      }
      task.confirmed = task.revision;
      task.status = "running";
      task.runs++;
      return changed(`已确认 ${task.id}，按当前计划执行。`);
    }
    case "finish": {
      if (
        !task ||
        task.status !== "running" ||
        !s.online ||
        s.viewState === "forbidden"
      )
        return s;
      if (task.agent && !agentCanContinue(task)) {
        task.status = "blocked";
        return changed("授权已撤销或范围变化，任务停止；已完成项保留。");
      }
      let failed = false;
      task.items.forEach((item, index) => {
        if (item.status === "done" || item.status === "skipped") return;
        if (item.included === false || item.decision === "skip") {
          item.status = "skipped";
          item.reason = "本次跳过，原文件保留";
          return;
        }
        const f = accessible(item.fileId);
        if (!f || f.version !== item.version) {
          item.status = "stale";
          item.reason = "文件版本或权限发生变化";
          failed = true;
          return;
        }
        const destination = s.files.find(
          (x) =>
            x.space === task.space &&
            !x.trashed &&
            x.folder === item.after &&
            (!task.targetConnectionId ||
              (x.connectionId || "drive") === task.targetConnectionId) &&
            x.name === f.name,
        );
        if (
          ["transfer", "copy"].includes(task.kind) &&
          destination &&
          !item.decision
        ) {
          item.status = "failed";
          item.reason = "目标存在同名文件，尚未覆盖";
          failed = true;
          return;
        }
        if (
          task.injectPartial &&
          task.runs === 1 &&
          index === task.items.length - 1
        ) {
          item.status = "failed";
          item.reason =
            task.kind === "transfer"
              ? "目标存在同名文件，尚未覆盖"
              : "连接中断，尚未移动";
          failed = true;
          return;
        }
        item.status = "done";
        item.reason = undefined;
        if (
          task.kind === "organize" ||
          task.kind === "restore" ||
          (task.kind === "automation" && task.automationMode === "organize")
        ) {
          f.folder = item.after;
          f.inbox = false;
        }
        if (task.kind === "trash") f.trashed = true;
        if (task.kind === "rename" && item.afterName) f.name = item.afterName;
        if (task.kind === "transfer" || task.kind === "copy") {
          if (
            item.decision === "overwrite" &&
            destination &&
            destination.id !== f.id
          ) {
            const snapshot = versionSnapshot(f, f.version);
            destination.snapshots = Array.from(
              { length: destination.version },
              (_, i) => versionSnapshot(destination, i + 1),
            );
            destination.version++;
            destination.snapshots.push({
              ...snapshot,
              version: destination.version,
            });
          } else
            s.files.push({
              ...f,
              id: `${task.id}-${f.id}`,
              name:
                task.runs > 1 && item.decision !== "overwrite"
                  ? f.name.replace(/(\.[^.]+)$/, "（副本）$1")
                  : f.name,
              folder: item.after,
              origin: "cloud",
              connectionId: task.targetConnectionId || f.connectionId,
              externalRevision:
                task.targetConnectionId && task.targetConnectionId !== "drive"
                  ? `${task.id}-revision-1`
                  : undefined,
              offlineVersion: undefined,
            });
        }
        if (["copy", "transfer"].includes(task.kind)) {
          item.outputId =
            item.decision === "overwrite" && destination
              ? destination.id
              : `${task.id}-${f.id}`;
          item.outputVersion = s.files.find(
            (x) => x.id === item.outputId,
          )?.version;
        }
        if (task.kind === "automation" && task.automationMode !== "organize")
          s.files.push({
            ...f,
            id: `${task.id}-${f.id}`,
            folder: task.target,
            origin: "cloud",
            inbox: false,
            offlineVersion: undefined,
          });
      });
      task.status = failed ? "partial" : "done";
      if (
        task.kind === "agent" &&
        !failed &&
        !s.files.some((f) => f.id === `${task.id}-output`)
      )
        s.files.unshift({
          id: `${task.id}-output`,
          name: "山野资料 · Agent 摘要.md",
          kind: "document",
          size: "42 KB",
          folder: "待整理",
          version: 1,
          space: s.space,
          origin: "agent",
          favorite: false,
          inbox: true,
          inputRefs: task.items.map((i) => ({
            id: i.fileId,
            version: i.version,
          })),
        });
      return changed(
        failed
          ? `${task.id} 部分完成。已完成项不会重做。`
          : `${task.id} 已完成，请检查结果。`,
      );
    }
    case "recover": {
      if (!task || !["partial", "paused"].includes(task.status)) return s;
      if (!s.online) return changed("请先恢复网络，原任务与进度已保留。");
      if (
        task.items.some(
          (i) =>
            i.status !== "done" &&
            i.status !== "skipped" &&
            i.included !== false &&
            (!accessible(i.fileId) ||
              accessible(i.fileId)!.version !== i.version),
        )
      ) {
        task.status = "stale";
        return changed("未完成项已变化，请重新预览。");
      }
      if (task.agent && !agentCanContinue(task)) {
        task.status = "blocked";
        return changed("需要恢复有效授权。");
      }
      task.items.forEach((i) => {
        if (i.status === "failed") {
          i.status = "pending";
          if (["transfer", "copy"].includes(task.kind))
            i.decision ??= "keep-both";
        }
      });
      task.status = "running";
      task.runs++;
      return changed(`继续原任务 ${task.id}，只处理未完成项。`);
    }
    case "pause":
      if (task?.status === "running") task.status = "paused";
      return changed("已暂停，已完成项与进度保留。");
    case "cancel":
      if (task && task.status !== "done") task.status = "cancelled";
      return changed("已取消剩余操作；已经完成的修改保留。");
    case "reconcile":
      if (task?.status === "unknown") {
        task.status = "partial";
        task.runs = 1;
        task.items[0].status = "done";
        const f = accessible(task.items[0].fileId);
        if (f && task.kind === "organize") f.folder = task.items[0].after;
      }
      return changed("已回读原操作，保留原编号与完成记录。");
    case "invalidate":
      if (task) {
        task.revision++;
        task.status = "stale";
      }
      return changed("计划内容发生变化，旧确认已失效。");
    case "refresh-plan": {
      if (!task) return s;
      task.revisions ??= [];
      task.revisions.push({
        revision: task.revision,
        summary: "重新预览前的清单，已完成结果保留",
      });
      task.revision++;
      task.confirmed = undefined;
      task.items = task.items.map((i) =>
        i.status === "done" || i.status === "skipped"
          ? i
          : accessible(i.fileId)
            ? {
                ...i,
                version: accessible(i.fileId)!.version,
                beforeName: i.beforeName
                  ? accessible(i.fileId)!.name
                  : undefined,
                status: "pending",
                reason: undefined,
              }
            : { ...i, status: "stale", reason: "当前无权访问" },
      );
      task.status = task.items.some(
        (i) => i.status === "pending" && i.included !== false,
      )
        ? "approval"
        : "blocked";
      return changed("已更新未完成项的预览；已完成结果与原操作编号保留。");
    }
    case "compensate": {
      if (!task) return s;
      const done = task.items.filter((i) => i.status === "done");
      if (
        done.some(
          (i) =>
            !accessible(i.fileId) ||
            accessible(i.fileId)!.version !== i.version ||
            accessible(i.fileId)!.folder !== i.after,
        )
      )
        return changed("恢复冲突：文件已被后续修改，请人工检查。");
      const t = addTask(
        "restore",
        done.map((i) => i.fileId),
        "原文件夹",
      );
      t.items = done.map((i) => ({
        ...i,
        before: i.after,
        after: i.before,
        status: "pending",
      }));
      return changed("已生成恢复计划，确认后才会移动文件。");
    }
    case "analysis-consent":
      s.analysisRefs = s.context.map(
        (id) => `${id}@${accessible(id)?.version}`,
      );
      return changed("本次选定文件已允许模拟分析，不会自动分析全盘。");
    case "ask": {
      if (action.contextRevision !== s.contextRevision) return s;
      if (!s.online || s.viewState === "forbidden")
        return changed("当前无法分析，请恢复连接与权限。");
      if (!s.context.length) return changed("请先添加文件上下文。");
      if (
        s.context.some(
          (id) => !s.analysisRefs.includes(`${id}@${accessible(id)?.version}`),
        )
      )
        return changed("请先确认本次内容分析范围。");
      if (s.answersPending) return s;
      s.messages.push({
        id: s.messages.length + 1,
        role: "user",
        text: action.question,
      });
      s.draft = "";
      s.answersPending = true;
      return s;
    }
    case "answer": {
      if (action.contextRevision !== s.contextRevision) return s;
      if (s.viewState === "forbidden" || !s.online) {
        s.answersPending = false;
        return s;
      }
      if (!s.answersPending) return s;
      if (
        action.refs.some(
          (ref) =>
            !accessible(ref.id) || accessible(ref.id)!.version !== ref.version,
        )
      ) {
        s.answersPending = false;
        return changed("资料版本发生变化，请重新确认分析范围。");
      }
      s.answersPending = false;
      s.messages.push({
        id: s.messages.length + 1,
        role: "assistant",
        kind: s.intent === "search" ? "candidates" : "answer",
        text:
          s.intent === "search"
            ? "在已选范围中找到以下相关候选。这里展示的是示例 Top-K 检索，不代表全盘或全部匹配；选择引用可查看固定版本。"
            : "这组资料围绕「山野里的慢生活」展开。简报强调自然光、真实的行走感和克制的叙述；脚本用抵达、停留、归途三个段落串联。建议先整理视觉参考，再补齐第二段的环境声。以下为示例回答，仅依据已选择的文件。",
        refs: action.refs,
      });
      return s;
    }
    case "save-answer": {
      if (!s.messages.some((m) => m.refs?.length))
        return changed("请先生成带引用的回答。");
      if (s.files.some((f) => f.id === `answer-${s.contextRevision}`))
        return changed("该回答已经保存，没有重复生成。");
      s.files.unshift({
        id: `answer-${s.contextRevision}`,
        name: "资料问答 · 已保存.md",
        kind: "document",
        size: "12 KB",
        folder: "待整理",
        version: 1,
        space: s.space,
        origin: "agent",
        favorite: false,
        inbox: true,
        inputRefs: s.messages.filter((m) => m.refs?.length).at(-1)?.refs,
      });
      return changed("回答已保存到收件箱，引用版本一并保留（演示）。");
    }
    case "agent":
      s.agent = action.value;
      if (action.scope) s.agentScope = action.scope;
      if (action.value !== "active")
        s.tasks
          .filter(
            (t) =>
              t.agent && ["running", "approval", "partial"].includes(t.status),
          )
          .forEach((t) => {
            t.status = "blocked";
          });
      else
        s.tasks
          .filter((t) => t.agent && t.status === "blocked")
          .forEach((t) => {
            t.status = "stale";
            t.revision++;
          });
      return changed(
        action.value === "active"
          ? "演示连接已验证，授权仅限所选目录；旧任务需要重新预览。"
          : "授权状态已更新，未完成任务已停止。",
      );
    case "agent-run": {
      if (s.agent !== "active") return changed("请先连接并授权 Agent。");
      const ids = s.files
        .filter(
          (f) => f.space === s.space && f.folder === s.agentScope && !f.trashed,
        )
        .slice(0, 2)
        .map((f) => f.id);
      if (!ids.length) return changed("授权目录内没有可读文件。");
      addTask("agent", ids, "产物收件箱", "running", true);
      s.page = "tasks";
      return changed("外部 Agent 正在处理授权目录（模拟）。");
    }
    case "accept":
      if (file?.inbox) {
        file.folder = action.folder;
        file.inbox = false;
      }
      return changed("已接收并归档，文件版本保持不变。");
    case "reject":
      if (file?.inbox) {
        file.inbox = false;
        file.trashed = true;
      }
      return changed("产物已退回并移入回收站，可恢复。");
    case "share": {
      const members = action.members || [
        { id: action.id, version: action.version },
      ];
      if (
        !members.length ||
        members.some(
          (r) =>
            !accessible(r.id) ||
            r.version < 1 ||
            r.version > accessible(r.id)!.version,
        )
      )
        return changed("交付清单缺项或失权，尚未发布。");
      s.shares.unshift({
        id: `share-${s.shares.length + 1}`,
        fileId: members[0].id,
        version: members[0].version,
        members,
        mode: action.mode || "fixed",
        audience: action.audience || "link",
        recipients: action.recipients || [],
        passcodeEnabled: action.passcodeEnabled || false,
        days: action.days || 7,
        download: action.download ?? true,
        state: "active",
        received: false,
      });
      s.page = "shares";
      return changed(
        action.mode === "live"
          ? "动态分享已确认，后续访问显示最新版本（演示）。"
          : "交付包已按清单固定版本，不会跟随原文件更新（演示）。",
      );
    }
    case "revoke-share": {
      const share = s.shares.find((x) => x.id === action.id);
      if (share) share.state = "revoked";
      return changed("分享已撤销。已经下载的副本无法召回。");
    }
    case "receipt": {
      const share = s.shares.find((x) => x.id === action.id);
      if (share?.state === "active") share.received = true;
      return changed("已记录模拟接收事实，不代表内容被采纳。");
    }
    case "comment":
      if (accessible(action.id) && action.body.trim())
        s.comments.push({
          fileId: action.id,
          version: action.version,
          body: action.body.trim(),
        });
      return changed("评论已保存在所选版本（演示）。");
    case "restore-version": {
      if (!file || action.version < 1 || action.version > file.version)
        return changed("所选历史版本不存在。");
      if (file.trashed) file.trashed = false;
      else {
        const restored = versionSnapshot(file, action.version);
        file.snapshots = Array.from({ length: file.version }, (_, i) =>
          versionSnapshot(file, i + 1),
        );
        file.version++;
        file.snapshots.push({ ...restored, version: file.version });
      }
      return changed(
        `已恢复所选 v${action.version} 的内容，所有历史版本保留（演示）。`,
      );
    }
    case "rule":
      if (
        action.config &&
        (!action.config.source ||
          !action.config.target ||
          !Number.isInteger(action.config.maxFiles) ||
          action.config.maxFiles < 1 ||
          action.config.maxFiles > 500 ||
          !Number.isInteger(action.config.validHours) ||
          action.config.validHours < 1 ||
          action.config.validHours > 168)
      )
        return changed("规则范围或限制无效，请重新检查。");
      s.rule = "active";
      s.ruleMode = action.mode;
      if (action.config) s.ruleConfig = action.config;
      return changed(
        `规则已启用：${s.ruleConfig.source}，最多 ${s.ruleConfig.maxFiles} 项，有效 ${s.ruleConfig.validHours} 小时。`,
      );
    case "rule-run": {
      if (s.rule !== "active") return changed("请先启用规则。");
      const ids = s.files
        .filter(
          (f) =>
            f.space === s.space &&
            !f.trashed &&
            (s.ruleConfig.source === "待整理"
              ? f.inbox
              : s.ruleConfig.source === "手机已选相册"
                ? f.origin === "phone"
                : s.ruleConfig.source === "桌面已选目录"
                  ? f.origin === "local"
                  : f.folder === s.ruleConfig.source),
        )
        .slice(0, s.ruleConfig.maxFiles)
        .map((f) => f.id);
      if (!ids.length) return changed("指定范围没有新文件，本次不创建空任务。");
      const t = addTask("automation", ids, s.ruleConfig.target, "paused");
      t.automationMode = s.ruleMode;
      t.title =
        s.ruleMode === "backup"
          ? "手机相册单向备份"
          : s.ruleMode === "sync"
            ? "指定目录同步新增副本"
            : "整理新收件";
      t.note = `等待 Wi-Fi 或系统恢复；${s.ruleMode === "organize" ? "恢复后移动到归档目录" : "恢复后新增副本，源文件保留、不传播删除"}。原任务已保留。`;
      s.rule = "paused";
      s.page = "tasks";
      return changed("规则已触发，当前条件不满足，任务暂停。");
    }
    case "rule-pause":
      s.rule = "paused";
      return changed("规则已暂停，不再产生新操作。");
    case "label":
      action.ids.forEach((id) => {
        const f = accessible(id);
        if (f) f.label = action.label;
      });
      return changed("已按本次确认更新标签。");
    case "link-device":
      s.deviceLinked = true;
      return changed("演示设备已关联，尚未授予目录访问或 Agent 权限。");
  }
}

export type Action =
  | BrowseAction
  | OrganizationAction
  | BaseAction
  | CloudAction
  | V4Action
  | KnowledgeAction
  | StorageAction;
function domainReducer(state: State, action: Action): State {
  if (
    "id" in action &&
    typeof action.id === "string" &&
    state.knowledge.taskPlans[action.id]
  ) {
    const task = state.tasks.find((t) => t.id === action.id);
    if (action.type === "confirm") {
      if (!task || action.revision !== task.revision)
        return { ...state, notice: "计划修订已变化，旧确认失效。" };
      return knowledgeReduce(structuredClone(state), {
        type: "k-task-apply",
        id: action.id,
      })!;
    }
    if (["finish", "recover"].includes(action.type))
      return { ...state, notice: "先查询并检查原资源任务；未重新执行。" };
    if (action.type === "refresh-plan")
      return knowledgeReduce(structuredClone(state), {
        type: "k-task-refresh",
        id: action.id,
      })!;
    if (action.type === "reconcile") {
      const next = structuredClone(state),
        t = next.tasks.find((t) => t.id === action.id)!;
      if (t.space !== state.space) return state;
      if (t.status === "unknown")
        t.status = t.items.every(
          (i) =>
            i.status === "done" &&
            i.outputId &&
            next.files.some((f) => f.id === i.outputId),
        )
          ? "done"
          : t.runs === 0
            ? "approval"
            : "blocked";
      next.notice = "已查询原操作；没有创建新任务或重新执行。";
      return next;
    }
  }
  if ("id" in action && typeof action.id === "string") {
    const upload = state.knowledge.uploads.find((u) => u.taskId === action.id);
    if (upload && action.type === "finish")
      return knowledgeReduce(structuredClone(state), {
        type: "k-upload-finish",
        id: upload.id,
      })!;
    if (upload && action.type === "recover")
      return knowledgeReduce(structuredClone(state), {
        type: "k-upload-recover",
        id: upload.id,
      })!;
    if (upload && ["pause", "cancel"].includes(action.type)) {
      const next = structuredClone(state);
      next.knowledge.uploads.find((u) => u.id === upload.id)!.status =
        action.type === "pause" ? "failed" : "cancelled";
      next.tasks.find((t) => t.id === upload.taskId)!.status =
        action.type === "pause" ? "paused" : "cancelled";
      return next;
    }
  }
  if (
    state.actor !== "yeisme" &&
    [
      "share",
      "revoke-share",
      "app-create",
      "app-state",
      "agent",
      "index-create",
      "index-clear",
      "rule",
      "tag-merge",
      "tag-delete",
      "import",
      "create-folder",
    ].includes(action.type)
  )
    return { ...state, notice: "当前身份没有管理或外发权限。" };
  if (
    [
      "plan",
      "confirm",
      "recover",
      "restore-version",
      "tag-apply",
      "label",
      "accept",
      "reject",
      "edit-plan",
      "resolve-conflicts",
    ].includes(action.type)
  ) {
    const ids =
      "ids" in action
        ? action.ids
        : "id" in action
          ? state.tasks
              .find((t) => t.id === action.id)
              ?.items.map((i) => i.fileId) || [action.id]
          : [];
    if (
      ids.some((id) => {
        const f = state.files.find((f) => f.id === id);
        return f && !canEditFile(state, f);
      })
    )
      return { ...state, notice: "当前文件只可查看，无法修改或整理。" };
  }
  if (
    ["ask", "analysis-consent", "answer"].includes(action.type) &&
    contextUnavailable(state)
  )
    return {
      ...state,
      answersPending: false,
      notice: "上下文引用已失效，请移除或重新选择；未改用最新版本。",
    };
  if (
    [
      "confirm",
      "recover",
      "finish",
      "refresh-plan",
      "resolve-conflicts",
    ].includes(action.type) &&
    "id" in action
  ) {
    const t = state.tasks.find((t) => t.id === action.id);
    if (
      t &&
      (t.items.some(
        (i) =>
          i.status !== "done" &&
          fileUnavailable(
            state,
            state.files.find((f) => f.id === i.fileId),
            i.version,
          ),
      ) ||
        (t.targetConnectionId &&
          !state.connections.some(
            (c) =>
              c.id === t.targetConnectionId &&
              c.status === "active" &&
              c.authorized,
          )))
    )
      return {
        ...state,
        notice: "来源或目标连接不可用，原任务保留，请恢复授权。",
      };
  }
  const extended =
    knowledgeReduce(structuredClone(state), action as KnowledgeAction) ||
    v4Reduce(structuredClone(state), action as V4Action) ||
    cloudReduce(structuredClone(state), action as CloudAction);
  const next = extended || baseReducer(state, action as BaseAction);
  if (
    action.type === "plan" &&
    action.targetConnectionId &&
    next.tasks[0]?.id !== state.tasks[0]?.id
  )
    next.tasks[0].targetConnectionId = action.targetConnectionId;
  if (next.page === "assistant") {
    next.page = "search";
    next.notice = "内置助手已移除，请使用检索或外部应用。";
  }
  next.agentOpen = false;
  if (action.type === "space") {
    next.search = emptySearch();
    next.searchSubmitted = false;
    next.options = {
      ...next.options,
      uploadTarget:
        next.folders.find((f) => f.space === next.space)?.path || "待整理",
      searchFolder: "全部文件",
      agentFolder: "已选文件",
    };
  }
  if (action.type === "remote-version")
    next.indexes
      .filter((i) => i.refs.some((r) => r.id === action.id))
      .forEach((i) => (i.status = "stale"));
  if (action.type === "label") {
    let tag = next.tags.find(
      (t) => t.space === next.space && t.name === action.label,
    );
    if (!tag && next.online && next.viewState !== "forbidden") {
      tag = {
        id: `tag-${next.nextTag++}`,
        space: next.space,
        name: action.label,
        group: "",
        color: "pine",
        favorite: false,
      };
      next.tags.push(tag);
    }
    if (tag)
      return cloudReduce(next, {
        type: "tag-apply",
        ids: action.ids,
        tagIds: [tag.id],
        mode: "add",
      })!;
  }
  return v4After(state, next, action);
}

function existingReducer(state: State, action: Action): State {
  const projected = storageReduce(state, action as StorageAction);
  if (projected) return projected;
  if (
    action.type === "agent-panel" ||
    action.type === "agent-tab" ||
    action.type === "ask" ||
    action.type === "answer"
  )
    return {
      ...state,
      agentOpen: false,
      notice: "内置助手已移除；外部应用与固定引用继续可用。",
    };
  if (action.type === "create-folder") {
    const s = structuredClone(state),
      node = s.storage.nodes.find((n) => n.id === s.storage.activeId)!;
    const connection = s.connections.find((c) => c.id === node.connectionId);
    if (
      !s.online ||
      s.actor !== "yeisme" ||
      directoryAvailable(s, node) ||
      connection?.write !== "passed"
    )
      return {
        ...state,
        notice: "目录写入能力尚未验证或权限不可用，请选择可写存储。",
      };
    const name = action.name.trim();
    if (!name || /[\\/]/.test(name))
      return { ...state, notice: "请输入有效目录名称。" };
    if (s.storage.nodes.some((n) => n.parentId === node.id && n.name === name))
      return { ...state, notice: "当前目录已有同名文件夹。" };
    const path = node.path === "全部文件" ? name : node.path + " / " + name;
    s.storage.nodes.push({
      ...node,
      id: `directory-${s.storage.next++}`,
      parentId: node.id,
      name,
      path,
      status: "ready",
    });
    s.notice = `已创建文件夹「${name}」。`;
    return ensureDirectories(s);
  }
  if (action.type === "import" && action.directoryId) {
    const { directoryId } = action;
    const node = state.storage.nodes.find((n) => n.id === directoryId),
      c = state.connections.find((c) => c.id === node?.connectionId);
    if (directoryAvailable(state, node) || c?.write !== "passed")
      return { ...state, notice: "目标目录写入尚未验证，请选择可写存储。" };
  }
  if (action.type === "plan" && action.targetDirectoryId) {
    const { targetDirectoryId } = action;
    const n = state.storage.nodes.find((n) => n.id === targetDirectoryId);
    if (!n || directoryAvailable(state, n))
      return { ...state, notice: "目标连接目录不可用。" };
    if (
      action.kind !== "copy" &&
      action.kind !== "transfer" &&
      action.ids.some(
        (id) =>
          state.files.find((f) => f.id === id)?.connectionId !== n.connectionId,
      )
    )
      return {
        ...state,
        notice: "跨存储请使用复制计划；来源清理需要独立确认。",
      };
    action = { ...action, target: n.path, targetConnectionId: n.connectionId };
  }
  let next = domainReducer(state, action);
  if (next === state) next = structuredClone(next);
  if (
    action.type === "plan" &&
    next.tasks[0]?.id !== state.tasks[0]?.id &&
    action.sourceDirectoryId
  ) {
    const root = state.storage.nodes.find(
      (n) => n.id === action.sourceDirectoryId,
    );
    if (root)
      next.tasks[0].items.forEach((item) => {
        const file = state.files.find((f) => f.id === item.fileId)!;
        const rel =
          root.path === "全部文件"
            ? file.folder === root.path
              ? ""
              : file.folder
            : file.folder === root.path
              ? ""
              : file.folder.startsWith(root.path + " / ")
                ? file.folder.slice(root.path.length + 3)
                : "";
        item.after =
          (action.target || next.tasks[0].target) + (rel ? " / " + rel : "");
      });
  }
  ensureDirectories(next);
  if (action.type === "connection-save" && action.scanExclusions) {
    const connectionId = action.connection.id;
    const saved = next.connections.find((c) => c.id === connectionId),
      previous = state.connections.find((c) => c.id === connectionId);
    if (
      saved?.revision === (previous?.revision || 0) + 1 &&
      saved.root === action.connection.root &&
      saved.read === "passed"
    ) {
      const directoryId = rootNode(next, saved.id)!.id;
      next = storageReduce(next, {
        type: "catalog-configure",
        directoryId,
        exclusions: action.scanExclusions,
        budget: 10000,
        policy: "manual",
      })!;
      next = storageReduce(next, { type: "catalog-start", directoryId })!;
    }
  }
  if (action.type === "connection-select") {
    const old = state.storage.activeId;
    next.storage.positions[`${state.space}:${state.activeConnection}`] = {
      id: old,
      scroll: action.scroll,
    };
    next.storage.activeId =
      next.storage.positions[`${next.space}:${action.id}`]?.id ||
      rootNode(next, action.id)?.id ||
      rootNode(next, "drive")!.id;
    next.browser.folder = next.storage.nodes.find(
      (n) => n.id === next.storage.activeId,
    )!.path;
    next.notice = "已切换存储，批量选择已清除。";
  }
  if (action.type === "folder") {
    const n = next.storage.nodes.find(
      (n) =>
        n.space === next.space &&
        n.connectionId === next.activeConnection &&
        n.path === action.path,
    );
    if (n) next.storage.activeId = n.id;
  }
  return next;
}

function domainWithOrganization(state: State, action: Action): State {
  if (action.type === "search-save") {
    state = structuredClone(state);
    const o = state.organization;
    state.search.view = {
      columns: o.columns,
      widths: o.widths,
      groupBy: o.groupBy,
      sortKey: o.sortKey,
      listSort: browseLocation(state).sort,
    };
  }
  const organized = organizationReduce(state, action as OrganizationAction);
  if (organized) return organized;
  const next = syncOrganization(existingReducer(state, action));
  if (action.type === "search-open" && next.search.view)
    Object.assign(next.organization, next.search.view);
  return next;
}

export function reducer(state: State, action: Action): State {
  const location = browseReduce(state, action as BrowseAction);
  return (
    location ||
    browseAfter(state, domainWithOrganization(state, action), action)
  );
}
