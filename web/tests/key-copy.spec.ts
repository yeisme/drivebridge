import { keyAction } from "./helpers/key-action";
import { test, expect } from "@playwright/test";
import path from "node:path";

test("team demo keeps identity switching after key rotation and styles context controls", async ({ page }) => {
  test.skip(process.env.DRIVEBRIDGE_TEST_TEAM_DEMO !== "1", "Requires the explicit team demo fixture");
  await page.goto("/");
  await page.getByRole("button", { name: "无需账号，进入本地体验" }).click();
  await page.getByRole("button", { name: "访问凭据", exact: true }).click();
  await page.getByRole("tab", { name: "Key", exact: true }).click();
  await page.getByRole("button",{name:"本地体验 的更多操作",exact:true}).click();
  await page.getByRole("menuitem",{name:"轮转 Key",exact:true}).click();
  await page.getByRole("button", { name: "确认轮转", exact: true }).click();
  await expect(page.getByRole("dialog", { name: "查看与复制 Key" })).toBeVisible();
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "工作区选项", exact: true }).click();
  await Promise.all([
    page.waitForEvent("load"),
    page.getByRole("button", { name: "切换体验身份", exact: true }).click(),
  ]);
  await page.getByRole("button", { name: "访问凭据", exact: true }).click();
  await page.getByRole("tab", { name: "Key", exact: true }).click();
  await expect(page.getByRole("button", { name: "复制 本地体验 的 Key", exact: true }).first()).toBeVisible();
  const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS || "temp/browser-artifacts";
  for (const viewport of [{ width: 868, height: 962 }, { width: 390, height: 844 }]) {
    await page.setViewportSize(viewport);
    if (viewport.width < 768) await page.getByRole("button", { name: "工作区选项", exact: true }).click();
    const select = page.getByRole("combobox", { name: "切换租户" });
    await select.focus();
    await expect(select).toHaveAttribute("aria-autocomplete", "list");
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: path.join(artifacts, `key-context-${viewport.width}.png`), fullPage: true });
  }
});

test("own key supports repeat copy, rotation, session isolation and responsive controls", async ({ page, context, browser }) => {
  const initial = "dbu_" + "1".repeat(96);
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  await page.goto("/");
  await page.locator("#login-key").fill(initial);
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await page.getByRole("button", { name: "访问凭据", exact: true }).click();
  await page.getByRole("tab", { name: "Key", exact: true }).click();
  const row = page.getByRole("row").filter({ hasText: "e2e-key-user" }).filter({ has: page.locator(".key-status.valid") });
  await expect(row).toContainText("当前登录");
  await row.getByRole("button", { name: "复制 e2e-key-user 的 Key", exact: true }).click();
  await expect.poll(() => page.evaluate(async expected => (await navigator.clipboard.readText()) === expected, initial)).toBe(true);
  await row.getByRole("button", { name: "再次复制 e2e-key-user 的 Key", exact: true }).click();
  await expect.poll(() => page.evaluate(async expected => (await navigator.clipboard.readText()) === expected, initial)).toBe(true);
  await keyAction(row,"查看 Key");
  await expect(page.getByLabel("完整 Key", { exact: true })).toHaveAttribute("type", "password");
  await page.getByRole("button", { name: "显示 Key", exact: true }).click();
  expect((await page.getByLabel("完整 Key", { exact: true }).inputValue()) === initial).toBe(true);
  await page.keyboard.press("Escape");
  await expect(page.getByLabel("完整 Key", { exact: true })).toHaveCount(0);

  const other = await browser.newContext();
  try {
    const second = await other.newPage();
    await second.goto("/");
    await second.locator("#login-key").fill(initial);
    await second.getByRole("button", { name: "登录", exact: true }).click();
    await expect(second.getByRole("heading", { name: "我的网盘", exact: true })).toBeVisible();
    await keyAction(row,"轮转 Key");
    await expect(page.getByRole("dialog", { name: "轮转 Key" })).toContainText("旧 Key 立即失效");
    await page.getByRole("button", { name: "确认轮转", exact: true }).click();
    const secret = page.getByRole("dialog", { name: "查看与复制 Key" });
    await expect(secret).toBeVisible();
    const fresh = await secret.getByLabel("完整 Key", { exact: true }).inputValue();
    expect(fresh !== initial && fresh.startsWith("dbu_")).toBe(true);
    await secret.getByRole("button", { name: "复制 Key", exact: true }).click();
    await secret.getByRole("button", { name: "再次复制 Key", exact: true }).click();
    await expect.poll(() => page.evaluate(async expected => (await navigator.clipboard.readText()) === expected, fresh)).toBe(true);
    await page.keyboard.press("Escape");
    expect((await other.request.get(new URL("/api/v1/me", page.url()).href)).status()).toBe(401);
    await page.reload();
    await expect(page.getByRole("heading", { name: "访问凭据", exact: true })).toBeVisible();
    await row.getByRole("button", { name: "复制 e2e-key-user 的 Key", exact: true }).click();
    await expect.poll(() => page.evaluate(async expected => (await navigator.clipboard.readText()) === expected, fresh)).toBe(true);
    const keyStatus = page.getByRole("combobox", { name: "Key 状态" });
    await keyStatus.click();
    await page.getByRole("option", { name: "有效", exact: true }).click();
    const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS || "temp/browser-artifacts";
    for (const viewport of [{ width: 1440, height: 1000 }, { width: 868, height: 962 }, { width: 390, height: 844 }]) {
      await page.setViewportSize(viewport);
      await expect(row.getByRole("button", { name: /再次复制/ })).toBeVisible();
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await page.screenshot({ path: path.join(artifacts, `key-controls-${viewport.width}.png`), fullPage: true });
    }
    // Clipboard denial still gives a masked, manually selectable key.
    await page.evaluate(() => Object.defineProperty(navigator.clipboard, "writeText", { configurable: true, value: async () => { throw new Error("denied"); } }));
    await row.getByRole("button", { name: /再次复制/ }).click();
    await expect(page.getByRole("dialog", { name: "查看与复制 Key" })).toContainText("手动复制");
    await expect(page.getByLabel("完整 Key", { exact: true })).toHaveAttribute("type", "password");
  } finally {
    await other.close();
    await context.clearPermissions();
  }
});
