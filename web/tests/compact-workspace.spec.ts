import { test, expect } from "@playwright/test";
import path from "node:path";

test("compact Finder and insights fit content widths in both themes", async ({page, request}) => {
  test.setTimeout(120000);
  test.skip(process.env.DRIVEBRIDGE_TEST_TEAM_DEMO === "1", "Uses a disposable personal owner");
  await page.goto("/");
  await page.getByRole("link", {name:"使用账号登录"}).click();
  await page.getByRole("link", {name:"以 Alice 登录"}).click();
  await expect(page.getByRole("button",{name:/^打开网盘 /}).first()).toBeVisible();
  await page.getByRole("button", {name:/^打开网盘 /}).first().click();
  const drive = new URL(page.url()).hash;
  const space = drive.split("?")[0].split("/").pop()!;
  const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!;
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  for (const theme of ["light", "dark"]) for (const width of [1440,1280,1213,957,882,872,390]) {
    await page.setViewportSize({width,height:963});
    await page.evaluate(theme=>document.documentElement.classList.toggle("dark",theme==="dark"),theme);
    for (const collapsed of [false,true]) {
      await page.evaluate(hash=>{location.hash=hash;},drive);
      await expect(page.locator(".finder")).toBeVisible();
      const toggle=page.getByRole("button",{name:collapsed ? "折叠侧边栏" : "展开侧边栏",exact:true});
      if(await toggle.count()) await toggle.click();
      await expect(page.locator(".drive-location")).toHaveCount(0);
      const heights=await page.locator('.finder-actions button:visible, .finder-actions [role=combobox]:visible').evaluateAll(nodes=>nodes.map(el=>el.getBoundingClientRect().height));
      expect(heights.length).toBeGreaterThan(2);
      expect(Math.max(...heights)-Math.min(...heights)).toBeLessThanOrEqual(1);
      expect(Math.max(...heights)).toBe(width < 768 ? 44 : 32);
      expect(await page.locator(".finder-body").evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBe(true);
      await expect(page.locator(".finder-toolbar").getByRole("link",{name:/在新标签页专注/})).toBeVisible();
      await page.screenshot({animations:"disabled",path:path.join(artifacts,`after-finder-${theme}-${width}-${collapsed}.png`)});
      for(const view of ["overview","capacity"]) {
        await page.evaluate(hash=>{location.hash=hash;},`#/insights?view=${view}&space=${space}`);
        await expect(page.locator(view==="overview" ? ".dashboard-overview" : ".insight-grid").first()).toBeVisible();
        expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
        expect(await page.locator(".content").evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBe(true);
        if(view==="capacity") {
          const contentWidth=await page.locator(".insights").evaluate(el=>el.clientWidth);
          const columns=await page.locator(".insight-grid").first().evaluate(el=>getComputedStyle(el).gridTemplateColumns.split(" ").length);
          expect(columns).toBe(contentWidth>=880 ? 2 : 1);
          await expect(page.locator(".insight-chart").first()).not.toContainText("0.5 B");
          await page.locator(".insight-grid").first().scrollIntoViewIfNeeded();
        }
        await page.screenshot({animations:"disabled",path:path.join(artifacts,`after-${view}-${theme}-${width}-${collapsed}.png`)});
      }
    }
  }
  await page.evaluate(hash=>{location.hash=hash;},drive);
  await page.setViewportSize({width:957,height:963});
  await page.getByRole("button",{name:"新建",exact:true}).click();
  await page.getByRole("menuitem",{name:"新建文件夹",exact:true}).click();
  await expect(page.getByRole("dialog",{name:"新建文件夹",exact:true})).toBeVisible();
  await page.keyboard.press("Escape");
  const popupPromise=page.waitForEvent("popup");
  await page.getByRole("link",{name:/在新标签页专注/}).click();
  const focused=await popupPromise;
  await expect(focused.locator(".finder")).toBeVisible();
  await expect(focused.locator(".sidebar")).toBeHidden();
  await focused.getByRole("link",{name:"返回完整工作区"}).click();
  await expect(focused.locator(".sidebar")).toBeVisible();
  expect(new URL(focused.url()).hash.split("?")[0]).toBe(drive.split("?")[0]);
  await focused.close();
  expect(errors).toEqual([]);
});

test("touch Finder controls retain 44px targets without page overflow", async ({browser}) => {
  test.skip(process.env.DRIVEBRIDGE_TEST_TEAM_DEMO === "1", "Uses a disposable personal owner");
  const context=await browser.newContext({hasTouch:true,viewport:{width:390,height:963}});
  const page=await context.newPage();
  try {
    await page.goto(`http://127.0.0.1:${process.env.DRIVEBRIDGE_TEST_PORT||31878}/`);
    await page.getByRole("link",{name:"使用账号登录"}).click();
    await page.getByRole("link",{name:"以 Alice 登录"}).click();
    await page.getByRole("button",{name:/^打开网盘 /}).first().click();
    for(const width of [390,957]) {
      await page.setViewportSize({width,height:963});
      const controls=await page.locator('.finder-actions button:visible, .finder-toolbar button:visible, .finder-toolbar a:visible').evaluateAll(nodes=>nodes.map(el=>({label:el.getAttribute("aria-label")||el.textContent,height:el.getBoundingClientRect().height,width:el.getBoundingClientRect().width})));
      for(const control of controls) {
        expect(control.height,control.label||"").toBeGreaterThanOrEqual(44);
        expect(control.width,control.label||"").toBeGreaterThanOrEqual(44);
      }
      expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
      await page.screenshot({animations:"disabled",path:path.join(process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!,`touch-finder-${width}.png`)});
    }
  } finally {await context.close();}
});
