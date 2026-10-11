import { useEffect, useRef, useState } from 'react';
import { api } from '../../api';
import { Button } from '../../components/ui/button';
import type { components } from '../../generated/personal-workspace';

type Detail = components['schemas']['OperationTaskDetailEnvelope']['data'];
type Task = Detail['task'];
const outputLabels: Record<Detail['output_state'], string> = {
  available: '固定输出引用可用', unavailable: '固定输出当前不可用',
  not_published: '原操作尚未发布输出', not_projected: '此操作类型尚未提供输出投影',
};
export function OperationTaskDetail({ task, close, onDenied }: { task: Task; close: () => void; onDenied: () => void }) {
  const [detail, setDetail] = useState<Detail | null>(null);
  const [state, setState] = useState<'loading' | 'ready' | 'error' | 'missing' | 'denied'>('loading');
  const [attempt, setAttempt] = useState(0);
  const denied = useRef(onDenied); denied.current = onDenied;
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => { heading.current?.focus(); }, []);
  useEffect(() => {
    const controller = new AbortController(); setDetail(null); setState('loading');
    const invalidate = () => { controller.abort(); setDetail(null); setState('denied'); };
    window.addEventListener('drivebridge-context', invalidate);
    api<Detail>(`/workspace/tasks/operation/${encodeURIComponent(task.owner_id)}`, 'GET', undefined, {}, controller.signal).then(value => {
      if (controller.signal.aborted) return;
      if (!value?.task || value.task.owner_kind !== 'operation' || value.task.owner_id !== task.owner_id || value.task.space_id !== task.space_id || value.task.actor_kind !== task.actor_kind || value.task.agent_id !== task.agent_id || typeof value.task.owner_status !== 'string' || typeof value.task.revision !== 'string' || !Object.hasOwn(outputLabels, value.output_state) || !Array.isArray(value.outputs) || value.outputs.length > 1) throw new Error('invalid_contract');
      if (value.output_state === 'available' ? value.outputs.length !== 1 || value.task.owner_status !== 'completed' : value.outputs.length !== 0) throw new Error('invalid_contract');
      if (!value.outputs.every(output => output.space_id === task.space_id && typeof output.file_id === 'string' && !!output.file_id && typeof output.version_id === 'string' && !!output.version_id)) throw new Error('invalid_contract');
      setDetail(value); setState('ready');
    }).catch((error: { status?: number }) => {
      if (controller.signal.aborted) return;
      setDetail(null);
      if (error.status === 401 || error.status === 403) { setState('denied'); denied.current(); }
      else setState(error.status === 404 ? 'missing' : 'error');
    });
    return () => { controller.abort(); window.removeEventListener('drivebridge-context', invalidate); };
  }, [task.owner_id, task.space_id, task.agent_id, task.actor_kind, attempt]);
  return <section aria-label="原操作详情" className="min-w-0 space-y-3">
    <header className="flex flex-wrap items-center gap-3"><h3 ref={heading} tabIndex={-1} className="m-0 text-base font-semibold">原操作详情</h3><Button variant="outline" className="min-h-11" onClick={close}>关闭操作详情</Button></header>
    <p className="text-sm">仅查询原操作元数据；没有读取正文，也未执行或恢复任务。操作完成不代表收件已验收。引用使用时仍需重新鉴权。</p>
    {state === 'loading' && <p role="status">正在读取原操作详情…</p>}
    {state === 'denied' && <p role="alert">详情读取权限或范围已变化，输出已清除。</p>}
    {state === 'missing' && <p role="alert">原操作详情不可见或服务尚未提供此入口，未改查其他操作。</p>}
    {state === 'error' && <p role="alert">原操作详情读取失败，未将失败当作没有输出。</p>}
    {(state === 'error' || state === 'missing' || state === 'ready') && <Button variant="outline" className="min-h-11" onClick={() => setAttempt(value => value + 1)}>重读原操作详情</Button>}
    {state === 'ready' && detail && <>
      <p className="break-all text-sm">原操作标识：operation:{detail.task.owner_id}</p>
      <p className="text-sm">当前原状态：{detail.task.owner_status}</p>
      <p role="status">{outputLabels[detail.output_state]}</p>
      <p className="text-xs text-muted-foreground">这是本次授权下的输出观察，不是新操作回执；任务变更标识不代表整个详情快照。</p>
      {detail.outputs.map(output => <dl key={output.file_id + ':' + output.version_id} aria-label="固定输出引用" className="space-y-1 break-all text-sm"><dt>文件标识</dt><dd>{output.file_id}</dd><dt>固定版本</dt><dd>{output.version_id}</dd><dt>空间标识</dt><dd>{output.space_id}</dd></dl>)}
    </>}
  </section>;
}
