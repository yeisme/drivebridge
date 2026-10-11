import { test, expect } from "@playwright/test";
import { selectChoice } from "./helpers/select";

test("recycle bin supports retention, selection and batch restore", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByRole("link", { name: "使用账号登录" }).click();
  await page.getByRole("link", { name: "以 Alice 登录" }).click();
  await expect(page.getByRole("heading", { name: "我的网盘", exact: true })).toBeVisible();
  const originalDrive = await page.getByRole("button", { name: /^打开网盘 / }).first().getAttribute("aria-label");
  const secondaryName = "Selection scope " + Date.now();
  const secondary = await page.evaluate(async name => {
    const me = (await (await fetch("/api/v1/me")).json()).data;
    const response = await fetch("/api/v1/spaces", { method: "POST", headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrf_token }, body: JSON.stringify({ name, connection_id: "platform" }) });
    if (!response.ok) throw new Error("Disposable space creation failed");
    return (await response.json()).data.id as string;
  }, secondaryName);
  await page.reload();
  await page.getByRole("button", { name: originalDrive!, exact: true }).click();
  const a = "batch-a-" + Date.now() + ".txt";
  const b = "batch-b-" + Date.now() + ".txt";
  await page.locator("input[type=file]").setInputFiles([
    { name: a, mimeType: "text/plain", buffer: Buffer.from("a") },
    { name: b, mimeType: "text/plain", buffer: Buffer.from("b") },
  ]);
  await expect(page.getByRole("button", { name: new RegExp(a) }).first()).toBeVisible();
  await page.getByRole("button", { name: new RegExp("详情 " + a) }).click();
  await page.getByRole("button", { name: "移入回收站" }).click();
  await page.getByRole("button", { name: new RegExp("详情 " + b) }).click();
  await page.getByRole("button", { name: "移入回收站" }).click();
  await page.getByRole("button", { name: "回收站", exact: true }).click();
  await expect(page.getByLabel("回收站自动清理天数")).toBeVisible();
  await expect(page.getByRole("button", { name: "永久删除 " + a, exact: true })).toBeDisabled();
  await page.getByRole("checkbox", { name: "全选回收站文件" }).check();
  await expect(page.getByRole("button", { name: "永久删除所选", exact: true })).toBeDisabled();
  const selector = page.getByRole("combobox", { name: "选择空间", exact: true });
  const originalSpace = await selector.getAttribute("data-value");
  await selectChoice(selector, secondary);
  await expect(page.getByRole("button", { name: "恢复所选", exact: true })).toBeDisabled();
  await expect(page.getByRole("button", { name: "恢复 " + a, exact: true })).toHaveCount(0);
  await selectChoice(selector, originalSpace!);
  await expect(page.getByRole("checkbox", { name: "选择 " + a, exact: true })).not.toBeChecked();
  await expect(page.getByRole("button", { name: "恢复所选", exact: true })).toBeDisabled();
  await page.getByRole("checkbox", { name: "全选回收站文件" }).check();
  await page.getByRole("button", { name: "恢复所选", exact: true }).click();
  await expect(page.getByRole("button", { name: "恢复 " + a, exact: true })).toHaveCount(0);
  await page.getByRole("button", { name: "我的网盘", exact: true }).click();
  await page.getByRole("button", { name: originalDrive!, exact: true }).click();
  await expect(page.getByRole("button", { name: new RegExp(a) }).first()).toBeVisible();
  await expect(page.getByRole("button", { name: new RegExp(b) }).first()).toBeVisible();
});
