import { test, expect } from "@playwright/test";

test("shell and navigation agree at the 767/768px boundary", async ({ page }) => {
  await page.setViewportSize({ width: 767, height: 900 });
  await page.goto("/#/agents");
  await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  const topbar = page.locator(".main > .topbar");
  const mobile = page.getByRole("navigation", { name: "手机主导航" });
  await expect(mobile).toBeVisible();
  await expect(page.locator(".sidebar")).toHaveCount(0);
  await expect.poll(() => topbar.evaluate(el => getComputedStyle(el).paddingLeft)).toBe("10px");
  await expect(page.locator(".topbar .page-actions")).toHaveCount(0);
  await expect.poll(async () => (await topbar.boundingBox())!.height).toBe(48);
  await expect(page.locator(".content").getByRole("button", { name: "接入 Agent", exact: true })).toBeVisible();
  await expect(page.locator(".main > .content")).toHaveCSS("padding-left", "12px");
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.setViewportSize({ width: 768, height: 900 });
  await expect(mobile).toHaveCount(0);
  await expect(page.locator(".sidebar")).toBeVisible();
  await expect.poll(() => topbar.evaluate(el => getComputedStyle(el).paddingLeft)).toBe("16px");
  await expect.poll(async () => Math.round((await page.locator(".sidebar").boundingBox())!.width)).toBe(56);
  await expect(page.locator(".main > .content")).toHaveCSS("padding-left", "16px");
  await page.setViewportSize({ width: 1440, height: 900 });
  await expect(page.locator(".main > .content")).toHaveCSS("padding-left", "16px");
  await page.setViewportSize({ width: 767, height: 900 });
  await expect(mobile).toBeVisible();
  await expect.poll(() => topbar.evaluate(el => getComputedStyle(el).paddingLeft)).toBe("10px");
});
