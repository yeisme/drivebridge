import { useEffect, useRef, useState } from 'react';
import { Button } from '../../components/ui/button';
import { requestResourceCheck, type ResourceCheckRequest, type ResourceIntegrityReceipt } from './clients/resource-checks';

type Receipt = ResourceIntegrityReceipt;
type Request = ResourceCheckRequest;
export function ResourceIntegrity({ descriptor, revision, close, onDenied }: { descriptor: string; revision: number; close: () => void; onDenied: () => void }) {
  const [receipt, setReceipt] = useState<Receipt | null>(null), [state, setState] = useState('idle'), [confirming, setConfirming] = useState(false);
  const active = useRef<AbortController | null>(null), blocked = useRef(false), heading = useRef<HTMLHeadingElement>(null), denied = useRef(onDenied);
  denied.current = onDenied;
  useEffect(() => {
    blocked.current = false; setReceipt(null); setState('idle'); setConfirming(false); heading.current?.focus();
    const invalidate = () => { blocked.current = true; active.current?.abort(); setReceipt(null); setConfirming(false); setState('denied'); };
    const offline = () => { invalidate(); setState('offline'); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('offline', offline);
    if (!navigator.onLine) offline();
    return () => { blocked.current = true; active.current?.abort(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', offline); };
  }, [descriptor, revision]);
  const request = async (lookup: boolean) => {
    if (blocked.current || active.current || !navigator.onLine) return;
    const controller = new AbortController(); active.current = controller; setReceipt(null); setConfirming(false); setState('loading');
    const input: Request = { expected_revision: revision, checker_version: 'utf8-integrity-v1' };
    try {
      const value = await requestResourceCheck(descriptor, input, lookup, controller.signal);
      if (controller.signal.aborted || blocked.current) return;
      if (!value || typeof value.check_id !== 'string' || !value.check_id || value.check_id.length > 512 || value.descriptor_id !== descriptor || value.descriptor_revision !== revision || value.checker_version !== input.checker_version || value.check_scope !== 'fixed_bytes_sha256_utf8_no_nul' || !['passed', 'failed'].includes(value.state) || typeof value.checked_at !== 'string' || !Number.isFinite(Date.parse(value.checked_at))) throw new Error('invalid_contract');
      setReceipt(value); setState('ready');
    } catch (error) {
      if (controller.signal.aborted || blocked.current) return;
      const status = (error as { status?: number }).status;
      if (status === 401 || status === 403) { blocked.current = true; setState('denied'); denied.current(); }
      else setState(lookup && status === 404 ? 'missing' : status === 409 ? 'stale' : 'unknown');
    } finally { if (active.current === controller) active.current = null; }
  };
  const canExecute = !blocked.current && (state === 'idle' || state === 'missing');
  return <section aria-label="固定字节编码检查" className="space-y-4 rounded-xl border border-border bg-card/40 p-4">
    <header className="flex flex-wrap items-start justify-between gap-3 border-b border-border pb-3"><div><p className="m-0 text-[11px] font-medium uppercase tracking-[0.12em] text-muted-foreground">检查回执</p><h3 ref={heading} tabIndex={-1} className="mt-1 font-semibold">固定字节编码检查</h3></div><Button variant="outline" className="min-h-11" onClick={close}>关闭编码检查</Button></header>
    <div className="space-y-2 rounded-lg border border-border bg-background/70 p-3"><p className="text-sm">仅检查原固定版本的字节长度、SHA-256、UTF-8 和无 NUL；最多 100 个文件、累计 2 MiB。不执行 Skill、模板或脚本，不抓取内容中的外部链接。</p><p className="text-sm text-muted-foreground">通过不代表 Skill 格式合规、内容安全、可安装或获准执行。回执是历史事实，不证明当前存储健康；资源总览仍为未检查。</p></div>
    <p className="break-all text-xs text-muted-foreground">描述：{descriptor} · 修订 {revision} · utf8-integrity-v1</p>
    <div className="flex flex-wrap gap-2"><Button variant="outline" className="min-h-11" disabled={state === 'loading' || blocked.current} onClick={() => void request(true)}>查询原编码检查回执</Button>
    {canExecute && !confirming && <Button variant="outline" className="min-h-11" onClick={() => setConfirming(true)}>准备检查固定字节</Button>}</div>
    {confirming && canExecute && <div className="space-y-3 rounded-lg border border-primary/20 bg-primary/5 p-3"><p>确认后服务端将读取当前描述绑定的固定字节；若已有相同修订与检查器的回执，仅返回历史结果，不重新检查。</p><div className="flex flex-wrap gap-2"><Button className="min-h-11" onClick={() => void request(false)}>确认检查固定字节</Button><Button variant="outline" className="min-h-11" onClick={() => setConfirming(false)}>取消检查</Button></div></div>}
    {state === 'loading' && <p role="status">等待原检查结果…</p>}
    {state === 'missing' && <p role="alert">未查到可读取的原回执，不代表从未执行或检查失败。请核对原目标；再次确认可能首次执行检查。</p>}
    {state === 'unknown' && <p role="alert">结果未知或响应不匹配，不自动重试检查。请查询原回执；关闭或刷新后仍可按原描述修订查询。</p>}
    {state === 'stale' && <p role="alert">描述或固定目标已变化，不能复用原结论。请关闭并刷新资源列表。</p>}
    {state === 'denied' && <p role="alert">权限或范围变化，检查结果已清除。</p>}
    {state === 'offline' && <p role="alert">已离线，检查结果已清除；联网不会自动读取，请关闭后重新打开。</p>}
    {receipt && <div role="status" className={`space-y-2 rounded-lg border p-3 ${receipt.state === 'passed' ? 'border-emerald-500/30 bg-emerald-500/5' : 'border-destructive/30 bg-destructive/5'}`}><p className="font-medium">{receipt.state === 'passed' ? '历史编码与字节完整性检查通过' : '历史编码检查未通过（UTF-8 或 NUL 条件不满足）'}</p><p className="break-all text-xs text-muted-foreground">原检查标识：{receipt.check_id}</p><p className="text-xs text-muted-foreground">原检查时间：{receipt.checked_at}</p></div>}
  </section>;
}
