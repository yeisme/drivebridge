import { useEffect, useRef, useState } from 'react';
import { api } from '../../api';
import { AppDialog } from '../../components/AppDialog';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { Checkbox } from '../../components/ui/checkbox';
import type { AutomationRuleView } from './client';
import { automationDigest, automationSlot, claimAutomation, clearAutomation, matchesAutomation, readAutomationRecovery, validatePreview, type AutomationAction, type AutomationChange, type AutomationExecute, type AutomationPreview, type AutomationReceipt, type AutomationRecovery } from './automation-recovery';

type Phase = 'initializing' | 'edit' | 'idle' | 'review' | 'busy' | 'unknown' | 'conflict' | 'done' | 'denied' | 'unavailable';
export function AutomationActionDialog({ scope, space, action, rule, close, denied }: {
  scope: string; space: string; action: AutomationAction; rule?: AutomationRuleView; close: () => void; denied: () => void;
}) {
  const slot = automationSlot(scope, space, action, action === 'execute' ? rule?.id : '');
  const target = action === 'execute' ? rule?.id || '' : space;
  const endpoint = action === 'execute' ? '/automation-rules/' + encodeURIComponent(target) + '/execute' : '/spaces/' + encodeURIComponent(space) + '/automation-rules/changes';
  const [phase, setPhase] = useState<Phase>('initializing'), [message, setMessage] = useState('');
  const [label, setLabel] = useState(rule?.detail.slice(rule.detail.indexOf(':') + 1) || ''), [prefix, setPrefix] = useState(rule?.prefix || '');
  const [budget, setBudget] = useState(String(rule?.budget && rule.budget <= 200 ? rule.budget : 50));
  const [enabled, setEnabled] = useState(rule?.enabled || false), [clearPause, setClearPause] = useState(false);
  const [preview, setPreview] = useState<AutomationPreview | null>(null), [receipt, setReceipt] = useState<AutomationReceipt | null>(null);
  const [record, setRecord] = useState<AutomationRecovery | null>(null), [retry, setRetry] = useState(false);
  const original = useRef<AutomationChange | AutomationExecute | null>(null), controller = useRef<AbortController | null>(null);
  const active = useRef(true), running = useRef(false), deniedCallback = useRef(denied);
  deniedCallback.current = denied;
  const clearProtected = () => { original.current = null; setPreview(null); setReceipt(null); setMessage(''); setLabel(''); setPrefix(''); };
  useEffect(() => {
    active.current = true;
    try { const value = readAutomationRecovery(slot); setRecord(value); setPhase(value ? 'unknown' : action === 'change' ? 'edit' : 'idle'); }
    catch { setPhase('unavailable'); }
    const invalidate = () => { active.current = false; controller.current?.abort(); clearProtected(); setPhase('denied'); };
    const storage = (event: StorageEvent) => { if (event.key === null || event.key === slot) invalidate(); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('offline', invalidate); window.addEventListener('storage', storage);
    if (!navigator.onLine) invalidate();
    return () => { active.current = false; controller.current?.abort(); original.current = null; window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', invalidate); window.removeEventListener('storage', storage); };
  }, [slot, action]);
  function reviewChange() {
    const count = Number(budget), name = label.trim();
    if (!name || new TextEncoder().encode(name).length > 200 || /[\u0000-\u001f]/.test(name) || !Number.isInteger(count) || count < 1 || count > 200 || new TextEncoder().encode(prefix).length > 1024 || /[\\\u0000-\u001f]/.test(prefix) || (prefix && prefix.split('/').some(part => !part || part === '.' || part === '..'))) {
      setMessage('请填写规范相对目录、1–200 项预算与有效标签（最多 200 UTF-8 字节）。'); return;
    }
    const namespace = rule?.detail.split(':', 1)[0] || '';
    if (rule && (rule.action !== 'add_label' || !rule.detail.includes(':') || !namespace || new TextEncoder().encode(namespace).length > 96 || /[\u0000-\u001f]/.test(namespace))) { setMessage('此规则动作不在当前可编辑合同内，保留原规则，不改写为其他动作。'); return; }
    original.current = { action: 'add_label', action_detail: rule ? rule.detail : 'rule:' + name, prefix: rule ? rule.prefix : prefix, ...(rule?.policyID ? { policy_id: rule.policyID } : {}), budget_items: count, enabled, clear_pause: clearPause, expected_revision: rule?.revision || 0 };
    setMessage(''); setPhase('review');
  }
  async function perform(mode: 'preview' | 'write' | 'lookup' | 'retry') {
    if (!active.current || running.current || !navigator.onLine) return;
    running.current = true; setPhase('busy'); setMessage(''); setRetry(false);
    const abort = new AbortController(); controller.current = abort;
    let recovery = record, sent = false;
    try {
      if (mode === 'preview') {
        if (!rule || action !== 'execute') throw new Error('missing_rule');
        const value = await api<AutomationPreview>('/automation-rules/' + encodeURIComponent(rule.id) + '/preview', 'POST', {}, {}, abort.signal);
        if (!active.current || abort.signal.aborted) return;
        const checked = validatePreview(value, space, rule.id);
        original.current = { expected_revision: checked.revision, preview_digest: checked.preview_digest };
        setPreview(checked); setPhase('review'); return;
      }
      if (mode === 'write') {
        if (!original.current) throw new Error('missing_original');
        const digest = await automationDigest(target, action, original.current);
        if (!active.current || abort.signal.aborted) return;
        const claim = await claimAutomation(slot, digest, abort.signal);
        if (!active.current || abort.signal.aborted) return;
        recovery = claim.record; setRecord(recovery);
        if (!claim.claimed) { original.current = null; setPreview(null); setPhase('unknown'); return; }
      }
      if (!recovery || JSON.stringify(readAutomationRecovery(slot)) !== JSON.stringify(recovery)) throw new Error('recovery_changed');
      let result: AutomationReceipt;
      if (mode === 'lookup') result = await api<AutomationReceipt>(endpoint + '/lookup', 'POST', { idempotency_key: recovery.key }, {}, abort.signal);
      else {
        if (!original.current || await automationDigest(target, action, original.current) !== recovery.digest) throw new Error('missing_original');
        if (!active.current || abort.signal.aborted || !navigator.onLine) return;
        sent = true;
        result = await api<AutomationReceipt>(endpoint, 'POST', original.current, { 'Idempotency-Key': recovery.key }, abort.signal);
      }
      if (!active.current || abort.signal.aborted) return;
      if (!matchesAutomation(result, recovery, space, action, rule?.id) || JSON.stringify(readAutomationRecovery(slot)) !== JSON.stringify(recovery)) throw new Error('receipt_mismatch');
      original.current = null; setPreview(null); setReceipt(result); setPhase('done');
    } catch (error) {
      if (!active.current || abort.signal.aborted) return;
      const failure = error as Error & { status?: number };
      if (failure.status === 401 || failure.status === 403) { clearProtected(); setPhase('denied'); deniedCallback.current(); }
      else if (mode === 'preview') { original.current = null; setPreview(null); setPhase('idle'); setMessage('预览读取失败或服务未开启，未执行规则。请检查权限和部署能力后重试。'); }
      else if (sent && failure.status === 409 && ['version_conflict', 'plan_stale', 'rule_paused'].includes(failure.message)) {
        original.current = null; setPreview(null); setPhase('conflict'); setMessage(action === 'execute' ? '服务端已拒绝此过期预览或暂停规则，没有执行本次请求。请保留目标并重读后重新确认。' : '服务端已拒绝此过期规则修订，没有保存本次修改。请保留目标并重读后重新确认。');
      } else if (sent && (failure.status === 400 || failure.status === 422)) {
        original.current = null; setPreview(null); setPhase('conflict'); setMessage('服务端拒绝了请求格式或未启用管理能力，本次请求未执行。请关闭并核对配置。');
      } else if (!recovery) setPhase('unavailable');
      else { setPreview(null); setPhase('unknown'); setRetry(mode === 'lookup' && failure.status === 404 && original.current !== null); setMessage('原请求结果尚未确认；只查原回执，未查到也不能证明未执行，不自动换键重发。'); }
    } finally { running.current = false; }
  }
  async function acknowledge() {
    if (!active.current || running.current || !record || (phase !== 'done' && phase !== 'conflict')) return;
    running.current = true;
    try { await clearAutomation(slot, record); if (active.current) close(); }
    catch { if (active.current) setMessage('恢复身份已变化，不能清除其他页面的请求；请关闭后重新确认。'); }
    finally { running.current = false; }
  }
  const title = action === 'execute' ? '预览并运行整理规则' : rule ? '配置整理规则' : '新建整理规则';
  return <AppDialog title={title} close={close} className="max-md:[&_button]:min-h-11 [&_input:not([type=hidden])]:min-h-11">
    <p className="text-[13px] text-muted-foreground">规则只添加标签，不移动或删除文件。启用仅允许人工确认的一次运行，不启动后台任务；回执与逐项结果分开查看。</p>
    {message && <p role="alert" className="text-sm">{message}</p>}
    {(phase === 'initializing' || phase === 'busy') && <p role="status">正在核对原请求…</p>}
    {phase === 'edit' && <form onSubmit={event => { event.preventDefault(); reviewChange(); }}>
      <label>规则目录<Input aria-label="规则目录" value={prefix} disabled={!!rule} onChange={event => setPrefix(event.target.value)} placeholder="例如 references；留空为当前空间"/></label>
      <label>标签名称<Input aria-label="标签名称" value={label} disabled={!!rule} required onChange={event => setLabel(event.target.value)}/></label>
      {rule && <p className="text-xs text-muted-foreground">目录和标签是此规则身份的一部分；要更换请新建规则，不改写历史规则。</p>}
      <label>单次条目预算<Input aria-label="单次条目预算" type="number" min={1} max={200} required value={budget} onChange={event => setBudget(event.target.value)}/></label>
      <label className="flex min-h-11 items-center gap-2 text-sm"><Checkbox checked={enabled} onCheckedChange={setEnabled}/>允许手动运行</label>
      {rule?.paused && <label className="flex min-h-11 items-center gap-2 text-sm"><Checkbox checked={clearPause} onCheckedChange={setClearPause}/>已检查失败原因，解除故障暂停</label>}
      <p className="text-xs text-muted-foreground">新规则默认不启用。字节预算和期限只来自已绑定的服务端策略；此表单不创建或扩大策略。</p>
      <Button type="submit" className="min-h-11">预览规则修改</Button>
    </form>}
    {phase === 'idle' && <Button className="min-h-11" onClick={() => void perform('preview')}>读取运行预览</Button>}
    {phase === 'review' && action === 'change' && <div className="space-y-3 text-sm">
      <p className="break-all">目录：{prefix || '当前空间全部目录'} · 标签：{rule?.detail || 'rule:' + label.trim()}</p>
      <p>预算 {budget} 项 · {enabled ? '允许手动运行' : '规则不启用'} · 原修订 {rule?.revision || 0}{clearPause ? ' · 解除故障暂停' : ''}</p>
      <p>这里只保存规则，不执行标签写入。</p>
      <div className="flex flex-wrap gap-2"><Button className="min-h-11" onClick={() => void perform('write')}>确认保存规则</Button><Button className="min-h-11" variant="outline" onClick={() => { original.current = null; setPhase('edit'); }}>返回修改</Button></div>
    </div>}
    {phase === 'review' && action === 'execute' && preview && <div className="space-y-3 text-sm">
      <p className="break-all">动作：添加标签 · {rule?.detail} · 目录：{rule?.prefix || '当前空间全部目录'}</p>
      <p>修订 {preview.revision} · 本批检查 {preview.scanned} 个事件 · 匹配 {preview.items.length} 项 · {preview.coverage === 'partial' ? '部分覆盖，仍可能有后续事件' : '本次检查范围完整'}</p>
      <p className="text-xs text-muted-foreground">策略字节预算：{preview.policy_max_bytes > 0 ? preview.policy_max_bytes + ' B' : '未设置'} · 期限：{preview.policy_expires_at ? new Date(preview.policy_expires_at).toLocaleString('zh-CN') : '未设置'}</p>
      <ul aria-label="整理运行预览" className="m-0 max-h-56 list-none divide-y divide-border overflow-auto p-0">{preview.items.map((item, index) => <li key={item.file_id + ':' + index} className="break-all py-2"><span>{item.path}</span><details><summary className="min-h-11 cursor-pointer py-2 text-xs text-muted-foreground">固定版本</summary><p className="text-xs">{item.version_id} · {item.size_bytes} B</p></details></li>)}</ul>
      {!preview.items.length && <p>本批没有匹配文件；确认只推进本批已检查事件，不承诺全部历史已扫描。</p>}
      {!preview.executable && <p role="status">规则未启用、已暂停或策略不可用，请先在配置中检查；本次不会运行。</p>}
      <div className="flex flex-wrap gap-2"><Button className="min-h-11" disabled={!preview.executable} onClick={() => void perform('write')}>确认运行本次预览</Button><Button className="min-h-11" variant="outline" onClick={() => { setPreview(null); original.current = null; setPhase('idle'); }}>重新读取预览</Button></div>
    </div>}
    {phase === 'unknown' && <div className="space-y-3"><p>存在待确认的原自动化请求。刷新后不自动重新运行；原回执只证明原请求提交。</p><Button className="min-h-11" onClick={() => void perform('lookup')}>查询原自动化回执</Button>{retry && <Button className="min-h-11" variant="outline" onClick={() => void perform('retry')}>确认按原键重试</Button>}</div>}
    {phase === 'conflict' && <Button className="min-h-11" onClick={() => void acknowledge()}>确认拒绝并重读规则</Button>}
    {phase === 'done' && receipt && <div className="space-y-3 text-sm"><p role="status">{action === 'change' ? '已确认原规则变更' : '已确认原运行请求'}</p><p className="break-all">规则：{receipt.rule_id} · 原结果修订 {receipt.result_revision}</p><p>提交时间：{new Date(receipt.committed_at).toLocaleString('zh-CN')}</p><p>不代表当前规则状态或每项业务效果。关闭后可查看原意图和实际文件标签。</p><Button className="min-h-11" onClick={() => void acknowledge()}>确认回执并刷新规则</Button></div>}
    {phase === 'denied' && <p role="alert">操作已停止，预览与结果已清除。权限、网络或恢复身份已变化，请重新进入。</p>}
    {phase === 'unavailable' && <p role="alert">恢复存储或跨标签锁不可用，不能安全登记本次请求；没有发送新的修改。</p>}
  </AppDialog>;
}
