import { afterEach, expect, it, vi } from "vitest";
import { queryFiles, statFile } from "../../info-workspace/client";
import { readKnowledgeFiles } from "./knowledge-files";
import type { DriveFile } from "../../api";
vi.mock("../../info-workspace/client", () => ({ queryFiles: vi.fn(), statFile: vi.fn() }));
afterEach(() => vi.resetAllMocks());
const file = (id: string, path = `${id}.md`): DriveFile => ({ id, path, version_id: "v1", space_id: "s", mime_type: "text/markdown", size_bytes: 12, trashed: false, updated_at: "" });
it("reads bounded metadata with opaque cursor and never substitutes a changed revision", async () => {
  vi.mocked(queryFiles).mockResolvedValue({ mode: "exact", items: [{ file_id: "a", version_id: "v1" }, { file_id: "b", version_id: "old" }], next_cursor: "opaque-next" });
  vi.mocked(statFile).mockImplementation(async id => file(id));
  const signal = new AbortController().signal;
  const result = await readKnowledgeFiles("s", "opaque-current", signal);
  expect(queryFiles).toHaveBeenCalledWith(expect.objectContaining({ cursor: "opaque-current", limit: 50, space_ids: ["s"] }), signal);
  expect(result.entries.map(entry => entry.ref.objectId)).toEqual(["a"]);
  expect(result).toMatchObject({ stale: 1, scanned: 2, nextCursor: "opaque-next" });
});
it("rejects foreign sources and identifies partial reads without exposing missing names", async () => {
  vi.mocked(queryFiles).mockResolvedValue({ mode: "exact", items: ["foreign", "missing", "binary"].map(file_id => ({ file_id, version_id: "v1" })), next_cursor: null });
  vi.mocked(statFile).mockImplementation(async id => {
    if (id === "missing") throw Object.assign(new Error("not_found"), { status: 404 });
    return id === "foreign" ? { ...file(id), space_id: "other" } : { ...file(id, "photo.png"), mime_type: "image/png" };
  });
  expect(await readKnowledgeFiles("s", null, new AbortController().signal)).toMatchObject({ entries: [], unavailable: 2, scanned: 3 });
});
it("fails closed on revoked authorization instead of retaining a successful sibling", async () => {
  vi.mocked(queryFiles).mockResolvedValue({ mode: "exact", items: ["a", "b"].map(file_id => ({ file_id, version_id: "v1" })), next_cursor: null });
  vi.mocked(statFile).mockImplementation(async id => {
    if (id === "b") throw Object.assign(new Error("forbidden"), { status: 403 });
    return file(id);
  });
  await expect(readKnowledgeFiles("s", null, new AbortController().signal)).rejects.toThrow("forbidden");
});
