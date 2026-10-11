import { parseSpreadsheet } from "./spreadsheet-preview";
import type { SpreadsheetOptions } from "./spreadsheet-source";

self.onmessage = (
  event: MessageEvent<{ buffer: ArrayBuffer; options: SpreadsheetOptions }>,
) => {
  try {
    self.postMessage({
      data: parseSpreadsheet(event.data.buffer, event.data.options),
    });
  } catch (error) {
    const message =
      error instanceof Error && /^(CSV |XLSX |表格)/.test(error.message)
        ? error.message
        : "表格解析失败，文件可能损坏、已加密或格式不受支持。";
    self.postMessage({ error: message });
  }
};
