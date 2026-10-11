import {chromium} from '@playwright/test';
import fs from 'node:fs';import os from 'node:os';import path from 'node:path';
const cache=path.join(os.homedir(),'.cache/ms-playwright');const executablePath=fs.readdirSync(cache).filter(n=>/^chromium-\d+$/.test(n)).sort().reverse().map(n=>path.join(cache,n,'chrome-linux64/chrome')).find(p=>fs.existsSync(p));
const browser=await chromium.launch({executablePath});const page=await browser.newPage({viewport:{width:1440,height:1000}});
const artifacts=process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS||'temp/browser-artifacts';fs.mkdirSync(artifacts,{recursive:true});
try {
 await page.goto('http://127.0.0.1:62044/');await page.getByRole('button',{name:'无需账号，进入本地体验'}).click();
 await page.locator('.drive-home').waitFor();
 const info=await page.evaluate(async()=>{const spaces=(await(await fetch('/api/v1/spaces')).json()).data;const space=spaces.find(s=>s.name==='体验空间');const files=(await(await fetch('/api/v1/files?space_id='+space.id)).json()).data;return {space,pdf:files.find(f=>/\.pdf$/i.test(f.path)),html:files.find(f=>/\.html$/i.test(f.path))};});
 if(!info.pdf||!info.html)throw Error('Original preview fixtures unavailable');
 await page.goto('http://127.0.0.1:62044/#/drive/'+info.space.id);
 for(const dir of info.pdf.path.split('/').slice(0,-1)) await page.locator('.finder-folder .finder-name').filter({hasText:dir}).dblclick();
 await page.getByRole('button',{name:'详情 '+info.pdf.path,exact:true}).click();
 await page.waitForFunction(()=>document.querySelector('.reader-status')?.textContent?.includes('第 '));
 const ink=await page.locator('.pdf-reader canvas').evaluate(c=>{const d=c.getContext('2d').getImageData(0,0,c.width,c.height).data;for(let i=0;i<d.length;i+=4)if(d[i]<180&&d[i+3])return true;return false;});if(!ink)throw Error('Original PDF has no rendered pixels');
 await page.screenshot({path:path.join(artifacts,'original-pdf-preview.png'),mask:[page.locator('.quicklook-dialog')]});await page.keyboard.press('Escape');
 await page.getByRole('button',{name:'详情 '+info.html.path,exact:true}).click();await page.locator('.html-stage iframe').waitFor({state:'visible'});
 await page.getByRole('button',{name:'交互预览',exact:true}).click();await page.locator('.html-stage iframe').waitFor({state:'visible'});
 await page.screenshot({path:path.join(artifacts,'original-html-interactive.png')});
 console.log('Original PDF renders pixels; original HTML loads in isolated interactive mode.');
} finally {await browser.close();}
