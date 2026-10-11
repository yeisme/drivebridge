import { describe, it, expect } from "vitest";
import { initialState, reducer } from "./model";
import { contextRefs, contextUnavailable, fileUnavailable } from "./v4-model";
import { fileConnection, searchResults } from "./cloud-model";

describe("v4 connection and collaboration invariants", () => {
  it("binds providers explicitly, keeps separate browser state and fixed conversation scope", () => {
    let s = initialState();
    const context = [...s.context];
    s.browser.filter.query = "山野";
    s.selected = ["f1"];
    s = reducer(s, { type: "connection-select", id: "od-work", scroll: 123 });
    expect(s.selected).toEqual([]);
    expect(s.context).toEqual(context);
    expect(s.browser.filter.query).toBe("");
    s = reducer(s, { type: "filter", patch: { query: "工作" } });
    s = reducer(s, { type: "connection-select", id: "drive", scroll: 55 });
    expect(s.browser.filter.query).toBe("山野");
    expect(s.connectionBrowsers["personal:drive"].scroll).toBe(123);
    expect(fileConnection({ ...s.files[0], folder: "S3 / renamed" })).toBe(
      "drive",
    );
  });
  it("invalidates old plans on root changes and never treats readonly checks as writes", () => {
    let s = initialState();
    s = reducer(s, {
      type: "plan",
      ids: ["f1"],
      kind: "transfer",
      target: "资料",
      targetConnectionId: "od-work",
    });
    const t = s.tasks[0];
    expect(t.targetConnectionId).toBe("od-work");
    const c = s.connections.find((c) => c.id === "od-work")!;
    s = reducer(s, {
      type: "connection-save",
      connection: { ...c, root: "新目录" },
      expectedRevision: c.revision,
    });
    expect(s.tasks[0].status).toBe("stale");
    expect(s.tasks[0].revision).toBe(t.revision + 1);
    s = reducer(s, { type: "connection-check", id: "od-work" });
    expect(s.connections.find((c) => c.id === "od-work")?.write).toBe(
      "unknown",
    );
  });
  it("pins source and target output identities for a recovered transfer", () => {
    let s = initialState();
    s = reducer(s, {
      type: "plan",
      ids: ["f1"],
      kind: "transfer",
      target: "资料",
      targetConnectionId: "od-work",
    });
    const id = s.tasks[0].id;
    s = reducer(s, {
      type: "confirm",
      id,
      revision: s.tasks[0].revision,
      contextRevision: s.contextRevision,
    });
    s = reducer(s, { type: "finish", id });
    s = reducer(s, { type: "recover", id });
    s = reducer(s, { type: "finish", id });
    const i = s.tasks[0].items[0];
    expect(i.outputId).toBeTruthy();
    expect(s.files.find((f) => f.id === i.outputId)?.connectionId).toBe(
      "od-work",
    );
    expect(s.files.find((f) => f.id === "f1")?.connectionId).toBe("drive");
    const count = s.files.length;
    s = reducer(s, { type: "finish", id });
    expect(s.files).toHaveLength(count);
  });
  it("preserves frozen directory context when files or providers change", () => {
    let s = initialState();
    s = reducer(s, { type: "context", ids: [] });
    s = reducer(s, {
      type: "context-reference",
      reference: {
        key: "folder:drive:山野计划",
        kind: "folder",
        name: "山野计划",
        connectionId: "drive",
        refs: [{ id: "f1", version: 3 }],
      },
    });
    s.files.push({ ...s.files[0], id: "new-file" });
    expect(contextRefs(s)[0].refs).toHaveLength(1);
    s = reducer(s, { type: "connection-select", id: "s3", scroll: 0 });
    expect(s.context).toEqual(["f1"]);
    s = reducer(s, { type: "remote-version", id: "f1" });
    expect(contextRefs(s)[0].refs[0].version).toBe(3);
  });
  it("rejects lost external context and stale late answers", () => {
    let s = initialState();
    s = reducer(s, { type: "context", ids: ["od-work-file"] });
    s = reducer(s, { type: "analysis-consent" });
    s = reducer(s, {
      type: "ask",
      question: "总结",
      contextRevision: s.contextRevision,
    });
    const rev = s.contextRevision;
    s = reducer(s, {
      type: "connection-state",
      id: "od-work",
      status: "expired",
    });
    expect(contextUnavailable(s)).toBe(true);
    s = reducer(s, {
      type: "answer",
      contextRevision: rev,
      refs: [{ id: "od-work-file", version: 1 }],
    });
    expect(s.messages.filter((m) => m.role === "assistant")).toHaveLength(0);
    expect(s.answersPending).toBe(false);
  });
  it("rejects unverified external historical versions", () => {
    const s = initialState();
    expect(
      fileUnavailable(
        s,
        s.files.find((f) => f.id === "od-work-file"),
        2,
      ),
    ).toContain("固定版本不可用");
  });
  it("keeps team membership separate from file permission and prevents repeat acceptance", () => {
    let s = initialState();
    s = reducer(s, { type: "invite", userId: "chen" });
    const id = s.invitations[0].id;
    s = reducer(s, { type: "invitation-state", id, status: "accepted" });
    expect(s.invitations[0].status).toBe("pending");
    s = reducer(s, { type: "actor", id: "chen" });
    s = reducer(s, { type: "invitation-state", id, status: "accepted" });
    expect(s.members.find((m) => m.id === "chen")?.permission).toBe("none");
    s = reducer(s, { type: "invitation-state", id, status: "accepted" });
    expect(s.notice).toContain("不能重复");
  });
  it("blocks self-grants, preserves last owner, and requires admin for membership changes", () => {
    let s = initialState();
    s = reducer(s, { type: "member-remove", id: "yeisme" });
    expect(s.members[0].active).toBe(true);
    s = reducer(s, { type: "actor", id: "lin" });
    s = reducer(s, {
      type: "member-permission",
      id: "lin",
      permission: "custodian",
    });
    expect(s.members[1].permission).toBe("readonly");
  });
  it("revocation removes recipient search/preview access and cannot be undone by late responses", () => {
    let s = initialState();
    s = reducer(s, { type: "actor", id: "lin" });
    expect(fileUnavailable(s, s.files[0])).toBe("");
    s = reducer(s, { type: "actor", id: "yeisme" });
    s = reducer(s, { type: "shared-revoke", id: "shared-1" });
    s = reducer(s, { type: "actor", id: "lin" });
    expect(fileUnavailable(s, s.files[0])).toContain("没有文件权限");
    expect(searchResults(s)).toHaveLength(0);
    expect(s.sharedAccess[0].active).toBe(false);
  });
  it("version threads preserve replies, resolve state and do not grant mentioned members permission", () => {
    let s = initialState();
    s = reducer(s, {
      type: "discussion",
      fileId: "f1",
      version: 2,
      body: "@陈雨 请审阅",
    });
    const id = s.discussions[0].id;
    s = reducer(s, {
      type: "discussion",
      fileId: "f1",
      version: 2,
      body: "收到",
      threadId: id,
    });
    s = reducer(s, { type: "discussion-resolve", id });
    expect(s.discussions[0].replies).toHaveLength(1);
    expect(s.discussions[0].resolved).toBe(true);
    expect(s.members[2].permission).toBe("none");
  });
  it("fixed and live shares keep distinct strategies, and conflict handling preserves the current file", () => {
    let s = initialState();
    s = reducer(s, { type: "share", id: "f1", version: 2, mode: "fixed" });
    s = reducer(s, {
      type: "share",
      id: "f1",
      version: 3,
      mode: "live",
      audience: "users",
      recipients: ["lin"],
      passcodeEnabled: true,
    });
    expect(s.shares.map((x) => x.mode)).toEqual(["live", "fixed"]);
    const n = s.files.length;
    s = reducer(s, {
      type: "collaboration-conflict",
      fileId: "f1",
      expectedVersion: 2,
      choice: "copy",
    });
    expect(s.files).toHaveLength(n + 1);
    expect(s.files[0].version).toBe(3);
  });
  it("reset restores the same connection sample identities without persisting domain data", () => {
    let s = initialState();
    s = reducer(s, {
      type: "connection-state",
      id: "s3",
      status: "disconnected",
    });
    s = reducer(s, { type: "reset" });
    expect(s.connections.find((c) => c.id === "s3")?.status).toBe("active");
    expect(s.invitations).toEqual([]);
  });
  it("preserves the acting member across spaces and rejects edits by readonly members", () => {
    let s = initialState();
    s = reducer(s, { type: "actor", id: "lin" });
    const count = s.tasks.length;
    s = reducer(s, { type: "plan", ids: ["f1"], target: "其他目录" });
    expect(s.tasks).toHaveLength(count);
    s = reducer(s, { type: "space", space: "team" });
    expect(s.actor).toBe("lin");
    expect(
      fileUnavailable(
        s,
        s.files.find((f) => f.id === "f10"),
      ),
    ).toBe("");
    s = reducer(s, { type: "actor", id: "chen" });
    expect(
      fileUnavailable(
        s,
        s.files.find((f) => f.id === "f10"),
      ),
    ).toContain("没有文件权限");
  });
  it("a collision in another provider does not overwrite that instance", () => {
    let s = initialState();
    s.files.push({
      ...s.files.find((f) => f.id === "od-personal-file")!,
      id: "other-duplicate",
      name: s.files[0].name,
    });
    s = reducer(s, {
      type: "plan",
      ids: ["f1"],
      kind: "transfer",
      target: "资料",
      targetConnectionId: "od-work",
    });
    const id = s.tasks[0].id;
    s = reducer(s, {
      type: "confirm",
      id,
      revision: 1,
      contextRevision: s.contextRevision,
    });
    s = reducer(s, { type: "finish", id });
    s = reducer(s, { type: "recover", id });
    s = reducer(s, { type: "finish", id });
    expect(s.files.find((f) => f.id === "other-duplicate")?.version).toBe(1);
    expect(s.tasks[0].items[0].outputId).not.toBe("other-duplicate");
    expect(s.connections.find((c) => c.id === "od-work")?.write).toBe("passed");
  });
});
