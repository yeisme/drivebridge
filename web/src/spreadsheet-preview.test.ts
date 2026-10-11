import { describe, expect, it } from "vitest";
import * as XLSX from "xlsx";
import { parseCSV, parseSpreadsheet } from "./spreadsheet-preview";
import {
  columnLabel,
  MAX_CELL_LENGTH,
  MAX_COLUMNS,
  MAX_ROWS,
  SPREADSHEET_LIMIT,
  readSpreadsheetResponse,
  type SpreadsheetOptions,
} from "./spreadsheet-source";
const options: SpreadsheetOptions = {
  format: "xlsx",
  sheetIndex: 0,
  delimiter: "auto",
  encoding: "utf-8",
};

function workbook() {
  const book = XLSX.utils.book_new();
  const sheet = XLSX.utils.aoa_to_sheet([
    ["编号", "金额", "日期"],
    ["001", 1234.5, 45292],
  ]);
  sheet.B2.z = "#,##0.00";
  sheet.C2.z = "yyyy-mm-dd";
  sheet.A3 = { t: "n", f: "SUM(B2:B2)", v: 1234.5, z: "0.00" };
  sheet["!ref"] = "A1:C3";
  XLSX.utils.book_append_sheet(book, sheet, "收入");
  XLSX.utils.book_append_sheet(
    book,
    XLSX.utils.aoa_to_sheet([["第二张表", "<script>bad()</script>"]]),
    "说明",
  );
  XLSX.utils.book_append_sheet(book, XLSX.utils.aoa_to_sheet([]), "空表");
  return XLSX.write(book, { type: "array", bookType: "xlsx" }) as ArrayBuffer;
}

describe("CSV preview parser", () => {
  it("preserves BOM, escaped quotes, embedded newlines, leading zeros and formulas as text", () => {
    const result = parseCSV(
      '\uFEFF编号,说明,值\r\n001,"含,逗号与""引号""\r\n第二行",=SUM(A1)\r\n',
    );
    expect(result.rows).toEqual([
      ["编号", "说明", "值"],
      ["001", '含,逗号与"引号"\r\n第二行', "=SUM(A1)"],
    ]);
    expect(result.rowCount).toBe(2);
    expect(result.columnCount).toBe(3);
  });
  it("detects semicolons and tabs, handles sep declarations and supports overrides", () => {
    expect(parseCSV('编号;金额\n001;"1,234"').delimiter).toBe(";");
    expect(parseCSV("a\tb\n1\t2").rows).toEqual([
      ["a", "b"],
      ["1", "2"],
    ]);
    expect(parseCSV("sep=;\r\na;b\r\n1;2").rows).toEqual([
      ["a", "b"],
      ["1", "2"],
    ]);
    expect(parseCSV("a,b;c", ";").rows).toEqual([["a,b", "c"]]);
  });
  it("keeps empty cells/blank rows without appending a phantom final row", () => {
    expect(parseCSV("a,b,\n,,\n\nx,y,").rows).toEqual([
      ["a", "b", ""],
      ["", "", ""],
      [""],
      ["x", "y", ""],
    ]);
    expect(parseCSV("").rowCount).toBe(0);
    expect(parseCSV('""').rows).toEqual([[""]]);
    expect(parseCSV('"a\n"').rows).toEqual([["a\n"]]);
  });
  it("reports broken quoted records", () => {
    expect(() => parseCSV('a,"unclosed')).toThrow("未闭合");
    expect(() => parseCSV('a,"closed"invalid')).toThrow("引号后的内容无效");
  });
  it("bounds rows, columns and cell text while retaining total dimensions", () => {
    const row = Array.from({ length: MAX_COLUMNS + 1 }, (_, i) => `c${i}`).join(
      ",",
    );
    const result = parseCSV(
      Array(MAX_ROWS + 5)
        .fill(row)
        .join("\n"),
    );
    expect(result.rows).toHaveLength(MAX_ROWS);
    expect(result.rows[0]).toHaveLength(MAX_COLUMNS);
    expect(result.rowCount).toBe(MAX_ROWS + 5);
    expect(result.columnCount).toBe(MAX_COLUMNS + 1);
    const long = parseCSV("a".repeat(MAX_CELL_LENGTH + 1));
    expect(long.rows[0][0]).toHaveLength(MAX_CELL_LENGTH);
    expect(long.clippedCells).toBe(true);
  });
  it("supports UTF16 BOM and GB18030 Chinese text", () => {
    const utf16 = new Uint8Array([0xff, 0xfe, 0x2d, 0x4e, 0x2c, 0, 0x31, 0]);
    expect(
      parseSpreadsheet(utf16.buffer, { ...options, format: "csv" }).rows,
    ).toEqual([["中", "1"]]);
    const gbk = new Uint8Array([0xd6, 0xd0, 0xce, 0xc4, 0x2c, 0x31]);
    expect(
      parseSpreadsheet(gbk.buffer, {
        ...options,
        format: "csv",
        encoding: "gb18030",
      }).rows,
    ).toEqual([["中文", "1"]]);
  });
});

describe("XLSX preview parser", () => {
  it("lists worksheets, preserves formatted numbers/dates and uses saved formula values", () => {
    const result = parseSpreadsheet(workbook(), options);
    expect(result.sheetNames).toEqual(["收入", "说明", "空表"]);
    expect(result.rows[1]).toEqual(["001", "1,234.50", "2024-01-01"]);
    expect(result.rows[2][0]).toBe("1234.50");
    expect(
      parseSpreadsheet(workbook(), { ...options, sheetIndex: 1 }).rows[0],
    ).toEqual(["第二张表", "<script>bad()</script>"]);
    expect(
      parseSpreadsheet(workbook(), { ...options, sheetIndex: 2 }).rows,
    ).toEqual([]);
  });
  it("bounds sparse worksheet dimensions without expanding every advertised cell", () => {
    const book = XLSX.utils.book_new();
    XLSX.utils.book_append_sheet(
      book,
      {
        A1: { t: "s", v: "start" },
        A2005: { t: "s", v: "last" },
        "!ref": "A1:CV2005",
      },
      "大表",
    );
    const result = parseSpreadsheet(
      XLSX.write(book, { type: "array", bookType: "xlsx" }),
      options,
    );
    expect(result.rowCount).toBe(2005);
    expect(result.columnCount).toBe(100);
    expect(result.rows).toHaveLength(MAX_ROWS);
    expect(result.rows[0]).toHaveLength(MAX_COLUMNS);
  });
  it("rejects non-XLSX bytes and over-limit input", () => {
    expect(() =>
      parseSpreadsheet(new TextEncoder().encode("not xlsx").buffer, options),
    ).toThrow("格式无效");
    expect(() =>
      parseSpreadsheet(new ArrayBuffer(SPREADSHEET_LIMIT + 1), options),
    ).toThrow("16 MiB");
  });
  it("provides spreadsheet column labels", () => {
    expect([0, 25, 26, 99].map(columnLabel)).toEqual(["A", "Z", "AA", "CV"]);
  });
});

describe("spreadsheet response bounds", () => {
  it("checks access failures and both advertised and streamed sizes", async () => {
    await expect(
      readSpreadsheetResponse(new Response("", { status: 403 })),
    ).rejects.toThrow("没有预览权限");
    await expect(
      readSpreadsheetResponse(
        new Response("", {
          headers: { "content-length": String(SPREADSHEET_LIMIT + 1) },
        }),
      ),
    ).rejects.toThrow("16 MiB");
    const stream = new ReadableStream({
      start(controller) {
        controller.enqueue(new Uint8Array(SPREADSHEET_LIMIT));
        controller.enqueue(new Uint8Array(1));
        controller.close();
      },
    });
    await expect(readSpreadsheetResponse(new Response(stream))).rejects.toThrow(
      "16 MiB",
    );
    expect(
      new TextDecoder().decode(
        await readSpreadsheetResponse(new Response("a,b\n1,2")),
      ),
    ).toBe("a,b\n1,2");
  });
});
