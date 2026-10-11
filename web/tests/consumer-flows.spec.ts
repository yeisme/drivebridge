import { test, expect, type Page } from "@playwright/test";
import { selectChoice } from "./helpers/select";

// Consumer flows e2e (drivebridge-file-platform-consumers-v2).
//
// The typed upload/organization/delivery/discussion surfaces are not
// implemented by the offline fixture backend yet, so these tests fulfill
// them at the network boundary with responses derived from the frozen
// backend contract samples. The capability probes, envelope handling, real
// login/session, routing and the discussion reads that the fixture does
// implement run against the real server. Everything the flow asserts uses
// only public HTTP contracts with fixture-legal IDs; original IDs read back.

async function login(page: Page) {
  await page.goto("/");
  await page.getByRole("link", { name: "使用账号登录" }).click();
  await page.getByRole("link", { name: "以 Alice 登录" }).click();
  await expect(page.getByRole("heading", { name: "我的网盘", exact: true })).toBeVisible();
}

// Route mocks derived from the frozen contract samples. `spaceId` scopes the
// exact file query; `fileId`/`versionId` come from the fixture's real
// uploaded file so the original ID stays readable through the real routes.
function mockTypedSurfaces(page: Page, ids: { spaceId: string; fileId: string; versionId: string; shareId: string }) {
  void page.route("**/api/v1/operations/lookup", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: { found: false } }),
    });
  });
  void page.route("**/api/v1/upload-batches", async (route) => {
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: { batch_id: "batch-fx-1", batch_revision: 1, phase: "enumerating", items_total: null, items_completed: 0, items_failed: 0, items_waiting: 0, next_cursor: null } }) });
  });
  void page.route("**/api/v1/upload-batches/batch-fx-1/items*", async (route) => {
    if (route.request().method() === "POST")
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: {} }) });
    else
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          data: {
            batch_id: "batch-fx-1",
            batch_revision: 2,
            phase: "transferring",
            items_total: 2,
            items_completed: 0,
            items_failed: 0,
            items_waiting: 2,
            next_cursor: null,
            items: [
              { item_key: "flow-upload/docs/flow.md:aa", relative_path: "flow-upload/docs/flow.md", operation_id: "op-fx-1", result_certainty: "confirmed", source_fingerprint: "a".repeat(64) },
              { item_key: "flow-upload/orig.md:bb", relative_path: "flow-upload/orig.md", operation_id: "op-fx-2", result_certainty: "confirmed", source_fingerprint: "b".repeat(64) },
            ],
          },
        }),
      });
  });
  void page.route("**/api/v1/upload-batches/batch-fx-1/seal", async (route) => {
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: { batch_id: "batch-fx-1", batch_revision: 2, phase: "sealed", items_total: 2, items_completed: 0, items_failed: 0, items_waiting: 2, next_cursor: null } }) });
  });
  void page.route("**/api/v1/file-queries", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data: { mode: "exact", items: [{ file_id: ids.fileId, version_id: ids.versionId }], catalogue_revision: "cat-fx-1", next_cursor: null, coverage: "complete", total_count: 1 },
      }),
    });
  });
  void page.route("**/api/v1/content-queries", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data: {
          mode: "discovery",
          coverage: "partial",
          items: [
            { file_ref: ids.fileId, asset_revision_ref: "asset-fx-1", evidence_ref: "evidence-fx-1", locator: { kind: "time_range", start_ms: 1000, end_ms: 2500 }, truth_level: "inferred", index_generation: "gen-fx-1" },
          ],
          missing: ["audio"],
          next_cursor: null,
          observed_at: "2026-09-25T00:00:00Z",
        },
      }),
    });
  });
  void page.route("**/api/v1/file-collections", async (route) => {
    if (route.request().method() === "POST")
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: { collection_id: "coll-fx-1", revision: 1 } }) });
    else await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: [] }) });
  });
  void page.route("**/api/v1/file-collections/coll-fx-1/snapshots", async (route) => {
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: { snapshot_id: "snap-fx-1", item_count: 1 } }) });
  });
  void page.route("**/api/v1/organization-plans**", async (route) => {
    const executing = new URL(route.request().url()).pathname.endsWith("/execute");
    const confirming = new URL(route.request().url()).pathname.endsWith("/confirm");
    if (executing) {
      await route.fulfill({ status: 409, contentType: "application/json", body: JSON.stringify({ error: { code: "plan_stale" } }) });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data: {
          plan_id: "plan-fx-1",
          revision: confirming ? 2 : 1,
          phase: confirming ? "authorized" : "previewed",
          risk: "policy_scoped",
          approval_required: false,
          policy_revision: 4,
          items_total: 1,
          impact_digest: "d".repeat(64),
          items: [{ item_key: "docs/flow.md:aa", file_id: ids.fileId, version_id: ids.versionId, action: "add_label", before: "/docs/flow.md", after: "/docs/flow.md", state: "pending" }],
        },
      }),
    });
  });
  void page.route("**/api/v1/shares", async (route) => {
    if (route.request().method() === "POST")
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: { share_id: ids.shareId, manifest_revision: 1 } }) });
    else
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ data: [{ id: ids.shareId, name: "固定交付一批", mode: "fixed", audience: "user", path: "/", expires_at: "", revoked: false, allow_download: true }] }),
      });
  });
  void page.route("**/api/v1/shares/*/delivery-manifests**", async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname.endsWith("/publish")) {
      await route.fulfill({ status: 409, contentType: "application/json", body: JSON.stringify({ error: { code: "delivery_incomplete" } }) });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data: { share_id: ids.shareId, manifest_revision: 1, phase: "verifying", required_items: 10, verified_items: 9, complete: false, retention_ready: true },
      }),
    });
  });
  void page.route("**/api/v1/files/*/threads", async (route) => {
    if (route.request().method() === "POST")
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: { thread_id: "thread-fx-1", file_id: ids.fileId, version_id: ids.versionId, revision: 1, state: "open", author_kind: "user" } }) });
    else
      // Live envelope {file_id,threads,next_cursor}; the file-level anchor
      // (version_id:"") is first-class and must survive normalization.
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          data: {
            file_id: ids.fileId,
            next_cursor: "",
            threads: [
              { thread_id: "thread-fx-1", file_id: ids.fileId, version_id: ids.versionId, revision: 1, state: "open", author_kind: "user" },
              { thread_id: "thread-fx-2", file_id: ids.fileId, version_id: "", revision: 1, state: "open", author_kind: "user" },
            ],
          },
        }),
      });
  });
  void page.route("**/api/v1/files/*/private-note/publish-preview", async (route) => {
    // Implemented shape (discussion.go PreviewNotePublication).
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data: { file_id: ids.fileId, target: "team_thread", body_preview: "内部确认过的备注", note_revision: 1, note: "preview only; publish explicitly with file_thread_create; the private note stays private" },
      }),
    });
  });
  void page.route("**/api/v1/files/*/private-note", async (route) => {
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: { body: "内部确认过的备注", revision: 1 } }) });
  });
}

test("every center degrades honestly on a deployment without the typed surfaces", async ({ page }) => {
  await login(page);
  // Simulate a legacy deployment at the network layer: the frozen contract
  // had unimplemented actions answer 400 invalid_request on the lookup
  // probe, which closes the capability gate honestly. The current backend
  // implements every action (lookup answers found:false), so the legacy
  // shape must be mocked rather than relied upon.
  await page.route("**/api/v1/operations/lookup", async (route) => {
    await route.fulfill({ status: 400, contentType: "application/json", body: JSON.stringify({ error: { code: "invalid_request" } }) });
  });
  await page.goto("#/flows");
  // Centers whose contract actions this deployment does not accept state the
  // absent action instead of rendering a broken form.
  await page.getByRole("tab", { name: "云端检索" }).click();
  // Metadata search remains independently authorized without content analysis.
  await expect(page.getByRole("searchbox", { name: "检索条件" })).toBeVisible();
  await page.getByRole("radio", { name: "内容发现（Top-K）", exact: true }).click();
  await expect(page.getByText("此部署未启用对应能力")).toBeVisible();
  await expect(page.getByText("content_query")).toBeVisible();
  await page.getByRole("tab", { name: "整理中心" }).click();
  await expect(page.getByText("organize_plan")).toBeVisible();
  // Old routes keep working while the new page exists.
  await page.getByRole("button", { name: "我的工作台", exact: true }).click();
  await page.getByRole("tab", { name: "容量与活动观察", exact: true }).click();
  await expect(page.getByRole("heading", { name: "容量", exact: true })).toBeVisible();
});

test("upload recovery to cloud query to organize preview to fixed delivery to version discussion", async ({ page }) => {
  await login(page);
  // A real fixture upload anchors the flow in real IDs readable through the
  // real routes (original-ID readback).
  await page.getByRole("button", { name: /^打开网盘 / }).first().click();
  const name = "cf-flow-" + Date.now() + ".md";
  await page.locator("input[type=file]").setInputFiles({ name, mimeType: "text/markdown", buffer: Buffer.from("# consumer flow\n") });
  await expect(page.getByRole("button", { name: "详情 " + name, exact: true })).toBeVisible();
  const spaces = ((await (await page.request.get("/api/v1/spaces")).json()).data as { id: string }[]);
  const uploadedList = ((
    await (await page.request.get("/api/v1/files?space_id=" + encodeURIComponent(spaces[0].id))).json()
  ).data as { id: string; version_id: string; path: string }[]).find((file) => file.path === "/" + name || file.path.endsWith(name));
  if (!uploadedList) throw new Error("uploaded file not found in fixture");
  const ids = { spaceId: spaces[0].id, fileId: uploadedList.id, versionId: uploadedList.version_id, shareId: "share-fx-1" };
  mockTypedSurfaces(page, ids);

  await page.goto("#/flows");

  // 上传恢复：the recovery identity is seeded by a real create flow in unit
  // tests; here the batch reads back with phased progress facts.
  await expect(page.getByText("选择要上传的文件").or(page.getByText("选择要上传的文件或文件夹")).first()).toBeVisible();
  await page.locator('[data-region="select"] input[type=file]').setInputFiles("tests/fixtures/flow-upload");
  await expect(page.getByRole("button", { name: "创建上传批次" })).toBeEnabled();
  await page.getByRole("button", { name: "创建上传批次" }).click();
  // Batch view: phased facts distinguish sent/durable/verified; 100% sent is
  // never 已完成.
  await expect(page.getByText("批次 batch-fx-1")).toBeVisible();
  // Phased facts are separated; the phase shown matches what actually
  // happened (sealed, nothing claimed beyond it).
  await expect(page.getByText("已封存批次")).toBeVisible();
  await expect(page.getByText("已耐久确认")).toBeVisible();
  await expect(page.getByText(/发送进度不代表完成/)).toBeVisible();

  // 云端查询：Top-K candidates grouped separately from the exact list.
  await page.getByRole("tab", { name: "云端检索" }).click();
  await page.getByRole("radio", { name: "内容发现（Top-K）" }).click();
  await page.getByLabel("检索条件").fill("雨天街道的镜头");
  await page.getByRole("button", { name: "检索", exact: true }).click();
  await expect(page.getByText("内容候选（Top-K，不代表全集）")).toBeVisible();
  await expect(page.getByText("缺失模态：audio")).toBeVisible();
  await expect(page.getByText("索引代次 gen-fx-1")).toBeVisible();
  // 整理预览：materialize, preview, confirm — then plan_stale demands a
  // fresh preview and never auto-reconfirms.
  await page.getByRole("tab", { name: "整理中心" }).click();
  await page.getByLabel("目录前缀").fill("docs/");
  await page.getByRole("button", { name: "物化快照" }).click();
  await expect(page.getByText(/快照已固定/)).toBeVisible();
  // The policy read runs against the REAL backend: a fresh space has no
  // policy, which must surface as first-use guidance, not an error banner
  // (drift D8).
  await expect(page.getByText(/整理策略未配置/)).toBeVisible();
  await page.getByRole("button", { name: "生成标签整理计划" }).click();
  await expect(page.getByText(/plan-fx-1/)).toBeVisible();
  await page.getByRole("button", { name: "确认并执行" }).click();
  await expect(page.getByText(/计划在预览后已发生变化/)).toBeVisible();

  // 固定交付：缺项 disables publishing with the recovery entry.
  await page.getByRole("tab", { name: "交付与讨论" }).click();
  await selectChoice(page.getByLabel("选择分享"), { label: "固定交付一批" });
  await page.getByRole("button", { name: "准备清单" }).click();
  await expect(page.getByText(/交付缺 1 项/)).toBeVisible();
  await expect(page.getByRole("button", { name: "发布完整清单" })).toBeDisabled();
  // Receipt facts stay graded; opened/bytes_served never read as acceptance.
  await expect(page.getByText(/可访问 → 已打开 → 已传出字节 → 接收方已验证 → 接收方已确认/)).toBeVisible();

  // 版本讨论：version-anchored thread plus private-note preview before any
  // publication.
  await page.getByLabel("文件 ID").fill(ids.fileId);
  await page.getByLabel("版本锚点").fill(ids.versionId);
  await page.getByLabel("讨论内容").fill("请确认这个版本的标题。");
  await page.getByRole("button", { name: "发布讨论" }).click();
  await expect(page.getByText("讨论已发布到固定版本锚点。")).toBeVisible();
  // The file-level anchor thread survives normalization and renders as such.
  await expect(page.getByText(/锚点 文件级/)).toBeVisible();
  await page.getByLabel("私人备注").fill("内部确认过的备注");
  await page.getByRole("button", { name: "公开前预览" }).click();
  await expect(page.getByText(/将公开到：团队讨论（基于备注第 1 次修订）/)).toBeVisible();
  await expect(page.getByText("正文预览：内部确认过的备注")).toBeVisible();
  await expect(page.getByText("preview only; publish explicitly with file_thread_create; the private note stays private")).toBeVisible();
});

test("keyboard-only and narrow viewports reach every center without horizontal overflow", async ({ page }) => {
  await login(page);
  await page.route("**/api/v1/operations/lookup", async (route) => {
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: { found: false } }) });
  });
  for (const width of [320, 390, 768, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("#/flows");
    await page.getByRole("tab", { name: "上传中心" }).click();
    await expect(page.getByRole("tabpanel", { name: "上传中心" })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width + 1);
    for (const label of ["云端检索", "整理中心", "交付与讨论"]) {
      await page.getByRole("tab", { name: label }).click();
      await expect(page.getByRole("tabpanel", { name: label })).toBeVisible();
    }
  }
  // Keyboard only: arrow keys move between center tabs and focus follows.
  await page.setViewportSize({ width: 390, height: 900 });
  await page.goto("#/flows");
  const uploadTab = page.getByRole("tab", { name: "上传中心" });
  await uploadTab.focus();
  await page.keyboard.press("ArrowRight");
  await expect(page.getByRole("tab", { name: "云端检索" })).toBeFocused();
  await page.keyboard.press("ArrowLeft");
  await expect(uploadTab).toBeFocused();
});

test("content mismatch keeps verified progress and names the next step in Chinese", async ({ page }) => {
  await login(page);
  // Seed the recovery identity through the real create flow, then refresh:
  // the handle is gone and the user must re-select the original file.
  await page.route("**/api/v1/operations/lookup", async (route) => {
    const body = route.request().postDataJSON() as { action: string };
    if (body.action === "upload_batch_create")
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: { found: true, batch_id: "batch-fx-1", operation_id: "op-fx-1", result_certainty: "confirmed", recovery_action: "read_original_operation" } }) });
    else await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: { found: false } }) });
  });
  await page.route("**/api/v1/upload-batches**", async (route) => {
    if (route.request().method() === "POST")
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: { batch_id: "batch-fx-1", batch_revision: 1, phase: "enumerating", items_total: null, items_completed: 0, items_failed: 0, items_waiting: 0, next_cursor: null } }) });
    else if (new URL(route.request().url()).pathname.endsWith("/items"))
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: { batch_id: "batch-fx-1", batch_revision: 1, phase: "enumerating", items_total: null, items_completed: 0, items_failed: 0, items_waiting: 0, next_cursor: null, items: [{ item_key: "orig.md:ff", relative_path: "flow-upload/orig.md", operation_id: "op-fx-1", result_certainty: "confirmed", source_fingerprint: "f".repeat(64) }] } }) });
    else await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: {} }) });
  });
  await page.goto("#/flows");
  await page.locator('[data-region="select"] input[type=file]').setInputFiles("tests/fixtures/flow-upload");
  await page.getByRole("button", { name: "创建上传批次" }).click();
  await expect(page.getByText("批次 batch-fx-1")).toBeVisible();
  // Refresh: the recovery identity survives in IndexedDB. The persisted
  // server-side fingerprint intentionally differs from the real digest, so
  // the re-selection check must report content_mismatch and keep progress.
  await page.reload();
  await expect(page.getByText("可恢复的上传身份")).toBeVisible();
  await page.getByRole("button", { name: "按原键恢复" }).click();
  await expect(page.getByText("重新选择原文件")).toBeVisible();
  await page.getByLabel("重新选择原文件").setInputFiles("tests/fixtures/flow-upload");
  await expect(page.getByText("已重新计算摘要，请校验并继续原任务。")).toBeVisible();
  await page.getByRole("button", { name: "校验并继续原任务" }).click();
  await expect(page.getByRole("heading", { name: "文件校验不一致" })).toBeVisible();
  await expect(page.getByText(/已验证的进度保留/)).toBeVisible();
  await expect(page.getByText("orig.md").first()).toBeVisible();
});
