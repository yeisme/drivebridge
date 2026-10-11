import * as XLSX from "xlsx";
import {
  SPREADSHEET_LIMIT,
  MAX_ROWS,
  MAX_COLUMNS,
  MAX_CELL_LENGTH,
  type SpreadsheetData,
  type SpreadsheetOptions,
  type Delimiter,
} from "./spreadsheet-source";

function detectDelimiter(text: string): string {
  const candidates = [",", ";", "\t", "|"];
  const counts = candidates.map(() => [0]);
  let quoted = false,
    row = 0;
  for (let i = 0; i < text.length && row < 20; i++) {
    const c = text[i];
    if (c === '"') {
      if (quoted && text[i + 1] === '"') i++;
      else quoted = !quoted;
    } else if (!quoted) {
      if (c === "\r" || c === "\n") {
        if (c === "\r" && text[i + 1] === "\n") i++;
        row++;
        counts.forEach((count) => count.push(0));
      } else {
        const index = candidates.indexOf(c);
        if (index >= 0) counts[index][row]++;
      }
    }
  }
  // Prefer separators present on the greatest number of logical records.
  const score = counts.map(
    (count) =>
      count.filter((n) => n > 0).length * 1000 +
      Math.min(
        999,
        count.reduce((a, b) => a + b, 0),
      ),
  );
  return candidates[score.indexOf(Math.max(...score))];
}

export function parseCSV(
  input: string,
  delimiter: Delimiter = "auto",
): SpreadsheetData {
  let text = input.replace(/^\uFEFF/, "");
  const declaration = /^sep=([,;\t|])(?:\r\n|\n|\r)/i.exec(text);
  if (declaration) text = text.slice(declaration[0].length);
  const separator =
    delimiter === "auto"
      ? declaration?.[1] || detectDelimiter(text)
      : delimiter;
  const rows: string[][] = [];
  let row: string[] = [],
    field = "",
    column = 0,
    rowCount = 0,
    columnCount = 0;
  let quoted = false,
    closedQuote = false,
    fieldStarted = false,
    clippedCells = false;
  function append(c: string) {
    if (rowCount < MAX_ROWS && column < MAX_COLUMNS) {
      if (field.length < MAX_CELL_LENGTH) field += c;
      else clippedCells = true;
    }
  }
  function finishField() {
    if (rowCount < MAX_ROWS && column < MAX_COLUMNS) row.push(field);
    column++;
    field = "";
    fieldStarted = false;
    closedQuote = false;
  }
  function finishRow() {
    finishField();
    columnCount = Math.max(columnCount, column);
    if (rowCount < MAX_ROWS) rows.push(row);
    rowCount++;
    row = [];
    column = 0;
  }
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (quoted) {
      if (c === '"') {
        if (text[i + 1] === '"') {
          append('"');
          i++;
        } else {
          quoted = false;
          closedQuote = true;
        }
      } else append(c);
      continue;
    }
    if (c === separator) finishField();
    else if (c === "\r" || c === "\n") {
      finishRow();
      if (c === "\r" && text[i + 1] === "\n") i++;
    } else if (c === '"' && !fieldStarted) {
      quoted = true;
      fieldStarted = true;
    } else {
      if (closedQuote && c !== " ")
        throw new Error("CSV 引号后的内容无效，请检查分隔符或原文件。");
      fieldStarted = true;
      append(c);
    }
  }
  if (quoted) throw new Error("CSV 引号未闭合，请检查原文件。");
  if ((text.length && !/[\r\n]$/.test(text)) || fieldStarted || column > 0)
    finishRow();
  return {
    sheetNames: ["数据"],
    sheetIndex: 0,
    rows,
    rowCount,
    columnCount,
    clippedCells,
    delimiter: separator,
  };
}

export function parseSpreadsheet(
  buffer: ArrayBuffer,
  options: SpreadsheetOptions,
): SpreadsheetData {
  if (buffer.byteLength > SPREADSHEET_LIMIT)
    throw new Error("表格超过 16 MiB，请下载查看。");
  const bytes = new Uint8Array(buffer);
  if (options.format !== "xlsx") {
    const bomEncoding =
      bytes[0] === 0xff && bytes[1] === 0xfe
        ? "utf-16le"
        : bytes[0] === 0xfe && bytes[1] === 0xff
          ? "utf-16be"
          : options.encoding;
    const text = new TextDecoder(bomEncoding).decode(bytes);
    return parseCSV(
      text,
      options.delimiter === "auto" && options.format === "tsv"
        ? "\t"
        : options.delimiter,
    );
  }
  // Reject unrelated formats instead of letting SheetJS interpret arbitrary text as a workbook.
  if (bytes[0] !== 0x50 || bytes[1] !== 0x4b)
    throw new Error(
      "XLSX 文件损坏、已加密或格式无效，请下载后使用表格软件打开。",
    );
  const names = XLSX.read(bytes, {
    type: "array",
    bookSheets: true,
  }).SheetNames;
  const sheetIndex = Math.max(
    0,
    Math.min(options.sheetIndex, names.length - 1),
  );
  if (!names.length)
    return {
      sheetNames: [],
      sheetIndex: 0,
      rows: [],
      rowCount: 0,
      columnCount: 0,
      clippedCells: false,
    };
  const workbook = XLSX.read(bytes, {
    type: "array",
    sheets: sheetIndex,
    sheetRows: MAX_ROWS,
    cellHTML: false,
    cellStyles: false,
    cellNF: false,
    cellFormula: true,
    cellText: true,
    bookVBA: false,
  });
  const sheet = workbook.Sheets[names[sheetIndex]];
  const ref = sheet?.["!fullref"] || sheet?.["!ref"];
  if (!ref)
    return {
      sheetNames: names,
      sheetIndex,
      rows: [],
      rowCount: 0,
      columnCount: 0,
      clippedCells: false,
    };
  const range = XLSX.utils.decode_range(ref);
  const rowCount = range.e.r + 1,
    columnCount = range.e.c + 1;
  const rows: string[][] = [];
  let clippedCells = false;
  for (let r = 0; r < Math.min(rowCount, MAX_ROWS); r++) {
    const row: string[] = [];
    for (let c = 0; c < Math.min(columnCount, MAX_COLUMNS); c++) {
      const cell = sheet[XLSX.utils.encode_cell({ r, c })];
      const value = cell
        ? cell.v == null && cell.f
          ? `=${cell.f}`
          : XLSX.utils.format_cell(cell)
        : "";
      if (value.length > MAX_CELL_LENGTH) clippedCells = true;
      row.push(value.slice(0, MAX_CELL_LENGTH));
    }
    rows.push(row);
  }
  return {
    sheetNames: names,
    sheetIndex,
    rows,
    rowCount,
    columnCount,
    clippedCells,
  };
}
