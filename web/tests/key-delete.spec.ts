import { keyAction } from "./helpers/key-action";
import { freshOwnerKey } from "./helpers/key-owner";
import { test, expect } from "@playwright/test";

test("owner can delete a spare key but not the last active key", async ({
  page, request,
}) => {
  await page.goto("/");
  await page.locator("#login-key").fill(await freshOwnerKey(request, "key-delete"));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await page.getByRole("button", { name: "访问凭据", exact: true }).click();
  await page.getByRole("tab", { name: "Key", exact: true }).click();
  const current = page.getByRole("row").filter({ hasText: "当前登录" });
  await current.getByRole("button",{name:/的更多操作$/}).click();
  await expect(page.getByRole("menu",{name:/的更多操作$/})).toBeVisible();
  await expect(page.getByRole("menuitem",{name:"删除",exact:true})).toHaveCount(0);
  await page.getByRole("menuitem",{name:"查看 Key",exact:true}).press("Escape");
  await expect(page.getByRole("menu",{name:/的更多操作$/})).toBeHidden();
  await page.getByRole("button", { name: "新建 Key", exact: true }).click();
  const create = page.getByRole("dialog", { name: "新建 Key", exact: true });
  await create.getByLabel("Key 名称").fill("备用电脑");
  await create.getByRole("button", { name: "创建 Key", exact: true }).click();
  await page.getByRole("dialog", { name: "查看与复制 Key", exact: true }).getByRole("button", { name: "关闭", exact: true }).first().click();
  const spare = page.getByRole("row").filter({ hasText: "备用电脑" });
  await keyAction(spare,"删除");
  await page.getByRole("button", { name: "确认删除", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("已删除");
  const keyStatus = page.getByRole("combobox", { name: "Key 状态" });
  await keyStatus.click();
  await page.getByRole("option", { name: "已归档", exact: true }).click();
  await expect(spare).toContainText("已归档");
});
