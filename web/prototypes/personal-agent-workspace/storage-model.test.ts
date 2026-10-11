import { describe, it, expect } from "vitest";
import { initialState, reducer } from "./model";
import { directoryFiles, rootNode, isObserved } from "./storage-model";
import { searchResults, emptySearch } from "./cloud-model";
const start = (connection = "drive", budget = 10000) => {
  let s = initialState();
  const id = rootNode(s, connection)!.id;
  s = reducer(s, {
    type: "catalog-configure",
    directoryId: id,
    exclusions: [],
    budget,
    policy: "manual",
  });
  s = reducer(s, { type: "catalog-start", directoryId: id });
  return s;
};
describe("v7 directory identity and storage indexing", () => {
  it("shows only immediate children while recursive searches are explicit", () => {
    const s = initialState(),
      root = rootNode(s, "drive")!;
    expect(directoryFiles(s, root.id)).toEqual([]);
    expect(directoryFiles(s, root.id, true).length).toBeGreaterThan(3);
    expect(s.files.find((f) => f.id === "f1")?.directoryId).not.toBe(root.id);
  });
  it("isolates identical folder names and empty directories across accounts", () => {
    let s = initialState();
    const nodes = s.storage.nodes.filter((n) => n.name === "项目");
    expect(nodes).toHaveLength(2);
    expect(nodes[0].id).not.toBe(nodes[1].id);
    for (const node of nodes) {
      s = reducer(s, { type: "directory-open", id: node.id });
      expect(directoryFiles(s, node.id)).toEqual([]);
      expect(
        s.storage.nodes
          .filter((n) => n.parentId === node.id)
          .map((n) => n.name),
      ).toEqual(["待归档"]);
    }
  });
  it("restores per connection directory and clears selection on navigation", () => {
    let s = initialState();
    const id = s.files.find((f) => f.id === "f1")!.directoryId!;
    s = reducer(s, { type: "directory-open", id });
    s = reducer(s, { type: "select", ids: ["f1"] });
    s = reducer(s, {
      type: "connection-select",
      id: "od-personal",
      scroll: 217,
    });
    expect(s.selected).toEqual([]);
    s = reducer(s, { type: "connection-select", id: "drive", scroll: 0 });
    expect(s.storage.activeId).toBe(id);
    expect(s.storage.positions["personal:drive"].scroll).toBe(217);
  });
  it("creates duplicate names only under separate parents and keeps empty folders", () => {
    let s = initialState();
    s = reducer(s, { type: "create-folder", name: "空目录" });
    const n = s.storage.nodes.find((n) => n.name === "空目录")!;
    expect(n.parentId).toBe(rootNode(s, "drive")!.id);
    s = reducer(s, { type: "create-folder", name: "空目录" });
    expect(s.storage.nodes.filter((n) => n.name === "空目录")).toHaveLength(1);
    expect(s.notice).toContain("同名");
  });
  it("rejects unverified writes and protected folder creation", () => {
    let s = initialState();
    s = reducer(s, { type: "connection-select", id: "od-personal", scroll: 0 });
    const before = s.storage.nodes.length;
    s = reducer(s, { type: "create-folder", name: "不应创建" });
    expect(s.storage.nodes).toHaveLength(before);
    s = reducer(s, {
      type: "import",
      source: "local",
      directoryId: s.storage.activeId,
    });
    expect(s.files.some((f) => f.id.startsWith("import"))).toBe(false);
  });
  it("requires confirmed scope, keeps content indexing separate and respects budget", () => {
    let s = initialState();
    const id = rootNode(s, "drive")!.id;
    s = reducer(s, { type: "catalog-start", directoryId: id });
    expect(s.storage.runs).toEqual([]);
    s = start("drive", 1);
    const run = s.storage.runs[0];
    s = reducer(s, {
      type: "catalog-complete",
      id: run.id,
      generation: run.generation,
      result: "complete",
    });
    expect(s.storage.runs[0].status).toBe("partial");
    expect(s.storage.runs[0].total).toBeNull();
    expect(s.indexes).toEqual([]);
  });
  it("does not infer deletions from partial scan and retains last complete result", () => {
    let s = start();
    let run = s.storage.runs[0];
    s = reducer(s, {
      type: "catalog-complete",
      id: run.id,
      generation: run.generation,
      result: "complete",
    });
    const complete = run.id;
    s = reducer(s, { type: "catalog-start", directoryId: run.directoryId });
    run = s.storage.runs[0];
    const files = structuredClone(s.files);
    s = reducer(s, {
      type: "catalog-complete",
      id: run.id,
      generation: run.generation,
      result: "partial",
    });
    expect(s.storage.scopes[run.directoryId].lastComplete).toBe(complete);
    expect(s.files).toEqual(files);
  });
  it("queries original unknown result without duplicate operations", () => {
    let s = start("s3");
    const run = s.storage.runs[0];
    s = reducer(s, {
      type: "catalog-complete",
      id: run.id,
      generation: run.generation,
      result: "unknown",
    });
    s = reducer(s, { type: "catalog-start", directoryId: run.directoryId });
    expect(s.storage.runs).toHaveLength(1);
    s = reducer(s, { type: "catalog-resume", id: run.id });
    expect(s.storage.runs[0].status).toBe("unknown");
    s = reducer(s, { type: "catalog-query", id: run.id });
    s = reducer(s, { type: "catalog-resume", id: run.id });
    expect(s.storage.runs[0]).toMatchObject({ id: run.id, status: "running" });
  });
  it("never offers fake resume for providers without checkpoints", () => {
    let s = start("od-personal");
    const run = s.storage.runs[0];
    s = reducer(s, {
      type: "catalog-complete",
      id: run.id,
      generation: run.generation,
      result: "partial",
    });
    s = reducer(s, { type: "catalog-query", id: run.id });
    s = reducer(s, { type: "catalog-resume", id: run.id });
    expect(s.storage.runs[0].status).toBe("partial");
    expect(s.notice).toContain("不支持检查点");
  });
  it("rejects late results after scope change and clear without deleting files", () => {
    let s = start();
    const run = s.storage.runs[0];
    s = reducer(s, {
      type: "catalog-configure",
      directoryId: run.directoryId,
      exclusions: ["私密"],
      budget: 10,
      policy: "manual",
    });
    s = reducer(s, {
      type: "catalog-complete",
      id: run.id,
      generation: run.generation,
      result: "complete",
    });
    expect(s.storage.scopes[run.directoryId].lastComplete).toBeUndefined();
    const files = structuredClone(s.files);
    s = reducer(s, { type: "catalog-clear", directoryId: run.directoryId });
    expect(s.files).toEqual(files);
    expect(s.storage.scopes[run.directoryId].confirmed).toBe(false);
  });
  it("revocation prevents late indexing and cancels inaccessible search disclosure", () => {
    let s = start("od-work");
    const run = s.storage.runs[0];
    s = reducer(s, {
      type: "connection-state",
      id: "od-work",
      status: "expired",
    });
    s = reducer(s, {
      type: "catalog-complete",
      id: run.id,
      generation: run.generation,
      result: "complete",
    });
    expect(s.storage.runs[0].observed).toEqual([]);
    expect(
      searchResults(s, { ...emptySearch(), connections: ["od-work"] }),
    ).toEqual([]);
  });
  it("separates content indexes for two accounts sharing root path", () => {
    let s = initialState();
    const node = rootNode(s, "od-personal")!;
    s = reducer(s, {
      type: "index-create",
      directoryId: node.id,
      connectionId: node.connectionId,
      folder: node.path,
      exclusions: [],
      kinds: ["document"],
      policy: "manual",
    });
    const id = s.indexes[0].id;
    s = reducer(s, { type: "index-status", id, status: "processing" });
    s = reducer(s, { type: "index-status", id, status: "ready" });
    expect(s.indexes[0].refs.map((r) => r.id)).toEqual(["od-personal-file"]);
  });
  it("uses directory identity and explicit recursion in saved search", () => {
    let s = initialState();
    const root = rootNode(s, "drive")!;
    expect(
      searchResults(s, {
        ...emptySearch(),
        directoryId: root.id,
        recursive: false,
      }),
    ).toHaveLength(0);
    const spec = { ...emptySearch(), directoryId: root.id, recursive: true };
    s = reducer(s, { type: "search-save", name: "全部已授权目录", spec });
    s = reducer(s, { type: "search-open", id: s.savedSearches[0].id });
    expect(s.search.directoryId).toBe(root.id);
    expect(searchResults(s).length).toBeGreaterThan(0);
  });
  it("copies pinned files while retaining relative paths and recovers only failed items", () => {
    let s = initialState();
    const source = rootNode(s, "drive")!,
      target = rootNode(s, "s3")!;
    s = reducer(s, {
      type: "plan",
      ids: ["f1", "f2"],
      kind: "transfer",
      sourceDirectoryId: source.id,
      targetDirectoryId: target.id,
    });
    const t = s.tasks[0];
    expect(t.items[0].after).toBe(target.path + " / 山野计划");
    s = reducer(s, {
      type: "confirm",
      id: t.id,
      revision: t.revision,
      contextRevision: s.contextRevision,
    });
    s = reducer(s, { type: "finish", id: t.id });
    const output = s.files.find((f) => f.id === `${t.id}-f1`)!;
    expect(output.folder).toBe(target.path + " / 山野计划");
    expect(output.connectionId).toBe("s3");
    expect(s.files.find((f) => f.id === "f1")!.folder).toBe("山野计划");
  });
  it("pauses only future scanning and protects files when clearing an index", () => {
    let s = start();
    const id = s.storage.runs[0].directoryId;
    s = reducer(s, { type: "catalog-pause", directoryId: id });
    s = reducer(s, { type: "catalog-start", directoryId: id });
    expect(s.storage.runs).toHaveLength(1);
    expect(s.storage.scopes[id].paused).toBe(true);
  });
});

describe("v7 final handoff boundaries", () => {
  it("does not start a scan if a connection save was rejected", () => {
    let s = initialState();
    const connection = { ...s.connections[1] };
    s = reducer(s, {
      type: "connection-save",
      connection,
      expectedRevision: 0,
      scanExclusions: [],
    });
    expect(s.storage.runs).toHaveLength(0);
  });
  it("confirms scan only after successful connection save and read verification", () => {
    let s = initialState();
    const connection = {
      ...s.connections[1],
      id: "new-account",
      name: "另一个账号",
      revision: 0,
    };
    s = reducer(s, {
      type: "connection-save",
      connection,
      expectedRevision: 0,
      scanExclusions: ["私密"],
    });
    expect(s.storage.runs[0].connectionId).toBe("new-account");
    expect(s.storage.scopes[rootNode(s, "new-account")!.id].exclusions).toEqual(
      ["私密"],
    );
  });
  it("records comparable usage observations without fabricating historical growth", () => {
    let s = initialState();
    expect(s.storage.usage).toEqual([]);
    s = reducer(s, { type: "usage-observe", connectionId: "drive" });
    s = reducer(s, { type: "usage-observe", connectionId: "drive" });
    expect(s.storage.usage).toHaveLength(2);
    expect(s.storage.usage[0].bytes).toBe(s.storage.usage[1].bytes);
    expect(s.storage.usage[0].count).toBeGreaterThan(0);
  });
  it("binds external application scope to an account and refuses revoked late results", () => {
    let s = initialState();
    const node = rootNode(s, "od-personal")!;
    s = reducer(s, {
      type: "app-create",
      directoryId: node.id,
      folder: node.path,
      name: "Scoped application",
      protocol: "MCP",
      hours: 24,
      actions: ["read", "upload"],
    });
    const app = s.apps[0];
    s = reducer(s, { type: "app-state", id: app.id, state: "verified" });
    s = reducer(s, { type: "app-call", id: app.id, action: "upload" });
    expect(s.files[0].inputRefs?.map((r) => r.id)).toEqual([
      "od-personal-file",
    ]);
    s = reducer(s, {
      type: "app-call",
      id: app.id,
      action: "read",
      unknown: true,
    });
    const call = s.calls[0];
    s = reducer(s, { type: "app-state", id: app.id, state: "revoked" });
    s = reducer(s, { type: "call-reconcile", id: call.id });
    expect(s.calls[0].result).toBe("unknown");
    expect(s.apps[0].state).toBe("revoked");
  });
});
