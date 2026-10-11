import { test, expect } from "@playwright/test";
import type { Page } from "@playwright/test";
import { nav7, folder7, choose7, operation7 } from "./v7-helpers";
const errors = new WeakMap<Page, string[]>();
test.beforeEach(async ({ page }) => {
  const list: string[] = [];
  errors.set(page, list);
  page.on("pageerror", (e) => list.push(e.message));
  page.on("console", (m) => {
    if (m.type() === "error") list.push(m.text());
  });
  await page.goto("/opendesign.html");
});
test.afterEach(async ({ page }) => expect(errors.get(page)).toEqual([]));
async function favoritePage(page: Page) {
  if (
    await page
      .getByRole("button", { name: "打开存储目录", exact: true })
      .isVisible()
  )
    await page
      .getByRole("button", { name: "打开存储目录", exact: true })
      .click();
  await page
    .getByRole("button", { name: /本盘收藏.*DriveBridge/ })
    .first()
    .click();
}
test("v8 directory bookmarks groups and return do not move files", async ({
  page,
}) => {
  if (
    await page
      .getByRole("button", { name: "收藏 当前目录", exact: true })
      .isVisible()
  )
    await page
      .getByRole("button", { name: "收藏 当前目录", exact: true })
      .click();
  else {
    await page
      .getByRole("button", { name: "目录更多操作", exact: true })
      .click();
    await page
      .getByRole("menuitem", { name: "收藏当前目录", exact: true })
      .click();
  }
  await favoritePage(page);
  await expect(page.getByRole("region", { name: "本盘收藏" })).toBeVisible();
  await page.getByRole("button", { name: "新建收藏夹" }).click();
  await page.getByRole("textbox", { name: "收藏夹名称" }).fill("每周资料");
  await page.getByRole("button", { name: "保存收藏夹", exact: true }).click();
  const row = page
    .locator("[data-bookmark]")
    .filter({
      has: page.getByRole("button", {
        name: "管理收藏 DriveBridge 云盘",
        exact: true,
      }),
    })
    .first();
  await row.getByRole("button", { name: /管理收藏/ }).click();
  await page.getByRole("checkbox", { name: /每周资料/ }).check();
  await page.getByRole("button", { name: "置顶快捷入口", exact: true }).click();
  await page.keyboard.press("Escape");
  await choose7(page, "收藏夹", "每周资料");
  await expect(page.locator("[data-bookmark]")).toHaveCount(1);
  await page
    .locator("[data-bookmark]")
    .getByRole("button", { name: "DriveBridge 云盘", exact: true })
    .click();
  await expect(page.locator("[data-cloud-explorer]")).toBeVisible();
  await expect(
    page.locator("[data-directory-id]").filter({ hasText: "山野计划" }),
  ).toBeVisible();
});
test("v8 selection and favorite updates leave files stationary", async ({
  page,
}) => {
  await folder7(page);
  const file = page.locator('[data-file-id="f1"]');
  const before = await file.boundingBox();
  await file.getByRole("checkbox").click();
  expect(
    Math.abs((await file.boundingBox())!.y - before!.y),
  ).toBeLessThanOrEqual(1);
  await page.getByRole("button", { name: "更多批量操作", exact: true }).click();
  await page.getByRole("menuitem", { name: "收藏", exact: true }).click();
  await page
    .getByRole("button", { name: "加入未分组收藏", exact: true })
    .click();
  await page.getByRole("button", { name: "关闭对话框", exact: true }).click();
  expect(
    Math.abs((await file.boundingBox())!.y - before!.y),
  ).toBeLessThanOrEqual(1);
  await expect(file.getByRole("button", { name: /取消收藏/ })).toBeVisible();
});
test("v8 markdown frontmatter form source undo and reading", async ({
  page,
}) => {
  await nav7(page, "知识库");
  await page
    .locator('[data-knowledge-file="k-guide"]')
    .getByRole("button", { name: "继续编辑" })
    .click();
  await page.getByRole("button", { name: "源码", exact: true }).click();
  const editor = page.getByRole("textbox", { name: "Markdown 编辑器" });
  await expect(editor).toContainText("priority: 0");
  await operation7(page, "文件属性");
  const props = page
    .locator("[data-file-properties]")
    .filter({ visible: true });
  await props
    .getByRole("textbox", { name: "项目", exact: true })
    .fill("城市观察");
  await props.getByRole("textbox", { name: "项目", exact: true }).press("Tab");
  await expect(page.locator("[data-save-status]")).toHaveText(
    "已保存到本轮演示",
  );
  await page.getByRole("button", { name: "收起详情", exact: true }).click();
  await expect(editor).toContainText("project: 城市观察");
  await expect(editor).toContainText("custom: 保留这个未知字段 # 示例注释");
  await editor.focus();
  await page.keyboard.press("Control+z");
  await expect(editor).toContainText("project: 山野");
  await page.getByRole("button", { name: "阅读", exact: true }).click();
  await expect(page.locator("[data-markdown-reading]")).not.toContainText(
    "priority:",
  );
});
test("v8 query directories tags and property table save a scoped view", async ({
  page,
}) => {
  await nav7(page, "知识库");
  await nav7(page, "搜索");
  await page
    .getByRole("textbox", { name: "搜索内容", exact: true })
    .fill("拍摄研究");
  await expect(page.locator("[data-find-entry]")).toHaveCount(1);
  await page.getByRole("textbox", { name: "搜索内容", exact: true }).fill("");
  await page
    .getByRole("button", { name: "检索范围与筛选", exact: true })
    .click();
  await page
    .getByRole("textbox", { name: "文档层级标签", exact: true })
    .fill("项目/山野");
  await page.getByRole("button", { name: "应用并检索", exact: true }).click();
  await expect(page.locator("[data-find-entry]")).toHaveCount(2);
  await page.getByRole("button", { name: "属性表格", exact: true }).click();
  await expect(
    page.getByRole("region", { name: "属性表格", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "保存搜索", exact: true }).click();
  await page.getByRole("textbox", { name: "保存搜索名称" }).fill("山野属性");
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "保存视图", exact: true })
    .click();
});
test("v8 paged large fixture only mounts current rows", async ({ page }) => {
  await page.getByRole("button", { name: "演示与审阅", exact: true }).click();
  await page
    .getByRole("button", { name: "打开万级目录演示", exact: true })
    .click();
  await expect(page.locator("[data-large-entry]")).toHaveCount(50);
  await page.getByRole("button", { name: "规模下一页", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "项目 0051", exact: true }),
  ).toBeVisible();
  await page.getByRole("textbox", { name: "规模场景检索" }).fill("00999");
  await expect(page.locator("[data-large-entry]")).toHaveCount(1);
  await page
    .getByRole("button", { name: "研究资料 00999.pdf", exact: true })
    .click();
  await expect(
    page.getByRole("region", { name: "文件预览工作台" }),
  ).toBeVisible();
});
test("v8 compact responsive pages retain no horizontal page overflow", async ({
  page,
}) => {
  await nav7(page, "知识库");
  for (const width of [320, 390, 768, 1024, 1440, 1920]) {
    await page.setViewportSize({ width, height: 900 });
    await page.getByRole("button", { name: "属性表格", exact: true }).click();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth + 1,
      ),
    ).toBe(true);
    await page.getByRole("button", { name: "紧凑列表", exact: true }).click();
  }
});

test("v8 quick find defers IME and cancels stale scope results", async ({
  page,
}) => {
  await nav7(page, "知识库");
  await page.keyboard.press("Control+k");
  const input = page.getByRole("textbox", {
    name: "命令与文件搜索",
    exact: true,
  });
  await input.dispatchEvent("compositionstart", { data: "" });
  await input.fill("山野知识");
  await page.waitForTimeout(300);
  await expect(
    page.getByRole("listbox", { name: "快速查找结果" }).getByRole("option"),
  ).toHaveCount(0);
  await input.dispatchEvent("compositionend", { data: "山野知识" });
  await expect(
    page.getByRole("option", { name: /山野知识地图/ }),
  ).toBeVisible();
  await input.fill("拍摄研究");
  await expect(page.getByRole("option", { name: /拍摄研究/ })).toBeVisible();
  await input.press("Enter");
  await expect(
    page.getByRole("region", { name: "文件预览工作台" }),
  ).toBeVisible();
  await expect(page.locator("[data-preview-toolbar]")).toContainText(
    "拍摄研究",
  );
});
test("v8 batch properties freezes versions and resumes only failed work", async ({
  page,
}) => {
  await nav7(page, "知识库");
  await page.getByRole("button", { name: "属性表格", exact: true }).click();
  await page
    .getByRole("checkbox", { name: "选择 山野知识地图.md", exact: true })
    .check();
  await page
    .getByRole("checkbox", { name: "选择 拍摄研究.md", exact: true })
    .check();
  await page.getByRole("button", { name: "更多批量操作", exact: true }).click();
  await page.getByRole("menuitem", { name: "属性", exact: true }).click();
  await page.getByRole("textbox", { name: "状态", exact: true }).fill("已归档");
  await page.getByRole("textbox", { name: "状态", exact: true }).press("Tab");
  await page.getByRole("button", { name: "预览属性差异", exact: true }).click();
  await page.getByRole("button", { name: "演示部分失败", exact: true }).click();
  await expect(
    page.getByRole("region", { name: "属性修改计划" }),
  ).toContainText("1/2 完成");
  await page
    .getByRole("button", { name: "恢复原操作未完成项", exact: true })
    .click();
  await expect(
    page.getByRole("region", { name: "属性修改计划" }),
  ).toContainText("2/2 完成");
});

test("v8 changing grouped property keeps the row until an explicit regroup", async ({
  page,
}) => {
  await nav7(page, "知识库");
  await page.getByRole("button", { name: "属性表格", exact: true }).click();
  await page.getByRole("button", { name: "列与分组", exact: true }).click();
  await choose7(page, "按属性分组", "项目");
  await page.getByRole("button", { name: "完成", exact: true }).click();
  const row = page.locator('[data-property-row="k-guide"]');
  await row.getByRole("button", { name: "山野", exact: true }).click();
  const field = row.getByRole("textbox", { name: "项目", exact: true });
  await field.fill("城市观察");
  await field.press("Tab");
  await expect(page.getByRole("heading", { name: /城市观察/ })).toHaveCount(0);
  await expect(
    row.getByRole("textbox", { name: "项目", exact: true }),
  ).toHaveValue("城市观察");
  await page.getByRole("button", { name: "刷新分组", exact: true }).click();
  await expect(page.getByRole("heading", { name: /城市观察/ })).toBeVisible();
});
