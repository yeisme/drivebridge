import { useEffect, useRef, useState } from 'react';
import { api, type Space } from '../../api';
import { Button } from '../../components/ui/button';
import { SelectAdapter } from '../../components/SelectAdapter';
import { WorkspacePageHeader } from '../../components/WorkspacePageHeader';
import { RegisterResourceDialog } from './RegisterResourceDialog';
import { ResourceSnapshot } from './ResourceSnapshot';
import { ResourceIntegrity } from './ResourceIntegrity';
import type { components } from '../../generated/personal-workspace';

type Resource = components['schemas']['ResourceEnvelope']['data'];
type Page = components['schemas']['ResourcePageEnvelope']['data'];
const kinds = { skill: 'Skill', template: '模板', reference_pack: '资料包' };
const text = (value: unknown): value is string => typeof value === 'string' && value.length > 0 && value.length <= 2048;
function validResource(row: Resource) {
  return row && text(row.descriptor_id) && text(row.space_id) && text(row.display_name) && Object.hasOwn(kinds, row.kind) && Number.isSafeInteger(row.descriptor_revision) && row.descriptor_revision > 0 && row.check_state === 'unknown' && typeof row.created_at === 'string' && Number.isFinite(Date.parse(row.created_at)) && ((text(row.file_ref) && row.collection_snapshot_id === undefined && row.kind !== 'reference_pack') || (text(row.collection_snapshot_id) && row.file_ref === undefined));
}
export function ResourcesPage({ scope, spaces }: { scope: string; spaces: Space[] }) {
  return <ResourceList key={scope} scope={scope} spaces={spaces}/>;
}
function ResourceList({ spaces, scope }: { spaces: Space[]; scope: string }) {
  const [registering, setRegistering] = useState(false);
  const [kind, setKind] = useState(''), [space, setSpace] = useState('');
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
    else void api<Page>('/resource-descriptors?' + query, 'GET', undefined, {}, controller.signal).then(page => {
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
  return <section aria-label="资源库" className="min-w-0 space-y-4">
    <WorkspacePageHeader title="资源库" description="登记 Skill、模板与资料包的固定版本。保存不等于检查通过、安装或执行。" actions={scope && state === 'ready' ? <Button ref={registerTrigger} className="min-h-11 text-[13px] md:min-h-8" onClick={() => setRegistering(true)}>登记固定资源</Button> : undefined}/>
    {registering && <RegisterResourceDialog scope={scope} spaces={spaces} close={() => { restoreRegistration.current = true; setRegistering(false); refresh(); }}/>}
    {state !== 'denied' && <div className="flex flex-wrap items-end gap-3 [&_label]:min-w-0 [&_label]:max-w-full md:[&_button]:min-h-8">
      <label className="grid gap-2 text-sm">资源类型<SelectAdapter aria-label="资源类型" className="min-h-11" value={kind} onChange={event => { if (event.target.value !== kind) { refresh(); setKind(event.target.value); } }}><option value="">全部类型</option>{Object.entries(kinds).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</SelectAdapter></label>
      <label className="grid gap-2 text-sm">资源空间<SelectAdapter aria-label="资源空间" className="min-h-11" value={space} onChange={event => { if (event.target.value !== space) { refresh(); setSpace(event.target.value); } }}><option value="">当前授权范围</option>{spaces.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</SelectAdapter></label>
      <Button ref={refreshTrigger} variant="outline" className="min-h-11 self-end" disabled={state === 'loading'} onClick={refresh}>刷新资源库</Button>
    </div>}
    {state === 'loading' && <p role="status">读取资源描述…</p>}
    {state === 'ready' && !items.length && <p role="status">当前范围与筛选下没有可见资源描述；原文件和集合不会自动变成已登记资源。</p>}
    {state === 'stale' && <p role="alert">资源分页已过期，旧结果已清除，请刷新重读。</p>}
    {state === 'error' && <p role="alert">资源描述不可用、超预算或合同不匹配，不代表没有资源。</p>}
    {state === 'unsupported' && <p role="status">当前服务尚未开放资源描述列表。</p>}
    {state === 'denied' && <p role="alert">权限或范围已变化，资源列表与详情已清除。</p>}
    {state === 'offline' && <p role="alert">已离线，资源已清除；联网后请手动刷新。</p>}
    <ul aria-label="资源描述列表" className="m-0 list-none divide-y divide-border rounded-lg border border-border bg-card p-0">{items.map(row => <li key={row.descriptor_id} className="flex min-w-0 items-center gap-3 px-3 py-2">
      <div className="min-w-0 flex-1"><h3 className="m-0 break-words text-sm font-semibold">{row.display_name}</h3><p className="m-0 text-xs text-muted-foreground">{kinds[row.kind]} · 未检查</p><p className="m-0 text-xs text-muted-foreground">修订 {row.descriptor_revision} · {row.collection_snapshot_id ? '集合快照（不是单文件）' : '固定文件版本'}</p></div>
      <Button variant="ghost" className="min-h-11 shrink-0 text-[13px] md:min-h-8" aria-label={`查看资源描述 ${row.display_name}`} disabled={state !== 'ready'} onClick={event => { trigger.current = event.currentTarget; setSelected(row); }}>查看</Button>
    </li>)}</ul>
    {state === 'ready' && next && <Button variant="outline" className="min-h-11" onClick={() => { setSelected(null); setCursor(next); }}>更多资源描述</Button>}
    {selected && <ResourceDetail key={selected.descriptor_id} original={selected} onDenied={deny} close={() => { setSelected(null); trigger.current?.focus(); }}/>} 
    <p className="text-xs text-muted-foreground">仅提供固定字节编码检查；完整格式检查、安装与领域交接尚未提供，本页不授予额外权限。列表按原标识分页，未提供全库数量。</p>
  </section>;
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
    else void api<Resource>('/resource-descriptors/' + encodeURIComponent(original.descriptor_id), 'GET', undefined, {}, controller.signal).then(value => {
      if (controller.signal.aborted) return;
      if (!validResource(value) || value.descriptor_id !== original.descriptor_id || value.space_id !== original.space_id || value.descriptor_revision !== original.descriptor_revision || value.kind !== original.kind || value.file_ref !== original.file_ref || value.collection_snapshot_id !== original.collection_snapshot_id) throw new Error('source_changed');
      setRow(value);
    }).catch((error: { status?: number }) => { if (controller.signal.aborted) return; setFailed(true); setRow(null); if (error.status === 401 || error.status === 403) denied.current(); });
    return () => { controller.abort(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', invalidate); };
  }, [original]);
  return <section aria-label="固定资源描述" className="space-y-3 rounded-lg border border-border bg-muted/20 p-4">
    <header className="flex flex-wrap items-center gap-3"><h3 ref={heading} tabIndex={-1} className="m-0 font-semibold">固定资源描述</h3><Button variant="outline" className="min-h-11" onClick={close}>关闭资源描述</Button></header>
    {failed ? <p role="alert">原资源描述不可用或已变化，未替换目标；请刷新列表后核对。</p> : !row ? <p role="status">重新鉴权并核对原描述…</p> : <><p className="break-all">{row.display_name}</p><p className="break-all text-xs">描述标识：{row.descriptor_id} · 修订 {row.descriptor_revision}</p><p className="break-all">{row.file_ref || `集合快照：${row.collection_snapshot_id}`}</p><p>检查状态：未检查，不代表内容安全、可执行或可安装。</p><Button ref={checkTrigger} variant="outline" className="min-h-11" aria-expanded={checking} onClick={() => setChecking(value => !value)}>查看编码检查</Button>{checking && <ResourceIntegrity descriptor={row.descriptor_id} revision={row.descriptor_revision} onDenied={() => denied.current()} close={() => { setChecking(false); checkTrigger.current?.focus(); }}/ >}{row.collection_snapshot_id && <><Button ref={membersTrigger} variant="outline" className="min-h-11" aria-expanded={members} onClick={() => setMembers(value => !value)}>{members ? '收起固定快照成员' : '查看固定快照成员'}</Button>{members && <ResourceSnapshot snapshot={row.collection_snapshot_id} onDenied={() => denied.current()} close={() => { setMembers(false); membersTrigger.current?.focus(); }}/>}</>}</>}
  </section>;
}
