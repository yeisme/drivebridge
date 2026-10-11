import { test, expect, type Page } from "@playwright/test";
import path from "node:path";
import { selectFinderView } from "./helpers/finder-view";

const baseline = process.env.DRIVEBRIDGE_DESIGN_BASELINE === "1";
const artifacts =
  process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS || "temp/browser-artifacts";
const sizes = [
  { width: 1440, height: 1000 },
  { width: 900, height: 900 },
  { width: 390, height: 844 },
];
const longName = "项目参考视频与素材归档需要完整显示的文件名称.mp4";

async function readableText(page: Page) {
  const ratios = await page
    .locator(
      ".finder-name strong, .finder-name small, .finder-file time, .finder-label, .finder-status, .finder-drop small, .finder-upload, .media-fallback > span",
    )
    .evaluateAll((nodes) => {
      const rgb = (value: string) => value.match(/[\d.]+/g)!.map(Number);
      const luminance = (color: number[]) =>
        color.slice(0, 3).reduce((sum, value, i) => {
          const c = value / 255;
          return (
            sum +
            (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4) *
              [0.2126, 0.7152, 0.0722][i]
          );
        }, 0);
      return nodes
        .filter((node) => (node as HTMLElement).offsetHeight > 0)
        .map((node) => {
          let parent: Element | null = node;
          let background = [255, 255, 255];
          while (parent) {
            const color = rgb(getComputedStyle(parent).backgroundColor);
            if (color.length === 3 || color[3] === 1) {
              background = color;
              break;
            }
            parent = parent.parentElement;
          }
          const fg = luminance(rgb(getComputedStyle(node).color)),
            bg = luminance(background);
          return {
            selector: node.className || node.tagName,
            ratio: (Math.max(fg, bg) + 0.05) / (Math.min(fg, bg) + 0.05),
          };
        });
    });
  for (const sample of ratios)
    expect(sample.ratio, sample.selector).toBeGreaterThanOrEqual(4.5);
}

async function capture(page: Page, entry: string) {
  for (const size of sizes) {
    await page.setViewportSize(size);
    const dismiss = page.getByRole("button", { name: "关闭通知", exact: true });
    if (await dismiss.isVisible()) await dismiss.click();
    await page.mouse.move(0, 0);
    if (!baseline) {
      await readableText(page);
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      ).toBe(true);
      const grid = page.locator(".finder-grid");
      if (size.width === 390) {
        expect(
          await grid.evaluate(
            (el) => getComputedStyle(el).gridTemplateColumns.split(" ").length,
          ),
        ).toBe(2);
      }
      const card = page.locator(".finder-grid .media-thumbnail").first();
      const box = await card.boundingBox();
      expect(box!.width / box!.height).toBeCloseTo(4 / 3, 1);
    }
    await page.screenshot({
      path: path.join(
        artifacts,
        `${baseline ? "before" : "after"}-${entry}-${size.width}.png`,
      ),
      fullPage: true,
    });
  }
}

test("shared drive design: hosted, focused and local layouts", async ({
  page,
  browser,
}) => {
  await page.setViewportSize(sizes[0]);
  await page.goto("/");
  await page.getByRole("link", { name: "使用账号登录" }).click();
  await page.getByRole("link", { name: "以 Alice 登录" }).click();
  // Each visual run owns its empty drive; other specs may have uploaded files.
  await page.evaluate(async () => {
    const me = (await (await fetch("/api/v1/me")).json()).data;
    const response = await fetch("/api/v1/spaces", {
      method:"POST", headers:{"Content-Type":"application/json","X-CSRF-Token":me.csrf_token},
      body:JSON.stringify({name:"Finder design fixture",connection_id:"platform"}),
    });
    if (!response.ok) throw new Error("Fixture space failed");
  });
  await page.reload();
  await page
    .getByRole("button", { name: "打开网盘 Finder design fixture",exact:true })
    .click();
  await page.getByRole("button", { name: "网格视图" }).click();
  await expect(page.getByText("把文件放到这里", { exact: true })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "上传文件", exact: true }),
  ).toBeVisible();
  await page.screenshot({
    path: path.join(artifacts, `${baseline ? "before" : "after"}-empty.png`),
  });
  await page.locator("input[type=file]").setInputFiles([
    {
      name: longName,
      mimeType: "video/mp4",
      buffer: Buffer.from("unavailable video thumbnail"),
    },
    {
      name: "项目说明.md",
      mimeType: "text/markdown",
      buffer: Buffer.from("# Design fixture"),
    },
    {
      name: "参考图片.png",
      mimeType: "image/png",
      buffer: Buffer.from("unavailable image thumbnail"),
    },
  ]);
  await expect(page.locator(".finder-file")).toHaveCount(3);
  await expect(page.locator(".finder-grid")).toHaveAttribute(
    "aria-busy",
    "false",
  );
  if (!baseline) {
    const chooser = page.waitForEvent("filechooser");
    await page.getByRole("button", { name: "上传文件", exact: true }).click();
    await (await chooser).setFiles([]);
    const transfer = await page.evaluateHandle(() => {
      const value = new DataTransfer();
      value.items.add(new File(["test"], "drag.txt", { type: "text/plain" }));
      return value;
    });
    const drop = page.getByRole("region", { name: "文件浏览" });
    await drop.dispatchEvent("dragover", { dataTransfer: transfer });
    await expect(drop).toHaveClass(/dragging/);
    await drop.dispatchEvent("dragleave");
    await expect(drop).not.toHaveClass(/dragging/);
    await transfer.dispose();
  }
  await capture(page, "workspace");
  const focused = await page
    .getByRole("link", { name: /在新标签页专注使用网盘/ })
    .getAttribute("href");
  await page.goto("/" + focused);
  await expect(page.locator(".drive-focused")).toBeVisible();
  await selectFinderView(page, "网格视图");
  await capture(page, "focused");
  if (!baseline) {
    const touch = await browser.newContext({
      storageState: await page.context().storageState(),
      viewport: sizes[2],
      hasTouch: true,
      isMobile: true,
    });
    try {
      const mobile = await touch.newPage();
      await mobile.goto(page.url());
      await selectFinderView(mobile, "网格视图");
      await expect(mobile.locator(".finder-row-actions").first()).toHaveCSS(
        "opacity",
        "1",
      );
      await mobile
        .getByRole("button", { name: /更多操作/ })
        .first()
        .tap();
      await expect(mobile.getByRole("menu")).toBeVisible();
      await mobile.screenshot({
        path: path.join(artifacts, "after-touch-menu.png"),
      });
    } finally {
      await touch.close();
    }
    // A large deterministic directory exercises scrolling without bulk writes.
    const entries = Array.from({ length: 48 }, (_, i) => ({
      kind: "file",
      path: `文件-${String(i).padStart(2, "0")}.txt`,
      file: {
        id: `layout-${i}`,
        space_id: "personal",
        path: `文件-${String(i).padStart(2, "0")}.txt`,
        version_id: "v1",
        mime_type: "text/plain",
        size_bytes: 1024,
        updated_at: "2026-09-17T00:00:00Z",
        trashed: false,
      },
    }));
    await page.route("**/api/v1/browser?*", (route) =>
      route.fulfill({
        json: { data: { entries, total: entries.length, next_cursor: "" } },
      }),
    );
    await page.reload();
    await selectFinderView(page, "网格视图");
    await page.setViewportSize(sizes[0]);
    await expect(page.locator(".finder-file")).toHaveCount(48);
    const name = page.locator(".finder-name").first();
    await name.focus();
    await expect(page.locator(".finder-row-actions").first()).toHaveCSS(
      "opacity",
      "1",
    );
    await page.getByRole("checkbox", { name: /选择 文件-00/ }).check();
    const scroll = page.locator(".finder-scroll");
    await scroll.evaluate((el) => {
      el.scrollTop = el.scrollHeight;
    });
    const last = (await page.locator(".finder-file").last().boundingBox())!;
    const status = (await page.locator(".finder-status").boundingBox())!;
    expect(last.y + last.height).toBeLessThanOrEqual(status.y);
    expect(await scroll.evaluate((el) => el.scrollTop)).toBeGreaterThan(0);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollHeight <= innerHeight,
      ),
    ).toBe(true);
    await page.screenshot({
      path: path.join(artifacts, "after-selection-scroll.png"),
    });
    await page.unroute("**/api/v1/browser?*");
  }
  await page.reload();
  await expect(page.locator(".drive-focused")).toBeVisible();
  await page.getByRole("link", { name: "返回完整工作区" }).click();
  await expect(page.locator(".drive-focused")).toHaveCount(0);

  // Exercise the real local entry bundle with a deterministic owner boundary.
  await page.route("**/api/v1/local/file/action", (route) =>
    route.fulfill({ json: { data: [] } }),
  );
  await page.route("**/api/v1/local/list", (route) =>
    route.fulfill({
      json: {
        data: {
          partial: false,
          items: [longName, "项目说明.md", "参考图片.png"].map((name, i) => ({
            ref: `fixture-${i}`,
            name,
            size_bytes: 1024,
            version: "v1",
            mime_type: ["video/mp4", "text/markdown", "image/png"][i],
            observed_at: "2026-09-17T00:00:00Z",
          })),
        },
      },
    }),
  );
  await page.route("**/api/v1/local/file/preview?*", (route) =>
    route.fulfill({ status: 404, body: "Preview unavailable" }),
  );
  await page.goto("/insights.html");
  await page
    .getByRole("button", { name: /^打开网盘 / })
    .first()
    .click();
  await selectFinderView(page, "网格视图");
  await expect(page.locator(".finder-file")).toHaveCount(3);
  await capture(page, "local");
  if (!baseline) {
    await expect(page.locator(".media-fallback")).toHaveCount(3);
    await expect(
      page.locator(".finder-name").filter({ hasText: longName }),
    ).toHaveAttribute("title", longName);
    await page.getByRole("searchbox", { name: "搜索文件" }).fill("不存在的文件");
    await expect(
      page.getByText("没有匹配的文件", { exact: true }),
    ).toBeVisible();
    await page.getByRole("searchbox", { name: "搜索文件" }).fill("");
    await expect(page.locator(".finder-file")).toHaveCount(3);
    await selectFinderView(page, "列表视图");
    await expect(page.locator(".finder-list .finder-file")).toHaveCount(3);
    await page
      .getByRole("button", { name: /更多操作/ })
      .first()
      .click();
    await expect(page.getByRole("menu")).toHaveCSS("position", "fixed");
    await page.keyboard.press("Escape");
    // An unavailable local owner keeps the observed index browseable but read-only.
    await page.route("**/api/v1/local/list", (route) =>
      route.fulfill({
        status: 503,
        json: { error: { code: "insights_owner_unavailable" } },
      }),
    );
    await page.route("**/api/v1/usage/files?*", (route) =>
      route.fulfill({
        json: {
          data: {
            partial: false,
            items: [
              {
                ref: "offline",
                name: "只读文件.md",
                size_bytes: 12,
                version: "v1",
                mime_type: "text/markdown",
              },
            ],
          },
        },
      }),
    );
    await page.reload();
    await page
      .getByRole("button", { name: /^打开网盘 / })
      .first()
      .click();
    await expect(page.locator(".finder-upload")).toBeDisabled();
    await page.locator(".finder-more").click();
    await expect(
      page.getByRole("menuitem", { name: "新建文件夹", exact: true }),
    ).toBeDisabled();
    await page.keyboard.press("Escape");
    await expect(
      page.getByText("当前网盘仅供浏览", { exact: true }),
    ).toBeVisible();
    await page.screenshot({
      path: path.join(artifacts, "after-local-readonly.png"),
    });
  }
});
