import { test, expect } from "@playwright/test";
import path from "node:path";

test("management page deep links, drawer recovery and tenant isolation", async ({page}) => {
  test.skip(process.env.DRIVEBRIDGE_TEST_TEAM_DEMO !== "1", "Requires disposable team demo");
  await page.goto("/");
  await page.getByRole("button",{name:"无需账号，进入本地体验"}).click();
  await page.getByRole("button",{name:"管理与授权",exact:true}).click();
  await expect(page.getByRole("region",{name:"管理与授权中心"})).toBeVisible();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await page.getByRole("tab",{name:"租户与权限",exact:true}).click();
  await expect(page).toHaveURL(/tab=teams/);
  await page.reload();
  await expect(page.getByRole("tab",{name:"租户与权限",exact:true})).toHaveAttribute("aria-selected","true");
  const trigger=page.getByRole("region",{name:"管理与授权中心"}).getByRole("button",{name:"创建租户",exact:true});
  for (const theme of ["light","dark"]) for(const width of [1440,1280,1213,957,882,872,390]) {
    await page.setViewportSize({width,height:963});
    await page.evaluate(theme=>document.documentElement.classList.toggle("dark",theme==="dark"),theme);
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
    const tabs=await page.getByRole("tablist",{name:"管理页面"}).boundingBox();
    const panel=await page.getByRole("tabpanel").boundingBox();
    expect(panel!.y).toBeGreaterThanOrEqual(tabs!.y+tabs!.height);
    const content=await page.locator(".content").boundingBox();
    expect(panel!.x-content!.x).toBeLessThanOrEqual(32);
    await page.screenshot({animations:"disabled",path:path.join(process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!,`after-governance-${theme}-${width}.png`)});
    await trigger.click();
    const sheet=page.getByRole("dialog",{name:"创建租户",exact:true});
    await expect(sheet).toBeVisible();
    await expect.poll(async()=>Math.round((await sheet.boundingBox())!.width)).toBe(Math.min(width,480));
    expect(await sheet.evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBe(true);
    await page.screenshot({animations:"disabled",path:path.join(process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!,`after-form-${theme}-${width}.png`)});
    await page.keyboard.press("Escape");
    await expect(sheet).toBeHidden();
    await expect(trigger).toBeFocused();
  }
  await page.getByRole("tab",{name:"申请与审批",exact:true}).click();
  await page.getByRole("button",{name:"提交修改申请",exact:true}).click();
  const sheet=page.getByRole("dialog",{name:"提交修改申请",exact:true});
  await sheet.getByLabel("申请原因（勿填写凭据）").fill("保留这段输入以便失败后重试");
  await page.route("**/api/v1/governance/requests",route=>route.fulfill({status:403,contentType:"application/json",body:JSON.stringify({error:{code:"permission_denied"}})}));
  await sheet.getByRole("button",{name:"提交修改申请",exact:true}).click();
  await expect(sheet.getByRole("alert")).toBeVisible();
  await expect(sheet.getByLabel("申请原因（勿填写凭据）")).toHaveValue("保留这段输入以便失败后重试");
  await page.unroute("**/api/v1/governance/requests");
  await page.keyboard.press("Escape");
  await page.getByRole("tab",{name:"访问记录",exact:true}).click();
  await page.goBack();
  await expect(page.getByRole("tab",{name:"申请与审批",exact:true})).toHaveAttribute("aria-selected","true");
  await page.getByRole("button",{name:"提交修改申请",exact:true}).click();
  await sheet.getByLabel("申请原因（勿填写凭据）").fill("不能带到另一租户的输入");
  await page.evaluate(()=>{location.hash="#/governance?tab=notifications&tenant=not-authorized";});
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page.getByText("不能带到另一租户的输入")).toHaveCount(0);
});

test("user directory, permission matrix and storage forms keep breathing room", async ({page}) => {
  test.skip(process.env.DRIVEBRIDGE_TEST_TEAM_DEMO !== "1", "Requires disposable team demo");
  test.setTimeout(120000);
  await page.goto("/");
  await page.getByRole("button",{name:"无需账号，进入本地体验"}).click();
  await expect(page.locator(".drive-home")).toBeVisible();
  const artifacts=process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!;
  for(const theme of ["light","dark"]) for(const width of [1440,1280,1213,957,882,872,390]) {
    await page.setViewportSize({width,height:963});
    await page.evaluate(theme=>document.documentElement.classList.toggle("dark",theme==="dark"),theme);
    const navigate = async (name: string) => {
      if (width < 768) {
        await page.getByRole("button", { name: "打开全部导航" }).click();
        await page.getByRole("dialog", { name: "全部导航" }).getByRole("button", { name, exact: true }).click();
      } else {
        await page.locator(".sidebar").getByRole("button", { name, exact: true }).click();
      }
    };
    await navigate("访问凭据");
    await page.getByRole("tab",{name:"用户",exact:true}).click();
    const directory=page.getByRole("region",{name:"用户目录",exact:true});
    await expect(directory.getByRole("row").filter({hasText:"本地体验"})).toBeVisible();
    const inspect = async (selector: string) => page.locator(selector).evaluateAll(rows=>rows.map(row=>{
      const td=row.querySelector("td")!;const css=getComputedStyle(td);const style=getComputedStyle(row);
      return {height:row.getBoundingClientRect().height,top:parseFloat(css.paddingTop),left:parseFloat(css.paddingLeft),line:style.borderBottomColor,text:style.color};
    }));
    const users=await inspect('[aria-label="用户目录"] tbody tr');
    expect(users.length).toBeGreaterThan(0);
    for(const row of users){expect(row.height).toBeGreaterThanOrEqual(64);expect(row.top).toBeGreaterThanOrEqual(8);expect(row.left).toBeGreaterThanOrEqual(12);expect(row.line).not.toBe(row.text);}
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
    await page.screenshot({animations:"disabled",path:path.join(artifacts,`after-users-${theme}-${width}.png`)});
    await navigate("团队");
    await page.getByRole("button",{name:"打开团队 设计协作团队",exact:true}).click();
    await page.getByRole("link",{name:"权限",exact:true}).click();
    await expect(page.getByRole("link",{name:"权限",exact:true})).toHaveAttribute("aria-current","page");
    await expect(page.getByRole("combobox",{name:"团队成员 在 团队素材 的权限"})).toBeVisible();
    const permissions=await inspect('.team-body tbody tr');
    for(const row of permissions){expect(row.height).toBeGreaterThanOrEqual(48);expect(row.top).toBeGreaterThanOrEqual(8);expect(row.line).not.toBe(row.text);}
    await expect(page.locator(".team-body").getByRole("button",{name:"下一页",exact:true})).toHaveCount(0);
    await page.screenshot({animations:"disabled",path:path.join(artifacts,`after-permissions-${theme}-${width}.png`)});
    await page.getByRole("link",{name:"设置",exact:true}).click();
    await page.getByRole("button",{name:"分配已有存储",exact:true}).click();
    const dialog=page.getByRole("dialog",{name:"分配已有存储",exact:true});
    const field=await dialog.getByRole("combobox",{name:"存储连接",exact:true}).boundingBox();
    const cancel=await dialog.getByRole("button",{name:"取消",exact:true}).boundingBox();
    const submit=await dialog.getByRole("button",{name:"保存并继续",exact:true}).boundingBox();
    expect(cancel!.y-field!.y-field!.height).toBeGreaterThanOrEqual(24);
    await expect(dialog.locator(".dialog-actions")).toHaveCSS("column-gap","8px");
    await expect.poll(async()=>{
      const a=await dialog.getByRole("button",{name:"取消",exact:true}).boundingBox();
      const b=await dialog.getByRole("button",{name:"保存并继续",exact:true}).boundingBox();
      return Math.round((b!.x-a!.x-a!.width)*100)/100;
    }).toBeGreaterThanOrEqual(8);
    expect(await dialog.evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBe(true);
    await page.screenshot({animations:"disabled",path:path.join(artifacts,`after-storage-form-${theme}-${width}.png`)});
    await dialog.getByRole("button",{name:"取消",exact:true}).click();
    await expect(dialog).toBeHidden();
  }
});
