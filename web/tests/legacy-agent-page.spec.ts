import { test, expect } from "@playwright/test";
import { selectChoice } from "./helpers/select";

test("legacy Agent filtering and cancellation keep writes behind confirmation", async ({ page }) => {
  const active = { id: "fixture-active", name: "检索助手", space_id: "fixture-space", prefix: "docs/", scopes: "read", expires_at: "2099-01-01T00:00:00Z", revoked: false };
  const agents = [active, { ...active, id: "fixture-revoked", name: "旧助手", revoked: true }, { ...active, id: "fixture-archived", name: "归档助手", revoked: true, archived: true }];
  let writes = 0;
  await page.route("**/api/v1/agents**", async route => {
    if (route.request().method() !== "GET") {
      writes++;
      return route.fulfill({ status: 403, json: { error: { code: "fixture_write_forbidden" } } });
    }
    return route.fulfill({ json: { data: agents } });
  });
  await page.goto("/#/agents");
  await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await page.getByRole("tab", { name: "旧版机器人授权", exact: true }).click();
  const list = page.getByRole("list", { name: "机器人授权" });
  await expect(list.getByRole("listitem")).toHaveCount(2);
  await page.getByRole("searchbox", { name: "搜索机器人" }).fill("检索");
  await expect(list.getByRole("listitem")).toHaveCount(1);
  await list.getByRole("button", { name: "撤销授权" }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByRole("button", { name: "确认撤销" })).toBeVisible();
  expect(writes).toBe(0);
  await dialog.getByRole("button", { name: "取消", exact: true }).click();
  await expect(dialog).toHaveCount(0);
  await page.getByRole("searchbox", { name: "搜索机器人" }).fill("");
  await selectChoice(page.getByRole("combobox", { name: "机器人状态" }), { label: "已撤销" });
  await expect(list.getByRole("listitem")).toHaveCount(1);
  await expect(list.getByRole("button", { name: "撤销授权" })).toHaveCount(0);
  await selectChoice(page.getByRole("combobox", { name: "机器人状态" }), { label: "全部" });
  await page.getByRole("checkbox", { name: "显示已归档" }).check();
  await expect(list.getByRole("listitem")).toHaveCount(3);
  await expect(list.getByRole("listitem").filter({ hasText: "归档助手" }).getByRole("button")).toHaveCount(0);
  expect(writes).toBe(0);
});
