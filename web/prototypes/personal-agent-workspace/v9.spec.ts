import { test, expect } from "@playwright/test";
import type { Page } from "@playwright/test";
import { folder7, nav7, choose7 } from "./v7-helpers";
const errors = new WeakMap<Page, string[]>();
test.beforeEach(async ({ page }) => {
  const list: string[] = [];
  errors.set(page, list);
  page.on("pageerror", (e) => list.push(e.message));
  await page.goto("/opendesign.html");
});
test.afterEach(async ({ page }) => expect(errors.get(page)).toEqual([]));
async function resources(page: Page) {
  if (
    await page
      .getByRole("button", { name: "打开存储目录", exact: true })
      .isVisible()
  )
    await page
      .getByRole("button", { name: "打开存储目录", exact: true })
      .click();
  return page.getByRole("region", { name: "文件资源栏", exact: true });
}
async function search(page: Page, value: string) {
  const field = page.getByRole("textbox", {
    name: "搜索当前目录",
    exact: true,
  });
  if (!(await field.isVisible()))
    await page
      .getByRole("button", { name: "搜索当前目录", exact: true })
      .click();
  await field.fill(value);
  return field;
}
test("v9 compact rows navigation history and local search return", async ({
  page,
}, info) => {
  await expect(
    page.getByRole("button", { name: "上一级", exact: true }),
  ).toBeDisabled();
  await folder7(page);
  const row = page.locator('[data-file-id="f1"]');
  expect((await row.boundingBox())!.height).toBeLessThanOrEqual(
    info.project.name === "desktop" ? 37 : 60,
  );
  const before = await row.boundingBox();
  await row.getByRole("checkbox").check();
  expect(
    Math.abs((await row.boundingBox())!.y - before!.y),
  ).toBeLessThanOrEqual(1);
  await page.getByRole("button", { name: "取消多选", exact: true }).click();
  const input = await search(page, "山野");
  await input.press("Enter");
  await expect(
    page.getByLabel("当前目录检索结果", { exact: true }),
  ).toContainText("创作简报");
  await expect(
    page.getByRole("region", { name: "文件检索", exact: true }),
  ).toHaveCount(0);
  await page
    .locator('[data-file-id="f1"]')
    .getByRole("button", { name: /^预览 / })
    .click();
  await page.getByRole("button", { name: "退出预览", exact: true }).click();
  await expect(
    page.getByLabel("当前目录检索结果", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "返回目录", exact: true }).click();
  await expect(page.locator('[data-file-id="f2"]')).toBeVisible();
  await page.getByRole("button", { name: "后退", exact: true }).click();
  await expect(
    page.getByLabel("当前目录检索结果", { exact: true }),
  ).toBeVisible();
});
test("v9 filter edits cancel and apply atomically with visible labels", async ({
  page,
}) => {
  await folder7(page);
  await page.getByRole("button", { name: "筛选条件", exact: true }).click();
  await expect(page.getByText("文件类型", { exact: true })).toBeVisible();
  await choose7(page, "文件类型", "图片");
  await page.getByRole("button", { name: "取消", exact: true }).click();
  await expect(page.locator('[data-file-id="f1"]')).toBeVisible();
  await page.getByRole("button", { name: "筛选条件", exact: true }).click();
  await choose7(page, "文件类型", "图片");
  await page.getByRole("button", { name: "应用筛选", exact: true }).click();
  await expect(page.locator('[data-file-id="f1"]')).toHaveCount(0);
  await expect(page.locator('[data-file-id="f2"]')).toBeVisible();
  await page.getByRole("button", { name: "后退", exact: true }).click();
  await expect(page.locator('[data-file-id="f1"]')).toBeVisible();
});
test("v9 tags properties and directory tabs share the current file pane", async ({
  page,
}) => {
  await nav7(page, "知识库");
  await nav7(page, "我的云盘");
  const tree = await resources(page);
  await tree.getByRole("tab", { name: "标签", exact: true }).click();
  await tree
    .getByRole("tree", { name: "标签树" })
    .getByRole("button", { name: /^项目\s/ })
    .click();
  await expect(
    page.getByLabel("当前目录检索结果", { exact: true }),
  ).toBeVisible();
  const again = await resources(page);
  await again.getByRole("tab", { name: "属性", exact: true }).click();
  await again
    .getByRole("tree", { name: "属性树" })
    .getByRole("button", { name: /^状态/ })
    .click();
  await expect(again.getByRole("treeitem", { name: /^草稿/ })).toBeVisible();
  await again.getByRole("treeitem", { name: /^草稿/ }).click();
  await expect(
    page.getByLabel("当前目录检索结果", { exact: true }),
  ).toContainText("知识库");
});
test("v9 anchored space menu retains floating navigation and restores positions", async ({
  page,
}, info) => {
  if (info.project.name === "desktop")
    await page.setViewportSize({ width: 855, height: 953 });
  await folder7(page);
  await page.getByRole("button", { name: "显示主导航", exact: true }).click();
  await page.getByRole("button", { name: "切换空间", exact: true }).click();
  const menu = page.getByRole("menu", { name: "切换空间", exact: true });
  await expect(menu).toBeVisible();
  const box = (await menu.boundingBox())!;
  expect(box.width).toBeLessThanOrEqual(281);
  expect(box.x).toBeLessThan(225);
  if (info.project.name === "desktop") {
    await menu.hover();
    await page.waitForTimeout(300);
    await expect(page.locator("[data-floating-navigation]")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(menu).toHaveCount(0);
    await expect(page.locator("[data-floating-navigation]")).toBeVisible();
    await page.getByRole("button", { name: "切换空间", exact: true }).click();
  }
  await page.getByRole("menuitem", { name: /山野 · 团队空间/ }).click();
  await expect(
    page.locator("[data-directory-id]").filter({ hasText: "共同创作" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "显示主导航", exact: true }).click();
  await page.getByRole("button", { name: "切换空间", exact: true }).click();
  await page.getByRole("menuitem", { name: /个人空间/ }).click();
  await expect(page.locator('[data-file-id="f1"]')).toBeVisible();
});
test("v9 responsive toolbar tree and list have no horizontal page overflow", async ({
  page,
}, info) => {
  test.skip(
    info.project.name !== "desktop",
    "Widths are covered once, touch paths run separately",
  );
  for (const width of [320, 390, 768, 855, 1024, 1305, 1440, 1920]) {
    await page.setViewportSize({ width, height: 953 });
    await expect(page.locator("[data-directory-toolbar]")).toHaveCSS(
      "height",
      "44px",
    );
    await folder7(page);
    const geometry = await page.evaluate(() => ({
      width: innerWidth,
      scroll: document.documentElement.scrollWidth,
      toolbar: document
        .querySelector("[data-directory-toolbar]")!
        .getBoundingClientRect().right,
      body: document
        .querySelector("[data-cloud-explorer]")!
        .getBoundingClientRect().right,
    }));
    expect(geometry.scroll, `${width}`).toBeLessThanOrEqual(width);
    expect(geometry.toolbar, `${width}`).toBeLessThanOrEqual(width + 1);
    expect(geometry.body).toBeLessThanOrEqual(width + 1);
    await page.getByRole("button", { name: "上一级", exact: true }).click();
  }
});
test("v9 IME debounce and history shortcuts stay scoped to the work area", async ({
  page,
}, info) => {
  await folder7(page);
  let input = await search(page, "");
  await page.clock.install();
  await page.clock.pauseAt(new Date());
  await input.dispatchEvent("compositionstart");
  await input.fill("创作");
  await page.clock.runFor(500);
  await expect(page.locator('[data-file-id="f1"]')).toHaveCount(0);
  await input.dispatchEvent("compositionend");
  await page.clock.runFor(249);
  await expect(page.locator('[data-file-id="f1"]')).toHaveCount(0);
  await page.clock.runFor(1);
  await expect(page.locator('[data-file-id="f1"]')).toBeVisible();
  await input.fill("不存在");
  await page.clock.runFor(100);
  await input.fill("视觉参考");
  await page.clock.runFor(250);
  await expect(page.locator('[data-file-id="f1"]')).toHaveCount(0);
  await expect(page.locator('[data-file-id="f2"]')).toBeVisible();
  await input.press("Alt+ArrowUp");
  await expect(page.locator('[data-file-id="f2"]')).toBeVisible();
  await input.fill("");
  await page.clock.runFor(250);
  await page.locator("[data-cloud-explorer]").focus();
  await page.keyboard.press("Alt+ArrowUp");
  await expect(page.locator('[data-file-id="f1"]')).toHaveCount(0);
  await page.keyboard.press("Alt+ArrowLeft");
  await expect(page.locator('[data-file-id="f1"]')).toBeVisible();
});
test("v9 search header sorting persists in a saved view", async ({
  page,
}, info) => {
  test.skip(
    info.project.name !== "desktop",
    "Table header sorting; mobile uses compact results",
  );
  await nav7(page, "搜索");
  await page.getByRole("button", { name: "执行搜索", exact: true }).click();
  const results = page.getByLabel("检索结果", { exact: true });
  await expect(results.locator("[data-file-id]").first()).toBeVisible();
  await results.getByRole("button", { name: "名称", exact: true }).click();
  const names = await results
    .locator("[data-file-id] [data-entry-name]")
    .allTextContents();
  expect(names).toEqual([...names].sort((a, b) => a.localeCompare(b, "zh-CN")));
  await page.getByRole("button", { name: "保存搜索", exact: true }).click();
  await page.getByRole("textbox", { name: "保存搜索名称" }).fill("名称排序");
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "保存视图", exact: true })
    .click();
  await results.getByRole("button", { name: "名称 ↑", exact: true }).click();
  await page.getByRole("button", { name: "保存视图", exact: true }).click();
  await page.getByRole("button", { name: "名称排序", exact: true }).click();
  await expect(
    results.getByRole("button", { name: "名称 ↑", exact: true }),
  ).toBeVisible();
});
test("v9 async query history restores scroll after results return", async ({
  page,
}, info) => {
  test.skip(info.project.name !== "desktop", "Shared restoration state");
  await page.setViewportSize({ width: 1440, height: 440 });
  await page.getByRole("button", { name: "检索当前目录", exact: true }).click();
  await expect(page.locator("[data-find-entry]")).not.toHaveCount(0);
  const scroll = page.locator("[data-directory-scroll]");
  await scroll.evaluate((el) => {
    el.scrollTop = 130;
  });
  await expect.poll(() => scroll.evaluate((el) => el.scrollTop)).toBe(130);
  await page.getByRole("button", { name: "返回目录", exact: true }).click();
  await page.getByRole("button", { name: "后退", exact: true }).click();
  await expect(
    page.getByLabel("当前目录检索结果", { exact: true }),
  ).toBeVisible();
  await expect.poll(() => scroll.evaluate((el) => el.scrollTop)).toBe(130);
});
