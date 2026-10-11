import { ConfirmDialog } from "./components/ConfirmDialog";
import { ActionMenu } from "./components/ActionMenu";
import { MultiSelect } from "./components/MultiSelect";
import { Textarea } from "./components/ui/textarea";
import { Checkbox } from "./components/ui/checkbox";
import { Input } from "./components/ui/input";
import { SearchField } from "./components/SearchField";
import { DataTable } from "./components/DataTable";
import { Button, buttonVariants } from "./components/ui/button";
import { SelectAdapter } from "./components/SelectAdapter";
import React, { useEffect, useRef, useState } from "react";
import { WorkspacePageHeader } from "./components/WorkspacePageHeader";
import { Share2, Copy, Link, Lock } from "lucide-react";
import { api, errorText, bytes, type DriveFile } from "./api";
import { AppDialog as WorkspaceDialog } from "./components/AppDialog";
import { MediaPreview } from "./Media";
import { handleListKey } from "./page-list";

import { loadShareDevice, openShareDevice, openShareProof, receiverError, registerShareDevice, type ShareOpenResult } from "./share-receiver";
import { ShareReceiversDialog } from "./ShareReceiversDialog";
export interface ShareRow {
  id: string;
  name: string;
  mode: string;
  audience: string;
  path: string;
  expires_at: string;
  revoked: boolean;
  allow_download: boolean;
  is_permanent?: boolean;
  presentation?: SharePresentation;
}
export interface SharePresentation {
  preset?: "default" | "dark" | "brand";
  font?: "system" | "serif" | "rounded";
  accent_color?: string;
  logo_ref?: string;
  welcome_message?: string;
}
export function ShareDialog({
  files,
  path,
  space,
  close,
}: {
  files: DriveFile[];
  path: string;
  space: string;
  close: () => void;
}) {
  const [mode, setMode] = useState("fixed"),
    [audience, setAudience] = useState("link"),
    [days, setDays] = useState("7"),
    [customDays, setCustomDays] = useState("14"),
    [recipients, setRecipients] = useState<Array<{id:string;name:string}>>([]),
    [recipientInput, setRecipientInput] = useState(""),
    [recipientError, setRecipientError] = useState(""),
    [recipientCandidate, setRecipientCandidate] = useState<{id:string;name:string} | null>(null),
    [recipientLoading, setRecipientLoading] = useState(false),
    [presentation, setPresentation] = useState<SharePresentation>({preset:"default",font:"system",accent_color:"#1f7661"}),
    [link, setLink] = useState(""),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [copied, setCopied] = useState(false);
  useEffect(()=>{
    if(audience!=="users"||!recipientInput.trim()){setRecipientCandidate(null);setRecipientError("");return;}
    const controller=new AbortController();
    const timer=setTimeout(()=>{
      setRecipientLoading(true);setRecipientError("");
      api<{id:string;name:string}>(`/share-recipients?id=${encodeURIComponent(recipientInput.trim().slice(0,120))}`,"GET",undefined,{},controller.signal)
       .then(result=>{if(!controller.signal.aborted)setRecipientCandidate(result);})
       .catch(e=>{if(!controller.signal.aborted){setRecipientCandidate(null);setRecipientError(errorText(e));}})
       .finally(()=>{if(!controller.signal.aborted)setRecipientLoading(false);});
    },250);
    return ()=>{clearTimeout(timer);controller.abort();};
  },[recipientInput,audience]);
  return (
    <WorkspaceDialog title="创建分享" close={close} className="max-md:[&_input:not([type=checkbox])]:min-h-11 max-md:[&_button:not([role=checkbox])]:min-h-11">
      {link ? (
        <div className="share-created">
          <Share2 size={34} />
          <h3>分享已创建</h3>
          <p>可在“分享”中查看权限、复制链接或撤销。</p>
          <Input aria-label="分享链接" readOnly value={link} />
          <Button variant="default" type="submit"

            onClick={() => {
              void navigator.clipboard
                .writeText(link)
                .then(() => setCopied(true))
                .catch(() => setError("无法复制，请手动选择链接。"));
            }}
          >
            <Copy size={16} />
            {copied ? "已复制" : "复制链接"}
          </Button>
          {error && <p role="alert">{error}</p>}
        </div>
      ) : (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            const form = new FormData(e.currentTarget);
            setBusy(true);
            setError("");
            void api<{ share: ShareRow; link: string }>("/shares", "POST", {
              name: form.get("name"),
              space_id: space,
              path,
              mode,
              audience,
              days: Number(days === "custom" ? customDays : days),
              allow_download: form.get("download") === "on",
              passcode: form.get("passcode") || "",
              recipients: recipients.map((r) => r.id),
              presentation,
              items: files.map((f) => ({
                file_id: f.id,
                version_id: f.version_id,
              })),
            })
              .then((out) => setLink(out.link))
              .catch((e) => setError(errorText(e)))
              .finally(() => setBusy(false));
          }}
        >
          <p>
            {files.length
              ? `${files.length} 个文件 · 固定所选版本`
              : `目录：${path || "根目录"}`}
          </p>
          <label>
            分享名称
            <Input
              name="name"
              required
              maxLength={120}
              defaultValue={
                files[0]?.path.split("/").pop() ||
                path.split("/").pop() ||
                "文件交付"
              }
            />
          </label>
          <div className="form-columns">
            <label>
              内容模式
              <SelectAdapter value={mode} onChange={(e) => setMode(e.target.value)}>
                <option value="fixed">固定交付</option>
                {!files.length && <option value="live">持续共享</option>}
              </SelectAdapter>
            </label>
            <label>
              接收方
              <SelectAdapter
                value={audience}
                onChange={(e) => setAudience(e.target.value)}
              >
                <option value="link">持链接的人</option>
                <option value="users">指定已登录用户</option>
              </SelectAdapter>
            </label>
          </div>
          <p className="field-help">
            {mode === "fixed"
              ? "固定当前文件与版本；新增和覆盖不会自动进入这次交付。"
              : "此目录新增或更新的文件会对接收方可见；待审核结果除外。"}
          </p>
          {audience === "users" && (
            <MultiSelect label="接收方用户 ID" values={recipients.map(r=>r.id)} options={[...recipients,...(recipientCandidate&&!recipients.some(r=>r.id===recipientCandidate.id)?[recipientCandidate]:[])].map(r=>({value:r.id,label:`${r.name} · ${r.id}`}))}
              onValueChange={ids=>setRecipients(ids.map(id=>recipients.find(r=>r.id===id)||recipientCandidate!).filter(Boolean))} onSearchChange={setRecipientInput} loading={recipientLoading} error={recipientError}/>

          )}
          <div className="form-columns">
            <label>
              有效天数
              <SelectAdapter name="days" value={days} onChange={(e)=>setDays(e.target.value)}>
                <option value="1">1 天</option><option value="7">7 天</option><option value="30">30 天</option><option value="90">90 天</option><option value="custom">自定义</option><option value="0">永久有效</option>
              </SelectAdapter>
            </label>
            {days === "custom" && <label>自定义天数<Input type="number" min={1} max={365} value={customDays} onChange={(e)=>setCustomDays(e.target.value)} required /></label>}
            <label>
              提取码（可选）
              <Input
                name="passcode"
                type="password"
                minLength={6}
                maxLength={72}
                autoComplete="new-password"
              />
            </label>
          </div>
          {days === "0" && <p className="field-help warning">永久分享不会自动过期，只能通过撤销来停止访问。</p>}
          <label className="checkbox-label max-md:min-h-11">
            <Checkbox name="download"  defaultChecked />
            允许下载原文件
          </label>
          <p className="field-help">
            关闭下载会隐藏下载入口，预览不能防止接收方复制内容。
          </p>
          <fieldset className="share-presentation">
            <legend>分享页外观</legend>
            <div className="form-columns"><label>主题<SelectAdapter value={presentation.preset} onChange={(e)=>setPresentation({...presentation,preset:e.target.value as SharePresentation["preset"]})}><option value="default">默认</option><option value="dark">深色</option><option value="brand">品牌</option></SelectAdapter></label><label>字体<SelectAdapter value={presentation.font} onChange={(e)=>setPresentation({...presentation,font:e.target.value as SharePresentation["font"]})}><option value="system">标准</option><option value="serif">衬线</option><option value="rounded">圆润</option></SelectAdapter></label></div>
            <div className="form-columns"><label>品牌色<Input type="color" value={presentation.accent_color || "#1f7661"} onChange={(e)=>setPresentation({...presentation,accent_color:e.target.value})}/></label><label>Logo 引用<Input value={presentation.logo_ref || ""} maxLength={240} placeholder="可选的受控资源引用" onChange={(e)=>setPresentation({...presentation,logo_ref:e.target.value})}/></label></div>
            <label>欢迎语<Textarea value={presentation.welcome_message || ""} maxLength={240} placeholder="可选，显示在分享页标题下方" onChange={(e)=>setPresentation({...presentation,welcome_message:e.target.value})}/></label>
            <div className={`share-preview preview-${presentation.preset || "default"}`} style={{"--share-accent":presentation.accent_color || "#1f7661"} as React.CSSProperties}><strong>{presentation.logo_ref ? "品牌标识" : "DriveBridge"}</strong><span>{presentation.welcome_message || "文件分享"}</span></div>
          </fieldset>
          {error && (
            <p role="alert" className="error">
              {error}
            </p>
          )}
          <div className="dialog-actions">
            <Button variant="outline" type="button"  onClick={close}>
              取消
            </Button>
            <Button variant="default" type="submit"  disabled={busy || (audience === "users" && !recipients.length)}>
              {busy ? "准备分享…" : "创建分享"}
            </Button>
          </div>
        </form>
      )}
    </WorkspaceDialog>
  );
}
export function shareStatus(row: ShareRow) {
  if (row.revoked) return "已撤销";
  if (row.is_permanent) return "永久有效";
  if (new Date(row.expires_at) < new Date()) return "已到期";
  return "有效";
}

export function filterShares(rows: ShareRow[], query: string, filter: string) {
  const q = query.trim().toLowerCase();
  return rows.filter((row) => {
    const state = shareStatus(row);
    if (filter !== "all" && state !== filter) return false;
    return `${row.name} ${row.path}`.toLowerCase().includes(q);
  });
}

export function SharesPage() {
  const [batch,setBatch] = useState<ShareRow[]>([]);
  const [receiverShare, setReceiverShare] = useState<string>();
  const [shares, setShares] = useState<ShareRow[]>([]),
    [query, setQuery] = useState(""),
    [filter, setFilter] = useState("all"),
    [error, setError] = useState(""),
    [notice, setNotice] = useState(""),
    [revoking, setRevoking] = useState<ShareRow>();
  const [state, setState] = useState<'loading' | 'ready' | 'error' | 'denied'>('loading');
  const lifetime = useRef(new AbortController());
  const readController = useRef<AbortController | null>(null);
  const invalidate = () => {
    lifetime.current.abort(); readController.current?.abort();
    setReceiverShare(undefined);
    setShares([]); setBatch([]); setRevoking(undefined); setNotice(''); setError(''); setState('denied');
  };
  async function reload() {
    if (lifetime.current.signal.aborted) return;
    readController.current?.abort();
    const controller = new AbortController(); readController.current = controller;
    setState('loading'); setError(''); setShares([]);
    try {
      const rows = await api<ShareRow[]>("/shares", 'GET', undefined, {}, controller.signal);
      if (controller.signal.aborted) return;
      if (!Array.isArray(rows)) throw new Error('invalid_contract');
      setShares(rows); setState('ready');
    } catch (error) {
      if (controller.signal.aborted) return;
      if ([401, 403].includes((error as { status?: number }).status || 0)) invalidate();
      else { setError(errorText(error)); setState('error'); }
    }
  }
  useEffect(() => {
    lifetime.current = new AbortController();
    void reload();
    window.addEventListener('drivebridge-context', invalidate);
    return () => { lifetime.current.abort(); readController.current?.abort(); window.removeEventListener('drivebridge-context', invalidate); };
  }, []);
  async function copyShare(row: ShareRow) {
    const signal = lifetime.current.signal;
    if (signal.aborted) return;
    try {
      const out = await api<{ link: string }>(`/shares/${row.id}/link`, 'POST', {}, {}, signal);
      if (signal.aborted) return;
      await navigator.clipboard.writeText(out.link);
      if (!signal.aborted) setNotice('已复制分享链接');
    } catch (error) {
      if (signal.aborted) return;
      if ([401, 403].includes((error as { status?: number }).status || 0)) invalidate();
      else setError(errorText(error));
    }
  }
  async function revokeShares(rows: ShareRow[]) {
    const signal = lifetime.current.signal;
    const failed: ShareRow[] = [];
    for (const row of rows) {
      if (signal.aborted) return;
      try { await api(`/shares/${row.id}/revoke`, 'POST', {}, {}, signal); }
      catch (error) {
        if (signal.aborted) return;
        if ([401, 403].includes((error as { status?: number }).status || 0)) { invalidate(); return; }
        failed.push(row);
      }
    }
    if (signal.aborted) return;
    await reload();
    if (signal.aborted) return;
    if (failed.length) { if (rows.length > 1) setBatch(failed); throw new Error('request_failed'); }
    setNotice(rows.length > 1 ? '所选分享已撤销' : '分享已撤销');
  }
  const visible = filterShares(shares, query, filter);
  return (
    <section aria-label="分享管理" className="min-w-0 space-y-4 max-md:[&_button:not([role=checkbox])]:min-h-11">
      <WorkspacePageHeader title="分享交付" description="分享是独立访问授权。撤销后停止新的访问请求，已下载的副本无法收回。" actions={<Button type="button" variant="outline" className="min-h-11 md:min-h-8" disabled={state==='loading'||state==='denied'} onClick={()=>void reload()}>刷新分享</Button>}/>
      <div className="flex min-w-0 flex-wrap items-center gap-2 [&_[data-slot=input-group]]:min-w-0 [&_[data-slot=input-group]]:flex-[1_1_14rem] [&_[data-slot=input-group]]:max-w-sm max-md:[&_[data-slot=input-group]]:min-h-11">
        <SearchField aria-label="搜索分享" placeholder="搜索名称或路径…" value={query} onChange={(e) => setQuery(e.target.value)} />
        <SelectAdapter aria-label="分享状态" value={filter} onChange={(e) => setFilter(e.target.value)}>
          {["all", "有效", "已到期", "已撤销"].map((id) => (
            <option key={id} value={id}>{id === "all" ? "全部" : id}</option>
          ))}
        </SelectAdapter>
        {state === 'ready' && <span className="text-xs text-muted-foreground">{visible.length} 个分享</span>}
      </div>
      {error && (
        <p role="alert" className="banner error">
          {error}
        </p>
      )}
      {notice && (
        <p role="status" className="banner">
          {notice}
        </p>
      )}
      {state === 'loading' && <p role="status">正在读取分享…</p>}
      {state === 'denied' && <p role="alert">身份或范围已变化，分享数据已清除。请重新进入。</p>}
      <div className="min-w-0 [&>section]:p-0 max-md:[&_td]:px-1 max-md:[&_th]:px-1 max-md:[&_td[data-label=类型]]:hidden max-md:[&_th:has([data-share-type])]:hidden max-md:[&_button[data-size=icon]]:min-w-11" role="list" aria-label="分享" onKeyDown={(e)=>{
        if(e.defaultPrevented || !(e.target instanceof HTMLElement) || !e.target.matches('[data-list-item]'))return;
        handleListKey(e.nativeEvent,e.currentTarget,{open(el){const row=visible.find(s=>s.id===el.dataset.id);if(row&&!row.revoked&&(row.is_permanent||shareStatus(row)==='有效'))void copyShare(row);},cancel(el){const row=visible.find(s=>s.id===el.dataset.id);if(row&&!row.revoked)setRevoking(row);}});
      }}>
        {state === 'ready' && visible.length > 0 && <DataTable searchable={false} label="分享交付" selectionActions={selected=><Button type="button" variant="destructive" disabled={!selected.some(row=>!row.revoked)} onClick={()=>setBatch(selected.filter(row=>!row.revoked))}>撤销所选</Button>} data={visible} getRowId={(row)=>String(row.id)} columns={[{id:"type",header:<span data-share-type>类型</span>,sortable:false,cell:(row)=><><span className="file-type">
              {row.audience === "users" ? <Lock /> : <Share2 />}
            </span></>},{id:"details",header:"内容与状态",cell:(row)=><><div className="min-w-0 space-y-1 break-all [&_small]:block [&_small]:text-xs [&_small]:text-muted-foreground">
              <strong className="block text-[13px] font-semibold">{row.name}</strong>
              <small>
                {row.mode === "live" ? "持续共享" : "固定交付"} ·{" "}
                {row.audience === "users" ? "指定用户" : "链接访问"} ·{" "}
                {row.allow_download ? "可下载" : "仅预览"}
              </small>
              <small>
                {row.is_permanent ? "永久有效" : shareStatus(row) === "有效"
                  ? `有效期至 ${new Date(row.expires_at).toLocaleDateString("zh-CN")}`
                  : shareStatus(row)}
              </small>
            </div></>},{id:"actions",header:"操作",sortable:false,cell:(row)=><div className="flex flex-wrap gap-2 max-md:flex-col max-md:items-center">{!row.revoked && (row.is_permanent || new Date(row.expires_at) > new Date()) && (
              <>
                <Button variant="outline" size="sm" type="button" className="max-md:min-w-11 max-md:px-2" aria-label="复制链接" title="复制链接" onClick={() => void copyShare(row)}
                >
                  <Link size={14} />
                  <span className="max-md:hidden">复制链接</span>
                </Button>
                <ActionMenu label={`分享 ${row.name} 的更多操作`} items={[...(row.audience === 'link' ? [{ label: '接收设备与额度', onSelect: () => setReceiverShare(row.id) }] : []), {label:"撤销分享",destructive:true,onSelect:()=>setRevoking(row)}]}/>
              </>
            )}</div>}]} />}
        {state === 'ready' && !shares.length && (
          <div className="space-y-3 border-y border-border py-8 text-sm text-muted-foreground">
            <Share2 size={36} />
            <h3>还没有分享</h3>
            <p>在文件页选择文件或目录，再创建分享链接。</p>
            <a className={buttonVariants({variant:"default",size:"default",className:""})} href="#/files">去网盘选择文件</a>
          </div>
        )}
        {state === 'ready' && !!shares.length && !visible.length && (
          <div className="space-y-2 border-y border-border py-8 text-sm text-muted-foreground">
            <h3>没有符合条件的分享</h3>
            <p>试试其他关键词或状态。</p>
          </div>
        )}
      </div>
      {revoking && <ConfirmDialog open onOpenChange={open=>{if(!open)setRevoking(undefined);}} title="撤销分享" description={`撤销「${revoking.name}」后，链接与指定用户将无法继续访问。`} destructive confirmLabel="确认撤销分享" onConfirm={()=>revokeShares([revoking])}/>}
      {batch.length>0 && <ConfirmDialog open onOpenChange={open=>{if(!open)setBatch([]);}} title="批量撤销分享" description={`将撤销 ${batch.length} 个分享，已下载的副本不会删除。`} destructive confirmLabel="确认撤销所选" onConfirm={()=>revokeShares(batch)}/>}
      {receiverShare && <ShareReceiversDialog key={receiverShare} id={receiverShare} close={() => setReceiverShare(undefined)}/>}

    </section>
  );
}
function SharedThumbnail({ file, share }: { file: DriveFile; share: string }) {
  const [failed, setFailed] = useState(false);
  const image =
    file.mime_type.startsWith("image/") && file.mime_type !== "image/svg+xml";
  return failed ? (
    <span className="shared-file-icon">
      {file.mime_type.startsWith("video/") ? "视频" : "文件"}
    </span>
  ) : (
    <img
      loading="lazy"
      src={`/api/v1/shared/${share}/files/${file.id}/${image ? "content" : "thumbnail"}`}
      alt=""
      onError={() => setFailed(true)}
    />
  );
}
export function SharedView() {
  const [id] = useState(() => location.hash.slice(1).split("/")[1] || "");
  const grant = useRef(location.hash.slice(1).split("/")[2] || "");
  const [passcode, setPasscode] = useState("");
  const [error, setError] = useState("");
  const [needsOpen, setNeedsOpen] = useState(false);
  const [busy, setBusy] = useState(true);
  const [opened, setOpened] = useState<ShareOpenResult>();
  const [remembered, setRemembered] = useState(false);
  const [deviceNotice, setDeviceNotice] = useState("");
  const lifetime = useRef(new AbortController());
  const active = useRef(false);
  const [data, setData] = useState<{
      name: string;
      mode: string;
      allow_download: boolean;
      expires_at: string;
      is_permanent?: boolean;
      presentation?: SharePresentation;
      files: DriveFile[];
    }>(),
    [selected, setSelected] = useState<DriveFile>();
  async function load(signal: AbortSignal) {
    const out = await api<NonNullable<typeof data>>(`/shared/${encodeURIComponent(id)}`, 'GET', undefined, {}, signal);
    signal.throwIfAborted();
    setData(out); setNeedsOpen(false); setError("");
  }
  async function open(signal: AbortSignal) {
    if (!grant.current) throw new Error('full_link_required');
    const result = await openShareProof(id, grant.current, passcode, signal);
    signal.throwIfAborted();
    grant.current = ''; setPasscode(''); setOpened(result);
    await load(signal);
  }
  async function submit() {
    if (active.current || lifetime.current.signal.aborted) return;
    active.current = true; setBusy(true); setError('');
    const signal = lifetime.current.signal;
    try { await open(signal); }
    catch (error) { if (!signal.aborted) { setData(undefined); setSelected(undefined); setError(receiverError(error)); } }
    finally { if (!signal.aborted) { active.current = false; setBusy(false); } }
  }
  async function rememberDevice() {
    if (active.current || lifetime.current.signal.aborted) return;
    const signal = lifetime.current.signal;
    active.current = true; setBusy(true); setDeviceNotice('');
    try { await registerShareDevice(id, signal); if (!signal.aborted) setRemembered(true); }
    catch (error) { if (!signal.aborted) setDeviceNotice(receiverError(error)); }
    finally { if (!signal.aborted) { active.current = false; setBusy(false); } }
  }
  useEffect(() => {
    const controller = new AbortController(); lifetime.current = controller;
    const signal = controller.signal;
    const cleanHash = '#share/' + encodeURIComponent(id);
    history.replaceState(null, '', location.pathname + location.search + cleanHash);
    const invalidate = () => {
      controller.abort(); grant.current = ''; active.current = false;
      setData(undefined); setSelected(undefined); setPasscode(''); setOpened(undefined);
      setRemembered(false); setDeviceNotice(''); setNeedsOpen(false); setBusy(false);
      setError('身份或分享链接已变化，内容已清除。请重新打开完整分享链接。');
    };
    const hashChanged = () => { if (location.hash !== cleanHash) invalidate(); };
    window.addEventListener('drivebridge-context', invalidate);
    window.addEventListener('hashchange', hashChanged);
    active.current = true;
    void (async () => {
      try {
        if (grant.current) await open(signal);
        else {
          try { await load(signal); }
          catch (error) {
            signal.throwIfAborted();
            if (!(error instanceof Error) || !['unauthorized', 'share_unavailable', 'share_passcode_required'].includes(error.message)) throw error;
            const device = await loadShareDevice(id);
            signal.throwIfAborted();
            if (!device?.id) throw new Error('full_link_required');
            const result = await openShareDevice(id, device, signal);
            signal.throwIfAborted(); setOpened(result); setRemembered(true);
            await load(signal);
          }
        }
      } catch (error) {
        if (!signal.aborted) { setData(undefined); setSelected(undefined); setNeedsOpen(!!grant.current); setError(receiverError(error)); }
      } finally { if (!signal.aborted) { active.current = false; setBusy(false); } }
    })();
    return () => {
      controller.abort();
      window.removeEventListener('drivebridge-context', invalidate);
      window.removeEventListener('hashchange', hashChanged);
    };
  }, []);
  return (
    <main className={`shared-view preview-${data?.presentation?.preset || "default"}`} style={{"--share-accent":data?.presentation?.accent_color || "#1f7661", fontFamily:data?.presentation?.font === "serif" ? "Georgia, serif" : data?.presentation?.font === "rounded" ? "Trebuchet MS, sans-serif" : undefined} as React.CSSProperties}>
      <header>
        <span className="brandmark">
          <Share2 />
        </span>
        <div>
          <small>{data?.presentation?.logo_ref || "DriveBridge"} · 文件分享</small>
          <h1>{data?.name || "查看分享"}</h1>
          {data?.presentation?.welcome_message && <p className="share-welcome">{data.presentation.welcome_message}</p>}
        </div>
      </header>
      {error && (
        <p className="banner error" role="alert">
          {error}
        </p>
      )}
      {needsOpen && (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void submit();
          }}
        >
          <Lock size={28} />
          <p>提取码分享需验证；指定用户分享需先在此站点登录。</p>
          <label>
            提取码
            <Input
              type="password"
              autoFocus
              value={passcode}
              onChange={(e) => setPasscode(e.target.value)}
            />
          </label>
          <Button variant="default" type="submit" disabled={busy}>{busy ? '正在验证…' : '打开分享'}</Button>
          <a href="/" target="_blank" rel="noopener">
            前往登录
          </a>
        </form>
      )}
      {busy && <p role="status">正在验证分享…</p>}
      {data && opened?.session_kind === 'proof' && opened.receiver_kind === 'anonymous' && !remembered && <section aria-label="记住接收设备">
        <p>仅在你信任的个人设备上登记。登记后，此浏览器可免提取码复访；分享者可随时撤销。私钥不可导出且仅保存在本站浏览器存储中，清除站点数据后失效。</p>
        <Button type="button" variant="outline" disabled={busy} onClick={() => void rememberDevice()}>记住此设备</Button>
      </section>}
      {data && remembered && <p role="status">此设备已登记，可在此浏览器免码复访。请保留原始完整链接以便设备失效后恢复。</p>}
      {deviceNotice && <p role="alert">{deviceNotice}</p>}
      {data && (
        <>
          <p className="field-help">
            {data.mode === "fixed" ? "固定交付" : "持续共享，内容可能更新"} ·
            {data.is_permanent ? "永久有效" : `有效期至 ${new Date(data.expires_at).toLocaleDateString("zh-CN")}`}
          </p>
          <div className="shared-grid">
            {data.files.map((f) => (
              <article key={f.id}>
                <Button variant="ghost" type="submit" className="shared-file" onClick={() => setSelected(f)}>
                  <SharedThumbnail file={f} share={id} />
                  <strong>{f.path}</strong>
                  <small>{bytes(f.size_bytes)}</small>
                </Button>
                {data.allow_download && (
                  <a
                    className={buttonVariants({variant:"outline",size:"sm",className:""})}
                    href={`/api/v1/shared/${id}/files/${f.id}/content?download=true`}
                  >
                    下载原文件
                  </a>
                )}
              </article>
            ))}
          </div>
          {!data.files.length && (
            <p>当前没有可访问的文件，文件可能已被移入回收站。</p>
          )}
        </>
      )}
      {selected && (
        <WorkspaceDialog
          title={selected.path}
          close={() => setSelected(undefined)}
        >
          <MediaPreview
            file={selected}
            sourceURL={`/api/v1/shared/${id}/files/${selected.id}/content`}
          />
        </WorkspaceDialog>
      )}
    </main>
  );
}
