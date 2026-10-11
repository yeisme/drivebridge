import { test, expect } from "@playwright/test";
import path from "node:path";
const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS || "temp/browser-artifacts";
test("single header, one analysis navigation and responsive quota popover", async ({ page }) => {
  await page.setViewportSize({width:1440,height:1000});
  await page.goto("/");
  await page.getByRole("link",{name:"使用账号登录"}).click();
  await page.getByRole("link",{name:"以 Alice 登录"}).click();
  await page.getByRole("button",{name:/^打开网盘 /}).first().click();
  await expect(page.getByRole("group",{name:"文件列表表头"})).toHaveCount(1);
  await expect(page.locator(".finder-list .finder-list")).toHaveCount(0);
  await expect(page.locator(".page-heading,.main-footer")).toHaveCount(0);
  await expect(page.getByRole("button",{name:"上传文件",exact:true})).toBeVisible();
  const url = page.url();
  const rows = await page.evaluate(async () => (await (await fetch("/api/v1/spaces")).json()).data);
  let quota: number | null = 100; let used = 20;
  await page.route("**/api/v1/spaces", route => route.fulfill({json:{data:rows.map((s:object)=>({...s,used_bytes:used,reserved_bytes:5,max_bytes:quota}))}}));
  for(const width of [1440,1024,768,390]) {
    await page.setViewportSize({width,height:1000});
    await page.getByRole("button",{name:"空间用量",exact:true}).click();
    const dialog=page.getByRole("dialog",{name:"空间用量",exact:true});
    await expect(dialog.locator(".capacity-center strong")).toHaveText("25%");
    await expect(dialog).toContainText("75 B");
    const box=(await dialog.boundingBox())!;
    expect(box.x).toBeGreaterThanOrEqual(0);expect(box.x+box.width).toBeLessThanOrEqual(width);
    await page.screenshot({path:path.join(artifacts,`capacity-${width}.png`)});
    await page.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
    expect(page.url()).toBe(url);
    await expect(page.getByRole("button",{name:"空间用量",exact:true})).toBeFocused();
  }
  await page.setViewportSize({width:1440,height:1000});
  await page.getByRole("button",{name:"折叠侧边栏",exact:true}).click();
  await page.getByRole("button",{name:"空间用量",exact:true}).click();
  await expect(page.locator(".capacity-center strong")).toHaveText("25%");
  await page.getByRole("button",{name:"查看详细用量",exact:true}).click();
  await expect(page.getByRole("tab",{name:"容量",exact:true})).toHaveAttribute("aria-selected","true");
  await expect(page.getByRole("tablist")).toHaveCount(1);
  await expect(page.locator(".capacity-page .capacity-center strong")).toHaveText("25%");
  await page.screenshot({path:path.join(artifacts,"capacity-page.png")});
  for (const width of [1440,1024,768,390]) {
    await page.setViewportSize({width,height:1000});
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({path:path.join(artifacts,`capacity-page-${width}.png`)});
  }
  await page.setViewportSize({width:1440,height:1000});
  await page.getByRole("button",{name:"空间用量",exact:true}).click();
  quota=null;
  await page.getByRole("button",{name:"刷新空间用量"}).click();
  await expect(page.getByRole("dialog").locator(".capacity-center")).toContainText("配额未提供");
  quota=100;used=110;
  await page.getByRole("button",{name:"刷新空间用量"}).click();
  await expect(page.getByRole("dialog").locator(".capacity-center strong")).toHaveText("115%");
  await expect(page.getByRole("dialog")).toContainText("超出配额 15 B");
  await page.unroute("**/api/v1/spaces");
  await page.route("**/api/v1/spaces",route=>route.fulfill({status:403,json:{error:{code:"forbidden"}}}));
  await page.getByRole("button",{name:"刷新空间用量"}).click();
  await expect(page.getByRole("dialog").getByRole("alert")).toBeVisible();
  await expect(page.getByRole("dialog").locator(".capacity-ring")).toHaveCount(0);
  await page.keyboard.press("Escape");
});


test("robot toolbar and guide remain reachable at all viewport widths", async ({page}) => {
  await page.goto("/");
  await page.getByRole("link", {name:"使用账号登录"}).click();
  await page.getByRole("link", {name:"以 Alice 登录"}).click();
  await page.getByRole("button", {name:"Agent 管理",exact:true}).click();
  await page.getByRole("tab", {name:"旧版机器人授权",exact:true}).click();
  for (const width of [1440,1024,768,390]) {
    await page.setViewportSize({width,height:1000});
    await expect(page.getByRole("button", {name:"授权机器人",exact:true})).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    const topbar = (await page.locator(".topbar").boundingBox())!;
    const search = (await page.locator(".content [data-slot=\"input-group\"]").first().boundingBox())!;
    expect(search.y).toBeGreaterThanOrEqual(topbar.y + topbar.height);
    await page.screenshot({path:path.join(artifacts,`robots-${width}.png`)});
  }
  await page.getByRole("button", {name:"连接指南",exact:true}).click();
  await expect(page.getByRole("dialog", {name:"连接机器人",exact:true})).toBeVisible();
  await page.screenshot({path:path.join(artifacts,"robot-guide-mobile.png")});
});
