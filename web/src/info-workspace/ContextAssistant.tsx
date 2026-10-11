import { useEffect, useId, useRef, useState } from 'react';
import { pinnedFile } from '../api';
import { Button } from '../components/ui/button';
import { statFile } from './client';

export function ContextAssistant({ fileId, versionId, onDenied }: {
  fileId: string; versionId: string; onDenied?: () => void;
}) {
  return <FixedContext key={JSON.stringify([fileId, versionId])} fileId={fileId} versionId={versionId} onDenied={onDenied} />;
}

function FixedContext({ fileId, versionId, onDenied }: {
  fileId: string; versionId: string; onDenied?: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [reference, setReference] = useState('');
  const [state, setState] = useState<'idle' | 'loading' | 'ready' | 'copying' | 'error' | 'denied' | 'offline'>('idle');
  const [message, setMessage] = useState('');
  const entry = useRef<HTMLButtonElement>(null);
  const request = useRef<AbortController | null>(null);
  const blocked = useRef(false);
  const denied = useRef(onDenied); denied.current = onDenied;
  const id = useId();
  useEffect(() => {
    const clear = (reason: 'denied' | 'offline') => {
      blocked.current = true; request.current?.abort(); setReference(''); setMessage(''); setState(reason);
    };
    const invalidate = () => clear('denied'), offline = () => clear('offline');
    window.addEventListener('drivebridge-context', invalidate);
    window.addEventListener('offline', offline);
    if (navigator.onLine === false) offline();
    return () => {
      request.current?.abort();
      window.removeEventListener('drivebridge-context', invalidate);
      window.removeEventListener('offline', offline);
    };
  }, []);

  async function resolve(copy: boolean) {
    if (blocked.current || request.current || !fileId || !versionId) return;
    const controller = new AbortController(); request.current = controller;
    setState(copy ? 'copying' : 'loading'); setReference(''); setMessage('');
    try {
      const file = await statFile(fileId, controller.signal);
      if (controller.signal.aborted) return;
      if (file.id !== fileId) throw new Error('fixed_context_changed');
      const fixed = await pinnedFile(file, versionId, {}, controller.signal);
      if (controller.signal.aborted) return;
      if (fixed.file.id !== fileId || fixed.file.space_id !== file.space_id || fixed.file.version_id !== versionId || typeof fixed.ref !== 'string' || !fixed.ref.startsWith('drivebridge://') || !fixed.ref.endsWith(`/file/${fileId}/version/${versionId}`)) throw new Error('fixed_context_changed');
      setReference(fixed.ref); setState('ready');
      if (copy) {
        await navigator.clipboard.writeText(fixed.ref);
        if (!controller.signal.aborted) setMessage('已复制固定版本引用；未发送给任何 Agent。');
      }
    } catch (error) {
      if (controller.signal.aborted) return;
      setReference('');
      if (error && typeof error === 'object' && 'status' in error && (error.status === 401 || error.status === 403)) {
        blocked.current = true; setState('denied'); denied.current?.();
      } else { setState('error'); setMessage(copy ? '复制未完成：原固定版本读取或剪贴板不可用。未改用最新版本。' : '原固定版本不可读或返回身份不匹配；未改用最新版本。'); }
    } finally {
      if (request.current === controller) request.current = null;
    }
  }
  function close() {
    request.current?.abort(); request.current = null;
    setOpen(false); setReference(''); setMessage('');
    if (!blocked.current) setState('idle');
    entry.current?.focus();
  }
  return <div className="mb-4 min-w-0 border-b border-border pb-4"
    onKeyDown={event => { if (open && event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); close(); } }}>
    <Button ref={entry} variant="outline" className="min-h-11" type="button" aria-expanded={open} aria-controls={id}
      onClick={() => { if (open) close(); else { setOpen(true); void resolve(false); } }}>上下文助手</Button>
    {open && <section id={id} aria-label="固定上下文助手" className="mt-3 min-w-0 space-y-3 text-sm">
      <h3 className="font-semibold">内置助手未就绪</h3>
      <p>文件上下文入口已保留；当前未接入已批准的助手 runtime，不会调用模型或自动执行计划。</p>
      <ul className="list-disc space-y-1 pl-5">
        <li>未提供固定会话与 runtime owner 交接合同，不能创建真实助手会话。</li>
        <li>未提供分析同意的服务端记录与撤回合同，打开此入口不表示同意分析。</li>
        <li>未提供预算与费用准入合同，不展示虚构余额或可用额度。</li>
        <li>未提供助手计划引用与确认合同，不能预览或执行助手计划。</li>
      </ul>
      <p>外部路径：复制下方服务端校验的固定版本引用，交给你已连接 DriveBridge 的外部 Agent。引用不是凭据，不扩大权限；外部 Agent 仍需独立授权。</p>
      {(state === 'loading' || state === 'copying') && <p role="status">正在校验原固定版本与当前读取权限…</p>}
      {state === 'denied' && <p role="alert">权限或范围已变化，上下文已清除。请关闭文件详情并重新打开以鉴权。</p>}
      {state === 'offline' && <p role="alert">已离线，上下文已清除。恢复网络后请关闭文件详情并重新打开。</p>}
      {state === 'error' && <p role="alert">{message}</p>}
      {reference && <p className="break-all"><code>{reference}</code></p>}
      {state === 'ready' && <Button variant="outline" type="button" className="min-h-11" onClick={() => void resolve(true)}>复制外部 Agent 固定引用</Button>}
      {state === 'error' && <Button variant="outline" type="button" className="min-h-11" onClick={() => void resolve(false)}>重新校验固定引用</Button>}
      {state === 'ready' && message && <p role="status">{message}</p>}
      <div><Button variant="ghost" type="button" className="min-h-11" onClick={close}>收起上下文助手</Button></div>
    </section>}
  </div>;
}
