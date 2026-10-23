import { useEffect, useRef, useState } from 'react';
import { ChevronDown, ChevronRight, Folder } from 'lucide-react';
import { Button } from '../../components/ui/button';
import { readNotebookDirectory } from './clients';

type LoadState = 'idle' | 'loading' | 'ready' | 'error' | 'offline' | 'forbidden';

function label(path: string) {
  return path ? path.split('/').at(-1) || path : '网盘根目录';
}

export function NotebookDirectoryTree({ space, selected, onSelect, reloadToken = 0 }: {
  space: string;
  selected: string;
  onSelect: (path: string) => void;
  reloadToken?: number;
}) {
  const [open, setOpen] = useState<Set<string>>(new Set(['']));
  const [folders, setFolders] = useState<Map<string, string[]>>(new Map());
  const [states, setStates] = useState<Map<string, LoadState>>(new Map());
  const controllers = useRef(new Set<AbortController>());

  useEffect(() => {
    setOpen(new Set(['']));
    setFolders(new Map());
    setStates(new Map());
  }, [space, reloadToken]);

  useEffect(() => {
    const controller = new AbortController();
    controllers.current.add(controller);
    const invalidate = () => { for (const pending of controllers.current) pending.abort(); setFolders(new Map()); setStates(new Map([['', 'forbidden']])); };
    const offline = () => { for (const pending of controllers.current) pending.abort(); setFolders(new Map()); setStates(new Map([['', 'offline']])); };
    window.addEventListener('drivebridge-context', invalidate);
    window.addEventListener('offline', offline);
    void load('', controller.signal).finally(() => controllers.current.delete(controller));
    return () => { for (const pending of controllers.current) pending.abort(); controllers.current.clear(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', offline); };
    // The source identity is the only request scope; child folders load on demand.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [space, reloadToken]);

  async function load(path: string, signal: AbortSignal) {
    if (!navigator.onLine) { setStates(current => new Map(current).set(path, 'offline')); return; }
    setStates(current => new Map(current).set(path, 'loading'));
    try {
      const result = await readNotebookDirectory(space, path, signal);
      if (signal.aborted) return;
      const children = [...new Set(result.entries.filter(entry => entry.kind === 'directory').map(entry => entry.path))].sort((a, b) => a.localeCompare(b));
      setFolders(current => new Map(current).set(path, children));
      setStates(current => new Map(current).set(path, 'ready'));
    } catch (error) {
      if (signal.aborted) return;
      const status = error && typeof error === 'object' && 'status' in error ? error.status : 0;
      setStates(current => new Map(current).set(path, status === 401 || status === 403 ? 'forbidden' : 'error'));
    }
  }

  function request(path: string) {
    const controller = new AbortController();
    controllers.current.add(controller);
    void load(path, controller.signal).finally(() => controllers.current.delete(controller));
  }

  const toggle = (path: string) => {
    const next = new Set(open);
    if (next.has(path)) next.delete(path);
    else {
      next.add(path);
      if (!folders.has(path)) request(path);
    }
    setOpen(next);
  };

  const render = (parent: string, depth: number): React.ReactNode => {
    const children = folders.get(parent) ?? [];
    return children.map(path => {
      const expanded = open.has(path);
      const state = states.get(path) ?? 'idle';
      return <li key={path} className="min-w-0">
        <div className="flex min-w-0 items-center gap-1" style={{ paddingLeft: `${depth * 12}px` }}>
          <Button type="button" variant="ghost" size="icon" className="size-11 shrink-0 md:size-8" aria-label={`${expanded ? '收起' : '展开'} ${label(path)}`} aria-expanded={expanded} onClick={() => toggle(path)}>
            {expanded ? <ChevronDown className="size-3.5"/> : <ChevronRight className="size-3.5"/>}
          </Button>
          <Button type="button" variant="ghost" className="min-h-11 min-w-0 flex-1 justify-start gap-2 truncate px-2 text-left text-xs md:min-h-8" aria-pressed={selected === path} onClick={() => onSelect(path)} title={path}>
            <Folder className="size-4 shrink-0 text-primary"/><span className="truncate">{label(path)}</span>
            {state === 'loading' && <span className="ml-auto text-[11px] text-muted-foreground">读取中</span>}
          </Button>
        </div>
        {state === 'error' && expanded && <p role="alert" className="m-0 px-2 py-1 text-xs text-destructive" style={{ paddingLeft: `${(depth + 1) * 12 + 40}px` }}>目录暂不可用，请重试。</p>}
        {state === 'forbidden' && expanded && <p role="alert" className="m-0 px-2 py-1 text-xs text-destructive" style={{ paddingLeft: `${(depth + 1) * 12 + 40}px` }}>目录权限已失效。</p>}
        {state === 'offline' && expanded && <p role="status" className="m-0 px-2 py-1 text-xs text-muted-foreground" style={{ paddingLeft: `${(depth + 1) * 12 + 40}px` }}>当前离线。</p>}
        {expanded && <ul className="m-0 list-none p-0">{render(path, depth + 1)}</ul>}
      </li>;
    });
  };

  const rootState = states.get('') ?? 'idle';
  return <section aria-label="网盘目录树" className="border-t px-3 py-2">
    <div className="flex items-center gap-2">
      <Button type="button" variant="ghost" className="min-h-11 min-w-0 flex-1 justify-start gap-2 px-2 text-xs md:min-h-8" aria-pressed={selected === ''} onClick={() => onSelect('')}><Folder className="size-4 shrink-0 text-primary"/>网盘根目录</Button>
      {rootState === 'loading' && <span role="status" className="text-[11px] text-muted-foreground">读取中</span>}
      {rootState !== 'loading' && rootState !== 'ready' && <Button type="button" variant="ghost" className="min-h-11 text-xs md:min-h-8" onClick={() => request('')}>重试</Button>}
    </div>
    {rootState === 'error' && <p role="alert" className="m-0 px-2 py-1 text-xs text-destructive">目录暂不可用，请重试。</p>}
    {rootState === 'forbidden' && <p role="alert" className="m-0 px-2 py-1 text-xs text-destructive">目录权限已失效。</p>}
    {rootState === 'offline' && <p role="status" className="m-0 px-2 py-1 text-xs text-muted-foreground">当前离线，目录树已清除。</p>}
    {rootState === 'ready' && !folders.get('')?.length && <p role="status" className="m-0 px-2 py-1 text-xs text-muted-foreground">当前没有子目录。</p>}
    <ul className="m-0 list-none p-0">{render('', 1)}</ul>
    {selected && <p className="m-0 px-2 pt-2 text-[11px] text-muted-foreground">当前目录：{selected}</p>}
  </section>;
}
