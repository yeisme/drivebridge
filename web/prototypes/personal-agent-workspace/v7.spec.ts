import { record7, closeRecords7 } from "./v7-helpers";
import { test, expect } from "@playwright/test";
import type { Page } from "@playwright/test";
import {
  nav7,
  folder7,
  file7,
  choose7,
  details7,
  operation7,
} from "./v7-helpers";
const errors = new WeakMap<Page, string[]>();
test.beforeEach(async ({ page }) => {
  const list: string[] = [];
  errors.set(page, list);
  page.on("pageerror", (e) => list.push(e.message));
  page.on("console", (m) => {
    if (m.type() === "error") list.push(m.text());
  });
  await page.goto("/opendesign.html");
  await expect(page.locator("[data-cloud-explorer]")).toBeVisible();
});
test.afterEach(async ({ page }) => expect(errors.get(page)).toEqual([]));
test("v7 removes builtin Agent and duplicates, compact preview expands content", async ({
  page,
}) => {
  await expect(
    page.locator("[data-global-agent], [data-demo-bar]"),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "打开 Agent 侧栏" }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("navigation", { name: "文件资源视图" }),
  ).toHaveCount(0);
  await file7(page);
  const sizes = await page.evaluate(() => ({
    top: document.querySelector("header")!.getBoundingClientRect().height,
    toolbar: document
      .querySelector("[data-preview-toolbar]")!
      .getBoundingClientRect().height,
    footer:
      document
        .querySelector('[aria-label="预览播放与缩放"]')
        ?.getBoundingClientRect().height || 0,
  }));
  expect(sizes).toEqual({ top: 48, toolbar: 44, footer: 0 });
  expect(
    44 + 64 + 64 + 48 + 56 + 64 - (sizes.top + sizes.toolbar + sizes.footer),
  ).toBeGreaterThanOrEqual(120);
  await operation7(page, "链接与引用");
  await expect(
    page.getByRole("button", { name: "复制定位链接", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "复制 Agent 引用", exact: true }),
  ).toBeVisible();
});
test("v7 keeps provider hierarchy and empty folders separate", async ({
  page,
}) => {
  await expect(page.locator("[data-file-id]")).toHaveCount(0);
  await folder7(page);
  await expect(page.locator('[data-file-id="f1"]')).toBeVisible();
  await page.getByRole("button", { name: "上一级", exact: true }).click();
  await expect(page.locator('[data-file-id="f1"]')).toHaveCount(0);
  if (await page.getByRole("button", { name: "打开存储目录" }).isVisible())
    await page.getByRole("button", { name: "打开存储目录" }).click();
  const tree = page.getByRole("navigation", { name: "存储目录树" }).last();
  await tree
    .getByRole("button", { name: "OneDrive · 个人", exact: true })
    .click();
  await expect(page.locator('[data-file-id="od-personal-file"]')).toBeVisible();
  await expect(page.locator('[data-file-id="od-work-file"]')).toHaveCount(0);
  await folder7(page, "项目");
  await folder7(page, "待归档");
  await expect(
    page.getByText("当前范围没有条目", { exact: true }),
  ).toBeVisible();
});
test("v7 batch selection remains stationary in list and grid, menus restore focus", async ({
  page,
}) => {
  await folder7(page);
  for (const layout of ["列表视图", "图库视图"]) {
    await page.getByRole("button", { name: layout, exact: true }).click();
    const area = page.getByLabel("文件区域", { exact: true });
    const first = await area.boundingBox();
    await page.locator('[data-file-id="f1"]').getByRole("checkbox").check();
    await page.locator('[data-file-id="f2"]').getByRole("checkbox").check();
    const selected = await area.boundingBox();
    expect(Math.abs(first!.y - selected!.y)).toBeLessThanOrEqual(1);
    await page
      .getByRole("button", { name: "更多批量操作", exact: true })
      .click();
    expect(
      Math.abs((await area.boundingBox())!.y - first!.y),
    ).toBeLessThanOrEqual(1);
    await expect(page.getByRole("menu")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(
      page.getByRole("button", { name: "更多批量操作", exact: true }),
    ).toBeFocused();
    await page.getByRole("button", { name: "取消多选", exact: true }).click();
    expect(
      Math.abs((await area.boundingBox())!.y - first!.y),
    ).toBeLessThanOrEqual(1);
  }
});
test("v7 folder search and saved scopes return to original provider directory", async ({
  page,
}) => {
  await folder7(page);
  if (
    !(await page
      .getByRole("textbox", { name: "搜索当前目录", exact: true })
      .isVisible())
  )
    await page
      .getByRole("button", { name: "搜索当前目录", exact: true })
      .click();
  await page
    .getByRole("textbox", { name: "搜索当前目录", exact: true })
    .fill("创作简报");
  await page.getByRole("button", { name: "检索当前目录", exact: true }).click();
  await expect(page.getByLabel("当前目录检索结果")).toContainText("创作简报");
  await page.getByRole("button", { name: "保存当前检索", exact: true }).click();
  await page.getByRole("textbox", { name: "保存搜索名称" }).fill("当前项目");
  await page.getByRole("button", { name: "保存视图", exact: true }).click();
  await nav7(page, "搜索");
  await page.getByRole("button", { name: "保存视图", exact: true }).click();
  await page.getByRole("button", { name: "当前项目", exact: true }).click();
  await page.getByRole("button", { name: /更多操作 .*创作简报/ }).click();
  await page
    .getByRole("menuitem", { name: "所在目录并定位", exact: true })
    .click();
  await expect(page.locator('[data-file-id="f1"]')).toBeVisible();
});
test("v7 scoped scan budget and resume states retain original operation", async ({
  page,
}) => {
  await nav7(page, "存储管理");
  await page.getByRole("button", { name: "索引", exact: true }).click();
  await page
    .getByRole("button", { name: /OneDrive · 个人/ })
    .last()
    .click();
  await page.getByRole("button", { name: "配置扫描范围", exact: true }).click();
  await page.getByRole("spinbutton", { name: "扫描条目预算" }).fill("1");
  await page
    .getByRole("button", { name: "确认范围并扫描", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "SCAN-1", exact: true }),
  ).toBeVisible();
  await expect(page.getByText("部分完成", { exact: true })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "重新扫描", exact: true }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "查询原操作", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "重新扫描", exact: true }),
  ).toBeEnabled();
  await page.getByRole("button", { name: "返回索引范围" }).click();
  await expect(page.getByLabel("索引范围详情")).toContainText("总条目：未知");
  await page.getByRole("button", { name: "暂停后续更新" }).click();
  await expect(
    page.getByRole("button", { name: "刷新目录索引" }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "清除索引", exact: true }).click();
  await page.getByRole("button", { name: "确认操作", exact: true }).click();
  await expect(page.getByRole("button", { name: /SCAN-1/ })).toBeVisible();
});
test("v7 content index requires separate consent and searches fixed evidence", async ({
  page,
}) => {
  await nav7(page, "存储管理");
  await page.getByRole("button", { name: "索引", exact: true }).click();
  await page
    .getByRole("button", { name: /DriveBridge 云盘.*尚未扫描/ })
    .click();
  await page.getByRole("button", { name: "内容索引", exact: true }).click();
  await page
    .getByRole("button", { name: "确认范围并建立索引", exact: true })
    .click();
  await page.getByRole("button", { name: "确认操作", exact: true }).click();
  await expect(page.locator("[data-index-id]").first()).toContainText(
    "部分失败",
  );
  await page
    .getByRole("button", { name: "恢复原索引任务", exact: true })
    .click();
  await expect(page.locator("[data-index-id]").first()).toContainText("可用");
  await nav7(page, "搜索");
  await page.getByRole("button", { name: "内容", exact: true }).click();
  await page.getByRole("textbox", { name: "搜索内容" }).fill("自然");
  await page.getByRole("button", { name: "执行搜索" }).click();
  await expect(page.getByLabel("检索结果")).toContainText("正文文本匹配");
  await expect(page.getByText("语义候选", { exact: true })).toBeVisible();
});
test("v7 file preview source stays inbox only and versions keep renderer", async ({
  page,
}) => {
  await nav7(page, "产物与收件");
  await record7(page, page.locator("[data-record-name]").first());
  await page.getByRole("button", { name: "预览", exact: true }).first().click();
  await expect(
    page.getByRole("region", { name: "文件预览工作台" }),
  ).toContainText("收件箱");
  const before = await page.locator("[data-preview-stage]").boundingBox();
  await details7(page, "版本");
  const stage = page.locator("[data-preview-stage]");
  await stage.evaluate((el) => el.setAttribute("data-probe", "stable"));
  await details7(page, "活动");
  await expect(stage).toHaveAttribute("data-probe", "stable");
  await page.getByRole("button", { name: "收起详情", exact: true }).click();
  await page.getByRole("button", { name: "退出预览", exact: true }).click();
  await closeRecords7(page);
  await expect(
    page.getByRole("heading", { name: "产物与收件", exact: true }),
  ).toBeVisible();
});
test("v7 settings find concrete entries without a builtin Agent category", async ({
  page,
}) => {
  await nav7(page, "设置");
  await page.getByRole("textbox", { name: "搜索设置" }).fill("索引");
  await page
    .getByRole("region", { name: "设置搜索结果" })
    .getByRole("button", { name: /内容索引/ })
    .click();
  await page.getByRole("button", { name: "管理目录与内容索引" }).click();
  await expect(page.getByLabel("索引总览")).toBeVisible();
  await nav7(page, "设置");
  await expect(
    page.getByRole("button", { name: "Agent", exact: true }),
  ).toHaveCount(0);
});
test("v7 preview hover and immediate click retain scope and layout", async ({
  page,
}, info) => {
  await folder7(page);
  const trigger = page
    .locator('[data-file-id="f1"]')
    .getByRole("button", { name: /^预览 / });
  if (info.project.name === "desktop") {
    await trigger.hover();
    await expect(page.locator("[data-hover-preview]")).toBeVisible();
    await page.mouse.move(1, 1);
    await expect(page.locator("[data-hover-preview]")).toHaveCount(0);
  }
  await trigger.click();
  await expect(page.locator("[data-preview-workspace]")).toBeVisible();
  await expect(page.locator("[data-hover-preview]")).toHaveCount(0);
});
test("v7 six widths keep bars compact, tree accessible and no horizontal overflow", async ({
  page,
}, info) => {
  test.skip(info.project.name === "mobile");
  for (const width of [320, 390, 768, 1024, 1440, 1920]) {
    await page.setViewportSize({ width, height: 1000 });
    await page.waitForTimeout(80);
    const size = await page.evaluate(() => ({
      body: document.documentElement.scrollWidth,
      width: innerWidth,
      bar: document
        .querySelector("[data-directory-toolbar]")!
        .getBoundingClientRect().height,
    }));
    expect(size.body).toBeLessThanOrEqual(size.width + 1);
    expect(size.bar).toBe(44);
    if (width < 760)
      await expect(
        page.getByRole("button", { name: "打开存储目录" }),
      ).toBeVisible();
  }
  await page.emulateMedia({ reducedMotion: "reduce" });
  await file7(page);
  for (const width of [320, 390, 768, 1024, 1440, 1920]) {
    await page.setViewportSize({ width, height: 1000 });
    await page.waitForTimeout(80);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth - innerWidth,
      ),
    ).toBeLessThanOrEqual(1);
    await expect(page.getByRole("button", { name: "文件操作" })).toBeVisible();
  }
});

test("v7 resource references and chapter copy remain available without a chat runtime", async ({
  page,
}) => {
  await page.evaluate(() =>
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        writeText: () =>
          Promise.reject(new Error("synthetic clipboard denial")),
      },
    }),
  );
  await nav7(page, "资源库");
  await page.locator("[data-resource-list] [data-entry-name]").first().click();
  await page.getByRole("button", { name: "检查资源", exact: true }).click();
  await page
    .getByRole("button", { name: "复制已检查版本引用", exact: true })
    .click();
  await expect(
    page.getByLabel("手动复制资源引用", { exact: true }),
  ).toContainText("drivebridge://demo-instance/");
  await expect(
    page.getByLabel("手动复制资源引用", { exact: true }),
  ).toContainText("固定版本：v1");
  await page.keyboard.press("Escape");
  await expect(page.locator("[data-global-agent]")).toHaveCount(0);
  await nav7(page, "知识库");
  await page
    .locator('[data-knowledge-file="k-guide"]')
    .getByRole("button", { name: "继续编辑" })
    .click();
  await page.getByRole("button", { name: "阅读", exact: true }).click();
  await page.getByRole("button", { name: "格式与引用", exact: true }).click();
  await page.getByRole("button", { name: "引用章节", exact: true }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "项目资料", exact: true })
    .click();
  await expect(
    page.getByLabel("手动复制上下文", { exact: true }),
  ).toContainText("章节：项目资料");
});
test("v7 capacity observations and list visualization use the chosen connection only", async ({
  page,
}) => {
  await nav7(page, "存储管理");
  await page.getByRole("button", { name: "用量", exact: true }).click();
  await expect(page.getByLabel("存储用量")).toContainText("尚无可比较");
  await page.getByRole("button", { name: "记录样本观测", exact: true }).click();
  await page.getByRole("button", { name: "记录样本观测", exact: true }).click();
  await expect(page.getByLabel("容量观测趋势")).toContainText("变化：0.0 MB");
  await page.getByRole("button", { name: "列表视图", exact: true }).click();
  await expect(page.getByLabel("容量观测趋势")).toContainText("观测 2");
  await choose7(page, "用量连接范围", "OneDrive · 工作");
  await expect(page.getByLabel("存储用量")).toContainText("尚无可比较");
});
test("v7 new provider connection confirms metadata scope and keeps content disabled", async ({
  page,
}) => {
  await nav7(page, "存储管理");
  await page.getByRole("button", { name: "添加存储连接", exact: true }).click();
  await page
    .getByRole("textbox", { name: "连接名称", exact: true })
    .fill("OneDrive · 第二账号");
  await page.getByRole("button", { name: "模拟授权账号", exact: true }).click();
  await page.getByRole("button", { name: "模拟只读验证", exact: true }).click();
  await page
    .getByRole("button", { name: "确认以上范围并在保存后扫描", exact: true })
    .click();
  await page.getByRole("button", { name: "保存连接", exact: true }).click();
  await expect(
    page.locator('[data-connection-id="connection-1"]'),
  ).toContainText("OneDrive · 第二账号");
  await page.getByRole("button", { name: "索引", exact: true }).click();
  await page.getByRole("button", { name: /OneDrive · 第二账号/ }).click();
  await page.getByRole("button", { name: "内容索引", exact: true }).click();
  await expect(
    page.getByText("尚未启用任何内容索引。", { exact: true }),
  ).toBeVisible();
});

test("v7 directory errors and unloaded state are distinct from empty content", async ({
  page,
}) => {
  await page.getByRole("button", { name: "演示与审阅", exact: true }).click();
  await choose7(page, "目录加载状态", "尚未加载");
  await expect(page.getByText("目录尚未加载", { exact: true })).toBeVisible();
  await expect(page.getByText("当前范围没有条目", { exact: true })).toHaveCount(
    0,
  );
  await page.getByRole("button", { name: "加载此目录", exact: true }).click();
  await expect(page.locator("[data-directory-id]").first()).toBeVisible();
  await page.getByRole("button", { name: "演示与审阅", exact: true }).click();
  await choose7(page, "目录加载状态", "目录失败");
  await expect(
    page.getByRole("button", { name: "重试原目录", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "重试原目录", exact: true }).click();
  await expect(page.locator("[data-directory-id]").first()).toBeVisible();
});
