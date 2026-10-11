import { test, expect } from "@playwright/test";
import path from "node:path";
import fs from "node:fs";
const artifacts =
  process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS ||
  path.resolve("temp/browser-artifacts");

async function openWorkspace(page: import("@playwright/test").Page) {
  await page.goto("/");
  await page.getByRole("link", { name: "使用账号登录" }).click();
  await page.getByRole("link", { name: "以 Alice 登录" }).click();
  await expect(
    page.getByRole("heading", { name: "我的网盘", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: /^打开网盘 / }).first().click();
  const folder = "预览-" + Date.now();
  await page.getByRole("button", { name: "新建", exact: true }).click();
  await page.getByRole("menuitem", { name: "新建文件夹", exact: true }).click();
  await page.getByRole("textbox", { name: "名称", exact: true }).fill(folder);
  await page.getByRole("button", { name: "确认", exact: true }).click();
  await expect(
    page.locator(".finder-folder").filter({ hasText: folder }),
  ).toBeVisible();
  await page.getByRole("button", { name: `打开文件夹 ${folder}` }).click();
  return folder;
}

test("图片查看器缩放、旋转、信息面板与标注导出", async ({ page }) => {
  const folder = await openWorkspace(page);
  const encoded = await page.evaluate(() => {
    const canvas = document.createElement("canvas");
    canvas.width = 960;
    canvas.height = 640;
    const ctx = canvas.getContext("2d")!;
    ctx.fillStyle = "#20494c";
    ctx.fillRect(0, 0, 960, 640);
    ctx.fillStyle = "#d9efb7";
    ctx.font = "48px sans-serif";
    ctx.fillText("DriveBridge 预览", 60, 120);
    return canvas.toDataURL("image/png").split(",")[1];
  });
  const name = "查看器样例.png";
  await page.locator("input[type=file]").setInputFiles({
    name,
    mimeType: "image/png",
    buffer: Buffer.from(encoded, "base64"),
  });
  await page
    .getByRole("button", { name: `详情 ${folder}/${name}`, exact: true })
    .click();
  const stage = page.locator(".viewer-stage.image");
  await expect(stage).toBeVisible();
  await expect(stage.locator("img")).toHaveJSProperty("naturalWidth", 960);

  // 适应窗口（960px 图在小舞台内缩放）→ 滚轮放大改变百分比
  const percent = stage.getByRole("button", { name: "适应窗口" });
  await expect(percent).toContainText("%");
  const before = await percent.textContent();
  await stage.hover({ position: { x: 320, y: 200 } });
  await page.mouse.wheel(0, -240);
  await expect(percent).not.toHaveText(before!);
  await percent.click();
  await expect(percent).toHaveText(before!);

  // 旋转改变图层变换并重置缩放
  const layer = stage.locator(".viewer-layer");
  const transformBefore = await layer.evaluate((el) => getComputedStyle(el).transform);
  await stage.getByRole("button", { name: "向右旋转" }).click();
  const transformAfter = await layer.evaluate((el) => getComputedStyle(el).transform);
  expect(transformAfter).not.toBe(transformBefore);
  await stage.getByRole("button", { name: "向左旋转" }).click();

  // 信息面板
  await stage.getByRole("button", { name: "文件信息" }).click();
  const info = page.locator(".viewer-info");
  await expect(info).toBeVisible();
  await expect(info).toContainText("960 × 640 px");
  await expect(info).toContainText("image/png");
  await info.getByRole("button", { name: "收起信息" }).click();
  await expect(info).toHaveCount(0);

  // 标注：画笔拖拽（避开底部工具条）→ 撤销可用 → 导出下载
  await stage.getByRole("button", { name: "标注", exact: true }).click();
  await stage.getByRole("button", { name: "画笔", exact: true }).click();
  const undo = stage.getByRole("button", { name: "撤销笔画" });
  await expect(undo).toBeDisabled();
  await page.mouse.move(300, 180);
  await page.mouse.down();
  await page.mouse.move(430, 220, { steps: 6 });
  await page.mouse.move(500, 190, { steps: 6 });
  await page.mouse.up();
  await expect(undo).toBeEnabled();
  const exportPromise = page.waitForEvent("download");
  await stage.getByRole("button", { name: "导出标注图" }).click();
  const download = await exportPromise;
  expect(download.suggestedFilename()).toContain("标注");

  await stage.getByRole("button", { name: "文字", exact: true }).click();
  await stage.getByRole("textbox", { name: "标注文字" }).fill("请调整这里");
  await stage.click({ position: { x: 210, y: 160 } });
  const pixels = () => stage.locator("canvas.viewer-canvas").evaluate((canvas: HTMLCanvasElement) => {
    const data = canvas.getContext("2d")!.getImageData(0, 0, canvas.width, canvas.height).data;
    let count = 0;
    for (let i = 3; i < data.length; i += 4) if (data[i]) count++;
    return count;
  });
  const withText = await pixels();
  await undo.click();
  await expect.poll(pixels).toBeLessThan(withText);
  await stage.getByRole("button", { name: "重做标注" }).click();
  await expect.poll(pixels).toBe(withText);
  await stage.getByRole("button", { name: "椭圆", exact: true }).click();
  const imageBox = (await stage.boundingBox())!;
  await page.mouse.move(imageBox.x + 300, imageBox.y + 200);
  await page.mouse.down();
  await page.mouse.move(imageBox.x + 410, imageBox.y + 270, { steps: 5 });
  await page.mouse.up();
  await expect.poll(pixels).toBeGreaterThan(withText);
  const beforeClear = await pixels();
  await stage.getByRole("button", { name: "清空标注" }).click();
  await expect.poll(pixels).toBe(0);
  await undo.click();
  await expect.poll(pixels).toBe(beforeClear);

  fs.mkdirSync(artifacts, { recursive: true });
  await page
    .locator(".quicklook-dialog")
    .screenshot({ path: path.join(artifacts, "preview-viewer-image.png") });
  await page.keyboard.press("Escape");
  await expect(page.locator(".quicklook-dialog")).toHaveCount(0);
});

test("视频查看器控制条与文本扩展名识别", async ({ page }) => {
  const folder = await openWorkspace(page);
  const webm = path.join(artifacts, "viewer-sample.webm");
  fs.mkdirSync(artifacts, { recursive: true });
  const encoded = await page.evaluate(async () => {
    const canvas = document.createElement("canvas");
    canvas.width = 320;
    canvas.height = 240;
    const ctx = canvas.getContext("2d")!;
    const stream = canvas.captureStream();
    const recorder = new MediaRecorder(stream, { mimeType: "video/webm" });
    const chunks: Blob[] = [];
    recorder.ondataavailable = (event) => chunks.push(event.data);
    const stopped = new Promise((resolve) => (recorder.onstop = resolve));
    recorder.start();
    ctx.fillStyle = "#35586c";
    ctx.fillRect(0, 0, 320, 240);
    await new Promise((resolve) => setTimeout(resolve, 300));
    ctx.fillStyle = "#d9efb7";
    ctx.fillRect(0, 0, 320, 240);
    await new Promise((resolve) => setTimeout(resolve, 300));
    recorder.stop();
    await stopped;
    const blob = new Blob(chunks, { type: "video/webm" });
    const bytes = new Uint8Array(await blob.arrayBuffer());
    let binary = "";
    for (const byte of bytes) binary += String.fromCharCode(byte);
    return btoa(binary);
  });
  fs.writeFileSync(webm, Buffer.from(encoded, "base64"));
  const name = "查看器片段.webm";
  await page
    .locator("input[type=file]")
    .setInputFiles({
      name,
      mimeType: "video/webm",
      buffer: fs.readFileSync(webm),
    });
  await page
    .getByRole("button", { name: `详情 ${folder}/${name}`, exact: true })
    .click();
  const stage = page.locator(".viewer-stage.video");
  await expect(stage).toBeVisible();
  await expect(stage.getByRole("button", { name: "截取当前帧" })).toBeVisible();
  await stage.getByRole("button", { name: "循环播放" }).click();
  await expect(stage.getByRole("button", { name: "循环播放" })).toHaveClass(
    /active/,
  );
  await expect(stage.locator("video")).toHaveJSProperty("videoWidth", 320);
  fs.mkdirSync(artifacts, { recursive: true });
  await page
    .locator(".quicklook-dialog")
    .screenshot({ path: path.join(artifacts, "preview-viewer-video.png") });
  await page.keyboard.press("Escape");
  await expect(page.locator(".quicklook-dialog")).toHaveCount(0);

  // 无 MIME 的 Markdown 按文本预览并统计行数
  const mdName = "说明.md";
  await page.locator("input[type=file]").setInputFiles({
    name: mdName,
    mimeType: "application/octet-stream",
    buffer: Buffer.from("# 标题\n\n正文一段。", "utf8"),
  });
  await page
    .getByRole("button", { name: `详情 ${folder}/${mdName}`, exact: true })
    .click();
  await expect(page.locator(".text-preview")).toContainText("# 标题");
  await expect(page.locator(".text-stats")).toContainText("3 行");
  await page.keyboard.press("Escape");
  await expect(page.locator(".quicklook-dialog")).toHaveCount(0);
});

test("HTML 沙箱渲染与源码切换、SVG 适应舞台", async ({ page }) => {
  const folder = await openWorkspace(page);
  const html = [
    "<!DOCTYPE html><html><head><style>",
    "h1 { color: #20494c; }",
    "</style></head><body>",
    "<h1>DriveBridge HTML 预览</h1>",
    "<script>window.__pwned = true;</script>",
    "</body></html>",
  ].join("");
  const htmlName = "报告页.html";
  await page.locator("input[type=file]").setInputFiles({
    name: htmlName,
    mimeType: "text/plain",
    buffer: Buffer.from(html, "utf8"),
  });
  await page
    .getByRole("button", { name: `详情 ${folder}/${htmlName}`, exact: true })
    .click();

  // 沙箱 iframe 渲染：脚本被阻止、静态内容与内联样式可见
  const frame = page.locator(".html-stage iframe");
  await expect(frame).toBeVisible();
  await expect(frame).toHaveAttribute("sandbox", "");
  await expect(frame).toHaveAttribute("referrerpolicy", "no-referrer");
  const inside = page.frameLocator(".html-stage iframe");
  const heading = inside.locator("h1");
  await expect(heading).toHaveText("DriveBridge HTML 预览");
  expect(
    await heading.evaluate((el) => getComputedStyle(el).color),
  ).toBe("rgb(32, 73, 76)");
  const content = await (await frame.elementHandle())!.contentFrame();
  expect(
    await content!.evaluate(
      () => (window as unknown as { __pwned?: boolean }).__pwned,
    ),
  ).toBeUndefined();

  // 源码视图：文本内容与行数统计
  await page.getByRole("button", { name: "源码", exact: true }).click();
  await expect(page.locator(".html-stage .text-preview")).toContainText(
    "DriveBridge HTML 预览",
  );
  await expect(page.locator(".html-stage .text-stats")).toContainText("行");
  await page.getByRole("button", { name: "安全预览", exact: true }).click();
  await expect(frame).toBeVisible();
  fs.mkdirSync(artifacts, { recursive: true });
  await page
    .locator(".quicklook-dialog")
    .screenshot({ path: path.join(artifacts, "preview-viewer-html.png") });
  await page.keyboard.press("Escape");
  await expect(page.locator(".quicklook-dialog")).toHaveCount(0);

  // viewBox-only SVG 走适应舞台（不依赖固有尺寸）
  const svgName = "矢量图.svg";
  await page.locator("input[type=file]").setInputFiles({
    name: svgName,
    mimeType: "text/plain",
    buffer: Buffer.from(
      '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100"><circle cx="60" cy="50" r="40" fill="#20494c"/></svg>',
      "utf8",
    ),
  });
  await page
    .getByRole("button", { name: `详情 ${folder}/${svgName}`, exact: true })
    .click();
  const vector = page.locator(".vector-stage img");
  await expect(vector).toBeVisible();
  const box = await vector.boundingBox();
  expect(box && box.width > 100).toBe(true);
  await page
    .locator(".quicklook-dialog")
    .screenshot({ path: path.join(artifacts, "preview-viewer-svg.png") });
});
