import { test, expect } from "@playwright/test";
test("new user wizard preserves name, confirms authority and shows activation once", async ({
  page,
  context,
}) => {
  test.skip(
    process.env.DRIVEBRIDGE_TEST_TEAM_DEMO !== "1",
    "Requires team fixture",
  );
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  await page.goto("/");
  await page.getByRole("button", { name: "无需账号，进入本地体验" }).click();
  await page.getByRole("button", { name: "访问凭据", exact: true }).click();
  await page.getByRole("tab", { name: "用户", exact: true }).click();
  await page.getByRole("button", { name: "新建用户", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "新建用户", exact: true });
  await dialog.getByLabel("用户名称").fill("向导验收成员");
  await dialog.getByRole("button", { name: "下一步" }).click();
  await expect(dialog).toContainText("待激活普通账号");
  await dialog.getByRole("button", { name: "上一步" }).click();
  await expect(dialog.getByLabel("用户名称")).toHaveValue("向导验收成员");
  await dialog.getByRole("button", { name: "下一步" }).click();
  await dialog.getByRole("button", { name: "下一步" }).click();
  await dialog.getByRole("button", { name: "确认创建" }).click();
  await expect(dialog).toContainText("管理员不会取得用户登录 Key");
  await dialog.getByRole("button", { name: "复制激活信息" }).click();
  await expect(
    dialog.getByRole("button", { name: "已复制激活信息" }),
  ).toBeVisible();
  await dialog.getByRole("button", { name: "完成并返回用户目录" }).click();
  await expect(page.getByLabel("激活码", { exact: true })).toHaveCount(0);
});
