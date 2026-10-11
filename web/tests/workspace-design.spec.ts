import { keyAction } from "./helpers/key-action";
import { test, expect } from "@playwright/test";
import path from "node:path";
const artifacts =
  process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS || "temp/browser-artifacts";
test("green workspace separates drive selection, supports folding and key identity", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto("/");
  await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "我的网盘", exact: true }),
  ).toBeVisible();
  // Create disposable drives through the real application contract.
  await page.evaluate(async () => {
    const me = (await (await fetch("/api/v1/me")).json()).data;
    for (const name of ["项目素材", "设计归档"]) {
      const r = await fetch("/api/v1/spaces", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-CSRF-Token": me.csrf_token,
        },
        body: JSON.stringify({ name, connection_id: "platform" }),
      });
      if (!r.ok) throw new Error("fixture space creation failed");
    }
  });
  await page.reload();
  await expect(
    page.getByRole("button", { name: "打开网盘 项目素材" }),
  ).toBeVisible();
  await expect(page.getByRole("region", { name: "文件浏览" })).toHaveCount(0);
  await page.screenshot({
    path: path.join(artifacts, "green-drive-home.png"),
  });
  await page.getByRole("button", { name: "打开网盘 项目素材" }).click();
  await expect(
    page.getByRole("complementary", { name: "目录树" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "折叠目录树" }).click();
  await expect(page.getByRole("complementary", { name: "目录树" })).toHaveCount(
    0,
  );
  await page.getByRole("button", { name: "展开目录树" }).click();
  await page.getByRole("button", { name: "折叠侧边栏", exact: true }).click();
  await expect(page.locator(".app")).toHaveClass(/sidebar-collapsed/);
  await expect(page.locator('.sidebar .nav-item[aria-label="资源态势"]')).toBeVisible();
  await page.screenshot({
    path: path.join(artifacts, "shell-collapsed-rail.png"),
  });
  await page.getByRole("button", { name: "展开侧边栏", exact: true }).click();
  await page.getByRole("button", { name: "Agent", exact: true }).click();
  await expect(page.getByRole("button", { name: "Agent", exact: true })).toHaveAttribute(
    "aria-expanded",
    "false",
  );
  await expect(page.getByRole("button", { name: "Agent 管理", exact: true })).toBeHidden();
  await page.getByRole("button", { name: "Agent", exact: true }).click();
  await expect(page.getByRole("button", { name: "Agent 管理", exact: true })).toBeVisible();
  await page.goto("/#/analytics");
  await expect(page.getByRole("heading", { name: "资源态势", exact: true })).toBeVisible();
  await expect(page.getByRole("tab", { name: "容量", exact: true })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await page.getByRole("button", { name: "资源态势", exact: true }).click();
  await expect(page.getByRole("heading", { name: "存储分布" })).toBeVisible();
  await page.screenshot({
    path: path.join(artifacts, "green-dashboard.png"),
  });
  await page.goto("/#/users");
  await expect(page.getByRole("heading", { name: "访问凭据", exact: true })).toBeVisible();
  await expect(page.getByRole("tab", { name: "Key", exact: true })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  const row = page.getByRole("row").filter({ hasText: "e2e-admin" });
  await expect(row).toContainText("当前登录");
  await keyAction(row,"修改名称");
  await page
    .getByLabel("Key 名称", { exact: true })
    .fill("设计工作台 · 浏览器");
  await page.getByRole("button", { name: "保存名称", exact: true }).click();
  await expect(
    page.getByRole("row").filter({ hasText: "设计工作台 · 浏览器" }),
  ).toContainText("e2e-admin");
  await page.getByRole("searchbox", { name: "搜索 Key" }).fill("设计工作台");
  await page.getByRole("button", { name: "详情", exact: true }).click();
  await expect(page.getByRole("dialog")).toContainText("所属用户");
  await expect(page.getByRole("dialog")).toContainText("e2e-admin");
  await page.getByRole("button", { name: "关闭", exact: true }).click();
  await page.screenshot({
    path: path.join(artifacts, "green-key-directory.png"),
  });
  await page.getByRole("tab", { name: "用户", exact: true }).click();
  await expect(
    page.getByRole("columnheader", { name: "所属团队" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "我的网盘", exact: true }).click();
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(
    page.getByRole("button", { name: "打开网盘 项目素材" }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: path.join(artifacts, "green-drive-mobile.png"),
  });
});
