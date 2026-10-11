import { useEffect, useRef, useState } from 'react';
import { api } from '../api';
import { Button } from '../components/ui/button';

type Saved = { query_id: string; name: string; query: Record<string, unknown>; created_at: string; updated_at: string };
export type SavedPath = { text: string; spaces: string[] };
const object = (value: unknown): value is Record<string, unknown> => !!value && typeof value === 'object' && !Array.isArray(value);
const id = (value: unknown): value is string => typeof value === 'string' && value.length > 0 && value.length <= 512;
// Only accept conditions this editor can preserve in full. Unknown options
// are not dropped to make an unsupported saved query appear usable.
export function savedPath(query: Record<string, unknown>): SavedPath | null {
  if (Object.keys(query).some(key => !['mode', 'space_ids', 'filters', 'sort', 'limit', 'cursor'].includes(key)) || query.mode !== 'exact' || !Array.isArray(query.space_ids) || !query.space_ids.length || query.space_ids.length > 100 || !query.space_ids.every(id) || new Set(query.space_ids).size !== query.space_ids.length || (query.sort !== undefined && (!Array.isArray(query.sort) || query.sort.length !== 0)) || (query.cursor !== undefined && query.cursor !== '') || (query.limit !== undefined && query.limit !== 0 && query.limit !== 50) || !Array.isArray(query.filters) || query.filters.length !== 1) return null;
  const filter = query.filters[0];
  if (!object(filter) || Object.keys(filter).some(key => !['field', 'op', 'value'].includes(key)) || filter.field !== 'path_prefix' || filter.op !== 'contains' || typeof filter.value !== 'string' || !filter.value.trim() || filter.value.includes('\0') || new TextEncoder().encode(filter.value).length > 4096) return null;
  return { text: filter.value, spaces: [...query.space_ids] };
}
export function SavedSearches({ onSelect, onDenied }: { onSelect: (query: SavedPath) => void; onDenied: () => void }) {
  const [rows, setRows] = useState<Saved[]>([]), [state, setState] = useState('loading'), [attempt, setAttempt] = useState(0);
  const denied = useRef(onDenied); denied.current = onDenied;
  useEffect(() => {
    const controller = new AbortController(); setRows([]); setState('loading');
    const invalidate = () => { controller.abort(); setRows([]); setState('denied'); };
    const offline = () => { invalidate(); setState('offline'); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('offline', offline);
    if (!navigator.onLine) offline();
    else void api<unknown>('/saved-searches', 'GET', undefined, {}, controller.signal).then(value => {
      if (controller.signal.aborted) return;
      if (!Array.isArray(value) || value.length > 1000 || !value.every(row => object(row) && id(row.query_id) && typeof row.name === 'string' && row.name.length > 0 && new TextEncoder().encode(row.name).length <= 200 && object(row.query) && JSON.stringify(row.query).length <= 65536 && typeof row.created_at === 'string' && Number.isFinite(Date.parse(row.created_at)) && typeof row.updated_at === 'string' && Number.isFinite(Date.parse(row.updated_at))) || new Set(value.map(row => row.query_id)).size !== value.length) throw new Error('invalid_contract');
      setRows(value as Saved[]); setState('ready');
    }).catch((error: { status?: number }) => {
      if (controller.signal.aborted) return;
      setRows([]);
      if (error.status === 401 || error.status === 403) { setState('denied'); denied.current(); }
      else setState(error.status === 404 || error.status === 405 ? 'unsupported' : 'error');
    });
    return () => { controller.abort(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', offline); };
  }, [attempt]);
  return <section aria-label="已保存检索条件" className="my-4 space-y-3 rounded-lg border border-border p-4">
    <h3 className="m-0 font-semibold">已保存条件</h3>
    <p className="text-sm text-muted-foreground">只核对当前可见记录。相同名称或条件不是原保存请求的回执；不要据此自动重存。选用仅填入条件，需点击检索重新鉴权。</p>
    {state === 'loading' && <p role="status">读取已保存条件…</p>}
    {state === 'ready' && !rows.length && <p role="status">当前范围没有可见保存条件，不证明先前请求未提交。</p>}
    {state === 'error' && <p role="alert">保存条件不可用、超预算或合同不匹配，不代表没有记录。</p>}
    {state === 'unsupported' && <p role="status">当前服务尚不支持读取已保存条件。</p>}
    {state === 'denied' && <p role="alert">权限或范围已变化，保存条件已清除。</p>}
    {state === 'offline' && <p role="alert">已离线，保存条件已清除；联网后请手动重读。</p>}
    <Button variant="outline" className="min-h-11" disabled={state === 'loading' || state === 'denied'} onClick={() => setAttempt(value => value + 1)}>重读已保存条件</Button>
    <ul className="m-0 list-none space-y-3 p-0">{rows.map(row => {
      const reusable = savedPath(row.query);
      return <li key={row.query_id} className="space-y-2 rounded border border-border p-3"><p className="break-all">{row.name}</p><p className="break-all text-xs">条件标识：{row.query_id}</p>
        {reusable ? <><p className="break-all text-sm">路径包含：{reusable.text}</p><p className="text-xs">固定范围：{reusable.spaces.length} 个已保存空间（执行时重新鉴权）</p><Button variant="outline" className="min-h-11" onClick={() => onSelect(reusable)}>选用条件 {row.name}</Button></> : <p className="text-sm">本编辑器暂不能完整表达此条件；不会丢弃筛选、排序或范围后执行。</p>}
      </li>;
    })}</ul>
  </section>;
}
