import { test, expect } from "@playwright/test";

test("touch navigation switches pages and closes the full menu after selection", async ({ browser, baseURL }) => {
  const context = await browser.newContext({
    baseURL, viewport: { width: 390, height: 844 },
    isMobile: true, hasTouch: true, deviceScaleFactor: 2,
  });
  try {
    const page = await context.newPage();
    await page.goto("/");
    await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
    await page.getByRole("button", { name: "登录", exact: true }).tap();
    const nav = page.getByRole("navigation", { name: "手机主导航" });
    await expect(nav).toBeVisible();
    for (const [label, route] of [["我的", "settings"], ["Agent", "agents"], ["任务", "tasks"], ["文件", "files"], ["首页", "home"]]) {
      const button = nav.getByRole("button", { name: label, exact: true });
      await button.tap();
      await expect(button).toHaveAttribute("aria-current", "page");
      await expect.poll(() => page.evaluate(() => location.hash)).toBe(`#/${route}`);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    }
    const trigger = page.getByRole("button", { name: "打开全部导航" });
    await trigger.tap();
    const dialog = page.getByRole("dialog", { name: "全部导航" });
    await expect(dialog).toBeVisible();
    for (const group of ["工作台", "Agent", "管理"]) await expect(dialog.getByRole("heading", { name: group, exact: true })).toBeVisible();
    await expect(dialog.getByRole("region", { name: "Agent", exact: true }).getByRole("button", { name: "Agent 管理", exact: true })).toBeVisible();
    await dialog.getByRole("button", { name: "个人设置", exact: true }).tap();
    await expect(dialog).toHaveCount(0);
    await expect(nav.getByRole("button", { name: "我的", exact: true })).toHaveAttribute("aria-current", "page");
    await trigger.tap();
    await dialog.getByRole("button", { name: "关闭", exact: true }).tap();
    await expect(dialog).toHaveCount(0);
    expect(await page.evaluate(() => navigator.maxTouchPoints)).toBeGreaterThan(0);
  } finally {
    await context.close();
  }
});
