import { useEffect, useRef, useState } from 'react';
import { api, type Space } from '../../api';
import { confirmRouteLeave } from '../../app/route-leave';
import { Button } from '../../components/ui/button';
import { SelectAdapter } from '../../components/SelectAdapter';
import { WorkspacePageHeader } from '../../components/WorkspacePageHeader';
import { PropertyEditor } from './PropertyEditor';
import { typeLabel, validCatalog, type Catalog } from './property-recovery';
import { PropertyMigration } from './PropertyMigration';
import { LabelDictionary } from './LabelDictionary';

export function PropertiesPage({ scope, spaces }: { scope: string; spaces: Space[] }) { return <ScopedProperties key={scope} scope={scope} spaces={spaces}/>; }
function ScopedProperties({ scope, spaces }: { scope: string; spaces: Space[] }) {
  const [chosen, setChosen] = useState('');
  const space = spaces.some(row => row.id === chosen) ? chosen : spaces[0]?.id || '';
  return <section aria-label="标签与属性" className="min-w-0 space-y-4"><WorkspacePageHeader title="标签与属性" description="管理空间的属性定义，稳定 key 与显示名称分开。定义不等于文件属性值，使用次数尚未提供，不显示为零。"/>
    {!space ? <p role="status">当前范围没有可用空间。</p> : <><label className="grid max-w-sm gap-2 text-[13px] text-muted-foreground">属性空间<SelectAdapter aria-label="属性空间" className="min-h-11 md:min-h-8" value={space} onChange={event => { const next = event.target.value; if (next !== space && confirmRouteLeave()) setChosen(next); }}>{spaces.map(row => <option key={row.id} value={row.id}>{row.name}</option>)}</SelectAdapter></label><CatalogList key={space} scope={scope} space={space}/></>}
    {space && <PropertyMigration key={space} scope={scope} space={space}/>}
    {space && <LabelDictionary key={'labels:' + space} scope={scope} space={space}/>}
    <p className="text-sm text-muted-foreground">字段 key／类型变更使用独立迁移计划，不直接编辑已有定义类型。仅支持有界 sidecar 迁移，不批量改写 Markdown／YAML。manual、rule、suggested 标签的来源与授权语义保持不变。</p>
  </section>;
}
function CatalogList({ scope, space }: { scope: string; space: string }) {
  const [data, setData] = useState<Catalog | null>(null), [state, setState] = useState('loading'), [attempt, setAttempt] = useState(0), [shown, setShown] = useState(50), [editing, setEditing] = useState(false);
  const active = useRef<AbortController | null>(null), trigger = useRef<HTMLButtonElement | null>(null);
  const deny = () => { active.current?.abort(); setData(null); setEditing(false); setState('denied'); };
  useEffect(() => {
    const abort = new AbortController(); active.current = abort; setData(null); setShown(50); setEditing(false); setState('loading');
    const invalidate = () => { abort.abort(); setData(null); setEditing(false); setState('denied'); };
    const offline = () => { invalidate(); setState('offline'); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('offline', offline);
    if (!navigator.onLine) offline();
    else void api<Catalog>('/spaces/' + encodeURIComponent(space) + '/property-definitions', 'GET', undefined, {}, abort.signal).then(value => {
      if (abort.signal.aborted) return;
      if (!validCatalog(value, space)) throw new Error('invalid_catalog');
      setData(value); setState('ready');
    }).catch((error: { status?: number }) => { if (!abort.signal.aborted) { setData(null); setState(error.status === 401 || error.status === 403 ? 'denied' : error.status === 404 || error.status === 405 ? 'unsupported' : 'error'); } });
    return () => { abort.abort(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', offline); };
  }, [space, attempt]);
  return <>
    <div className="flex flex-wrap gap-2 md:[&_button]:min-h-8"><Button className="min-h-11" variant="outline" disabled={state === 'loading' || state === 'denied'} onClick={() => { if (confirmRouteLeave()) setAttempt(value => value + 1); }}>刷新属性字典</Button><Button ref={trigger} className="min-h-11" disabled={state !== 'ready'} onClick={() => setEditing(true)}>编辑属性字典</Button></div>
    {state === 'loading' && <p role="status">读取授权属性字典…</p>}{state === 'error' && <p role="alert">字典不可用、需迁移或合同不匹配，不代表没有定义。</p>}{state === 'unsupported' && <p role="alert">空间不可见或当前服务尚未提供属性字典。</p>}{state === 'denied' && <p role="alert">权限或范围变化，字典已清除，请重新进入。</p>}{state === 'offline' && <p role="alert">当前离线，字典已清除；联网后请手动刷新。</p>}
    {data && <><p>字典修订 {data.revision} · {data.definitions.length} 个定义（最多 200 个）</p>{!data.definitions.length && <p role="status">当前空间尚无属性定义。</p>}<ul aria-label="属性定义" className="m-0 list-none divide-y divide-border rounded-lg border border-border bg-card p-0">{data.definitions.slice(0, shown).map(row => <li key={row.key} className="px-3 py-2"><h3 className="m-0 break-all text-sm font-semibold">{row.display_name}</h3><p className="m-0 break-all text-xs text-muted-foreground">{row.key} · {typeLabel[row.type]} · {row.group || '未分组'}</p></li>)}</ul>{shown < data.definitions.length && <Button className="min-h-11" onClick={() => setShown(value => value + 50)}>展开更多已读取定义</Button>}<p className="text-xs text-muted-foreground">按稳定 key 排序；每次展开 50 个仅是本地展示，不是服务端分页。</p></>}
    {editing && <PropertyEditor scope={scope} space={space} denied={deny} close={() => { setEditing(false); trigger.current?.focus(); }}/ >}
  </>;
}
