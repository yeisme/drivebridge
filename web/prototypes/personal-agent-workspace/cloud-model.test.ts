import { describe, expect, it } from "vitest";
import { initialState, reducer } from "./model";
import {
  assignedTags,
  emptySearch,
  safePreferences,
  searchResults,
} from "./cloud-model";

describe("DriveBridge v3 independent drive contracts", () => {
  it("redirects legacy assistant entry to search without a chat host", () => {
    let s = initialState();
    expect(s.page).toBe("files");
    s = reducer(s, { type: "folder", path: "山野计划" });
    s = reducer(s, { type: "navigate", page: "assistant" });
    expect(s.page).toBe("search");
    expect(s.browser.folder).toBe("山野计划");
    expect(s.agentOpen).toBe(false);
  });
  it("appends and removes only manual tags and requires separate replacement confirmation", () => {
    let s = initialState();
    s = reducer(s, {
      type: "tag-apply",
      ids: ["f3"],
      tagIds: ["t-delivery"],
      mode: "add",
    });
    expect(s.files.find((f) => f.id === "f3")?.tags).toContainEqual({
      tagId: "t-project",
      source: "rule",
    });
    s = reducer(s, {
      type: "tag-apply",
      ids: ["f3"],
      tagIds: [],
      mode: "replace",
    });
    expect(assignedTags(s.files.find((f) => f.id === "f3")!)).toContain(
      "t-delivery",
    );
    s = reducer(s, {
      type: "tag-apply",
      ids: ["f3"],
      tagIds: [],
      mode: "replace",
      confirmed: true,
    });
    expect(s.files.find((f) => f.id === "f3")?.tags).toEqual([
      { tagId: "t-project", source: "rule" },
    ]);
  });
  it("accepts a suggestion without replacing existing manual tags", () => {
    let s = reducer(initialState(), {
      type: "tag-suggestion",
      fileId: "f2",
      tagId: "t-review",
      accept: true,
    });
    expect(s.files.find((f) => f.id === "f2")?.tags).toEqual(
      expect.arrayContaining([
        { tagId: "t-review", source: "manual" },
        { tagId: "t-outdoor", source: "manual" },
      ]),
    );
  });
  it("merges references and saved filters without moving files, and delete never deletes files", () => {
    let s = initialState();
    s = reducer(s, {
      type: "search-save",
      name: "交付",
      spec: { ...emptySearch(), tagIds: ["t-delivery"] },
    });
    const count = s.files.length;
    const folder = s.files[0].folder;
    s = reducer(s, { type: "tag-merge", from: "t-delivery", to: "t-project" });
    expect(s.savedSearches[0].spec.tagIds).toEqual(["t-project"]);
    expect(s.files[0].folder).toBe(folder);
    s = reducer(s, { type: "tag-delete", id: "t-project" });
    expect(s.files).toHaveLength(count);
    expect(s.savedSearches[0].spec.tagIds).toEqual([]);
  });
  it("separates exact tag filters from suggestions and scopes cross-connection search", () => {
    let s = initialState();
    s.search = { ...emptySearch(), tagIds: ["t-project", "t-delivery"] };
    expect(searchResults(s).map((r) => r.file.id)).toEqual(["f1"]);
    s.search.tagIds = ["t-review"];
    expect(searchResults(s)).toEqual([]);
    s.search = { ...emptySearch(), connections: ["s3"] };
    expect(searchResults(s).every((r) => r.file.folder.startsWith("S3"))).toBe(
      true,
    );
    s = reducer(s, { type: "space", space: "team" });
    expect(searchResults(s).every((r) => r.file.space === "team")).toBe(true);
  });
  it("indexes only consented types and exclusions, rejects late completion after pause, and preserves original recovery ID", () => {
    let s = initialState();
    s = reducer(s, {
      type: "index-create",
      folder: "山野计划",
      exclusions: ["脚本"],
      kinds: ["document"],
      policy: "manual",
    });
    const id = s.indexes[0].id;
    s = reducer(s, { type: "index-status", id, status: "processing" });
    s = reducer(s, { type: "index-status", id, status: "paused" });
    s = reducer(s, { type: "index-status", id, status: "partial" });
    expect(s.indexes[0].status).toBe("paused");
    expect(s.indexes[0].refs).toEqual([]);
    s = reducer(s, { type: "index-recover", id });
    expect(s.indexes[0].id).toBe(id);
    expect(s.indexes[0].refs.map((r) => r.id)).toEqual(["f1"]);
    s.search = { ...emptySearch(), tab: "content", query: "自然光" };
    expect(searchResults(s)[0]).toMatchObject({
      version: 3,
      indexed: true,
      stale: false,
    });
    s = reducer(s, { type: "remote-version", id: "f1" });
    expect(searchResults(s)[0].stale).toBe(true);
    s = reducer(s, { type: "index-clear", id });
    expect(searchResults(s)).toEqual([]);
    expect(s.files.find((f) => f.id === "f1")).toBeTruthy();
  });
  it("does not confuse configured, read-verified and write-verified applications", () => {
    let s = initialState();
    s = reducer(s, {
      type: "app-create",
      name: "Fixture",
      protocol: "MCP",
      folder: "山野计划",
      actions: ["read", "upload"],
      hours: 24,
    });
    const id = s.apps[0].id;
    s = reducer(s, { type: "app-call", id, action: "upload" });
    expect(s.calls[0].result).toBe("denied");
    s = reducer(s, { type: "app-state", id, state: "verified" });
    expect(s.apps[0].writeVerified).toBe(false);
    s = reducer(s, { type: "app-call", id, action: "upload" });
    const file = s.files.find((f) => f.id === s.calls[0].outputId)!;
    expect(file).toMatchObject({ inbox: true, origin: "agent" });
    expect(file.inputRefs?.length).toBeGreaterThan(0);
    expect(s.apps[0].writeVerified).toBe(true);
    s = reducer(s, { type: "app-state", id, state: "revoked" });
    s = reducer(s, { type: "app-call", id, action: "upload" });
    expect(s.calls[0].result).toBe("denied");
    expect(s.files.filter((f) => f.id.startsWith("EXT-"))).toHaveLength(1);
  });
  it("reconciles unknown calls under the same identity without a second application call", () => {
    let s = reducer(initialState(), {
      type: "app-create",
      name: "Fixture",
      protocol: "API",
      folder: "山野计划",
      actions: ["read"],
      hours: 1,
    });
    s = reducer(s, { type: "app-state", id: s.apps[0].id, state: "verified" });
    s = reducer(s, {
      type: "app-call",
      id: s.apps[0].id,
      action: "read",
      unknown: true,
    });
    const id = s.calls[0].id,
      op = s.calls[0].operationId;
    s = reducer(s, { type: "call-reconcile", id });
    expect(s.calls).toHaveLength(1);
    expect(s.calls[0]).toMatchObject({ operationId: op, result: "done" });
  });
  it("stores only validated non-sensitive preferences", () => {
    expect(
      safePreferences({
        agentWidth: 999,
        theme: "malformed",
        credential: "secret",
        messages: ["private"],
      }),
    ).toEqual({
      theme: "system",
      density: "comfortable",
      layout: "list",
      startup: "files",
      reduceMotion: false,
      agentWidth: 480,
      navCollapsed: false,
      previewListWidth: 220,
      inspectorPosition: "auto",
      agentRememberOpen: false,
    });
  });
});

it("updates existing single-tag collection queries during merge and preserves namespace layers", () => {
  let s = initialState();
  s = reducer(s, { type: "filter", patch: { tag: "t-delivery" } });
  s = reducer(s, {
    type: "collection-create",
    name: "Legacy single-tag view",
    ids: [],
    query: { ...s.browser.filter },
  });
  s = reducer(s, { type: "tag-merge", from: "t-delivery", to: "t-project" });
  expect(s.collections.at(-1)?.query?.tag).toBe("t-project");
  expect(s.browser.filter.tag).toBe("t-project");
});
it("an empty selected Agent scope never becomes a whole-drive search", () => {
  const s = initialState();
  s.search = { ...emptySearch(), fileIds: [] };
  expect(searchResults(s)).toEqual([]);
});
