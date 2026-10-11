import { test, expect } from "@playwright/test";
import path from "node:path";
import fs from "node:fs";
const artifacts =
  process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS ||
  path.resolve("temp/browser-artifacts");
test("Finder file-only workspace, folders, image preview, fixed sharing and revocation", async ({
  page,
  browser,
}) => {
  await page.goto("/");
  await page.getByRole("link", { name: "使用账号登录" }).click();
  await page.getByRole("link", { name: "以 Alice 登录" }).click();
  await expect(
    page.getByRole("heading", { name: "我的网盘", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: /^打开网盘 / }).first().click();
  await expect(page.getByRole("button", { name: "用于创作", exact: true })).toHaveCount(0);
  const folder = "素材-" + Date.now();
  await page.getByRole("button", { name: "新建", exact: true }).click();
  await page.getByRole("menuitem", { name: "新建文件夹", exact: true }).click();
  await page.getByRole("textbox", { name: "名称", exact: true }).fill(folder);
  await page.getByRole("button", { name: "确认", exact: true }).click();
  await expect(
    page.locator(".finder-folder").filter({ hasText: folder }),
  ).toBeVisible();
  await page.locator(".finder-folder").filter({ hasText: folder }).dblclick();
  const encoded = await page.evaluate(() => {
    const canvas = document.createElement("canvas");
    canvas.width = 480;
    canvas.height = 320;
    const ctx = canvas.getContext("2d")!;
    const gradient = ctx.createLinearGradient(0, 0, 480, 320);
    gradient.addColorStop(0, "#8ec5fc");
    gradient.addColorStop(1, "#c4a9ef");
    ctx.fillStyle = gradient;
    ctx.fillRect(0, 0, 480, 320);
    ctx.fillStyle = "#fff9";
    ctx.beginPath();
    ctx.arc(320, 140, 80, 0, 2 * Math.PI);
    ctx.fill();
    return canvas.toDataURL("image/png").split(",")[1];
  });
  const name = "参考画面.png";
  await page
    .locator("input[type=file]")
    .setInputFiles({
      name,
      mimeType: "image/png",
      buffer: Buffer.from(encoded, "base64"),
    });
  await expect(
    page.getByRole("button", {
      name: "详情 " + folder + "/" + name,
      exact: true,
    }),
  ).toBeVisible();
  await page.getByRole("button", { name: "网格视图" }).click();
  await page.locator(".finder-name").filter({ hasText: name }).click({button:"right"});
  await page.getByRole("menuitem",{name:"查看元数据",exact:true}).click();
  await expect(
    page.getByRole("complementary", { name: "文件检查器" }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "详情 " + folder + "/" + name, exact: true })
    .click();
  await expect(
    page.getByRole("region", { name: "文件预览" }).last().locator("img"),
  ).toBeVisible();
  await expect
    .poll(() =>
      page
        .getByRole("region", { name: "文件预览" })
        .last()
        .locator("img")
        .evaluate((img: HTMLImageElement) => img.naturalWidth),
    )
    .toBe(480);
  await page.keyboard.press("Escape");
  await expect(page.getByRole("button", { name: "用于创作", exact: true })).toHaveCount(0);
  await expect(page.getByRole("dialog", { name: "素材与创作" })).toHaveCount(0);
  fs.mkdirSync(artifacts, { recursive: true });
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({
    path: path.join(artifacts, "finder-media-desktop.png"),
    fullPage: true,
  });
  const share = page.locator(".finder-actions").getByRole("button", { name: /分享/ });
  if (await share.isVisible()) await share.click();
  else {
    await page.getByRole("button", {name:"更多操作",exact:true}).click();
    await page.getByRole("menuitem", {name:"分享",exact:true}).click();
  }
  await page.getByRole("button", { name: "创建分享", exact: true }).click();
  await expect(page.getByLabel("分享链接")).toBeVisible();
  const link = await page.getByLabel("分享链接").inputValue();
  await page.keyboard.press("Escape");
  const guest = await browser.newContext();
  const shared = await guest.newPage();
  await shared.goto(link);
  await expect(
    shared.getByRole("button", { name: new RegExp(name) }),
  ).toBeVisible();
  await shared.getByRole("button", { name: new RegExp(name) }).click();
  await expect(
    shared.getByRole("region", { name: "文件预览" }).locator("img"),
  ).toBeVisible();
  await shared.keyboard.press("Escape");
  await shared.screenshot({
    path: path.join(artifacts, "share-media.png"),
    fullPage: true,
  });
  await page.getByRole("button", { name: "分享交付", exact: true }).first().click();
  await page
    .getByRole("row")
    .filter({ hasText: name })
    .getByRole("button", { name: /分享 .* 的更多操作/ })
    .click();
  await page.getByRole("menuitem", { name: "撤销分享", exact: true }).click();
  await page.getByRole("button", { name: "确认撤销分享" }).click();
  await shared.reload();
  await expect(shared.getByRole("alert")).toContainText("失效");
  await guest.close();
  await page.getByRole("button", { name: "我的网盘", exact: true }).click();
  await page.getByRole("button", { name: /^打开网盘 / }).first().click();
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({
    path: path.join(artifacts, "finder-media-mobile.png"),
    fullPage: true,
  });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
});
