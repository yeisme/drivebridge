import { useEffect, useRef, useState } from 'react';
import { api, type Space } from '../../api';
import { confirmRouteLeave } from '../../app/route-leave';
import { Button } from '../../components/ui/button';
import { SelectAdapter } from '../../components/SelectAdapter';
import { WorkspacePageHeader } from '../../components/WorkspacePageHeader';
import { HomeCollection } from '../home/HomeCollection';
import { CreateCollectionDialog } from './CreateCollectionDialog';
import { CollectionMembersDialog } from './CollectionMembersDialog';
import { CreateSnapshotDialog } from './CreateSnapshotDialog';
import { SnapshotHistory } from './SnapshotHistory';
import { FixedReference } from '../home/WorkspaceHome';
import type { components } from '../../generated/personal-workspace';

type Home = components['schemas']['WorkspaceHomeEnvelope']['data'];
type Collection = Home['collections'][number] & { space_id: string; scope: string };
export function CollectionsPage({ scope, spaces }: { scope: string; spaces: Space[] }) {
  return <ScopedCollections key={scope} scope={scope} spaces={spaces}/>;
}
function ScopedCollections({ scope, spaces }: { scope: string; spaces: Space[] }) {
  const [chosen, setChosen] = useState('');
  const space = spaces.some(item => item.id === chosen) ? chosen : spaces[0]?.id || '';
  return <section aria-label="项目集合" className="min-w-0 space-y-4">
    <WorkspacePageHeader title="项目集合" description="引用原文件组织项目，不移动或复制原目录。当前成员与固定快照分别查看。"/>
    {!space ? <p role="status">当前范围没有可用空间。</p> : <><label className="grid max-w-sm gap-2 text-[13px] text-muted-foreground">集合空间<SelectAdapter aria-label="集合空间" className="min-h-11 md:min-h-8" value={space} onChange={event => { const next = event.target.value; if (next !== space && confirmRouteLeave()) setChosen(next); }}>{spaces.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</SelectAdapter></label><CollectionList key={space} scope={scope} space={space} spaces={spaces} spaceName={spaces.find(item => item.id === space)!.name}/></>}
  </section>;
}
function CollectionList({ scope, space, spaceName, spaces }: { scope: string; space: string; spaceName: string; spaces: Space[] }) {
  const [creating, setCreating] = useState<'manual' | 'rule' | null>(null), createTrigger = useRef<HTMLButtonElement | null>(null);
  const [editing, setEditing] = useState<string | null>(null), editTrigger = useRef<HTMLButtonElement | null>(null);
  const [history, setHistory] = useState<string | null>(null), historyTrigger = useRef<HTMLButtonElement | null>(null);
  const [snapshotting, setSnapshotting] = useState<string | null>(null), snapshotTrigger = useRef<HTMLButtonElement | null>(null);
  const [rows, setRows] = useState<Collection[]>([]), [state, setState] = useState('loading'), [attempt, setAttempt] = useState(0), [shown, setShown] = useState(50);
  const [collection, setCollection] = useState<Collection | null>(null), [file, setFile] = useState<Home['recent_files'][number] | null>(null);
  const active = useRef<AbortController | null>(null), collectionTrigger = useRef<HTMLButtonElement | null>(null), fileTrigger = useRef<HTMLButtonElement | null>(null);
  const clear = () => { setRows([]); setCollection(null); setFile(null); setCreating(null); setEditing(null); setSnapshotting(null); setHistory(null); };
  const deny = () => { active.current?.abort(); clear(); setState('denied'); };
  useEffect(() => {
    const controller = new AbortController(); active.current = controller; clear(); setShown(50); setState('loading');
    const invalidate = () => { controller.abort(); clear(); setState('denied'); };
    const offline = () => { invalidate(); setState('offline'); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('offline', offline);
    if (!navigator.onLine) offline();
    else void api<Collection[]>('/file-collections?' + new URLSearchParams({ space_id: space }), 'GET', undefined, {}, controller.signal).then(value => {
      if (controller.signal.aborted) return;
      if (!Array.isArray(value) || value.length > 1000 || !value.every(row => row && typeof row.collection_id === 'string' && row.collection_id.length > 0 && row.collection_id.length <= 512 && row.space_id === space && typeof row.name === 'string' && row.name.length > 0 && row.name.length <= 200 && ['manual', 'rule'].includes(row.kind) && ['private', 'team'].includes(row.scope) && Number.isSafeInteger(row.revision) && row.revision > 0) || new Set(value.map(row => row.collection_id)).size !== value.length) throw new Error('invalid_contract');
      setRows(value); setState('ready');
    }).catch((error: { status?: number }) => { if (controller.signal.aborted) return; clear(); setState(error.status === 401 || error.status === 403 ? 'denied' : error.status === 404 || error.status === 405 ? 'unsupported' : 'error'); });
    return () => { controller.abort(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', offline); };
  }, [space, attempt]);
  return <>
    <div className="flex flex-wrap items-center gap-2 md:[&_button]:min-h-8">
    <Button variant="outline" className="min-h-11 text-[13px] md:min-h-8" disabled={state === 'loading' || state === 'denied'} onClick={() => { if (confirmRouteLeave()) setAttempt(value => value + 1); }}>刷新集合列表</Button>
    <Button className="min-h-11" disabled={state !== 'ready'} onClick={event => { createTrigger.current = event.currentTarget; setCreating('manual'); }}>新建私有手动集合</Button>
    <Button className="min-h-11" variant="outline" disabled={state !== 'ready'} onClick={event => { createTrigger.current = event.currentTarget; setCreating('rule'); }}>新建私有规则集合</Button>
    </div>
    {creating && <CreateCollectionDialog kind={creating} scope={JSON.stringify([scope, space])} space={{ id: space, name: spaceName }} denied={deny} close={() => { setCreating(null); createTrigger.current?.focus(); }}/ >}
    {editing && <CollectionMembersDialog key={editing} scope={JSON.stringify([scope, space])} id={editing} space={space} spaces={spaces} denied={deny} close={() => { setEditing(null); editTrigger.current?.focus(); }}/ >}
    {history && <SnapshotHistory key={history} id={history} denied={deny} close={() => { setHistory(null); historyTrigger.current?.focus(); }}/ >}
    {snapshotting && <CreateSnapshotDialog key={snapshotting} scope={JSON.stringify([scope, space])} id={snapshotting} denied={deny} close={() => { setSnapshotting(null); snapshotTrigger.current?.focus(); }}/ >}
    {state === 'loading' && <p role="status">读取授权集合…</p>}
    {state === 'ready' && !rows.length && <p role="status">当前授权空间没有集合。</p>}
    {state === 'error' && <p role="alert">集合列表不可用、超预算或合同不匹配，不代表空列表。</p>}
    {state === 'unsupported' && <p role="alert">当前服务未提供此集合列表或目标不可见。</p>}
    {state === 'denied' && <p role="alert">权限或范围变化，集合及正文已清除，请重新进入。</p>}
    {state === 'offline' && <p role="alert">当前离线，集合及正文已清除；联网后请手动刷新。</p>}
    <ul aria-label="集合索引" className="m-0 list-none divide-y divide-border rounded-lg border border-border bg-card p-0">{rows.slice(0, shown).map(row => <li key={row.collection_id} className="flex min-w-0 flex-wrap items-center justify-between gap-2 px-3 py-2">
      <div className="min-w-0 flex-[1_1_16rem]"><h3 className="m-0 break-words text-sm font-semibold">{row.name}</h3><p className="m-0 text-xs text-muted-foreground">{row.kind === 'manual' ? '手动集合' : '规则集合'} · {row.scope === 'team' ? '团队范围' : '私有范围'} · 修订 {row.revision}</p></div>
      <div className="grid grid-cols-2 gap-1 max-md:w-full md:flex md:flex-wrap md:[&_button]:min-h-8">
        <Button variant="ghost" className="min-h-11 text-[13px]" aria-label={`查看集合 ${row.name}`} onClick={event => { collectionTrigger.current = event.currentTarget; setFile(null); setCollection(row); }}>查看成员</Button>
        <Button variant="ghost" className="min-h-11 text-[13px]" aria-label={`编辑成员 ${row.name}`} onClick={event => { editTrigger.current = event.currentTarget; setCollection(null); setFile(null); setEditing(row.collection_id); }}>编辑成员</Button>
        <Button variant="ghost" className="min-h-11 text-[13px]" aria-label={`创建快照 ${row.name}`} onClick={event => { snapshotTrigger.current = event.currentTarget; setCollection(null); setFile(null); setSnapshotting(row.collection_id); }}>创建快照</Button>
        <Button variant="ghost" className="min-h-11 text-[13px]" aria-label={`历史快照 ${row.name}`} onClick={event => { historyTrigger.current = event.currentTarget; setCollection(null); setFile(null); setHistory(row.collection_id); }}>历史快照</Button>
      </div>
    </li>)}</ul>
    {shown < rows.length && <Button variant="outline" className="min-h-11" onClick={() => setShown(value => value + 50)}>展开更多已读取集合</Button>}
    <details className="text-xs text-muted-foreground"><summary className="flex min-h-11 cursor-pointer items-center py-2 focus-visible:outline-2 focus-visible:outline-ring md:min-h-8">集合与快照说明</summary><p className="m-0 pb-3 leading-5">列表最多读取 1000 个集合，按创建时间与标识排序；本地分段展示不是服务端分页。不把 include 决策计数当作可见成员总数。支持新建私有手动或规则集合；可编辑完整显式成员决策；支持创建与恢复原快照；可观察历史快照并按固定版本阅读；历史索引不是原创建回执，不提供删除操作。</p></details>
    {collection && <HomeCollection key={collection.collection_id} sourceLabel="集合页" collection={collection} onDenied={deny} onOpen={(value, trigger) => { fileTrigger.current = trigger; setFile(value); }} close={() => { setCollection(null); setFile(null); collectionTrigger.current?.focus(); }}/ >}
    {file && <FixedReference key={file.file_id + ':' + file.version_id} sourceLabel="集合页" file={file} denied={deny} close={() => { setFile(null); fileTrigger.current?.focus(); }}/ >}
  </>;
}
