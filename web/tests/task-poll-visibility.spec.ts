import { test, expect } from "@playwright/test";

test("task polling pauses on hidden state and resumes without writes", async ({ page }) => {
  let operationReads = 0;
  let inboxReads = 0;
  await page.route("**/api/v1/operations", route => {
    expect(route.request().method()).toBe("GET");
    operationReads++;
    return route.fulfill({ json: { data: [{ id: "visibility-fixture", path: "visibility.txt", kind: "upload", status: "running", size_bytes: 10, bytes_transferred: 1, space_id: "fixture-space" }] } });
  });
  await page.route("**/api/v1/inbox", route => {
    expect(route.request().method()).toBe("GET");
    inboxReads++;
    return route.fulfill({ json: { data: [] } });
  });
  await page.goto("/#/transfers");
  await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(page.getByRole("list", { name: "传输记录" })).toBeVisible();
  await expect.poll(() => operationReads).toBeGreaterThan(0);
  await expect.poll(() => inboxReads).toBeGreaterThan(0);
  await page.evaluate(() => {
    Object.defineProperty(document, "hidden", { configurable: true, get: () => true });
    document.dispatchEvent(new Event("visibilitychange"));
  });
  // Mocked responses may already be in flight when visibility changes.
  await page.waitForTimeout(200);
  const hiddenCounts = [operationReads, inboxReads];
  // Deliberate observation window longer than the active 2s interval.
  await page.waitForTimeout(4500);
  expect([operationReads, inboxReads]).toEqual(hiddenCounts);
  const resumeReads: string[] = [];
  page.on("request", request => {
    const pathname = new URL(request.url()).pathname;
    if (["/api/v1/me", "/api/v1/operations", "/api/v1/inbox"].includes(pathname)) resumeReads.push(pathname);
  });
  await page.evaluate(() => {
    Object.defineProperty(document, "hidden", { configurable: true, get: () => false });
    document.dispatchEvent(new Event("visibilitychange"));
  });
  await expect.poll(() => operationReads, { timeout: 6000 }).toBeGreaterThan(hiddenCounts[0]);
  await expect.poll(() => inboxReads).toBeGreaterThan(hiddenCounts[1]);
  expect(resumeReads[0]).toBe("/api/v1/me");
  await page.evaluate(() => {
    Object.defineProperty(document, "hidden", { configurable: true, get: () => true });
    document.dispatchEvent(new Event("visibilitychange"));
  });
  await page.route("**/api/v1/me", route => route.fulfill({ status: 401, json: { error: { code: "unauthorized" } } }));
  const beforeDenial = [operationReads, inboxReads];
  await page.evaluate(() => {
    Object.defineProperty(document, "hidden", { configurable: true, get: () => false });
    document.dispatchEvent(new Event("visibilitychange"));
  });
  await expect(page.getByRole("heading", { name: "任务记录暂不可访问" })).toBeVisible();
  expect([operationReads, inboxReads]).toEqual(beforeDenial);
  await expect(page.getByText("visibility.txt", { exact: true })).toHaveCount(0);
});
