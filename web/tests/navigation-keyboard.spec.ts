import { test, expect } from "@playwright/test";

test("navigation shortcut yields to editors and selection and uses the mobile drawer", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto("/");
  await page.locator("#login-key").fill("dbu_" + "2".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(page.locator(".app")).toBeVisible();
  const me = (await (await page.request.get("/api/v1/me")).json()).data;
  const preferenceKey = `drivebridge:sidebar:${encodeURIComponent(me.instance_id)}:${encodeURIComponent(me.user.id)}`;
  await page.evaluate(() => {
    const editor = document.createElement("div");
    editor.id = "fixture-editor"; editor.contentEditable = "true"; editor.textContent = "editable text";
    document.body.append(editor); editor.focus();
  });
  await page.keyboard.press("Control+b");
  await expect(page.locator(".app")).not.toHaveClass(/sidebar-collapsed/);
  expect(await page.evaluate(key => localStorage.getItem(key), preferenceKey)).toBeNull();
  await page.evaluate(() => {
    document.getElementById("fixture-editor")!.remove();
    const text = document.createElement("p"); text.id = "fixture-selection"; text.textContent = "selected text";
    document.body.append(text);
    const range = document.createRange(); range.selectNodeContents(text);
    getSelection()!.removeAllRanges(); getSelection()!.addRange(range);
  });
  await page.keyboard.press("Control+b");
  await expect(page.locator(".app")).not.toHaveClass(/sidebar-collapsed/);
  await page.evaluate(() => { getSelection()!.removeAllRanges(); document.getElementById("fixture-selection")!.remove(); });
  await page.keyboard.press("Control+b");
  await expect(page.locator(".app")).toHaveClass(/sidebar-collapsed/);
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByRole("navigation", { name: "手机主导航" })).toBeVisible();
  await page.keyboard.press("Control+b");
  await expect(page.getByRole("dialog", { name: "全部导航" })).toBeVisible();
  expect(await page.evaluate(key => localStorage.getItem(key), preferenceKey)).toBe("true");
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog", { name: "全部导航" })).toHaveCount(0);
});
