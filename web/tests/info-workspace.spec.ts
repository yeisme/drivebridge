import { test, expect, type Page } from "@playwright/test";

// The real fixture backend implements the typed analytics (live) and
// file-query surfaces, so the overview exercises them directly. The second
// test pins a deterministic panel observation at the network boundary with
// the implemented response shape, and the first test also covers the honest
// degradation against a simulated older deployment (plain-text 404).

async function login(page: Page) {
  await page.goto("/");
  await page.getByRole("link", { name: "使用账号登录" }).click();
  await page.getByRole("link", { name: "以 Alice 登录" }).click();
  await expect(page.getByRole("heading", { name: "我的网盘", exact: true })).toBeVisible();
}

const observedAt = "2026-09-25T00:00:00Z";

// Implemented live-backend shape (analytics.go AnalyticsQueryView) with
// fixture-legal values.
function analyticsResponse(mime: string) {
  return {
    query_id: "query-fixture-1",
    backend: "transactional",
    observed_at: observedAt,
    coverage: "partial",
    partial: true,
    rows: [{ mime_type: mime, logical_bytes: 24, file_count: 1 }],
  };
}

test("overview renders from preserved usage endpoints and degrades the typed panel honestly", async ({
  page,
}) => {
  await login(page);
  await page.goto("#/overview");
  // Usage-backed panels come from the real fixture contract.
  await expect(page.getByRole("heading", { name: "容量", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "增长", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "传输", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Agent 活动", exact: true })).toBeVisible();
  await expect(page.getByText(/观测于 /).first()).toBeVisible();
  // The real backend implements the typed analytics surface: the panel runs
  // live consistency and shows real data or an honest empty state — never
  // the pre-integration "unsupported" placeholder or an error.
  const organization = page.locator('[data-panel="organization"]');
  await expect(organization).toHaveAttribute("data-state", /^(ready|empty|partial)$/);
  // An older deployment without the surface still degrades honestly: the
  // panel says so and its drilldown is disabled with a reason, not faked.
  await page.route("**/api/v1/analytics/queries", async (route) => {
    await route.fulfill({ status: 404, contentType: "text/plain", body: "404 page not found" });
  });
  await page.reload();
  await expect(organization).toHaveAttribute("data-state", "unsupported");
  await expect(organization.getByText("此部署未启用对应分析能力")).toBeVisible();
  await expect(organization.locator(".iw-drill-btn")).toBeDisabled();
  // The legacy insights route keeps working (old consumers stay compatible).
  await page.getByRole("button", { name: "资源态势", exact: true }).click();
  await expect(page.getByRole("heading", { name: "资源态势" })).toBeVisible();
});

test("resource overview to drilldown to pinned version to evidence readback", async ({ page }) => {
  await login(page);
  // Upload a real file so version, source operation, and evidence readback
  // all resolve through the fixture's real endpoints.
  await page.getByRole("button", { name: /^打开网盘 / }).first().click();
  const name = "iw-flow-" + Date.now() + ".md";
  await page.locator("input[type=file]").setInputFiles({
    name,
    mimeType: "text/markdown",
    buffer: Buffer.from("# information workspace flow\n"),
  });
  await expect(page.getByRole("button", { name: "详情 " + name, exact: true })).toBeVisible();
  const spaces = ((await (await page.request.get("/api/v1/spaces")).json()).data as {
    id: string;
  }[]);
  const uploadedList = ((
    await (
      await page.request.get("/api/v1/files?space_id=" + encodeURIComponent(spaces[0].id))
    ).json()
  ).data as { id: string; version_id: string; path: string }[]).find(
    (file) => file.path === "/" + name || file.path.endsWith(name),
  );
  if (!uploadedList) throw new Error("uploaded file not found in fixture");
  const uploaded = uploadedList;

  // Typed surfaces are fulfilled from the frozen contract samples.
  await page.route("**/api/v1/analytics/queries", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: analyticsResponse("text/markdown") }),
    });
  });
  await page.route("**/api/v1/file-queries", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data: {
          mode: "exact",
          items: [{ file_id: uploaded.id, version_id: uploaded.version_id }],
          catalogue_revision: "cat-fixture-1",
          next_cursor: null,
          coverage: "complete",
          total_count: 1,
        },
      }),
    });
  });

  await page.goto("#/overview");
  // Overview states its caliber and observation metadata per panel.
  const organization = page.locator('[data-panel="organization"]');
  await expect(organization).toHaveAttribute("data-state", "partial");
  await expect(organization.locator(".iw-chip-partial")).toHaveText("部分覆盖");
  await expect(organization.getByText(/不代表全盘总量/)).toBeVisible();
  await expect(organization.getByText("text/markdown")).toBeVisible();

  // Drilldown uses the same caliber; the exact file list is clickable.
  await organization.locator(".iw-drill-btn").click();
  const drill = page.getByRole("region", { name: "筛选明细" });
  await expect(drill.getByText(/同口径/)).toBeVisible();
  await expect(drill.getByRole("region", { name: "筛选明细数据表" })).toBeVisible();

  // File detail opens with the selected (pinned) version.
  await drill.getByRole("button", { name: "打开详情" }).click();
  const sheet = page.locator('[data-slot="sheet-content"]');
  await expect(sheet.getByText(/文件详情：/)).toBeVisible();
  await sheet.getByRole("tab", { name: "版本" }).click();
  await expect(sheet.getByText(uploaded.version_id).first()).toBeVisible();
  for (const [viewport, expected] of [[1440, 360], [390, 390], [320, 320], [768, 360], [1440, 360]]) {
    await page.setViewportSize({ width: viewport, height: 900 });
    await expect.poll(async () => Math.round((await sheet.boundingBox())!.width)).toBe(expected);
    await expect(sheet.getByText(uploaded.version_id).first()).toBeVisible();
    expect(await sheet.evaluate(el => el.scrollWidth <= el.clientWidth + 1)).toBe(true);
  }
  const resize = sheet.getByRole("separator", { name: "调整详情宽度" });
  await resize.focus();
  await page.keyboard.press("ArrowLeft");
  await expect(resize).toHaveAttribute("aria-valuenow", "376");
  await page.keyboard.press("Enter");
  await expect(resize).toHaveAttribute("aria-valuenow", "360");
  let handle = (await resize.boundingBox())!;
  await page.mouse.move(handle.x + 4, handle.y + 100);
  await page.mouse.down();
  await page.mouse.move(handle.x - 76, handle.y + 100);
  await expect(resize).toHaveAttribute("aria-valuenow", "440");
  await page.keyboard.press("Escape");
  await expect(sheet).toBeVisible();
  await expect(resize).toHaveAttribute("aria-valuenow", "360");
  await page.mouse.up();
  handle = (await resize.boundingBox())!;
  await page.mouse.move(handle.x + 4, handle.y + 100);
  await page.mouse.down();
  await page.mouse.move(handle.x - 76, handle.y + 100);
  await page.mouse.up();
  await expect.poll(async () => Math.round((await sheet.boundingBox())!.width)).toBe(440);
  await resize.focus();
  await page.keyboard.press("Enter");
  await expect.poll(async () => Math.round((await sheet.boundingBox())!.width)).toBe(360);
  handle = (await resize.boundingBox())!;
  await page.mouse.move(handle.x + 4, handle.y + 100);
  await page.mouse.down();
  await page.mouse.move(handle.x - 76, handle.y + 100);
  await expect(resize).toHaveAttribute("aria-valuenow", "440");
  await page.setViewportSize({ width: 390, height: 900 });
  await expect(resize).toBeHidden();
  await page.mouse.up();
  await page.setViewportSize({ width: 1440, height: 900 });
  await expect(resize).toBeVisible();
  await expect.poll(async () => Math.round((await sheet.boundingBox())!.width)).toBe(360);
  for (const interruption of ["pointercancel", "lostpointercapture"]) {
    handle = (await resize.boundingBox())!;
    await page.mouse.move(handle.x + 4, handle.y + 100);
    await page.mouse.down();
    await page.mouse.move(handle.x - 76, handle.y + 100);
    await expect(resize).toHaveAttribute("aria-valuenow", "440");
    await resize.dispatchEvent(interruption, { pointerId: 1 });
    await expect(resize).toHaveAttribute("aria-valuenow", "360");
    await expect(sheet).toBeVisible();
    await page.mouse.up();
  }
  const preferenceKey = await page.evaluate(async () => {
    const me = (await (await fetch("/api/v1/me")).json()).data;
    return `drivebridge:detail-width:${encodeURIComponent(me.instance_id)}:${encodeURIComponent(me.user.id)}:file`;
  });
  expect(await page.evaluate(key => localStorage.getItem(key), preferenceKey)).toBe("360");
  await resize.focus();
  await page.keyboard.press("End");
  await expect.poll(() => page.evaluate(key => localStorage.getItem(key), preferenceKey)).toBe("640");
  handle = (await resize.boundingBox())!;
  await page.mouse.move(handle.x + 4, handle.y + 100);
  await page.mouse.down();
  await page.mouse.move(handle.x + 84, handle.y + 100);
  await expect(resize).toHaveAttribute("aria-valuenow", "560");
  await page.keyboard.press("Escape");
  await page.mouse.up();
  await expect(resize).toHaveAttribute("aria-valuenow", "640");
  expect(await page.evaluate(key => localStorage.getItem(key), preferenceKey)).toBe("640");
  await expect(sheet.getByRole("button", { name: "复制所选版本引用" })).toBeVisible();

  // Source & references read the original operation back by its original ID.
  await sheet.getByRole("tab", { name: "来源与引用" }).click();
  await expect(sheet.getByText("drivebridge-client")).toBeVisible();
  await sheet.getByRole("button", { name: "回读原操作" }).click();
  const readback = sheet.getByRole("region", { name: /原操作回读/ });
  await expect(readback.getByText(/已完成/)).toBeVisible();

  // Evidence tab reads the real audit surface for this file.
  await sheet.getByRole("tab", { name: "活动" }).click();
  await expect(sheet.getByRole("region", { name: "活动" }).getByText(/创建上传|完成文件上传/).first()).toBeVisible();
  await sheet.getByRole("button", { name: "关闭" }).click();
  await expect(sheet).toBeHidden();
  await page.reload();
  await page.locator('[data-panel="organization"] .iw-drill-btn').click();
  await page.getByRole("region", { name: "筛选明细" }).getByRole("button", { name: "打开详情" }).click();
  await expect(sheet).toBeVisible();
  await expect.poll(async () => Math.round((await sheet.boundingBox())!.width)).toBe(640);
  await expect(sheet.getByRole("separator")).toHaveAttribute("aria-valuenow", "640");
});

test("keyboard-only and narrow viewports can reach the table, detail, and back", async ({ page }) => {
  await login(page);
  await page.route("**/api/v1/analytics/queries", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: analyticsResponse("text/markdown") }),
    });
  });
  await page.route("**/api/v1/file-queries", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data: { mode: "exact", items: [], catalogue_revision: "cat-fixture-1", next_cursor: null, coverage: "complete", total_count: 0 },
      }),
    });
  });
  for (const width of [320, 390, 768, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("#/overview");
    const organization = page.locator('[data-panel="organization"]');
    await expect(organization.locator(".iw-drill-btn")).toBeEnabled();
    // No horizontal overflow at the acceptance widths.
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(
      width + 1,
    );
    await organization.locator(".iw-drill-btn").click();
    await expect(page.getByRole("region", { name: "筛选明细", exact: true })).toBeVisible();
    await page.getByRole("button", { name: "返回总览" }).click();
    await expect(page.locator('[data-panel="capacity"]')).toBeVisible();
  }

  // Keyboard only: reach the capacity drill table, open the file detail from
  // a row action, and close it with Escape back to the same control.
  await page.setViewportSize({ width: 390, height: 900 });
  await page.goto("#/overview");
  const drillButton = page.locator('[data-panel="capacity"]').locator(".iw-drill-btn");
  await drillButton.focus();
  await expect(drillButton).toBeFocused();
  await page.keyboard.press("Enter");
  const drill = page.getByRole("region", { name: "筛选明细", exact: true });
  await expect(drill).toBeVisible();
  const openDetail = drill.getByRole("button", { name: "打开详情" }).first();
  await openDetail.focus();
  await page.keyboard.press("Enter");
  const sheet = page.locator('[data-slot="sheet-content"]');
  await expect(sheet.getByText(/文件详情：/)).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(sheet).toBeHidden();
  await expect(openDetail).toBeFocused();
});
