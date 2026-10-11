import { test, expect } from "@playwright/test";

test("task polling honors Retry-After and retains the previous projection", async ({ page }) => {
  let failNext = false;
  let failedAt = 0;
  let recoveredAt = 0;
  let reads = 0;
  const operation = { id: "retry-fixture", path: "retry-fixture.txt", kind: "upload", status: "running", size_bytes: 10, bytes_transferred: 1, space_id: "fixture-space" };
  await page.route("**/api/v1/operations", route => {
    expect(route.request().method()).toBe("GET");
    reads++;
    if (failNext) {
      failNext = false;
      failedAt = Date.now();
      return route.fulfill({ status: 503, headers: { "Retry-After": "6" }, json: { error: { code: "unavailable" } } });
    }
    if (failedAt && !recoveredAt) recoveredAt = Date.now();
    return route.fulfill({ json: { data: [operation] } });
  });
  await page.route("**/api/v1/inbox", route => route.fulfill({ json: { data: [] } }));
  await page.goto("/#/transfers");
  await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(page.getByText(operation.path, { exact: true })).toBeVisible();
  failNext = true;
  await expect.poll(() => failedAt, { timeout: 6000 }).toBeGreaterThan(0);
  const failedReads = reads;
  await expect(page.getByText(operation.path, { exact: true })).toBeVisible();
  // Longer than ordinary 4s polling, shorter than the server's 6s minimum.
  await page.waitForTimeout(4200);
  expect(reads).toBe(failedReads);
  await expect.poll(() => recoveredAt, { timeout: 5000 }).toBeGreaterThan(0);
  expect(recoveredAt - failedAt).toBeGreaterThanOrEqual(6000);
  await expect(page.getByText(operation.path, { exact: true })).toBeVisible();
});
