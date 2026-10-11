import { test, expect } from "@playwright/test";

test("local demo enters without an account and supports authenticated writes", async ({
  page,
}) => {
  test.skip(
    process.env.DRIVEBRIDGE_TEST_DEMO !== "1",
    "Requires an explicitly enabled disposable demo",
  );
  await page.goto("/");
  await expect(
    page.getByRole("button", { name: "无需账号，进入本地体验" }),
  ).toBeVisible();
  await expect(page.getByRole("link", { name: /使用账号登录/ })).toHaveCount(0);
  await page.getByRole("button", { name: "无需账号，进入本地体验" }).click();
  await expect(
    page.getByRole("heading", { name: "我的网盘", exact: true }),
  ).toBeVisible();
  const result = await page.evaluate(async () => {
    const me = (await (await fetch("/api/v1/me")).json()).data;
    const spaces = (await (await fetch("/api/v1/spaces")).json()).data;
    const response = await fetch("/api/v1/directories", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-CSRF-Token": me.csrf_token,
      },
      body: JSON.stringify({ space_id: spaces[0].id, path: "免账号体验" }),
    });
    return response.status;
  });
  expect(result).toBeGreaterThanOrEqual(200);
  expect(result).toBeLessThan(300);
  await page.reload();
  await page
    .getByRole("button", { name: /^打开网盘 / })
    .first()
    .click();
  await expect(page.getByText("免账号体验").first()).toBeVisible();
  await page.getByRole("button", { name: "账户菜单" }).click();
  await page.getByRole("menuitem", { name: "退出登录" }).click();
  await expect(
    page.getByRole("button", { name: "无需账号，进入本地体验" }),
  ).toBeVisible();
});
