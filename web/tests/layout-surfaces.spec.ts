import { test, expect } from "@playwright/test";
import path from "node:path";
const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS || "temp/browser-artifacts";
test("composite surfaces retain layout, icons and workspace return navigation", async ({page}) => {
  await page.setViewportSize({width:882,height:962});
  await page.goto("/");
  await page.getByRole("link",{name:"使用账号登录"}).click();
  await page.getByRole("link",{name:"以 Alice 登录"}).click();
  await page.getByRole("button", {name:/^打开网盘 /}).first().click();
  await page.locator("input[type=file]").setInputFiles(Array.from({length:5}, (_,i)=>({name:`设计参考-长文件名布局验证-${i}.txt`,mimeType:"text/plain",buffer:Buffer.from("Layout regression fixture")})));
  await expect(page.getByRole("button", {name:"详情 设计参考-长文件名布局验证-4.txt",exact:true})).toBeVisible();
  for (const theme of ["light","dark"] as const) {
    await page.emulateMedia({colorScheme:theme});
    for(const width of [882,1440,390]) {
      await page.setViewportSize({width,height:962});
      if (width < 768) await page.getByRole("navigation", {name:"手机主导航"}).getByRole("button", {name:"文件",exact:true}).click();
      else await page.getByRole("button",{name:"我的网盘",exact:true}).click();
      await page.getByRole("button",{name:"卡片视图",exact:true}).click();
      const card = page.locator("button.drive-card").first();
      await expect(card).toBeVisible();
      const heading = (await card.locator("h2").boundingBox())!;
      const footer = (await card.locator(".drive-card-footer").boundingBox())!;
      const bounds = (await card.boundingBox())!;
      expect(bounds.height).toBeGreaterThan(180);
      expect(footer.y).toBeGreaterThan(heading.y + heading.height);
      expect(footer.x + footer.width).toBeLessThanOrEqual(bounds.x + bounds.width + 1);
      if (width < 768) await expect(page.locator(".sidebar")).toHaveCount(0);
      else expect(await page.locator(".sidebar").evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
      if(width >= 768) {
        const items = await page.locator(".nav-item > svg").evaluateAll(nodes => nodes.map(n => n.getBoundingClientRect().x));
        expect(Math.max(...items)-Math.min(...items)).toBeLessThan(2);
      }
      await page.screenshot({path:path.join(artifacts,`surfaces-drives-${theme}-${width}.png`)});
      if (width < 768) {
        await page.getByRole("button", {name:"打开全部导航"}).click();
        await page.getByRole("dialog", {name:"全部导航"}).getByRole("button", {name:"资源态势",exact:true}).click();
      } else await page.getByRole("button",{name:"资源态势",exact:true}).click();
      const metrics=page.locator(".metric-card");
      await expect(metrics).toHaveCount(4);
      const heights=await metrics.evaluateAll(nodes=>nodes.map(n=>n.getBoundingClientRect().height));
      expect(Math.max(...heights)-Math.min(...heights)).toBeLessThan(3);
      expect(Math.min(...heights)).toBeGreaterThanOrEqual(96);
      expect(Math.max(...heights)).toBeLessThanOrEqual(150);
      const row=page.locator(".workspace-capacity-row").first();
      const panel=(await row.locator("..").boundingBox())!;
      const rowBox=(await row.boundingBox())!;
      expect(rowBox.x+rowBox.width).toBeLessThanOrEqual(panel.x+panel.width);
      expect(await page.evaluate(()=>document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      const recent=page.locator(".recent-media > button");
      await expect(recent).toHaveCount(5);
      const boxes=await recent.evaluateAll(nodes=>nodes.map(el=>({height:el.getBoundingClientRect().height,overflow:el.scrollWidth>el.clientWidth})));
      expect(boxes.every(b=>b.height>=48&&b.height<=64&&!b.overflow)).toBe(true);
      await page.screenshot({path:path.join(artifacts,`surfaces-overview-${theme}-${width}.png`),fullPage:true});
      if (width < 768) await page.getByRole("navigation", {name:"手机主导航"}).getByRole("button", {name:"文件",exact:true}).click();
      else await page.getByRole("button",{name:"返回我的工作空间"}).click();
      await expect(page.locator(".drive-home")).toBeVisible();
    }
  }
});

test("all workspace pages keep their shell inside the viewport", async ({page}) => {
  await page.goto("/");
  await page.locator("#login-key").fill("dbu_"+"2".repeat(96));
  await page.getByRole("button",{name:"登录",exact:true}).click();
  await expect(page.locator(".sidebar")).toBeVisible();
  for(const width of [1248,882,390]) {
    await page.setViewportSize({width,height:963});
    for(const route of ["drives","insights","transfers","shares","teams","trash","connections","agents","users","settings"]) {
      await page.goto('/#/'+route);
      await page.waitForLoadState("networkidle");
      if (width < 768) {
        await expect(page.locator(".sidebar")).toHaveCount(0);
        await expect(page.getByRole("navigation", {name:"手机主导航"})).toBeVisible();
      } else {
        await expect(page.locator(".sidebar")).toBeVisible();
        expect(await page.locator(".sidebar").evaluate(el=>el.scrollWidth<=el.clientWidth),`${route} sidebar at ${width}`).toBe(true);
      }
      expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),`${route} page at ${width}`).toBe(true);
      if(width>=768){const meter=(await page.locator(".sidebar .storage-meter").boundingBox())!;expect(meter.height).toBeLessThan(100);}
      await page.screenshot({path:path.join(artifacts,`review-${route}-${width}.png`)});
    }
  }
});
