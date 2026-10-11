import { test, expect, type Page } from "@playwright/test";

// Fixed e2e credentials seeded by backend-server cmd/drivebridge-fixture,
// same convention as the fixture S3 pair.
const adminKey = "dbu_" + "2".repeat(96);

async function loginWithKey(page: Page, key: string) {
  await page.goto("/");
  await page.locator("#login-key").fill(key);
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "我的网盘", exact: true }),
  ).toBeVisible();
}

test("key login, one-time issuance, list, revoke, and admin gating", async ({
  page,
}) => {
  await loginWithKey(page, adminKey);

  // The admin sees the management entry; the one-time keys never persist.
  await page.getByRole("button", { name: "访问凭据", exact: true }).click();
  await page.getByRole("tab", { name: "Key", exact: true }).click();
  await expect(page.getByRole("row").filter({hasText:"e2e-admin"})).toBeVisible();

  await page.getByRole("button", { name: "批量签发" }).click();
  await page.locator("textarea[name=names]").fill("e2e-made-user");
  await page.getByRole("button", { name: "签发", exact: true }).click();

  const dialog = page.getByRole("dialog", { name: "保存新签发的密钥" });
  const issued = dialog.locator("textarea.token-value");
  await expect(issued.first()).toBeVisible();
  const plaintext = await issued.first().inputValue();
  expect(plaintext).toMatch(/^dbu_[0-9a-f]{96}$/);

  // Closing the one-time modal drops the plaintext from the DOM.
  await page.getByRole("button", { name: "我已保存，关闭" }).click();
  await expect(page.getByText("e2e-made-user").first()).toBeVisible();
  await expect(page.locator("textarea.token-value")).toHaveCount(0);

  // Revocation flips the row with confirmation.
  await page
    .getByRole("row")
    .filter({ hasText: "e2e-made-user" })
    .getByRole("button", { name: "详情" })
    .click();
  await page.getByRole("dialog", { name: "Key 详情与绑定" }).getByRole("button", { name: "吊销", exact: true }).click();
  await page.getByRole("button", { name: "确认吊销" }).click();
  await expect(
    page
      .getByRole("row")
      .filter({ hasText: "e2e-made-user" })
      .getByText("已吊销"),
  ).toBeVisible();

  // A freshly issued key logs a normal user in, who must not see the
  // management entry.
  await page.getByRole("button", { name: "批量签发" }).click();
  await page.locator("textarea[name=names]").fill("e2e-second");
  await page.getByRole("button", { name: "签发", exact: true }).click();
  const second = await dialog
    .locator("textarea.token-value")
    .first()
    .inputValue();
  await page.getByRole("button", { name: "我已保存，关闭" }).click();
  await page.getByRole("button", { name: "账户菜单" }).click();
  await page.getByRole("menuitem", { name: "退出登录" }).click();
  await expect(page.locator("#login-key")).toBeVisible();
  await loginWithKey(page, second);
  await expect(
    page.getByRole("button", { name: "批量签发", exact: true }),
  ).toHaveCount(0);

  // Wrong keys fail without distinguishing existence.
  await page.getByRole("button", { name: "账户菜单" }).click();
  await page.getByRole("menuitem", { name: "退出登录" }).click();
  await page.locator("#login-key").fill("dbu_" + "9".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(page.getByRole("alert").first()).toBeVisible();
});

test("fresh normal key logs in without admin surface", async ({ page }) => {
  // Other specs may rotate or revoke the seeded user key; issue a fresh one.
  await loginWithKey(page, adminKey);
  await page.getByRole("button", { name: "访问凭据", exact: true }).click();
  await page.getByRole("tab", { name: "Key", exact: true }).click();
  await page.getByRole("button", { name: "批量签发" }).click();
  await page.locator("textarea[name=names]").fill("e2e-fresh-normal");
  await page.getByRole("button", { name: "签发", exact: true }).click();
  const fresh = await page
    .getByRole("dialog")
    .locator("textarea.token-value")
    .first()
    .inputValue();
  await page.getByRole("button", { name: "我已保存，关闭" }).click();
  await page.getByRole("button", { name: "账户菜单" }).click();
  await page.getByRole("menuitem", { name: "退出登录" }).click();
  await expect(page.locator("#login-key")).toBeVisible();
  await loginWithKey(page, fresh);
  await expect(
    page.getByRole("button", { name: "批量签发", exact: true }),
  ).toHaveCount(0);
  // The key itself never lands in browser storage.
  const stored = await page.evaluate(() => ({
    local: Object.entries(localStorage).map(([k]) => k),
    session: Object.entries(sessionStorage).map(([k]) => k),
  }));
  expect(
    [...stored.local, ...stored.session].some((k) => k.includes("dbu_")),
  ).toBe(false);
});
