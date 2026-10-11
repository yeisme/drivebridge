import { describe, expect, it } from "vitest";
import { initialState, reducer } from "./model";
import {
  sortEntries,
  browseHistory,
  browseLocation,
  browseQuery,
  explorerState,
  propertyFacets,
  tagFacets,
} from "./browse-model";
import { rootNode } from "./storage-model";
import { findEntries } from "./organization-model";
const init = () => reducer(initialState(), { type: "k-seed" });
describe("v9 scoped browsing", () => {
  it("back differs from parent, forward restores, new navigation removes forward branch", () => {
    let s = init();
    const root = rootNode(s, "drive")!.id,
      a = s.files.find((f) => f.id === "f1")!.directoryId!,
      b = s.storage.nodes.find((n) => n.name === "我的资料")!.id;
    s = reducer(s, { type: "directory-open", id: a });
    s = reducer(s, { type: "directory-open", id: b });
    const parent = reducer(s, { type: "browse-up" });
    expect(browseLocation(parent).directoryId).toBe(root);
    s = reducer(s, { type: "browse-history", direction: "back" });
    expect(browseLocation(s).directoryId).toBe(a);
    s = reducer(s, { type: "browse-history", direction: "forward" });
    expect(browseLocation(s).directoryId).toBe(b);
    s = reducer(s, { type: "browse-history", direction: "back" });
    s = reducer(s, { type: "directory-open", id: root });
    expect(browseHistory(s).forward).toEqual([]);
    expect(browseHistory(reducer(s, { type: "browse-up" }))).toEqual(
      browseHistory(s),
    );
  });
  it("query return retains source scroll focus sort and page; keystrokes do not commit history", () => {
    let s = init();
    const id = s.files.find((f) => f.id === "f1")!.directoryId!;
    s = reducer(s, { type: "directory-open", id });
    s = reducer(s, { type: "browser", patch: { sort: "size", page: 2 } });
    s = reducer(s, { type: "browse-checkpoint", scroll: 127, focus: "f1" });
    const old = browseLocation(s);
    s = reducer(s, {
      type: "browse-query",
      query: { ...browseQuery(s), query: "山野" },
    });
    expect(s.selected).toEqual([]);
    expect(s.page).toBe("files");
    const count = browseHistory(s).back.length;
    s = reducer(s, { type: "search-patch", patch: { query: "draft" } });
    expect(browseHistory(s).back).toHaveLength(count);
    s = reducer(s, { type: "browse-return" });
    expect(browseLocation(s)).toEqual(old);
  });
  it("history crosses connections by identity and preserves inaccessible original location", () => {
    let s = init();
    const dirs = s.storage.nodes.filter((n) => n.name === "项目");
    s = reducer(s, { type: "directory-open", id: dirs[0].id });
    s = reducer(s, { type: "directory-open", id: dirs[1].id });
    s.connections.find((c) => c.id === dirs[0].connectionId)!.status =
      "disconnected";
    s = reducer(s, { type: "browse-history", direction: "back" });
    expect(s.storage.activeId).toBe(dirs[0].id);
    expect(s.notice).toContain("不可用");
    expect(s.storage.activeId).not.toBe(dirs[1].id);
  });
  it("space histories and tree presentation stay isolated", () => {
    let s = init();
    const id = s.files.find((f) => f.id === "f1")!.directoryId!;
    s = reducer(s, { type: "directory-open", id });
    s = reducer(s, { type: "explorer-mode", mode: "tags" });
    s = reducer(s, { type: "explorer-scroll", mode: "tags", value: 80 });
    s = reducer(s, { type: "explorer-expand", id: "tag:项目" });
    s = reducer(s, { type: "space", space: "team" });
    expect(browseHistory(s).back).toEqual([]);
    expect(explorerState(s).mode).toBe("directory");
    s = reducer(s, { type: "space", space: "personal" });
    expect(s.storage.activeId).toBe(id);
    expect(explorerState(s).scroll.tags).toBe(80);
    expect(explorerState(s).expanded).toContain("tag:项目");
  });
  it("refresh does not create an index run and invalid IDs do not replace location", () => {
    let s = init();
    const old = browseLocation(s);
    s = reducer(s, { type: "directory-open", id: "missing" });
    expect(browseLocation(s)).toEqual(old);
    const runs = s.storage.runs;
    s = reducer(s, { type: "browse-refresh" });
    expect(s.storage.runs).toEqual(runs);
    expect(s.notice).toContain("没有发起全盘");
  });
  it("facets are scoped and only use formal tags and parsed saved properties", () => {
    const s = init(),
      tags = tagFacets(s);
    expect(tags.some((t) => t.name === "项目")).toBe(true);
    const q = { ...browseQuery(s), documentTag: "项目" };
    const ids = findEntries(s, q)
      .filter((r) => r.file)
      .map((r) => r.ref.id);
    expect(ids.length).toBeGreaterThan(0);
    const d = propertyFacets(s, "status");
    expect(d.coverage.ready).toBeGreaterThan(0);
    expect(d.values.some((v) => v.state === "unset")).toBe(true);
    const external = {
      ...q,
      connections: ["od-work"],
      directoryId: rootNode(s, "od-work")!.id,
    };
    expect(tagFacets(s, external).some((t) => t.name === "项目/山野")).toBe(
      false,
    );
  });
  it("keeps cooperation screen while changing actors and applies explicit search sort", () => {
    let s = init();
    s = reducer(s, { type: "navigate", page: "collaboration" });
    s = reducer(s, { type: "actor", id: "chen" });
    s = reducer(s, { type: "actor", id: "yeisme" });
    expect(s.page).toBe("collaboration");
    const result = findEntries(s, { ...browseQuery(s), query: "" }),
      before = result.map((x) => x.ref.id);
    const ordered = sortEntries(result, "size"),
      files = ordered.filter((x) => x.file);
    expect(files.length).toBeGreaterThan(1);
    expect(result.map((x) => x.ref.id)).toEqual(before);
    expect(sortEntries(result, "relevance")).toBe(result);
    s = reducer(s, {
      type: "browse-query",
      surface: "search",
      query: {
        ...browseQuery(s),
        view: {
          columns: [],
          widths: {},
          groupBy: "",
          sortKey: "name",
          listSort: "size",
        },
      },
    });
    expect(browseLocation(s).sort).toBe("size");
  });
});
