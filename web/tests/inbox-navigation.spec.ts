import { test, expect } from "@playwright/test";

test("inbox deep link survives login and reload and is reachable in mobile navigation", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto("/#/inbox");
  await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(page.getByRole("region", { name: "Agent 提交收件", exact: true })).toBeVisible();
  await expect(page.locator(".sidebar").getByRole("button", { name: "产物与收件", exact: true })).toHaveAttribute("aria-current", "page");
  await page.reload();
  await expect(page.getByRole("region", { name: "Agent 提交收件", exact: true })).toBeVisible();
  await page.goto("/#/inbox?view=legacy");
  await expect(page.getByRole("list", { name: "产物收件箱" })).toBeVisible();
  await page.reload();
  await expect(page.getByRole("list", { name: "产物收件箱" })).toBeVisible();
  await page.setViewportSize({ width: 390, height: 844 });
  const tabs = page.getByRole("navigation", { name: "手机主导航" });
  await expect(tabs.getByRole("button", { name: "任务", exact: true })).toHaveAttribute("aria-current", "page");
  await tabs.getByRole("button", { name: "文件", exact: true }).click();
  await page.getByRole("button", { name: "打开全部导航" }).click();
  const dialog = page.getByRole("dialog", { name: "全部导航" });
  await dialog.getByRole("region", { name: "Agent", exact: true }).getByRole("button", { name: "产物与收件", exact: true }).click();
  await expect(dialog).toHaveCount(0);
  await expect(page.getByRole("region", { name: "Agent 提交收件", exact: true })).toBeVisible();
  await page.getByRole("tab", { name: "历史传输收件", exact: true }).click();
  await expect(page.getByRole("list", { name: "产物收件箱" })).toBeVisible();
  await expect.poll(() => page.evaluate(() => location.hash)).toBe("#/inbox");
});
