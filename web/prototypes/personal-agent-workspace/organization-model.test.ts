import { describe, it, expect } from "vitest";
import { initialState, reducer } from "./model";
import {
  entryRef,
  bookmarkFor,
  currentBookmarks,
  findEntries,
  propertyState,
  conditionMatches,
  pageEntries,
} from "./organization-model";
import {
  readProperties,
  patchProperty,
  documentTags,
  bodyWithoutProperties,
} from "./properties";
import { emptySearch } from "./cloud-model";
import { noteBody } from "./knowledge-model";
const init = () => reducer(initialState(), { type: "k-seed" });
describe("v8 YAML source authority", () => {
  it("patches only value while preserving comments unknown data and CRLF", () => {
    const body =
      "---\r\nproject: 山野 # keep\r\nunknown: {a: 1, b: [2]}\r\nempty:\r\n---\r\n# Title\r\nunchanged";
    const next = patchProperty(body, "project", "城市");
    expect(next).toBe(body.replace("山野", "城市"));
    expect(
      readProperties(next).fields.find((f) => f.key === "unknown")?.complex,
    ).toBe(true);
    expect(patchProperty(body, "empty", false)).toContain("empty: false");
    expect(bodyWithoutProperties(next)).toBe("# Title\r\nunchanged");
  });
  it("handles list block ranges and new keys without swallowing next line", () => {
    const before = "---\ntags:\n  - a\n  - b\nother: 0 # preserve\n---\nbody";
    const after = patchProperty(before, "tags", ["c", "d"]);
    expect(readProperties(after).valid).toBe(true);
    expect(after).toContain("other: 0 # preserve");
    expect(
      readProperties(after).fields.find((f) => f.key === "tags")?.value,
    ).toEqual(["c", "d"]);
    expect(patchProperty(before, "new", true)).toContain(
      "new: true\n---\nbody",
    );
    expect(patchProperty(before, "tags", undefined)).toBe(
      "---\nother: 0 # preserve\n---\nbody",
    );
  });
  it("rejects malformed YAML duplicate keys complex and aliases without rewriting", () => {
    for (const body of [
      "---\na: 1\na: 2\n---\nbody",
      "---\na: [\n---\nbody",
      "---\na: 1",
    ]) {
      expect(readProperties(body).valid).toBe(false);
      expect(() => patchProperty(body, "a", 3)).toThrow();
    }
    expect(() =>
      patchProperty("---\na: {b: 1}\n---\nbody", "a", "x"),
    ).toThrow();
    expect(
      readProperties("---\na: &x [one]\nb: *x\n---\nbody").fields.every(
        (f) => f.complex,
      ),
    ).toBe(true);
  });
  it("extracts hierarchical tags excluding headings code and links", () => {
    const body =
      "---\ntags: [项目/山野, 研究]\n---\n# Title\nText #项目/山野/拍摄 #99abc\n`#ignore`\n```md\n#ignore2\n```\n[link](https://site/#ignore3)\n#123";
    expect(documentTags(body).map((t) => t.name)).toEqual([
      "项目/山野",
      "研究",
      "项目/山野/拍摄",
      "99abc",
    ]);
  });
  it("shares property and source saves and preserves unknown fields", () => {
    let s = init(),
      v = s.files.find((f) => f.id === "k-guide")!.version;
    s = reducer(s, {
      type: "property-set",
      id: "k-guide",
      key: "status",
      value: "已完成",
      version: v,
    });
    expect(s.knowledge.notes["k-guide"].dirty).toBe(true);
    expect(s.knowledge.notes["k-guide"].draft).toContain(
      "custom: 保留这个未知字段 # 示例注释",
    );
    s = reducer(s, { type: "k-save", id: "k-guide" });
    expect(noteBody(s, "k-guide")).toContain("status: 已完成");
    expect(s.files.find((f) => f.id === "k-guide")!.version).toBe(v + 1);
  });
  it("invalid YAML is not absent, source can still save", () => {
    let s = init();
    s = reducer(s, {
      type: "k-edit",
      id: "k-guide",
      body: "---\nx: 1\nx: 2\n---\nbody",
    });
    const before = s.knowledge.notes["k-guide"].draft;
    s = reducer(s, {
      type: "property-set",
      id: "k-guide",
      key: "status",
      value: "done",
      version: 1,
    });
    expect(s.knowledge.notes["k-guide"].draft).toBe(before);
    s = reducer(s, { type: "k-save", id: "k-guide" });
    expect(
      propertyState(s, s.files.find((f) => f.id === "k-guide")!).status,
    ).toBe("invalid");
  });
  it("Skill structured fields and source have one truth", () => {
    let s = init();
    s = reducer(s, { type: "k-check", id: "k-skill" });
    expect(s.knowledge.resources[0].checkedVersion).toBe(1);
    s = reducer(s, {
      type: "k-skill-fields",
      id: "k-skill",
      name: "Changed",
      description: "New",
      tools: ["files.read"],
    });
    expect(noteBody(s, "k-skill")).toContain("name: Changed");
    expect(noteBody(s, "k-skill")).toContain("compatibility: demo-only");
    expect(s.knowledge.resources[0].checkedVersion).toBeUndefined();
  });
});
describe("v8 private provider bookmarks and query", () => {
  it("migrates legacy stars and isolates actor space connection identity", () => {
    let s = init();
    const ref = entryRef(s, "k-guide")!;
    s = reducer(s, { type: "bookmark-add", refs: [ref] });
    expect(bookmarkFor(s, ref)).toBeTruthy();
    s = { ...s, actor: "member-lin" };
    expect(bookmarkFor(s, ref)).toBeUndefined();
    s = { ...s, actor: "yeisme" };
    expect(
      bookmarkFor(s, { ...ref, connectionId: "onedrive-work" }),
    ).toBeUndefined();
    expect(bookmarkFor(s, { ...ref, space: "team" })).toBeUndefined();
  });
  it("multiple groups and delete group never mutate files", () => {
    let s = init();
    const ref = entryRef(s, "k-guide")!;
    s = reducer(s, {
      type: "bookmark-group",
      connectionId: "drive",
      name: "A",
    });
    s = reducer(s, {
      type: "bookmark-group",
      connectionId: "drive",
      name: "B",
    });
    s = reducer(s, {
      type: "bookmark-add",
      refs: [ref],
      groups: s.organization.groups.map((g) => g.id),
    });
    const files = structuredClone(s.files),
      g = s.organization.groups[0];
    s = reducer(s, {
      type: "bookmark-group",
      connectionId: "drive",
      id: g.id,
      remove: true,
    });
    expect(bookmarkFor(s, ref)?.groups).toHaveLength(1);
    expect(s.files).toEqual(files);
    s = reducer(s, {
      type: "bookmark-group",
      connectionId: "drive",
      id: s.organization.groups[0].id,
      remove: true,
    });
    expect(bookmarkFor(s, ref)?.groups).toEqual([]);
  });
  it("favorite directory does not favorite descendants", () => {
    let s = init();
    const f = s.files.find((f) => f.id === "k-guide")!,
      ref = entryRef(s, f.directoryId!, "directory")!;
    s = reducer(s, { type: "bookmark-add", refs: [ref] });
    const q = { ...emptySearch(), query: "山野知识", onlyFavorites: true };
    expect(findEntries(s, q)).toHaveLength(0);
    expect(
      findEntries(s, {
        ...q,
        onlyFavorites: false,
        favoriteDirectories: true,
      }).some((r) => r.ref.id === f.id),
    ).toBe(true);
  });
  it("does not rebind missing identity by same name", () => {
    let s = init();
    const ref = entryRef(s, "k-guide")!;
    s = reducer(s, { type: "bookmark-add", refs: [ref] });
    s.files = s.files.filter((f) => f.id !== ref.id);
    const found = findEntries(s, {
      ...emptySearch(),
      query: "山野知识",
      onlyFavorites: true,
    });
    expect(found[0]?.available).toContain("删除");
    expect(found[0]?.ref.id).toBe(ref.id);
  });
  it("matches zero false empty unset and never matches parse errors", () => {
    const s = init(),
      f = s.files.find((f) => f.id === "k-guide")!;
    expect(
      conditionMatches(s, f, { key: "priority", op: "eq", value: 0 }),
    ).toBe(true);
    expect(
      conditionMatches(s, f, { key: "reviewed", op: "eq", value: false }),
    ).toBe(true);
    expect(conditionMatches(s, f, { key: "priority", op: "empty" })).toBe(
      false,
    );
    expect(conditionMatches(s, f, { key: "x", op: "unset" })).toBe(true);
    s.knowledge.notes[f.id].draft = "---\nx: [\n---\n";
    expect(conditionMatches(s, f, { key: "x", op: "unset" })).toBe(false);
  });
  it("hierarchical tag defaults descendants and excludes unaccepted suggestions", () => {
    const s = init();
    const q = { ...emptySearch(), documentTag: "项目/山野" };
    expect(
      findEntries(s, q)
        .filter((r) => r.file)
        .map((r) => r.ref.id),
    ).toContain("k-research");
    expect(
      findEntries(s, { ...q, exactTag: true }).map((r) => r.ref.id),
    ).not.toContain("k-research");
  });
  it("batch freezes versions, resumes original incomplete items only", () => {
    let s = init();
    s = reducer(s, {
      type: "property-batch",
      ids: ["k-guide", "k-research", "k-checklist"],
      key: "status",
      value: "已完成",
    });
    const id = s.organization.batch!.id;
    s = reducer(s, { type: "property-batch-run", id, partial: true });
    expect(s.organization.batch!.items.map((i) => i.status)).toEqual([
      "done",
      "failed",
      "done",
    ]);
    const version = s.files.find((f) => f.id === "k-guide")!.version;
    s = reducer(s, { type: "property-batch-run", id });
    expect(s.organization.batch!.items.every((i) => i.status === "done")).toBe(
      true,
    );
    expect(s.files.find((f) => f.id === "k-guide")!.version).toBe(version);
  });
  it("conflicting new version is never overwritten", () => {
    let s = init();
    s = reducer(s, {
      type: "property-batch",
      ids: ["k-guide"],
      key: "status",
      value: "已完成",
    });
    s = reducer(s, { type: "k-edit", id: "k-guide", body: "# changed" });
    s = reducer(s, { type: "k-save", id: "k-guide" });
    s = reducer(s, {
      type: "property-batch-run",
      id: s.organization.batch!.id,
    });
    expect(s.organization.batch!.items[0].status).toBe("failed");
    expect(noteBody(s, "k-guide")).toBe("# changed");
  });
  it("paged projection caps mounted candidates independent of catalog volume", () => {
    const rows = Array.from({ length: 11000 }, (_, i) => i);
    expect(pageEntries(rows, 1)).toHaveLength(50);
    expect(pageEntries(rows, 201)).toEqual(rows.slice(10000, 10050));
  });
});

it("v8 explicit key migration previews and updates saved view keys only on complete", () => {
  let s = init();
  s = reducer(s, {
    type: "search-patch",
    patch: { propertyConditions: [{ key: "priority", op: "eq", value: 0 }] },
  });
  s = reducer(s, { type: "search-save", name: "zero" });
  s = reducer(s, {
    type: "property-migrate",
    from: "priority",
    to: "rank",
    definition: { key: "rank", label: "排序", type: "text", group: "组织" },
  });
  const b = s.organization.batch!;
  expect(noteBody(s, "k-guide")).toContain("priority: 0");
  expect(b.items[0].after).toContain('rank: "0"');
  s = reducer(s, { type: "property-batch-run", id: b.id });
  expect(noteBody(s, "k-guide")).not.toContain("priority:");
  expect(
    readProperties(noteBody(s, "k-guide")).fields.find((f) => f.key === "rank")
      ?.value,
  ).toBe("0");
  expect(s.savedSearches[0].spec.propertyConditions?.[0].key).toBe("rank");
});
it("v8 extractor remains separately authorized and stale generations are ignored", () => {
  let s = init();
  const id = s.files.find((f) => f.id === "k-guide")!.directoryId!;
  expect(s.organization.extraction[id]).toBeUndefined();
  s = reducer(s, {
    type: "property-extract",
    directoryId: id,
    bodyTags: false,
  });
  const gen = s.organization.extraction[id].generation;
  s = reducer(s, {
    type: "property-extract",
    directoryId: id,
    bodyTags: false,
    pause: true,
  });
  const projections = structuredClone(s.organization.projections);
  s = reducer(s, {
    type: "property-extract",
    directoryId: id,
    bodyTags: true,
    generation: gen,
  });
  expect(s.organization.extraction[id].status).toBe("paused");
  expect(s.organization.projections).toEqual(projections);
});
it("v8 large fixture constructs one requested page and its parent nodes", async () => {
  const { largeCatalogPage } = await import("./large-catalog");
  const s = init();
  const dirs = largeCatalogPage(s, null, 1);
  expect(dirs.total).toBe(1000);
  expect(dirs.directories).toHaveLength(50);
  expect(dirs.files).toHaveLength(0);
  const results = largeCatalogPage(s, null, 100, "研究资料");
  expect(results.total).toBe(10000);
  expect(results.files).toHaveLength(50);
  expect(results.directories).toHaveLength(5);
  expect(results.files[0].name).toContain("04951");
});

it("v8 non Markdown metadata revisions prevent stale batch overwrites", () => {
  let s = init();
  const f = s.files.find((f) => f.id === "f1")!;
  s = reducer(s, {
    type: "property-batch",
    ids: [f.id],
    key: "status",
    value: "old-plan",
  });
  const id = s.organization.batch!.id;
  s = reducer(s, {
    type: "property-set",
    id: f.id,
    key: "status",
    value: "newer",
    version: f.version,
  });
  s = reducer(s, { type: "property-batch-run", id });
  expect(s.organization.batch!.items[0].status).toBe("failed");
  expect(s.organization.metadata[f.id].status).toBe("newer");
});
it("v8 stable managed identity updates bookmark labels but S3 moves need relocation", () => {
  let s = init();
  s = reducer(s, { type: "bookmark-add", refs: [entryRef(s, "f1")!] });
  s.files.find((f) => f.id === "f1")!.name = "Changed.pdf";
  s = reducer(s, { type: "notice", text: "refresh" });
  expect(bookmarkFor(s, entryRef(s, "f1")!)?.name).toBe("Changed.pdf");
  const f = s.files.find((f) => f.connectionId === "s3")!;
  s.space = f.space;
  s = reducer(s, { type: "bookmark-add", refs: [entryRef(s, f.id)!] });
  s.files.find((x) => x.id === f.id)!.name = "Other.pdf";
  s = reducer(s, { type: "notice", text: "refresh" });
  expect(bookmarkFor(s, entryRef(s, f.id)!)?.invalid).toContain("重新定位");
});
it("v8 merging frontmatter tags keeps body tags and changes saved query deliberately", () => {
  let s = init();
  s = reducer(s, { type: "search-patch", patch: { documentTag: "项目/山野" } });
  s = reducer(s, {
    type: "document-tag-merge",
    from: "项目/山野",
    to: "项目/自然",
  });
  expect(s.organization.batch!.items).toHaveLength(2);
  s = reducer(s, { type: "property-batch-run", id: s.organization.batch!.id });
  expect(noteBody(s, "k-research")).toContain("项目/自然/拍摄");
  expect(noteBody(s, "k-research")).toContain("#主题/光线");
  expect(s.search.documentTag).toBe("项目/自然");
});
it("v8 metadata reserved names cannot alter object prototypes", () => {
  let s = init();
  s = reducer(s, {
    type: "property-set",
    id: "f1",
    key: "__proto__",
    value: ["bad"],
    version: s.files.find((f) => f.id === "f1")!.version,
  });
  expect(s.organization.metadata.f1).toBeUndefined();
  expect(() => patchProperty("body", "__proto__", ["bad"])).toThrow();
});
it("v8 a new Skill starts with one YAML authority and extra fields follow source", () => {
  let s = init();
  s = reducer(s, { type: "k-resource-new", kind: "skill", name: "新的 Skill" });
  const r = s.knowledge.resources.at(-1)!;
  expect(r.name).toBe("新的 Skill");
  expect(noteBody(s, r.fileId)).toContain("name: 新的 Skill");
  const body = patchProperty(noteBody(s, r.fileId), "custom", "kept");
  s = reducer(s, { type: "k-edit", id: r.fileId, body });
  expect(s.knowledge.resources.at(-1)!.extra.custom).toBe("kept");
});
