import { selectChoice } from "./helpers/select";
import { test, expect, type Page } from "@playwright/test";
import path from "node:path";
async function login(page: Page, admin: boolean) {
  await page.goto("/");
  await page
    .locator("#login-key")
    .fill("dbu_" + (admin ? "22" : "11").repeat(48));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(page.getByLabel("切换租户")).toBeVisible();
}

test("partial and interrupted receipts preserve results without replaying the request", async ({page}) => {
  test.skip(!test.info().config.configFile?.includes("governance"), "Requires governance fixture");
  await login(page,true);
  const state=(await(await page.request.get("/api/v1/governance")).json()).data;
  let status="partial";
  let writes=0;
  page.on("request",request=>{if(request.method()!=="GET" && request.url().includes("/governance/")) writes++;});
  await page.route("**/api/v1/governance",route=>route.fulfill({json:{data:{...state,requests:[{
    id:"receipt-layout-fixture",requester:state.actor_id,space_id:state.spaces[0].id,kind:"exact",status,
    reason:"分项执行回执"+"长中文原因".repeat(20),grant_until:"2026-09-23T12:00:00Z",expires_at:"2026-09-23T12:00:00Z",
    items:[{file_id:"file-success",version_id:"version-1",action:"trash",status:"succeeded"},{file_id:"file-failure",version_id:"version-2",action:"trash",status:"failed",error:"permission_denied"}],
  }]}}}));
  for(const next of ["partial","interrupted"]) {
    status=next;
    await page.goto(`/#/governance?tab=requests&tenant=${state.tenant_id}`);
    await page.reload();
    const trigger=page.getByRole("button",{name:"查看申请",exact:true});
    await trigger.click();
    const sheet=page.getByRole("dialog",{name:"申请详情",exact:true});
    await expect(sheet).toContainText(next==="partial"?"部分失败":"执行中断");
    await expect(sheet).toContainText("成功");
    await expect(sheet).toContainText("失败");
    await expect(sheet.getByRole("status")).toContainText("不要重复执行原清单");
    await expect(sheet.getByRole("button",{name:"确认执行清单",exact:true})).toHaveCount(0);
    await page.screenshot({path:path.join(process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!,`governance-${next}-receipt.png`)});
    await page.keyboard.press("Escape");
    await expect(sheet).toBeHidden();
    await expect(trigger).toBeFocused();
  }
  expect(writes).toBe(0);
});

test("an approved exact request waits for the requester to explicitly execute", async ({browser}) => {
  test.skip(!test.info().config.configFile?.includes("governance"), "Requires governance fixture");
  const ownerContext=await browser.newContext(), adminContext=await browser.newContext();
  const owner=await ownerContext.newPage(), admin=await adminContext.newPage();
  try {
    await login(owner,false);
    await owner.getByRole("button",{name:"打开网盘 e2e-key-user personal",exact:true}).click();
    await owner.locator("input[type=file]").setInputFiles({name:"explicit-approval.txt",mimeType:"text/plain",buffer:Buffer.from("Keep until explicit execution")});
    await expect(owner.getByRole("button",{name:"详情 explicit-approval.txt",exact:true})).toBeVisible();
    const spaces=(await(await owner.request.get("/api/v1/spaces")).json()).data;
    const target=spaces.find((s:{name:string})=>s.name==="e2e-key-user personal");
    await login(admin,true);
    await admin.getByRole("button",{name:"工作区选项",exact:true}).click();
    await selectChoice(admin.getByLabel("切换管理空间"),{label:"e2e-key-user personal · 管理只读"});
    await admin.getByRole("button",{name:"管理与授权",exact:true}).click();
    await admin.getByRole("button",{name:"提交修改申请",exact:true}).click();
    const form=admin.getByRole("dialog",{name:"提交修改申请",exact:true});
    await selectChoice(form.getByLabel("授权方式"),"exact");
    await form.getByLabel("申请原因（勿填写凭据）").fill("明确执行验收");
    await form.getByRole("checkbox",{name:/explicit-approval.txt/}).check();
    await form.getByRole("button",{name:"提交修改申请",exact:true}).click();
    await expect(form).toBeHidden();
    await owner.getByRole("button",{name:"管理与授权",exact:true}).click();
    await owner.getByRole("row").filter({hasText:"明确执行验收"}).getByRole("button",{name:"查看申请"}).click();
    await owner.getByRole("button",{name:"批准申请",exact:true}).click();
    await expect(owner.getByRole("dialog",{name:"申请详情"})).toContainText("已授权");
    const before=(await(await owner.request.get(`/api/v1/files?space_id=${target.id}`)).json()).data;
    expect(before.some((f:{path:string})=>f.path==="explicit-approval.txt")).toBe(true);
    await admin.getByRole("row").filter({hasText:"明确执行验收"}).getByRole("button",{name:"查看申请"}).click();
    await expect(admin.getByRole("button",{name:"确认执行清单",exact:true})).toBeVisible({timeout:20000});
    await admin.getByRole("button",{name:"确认执行清单",exact:true}).click();
    await expect(admin.getByRole("dialog",{name:"申请详情"})).toContainText("已完成");
    const trashed=(await(await owner.request.get(`/api/v1/files?space_id=${target.id}&trash=true`)).json()).data;
    expect(trashed.some((f:{path:string})=>f.path==="explicit-approval.txt")).toBe(true);
    await expect(admin.getByRole("button",{name:"确认执行清单",exact:true})).toHaveCount(0);
  } finally { await ownerContext.close();await adminContext.close(); }
});
test("governance read-only access, owner approval and notifications", async ({
  browser,
  baseURL,
}) => {
  test.skip(
    !test.info().config.configFile?.includes("governance"),
    "Use the dedicated governance fixture config.",
  );
  const ownerContext = await browser.newContext(),
    adminContext = await browser.newContext();
  const owner = await ownerContext.newPage(),
    admin = await adminContext.newPage();
  try {
    await login(owner, false);
    await login(admin, true);
    await admin.getByRole("button", { name: "工作区选项", exact: true }).click();
    await selectChoice(admin
      .getByLabel("切换管理空间"),{ label: "e2e-key-user personal · 管理只读" });
    await expect(
      admin.getByRole("status").filter({ hasText: "管理只读" }),
    ).toBeVisible();
    await expect(
      admin.getByRole("button", { name: "新建", exact: true }),
    ).toBeDisabled();
    await admin.getByRole("button", { name: "工作区选项", exact: true }).click();
    await admin.getByRole("button", { name: /管理与授权中心/ }).click();
    await admin.getByRole("button",{name:"提交修改申请",exact:true}).click();
    await admin.getByLabel("申请原因（勿填写凭据）").fill("协助整理文件目录");
    await admin.getByRole("dialog",{name:"提交修改申请",exact:true})
      .getByRole("button", { name: "提交修改申请", exact: true })
      .click();
    await expect(
      admin.getByText("申请已提交，批准后仍需由你确认执行。"),
    ).toBeVisible();
    await owner.getByRole("button", { name: "工作区选项", exact: true }).click();
    await owner.getByRole("button", { name: /管理与授权中心/ }).click();
    await owner.getByRole("row").filter({hasText:"协助整理文件目录"}).getByRole("button",{name:"查看申请"}).click();
    await expect(
      owner.getByRole("button", { name: "批准申请", exact: true }),
    ).toBeVisible({ timeout: 20000 });
    await owner.getByRole("button", { name: "批准申请", exact: true }).click();
    await expect(
      owner.getByText(/e2e-key-user personal · 已授权/),
    ).toBeVisible();
    await expect(admin.getByRole("row").filter({hasText:"协助整理文件目录"})).toContainText("已授权",
      { timeout: 20000 },
    );
    await owner.keyboard.press("Escape");
    await owner.getByRole("tab", { name: "通知", exact: true }).click();
    await expect(owner.getByText("管理员查看", { exact: true })).toBeVisible();
    await owner
      .getByRole("tab", { name: "申请与审批", exact: true })
      .click();
    await owner.getByRole("row").filter({hasText:"协助整理文件目录"}).getByRole("button",{name:"查看申请"}).click();
    await owner.getByRole("button", { name: "撤销", exact: true }).click();
    await expect(admin.getByRole("row").filter({hasText:"协助整理文件目录"})).toContainText("已撤销",
      { timeout: 20000 },
    );
    // The HTTP boundary must reject a write even if a stale UI still shows controls.
    const me = await admin.request.get(baseURL + "/api/v1/me");
    const identity = (await me.json()).data;
    const overview = (
      await (await admin.request.get(baseURL + "/api/v1/governance")).json()
    ).data;
    const target = overview.spaces.find(
      (s: { name: string }) => s.name === "e2e-key-user personal",
    );
    const denied = await admin.request.post(baseURL + "/api/v1/directories", {
      headers: {
        Origin: baseURL!,
        "X-CSRF-Token": identity.csrf_token,
        "X-DriveBridge-Tenant": overview.tenant_id,
        "X-DriveBridge-Space": target.id,
      },
      data: { space_id: target.id, path: "not-authorized" },
    });
    expect(denied.status()).toBe(403);
  } finally {
    await ownerContext.close();
    await adminContext.close();
  }
});

test("mobile recovery and account exit remain reachable", async ({ page }) => {
  test.skip(
    !test.info().config.configFile?.includes("governance"),
    "Use the dedicated governance fixture config.",
  );
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await page
    .getByRole("button", { name: "激活或恢复账号", exact: true })
    .click();
  await expect(
    page.getByRole("dialog", { name: "欢迎使用 DriveBridge" }),
  ).toBeVisible();
  const activation = page.getByLabel("一次性激活码");
  await expect(activation).toBeVisible();
  expect((await activation.boundingBox())!.width).toBeGreaterThan(200);
  await expect(page.getByRole("dialog").locator("details")).toHaveCount(0);
  await page.getByRole("tab", { name: "恢复已有账号" }).click();
  await expect(page.getByLabel("本人账号 ID")).toBeVisible();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "关闭", exact: true })
    .click();
  await login(page, true);
  await page.getByRole("button", { name: "工作区选项", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "退出当前账号", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "退出当前账号", exact: true }).click();
  await expect(page.locator("#login-key")).toBeVisible();
});

test("team member selection and drive binding appear in the unified workspace", async ({
  page,
}) => {
  test.skip(
    !test.info().config.configFile?.includes("governance"),
    "Requires the governance fixture",
  );
  await login(page, true);
  await page.getByRole("button", { name: "访问凭据", exact: true }).click();
  await page.getByRole("tab", { name: "用户", exact: true }).click();
  await page
    .getByRole("button", { name: "租户设置", exact: true })
    .click();
  await expect(page.getByRole("tab",{name:"租户与权限",exact:true})).toHaveAttribute("aria-selected","true");
  await page.getByRole("button",{name:"创建团队",exact:true}).click();
  const form=page.getByRole("dialog",{name:"创建团队",exact:true});
  await form.getByLabel("团队名称").fill("设计协作组");
  await selectChoice(form.getByLabel("团队管理员"),{labelContains:"e2e-key-user"});
  await form.getByRole("button",{name:"创建团队",exact:true}).click();
  const row=page.getByRole("row").filter({hasText:"设计协作组"});
  await row.getByRole("link",{name:"管理成员与网盘"}).click();
  await expect(page.getByRole("row").filter({hasText:"e2e-key-user"})).toBeVisible();
  await page.getByRole("link",{name:"网盘",exact:true}).click();
  await page.getByRole("button",{name:"创建团队网盘",exact:true}).click();
  const spaceForm=page.getByRole("dialog",{name:"创建团队网盘",exact:true});
  await spaceForm.getByLabel("网盘名称").fill("团队设计素材");
  await selectChoice(spaceForm.getByLabel("数据负责人",{exact:true}),{labelContains:"e2e-key-user"});
  await selectChoice(spaceForm.getByLabel("存储连接",{exact:true}),{labelContains:"平台"});
  await spaceForm.getByRole("button",{name:"保存并继续",exact:true}).click();
  await expect(page.locator("button.drive-card").filter({hasText:"团队设计素材"})).toBeVisible();
  await page.getByRole("button", { name: "我的网盘", exact: true }).click();
  await page.getByRole("button", { name: "团队网盘", exact: true }).click();
  await page.getByRole("button", { name: "打开网盘 团队设计素材" }).click();
  await expect(
    page.getByRole("button", { name: "新建", exact: true }),
  ).toBeDisabled();
  await expect(
    page.getByRole("status").filter({ hasText: "管理只读" }),
  ).toBeVisible();
});
