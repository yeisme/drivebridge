import { chromium } from '@playwright/test';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
const cache=path.join(os.homedir(),'.cache/ms-playwright');
const executablePath=fs.readdirSync(cache).filter(n=>/^chromium-\d+$/.test(n)).sort().reverse().map(n=>path.join(cache,n,'chrome-linux64/chrome')).find(p=>fs.existsSync(p));
const browser=await chromium.launch({executablePath});
const page=await browser.newPage();
const artifacts=process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS||'temp/browser-artifacts';fs.mkdirSync(artifacts,{recursive:true});
try {
 await page.goto('http://127.0.0.1:62044/');
 await page.getByRole('button',{name:'无需账号，进入本地体验'}).click();
 await page.locator('.drive-home').waitFor();
 await page.goto('http://127.0.0.1:62044/#/drive/1c5ed8240ad58da0b7c5cc8918036e31b5f3ff7ae55f0452?tenant=ea34935d148e4a55d0785f7f0b2bdc5ad405ec3ec8693186&team=2461481f4c65609896742e4ab31c01a866d4f3441e802ebc&scope=managed');
 await page.getByRole('button',{name:'详情 团队空间验证.txt',exact:true}).click();
 const dialog=page.locator('.quicklook-dialog');await dialog.waitFor();
 await page.getByRole('button',{name:'复制到其他网盘',exact:true}).click();
 for (const theme of ['light','dark']) for(const width of [1440,1001,390]) {
  await page.setViewportSize({width,height:963});
  await page.evaluate(t=>document.documentElement.classList.toggle('dark',t==='dark'),theme);
  await page.waitForTimeout(200);
  const result=await dialog.evaluate(el=>{
   const header=el.querySelector(':scope > header').getBoundingClientRect();
   const buttons=[...el.querySelectorAll('.quicklook-controls button')].map(b=>b.getBoundingClientRect());
   const aside=el.querySelector('.quicklook-sidebar');
   return {headerHeight:header.height,aligned:Math.abs(buttons[0].y-buttons[1].y)<2,overflow:aside.scrollWidth>aside.clientWidth+1};
  });
  if(result.headerHeight<48||!result.aligned||result.overflow) throw new Error(JSON.stringify({theme,width,...result}));
  await page.screenshot({path:path.join(artifacts,`preview-${theme}-${width}.png`)});
  console.log('Preview layout passed',theme,width,JSON.stringify(result));
 }
 await page.getByRole('button',{name:'收起信息栏',exact:true}).click();
 if(await page.locator('#quicklook-sidebar').isVisible())throw Error('Sidebar did not close');
 await page.getByRole('button',{name:'展开信息栏',exact:true}).click();
 await page.getByRole('button',{name:'文件标识与引用',exact:true}).click();
 await page.getByRole('button',{name:'复制文件引用',exact:true}).waitFor();
 await page.keyboard.press('Escape');await dialog.waitFor({state:'hidden'});
 console.log('Sidebar, disclosure, and Escape passed');
} finally {await browser.close();}
