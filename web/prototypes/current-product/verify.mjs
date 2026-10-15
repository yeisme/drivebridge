import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';import path from 'node:path';
const base=process.env.DRIVEBRIDGE_REPLICA_URL||'http://127.0.0.1:32016/';
const artifacts=process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS||path.resolve(import.meta.dirname,'../../../temp/opendesign-replica/screenshots');fs.mkdirSync(artifacts,{recursive:true});
const browser=await chromium.launch({headless:true,executablePath:process.env.PLAYWRIGHT_CHROMIUM||'/data2/yeshugen/cache/ms-playwright/chromium-1243/chrome-linux64/chrome'});
const page=await browser.newPage();page.setDefaultTimeout(15000);const errors=[],apiRequests=[];page.on('pageerror',e=>errors.push(e.message));page.on('request',r=>{if(new URL(r.url()).pathname.startsWith('/api/v1/')||new URL(r.url()).pathname.startsWith('/auth/'))apiRequests.push(new URL(r.url()).pathname);});
const routes=[['home','我的工作台'],['insights','资源态势'],['files','我的网盘'],['knowledge','知识库'],['resources','资源库'],['collections','项目集合'],['properties','标签与属性'],['search','搜索'],['flows','文件流程'],['tasks','任务中心'],['shares','分享交付'],['teams','团队'],['trash','回收站'],['storage','存储管理'],['devices','设备与离线'],['automation','备份与整理'],['capabilities','能力与连接说明'],['inbox','产物与收件'],['agents','Agent 管理'],['members','访问凭据'],['governance','管理与授权'],['settings','个人设置']];
const evidence={source:{url:base,kind:'current-product-prototype',data:'isolated-browser'},pages:[],flows:[],apiRequests,errors};
try{
 await page.goto(base);await page.getByRole('button',{name:'账户菜单'}).waitFor();
 for(const width of [1440,390]){
  await page.setViewportSize({width,height:900});await page.emulateMedia({colorScheme:'dark',reducedMotion:'reduce'});
  for(const [route,title] of routes){
   await page.goto(base+'#/'+route);await expect(page.locator('.breadcrumb')).toContainText(title);await expect(page.getByText('正在加载页面…',{exact:true})).toHaveCount(0);await page.waitForTimeout(150);
   expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),route+' width '+width).toBe(true);
   await expect(page.getByText('此页面暂时无法显示',{exact:true})).toHaveCount(0);
   await page.screenshot({path:path.join(artifacts,`${route}-${width}.png`),animations:'disabled'});evidence.pages.push({route,width,title});console.log('verified_page',route,width);
  }
 }
 await page.setViewportSize({width:1440,height:900});
 const sid=await page.evaluate(()=>window.__DRIVEBRIDGE_PROTOTYPE__.state().spaces[0].id);
 await page.goto(base+'#/drive/'+sid);const file=page.locator('.finder-file').filter({hasText:'产品说明.md'});await expect(file).toBeVisible();
 await file.click({button:'right'});const menu=page.getByRole('menu',{name:'文件操作菜单'});const xs=await menu.locator('.menu-label').evaluateAll(nodes=>nodes.map(n=>n.getBoundingClientRect().x));expect(Math.max(...xs)-Math.min(...xs)).toBeLessThan(2);await page.screenshot({path:path.join(artifacts,'file-menu.png')});await page.keyboard.press('Escape');
 await file.locator('.finder-name').dragTo(page.locator('.finder-folder').filter({hasText:'归档'}).locator('.finder-name'));await expect(file).toHaveCount(0);await page.reload();await page.getByRole('button',{name:'打开文件夹 归档',exact:true}).click();await expect(page.locator('.finder-file').filter({hasText:'产品说明.md'})).toBeVisible();evidence.flows.push('context-menu-alignment-and-drag-persistence');
 const chooser=page.waitForEvent('filechooser');await page.getByRole('button',{name:'上传文件',exact:true}).click();await(await chooser).setFiles({name:'原型上传.txt',mimeType:'text/plain',buffer:Buffer.from('Only prototype state changes.')});await expect(page.locator('.finder-file').filter({hasText:'原型上传.txt'})).toBeVisible();evidence.flows.push('upload');
 await page.goto(base+'#/search');await page.getByRole('searchbox',{name:'检索条件'}).fill('产品说明');await page.getByRole('button',{name:'检索',exact:true}).click();await expect(page.getByRole('table',{name:'精确元数据结果'})).toBeVisible();evidence.flows.push('exact-search');
 await page.goto(base+'#/knowledge');await expect(page.getByRole('button',{name:'产品说明.md',exact:true})).toBeVisible();await page.getByRole('button',{name:'产品说明.md',exact:true}).dblclick();await expect(page.getByRole('region',{name:'产品说明.md',exact:true})).toBeVisible();await page.screenshot({path:path.join(artifacts,'knowledge-open.png')});evidence.flows.push('markdown-notebook');
 const notebook=page.getByRole('region',{name:'产品说明.md',exact:true});await notebook.getByRole('button',{name:'源码',exact:true}).click();await notebook.locator('.cm-content').fill('# 原型编辑验证\n\n只修改原型中的保存版本。');await notebook.getByRole('button',{name:'预览保存',exact:true}).click();await page.getByRole('dialog',{name:'确认文档修改'}).getByRole('button',{name:'确认保存新版本',exact:true}).click();await expect(page.getByRole('heading',{name:'原型编辑验证',exact:true})).toBeVisible();evidence.flows.push('markdown-edit-and-version-save');
 await page.goto(base+'#/agents');await page.getByRole('button',{name:'接入 Agent',exact:true}).click();const dialog=page.getByRole('dialog',{name:'接入 Agent',exact:true});await dialog.getByRole('textbox',{name:'Agent 名称'}).fill('原型接入助手');await dialog.getByRole('combobox',{name:'授权空间'}).click();await page.getByRole('option',{name:'我的创作空间',exact:true}).click();await dialog.getByRole('button',{name:'确认创建授权'}).click();await expect(dialog.getByText('已确认授权：原型接入助手')).toBeVisible();await dialog.getByRole('button',{name:'已安全保存凭据'}).click();await page.reload();await expect(page.getByRole('heading',{name:'原型接入助手',exact:true})).toBeVisible();evidence.flows.push('agent-create-and-readback');
 await page.emulateMedia({colorScheme:'light',reducedMotion:'reduce'});
 for(const route of ['home','files','knowledge','agents','settings']){await page.goto(base+'#/'+route);await page.waitForTimeout(100);await page.screenshot({path:path.join(artifacts,route+'-light.png'),animations:'disabled'});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);}evidence.flows.push('light-theme');
 await page.getByRole('button',{name:'原型场景',exact:true}).click();await page.getByRole('button',{name:'只读协作',exact:true}).click();await page.goto(base+'#/drive/'+sid+'?scope=managed');await expect(page.getByRole('button',{name:'上传文件',exact:true})).toBeDisabled();await page.getByRole('button',{name:'打开文件夹 归档',exact:true}).click();expect(await page.locator('.finder-file').first().getAttribute('draggable')).toBe('false');evidence.flows.push('readonly-boundary');
 await page.getByRole('button',{name:'原型场景',exact:true}).click();await page.getByRole('button',{name:'空空间',exact:true}).click();await page.goto(base+'#/files');await expect(page.locator('button.drive-card')).toHaveCount(0);evidence.flows.push('empty-state');
 expect(errors).toEqual([]);expect(apiRequests).toEqual([]);
 evidence.unhandled=await page.evaluate(()=>window.__DRIVEBRIDGE_PROTOTYPE__.diagnostics.unhandled);
 fs.writeFileSync(path.join(artifacts,'verification.json'),JSON.stringify(evidence,null,2));console.log('pages',evidence.pages.length,'flows',evidence.flows.length,'unhandled',JSON.stringify(evidence.unhandled));
}finally{await browser.close();}
