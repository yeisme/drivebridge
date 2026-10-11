import { test, expect } from "@playwright/test";

test("command search distinguishes complete empty results from partial and unknown coverage", async ({ page }) => {
  let coverage: string | undefined = "complete";
  let queries = 0;
  await page.route("**/api/v1/operations/lookup", route => route.fulfill({ json: { data: { found: false } } }));
  await page.route("**/api/v1/file-queries", route => {
    queries++;
    return route.fulfill({ json: { data: { mode: "exact", items: [], next_cursor: null, coverage } } });
  });
  await page.goto("/#/files");
  await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await page.getByRole("button", { name: "搜索工作区", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "搜索工作区" });
  await expect(dialog.getByText(/尚未读取搜索结果/)).toBeVisible();
  expect(queries).toBe(0);
  for (const value of ["complete", "partial", undefined]) {
    coverage = value;
    await dialog.getByRole("searchbox", { name: "检索条件" }).fill(`fixture-${value || "unknown"}`);
    await dialog.getByRole("button", { name: "检索", exact: true }).click();
    if (value === "complete") {
      await expect(dialog.getByText("没有符合条件的结果。", { exact: true })).toBeVisible();
    } else {
      await expect(dialog.getByText(/不能据此断言没有文件/)).toBeVisible();
      await expect(dialog.getByText("当前已覆盖范围未返回结果，请检查覆盖状态后重试。", { exact: true })).toBeVisible();
      await expect(dialog.getByText("没有符合条件的结果。", { exact: true })).toHaveCount(0);
    }
  }
  expect(queries).toBe(3);
});
