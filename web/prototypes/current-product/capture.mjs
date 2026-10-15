import { chromium } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
const output=path.resolve(import.meta.dirname,'../../../temp/opendesign-replica');
const browser=await chromium.launch({headless:true,executablePath:process.env.PLAYWRIGHT_CHROMIUM || '/data2/yeshugen/cache/ms-playwright/chromium-1243/chrome-linux64/chrome'});
const page=await browser.newPage();page.setDefaultTimeout(15000);
const base='http://127.0.0.1:32015';
try {
 await page.goto(base);
 await page.locator('#login-key').fill('dbu_'+'2'.repeat(96));
 await page.getByRole('button',{name:'登录',exact:true}).click();
 await page.getByRole('button',{name:'账户菜单'}).waitFor();
 const seed=await page.evaluate(async()=>{
  const me=(await(await fetch('/api/v1/me')).json()).data;
  const headers={'Content-Type':'application/json','X-CSRF-Token':me.csrf_token};
  const write=async(endpoint,body,key,method='POST')=>{const r=await fetch('/api/v1'+endpoint,{method,headers:{...headers,'Idempotency-Key':btoa(unescape(encodeURIComponent(key)))},body:JSON.stringify(body)});if(!r.ok){const error=(await r.json()).error;throw Error('Fixture seed failed: '+endpoint+' '+error?.code);}return(await r.json()).data;};
  const existing=(await(await fetch('/api/v1/spaces')).json()).data;
  const space=existing.find(x=>x.name==='我的创作空间') || await write('/spaces',{name:'我的创作空间',connection_id:'platform'},'replica-space');
  const archive=existing.find(x=>x.name==='项目资料库') || await write('/spaces',{name:'项目资料库',connection_id:'platform'},'replica-archive');
  const texts={'产品说明.md':'# DriveBridge 产品说明\n\n把文件放在自己的空间中，让 Agent 在明确的授权范围内协作。\n\n## 工作流程\n\n1. 整理文件\n2. 建立项目集合\n3. 授权 Agent\n\n## 双向链接\n\n参考 [[会议记录]] 和 [[项目计划]]。\n\n## 公式\n\n$E=mc^2$\n','笔记/会议记录.md':'# 会议记录\n\n## 本周决定\n\n- 完成产品原型复刻\n- 验证文件拖拽和右键菜单\n\n参考 [[产品说明]]。','笔记/项目计划.md':'# 项目计划\n\n## 待办\n\n- [x] 汇总产品页面\n- [ ] 评审交互原型\n\n```mermaid\nflowchart LR\nA[文件] --> B[集合]\nB --> C[Agent]\n```\n','资料/品牌参考.txt':'产品界面使用森林绿导航、紧凑工具栏与柔和表面。','待归档/旧草稿.txt':'用于体验回收站恢复的示例草稿。'};
  const files=[];const stored=[...(await(await fetch('/api/v1/files?space_id='+space.id)).json()).data,...(await(await fetch('/api/v1/files?space_id='+space.id+'&trash=true')).json()).data];
  for(const [name,text] of Object.entries(texts)){
   const saved=stored.find(x=>x.path===name);if(saved){const stat=(await(await fetch('/api/v1/files/'+saved.id)).json()).data;files.push({...stat.file,ref:stat.ref,text});continue;}
   const body=new TextEncoder().encode(text);const sha256=Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',body)),b=>b.toString(16).padStart(2,'0')).join('');
   const op=await write('/uploads',{space_id:space.id,path:name,size_bytes:body.length,sha256},'replica-'+files.length);
   const part=await fetch('/api/v1/operations/'+op.id+'/parts/0',{method:'PUT',headers:{'X-CSRF-Token':me.csrf_token},body});if(!part.ok)throw Error('Fixture part failed');
   const done=await write('/operations/'+op.id+'/complete',{},'replica-complete-'+files.length);const stat=(await(await fetch('/api/v1/files/'+done.file_id)).json()).data;
   files.push({...stat.file,ref:stat.ref,text});
  }
  const dirs=(await(await fetch('/api/v1/browser?space_id='+space.id+'&path=')).json()).data;
  if(!dirs.entries.some(x=>x.kind==='directory'&&x.path==='归档'))await write('/directories',{space_id:space.id,path:'归档'},'replica-dir');
  const collections=[];const existingCollections=(await(await fetch('/api/v1/file-collections?space_id='+space.id)).json()).data;
  for(const name of ['产品发布资料','知识与会议']){
   const c=existingCollections.find(x=>x.name===name)||await write('/file-collections',{space_id:space.id,name,kind:'manual',scope:'private'},'replica-collection-'+name);
   await write('/file-collections/'+c.collection_id+'/members',{expected_revision:c.revision,include:files.slice(0,3).map(f=>f.id)},'replica-members-'+name,'PUT');collections.push(c);
  }
  const resources=[];
  for(const [kind,display_name] of [['template','项目计划模板'],['reference_pack','品牌与产品说明']]){try{resources.push(await write('/resource-descriptors',{space_id:space.id,file_ref:files[0].ref,kind,display_name},'replica-resource-'+kind));}catch{ /* Resource descriptor admission is separate; its real empty state is still captured. */ }}
  await write('/spaces/'+space.id+'/property-definitions',{expected_revision:0,definitions:[{key:'stage',display_name:'项目阶段',group:'项目',type:'text'},{key:'owner',display_name:'负责人',group:'项目',type:'text'},{key:'notes',display_name:'补充说明',group:'资料',type:'text'}]},'replica-properties','PUT');
  const created=await write('/agent-grants',{name:'资料整理助手',protocol:'http',space_id:space.id,connection_id:'platform',prefix:'资料',actions:['files.read'],expiry_mode:'duration',expires_in_seconds:604800},'replica-agent');
  const grant=created.grant || created;
  await write('/files/'+files.at(-1).id+'/trash',{},'replica-trash');files.at(-1).trashed=true;
  const shared=await write('/shares',{name:'产品原型评审包',mode:'fixed',audience:'users',space_id:space.id,days:7,recipients:[me.user.id],allow_download:false,items:[{file_id:files[0].id,version_id:files[0].version_id}]},'replica-share');
  return {me,space,archive,files,collections,resources,grant,share:shared.share};
 });
 const responses={};const pending=[];
 const normalize=url=>{const u=new URL(url);u.searchParams.sort();return u.pathname+u.search;};
 const capture=async r=>{if(!r.url().startsWith(base+'/api/v1/')&&!r.url().startsWith(base+'/auth/mode'))return;if(r.request().method()!=='GET'&&!['/api/v1/file-queries','/api/v1/analytics/queries','/api/v1/capabilities/check','/api/v1/operations/lookup'].includes(new URL(r.url()).pathname))return;try{const body=await r.json();responses[r.request().method()+' '+normalize(r.url())]={status:r.status(),body};}catch{}};
 page.on('response',r=>pending.push(capture(r)));
 await page.reload();await page.getByRole('button',{name:'账户菜单'}).waitFor();
 const routes=['home','insights','files','knowledge','resources','collections','properties','search','flows','tasks','shares','teams','trash','storage','devices','automation','capabilities','inbox','agents','members','governance','settings'];
 for(const route of routes){
  await page.goto(base+'/#/'+route);await page.waitForTimeout(300);await page.locator('.content').waitFor();await page.getByText('正在加载页面…',{exact:true}).waitFor({state:'hidden'}).catch(()=>{});await page.waitForTimeout(200);
  console.log('captured',route);
 }
 await page.goto(base+'/#/drive/'+seed.space.id);await page.waitForTimeout(500);await page.waitForTimeout(500);
 // Capture typed contracts needed by non-default views and explicit refreshes.
 await page.evaluate(async({space,files,collections,resources})=>{
  const paths=['/auth/mode','/api/v1/me','/api/v1/files?space_id='+space.id+'&trash=false','/api/v1/files?space_id='+space.id+'&trash=true','/api/v1/browser?space_id='+space.id+'&path=&q=&cursor=','/api/v1/notebook/pages?space_id='+space.id,'/api/v1/workspace/home?space_id='+space.id+'&recent_limit=6','/api/v1/spaces/'+space.id+'/label-definitions','/api/v1/spaces/'+space.id+'/property-definitions','/api/v1/spaces/'+space.id+'/automation-management','/api/v1/spaces/'+space.id+'/automation-rules','/api/v1/capabilities?space_id='+space.id];
  for(const file of files)paths.push('/api/v1/files/'+file.id,'/api/v1/files/'+file.id+'/versions','/api/v1/files/'+file.id+'/document?version_id='+file.version_id);
  for(const collection of collections)paths.push('/api/v1/file-collections/'+collection.collection_id,'/api/v1/file-collections/'+collection.collection_id+'/members');
  for(const resource of resources)paths.push('/api/v1/resource-descriptors/'+resource.descriptor_id);
  await Promise.all(paths.map(p=>fetch(p).catch(()=>{})));
 },seed);
 await Promise.all(pending);
 function scrub(v,key=''){
  if(/csrf_token/.test(key))return 'prototype-csrf';
  if(/(^|_)(token|secret|password|sealed|wrapped|hash)(_|$)/i.test(key))return undefined;
  if(Array.isArray(v))return v.map(x=>scrub(x));
  if(v&&typeof v==='object')return Object.fromEntries(Object.entries(v).map(([k,x])=>[k,scrub(x,k)]).filter(([,x])=>x!==undefined));
  if(typeof v==='string'){
   if(v==='e2e-admin')return 'Yeisme';if(v==='e2e-key-user')return '协作成员';
   if(/^https?:/.test(v))return undefined;
   if(/^dbu?_[a-f0-9]{40,}$/.test(v))return 'prototype-only';
  }
  return v;
 }
 const safe=scrub({...seed,responses});safe.me.user.name='Yeisme';safe.me.csrf_token='prototype-csrf';safe.me.admin=true;safe.me.governance_enabled=true;
 for(const value of Object.values(safe.responses))if(value.body?.data?.space_roles)value.body.data.admin_direct_write=true;
 fs.mkdirSync(output,{recursive:true});fs.writeFileSync(path.join(output,'seed.js'),'window.__DRIVEBRIDGE_PROTOTYPE_SEED__ = '+JSON.stringify(safe)+';\n');
 console.log('safe_contracts',Object.keys(responses).length,'files',safe.files.length,'routes',routes.length);
} finally {await browser.close();}
