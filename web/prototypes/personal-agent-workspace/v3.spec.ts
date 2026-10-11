import { revealNavigation } from "./test-helpers";
async function previewDetails(page: Page) {
  if (
    !(await page.getByRole("button", { name: "讨论", exact: true }).isVisible())
  )
    await page
      .getByRole("button", { name: "显示文件详情", exact: true })
      .click();
}
import { expect, test } from "@playwright/test";
import type { Page } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
async function shot(page: Page, name: string) {
  if (process.env.DRIVEBRIDGE_SKIP_SCREENSHOTS === "1") return;
  await page.evaluate(async () => {
    await document.fonts.ready;
    await Promise.all(
      document
        .getAnimations()
        .filter((a) => a.effect?.getTiming().iterations !== Infinity)
        .map((a) => a.finished.catch(() => {})),
    );
  });
  const dir =
    process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS || "temp/prototype-diagnostics";
  fs.mkdirSync(dir, { recursive: true });
  await page.screenshot({
    path: path.join(dir, name + ".png"),
    animations: "disabled",
  });
}
async function choose(page: Page, label: string, value: string) {
  await page.getByRole("combobox", { name: label, exact: true }).last().click();
  await page.getByRole("option", { name: value, exact: true }).click();
}
async function nav(page: Page, target: string) {
  if (
    await page
      .getByRole("button", { name: "退出预览", exact: true })
      .isVisible()
  )
    await page.getByRole("button", { name: "退出预览", exact: true }).click();
  if (
    await page
      .getByRole("button", { name: "关闭 Agent 侧栏", exact: true })
      .isVisible()
  )
    await page
      .getByRole("button", { name: "关闭 Agent 侧栏", exact: true })
      .click();
  if (
    await page
      .getByRole("button", { name: "取消多选", exact: true })
      .isVisible()
  )
    await page.getByRole("button", { name: "取消多选", exact: true }).click();
  await revealNavigation(page, target);
  if (target === "设置") {
    (await page
      .getByRole("button", { name: "个人设置", exact: true })
      .isVisible())
      ? await page
          .getByRole("button", { name: "个人设置", exact: true })
          .click()
      : await page
          .getByRole("navigation", { name: "底部导航" })
          .getByRole("button", { name: "我的", exact: true })
          .click();
    return;
  }
  if (target === "搜索") {
    await page
      .getByRole("banner")
      .getByRole("button", { name: /^搜索/ })
      .click();
    return;
  }
  const bottom = page.getByRole("navigation", { name: "底部导航" });
  if (await bottom.isVisible()) {
    const map: Record<string, string> = {
      我的云盘: "文件",
      任务中心: "任务",
      我的工作台: "首页",
    };
    await bottom
      .getByRole("button", { name: map[target] || "我的", exact: true })
      .click();
    if (!map[target])
      await page
        .getByRole("main")
        .getByRole("button", { name: target, exact: true })
        .click();
  } else
    await page
      .getByRole("navigation")
      .getByRole("button", { name: target, exact: true })
      .first()
      .click();
}
async function pick(page: Page, id: string) {
  await page.locator(`[data-file-id="${id}"]`).getByRole("checkbox").check();
}
async function batch(page: Page, label: string) {
  const bar = page.getByRole("toolbar", { name: "批量操作" });
  if (await bar.getByRole("button", { name: label, exact: true }).isVisible())
    await bar.getByRole("button", { name: label, exact: true }).click();
  else {
    await bar
      .getByRole("button", { name: "更多批量操作", exact: true })
      .click();
    await page.getByRole("menuitem", { name: label, exact: true }).click();
  }
}
const errors = new WeakMap<Page, string[]>();
test.beforeEach(async ({ page }) => {
  const list: string[] = [];
  errors.set(page, list);
  page.on("pageerror", (e) => list.push(e.message));
  await page.goto("/opendesign.html");
  await expect(
    page.getByRole("heading", { name: "我的云盘", exact: true }),
  ).toBeVisible();
});
test.afterEach(async ({ page }) => expect(errors.get(page)).toEqual([]));

test("v3 batch toolbar never moves files in list, grid, collection or search", async ({
  page,
}, info) => {
  for (const layout of ["列表视图", "图库视图"]) {
    await page.getByRole("button", { name: layout, exact: true }).click();
    await page.getByRole("main").evaluate((el) => {
      const f = el.querySelector('[data-file-id="f1"]')!;
      el.scrollTop +=
        f.getBoundingClientRect().top - el.getBoundingClientRect().top - 70;
    });
    const file = page.locator('[data-file-id="f1"]');
    const before = (await file.boundingBox())!;
    const scroll = await page.getByRole("main").evaluate((el) => el.scrollTop);
    await shot(page, `${info.project.name}-v3-${layout}-before`);
    await pick(page, "f1");
    await pick(page, "f2");
    await expect(page.getByRole("toolbar", { name: "批量操作" })).toBeVisible();
    expect(
      Math.abs((await file.boundingBox())!.y - before.y),
    ).toBeLessThanOrEqual(1);
    expect(await page.getByRole("main").evaluate((el) => el.scrollTop)).toBe(
      scroll,
    );
    expect(
      (await page.locator("[data-file-toolbar]").boundingBox())!.height,
    ).toBe(44);
    await page
      .getByRole("button", { name: "更多批量操作", exact: true })
      .click();
    expect(
      Math.abs((await file.boundingBox())!.y - before.y),
    ).toBeLessThanOrEqual(1);
    await expect(page.getByRole("menu")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.getByRole("menu")).toHaveCount(0);
    await shot(page, `${info.project.name}-v3-${layout}-selected`);
    await page.getByRole("button", { name: "取消多选", exact: true }).click();
    expect(
      Math.abs((await file.boundingBox())!.y - before.y),
    ).toBeLessThanOrEqual(1);
  }
  await nav(page, "项目集合");
  await page.getByRole("button", { name: /山野计划.*份素材/ }).click();
  let top = (await page.locator('[data-file-id="f1"]').boundingBox())!.y;
  await pick(page, "f1");
  expect(
    Math.abs(
      (await page.locator('[data-file-id="f1"]').boundingBox())!.y - top,
    ),
  ).toBeLessThanOrEqual(1);
  await nav(page, "搜索");
  await page
    .getByRole("textbox", { name: "搜索内容", exact: true })
    .fill("山野");
  await page
    .getByRole("main")
    .getByRole("button", { name: "搜索", exact: true })
    .click();
  await page
    .locator('[data-file-id="f1"]')
    .getByRole("checkbox")
    .scrollIntoViewIfNeeded();
  top = (await page.locator('[data-file-id="f1"]').boundingBox())!.y;
  await pick(page, "f1");
  expect(
    Math.abs(
      (await page.locator('[data-file-id="f1"]').boundingBox())!.y - top,
    ),
  ).toBeLessThanOrEqual(1);
});

test("v3 manual tags append and replace separately with group management", async ({
  page,
}, info) => {
  await pick(page, "f1");
  await pick(page, "f2");
  await batch(page, "标签");
  await expect(page.getByRole("dialog")).toContainText("部分拥有");
  await page
    .getByRole("checkbox", { name: "使用标签 待审阅", exact: true })
    .check();
  await page.getByRole("button", { name: "确认更新标签", exact: true }).click();
  await expect(page.locator('[data-file-id="f1"]')).toContainText("+1");
  await batch(page, "标签");
  await choose(page, "标签操作", "替换全部人工标签");
  await page
    .getByRole("checkbox", { name: "使用标签 自然", exact: true })
    .check();
  await page.getByRole("button", { name: "确认更新标签", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "替换全部人工标签？", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "确认操作", exact: true }).click();
  await nav(page, "标签");
  await page
    .getByRole("textbox", { name: "新分组名称", exact: true })
    .fill("归档用途");
  await page.getByRole("button", { name: "创建标签分组", exact: true }).click();
  await page
    .getByRole("textbox", { name: "标签名称", exact: true })
    .fill("年度资料");
  await choose(page, "标签分组", "归档用途");
  await page.getByRole("button", { name: "创建标签", exact: true }).click();
  await expect(
    page.getByRole("button", { name: /年度资料.*个文件/ }),
  ).toBeVisible();
  await shot(page, `${info.project.name}-v3-tags`);
});

test("v3 fixed Agent panel preserves route and scroll and supports retrieval", async ({
  page,
}, info) => {
  await page.getByRole("main").evaluate((el) => (el.scrollTop = 120));
  const scroll = await page.getByRole("main").evaluate((el) => el.scrollTop);
  await page
    .getByRole("button", { name: "打开 Agent 侧栏", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "关闭 Agent 侧栏", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "检索文件", exact: true }).click();
  await page
    .getByRole("textbox", { name: "向助手提问", exact: true })
    .fill("找山野项目中带交付标签的 PDF");
  await page.getByRole("button", { name: "解析检索条件", exact: true }).click();
  await expect(page.getByText("目录：山野计划", { exact: true })).toBeVisible();
  await page
    .getByRole("button", { name: "确认条件并检索", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: /创作简报.pdf · v3/ }),
  ).toBeVisible();
  await page.getByRole("button", { name: "保存搜索视图", exact: true }).click();
  await shot(page, `${info.project.name}-v3-agent-search`);
  await page
    .getByRole("button", { name: "关闭 Agent 侧栏", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "我的云盘", exact: true }),
  ).toBeVisible();
  expect(await page.getByRole("main").evaluate((el) => el.scrollTop)).toBe(
    scroll,
  );
});

test("v3 scoped indexing, exclusion and partial recovery lead to version evidence", async ({
  page,
}, info) => {
  await nav(page, "设置");
  await page
    .getByRole("textbox", { name: "搜索设置", exact: true })
    .fill("索引");
  await page.getByRole("button", { name: "搜索与索引", exact: true }).click();
  await choose(page, "索引目录", "山野计划");
  await page
    .getByRole("textbox", { name: "索引排除项", exact: true })
    .fill("脚本");
  await page
    .getByRole("button", { name: "确认范围并建立索引", exact: true })
    .click();
  await page.getByRole("button", { name: "确认操作", exact: true }).click();
  const job = page.locator('[data-index-id="IDX-1"]');
  await expect(job).toContainText("部分失败");
  await shot(page, `${info.project.name}-v3-index-partial`);
  await job
    .getByRole("button", { name: "恢复原索引任务", exact: true })
    .click();
  await expect(job).toContainText("可用");
  await nav(page, "搜索");
  await page
    .getByRole("main")
    .getByRole("button", { name: "内容", exact: true })
    .click();
  await page
    .getByRole("textbox", { name: "搜索内容", exact: true })
    .fill("自然光");
  await page
    .getByRole("main")
    .getByRole("button", { name: "搜索", exact: true })
    .click();
  await expect(page.locator("[data-search-results]")).toContainText(
    "内容引用 · v3",
  );
  await expect(page.locator('[data-file-id="f3"]')).toHaveCount(0);
  await shot(page, `${info.project.name}-v3-search-evidence`);
});

test("v3 application configuration does not imply write access and receipts keep operation identity", async ({
  page,
}, info) => {
  await nav(page, "外部调用");
  await page.getByRole("button", { name: "接入应用", exact: true }).click();
  await page
    .getByRole("textbox", { name: "应用名称", exact: true })
    .fill("研究助手");
  await page
    .getByRole("checkbox", { name: "提交产物到收件箱", exact: true })
    .check();
  await page.getByRole("button", { name: "保存演示配置", exact: true }).click();
  const app = page.locator('[data-app-id="APP-1"]');
  await expect(app).toContainText("配置已保存 · 尚未验证");
  await app
    .getByRole("button", { name: "5. 模拟只读验证", exact: true })
    .click();
  await expect(app).toContainText("写入：尚未验证");
  await app.getByRole("button", { name: "演示结果未知", exact: true }).click();
  const call = page.locator('[data-call-id="CALL-1"]');
  await call.getByRole("button", { name: "查询原操作", exact: true }).click();
  await expect(call).toContainText("EXT-1");
  await expect(call).toContainText("已完成");
  await app.getByRole("button", { name: "模拟回存产物", exact: true }).click();
  await expect(app).toContainText("示例产物回存已验证");
  await shot(page, `${info.project.name}-v3-external`);
  await page.getByRole("button", { name: "验收回存产物", exact: true }).click();
  await expect(page.getByRole("main")).toContainText("研究助手 · 检索摘要.md");
});

test("v3 settings preferences persist alone and unknown capabilities stay unknown", async ({
  page,
}, info) => {
  await nav(page, "设置");
  await page.getByRole("button", { name: "通用与外观", exact: true }).click();
  await choose(page, "外观主题", "深色");
  await choose(page, "默认文件视图", "网格");
  await page.reload();
  await expect(page.locator("html")).toHaveClass(/dark/);
  await expect(
    page.getByRole("button", { name: "图库视图", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
  const persisted = await page.evaluate(() =>
    JSON.parse(localStorage.getItem("drivebridge-prototype-v3-preferences")!),
  );
  expect(Object.keys(persisted).sort()).toEqual(
    [
      "agentRememberOpen",
      "agentWidth",
      "navCollapsed",
      "previewListWidth",
      "inspectorPosition",
      "density",
      "layout",
      "reduceMotion",
      "startup",
      "theme",
    ].sort(),
  );
  await nav(page, "能力与连接说明");
  await choose(page, "检查的连接", "S3 · 归档 · 演示连接");
  await expect(page.getByRole("main")).toContainText("未知 · 尚未验证");
  await shot(page, `${info.project.name}-v3-capabilities`);
});

test("v3 tag merge updates saved views and deletion leaves files intact", async ({
  page,
}) => {
  await nav(page, "搜索");
  await page.getByRole("button", { name: "交付", exact: true }).click();
  await page
    .getByRole("textbox", { name: "保存搜索名称", exact: true })
    .fill("交付文件视图");
  await page.getByRole("button", { name: "保存搜索", exact: true }).click();
  await nav(page, "标签");
  const row = page.getByRole("button", { name: /#交付.*个文件/ }).locator("..");
  await row.getByRole("button", { name: "管理", exact: true }).click();
  await choose(page, "合并到标签", "项目资料");
  await page.getByRole("button", { name: "合并", exact: true }).click();
  await expect(page.getByRole("dialog")).toContainText("1 个保存视图");
  await page.getByRole("button", { name: "确认操作", exact: true }).click();
  await expect(page.getByRole("button", { name: /#交付.*个文件/ })).toHaveCount(
    0,
  );
  await nav(page, "搜索");
  await page.getByRole("button", { name: "交付文件视图", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "项目资料", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
  await expect(page.locator('[data-file-id="f1"]')).toBeVisible();
});

test("v3 width matrix, tablet drawer, scope lock and keyboard focus", async ({
  page,
}, info) => {
  test.skip(
    info.project.name !== "desktop",
    "Desktop matrix covers all five widths.",
  );
  for (const width of [320, 390, 768, 1024, 1440]) {
    await page.setViewportSize({ width, height: width < 768 ? 844 : 1000 });
    await expect(page.locator(`[data-compact=${width < 768}]`)).toBeVisible();
    await expect
      .poll(() =>
        page
          .getByRole("main")
          .evaluate((el) => el.scrollWidth <= el.clientWidth),
      )
      .toBe(true);
    await shot(page, `v3-drive-${width}`);
    await page
      .getByRole("button", { name: "打开 Agent 侧栏", exact: true })
      .click();
    await expect(
      page.getByRole("button", { name: "关闭 Agent 侧栏", exact: true }),
    ).toBeVisible();
    if (width < 1280)
      await expect(
        page.getByRole("dialog", { name: "Agent 侧栏", exact: true }),
      ).toBeVisible();
    else {
      const divider = page.getByRole("separator", {
        name: "调整 Agent 侧栏宽度",
        exact: true,
      });
      await divider.focus();
      await page.keyboard.press("ArrowLeft");
      await expect(divider).toHaveAttribute("aria-valuenow", "376");
    }
    await shot(page, `v3-agent-${width}`);
    await page
      .getByRole("button", { name: "关闭 Agent 侧栏", exact: true })
      .click();
    await expect(
      page.getByRole("button", { name: "打开 Agent 侧栏", exact: true }),
    ).toBeFocused();
  }
  await page.getByRole("button", { name: "演示与审阅", exact: true }).click();
  await choose(page, "演示状态", "无权限");
  await page
    .getByRole("button", { name: "打开 Agent 侧栏", exact: true })
    .click();
  await expect(
    page.getByRole("complementary", { name: "Agent 侧栏", exact: true }),
  ).toContainText("当前范围不可访问");
  await expect(
    page.getByRole("complementary", { name: "Agent 侧栏", exact: true }),
  ).not.toContainText("创作简报");
});
