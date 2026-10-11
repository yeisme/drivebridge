import { test, expect, type Page } from "@playwright/test";
import * as XLSX from "xlsx";
import fs from "node:fs";
import path from "node:path";

async function openWorkspace(page: Page) {
  await page.goto("/");
  await page.getByRole("link", { name: "使用账号登录" }).click();
  await page.getByRole("link", { name: "以 Alice 登录" }).click();
  await expect(
    page.getByRole("heading", { name: "我的网盘", exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: /^打开网盘 / })
    .first()
    .click();
}
async function uploadAndOpen(
  page: Page,
  name: string,
  mimeType: string,
  buffer: Buffer,
) {
  const chooser = page.waitForEvent('filechooser');
  await page.getByRole('button', { name: '上传文件', exact: true }).click();
  await (await chooser).setFiles({ name, mimeType, buffer });
  await page.getByRole("button", { name: `详情 ${name}`, exact: true }).click();
  await expect(page.locator(".spreadsheet-reader")).toBeVisible();
}
async function screenshot(page: Page, name: string) {
  const artifacts =
    process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS ||
    path.resolve("temp/browser-artifacts");
  fs.mkdirSync(artifacts, { recursive: true });
  await page
    .locator(".quicklook-dialog")
    .screenshot({ path: path.join(artifacts, name) });
}

test("CSV 表格保留引号换行与编号，分页、分隔符、移动端滚动可用", async ({
  page,
}) => {
  await openWorkspace(page);
  // Keep a following file available: arrow keys inside the table must scroll,
  // not trigger the surrounding Quick Look file-navigation shortcut.
  const siblingName = `表格-zzzz-${Date.now()}.txt`;
  const chooser = page.waitForEvent('filechooser');
  await page.getByRole('button', { name: '上传文件', exact: true }).click();
  await (await chooser).setFiles({ name: siblingName, mimeType: 'text/plain', buffer: Buffer.from('next file') });
  await expect(page.getByRole('button', { name: `详情 ${siblingName}`, exact: true })).toBeVisible();
  const rows = [
    "\uFEFF编号;说明;金额",
    '001;"含,逗号与""引号""\n第二行";1.20',
    ...Array.from(
      { length: 120 },
      (_, i) => `${String(i + 2).padStart(3, "0")};记录 ${i + 2};2.00`,
    ),
  ];
  await uploadAndOpen(
    page,
    `表格-${Date.now()}.csv`,
    "text/csv",
    Buffer.from(rows.join("\r\n")),
  );
  const reader = page.locator(".spreadsheet-reader");
  await expect(
    reader.getByRole("cell", { name: "001", exact: true }),
  ).toBeVisible();
  await expect(
    reader.getByRole("cell", { name: '含,逗号与"引号" 第二行' }),
  ).toBeVisible();
  await expect(reader.getByText(/当前分隔符：分号/)).toBeVisible();
  await expect(
    reader.getByRole("rowheader", { name: "100", exact: true }),
  ).toHaveCount(1);
  await reader.getByRole("button", { name: "下一页表格" }).click();
  await expect(
    reader.getByRole("rowheader", { name: "101", exact: true }),
  ).toHaveCount(1);
  await expect(
    reader.getByRole("button", { name: "下一页表格" }),
  ).toBeDisabled();
  await reader.getByRole("button", { name: "上一页表格" }).click();
  await reader.getByRole("combobox", { name: "分隔符", exact: true }).click();
  await page.getByRole("option", { name: "分号", exact: true }).click();
  await expect(
    reader.getByRole("cell", { name: "001", exact: true }),
  ).toBeVisible();
  await screenshot(page, "spreadsheet-csv-desktop.png");
  await page.setViewportSize({ width: 390, height: 844 });
  const region = reader.getByRole("region");
  await expect(region).toBeVisible();
  await expect(page.getByRole('complementary', { name: '预览文件列表与信息' })).toBeHidden();
  await expect(
    page.getByRole("button", { name: "下一个", exact: true }),
  ).toBeEnabled();
  await expect
    .poll(() =>
      region.evaluate((element) => element.scrollWidth > element.clientWidth),
    )
    .toBe(true);
  await region.focus();
  await page.keyboard.press("ArrowRight");
  await expect
    .poll(() => region.evaluate((element) => element.scrollLeft))
    .toBeGreaterThan(0);
  const box = await reader.boundingBox();
  expect(box!.width).toBeLessThanOrEqual(390);
  await screenshot(page, "spreadsheet-csv-mobile.png");
  const geometry = await region.evaluate((element) => ({
    viewportWidth: window.innerWidth,
    readerWidth: element.parentElement!.getBoundingClientRect().width,
    clientWidth: element.clientWidth,
    scrollWidth: element.scrollWidth,
    scrollLeft: element.scrollLeft,
  }));
  const artifacts =
    process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS ||
    path.resolve("temp/browser-artifacts");
  fs.writeFileSync(
    path.join(artifacts, "spreadsheet-mobile-geometry.json"),
    JSON.stringify(geometry, null, 2),
  );
});

test("XLSX 上传原文件可读格式化值并切换工作表，不调用转换服务", async ({
  page,
}) => {
  await openWorkspace(page);
  const conversions: { fileID: string; kind: string }[] = [];
  page.on("request", (request) => {
    const match = /\/files\/([^/]+)\/previews(?:\?|$)/.exec(request.url());
    if (request.method() === "POST" && match)
      conversions.push({
        fileID: match[1],
        kind: request.postDataJSON()?.kind,
      });
  });
  const book = XLSX.utils.book_new();
  const first = XLSX.utils.aoa_to_sheet([
    ["编号", "金额", "日期"],
    ["001", 1234.5, 45292],
  ]);
  first.B2.z = "#,##0.00";
  first.C2.z = "yyyy-mm-dd";
  XLSX.utils.book_append_sheet(book, first, "收支");
  XLSX.utils.book_append_sheet(
    book,
    XLSX.utils.aoa_to_sheet([["第二张表", "<script>alert(1)</script>"]]),
    "说明",
  );
  XLSX.utils.book_append_sheet(book, XLSX.utils.aoa_to_sheet([]), "空白");
  const mediaRequest = page.waitForRequest(
    (request) =>
      request.method() === "GET" &&
      /\/files\/[^/]+\/media(?:\?|$)/.test(request.url()),
  );
  await uploadAndOpen(
    page,
    `工作簿-${Date.now()}.xlsx`,
    "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    XLSX.write(book, { type: "buffer", bookType: "xlsx" }),
  );
  const reader = page.locator(".spreadsheet-reader");
  await expect(
    reader.getByRole("cell", { name: "1,234.50", exact: true }),
  ).toBeVisible();
  await expect(
    reader.getByRole("cell", { name: "2024-01-01", exact: true }),
  ).toBeVisible();
  await screenshot(page, "spreadsheet-xlsx-desktop.png");
  await reader.getByRole("combobox", { name: "工作表", exact: true }).click();
  await page.getByRole("option", { name: "收支", exact: true }).focus();
  await page.keyboard.press("ArrowDown");
  await expect(
    page.getByRole("option", { name: "说明", exact: true }),
  ).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(
    reader.getByRole("cell", { name: "第二张表", exact: true }),
  ).toBeVisible();
  await expect(
    reader.getByRole("cell", {
      name: "<script>alert(1)</script>",
      exact: true,
    }),
  ).toBeVisible();
  await reader.getByRole("combobox", { name: "工作表", exact: true }).click();
  await page.getByRole("option", { name: "空白", exact: true }).click();
  await expect(reader.getByText("此工作表为空。")).toBeVisible();
  const spreadsheetFileID = new URL((await mediaRequest).url()).pathname
    .split("/")
    .at(-2);
  expect(
    conversions.filter(
      (request) =>
        request.fileID === spreadsheetFileID && request.kind === "compatible",
    ),
  ).toEqual([]);
});
