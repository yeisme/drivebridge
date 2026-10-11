import { useEffect, useRef, useState } from "react";
import {
  FolderInput,
  MoreHorizontal,
  Share2,
  Sparkles,
  Tag,
  X,
} from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Btn, IconBtn, useDemo } from "./ui";

export function BatchActions({ mobile = false }: { mobile?: boolean }) {
  const { state: s, dispatch, open, go } = useDemo();
  const ref = useRef<HTMLDivElement>(null);
  const [wide, setWide] = useState(false);
  const [moreOpen, setMoreOpen] = useState(false);
  useEffect(() => {
    const observer = new ResizeObserver(([e]) =>
      setWide(e.contentRect.width >= 570),
    );
    if (ref.current) observer.observe(ref.current);
    return () => observer.disconnect();
  }, []);
  const actions = [
    { name: "标签", run: () => open({ type: "label", ids: s.selected }) },
    { name: "移动", run: () => open({ type: "organize", ids: s.selected }) },
    { name: "分享", run: () => open({ type: "delivery", ids: s.selected }) },
    {
      name: "收藏",
      run: () => open({ type: "bookmark-groups", ids: s.selected }),
    },
    {
      name: "属性",
      run: () => open({ type: "property-batch", ids: s.selected }),
    },
    {
      name: "复制",
      run: () => open({ type: "organize", ids: s.selected, kind: "copy" }),
    },
    {
      name: "重命名",
      run: () => open({ type: "organize", ids: s.selected, kind: "rename" }),
    },
    {
      name: "加入集合",
      run: () => open({ type: "collection", ids: s.selected }),
    },
    {
      name: "比较",
      run: () =>
        s.selected.length <= 2
          ? open({ type: "compare", ids: s.selected })
          : dispatch({
              type: "notice",
              text: "比较最多两份文件，请调整选择。",
            }),
    },
    {
      name: "下载",
      run: () => {
        s.selected.forEach((id) => dispatch({ type: "offline", id }));
        dispatch({
          type: "notice",
          text: "已为所选文件保存模拟离线副本，不会下载真实文件。",
        });
      },
    },
    {
      name: "移入回收站",
      run: () =>
        dispatch({
          type: "plan",
          ids: s.selected,
          kind: "trash",
          target: "回收站",
        }),
    },
  ];
  return (
    <div
      ref={ref}
      role="toolbar"
      aria-label="批量操作"
      data-batch-menu-open={moreOpen}
      className={`flex min-w-0 flex-1 items-center gap-1 ${mobile ? "h-full px-3" : "h-[44px]"}`}
    >
      <span
        className="mr-auto shrink-0 whitespace-nowrap text-xs font-medium text-primary"
        aria-live="polite"
      >
        {mobile ? `已选 ${s.selected.length}` : `已选 ${s.selected.length} 项`}
      </span>
      <Btn variant="ghost" onClick={actions[0].run}>
        <Tag size={15} />
        <span>标签</span>
      </Btn>
      {(wide || mobile) && (
        <Btn variant="ghost" onClick={actions[1].run}>
          <FolderInput size={15} />
          <span>移动</span>
        </Btn>
      )}
      {wide && !mobile && (
        <>
          <Btn variant="ghost" onClick={actions[2].run}>
            <Share2 size={15} />
            分享
          </Btn>
        </>
      )}
      <DropdownMenu open={moreOpen} onOpenChange={setMoreOpen}>
        <DropdownMenuTrigger
          render={
            <Btn variant="ghost" aria-label="更多批量操作">
              <MoreHorizontal size={18} />
              {!mobile && "更多"}
            </Btn>
          }
        />
        <DropdownMenuContent
          align="end"
          side={mobile ? "top" : "bottom"}
          className="!w-48"
        >
          {actions
            .filter((_, i) => i > (wide && !mobile ? 2 : mobile ? 1 : 0))
            .map((a) => (
              <DropdownMenuItem
                key={a.name}
                className="min-h-11"
                onClick={a.run}
              >
                {a.name}
              </DropdownMenuItem>
            ))}
        </DropdownMenuContent>
      </DropdownMenu>
      <IconBtn
        label="取消多选"
        onClick={() => dispatch({ type: "select", ids: [] })}
      >
        <X size={16} />
      </IconBtn>
    </div>
  );
}
export function FileToolbar({
  count,
  shown,
  matched = shown,
  page = 1,
  pages = 1,
  summary,
  extra,
}: {
  count: number;
  shown: string[];
  matched?: string[];
  page?: number;
  pages?: number;
  summary?: string;
  extra?: React.ReactNode;
}) {
  const { state: s, dispatch, open, compact } = useDemo();
  return (
    <div
      data-file-toolbar
      className="sticky top-0 z-20 flex h-[44px] shrink-0 items-center gap-2 bg-background text-xs text-muted-foreground"
    >
      {s.selected.length > 0 && !compact ? (
        <BatchActions />
      ) : (
        <>
          <span className="mr-auto min-w-0 truncate">
            {summary || `${count} 个文件 · 第 ${page} / ${pages} 页`}
            {compact && s.selected.length > 0
              ? ` · 已选 ${s.selected.length}`
              : ""}
          </span>
          {extra}
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Btn variant="ghost" aria-label="选择文件">
                  {!compact && "选择"}
                  <MoreHorizontal size={15} />
                </Btn>
              }
            />
            <DropdownMenuContent align="end" className="!w-48">
              <DropdownMenuItem
                className="min-h-11"
                onClick={() => dispatch({ type: "select", ids: shown })}
              >
                全选当前页
              </DropdownMenuItem>
              <DropdownMenuItem
                className="min-h-11"
                onClick={() =>
                  open({
                    type: "danger",
                    title: `选择全部 ${count} 份匹配文件？`,
                    text: "选择固定为当前筛选结果，后续新增文件不会自动加入。",
                    action: { type: "select", ids: matched },
                  })
                }
              >
                全部匹配
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </>
      )}
    </div>
  );
}
