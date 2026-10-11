import { useEffect, useRef, useState } from 'react';
import { RefreshCw, ShieldCheck } from 'lucide-react';
import { bytes, type Space } from '../../api';
import { Button } from '../../components/ui/button';
import { SelectAdapter } from '../../components/SelectAdapter';
import { WorkspacePageHeader } from '../../components/WorkspacePageHeader';
import { readCapabilities, readReadiness, type Capabilities, type Readiness } from './client';

const labels: Record<string, string> = { file_list: '浏览文件', file_read: '读取文件', file_search: '查找文件', upload_create: '创建上传', upload_complete: '完成上传', file_archive: '归档文件', file_trash: '移入回收站', file_restore: '恢复文件', mount_directory: '本机目录挂载', usage_scan: '查看用量' };
const reasons: Record<string, string> = { storage_not_configured: '尚未配置存储', dependency_unavailable: '存储依赖不可用', owner_host_dependency: '需要本机宿主确认', forbidden: '当前身份无权执行', agent_not_permitted: '此动作不向 Agent 开放' };
const limits: Record<string, string> = { max_file_bytes: '单文件上限', max_batch_items: '批次条目上限', space_quota_bytes: '空间配额', space_remaining_bytes: '空间剩余额度' };

export function CapabilitiesPage({ scope, spaces }: { scope: string; spaces: Space[] }) {
  return <CapabilityScope key={scope} spaces={spaces}/>;
}
function CapabilityScope({ spaces }: { spaces: Space[] }) {
  const [chosen, setChosen] = useState('');
  const space = spaces.some(item => item.id === chosen) ? chosen : spaces[0]?.id || '';
  return <section aria-label="能力与连接说明" className="min-w-0 space-y-4">
    <WorkspacePageHeader title="能力与连接说明" description="实现、配置、权限和连接观察是不同事实。打开此页不会连接存储或运行任务。"/>
    {spaces.length > 0 && <label className="flex max-w-sm items-center gap-3 text-[13px]"><span className="shrink-0 text-muted-foreground">查看空间</span><SelectAdapter aria-label="能力检查空间" className="min-h-11 md:min-h-8" value={space} onChange={event => setChosen(event.target.value)}>{spaces.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</SelectAdapter></label>}
    <CapabilityData key={space} space={space}/>
  </section>;
}
function CapabilityData({ space }: { space: string }) {
  const [data, setData] = useState<Capabilities | null>(null), [state, setState] = useState('loading'), [attempt, setAttempt] = useState(0);
  const [checking, setChecking] = useState(''), [report, setReport] = useState<Readiness | null>(null), [checkError, setCheckError] = useState('');
  const check = useRef<AbortController | null>(null);
  const denied = useRef(false);
  const clear = () => { check.current?.abort(); setChecking(''); setCheckError(''); setReport(null); setData(null); };
  useEffect(() => {
    const controller = new AbortController(); denied.current = false; clear(); setState('loading');
    const invalidate = () => { denied.current = true; controller.abort(); clear(); setState('denied'); };
    const offline = () => { controller.abort(); clear(); setState('offline'); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('offline', offline);
    if (!navigator.onLine) offline();
    else readCapabilities(space, controller.signal).then(value => { if (!controller.signal.aborted) { setData(value); setState('ready'); } }).catch((error: { status?: number }) => {
      if (controller.signal.aborted) return;
      if (error.status === 401 || error.status === 403) invalidate();
      else { clear(); setState(error.status === 404 || error.status === 405 ? 'unsupported' : 'error'); }
    });
    return () => { controller.abort(); check.current?.abort(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', offline); };
  }, [space, attempt]);
  async function inspect(action: string) {
    if (state !== 'ready' || denied.current || !navigator.onLine || !space || checking || (check.current && !check.current.signal.aborted) || !data?.actions.includes(action)) return;
    check.current?.abort(); const controller = new AbortController(); check.current = controller;
    setChecking(action); setReport(null); setCheckError('');
    try { const result = await readReadiness(action, space, controller.signal); if (!controller.signal.aborted) setReport(result); }
    catch (error) {
      if (controller.signal.aborted) return;
      const status = (error as { status?: number }).status;
      if (status === 401 || status === 403) { denied.current = true; clear(); setState('denied'); }
      else setCheckError('未取得此动作的可用性，请重读或稍后显式检查；不会自动重试。');
    } finally { if (!controller.signal.aborted) { controller.abort(); setChecking(''); } }
  }
  return <>
    <div className="flex items-center justify-between gap-3"><span className="text-xs text-muted-foreground">服务端动作目录 · 不代表全部产品能力</span><Button variant="outline" className="min-h-11 text-[13px] md:min-h-8" disabled={state === 'loading' || state === 'denied'} onClick={() => setAttempt(value => value + 1)}><RefreshCw className="size-4"/>重读能力说明</Button></div>
    {state === 'loading' && <p role="status">正在读取服务能力…</p>}
    {state === 'denied' && <p role="alert">权限或范围已变化，能力与检查结果已清除。请重新进入。</p>}
    {state === 'offline' && <p role="alert">当前离线，旧观察已清除；联网后请手动重读。</p>}
    {state === 'unsupported' && <p role="status">此服务没有提供可识别的能力说明入口。</p>}
    {state === 'error' && <p role="alert">能力说明读取失败或格式不匹配，不能据此判断能力不存在。</p>}
    {checkError && <p role="alert">{checkError}</p>}
    {report && <section aria-label="动作可用性观察" className="space-y-3 rounded-lg border border-border bg-card p-4"><h3 className="flex items-center gap-2 text-sm font-semibold"><ShieldCheck className="size-4"/>{labels[report.action] || report.action} · {report.availability === 'available' ? '当前条件允许' : report.availability === 'blocked' ? '当前受阻' : '尚未确定'}</h3><dl className="grid gap-3 text-[13px] sm:grid-cols-3">{[['实现', report.implemented ? '已实现' : '未实现'], ['配置', report.configured ? '已配置' : '未声明或不需要'], ['授权', report.authorized ? '当前范围允许' : '未获授权']].map(([name,value]) => <div key={name}><dt className="text-muted-foreground">{name}</dt><dd className="m-0 mt-1">{value}</dd></div>)}</dl><p className="text-xs text-muted-foreground">连接观察：{report.observedAt ? new Date(report.observedAt).toLocaleString('zh-CN') : '未提供'} · {report.dependency === 'ok' ? '已观察可用' : report.dependency === 'none' ? '不依赖连接' : '依赖未就绪或未观察'}</p>{report.reason && <p className="text-sm">{reasons[report.reason] || '当前存在限制，请按原服务合同检查。'}</p>}<p className="text-xs text-muted-foreground">这是读取时的判断，不是后续执行许可；实际操作仍由服务端重新鉴权。</p></section>}
    {data && <>
      <ul aria-label="服务动作目录" className="m-0 list-none divide-y divide-border rounded-lg border border-border bg-card p-0">{data.actions.map(action => <li key={action} className="flex flex-wrap items-center gap-3 px-3 py-2"><div className="min-w-0 flex-1"><p className="text-sm font-medium">{labels[action] || action}</p><p className="text-xs text-muted-foreground">服务端已声明 · 可用性需按当前空间检查</p></div><Button variant="outline" className="min-h-11 text-[13px] md:min-h-8" disabled={!space || !!checking || state !== 'ready'} onClick={() => void inspect(action)} aria-label={'检查能力 '+action}>{checking === action ? '读取中…' : '检查条件'}</Button></li>)}</ul>
      {!space && <p role="status" className="text-[13px] text-muted-foreground">没有可选空间；仅展示动作目录，不推断你的执行权限。</p>}
      <section aria-label="服务限额" className="space-y-3 border-t border-border pt-4"><h3 className="text-sm font-semibold">当前限额</h3><dl className="grid gap-4 text-[13px] sm:grid-cols-2 lg:grid-cols-4">{Object.entries(data.limits).map(([key,value]) => <div key={key}><dt className="text-muted-foreground">{limits[key]}</dt><dd className="m-0 mt-1">{value === null ? '未提供' : key === 'max_batch_items' ? value+' 项' : bytes(value)}</dd></div>)}</dl><p className="text-xs text-muted-foreground">未提供不表示无限；没有空间时的限额仅是实例级信息。</p></section>
    </>}
  </>;
}
