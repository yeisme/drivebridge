import { selectChoice } from "./helpers/select";
import { test, expect, type Page } from "@playwright/test";
import path from "node:path";
const artifacts =
  process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS || "temp/browser-artifacts";
async function persona(page: Page, name: string) {
  await page.getByRole("button", { name: "工作区选项", exact: true }).click();
  await selectChoice(page.getByLabel("体验身份"),name);
  await page.getByRole("button", { name: "切换体验身份", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "团队", exact: true }),
  ).toBeVisible();
}
async function openTeam(page: Page, name = "设计协作团队") {
  await page.getByRole("button", { name: "团队", exact: true }).click();
  await page.getByRole("button",{name:`打开团队 ${name}`,exact:true}).click();
  await expect(
    page.getByRole("complementary", { name: "团队页面" }),
  ).toBeVisible();
}
test("team workspace real persona, routes, permissions, archive and restore", async ({
  page,
}) => {
  test.skip(
    process.env.DRIVEBRIDGE_TEST_TEAM_DEMO !== "1",
    "Requires explicit isolated team demo",
  );
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto("/");
  await page.getByRole("button", { name: "无需账号，进入本地体验" }).click();
  await openTeam(page);
  await page.getByRole("link", { name: "成员", exact: true }).click();
  await expect(
    page.getByRole("row").filter({ hasText: "素材负责人" }),
  ).toBeVisible();
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "团队成员", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "折叠团队导航" }).click();
  await page.getByRole("link", { name: "网盘", exact: true }).click();
  await page.goBack();
  await expect(
    page.getByRole("heading", { name: "团队成员", exact: true }),
  ).toBeVisible();
  await page.getByRole("link", { name: "权限", exact: true }).click();
  const select = page.getByLabel("团队成员 在 团队素材 的权限");
  await selectChoice(select,"editor");
  await expect(
    page.getByRole("dialog", { name: "确认团队变更" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "确认变更", exact: true }).click();
  await expect(select).toHaveAttribute("data-value", "editor");
  await page.getByRole("link", { name: "设置", exact: true }).click();
  await page.getByRole("button", { name: "编辑资料" }).click();
  await page.getByLabel("团队说明").fill("绿色团队工作区 · 浏览器验收");
  await page.getByRole("button", { name: "保存并继续" }).click();
  await expect(page.locator(".team-header")).toContainText("绿色团队工作区");
  await page.screenshot({
    path: path.join(artifacts, "team-settings-desktop.png"),
    fullPage: true,
  });
  await persona(page, "custodian");
  await openTeam(page);
  await page.getByRole("link", { name: "网盘", exact: true }).click();
  await page
    .locator("button.drive-card")
    .filter({ hasText: "团队素材" })
    .click();
  await expect(
    page.getByRole("button", { name: "新建", exact: true }),
  ).toBeEnabled();
  await page.locator("input[type=file]").setInputFiles({
    name: "team-preview.txt",
    mimeType: "text/plain",
    buffer: Buffer.from("Preview in exact team space"),
  });
  await expect(
    page.getByRole("button", { name: "详情 team-preview.txt", exact: true }),
  ).toBeVisible();
  await page.goBack();
  await expect(
    page.getByRole("complementary", { name: "团队页面" }),
  ).toBeVisible();
  await page.getByRole("link", { name: "概览", exact: true }).click();
  await page
    .locator(".recent-media button")
    .filter({ hasText: "team-preview.txt" })
    .click();
  await expect(page.getByRole("dialog")).toContainText(
    "Preview in exact team space",
  );
  await page.keyboard.press("Escape");
  await persona(page, "admin");
  await openTeam(page);
  await page.getByRole("link", { name: "设置", exact: true }).click();
  await page.getByRole("button", { name: "归档团队", exact: true }).click();
  await page.getByRole("button", { name: "确认变更", exact: true }).click();
  await expect(page.locator(".team-header")).toContainText("只读");
  await persona(page, "member");
  await page.getByRole("button", { name: "已归档", exact: true }).click();
  await page.getByRole("button",{name:"卡片视图",exact:true}).click();
  await page
    .locator("button.drive-card")
    .filter({ hasText: "设计协作团队" })
    .click();
  await page.getByRole("link", { name: "网盘", exact: true }).click();
  await page
    .locator("button.drive-card")
    .filter({ hasText: "团队素材" })
    .click();
  await expect(
    page.getByRole("button", { name: "新建", exact: true }),
  ).toBeDisabled();
  await page
    .getByRole("button", { name: "详情 team-preview.txt", exact: true })
    .click();
  await expect(page.getByRole("dialog")).toContainText(
    "Preview in exact team space",
  );
  await page.keyboard.press("Escape");
  await persona(page, "admin");
  await page.getByRole("button", { name: "已归档", exact: true }).click();
  await page.getByRole("button",{name:"卡片视图",exact:true}).click();
  await page
    .locator("button.drive-card")
    .filter({ hasText: "设计协作团队" })
    .click();
  await page.getByRole("link", { name: "设置", exact: true }).click();
  await page.getByRole("button", { name: "恢复团队", exact: true }).click();
  await page.getByRole("button", { name: "确认变更", exact: true }).click();
  await expect(page.locator(".team-header")).toContainText("正常协作");
  await page.setViewportSize({ width: 390, height: 844 });
  await page.getByRole("link", { name: "概览", exact: true }).click();
  await expect(
    page.getByRole("link", { name: "概览", exact: true }),
  ).toHaveAttribute("aria-current", "page");
  await page.evaluate(() => scrollTo(0, 0));
  await page.screenshot({
    path: path.join(artifacts, "team-overview-mobile.png"),
    fullPage: true,
  });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
});

test("new account activation, tenant invitation and team membership use real backend", async ({
  page,
  browser,
}) => {
  test.skip(
    process.env.DRIVEBRIDGE_TEST_TEAM_DEMO !== "1",
    "Requires explicit team demo",
  );
  await page.goto("/");
  await page.getByRole("button", { name: "无需账号，进入本地体验" }).click();
  await openTeam(page);
  await page.getByRole("link", { name: "成员", exact: true }).click();
  await page.getByRole("button", { name: "邀请加入" }).click();
  await page.getByLabel("用户名称", { exact: true }).fill("协作新成员");
  await page.getByRole("button", { name: "保存并继续" }).click();
  const secret = await page.locator("textarea.token-value").inputValue();
  await expect(
    page.getByRole("status").filter({ hasText: "账号及邀请已准备好" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "我已交给本人" }).click();
  const other = await browser.newContext();
  const joiner = await other.newPage();
  try {
    await joiner.goto("/");
    await joiner.getByRole("button", { name: "激活或恢复账号" }).click();
    await joiner.getByRole("tab", {name:"激活新账号", exact:true}).click();
    await joiner.getByLabel("一次性激活码").fill(secret);
    await joiner
      .getByRole("button", { name: "激活新账号", exact: true })
      .click();
    await expect(joiner.getByRole("status")).toContainText("账号激活完成");
    const key = await joiner.locator(".one-time-secret").innerText();
    await joiner.getByRole("button", { name: "关闭", exact: true }).click();
    await joiner.locator("#login-key").fill(key);
    await joiner.getByRole("button", { name: "登录", exact: true }).click();
    await expect(
      joiner.getByRole("button", { name: "团队", exact: true }),
    ).toBeVisible();
    await joiner.getByRole("button", { name: "团队", exact: true }).click();
    await joiner
      .getByRole("button", { name: "接受租户邀请", exact: true })
      .click();
    await expect(
      joiner.getByRole("button", { name: "接受团队邀请", exact: true }),
    ).toBeVisible();
    await joiner
      .getByRole("button", { name: "接受团队邀请", exact: true })
      .click();
    await joiner.getByRole("button",{name:"打开团队 设计协作团队",exact:true}).click();
    await joiner.getByRole("link", { name: "网盘", exact: true }).click();
    await expect(
      joiner.getByText("暂无可访问的团队网盘。加入团队不会自动获得文件权限。"),
    ).toBeVisible();
  } finally {
    await other.close();
  }
  await page.getByRole("button", { name: "刷新团队", exact: true }).click();
  await expect(
    page.getByRole("row").filter({ hasText: "协作新成员" }),
  ).toBeVisible();
});

test("create team and drive, protect assigned storage and require custodian handoff", async ({
  page,
}) => {
  test.skip(
    process.env.DRIVEBRIDGE_TEST_TEAM_DEMO !== "1",
    "Requires explicit team demo",
  );
  await page.goto("/");
  await page.getByRole("button", { name: "无需账号，进入本地体验" }).click();
  await page.getByRole("button", { name: "团队", exact: true }).click();
  await page.getByRole("button", { name: "创建团队", exact: true }).click();
  await page.getByLabel("团队名称", { exact: true }).fill("摄影协作组");
  let picker = page.getByRole("combobox", {name:"团队管理员", exact:true});
  await selectChoice(picker,{labelContains:"素材负责人"});
  await page.getByRole("button", { name: "保存并继续" }).click();
  await expect(page.locator(".team-header")).toContainText("摄影协作组");
  await page.getByRole("link", { name: "网盘", exact: true }).click();
  await page.getByRole("button", { name: "创建团队网盘", exact: true }).click();
  await page.getByLabel("网盘名称", { exact: true }).fill("交付素材");
  picker = page.getByRole("combobox", {name:"数据负责人", exact:true});
  await selectChoice(picker,{labelContains:"素材负责人"});
  await selectChoice(page.getByRole("combobox", {name:"存储连接", exact:true}),"platform");
  await page.getByRole("button", { name: "保存并继续" }).click();
  await expect(
    page.locator("button.drive-card").filter({ hasText: "交付素材" }),
  ).toBeVisible();
  await page.getByRole("link", { name: "设置", exact: true }).click();
  await page.getByRole("button", { name: "取消分配", exact: true }).click();
  await expect(page.getByRole("dialog")).toContainText("交付素材");
  await expect(
    page.getByRole("button", { name: "确认变更", exact: true }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "取消", exact: true }).click();
  await page.getByRole("link", { name: "权限", exact: true }).click();
  await selectChoice(page.getByLabel("素材负责人 在 交付素材 的权限"),"none");
  await expect(page.getByRole("dialog")).toContainText(
    "不能移除最后一位网盘数据负责人",
  );
  await expect(
    page.getByRole("button", { name: "确认变更", exact: true }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "取消", exact: true }).click();
  await page.getByRole("link", { name: "成员", exact: true }).click();
  await page.getByRole("button", { name: "邀请加入" }).click();
  await page.getByRole("option", { name: "团队成员", exact: true }).click();
  await page.getByRole("button", { name: "保存并继续" }).click();
  await expect(
    page.getByRole("row").filter({ hasText: "团队成员" }),
  ).toBeVisible();
  await page.getByRole("link", { name: "权限", exact: true }).click();
  await selectChoice(page
    .getByLabel("团队成员 在 交付素材 的权限"),"custodian");
  await page.getByRole("button", { name: "确认变更", exact: true }).click();
  await expect(page.getByLabel("团队成员 在 交付素材 的权限")).toHaveAttribute("data-value",
    "custodian",
  );
  await selectChoice(page.getByLabel("素材负责人 在 交付素材 的权限"),"none");
  await page.getByRole("button", { name: "确认变更", exact: true }).click();
  await expect(page.getByLabel("素材负责人 在 交付素材 的权限")).toHaveAttribute("data-value",
    "none",
  );
});
