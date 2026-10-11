import { keyAction } from "./helpers/key-action";
import { freshOwnerKey } from "./helpers/key-owner";
import { test, expect } from "@playwright/test";

test("owner can disable and enable a spare key", async ({ page, request }) => {
  await page.goto("/");
  await page.locator("#login-key").fill(await freshOwnerKey(request, "key-disable"));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await page.getByRole("button", { name: "访问凭据", exact: true }).click();
  await page.getByRole("tab", { name: "Key", exact: true }).click();
  await page.getByRole("button", { name: "新建 Key", exact: true }).click();
  const create = page.getByRole("dialog", { name: "新建 Key", exact: true });
  await create.getByLabel("Key 名称").fill("出差电脑");
  await create.getByRole("button", { name: "创建 Key", exact: true }).click();
  const shown = page.getByRole("dialog", { name: "查看与复制 Key", exact: true });
  const spareKey = await shown.getByLabel("完整 Key").inputValue();
  await shown.getByRole("button", { name: "关闭", exact: true }).first().click();
  const row = page.getByRole("row").filter({ hasText: "出差电脑" });
  await keyAction(row,"禁用");
  await page.getByRole("button", { name: "确认禁用", exact: true }).click();
  await expect(row).toContainText("已禁用");
  expect((await request.post("/auth/key", { data: { key: spareKey } })).status()).toBe(401);
  await keyAction(row,"启用");
  await expect(row).toContainText("有效");
  expect((await request.post("/auth/key", { data: { key: spareKey } })).status()).toBe(200);
});
