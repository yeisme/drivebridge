import { useState } from "react";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "./Dialog";
import {
  ArrowLeft,
  ArrowRight,
  ArrowUp,
  MoreHorizontal,
  RefreshCw,
  Locate,
  Copy,
} from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Btn, IconBtn, useDemo } from "./ui";
import { entryRef, bookmarkFor } from "./organization-model";
import { browseHistory, browseLocation } from "./browse-model";
export function BrowseControls({ more = false }: { more?: boolean }) {
  const [manual, setManual] = useState("");
  const { state: s, dispatch, compact, open } = useDemo(),
    h = browseHistory(s),
    loc = browseLocation(s),
    n = s.storage.nodes.find((n) => n.id === loc.directoryId);
  const copy = async () => {
    const path = `${s.connections.find((c) => c.id === loc.connectionId)?.name} / ${n?.path || loc.label}`;
    try {
      await navigator.clipboard.writeText(path);
      dispatch({
        type: "notice",
        text: "已复制目录显示路径；不包含凭据或下载地址。",
      });
    } catch {
      setManual(path);
    }
  };
  return (
    <>
      <IconBtn
        label="后退"
        title="后退 Alt+←"
        className="!size-8 !min-w-0 [@media(pointer:coarse)]:!size-11 max-[767px]:!size-11"
        disabled={!h.back.length}
        onClick={() => dispatch({ type: "browse-history", direction: "back" })}
      >
        <ArrowLeft size={15} />
      </IconBtn>
      {!compact && (
        <IconBtn
          label="前进"
          title="前进 Alt+→"
          className="!size-8 !min-w-0 [@media(pointer:coarse)]:!size-11"
          disabled={!h.forward.length}
          onClick={() =>
            dispatch({ type: "browse-history", direction: "forward" })
          }
        >
          <ArrowRight size={15} />
        </IconBtn>
      )}
      <IconBtn
        label={loc.kind === "query" ? "返回目录" : "上一级"}
        title={loc.kind === "query" ? "退出检索，返回来源目录" : "上一级 Alt+↑"}
        className="!size-8 !min-w-0 [@media(pointer:coarse)]:!size-11 max-[767px]:!size-11"
        disabled={
          loc.kind !== "query" && (!n?.parentId || loc.kind !== "directory")
        }
        onClick={() =>
          dispatch({
            type: loc.kind === "query" ? "browse-return" : "browse-up",
          })
        }
      >
        <ArrowUp size={15} />
      </IconBtn>
      {more && (
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <IconBtn
                label="目录更多操作"
                className="!size-8 !min-w-0 [@media(pointer:coarse)]:!size-11 max-[767px]:!size-11"
              >
                <MoreHorizontal size={15} />
              </IconBtn>
            }
          />
          <DropdownMenuContent align="start" className="!w-64">
            {n && (
              <DropdownMenuItem
                className="min-h-11"
                onClick={() =>
                  dispatch({
                    type: "bookmark-toggle",
                    ref: entryRef(s, n.id, "directory")!,
                  })
                }
              >
                {bookmarkFor(s, entryRef(s, n.id, "directory")!)
                  ? "取消收藏当前目录"
                  : "收藏当前目录"}
              </DropdownMenuItem>
            )}
            <DropdownMenuItem
              className="min-h-11"
              onClick={() => open({ type: "upload" })}
            >
              上传文件
            </DropdownMenuItem>
            <DropdownMenuItem
              className="min-h-11"
              onClick={() => open({ type: "new-folder" })}
            >
              新建目录
            </DropdownMenuItem>
            <DropdownMenuItem
              className="min-h-11"
              disabled={!h.forward.length}
              onClick={() =>
                dispatch({ type: "browse-history", direction: "forward" })
              }
            >
              前进
            </DropdownMenuItem>
            <DropdownMenuItem
              className="min-h-11"
              onClick={() => dispatch({ type: "browse-refresh" })}
            >
              <RefreshCw size={14} />
              刷新当前范围
            </DropdownMenuItem>
            <DropdownMenuItem
              className="min-h-11"
              onClick={() => dispatch({ type: "browse-reveal" })}
            >
              <Locate size={14} />
              定位当前目录树
            </DropdownMenuItem>
            <DropdownMenuItem className="min-h-11" onClick={copy}>
              <Copy size={14} />
              复制目录路径
            </DropdownMenuItem>
            {["files", "search"].includes(s.page) && (
              <>
                <p className="border-t px-2 py-2 text-[10px] text-muted-foreground">
                  排序
                </p>
                {[
                  { value: "name", label: "按名称" },
                  { value: "size", label: "按大小" },
                  { value: "recent", label: "最近更新" },
                ].map((option) => (
                  <DropdownMenuItem
                    key={option.value}
                    className="min-h-11"
                    onClick={() => {
                      if (s.page === "search" && loc.surface !== "search")
                        dispatch({
                          type: "browse-query",
                          surface: "search",
                          query: s.search,
                        });
                      dispatch({
                        type: "browser",
                        patch: { sort: option.value },
                      });
                    }}
                  >
                    {option.label}
                    {loc.sort === option.value ? " · 当前" : ""}
                  </DropdownMenuItem>
                ))}
              </>
            )}
            {n?.parentId && (
              <>
                <p className="border-t px-2 py-2 text-[10px] text-muted-foreground">
                  上级路径
                </p>
                {(() => {
                  const chain = [];
                  let parent = s.storage.nodes.find((x) => x.id === n.parentId);
                  while (parent) {
                    chain.unshift(parent);
                    parent = s.storage.nodes.find(
                      (x) => x.id === parent!.parentId,
                    );
                  }
                  return chain.map((parent) => (
                    <DropdownMenuItem
                      key={parent.id}
                      className="min-h-11"
                      onClick={() =>
                        dispatch({ type: "directory-open", id: parent.id })
                      }
                    >
                      {parent.name}
                    </DropdownMenuItem>
                  ));
                })()}
              </>
            )}
            <p className="border-t px-2 py-2 text-[10px] text-muted-foreground">
              访问历史 · 当前用户与空间
            </p>
            {[...h.back]
              .reverse()
              .slice(0, 12)
              .map((l, i) => (
                <DropdownMenuItem
                  key={i}
                  className="min-h-11"
                  onClick={() =>
                    dispatch({
                      type: "browse-history",
                      direction: "back",
                      index: i,
                    })
                  }
                >
                  <span className="min-w-0 truncate">
                    {s.connections.find((c) => c.id === l.connectionId)?.name} /{" "}
                    {l.kind === "query"
                      ? `检索 ${l.query?.query || l.query?.documentTag || "组合条件"}`
                      : l.kind === "favorites"
                        ? "收藏"
                        : l.label}
                  </span>
                </DropdownMenuItem>
              ))}
          </DropdownMenuContent>
        </DropdownMenu>
      )}
      <Dialog open={!!manual} onOpenChange={(value) => !value && setManual("")}>
        <DialogContent>
          <DialogTitle>手动复制路径</DialogTitle>
          <DialogDescription>
            剪贴板不可用；请选中完整路径复制。
          </DialogDescription>
          <Input
            value={manual}
            readOnly
            aria-label="目录显示路径"
            onFocus={(e) => e.target.select()}
          />
        </DialogContent>
      </Dialog>
    </>
  );
}
