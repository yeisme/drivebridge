import { useEffect, useRef, useState } from 'react';
import { api } from '../../api';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { invocationBounds } from './invocation-time';
import { invocationDateBounds, type InvocationDateZone } from './invocation-dates';
import { SelectAdapter } from '../../components/SelectAdapter';
import type { components } from '../../generated/personal-workspace';
import { AgentInvocationDetail } from './AgentInvocationDetail';

type Page = components['schemas']['InvocationPageEnvelope']['data'];
const results = { unknown: '结果未知', denied: '请求被拒绝', succeeded: '成功', partial: '部分完成', failed: '失败' };
const text = (value: unknown): value is string => typeof value === 'string' && value.length > 0 && value.length <= 512;
const date = (value: unknown): value is string => typeof value === 'string' && value.length <= 64 && Number.isFinite(Date.parse(value));
function valid(page: Page, agent: string, filter: string, cursor: string) {
  return page && page.coverage === 'best_effort' && page.total === null && page.total_kind === 'unknown' && date(page.observed_at) && typeof page.next_cursor === 'string' && page.next_cursor.length <= 16384 && (!cursor || page.next_cursor !== cursor) && Array.isArray(page.items) && page.items.length <= 50 && new Set(page.items.map(row => row?.invocation_id)).size === page.items.length && page.items.every(row => row && text(row.invocation_id) && row.agent_id === agent && text(row.grant_id) && ['http', 'mcp'].includes(row.protocol) && text(row.action) && text(row.request_id) && Object.hasOwn(results, row.result) && (filter === 'all' || row.result === filter) && date(row.started_at) && (row.finished_at === null || date(row.finished_at) && Date.parse(row.finished_at) >= Date.parse(row.started_at)) && Array.isArray(row.resource_refs) && row.resource_refs.length === 0 && !row.operation_ref && (row.error_code === undefined || typeof row.error_code === 'string' && row.error_code.length <= 128));
}

export function AgentInvocations({ agent, scope, onDenied }: { agent: string; scope: string; onDenied: () => void }) {
  // A changed trusted identity remounts the entire view, including pending pages.
  return <InvocationList key={JSON.stringify([scope, agent])} agent={agent} onDenied={onDenied}/>;
}
function InvocationList({ agent, onDenied }: { agent: string; onDenied: () => void }) {
  const [items, setItems] = useState<Page['items']>([]);
  const [observedAt, setObservedAt] = useState('');
  const [cursor, setCursor] = useState(''), [next, setNext] = useState('');
  const [filter, setFilter] = useState('all'), [attempt, setAttempt] = useState(0);
  const [order, setOrder] = useState<'legacy' | 'started_desc'>('legacy');
  const [after, setAfter] = useState(''), [before, setBefore] = useState(''), [timeError, setTimeError] = useState('');
  const [bounds, setBounds] = useState({ after: '', before: '', inclusive: false, zone: '' });
  const [startDay, setStartDay] = useState(''), [endDay, setEndDay] = useState('');
  const [zone, setZone] = useState<InvocationDateZone>('utc'), [dateError, setDateError] = useState('');
  const localZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  const [state, setState] = useState<'loading' | 'ready' | 'error' | 'stale' | 'denied' | 'offline' | 'unsupported'>('loading');
  const denied = useRef(onDenied); denied.current = onDenied;
  useEffect(() => {
    const controller = new AbortController();
    setState('loading');
    const clear = (reason: 'denied' | 'offline') => { controller.abort(); setItems([]); setNext(''); setState(reason); };
    const invalidate = () => clear('denied'), offline = () => clear('offline');
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('offline', offline);
    if (navigator.onLine === false) offline();
    const query = new URLSearchParams({ agent_id: agent, limit: '50' });
    if (cursor) query.set('cursor', cursor);
    if (filter !== 'all') query.set('result', filter);
    if (order === 'started_desc') query.set('order', order);
    if (bounds.after) query.set(bounds.inclusive ? 'started_at_or_after' : 'started_after', bounds.after);
    if (bounds.before) query.set('started_before', bounds.before);
    if (!controller.signal.aborted) api<Page>(`/agent-invocations?${query}`, 'GET', undefined, {}, controller.signal).then(page => {
      if (controller.signal.aborted) return;
      if (!valid(page, agent, filter, cursor)) throw new Error('invalid_contract');
      setItems(previous => cursor ? [...new Map([...previous, ...page.items].map(row => [row.invocation_id, row])).values()] : page.items);
      setObservedAt(page.observed_at); setNext(page.next_cursor); setState('ready');
    }).catch((error: { status?: number }) => {
      if (controller.signal.aborted) return;
      setItems([]); setNext('');
      if (error.status === 401 || error.status === 403) { setState('denied'); denied.current(); }
      else setState(error.status === 410 ? 'stale' : error.status === 404 || error.status === 405 ? 'unsupported' : 'error');
    });
    return () => { controller.abort(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', offline); };
  }, [agent, cursor, filter, attempt, bounds, order]);
  const refresh = () => { setItems([]); setCursor(''); setNext(''); setAttempt(value => value + 1); };
  function applyTime() {
    if (state === 'denied') return;
    try { const value = invocationBounds(after, before); setBounds({ ...value, inclusive: false, zone: '' }); setTimeError(''); setDateError(''); refresh(); }
    catch { setTimeError('请输入有效的带时区时间（最多三位小数），晚于边界必须早于早于边界。未更改当前查询。'); }
  }
  function applyDates() {
    if (state === 'denied') return;
    try {
      if (zone === 'local' && !localZone) throw new Error('unknown_zone');
      setBounds({ ...invocationDateBounds(startDay, endDay, zone), inclusive: true, zone: zone === 'utc' ? 'UTC' : localZone });
      setDateError(''); setTimeError(''); refresh();
    } catch { setDateError('日期无效、顺序错误或所选时区的午夜不存在；未更改当前查询。请核对日期，必要时使用高级时间点筛选。'); }
  }
  return <section aria-label="调用观察" className="min-w-0 space-y-3">
    <p className="m-0 text-[13px] leading-5 text-muted-foreground">非全量调用观察（best_effort）。仅记录已认证的新协议 HTTP 与 MCP 工具调用；空列表不证明没有调用。读取不执行 Agent，不重试业务。</p>
    <div className="grid min-w-0 grid-cols-2 gap-3">
    {state !== 'denied' && <label className="grid gap-2 text-sm">调用记录排序<SelectAdapter aria-label="调用记录排序" className="min-h-11" value={order} onChange={event => { const value = event.target.value; if ((value === 'legacy' || value === 'started_desc') && value !== order) { refresh(); setOrder(value); } }}><option value="legacy">原记录标识顺序</option><option value="started_desc">开始时间最新优先</option></SelectAdapter></label>}
    {state !== 'denied' && <label className="grid gap-2 text-sm">调用结果筛选<SelectAdapter aria-label="调用结果筛选" className="min-h-11" value={filter} onChange={event => { if (event.target.value !== filter) { refresh(); setFilter(event.target.value); } }}><option value="all">全部结果</option>{Object.entries(results).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</SelectAdapter></label>}
    </div>
    {state !== 'denied' && <details className="min-w-0 border-y border-border">
      <summary className="min-h-11 cursor-pointer py-3 text-[13px] font-medium focus-visible:outline-2 focus-visible:outline-ring md:min-h-8 md:py-2">日期与高级时间筛选</summary>
      <div className="grid min-w-0 gap-5 py-3 sm:grid-cols-2">
      <form className="min-w-0 space-y-3" onSubmit={event => { event.preventDefault(); applyDates(); }}>
      <h3 className="text-sm font-semibold">按日期筛选</h3>
      <label className="grid gap-2 text-sm">日期时区<SelectAdapter aria-label="日期时区" className="min-h-11" value={zone} onChange={event => { if (event.target.value === 'utc' || event.target.value === 'local') setZone(event.target.value); }}><option value="utc">UTC</option>{localZone && <option value="local">本机时区（{localZone}）</option>}</SelectAdapter></label>
      <label className="grid gap-2 text-sm">开始日期<Input type="date" aria-label="开始日期" className="min-h-11" value={startDay} onChange={event => setStartDay(event.target.value)} /></label>
      <label className="grid gap-2 text-sm">结束日期（含当日）<Input type="date" aria-label="结束日期（含当日）" className="min-h-11" value={endDay} onChange={event => setEndDay(event.target.value)} /></label>
      <p className="text-xs text-muted-foreground">包含开始日零点，不包含结束日的次日零点；按所选时区的日历计算，夏令时日可能不是 24 小时。重复午夜取较早时刻；不存在的午夜拒绝应用。两种筛选相互替代，修改后需应用。</p>
      {dateError && <p role="alert">{dateError}</p>}
      <Button type="submit" className="min-h-11">应用日期筛选</Button>
    </form>
    <form className="min-w-0 space-y-3" onSubmit={event => { event.preventDefault(); applyTime(); }}>
      <h3 className="text-sm font-semibold">高级时间点筛选</h3>
      <p className="text-xs text-muted-foreground">按开始时间筛选，两端均不包含。留空表示不限；请输入带时区的 RFC3339 时间，例如 2026-10-06T09:00:00+08:00，最多三位小数。不自动采用浏览器时区；修改后需应用。</p>
      <label className="grid gap-2 text-sm">开始时间晚于<Input aria-label="开始时间晚于" className="min-h-11" value={after} maxLength={35} onChange={event => setAfter(event.target.value)} /></label>
      <label className="grid gap-2 text-sm">开始时间早于<Input aria-label="开始时间早于" className="min-h-11" value={before} maxLength={35} onChange={event => setBefore(event.target.value)} /></label>
      {timeError && <p role="alert">{timeError}</p>}
      <div className="flex flex-wrap gap-2"><Button type="submit" className="min-h-11">应用时间筛选</Button><Button type="button" variant="outline" className="min-h-11" onClick={() => { setAfter(''); setBefore(''); setStartDay(''); setEndDay(''); setBounds({ after: '', before: '', inclusive: false, zone: '' }); setTimeError(''); setDateError(''); refresh(); }}>清除时间筛选</Button></div>
    </form>
      </div>
    </details>}
    {(bounds.after || bounds.before) && <p className="m-0 break-all text-xs text-muted-foreground">已应用（UTC）：{bounds.inclusive ? '不早于' : '晚于'} {bounds.after || '不限'}；早于 {bounds.before || '不限'}。{bounds.inclusive ? `包含起点、不包含终点；日期时区 ${bounds.zone}。` : '不包含边界时刻。'}</p>}
    <p className="m-0 text-xs leading-5 text-muted-foreground">{order === 'started_desc' ? '请求服务端按开始时间倒序、同刻按记录标识升序分页。' : '按原记录标识分页，非时间排序。'}返回时间不等于业务完成；没有返回时间也不能证明仍在运行。</p>
    {state === 'loading' && <p role="status">正在读取调用观察…</p>}
    {state === 'ready' && !items.length && <p role="status">当前筛选没有可见观察记录，不代表没有发生调用。</p>}
    {state === 'stale' && <p role="alert">调用记录已变化，旧分页已清除，请从第一页重读。</p>}
    {state === 'error' && <p role="alert">调用记录不可用或合同不匹配，不将失败当作空列表。旧服务可能不支持所选排序或时间条件；不会自动放宽查询。</p>}
    {state === 'unsupported' && <p role="status">当前服务尚未提供调用观察列表。</p>}
    {state === 'denied' && <p role="alert">权限或范围已变化，调用记录已清除。</p>}
    {state === 'offline' && <p role="alert">已离线，调用记录已清除。恢复网络后请手动重读。</p>}
    {state !== 'denied' && <Button variant="outline" className="min-h-11" disabled={state === 'loading'} onClick={refresh}>重读调用记录</Button>}
    <ul aria-label="调用记录列表" className="m-0 list-none divide-y divide-border rounded-lg border border-border bg-card p-0">{items.map(row => <li key={row.invocation_id} className="min-w-0 space-y-1 p-3 text-[13px]">
      <h3 className="m-0 break-all text-sm font-semibold">{row.protocol.toUpperCase()} · {row.action}</h3>
      <p>{results[row.result]}{row.result === 'unknown' ? ' · 未独立证明业务效果' : ''}</p>
      <p className="break-all">调用标识：{row.invocation_id}</p><p className="break-all">请求标识：{row.request_id}</p>
      <p className="break-all">开始时间：{row.started_at}</p><p className="break-all">{row.finished_at ? `观察返回时间：${row.finished_at}` : '未观察到返回时间（不代表仍在运行）'}</p>
      {row.error_code && <p className="break-all">安全错误码：{row.error_code}</p>}
      <p className="text-xs text-muted-foreground">未提供经证明的资源或业务操作关联。</p>
      <AgentInvocationDetail invocation={row} observedAt={observedAt} resultLabel={results[row.result]} />
    </li>)}</ul>
    {state === 'ready' && next && <Button variant="outline" className="min-h-11" onClick={() => setCursor(next)}>更多调用记录</Button>}
  </section>;
}
