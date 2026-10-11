import { describe, expect, it } from "vitest";
import { ORGANIZATION_PANEL_ANALYTICS } from "./model";
import {
  DRAFT_KEY,
  RevisionCache,
  migrateWorkspaceDraft,
  normalizeAnalyticsResult,
  normalizeBindingList,
  normalizeFileQueryResult,
  normalizeThreadList,
  readDraft,
  writeDraft,
} from "./compat";

describe("TestFPInformationWorkspace", () => {
  describe("compat: analytics payloads tolerate old services without inventing data", () => {
    it("accepts the frozen contract sample verbatim", () => {
      const sample = {
        backend: "clickhouse",
        generation: "GEN_ID",
        source_watermark: 12000,
        applied_watermark: 11980,
        authorization_epoch: 9,
        observed_at: "2026-09-25T00:00:00Z",
        coverage: "partial",
        partial: true,
        rows: [{ mime_type: "image/png", logical_bytes: 4096, file_count: 1 }],
        next_cursor: null,
      };
      expect(normalizeAnalyticsResult(sample)).toEqual(sample);
    });

    it("keeps missing optional fields absent and defaults honestly", () => {
      const result = normalizeAnalyticsResult({
        observed_at: "2026-09-25T00:00:00Z",
        rows: [],
      });
      expect(result?.coverage).toBe("unknown");
      expect(result?.backend).toBeUndefined();
      expect(result?.next_cursor).toBeNull();
      expect(result?.partial).toBeUndefined();
      // Unknown coverage is never coerced to complete.
      expect(result?.coverage).not.toBe("complete");
    });

    it("rejects payloads it cannot interpret", () => {
      expect(normalizeAnalyticsResult(null)).toBeNull();
      expect(normalizeAnalyticsResult("nope")).toBeNull();
      expect(normalizeAnalyticsResult({ rows: [] })).toBeNull();
      expect(normalizeAnalyticsResult({ observed_at: "x", rows: "nope" })).toBeNull();
    });

    // Drift D4: the live backend marshals an empty group-by as "rows":null;
    // an honestly empty observation is not an invalid response.
    it("normalizes a null rows aggregation to an empty list instead of failing", () => {
      const result = normalizeAnalyticsResult({
        query_id: "q-1",
        backend: "transactional",
        observed_at: "2026-09-26T05:14:27Z",
        coverage: "complete",
        partial: false,
        rows: null,
      });
      expect(result?.rows).toEqual([]);
      // A present-but-non-array rows value is still rejected, not guessed at.
      expect(normalizeAnalyticsResult({ observed_at: "x", rows: 7 })).toBeNull();
    });

  });

  describe("compat: file query results keep null totals as unknown", () => {
    it("accepts the frozen contract sample verbatim", () => {
      const sample = {
        mode: "exact",
        items: [{ file_id: "FILE_ID", version_id: "VERSION_ID" }],
        catalogue_revision: "CATALOGUE_REV",
        next_cursor: null,
        coverage: "complete",
        total_count: 1,
      };
      expect(normalizeFileQueryResult(sample)).toEqual(sample);
    });

    it("preserves an uncomputed total as null instead of zero", () => {
      const result = normalizeFileQueryResult({
        mode: "exact",
        items: [],
        next_cursor: null,
        coverage: "partial",
        total_count: null,
      });
      expect(result?.total_count).toBeNull();
      expect(result?.total_count).not.toBe(0);
    });

    it("drops malformed items rather than guessing identities", () => {
      const result = normalizeFileQueryResult({
        items: [{ file_id: "f-1" }, { file_id: "f-2", version_id: "v-2" }, "junk"],
      });
      expect(result?.items).toEqual([{ file_id: "f-2", version_id: "v-2" }]);
    });
  });

  describe("compat: thread and binding projections stay verbatim", () => {
    it("normalizes the frozen thread sample", () => {
      const sample = {
        thread_id: "THREAD_ID",
        file_id: "FILE_ID",
        version_id: "VERSION_ID",
        revision: 1,
        state: "open",
        author_kind: "user",
      };
      expect(normalizeThreadList([sample])).toEqual([sample]);
      expect(normalizeThreadList({ threads: [sample] })).toEqual([sample]);
      expect(normalizeThreadList([{ thread_id: "t" }])).toEqual([]);
    });

    it("accepts file-level anchors (empty version_id) as first-class threads", () => {
      // Drift D11: the contract's default anchor is the file itself and the
      // live list answers version_id:"" — that is a real thread, not junk.
      const fileLevel = {
        thread_id: "THREAD_FILE",
        file_id: "FILE_ID",
        version_id: "",
        revision: 1,
        state: "open",
        author_kind: "user",
      };
      expect(normalizeThreadList([fileLevel])).toEqual([fileLevel]);
      // Threads without an identity still never render.
      expect(normalizeThreadList([{ thread_id: "t" }])).toEqual([]);
    });

    it("keeps real content bindings by binding_id/file_ref/state; junk stays out", () => {
      // Drift D6: the implemented ContentBindingView carries
      // binding_id/file_ref/state (not file_id/version_id/status); the old
      // normalizer silently dropped every real binding the backend sent.
      expect(
        normalizeBindingList([
          {
            binding_id: "BINDING_ID",
            file_ref: "drivebridge://instance/file/FILE_ID/version/VERSION_ID",
            manju_connection_id: "conn-1",
            project_ref: "project-a",
            purpose: "indexing",
            state: "searchable",
            policy_revision: 4,
            index_generation: "GEN_ID",
            recovery_ref: "drivebridge://instance/file/FILE_ID/version/VERSION_ID",
          },
        ]),
      ).toEqual([
        {
          binding_id: "BINDING_ID",
          file_ref: "drivebridge://instance/file/FILE_ID/version/VERSION_ID",
          project_ref: "project-a",
          purpose: "indexing",
          state: "searchable",
        },
      ]);
      // Unknown states pass through verbatim instead of being dropped.
      expect(normalizeBindingList([{ binding_id: "b-2", state: "future-state" }])).toEqual([
        { binding_id: "b-2", file_ref: "", state: "future-state" },
      ]);
      // The pre-integration file_id/version_id/status shape is not the contract.
      expect(normalizeBindingList([{ file_id: "f", version_id: "v", status: "searchable" }])).toEqual([]);
    });
  });

  it("runs the organization panel query on live consistency with explicit typed selections", () => {
    // Drift D1/D2: omitted consistency defaults to observed (which this
    // deployment answers with 503 analytics_backend_unavailable) and empty
    // dimensions+metrics answers 400 invalid_query. A panel that needs the
    // precise current inventory always carries both, per the analytics
    // contract's fixed per-dataset vocabulary.
    expect(ORGANIZATION_PANEL_ANALYTICS).toEqual({
      dataset: "file_current",
      dimensions: ["mime_type"],
      metrics: ["logical_bytes", "file_count"],
      consistency: "live",
    });
  });

  describe("compat: workspace draft migration keeps references readable", () => {
    it("migrates a version-1 filter draft into the current shape", () => {
      const migrated = migrateWorkspaceDraft({
        space_id: "sp-1",
        from: "2026-08-26",
        to: "2026-09-25",
      });
      expect(migrated).toEqual({
        version: 2,
        filters: { space_id: "sp-1", from: "2026-08-26", to: "2026-09-25" },
      });
    });

    it("round-trips a current draft without losing its pinned reference", () => {
      const draft = {
        version: 2 as const,
        filters: { space_id: "sp-1", from: "2026-08-26", to: "2026-09-25" },
        panel: "transfer" as const,
        selected_file_ref: "drivebridge://instance/file/f-1/version/v-1",
        discussion: "请确认这个版本的标题。",
      };
      expect(migrateWorkspaceDraft(draft)).toEqual(draft);
    });

    it("discards drafts from unknown future versions instead of guessing", () => {
      expect(migrateWorkspaceDraft({ version: 3, filters: {} })).toBeNull();
      expect(migrateWorkspaceDraft("junk")).toBeNull();
    });

    it("reads and writes through storage with malformed payloads tolerated", () => {
      const storage = new Map<string, string>();
      const shim = {
        getItem: (key: string) => storage.get(key) ?? null,
        setItem: (key: string, value: string) => void storage.set(key, value),
        removeItem: (key: string) => void storage.delete(key),
      };
      writeDraft(shim, {
        version: 2,
        filters: { space_id: "sp-1", from: "", to: "" },
        discussion: "草稿",
      });
      expect(readDraft(shim)?.discussion).toBe("草稿");
      storage.set(DRAFT_KEY, "{not json");
      expect(readDraft(shim)).toBeNull();
      storage.set(DRAFT_KEY, JSON.stringify({ version: 9 }));
      expect(readDraft(shim)).toBeNull();
    });
  });

  describe("compat: the observation cache is scoped to one context revision", () => {
    it("serves entries only while the revision is unchanged", () => {
      const cache = new RevisionCache<{ space: string }>();
      cache.put(1, "organization", { space: "tenant-a" });
      expect(cache.get(1, "organization")).toEqual({ space: "tenant-a" });
      // A tenant/space switch bumps the revision; the old observation is gone.
      expect(cache.get(2, "organization")).toBeUndefined();
      cache.put(2, "organization", { space: "tenant-b" });
      expect(cache.get(2, "organization")).toEqual({ space: "tenant-b" });
      cache.drop("organization");
      expect(cache.get(2, "organization")).toBeUndefined();
    });

    it("drops stale revisions when new entries arrive", () => {
      const cache = new RevisionCache<number>();
      cache.put(1, "a", 1);
      cache.put(1, "b", 2);
      cache.put(2, "a", 3);
      expect(cache.get(1, "b")).toBeUndefined();
      expect(cache.get(2, "a")).toBe(3);
      cache.clear();
      expect(cache.get(2, "a")).toBeUndefined();
    });
  });
});
