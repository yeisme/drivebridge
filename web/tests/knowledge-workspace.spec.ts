import { test, expect } from "@playwright/test";
import path from "node:path";

test("knowledge route lists actual uploaded Markdown and opens the same fixed version", async ({ page, context }) => {
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto("/");
  await page.getByRole("link", { name: "使用账号登录" }).click();
  await page.getByRole("link", { name: "以 Alice 登录" }).click();
  const space = await page.evaluate(async () => {
    const me = (await (await fetch("/api/v1/me")).json()).data;
    const response = await fetch("/api/v1/spaces", { method: "POST", headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrf_token }, body: JSON.stringify({ name: "Knowledge integration", connection_id: "platform" }) });
    if (!response.ok) throw new Error("Fixture space creation failed");
    return (await response.json()).data.id as string;
  });
  await page.reload();
  await page.getByRole("button", { name: "打开网盘 Knowledge integration", exact: true }).click();
  const chooserPromise = page.waitForEvent("filechooser");
  await page.getByRole("button", { name: "上传文件", exact: true }).click();
  const chooser = await chooserPromise;
  const [uploadRequest] = await Promise.all([
    page.waitForRequest(request => new URL(request.url()).pathname === "/api/v1/uploads" && request.method() === "POST"),
    chooser.setFiles({ name: "knowledge.md", mimeType: "text/markdown", buffer: Buffer.from("# 真正的知识库\n\n来自已发布的固定文件版本，不是演示笔记。") }),
  ]);
  expect(uploadRequest.postDataJSON().space_id).toBe(space);
  await expect(page.getByRole("button", { name: "详情 knowledge.md", exact: true })).toBeVisible();
  await page.locator(".sidebar").getByRole("button", { name: "知识库", exact: true }).click();
  await expect(page).toHaveURL(/#\/knowledge/);
  await page.getByRole("combobox", { name: "知识库文件来源" }).click();
  await page.getByRole("option", { name: "Knowledge integration", exact: true }).click();
  await expect(page.getByText(/篇 Markdown/)).toBeVisible();
  const notebook = page.getByLabel("笔记工作台", { exact: true });
  await notebook.getByRole("button", { name: "快速打开", exact: true }).click();
  await page.getByRole("dialog", { name: "快速打开", exact: true }).getByRole("button", { name: "knowledge.md", exact: true }).click();
  await expect(notebook.getByRole("heading", { name: "真正的知识库" })).toBeVisible();
  await page.locator('section[aria-label="知识库"] > details > summary').click();
  await page.getByRole("textbox", { name: "筛选已加载笔记" }).fill("不会命中");
  await expect(notebook.getByRole("heading", { name: "真正的知识库" })).toBeVisible();
  await page.getByRole("textbox", { name: "筛选已加载笔记" }).fill("");
  await page.locator('section[aria-label="知识库"] > details > summary').click();
  await notebook.getByRole("button", { name: "源码", exact: true }).click();
  const source = notebook.getByRole("textbox", { name: "Markdown 源码" });
  await expect(source).not.toHaveAttribute("aria-readonly", "true");
  await source.fill("# 保存后的知识库\n\n新版本正文。");
  page.once("dialog", dialog => dialog.dismiss());
  await page.locator(".sidebar").getByRole("button", { name: "我的网盘", exact: true }).click();
  await expect(page).toHaveURL(/#\/knowledge/);
  await notebook.getByRole("button", { name: "预览保存", exact: true }).click();
  await expect(page.getByRole("dialog", { name: "确认文档修改" })).toContainText("新版本正文。");
  const publishedResponse = page.waitForResponse(response => response.request().method() === "PUT" && /\/files\/[^/]+\/document$/.test(new URL(response.url()).pathname));
  await page.getByRole("button", { name: "确认保存新版本", exact: true }).click();
  const response = await publishedResponse;
  expect(response.status()).toBe(200);
  const saved = (await response.json()).data as { file_id: string; version_id: string; operation_id: string };
  const original = response.request().postDataJSON() as { base_version_id: string; text: string };
  expect(saved.version_id).not.toBe(original.base_version_id);
  await expect(notebook.getByRole("heading", { name: "保存后的知识库" })).toBeVisible();
  await expect(notebook.getByLabel("未保存")).toHaveCount(0);
  await notebook.getByRole("button", { name: "切换分屏", exact: true }).click();
  const verification = await page.evaluate(async ({ saved, original, key }) => {
    const me = (await (await fetch("/api/v1/me")).json()).data;
    const headers = { "Content-Type": "application/json", "X-CSRF-Token": me.csrf_token, "Idempotency-Key": key };
    const old = await fetch(`/api/v1/files/${saved.file_id}/content?version=${original.base_version_id}`);
    const replay = await fetch(`/api/v1/files/${saved.file_id}/document`, { method: "PUT", headers, body: JSON.stringify(original) });
    const conflict = await fetch(`/api/v1/files/${saved.file_id}/document`, { method: "PUT", headers: { ...headers, "Idempotency-Key": "browser-stale-attempt" }, body: JSON.stringify({ ...original, text: "must not replace" }) });
    return { old: await old.text(), replayStatus: replay.status, replay: (await replay.json()).data, conflict: conflict.status };
  }, { saved, original, key: response.request().headers()["idempotency-key"] });
  expect(verification.old).toContain("来自已发布的固定文件版本");
  expect(verification.replayStatus).toBe(200);
  expect(verification.replay.operation_id).toBe(saved.operation_id);
  expect(verification.replay.version_id).toBe(saved.version_id);
  expect(verification.conflict).toBe(409);

  // Two real tabs edit the same fixed revision. The second commits first;
  // the stale first tab must preserve its draft rather than overwrite it.
  await notebook.getByRole("button", { name: "切换分屏", exact: true }).click();
  await notebook.getByRole("button", { name: "源码", exact: true }).click();
  await notebook.getByRole("textbox", { name: "Markdown 源码" }).fill("# 尚未保存的另一稿");
  const other = await context.newPage();
  await other.goto("/#/knowledge");
  await other.getByRole("combobox", { name: "知识库文件来源" }).click();
  await other.getByRole("option", { name: "Knowledge integration", exact: true }).click();
  const otherNotebook = other.getByLabel("笔记工作台", { exact: true });
  await otherNotebook.getByRole("button", { name: "快速打开", exact: true }).click();
  await other.getByRole("dialog", { name: "快速打开", exact: true }).getByRole("button", { name: "knowledge.md", exact: true }).click();
  await expect(otherNotebook.getByRole("heading", { name: "保存后的知识库" })).toBeVisible();
  await otherNotebook.getByRole("button", { name: "源码", exact: true }).click();
  await otherNotebook.getByRole("textbox", { name: "Markdown 源码" }).fill("# 另一个标签页的已保存版本");
  await otherNotebook.getByRole("button", { name: "预览保存", exact: true }).click();
  await other.getByRole("button", { name: "确认保存新版本", exact: true }).click();
  await expect(otherNotebook.getByRole("heading", { name: "另一个标签页的已保存版本" })).toBeVisible();
  await notebook.getByRole("button", { name: "预览保存", exact: true }).click();
  await page.getByRole("button", { name: "确认保存新版本", exact: true }).click();
  await expect(notebook.getByText(/文件版本已变化，草稿保留/)).toBeVisible();
  await expect(notebook.getByRole("textbox", { name: "Markdown 源码" })).toContainText("尚未保存的另一稿");
  await expect(notebook.getByRole("button", { name: "查询原保存结果" })).toBeVisible();
  await other.close();
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    if (process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS) await page.screenshot({ path: path.join(process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS, `knowledge-${width}.png`) });
  }
  page.once("dialog", dialog => dialog.accept());
  await page.reload();
  await expect(page.getByRole("heading", { name: "知识库", exact: true }).last()).toBeVisible();
  expect(errors).toEqual([]);
});
