import { test, expect } from "@playwright/test";

test("search uses the server cursor and resets paging when criteria change", async ({ page }) => {
  const cursors: unknown[] = [];
  let expireNext = false;
  await page.route("**/api/v1/operations/lookup", route => route.fulfill({ json: { data: { found: false } } }));
  await page.route("**/api/v1/file-queries", route => {
    const body = route.request().postDataJSON();
    cursors.push(body.cursor);
    if (expireNext && body.cursor) {
      expireNext = false;
      return route.fulfill({ status: 410, json: { error: { code: "cursor_expired" } } });
    }
    const next = body.cursor === "fixture-opaque-next";
    return route.fulfill({ json: { data: { mode: "exact", items: [{ file_id: next ? "second-file" : "first-file", version_id: "fixed-version" }], next_cursor: next ? null : "fixture-opaque-next", coverage: "complete" } } });
  });
  await page.goto("/#/files");
  await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await page.getByRole("button", { name: "搜索工作区", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "搜索工作区" });
  const input = dialog.getByRole("searchbox", { name: "检索条件" });
  await input.fill("fixture-query");
  await dialog.getByRole("button", { name: "检索", exact: true }).click();
  await expect(dialog.getByText("first-file", { exact: true })).toBeVisible();
  await dialog.getByRole("button", { name: "下一页", exact: true }).click();
  await expect(dialog.getByText("second-file", { exact: true })).toBeVisible();
  await expect(dialog.getByText("first-file", { exact: true })).toHaveCount(0);
  await expect(dialog.getByRole("button", { name: "下一页", exact: true })).toHaveCount(0);
  await dialog.getByRole("button", { name: "上一页", exact: true }).click();
  await expect(dialog.getByText("first-file", { exact: true })).toBeVisible();
  await expect(dialog.getByRole("button", { name: "上一页", exact: true })).toHaveCount(0);
  await input.fill("new-query");
  await expect(dialog.getByText("second-file", { exact: true })).toHaveCount(0);
  await dialog.getByRole("button", { name: "检索", exact: true }).click();
  await expect(dialog.getByText("first-file", { exact: true })).toBeVisible();
  expect(cursors).toEqual([undefined, "fixture-opaque-next", undefined, undefined]);
  expireNext = true;
  await dialog.getByRole("button", { name: "下一页", exact: true }).click();
  await expect(dialog.getByRole("alert")).toContainText("搜索分页已失效");
  await expect(input).toHaveValue("new-query");
  await expect(dialog.getByText("first-file", { exact: true })).toHaveCount(0);
  await expect(dialog.getByRole("navigation", { name: "搜索结果分页" })).toHaveCount(0);
  expect(cursors).toHaveLength(5);
  await dialog.getByRole("button", { name: "检索", exact: true }).click();
  await expect(dialog.getByText("first-file", { exact: true })).toBeVisible();
  expect(cursors).toEqual([undefined, "fixture-opaque-next", undefined, undefined, "fixture-opaque-next", undefined]);
});
