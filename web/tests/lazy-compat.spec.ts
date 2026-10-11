import { test, expect } from "@playwright/test";

test("team and key compatibility JavaScript loads only when its page opens", async ({ page }) => {
  const scripts: string[] = [];
  page.on("request", request => {
    if (request.resourceType() === "script") scripts.push(new URL(request.url()).pathname);
  });
  const loaded = (name: string) => scripts.some(path => new RegExp(`/assets/${name}-[^/]+\\.js$`).test(path));
  await page.goto("/#/files");
  await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(page.locator(".drive-home")).toBeVisible();
  expect(loaded("Teams")).toBe(false);
  expect(loaded("KeyDirectory")).toBe(false);
  await page.locator(".sidebar").getByRole("button", { name: "访问凭据", exact: true }).click();
  await page.getByRole("tab", { name: "Key", exact: true }).click();
  // The previous user directory has the same row while Suspense transitions.
  await expect(page.getByRole("combobox", { name: "Key 状态", exact: true })).toBeVisible();
  await expect(page.getByRole("row").filter({ hasText: "e2e-admin" })).toBeVisible();
  expect(loaded("KeyDirectory"), JSON.stringify(scripts)).toBe(true);
  expect(loaded("Teams")).toBe(false);
  await page.locator(".sidebar").getByRole("button", { name: "团队", exact: true }).click();
  await expect.poll(() => loaded("Teams")).toBe(true);
  await expect(page.getByText("正在加载页面…", { exact: true })).toHaveCount(0);
});
