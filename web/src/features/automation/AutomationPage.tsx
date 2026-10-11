import { useEffect, useRef, useState } from 'react';
import { Folder, RefreshCw, Workflow } from 'lucide-react';
import { api, type Space } from '../../api';
import type { Page } from '../../app/navigation-state';
import { WorkspacePageHeader } from '../../components/WorkspacePageHeader';
import { Button } from '../../components/ui/button';
import { SelectAdapter } from '../../components/SelectAdapter';
import { readRules, type AutomationRuleView } from './client';
import { AutomationActionDialog } from './AutomationActionDialog';
import type { AutomationAction } from './automation-recovery';

export function AutomationPage({ scope, spaces, onNavigate }: { scope: string; spaces: Space[]; onNavigate: (page: Page) => void }) {
  return <AutomationScope key={scope} scope={scope} spaces={spaces} onNavigate={onNavigate}/>;
}
function AutomationScope({ scope, spaces, onNavigate }: { scope: string; spaces: Space[]; onNavigate: (page: Page) => void }) {
  const [chosen, setChosen] = useState('');
  const space = spaces.some(item => item.id === chosen) ? chosen : spaces[0]?.id || '';
  return <section aria-label="备份与整理" className="min-w-0 space-y-5">
    <WorkspacePageHeader title="备份与整理" description="查看服务端已保存的整理规则。打开页面只读取，不运行规则、不推进进度，也不启用设备备份。" actions={<Button variant="outline" className="min-h-11 text-[13px] md:min-h-8" onClick={() => onNavigate('flows')}>打开受控文件流程</Button>}/>
    {spaces.length ? <><label className="flex max-w-sm items-center gap-3 text-[13px]"><span className="shrink-0 text-muted-foreground">规则空间</span><SelectAdapter aria-label="整理规则空间" className="min-h-11 md:min-h-8" value={space} onChange={event => setChosen(event.target.value)}>{spaces.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</SelectAdapter></label><Rules key={space} space={space} scope={scope}/></> : <p role="status">当前范围没有可用空间，未读取整理规则。</p>}
    <section className="space-y-2 border-t border-border pt-4" aria-label="设备备份说明"><h3 className="text-sm font-semibold">设备备份与服务端整理分开管理</h3><p className="max-w-3xl text-[13px] leading-6 text-muted-foreground">当前网页没有相册或后台备份宿主，不会显示演示任务。规则处于启用状态也不代表有后台执行器；执行结果以原意图和操作回执为准。</p><div className="flex flex-wrap gap-2"><Button variant="ghost" className="min-h-11 text-[13px] md:min-h-8" onClick={() => onNavigate('devices')}>查看设备能力</Button><Button variant="ghost" className="min-h-11 text-[13px] md:min-h-8" onClick={() => onNavigate('transfers')}>查看任务与原意图</Button></div></section>
  </section>;
}
function Rules({ space, scope }: { space: string; scope: string }) {
  const [rules, setRules] = useState<AutomationRuleView[]>([]), [state, setState] = useState('loading'), [attempt, setAttempt] = useState(0);
  const [management, setManagement] = useState<'unknown' | 'checking' | 'available' | 'closed' | 'error'>('unknown');
  const [editing, setEditing] = useState<{ action: AutomationAction; rule?: AutomationRuleView } | null>(null);
  const managementController = useRef<AbortController | null>(null), readController = useRef<AbortController | null>(null);
  const targets = useRef(new Map<string, HTMLButtonElement>()), returnKey = useRef<string | null>(null);
  const targetRef = (key: string) => (node: HTMLButtonElement | null) => { if (node) targets.current.set(key, node); else targets.current.delete(key); };
  const deny = () => { readController.current?.abort(); managementController.current?.abort(); setManagement('unknown'); setEditing(null); setRules([]); setState('denied'); returnKey.current = null; };
  const close = () => { setEditing(null); setState('loading'); setAttempt(value => value + 1); };
  const open = (action: AutomationAction, rule?: AutomationRuleView) => { returnKey.current = rule ? action + ':' + rule.id : 'create'; setEditing({ action, rule }); };
  useEffect(() => { if (state === 'loading' || !returnKey.current || editing) return; (targets.current.get(returnKey.current) || targets.current.get('refresh'))?.focus(); returnKey.current = null; }, [state, rules, editing]);
  async function enableManagement() {
    managementController.current?.abort(); const controller = new AbortController(); managementController.current = controller; setManagement('checking');
    try {
      const value = await api<{ available: boolean }>('/spaces/' + encodeURIComponent(space) + '/automation-management', 'GET', undefined, {}, controller.signal);
      if (controller.signal.aborted) return;
      if (!value || typeof value.available !== 'boolean') throw new Error('invalid_contract');
      setManagement(value.available ? 'available' : 'closed');
    } catch (error) {
      if (controller.signal.aborted) return;
      const status = (error as { status?: number }).status;
      if (status === 401 || status === 403) deny(); else setManagement([404, 405, 422].includes(status || 0) ? 'closed' : 'error');
    }
  }
  useEffect(() => {
    const controller = new AbortController(); readController.current = controller; setRules([]); setState('loading');
    const invalidate = () => { controller.abort(); deny(); };
    const offline = () => { controller.abort(); managementController.current?.abort(); setRules([]); setManagement('unknown'); setEditing(null); returnKey.current = null; setState('offline'); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('offline', offline);
    if (!navigator.onLine) offline();
    else readRules(space, controller.signal).then(value => { if (!controller.signal.aborted) { setRules(value); setState('ready'); } }).catch((error: { status?: number }) => {
      if (controller.signal.aborted) return;
      if (error.status === 401 || error.status === 403) deny();
      else { setRules([]); setState(error.status === 404 || error.status === 405 ? 'unsupported' : 'error'); }
    });
    return () => { controller.abort(); managementController.current?.abort(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', offline); };
  }, [space, attempt]);
  return <>
    <div className="flex flex-wrap items-center justify-between gap-3"><h3 className="text-sm font-semibold">整理规则</h3><div className="flex flex-wrap gap-2">{management === 'available' ? <Button ref={targetRef('create')} className="min-h-11 md:min-h-8" disabled={state !== 'ready'} onClick={() => open('change')}>新建规则/恢复</Button> : <Button variant="outline" className="min-h-11 md:min-h-8" disabled={state !== 'ready' || management === 'checking'} onClick={() => void enableManagement()}>{management === 'checking' ? '正在读取管理能力…' : '配置与运行'}</Button>}<Button ref={targetRef('refresh')} variant="outline" className="min-h-11 text-[13px] md:min-h-8" disabled={state === 'loading' || state === 'denied'} onClick={() => setAttempt(value => value + 1)}><RefreshCw className="size-4"/>重读整理规则</Button></div></div>
    {management === 'closed' && <p role="status" className="text-[13px] text-muted-foreground">服务端尚未开放人工管理或当前权限不足。需由服务端操作者显式启用 automation-management；旧规则仍只读，不自动运行。</p>}
    {management === 'error' && <p role="alert">管理能力读取失败；请手动重试，没有发送规则修改。</p>}
    {state === 'loading' && <p role="status">正在读取整理规则…</p>}
    {state === 'denied' && <p role="alert">权限或范围已变化，规则已清除，请重新进入。</p>}
    {state === 'offline' && <p role="alert">当前离线，规则观察已清除；联网后请手动重读。</p>}
    {state === 'unsupported' && <p role="status">当前服务未提供整理规则读取入口。</p>}
    {state === 'error' && <p role="alert">规则读取失败、超出展示预算或格式不匹配，不代表没有规则。</p>}
    {state === 'ready' && !rules.length && <div role="status" className="flex items-start gap-3 border-y border-border py-8"><Workflow className="mt-0.5 size-5 shrink-0 text-muted-foreground"/><div className="space-y-2"><p className="text-sm font-medium">当前空间没有已保存的整理规则。</p><p className="text-[13px] text-muted-foreground">可以先从受控文件流程预览一次整理计划；不会自动创建规则。</p></div></div>}
    <ul aria-label="整理规则列表" className="m-0 list-none divide-y divide-border p-0">{rules.map(rule => <li key={rule.id} className="space-y-3 border-x border-border bg-card p-4 first:rounded-t-lg first:border-t last:rounded-b-lg last:border-b"><div className="flex min-w-0 flex-wrap items-center gap-3"><h4 className="min-w-0 flex-1 break-words text-sm font-semibold">{rule.action === 'add_label' ? '添加标签 · '+rule.detail : '未识别规则动作'}</h4><span className="rounded bg-muted px-2 py-1 text-xs">{rule.paused ? '已暂停' : rule.enabled ? '规则已启用' : '未启用'}</span></div><p className="flex items-start gap-2 break-all text-[13px]"><Folder className="mt-0.5 size-4 shrink-0 text-muted-foreground"/>{rule.prefix || '当前空间全部目录'}</p><dl className="grid grid-cols-2 gap-3 text-xs text-muted-foreground sm:grid-cols-3"><div><dt>单次条目预算</dt><dd className="m-0 mt-1">{rule.budget > 0 ? rule.budget+' 项' : '规则未设条目预算，不代表无系统限制'}</dd></div><div><dt>规则修订</dt><dd className="m-0 mt-1">{rule.revision}</dd></div><div className="col-span-2 sm:col-span-1"><dt>最后变更</dt><dd className="m-0 mt-1">{new Date(rule.updatedAt).toLocaleString('zh-CN')}</dd></div></dl><details className="text-xs text-muted-foreground"><summary className="min-h-11 cursor-pointer py-3 md:min-h-8 md:py-2">规则与策略标识</summary><p className="break-all">规则标识：{rule.id}</p><p className="break-all">策略：{rule.policyID || '未绑定（字节预算与期限未设置）'}</p></details>
      {management === 'available' && <div className="flex flex-wrap gap-2 border-t border-border pt-2"><Button ref={targetRef('change:' + rule.id)} variant="outline" className="min-h-11 md:min-h-8" aria-label={'配置规则 ' + rule.id} disabled={state !== 'ready'} onClick={() => open('change', rule)}>配置/暂停/恢复</Button><Button ref={targetRef('execute:' + rule.id)} variant="outline" className="min-h-11 md:min-h-8" aria-label={'预览运行 ' + rule.id} disabled={state !== 'ready'} onClick={() => open('execute', rule)}>预览运行/恢复</Button></div>}</li>)}</ul>
    <p className="text-xs text-muted-foreground">打开页面只读规则；配置与运行需人工确认和服务端开放。运行仅处理预览内的有界事件；查看任务与原意图核对逐项状态，不把调用返回等同于全部完成。</p>
    {editing && <AutomationActionDialog scope={scope} space={space} action={editing.action} rule={editing.rule} close={close} denied={deny}/>}
  </>;
}
