/* Isolated presentation data for the current product replica. No live service requests. */
(() => {
  if(window.__DRIVEBRIDGE_PROTOTYPE__)return;
  const seed=window.__DRIVEBRIDGE_PROTOTYPE_SEED__;
  if(!seed)throw Error('Prototype seed missing');
  const originalFetch=window.fetch.bind(window);
  const clone=value=>structuredClone(value);
  const storageKey='drivebridge:prototype:current-product-20261012';
  const fresh=()=>({user:seed.me.user,spaces:[seed.space,seed.archive],files:seed.files,grants:seed.grant?.grant_id?[seed.grant]:[],collections:seed.collections,resources:seed.resources,shares:seed.share?[seed.share]:[],folders:[{space_id:seed.space.id,path:'归档'}],operations:seed.responses['GET /api/v1/operations']?.body.data||[],groups:[],receipts:{},properties:{},scenario:'normal',signedIn:true});
  let state;try{state=JSON.parse(localStorage.getItem(storageKey)||'null')||clone(fresh());}catch{state=clone(fresh());}
  const save=()=>{try{localStorage.setItem(storageKey,JSON.stringify(state));}catch{}};
  const now=()=>new Date().toISOString();
  const id=()=>Array.from(crypto.getRandomValues(new Uint8Array(24)),b=>b.toString(16).padStart(2,'0')).join('');
  const page=items=>({items,next_cursor:'',observed_at:now(),total_kind:'exact',total:items.length});
  const ok=data=>new Response(JSON.stringify({data}),{status:200,headers:{'Content-Type':'application/json'}});
  const fail=(code,status=400)=>new Response(JSON.stringify({error:{code,message:code}}),{status,headers:{'Content-Type':'application/json'}});
  const safeFile=f=>{const {text,...file}=f;return file;};
  const ref=f=>`drivebridge://${seed.me.instance_id}/file/${f.id}/version/${f.version_id}`;
  const files=(space,trash=false)=>state.scenario==='empty'?[]:state.files.filter(f=>(!space||f.space_id===space)&&!!f.trashed===trash);
  const totalSpace=sp=>({...sp,used_bytes:state.files.filter(f=>f.space_id===sp.id).reduce((n,f)=>n+f.size_bytes,0),reserved_bytes:0});
  const registry=seed.responses;
  const byPath=new Map(Object.entries(registry).map(([key,value])=>[key.split('?')[0],value]));
  const responseFromSnapshot=(method,path,url)=>{
    const q=new URLSearchParams(url.search);q.sort();
    const hit=registry[method+' '+path+(q.size?'?'+q:'')]||byPath.get(method+' '+path);
    return hit?new Response(JSON.stringify(clone(hit.body)),{status:hit.status,headers:{'Content-Type':'application/json'}}):null;
  };
  const addFile=(space,path,text='',mime='text/plain')=>{const file={id:id(),space_id:space,path,version_id:id(),size_bytes:new TextEncoder().encode(text).length,mime_type:mime,trashed:false,updated_at:now(),text};file.ref=ref(file);state.files.push(file);save();return file;};
  const readResult=(space,filters=[])=>{let list=files(space);for(const f of filters){if(f.field==='path_prefix')list=list.filter(row=>f.op==='prefix'?row.path.startsWith(f.value):row.path.includes(f.value));if(f.field==='mime_type')list=list.filter(row=>row.mime_type===f.value);}return {mode:'exact',items:list.map(f=>({file_id:f.id,version_id:f.version_id})),catalogue_revision:'prototype-'+state.files.length,next_cursor:null,coverage:'complete',total_count:list.length};};
  const diagnostics={unhandled:[],mutations:0,mode:'isolated-prototype'};
  window.__DRIVEBRIDGE_PROTOTYPE__={state:()=>clone(state),diagnostics,reset:()=>{state=clone(fresh());save();location.reload();}};
  if(!location.hash)location.hash='#/home';
  if(!navigator.locks)Object.defineProperty(navigator,'locks',{value:{request:async(_name,options,callback)=>(callback||options)({name:_name})}});
  if(!navigator.clipboard)Object.defineProperty(navigator,'clipboard',{value:{writeText:async text=>{const box=document.createElement('textarea');box.value=text;box.style.cssText='position:fixed;left:-9999px';document.body.append(box);box.select();document.execCommand('copy');box.remove();}}});

  window.fetch=async(input,options={})=>{
    const request=input instanceof Request?input:null;
    const url=new URL(request?request.url:String(input),location.href);
    const method=(options.method||request?.method||'GET').toUpperCase();
    const signal=options.signal||request?.signal;signal?.throwIfAborted();
    const path=url.pathname;
    if(!path.startsWith('/api/v1')&&!path.startsWith('/auth/')){
      // Assets are served inside the replica; remote API/service calls are never forwarded.
      if(url.origin!==location.origin||!['GET','HEAD'].includes(method))return fail('prototype_external_request_blocked',403);
      return originalFetch(input,options);
    }
    if(state.scenario==='error')return fail('dependency_unavailable',503);
    let body=options.body;if(body===undefined&&request&&method!=='GET')body=await request.clone().text();
    let q={};if(typeof body==='string'){try{q=JSON.parse(body);}catch{}}
    const headers=new Headers(options.headers||request?.headers);const key=headers.get('Idempotency-Key')||q.idempotency_key;
    const apiPath=path.replace(/^\/api\/v1/,'');
    const sid=url.searchParams.get('space_id')||headers.get('X-DriveBridge-Space')||q.space_id||state.spaces[0]?.id;
    if(path==='/auth/mode')return ok({key:true,oidc:false,governance:true});
    if(path==='/auth/key'){state.signedIn=true;state.scenario='normal';save();return ok({csrf_token:'prototype-csrf'});}
    if(path==='/auth/logout'){state.signedIn=false;save();return ok({});}
    if(!state.signedIn||state.scenario==='expired')return fail('unauthorized',401);
    if(method!=='GET'&&method!=='HEAD')diagnostics.mutations++;
    if(apiPath==='/me'){
      if(method==='PATCH'){state.user={...state.user,display_name:q.display_name,avatar_ref:q.avatar_ref};save();}
      return ok({...seed.me,user:state.user,csrf_token:'prototype-csrf',admin:true,governance_enabled:true});
    }
    if(apiPath==='/me/avatar-candidates')return ok({items:[]});
    if(apiPath==='/governance'){
      const original=byPath.get('GET /api/v1/governance')?.body.data||{};
      return ok({...original,admin_direct_write:true,spaces:state.scenario==='empty'?[]:state.spaces.map(totalSpace),space_roles:Object.fromEntries(state.spaces.map(sp=>[sp.id,state.scenario==='readonly'?'viewer':'owner'])),actor_id:state.user.id});
    }
    if(state.scenario==='readonly'&&method!=='GET'&&!['/file-queries','/analytics/queries','/capabilities/check','/operations/lookup'].includes(apiPath))return fail('approval_required',403);
    if(apiPath==='/spaces'){
      if(method==='POST'){const sp={...seed.space,id:id(),name:q.name,used_bytes:0,reserved_bytes:0,connection_id:q.connection_id};state.spaces.push(sp);save();return ok(sp);}
      return ok(state.scenario==='empty'?[]:state.spaces.map(totalSpace));
    }
    if(apiPath==='/workspace/home')return ok({space_id:sid,observed_at:now(),sections:Object.fromEntries(['recent_files','collections','pending_counts','usage'].map(key=>[key,{state:'ready'}])),recent_files:files(sid).slice(0,6).map(f=>({file_id:f.id,version_id:f.version_id,space_id:sid,path:f.path,ref:ref(f),size_bytes:f.size_bytes,updated_at:f.updated_at})),collections:state.collections.filter(c=>c.space_id===sid),pending_counts:{submitted:0,accepted:0},usage:{used_bytes:files(sid).reduce((n,f)=>n+f.size_bytes,0),reserved_bytes:0,quota_bytes:state.spaces.find(sp=>sp.id===sid)?.max_bytes||0}});
    if(apiPath==='/files'&&method==='GET')return ok(files(sid,url.searchParams.get('trash')==='true').map(safeFile));
    if(apiPath==='/browser'){
      const prefix=url.searchParams.get('path')||'', search=url.searchParams.get('q')||'';
      const all=files(sid);const start=prefix?prefix+'/':'';const dirs=new Set(state.folders.filter(f=>f.space_id===sid&&f.path.startsWith(start)).map(f=>start+f.path.slice(start.length).split('/')[0]));
      for(const f of all)if(f.path.startsWith(start)&&f.path.slice(start.length).includes('/'))dirs.add(start+f.path.slice(start.length).split('/')[0]);
      const list=all.filter(f=>search?f.path.includes(search):f.path.startsWith(start)&&!f.path.slice(start.length).includes('/')).map(f=>({kind:'file',path:f.path,file:safeFile(f)}));
      return ok({entries:[...(!search?[...dirs].filter(p=>p!==prefix).map(path=>({kind:'directory',path})):[]),...list],next_cursor:'',total:list.length+dirs.size});
    }
    if(apiPath==='/file-queries')return ok(readResult(q.space_ids?.[0]||sid,q.filters));
    if(apiPath==='/content-queries')return ok({mode:'discovery',coverage:'partial',items:files(q.spaces?.[0]||sid).filter(f=>(f.text||'').includes(q.text||q.query||'')).slice(0,5).map(f=>({file_ref:ref(f),truth_level:'prototype_sample',index_generation:'prototype-text-index',locator:{kind:'document'}})),missing:['image','video','audio'],next_cursor:null,observed_at:now()});
    if(/^\/connections\/[^/]+\/check$/.test(apiPath))return ok({status:'verified',observed_at:now()});
    if(apiPath==='/directories'&&method==='POST'){
      if(state.folders.some(f=>f.space_id===q.space_id&&f.path===q.path))return fail('path_conflict',409);
      state.folders.push({space_id:q.space_id,path:q.path});save();return ok({path:q.path,created:true});
    }
    const fileMatch=apiPath.match(/^\/files\/([^/]+)(?:\/(.*))?$/);
    if(fileMatch){
      const file=state.files.find(f=>f.id===fileMatch[1]);if(!file)return fail('not_found',404);
      const tail=fileMatch[2]||'';
      if(!tail&&method==='GET')return ok({file:safeFile(file),ref:ref(file)});
      if(['content','media','download'].includes(tail))return new Response(file.text||'',{status:200,headers:{'Content-Type':file.mime_type,'Content-Length':String(new TextEncoder().encode(file.text||'').length),'ETag':'"'+file.version_id+'"'}});
      if(tail==='versions')return ok([{id:file.version_id,version_id:file.version_id,file_id:file.id,size_bytes:file.size_bytes,created_at:file.updated_at,sha256:'0'.repeat(64)}]);
      if(tail==='document-capability')return ok({file_id:file.id,version_id:file.version_id,editable:state.scenario!=='readonly',reason:'',max_bytes:2*1024*1024});
      if(tail==='document'&&method==='PUT'){
        if(q.base_version_id!==file.version_id)return fail('version_conflict',409);
        file.text=q.text;file.size_bytes=new TextEncoder().encode(q.text).length;file.version_id=id();file.updated_at=now();file.ref=ref(file);
        const result={file_id:file.id,version_id:file.version_id,operation_id:id(),status:'completed',file_ref:ref(file)};state.receipts[key]=result;save();return ok(result);
      }
      if(tail==='document/lookup')return ok({found:!!state.receipts[key],save:state.receipts[key]||null});
      if(tail==='move'||tail==='rename'){
        if(q.expected_version&&q.expected_version!==file.version_id)return fail('version_conflict',409);
        if(state.files.some(f=>f!==file&&f.space_id===file.space_id&&f.path===q.path))return fail('path_conflict',409);
        file.path=q.path||q.name;file.updated_at=now();file.ref=ref(file);save();return ok(safeFile(file));
      }
      if(tail==='trash'||tail==='restore'){file.trashed=tail==='trash';save();return ok(safeFile(file));}
      if(tail==='properties'){
        if(method==='PUT'||method==='PATCH'){state.properties[file.id]=q.values||q.properties||{};save();}
        return ok({file_id:file.id,version_id:file.version_id,revision:1,values:state.properties[file.id]||{}});
      }
      if(tail==='labels')return ok({file_id:file.id,manual:[],rule:[],suggested:[]});
      if(tail==='label-suggestions')return ok(page([]));
      if(tail==='preview'||tail==='preview-result')return ok({file_id:file.id,version_id:file.version_id,status:'unavailable',reason:'prototype_text_preview'});
    }
    if(apiPath==='/uploads'&&method==='POST'){
      const op={id:id(),space_id:q.space_id,path:q.path,size_bytes:q.size_bytes,sha256:q.sha256,bytes_transferred:0,kind:'upload',status:'awaiting_upload',updated_at:now(),parts:[]};state.operations.push(op);state.receipts[key]=op;save();return ok(op);
    }
    const opMatch=apiPath.match(/^\/operations\/([^/]+)(?:\/(.*))?$/);
    if(opMatch){const op=state.operations.find(o=>o.id===opMatch[1]);if(op){
      if(opMatch[2]?.startsWith('parts/')){op.text=body instanceof Blob?await body.text():typeof body==='string'?body:'';op.bytes_transferred=op.size_bytes;save();return ok(op);}
      if(opMatch[2]==='complete'){const f=addFile(op.space_id,op.path,op.text||'',op.path.endsWith('.md')?'text/markdown':'text/plain');Object.assign(op,{file_id:f.id,version_id:f.version_id,status:'completed'});save();return ok(op);}
      if(opMatch[2]==='cancel'){op.status='cancelled';save();return ok(op);}
      return ok({operation:op,received_parts:op.text?[0]:[]});
    }}
    if(apiPath==='/operations/lookup')return ok(state.receipts[key]?{found:true,operation:state.receipts[key]}:{found:false});
    if(apiPath==='/operations')return ok(state.operations);
    if(apiPath==='/notebook/pages'){
      if(method==='GET')return ok({items:files(sid).filter(f=>f.path.endsWith('.md')).map(f=>({file_id:f.id,space_id:f.space_id,version_id:f.version_id,path:f.path,parent_file_id:'',icon:'file',revision:1})),next_cursor:''});
      if(state.receipts[key])return ok(state.receipts[key]);
      const f=addFile(q.space_id,(q.daily_date?'每日笔记/':'笔记/')+(q.daily_date||q.title)+'.md','# '+q.title+'\n','text/markdown');const result={found:true,status:'completed',operation_id:id(),page:{file_id:f.id,space_id:f.space_id,version_id:f.version_id,path:f.path,parent_file_id:q.parent_file_id||'',icon:q.icon}};state.receipts[key]=result;save();return ok(result);
    }
    if(apiPath.startsWith('/notebook/pages/')&&method==='POST')return ok(state.receipts[key]||{found:false,status:'not_found'});
    if(apiPath==='/references'){const target=q.ref||url.searchParams.get('ref');const f=state.files.find(f=>ref(f)===target);return f?ok({file:safeFile(f),ref:ref(f)}):fail('not_found',404);}
    if(apiPath==='/agent-grants'){
      if(method==='GET')return ok(page(state.scenario==='empty'?[]:state.grants.filter(g=>!url.searchParams.get('state')||g.state===url.searchParams.get('state'))));
      if(state.receipts[key])return ok({grant:state.receipts[key],credential:{state:'not_replayable'}});
      const grant={grant_id:id(),agent_id:id(),name:q.name,protocol:q.protocol,space_id:q.space_id,connection_id:q.connection_id,prefix:q.prefix,actions:q.actions,restriction_profile:'personal_workspace_v1',revision:1,state:'active',expiry_mode:q.expiry_mode||'never',expires_at:q.expiry_mode==='duration'?new Date(Date.now()+q.expires_in_seconds*1000).toISOString():null,issued_at:now(),expiry_changed_at:null,credential_generation:1,created_at:now(),updated_at:now()};state.grants.push(grant);state.receipts[key]=grant;save();return ok({grant,credential:{state:'issued',secret:'prototype-example-not-a-real-key'}});
    }
    if(apiPath==='/agent-grants/lookup')return state.receipts[key]?ok({grant:state.receipts[key],credential:{state:'not_replayable'}}):fail('not_found',404);
    const grantMatch=apiPath.match(/^\/agent-grants\/([^/]+)(?:\/(.*))?$/);
    if(grantMatch){const g=state.grants.find(g=>g.grant_id===grantMatch[1]);if(!g)return fail('not_found',404);const tail=grantMatch[2]||'';
      if(tail==='checks')return ok({checked_at:now(),connection_state:'observed_ok',checks:[{name:'configuration',status:'observed_ok'},{name:'authorization',status:'observed_ok'}]});
      if(['enable','disable','revoke'].includes(tail)){g.state=tail==='enable'?'active':tail==='disable'?'disabled':'revoked';g.revision++;save();return ok({grant_id:g.grant_id,result_revision:g.revision,action:tail,committed_at:now()});}
      if(tail==='name'){g.name=q.name;g.revision++;save();return ok({grant_id:g.grant_id,name:g.name,result_revision:g.revision});}
      return ok(g);
    }
    if(apiPath==='/agent-groups'){if(method==='POST'){const group={group_id:id(),name:q.name,revision:1,created_at:now(),updated_at:now()};state.groups.push(group);save();return ok(group);}return ok(page(state.groups));}
    if(apiPath==='/agents')return ok([]);
    if(apiPath==='/workspace/tasks'){const kind=url.searchParams.get('kind')||'operation';return ok(page(kind==='operation'?state.operations.map(op=>({owner_kind:'operation',owner_id:op.id,space_id:op.space_id,title:op.path,owner_status:op.status,ui_state:op.status==='completed'?'completed':op.status==='cancelled'?'cancelled':'running',progress:{total:1,succeeded:op.status==='completed'?1:0,failed:0,pending:op.status==='completed'?0:1,bytes_sent:op.size_bytes||0,durable_bytes:op.size_bytes||0,verified_bytes:op.size_bytes||0}})):[]));}
    if(apiPath==='/agent-invocations'||apiPath==='/inbox-submissions')return ok(page([]));
    if(apiPath==='/file-collections'){
      if(method==='POST'){const collection={collection_id:id(),space_id:q.space_id,name:q.name,kind:q.kind,scope:q.scope||'private',rule_revision:1,revision:1,members_total:0,created_at:now(),updated_at:now(),items:[]};state.collections.push(collection);state.receipts[key]=collection;save();return ok(collection);}
      return ok(state.scenario==='empty'?[]:state.collections.filter(c=>c.space_id===sid));
    }
    const collectionMatch=apiPath.match(/^\/file-collections\/([^/]+)(?:\/(.*))?$/);
    if(collectionMatch){const c=state.collections.find(c=>c.collection_id===collectionMatch[1]);if(!c)return fail('not_found',404);
      if(collectionMatch[2]==='members'&&method==='PUT'){c.items=(q.include||[]).map(file_id=>({file_id,version_id:state.files.find(f=>f.id===file_id)?.version_id}));c.revision++;save();return ok(c);}
      if(collectionMatch[2]==='snapshots')return ok([]);
      return ok({collection_id:c.collection_id,kind:c.kind,revision:c.revision,computed_at:now(),items:c.items||files(c.space_id).slice(0,3).map(f=>({file_id:f.id,version_id:f.version_id})),visible_count:c.members_total||3,coverage:'complete'});
    }
    if(apiPath==='/resource-descriptors'){
      if(method==='POST'){const resource={descriptor_id:id(),space_id:q.space_id,display_name:q.display_name,kind:q.kind,descriptor_revision:1,check_state:'unknown',...(q.collection_snapshot_id?{collection_snapshot_id:q.collection_snapshot_id}:{file_ref:q.file_ref}),created_at:now(),updated_at:now()};state.resources.push(resource);save();return ok(resource);}
      return ok(page(state.scenario==='empty'?[]:state.resources.filter(r=>!q.space_id||r.space_id===q.space_id)));
    }
    if(apiPath.startsWith('/resource-descriptors/')){const r=state.resources.find(r=>r.descriptor_id===apiPath.split('/')[2]);if(r)return ok(r);}
    if(apiPath==='/shares'){if(method==='POST'){const share={...(seed.share||{}),id:id(),name:q.name,space_id:q.space_id,mode:q.mode,audience:q.audience,expires_at:new Date(Date.now()+(q.days||7)*86400000).toISOString(),revoked:false,revision:1,created_at:now(),items:q.items};state.shares.push(share);save();return ok({share,url:location.href.split('#')[0]+'#share/prototype'});}return ok(state.scenario==='empty'?[]:state.shares);}
    if(apiPath.startsWith('/shares/')&&apiPath.endsWith('/revoke')){const s=state.shares.find(s=>s.id===apiPath.split('/')[2]);if(s){s.revoked=true;save();return ok(s);}}
    if(apiPath==='/capabilities/check')return ok({action:q.action,implemented:true,configured:true,authorized:state.scenario!=='readonly',availability:state.scenario==='readonly'?'blocked':'available',dependency_status:'none',connection_observed_at:now(),block_reason:state.scenario==='readonly'?'forbidden':''});
    if(apiPath==='/favorites'){if(method==='GET')return ok([]);return ok({favorite_id:id(),revision:1,...q});}
    if(apiPath==='/inbox')return ok([]);
    const snapshot=responseFromSnapshot(method,path,url);if(snapshot)return snapshot;
    // Preserve an honest unsupported state rather than forwarding any request.
    diagnostics.unhandled.push(method+' '+apiPath);return fail('prototype_action_unavailable',501);
  };
  const mountControls=()=>{
    const control=document.createElement('div');control.id='prototype-controls';control.innerHTML='<button type="button" aria-label="原型场景">交互原型</button><div hidden><strong>原型场景</strong><button data-scenario="normal">有内容</button><button data-scenario="empty">空空间</button><button data-scenario="readonly">只读协作</button><button data-scenario="error">服务暂不可用</button><button data-scenario="expired">登录失效</button><button data-reset>恢复初始示例</button><small>操作仅保存于当前浏览器，不连接正式服务。</small></div>';
    control.style.cssText='position:fixed;right:16px;bottom:16px;z-index:120;font:12px system-ui;color:var(--ink,#24382d)';
    const style=document.createElement('style');style.textContent='#prototype-controls>button{border:1px solid var(--line,#d9e3dc);border-radius:6px;background:var(--surface,#fff);color:inherit;padding:6px 10px;cursor:pointer}#prototype-controls>div{position:absolute;bottom:36px;right:0;width:240px;padding:12px;border:1px solid var(--line,#d9e3dc);background:var(--surface,#fff);border-radius:8px;box-shadow:0 8px 32px #0002}#prototype-controls>div:not([hidden]){display:grid;gap:8px}#prototype-controls>div button{text-align:left;padding:8px;background:var(--paper,#f5f6f4);color:inherit;border:0;border-radius:4px;cursor:pointer}#prototype-controls small{line-height:1.6;color:var(--muted,#4b6254)}@media(max-width:767px){#prototype-controls{bottom:72px!important;right:10px!important}}';
    document.head.append(style);document.body.append(control);
    control.firstElementChild.onclick=()=>{const panel=control.lastElementChild;panel.hidden=!panel.hidden;};
    control.querySelectorAll('[data-scenario]').forEach(button=>button.onclick=()=>{state.scenario=button.dataset.scenario;save();location.reload();});
    control.querySelector('[data-reset]').onclick=()=>window.__DRIVEBRIDGE_PROTOTYPE__.reset();
  };
  if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',mountControls,{once:true});else mountControls();
})();
