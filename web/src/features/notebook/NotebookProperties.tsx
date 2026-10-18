import type { NoteIdentity } from "./notebook-state";
import { NotebookRelations } from "./NotebookRelations";
import { NotebookFavorite } from "./NotebookFavorite";

export function NotebookProperties({ identity, actor, fileRelations = false }: { identity?: NoteIdentity; actor?: string; fileRelations?: boolean }) {
  if (!identity) return <div className="m-3 rounded-lg border border-dashed border-border bg-card/50 p-4 text-sm text-muted-foreground">请选择可访问的笔记。</div>;
  return <section aria-label="固定笔记信息" className="space-y-4 bg-card/40 p-4 text-sm">
    <header><p className="m-0 text-[11px] font-medium uppercase tracking-[0.12em] text-muted-foreground">当前对象</p><h2 className="mt-1 text-base font-semibold tracking-tight">笔记信息</h2><p className="mt-1 break-all text-xs text-muted-foreground">{identity.objectId}</p></header>
    {fileRelations && actor && <NotebookFavorite identity={identity} actor={actor}/>}
    <details open className="border-t pt-3">
      <summary className="min-h-10 cursor-pointer py-2 text-xs font-medium text-muted-foreground">固定版本与来源</summary>
      <dl className="mt-2 grid gap-3 break-all text-xs leading-5">
        <div><dt className="text-muted-foreground">来源</dt><dd className="mt-0.5 font-medium">{identity.sourceId}</dd></div>
        <div><dt className="text-muted-foreground">版本</dt><dd className="mt-0.5 font-mono text-[11px]">{identity.revision}</dd></div>
      </dl>
    </details>
    {fileRelations && actor ? <section className="border-t pt-3"><NotebookRelations identity={identity} actor={actor}/></section> : <section aria-label="笔记关系能力" className="border-t pt-3">
      <h3 className="font-semibold">反向链接</h3>
      <p className="mt-1 text-xs leading-5 text-muted-foreground">当前来源暂不支持关系查询；这不表示没有关联笔记。</p>
    </section>}
  </section>;
}
