import { test, expect } from "@playwright/test";
import path from "node:path";
const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS || "temp/browser-artifacts";

test("ordinary Markdown notebook opens fixed content with nested navigation and narrow panes", async ({ page }, testInfo) => {
  const spaceName = `Notebook browser fixture ${testInfo.repeatEachIndex}`;
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto("/");
  await page.getByRole("link", { name: "使用账号登录" }).click();
  await page.getByRole("link", { name: "以 Alice 登录" }).click();
  const space = await page.evaluate(async name => {
    const me = (await (await fetch("/api/v1/me")).json()).data;
    const response = await fetch("/api/v1/spaces", { method: "POST", headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrf_token }, body: JSON.stringify({ name, connection_id: "platform" }) });
    if (!response.ok) throw new Error("Fixture space failed");
    return (await response.json()).data.id as string;
  }, spaceName);
  await page.reload();
  await page.getByRole("button", { name: `打开网盘 ${spaceName}`, exact: true }).click();
  // Use the visible, route-owned entry; the global hidden input also exists on DriveHome.
  const chooserPromise = page.waitForEvent('filechooser');
  await page.getByRole('button', { name: '上传文件', exact: true }).click();
  const chooser = await chooserPromise;
  const [uploadRequest] = await Promise.all([
    page.waitForRequest(request => new URL(request.url()).pathname === '/api/v1/uploads' && request.method() === 'POST', { timeout: 10_000 }),
    chooser.setFiles({ name: "notebook.md", mimeType: "text/markdown", buffer: Buffer.from("# 固定版本笔记\n\n正文用于真实文件读取验证。 ^body-block\n\n[[#固定版本笔记|返回标题]] [[#^body-block|定位正文]]\n\n![blocked](https://notebook-tracker.invalid/image.png)") }),
  ]);
  expect(uploadRequest.postDataJSON().space_id, 'Upload must use the newly selected space').toBe(space);
  await expect(page.getByRole("button", { name: "详情 notebook.md", exact: true })).toBeVisible();
  const siblingChooser = page.waitForEvent("filechooser");
  await page.getByRole("button", { name: "上传文件", exact: true }).click();
  await (await siblingChooser).setFiles({ name: "z-sibling.md", mimeType: "text/markdown", buffer: Buffer.from("# Another preview sibling") });
  await expect(page.getByRole("button", { name: "详情 z-sibling.md", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "详情 notebook.md", exact: true }).click();
  await page.getByRole("button", { name: "笔记工作台", exact: true }).click();
  const workspace = page.getByLabel("笔记工作台", { exact: true });
  await expect(workspace.getByRole("heading", { name: "固定版本笔记" })).toBeVisible();
  await expect(workspace.locator("img")).toHaveCount(0);
  await workspace.getByRole("tab", { name: "notebook.md", exact: true }).focus();
  await page.keyboard.press("ArrowRight");
  await expect(workspace.getByRole("heading", { name: "固定版本笔记" })).toBeVisible();
  await expect(page.getByRole("dialog", { name: "notebook.md", exact: true })).toBeVisible();
  const originalRoute = page.url();
  await workspace.getByRole("button", { name: "定位正文", exact: true }).click();
  await expect(workspace.locator('[data-notebook-anchor="^body-block"]')).toBeFocused();
  await workspace.getByRole("button", { name: "返回标题", exact: true }).click();
  await expect(workspace.getByRole("heading", { name: "固定版本笔记" })).toBeFocused();
  expect(page.url()).toBe(originalRoute);
  await page.getByRole("button", { name: "收起信息栏", exact: true }).click();
  await expect(workspace.getByRole("complementary", { name: "笔记目录侧栏" })).toBeVisible();
  const section = workspace.getByRole("region").locator("details").first();
  await section.locator("summary").click();
  await expect(section).not.toHaveAttribute("open", "");
  await expect(section.getByText("正文用于真实文件读取验证。", { exact: true })).toBeHidden();
  await workspace.getByRole("button", { name: "文档大纲", exact: true }).click();
  await workspace.getByRole("navigation", { name: "文档大纲" }).getByRole("button", { name: "固定版本笔记", exact: true }).click();
  await expect(section).toHaveAttribute("open", "");
  await expect(section.getByText("正文用于真实文件读取验证。", { exact: true })).toBeVisible();
  await workspace.getByRole("button", { name: "快速打开", exact: true }).click();
  await expect(page.getByRole("dialog", { name: "快速打开", exact: true }).getByRole("button", { name: "notebook.md", exact: true })).toBeVisible();
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await page.screenshot({ path: path.join(artifacts, `notebook-quick-open-${width}.png`), animations: "disabled" });
  }
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog", { name: "快速打开", exact: true })).toHaveCount(0);
  await expect(workspace).toBeVisible();
  // ResizeObserver owns the inner workspace breakpoint. Wait for its commit
  // before toggling, rather than using an aria state from the previous width.
  await expect(workspace.getByRole("complementary", { name: "笔记信息侧栏", exact: true })).toBeVisible();
  await expect(page.getByRole("region", { name: "笔记关系能力", exact: true }).getByRole("button", { name: "重建此版本关系索引", exact: true })).toBeVisible();
  await page.screenshot({ path: path.join(artifacts, "notebook-relations-1440.png"), animations: "disabled" });
  await workspace.getByRole("button", { name: "笔记信息", exact: true }).click();
  await page.setViewportSize({ width: 390, height: 900 });
  await expect(workspace.getByRole("complementary", { name: "笔记目录侧栏", exact: true })).toHaveCount(0);
  await workspace.getByRole("button", { name: "笔记信息", exact: true }).click();
  const noteInformation = page.getByRole("dialog", { name: "笔记信息", exact: true });
  await expect(noteInformation.getByRole("region", { name: "笔记关系能力", exact: true }).getByRole("button", { name: "重建此版本关系索引", exact: true })).toBeVisible();
  await page.screenshot({ path: path.join(artifacts, "notebook-relations-390.png"), animations: "disabled" });
  await noteInformation.getByRole("button", { name: "关闭", exact: true }).click();
  await page.setViewportSize({ width: 1440, height: 1000 });
  await workspace.getByRole("button", { name: "切换分屏", exact: true }).click();
  await page.screenshot({ path: path.join(artifacts, "notebook-desktop.png") });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(workspace.getByRole("complementary", { name: "笔记目录侧栏" })).toHaveCount(0);
  await workspace.getByRole("button", { name: "笔记目录", exact: true }).click();
  const directory = page.getByRole("dialog", { name: "当前来源笔记", exact: true });
  await expect(directory.getByRole("button", { name: "notebook.md", exact: true })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(directory).toHaveCount(0);
  await expect(workspace.getByRole("button", { name: "副窗格", exact: true })).toBeVisible();
  await workspace.getByRole("button", { name: "副窗格", exact: true }).click();
  await expect(workspace.getByRole("heading", { name: "固定版本笔记" })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: path.join(artifacts, "notebook-mobile.png") });
  expect(errors).toEqual([]);
});
