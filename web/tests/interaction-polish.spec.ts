import {test,expect} from '@playwright/test';
import path from 'node:path';
const baseline=process.env.DRIVEBRIDGE_POLISH_BASELINE==='1';
const artifact=(name:string)=>path.join(process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!,`${baseline?'before':'after'}-${name}.png`);

test('filters, audit, workspace menu, keys and settings stay readable',async({page})=>{
 test.skip(process.env.DRIVEBRIDGE_TEST_TEAM_DEMO!=='1','Requires disposable team demo');
 test.setTimeout(120000);
 await page.goto('/');
 await page.getByRole('button',{name:'无需账号，进入本地体验'}).click();
 await expect(page.getByRole('button',{name:'打开网盘 体验空间',exact:true})).toBeVisible();
 const me=(await(await page.request.get('/api/v1/me')).json()).data;
 const tenant=(await(await page.request.get('/api/v1/governance')).json()).data.tenant_id;
 if(!baseline) {
  await page.evaluate(async()=>{
   const me=(await(await fetch('/api/v1/me')).json()).data;
   const response=await fetch('/api/v1/key-directory',{method:'POST',headers:{'Content-Type':'application/json','X-CSRF-Token':me.csrf_token},body:JSON.stringify({name:'团队预览与长期协作设备名称'.repeat(3),days:30})});
   if(!response.ok) throw new Error('Fixture key creation failed');
  });
 }
 for(const theme of (baseline?['dark']:['light','dark'])) for(const width of (baseline?[1186]:[1440,1186,957,390])) {
  await page.setViewportSize({width,height:1040});
  await page.emulateMedia({colorScheme:theme==='dark'?'dark':'light'});
  const go=async(route:string)=>{await page.evaluate(hash=>{location.hash=hash;},`#/${route}${route.includes('?')?'&':'?'}tenant=${tenant}`);};
  await go('insights?view=robots');
  await expect(page.locator('.insight-kpis')).toBeVisible();
  const before=await page.locator('.insight-kpis').boundingBox();
  const trigger=baseline?page.locator('.insight-advanced summary'):page.getByRole('button',{name:'更多筛选',exact:true});
  await trigger.click();
  if(!baseline){
   const popup=page.getByRole('dialog',{name:'高级筛选',exact:true});
   await expect(popup).toBeVisible();
   const box=await popup.boundingBox();
   expect(box!.x).toBeGreaterThanOrEqual(0);expect(box!.x+box!.width).toBeLessThanOrEqual(width);
   expect((await page.locator('.insight-kpis').boundingBox())!.y).toBe(before!.y);
   const connection=popup.getByRole('combobox',{name:'存储连接筛选',exact:true});
   await connection.click();
   await page.getByRole('option').last().click();
   await expect(connection).not.toHaveAttribute('data-value','');
   await connection.click();
   await page.getByRole('option',{name:'全部连接',exact:true}).click();
   await expect(connection).toHaveAttribute('data-value','');
   await page.screenshot({path:artifact(`filters-${theme}-${width}`),animations:'disabled'});
   await popup.getByRole('button',{name:'完成',exact:true}).press('Escape');await expect(popup).toBeHidden();await expect(trigger).toBeFocused();
  }else{await page.screenshot({path:artifact(`filters-${theme}-${width}`)});await trigger.click();}
  await page.getByRole('button',{name:'工作区选项',exact:true}).click();
  const menu=baseline?page.locator('[data-slot=popover-content]'):page.getByRole('dialog',{name:'工作区菜单',exact:true});
  await expect(menu).toBeVisible();
  if(!baseline){expect(await menu.evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBe(true);}
  await page.screenshot({path:artifact(`workspace-menu-${theme}-${width}`),animations:'disabled'});
  await page.keyboard.press('Escape');await expect(menu).toBeHidden();
  await go('insights?view=audit');
  const events=page.getByRole('region',{name:'记录',exact:true});
  await expect(events.getByRole('row').nth(1)).toBeVisible();
  if(!baseline){
   const action=events.locator('[title="governance.overview"]').first();
   await expect(action).toHaveText('查看管理概览');
   const row=events.getByRole('row').filter({has:page.locator('[title="governance.overview"]')}).first();
   await expect(row).toContainText('本地体验');await expect(row).not.toContainText(me.user.id);
   await expect(row.locator(`[title="${me.user.id}"]`)).toContainText('…');
   const search=events.getByRole('searchbox',{name:'搜索记录',exact:true});
   await search.fill(me.user.id);await expect(row).toBeVisible();
   await search.fill('governance.overview');await expect(row).toBeVisible();
   await search.fill('');
  }
  await events.evaluate(el=>el.scrollIntoView({block:'start'}));await page.screenshot({path:artifact(`audit-${theme}-${width}`),animations:'disabled'});
  await go('users');
  const keys=page.getByRole('region',{name:'Key 目录',exact:true});
  await expect(keys.getByRole('row').nth(1)).toBeVisible();
  if(!baseline){
   await expect(page.getByRole('searchbox',{name:'搜索Key 目录',exact:true})).toHaveCount(0);
   const bounds=await keys.locator('tbody td:last-child').evaluateAll(cells=>cells.map(cell=>{
    const box=cell.getBoundingClientRect();const actions=cell.querySelector('div')!.getBoundingClientRect();
    return actions.left>=box.left && actions.right<=box.right;
   }));expect(bounds.every(Boolean)).toBe(true);
  }
  await page.screenshot({path:artifact(`keys-${theme}-${width}`),animations:'disabled'});
  await go('settings');
  await expect(page.getByTestId('personal-settings')).toBeVisible();
  if(!baseline){
   await expect(page.getByText('使用偏好',{exact:true})).toHaveCount(0);
   const hash=new URL(page.url()).hash;
   await page.getByLabel('显示名称',{exact:true}).fill('未保存的测试名称');
   await page.getByRole('navigation',{name:'设置大纲'}).getByRole('button',{name:'账号信息',exact:true}).press('Enter');
   await expect(page.getByRole('region',{name:'账号信息设置'})).toBeFocused();
   await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
   await expect(page.getByRole('navigation',{name:'设置大纲'}).getByRole('button',{name:'账号信息',exact:true})).toHaveAttribute('aria-current','location');
   expect(new URL(page.url()).hash).toBe(hash);
   await expect(page.getByLabel('显示名称',{exact:true})).toHaveValue('未保存的测试名称');
   await page.getByRole('navigation',{name:'设置大纲'}).getByRole('button',{name:'外观',exact:true}).click();
  }
  await page.screenshot({path:artifact(`settings-${theme}-${width}`),animations:'disabled'});
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
 }
});
