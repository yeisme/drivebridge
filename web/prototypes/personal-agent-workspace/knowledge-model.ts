import { patchProperty, readProperties } from "./properties";
import { markdownLanguage } from "@codemirror/lang-markdown";
import type { State, FileRecord, Space, Task } from "./model";
import { canEditFile, fileUnavailable, contextRefs } from "./v4-model";

// Resettable prototype state, never a production API or persisted knowledge index.
export type EditorMode = "live" | "source" | "read";
export type SaveStatus =
  | "saved"
  | "dirty"
  | "saving"
  | "offline"
  | "failed"
  | "conflict";
export interface NoteRecord {
  versions: Record<number, string>;
  draft: string;
  base: number;
  dirty: boolean;
  mode: EditorMode;
  showDraft: boolean;
  status: SaveStatus;
  composing: boolean;
  selection: { from: number; to: number };
  fault: boolean;
}
export interface SkillResource {
  fileId: string;
  kind: "skill" | "template";
  name: string;
  description: string;
  author: string;
  source: string;
  tools: string[];
  inputs: string;
  outputs: string;
  extra: Record<string, string>;
  checkedVersion?: number;
  checkedAt?: string;
  missing: string[];
  files: { path: string; fileId?: string }[];
}
export interface KnowledgeState {
  notes: Record<string, NoteRecord>;
  resources: SkillResource[];
  seeded: Space[];
  counter: number;
  graphFile: string;
  graphKind: "document" | "attachment" | "tag";
  options: {
    autosave: boolean;
    defaultMode: EditorMode;
    attachmentFolder: string;
    linkFormat: "wiki" | "markdown";
    relatedApproval: boolean;
  };
  taskPlans: Record<
    string,
    {
      target?: string;
      body?: string;
      sections?: {
        id: string;
        version: number;
        anchor?: string;
        excerpt: string;
      }[];
      skillId: string;
      skillVersion: number;
      refs: { id: string; version: number }[];
    }
  >;
  selectedSkills: Record<string, { id: string; version: number }>;
  uploads: {
    id: string;
    taskId: string;
    noteId: string;
    name: string;
    kind: FileRecord["kind"];
    size: string;
    status: "pending" | "failed" | "done" | "cancelled";
    fileId?: string;
  }[];
  renamePlan?: {
    id: string;
    name: string;
    folder: string;
    version: number;
    refs: { id: string; version: number; before: string; after: string }[];
  };
}
export type KnowledgeAction =
  | { type: "k-seed" }
  | { type: "k-list-demo" }
  | { type: "k-resource-new"; name: string; kind: "skill" | "template" }
  | { type: "k-link-mention"; id: string; target: string }
  | { type: "k-new"; name: string; body?: string; copyId?: string }
  | { type: "k-open"; id: string; mode?: EditorMode }
  | { type: "k-inspect"; id: string }
  | { type: "k-restore-view"; id: string; mode: EditorMode; showDraft: boolean }
  | { type: "k-mode"; id: string; mode: EditorMode }
  | {
      type: "k-edit";
      id: string;
      body: string;
      selection?: { from: number; to: number };
    }
  | { type: "k-compose"; id: string; value: boolean }
  | {
      type: "k-save-start" | "k-save" | "k-reload" | "k-remote" | "k-fault";
      id: string;
    }
  | { type: "k-options"; patch: Partial<KnowledgeState["options"]> }
  | { type: "k-graph"; id: string; kind?: KnowledgeState["graphKind"] }
  | {
      type: "k-skill-fields";
      id: string;
      name: string;
      description: string;
      tools: string[];
    }
  | { type: "k-check" | "k-skill-select" | "k-skill-remove"; id: string }
  | {
      type: "k-attach";
      id: string;
      name: string;
      kind: FileRecord["kind"];
      size: string;
      fail?: boolean;
    }
  | { type: "k-upload-finish" | "k-upload-recover"; id: string }
  | { type: "k-rename-plan"; id: string; name: string; folder: string }
  | { type: "k-rename-apply" | "k-rename-cancel" }
  | { type: "k-task"; mode: "output" | "edit"; target?: string; goal: string }
  | { type: "k-task-apply" | "k-task-refresh"; id: string };
export const knowledgeInitial = (): KnowledgeState => ({
  notes: {},
  resources: [],
  seeded: [],
  counter: 1,
  graphFile: "",
  graphKind: "document",
  options: {
    autosave: true,
    defaultMode: "live",
    attachmentFolder: "知识库 / 附件",
    linkFormat: "wiki",
    relatedApproval: true,
  },
  taskPlans: {},
  selectedSkills: {},
  uploads: [],
});
export const isMarkdown = (f?: FileRecord) =>
  !!f && /\.(md|markdown)$/i.test(f.name);
export function noteBody(s: State, id: string, version?: number) {
  const f = s.files.find((x) => x.id === id);
  return (
    s.knowledge.notes[id]?.versions[version ?? f?.version ?? 1] ??
    f?.snapshots?.find((x) => x.version === (version ?? f.version))?.content ??
    "# " + (f?.name.replace(/\.md$/i, "") || "文档") + "\n\n在这里开始记录。"
  );
}
export function noteRecord(s: State, f: FileRecord): NoteRecord {
  const body = noteBody(s, f.id);
  return {
    versions: { [f.version]: body },
    draft: body,
    base: f.version,
    dirty: false,
    mode: "read",
    showDraft: false,
    status: "saved",
    composing: false,
    selection: { from: 0, to: 0 },
    fault: false,
  };
}
export function writableNote(s: State, f?: FileRecord) {
  if (!f || !canEditFile(s, f)) return false;
  const c = s.connections.find((x) => x.id === f.connectionId);
  return !c || c.kind === "drive" || c.write === "passed";
}
function addNote(
  s: State,
  id: string,
  name: string,
  body: string,
  folder = "知识库",
  mode: EditorMode = "read",
) {
  const f: FileRecord = {
    id,
    name,
    kind: "document",
    size: `${new TextEncoder().encode(body).length} B`,
    folder,
    version: 1,
    space: s.space,
    origin: "cloud",
    favorite: false,
    connectionId: "drive",
    rootId: "root",
    updatedAt: "2026-09-28",
    metadata: "Markdown · UTF-8",
    snapshots: [{ version: 1, content: body, accent: 1 }],
    tags: [],
  };
  s.files.push(f);
  if (!s.folders.some((x) => x.path === folder && x.space === s.space))
    s.folders.push({ path: folder, space: s.space, connectionId: "drive" });
  s.knowledge.notes[id] = {
    ...noteRecord(s, f),
    mode,
    showDraft: mode !== "read",
  };
  return f;
}
export function seedKnowledge(s: State) {
  if (s.knowledge.seeded.includes(s.space)) return;
  const prefix = s.space === "personal" ? "k" : "kt";
  addNote(
    s,
    `${prefix}-guide`,
    "山野知识地图.md",
    "---\nproject: 山野\nstatus: 进行中\ntags: [项目/山野, 研究]\npriority: 0\nreviewed: false\naliases: [项目地图]\ncustom: 保留这个未知字段 # 示例注释\n---\n# 山野知识地图\n\n把灵感、资料和执行方法放在同一张地图上。\n\n## 项目资料\n\n- [[拍摄研究#光线|光线与拍摄研究]]\n- [[交付检查清单]]\n- [创作简报](山野计划%20·%20创作简报.pdf)\n\n## 视觉附件\n\n![[秋日山行 · 视觉参考.jpg]]\n\n## 下一步\n\n- [ ] 整理选题与参考资料\n- [x] 确认素材归属\n\n> 引用关联资料不会自动扩大 Agent 的授权范围。\n\n| 材料 | 状态 |\n| --- | --- |\n| 文档 | 待整理 |\n| 附件 | 已收集 |\n\n```text\n资料 → 引用 → 任务 → 验收\n```",
  );
  addNote(
    s,
    `${prefix}-research`,
    "拍摄研究.md",
    "---\nproject: 山野\nstatus: 草稿\ntags: [项目/山野/拍摄]\ndate: 2026-09-28\n---\n# 拍摄研究\n\n本笔记来自 [[山野知识地图]]。 #主题/光线\n\n## 光线\n\n黄昏的侧光能呈现山体层次。保留自然明暗，不增加过度锐化。\n\n## 声音\n\n记录风声、脚步与水声。交付检查清单可以辅助验收。\n\n![[林间片段 · 初剪.mp4]]",
  );
  addNote(
    s,
    `${prefix}-checklist`,
    "交付检查清单.md",
    "# 交付检查清单\n\n- [ ] 检查 [[拍摄研究]] 的素材引用\n- [ ] 固定交付版本\n- [ ] 确认接收对象与期限\n\n关联回到 [[山野知识地图]]。",
  );
  const sk = addNote(
    s,
    `${prefix}-skill`,
    "SKILL.md",
    "---\nname: 资料整理助手\ndescription: 将指定文档整理为有引用的摘要，修改前提交计划。\ntools: [files.read, artifacts.propose]\nlicense: 示例内容\ncompatibility: demo-only\n---\n# 资料整理助手\n\n## 使用方法\n\n读取本次明确选择的文档，整理成有来源的摘要。\n\n## 执行边界\n\n只引用当前授权范围。修改正文前提交差异计划，新产物进入收件箱。\n\n## 输出\n\n摘要、固定版本引用、待确认事项。",
    "Agent 资源 / 资料整理助手",
  );
  const reference = addNote(
    s,
    `${prefix}-skill-reference`,
    "review-guide.md",
    "# 资料审核说明\n\n检查来源、版本和引用位置。引用缺失时保留问题，不补造证据。",
    "Agent 资源 / 资料整理助手 / references",
  );
  const script = addNote(
    s,
    `${prefix}-skill-script`,
    "validate.py",
    '# Demonstration only. This script is displayed, never executed.\nprint("Review fixed references")',
    "Agent 资源 / 资料整理助手 / scripts",
  );
  const tmpl = addNote(
    s,
    `${prefix}-template`,
    "研究摘要模板.md",
    "# 研究摘要\n\n## 目标\n\n说明本次需要回答的问题。\n\n## 来源与证据\n\n列出固定版本引用。\n\n## 结论与待确认\n\n区分已知事实与建议。",
    "Agent 资源 / 模板",
  );
  s.knowledge.resources.push(
    {
      fileId: sk.id,
      kind: "skill",
      name: "资料整理助手",
      description: "将指定文档整理为有引用的摘要，修改前提交计划。",
      author: "Yeisme · 演示声明",
      source: "当前空间 / 示例资源包",
      tools: ["files.read", "artifacts.propose"],
      inputs: "已保存的文档与选定附件",
      outputs: "摘要 Markdown 或正文修改计划",
      extra: { license: "示例内容", compatibility: "demo-only" },
      missing: [],
      files: [
        { path: "SKILL.md", fileId: sk.id },
        { path: "references/review-guide.md", fileId: reference.id },
        { path: "scripts/validate.py", fileId: script.id },
        {
          path: "assets/visual-reference.jpg",
          fileId: s.space === "personal" ? "f2" : undefined,
        },
      ],
    },
    {
      fileId: tmpl.id,
      kind: "template",
      name: "研究摘要模板",
      description: "用同一结构整理目标、来源与结论。",
      author: "Yeisme · 演示声明",
      source: "当前空间 / 示例模板",
      tools: [],
      inputs: "文档引用",
      outputs: "研究摘要",
      extra: {},
      missing: [],
      files: [{ path: tmpl.name, fileId: tmpl.id }],
    },
  );
  s.collections.push({
    id: `${prefix}-pack`,
    name: "山野研究资料包",
    space: s.space,
    ids: [
      `${prefix}-guide`,
      `${prefix}-research`,
      sk.id,
      ...(s.space === "personal" ? ["f2"] : []),
    ],
    groups: {},
  });
  s.knowledge.seeded.push(s.space);
}
export function markdownTasks(body: string) {
  const ignored: { from: number; to: number }[] = [];
  markdownLanguage.parser.parse(body).iterate({
    enter(n) {
      if (["FencedCode", "CodeBlock", "InlineCode"].includes(n.name)) {
        ignored.push({ from: n.from, to: n.to });
        return false;
      }
    },
  });
  return Array.from(
    body.matchAll(/^(\s*(?:[-*+]|\d+\.)\s+\[)([ xX])(\])([^\n]*)/gm),
  )
    .filter((m) => !ignored.some((r) => m.index! >= r.from && m.index! < r.to))
    .map((m) => ({
      offset: m.index! + m[1].length,
      label: m[4].trim(),
      checked: m[2].toLowerCase() === "x",
    }));
}
export interface NoteLink {
  raw: string;
  target: string;
  anchor: string;
  label: string;
  embed: boolean;
  from: number;
  to: number;
}
export function parseNoteLinks(body: string): NoteLink[] {
  // Derive source ranges from the editor parser, including unclosed fences.
  const code: { from: number; to: number }[] = [];
  markdownLanguage.parser.parse(body).iterate({
    enter(node) {
      if (["FencedCode", "CodeBlock", "InlineCode"].includes(node.name)) {
        code.push({ from: node.from, to: node.to });
        return false;
      }
    },
  });
  let masked = body;
  for (const range of code.reverse())
    masked =
      masked.slice(0, range.from) +
      " ".repeat(range.to - range.from) +
      masked.slice(range.to);
  const result: NoteLink[] = [];
  const regex = /(!?)\[\[([^\]\n]+)\]\]|(!?)\[([^\]\n]*)\]\(([^)\s]+)\)/g;
  for (const match of masked.matchAll(regex)) {
    const wiki = match[2] !== undefined;
    const target = wiki ? match[2].split("|")[0] : match[5];
    if (/^(https?:|mailto:|data:|javascript:)/i.test(target)) continue;
    let decoded = target;
    try {
      decoded = decodeURIComponent(target);
    } catch {}
    const [path, ...anchor] = decoded.split("#");
    result.push({
      raw: body.slice(match.index!, match.index! + match[0].length),
      target: path,
      anchor: anchor.join("#"),
      label: wiki ? match[2].split("|")[1] || path : match[4],
      embed: !!(wiki ? match[1] : match[3]),
      from: match.index!,
      to: match.index! + match[0].length,
    });
  }
  return result;
}
const normalize = (v: string) =>
  v.replace(/\.md$/i, "").trim().normalize("NFC").toLowerCase();
export function resolveNoteLink(
  s: State,
  source: FileRecord,
  link: NoteLink,
): { file?: FileRecord; status: string } {
  const exact = s.files.filter(
    (f) =>
      f.space === source.space &&
      !f.trashed &&
      (normalize(f.name) === normalize(link.target) ||
        normalize(`${f.folder}/${f.name}`) === normalize(link.target)),
  );
  const accessible = exact.filter((f) => !fileUnavailable(s, f));
  if (!accessible.length) return { status: "目标缺失或无权访问" };
  if (accessible.length > 1) return { status: "存在同名文件，请指定目录" };
  const f = accessible[0];
  if (
    link.anchor &&
    !/^page=\d+$/.test(link.anchor) &&
    !noteBody(s, f.id)
      .split("\n")
      .some(
        (line) =>
          /^#{1,6}\s/.test(line) &&
          normalize(line.replace(/^#+\s*/, "")) === normalize(link.anchor),
      )
  )
    return { file: f, status: "标题不存在" };
  return { file: f, status: "可访问" };
}
export function linkedNotes(s: State, target: FileRecord) {
  return s.files
    .filter(
      (f) =>
        isMarkdown(f) && f.space === target.space && !fileUnavailable(s, f),
    )
    .flatMap((f) =>
      parseNoteLinks(noteBody(s, f.id)).flatMap((link) =>
        resolveNoteLink(s, f, link).file?.id === target.id
          ? [
              {
                file: f,
                link,
                snippet: noteBody(s, f.id).slice(
                  Math.max(0, link.from - 30),
                  link.to + 45,
                ),
              },
            ]
          : [],
      ),
    );
}
function saveNote(s: State, f: FileRecord) {
  const n = s.knowledge.notes[f.id];
  if (!n || n.composing) return;
  if (!s.online) {
    n.status = "offline";
    return;
  }
  if (!writableNote(s, f)) {
    n.status = "failed";
    s.notice = "写入权限或连接能力不可用，草稿已保留。";
    return;
  }
  if (n.base !== f.version) {
    n.status = "conflict";
    return;
  }
  if (n.fault) {
    n.status = "failed";
    n.fault = false;
    return;
  }
  if (n.draft === n.versions[n.base]) {
    n.dirty = false;
    n.status = "saved";
    return;
  }
  f.version++;
  n.versions[f.version] = n.draft;
  n.base = f.version;
  n.dirty = false;
  n.status = "saved";
  f.size = `${new TextEncoder().encode(n.draft).length} B`;
  f.updatedAt = "2026-09-28";
  f.snapshots = [
    ...(f.snapshots || []),
    { version: f.version, content: n.draft, accent: f.version },
  ];
  const resource = s.knowledge.resources.find((x) => x.fileId === f.id);
  if (resource) resource.checkedVersion = undefined;
  s.activity.unshift({
    id: `knowledge-${s.knowledge.counter++}`,
    space: s.space,
    fileId: f.id,
    version: f.version,
    text: `保存 Markdown 新版本 v${f.version}（演示）`,
  });
}
export function knowledgeReduce(
  s: State,
  a: KnowledgeAction,
): State | undefined {
  if (!a.type.startsWith("k-")) return;
  const k = s.knowledge;
  const file =
    "id" in a
      ? s.files.find((f) => f.id === a.id && f.space === s.space)
      : undefined;
  const note = file && k.notes[file.id];
  switch (a.type) {
    case "k-seed":
      seedKnowledge(s);
      break;
    case "k-list-demo": {
      if (s.actor !== "yeisme" || s.viewState === "forbidden") break;
      seedKnowledge(s);
      for (let i = 1; i <= 120; i++) {
        const id = `demo-note-${s.space}-${i}`;
        if (!s.files.some((f) => f.id === id))
          addNote(
            s,
            id,
            `分页文档 ${String(i).padStart(3, "0")}.md`,
            `# 分页文档 ${i}\n\n可重置的列表演示，不代表真实 Provider 性能。`,
            "知识库",
          );
      }
      const original = k.resources.find(
        (r) => s.files.find((f) => f.id === r.fileId)?.space === s.space,
      );
      if (original)
        for (let i = 1; i <= 60; i++) {
          const id = `demo-resource-${s.space}-${i}`,
            name = `分页资源 ${String(i).padStart(3, "0")}`;
          if (s.files.some((f) => f.id === id)) continue;
          addNote(
            s,
            id,
            `${name}.md`,
            `# ${name}\n\n只读资料模板演示。`,
            "Agent 资源",
          );
          k.resources.push({
            ...structuredClone(original),
            fileId: id,
            name,
            kind: "template",
            checkedVersion: undefined,
            files: [{ path: `${name}.md`, fileId: id }],
          });
        }
      s.notice = "已添加文档与资源分页样本；重置演示可移除。";
      break;
    }
    case "k-resource-new": {
      if (
        !a.name.trim() ||
        /[\\/]/.test(a.name) ||
        !s.online ||
        s.actor !== "yeisme" ||
        s.viewState === "forbidden"
      )
        break;
      const id = `resource-${s.space}-${k.counter++}`;
      const f = addNote(
        s,
        id,
        a.kind === "skill" ? "SKILL.md" : a.name + ".md",
        a.kind === "skill"
          ? patchProperty(
              patchProperty(
                patchProperty(
                  "# " + a.name + "\n\n## 用途\n\n描述资源的适用范围。",
                  "name",
                  a.name,
                ),
                "description",
                "新建资源，补充说明后检查。",
              ),
              "tools",
              ["files.read"],
            )
          : "# " + a.name + "\n\n## 用途\n\n描述资源的适用范围。",
        "Agent 资源 / " + a.name,
      );
      k.resources.push({
        fileId: f.id,
        kind: a.kind,
        name: a.name,
        description: "新建资源，补充说明后检查。",
        author: "当前身份 · 演示声明",
        source: "当前空间 / 新建",
        tools: ["files.read"],
        inputs: "待补充",
        outputs: "待补充",
        extra: {},
        missing: [],
        files: [{ path: f.name, fileId: f.id }],
      });
      break;
    }
    case "k-link-mention": {
      const target = s.files.find((x) => x.id === a.target);
      if (
        !file ||
        !target ||
        !writableNote(s, file) ||
        fileUnavailable(s, target) ||
        k.notes[file.id]?.dirty
      ) {
        s.notice = "来源、目标或草稿状态已变化，请重新检查。";
        break;
      }
      k.notes[file.id] ||= noteRecord(s, file);
      const title = target.name.replace(/\.md$/i, "");
      const n = k.notes[file.id];
      n.draft = n.draft.replace(title, "[[" + title + "]]");
      n.dirty = true;
      saveNote(s, file);
      break;
    }
    case "k-new": {
      if (!s.online || s.viewState === "forbidden" || s.actor !== "yeisme") {
        s.notice = "当前无法创建文档，请恢复网络或权限。";
        break;
      }
      const name = a.name.trim();
      if (!name || /[\\/]/.test(name)) {
        s.notice = "请填写有效文件名。";
        break;
      }
      const title = /\.md$/i.test(name) ? name : `${name}.md`;
      if (
        s.files.some(
          (f) =>
            f.space === s.space &&
            !f.trashed &&
            f.name === title &&
            f.folder === "知识库",
        )
      ) {
        s.notice = "同名文档已存在，请换一个名称。";
        break;
      }
      const source = s.files.find((f) => f.id === a.copyId);
      if (source && fileUnavailable(s, source)) {
        s.notice = "来源不可访问，未创建副本。";
        break;
      }
      const id = `note-${s.space}-${k.counter++}`;
      addNote(
        s,
        id,
        title,
        a.body ??
          (a.copyId
            ? k.notes[a.copyId]?.draft || noteBody(s, a.copyId)
            : `# ${name.replace(/\.md$/i, "")}\n\n`),
        "知识库",
        k.options.defaultMode,
      );
      k.graphFile = id;
      s.notice = "已创建演示文档，可开始编辑。";
      break;
    }
    case "k-open":
      if (file && !fileUnavailable(s, file)) {
        k.notes[file.id] ||= noteRecord(s, file);
        if (a.mode) {
          k.notes[file.id].mode = a.mode;
          k.notes[file.id].showDraft = a.mode !== "read";
        }
      }
      break;
    case "k-restore-view":
      if (note && file && !fileUnavailable(s, file)) {
        note.mode = a.mode;
        note.showDraft = a.showDraft;
      }
      break;
    case "k-inspect":
      if (note) {
        note.mode = "read";
        note.showDraft = false;
      }
      break;
    case "k-mode":
      if (note) {
        note.mode = a.mode;
        if (a.mode !== "read") note.showDraft = true;
      }
      break;
    case "k-edit":
      if (note && file && writableNote(s, file)) {
        note.draft = a.body;
        note.dirty = a.body !== note.versions[note.base];
        note.status = note.dirty ? "dirty" : "saved";
        if (a.selection) note.selection = a.selection;
      }
      break;
    case "k-compose":
      if (note) note.composing = a.value;
      break;
    case "k-save-start":
      if (note && note.dirty && !note.composing)
        note.status = s.online ? "saving" : "offline";
      break;
    case "k-save":
      if (file) saveNote(s, file);
      break;
    case "k-reload":
      if (file && note && !fileUnavailable(s, file)) {
        note.draft = noteBody(s, file.id, file.version);
        note.base = file.version;
        note.dirty = false;
        note.status = "saved";
      }
      break;
    case "k-remote":
      if (file && note && !fileUnavailable(s, file)) {
        file.version++;
        const body =
          noteBody(s, file.id, file.version - 1) +
          "\n\n> 另一设备补充了这一段（演示）。";
        note.versions[file.version] = body;
        file.snapshots?.push({
          version: file.version,
          content: body,
          accent: file.version,
        });
        if (note.dirty) note.status = "conflict";
      }
      break;
    case "k-fault":
      if (note) {
        note.fault = true;
        s.notice = "下一次保存将模拟失败，草稿会保留。";
      }
      break;
    case "k-options":
      k.options = { ...k.options, ...a.patch };
      s.settingsNotice = "已保存 · 当前演示会话";
      break;
    case "k-graph":
      k.graphFile = a.id;
      if (a.kind) k.graphKind = a.kind;
      s.page = "knowledge-graph";
      break;
    case "k-skill-fields": {
      const r = k.resources.find((x) => x.fileId === a.id);
      if (!r || !file || !writableNote(s, file)) break;
      if (!a.name.trim() || !a.description.trim()) {
        s.notice = "名称和说明不能为空。";
        break;
      }
      if (!note || note.composing) break;
      try {
        let body = patchProperty(note.draft, "name", a.name.trim());
        body = patchProperty(body, "description", a.description.trim());
        body = patchProperty(body, "tools", a.tools);
        note.draft = body;
        note.dirty = true;
        note.showDraft = true;
        note.status = "dirty";
        r.checkedVersion = undefined;
        saveNote(s, file);
        s.notice = note.dirty
          ? "字段写入草稿，保存尚未完成。"
          : "同一文首属性已保存，未知字段保留；需要重新检查。";
      } catch (e) {
        s.notice = (e as Error).message;
      }
      syncResourceFields(s);
      break;
    }
    case "k-check": {
      syncResourceFields(s);
      const r = k.resources.find((x) => x.fileId === a.id);
      if (!r || !file || fileUnavailable(s, file)) break;
      r.missing = r.tools.filter(
        (t) => !["files.read", "artifacts.propose"].includes(t),
      );
      r.checkedVersion =
        !!note &&
        readProperties(note.draft).valid &&
        !r.missing.length &&
        !note?.dirty &&
        !!r.name.trim() &&
        !!r.description.trim()
          ? file.version
          : undefined;
      r.checkedAt = "2026-09-28 · 本轮演示";
      s.notice = r.checkedVersion
        ? "示例能力检查通过；未安装或执行。"
        : "缺少依赖或存在未保存内容，暂不能用于任务。";
      break;
    }
    case "k-skill-select": {
      const r = k.resources.find((x) => x.fileId === a.id);
      if (
        !r ||
        !file ||
        fileUnavailable(s, file) ||
        r.checkedVersion !== file.version ||
        note?.dirty
      ) {
        s.notice = "请先保存并检查当前 Skill 版本。";
        break;
      }
      k.selectedSkills[s.activeConversation] = {
        id: file.id,
        version: file.version,
      };
      s.agentOpen = true;
      s.notice = "已选用固定 Skill 版本；尚未执行任务。";
      break;
    }
    case "k-skill-remove":
      delete k.selectedSkills[s.activeConversation];
      break;
    case "k-attach": {
      if (!file || !writableNote(s, file)) break;
      const id = `attachment-${k.counter++}`,
        taskId = `DB-${String(s.nextTask++).padStart(3, "0")}`;
      k.uploads.push({
        id,
        taskId,
        noteId: file.id,
        name: a.name,
        kind: a.kind,
        size: a.size,
        status: a.fail || !s.online ? "failed" : "pending",
      });
      s.tasks.unshift({
        id: taskId,
        kind: "upload",
        title: `附件上传：${a.name}`,
        space: s.space,
        status: a.fail || !s.online ? "paused" : "running",
        revision: 1,
        items: [],
        source: "本设备 · 演示",
        target: k.options.attachmentFolder,
        note: "上传完成后插入有效引用；恢复沿用原任务。",
        runs: 1,
        injectPartial: false,
      });
      break;
    }
    case "k-upload-recover": {
      const u = k.uploads.find((x) => x.id === a.id);
      if (u && s.online && u.status === "failed") {
        u.status = "pending";
        const t = s.tasks.find((t) => t.id === u.taskId);
        if (t) t.status = "running";
      }
      break;
    }
    case "k-upload-finish": {
      const u = k.uploads.find((x) => x.id === a.id);
      if (!u || u.status !== "pending") break;
      const owner = s.files.find((f) => f.id === u.noteId);
      const t = s.tasks.find((t) => t.id === u.taskId);
      if (!owner || !writableNote(s, owner) || !s.online) {
        u.status = "failed";
        if (t) t.status = "paused";
        break;
      }
      if (
        s.files.some(
          (f) =>
            f.space === owner.space &&
            f.name === u.name &&
            f.folder === k.options.attachmentFolder &&
            !f.trashed,
        )
      ) {
        u.status = "failed";
        if (t) {
          t.status = "paused";
          t.note = "同名文件冲突：请重命名待上传文件或选择已有附件。";
        }
        break;
      }
      const id = `asset-${k.counter++}`;
      const asset: FileRecord = {
        ...owner,
        id,
        name: u.name,
        kind: u.kind,
        size: u.size,
        folder: k.options.attachmentFolder,
        version: 1,
        snapshots: [],
        tags: [],
        metadata: "附件 · 模拟上传",
        inputRefs: undefined,
      };
      s.files.push(asset);
      u.fileId = id;
      u.status = "done";
      const n = k.notes[owner.id];
      if (n) {
        n.draft += `\n\n![[${asset.name}]]`;
        n.dirty = true;
        n.status = "dirty";
      }
      if (t) {
        t.status = "done";
        t.items = [
          {
            fileId: id,
            version: 1,
            before: "本设备",
            after: asset.folder,
            status: "done",
            outputId: id,
            outputVersion: 1,
          },
        ];
      }
      break;
    }
    case "k-rename-plan": {
      if (
        !file ||
        !writableNote(s, file) ||
        !a.name.trim() ||
        /[\\/]/.test(a.name)
      )
        break;
      const refs = linkedNotes(s, file)
        .map(({ file: src }) => src)
        .filter((f, i, all) => all.findIndex((x) => x.id === f.id) === i)
        .map((src) => {
          const before = noteBody(s, src.id);
          let after = before;
          for (const l of parseNoteLinks(before)
            .filter((l) => resolveNoteLink(s, src, l).file?.id === file.id)
            .reverse()) {
            const path = a.folder.trim() + "/" + a.name.trim();
            const raw = l.raw.includes("[[")
              ? `${l.embed ? "!" : ""}[[${path.replace(/\.md$/i, "")}${l.anchor ? "#" + l.anchor : ""}${l.label ? "|" + l.label : ""}]]`
              : `${l.embed ? "!" : ""}[${l.label}](${encodeURI(path)}${l.anchor ? "#" + encodeURIComponent(l.anchor) : ""})`;
            after = after.slice(0, l.from) + raw + after.slice(l.to);
          }
          return { id: src.id, version: src.version, before, after };
        });
      k.renamePlan = {
        id: file.id,
        name: a.name.trim(),
        folder: a.folder.trim() || file.folder,
        version: file.version,
        refs,
      };
      break;
    }
    case "k-rename-cancel":
      delete k.renamePlan;
      break;
    case "k-rename-apply": {
      const p = k.renamePlan;
      if (!p) break;
      const f = s.files.find((x) => x.id === p.id);
      if (
        !s.online ||
        !f ||
        !writableNote(s, f) ||
        f.version !== p.version ||
        p.refs.some((r) => {
          const x = s.files.find((f) => f.id === r.id);
          return (
            !x ||
            !writableNote(s, x) ||
            x.version !== r.version ||
            k.notes[x.id]?.dirty ||
            k.notes[x.id]?.fault
          );
        })
      ) {
        s.notice = "计划过期、存在草稿或权限变化，请重新预览。";
        break;
      }
      if (
        s.files.some(
          (x) =>
            x.id !== f.id &&
            x.space === f.space &&
            !x.trashed &&
            x.name === p.name &&
            x.folder === p.folder,
        )
      ) {
        s.notice = "目标位置存在同名文件，未修改。";
        break;
      }
      f.name = p.name;
      f.folder = p.folder;
      p.refs.forEach((r) => {
        const src = s.files.find((f) => f.id === r.id)!;
        k.notes[src.id] ||= noteRecord(s, src);
        k.notes[src.id].base = src.version;
        k.notes[src.id].draft = r.after;
        k.notes[src.id].dirty = true;
        saveNote(s, src);
      });
      delete k.renamePlan;
      s.notice = "已更新名称、位置与确认的引用（演示）。";
      break;
    }
    case "k-task": {
      const selected = k.selectedSkills[s.activeConversation];
      const resource = k.resources.find((x) => x.fileId === selected?.id);
      const sf = s.files.find((f) => f.id === selected?.id);
      const refs = contextRefs(s).flatMap((x) => x.refs);
      if (
        !selected ||
        !resource ||
        !sf ||
        selected.version !== sf.version ||
        resource.checkedVersion !== sf.version ||
        fileUnavailable(s, sf) ||
        k.notes[sf.id]?.dirty ||
        !s.online ||
        !refs.length ||
        refs.some((r) =>
          fileUnavailable(
            s,
            s.files.find((f) => f.id === r.id),
            r.version,
          ),
        )
      ) {
        s.notice = "请检查 Skill、固定资料范围、网络和当前权限。";
        break;
      }
      const target =
        a.target ||
        refs.find((r) => isMarkdown(s.files.find((f) => f.id === r.id)))?.id;
      const targetFile = s.files.find((f) => f.id === target);
      if (
        a.mode === "edit" &&
        (!targetFile || !writableNote(s, targetFile) || k.notes[target!]?.dirty)
      ) {
        s.notice = "请先保存目标文档并确认写入权限。";
        break;
      }
      const id = `DB-${String(s.nextTask++).padStart(3, "0")}`;
      const body =
        a.mode === "edit"
          ? noteBody(s, target!) +
            `\n\n## Agent 整理建议\n\n${a.goal || "整理资料"}\n\n此段来自已确认的演示修改计划。`
          : `# ${a.goal || "资料摘要"}\n\n仅整理本次明确选择的资料。\n\n${refs.map((r) => "- " + s.files.find((f) => f.id === r.id)?.name + " · 固定 v" + r.version).join("\n")}\n\n## 待确认\n\n请核对引用与内容后接收。`;
      k.taskPlans[id] = {
        target: a.mode === "edit" ? target : undefined,
        body,
        sections: contextRefs(s)
          .filter((r) => r.excerpt)
          .flatMap((r) =>
            r.refs.map((ref) => ({
              ...ref,
              anchor: r.anchor,
              excerpt: r.excerpt!,
            })),
          ),
        skillId: selected.id,
        skillVersion: selected.version,
        refs,
      };
      const t: Task = {
        id,
        title: a.mode === "edit" ? "文档修改计划" : "资料摘要与回存",
        kind: "agent",
        space: s.space,
        status: "approval",
        revision: 1,
        source: resource.name + " · v" + selected.version,
        target: a.mode === "edit" ? targetFile!.name : "收件箱",
        note: "资源已冻结；检查输入和修改差异后确认。",
        runs: 0,
        injectPartial: false,
        items: refs.map((r) => ({
          fileId: r.id,
          version: r.version,
          before: "固定输入",
          after: "等待确认",
          status: "pending",
        })),
      };
      s.tasks.unshift(t);
      s.notice = "已生成固定范围计划，尚未修改文件。";
      break;
    }
    case "k-task-refresh": {
      const p = k.taskPlans[a.id],
        t = s.tasks.find((t) => t.id === a.id);
      if (
        !p ||
        !t ||
        t.space !== s.space ||
        !["stale", "blocked"].includes(t.status)
      )
        break;
      const sf = s.files.find((f) => f.id === p.skillId),
        r = k.resources.find((r) => r.fileId === p.skillId);
      const target = s.files.find((f) => f.id === p.target);
      if (
        !s.online ||
        !sf ||
        fileUnavailable(s, sf) ||
        r?.checkedVersion !== sf.version ||
        k.notes[sf.id]?.dirty ||
        p.refs.some((ref) =>
          fileUnavailable(
            s,
            s.files.find((f) => f.id === ref.id),
          ),
        ) ||
        (p.target &&
          (!target || !writableNote(s, target) || k.notes[target.id]?.dirty))
      ) {
        t.status = "blocked";
        s.notice = "当前范围仍不可用，请检查权限、版本或草稿。";
        break;
      }
      if (
        p.sections?.some(
          (section) =>
            s.files.find((f) => f.id === section.id)?.version !==
            section.version,
        )
      ) {
        t.status = "blocked";
        s.notice = "章节或选区已变化，请重新选择引用后生成计划。";
        break;
      }
      p.refs = p.refs.map((ref) => ({
        ...ref,
        version: s.files.find((f) => f.id === ref.id)!.version,
      }));
      p.skillVersion = sf.version;
      if (target)
        p.body =
          noteBody(s, target.id) +
          "\n\n## Agent 整理建议\n\n" +
          (p.body?.split("## Agent 整理建议").at(-1)?.trim() ||
            "整理当前资料。");
      t.revisions ||= [];
      t.revisions.push({
        revision: t.revision,
        summary: "原固定范围与旧确认已失效",
      });
      t.revision++;
      t.confirmed = undefined;
      t.status = "approval";
      t.items = p.refs.map((ref) => ({
        fileId: ref.id,
        version: ref.version,
        before: "固定输入",
        after: "等待重新确认",
        status: "pending",
      }));
      s.notice = "已重新预览同一任务，需确认新的计划修订。";
      break;
    }
    case "k-task-apply": {
      const p = k.taskPlans[a.id],
        t = s.tasks.find((t) => t.id === a.id);
      if (!p || !t || t.space !== s.space || t.status !== "approval") {
        s.notice = "原任务已处理或不可确认，未重复执行。";
        break;
      }
      const sf = s.files.find((f) => f.id === p.skillId);
      const target = s.files.find((f) => f.id === p.target);
      if (
        (p.target && !target) ||
        !s.online ||
        fileUnavailable(s, sf, p.skillVersion) ||
        k.resources.find((r) => r.fileId === p.skillId)?.checkedVersion !==
          p.skillVersion ||
        p.refs.some((r) =>
          fileUnavailable(
            s,
            s.files.find((f) => f.id === r.id),
            r.version,
          ),
        ) ||
        (target &&
          (!writableNote(s, target) ||
            target.version !==
              p.refs.find((r) => r.id === target.id)?.version ||
            k.notes[target.id]?.dirty))
      ) {
        t.status = "stale";
        s.notice = "计划已过期或权限变化，当前文件保留。";
        break;
      }
      let output: FileRecord;
      if (target) {
        k.notes[target.id] ||= noteRecord(s, target);
        const n = k.notes[target.id];
        n.versions[target.version] = noteBody(s, target.id, target.version);
        n.base = target.version;
        n.draft = p.body!;
        n.dirty = true;
        saveNote(s, target);
        if (n.status !== "saved") {
          t.status = "blocked";
          break;
        }
        output = target;
      } else {
        output = addNote(
          s,
          `knowledge-output-${k.counter++}`,
          "资料摘要.md",
          p.body!,
          "待整理",
        );
        output.inbox = true;
        output.origin = "agent";
        output.inputRefs = p.refs;
      }
      t.items.forEach((i) => {
        i.status = "done";
        i.outputId = output.id;
        i.outputVersion = output.version;
        i.after = output.folder;
      });
      t.status = "done";
      t.runs = 1;
      t.note = `使用 Skill 固定 v${p.skillVersion}，输入与产物保留原任务记录。`;
      s.notice = target
        ? "已确认并生成新版本。"
        : "产物已进入收件箱，等待验收。";
      break;
    }
  }
  return s;
}

export function syncResourceFields(s: State) {
  for (const r of s.knowledge.resources.filter((r) => r.kind === "skill")) {
    const n = s.knowledge.notes[r.fileId];
    if (!n) continue;
    const p = readProperties(n.draft);
    if (!p.valid) {
      r.checkedVersion = undefined;
      continue;
    }
    const values = Object.fromEntries(
      p.fields.filter((f) => !f.complex).map((f) => [f.key, f.value]),
    );
    r.name = typeof values.name === "string" ? values.name : "";
    r.description =
      typeof values.description === "string" ? values.description : "";
    r.tools = Array.isArray(values.tools) ? values.tools : [];
    r.extra = Object.fromEntries(
      p.fields
        .filter((f) => !["name", "description", "tools"].includes(f.key))
        .map((f) => [
          f.key,
          f.complex
            ? "复杂值，见源码"
            : Array.isArray(f.value)
              ? f.value.join(", ")
              : String(f.value ?? ""),
        ]),
    );
    if (n.dirty) r.checkedVersion = undefined;
  }
}
