import { selectChoice } from "./helpers/select";
import {test,expect} from "@playwright/test";
import path from "node:path";

test("highest administrator confirms by name and takes over the last custodian",async({page})=>{
  test.skip(process.env.DRIVEBRIDGE_TEST_TEAM_DEMO!=="1","Requires isolated team demo");
  await page.goto("/");
  await page.getByRole("button",{name:"无需账号，进入本地体验"}).click();
  await page.getByRole("button",{name:"访问凭据",exact:true}).click();
  await page.getByRole("tab",{name:"用户",exact:true}).click();
  const row=page.getByRole("row").filter({hasText:"素材负责人"});
  await row.getByRole("button",{name:"删除并归档 素材负责人",exact:true}).click();
  const modal=page.getByRole("dialog",{name:"删除并归档",exact:true});
  await expect(modal).toContainText("由最高管理员接管后继续");
  await expect(modal).toContainText("团队素材 · 网盘数据负责人");
  await modal.getByLabel("输入目标用户名确认").fill("错误的名字");
  await expect(modal.getByRole("button",{name:"确认删除并归档",exact:true})).toBeDisabled();
  await modal.getByLabel("输入目标用户名确认").fill("素材负责人");
  await expect(modal.getByRole("button",{name:"确认删除并归档",exact:true})).toBeDisabled();
  await modal.getByRole("checkbox", { name: "由我接管这些职责并继续" }).check();
  const artifacts=process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS||"temp/browser-artifacts";
  await page.screenshot({path:path.join(artifacts,"admin-name-takeover.png"),fullPage:true});
  await modal.getByRole("button",{name:"确认删除并归档",exact:true}).click();
  await expect(modal).toHaveCount(0);
  await selectChoice(page.getByLabel("筛选账号状态"),"archived");
  await expect(row).toContainText("已归档");
  const role=await page.evaluate(async()=>{
    const data=(await(await fetch("/api/v1/governance")).json()).data;
    const space=data.spaces.find((s:{name:string})=>s.name==="团队素材");
    return data.space_roles[space.id];
  });
  expect(role).toBe("custodian");
  await page.getByRole("button",{name:"工作区选项",exact:true}).click();
  await expect(page.locator(".context-identity")).toContainText("本地体验");
  await page.screenshot({path:path.join(artifacts,"workspace-menu-polished.png"),fullPage:true});
});
