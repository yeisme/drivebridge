import { test, expect } from "@playwright/test";

test("canonical search survives login and refresh while legacy search remains available", async ({ page }) => {
  await page.goto("/#/search");
  await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(page.getByRole("region", { name: "云端检索", exact: true })).toBeVisible();
  await expect(page.getByRole("tablist", { name: "文件流程视图" })).toHaveCount(0);
  await expect(page.locator(".sidebar").getByRole("button", { name: "搜索", exact: true })).toHaveAttribute("aria-current", "page");
  await page.reload();
  await expect(page.getByRole("region", { name: "云端检索", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "打开文件流程" }).click();
  await expect.poll(() => page.evaluate(() => location.hash)).toBe("#/flows?center=upload");
  await page.goto("/#/flows?center=search");
  await expect(page.getByRole("tab", { name: "云端检索" })).toHaveAttribute("aria-selected", "true");
  await page.setViewportSize({ width: 390, height: 844 });
  await page.getByRole("button", { name: "打开全部导航" }).click();
  await page.getByRole("dialog", { name: "全部导航" }).getByRole("button", { name: "搜索", exact: true }).click();
  await expect.poll(() => page.evaluate(() => location.hash)).toBe("#/search");
  await expect(page.getByRole("region", { name: "云端检索", exact: true })).toBeVisible();
  await expect(page.getByRole("tablist", { name: "文件流程视图" })).toHaveCount(0);
});
