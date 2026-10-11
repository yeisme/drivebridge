import { test, expect } from "@playwright/test";

test("page render failure preserves shell navigation and recovers after data reload", async ({ page }) => {
  let malformed = true;
  await page.route("**/api/v1/agents", route => route.fulfill({ json: {
    data: malformed ? [{ id: "fixture-malformed", name: "fixture", space_id: "fixture-space", prefix: "", scopes: null, expires_at: "2099-01-01T00:00:00Z", revoked: false }] : [],
  } }));
  await page.goto("/#/agents");
  await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await page.getByRole("tab", { name: "旧版机器人授权", exact: true }).click();
  await expect(page.getByRole("heading", { name: "此页面暂时无法显示" })).toBeVisible();
  await expect(page.locator(".sidebar")).toBeVisible();
  await expect(page.getByRole("button", { name: "账户菜单" })).toBeVisible();
  await expect(page.getByRole("alert")).not.toContainText("scopes");
  await expect(page.getByRole("alert")).not.toContainText("TypeError");
  await page.getByRole("button", { name: "重试显示页面" }).click();
  // A render retry remounts the view boundary at its safe primary tab.
  await expect(page.getByRole("region", { name: "新协议 Agent 授权" })).toBeVisible();
  await page.getByRole("tab", { name: "旧版机器人授权", exact: true }).click();
  await expect(page.getByRole("heading", { name: "此页面暂时无法显示" })).toBeVisible();
  await page.locator(".sidebar").getByRole("button", { name: "我的网盘", exact: true }).click();
  await expect(page.locator(".drive-home")).toBeVisible();
  await expect(page.getByRole("heading", { name: "此页面暂时无法显示" })).toHaveCount(0);
  malformed = false;
  const reloaded = page.waitForResponse(response => response.url().endsWith("/api/v1/agents") && response.ok());
  await page.evaluate(() => window.dispatchEvent(new Event("drivebridge-governance-changed")));
  await reloaded;
  await page.locator(".sidebar").getByRole("button", { name: "Agent 管理", exact: true }).click();
  await page.getByRole("tab", { name: "旧版机器人授权", exact: true }).click();
  await expect(page.getByRole("heading", { name: "还没有获准的机器人" })).toBeVisible();
});
