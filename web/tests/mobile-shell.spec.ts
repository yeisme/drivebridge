import { test, expect } from "@playwright/test";
import path from "node:path";

test("mobile production shell uses bottom navigation and a dismissible full menu", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  const nav = page.getByRole("navigation", { name: "手机主导航" });
  await expect(nav).toBeVisible();
  await expect(nav.getByRole("button")).toHaveCount(5);
  await expect(page.locator(".sidebar")).toHaveCount(0);
  for (const width of [320, 390, 767]) {
    await page.setViewportSize({ width, height: 844 });
    const box = await nav.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.y + box!.height).toBeLessThanOrEqual(845);
    expect(box!.y).toBeGreaterThan(700);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  }
  await nav.getByRole("button", { name: "我的", exact: true }).click();
  await expect(nav.getByRole("button", { name: "我的", exact: true })).toHaveAttribute("aria-current", "page");
  await page.getByRole("button", { name: "打开全部导航" }).click();
  await expect(page.getByRole("dialog", { name: "全部导航" })).toBeVisible();
  for (let i = 0; i < 20; i++) {
    await page.keyboard.press("Tab");
    await expect(page.getByRole("dialog", { name: "全部导航" })).toContainText("全部导航");
    await expect.poll(() => page.getByRole("dialog", { name: "全部导航" }).evaluate(el => el.contains(document.activeElement))).toBe(true);
  }
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog", { name: "全部导航" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "打开全部导航" })).toBeFocused();
  await page.getByRole("button", { name: "打开全部导航" }).click();
  await page.setViewportSize({ width: 1440, height: 1000 });
  await expect(page.getByRole("dialog", { name: "全部导航" })).toHaveCount(0);
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByRole("dialog", { name: "全部导航" })).toHaveCount(0);
  await nav.getByRole("button", { name: "文件", exact: true }).click();
  if (process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS) await page.screenshot({ path: path.join(process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS, "mobile-shell.png") });
  await page.setViewportSize({ width: 1440, height: 1000 });
  await expect(nav).toHaveCount(0);
  await expect(page.locator(".sidebar")).toBeVisible();
});
