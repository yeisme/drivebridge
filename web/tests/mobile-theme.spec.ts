import { test, expect } from "@playwright/test";
import path from "node:path";

for (const theme of ["light", "dark"] as const) {
  test(`mobile navigation resolves ${theme} semantic colors`, async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.addInitScript(value => localStorage.setItem("drivebridge:theme", value), theme);
    await page.goto("/");
    await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
    await page.getByRole("button", { name: "登录", exact: true }).click();
    const nav = page.getByRole("navigation", { name: "手机主导航" });
    await expect(nav).toBeVisible();
    await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
    const colors = await nav.evaluate(el => {
      const style = getComputedStyle(el);
      const selected = getComputedStyle(el.querySelector('[aria-current="page"]')!);
      const probe = document.createElement("span");
      probe.style.backgroundColor = "var(--ui-background)";
      probe.style.color = "var(--ui-foreground)";
      el.append(probe);
      const expected = getComputedStyle(probe);
      const result = { background: style.backgroundColor, foreground: style.color,
        expectedBackground: expected.backgroundColor, expectedForeground: expected.color,
        selected: selected.backgroundColor };
      probe.remove();
      return result;
    });
    expect(colors.background).not.toBe("rgba(0, 0, 0, 0)");
    expect(colors.background).toBe(colors.expectedBackground);
    expect(colors.foreground).toBe(colors.expectedForeground);
    expect(colors.selected).not.toBe(colors.background);
    for (const button of await nav.getByRole("button").all()) {
      const box = await button.boundingBox();
      expect(box!.height).toBeGreaterThanOrEqual(44);
      expect(box!.width).toBeGreaterThanOrEqual(44);
    }
    if (process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS) await page.screenshot({ path: path.join(process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS, `mobile-${theme}.png`) });
  });
}
