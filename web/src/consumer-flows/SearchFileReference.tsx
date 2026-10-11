import { useEffect, useRef, useState } from 'react';
import { pinnedFile, type DriveFile } from '../api';
import { statFile } from '../info-workspace/client';
import { Button } from '../components/ui/button';
import { HomeFileText } from '../features/home/HomeFileText';

// Search identities are observations, not grants. Resolve the original version
// again and keep metadata admission separate from explicit body consumption.
export function SearchFileReference({ file, version, close, onDenied, sourceLabel = '搜索' }: {
  file: string; version: string; close: () => void; onDenied: () => void; sourceLabel?: '搜索' | '资料包成员';
}) {
  const [source, setSource] = useState<{ file: DriveFile; ref: string } | null>(null);
  const [state, setState] = useState('loading');
  const [reading, setReading] = useState(false);
  const heading = useRef<HTMLHeadingElement>(null), denied = useRef(onDenied);
  denied.current = onDenied;
  useEffect(() => {
    const controller = new AbortController();
    heading.current?.focus(); setSource(null); setReading(false); setState('loading');
    const invalidate = () => { controller.abort(); setSource(null); setReading(false); setState('denied'); };
    const offline = () => { invalidate(); setState('offline'); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('offline', offline);
    if (!navigator.onLine) offline();
    else void (async () => {
      const current = await statFile(file, controller.signal);
      if (controller.signal.aborted) return;
      if (current.id !== file || current.trashed || !current.space_id) throw new Error('source_changed');
      const fixed = await pinnedFile(current, version, {}, controller.signal);
      if (controller.signal.aborted) return;
      if (fixed.file.id !== file || fixed.file.space_id !== current.space_id || fixed.file.version_id !== version || typeof fixed.ref !== 'string' || !fixed.ref.endsWith('/file/' + file + '/version/' + version)) throw new Error('fixed_source_changed');
      setSource(fixed); setState('ready');
    })().catch((error: { status?: number }) => {
      if (controller.signal.aborted) return;
      setSource(null); setReading(false); setState('error');
      if (error.status === 401 || error.status === 403) denied.current();
    });
    return () => { controller.abort(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', offline); };
  }, [file, version]);
  return <section aria-label={`${sourceLabel}结果固定引用`} className="my-4 space-y-3 rounded-lg border border-border p-4">
    <header className="flex flex-wrap items-center gap-3"><h3 ref={heading} tabIndex={-1} className="m-0 font-semibold">{sourceLabel}结果原版本</h3><Button variant="outline" className="min-h-11" onClick={close}>关闭{sourceLabel}结果引用</Button></header>
    {state === 'loading' && <p role="status">重新鉴权并核对{sourceLabel}原版本…</p>}
    {state === 'error' && <p role="alert">原{sourceLabel}版本不可用或来源不匹配，未改读最新版本。</p>}
    {state === 'denied' && <p role="alert">权限或范围已变化，固定引用已清除。</p>}
    {state === 'offline' && <p role="alert">已离线，固定引用已清除；联网不会自动重读。</p>}
    {source && <><p className="break-all">{source.file.path}</p><p className="break-all">{source.ref}</p>
      <p className="text-sm text-muted-foreground">已重新核对原文件与版本。元数据读取不自动读取正文；不解析 Pinax 或内容候选引用。</p>
      <Button variant="outline" className="min-h-11" aria-expanded={reading} onClick={() => setReading(value => !value)}>{reading ? `关闭${sourceLabel}原版本正文` : `读取${sourceLabel}原版本正文`}</Button>
      {reading && <HomeFileText space={source.file.space_id} file={file} version={version} onDenied={() => { setSource(null); setReading(false); denied.current(); }} label={`${sourceLabel}原版本纯文本`}/>}
    </>}
  </section>;
}
