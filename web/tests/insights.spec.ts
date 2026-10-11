import { test, expect } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
const artifacts =
  process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS ||
  path.resolve("temp/browser-artifacts");
test("resource overview, explicit inventory scan, chart drilldown and audit", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByRole("link", { name: "使用账号登录" }).click();
  await page.getByRole("link", { name: "以 Alice 登录" }).click();
  await expect(page.getByRole("heading", { name: "我的网盘", exact: true })).toBeVisible({
    timeout: 20000,
  });
  await page.goto("/#/analytics");
  await expect(page.getByRole("heading", { name: "资源态势", exact:true })).toBeVisible();
  await expect(page.getByRole("tab", { name: "容量", exact:true })).toHaveAttribute("aria-selected", "true");
  await expect(page).toHaveURL(/#\/analytics/);
  await expect(page.getByText("受管逻辑用量", { exact: true })).toBeVisible();
  const spaces = (await (await page.request.get("/api/v1/spaces")).json()).data;
  const me = (await (await page.request.get("/api/v1/me")).json()).data;
  // A resource GET must not enable or run a scan.
  const scans = (await (await page.request.get("/api/v1/usage/scans")).json())
    .data;
  expect(scans.length).toBe(0);
  await page.getByRole("combobox", { name: "空间", exact: true }).click();
  await page.getByRole("option", { name: spaces[0].name || spaces[0].id, exact: true }).click();
  await page.getByRole("button", { name: "扫描所选空间", exact: true }).click();
  await expect(
    page.getByRole("dialog", { name: "启用授权目录扫描" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "启用并扫描", exact: true }).click();
  await expect(
    page.getByText("扫描结果：完整观测", { exact: true }),
  ).toBeVisible();
  await expect
    .poll(
      async () =>
        (await (await page.request.get("/api/v1/usage/scans")).json()).data[0]
          ?.status,
    )
    .toBe("complete");
  // The summary shares its numeric contract with the charts.
  const summary = (
    await (await page.request.get("/api/v1/usage/summary")).json()
  ).data;
  expect(summary.metric_version).toBe("1.0");
  expect(summary.coverage).toContain(
    "application_observed_bytes_not_network_billing",
  );
  await page.getByRole("tab", { name: "传输", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: /有效文件量与应用观测量/ }),
  ).toBeVisible();
  await page.getByRole("tab", { name: "机器人", exact: true }).click();
  await expect(page.getByText(/caller 是调用方声明/)).toBeVisible();
  await page.getByRole("tab", { name: "安全审计", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "事件时间线", exact: true }),
  ).toBeVisible();
  await page.getByRole("tab", { name: "容量", exact: true }).click();
  fs.mkdirSync(artifacts, { recursive: true });
  const day = page.locator(".insight-chart g[role='button']").first();
  await expect(day).toBeVisible();
  await day.focus();
  await expect(page.locator(".chart-float")).toBeVisible();
  await expect(page.locator(".chart-float")).toContainText("未采集");
  await expect(page.getByText("查看数据表").first()).toBeVisible();
  await page.screenshot({
    path: path.join(artifacts, "insights-desktop.png"),
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByText("受管逻辑用量", { exact: true })).toBeVisible();
  await page.screenshot({
    path: path.join(artifacts, "insights-mobile.png"),
  });
  // Policy deactivation uses the authenticated service, not file deletion.
  await page.request.post("/api/v1/usage/scans", {
    headers: {
      Origin: new URL(page.url()).origin,
      "X-CSRF-Token": me.csrf_token,
    },
    data: { space_id: spaces[0].id, enabled: false, configure_only: true },
  });
});
