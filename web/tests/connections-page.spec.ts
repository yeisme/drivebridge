import { test, expect } from "@playwright/test";

test("connection cards retain disabled and confirmation boundaries", async ({ page }) => {
  const base = { id: "fixture-s3", name: "本地替身存储", kind: "s3", status: "verified", disabled: false };
  const connections = [base, { ...base, id: "platform", name: "平台替身" }, { ...base, id: "fixture-disabled", name: "断开替身", disabled: true }];
  let writes = 0;
  await page.route("**/api/v1/connections**", route => {
    if (route.request().method() !== "GET") {
      writes++;
      return route.fulfill({ status: 403, json: { error: { code: "fixture_write_forbidden" } } });
    }
    return route.fulfill({ json: { data: connections } });
  });
  await page.goto("/#/connections");
  await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  const card = page.getByRole("article").filter({ has: page.getByRole("heading", { name: base.name, exact: true }) });
  await expect(card.getByText("验证记录：通过", { exact: true })).toBeVisible();
  await card.getByRole("button", { name: "断开连接", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByRole("button", { name: "确认断开", exact: true })).toBeVisible();
  expect(writes).toBe(0);
  await dialog.getByRole("button", { name: "取消", exact: true }).click();
  await expect(dialog).toHaveCount(0);
  const disabled = page.getByRole("article").filter({ hasText: "断开替身" });
  await expect(disabled.getByRole("button", { name: "验证连接" })).toBeDisabled();
  await expect(disabled.getByRole("button", { name: "用于空间" })).toBeDisabled();
  await expect(disabled.getByRole("button", { name: "断开连接" })).toHaveCount(0);
  const platform = page.getByRole("article").filter({ hasText: "平台替身" });
  await expect(platform.getByRole("button", { name: "断开连接" })).toHaveCount(0);
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(card.getByRole("button", { name: "验证连接" })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  expect(writes).toBe(0);
});
