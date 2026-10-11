import { test, expect } from "@playwright/test";
import path from "node:path";
function pdf() {
  const objects = [
    "<< /Type /Catalog /Pages 2 0 R >>",
    "<< /Type /Pages /Kids [3 0 R 5 0 R] /Count 2 >>",
    "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 400] /Resources << /Font << /F1 4 0 R >> >> /Contents 6 0 R >>",
    "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
    "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 400] /Resources << /Font << /F1 4 0 R >> >> /Contents 6 0 R >>",
  ];
  const stream = "BT /F1 24 Tf 25 320 Td (DriveBridge PDF) Tj ET";
  objects.push(`<< /Length ${stream.length} >>\nstream\n${stream}\nendstream`);
  let out = "%PDF-1.4\n";
  const offsets = [0];
  objects.forEach((v, i) => {
    offsets.push(Buffer.byteLength(out));
    out += `${i + 1} 0 obj\n${v}\nendobj\n`;
  });
  const start = Buffer.byteLength(out);
  out +=
    `xref\n0 ${offsets.length}\n0000000000 65535 f \n` +
    offsets
      .slice(1)
      .map((o) => `${String(o).padStart(10, "0")} 00000 n \n`)
      .join("") +
    `trailer\n<< /Size ${offsets.length} /Root 1 0 R >>\nstartxref\n${start}\n%%EOF`;
  return Buffer.from(out);
}
test("PDF renders real pixels and text, changes pages and reports broken files", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1098, height: 963 });
  await page.goto("/");
  await page.getByRole("link", { name: "使用账号登录" }).click();
  await page.getByRole("link", { name: "以 Alice 登录" }).click();
  await page
    .getByRole("button", { name: /^打开网盘 / })
    .first()
    .click();
  await page.locator("input[type=file]").setInputFiles([
    { name: "reader.pdf", mimeType: "application/pdf", buffer: pdf() },
    {
      name: "broken.pdf",
      mimeType: "application/pdf",
      buffer: Buffer.from("%PDF-1.4\nbroken"),
    },
  ]);
  await page
    .getByRole("button", { name: "详情 reader.pdf", exact: true })
    .click();
  await expect(page.locator(".pdf-reader .textLayer")).toContainText(
    "DriveBridge PDF",
  );
  const ink = await page
    .locator(".pdf-reader canvas")
    .evaluate((el: HTMLCanvasElement) => {
      const d = el
        .getContext("2d")!
        .getImageData(0, 0, el.width, el.height).data;
      let dark = 0;
      for (let i = 0; i < d.length; i += 4)
        if (d[i] < 100 && d[i + 3] > 0) dark++;
      return dark;
    });
  expect(ink).toBeGreaterThan(50);
  await page.getByRole("button", { name: "下一页", exact: true }).click();
  await expect(page.locator(".reader-status")).toContainText("第 2 / 2 页");
  await page.getByRole("button", { name: "适应宽度", exact: true }).click();
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.locator(".pdf-reader canvas")).toBeVisible();
  await page.screenshot({
    path: path.join(
      process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS || "temp/browser-artifacts",
      "pdf-mobile.png",
    ),
  });
  await page.keyboard.press("Escape");
  await page
    .getByRole("button", { name: "详情 broken.pdf", exact: true })
    .click();
  await expect(page.locator(".pdf-reader [role=alert]")).toContainText("损坏");
});

test("PDF password retries, denied access and network recovery are explicit", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByRole("link", { name: "使用账号登录" }).click();
  await page.getByRole("link", { name: "以 Alice 登录" }).click();
  await page
    .getByRole("button", { name: /^打开网盘 / })
    .first()
    .click();
  await page
    .locator("input[type=file]")
    .setInputFiles("tests/fixtures/reader-encrypted.pdf");
  await page
    .getByRole("button", { name: "详情 reader-encrypted.pdf", exact: true })
    .click();
  await page.getByRole("textbox", { name: "PDF 密码" }).fill("wrong");
  await page.getByRole("button", { name: "解锁 PDF" }).click();
  await expect(page.locator(".reader-message")).toContainText("密码错误");
  await page.getByRole("textbox", { name: "PDF 密码" }).fill("preview-test");
  await page.getByRole("button", { name: "解锁 PDF" }).click();
  await expect(page.locator(".pdf-reader .textLayer")).toContainText(
    "Password-protected",
  );
  await page.keyboard.press("Escape");
  await page.route("**/api/v1/files/*/media?*", (route) =>
    route.fulfill({ status: 403, body: "Forbidden" }),
  );
  await page
    .getByRole("button", { name: "详情 reader-encrypted.pdf", exact: true })
    .click();
  await expect(page.locator(".pdf-reader [role=alert]")).toContainText("权限");
  await page.unroute("**/api/v1/files/*/media?*");
  await page.getByRole("button", { name: "重新加载 PDF" }).click();
  await expect(page.getByRole("textbox", { name: "PDF 密码" })).toBeVisible();
});

test("Chinese text and scanned pages render across viewport sizes", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByRole("link", { name: "使用账号登录" }).click();
  await page.getByRole("link", { name: "以 Alice 登录" }).click();
  await page
    .getByRole("button", { name: /^打开网盘 / })
    .first()
    .click();
  await page
    .locator("input[type=file]")
    .setInputFiles("tests/fixtures/reader-cjk-scan.pdf");
  await page
    .getByRole("button", { name: "详情 reader-cjk-scan.pdf", exact: true })
    .click();
  await expect(page.locator(".pdf-reader .textLayer")).toContainText(
    "中文测试",
  );
  for (const width of [1440, 1248, 1098, 882, 390]) {
    await page.setViewportSize({ width, height: 963 });
    if (
      width < 768 &&
      (await page.getByRole("button", { name: "收起信息栏" }).count())
    )
      await page.getByRole("button", { name: "收起信息栏" }).click();
    await expect(page.locator(".reader-status")).toContainText("第 1 / 2 页");
    await page.screenshot({
      path: path.join(
        process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS || "temp/browser-artifacts",
        `pdf-cjk-${width}.png`,
      ),
    });
  }
  await page.getByRole("button", { name: "下一页", exact: true }).click();
  await expect(page.locator(".reader-status")).toContainText("第 2 / 2 页");
  expect(await page.locator(".pdf-page").count()).toBe(1);
  const ink = await page
    .locator(".pdf-reader canvas")
    .evaluate((el: HTMLCanvasElement) => {
      const data = el
        .getContext("2d")!
        .getImageData(0, 0, el.width, el.height).data;
      return Array.from(data).some((v, i) => i % 4 === 0 && v < 100);
    });
  expect(ink).toBe(true);
});

test("long PDF uses one page surface without Range and stale loads cannot replace another file", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByRole("link", { name: "使用账号登录" }).click();
  await page.getByRole("link", { name: "以 Alice 登录" }).click();
  await page
    .getByRole("button", { name: /^打开网盘 / })
    .first()
    .click();
  await page
    .locator("input[type=file]")
    .setInputFiles("tests/fixtures/reader-many-pages.pdf");
  await page
    .getByRole("button", { name: "详情 reader-many-pages.pdf", exact: true })
    .waitFor();
  await page
    .locator("input[type=file]")
    .setInputFiles({
      name: "reader-fast.pdf",
      mimeType: "application/pdf",
      buffer: pdf(),
    });
  await page
    .getByRole("button", { name: "详情 reader-fast.pdf", exact: true })
    .waitFor();
  const original = await page.request.get("/api/v1/spaces");
  const space = (await original.json()).data[0];
  const listing = (
    await (await page.request.get("/api/v1/files?space_id=" + space.id)).json()
  ).data;
  const file = listing.find(
    (f: { path: string }) => f.path === "reader-many-pages.pdf",
  );
  const matcher = `**/api/v1/files/${file.id}/media?*`;
  await page.route(matcher, async (route) => {
    const response = await route.fetch({
      headers: { ...route.request().headers(), range: "" },
    });
    await route.fulfill({
      status: 200,
      body: await response.body(),
      headers: { "content-type": "application/pdf" },
    });
  });
  await page
    .getByRole("button", { name: "详情 reader-many-pages.pdf", exact: true })
    .click();
  await expect(page.locator(".reader-status")).toContainText("第 1 / 250 页");
  await page.getByRole("spinbutton", { name: "PDF 页码" }).fill("249");
  await expect(page.locator(".reader-status")).toContainText("第 249 / 250 页");
  await expect(page.locator(".pdf-page")).toHaveCount(1);
  await page.keyboard.press("Escape");
  await page.unroute(matcher);
  let started = false;
  await page.route(matcher, async (route) => {
    started = true;
    await new Promise((r) => setTimeout(r, 1200));
    await route.abort().catch(() => {});
  });
  await page
    .getByRole("button", { name: "详情 reader-many-pages.pdf", exact: true })
    .click();
  await expect.poll(() => started).toBe(true);
  await page.keyboard.press("Escape");
  await page
    .getByRole("button", { name: "详情 reader-fast.pdf", exact: true })
    .click();
  await expect(page.locator(".pdf-reader .textLayer")).toContainText(
    "DriveBridge PDF",
  );
  await page.waitForTimeout(1400);
  await expect(page.locator(".pdf-reader")).toHaveAttribute(
    "aria-label",
    "reader-fast.pdf PDF 阅读器",
  );
  await expect(page.locator(".pdf-reader [role=alert]")).toHaveCount(0);
});
