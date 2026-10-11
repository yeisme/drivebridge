import { describe, it, expect } from "vitest";
import { initialState, reducer } from "./model";
import { safePreferences } from "./cloud-model";
import {
  agentContext,
  agentReference,
  createPreviewSession,
  fileRef,
  parsePreviewFragment,
  previewFragment,
  previewUnavailable,
  taskPreviewScope,
} from "./preview-model";
describe("v5 scoped preview sessions", () => {
  it("freezes only two inbox references and keeps origin selection", () => {
    const s = initialState();
    s.page = "inbox";
    s.selected = ["f1"];
    const files = s.files.filter((f) => f.inbox && f.space === s.space);
    expect(files).toHaveLength(2);
    const session = createPreviewSession(s, { id: files[0].id }, 125);
    expect(session.scope.refs.map((r) => r.id)).toEqual(files.map((f) => f.id));
    s.files.push({ ...files[0], id: "later" });
    expect(session.scope.refs).toHaveLength(2);
    expect(s.selected).toEqual(["f1"]);
    expect(session.returnScroll).toBe(125);
  });
  it("preserves distinct fixed versions and external revisions within explicit scopes", () => {
    const s = initialState();
    const f = s.files[0];
    const scope = {
      label: "固定版本对照",
      refs: [fileRef(f, 1), fileRef(f, 3)],
    };
    const session = createPreviewSession(s, { id: f.id, version: 1, scope });
    expect(session.scope.refs.map((r) => r.version)).toEqual([1, 3]);
    const ext = s.files.find((f) => f.externalRevision)!;
    const old = { ...fileRef(ext), externalRevision: "old-revision" };
    const external = createPreviewSession(s, {
      id: ext.id,
      scope: { label: "外部快照", refs: [old] },
    });
    expect(external.active.externalRevision).toBe("old-revision");
    expect(previewUnavailable(s, external.active)).toContain("外部修订已变化");
  });
  it("single Agent refs do not inherit the directory or query", () => {
    const s = initialState();
    s.page = "search";
    const session = createPreviewSession(s, {
      id: "f1",
      single: true,
      version: 1,
    });
    expect(session.scope.refs).toHaveLength(1);
    expect(session.active.version).toBe(1);
  });
  it("query results keep order and fixed versions rather than widening to space", () => {
    const s = initialState();
    s.page = "search";
    s.search.query = "创作简报";
    const session = createPreviewSession(s, { id: "f1", version: 3 });
    expect(session.scope.label).toBe("当前搜索结果");
    expect(session.scope.refs.map((r) => r.id)).toEqual(["f1"]);
  });
  it("task scope distinguishes source and generated targets", () => {
    const s = initialState();
    const t = s.tasks[0];
    t.items[0].outputId = "f2";
    t.items[0].outputVersion = 1;
    const scope = taskPreviewScope(s, t.id);
    expect(scope.refs).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ id: t.items[0].fileId, role: "输入" }),
        expect.objectContaining({ id: "f2", version: 1, role: "目标" }),
      ]),
    );
  });
  it("round trips location fragments and rejects malformed links", () => {
    const ref = {
      id: "od-work-file",
      version: 1,
      externalRevision: "a/b 中文",
    };
    expect(parsePreviewFragment(previewFragment("personal", ref))).toEqual({
      space: "personal",
      ref,
    });
    for (const hash of [
      "#f/personal/f1/0",
      "#f/unknown/f1/1",
      "#f/personal/../1",
      "#f/personal/f1/1?revision=%XX",
    ])
      expect(parsePreviewFragment(hash)).toBeNull();
  });
  it("unavailable fixed versions, deleted files and revoked access never fall forward", () => {
    let s = initialState();
    expect(previewUnavailable(s, { id: "f1", version: 99 })).toContain(
      "固定版本不可用",
    );
    s.files[0].trashed = true;
    expect(previewUnavailable(s, { id: "f1", version: 1 })).toContain("不存在");
    s = initialState();
    s.viewState = "forbidden";
    expect(previewUnavailable(s, { id: "f1", version: 3 })).toContain(
      "无权访问",
    );
    expect(
      previewUnavailable(s, { id: "temporary-artifact", version: 1 }),
    ).toContain("不存在");
  });
  it("copies demo authority-free references and offline download uses selected version", () => {
    const s = initialState();
    const r = { id: "f1", version: 1 };
    expect(agentReference(s, r)).toBe(
      "drivebridge://demo-instance/file/f1/version/demo-v1",
    );
    expect(agentContext(s, r)).toContain("不可调用真实服务");
    expect(agentContext(s, r)).not.toMatch(/token=|secret=|https?:/);
    expect(
      reducer(s, { type: "offline", id: "f1", version: 1 }).files[0]
        .offlineVersion,
    ).toBe(1);
  });
  it("persists only bounded layout preferences", () => {
    const p = safePreferences({
      navCollapsed: true,
      previewListWidth: 900,
      inspectorPosition: "bottom",
      members: ["private"],
    });
    expect(p.navCollapsed).toBe(true);
    expect(p.previewListWidth).toBe(320);
    expect(p.inspectorPosition).toBe("auto");
    expect(p).not.toHaveProperty("members");
  });
});
