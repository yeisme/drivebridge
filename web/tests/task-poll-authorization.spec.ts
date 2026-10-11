import { test, expect } from "@playwright/test";

test("authorization loss clears the task projection and stops automatic reads", async ({ page }) => {
  let denied = false;
  let reads = 0;
  for (const endpoint of ["operations", "inbox"]) {
    await page.route(`**/api/v1/${endpoint}`, route => {
      reads++;
      expect(route.request().method()).toBe("GET");
      return route.fulfill(denied
        ? { status: 403, json: { error: { code: "forbidden" } } }
        : { json: { data: [{ id: "fixture-operation", path: "authorization-fixture.txt", kind: "upload", status: "running", size_bytes: 10, bytes_transferred: 1, space_id: "fixture-space" }] } });
    });
  }
  await page.goto("/#/transfers");
  await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(page.getByText("authorization-fixture.txt", { exact: true })).toBeVisible();
  denied = true;
  await expect(page.getByRole("alert")).toContainText("任务读取权限已失效", { timeout: 6000 });
  await expect(page.getByText("authorization-fixture.txt", { exact: true })).toHaveCount(0);
  const stopped = reads;
  await page.evaluate(() => document.dispatchEvent(new Event("visibilitychange")));
  // Observe longer than the production polling interval, not a retry timeout.
  await page.waitForTimeout(4500);
  expect(reads).toBe(stopped);
  await expect(page.getByText("还没有传输记录", { exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "清理历史", exact: true })).toHaveCount(0);
  await page.goto("/#/inbox?view=legacy");
  await expect(page.getByRole("alert")).toContainText("任务读取权限已失效");
  await expect(page.getByText("最近没有产物", { exact: true })).toHaveCount(0);
  await expect(page.getByText("authorization-fixture.txt", { exact: true })).toHaveCount(0);
  await page.waitForTimeout(4500);
  expect(reads).toBe(stopped);
  denied = false;
  await page.route("**/api/v1/me", async route => {
    const response = await route.fetch();
    const body = await response.json();
    body.data.user.id = "fixture-other-identity";
    await route.fulfill({ response, json: body });
  });
  await page.getByRole("button", { name: "重新检查访问权限" }).click();
  await expect(page.getByRole("button", { name: "重新检查访问权限" })).toBeEnabled();
  await expect(page.getByRole("heading", { name: "任务记录暂不可访问" })).toBeVisible();
  expect(reads).toBe(stopped);
  await expect(page.getByText("authorization-fixture.txt", { exact: true })).toHaveCount(0);
  await page.unroute("**/api/v1/me");
  await page.getByRole("button", { name: "重新检查访问权限" }).click();
  await expect(page.getByText("authorization-fixture.txt", { exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "任务记录暂不可访问" })).toHaveCount(0);
  expect(reads).toBe(stopped + 2);
});
