import { test, expect } from "@playwright/test";

test("other workspace pages expose next actions and filters", async ({ page }) => {
  for (const endpoint of ["operations", "inbox", "shares", "agents"]) {
    await page.route(`**/api/v1/${endpoint}`, route => route.fulfill({json:{data:[]}}));
  }
  await page.goto("/");
  await page.getByRole("link", { name: "使用账号登录" }).click();
  await page.getByRole("link", { name: "以 Alice 登录" }).click();
  await expect(page.getByRole("heading", { name: "我的网盘", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "打开交付中心", exact: true }).click();
  await expect(page.getByRole("button", { name: "去网盘上传", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "待处理产物", exact: true }).click();
  await expect(page.getByRole("button", { name: "查看传输记录", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Agent 管理", exact: true }).click();
  await page.getByRole("tab", { name: "旧版机器人授权", exact: true }).click();
  await expect(page.getByRole("button", { name: "授权机器人", exact: true })).toBeVisible();
  await expect(page.getByLabel("机器人状态")).toBeVisible();
  await page.getByRole("navigation").first().getByRole("button", { name: "分享交付", exact: true }).click();
  await expect(page.getByRole("link", { name: "去网盘选择文件" })).toBeVisible();
  await page.evaluate(() => {
    location.hash = "#/connections";
  });
  await expect(page.getByRole("heading", { name: "存储管理", level: 2, exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "连接存储", exact: true })).toBeVisible();
});
