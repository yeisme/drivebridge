import { test, expect } from "@playwright/test";

// Half of a 1440x900 viewport exercises the available CSS layout space at
// 200% desktop zoom. This is reflow coverage, not browser zoom emulation.
test("shell remains navigable in reduced layout space with reduced motion", async ({ page }) => {
  await page.setViewportSize({ width: 720, height: 450 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/");
  await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  const nav = page.getByRole("navigation", { name: "手机主导航" });
  await expect(nav).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  const box = await nav.boundingBox();
  expect(box!.y).toBeGreaterThanOrEqual(0);
  expect(box!.y + box!.height).toBeLessThanOrEqual(450);
  await nav.getByRole("button", { name: "我的", exact: true }).click();
  await expect(nav.getByRole("button", { name: "我的", exact: true })).toHaveAttribute("aria-current", "page");
  const trigger = page.getByRole("button", { name: "打开全部导航" });
  await trigger.click();
  const dialog = page.getByRole("dialog", { name: "全部导航" });
  await expect(dialog).toBeVisible();
  const motion = await page.locator('[data-slot="sheet-content"], [data-slot="sheet-overlay"]').evaluateAll(elements =>
    elements.map(element => {
      const style = getComputedStyle(element);
      const seconds = (value: string) => value.split(",").map(part => {
        const text = part.trim();
        return parseFloat(text) / (text.endsWith("ms") ? 1000 : 1);
      });
      return { transition: seconds(style.transitionDuration), animation: seconds(style.animationDuration), scroll: style.scrollBehavior };
    }),
  );
  expect(motion).toHaveLength(2);
  for (const styles of motion) {
    for (const duration of [...styles.transition, ...styles.animation]) expect(duration).toBeLessThanOrEqual(0.00001);
    expect(styles.scroll).toBe("auto");
  }
  // Scroll within the existing sheet to the final action, not the body.
  const logout = dialog.getByRole("button", { name: "退出登录", exact: true });
  await logout.scrollIntoViewIfNeeded();
  const action = await logout.boundingBox();
  expect(action!.y).toBeGreaterThanOrEqual(0);
  expect(action!.y + action!.height).toBeLessThanOrEqual(450);
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);
  await expect(trigger).toBeFocused();
  expect(await page.evaluate(() => matchMedia("(prefers-reduced-motion: reduce)").matches)).toBe(true);
});
