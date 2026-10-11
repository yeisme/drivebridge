import { nav7, operation7 } from "./v7-helpers";
import { test, expect } from "@playwright/test";
import type { Page } from "@playwright/test";
const errors = new WeakMap<Page, string[]>();
test.beforeEach(async ({ page }) => {
  const e: string[] = [];
  errors.set(page, e);
  page.on("pageerror", (x) => e.push(x.message));
  page.on("console", (message) => {
    if (message.type() === "error") e.push(message.text());
  });
  await page.goto("/opendesign.html");
});
test.afterEach(async ({ page }) => expect(errors.get(page)).toEqual([]));
async function library(page: Page, name = "知识库") {
  await nav7(page, name);
}
async function edit(page: Page) {
  await library(page);
  await page
    .locator('[data-knowledge-file="k-guide"]')
    .getByRole("button", { name: "继续编辑" })
    .click();
  await expect(
    page.getByRole("textbox", { name: "Markdown 编辑器" }),
  ).toBeVisible();
}
async function source(page: Page) {
  await page.getByRole("button", { name: "源码", exact: true }).click();
}
async function details(page: Page) {
  if (
    !(await page.getByRole("button", { name: "关联", exact: true }).isVisible())
  )
    await operation7(page, "显示文件详情");
  await page.getByRole("button", { name: "关联", exact: true }).click();
}

test("v6 markdown live source reading preserve content and undo across saving and panels", async ({
  page,
}) => {
  await edit(page);
  await source(page);
  const input = page.getByRole("textbox", { name: "Markdown 编辑器" });
  await input.fill("# 实时编辑测试\n\n**保持内容**\n\n[[拍摄研究]]");
  await expect(page.locator("[data-save-status]")).toHaveText(
    "已保存到本轮演示",
  );
  await page.getByRole("button", { name: "阅读", exact: true }).click();
  await expect(
    page
      .locator("[data-markdown-reading]")
      .getByRole("heading", { name: "实时编辑测试" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "实时编辑", exact: true }).click();
  await expect(input).toContainText("实时编辑测试");
  await input.focus();
  await page.keyboard.press("Control+End");
  await page.keyboard.insertText("\n新内容");
  await page.keyboard.press("Control+z");
  await expect(input).not.toContainText("新内容");
  await operation7(page, "显示文件详情");
  await page.getByRole("button", { name: "收起详情", exact: true }).click();
  await expect(input).toContainText("实时编辑测试");
  await expect(page.locator("[data-markdown-editor]")).toHaveCount(1);
});
test("v6 wikilinks navigate immediately within separate scope and back restores editor", async ({
  page,
}) => {
  await edit(page);
  await page.getByRole("button", { name: "阅读", exact: true }).click();
  await page
    .locator("[data-markdown-reading]")
    .getByRole("button", { name: "光线与拍摄研究", exact: true })
    .click();
  await expect(
    page
      .locator("[data-markdown-reading]")
      .getByRole("heading", { name: "拍摄研究", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "退出预览", exact: true }).click();
  await expect(
    page
      .locator("[data-markdown-reading]")
      .getByRole("heading", { name: "山野知识地图", exact: true }),
  ).toBeVisible();
  await details(page);
  await expect(page.getByRole("heading", { name: /反链/ })).toBeVisible();
});
test("v6 IME defers save and autocomplete links exposes candidate source", async ({
  page,
}) => {
  await edit(page);
  await source(page);
  const input = page.getByRole("textbox", { name: "Markdown 编辑器" });
  await input.dispatchEvent("compositionstart", { data: "" });
  await input.fill("# 输入法草稿");
  await page.waitForTimeout(1100);
  await expect(page.locator("[data-save-status]")).toHaveText("未保存");
  await input.dispatchEvent("compositionend", { data: "草稿" });
  await expect(page.locator("[data-save-status]")).toHaveText(
    "已保存到本轮演示",
  );
  await input.focus();
  await page.keyboard.press("Control+End");
  await page.keyboard.insertText("\n[[");
  await expect(page.getByRole("listbox")).toBeVisible();
  await expect(page.getByRole("listbox")).toContainText("拍摄研究");
  await page.keyboard.press("Escape");
});
test("v6 attachment interrupted upload recovers same task before inserting reference", async ({
  page,
}) => {
  await edit(page);
  await page.getByRole("button", { name: "格式与引用", exact: true }).click();
  await page.getByRole("button", { name: "添加文档附件", exact: true }).click();
  await page.getByRole("button", { name: "演示上传中断", exact: true }).click();
  await expect(page.getByText(/现场照片.png · 上传未完成/)).toBeVisible();
  await page.getByRole("button", { name: "恢复", exact: true }).click();
  await source(page);
  await expect(
    page.getByRole("textbox", { name: "Markdown 编辑器" }),
  ).toContainText("![[现场照片.png]]");
  await expect(page.locator("[data-save-status]")).toHaveText(
    "已保存到本轮演示",
  );
});
test("v6 settings search locates editor option and new documents enter live editing", async ({
  page,
}) => {
  await library(page);
  await page.getByRole("button", { name: "新建文档", exact: true }).click();
  await page.getByRole("textbox", { name: "新文档名称" }).fill("新的记录");
  await page.getByRole("button", { name: "创建并编辑", exact: true }).click();
  await expect(
    page.getByRole("textbox", { name: "Markdown 编辑器" }),
  ).toContainText("新的记录");
  await page.getByRole("button", { name: "退出预览", exact: true }).click();
  if (await page.getByRole("navigation", { name: "底部导航" }).isVisible())
    await page
      .getByRole("navigation", { name: "底部导航" })
      .getByRole("button", { name: "我的", exact: true })
      .click();
  else {
    if (
      !(await page
        .getByRole("button", { name: "个人设置", exact: true })
        .isVisible())
    )
      await page
        .getByRole("button", { name: "显示主导航", exact: true })
        .click();
    await page.getByRole("button", { name: "个人设置", exact: true }).click();
  }
  await page
    .getByRole("textbox", { name: "搜索设置", exact: true })
    .fill("自动保存");
  await page
    .getByRole("region", { name: "设置搜索结果" })
    .getByRole("button", { name: /Markdown 自动保存/ })
    .click();
  await expect(
    page.getByRole("checkbox", { name: "Markdown 自动保存", exact: true }),
  ).toBeVisible();
});
test("v6 responsive document workbench has no horizontal page overflow", async ({
  page,
}, info) => {
  test.skip(info.project.name === "mobile");
  await edit(page);
  for (const width of [320, 390, 768, 1024, 1440, 1920]) {
    await page.setViewportSize({ width, height: 1000 });
    await page.waitForTimeout(100);
    const data = await page.evaluate(() => ({
      body: document.documentElement.scrollWidth,
      viewport: innerWidth,
      editor: document
        .querySelector("[data-markdown-editor]")
        ?.getBoundingClientRect().width,
    }));
    expect(data.body).toBeLessThanOrEqual(data.viewport + 1);
    expect(data.editor).toBeGreaterThan(200);
  }
});

test("v6 conflict keeps editable draft and allows difference review without overwriting", async ({
  page,
}) => {
  await edit(page);
  await page.getByRole("button", { name: "演示与审阅", exact: true }).click();
  await page
    .getByRole("button", { name: "暂停文档自动保存", exact: true })
    .click();
  await source(page);
  await page
    .getByRole("textbox", { name: "Markdown 编辑器" })
    .fill("# 我的并发草稿");
  await page.getByRole("button", { name: "演示与审阅", exact: true }).click();
  await page
    .getByRole("button", { name: "模拟文档并发修改", exact: true })
    .click();
  await expect(
    page.getByRole("textbox", { name: "Markdown 编辑器" }),
  ).toContainText("我的并发草稿");
  await expect(page.locator("[data-save-status]")).toHaveText(
    "版本冲突 · 草稿保留",
  );
  await page.getByRole("button", { name: "查看差异", exact: true }).click();
  await expect(page.getByRole("dialog")).toContainText("另一设备补充");
  await expect(page.getByRole("dialog")).toContainText("我的并发草稿");
  await page.keyboard.press("Escape");
  await expect(
    page.getByRole("textbox", { name: "Markdown 编辑器" }),
  ).toContainText("我的并发草稿");
});
test("v6 offline draft remains editable and saves after reconnect", async ({
  page,
}) => {
  await edit(page);
  await source(page);
  await page.getByRole("button", { name: "演示与审阅", exact: true }).click();
  await page.getByRole("combobox", { name: "演示状态", exact: true }).click();
  await page.getByRole("option", { name: "离线", exact: true }).click();
  await expect(
    page.getByRole("textbox", { name: "Markdown 编辑器" }),
  ).toBeVisible();
  await page
    .getByRole("textbox", { name: "Markdown 编辑器" })
    .fill("# 离线内容");
  await expect(page.locator("[data-save-status]")).toHaveText(
    "等待网络 · 草稿保留",
  );
  await page.getByRole("button", { name: "恢复网络", exact: true }).click();
  await expect(page.locator("[data-save-status]")).toHaveText(
    "已保存到本轮演示",
  );
});
test("v6 live blocks render tables and attachments, and links use the existing hover timer", async ({
  page,
}, info) => {
  await edit(page);
  await expect(page.locator("[data-live-block] table")).toBeVisible();
  await expect(
    page.locator("[data-live-block]").getByRole("button", { name: /秋日山行/ }),
  ).toBeVisible();
  if (info.project.name === "desktop") {
    const trigger = page
      .locator("[data-live-block]")
      .getByRole("button", { name: "光线与拍摄研究", exact: true });
    await trigger.hover();
    await expect(page.locator("[data-hover-preview]")).toBeVisible();
    await trigger.click();
    await expect(
      page
        .locator("[data-markdown-reading]")
        .getByRole("heading", { name: "拍摄研究", exact: true }),
    ).toBeVisible();
  }
});
test("v6 skill metadata invalidates check and preserves unknown fields", async ({
  page,
}) => {
  await library(page, "Agent 资源");
  await page.locator("[data-resource-list] [data-entry-name]").first().click();
  await page.getByRole("button", { name: "检查资源", exact: true }).click();
  await page.getByRole("button", { name: "修改字段", exact: true }).click();
  await page
    .getByRole("textbox", { name: "所需工具", exact: true })
    .fill("files.read, missing.tool");
  await page.getByRole("button", { name: "保存字段", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "复制已检查版本引用", exact: true }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "检查资源", exact: true }).click();
  await expect(
    page.getByText("缺少依赖：missing.tool，当前不可用于任务。"),
  ).toBeVisible();
  await expect(page.getByRole("dialog")).toContainText(
    "license、compatibility",
  );
});

test("v6 live task checkbox edits source and search controls support touch", async ({
  page,
}) => {
  await edit(page);
  const task = page
    .locator("[data-live-block]")
    .getByRole("checkbox", { name: "整理选题与参考资料", exact: true });
  await expect(task).toBeEnabled();
  await task.focus();
  await page.keyboard.press("Space");
  await expect(task).toBeChecked();
  await expect(task).toBeFocused();
  await expect(page.locator("[data-save-status]")).toHaveText(
    "已保存到本轮演示",
  );
  await expect(task).toBeFocused();
  const touch = await task.evaluate(
    (el) => el.closest("label")!.getBoundingClientRect().height,
  );
  expect(touch).toBeGreaterThanOrEqual(44);
  await source(page);
  await expect(
    page.getByRole("textbox", { name: "Markdown 编辑器" }),
  ).toContainText("- [x] 整理选题与参考资料");
  if (
    await page
      .getByRole("button", { name: "查找与替换", exact: true })
      .isVisible()
  )
    await page.getByRole("button", { name: "查找与替换", exact: true }).click();
  else {
    await page.getByRole("button", { name: "格式与引用" }).click();
    await page.getByRole("button", { name: "查找与替换正文" }).click();
  }
  const sizes = await page
    .locator(".cm-search button")
    .evaluateAll((items) =>
      items
        .filter((el) => el.getClientRects().length)
        .map((el) => el.getBoundingClientRect().height),
    );
  expect(sizes.length).toBeGreaterThan(0);
  expect(Math.min(...sizes)).toBeGreaterThanOrEqual(44);
  const width = await page.evaluate(() => ({
    page: document.documentElement.scrollWidth,
    viewport: innerWidth,
  }));
  expect(width.page).toBeLessThanOrEqual(width.viewport + 1);
});
