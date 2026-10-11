import { useState, type ReactNode } from "react";
import { Info, ArrowLeft, ArrowRight, PanelRight, X } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "./Dialog";
import {
  Popover,
  PopoverTrigger,
  PopoverContent,
  PopoverTitle,
} from "@/components/ui/popover";
import { Btn, IconBtn } from "./ui";

export function RecordRow({
  name,
  title,
  summary,
  status,
  actions,
  children,
  icon,
  onOpen,
  identity,
}: {
  name: string;
  title?: ReactNode;
  summary?: ReactNode;
  status?: ReactNode;
  actions?: ReactNode;
  children?: ReactNode;
  icon?: ReactNode;
  onOpen?: () => void;
  identity?: Record<string, string>;
}) {
  const [opened, setOpened] = useState(false);
  return (
    <div
      {...(opened ? {} : identity)}
      data-record-name={name}
      className="min-w-0"
    >
      <div
        data-compact-status
        className="flex h-12 min-w-0 items-center gap-2 px-2 text-[13px] max-[767px]:h-16 [@media(pointer:coarse)]:h-16"
      >
        {icon && (
          <span className="shrink-0 text-primary [&>svg]:size-4">{icon}</span>
        )}
        <div className="min-w-0 flex-1">
          {title || (
            <Btn
              variant="ghost"
              title={name}
              className="!h-6 !min-h-0 !max-w-full !justify-start !px-0 !font-medium max-[767px]:!h-11 [@media(pointer:coarse)]:!h-11"
              onClick={onOpen || (() => setOpened(true))}
            >
              <span className="truncate">{name}</span>
            </Btn>
          )}
          {summary && (
            <div className="truncate text-[11px] text-muted-foreground">
              {summary}
            </div>
          )}
        </div>
        {status && <div className="shrink-0 text-xs">{status}</div>}
        <div className="hidden shrink-0 items-center gap-1 min-[1100px]:flex">
          {actions}
        </div>
        {children && (
          <IconBtn
            label={`查看${name}详情`}
            className="!size-8 !min-h-0 !min-w-0 max-[767px]:!size-11 [@media(pointer:coarse)]:!size-11"
            onClick={() => setOpened(true)}
          >
            <PanelRight size={15} />
          </IconBtn>
        )}
      </div>
      {children && (
        <Dialog open={opened} onOpenChange={setOpened}>
          <DialogContent {...identity} kind="detail" className="record-details">
            <DialogTitle>{name}</DialogTitle>
            <DialogDescription className="sr-only">
              查看记录与可用操作
            </DialogDescription>
            {opened && children}
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}

export function ViewToolbar({
  title,
  note,
  search,
  children,
  onClear,
  hasQuery,
}: {
  title: string;
  note?: string;
  search?: ReactNode;
  onClear?: () => void;
  hasQuery?: boolean;
  children?: ReactNode;
}) {
  return (
    <header
      data-view-toolbar
      className="@container flex h-11 shrink-0 items-center gap-2 border-b px-2"
    >
      <h1
        className="max-w-40 shrink-0 truncate text-sm font-semibold max-[767px]:max-w-24"
        title={title}
      >
        {title}
      </h1>
      {note && (
        <Popover>
          <PopoverTrigger
            render={
              <IconBtn
                label={`${title}说明`}
                className="!size-8 !min-h-0 !min-w-0 max-[767px]:!size-11 [@media(pointer:coarse)]:!size-11"
              >
                <Info size={14} />
              </IconBtn>
            }
          />
          <PopoverContent className="max-w-[calc(100vw-24px)]">
            <PopoverTitle>{title}</PopoverTitle>
            <p className="text-xs leading-6 text-muted-foreground">{note}</p>
          </PopoverContent>
        </Popover>
      )}
      <div className="ml-auto flex min-w-0 flex-1 items-center justify-end gap-1 [&>input]:min-w-0 [&>input]:max-w-sm [&>input]:flex-1">
        {search}
        {onClear && (
          <IconBtn
            label="清除搜索"
            disabled={!hasQuery}
            className={`!size-8 !min-h-0 !min-w-0 max-[767px]:!size-11 [@media(pointer:coarse)]:!size-11 ${hasQuery ? "" : "invisible"}`}
            onClick={(e) => {
              onClear();
              e.currentTarget.parentElement?.querySelector("input")?.focus();
            }}
          >
            <X size={14} />
          </IconBtn>
        )}
        {children}
      </div>
    </header>
  );
}
export function ListPagination({
  page,
  pages,
  onChange,
}: {
  page: number;
  pages: number;
  onChange: (p: number) => void;
}) {
  return (
    <div
      className="flex shrink-0 items-center text-xs tabular-nums"
      aria-label="列表分页"
    >
      <IconBtn
        label="上一页"
        disabled={page <= 1}
        onClick={() => onChange(page - 1)}
      >
        <ArrowLeft size={14} />
      </IconBtn>
      <span>
        {page}/{pages}
      </span>
      <IconBtn
        label="下一页"
        disabled={page >= pages}
        onClick={() => onChange(page + 1)}
      >
        <ArrowRight size={14} />
      </IconBtn>
    </div>
  );
}
