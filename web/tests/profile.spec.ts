import { test, expect } from "@playwright/test";
import { selectChoice } from "./helpers/select";
test("profile saves across sessions, validates ownership, preserves failed edits and resets appearance", async ({
  page,
  context,
  browser,
}) => {
  await page.goto("/");
  await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await page.getByRole("button", { name: "个人设置", exact: true }).click();
  const settings = page.getByTestId("personal-settings");
  await settings.getByLabel("显示名称").fill("跨设备资料");
  await page.route("**/api/v1/me", async (route) => {
    if (route.request().method() === "PATCH")
      await route.fulfill({
        status: 503,
        contentType: "application/json",
        body: JSON.stringify({ error: { code: "database_unavailable" } }),
      });
    else await route.continue();
  });
  await settings.getByRole("button", { name: "保存资料" }).click();
  await expect(settings.getByRole("alert")).toBeVisible();
  await expect(settings.getByLabel("显示名称")).toHaveValue("跨设备资料");
  await page.unroute("**/api/v1/me");
  await settings.getByRole("button", { name: "保存资料" }).click();
  await expect(settings.getByText("个人资料已保存。")).toBeVisible();
  const other = await browser.newContext();
  try {
    const second = await other.newPage();
    await second.goto("/");
    await second.locator("#login-key").fill("dbu_" + "2".repeat(96));
    await second.getByRole("button", { name: "登录", exact: true }).click();
    await second.getByRole("button", { name: "个人设置", exact: true }).click();
    await expect(second.getByLabel("显示名称")).toHaveValue("跨设备资料");
  } finally {
    await other.close();
  }
  await settings.getByLabel("显示名称").fill(" ");
  await expect(
    settings.getByRole("button", { name: "保存资料" }),
  ).toBeDisabled();
  await settings.getByRole("radio", { name: /^深色/ }).check();
  await settings.getByRole("button", { name: "恢复默认外观" }).click();
  await expect(settings.getByRole("radio", { name: "跟随系统" })).toBeChecked();
  const before = await page.request.get("/api/v1/me");
  const me = (await before.json()).data;
  const denied = await page.request.patch("/api/v1/me", {
    headers: {
      "X-CSRF-Token": me.csrf_token,
      Origin: new URL(page.url()).origin,
    },
    data: { display_name: "changed", avatar_ref: "https://example.com/avatar" },
  });
  expect(denied.status()).toBe(400);
  const after = (await (await page.request.get("/api/v1/me")).json()).data;
  expect(after.user.display_name).toBe("跨设备资料");
  await settings.getByRole("combobox", { name: "选择头像" }).fill("不存在");
  await expect(page.getByText("没有匹配结果")).toBeVisible();
  await page.keyboard.press("Escape");
});

test('selects an owned image as a persistent avatar and clears without deleting the file',async({page})=>{
 await page.goto('/');await page.locator('#login-key').fill('dbu_'+'2'.repeat(96));await page.getByRole('button',{name:'登录',exact:true}).click();await page.locator('.sidebar').waitFor();
 const name='头像验收空间';
 await page.evaluate(async name=>{const me=(await(await fetch('/api/v1/me')).json()).data;const response=await fetch('/api/v1/spaces',{method:'POST',headers:{'Content-Type':'application/json','X-CSRF-Token':me.csrf_token},body:JSON.stringify({name,connection_id:'platform'})});if(!response.ok)throw new Error('Fixture space creation failed');},name);
 await page.reload();await page.getByRole('button',{name:'打开网盘 '+name}).click();
 await page.locator('input[type=file]').first().setInputFiles({name:'profile-avatar.png',mimeType:'image/png',buffer:Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jw1sAAAAASUVORK5CYII=','base64')});
 await expect(page.getByRole('button',{name:/profile-avatar.png/}).first()).toBeVisible();
 await page.getByRole('button',{name:'个人设置',exact:true}).click();const settings=page.getByTestId('personal-settings');const picker=settings.getByRole('combobox',{name:'选择头像'});
 await picker.fill('profile-avatar');await page.getByRole('option',{name:'profile-avatar.png',exact:true}).click();await settings.getByRole('button',{name:'保存资料'}).click();await expect(settings.getByRole('img',{name:'我的头像'})).toBeVisible();await page.reload();await expect(settings.getByRole('img',{name:'我的头像'})).toBeVisible();
 const clear=picker.locator('..').locator('..').locator('[data-slot=combobox-clear]');await clear.click();await settings.getByRole('button',{name:'保存资料'}).click();await expect(settings.getByRole('img',{name:'我的头像'})).toHaveCount(0);
 const files=await page.request.get('/api/v1/me/avatar-options?q=profile-avatar');expect((await files.json()).data.items).toHaveLength(1);
});
