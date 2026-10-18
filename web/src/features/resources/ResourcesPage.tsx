import { EmptyState } from "../../components/EmptyState";
import { useEffect, useRef, useState, type ReactNode } from 'react';
import { api, type Space } from '../../api';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { SelectAdapter } from '../../components/SelectAdapter';
import { WorkspacePageHeader } from '../../components/WorkspacePageHeader';
import { AppSheet } from '../../components/AppSheet';
import { BookOpen, FileCheck2, Package2, Search, Shapes } from 'lucide-react';
import { RegisterResourceDialog, ResourceSnapshot, ResourceIntegrity } from './components';
import { getResourceDescriptor, listResourceDescriptors } from './clients/resource-descriptors';
import { isResourceDescriptor, resourceKinds } from './contract';
import { projectResourceList } from './projection';
import type { ResourceDescriptor as Resource, ResourceDescriptorPage as Page } from './contract';
const kinds = resourceKinds;
const validResource = isResourceDescriptor;
export function ResourcesPage({ scope, spaces }: { scope: string; spaces: Space[] }) {
  return <ResourceList key={scope} scope={scope} spaces={spaces}/>;
}
function ResourceList({ spaces, scope }: { spaces: Space[]; scope: string }) {
  const [registering, setRegistering] = useState(false);
  const [kind, setKind] = useState(''), [space, setSpace] = useState('');
  const [search, setSearch] = useState('');
  const [items, setItems] = useState<Resource[]>([]), [cursor, setCursor] = useState(''), [next, setNext] = useState(''), [attempt, setAttempt] = useState(0);
  const [selected, setSelected] = useState<Resource | null>(null), [state, setState] = useState('loading');
  const active = useRef<AbortController | null>(null), trigger = useRef<HTMLButtonElement | null>(null);
  const registerTrigger = useRef<HTMLButtonElement | null>(null), refreshTrigger = useRef<HTMLButtonElement | null>(null);
  const restoreRegistration = useRef(false);
  useEffect(() => {
    if (!restoreRegistration.current || state === 'loading' || registering) return;
    restoreRegistration.current = false;
    if (state !== 'denied' && state !== 'offline' && document.activeElement === document.body) (registerTrigger.current || refreshTrigger.current)?.focus();
  }, [state, registering]);
  const deny = () => { restoreRegistration.current = false; active.current?.abort(); setSelected(null); setItems([]); setNext(''); setState('denied'); };
  useEffect(() => {
    const controller = new AbortController(); active.current = controller; setState('loading');
    const invalidate = () => { restoreRegistration.current = false; controller.abort(); setSelected(null); setItems([]); setNext(''); setState('denied'); };
    const offline = () => { invalidate(); setState('offline'); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('offline', offline);
    const query = new URLSearchParams({ limit: '50' });
    if (kind) query.set('kind', kind); if (space) query.set('space_id', space); if (cursor) query.set('cursor', cursor);
    if (!navigator.onLine) offline();
    else void listResourceDescriptors(query, controller.signal).then(page => {
      if (controller.signal.aborted) return;
      if (!page || page.total !== null || page.total_kind !== 'unknown' || !Array.isArray(page.items) || page.items.length > 50 || typeof page.next_cursor !== 'string' || page.next_cursor.length > 16384 || (cursor && page.next_cursor === cursor) || !page.items.every(row => validResource(row) && (!kind || row.kind === kind) && (!space || row.space_id === space)) || new Set(page.items.map(row => row.descriptor_id)).size !== page.items.length) throw new Error('invalid_contract');
      setItems(old => cursor ? [...new Map([...old, ...page.items].map(row => [row.descriptor_id, row])).values()] : page.items);
      setNext(page.next_cursor); setState('ready');
    }).catch((error: { status?: number }) => {
      if (controller.signal.aborted) return;
      setItems([]); setSelected(null); setNext('');
      setState(error.status === 401 || error.status === 403 ? 'denied' : error.status === 410 ? 'stale' : error.status === 404 || error.status === 405 ? 'unsupported' : 'error');
    });
    return () => { controller.abort(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', offline); };
  }, [kind, space, cursor, attempt]);
  const refresh = () => { active.current?.abort(); setState('loading'); setItems([]); setSelected(null); setCursor(''); setNext(''); setAttempt(value => value + 1); };
  const filteredItems = projectResourceList(items, search).filteredItems;
  const kindIcon = (value: Resource['kind']) => value === 'skill' ? <Shapes className="size-4"/> : value === 'template' ? <BookOpen className="size-4"/> : <Package2 className="size-4"/>;
  return <section aria-label="资源库" className="min-w-0 space-y-4">
    <WorkspacePageHeader title="资源库" description="登记 Skill、模板与资料包的固定版本。保存不等于检查通过、安装或执行。" actions={<div className="flex items-center gap-2"><span className="hidden rounded-full bg-primary/10 px-2 py-1 text-[11px] font-medium text-primary md:inline">{state === 'ready' ? `${items.length} 个已加载资源` : '按当前范围读取'}</span>{scope && state === 'ready' ? <Button ref={registerTrigger} className="min-h-11 gap-1.5 text-[13px] md:min-h-8" onClick={() => setRegistering(true)}><FileCheck2/>登记固定资源</Button> : undefined}</div>}/>
    {registering && <RegisterResourceDialog scope={scope} spaces={spaces} close={() => { restoreRegistration.current = true; setRegistering(false); refresh(); }}/>}
    {state !== 'denied' && <div className="flex flex-wrap items-end gap-3 [&_label]:min-w-0 [&_label]:max-w-full md:[&_button]:min-h-8">
      <label className="grid min-w-56 flex-1 gap-2 text-sm"><span>筛选已加载资源</span><span className="relative"><Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"/><Input aria-label="筛选已加载资源" className="min-h-11 pl-9" value={search} onChange={event => setSearch(event.target.value)} placeholder="按名称筛选当前页…"/></span></label>
      <label className="grid gap-2 text-sm">资源类型<SelectAdapter aria-label="资源类型" className="min-h-11" value={kind} onChange={event => { if (event.target.value !== kind) { refresh(); setKind(event.target.value); } }}><option value="">全部类型</option>{Object.entries(kinds).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</SelectAdapter></label>
      <label className="grid gap-2 text-sm">资源空间<SelectAdapter aria-label="资源空间" className="min-h-11" value={space} onChange={event => { if (event.target.value !== space) { refresh(); setSpace(event.target.value); } }}><option value="">当前授权范围</option>{spaces.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</SelectAdapter></label>
      <Button ref={refreshTrigger} variant="outline" className="min-h-11 self-end" disabled={state === 'loading'} onClick={refresh}>刷新资源库</Button>
    </div>}
    {state === 'loading' && <ResourceSkeleton/>}
    {state === 'ready' && !filteredItems.length && <div role="status"><EmptyState title={items.length ? '当前页没有匹配资源' : '还没有登记资源'} description={items.length ? '搜索只作用于当前已加载页；继续翻页或清除筛选后再查看。' : '登记固定版本的 Skill、模板或资料包，资源会保留来源和修订。'} action={scope ? <Button className="min-h-11 gap-1.5" onClick={() => setRegistering(true)}><FileCheck2/>登记固定资源</Button> : undefined}/></div>}
    {state === 'stale' && <ResourceNotice tone="warning" title="资源分页已过期" detail="旧结果已清除，请刷新重读。" action={<Button variant="outline" className="min-h-11" onClick={refresh}>重新读取资源</Button>}/>}
    {state === 'error' && <ResourceNotice tone="danger" title="资源描述暂不可用" detail="合同不匹配或服务错误不代表没有资源。" action={<Button variant="outline" className="min-h-11" onClick={refresh}>重试读取</Button>}/>}
    {state === 'unsupported' && <ResourceNotice title="当前服务尚未开放资源描述列表" detail="资源库会保留明确的 unsupported 状态，不使用模拟数据填充。"/>}
    {state === 'denied' && <ResourceNotice tone="danger" title="权限或范围已变化" detail="资源列表与详情已清除，请重新登录或切换范围。"/>}
    {state === 'offline' && <ResourceNotice tone="warning" title="当前离线" detail="资源已清除；联网后请手动刷新。" action={<Button variant="outline" className="min-h-11" onClick={refresh}>重新读取</Button>}/>}
    {!!filteredItems.length && <ul aria-label="资源描述列表" className="m-0 list-none divide-y divide-border overflow-hidden rounded-xl border border-border bg-card p-0">{filteredItems.map(row => <li key={row.descriptor_id} className="group flex min-w-0 items-center gap-3 px-3 py-3 transition-colors hover:bg-accent/40 md:px-4">
      <div className="grid size-9 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary">{kindIcon(row.kind)}</div>
      <div className="min-w-0 flex-1"><div className="flex min-w-0 flex-wrap items-center gap-2"><h3 className="m-0 min-w-0 break-words text-sm font-semibold">{row.display_name}</h3><span className="rounded-full bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">{kinds[row.kind]}</span><span className="rounded-full border border-border px-2 py-0.5 text-[11px] text-muted-foreground">未检查</span></div><p className="mt-1 text-xs text-muted-foreground">修订 {row.descriptor_revision} · {row.collection_snapshot_id ? '集合快照' : '固定文件版本'} · {row.space_id}</p></div>
      <Button variant="outline" className="min-h-11 shrink-0 text-[13px] md:min-h-8" aria-label={`查看资源描述 ${row.display_name}`} disabled={state !== 'ready'} onClick={event => { trigger.current = event.currentTarget; setSelected(row); }}>查看详情</Button>
    </li>)}</ul>}
    {state === 'ready' && next && <Button variant="outline" aria-label="更多资源描述" className="min-h-11" onClick={() => { setSelected(null); setCursor(next); }}>加载更多资源</Button>}
    {selected && <AppSheet title="固定资源描述" open onOpenChange={open => { if (!open) { setSelected(null); trigger.current?.focus(); } }} detailWidth={420} initialDetailWidth={420}><ResourceDetail key={selected.descriptor_id} original={selected} onDenied={deny} close={() => { setSelected(null); trigger.current?.focus(); }}/></AppSheet>}
    <p className="flex items-center gap-1.5 text-xs text-muted-foreground"><FileCheck2 className="size-3.5"/>仅提供固定字节编码检查；完整格式检查、安装与领域交接尚未提供，本页不授予额外权限。列表按原标识分页，未提供全库数量。</p>
  </section>;
}

function ResourceSkeleton() {
  return <div role="status" aria-label="读取资源描述" className="space-y-2 rounded-xl border border-border bg-card p-4"><span className="sr-only">读取资源描述…</span>{Array.from({ length: 4 }, (_, index) => <div key={index} className="flex items-center gap-3 py-2"><div className="size-9 animate-pulse rounded-lg bg-muted"/><div className="min-w-0 flex-1 space-y-2"><div className="h-3 w-2/5 animate-pulse rounded bg-muted"/><div className="h-2.5 w-3/5 animate-pulse rounded bg-muted"/></div><div className="h-8 w-20 animate-pulse rounded bg-muted"/></div>)}</div>;
}

function ResourceNotice({ title, detail, action, tone = 'neutral' }: { title: string; detail: string; action?: ReactNode; tone?: 'neutral' | 'warning' | 'danger' }) {
  return <section role={tone === 'danger' ? 'alert' : 'status'} className={`rounded-xl border p-4 ${tone === 'danger' ? 'border-destructive/30 bg-destructive/5' : tone === 'warning' ? 'border-amber-500/30 bg-amber-500/5' : 'border-border bg-card'}`}><div className="flex flex-wrap items-start justify-between gap-3"><div><h3 className="m-0 text-sm font-semibold">{title}</h3><p className="mt-1 text-sm leading-5 text-muted-foreground">{detail}</p></div>{action}</div></section>;
}
function ResourceDetail({ original, close, onDenied }: { original: Resource; close: () => void; onDenied: () => void }) {
  const [row, setRow] = useState<Resource | null>(null), [failed, setFailed] = useState(false);
  const [checking, setChecking] = useState(false); const checkTrigger = useRef<HTMLButtonElement | null>(null);
  const [members, setMembers] = useState(false); const membersTrigger = useRef<HTMLButtonElement | null>(null);
  const heading = useRef<HTMLHeadingElement>(null), denied = useRef(onDenied); denied.current = onDenied;
  useEffect(() => {
    const controller = new AbortController(); heading.current?.focus();
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
  return <section aria-label="固定资源描述" className="space-y-4 bg-background">
    <header className="flex flex-wrap items-center gap-3"><h3 ref={heading} tabIndex={-1} className="m-0 font-semibold">固定资源描述</h3><Button variant="outline" className="min-h-11" onClick={close}>关闭资源描述</Button></header>
    {failed ? <p role="alert">原资源描述不可用或已变化，未替换目标；请刷新列表后核对。</p> : !row ? <p role="status">重新鉴权并核对原描述…</p> : <><p className="break-all">{row.display_name}</p><p className="break-all text-xs">描述标识：{row.descriptor_id} · 修订 {row.descriptor_revision}</p><p className="break-all">{row.file_ref || `集合快照：${row.collection_snapshot_id}`}</p><p>检查状态：未检查，不代表内容安全、可执行或可安装。</p><Button ref={checkTrigger} variant="outline" className="min-h-11" aria-expanded={checking} onClick={() => setChecking(value => !value)}>查看编码检查</Button>{checking && <ResourceIntegrity descriptor={row.descriptor_id} revision={row.descriptor_revision} onDenied={() => denied.current()} close={() => { setChecking(false); checkTrigger.current?.focus(); }}/ >}{row.collection_snapshot_id && <><Button ref={membersTrigger} variant="outline" className="min-h-11" aria-expanded={members} onClick={() => setMembers(value => !value)}>{members ? '收起固定快照成员' : '查看固定快照成员'}</Button>{members && <ResourceSnapshot snapshot={row.collection_snapshot_id} onDenied={() => denied.current()} close={() => { setMembers(false); membersTrigger.current?.focus(); }}/>}</>}</>}
  </section>;
}
