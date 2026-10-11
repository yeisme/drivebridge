import { describe, expect, it } from "vitest";
import {
  collectionFiles,
  currentConversation,
  emptyFilter,
  initialState,
  reducer,
  versionSnapshot,
} from "./model";

describe("Prototype authority and recovery invariants", () => {
  it("binds confirmation to the original revision and rejects repeat confirmation", () => {
    let s = initialState();
    const confirm = {
      type: "confirm" as const,
      id: "DB-024",
      revision: 1,
      contextRevision: 1,
    };
    s = reducer(s, confirm);
    s = reducer(s, confirm);
    expect(s.tasks[0].runs).toBe(1);
    expect(s.tasks[0].id).toBe("DB-024");
    let stale = reducer(initialState(), { type: "invalidate", id: "DB-024" });
    stale = reducer(stale, confirm);
    expect(stale.tasks[0].runs).toBe(0);
    expect(stale.tasks[0].status).toBe("stale");
  });
  it("recovers only failed items under the original operation", () => {
    let s = reducer(initialState(), {
      type: "confirm",
      id: "DB-024",
      revision: 1,
      contextRevision: 1,
    });
    s = reducer(s, { type: "finish", id: "DB-024" });
    expect(s.tasks[0].status).toBe("partial");
    const completed = s.tasks[0].items.filter((i) => i.status === "done");
    s = reducer(s, { type: "recover", id: "DB-024" });
    s = reducer(s, { type: "finish", id: "DB-024" });
    expect(s.tasks).toHaveLength(1);
    expect(s.tasks[0].status).toBe("done");
    expect(
      s.tasks[0].items.filter((i) =>
        completed.some((c) => c.fileId === i.fileId),
      ),
    ).toEqual(completed);
  });
  it("blocks a confirmation after space switching", () => {
    let s = reducer(initialState(), { type: "space", space: "team" });
    s = reducer(s, {
      type: "confirm",
      id: "DB-024",
      revision: 1,
      contextRevision: 1,
    });
    expect(s.tasks[0].runs).toBe(0);
    expect(s.context).toEqual([]);
  });
  it("discards late answers from an old scope and does not persist the old question", () => {
    let s = reducer(initialState(), { type: "analysis-consent" });
    s = reducer(s, {
      type: "ask",
      question: "Synthetic fixture question",
      contextRevision: 1,
    });
    s = reducer(s, { type: "space", space: "team" });
    s = reducer(s, {
      type: "answer",
      contextRevision: 1,
      refs: [{ id: "f1", version: 3 }],
    });
    expect(s.messages).toEqual([]);
    expect(s.answersPending).toBe(false);
  });
  it("revoking permission removes answers and prevents delayed output", () => {
    let s = reducer(initialState(), { type: "analysis-consent" });
    s = reducer(s, {
      type: "ask",
      question: "Synthetic fixture",
      contextRevision: 1,
    });
    s = reducer(s, { type: "view", state: "forbidden" });
    s = reducer(s, {
      type: "answer",
      contextRevision: 1,
      refs: [{ id: "f1", version: 3 }],
    });
    expect(s.messages).toEqual([]);
    expect(s.analysisRefs).toEqual([]);
  });
  it("revokes in-flight external agent work without inventing an output", () => {
    let s = reducer(initialState(), {
      type: "agent",
      value: "active",
      scope: "山野计划",
    });
    s = reducer(s, { type: "agent-run" });
    const id = s.tasks[0].id;
    s = reducer(s, { type: "agent", value: "revoked" });
    s = reducer(s, { type: "finish", id });
    expect(s.tasks[0].status).toBe("blocked");
    expect(s.files.some((f) => f.id === `${id}-output`)).toBe(false);
  });
  it("does not auto-execute when an agent gets authorized again", () => {
    let s = reducer(initialState(), { type: "agent", value: "active" });
    s = reducer(s, { type: "agent-run" });
    s = reducer(s, { type: "agent", value: "expired" });
    s = reducer(s, { type: "agent", value: "active" });
    expect(s.tasks[0].status).toBe("stale");
  });
  it("keeps offline copies pinned and blocks mutations while offline", () => {
    let s = reducer(initialState(), { type: "offline", id: "f1" });
    s = reducer(s, { type: "remote-version", id: "f1" });
    s = reducer(s, { type: "network", online: false });
    s = reducer(s, { type: "share", id: "f1", version: 3 });
    expect(s.files.find((f) => f.id === "f1")?.offlineVersion).toBe(3);
    expect(s.shares).toEqual([]);
  });
  it("preserves fixed-version sharing and reception facts after revocation", () => {
    let s = reducer(initialState(), { type: "share", id: "f1", version: 1 });
    s = reducer(s, { type: "remote-version", id: "f1" });
    s = reducer(s, { type: "receipt", id: "share-1" });
    s = reducer(s, { type: "revoke-share", id: "share-1" });
    expect(s.shares[0]).toMatchObject({
      version: 1,
      state: "revoked",
      received: true,
    });
  });
  it("requires new consent when a selected file version changes", () => {
    let s = reducer(initialState(), { type: "analysis-consent" });
    s = reducer(s, { type: "remote-version", id: "f1" });
    s = reducer(s, {
      type: "ask",
      question: "Synthetic fixture",
      contextRevision: 1,
    });
    expect(s.answersPending).toBe(false);
    expect(s.messages).toEqual([]);
  });
  it("restoration requires a fresh plan and rejects subsequent file changes", () => {
    let s = reducer(initialState(), {
      type: "confirm",
      id: "DB-024",
      revision: 1,
      contextRevision: 1,
    });
    s = reducer(s, { type: "finish", id: "DB-024" });
    s = reducer(s, { type: "remote-version", id: "f1" });
    s = reducer(s, { type: "compensate", id: "DB-024" });
    expect(s.tasks).toHaveLength(1);
    expect(s.notice).toContain("恢复冲突");
  });
  it("does not duplicate completed cross-storage copies while recovering a conflict", () => {
    let s = reducer(initialState(), {
      type: "plan",
      ids: ["f1", "f2"],
      kind: "transfer",
      target: "S3 / 创作归档",
    });
    const id = s.tasks[0].id;
    s = reducer(s, { type: "confirm", id, revision: 1, contextRevision: 1 });
    s = reducer(s, { type: "finish", id });
    s = reducer(s, { type: "recover", id });
    s = reducer(s, { type: "finish", id });
    s = reducer(s, { type: "finish", id });
    expect(s.files.filter((f) => f.id.startsWith(id))).toHaveLength(2);
    expect(s.files.find((f) => f.id === `${id}-f2`)?.name).toContain("副本");
  });
  it("backup creates a new copy and never moves or deletes the source", () => {
    let s = reducer(initialState(), { type: "rule", mode: "backup" });
    s = reducer(s, { type: "rule-run" });
    const id = s.tasks[0].id;
    s = reducer(s, { type: "recover", id });
    s = reducer(s, { type: "finish", id });
    expect(s.files.find((f) => f.id === "f9")).toMatchObject({
      folder: "待整理",
      inbox: true,
      origin: "phone",
    });
    expect(s.files.find((f) => f.id === `${id}-f9`)).toMatchObject({
      folder: "我的资料 / 自动归档",
      origin: "cloud",
      inbox: false,
    });
  });
});

describe("Creative workspace v2", () => {
  it("does not execute old builtin ask and answer actions",()=>{let s=initialState();s=reducer(s,{type:"ask",question:"fixture",contextRevision:s.contextRevision});s=reducer(s,{type:"answer",contextRevision:s.contextRevision,refs:[{id:"f1",version:3}]});expect(s.messages).toEqual([]);expect(s.answersPending).toBe(false);});
  it("keeps external tasks and file references when the old sidebar is requested",()=>{const before=initialState();const s=reducer(before,{type:"agent-panel",open:true});expect(s.agentOpen).toBe(false);expect(s.tasks).toEqual(before.tasks);expect(s.files).toEqual(before.files);});
  it("virtual collections and groups never move the underlying files", () => {
    let s = initialState();
    const before = structuredClone(s.files);
    s = reducer(s, {
      type: "collection-create",
      name: "Research board",
      ids: ["f1", "f2", "f10"],
    });
    const c = s.collections.at(-1)!;
    expect(c.ids).toEqual(["f1", "f2"]);
    s = reducer(s, {
      type: "collection-group",
      id: c.id,
      fileId: "f2",
      group: "参考",
    });
    expect(s.files).toEqual(before);
    expect(s.collections.at(-1)?.groups.f2).toBe("参考");
  });
  it("smart collections evaluate filters in the current authorized space", () => {
    let s = reducer(initialState(), {
      type: "collection-create",
      name: "Images",
      ids: [],
      query: { ...emptyFilter(), kind: "image" },
    });
    const c = s.collections.at(-1)!;
    expect(collectionFiles(s, c).every((f) => f.kind === "image")).toBe(true);
    s = reducer(s, { type: "space", space: "team" });
    expect(collectionFiles(s, c)).toEqual([]);
  });
  it("editing a pending plan rejects its old approval and preserves completed evidence during re-preview", () => {
    let s = reducer(initialState(), {
      type: "edit-plan",
      id: "DB-024",
      revision: 1,
      ids: ["f1", "f2", "f3"],
      target: "我的资料",
      prefix: "",
    });
    s = reducer(s, {
      type: "confirm",
      id: "DB-024",
      revision: 1,
      contextRevision: s.contextRevision,
    });
    expect(s.tasks[0].runs).toBe(0);
    s = reducer(s, { type: "refresh-plan", id: "DB-024" });
    s = reducer(s, {
      type: "confirm",
      id: "DB-024",
      revision: s.tasks[0].revision,
      contextRevision: s.contextRevision,
    });
    s = reducer(s, { type: "finish", id: "DB-024" });
    const done = structuredClone(
      s.tasks[0].items.filter((i) => i.status === "done"),
    );
    s = reducer(s, { type: "refresh-plan", id: "DB-024" });
    expect(s.tasks[0].items.filter((i) => i.status === "done")).toEqual(done);
    expect(s.tasks[0].id).toBe("DB-024");
  });
  it("rename is previewed and excluded items are left intact", () => {
    let s = reducer(initialState(), {
      type: "plan",
      kind: "rename",
      ids: ["f1", "f2"],
      prefix: "选片",
    });
    const id = s.tasks[0].id;
    expect(s.files.find((f) => f.id === "f1")?.name).toContain("创作简报");
    s = reducer(s, {
      type: "edit-plan",
      id,
      revision: 1,
      ids: ["f1"],
      target: "山野计划",
      prefix: "定稿",
    });
    s = reducer(s, {
      type: "confirm",
      id,
      revision: 2,
      contextRevision: s.contextRevision,
    });
    s = reducer(s, { type: "finish", id });
    expect(s.files.find((f) => f.id === "f1")?.name).toBe("定稿_01.pdf");
    expect(s.files.find((f) => f.id === "f2")?.name).toContain("视觉参考");
    expect(s.tasks[0].items[1].status).toBe("skipped");
  });
  it("restores the old content into a new version while keeping every earlier snapshot", () => {
    let s = initialState();
    const f = s.files.find((f) => f.id === "f1")!;
    const old = versionSnapshot(f, 1);
    const latest = versionSnapshot(f, 3);
    s = reducer(s, { type: "restore-version", id: "f1", version: 1 });
    const restored = s.files.find((f) => f.id === "f1")!;
    expect(restored.version).toBe(4);
    expect(versionSnapshot(restored, 4).content).toBe(old.content);
    expect(versionSnapshot(restored, 3)).toEqual(latest);
  });
  it("creates a multi-file fixed delivery and blocks a missing member", () => {
    let s = reducer(initialState(), {
      type: "share",
      id: "f1",
      version: 1,
      members: [
        { id: "f1", version: 1 },
        { id: "f2", version: 2 },
      ],
      days: 30,
      download: false,
    });
    s = reducer(s, { type: "remote-version", id: "f1" });
    expect(s.shares[0].members).toEqual([
      { id: "f1", version: 1 },
      { id: "f2", version: 2 },
    ]);
    expect(s.shares[0].download).toBe(false);
    s = reducer(s, {
      type: "share",
      id: "f1",
      version: 1,
      members: [
        { id: "f1", version: 1 },
        { id: "f10", version: 1 },
      ],
    });
    expect(s.shares).toHaveLength(1);
  });
  it("an overwrite decision preserves the target history and source file", () => {
    let s = reducer(initialState(), {
      type: "plan",
      kind: "transfer",
      ids: ["f3"],
      target: "S3 / 创作归档",
    });
    const id = s.tasks[0].id;
    const source = structuredClone(s.files.find((f) => f.id === "f3")!);
    s = reducer(s, {
      type: "confirm",
      id,
      revision: 1,
      contextRevision: s.contextRevision,
    });
    s = reducer(s, { type: "finish", id });
    expect(s.tasks[0].status).toBe("partial");
    s = reducer(s, {
      type: "resolve-conflicts",
      id,
      revision: 1,
      choice: "overwrite",
    });
    s = reducer(s, { type: "finish", id });
    expect(s.files.find((f) => f.id === "f3")).toEqual(source);
    const target = s.files.find((f) => f.id === "f13")!;
    expect(target.version).toBe(2);
    expect(versionSnapshot(target, 2).content).toBe(
      versionSnapshot(source, 2).content,
    );
    expect(target.snapshots).toHaveLength(2);
  });
  it("a configured rule limits its source, count, and destination without moving originals", () => {
    let s = reducer(initialState(), {
      type: "rule",
      mode: "backup",
      config: {
        source: "山野计划",
        target: "生活记录",
        wifiOnly: true,
        maxFiles: 1,
        validHours: 2,
      },
    });
    s = reducer(s, { type: "rule-run" });
    const id = s.tasks[0].id;
    expect(s.tasks[0].items).toHaveLength(1);
    s = reducer(s, { type: "recover", id });
    s = reducer(s, { type: "finish", id });
    expect(s.files.find((f) => f.id === "f1")?.folder).toBe("山野计划");
    expect(s.files.find((f) => f.id === `${id}-f1`)?.folder).toBe("生活记录");
  });
});

describe("Scope and rule constraints", () => {
  it("rejects original work after an active grant changes directories", () => {
    let s = reducer(initialState(), {
      type: "agent",
      value: "active",
      scope: "山野计划",
    });
    s = reducer(s, { type: "agent-run" });
    const id = s.tasks[0].id;
    s = reducer(s, { type: "agent", value: "active", scope: "我的资料" });
    s = reducer(s, { type: "finish", id });
    expect(s.tasks[0].status).toBe("blocked");
    expect(s.files.some((f) => f.id === `${id}-output`)).toBe(false);
    s = reducer(s, { type: "refresh-plan", id });
    s = reducer(s, {
      type: "confirm",
      id,
      revision: s.tasks[0].revision,
      contextRevision: s.contextRevision,
    });
    expect(s.tasks[0].status).toBe("blocked");
  });
  it("preserves the previous rule when limits are invalid", () => {
    const original = initialState();
    const s = reducer(original, {
      type: "rule",
      mode: "backup",
      config: { ...original.ruleConfig, maxFiles: -1 },
    });
    expect(s.rule).toBe(original.rule);
    expect(s.ruleConfig).toEqual(original.ruleConfig);
    expect(s.notice).toContain("无效");
  });
});
