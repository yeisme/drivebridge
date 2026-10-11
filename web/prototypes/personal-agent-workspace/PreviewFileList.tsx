import { Fragment, useLayoutEffect } from "react";
import { FileText, Image, Video, Music2, Sheet } from "lucide-react";
import { Input } from "@/components/ui/input";
import { FileButton } from "./HoverPreview";
import { useDemo } from "./ui";
import { previewUnavailable, refKey } from "./preview-model";
import type { PreviewRef, PreviewSession } from "./preview-model";
export function PreviewFileList({
  session,
  query,
  onQuery,
  onSelect,
  listRef,
  scroll,
}: {
  session: PreviewSession;
  query: string;
  onQuery: (q: string) => void;
  onSelect: (r: PreviewRef) => void;
  listRef: React.RefObject<HTMLDivElement | null>;
  scroll: React.RefObject<number>;
}) {
  const { state: s } = useDemo();
  const items = session.scope.refs
    .map((reference, index) => {
      const file = s.files.find((f) => f.id === reference.id);
      const error = previewUnavailable(s, reference);
      return {
        reference,
        index,
        file,
        error,
        path: `${s.connections.find((c) => c.id === (reference.connectionId || file?.connectionId))?.name || "来源未知"} / ${file?.folder || "位置未知"}`,
      };
    })
    .filter(
      (x) =>
        !query ||
        (x.file?.name || "")
          .toLocaleLowerCase()
          .includes(query.toLocaleLowerCase()),
    );
  const grouped = new Set(items.map((x) => x.path)).size > 1;

  useLayoutEffect(() => {
    if (listRef.current) listRef.current.scrollTop = scroll.current;
  }, []);
  return (
    <div data-preview-list className="flex h-full min-h-0 flex-col bg-card">
      <div className="shrink-0 border-b px-2 pb-2">
        <div className="flex h-9 items-center gap-2 text-[11px]">
          <span
            className="min-w-0 flex-1 truncate font-medium"
            title={`固定本次清单：${session.scope.label}；不自动加入其他文件`}
          >
            来自：{session.scope.label}
          </span>
          <span className="tabular-nums text-muted-foreground">
            {session.scope.refs.length} 项
          </span>
        </div>
        <Input
          aria-label="搜索预览列表"
          placeholder="查找本次清单"
          value={query}
          onChange={(e) => onQuery(e.target.value)}
          className="!h-8 text-xs max-[767px]:!h-11 [@media(pointer:coarse)]:!h-11"
        />
      </div>
      <div
        ref={listRef}
        aria-label="本次预览文件列表"
        className="min-h-0 flex-1 overflow-auto overscroll-contain p-1"
        onScroll={(e) => {
          scroll.current = e.currentTarget.scrollTop;
        }}
      >
        {items.map((item, i) => {
          const { file, reference, index, error, path } = item,
            Icon =
              file?.kind === "sheet"
                ? Sheet
                : file?.kind === "image"
                  ? Image
                  : file?.kind === "video"
                    ? Video
                    : file?.kind === "audio"
                      ? Music2
                      : FileText;
          return (
            <Fragment key={`${refKey(reference)}-${index}`}>
              {grouped && (i === 0 || items[i - 1].path !== path) && (
                <p
                  className="flex h-7 items-center truncate px-2 text-[10px] text-muted-foreground"
                  title={path}
                >
                  {path}
                </p>
              )}
              <FileButton
                reference={reference}
                data-preview-file={reference.id}
                aria-current={index === session.cursor ? "true" : undefined}
                variant="ghost"
                className={`!min-h-0 !w-full !justify-start !gap-2 !rounded-sm !px-2 !font-normal ${s.preferences.density === "compact" ? "!h-8" : "!h-9"} max-[767px]:!h-11 [@media(pointer:coarse)]:!h-11 ${index === session.cursor ? "bg-accent text-primary" : "text-foreground"}`}
                title={
                  error ||
                  `${file?.name} · ${path} · ${reference.externalRevision || `v${reference.version}`} · ${file?.size || "大小未知"}`
                }
                onOpen={() => onSelect(reference)}
              >
                <Icon size={16} className="shrink-0" />
                <span className="min-w-0 flex-1 truncate text-left text-xs">
                  {error ? "文件不可访问" : file?.name || "文件不存在"}
                </span>
                <span className="shrink-0 text-[10px] text-muted-foreground">
                  {error
                    ? "不可用"
                    : reference.externalRevision
                      ? "外部"
                      : `v${reference.version}`}
                </span>
              </FileButton>
            </Fragment>
          );
        })}
        {!items.length && (
          <p className="p-3 text-xs text-muted-foreground">
            本次清单中未找到。
          </p>
        )}
      </div>
    </div>
  );
}
