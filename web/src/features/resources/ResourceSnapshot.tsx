import { useEffect, useRef, useState } from 'react';
import { api } from '../../api';
import { Button } from '../../components/ui/button';
import { SearchFileReference } from '../../consumer-flows/SearchFileReference';

type Member = { file_id: string; version_id: string; path: string };
type Snapshot = { snapshot_id: string; collection_id: string; collection_revision: number; entries_total: number; entries_visible: number; created_at: string; items: Member[] };
const id = (value: unknown): value is string => typeof value === 'string' && value.length > 0 && value.length <= 512;
export function ResourceSnapshot({ snapshot, close, onDenied, expectedCollection }: { snapshot: string; close: () => void; onDenied: () => void; expectedCollection?: string }) {
  const [data, setData] = useState<Snapshot | null>(null), [state, setState] = useState('loading'), [shown, setShown] = useState(50), [attempt, setAttempt] = useState(0);
  const [selected, setSelected] = useState<Member | null>(null);
  const trigger = useRef<HTMLButtonElement | null>(null), heading = useRef<HTMLHeadingElement>(null), denied = useRef(onDenied);
  denied.current = onDenied;
  useEffect(() => {
    const controller = new AbortController(); setData(null); setSelected(null); setShown(50); setState('loading'); heading.current?.focus();
    const invalidate = () => { controller.abort(); setData(null); setSelected(null); setState('denied'); };
    const offline = () => { invalidate(); setState('offline'); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('offline', offline);
    if (!navigator.onLine) offline();
    else void api<Snapshot>('/collection-snapshots/' + encodeURIComponent(snapshot), 'GET', undefined, {}, controller.signal).then(value => {
      if (controller.signal.aborted) return;
      if (!value || value.snapshot_id !== snapshot || (expectedCollection !== undefined && value.collection_id !== expectedCollection) || !id(value.collection_id) || !Number.isSafeInteger(value.collection_revision) || value.collection_revision < 1 || !Number.isSafeInteger(value.entries_total) || value.entries_total < 0 || value.entries_total > 30000 || !Number.isSafeInteger(value.entries_visible) || value.entries_visible < 0 || value.entries_visible > value.entries_total || !Array.isArray(value.items) || value.items.length !== value.entries_visible || !value.items.every(item => item && id(item.file_id) && id(item.version_id) && typeof item.path === 'string' && item.path.length <= 4096) || new Set(value.items.map(item => item.file_id)).size !== value.items.length || typeof value.created_at !== 'string' || !Number.isFinite(Date.parse(value.created_at))) throw new Error('invalid_contract');
      setData(value); setState('ready');
    }).catch((error: { status?: number }) => {
      if (controller.signal.aborted) return; setData(null); setSelected(null);
      if (error.status === 401 || error.status === 403) { setState('denied'); denied.current(); } else setState('error');
    });
    return () => { controller.abort(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', offline); };
  }, [snapshot, expectedCollection, attempt]);
  return <section aria-label="资料包固定成员" className="space-y-3 rounded-lg border border-border p-4">
    <header className="flex flex-wrap items-center gap-3"><h3 ref={heading} tabIndex={-1} className="m-0 font-semibold">资料包固定成员</h3><Button variant="outline" className="min-h-11" onClick={close}>关闭资料包成员</Button></header>
    <p className="text-sm text-muted-foreground">读取原快照，不执行内容或检查器。展开仅控制已读取条目的本地显示，不是服务端分页；成员阅读仍需重新鉴权。</p>
    {state === 'loading' && <p role="status">读取原快照成员…</p>}
    {state === 'error' && <p role="alert">原快照不可用、超预算或合同不匹配，未换用当前集合。</p>}
    {state === 'denied' && <p role="alert">权限或范围已变化，资料包成员已清除。</p>}
    {state === 'offline' && <p role="alert">已离线，资料包成员已清除；联网不会自动读取。</p>}
    <Button variant="outline" className="min-h-11" disabled={state === 'loading' || state === 'denied'} onClick={() => setAttempt(value => value + 1)}>重读原快照</Button>
    {data && <><p>历史条目：{data.entries_total} · 本次可见：{data.entries_visible}</p><p className="text-sm">{data.entries_visible < data.entries_total ? '部分成员不可见或固定版本不可用；不能认定资料包完整。' : '本次可见计数与历史一致；不代表内容安全或允许执行。'}</p>
      <ul className="m-0 list-none space-y-2 p-0">{data.items.slice(0, shown).map(item => <li key={item.file_id}><Button variant="outline" className="min-h-11 whitespace-normal break-all text-left" onClick={event => { trigger.current = event.currentTarget; setSelected(item); }}>打开固定成员：{item.path || item.file_id}</Button></li>)}</ul>
      {shown < data.items.length && <Button variant="outline" className="min-h-11" onClick={() => setShown(value => value + 50)}>展开更多快照成员</Button>}
    </>}
    {selected && <SearchFileReference key={selected.file_id + ':' + selected.version_id} file={selected.file_id} version={selected.version_id} sourceLabel="资料包成员" onDenied={() => { setData(null); setSelected(null); setState('denied'); denied.current(); }} close={() => { setSelected(null); trigger.current?.focus(); }}/>} 
  </section>;
}
