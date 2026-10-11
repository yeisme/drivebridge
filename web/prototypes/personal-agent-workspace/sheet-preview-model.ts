// Private read-only fixtures. No workbook parser, provider IO, formula or script execution.
export interface SheetPageView {
  page: number;
  query: string;
  scrollTop: number;
  scrollLeft: number;
  widths: Record<number, number>;
  cell?: { row: number; column: number };
}
export interface SheetReading {
  active: string;
  state: "ready" | "loading" | "failed";
  sheets: Record<string, SheetPageView>;
}
export interface SheetSample {
  id: string;
  name: string;
  columns: string[];
  widths: number[];
  count: number;
  row: (index: number) => Array<string | number>;
}
export const defaultSheetPage = (): SheetPageView => ({
  page: 1,
  query: "",
  scrollTop: 0,
  scrollLeft: 0,
  widths: {},
});
export const defaultSheetReading = (): SheetReading => ({
  active: "expenses",
  state: "ready",
  sheets: {},
});
export const sheetPageSize = 50;
export function sheetSamples(version: number): SheetSample[] {
  const rows: Array<Array<string | number>> = [
    ["交通", "09/21", 248, "往返"],
    ["住宿", "09/21", version === 1 ? 360 : 420, "山间小屋"],
    ["餐食", "09/22", 126, "两人"],
    ["合计", "—", version === 1 ? 734 : 794, "已核对"],
  ];
  return [
    {
      id: "expenses",
      name: "旅行收支",
      columns: ["项目", "日期", "支出", "备注"],
      widths: [144, 104, 96, 240],
      count: 4,
      row: (i) => rows[i],
    },
    {
      id: "notes",
      name: "长文本示例",
      columns: ["项目", "日期", "支出", "备注"],
      widths: [144, 104, 96, 240],
      count: 8,
      row: (i) => [
        `记录 ${i + 1}`,
        "09/22",
        (i + 1) * 35,
        i === 0
          ? "山间小屋的入住时间已与房东确认；公交站到住处需要步行二十分钟。此条用于演示长单元格的完整阅读与复制，不会自动换行拉高整行。"
          : "仅演示格式与阅读，不读取真实账目。",
      ],
    },
    {
      id: "wide",
      name: "多列示例",
      columns: Array.from({ length: 20 }, (_, i) =>
        i === 0 ? "项目" : `字段 ${i + 1}`,
      ),
      widths: Array(20).fill(112),
      count: 80,
      row: (i) =>
        Array.from({ length: 20 }, (_, j) =>
          j === 0 ? `明细 ${i + 1}` : `${i + 1}-${j + 1}`,
        ),
    },
    {
      id: "ledger",
      name: "千行样本",
      columns: ["项目", "日期", "支出", "备注"],
      widths: [144, 104, 96, 240],
      count: 1000,
      row: (i) => [
        `明细 ${String(i + 1).padStart(4, "0")}`,
        `09/${String((i % 28) + 1).padStart(2, "0")}`,
        (i + 1) * 3,
        `版本 ${version} · 可重置模拟记录`,
      ],
    },
    {
      id: "empty",
      name: "空表示例",
      columns: ["项目", "日期", "支出", "备注"],
      widths: [144, 104, 96, 240],
      count: 0,
      row: () => [],
    },
  ];
}
export function sheetPage(sample: SheetSample, view: SheetPageView) {
  const query = view.query.trim().toLocaleLowerCase();
  const matched: number[] = [];
  for (let i = 0; i < sample.count; i++)
    if (
      !query ||
      sample.row(i).some((v) => String(v).toLocaleLowerCase().includes(query))
    )
      matched.push(i);
  const pages = Math.max(1, Math.ceil(matched.length / sheetPageSize)),
    page = Math.max(1, Math.min(pages, view.page));
  return {
    page,
    pages,
    total: matched.length,
    rows: matched
      .slice((page - 1) * sheetPageSize, page * sheetPageSize)
      .map((index) => ({ index, cells: sample.row(index) })),
  };
}
export function sheetColumn(index: number) {
  let result = "",
    n = index + 1;
  while (n) {
    n--;
    result = String.fromCharCode(65 + (n % 26)) + result;
    n = Math.floor(n / 26);
  }
  return result;
}
export const sheetWidth = (width: number) =>
  Math.max(72, Math.min(480, Math.round(width)));
