import { test, expect } from "@playwright/test";
import { freshOwnerKey } from "./helpers/key-owner";

test("storage select and theme remain usable with the official primitives", async ({ page, request }) => {
  // Own identity: other specs rotate or revoke the seeded user key.
  const key = await freshOwnerKey(request, "e2e-shadcn-controls");
  await page.goto("/");
  await page.locator("#login-key").fill(key);
  await page.getByRole("button", { name: "登录", exact: true }).click();
  const storage = page.getByRole("combobox", { name: "筛选存储" });
  await storage.click();
  await page.getByRole("option", { name: "平台存储", exact: true }).click();
  await expect(storage).toContainText("平台存储");
  await storage.click();
  await page.keyboard.press("Escape");
  await expect(storage).toBeFocused();
  await page.getByRole("button", { name: "切换深色主题" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
});
