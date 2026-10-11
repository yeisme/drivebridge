import { test, expect } from "@playwright/test";

test("collapsed group projects only unfinished loaded operations", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  let finished = false;
  await page.route("**/api/v1/operations", route => route.fulfill({
    json: { data: [
      { id: "fixture-active", kind: "upload", status: finished ? "completed" : "verifying", path: "fixture.txt", size_bytes: 0, bytes_transferred: 0, space_id: "fixture", sha256: "fixture" },
      { id: "fixture-done", kind: "upload", status: "completed", path: "done.txt", size_bytes: 0, bytes_transferred: 0, space_id: "fixture", sha256: "fixture" },
    ] },
  }));
  await page.goto("/");
  await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  const group = page.locator(".sidebar").getByRole("button", { name: "工作台", exact: true });
  await group.click();
  const count = group.locator(".nav-count");
  await expect(count).toHaveText("1");
  await expect(count).toHaveAttribute("aria-label", "当前已加载列表有 1 个未结束任务");
  finished = true;
  await page.evaluate(() => window.dispatchEvent(new Event("drivebridge-governance-changed")));
  await expect(count).toHaveCount(0);
  await expect(group).toHaveAttribute("aria-expanded", "false");
});
