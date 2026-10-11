import { describe, expect, it } from "vitest";
import { batchItemPage, catalogueCandidatePage, planItemPage } from "./model";
import { directoryPage, directoryRow } from "../info-workspace/model";

// TestFPPlatformConsumers: scale acceptance (design: 万项上传/整理分页、
// 百万目录检索 fixture；多文件、原键恢复与空间切换晚响应用时序断言)。
//
// Pages are addressed page-by-page against deterministic fixtures — the
// million-entry catalogue and the 10,000-item batch/plan sets are never
// materialized as one array. Long-running scale cases get an explicit
// timeout so a regression fails loudly instead of silently hanging.

const TEN_THOUSAND = 10000;
const MILLION = 1000000;

describe("TestFPPlatformConsumers/scale", () => {
  it("walks a 10,000-item upload batch through bounded pages deterministically", () => {
    const seen = new Set<string>();
    let cursor = 0;
    let pages = 0;
    while (cursor < TEN_THOUSAND) {
      const page = batchItemPage(TEN_THOUSAND, cursor, 200);
      expect(page.length).toBeLessThanOrEqual(200);
      expect(page.length).toBeGreaterThan(0);
      for (const item of page) {
        // Keyset stability: no item repeats across pages (ID tiebreak).
        expect(seen.has(item.item_key)).toBe(false);
        seen.add(item.item_key);
      }
      cursor += page.length;
      pages++;
    }
    expect(seen.size).toBe(TEN_THOUSAND);
    expect(pages).toBe(50);
    // Deterministic rows: a re-read of any page is byte-identical, so the
    // fixture can back evidence without state.
    expect(batchItemPage(TEN_THOUSAND, 4600, 200)).toEqual(batchItemPage(TEN_THOUSAND, 4600, 200));
  }, 30000);

  it("reads 10,000-item organization plans in 200-item pages with per-item facts", () => {
    // The acceptance run: the whole plan is walkable in bounded memory while
    // per-item before/after and failure reasons stay attached to rows.
    const first = planItemPage(TEN_THOUSAND, 0, 200);
    const last = planItemPage(TEN_THOUSAND, 9800, 200);
    expect(first).toHaveLength(200);
    expect(last).toHaveLength(200);
    expect(first.every((item) => item.before && item.after && item.state)).toBe(true);
    // Failure reasons surface for retry decisions, not just counts.
    const failures = planItemPage(TEN_THOUSAND, 0, 200).filter((item) => item.state === "failed");
    expect(failures.length).toBeGreaterThan(0);
    expect(failures.every((item) => item.reason !== undefined)).toBe(true);
  }, 30000);

  it("links million-entry catalogue pagination with a bounded Top-K candidate page", () => {
    // Exact list: the shared million-directory fixture pages keyset rows.
    const page = directoryPage(MILLION, 999950, 50);
    expect(page).toHaveLength(50);
    expect(page[0]).toEqual(directoryRow(MILLION, 999950));
    // Semantic side: Top-K stays bounded at the contract limit regardless of
    // catalogue size — 20 candidates, never the whole catalogue.
    const candidates = catalogueCandidatePage(20);
    expect(candidates).toHaveLength(20);
    expect(candidates.every((item) => item.evidence_ref && item.index_generation)).toBe(true);
    // The two calibers are different facts: candidate count != coverage.
    expect(candidates.length).toBeLessThan(MILLION);
  }, 30000);
});
