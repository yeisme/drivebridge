import { describe, it, expect } from "vitest";
import {
  defaultSheetPage,
  sheetPage,
  sheetSamples,
  sheetWidth,
  sheetColumn,
} from "./sheet-preview-model";
import { safePreferences, defaultPreferences } from "./cloud-model";
describe("v9.1 bounded read-only sheet model", () => {
  it("keeps defaults compact and maps old inspector preference without domain state", () => {
    expect(defaultPreferences.previewListWidth).toBe(220);
    expect(
      safePreferences({ previewListWidth: 480, inspectorPosition: "bottom" }),
    ).toMatchObject({ previewListWidth: 320, inspectorPosition: "auto" });
    expect(
      safePreferences({ previewListWidth: 100, inspectorPosition: "right" }),
    ).toMatchObject({ previewListWidth: 200, inspectorPosition: "right" });
  });
  it("provides fixed version values, long text, wide and empty fixtures", () => {
    expect(sheetSamples(1)[0].row(3)[2]).toBe(734);
    expect(sheetSamples(2)[0].row(3)[2]).toBe(794);
    const long = sheetSamples(2).find((s) => s.id === "notes")!;
    expect(
      sheetPage(long, { ...defaultSheetPage(), query: "不会自动换行" }).rows[0]
        .cells[3],
    ).toBe(long.row(0)[3]);
    expect(sheetSamples(2).find((s) => s.id === "wide")!.columns).toHaveLength(
      20,
    );
    expect(
      sheetPage(
        sheetSamples(2).find((s) => s.id === "empty")!,
        defaultSheetPage(),
      ).rows,
    ).toEqual([]);
  });
  it("bounds page materialization, preserves original row identities and clamps pages", () => {
    const sheet = sheetSamples(2).find((s) => s.id === "ledger")!;
    let result = sheetPage(sheet, { ...defaultSheetPage(), page: 2 });
    expect(result.rows).toHaveLength(50);
    expect(result.rows[0].index).toBe(50);
    expect(result.total).toBe(1000);
    expect(result.pages).toBe(20);
    expect(sheetPage(sheet, { ...defaultSheetPage(), page: 99 }).page).toBe(20);
    result = sheetPage(sheet, { ...defaultSheetPage(), query: "明细 0999" });
    expect(result.rows).toHaveLength(1);
    expect(result.rows[0].index).toBe(998);
    expect(result.rows[0].cells[0]).toBe("明细 0999");
  });
  it("bounds column widths and maps stable cell addresses", () => {
    expect(sheetWidth(-1)).toBe(72);
    expect(sheetWidth(999)).toBe(480);
    expect(sheetColumn(0)).toBe("A");
    expect(sheetColumn(26)).toBe("AA");
  });
});
