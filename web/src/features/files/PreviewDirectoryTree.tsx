import { useEffect, useState } from 'react';
import { ChevronDown, ChevronRight, File, Folder, RefreshCw, Search } from 'lucide-react';
import { api, errorText, type DriveFile } from '../../api';
import { Button } from '../../components/ui/button';
import { InputGroup, InputGroupAddon, InputGroupInput } from '../../components/ui/input-group';
import type { BrowserEntry, DirectoryPage } from './useDirectoryRead';
import { previewKey } from './preview-state';

async function readDirectory(space: string, path: string, query: string, signal: AbortSignal) {
  const entries: BrowserEntry[] = [];
  let cursor = '';
  do {
    const page = await api<DirectoryPage>(`/browser?${new URLSearchParams({ space_id: space, path, q: query, cursor })}`, 'GET', undefined, {}, signal);
    entries.push(...page.entries);
    cursor = page.next_cursor;
  } while (cursor && !signal.aborted);
  return entries;
}

function TreeBranch({
  space,
  path,
  expanded,
  expandedPaths,
  selectedKey,
  onToggle,
  onOpenFile,
}: {
  space: string;
  path: string;
  expanded: boolean;
  expandedPaths: string[];
  selectedKey?: string;
  onToggle: (path: string, expanded: boolean) => void;
  onOpenFile: (file: DriveFile) => void;
}) {
  const [entries, setEntries] = useState<BrowserEntry[]>();
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const reload = () => {
    const controller = new AbortController();
    setLoading(true);
    setError('');
    void readDirectory(space, path, '', controller.signal)
      .then(rows => { if (!controller.signal.aborted) setEntries(rows); })
      .catch(errorValue => { if (!controller.signal.aborted) setError(errorText(errorValue)); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  };
  useEffect(() => expanded ? reload() : undefined, [expanded, path, space]);
  const directories = (entries || []).filter(entry => entry.kind === 'directory');
  const files = (entries || []).filter(entry => entry.kind === 'file' && entry.file);
  return <li className="preview-tree-branch">
    <div className={`preview-tree-folder ${path === '' ? 'root' : ''}`}>
      <Button type="button" variant="ghost" size="icon" className="size-7 shrink-0" aria-label={`${expanded ? '收起' : '展开'} ${path || '根目录'}`} aria-expanded={expanded} onClick={() => onToggle(path, !expanded)}>
        {expanded ? <ChevronDown size={14}/> : <ChevronRight size={14}/>} 
      </Button>
      <Button type="button" variant="ghost" className="min-w-0 flex-1 justify-start gap-2 px-2 py-1.5 text-left text-xs" onClick={() => onToggle(path, !expanded)}>
        <Folder size={15}/><span className="truncate">{path ? path.split('/').at(-1) : '全部文件'}</span>
      </Button>
      {expanded && <Button type="button" variant="ghost" size="icon" className="size-7 shrink-0" aria-label={`刷新 ${path || '根目录'}`} onClick={reload}><RefreshCw size={13}/></Button>}
    </div>
    {expanded && <ul className="preview-tree-children">
      {loading && <li className="preview-tree-status" role="status">正在读取…</li>}
      {error && <li className="preview-tree-status error" role="alert">{error}</li>}
      {!loading && !error && !directories.length && !files.length && <li className="preview-tree-status">目录为空</li>}
      {directories.map(entry => <TreeBranch key={entry.path} space={space} path={entry.path} expanded={expandedPaths.includes(entry.path)} expandedPaths={expandedPaths} selectedKey={selectedKey} onToggle={onToggle} onOpenFile={onOpenFile}/>) }
      {files.map(entry => {
        const file = entry.file!;
        return <li key={file.id + ':' + file.version_id}>
          <Button type="button" variant="ghost" className={`preview-tree-file ${selectedKey === previewKey(file) ? 'selected' : ''}`} aria-current={selectedKey === previewKey(file) ? 'true' : undefined} title={file.path} onClick={() => onOpenFile(file)}>
            <File size={14}/><span className="min-w-0 flex-1 truncate">{file.path.split('/').at(-1) || file.path}</span>
          </Button>
        </li>;
      })}
    </ul>}
  </li>;
}

export function PreviewDirectoryTree({ space, selectedKey, expandedPaths, onToggle, onOpenFile }: {
  space: string;
  selectedKey?: string;
  expandedPaths: string[];
  onToggle: (path: string, expanded: boolean) => void;
  onOpenFile: (file: DriveFile) => void;
}) {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<DriveFile[]>();
  const [searchError, setSearchError] = useState('');
  useEffect(() => {
    if (!query.trim()) { setResults(undefined); setSearchError(''); return; }
    const controller = new AbortController();
    const timer = setTimeout(() => {
      void readDirectory(space, '', query.trim(), controller.signal)
        .then(rows => setResults(rows.flatMap(row => row.file ? [row.file] : [])))
        .catch(errorValue => { if (!controller.signal.aborted) setSearchError(errorText(errorValue)); });
    }, 180);
    return () => { clearTimeout(timer); controller.abort(); };
  }, [query, space]);
  const rootExpanded = expandedPaths.includes('');
  return <div className="preview-directory-tree">
    <InputGroup className="h-8 shrink-0">
      <InputGroupAddon><Search size={14}/></InputGroupAddon>
      <InputGroupInput aria-label="搜索工作区文件" placeholder="搜索文件或路径" value={query} onChange={event => setQuery(event.target.value)} />
    </InputGroup>
    {query.trim() ? <ul className="preview-tree-results" aria-label="文件搜索结果">
      {searchError && <li className="preview-tree-status error" role="alert">{searchError}</li>}
      {!searchError && results === undefined && <li className="preview-tree-status" role="status">正在搜索…</li>}
      {!searchError && results?.length === 0 && <li className="preview-tree-status">没有匹配文件</li>}
      {results?.map(file => <li key={file.id + ':' + file.version_id}><Button type="button" variant="ghost" className={`preview-tree-file ${selectedKey === previewKey(file) ? 'selected' : ''}`} onClick={() => onOpenFile(file)}><File size={14}/><span className="min-w-0 flex-1 truncate">{file.path}</span></Button></li>)}
    </ul> : <ul className="preview-tree-root" aria-label="workspace 目录树">
      <TreeBranch space={space} path="" expanded={rootExpanded} expandedPaths={expandedPaths} selectedKey={selectedKey} onToggle={onToggle} onOpenFile={onOpenFile}/>
    </ul>}
  </div>;
}
