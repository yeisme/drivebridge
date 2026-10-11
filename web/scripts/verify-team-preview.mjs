import {chromium} from '@playwright/test';
import fs from 'node:fs';import os from 'node:os';import path from 'node:path';
const cache=path.join(os.homedir(),'.cache/ms-playwright');const executablePath=fs.readdirSync(cache).filter(n=>/^chromium-\d+$/.test(n)).sort().reverse().map(n=>path.join(cache,n,'chrome-linux64/chrome')).find(p=>fs.existsSync(p));
const browser=await chromium.launch({executablePath});const page=await browser.newPage({viewport:{width:1440,height:1000}});
const artifacts=process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS||'temp/browser-artifacts';fs.mkdirSync(artifacts,{recursive:true});
try{
 await page.goto('http://127.0.0.1:62044/');await page.getByRole('button',{name:'无需账号，进入本地体验'}).click();
 await page.getByRole('button',{name:'团队',exact:true}).click();
 await page.locator('button.drive-card').filter({hasText:'yeisme'}).click();
 await page.getByRole('link',{name:'网盘',exact:true}).click();
 await page.getByRole('button',{name:'创建团队网盘',exact:true}).waitFor();
 await page.screenshot({path:path.join(artifacts,'yeisme-team-spaces.png')});
 await page.locator('button.drive-card').filter({hasText:'yeisme 测试空间'}).click();
 const upload=page.getByRole('button',{name:'上传文件',exact:true});await upload.waitFor();if(await upload.isDisabled())throw new Error('Custodian cannot upload');
 await page.waitForFunction(()=>document.querySelector(".finder-content-view")?.getAttribute("aria-busy")==="false");
 if(await page.getByRole("button",{name:"详情 团队空间验证.txt",exact:true}).count()===0) await page.locator('input[type=file]').setInputFiles({name:'团队空间验证.txt',mimeType:'text/plain',buffer:Buffer.from('yeisme team space: upload and download verified\n')});
 await page.getByRole('button',{name:'详情 团队空间验证.txt',exact:true}).waitFor();
 const checked=await page.evaluate(async()=>{
  const me=(await(await fetch('/api/v1/me')).json()).data;const g=(await(await fetch('/api/v1/governance')).json()).data;
  const sp=g.spaces.find(s=>s.name==='yeisme 测试空间');if(!sp||sp.max_bytes!==1073741824)throw Error('Quota mismatch');
  const headers={'X-DriveBridge-Tenant':g.tenant_id,'X-DriveBridge-Space':sp.id};
  const files=(await(await fetch('/api/v1/files?space_id='+sp.id,{headers})).json()).data;
  const f=files.find(f=>f.path==='团队空间验证.txt');if(!f)throw Error('Upload missing');
  const r=await fetch('/api/v1/files/'+f.id+'/content?version='+f.version_id,{headers});const text=await r.text();if(!r.ok||text!=='yeisme team space: upload and download verified\n')throw Error('Download mismatch');
  return {quota:sp.max_bytes,role:g.space_roles[sp.id],platformAdmin:g.platform_admin};
 });
 console.log('Verified team upload/download:',JSON.stringify(checked));
 await page.screenshot({path:path.join(artifacts,'yeisme-team-drive.png')});
}finally{await browser.close();}
