import { test, expect } from "@playwright/test";
import path from "node:path";

for (const theme of ["light", "dark"]) {
  test(`pine sidebar stays scoped and readable in ${theme}`, async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.addInitScript(theme => localStorage.setItem("drivebridge:theme", theme), theme);
    await page.goto("/");
    await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
    await page.getByRole("button", { name: "登录", exact: true }).click();
    const sidebar = page.locator(".sidebar");
    await expect(sidebar).toBeVisible();
    const results = await sidebar.evaluate(el => {
      const canvas = document.createElement("canvas");
      const ctx = canvas.getContext("2d")!;
      function rgb(color: string) {
        ctx.clearRect(0, 0, 1, 1); ctx.fillStyle = color; ctx.fillRect(0, 0, 1, 1);
        return Array.from(ctx.getImageData(0, 0, 1, 1).data).slice(0, 3);
      }
      function luminance(color: string) {
        const c = rgb(color).map(v => { v /= 255; return v <= .04045 ? v / 12.92 : ((v + .055) / 1.055) ** 2.4; });
        return c[0] * .2126 + c[1] * .7152 + c[2] * .0722;
      }
      const background = getComputedStyle(el).backgroundColor;
      return Array.from(el.querySelectorAll<HTMLButtonElement>(".nav-item")).filter(button => button.getBoundingClientRect().height > 0).map(button => {
        const style = getComputedStyle(button);
        const bg = button.classList.contains("active") ? style.backgroundColor : background;
        const a = luminance(style.color), b = luminance(bg);
        return { height: button.getBoundingClientRect().height, contrast: (Math.max(a, b) + .05) / (Math.min(a, b) + .05) };
      });
    });
    expect(results.length).toBeGreaterThan(5);
    for (const result of results) {
      // Desktop Pine rows are 36px; mobile/touch navigation has its separate 44px gate.
      expect(result.height).toBe(36);
      expect(result.contrast).toBeGreaterThanOrEqual(4.5);
    }
    const colors = await page.evaluate(() => ({ sidebar: getComputedStyle(document.querySelector(".sidebar")!).color, main: getComputedStyle(document.querySelector(".main")!).color }));
    if (theme === "light") expect(colors.sidebar).not.toBe(colors.main);
    if (process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS) await page.screenshot({ path: path.join(process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS, `sidebar-${theme}.png`) });
  });
}
