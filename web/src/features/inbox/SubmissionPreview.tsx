import { useEffect, useRef, useState } from 'react';
import { AppDialog } from '../../components/AppDialog';
import { Button } from '../../components/ui/button';
import { bytes, contextURL, type DriveFile } from '../../api';
import { MediaPreview, mediaKind } from '../../Media';
import type { Submission } from './submission-client';
import { downloadSubmissionFile, readSubmissionFile } from './submission-file';
import { NotebookReadError, readMarkdownFile } from '../notebook/notebook-adapters';

const deniedRead = (error: unknown) => (error instanceof NotebookReadError && error.state === 'forbidden') || [401, 403].includes((error as { status?: number })?.status || 0);

// Reuse fixed-version readers, but never invoke a conversion service or run HTML
// scripts from an unreviewed submission. All formats retain an authorized download.
export function SubmissionPreview({ submission, close }: { submission: Submission; close: () => void }) {
  const [file, setFile] = useState<DriveFile | null>(null), [text, setText] = useState('');
  const [state, setState] = useState<'loading' | 'ready' | 'unavailable' | 'denied'>('loading');
  const [contentError, setContentError] = useState(''), [downloadError, setDownloadError] = useState('');
  const [downloading, setDownloading] = useState(false);
  const lifetime = useRef<AbortController | null>(null);
  useEffect(() => {
    const abort = new AbortController(); lifetime.current = abort;
    setFile(null); setText(''); setContentError(''); setDownloadError(''); setState('loading'); setDownloading(false);
    const invalidate = () => { abort.abort(); setFile(null); setText(''); setContentError(''); setDownloadError(''); setDownloading(false); setState('denied'); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('offline', invalidate);
    async function load() {
      try {
        const fixed = await readSubmissionFile(submission, abort.signal);
        if (abort.signal.aborted) return;
        setFile(fixed);
        const kind = mediaKind(fixed);
        if (kind === 'text' || kind === 'html') {
          try {
            const body = await readMarkdownFile({ sourceId: fixed.space_id, objectId: fixed.id, revision: fixed.version_id }, abort.signal);
            if (abort.signal.aborted) return;
            if (body.includes('\0')) throw new Error('binary_content');
            setText(body);
          } catch (error) {
            if (abort.signal.aborted) return;
            if (deniedRead(error)) throw error;
            setContentError('此内容不是可直接显示的 UTF-8 文本，或超出 2 MiB 阅读预算。可下载原提交版本检查。');
          }
        }
        if (!abort.signal.aborted) setState('ready');
      } catch (error) {
        if (abort.signal.aborted) return;
        setFile(null); setText(''); setState(deniedRead(error) ? 'denied' : 'unavailable');
      }
    }
    if (!navigator.onLine) invalidate(); else void load();
    return () => { abort.abort(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', invalidate); };
  }, [submission]);
  async function download() {
    const abort = lifetime.current;
    if (!abort || abort.signal.aborted || downloading || state !== 'ready') return;
    setDownloading(true); setDownloadError('');
    try { await downloadSubmissionFile(submission, abort.signal); }
    catch (error) {
      if (abort.signal.aborted) return;
      if (deniedRead(error) || (error as { status?: number })?.status === 404) { setFile(null); setText(''); setState('denied'); }
      else setDownloadError('原版本下载未能启动，请重读权限与文件后重试。未改用最新版本。');
    } finally { if (!abort.signal.aborted) setDownloading(false); }
  }
  const kind = file ? mediaKind(file) : 'unsupported';
  const textOnly = kind === 'text' || kind === 'html';
  const canRender = ['image', 'video', 'audio', 'pdf', 'spreadsheet'].includes(kind);
  return <AppDialog title="收件固定版本预览" close={close} className="sm:max-w-5xl max-md:[&_button]:min-h-11 max-md:[&_input]:min-h-11 [&_[data-slot=dialog-close]]:size-11">
    {state !== 'denied' && <div className="min-w-0 space-y-2">
      <h3 className="m-0 break-all text-base font-semibold">{file?.path || submission.file_name || '原提交产物'}</h3>
      <p className="text-[13px] text-muted-foreground">仅检查原提交版本，不自动接收或归档。文本与 HTML 按源码转义显示，不运行文件内脚本；不支持内嵌的格式可下载检查。</p>
      <details className="text-xs text-muted-foreground"><summary className="min-h-11 cursor-pointer py-3 focus-visible:outline-2 focus-visible:outline-ring md:min-h-8 md:py-2">原版本与来源</summary><p className="break-all">提交版本：{submission.version_id}</p>{file && <p>{file.mime_type} · {bytes(file.size_bytes)}</p>}</details>
    </div>}
    {state === 'loading' && <p role="status">读取固定版本…</p>}
    {state === 'ready' && file && <>
      {contentError && <p role="alert" className="text-sm">{contentError}</p>}
      {downloadError && <p role="alert" className="text-sm">{downloadError}</p>}
      {textOnly && !contentError && <pre tabIndex={0} aria-label="收件原版本纯文本" className="max-h-[55dvh] overflow-auto whitespace-pre-wrap break-words rounded border border-border p-3 text-sm">{text || '（空文件）'}</pre>}
      {canRender && <div className="quicklook-stage h-[55dvh] min-h-64 overflow-hidden rounded border border-border [&_.image-stage]:h-full [&_.image-stage]:max-h-none" aria-label="收件原版本媒体"><MediaPreview key={file.id + ':' + file.version_id} file={file} sourceURL={contextURL('/api/v1/files/' + encodeURIComponent(file.id) + '/media?version=' + encodeURIComponent(file.version_id))} compact/></div>}
      {!canRender && !textOnly && <p role="status">此格式无法在浏览器内直接预览。请下载原提交版本检查后再决定；不会自动转码或执行文件。</p>}
      <div className="flex flex-wrap items-center gap-3"><Button variant="outline" className="min-h-11" disabled={downloading} onClick={() => void download()}>{downloading ? '正在核对下载权限…' : '下载原提交版本'}</Button><span className="text-xs text-muted-foreground">下载和预览都不代表人工验收。</span></div>
    </>}
    {state === 'unavailable' && <p role="alert">此固定版本不可用或原引用已变化。未改读最新版本。</p>}
    {state === 'denied' && <p role="alert">读取权限、网络或范围已变化，正文已清除。</p>}
  </AppDialog>;
}
