import {test, expect} from "@playwright/test";
import path from "node:path";
import { selectFinderView } from "./helpers/finder-view";

test("directory pages show 50 entries and preserve offsets when moving backwards", async ({page}) => {
  await page.goto("/");
  await page.getByRole("link", {name: "使用账号登录"}).click();
  await page.getByRole("link", {name: "以 Alice 登录"}).click();
  await expect(page.getByRole("heading", {name: "我的网盘", exact: true})).toBeVisible();
  const spaceID = await page.evaluate(async () => {
    const me = (await (await fetch("/api/v1/me")).json()).data;
    const headers = {"Content-Type": "application/json", "X-CSRF-Token": me.csrf_token};
    const created = await fetch("/api/v1/spaces", {method: "POST", headers, body: JSON.stringify({name: "Paging acceptance", connection_id: "platform"})});
    if (!created.ok) throw new Error("Fixture space failed");
    const space = (await created.json()).data;
    for (let i = 0; i < 105; i++) {
      const response = await fetch("/api/v1/directories", {method: "POST", headers, body: JSON.stringify({space_id: space.id, path: `page-${String(i).padStart(3, "0")}`})});
      if (!response.ok) throw new Error("Fixture directory failed");
    }
    return space.id as string;
  });
  const offsets: string[] = [];
  page.on("request", request => {
    const url = new URL(request.url());
    // Finder's sorted page differs from tree and cover observations.
    if (url.pathname === "/api/v1/browser" && url.searchParams.get("space_id") === spaceID && url.searchParams.has("cursor") && url.searchParams.has("sort")) {
      expect(url.searchParams.get("limit")).toBe("50");
      offsets.push(url.searchParams.get("cursor") || "");
    }
  });
  await page.reload();
  await page.getByRole("button", {name: "打开网盘 Paging acceptance", exact: true}).click();
  const folders = page.locator(".finder").getByRole("button", {name: /^打开文件夹 page-/});
  const pagination = page.locator(".finder-pagination");
  await expect(folders).toHaveCount(50);
  await expect(folders.first()).toHaveAccessibleName("打开文件夹 page-000");
  await expect(pagination).toContainText("第 1 页 · 共 105 项");
  await expect(pagination.getByRole("button", {name: "上一页"})).toBeDisabled();
  // Initial workspace hydration can refresh the same read-only first page.
  expect(offsets.length).toBeGreaterThan(0);
  expect(offsets.every(cursor => cursor === "")).toBe(true);
  offsets.length = 0;
  await pagination.getByRole("button", {name: "下一页"}).click();
  await expect(folders.first()).toHaveAccessibleName("打开文件夹 page-050");
  await expect(folders).toHaveCount(50);
  await expect(pagination).toContainText("第 2 页 · 共 105 项");
  await pagination.getByRole("button", {name: "下一页"}).click();
  await expect(folders).toHaveCount(5);
  await expect(folders.first()).toHaveAccessibleName("打开文件夹 page-100");
  await expect(pagination).toContainText("第 3 页 · 共 105 项");
  await expect(pagination.getByRole("button", {name: "下一页"})).toBeDisabled();
  await pagination.getByRole("button", {name: "上一页"}).click();
  await expect(folders.first()).toHaveAccessibleName("打开文件夹 page-050");
  await expect(folders).toHaveCount(50);
  await pagination.getByRole("button", {name: "上一页"}).click();
  await expect(folders.first()).toHaveAccessibleName("打开文件夹 page-000");
  await expect(pagination).toContainText("第 1 页 · 共 105 项");
  expect(offsets).toEqual(["50", "100", "50", ""]);
});

test("directory list/detail and cards preserve routes, selection and read access", async ({page}) => {
  await page.goto("/");
  await page.getByRole("link",{name:"使用账号登录"}).click();
  await page.getByRole("link",{name:"以 Alice 登录"}).click();
  await page.setViewportSize({width:1440,height:963});
  await expect(page.getByRole("button",{name:"列表详情",exact:true})).toHaveAttribute("aria-pressed","true");
  await expect(page.getByRole("complementary",{name:"网盘详情"})).toBeVisible();
  const item=page.getByRole("button",{name:/^查看网盘详情 /}).first();
  await item.click();
  await expect(page).toHaveURL(/selected=/);
  const directoryURL=page.url();
  await page.getByRole("button",{name:/^打开网盘 /}).first().click();
  await expect(page.locator(".finder")).toBeVisible();
  await page.goBack();
  await expect(page).toHaveURL(directoryURL);
  await expect(page.getByRole("button",{name:/^查看网盘详情 /}).first()).toHaveAttribute("aria-pressed","true");
  await page.getByRole("button",{name:"卡片视图",exact:true}).click();
  await expect(page.locator("button.drive-card").first()).toBeVisible();
  await page.getByRole("button",{name:"列表详情",exact:true}).click();
  await page.setViewportSize({width:872,height:963});
  await item.click();
  await expect(page.getByRole("dialog",{name:"网盘详情"})).toBeVisible();
  await page.screenshot({animations:"disabled",path:path.join(process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!,"directory-detail-872.png")});
  await page.keyboard.press("Escape");
  await page.getByRole("searchbox",{name:"搜索网盘"}).fill("无匹配网盘");
  await expect(page.getByRole("button",{name:/^打开网盘 /})).toHaveCount(0);
  await expect(page.getByRole("dialog")).toHaveCount(0);
});

test("columns and gallery keep long names readable and open the same full reader", async ({page}) => {
  await page.goto("/");
  await page.getByRole("link",{name:"使用账号登录"}).click();
  await page.getByRole("link",{name:"以 Alice 登录"}).click();
  await page.getByRole("button",{name:/^打开网盘 /}).first().click();
  const name="长文件名阅读验收-"+"设计素材与版本记录".repeat(10)+".txt";
  await page.locator("input[type=file]").setInputFiles({name,mimeType:"text/plain",buffer:Buffer.from("Stable full reader content")});
  await expect(page.getByRole("button",{name:`详情 ${name}`,exact:true})).toBeVisible();
  const artifacts=process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!;
  for(const width of [1213,957,872,390]) for(const mode of ["分栏视图","画廊视图"] as const) {
    await page.setViewportSize({width,height:963});
    await selectFinderView(page, mode);
    const choice=page.getByRole("button",{name:`选择 ${name}`,exact:true});
    await choice.click();
    expect(await choice.evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBe(true);
    expect((await choice.boundingBox())!.height).toBeGreaterThanOrEqual(mode==="分栏视图" ? 56 : 132);
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
    await page.screenshot({animations:"disabled",path:path.join(artifacts,`reader-${mode==="分栏视图" ? "columns" : "gallery"}-${width}.png`)});
    await page.getByRole("button",{name:"打开阅读器",exact:true}).click();
    await expect(page.locator(".quicklook-dialog")).toContainText("Stable full reader content");
    await page.keyboard.press("Escape");
    await expect(page.locator(".quicklook-dialog")).toHaveCount(0);
  }
  await page.setViewportSize({width:1213,height:963});
  await page.getByRole("button",{name:"分栏视图",exact:true}).click();
  await page.getByRole("button",{name:`选择 ${name}`,exact:true}).press("Enter");
  await expect(page.locator(".quicklook-dialog")).toContainText("Stable full reader content");
});
