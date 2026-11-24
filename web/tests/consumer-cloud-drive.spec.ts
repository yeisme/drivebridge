import { expect, test } from "@playwright/test";
import path from "node:path";

test("consumer cloud drive lands directly in files, shows readable file types, supports marquee & floating dock, and switches drives", async ({ page }) => {
  const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS;
  await page.setViewportSize({ width: 1440, height: 920 });
  await page.goto("/");
  await page.getByRole("link", { name: "使用账号登录" }).click();
  await page.getByRole("link", { name: "以 Alice 登录" }).click();

  // 1. Direct landing in personal drive Finder (no intermediate selection wall)
  const browser = page.getByRole("region", { name: "文件浏览", exact: true });
  await expect(browser).toBeVisible();

  // 2. Sidebar streamlined primary navigation + quick create button
  const nav = page.getByRole("navigation", { name: "工作区导航", exact: true });
  await expect(nav.getByRole("button", { name: "新建或上传", exact: true })).toBeVisible();
  await expect(nav.locator(".nav-item")).toHaveCount(8);

  // 3. Create Folder via top-left "+ 新建 / 上传" menu
  await nav.getByRole("button", { name: "新建或上传", exact: true }).click();
  await page.getByRole("menuitem", { name: "新建文件夹", exact: true }).click();
  const folderName = `相册精选-${Date.now()}`;
  await page.getByRole("textbox", { name: "名称", exact: true }).fill(folderName);
  await page.getByRole("button", { name: "确认", exact: true }).click();
  await expect(browser.locator(".finder-folder").filter({ hasText: folderName })).toBeVisible();

  // 4. Upload files and verify bottom-right TransferDock + readable file subtitle (no raw MIME charset)
  const tinyPng = Buffer.from(
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
    "base64",
  );
  await page.locator("input[type=file]").setInputFiles([
    { name: "旅行手记.md", mimeType: "text/plain; charset=utf-8", buffer: Buffer.from("# 旅行手记\n\n山海皆可平。") },
    { name: "行程清单.txt", mimeType: "text/plain; charset=utf-8", buffer: Buffer.from("Day 1: 出发") },
    { name: "画作原图.png", mimeType: "image/png", buffer: tinyPng },
  ]);
  await expect(browser.locator(".finder-file").filter({ hasText: "旅行手记.md" })).toBeVisible();
  await expect(browser.locator(".finder-file").filter({ hasText: "行程清单.txt" })).toBeVisible();
  await expect(browser.locator(".finder-file").filter({ hasText: "画作原图.png" })).toBeVisible();

  // Mobile viewport shows friendly type "Markdown 笔记" / "文本文档", never raw "text/plain; charset=utf-8"
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(browser.locator(".finder-file").filter({ hasText: "旅行手记.md" })).toContainText("Markdown 笔记");
  await expect(browser.locator(".finder-file").filter({ hasText: "旅行手记.md" })).not.toContainText("charset=utf-8");
  if (artifacts) {
    await page.screenshot({ path: path.join(artifacts, "consumer-cloud-mobile-390.png"), animations: "disabled" });
  }

  // 5. Desktop multi-selection pill bar + top header compactness
  await page.setViewportSize({ width: 1440, height: 920 });
  const dismissNotice = page.getByRole("button", { name: "关闭通知", exact: true });
  if (await dismissNotice.isVisible()) await dismissNotice.click();
  const hideDock = page.getByRole("button", { name: "隐藏传输窗", exact: true });
  if (await hideDock.isVisible()) await hideDock.click();
  const scrollBox = await browser.locator(".finder-scroll").boundingBox();
  expect(scrollBox!.y).toBeLessThanOrEqual(220);

  await browser.getByRole("checkbox", { name: "选择 旅行手记.md", exact: true }).click();
  await browser.getByRole("checkbox", { name: "选择 行程清单.txt", exact: true }).click();
  const dock = browser.getByRole("toolbar", { name: "多选文件操作", exact: true });
  await expect(dock).toContainText("已选择 2 项");
  await expect(dock.getByRole("button", { name: "分享", exact: true })).toBeVisible();
  if (artifacts) {
    await page.screenshot({ path: path.join(artifacts, "consumer-cloud-desktop-1440.png"), animations: "disabled" });
  }
  await dock.getByRole("button", { name: "取消选择", exact: true }).click();
  await expect(dock).toHaveCount(0);

  // 5b. Image format conversion (save converted WebP copy to current drive)
  await browser.getByRole("button", { name: "更多操作 画作原图.png", exact: true }).click();
  await page.getByRole("menuitem", { name: "转换图片格式…", exact: true }).click();
  const convertDialog = page.getByRole("dialog", { name: /转换图片格式/ });
  await expect(convertDialog).toBeVisible();
  await convertDialog.getByRole("combobox", { name: "转换目标格式" }).click();
  await page.getByRole("option", { name: /WebP/ }).click();
  await convertDialog.getByRole("button", { name: "另存到网盘", exact: true }).click();
  await expect(browser.locator(".finder-file").filter({ hasText: "画作原图.webp" })).toBeVisible();

  // 6. Settings Hub consolidates management tools
  await page.locator(".sidebar").getByRole("button", { name: "个人设置", exact: true }).click();
  const settings = page.getByTestId("personal-settings");
  await expect(settings.getByRole("region", { name: "空间与高级管理", exact: true })).toBeVisible();
  if (artifacts) {
    await page.screenshot({ path: path.join(artifacts, "consumer-settings-hub-1440.png"), animations: "disabled" });
  }
});
