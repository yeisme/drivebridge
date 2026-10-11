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
        .last()
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

test("v4 provider instances switch without changing context and configure through the page", async ({
  page,
}, info) => {
  const strip = page.getByRole("navigation", { name: "存储切换" });
  await strip.getByRole("button", { name: /OneDrive · 工作/ }).click();
  await expect(page.locator('[data-file-id="od-work-file"]')).toBeVisible();
  expect((await page.locator("[data-demo-bar]").boundingBox())!.height).toBe(
    44,
  );
  await expect(page.locator('[data-file-id="od-personal-file"]')).toHaveCount(
    0,
  );
  await strip.getByRole("button", { name: /OneDrive · 个人/ }).click();
  await expect(page.locator('[data-file-id="od-personal-file"]')).toBeVisible();
  await nav(page, "存储与传输");
  await shot(page, `${info.project.name}-v4-providers`);
  await page.getByRole("button", { name: "添加存储连接", exact: true }).click();
  await page
    .getByRole("textbox", { name: "连接名称", exact: true })
    .fill("OneDrive · 新账号");
  await page.getByRole("button", { name: "模拟授权账号", exact: true }).click();
  await page.getByRole("button", { name: "模拟只读验证", exact: true }).click();
  await shot(page, `${info.project.name}-v4-connection-editor`);
  await page.getByRole("button", { name: "保存连接", exact: true }).click();
  const instance = page.locator('[data-connection-id="connection-1"]');
  await expect(instance).toContainText("通过（演示）");
  await expect(instance).toContainText("尚未验证");
  await instance
    .getByRole("button", { name: "配置 OneDrive · 新账号" })
    .click();
  await page
    .getByRole("textbox", { name: "连接名称", exact: true })
    .fill("未保存名称");
  await page.getByRole("button", { name: "返回存储连接", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "放弃未保存的配置？" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "放弃修改", exact: true }).click();
  await expect(instance).toContainText("OneDrive · 新账号");
});

test("v4 composer references sit above input, keyboard picker and draft survive dismissal", async ({
  page,
}, info) => {
  await page
    .getByRole("button", { name: "打开 Agent 侧栏", exact: true })
    .click();
  const input = page.getByRole("textbox", { name: "向助手提问", exact: true });
  await input.fill("保留这段草稿");
  const rail = page.locator("[data-context-rail]");
  const a = await rail.boundingBox();
  const b = await input.boundingBox();
  expect(a!.y + a!.height).toBeLessThanOrEqual(b!.y);
  await page.getByRole("button", { name: "添加上下文", exact: true }).click();
  await page
    .getByRole("textbox", { name: "搜索上下文", exact: true })
    .fill("旅行清单");
  await page.keyboard.press("Enter");
  await expect(rail).toContainText("旅行清单.pdf");
  await expect(input).toHaveValue("保留这段草稿");
  await page.getByRole("button", { name: "添加上下文", exact: true }).click();
  await page.keyboard.press("Escape");
  await expect(input).toHaveValue("保留这段草稿");
  await shot(page, `${info.project.name}-v4-agent-composer`);
  await page.getByRole("button", { name: "会话历史", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "会话历史", exact: true }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.locator("details,summary")).toHaveCount(0);
});

test("v4 settings search locates actual setting and uses responsive outline", async ({
  page,
}, info) => {
  await nav(page, "设置");
  await page
    .getByRole("textbox", { name: "搜索设置", exact: true })
    .fill("Wi-Fi");
  await expect(
    page.getByRole("region", { name: "设置搜索结果" }),
  ).toContainText("仅在 Wi-Fi 下传输");
  await page
    .getByRole("region", { name: "设置搜索结果" })
    .getByRole("button")
    .click();
  await expect(
    page.locator('[data-setting="仅在 Wi-Fi 下传输"]'),
  ).toBeFocused();
  await shot(page, `${info.project.name}-v4-settings`);
  await page
    .getByRole("textbox", { name: "搜索设置", exact: true })
    .fill("分享有效期");
  await page
    .getByRole("region", { name: "设置搜索结果" })
    .getByRole("button")
    .click();
  await choose(page, "分享有效期", "30 天");
  await expect(page.getByRole("main")).toContainText("共享默认值已保存");
});

test("v4 task files open fixed preview and restore list without expansion", async ({
  page,
}, info) => {
  await nav(page, "任务中心");
  const card = page.locator('[data-task-id="DB-024"]');
  const before = await card.boundingBox();
  await card.getByRole("button", { name: /来源 · 山野计划/ }).click();
  await expect(
    page.getByRole("region", { name: "文件预览工作台" }),
  ).toContainText("v3");
  await page.getByRole("button", { name: "退出预览", exact: true }).click();
  const after = await card.boundingBox();
  expect(Math.abs(before!.y - after!.y)).toBeLessThanOrEqual(1);
  await card.getByRole("button", { name: "任务记录", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "任务记录与关联文件" }),
  ).toBeVisible();
  await shot(page, `${info.project.name}-v4-task-files`);
  await page.keyboard.press("Escape");
  await expect(card).toBeVisible();
});

test("v4 invitation does not grant access, permissions and revocation stay explicit", async ({
  page,
}, info) => {
  await nav(page, "共享与协作");
  await page.getByRole("button", { name: "团队空间", exact: true }).click();
  await page
    .getByRole("button", { name: "邀请陈雨加入团队", exact: true })
    .click();
  await choose(page, "模拟协作身份", "陈雨 · 模拟身份");
  await page.getByRole("button", { name: "接受邀请", exact: true }).click();
  await page.getByRole("button", { name: "团队空间", exact: true }).click();
  await expect(page.getByRole("main")).toContainText("未获文件权限");
  await choose(page, "模拟协作身份", "Yeisme · 模拟身份");
  await choose(page, "陈雨 文件权限", "可查看");
  await page.getByRole("button", { name: "确认操作", exact: true }).click();
  await shot(page, `${info.project.name}-v4-team`);
  await page.getByRole("button", { name: "我发出的共享", exact: true }).click();
  await page
    .getByRole("button", { name: "撤销成员共享", exact: true })
    .first()
    .click();
  await page.getByRole("button", { name: "确认操作", exact: true }).click();
  await choose(page, "模拟协作身份", "林舟 · 模拟身份");
  await page.getByRole("button", { name: "共享给我", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "预览共享文件", exact: true }),
  ).toBeDisabled();
});

test("v4 version threads reply and resolve without changing permission", async ({
  page,
}, info) => {
  await page
    .getByRole("button", { name: "预览 山野计划 · 创作简报.pdf", exact: true })
    .click();
  await previewDetails(page);
  await page.getByRole("button", { name: "讨论", exact: true }).click();
  await page
    .getByRole("textbox", { name: "版本评论", exact: true })
    .fill("@陈雨 请核对引用");
  await page.getByRole("button", { name: "发送评论", exact: true }).click();
  await page.getByRole("button", { name: "回复讨论", exact: true }).click();
  await page
    .getByRole("textbox", { name: "回复内容", exact: true })
    .fill("已核对固定版本");
  await page.getByRole("button", { name: "发送回复", exact: true }).click();
  await page.getByRole("button", { name: "标记已解决", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "重新打开", exact: true }),
  ).toBeVisible();
  await shot(page, `${info.project.name}-v4-comments`);
});

test("v4 layout matrix remains readable with no body disclosure blocks", async ({
  page,
}, info) => {
  test.skip(
    info.project.name === "mobile",
    "Desktop run covers all requested widths",
  );
  test.setTimeout(60000);
  for (const width of [320, 390, 768, 1024, 1440]) {
    await page.setViewportSize({ width, height: 1000 });
    await expect(
      page.getByRole("navigation", { name: "底部导航" }),
    ).toHaveCount(width < 768 ? 1 : 0);
    await nav(page, "我的云盘");
    await shot(page, `v4-drive-${width}`);
    for (const route of ["我的云盘", "存储与传输", "设置", "共享与协作"]) {
      await nav(page, route);
      const overflow = await page
        .getByRole("main")
        .evaluate((el) => el.scrollWidth - el.clientWidth);
      expect(overflow, `${route} at ${width}`).toBeLessThanOrEqual(1);
      await expect(page.locator("details,summary")).toHaveCount(0);
    }
  }
  await page.emulateMedia({ reducedMotion: "reduce" });
  await nav(page, "我的云盘");
  await page
    .getByRole("button", { name: "打开 Agent 侧栏", exact: true })
    .click();
  await shot(page, "v4-reduced-motion");
});

test("v4 S3 and Google Drive configuration, expired access and original reconnect", async ({
  page,
}, info) => {
  await nav(page, "存储与传输");
  await page.getByRole("button", { name: "添加存储连接", exact: true }).click();
  await page.getByRole("button", { name: "S3 兼容", exact: true }).click();
  await page
    .getByRole("textbox", { name: "连接名称", exact: true })
    .fill("S3 · 第二归档");
  await page
    .getByRole("textbox", { name: "endpoint", exact: true })
    .fill("https://storage.example.invalid");
  await page.getByRole("textbox", { name: "bucket", exact: true }).fill("demo");
  await page.getByRole("button", { name: "使用演示凭据", exact: true }).click();
  await page.getByRole("button", { name: "模拟只读验证", exact: true }).click();
  await page.getByRole("button", { name: "保存连接", exact: true }).click();
  await page.getByRole("button", { name: "添加存储连接", exact: true }).click();
  await page.getByRole("button", { name: "Google Drive", exact: true }).click();
  await page
    .getByRole("textbox", { name: "连接名称", exact: true })
    .fill("Google Drive · 第二账号");
  await page.getByRole("button", { name: "模拟授权账号", exact: true }).click();
  await page.getByRole("button", { name: "保存连接", exact: true }).click();
  await page
    .getByRole("button", { name: "配置 OneDrive · 工作", exact: true })
    .click();
  await page.getByRole("button", { name: "演示授权到期", exact: true }).click();
  const c = page.locator('[data-connection-id="od-work"]');
  await expect(c).toContainText("授权过期");
  await c
    .getByRole("button", { name: "配置 OneDrive · 工作", exact: true })
    .click();
  await page.getByRole("button", { name: "模拟重新授权", exact: true }).click();
  await expect(c).toContainText("尚未验证");
  await c
    .getByRole("button", { name: "配置 OneDrive · 工作", exact: true })
    .click();
  await page.getByRole("button", { name: "模拟只读验证", exact: true }).click();
  await page.getByRole("button", { name: "保存连接", exact: true }).click();
  await expect(c).toContainText("通过（演示）");
  await shot(page, `${info.project.name}-v4-multiple-connections`);
});

test("v4 dynamic sharing declares audience and policy and revokes explicitly", async ({
  page,
}, info) => {
  await pick(page, "f1");
  await batch(page, "分享");
  await choose(page, "分享版本策略", "持续更新最新版本");
  await choose(page, "访问对象", "指定成员");
  await choose(page, "分享接收人", "林舟");
  await page
    .getByRole("button", { name: "确认分享持续更新内容", exact: true })
    .click();
  await expect(page.getByRole("main")).toContainText("持续更新最新版本");
  await expect(page.getByRole("main")).toContainText("指定成员");
  await shot(page, `${info.project.name}-v4-live-share`);
  await page.getByRole("button", { name: "撤销分享", exact: true }).click();
  await page.getByRole("button", { name: "确认操作", exact: true }).click();
  await expect(page.getByRole("main")).toContainText("已撤销");
});

test("v4 global navigation protects an unsaved connection form", async ({
  page,
}) => {
  await nav(page, "存储与传输");
  await page.getByRole("button", { name: "添加存储连接", exact: true }).click();
  await page
    .getByRole("textbox", { name: "连接名称", exact: true })
    .fill("仍在编辑");
  const leave = async () => {
    const bottom = page.getByRole("navigation", { name: "底部导航" });
    if (await bottom.isVisible())
      await bottom.getByRole("button", { name: "文件", exact: true }).click();
    else
      await page
        .getByRole("navigation")
        .getByRole("button", { name: "我的云盘", exact: true })
        .click();
  };
  await leave();
  await expect(
    page.getByRole("heading", { name: "离开未保存的连接配置？", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "继续编辑连接", exact: true }).click();
  await expect(
    page.getByRole("textbox", { name: "连接名称", exact: true }),
  ).toHaveValue("仍在编辑");
  await leave();
  await page
    .getByRole("button", { name: "放弃修改并离开", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "我的云盘", exact: true }),
  ).toBeVisible();
});
