import { test, expect } from "@playwright/test";
import path from "node:path";
const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS || "temp/browser-artifacts";

test("large preview, isolated interactive HTML, collapsible information and file search", async ({ page }) => {
  await page.setViewportSize({ width: 1395, height: 1040 });
  await page.goto("/");
  await page.getByRole("link", { name: "使用账号登录" }).click();
  await page.getByRole("link", { name: "以 Alice 登录" }).click();
  await page.evaluate(async () => {
    const me = (await (await fetch("/api/v1/me")).json()).data;
    const response = await fetch("/api/v1/spaces", {
      method:"POST",headers:{"Content-Type":"application/json","X-CSRF-Token":me.csrf_token},
      body:JSON.stringify({name:"Interactive preview fixture",connection_id:"platform"}),
    });
    if (!response.ok) throw new Error("Fixture space failed");
  });
  await page.reload();
  await page.getByRole("button", { name:"打开网盘 Interactive preview fixture",exact:true }).click();
  await expect(page.locator(".finder-list")).toHaveAttribute("aria-busy","false");
  const html = `<!doctype html><html><body><h1>交互图表</h1><button id="counter">0</button><script>
    let n=0;counter.onclick=()=>counter.textContent=String(++n);
    try { parent.document.body.dataset.escaped='yes'; } catch { document.body.dataset.isolated='yes'; }
    fetch('https://preview-blocked.invalid/leak').catch(()=>document.body.dataset.network='blocked');
    try { localStorage.setItem('preview','bad'); } catch { document.body.dataset.storage='blocked'; }
    </script></body></html>`;
  await page.locator("input[type=file]").setInputFiles([{ name: "interactive.html", mimeType: "text/html", buffer: Buffer.from(html) }, { name: "notes.txt", mimeType: "text/plain", buffer: Buffer.from("备注草稿") }]);
  await page.getByRole("button", { name: "详情 interactive.html", exact: true }).click();
  const iframe = page.locator(".html-stage iframe");
  await expect(iframe).toHaveAttribute("sandbox", "");
  const initial = (await iframe.boundingBox())!;
  expect(initial.height).toBeGreaterThan(600);
  await page.getByRole("button", { name: "收起信息栏" }).click();
  await expect(page.getByRole("complementary", { name: "预览文件列表与信息" })).toBeHidden();
  expect((await iframe.boundingBox())!.width).toBeGreaterThan(initial.width + 200);
  await page.getByRole("button", { name: "交互预览", exact: true }).click();
  await expect(iframe).toHaveAttribute("sandbox", "allow-scripts");
  const inside = page.frameLocator(".html-stage iframe");
  await inside.getByRole("button", { name: "0", exact: true }).click();
  await expect(inside.locator("#counter")).toHaveText("1");
  await expect(inside.locator("body")).toHaveAttribute("data-isolated", "yes");
  await expect(inside.locator("body")).toHaveAttribute("data-network", "blocked");
  await expect(inside.locator("body")).toHaveAttribute("data-storage", "blocked");
  expect(await page.locator("body").getAttribute("data-escaped")).toBeNull();
  await page.getByRole("button", { name: "重新运行预览" }).click();
  await expect(inside.locator("#counter")).toHaveText("0");
  await page.getByRole("button", { name: "源码", exact: true }).click();
  await expect(page.locator(".html-stage pre")).toContainText("交互图表");
  await page.getByRole("button", { name: "交互预览", exact: true }).click();
  await expect(inside.locator("#counter")).toHaveText("0");
  await page.screenshot({ path: path.join(artifacts, "preview-expanded-interactive.png") });
  await page.getByRole("button", { name: "展开信息栏" }).click();
  const previewCount = await page.locator(".quicklook-files > button").count();
  await page.getByRole("textbox", { name: "搜索预览文件" }).fill("notes");
  await expect(page.locator(".quicklook-files > button")).toHaveCount(1);
  await page.getByRole("button", { name: "定位当前文件" }).click();
  await expect(page.locator(".quicklook-files > button")).toHaveCount(previewCount);
  await page.setViewportSize({ width: 390, height: 844 });
  // Narrow mode starts with information closed, independently of desktop preference.
  await expect(page.getByRole("complementary", { name: "预览文件列表与信息" })).toBeHidden();
  await page.getByRole("button", { name: "展开信息栏", exact: true }).focus();
  expect((await iframe.boundingBox())!.height).toBeGreaterThan(400);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: path.join(artifacts, "preview-mobile-interactive.png") });
  await page.keyboard.press("Escape");
  await page.setViewportSize({ width: 1395, height: 1040 });
  await page.getByRole("button", { name: "Agent 管理", exact: true }).click();
  await page.getByRole("button", { name: "连接指南", exact: true }).click();
  await expect(page.locator(".robot-setup pre")).toContainText("<ROBOT_TOKEN>");
  await expect(page.locator(".robot-setup pre")).toContainText("/mcp");
  await page.getByRole("button", { name: "本机 CLI", exact: true }).click();
  await expect(page.locator(".robot-setup pre")).toContainText("drivebridge install --client generic --json");
  await page.getByRole("textbox", { name: "机器人连接服务地址" }).fill("javascript:alert(1)");
  await expect(page.getByRole("button", { name: "复制配置命令" })).toBeDisabled();
  await page.getByRole("textbox", { name: "机器人连接服务地址" }).fill("https://files.example.com");
  await page.screenshot({ path: path.join(artifacts, "robot-setup-guide.png") });
});
