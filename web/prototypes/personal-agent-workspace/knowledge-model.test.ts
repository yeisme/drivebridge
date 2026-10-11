import { describe, expect, it } from "vitest";
import { initialState, reducer } from "./model";
import {
  markdownTasks,
  linkedNotes,
  noteBody,
  parseNoteLinks,
  resolveNoteLink,
} from "./knowledge-model";
import type { Action, State } from "./model";
const init = () => reducer(initialState(), { type: "k-seed" });
const edit = (s: State, body: string, id = "k-guide") =>
  reducer(s, { type: "k-edit", id, body });
const task = (mode: "output" | "edit" = "output") => {
  let s = init();
  s = reducer(s, { type: "k-check", id: "k-skill" });
  s = reducer(s, { type: "k-skill-select", id: "k-skill" });
  s = reducer(s, { type: "context", ids: ["k-guide", "f2"] });
  return reducer(s, {
    type: "k-task",
    mode,
    target: "k-guide",
    goal: "有来源的摘要",
  });
};
describe("v6 document state and knowledge authority", () => {
  it("shares file identity and creates seed resources once", () => {
    const s = init();
    expect(reducer(s, { type: "k-seed" }).files).toEqual(s.files);
    expect(s.knowledge.resources[0].files).toHaveLength(4);
  });
  it("saves immutable versions and ignores repeated equal saves", () => {
    let s = edit(init(), "# New body");
    s = reducer(s, { type: "k-save", id: "k-guide" });
    expect(s.files.find((f) => f.id === "k-guide")?.version).toBe(2);
    expect(noteBody(s, "k-guide", 1)).toContain("山野知识地图");
    expect(noteBody(s, "k-guide", 2)).toBe("# New body");
    expect(reducer(s, { type: "k-save", id: "k-guide" }).files).toEqual(
      s.files,
    );
  });
  it("does not save IME composition or lose offline draft", () => {
    let s = edit(init(), "输入中");
    s = reducer(s, { type: "k-compose", id: "k-guide", value: true });
    s = reducer(s, { type: "k-save", id: "k-guide" });
    expect(s.knowledge.notes["k-guide"].base).toBe(1);
    s = reducer(s, { type: "k-compose", id: "k-guide", value: false });
    s = reducer(s, { type: "network", online: false });
    s = reducer(s, { type: "k-save", id: "k-guide" });
    expect(s.knowledge.notes["k-guide"].status).toBe("offline");
    expect(s.knowledge.notes["k-guide"].draft).toBe("输入中");
  });
  it("preserves conflict draft and never replaces remote content", () => {
    let s = edit(init(), "我的草稿");
    s = reducer(s, { type: "k-remote", id: "k-guide" });
    s = reducer(s, { type: "k-save", id: "k-guide" });
    expect(s.knowledge.notes["k-guide"].status).toBe("conflict");
    expect(s.knowledge.notes["k-guide"].draft).toBe("我的草稿");
    expect(noteBody(s, "k-guide", 2)).toContain("另一设备");
  });
  it("recovers a failed save without creating a second revision", () => {
    let s = edit(init(), "可恢复");
    s = reducer(s, { type: "k-fault", id: "k-guide" });
    s = reducer(s, { type: "k-save", id: "k-guide" });
    expect(s.knowledge.notes["k-guide"].status).toBe("failed");
    s = reducer(s, { type: "k-save", id: "k-guide" });
    expect(s.knowledge.notes["k-guide"].base).toBe(2);
  });
  it("rechecks access and verified external write capability", () => {
    let s = edit(init(), "不可写入");
    s.viewState = "forbidden";
    s = reducer(s, { type: "k-save", id: "k-guide" });
    expect(s.knowledge.notes["k-guide"].base).toBe(1);
    expect(s.knowledge.notes["k-guide"].dirty).toBe(true);
  });
  it("parses links and embeds while excluding inline and fenced code", () => {
    const links = parseNoteLinks(
      "[[A#标题|名称]] ![[image.png]] [B](B.md) `[[ignore]]`\n```\n[[ignore]]\n```",
    );
    expect(links.map((x) => x.target)).toEqual(["A", "image.png", "B.md"]);
    expect(links[0].anchor).toBe("标题");
    expect(links[1].embed).toBe(true);
  });
  it("detects ambiguity, missing headings and denied targets", () => {
    const s = init(),
      f = s.files.find((f) => f.id === "k-guide")!;
    const link = parseNoteLinks("[[拍摄研究#光线]]")[0];
    expect(resolveNoteLink(s, f, link).status).toBe("可访问");
    expect(resolveNoteLink(s, f, { ...link, anchor: "不存在" }).status).toBe(
      "标题不存在",
    );
    s.files.push({
      ...s.files.find((f) => f.id === "k-research")!,
      id: "duplicate",
      folder: "其他",
    });
    expect(resolveNoteLink(s, f, link).status).toContain("同名");
    s.viewState = "forbidden";
    expect(resolveNoteLink(s, f, link).file).toBeUndefined();
  });
  it("finds attachment backlinks only within visible scanned content", () => {
    const s = init();
    expect(
      linkedNotes(s, s.files.find((f) => f.id === "f2")!).map((x) => x.file.id),
    ).toContain("k-guide");
    s.viewState = "forbidden";
    expect(linkedNotes(s, s.files.find((f) => f.id === "f2")!)).toHaveLength(0);
  });
  it("rename requires a plan and updates acknowledged source versions", () => {
    let s = init();
    s = reducer(s, {
      type: "k-rename-plan",
      id: "k-research",
      name: "光线研究.md",
      folder: "知识库",
    });
    expect(s.files.find((f) => f.id === "k-research")?.name).toBe(
      "拍摄研究.md",
    );
    expect(s.knowledge.renamePlan?.refs.length).toBe(2);
    s = reducer(s, { type: "k-rename-apply" });
    expect(noteBody(s, "k-guide")).toContain(
      "[[知识库/光线研究#光线|光线与拍摄研究]]",
    );
    expect(noteBody(s, "k-guide", 1)).toContain("拍摄研究");
  });
  it("stale reference rewrite plan cannot overwrite a new draft", () => {
    let s = init();
    s = reducer(s, {
      type: "k-rename-plan",
      id: "k-research",
      name: "新研究.md",
      folder: "知识库",
    });
    s = edit(s, "保留草稿");
    s = reducer(s, { type: "k-rename-apply" });
    expect(s.files.find((f) => f.id === "k-research")?.name).toBe(
      "拍摄研究.md",
    );
    expect(s.knowledge.notes["k-guide"].draft).toBe("保留草稿");
  });
  it("attachment failures keep original task and insert only after success", () => {
    let s = init();
    const original = noteBody(s, "k-guide");
    s = reducer(s, {
      type: "k-attach",
      id: "k-guide",
      name: "新附件.png",
      kind: "image",
      size: "1 MB",
      fail: true,
    });
    const u = s.knowledge.uploads[0];
    expect(s.knowledge.notes["k-guide"].draft).toBe(original);
    s = reducer(s, { type: "k-upload-recover", id: u.id });
    s = reducer(s, { type: "k-upload-finish", id: u.id });
    expect(s.knowledge.uploads[0].taskId).toBe(u.taskId);
    expect(s.knowledge.notes["k-guide"].draft).toContain("![[新附件.png]]");
    const count = s.files.length;
    s = reducer(s, { type: "k-upload-finish", id: u.id });
    expect(s.files).toHaveLength(count);
  });
  it("late upload completion after revocation does not insert content", () => {
    let s = init();
    s = reducer(s, {
      type: "k-attach",
      id: "k-guide",
      name: "新附件.png",
      kind: "image",
      size: "1 MB",
    });
    const u = s.knowledge.uploads[0];
    s.viewState = "forbidden";
    s = reducer(s, { type: "k-upload-finish", id: u.id });
    expect(s.knowledge.uploads[0].status).toBe("failed");
    expect(s.files.some((f) => f.name === "新附件.png")).toBe(false);
  });
  it("skill checks do not install or execute and edits invalidate checks", () => {
    let s = init();
    s = reducer(s, { type: "k-check", id: "k-skill" });
    expect(s.knowledge.resources[0].checkedVersion).toBe(1);
    expect(s.tasks).toHaveLength(1);
    s = edit(s, "# changed", "k-skill");
    s = reducer(s, { type: "k-save", id: "k-skill" });
    expect(s.knowledge.resources[0].checkedVersion).toBeUndefined();
  });
  it("resource metadata preserves unknown fields and missing dependencies block use", () => {
    let s = init();
    s = reducer(s, {
      type: "k-skill-fields",
      id: "k-skill",
      name: "资源",
      description: "说明",
      tools: ["unknown.tool"],
    });
    s = reducer(s, { type: "k-check", id: "k-skill" });
    s = reducer(s, { type: "k-skill-select", id: "k-skill" });
    expect(s.knowledge.resources[0].extra.license).toBe("示例内容");
    expect(s.knowledge.resources[0].missing).toEqual(["unknown.tool"]);
    expect(s.knowledge.selectedSkills[s.activeConversation]).toBeUndefined();
  });
  it("freezes input and skill references then generates one reviewable output", () => {
    let s = task();
    const t = s.tasks[0];
    expect(t.status).toBe("approval");
    expect(s.knowledge.taskPlans[t.id].refs.map((r) => r.id)).toEqual([
      "k-guide",
      "f2",
    ]);
    expect(
      s.knowledge.taskPlans[t.id].refs.some((r) => r.id === "k-research"),
    ).toBe(false);
    s = reducer(s, { type: "k-task-apply", id: t.id });
    const output = s.tasks[0].items[0].outputId!;
    expect(s.files.find((f) => f.id === output)?.inbox).toBe(true);
    expect(s.files.find((f) => f.id === output)?.inputRefs).toHaveLength(2);
    const count = s.files.length;
    s = reducer(s, { type: "k-task-apply", id: t.id });
    expect(s.files).toHaveLength(count);
  });
  it("previewed edit plan produces a new version only after confirmation", () => {
    let s = task("edit");
    const t = s.tasks[0];
    expect(s.files.find((f) => f.id === "k-guide")?.version).toBe(1);
    s = reducer(s, { type: "k-task-apply", id: t.id });
    expect(s.files.find((f) => f.id === "k-guide")?.version).toBe(2);
    expect(noteBody(s, "k-guide", 1)).not.toContain("Agent 整理建议");
  });
  it("late confirmation after draft, version, permission or space changes is blocked", () => {
    for (const change of ["draft", "version", "permission", "space"]) {
      let s = task("edit");
      const id = s.tasks[0].id;
      if (change === "draft") s = edit(s, "继续编辑");
      if (change === "version")
        s = reducer(s, { type: "k-remote", id: "k-guide" });
      if (change === "permission") s.viewState = "forbidden";
      if (change === "space") s = reducer(s, { type: "space", space: "team" });
      s = reducer(s, { type: "k-task-apply", id });
      expect(s.tasks.find((t) => t.id === id)?.runs).toBe(0);
    }
  });
  it("generic confirmation cannot bypass the resource plan gate", () => {
    let s = task("edit");
    const id = s.tasks[0].id;
    s = reducer(s, { type: "k-remote", id: "k-guide" });
    s = reducer(s, {
      type: "confirm",
      id,
      revision: 1,
      contextRevision: s.contextRevision,
    });
    expect(s.tasks[0].runs).toBe(0);
  });
  it("reset discards docs and preferences never persist document content", () => {
    let s = edit(init(), "private draft");
    expect(JSON.stringify(s.preferences)).not.toContain("private draft");
    s = reducer(s, { type: "reset" });
    expect(s.knowledge.notes).toEqual({});
    expect(s.files.some((f) => f.id === "k-guide")).toBe(false);
  });
});

it("v6 excludes unclosed code fences and whitespace-distinct filenames", () => {
  expect(parseNoteLinks("```md\n[[not-a-link]]")).toHaveLength(0);
  const s = init(),
    f = s.files.find((f) => f.id === "k-guide")!;
  s.files.push(
    { ...f, id: "spaced", name: "A B.md" },
    { ...f, id: "compact", name: "AB.md" },
  );
  expect(resolveNoteLink(s, f, parseNoteLinks("[[A B]]")[0]).file?.id).toBe(
    "spaced",
  );
});
it("v6 cancelled attachment and unknown Agent result cannot duplicate work", () => {
  let s = init();
  s = reducer(s, {
    type: "k-attach",
    id: "k-guide",
    name: "cancel.png",
    kind: "image",
    size: "1 MB",
  });
  const u = s.knowledge.uploads[0];
  s = reducer(s, { type: "cancel", id: u.taskId });
  s = reducer(s, { type: "k-upload-finish", id: u.id });
  expect(s.files.some((f) => f.name === "cancel.png")).toBe(false);
  s = task();
  const id = s.tasks[0].id;
  s = reducer(s, { type: "k-task-apply", id });
  const count = s.files.length;
  s.tasks[0].status = "unknown";
  s = reducer(s, { type: "reconcile", id });
  expect(s.tasks[0].status).toBe("done");
  s = reducer(s, { type: "recover", id });
  expect(s.files).toHaveLength(count);
  expect(s.tasks[0].runs).toBe(1);
});
it("v6 changed plan invalidates an old confirmation while preserving the operation", () => {
  let s = task("edit");
  const id = s.tasks[0].id;
  s = reducer(s, { type: "k-remote", id: "k-guide" });
  s = reducer(s, { type: "k-task-apply", id });
  s = reducer(s, { type: "refresh-plan", id });
  expect(s.tasks[0].id).toBe(id);
  expect(s.tasks[0].revision).toBe(2);
  s = reducer(s, {
    type: "confirm",
    id,
    revision: 1,
    contextRevision: s.contextRevision,
  });
  expect(s.tasks[0].runs).toBe(0);
  s = reducer(s, {
    type: "confirm",
    id,
    revision: 2,
    contextRevision: s.contextRevision,
  });
  expect(s.tasks[0].runs).toBe(1);
});

it("v6 task markers preserve exact source offsets and skip code examples", () => {
  const body = "- [ ] task\n\n```md\n- [ ] example\n```\n\n1. [x] done";
  const tasks = markdownTasks(body);
  expect(tasks.map((t) => t.label)).toEqual(["task", "done"]);
  expect(body[tasks[0].offset]).toBe(" ");
  expect(body[tasks[1].offset]).toBe("x");
});

it("list fixtures preserve identities and stay idempotent in their space", () => {
  const base = init();
  const before = base.files.map((f) => f.id);
  const next = reducer(base, { type: "k-list-demo" });
  expect(
    next.files.filter((f) => f.id.startsWith("demo-note-personal-")),
  ).toHaveLength(120);
  expect(
    next.files.filter((f) => f.id.startsWith("demo-resource-personal-")),
  ).toHaveLength(60);
  expect(before.every((id) => next.files.some((f) => f.id === id))).toBe(true);
  expect(reducer(next, { type: "k-list-demo" }).files).toHaveLength(
    next.files.length,
  );
  expect(
    next.files
      .filter((f) => f.id.startsWith("demo-"))
      .every((f) => f.space === "personal"),
  ).toBe(true);
});
