import { EmptyState } from '../../components/EmptyState';
import { useEffect, useReducer, useRef, useState, type ReactNode } from 'react';
import type { DriveFile, Space } from '../../api';
import { api, bytes } from '../../api';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { SelectAdapter } from '../../components/SelectAdapter';
import { WorkspacePageHeader } from '../../components/WorkspacePageHeader';
import { AppSheet } from '../../components/AppSheet';
import { FilePreviewDialog } from '../files/FilePreviewDialog';
import { MediaIcon, MediaPreview } from '../../Media';
import { BookOpen, FileCheck2, FileText, FolderOpen, Grid2X2, List, Package2, PanelLeft, Search, Shapes, X } from 'lucide-react';
import { RegisterResourceDialog, ResourceSnapshot, ResourceIntegrity } from './components';
import { getResourceDescriptor, listResourceDescriptors } from './clients/resource-descriptors';
import { isResourceDescriptor, resourceKinds } from './contract';
import { projectResourceList, type ResourceSort } from './projection';
import { initialResourcePageUiState, resourcePageUiReducer } from './state';
import type { ResourceDescriptor as Resource, ResourceLifecycle } from './contract';
import './resource-browser.css';

const kinds = resourceKinds;
const validResource = isResourceDescriptor;

export function ResourcesPage({ scope, spaces }: { scope: string; spaces: Space[] }) {
  return <ResourceList key={scope} scope={scope} spaces={spaces} />;
}

function ResourceList({ spaces, scope }: { spaces: Space[]; scope: string }) {
  const [ui, dispatchUi] = useReducer(resourcePageUiReducer, initialResourcePageUiState);
  const registering = ui.registrationOpen;
  const [kind, setKind] = useState('');
  const [space, setSpace] = useState('');
  const [search, setSearch] = useState('');
  const [sort, setSort] = useState<ResourceSort>('created');
  const [items, setItems] = useState<Resource[]>([]);
  const [cursor, setCursor] = useState('');
  const [next, setNext] = useState('');
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<ResourceLifecycle>('loading');
  const [previewResource, setPreviewResource] = useState<Resource | null>(null);
  const selected = ui.selectedDescriptor ? items.find(item => item.descriptor_id === ui.selectedDescriptor) ?? null : null;
  const active = useRef<AbortController | null>(null);
  const trigger = useRef<HTMLButtonElement | HTMLElement | null>(null);
  const registerTrigger = useRef<HTMLButtonElement | null>(null);
  const refreshTrigger = useRef<HTMLButtonElement | null>(null);
  const restoreRegistration = useRef(false);

  useEffect(() => {
    try {
      const saved = localStorage.getItem(`drivebridge:resources-view:${scope}`);
      if (saved === 'grid' || saved === 'list') dispatchUi({ type: 'set-view', mode: saved });
    } catch { /* optional preference */ }
  }, [scope]);
  useEffect(() => {
    try { localStorage.setItem(`drivebridge:resources-view:${scope}`, ui.viewMode); } catch { /* optional preference */ }
  }, [scope, ui.viewMode]);
  useEffect(() => {
    if (!restoreRegistration.current || state === 'loading' || registering) return;
    restoreRegistration.current = false;
    if (state !== 'denied' && state !== 'offline' && document.activeElement === document.body) (registerTrigger.current || refreshTrigger.current)?.focus();
  }, [state, registering]);

  const deny = () => {
    restoreRegistration.current = false;
    active.current?.abort();
    setPreviewResource(null);
    dispatchUi({ type: 'close-inspector' });
    setItems([]);
    setNext('');
    setState('denied');
  };
  useEffect(() => {
    const controller = new AbortController();
    active.current = controller;
    setState('loading');
    const invalidate = () => { restoreRegistration.current = false; controller.abort(); setPreviewResource(null); dispatchUi({ type: 'close-inspector' }); setItems([]); setNext(''); setState('denied'); };
    const offline = () => { invalidate(); setState('offline'); };
    window.addEventListener('drivebridge-context', invalidate);
    window.addEventListener('offline', offline);
    const query = new URLSearchParams({ limit: '50' });
    if (kind) query.set('kind', kind);
    if (space) query.set('space_id', space);
    if (cursor) query.set('cursor', cursor);
    if (!navigator.onLine) offline();
    else void listResourceDescriptors(query, controller.signal).then(page => {
      if (controller.signal.aborted) return;
      if (!page || page.total !== null || page.total_kind !== 'unknown' || !Array.isArray(page.items) || page.items.length > 50 || typeof page.next_cursor !== 'string' || page.next_cursor.length > 16384 || (cursor && page.next_cursor === cursor) || !page.items.every(row => validResource(row) && (!kind || row.kind === kind) && (!space || row.space_id === space)) || new Set(page.items.map(row => row.descriptor_id)).size !== page.items.length) throw new Error('invalid_contract');
      setItems(old => cursor ? [...new Map([...old, ...page.items].map(row => [row.descriptor_id, row])).values()] : page.items);
      setNext(page.next_cursor);
      setState('ready');
    }).catch((error: { status?: number }) => {
      if (controller.signal.aborted) return;
      setItems([]); setPreviewResource(null); dispatchUi({ type: 'close-inspector' }); setNext('');
      setState(error.status === 401 || error.status === 403 ? 'denied' : error.status === 410 ? 'stale' : error.status === 404 || error.status === 405 ? 'unsupported' : 'error');
    });
    return () => { controller.abort(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', offline); };
  }, [kind, space, cursor, attempt]);

  const refresh = () => { active.current?.abort(); setPreviewResource(null); setState('loading'); setItems([]); dispatchUi({ type: 'close-inspector' }); setCursor(''); setNext(''); setAttempt(value => value + 1); };
  const filteredItems = projectResourceList(items, search, sort).filteredItems;
  const selectResource = (row: Resource, target?: HTMLElement) => { if (target) trigger.current = target; dispatchUi({ type: 'select', descriptor: row.descriptor_id }); };
  const openResource = (row: Resource, target?: HTMLElement) => { selectResource(row, target); if (row.file_ref) setPreviewResource(row); };
  const kindIcon = (value: Resource['kind'], size = 17) => value === 'skill' ? <Shapes size={size} /> : value === 'template' ? <BookOpen size={size} /> : <Package2 size={size} />;
  const categoryCount = (value: Resource['kind'] | '') => value ? items.filter(item => item.kind === value).length : items.length;

  return <section aria-label="资源库" className="min-w-0 space-y-4">
    <WorkspacePageHeader title="资源库" description="用 Finder 式浏览查看 Skill、模板与资料包的固定版本。保存不等于检查通过、安装或执行。" actions={<div className="flex items-center gap-2"><span className="hidden rounded-full bg-primary/10 px-2 py-1 text-[11px] font-medium text-primary md:inline">{state === 'ready' ? `${items.length} 个已加载资源` : '按当前范围读取'}</span>{scope && state === 'ready' ? <Button ref={registerTrigger} className="min-h-11 gap-1.5 text-[13px] md:min-h-8" onClick={() => dispatchUi({ type: 'set-registration', open: true })}><FileCheck2 />登记固定资源</Button> : undefined}</div>} />
    {registering && <RegisterResourceDialog scope={scope} spaces={spaces} close={() => { restoreRegistration.current = true; dispatchUi({ type: 'set-registration', open: false }); refresh(); }} />}
    {state !== 'denied' && <div className="flex flex-wrap items-end gap-3 [&_label]:min-w-0 [&_label]:max-w-full md:[&_button]:min-h-8">
      <label className="grid min-w-56 flex-1 gap-2 text-sm"><span>筛选已加载资源</span><span className="relative"><Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" /><Input aria-label="筛选已加载资源" className="min-h-11 pl-9" value={search} onChange={event => setSearch(event.target.value)} placeholder="按名称筛选当前页…" /></span></label>
      <label className="grid gap-2 text-sm">资源类型<SelectAdapter aria-label="资源类型" className="min-h-11" value={kind} onChange={event => { if (event.target.value !== kind) { refresh(); setKind(event.target.value); } }}><option value="">全部类型</option>{Object.entries(kinds).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</SelectAdapter></label>
      <label className="grid gap-2 text-sm">资源空间<SelectAdapter aria-label="资源空间" className="min-h-11" value={space} onChange={event => { if (event.target.value !== space) { refresh(); setSpace(event.target.value); } }}><option value="">当前授权范围</option>{spaces.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</SelectAdapter></label>
      <label className="grid gap-2 text-sm">排序<SelectAdapter aria-label="资源排序" className="min-h-11" value={sort} onChange={event => setSort(event.target.value as ResourceSort)}><option value="created">最近登记</option><option value="name">名称</option></SelectAdapter></label>
      <Button ref={refreshTrigger} variant="outline" className="min-h-11 self-end" disabled={state === 'loading'} onClick={refresh}>刷新资源库</Button>
    </div>}
    {state === 'loading' && <ResourceSkeleton />}
    {state === 'ready' && !filteredItems.length && <div role="status"><EmptyState title={items.length ? '当前页没有匹配资源' : '还没有登记资源'} description={items.length ? '搜索只作用于当前已加载页；继续翻页或清除筛选后再查看。' : '登记固定版本的 Skill、模板或资料包，资源会保留来源和修订。'} action={scope ? <Button className="min-h-11 gap-1.5" onClick={() => dispatchUi({ type: 'set-registration', open: true })}><FileCheck2 />登记固定资源</Button> : undefined} /></div>}
    {state === 'stale' && <ResourceNotice tone="warning" title="资源分页已过期" detail="旧结果已清除，请刷新重读。" action={<Button variant="outline" className="min-h-11" onClick={refresh}>重新读取资源</Button>} />}
    {state === 'error' && <ResourceNotice tone="danger" title="资源描述暂不可用" detail="合同不匹配或服务错误不代表没有资源。" action={<Button variant="outline" className="min-h-11" onClick={refresh}>重试读取</Button>} />}
    {state === 'unsupported' && <ResourceNotice title="当前服务尚未开放资源描述列表" detail="资源库会保留明确的 unsupported 状态，不使用模拟数据填充。" />}
    {state === 'denied' && <ResourceNotice tone="danger" title="权限或范围已变化" detail="资源列表与详情已清除，请重新登录或切换范围。" />}
    {state === 'offline' && <ResourceNotice tone="warning" title="当前离线" detail="资源已清除；联网后请手动刷新。" action={<Button variant="outline" className="min-h-11" onClick={refresh}>重新读取</Button>} />}
    {!!filteredItems.length && <div className="resource-browser" data-view={ui.viewMode}>
      <aside className="resource-sidebar" aria-label="资源分类"><p className="resource-sidebar-heading">浏览</p><div className="resource-sidebar-list"><Button type="button" variant="ghost" className="resource-sidebar-button" aria-current={!kind ? 'true' : undefined} onClick={() => { if (kind) { refresh(); setKind(''); } }}><FolderOpen size={16} /><span>全部资源</span><span className="resource-sidebar-count">{categoryCount('')}</span></Button>{(Object.entries(kinds) as [Resource['kind'], string][]).map(([value, label]) => <Button key={value} type="button" variant="ghost" className="resource-sidebar-button" aria-current={kind === value ? 'true' : undefined} onClick={() => { if (kind !== value) { refresh(); setKind(value); } }}>{kindIcon(value, 16)}<span>{label}</span><span className="resource-sidebar-count">{categoryCount(value)}</span></Button>)}</div></aside>
      <main className="resource-browser-main" aria-label="资源浏览"><div className="resource-browser-toolbar"><div className="flex min-w-0 items-center gap-2"><PanelLeft className="size-4 text-muted-foreground" /><span className="truncate text-sm font-semibold">{kind ? kinds[kind as Resource['kind']] : '全部资源'}</span><span className="text-xs text-muted-foreground">{filteredItems.length} 项</span></div><div className="ml-auto flex items-center gap-2"><div className="resource-view-switch" role="group" aria-label="资源视图"><Button type="button" variant="ghost" size="icon" aria-label="资源详情列表" aria-pressed={ui.viewMode === 'list'} onClick={() => dispatchUi({ type: 'set-view', mode: 'list' })}><List size={16} /></Button><Button type="button" variant="ghost" size="icon" aria-label="资源大图网格" aria-pressed={ui.viewMode === 'grid'} onClick={() => dispatchUi({ type: 'set-view', mode: 'grid' })}><Grid2X2 size={16} /></Button></div></div></div><div className="resource-browser-content">{ui.viewMode === 'list' ? <ResourceListView items={filteredItems} selected={selected?.descriptor_id} state={state} selectResource={selectResource} openResource={openResource} /> : <ResourceGridView items={filteredItems} selected={selected?.descriptor_id} state={state} selectResource={selectResource} openResource={openResource} />}{state === 'ready' && next && <Button variant="outline" aria-label="更多资源描述" className="mt-3 min-h-11" onClick={() => { dispatchUi({ type: 'close-inspector' }); setCursor(next); }}>加载更多资源</Button>}</div></main>
      <aside className={`resource-inspector${selected ? ' resource-inspector-open' : ''}`} aria-label="资源检查器">{selected ? <ResourceDetail key={selected.descriptor_id} original={selected} onDenied={deny} close={() => { dispatchUi({ type: 'close-inspector' }); trigger.current?.focus(); }} onPreview={() => openResource(selected)} /> : <div className="resource-inspector-empty"><div><FileText className="mx-auto mb-3 size-8" /><p className="m-0">选择一个资源</p><p className="mt-1">单击查看固定版本与检查状态</p></div></div>}</aside>
    </div>}
    {previewResource && <ResourceQuickLook resource={previewResource} close={() => setPreviewResource(null)} onDenied={deny} />}
    <p className="flex items-center gap-1.5 text-xs text-muted-foreground"><FileCheck2 className="size-3.5" />仅提供固定字节编码检查；完整格式检查、安装与领域交接尚未提供，本页不授予额外权限。列表按原标识分页，未提供全库数量。</p>
  </section>;
}

function ResourceListView({ items, selected, state, selectResource, openResource }: { items: Resource[]; selected?: string; state: ResourceLifecycle; selectResource: (row: Resource, target?: HTMLElement) => void; openResource: (row: Resource, target?: HTMLElement) => void }) {
  return <ul aria-label="资源描述列表" className="resource-list">{items.map((row, index) => <li key={row.descriptor_id} className="resource-list-row" data-selected={selected === row.descriptor_id} tabIndex={state === 'ready' ? 0 : -1} aria-selected={selected === row.descriptor_id} onClick={event => selectResource(row, event.currentTarget)} onDoubleClick={event => openResource(row, event.currentTarget)} onKeyDown={event => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); openResource(row, event.currentTarget); } else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') { event.preventDefault(); focusResourceSibling(event.currentTarget, event.key === 'ArrowDown' ? index + 1 : index - 1); } }}><div className="resource-type-icon">{row.file_ref ? <MediaIcon file={{ path: row.file_ref, mime_type: 'application/octet-stream' }} /> : row.kind === 'reference_pack' ? <Package2 size={18} /> : <FileCheck2 size={18} />}</div><div className="resource-row-copy"><h3 className="resource-row-title" title={row.display_name}>{row.display_name}</h3><p className="resource-row-meta">{kinds[row.kind]} · 修订 {row.descriptor_revision} · {row.collection_snapshot_id ? '固定快照' : '固定文件版本'} · {new Date(row.created_at).toLocaleDateString('zh-CN')}</p></div><div className="resource-row-actions"><span className="hidden rounded-full border border-border px-2 py-1 text-[11px] text-muted-foreground md:inline">未检查</span><Button type="button" variant="outline" className="min-h-10 shrink-0 text-[13px] md:min-h-8" aria-label={`查看资源描述 ${row.display_name}`} disabled={state !== 'ready'} onClick={event => { event.stopPropagation(); selectResource(row, event.currentTarget); }}>查看详情</Button>{row.file_ref && <Button type="button" variant="ghost" className="min-h-10 shrink-0 text-[13px] md:min-h-8" aria-label={`预览固定版本 ${row.display_name}`} onClick={event => { event.stopPropagation(); openResource(row, event.currentTarget); }}>预览</Button>}</div></li>)}</ul>;
}

function ResourceGridView({ items, selected, state, selectResource, openResource }: { items: Resource[]; selected?: string; state: ResourceLifecycle; selectResource: (row: Resource, target?: HTMLElement) => void; openResource: (row: Resource, target?: HTMLElement) => void }) {
  return <div className="resource-grid" aria-label="资源大图网格">{items.map((row, index) => <article key={row.descriptor_id} className="resource-grid-card" data-selected={selected === row.descriptor_id} tabIndex={state === 'ready' ? 0 : -1} aria-selected={selected === row.descriptor_id} onClick={event => selectResource(row, event.currentTarget)} onDoubleClick={event => openResource(row, event.currentTarget)} onKeyDown={event => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); openResource(row, event.currentTarget); } else if (event.key === 'ArrowRight' || event.key === 'ArrowDown' || event.key === 'ArrowLeft' || event.key === 'ArrowUp') { event.preventDefault(); focusResourceSibling(event.currentTarget, index + (event.key === 'ArrowRight' || event.key === 'ArrowDown' ? 1 : -1)); } }}><div className="resource-grid-icon">{row.file_ref ? <MediaIcon file={{ path: row.file_ref, mime_type: 'application/octet-stream' }} size={42} /> : kindGridIcon(row.kind)}</div><div className="resource-grid-copy"><h3 className="resource-grid-title" title={row.display_name}>{row.display_name}</h3><p className="resource-grid-meta">{kinds[row.kind]} · 修订 {row.descriptor_revision}<br />{row.collection_snapshot_id ? '固定快照' : '固定文件版本'} · 未检查</p></div><div className="flex flex-wrap gap-2"><Button type="button" variant="outline" className="min-h-10 text-[13px] md:min-h-8" aria-label={`查看资源描述 ${row.display_name}`} onClick={event => { event.stopPropagation(); selectResource(row, event.currentTarget); }}>查看详情</Button>{row.file_ref && <Button type="button" variant="ghost" className="min-h-10 text-[13px] md:min-h-8" aria-label={`预览固定版本 ${row.display_name}`} onClick={event => { event.stopPropagation(); openResource(row, event.currentTarget); }}>预览</Button>}</div></article>)}</div>;
}

function focusResourceSibling(current: HTMLElement, index: number) {
  const container = current.parentElement;
  if (!container) return;
  const rows = Array.from(container.querySelectorAll<HTMLElement>('[tabindex="0"]'));
  rows[Math.max(0, Math.min(rows.length - 1, index))]?.focus();
}

function kindGridIcon(kind: Resource['kind']) { return kind === 'skill' ? <Shapes /> : kind === 'template' ? <BookOpen /> : <Package2 />; }
function ResourceSkeleton() { return <div role="status" aria-label="读取资源描述" className="space-y-2 rounded-xl border border-border bg-card p-4"><span className="sr-only">读取资源描述…</span>{Array.from({ length: 4 }, (_, index) => <div key={index} className="flex items-center gap-3 py-2"><div className="size-9 animate-pulse rounded-lg bg-muted" /><div className="min-w-0 flex-1 space-y-2"><div className="h-3 w-2/5 animate-pulse rounded bg-muted" /><div className="h-2.5 w-3/5 animate-pulse rounded bg-muted" /></div><div className="h-8 w-20 animate-pulse rounded bg-muted" /></div>)}</div>; }
function ResourceNotice({ title, detail, action, tone = 'neutral' }: { title: string; detail: string; action?: ReactNode; tone?: 'neutral' | 'warning' | 'danger' }) { return <section role={tone === 'danger' ? 'alert' : 'status'} className={`rounded-xl border p-4 ${tone === 'danger' ? 'border-destructive/30 bg-destructive/5' : tone === 'warning' ? 'border-amber-500/30 bg-amber-500/5' : 'border-border bg-card'}`}><div className="flex flex-wrap items-start justify-between gap-3"><div><h3 className="m-0 text-sm font-semibold">{title}</h3><p className="mt-1 text-sm leading-5 text-muted-foreground">{detail}</p></div>{action}</div></section>; }

function ResourceDetail({ original, close, onDenied, onPreview }: { original: Resource; close: () => void; onDenied: () => void; onPreview: () => void }) {
  const [row, setRow] = useState<Resource | null>(original);
  const [failed, setFailed] = useState(false);
  const [checking, setChecking] = useState(false);
  const [members, setMembers] = useState(false);
  const checkTrigger = useRef<HTMLButtonElement | null>(null);
  const membersTrigger = useRef<HTMLButtonElement | null>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  const denied = useRef(onDenied);
  denied.current = onDenied;
  const displayRow = row ?? original;
  useEffect(() => {
    const controller = new AbortController();
    heading.current?.focus();
    setRow(original);
    setFailed(false);
    const invalidate = () => { controller.abort(); setRow(null); setFailed(true); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('offline', invalidate);
    if (!navigator.onLine) invalidate();
    else void getResourceDescriptor(original.descriptor_id, controller.signal).then(value => {
      if (controller.signal.aborted) return;
      if (!validResource(value) || value.descriptor_id !== original.descriptor_id || value.space_id !== original.space_id || value.descriptor_revision !== original.descriptor_revision || value.kind !== original.kind || value.file_ref !== original.file_ref || value.collection_snapshot_id !== original.collection_snapshot_id) throw new Error('source_changed');
      setRow(value);
    }).catch((error: { status?: number }) => { if (controller.signal.aborted) return; setFailed(true); setRow(null); if (error.status === 401 || error.status === 403) denied.current(); });
    return () => { controller.abort(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', invalidate); };
  }, [original]);
  return <section aria-label="固定资源描述" className="space-y-4 bg-background"><header className="flex flex-wrap items-center gap-3"><div className="min-w-0 flex-1"><p className="m-0 text-[11px] font-medium uppercase tracking-[0.12em] text-muted-foreground">资源检查器</p><h3 ref={heading} tabIndex={-1} className="mt-1 truncate font-semibold" title={original.display_name}>{original.display_name}</h3></div><Button variant="ghost" size="icon" className="size-9" aria-label="关闭资源描述" onClick={close}><X /></Button></header>{failed ? <p role="alert">原资源描述不可用或已变化，未替换目标；请刷新列表后核对。</p> : <><div className="rounded-lg border border-border bg-muted/40 p-3 text-sm"><p className="break-all font-medium">{displayRow.file_ref || `集合快照：${displayRow.collection_snapshot_id}`}</p><p className="mt-2 break-all text-xs text-muted-foreground">描述标识：{displayRow.descriptor_id} · 修订 {displayRow.descriptor_revision}</p><p className="mt-1 text-xs text-muted-foreground">{kinds[displayRow.kind]} · 登记于 {new Date(displayRow.created_at).toLocaleString('zh-CN')}</p></div>{!row && <p role="status">重新鉴权并核对原描述…</p>}<p className="text-sm">检查状态：未检查，不代表内容安全、可执行或可安装。</p><div className="flex flex-wrap gap-2">{displayRow.file_ref && <Button variant="default" className="min-h-11 gap-1.5" onClick={onPreview}><FileText size={16} />预览固定版本</Button>}<Button ref={checkTrigger} variant="outline" className="min-h-11" aria-expanded={checking} onClick={() => setChecking(value => !value)}>查看编码检查</Button>{displayRow.collection_snapshot_id && <Button ref={membersTrigger} variant="outline" className="min-h-11" aria-expanded={members} onClick={() => setMembers(value => !value)}>{members ? '收起固定快照成员' : '查看固定快照成员'}</Button>}</div>{checking && <ResourceIntegrity descriptor={displayRow.descriptor_id} revision={displayRow.descriptor_revision} onDenied={() => denied.current()} close={() => { setChecking(false); checkTrigger.current?.focus(); }} />}{members && displayRow.collection_snapshot_id && <ResourceSnapshot snapshot={displayRow.collection_snapshot_id} onDenied={() => denied.current()} close={() => { setMembers(false); membersTrigger.current?.focus(); }} />}</>}</section>;
}

function ResourceQuickLook({ resource, close, onDenied }: { resource: Resource; close: () => void; onDenied: () => void }) {
  const [file, setFile] = useState<DriveFile | null>(null);
  const [status, setStatus] = useState<'loading' | 'ready' | 'error' | 'offline'>('loading');
  const denied = useRef(onDenied);
  denied.current = onDenied;
  useEffect(() => {
    if (!resource.file_ref) return;
    const controller = new AbortController();
    setFile(null); setStatus('loading');
    const invalidate = () => { controller.abort(); setFile(null); setStatus(navigator.onLine ? 'error' : 'offline'); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('offline', invalidate);
    if (!navigator.onLine) invalidate();
    else void api<{ file: DriveFile; ref: string }>(`/references?ref=${encodeURIComponent(resource.file_ref)}`, 'GET', undefined, {}, controller.signal).then(value => {
      if (controller.signal.aborted) return;
      if (!value || value.ref !== resource.file_ref || !value.file || !value.file.id || value.file.space_id !== resource.space_id || value.file.trashed) throw new Error('fixed_reference_mismatch');
      setFile(value.file); setStatus('ready');
    }).catch((error: { status?: number }) => { if (controller.signal.aborted) return; setFile(null); setStatus('error'); if (error.status === 401 || error.status === 403) denied.current(); });
    return () => { controller.abort(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', invalidate); };
  }, [resource]);
  if (status !== 'ready' || !file) return <AppSheet title="固定版本预览" open onOpenChange={open => { if (!open) close(); }}><div className="resource-preview-loading" role={status === 'error' || status === 'offline' ? 'alert' : 'status'}><div>{status === 'loading' ? '重新鉴权并核对固定版本…' : status === 'offline' ? '已离线，固定版本预览已清除。' : '固定版本不可用或引用已变化，未改读最新版本。'}<div className="mt-3"><Button variant="outline" className="min-h-11" onClick={close}>关闭预览</Button></div></div></div></AppSheet>;
  return <FilePreviewDialog title={resource.display_name} close={close} information={<section className="space-y-3"><h3 className="text-sm font-semibold">固定资源信息</h3><dl className="quicklook-metadata"><dt>类型</dt><dd>{kinds[resource.kind]}</dd><dt>修订</dt><dd>{resource.descriptor_revision}</dd><dt>文件</dt><dd title={file.path}>{file.path}</dd><dt>大小</dt><dd>{bytes(file.size_bytes)}</dd><dt>版本</dt><dd>{file.version_id}</dd></dl><p className="text-xs text-muted-foreground">此预览绑定资源登记时的固定版本，不会自动切换到最新文件。</p></section>}><MediaPreview file={file} /></FilePreviewDialog>;
}
