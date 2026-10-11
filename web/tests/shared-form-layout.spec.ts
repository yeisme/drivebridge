import { test, expect } from '@playwright/test';
import path from 'node:path';

test('shared form layout keeps options, nested fields and actions readable', async ({page}) => {
  test.skip(process.env.DRIVEBRIDGE_TEST_TEAM_DEMO !== '1', 'Requires disposable team demo');
  await page.goto('/');
  await page.getByRole('button', {name:'无需账号，进入本地体验'}).click();
  await expect(page.getByRole('button', {name:'打开网盘 体验空间',exact:true})).toBeVisible();
  const tenant=(await(await page.request.get('/api/v1/governance')).json()).data.tenant_id;
  for(const theme of ['light','dark'] as const) for(const width of [1213,959,390]) {
    await page.setViewportSize({width,height:963});
    await page.emulateMedia({colorScheme:theme});
    await page.evaluate(hash=>{location.hash=hash;},`#/agents?tenant=${tenant}`);
    await page.getByRole('tab',{name:'旧版机器人授权',exact:true}).click();
    await page.getByRole('button',{name:'授权机器人',exact:true}).click();
    const dialog=page.getByRole('dialog',{name:'授权机器人',exact:true});
    await expect(dialog.getByRole('checkbox')).toHaveCount(5);
    for(const checkbox of await dialog.getByRole('checkbox').all()) {
      const alignment=await checkbox.evaluate(el=>{
        const box=el.getBoundingClientRect();
        const label=el.closest('label')!;
        const text=Array.from(label.childNodes).find(n=>n.nodeType===Node.TEXT_NODE && n.textContent?.trim())!;
        const range=document.createRange();range.selectNode(text);
        const bounds=range.getBoundingClientRect();
        return {dy:Math.abs(box.y+box.height/2-bounds.y-bounds.height/2),dx:bounds.x-box.right};
      });
      expect(alignment.dy).toBeLessThan(5);expect(alignment.dx).toBeGreaterThanOrEqual(8);
    }
    await expect(dialog.getByRole('checkbox',{name:'读取文件',exact:true})).toBeChecked();
    await expect(dialog.getByRole('checkbox',{name:'上传与更新',exact:true})).not.toBeChecked();
    await expect.poll(async()=>{const b=await dialog.boundingBox();return b!.x>=0&&b!.x+b!.width<=width&&b!.height<=963;}).toBe(true);
    await page.screenshot({animations:'disabled',path:path.join(process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!,`agent-form-${theme}-${width}.png`)});
    await dialog.getByRole('button',{name:'取消',exact:true}).click();
    await expect(page.getByRole('button',{name:'授权机器人',exact:true})).toBeFocused();
    await page.evaluate(hash=>{location.hash=hash;},`#/connections?tenant=${tenant}`);
    await page.getByRole('button',{name:'连接存储',exact:true}).click();
    const storage=page.getByRole('dialog',{name:'连接存储',exact:true});
    for(const name of ['存储桶','区域']) {
      const layout=await storage.getByLabel(name,{exact:true}).evaluate(el=>{const l=el.closest('label')!;return {display:getComputedStyle(l).display,gap:getComputedStyle(l).gap};});
      expect(layout).toEqual({display:'grid',gap:'8px'});
    }
    await page.screenshot({animations:'disabled',path:path.join(process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS!,`storage-form-${theme}-${width}.png`)});
    await storage.getByRole('button',{name:'取消',exact:true}).click();
  }
});
