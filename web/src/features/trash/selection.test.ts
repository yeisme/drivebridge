import { expect, it } from "vitest";
import type { DriveFile } from "../../api";
import { restorableSelection } from "./selection";

const file: DriveFile = { id: "a", space_id: "space-a", path: "fixture.txt", version_id: "version-a", mime_type: "text/plain", size_bytes: 1, trashed: true, updated_at: "" };
it("rejects stale, other-space and non-trash selections and deduplicates IDs", () => {
  const files = [file, file, { ...file, id: "b", space_id: "space-b" }, { ...file, id: "c", trashed: false }];
  expect(restorableSelection(files, { a: true, b: true, c: true, missing: true }, "space-a")).toEqual(["a"]);
  expect(restorableSelection(files, { a: true }, "space-b")).toEqual([]);
  expect(restorableSelection(files, { a: false }, "space-a")).toEqual([]);
  expect(restorableSelection([], { a: true }, "space-a")).toEqual([]);
});
