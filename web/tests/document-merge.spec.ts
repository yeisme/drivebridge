import { test, expect } from "@playwright/test";
import type { Page } from "@playwright/test";

const documentWrite = (url: string) => /\/api\/v1\/files\/[^/]+\/document$/.test(new URL(url).pathname);
async function openNote(page: Page, spaceName: string) {
  await page.goto("/#/knowledge");
  await page.getByRole("combobox", { name: "知识库文件来源" }).click();
  await page.getByRole("option", { name: spaceName, exact: true }).click();
  const notebook = page.getByLabel("笔记工作台", { exact: true });
  await notebook.getByRole("button", { name: "快速打开", exact: true }).click();
  await page.getByRole("dialog", { name: "快速打开", exact: true }).getByRole("button", { name: "merge.md", exact: true }).click();
  await expect(notebook.getByRole("button", { name: "源码", exact: true })).toBeVisible();
  await notebook.getByRole("button", { name: "源码", exact: true }).click();
  return notebook;
}
async function save(page: Page, body: string) {
  const notebook = page.getByLabel("笔记工作台", { exact: true });
  await notebook.getByRole("textbox", { name: "Markdown 源码" }).fill(body);
  await notebook.getByRole("button", { name: "预览保存", exact: true }).click();
  const response = page.waitForResponse(response => response.request().method() === "PUT" && documentWrite(response.url()));
  await page.getByRole("button", { name: "确认保存新版本", exact: true }).click();
  return response;
}
async function mergeAfterLookup(page: Page, key: string) {
  const lookup = page.waitForResponse(response => new URL(response.url()).pathname.endsWith("/document/lookup"));
  await page.getByRole("button", { name: "查询原保存结果", exact: true }).click();
  const response = await lookup;
  expect(response.status()).toBe(200);
  expect(response.request().postDataJSON()).toEqual({ idempotency_key: key });
  const record = (await response.json()).data;
  expect(!record.found || record.save.status === "cancelled").toBe(true);
  await page.getByRole("button", { name: "读取当前版本并人工合并", exact: true }).click();
  return page.getByRole("dialog", { name: "人工合并冲突", exact: true });
}
async function confirmMerge(page: Page, body: string) {
  await page.getByRole("textbox", { name: "人工合并正文" }).fill(body);
  await page.getByRole("button", { name: "预览合并差异" }).click();
  const confirm = page.getByRole("button", { name: "确认新基线并保存新版本" });
  await expect(confirm).toBeDisabled();
  await page.getByRole("checkbox", { name: /我已审阅差异/ }).check();
  const response = page.waitForResponse(response => response.request().method() === "PUT" && documentWrite(response.url()));
  await confirm.click();
  return response;
}

test("two real tabs manually rebase pinned versions, retain repeated-conflict drafts, and preserve every published body", async ({ page, context }) => {
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  await page.goto("/");
  await page.getByRole("link", { name: "使用账号登录" }).click();
  await page.getByRole("link", { name: "以 Alice 登录" }).click();
  const spaceName = "Manual merge integration";
  await page.evaluate(async name => {
    const me = (await (await fetch("/api/v1/me")).json()).data;
    const response = await fetch("/api/v1/spaces", { method: "POST", headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrf_token }, body: JSON.stringify({ name, connection_id: "platform" }) });
    if (!response.ok) throw new Error("Fixture space creation failed");
  }, spaceName);
  await page.reload();
  await page.getByRole("button", { name: `打开网盘 ${spaceName}`, exact: true }).click();
  const chooserPromise = page.waitForEvent("filechooser");
  await page.getByRole("button", { name: "上传文件", exact: true }).click();
  await (await chooserPromise).setFiles({ name: "merge.md", mimeType: "text/markdown", buffer: Buffer.from("# Original\n\nOriginal retained body.") });
  await expect(page.getByRole("button", { name: "详情 merge.md", exact: true })).toBeVisible();
  const notebook = await openNote(page, spaceName);
  const other = await context.newPage();
  other.on("pageerror", error => errors.push(error.message));
  await openNote(other, spaceName);
  const remoteBody = "# Remote\n\nOther tab retained body.";
  const remoteResponse = await save(other, remoteBody);
  expect(remoteResponse.status()).toBe(200);
  const remote = (await remoteResponse.json()).data;
  const baseVersion = remoteResponse.request().postDataJSON().base_version_id;
  const localBody = "# Local\n\nUnsaved local secret.";
  const conflictResponse = await save(page, localBody);
  expect(conflictResponse.status()).toBe(409);
  const originalKey = conflictResponse.request().headers()["idempotency-key"];
  await expect(notebook.getByText(/文件版本已变化，草稿保留/)).toBeVisible();
  await expect(page.getByRole("button", { name: "读取当前版本并人工合并" })).toHaveCount(0);
  // CodeMirror renders each logical line separately; textContent omits their
  // separators. Reconstruct all lines, including empty ones, without trimming.
  await expect.poll(async () => (await notebook.getByRole("textbox", { name: "Markdown 源码" }).locator(".cm-line").allTextContents()).join("\n")).toBe(localBody);
  const dialog = await mergeAfterLookup(page, originalKey);
  await expect(dialog).toContainText(baseVersion);
  await expect(dialog).toContainText(remote.version_id);
  await expect(dialog).toContainText("Original retained body.");
  await expect(dialog).toContainText("Other tab retained body.");
  await expect(dialog.getByRole("textbox", { name: "人工合并正文" })).toHaveValue(localBody);
  await expect(notebook.getByLabel("未保存")).toBeVisible();
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  }
  // Move head after the dialog pinned its remote revision: CAS must reject,
  // rather than silently rebasing or replacing the user's merge.
  await expect(other.getByRole("heading", { name: "Remote", exact: true })).toBeVisible();
  await other.getByRole("button", { name: "源码", exact: true }).click();
  const newerBody = "# Newer\n\nThird retained body.";
  const newerResponse = await save(other, newerBody);
  expect(newerResponse.status()).toBe(200);
  const newer = (await newerResponse.json()).data;
  const firstMerge = "# Combined\n\nOther tab retained body.\nUnsaved local secret.";
  const repeated = await confirmMerge(page, firstMerge);
  expect(repeated.status()).toBe(409);
  expect(repeated.request().postDataJSON().base_version_id).toBe(remote.version_id);
  const mergeKey = repeated.request().headers()["idempotency-key"];
  expect(mergeKey).not.toBe(originalKey);
  await expect.poll(async () => (await notebook.getByRole("textbox", { name: "Markdown 源码" }).locator(".cm-line").allTextContents()).join("\n")).toBe(firstMerge);
  await expect(page.getByRole("button", { name: "读取当前版本并人工合并" })).toHaveCount(0);
  const secondDialog = await mergeAfterLookup(page, mergeKey);
  await expect(secondDialog).toContainText(newer.version_id);
  await expect(secondDialog.getByRole("textbox", { name: "人工合并正文" })).toHaveValue(firstMerge);
  const finalBody = firstMerge + "\nThird retained body.";
  const published = await confirmMerge(page, finalBody);
  expect(published.status()).toBe(200);
  expect(published.request().postDataJSON()).toEqual({ base_version_id: newer.version_id, text: finalBody });
  expect(published.request().headers()["idempotency-key"]).not.toBe(mergeKey);
  const saved = (await published.json()).data;
  await expect(notebook.getByRole("heading", { name: "Combined", exact: true })).toBeVisible();
  await expect(notebook.getByLabel("未保存")).toHaveCount(0);
  const bodies = await page.evaluate(async ({ file, versions }) => {
    const bodies: string[] = [];
    for (const version of versions) {
      const response = await fetch(`/api/v1/files/${file}/content?version=${version}`);
      if (!response.ok) throw new Error("Published historical version is unavailable");
      bodies.push(await response.text());
    }
    return { bodies, storage: Object.values(localStorage).join("\n") };
  }, { file: saved.file_id, versions: [baseVersion, remote.version_id, newer.version_id, saved.version_id] });
  expect(bodies.bodies).toEqual(["# Original\n\nOriginal retained body.", remoteBody, newerBody, finalBody]);
  expect(bodies.storage).not.toContain("Unsaved local secret");
  expect(bodies.storage).not.toContain("retained body");
  await other.close();
  expect(errors).toEqual([]);
});
