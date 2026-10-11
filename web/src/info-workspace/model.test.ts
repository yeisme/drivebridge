import { describe, expect, it } from "vitest";
import {
  DEFAULT_PAGE_LIMIT,
  MAX_PAGE_LIMIT,
  clampLimit,
  canonicalQueryKey,
  decodeWorkspaceQuery,
  directoryPage,
  directoryRow,
  encodeWorkspaceQuery,
  errorViewState,
  isAbort,
  mergeViewState,
  pageCount,
  panelStateFromResult,
  sameCaliberDrilldown,
} from "./model";

const online = { online: true, hasPrevious: false };

describe("TestFPInformationWorkspace", () => {
  describe("model: view states follow the fixed contract vocabulary", () => {
    it("maps observations to states without inventing coverage", () => {
      expect(
        panelStateFromResult(
          { coverage: "complete", rows: [{ file_count: 1 }], partial: false },
          online,
        ),
      ).toBe("ready");
      expect(
        panelStateFromResult({ coverage: "partial", rows: [], partial: true }, online),
      ).toBe("partial");
      // An unrecognized coverage string must not upgrade itself to a full
      // observation.
      expect(
        panelStateFromResult({ coverage: "weird-future-mode", rows: [{}], partial: false }, online),
      ).toBe("partial");
      expect(
        panelStateFromResult({ coverage: "complete", rows: [], partial: false }, online),
      ).toBe("empty");
      expect(panelStateFromResult({ coverage: "complete", rows: [{}] }, { online: false, hasPrevious: false })).toBe(
        "offline",
      );
    });

    it("maps transport errors onto recovery states", () => {
      expect(errorViewState(new Error("forbidden"), online)).toBe("forbidden");
      expect(errorViewState(new Error("unsupported"), online)).toBe("unsupported");
      // dependency_unavailable keeps the previous observation as explicitly
      // stale instead of presenting the cache as realtime or failing hard.
      expect(
        errorViewState(new Error("dependency_unavailable"), { online: true, hasPrevious: true }),
      ).toBe("stale");
      expect(
        errorViewState(new Error("dependency_unavailable"), { online: true, hasPrevious: false }),
      ).toBe("error");
      expect(
        errorViewState(new Error("anything"), { online: false, hasPrevious: false }),
      ).toBe("offline");
      expect(errorViewState(new DOMException("Context changed", "AbortError"), online)).toBe("loading");
      expect(isAbort(new DOMException("Aborted", "AbortError"))).toBe(true);
    });

    it("aggregates panel states worst-first", () => {
      expect(mergeViewState(["ready", "partial", "empty"])).toBe("partial");
      expect(mergeViewState(["ready", "forbidden"])).toBe("forbidden");
      expect(mergeViewState(["ready", "ready"])).toBe("ready");
      expect(mergeViewState(["loading", "error"])).toBe("loading");
    });
  });

  describe("model: drilldown keeps the panel caliber", () => {
    it("carries only contract-sampled filters into the typed query", () => {
      const filters = { space_id: "sp-1", from: "2026-08-26", to: "2026-09-25" };
      expect(sameCaliberDrilldown(filters, "file_current")).toEqual({
        dataset: "file_current",
        filters: [{ field: "space_id", op: "eq", value: "sp-1" }],
      });
      expect(
        sameCaliberDrilldown(filters, "file_current", { field: "mime_type", value: "image/png" }).filters,
      ).toEqual([
        { field: "space_id", op: "eq", value: "sp-1" },
        { field: "mime_type", op: "eq", value: "image/png" },
      ]);
    });

    it("binds cursors to their canonical query", () => {
      const base = {
        mode: "exact" as const,
        space_ids: ["sp-1"],
        filters: [{ field: "mime_type", op: "eq" as const, value: "image/png" }],
        limit: 50,
      };
      expect(canonicalQueryKey(base)).toBe(canonicalQueryKey({ ...base }));
      // Different scope, filter, sort, or limit is a different cursor domain.
      expect(canonicalQueryKey({ ...base, space_ids: ["sp-2"] })).not.toBe(canonicalQueryKey(base));
      expect(canonicalQueryKey({ ...base, filters: [] })).not.toBe(canonicalQueryKey(base));
      expect(canonicalQueryKey({ ...base, sort: [{ field: "updated_at", direction: "desc" as const }] })).not.toBe(
        canonicalQueryKey(base),
      );
      expect(canonicalQueryKey({ ...base, limit: 200 })).not.toBe(canonicalQueryKey(base));
    });
  });

  describe("model: pagination contract", () => {
    it("defaults to 50 and caps at 200", () => {
      expect(DEFAULT_PAGE_LIMIT).toBe(50);
      expect(MAX_PAGE_LIMIT).toBe(200);
      expect(clampLimit(undefined)).toBe(50);
      expect(clampLimit(0)).toBe(50);
      expect(clampLimit(-5)).toBe(50);
      expect(clampLimit(Number.NaN)).toBe(50);
      expect(clampLimit(1)).toBe(1);
      expect(clampLimit(200)).toBe(200);
      expect(clampLimit(500)).toBe(200);
      expect(clampLimit(50.9)).toBe(50);
    });
  });

  describe("model: URL carries only safe filter and selection values", () => {
    it("round-trips filters, view, drill, and pinned file selection", () => {
      const state = {
        filters: { space_id: "sp-1", from: "2026-08-26", to: "2026-09-25" },
        panel: "transfer" as const,
        table: true,
        drill: "transfer_facts" as const,
        file: { id: "f-1", version: "v-1" },
      };
      expect(decodeWorkspaceQuery(encodeWorkspaceQuery(state))).toEqual(state);
    });

    it("drops unsafe, oversized, and unknown values", () => {
      const search = encodeWorkspaceQuery({
        filters: { space_id: "sp bad token=abc", from: "not-a-day", to: "2026-09-25" },
        table: false,
      });
      const decoded = decodeWorkspaceQuery(search);
      expect(decoded.filters.space_id).toBe("");
      expect(decoded.filters.from).toBe("");
      expect(decoded.filters.to).toBe("2026-09-25");
      const hostile = decodeWorkspaceQuery(
        "space=" + encodeURIComponent("x".repeat(201)) + "&token=secret&file=../../etc/passwd&panel=nonsense",
      );
      expect(hostile.filters.space_id).toBe("");
      expect(hostile.panel).toBeUndefined();
      expect(hostile.file?.id || "").not.toContain("/");
    });
  });

  describe("model: scale fixture addresses a million directories by page", () => {
    it("derives stable rows from the index without materializing the catalogue", () => {
      const total = 1_000_000;
      expect(directoryRow(total, 0)).toEqual(directoryRow(total, 0));
      expect(directoryRow(total, 0).file_id).not.toBe(directoryRow(total, 1).file_id);
      // Keyset order is the padded file id: strictly increasing.
      expect(directoryRow(total, 9).file_id < directoryRow(total, 10).file_id).toBe(true);
    });

    it("pages within the mounted-row budget at every offset", () => {
      const total = 1_000_000;
      expect(pageCount(total, 50)).toBe(20_000);
      expect(pageCount(total, 200)).toBe(5_000);
      for (const cursor of [0, 50, 999_950, 999_999, 1_000_000]) {
        const page = directoryPage(total, cursor, 50);
        expect(page.length).toBeLessThanOrEqual(200);
      }
      expect(directoryPage(total, 999_999, 50)).toHaveLength(1);
      // Every page is sorted by id with unique keys across page boundaries.
      const first = directoryPage(total, 0, 50);
      const second = directoryPage(total, 50, 50);
      expect(new Set([...first, ...second].map((r) => r.file_id)).size).toBe(100);
      expect(first[49].file_id < second[0].file_id).toBe(true);
    });

    it("keeps a ten-thousand-row batch within the page cap", () => {
      let mounted = 0;
      for (let cursor = 0; cursor < 10_000; cursor += 200) {
        mounted = Math.max(mounted, directoryPage(10_000, cursor, 200).length);
      }
      expect(mounted).toBeLessThanOrEqual(200);
    });
  });
});
