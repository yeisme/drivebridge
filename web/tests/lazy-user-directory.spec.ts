import { test, expect } from "@playwright/test";

test("user directory is fetched only on the authorized management page", async ({ page }) => {
  test.skip(process.env.DRIVEBRIDGE_TEST_TEAM_DEMO !== "1", "Requires disposable team fixture");
  const requests: string[] = [];
  page.on("request", request => {
    if (request.resourceType() === "script") requests.push(new URL(request.url()).pathname);
  });
  const loaded = () => requests.some(path => /\/assets\/UserDirectory-[^/]+\.js$/.test(path));
  await page.goto("/");
  await page.getByRole("button", { name: "无需账号，进入本地体验" }).click();
  await expect(page.locator(".drive-home")).toBeVisible();
  expect(loaded()).toBe(false);
  await page.locator(".sidebar").getByRole("button", { name: "访问凭据", exact: true }).click();
  await page.getByRole("tab", { name: "用户", exact: true }).click();
  await expect(page.getByRole("region", { name: "用户目录", exact: true })).toBeVisible();
  expect(loaded()).toBe(true);
});
