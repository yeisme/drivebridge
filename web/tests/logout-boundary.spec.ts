import { test, expect } from "@playwright/test";

test("failed logout retains the workspace and confirmed logout returns to login", async ({ page }) => {
  await page.goto("/");
  await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(page.locator(".app")).toBeVisible();
  let navigations = 0;
  page.on("framenavigated", frame => { if (frame === page.mainFrame()) navigations++; });
  await page.route("**/auth/logout", route => route.fulfill({ status: 403, json: { error: { code: "forbidden" } } }));
  await page.getByRole("button", { name: "账户菜单" }).click();
  await page.getByRole("menuitem", { name: "退出登录" }).click();
  await expect(page.locator(".app .error").first()).toBeVisible();
  await expect(page.locator(".app")).toBeVisible();
  expect(navigations).toBe(0);
  expect((await page.request.get("/api/v1/me")).status()).toBe(200);
  await page.unroute("**/auth/logout");
  await page.getByRole("button", { name: "账户菜单" }).click();
  await page.getByRole("menuitem", { name: "退出登录" }).click();
  await expect(page.locator("#login-key")).toBeVisible();
  await expect(page.locator(".app")).toHaveCount(0);
  expect((await page.request.get("/api/v1/me")).status()).toBe(401);
});
