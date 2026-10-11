import { test, expect } from "@playwright/test";

test("search capability failure remains retryable rather than unsupported", async ({ page }) => {
  let unavailable = true;
  await page.route("**/api/v1/operations/lookup", route => route.fulfill(unavailable
    ? { status: 503, contentType: "text/plain", body: "fixture unavailable" }
    : { json: { data: { found: false } } }));
  await page.goto("/#/files");
  await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await page.getByRole("button", { name: "搜索工作区", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "搜索工作区" });
  await expect(dialog.getByRole("alert")).toContainText("无法读取内容发现能力，请重试");
  await expect(dialog.getByRole("searchbox", { name: "检索条件" })).toBeVisible();
  await expect(dialog.getByRole("button", { name: "检索", exact: true })).toBeEnabled();
  unavailable = false;
  await dialog.getByRole("button", { name: "重试", exact: true }).click();
  await expect(dialog.getByRole("searchbox", { name: "检索条件" })).toBeVisible();
  await expect(dialog.getByText(/尚未读取搜索结果/)).toBeVisible();
  await expect(dialog.getByRole("alert")).toHaveCount(0);
});
