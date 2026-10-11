import { test, expect } from "@playwright/test";

test("command search opens from header and shortcut and restores focus", async ({ page }) => {
  await page.goto("/#/files");
  await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  const trigger = page.getByRole("button", { name: "搜索工作区", exact: true });
  await trigger.click();
  const dialog = page.getByRole("dialog", { name: "搜索工作区" });
  await expect(dialog).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);
  await expect(trigger).toBeFocused();
  await page.keyboard.press("Control+k");
  await expect(dialog).toBeVisible();
  await dialog.getByRole("button", { name: "关闭", exact: true }).click();
  await expect(trigger).toBeFocused();
  await page.evaluate(() => {
    const input = document.createElement("input"); input.id = "fixture-editor";
    document.body.append(input); input.focus();
  });
  await page.keyboard.press("Control+k");
  await expect(dialog).toHaveCount(0);
  await page.evaluate(() => document.getElementById("fixture-editor")?.remove());
  await page.setViewportSize({ width: 390, height: 844 });
  await trigger.click();
  await expect(dialog).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});
