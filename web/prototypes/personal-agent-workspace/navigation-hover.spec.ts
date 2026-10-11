import { test, expect } from "@playwright/test";
import { folder7, file7, nav7 } from "./v7-helpers";
import type { Page } from "@playwright/test";
const errors = new WeakMap<Page, string[]>();
test.beforeEach(async ({ page }) => {
  const list: string[] = [];
  errors.set(page, list);
  page.on("pageerror", (e) => list.push(e.message));
  await page.goto("/opendesign.html");
});
test.afterEach(async ({ page }) => expect(errors.get(page)).toEqual([]));
test("pointer navigation at 855px previews without a backdrop or layout shift", async ({
  page,
}, info) => {
  test.skip(info.project.name !== "desktop", "Fine pointer behavior");
  await page.setViewportSize({ width: 855, height: 953 });
  const trigger = page.getByRole("button", { name: "显示主导航", exact: true });
  await expect(trigger).toBeVisible();
  const before = await page.locator("[data-directory-toolbar]").boundingBox();
  await page.clock.install();
  await page.clock.pauseAt(new Date());
  await trigger.hover();
  await page.clock.runFor(149);
  await expect(page.locator("[data-floating-navigation]")).toHaveCount(0);
  await page.clock.runFor(1);
  const nav = page.locator("[data-floating-navigation]");
  await expect(nav).toBeVisible();
  await expect(page.locator('[data-slot="dialog-overlay"]')).toHaveCount(0);
  const after = await page.locator("[data-directory-toolbar]").boundingBox();
  expect(after?.x).toBe(before?.x);
  expect(after?.width).toBe(before?.width);
  await nav.getByRole("button", { name: "知识库", exact: true }).hover();
  await page.clock.runFor(500);
  await expect(nav).toBeVisible();
  await page.mouse.move(600, 300);
  await page.clock.runFor(219);
  await expect(nav).toBeVisible();
  await page.clock.runFor(1);
  await expect(nav).toHaveCount(0);
  // The slim activation zone is an overlay, not a reserved navigation column.
  await page.locator("[data-nav-hover-edge]").hover();
  await page.clock.runFor(150);
  await expect(nav).toBeVisible();
  await nav.getByRole("button", { name: "知识库", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "知识库", exact: true }),
  ).toBeVisible();
  await expect(nav).toHaveCount(0);
});
test("folded desktop peeks and can pin without changing stored preference on hover", async ({
  page,
}, info) => {
  test.skip(info.project.name !== "desktop", "Desktop pinning");
  await page.getByRole("button", { name: "收起主导航", exact: true }).click();
  const preference = await page.evaluate(() =>
    localStorage.getItem("drivebridge-prototype-v3-preferences"),
  );
  await page.mouse.move(700, 500);
  await page.getByRole("button", { name: "显示主导航", exact: true }).hover();
  await expect(page.locator("[data-floating-navigation]")).toBeVisible();
  expect(
    await page.evaluate(() =>
      localStorage.getItem("drivebridge-prototype-v3-preferences"),
    ),
  ).toBe(preference);
  await page.getByRole("button", { name: "固定主导航", exact: true }).click();
  await expect(
    page.getByRole("complementary", { name: "主导航", exact: true }),
  ).toBeVisible();
  await expect(page.locator("[data-floating-navigation]")).toHaveCount(0);
});
test("early leave cancels pending hover and focus is never stolen", async ({
  page,
}, info) => {
  test.skip(info.project.name !== "desktop", "Hover cancellation");
  await page.setViewportSize({ width: 855, height: 953 });
  const input = page.getByRole("textbox", {
    name: "搜索当前目录",
    exact: true,
  });
  await input.focus();
  await input.fill("草稿");
  await page.clock.install();
  await page.clock.pauseAt(new Date());
  const trigger = page.getByRole("button", { name: "显示主导航", exact: true });
  await trigger.hover();
  await page.clock.runFor(100);
  await page.mouse.move(650, 300);
  await page.clock.runFor(400);
  await expect(page.locator("[data-floating-navigation]")).toHaveCount(0);
  await trigger.hover();
  await page.clock.runFor(150);
  await expect(page.locator("[data-floating-navigation]")).toBeVisible();
  await expect(input).toBeFocused();
  await expect(input).toHaveValue("草稿");
  await page.mouse.move(650, 300);
  await page.clock.runFor(220);
  await expect(input).toBeFocused();
});
test("keyboard navigation restores focus and Escape leaves preview panels intact", async ({
  page,
}, info) => {
  test.skip(info.project.name !== "desktop", "Nonmodal keyboard path");
  await page.setViewportSize({ width: 855, height: 953 });
  await file7(page);
  const before = await page.locator("[data-details-content]").count();
  const trigger = page.getByRole("button", { name: "显示主导航", exact: true });
  await trigger.focus();
  await trigger.press("Enter");
  const nav = page.locator("[data-floating-navigation]");
  await expect(nav).toBeVisible();
  expect(await nav.evaluate((el) => el.contains(document.activeElement))).toBe(
    true,
  );
  await page.keyboard.press("Escape");
  await expect(nav).toHaveCount(0);
  await expect(trigger).toBeFocused();
  await expect(page.locator("[data-preview-workspace]")).toBeVisible();
  expect(await page.locator("[data-details-content]").count()).toBe(before);
  await trigger.hover();
  await expect(nav).toBeVisible();
  await nav.getByRole("button", { name: "个人设置", exact: true }).focus();
  await page.keyboard.press("Tab");
  await expect(nav).toHaveCount(0);
});
test("touch opens by click and does not create a hover edge", async ({
  page,
}, info) => {
  test.skip(info.project.name !== "mobile", "Touch path");
  for (const width of [390, 855]) {
    await page.setViewportSize({ width, height: 953 });
    await expect(page.locator("[data-nav-hover-edge]")).toHaveCount(0);
    const trigger = page.getByRole("button", {
      name: "显示主导航",
      exact: true,
    });
    await trigger.dispatchEvent("pointerenter", { pointerType: "touch" });
    await page.waitForTimeout(180);
    await expect(
      page.getByRole("dialog", { name: "工作区导航", exact: true }),
    ).toHaveCount(0);
    await trigger.click();
    await expect(
      page.getByRole("dialog", { name: "工作区导航", exact: true }),
    ).toBeVisible();
    await page.getByRole("button", { name: "折叠主导航", exact: true }).click();
    await expect(
      page.getByRole("dialog", { name: "工作区导航", exact: true }),
    ).toHaveCount(0);
  }
});
test("outside clicks, reduced motion and width changes dismiss floating navigation", async ({
  page,
}, info) => {
  test.skip(
    info.project.name !== "desktop",
    "Fine pointer breakpoint behavior",
  );
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.setViewportSize({ width: 1024, height: 900 });
  const trigger = page.getByRole("button", { name: "显示主导航", exact: true });
  await trigger.click();
  await expect(page.locator("[data-floating-navigation]")).toBeVisible();
  await page
    .locator("[data-directory-toolbar]")
    .click({ position: { x: 500, y: 20 } });
  await expect(page.locator("[data-floating-navigation]")).toHaveCount(0);
  await trigger.hover();
  await expect(page.locator("[data-floating-navigation]")).toBeVisible();
  await page.setViewportSize({ width: 768, height: 900 });
  await expect(page.locator("[data-floating-navigation]")).toHaveCount(0);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth + 1,
    ),
  ).toBe(true);
});
