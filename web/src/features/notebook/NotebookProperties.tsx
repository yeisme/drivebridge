import type { NoteIdentity } from "./notebook-state";
import { NotebookRelations } from "./NotebookRelations";
import { NotebookFavorite } from "./NotebookFavorite";

export function NotebookProperties({ identity, actor, fileRelations = false }: { identity?: NoteIdentity; actor?: string; fileRelations?: boolean }) {
  if (!identity) return <p className="p-3 text-sm">请选择可访问的笔记。</p>;
  return <section aria-label="固定笔记信息" className="space-y-4 p-3 text-sm">
    <h2 className="font-semibold">笔记信息</h2>
    {fileRelations && actor && <NotebookFavorite identity={identity} actor={actor}/>}
    <details>
      <summary className="min-h-11 cursor-pointer py-3 text-xs text-muted-foreground">固定版本与来源</summary>
      <dl className="space-y-2 break-all text-xs leading-5">
        <dt className="text-muted-foreground">来源</dt><dd>{identity.sourceId}</dd>
        <dt className="text-muted-foreground">对象</dt><dd>{identity.objectId}</dd>
        <dt className="text-muted-foreground">Revision</dt><dd>{identity.revision}</dd>
      </dl>
    </details>
    {fileRelations && actor ? <NotebookRelations identity={identity} actor={actor}/> : <section aria-label="笔记关系能力">
      <h3 className="font-semibold">反向链接</h3>
      <p className="text-muted-foreground">当前来源暂不支持关系查询；这不表示没有关联笔记。</p>
    </section>}
  </section>;
}
