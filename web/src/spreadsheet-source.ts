export const SPREADSHEET_LIMIT = 16 * 1024 * 1024;
export const MAX_ROWS = 2000;
export const MAX_COLUMNS = 100;
export const MAX_CELL_LENGTH = 10000;
export const PAGE_ROWS = 100;
export type SpreadsheetFormat = "xlsx" | "csv" | "tsv";
export type Delimiter = "auto" | "," | ";" | "\t" | "|";
export type Encoding = "utf-8" | "gb18030";
export interface SpreadsheetOptions {
  format: SpreadsheetFormat;
  sheetIndex: number;
  delimiter: Delimiter;
  encoding: Encoding;
}
export interface SpreadsheetData {
  sheetNames: string[];
  sheetIndex: number;
  rows: string[][];
  rowCount: number;
  columnCount: number;
  clippedCells: boolean;
  delimiter?: string;
}

export function spreadsheetFormat(file: {
  path: string;
  mime_type: string;
}): SpreadsheetFormat | undefined {
  const ext = file.path.split(".").pop()?.toLowerCase();
  const mime = file.mime_type.split(";")[0].trim().toLowerCase();
  if (
    ext === "xlsx" ||
    mime === "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
  )
    return "xlsx";
  if (ext === "tsv" || mime === "text/tab-separated-values") return "tsv";
  if (ext === "csv" || mime === "text/csv" || mime === "application/csv")
    return "csv";
}

export function columnLabel(index: number): string {
  let label = "";
  for (let n = index + 1; n > 0; n = Math.floor((n - 1) / 26))
    label = String.fromCharCode(65 + ((n - 1) % 26)) + label;
  return label;
}

export async function readSpreadsheetResponse(
  response: Response,
): Promise<ArrayBuffer> {
  if (!response.ok) {
    if (response.status === 401 || response.status === 403)
      throw new Error("访问已失效或没有预览权限，请重新登录后再试。");
    if (response.status === 404) throw new Error("文件或版本已不存在。");
    throw new Error("表格加载失败，请检查连接后重试。");
  }
  const tooLarge = () => new Error("表格超过 16 MiB，请下载查看。");
  if (Number(response.headers.get("content-length")) > SPREADSHEET_LIMIT) {
    await response.body?.cancel();
    throw tooLarge();
  }
  if (!response.body) {
    const buffer = await response.arrayBuffer();
    if (buffer.byteLength > SPREADSHEET_LIMIT) throw tooLarge();
    return buffer;
  }
  const reader = response.body.getReader();
  const chunks: Uint8Array[] = [];
  let length = 0;
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      length += value.byteLength;
      if (length > SPREADSHEET_LIMIT) {
        await reader.cancel();
        throw tooLarge();
      }
      chunks.push(value);
    }
  } finally {
    reader.releaseLock();
  }
  const buffer = new Uint8Array(length);
  let offset = 0;
  for (const chunk of chunks) {
    buffer.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return buffer.buffer;
}
