import { test, expect, type Locator, type Page } from "@playwright/test";
import path from "node:path";

const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS || "temp/browser-artifacts";

async function expectDark(surface: Locator) {
  await expect(surface).toBeVisible();
  const color = await surface.evaluate(el => getComputedStyle(el).backgroundColor);
  const values = color.match(/[\d.]+/g)?.map(Number) ?? [];
  expect(values.length, color).toBeGreaterThanOrEqual(3);
  expect(Math.max(...values.slice(0, 3)), color).toBeLessThan(85);
}

async function login(page: Page) {
  await page.goto("/");
  await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(page.locator(".sidebar")).toBeVisible();
  await page.evaluate(async () => {
    const me = (await (await fetch("/api/v1/me")).json()).data;
    const response = await fetch("/api/v1/spaces", { method: "POST", headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrf_token }, body: JSON.stringify({ name: "主题验收空间", connection_id: "platform" }) });
    if (!response.ok) throw new Error("Fixture space creation failed");
  });
  await page.reload();
}

test("dark surfaces, settings, system mode, persistence and mobile layout", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.emulateMedia({ colorScheme: "dark" });
  await login(page);
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expectDark(page.locator(".topbar"));
  await page.getByRole("button",{name:"卡片视图",exact:true}).click();
  await expectDark(page.locator(".drive-card").first());
  await expectDark(page.getByRole("searchbox", { name: "搜索网盘" }).locator(".."));
  const card = page.locator(".drive-card").first();
  expect(await card.evaluate(el => getComputedStyle(el).color)).toBe("rgb(228, 239, 232)");
  await page.getByRole("combobox", { name: "筛选存储" }).click();
  await expectDark(page.locator('[data-slot="select-content"]'));
  await page.keyboard.press("Escape");
  await page.screenshot({ path: path.join(artifacts, "dark-drives.png"), fullPage: true });

  await page.getByRole("button", { name: "个人设置", exact: true }).click();
  const settings = page.getByTestId("personal-settings");
  await expect(settings.getByRole("heading", { name: "个人设置" })).toBeVisible();
  await settings.getByLabel("显示名称").fill("验收资料用户");
  await settings.getByRole("button", { name: "保存资料" }).click();
  await expect(settings.getByText("个人资料已保存。", { exact: true })).toBeVisible();
  await page.reload();
  await expect(page.getByTestId("personal-settings").getByLabel("显示名称")).toHaveValue("验收资料用户");
  await expectDark(settings.locator('[data-slot="card"]').first());
  await expect(settings.getByRole("radio", { name: "跟随系统" })).toBeChecked();
  await settings.getByRole("radio", { name: /^浅色/ }).check();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  // The capacity shortcut belongs to the dark Pine sidebar, not the page card.
  await expect(page.locator(".storage-meter")).toHaveCSS("background-color", "oklch(0.305 0.048 165)");
  await expect(page.locator(".storage-meter")).toHaveCSS("color", "rgb(228, 239, 232)");
  await expect(settings.locator('[data-slot="card"]').first()).toHaveCSS("background-color", "rgb(255, 255, 255)");
  await expect(settings.getByRole("radio", { name: /^浅色/ }).locator("../..")).toHaveCSS("background-color", "rgb(229, 240, 233)");
  await page.screenshot({ path: path.join(artifacts, "light-settings.png"), fullPage: true });
  await page.emulateMedia({ colorScheme: "light" });
  await settings.getByRole("radio", { name: /^深色/ }).check();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect(page.locator(".storage-meter")).toHaveCSS("background-color", "oklch(0.305 0.048 165)");
  await expect(settings.locator('[data-slot="card"]').first()).toHaveCSS("background-color", "rgb(26, 37, 32)");
  await settings.getByRole("radio", { name: "紧凑" }).check();
  await settings.getByRole("switch", { name: "减少动效" }).check();
  await page.reload();
  await expect(settings).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("data-density", "compact");
  await expect(page.locator("html")).toHaveAttribute("data-motion", "reduced");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.screenshot({ path: path.join(artifacts, "dark-settings.png"), fullPage: true });
  await settings.getByRole("radio", { name: "跟随系统" }).check();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  await page.emulateMedia({ colorScheme: "dark" });
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");

  for (const label of ["任务中心", "分享交付", "团队", "Agent 管理", "访问凭据", "存储管理", "资源态势"]) {
    await page.locator(".sidebar").getByRole("button", { name: label, exact: true }).click();
    await expectDark(page.locator(".content"));
    await expectDark(page.locator(".topbar"));
  }
  await page.locator(".sidebar").getByRole("button", { name: "个人设置", exact: true }).click();
  for (const width of [768, 390]) {
    await page.setViewportSize({ width, height: 844 });
    await expect(settings.getByRole("heading", { name: "个人设置" })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: path.join(artifacts, `settings-${width}.png`), fullPage: true });
  }
  expect(errors).toEqual([]);
});
