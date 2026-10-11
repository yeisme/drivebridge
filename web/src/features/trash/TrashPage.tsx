import type { Dispatch, SetStateAction } from "react";
import { ChevronRight, Folder, LayoutGrid, List, RotateCcw, Trash2 } from "lucide-react";
import { bytes, type DriveFile, type Space } from "../../api";
import { MediaIcon } from "../../Media";
import { Button } from "../../components/ui/button";
import { Checkbox } from "../../components/ui/checkbox";
import { SelectAdapter } from "../../components/SelectAdapter";
import { SearchField } from "../../components/SearchField";
import { WorkspacePageHeader } from "../../components/WorkspacePageHeader";
import { TrashHolds } from "./TrashHolds";

interface Props {
  readState: 'loading' | 'ready' | 'error' | 'unavailable';
  onReload: () => void;
  spaces: Space[];
  space: string;
  retention: number;
  backend?: { label: string; description: string; unrecognized?: boolean };
  search: string;
  prefix: string;
  grid: boolean;
  busy: boolean;
  restricted: boolean;
  count: number;
  folders: string[];
  visible: DriveFile[];
  restorableCount: number;
  picked: Record<string, boolean>;
  setPicked: Dispatch<SetStateAction<Record<string, boolean>>>;
  onSpace: (id: string) => void;
  onSearch: (value: string) => void;
  onPrefix: (value: string) => void;
  onGrid: (value: boolean) => void;
  onRetention: (days: number) => void;
  onRestoreSelected: () => void;
  onRestore: (file: DriveFile) => void;
  onPreview: (file: DriveFile) => void;
}

// Restore and retention changes remain owner operations; holds use the authorized management contract.
export function TrashPage(p: Props) {
  return <section aria-label="回收站" className="min-w-0 space-y-4">
    <WorkspacePageHeader title="回收站" description="恢复原文件；当前工作区未启用手动永久删除。" actions={<Button variant="outline" type="button" className="min-h-11 md:min-h-8" disabled={p.busy || p.readState === 'loading' || p.readState === 'unavailable'} onClick={p.onReload}>重读回收站</Button>}/>
    <div className="grid min-w-0 gap-3 sm:grid-cols-2">
      <label className="grid min-w-0 gap-2 text-[13px] text-muted-foreground">空间<SelectAdapter className="min-h-11 w-full max-w-full md:min-h-8" aria-label="选择空间" value={p.space} onChange={e => p.onSpace(e.target.value)}>{p.spaces.map(s => <option key={s.id} value={s.id}>{s.name}（{bytes(Math.max(0, s.max_bytes - s.used_bytes - s.reserved_bytes))} 可用）</option>)}</SelectAdapter></label>
      <div className="flex min-w-0 items-end gap-2 [&_[data-slot=input-group]]:min-h-11 [&_[data-slot=input-group]]:flex-1 md:[&_[data-slot=input-group]]:min-h-8">
        <SearchField aria-label="搜索文件" placeholder="搜索回收站文件…" value={p.search} onChange={e => p.onSearch(e.target.value)} />
        <div className="flex shrink-0 gap-1" role="group" aria-label="回收站视图"><Button variant={!p.grid ? "secondary" : "ghost"} className="size-11 p-0 md:size-8" type="button" aria-label="列表视图" aria-pressed={!p.grid} onClick={() => p.onGrid(false)}><List size={18}/></Button><Button variant={p.grid ? "secondary" : "ghost"} className="size-11 p-0 md:size-8" type="button" aria-label="网格视图" aria-pressed={p.grid} onClick={() => p.onGrid(true)}><LayoutGrid size={18}/></Button></div>
      </div>
    </div>
    <div className="flex flex-wrap items-center gap-2">
      <label className="flex items-center gap-2 text-[13px] text-muted-foreground">自动清理<SelectAdapter className="min-h-11 md:min-h-8" aria-label="回收站自动清理天数" value={String(p.retention)} disabled={p.restricted || p.busy} onChange={e => p.onRetention(Number(e.target.value))}>{[7, 14, 30, 90, 365].map(days => <option key={days} value={days}>{days} 天</option>)}</SelectAdapter></label>
      <Button variant="outline" className="min-h-11 md:min-h-8" type="button" disabled={p.readState !== 'ready' || p.busy || p.restricted || p.restorableCount === 0} onClick={p.onRestoreSelected}>恢复所选</Button>
      <Button variant="outline" className="min-h-11 md:min-h-8" type="button" disabled title="当前工作区未启用永久删除">永久删除所选</Button>
      {p.backend && <span className="text-xs text-muted-foreground" title={p.backend.description}>{p.backend.label}</span>}
    </div>
    {p.readState === 'loading' && <p role="status">正在读取回收站…</p>}
    {p.readState === 'error' && <p role="alert">无法读取回收站，旧记录和选择已清除。请手动重读；此状态不代表回收站为空。</p>}
    {p.readState === 'unavailable' && <p role="status">请选择当前可访问的空间后读取回收站。</p>}
    {p.readState === 'ready' && <>
      <div className="flex min-w-0 flex-wrap items-center gap-2 text-[13px]"><Button variant="ghost" type="button" className="min-h-11 md:min-h-8" onClick={() => p.onPrefix("")}>全部文件</Button>{p.prefix && <><ChevronRight className="size-4 shrink-0"/><Button variant="ghost" type="button" className="h-auto min-h-11 min-w-0 max-w-full whitespace-normal break-all text-left md:min-h-8" onClick={() => p.onPrefix(p.prefix.includes("/") ? p.prefix.slice(0, p.prefix.lastIndexOf("/")) : "")}>{p.prefix}</Button></>}<span className="text-xs text-muted-foreground">{p.count} 项</span></div>
      {p.count > 0 && <>
        <label className="flex min-h-11 w-fit items-center gap-3 text-[13px] md:min-h-8"><Checkbox aria-label="全选回收站文件" checked={p.visible.length > 0 && p.visible.every(f => p.picked[f.id])} onCheckedChange={on => p.setPicked(old => { const next = { ...old }; for (const f of p.visible) next[f.id] = on; return next; })}/><span aria-hidden="true">选择当前已显示文件</span></label>
        <div className={p.grid ? "grid min-w-0 gap-3 sm:grid-cols-2 xl:grid-cols-3" : "min-w-0 divide-y divide-border rounded-lg border border-border bg-card"}>
          {!p.search && p.folders.map(name => <Button variant="ghost" type="button" className="h-auto min-h-11 w-full justify-start gap-3 whitespace-normal break-all px-3 py-2 text-left" key={name} onClick={() => p.onPrefix((p.prefix ? p.prefix + "/" : "") + name)}><Folder className="size-5 shrink-0"/><span className="min-w-0 flex-1">{name}</span><ChevronRight className="size-4 shrink-0"/></Button>)}
          {p.visible.map(f => <article key={f.id} className={`flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 p-2 ${p.grid ? 'rounded-lg border border-border bg-card' : ''}`}>
            <label className="flex size-11 shrink-0 items-center justify-center md:size-8"><Checkbox aria-label={"选择 " + f.path} checked={!!p.picked[f.id]} onCheckedChange={on => p.setPicked(old => ({ ...old, [f.id]: on }))}/></label>
            <Button variant="ghost" type="button" className="h-auto min-h-11 min-w-0 flex-[1_1_10rem] justify-start gap-2 whitespace-normal px-1 py-2 text-left md:min-h-8" onClick={() => p.onPreview(f)}>
              <span className="shrink-0 text-muted-foreground"><MediaIcon file={f}/></span>
              <span className="min-w-0"><strong className="line-clamp-2 break-all text-[13px] font-medium" title={f.path}>{p.search ? f.path : f.path.split('/').pop()}</strong><small className="block break-all text-xs text-muted-foreground">{f.mime_type.split(';')[0]}{f.caller ? ' · 来自 ' + f.caller : ''}</small></span>
            </Button>
            <div className="flex w-full flex-wrap items-center justify-between gap-2 pl-11 text-xs text-muted-foreground md:w-auto md:pl-0">
              <span className="flex flex-wrap gap-x-3"><time>{f.purge_after ? new Date(f.purge_after).toLocaleDateString('zh-CN') + ' 前清理' : new Date(f.updated_at).toLocaleDateString('zh-CN')}</time><span>{bytes(f.size_bytes)}</span></span>
              <div className="flex gap-1"><Button variant="ghost" size="icon" type="button" className="size-11 md:size-8" aria-label={"恢复 " + f.path} disabled={p.restricted || p.busy} onClick={() => p.onRestore(f)}><RotateCcw size={17}/></Button><Button variant="ghost" size="icon" type="button" className="size-11 md:size-8" aria-label={"永久删除 " + f.path} disabled title="当前工作区未启用永久删除"><Trash2 size={17}/></Button></div>
            </div>
            <div className="w-full pl-11 md:pl-0"><TrashHolds file={f} restricted={p.restricted || p.busy}/></div>
          </article>)}
        </div>
      </>}
      {!p.count && <div className="space-y-2 border-y border-border py-8 text-sm"><h3 className="m-0 font-semibold">{p.search ? '没有匹配的文件' : '回收站是空的'}</h3><p className="text-[13px] leading-5 text-muted-foreground">{p.search ? '试试其他关键词。' : '可恢复回收站中的文件。当前工作区未启用手动永久删除；服务端保留策略由管理员管理。'}</p></div>}
    </>}
  </section>;
}
