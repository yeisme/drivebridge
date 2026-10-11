import { test, expect } from "@playwright/test";
import { selectChoice } from "./helpers/select";
test("administrator creates their own team drive and adjusts its quota", async ({
  page,
}) => {
  test.skip(
    process.env.DRIVEBRIDGE_TEST_TEAM_DEMO !== "1",
    "Requires team fixture",
  );
  await page.goto("/");
  await page.getByRole("button", { name: "无需账号，进入本地体验" }).click();
  await page.getByRole("button", { name: "团队", exact: true }).click();
  await page.getByRole("button", { name: "创建团队", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "创建团队", exact: true });
  await dialog
    .getByLabel("团队名称", { exact: true })
    .fill("yeisme quota test");
  await selectChoice(dialog.getByLabel("团队管理员"), {
    labelContains: "本地体验",
  });
  await dialog.getByRole("button", { name: "保存并继续" }).click();
  await page.getByRole("link", { name: "网盘", exact: true }).click();
  await page.getByRole("button", { name: "创建团队网盘", exact: true }).click();
  const form = page.getByRole("dialog", { name: "创建团队网盘", exact: true });
  await form.getByLabel("网盘名称", { exact: true }).fill("One GiB");
  await selectChoice(form.getByLabel("数据负责人"), {
    labelContains: "本地体验",
  });
  await selectChoice(form.getByLabel("存储连接"), {
    labelContains: "平台存储",
  });
  await form.getByLabel("空间配额（GiB）", { exact: true }).fill("1");
  await form.getByRole("button", { name: "保存并继续" }).click();
  await expect(form).toHaveCount(0);
  await expect(
    page.locator(".drive-card").filter({ hasText: "One GiB" }),
  ).toContainText("1.0 GB");
  await page.getByRole("button", { name: "调整空间配额" }).click();
  const quota = page.getByRole("dialog", { name: "调整空间配额" });
  await quota.getByLabel("空间配额（GiB）", { exact: true }).fill("2");
  await quota.getByRole("button", { name: "保存并继续" }).click();
  await expect(quota).toHaveCount(0);
  await expect(
    page.locator(".drive-card").filter({ hasText: "One GiB" }),
  ).toContainText("2.0 GB");
});
