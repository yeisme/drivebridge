import { useEffect, useRef, useState } from 'react';
import { api } from '../../api';
import { AppDialog } from '../../components/AppDialog';
import { Button } from '../../components/ui/button';
import { ResourceSnapshot } from '../resources/ResourceSnapshot';
import type { components } from '../../generated/personal-workspace';
type Index = components['schemas']['SnapshotIndexEnvelope']['data'];
export function validSnapshotIndex(value: Index, id: string) {
  return !!value && Object.keys(value).sort().join(',') === 'collection_id,items,observed_at' && value.collection_id === id && typeof value.observed_at === 'string' && Number.isFinite(Date.parse(value.observed_at)) && Array.isArray(value.items) && value.items.length <= 1000 && value.items.every(row => row && Object.keys(row).sort().join(',') === 'collection_revision,created_at,snapshot_id' && typeof row.snapshot_id === 'string' && /^[a-f0-9]{48}$/.test(row.snapshot_id) && Number.isSafeInteger(row.collection_revision) && row.collection_revision > 0 && typeof row.created_at === 'string' && Number.isFinite(Date.parse(row.created_at))) && new Set(value.items.map(row => row.snapshot_id)).size === value.items.length;
}
export function SnapshotHistory({ id, close, denied }: { id: string; close: () => void; denied: () => void }) {
  const [state, setState] = useState('idle'), [index, setIndex] = useState<Index | null>(null), [shown, setShown] = useState(50), [selected, setSelected] = useState('');
  const active = useRef(true), running = useRef(false), controller = useRef<AbortController | null>(null), trigger = useRef<HTMLButtonElement | null>(null);
  const clear = () => { setIndex(null); setSelected(''); setShown(50); };
  useEffect(() => {
    active.current = true; clear(); setState('idle');
    const invalidate = () => { active.current = false; controller.current?.abort(); clear(); setState('denied'); };
    const offline = () => { invalidate(); setState('offline'); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('offline', offline);
    if (!navigator.onLine) offline();
    return () => { active.current = false; controller.current?.abort(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', offline); };
  }, [id]);
  async function load() {
    if (!active.current || running.current || !navigator.onLine) return;
    running.current = true; clear(); setState('loading'); const abort = new AbortController(); controller.current = abort;
    try {
      const value = await api<Index>('/file-collections/' + encodeURIComponent(id) + '/snapshots', 'GET', undefined, {}, abort.signal);
      if (!active.current || abort.signal.aborted) return;
      if (!validSnapshotIndex(value, id)) throw new Error('invalid_index');
      setIndex(value); setState('ready');
    } catch (error) {
      if (!active.current || abort.signal.aborted) return;
      clear(); const status = (error as { status?: number }).status;
      if (status === 401 || status === 403) { setState('denied'); denied(); }
      else setState(status === 429 ? 'budget' : status === 404 ? 'unavailable' : 'error');
    } finally { running.current = false; }
  }
  return <AppDialog title="集合历史快照" close={close}>
    <p className="text-sm text-muted-foreground">历史索引不是原创建回执。缺项不能证明未提交；出现也不保证成员仍可读或内容安全。原创建结果未知时，请使用“创建快照”中的原键回执查询。</p>
    <Button className="min-h-11" disabled={state === 'loading' || state === 'denied' || state === 'offline'} onClick={() => void load()}>{index ? '刷新历史快照索引' : '读取历史快照索引'}</Button>
    {state === 'loading' && <p role="status">读取历史快照索引…</p>}
    {state === 'budget' && <p role="alert">历史快照超出 1000 项查询预算，本次未返回部分列表；此入口没有服务端分页。</p>}
    {state === 'unavailable' && <p role="alert">集合不可见或历史入口不可用，不代表没有快照。</p>}
    {state === 'error' && <p role="alert">历史索引读取失败或合同不匹配，不代表没有快照。</p>}
    {state === 'denied' && <p role="alert">权限或范围已变化，索引与阅读内容已清除，请关闭后重新进入。</p>}
    {state === 'offline' && <p role="alert">已离线，索引与阅读内容已清除；联网不会自动读取，请重新打开。</p>}
    {index && <><p>观察时间：{index.observed_at}。按创建时间与标识倒序，最多 1000 项；每次展开 50 项只是本地展示。</p>
      {!index.items.length && <p role="status">当前授权集合没有已返回的历史快照。</p>}
      <ul aria-label="历史快照索引" className="m-0 list-none divide-y divide-border p-0">{index.items.slice(0, shown).map(row => <li key={row.snapshot_id} className="space-y-2 py-3"><p className="break-all text-xs">快照 {row.snapshot_id}</p><p>来源集合修订 {row.collection_revision} · {row.created_at}</p><Button variant="outline" className="min-h-11" onClick={event => { trigger.current = event.currentTarget; setSelected(row.snapshot_id); }}>查看历史快照 {row.snapshot_id}</Button></li>)}</ul>
      {shown < index.items.length && <Button variant="outline" className="min-h-11" onClick={() => setShown(value => value + 50)}>展开更多已读取快照</Button>}
    </>}
    {selected && <ResourceSnapshot key={selected} snapshot={selected} expectedCollection={id} close={() => { setSelected(''); trigger.current?.focus(); }} onDenied={() => { controller.current?.abort(); active.current = false; clear(); setState('denied'); denied(); }}/ >}
  </AppDialog>;
}
