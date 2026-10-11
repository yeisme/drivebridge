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
  await page.goto("/opendesign.html");
});
test.afterEach(async ({ page }) => expect(errors.get(page)).toEqual([]));
const choose = choose7;
async function nav(page: Page, target: string) {
  await nav7(page, target);
  if (["我的文件", "我的云盘", "素材库"].includes(target)) await folder7(page);
}
const navigate = nav;
async function scenario(page: Page, name: string) {
  await choose(page, "演示场景", name);
  if (name.startsWith("07")) await folder7(page);
}
async function confirm(page: Page) {
  if (
    !(await page
      .getByRole("button", { name: "检查并确认", exact: true })
      .first()
      .isVisible())
  )
    await record7(page, page.locator("[data-task-id]").first());
  await page
    .getByRole("button", { name: "检查并确认", exact: true })
    .first()
    .click();
  await page
    .getByRole("button", { name: "确认执行这份计划", exact: true })
    .click();
}
async function previewDetails(page: Page) {
  await details7(page);
}
async function pick(page: Page, id: string) {
  if (!(await page.locator(`[data-file-id="${id}"]`).isVisible()))
    await folder7(page);
  await page.locator(`[data-file-id="${id}"]`).getByRole("checkbox").check();
}
async function batch(page: Page, name: string) {
  const bar = page.getByRole("toolbar", { name: "批量操作" });
  if (await bar.getByRole("button", { name, exact: true }).isVisible())
    await bar.getByRole("button", { name, exact: true }).click();
  else {
    await bar
      .getByRole("button", { name: "更多批量操作", exact: true })
      .click();
    await page.getByRole("menuitem", { name, exact: true }).click();
  }
}
test("01 phone import, verify, classify, and archive", async ({
  page,
}, info) => {
  await scenario(page, "01 手机采集与归档");
  await page.getByRole("button", { name: "手机导入", exact: true }).click();
  await page.getByRole("button", { name: "选择手机照片", exact: true }).click();
  const upload = page.locator('[data-task-id="DB-025"]');
  await expect(
    upload.getByText("已完成", { exact: true }).first(),
  ).toBeVisible();
  await navigate(page, "产物与收件");
  await record7(
    page,
    page.locator('[data-record-name="山行随拍 · 手机导入.jpg"]'),
  );
  const item = page
    .getByRole("article")
    .filter({ hasText: "山行随拍 · 手机导入.jpg" });
  await item.getByRole("button", { name: "预览归档计划", exact: true }).click();
  await confirm(page);
  const plan = page.locator('[data-task-id="DB-026"]');
  await expect(plan.getByText("部分完成", { exact: true })).toBeVisible();
  await plan.getByRole("button", { name: "恢复未完成项", exact: true }).click();
  await expect(plan.getByText("已完成", { exact: true }).first()).toBeVisible();
  await navigate(page, "我的文件");
  await expect(
    page.getByRole("button", { name: /山行随拍 · 手机导入/ }).first(),
  ).toBeVisible();
});

test("02 shared plan confirmation, partial recovery, and compensation", async ({
  page,
}, info) => {
  await scenario(page, "02 跨端确认与恢复");
  if (info.project.name === "desktop") {
    await page.getByRole("button", { name: "演示与审阅", exact: true }).click();
    await page.getByRole("button", { name: "模拟手机", exact: true }).click();
  }
  await confirm(page);
  const card = page.locator('[data-task-id="DB-024"]');
  await expect(card.getByText("部分完成", { exact: true })).toBeVisible();
  await card.getByRole("button", { name: "恢复未完成项", exact: true }).click();
  await expect(card.getByText("已完成", { exact: true }).first()).toBeVisible();
  await expect(page.locator("[data-task-id]")).toHaveCount(1);
  await card.getByRole("button", { name: "生成恢复计划", exact: true }).click();
  await expect(
    page
      .locator('[data-task-id="DB-025"]')
      .getByText("待确认", { exact: true }),
  ).toBeVisible();
});

test("04 external Agent setup and artifact acceptance", async ({
  page,
}, info) => {
  await scenario(page, "04 外部 Agent 产物");
  await page.getByRole("button", { name: "查看资料助手", exact: true }).click();
  await page.getByRole("button", { name: "设置接入", exact: true }).click();
  await page.getByRole("button", { name: "下一步：接入说明" }).click();
  await page.getByRole("button", { name: "下一步：验证连接" }).click();
  await page.getByRole("button", { name: "完成演示接入", exact: true }).click();
  await page.getByRole("button", { name: "模拟处理资料", exact: true }).click();
  await expect(
    page
      .locator('[data-task-id="DB-025"]')
      .getByText("已完成", { exact: true })
      .first(),
  ).toBeVisible();
  await navigate(page, "产物与收件");
  await record7(
    page,
    page.locator('[data-record-name="山野资料 · Agent 摘要.md"]'),
  );
  const artifact = page
    .getByRole("article")
    .filter({ hasText: "山野资料 · Agent 摘要.md" });
  await artifact.getByRole("button", { name: "预览", exact: true }).click();
  await page.getByRole("button", { name: "退出预览", exact: true }).click();
  await artifact
    .getByRole("button", { name: "接收并归档", exact: true })
    .click();
  await expect(artifact).toHaveCount(0);
});

test("05 transfer conflict preserves originals and resumes same operation", async ({
  page,
}, info) => {
  await scenario(page, "05 跨存储复制");
  await page.getByRole("button", { name: "预览复制计划", exact: true }).click();
  await confirm(page);
  const card = page.locator('[data-task-id="DB-025"]');
  await expect(card.getByText("部分完成", { exact: true })).toBeVisible();
  await expect(card).toContainText("尚未覆盖");
  await card
    .getByRole("button", { name: "保留两份并继续", exact: true })
    .click();
  await expect(card.getByText("已完成", { exact: true }).first()).toBeVisible();
  await nav(page, "存储管理");
  await page.getByRole("button", { name: "用量", exact: true }).click();
  await expect(page.getByLabel("存储用量")).toBeVisible();
});

test("06 offline reading and explicit version update", async ({
  page,
}, info) => {
  await scenario(page, "06 离线与版本冲突");
  await page.getByRole("button", { name: "模拟断网", exact: true }).click();
  await page
    .getByRole("button", { name: "离线阅读", exact: true })
    .first()
    .click();
  await expect(page.locator("[data-renderer]")).toHaveAttribute(
    "data-renderer",
    "f1@2",
  );
  await page.getByRole("button", { name: "退出预览", exact: true }).click();
  await page
    .getByRole("button", { name: "重新连接", exact: true })
    .first()
    .click();
  await page
    .getByRole("button", { name: "更新副本", exact: true })
    .first()
    .click();
  await expect(
    page.getByText("云端 v3 · 本设备 v3", { exact: true }),
  ).toBeVisible();
});

test("07 version comments, fixed delivery and revocation", async ({
  page,
}, info) => {
  await scenario(page, "07 版本讨论与交付");
  await page
    .getByRole("button", { name: /山野计划 · 创作简报.pdf/ })
    .first()
    .click();
  await details7(page, "版本");
  await page
    .getByText("版本 1", { exact: true })
    .locator("../..")
    .getByRole("button", { name: "查看", exact: true })
    .click();
  await previewDetails(page);
  await page.getByRole("button", { name: "讨论", exact: true }).click();
  await page
    .getByRole("textbox", { name: "版本评论", exact: true })
    .fill("此处保留最初的创作方向。示例评论。");
  await page.getByRole("button", { name: "发送评论", exact: true }).click();
  await expect(page.getByText("你 · v1", { exact: true })).toBeVisible();
  if (await page.getByRole("dialog").isVisible())
    await page.keyboard.press("Escape");
  await operation7(page, "交付此版本");
  await page
    .getByRole("button", { name: "确认交付固定版本", exact: true })
    .click();
  await record7(page, page.locator("[data-record-name]").first());
  await expect(
    page.getByRole("heading", {
      name: "山野计划 · 创作简报.pdf · v1",
      exact: true,
    }),
  ).toBeVisible();
  await record7(page, page.locator("[data-record-name]").first());
  await page.getByRole("button", { name: "模拟对方接收", exact: true }).click();
  await record7(page, page.locator("[data-record-name]").first());
  await page.getByRole("button", { name: "撤销分享", exact: true }).click();
  await expect(page.getByRole("dialog").last()).toContainText("无法召回");
  await page.getByRole("button", { name: "确认操作", exact: true }).click();
  await closeRecords7(page);
  await expect(
    page.getByRole("main").getByText("已撤销", { exact: true }),
  ).toBeVisible();
});

test("08 scoped rule pauses and resumes original task", async ({
  page,
}, info) => {
  await scenario(page, "08 备份与自动整理");
  await page.getByRole("button", { name: "配置规则", exact: true }).click();
  await choose(page, "规则类型", "新收件 · 自动归档");
  await expect(page.getByRole("dialog").last()).toContainText("删除传播：关闭");
  await page
    .getByRole("button", { name: "确认范围并启用", exact: true })
    .click();
  await record7(page, page.locator("[data-record-name]").first());
  await page
    .getByRole("button", { name: "模拟条件受限的运行", exact: true })
    .click();
  const card = page.locator('[data-task-id="DB-025"]');
  await expect(card.getByText("已暂停", { exact: true })).toBeVisible();
  await record7(page, card);
  await card.getByRole("button", { name: "恢复未完成项", exact: true }).click();
  await expect(card.getByText("已完成", { exact: true }).first()).toBeVisible();
});

test("stale confirmation and space switch do not submit hidden work", async ({
  page,
}) => {
  await scenario(page, "02 跨端确认与恢复");
  await record7(page, page.locator("[data-task-id]").first());
  await page.getByRole("button", { name: "检查并确认", exact: true }).click();
  await page
    .getByRole("button", { name: "演示异常：确认前计划发生变化", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "确认执行这份计划", exact: true }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "关闭对话框", exact: true }).click();
  await navigate(page, "我的");
  await page
    .getByRole("main")
    .getByRole("button", { name: "账户与空间", exact: true })
    .click();
  await choose(page, "切换空间", "团队空间");
  await navigate(page, "任务中心");
  await expect(page.locator("[data-task-id]")).toHaveCount(0);
});

test("unknown result is reconciled before the original operation resumes", async ({
  page,
}) => {
  await choose(page, "演示状态", "结果未知");
  const card = page.locator('[data-task-id="DB-024"]');
  await expect(card.getByText("结果待确认", { exact: true })).toBeVisible();
  await expect(
    card.getByRole("button", { name: "恢复未完成项", exact: true }),
  ).toHaveCount(0);
  await record7(page, card);
  await card.getByRole("button", { name: "查询原操作", exact: true }).click();
  await expect(card.getByText("部分完成", { exact: true })).toBeVisible();
  await card.getByRole("button", { name: "恢复未完成项", exact: true }).click();
  await expect(card.getByText("已完成", { exact: true }).first()).toBeVisible();
  await expect(page.locator("[data-task-id]")).toHaveCount(1);
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
  await record7(page, app);
  await expect(app).toContainText("配置已保存 · 尚未验证");
  await app
    .getByRole("button", { name: "5. 模拟只读验证", exact: true })
    .click();
  await expect(app).toContainText("写入：尚未验证");
  await app.getByRole("button", { name: "演示结果未知", exact: true }).click();
  await closeRecords7(page);
  const call = page.locator('[data-call-id="CALL-1"]');
  await call.getByRole("button", { name: "查询原操作", exact: true }).click();
  await expect(call).toContainText("EXT-1");
  await expect(call).toContainText("已完成");
  await record7(page, app);
  await app.getByRole("button", { name: "模拟回存产物", exact: true }).click();
  await expect(app).toContainText("示例产物回存已验证");
  await closeRecords7(page);
  await page.getByRole("button", { name: "验收回存产物", exact: true }).click();
  await expect(page.getByRole("main")).toContainText("研究助手 · 检索摘要.md");
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
  await record7(
    page,
    page.locator("[data-record-name]").filter({ hasText: "邀请 陈雨" }),
  );
  await page.getByRole("button", { name: "接受邀请", exact: true }).click();
  await closeRecords7(page);
  await page.getByRole("button", { name: "团队空间", exact: true }).click();
  await expect(page.getByRole("main")).toContainText("未获文件权限");
  await choose(page, "模拟协作身份", "Yeisme · 模拟身份");
  await choose(page, "陈雨 文件权限", "可查看");
  await page.getByRole("button", { name: "确认操作", exact: true }).click();
  await page.getByRole("button", { name: "我发出的共享", exact: true }).click();
  await record7(page, page.locator("[data-record-name]").first());
  await page
    .getByRole("button", { name: "撤销成员共享", exact: true })
    .first()
    .click();
  await page.getByRole("button", { name: "确认操作", exact: true }).click();
  await closeRecords7(page);
  await choose(page, "模拟协作身份", "林舟 · 模拟身份");
  await page.getByRole("button", { name: "共享给我", exact: true }).click();
  await record7(page, page.locator("[data-record-name]").first());
  await expect(
    page.getByRole("button", { name: "预览共享文件", exact: true }),
  ).toBeDisabled();
});

test("v4 version threads reply and resolve without changing permission", async ({
  page,
}, info) => {
  await file7(page);
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
  await expect(page.getByRole("main")).toContainText("持续更新");
  await expect(page.getByRole("main")).toContainText("指定成员");
  await record7(page, page.locator("[data-record-name]").first());
  await page.getByRole("button", { name: "撤销分享", exact: true }).click();
  await page.getByRole("button", { name: "确认操作", exact: true }).click();
  await closeRecords7(page);
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
  const leave = () => nav7(page, "我的云盘");
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
  await expect(page.locator("[data-cloud-explorer]")).toBeVisible();
});
