import { useState } from "react";
import { SlidersHorizontal } from "lucide-react";
import type { ReactNode } from "react";
import { createPortal } from "react-dom";
import {
  Popover,
  PopoverTrigger,
  PopoverContent,
  PopoverTitle,
  PopoverDescription,
} from "@/components/ui/popover";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "./Dialog";
import { Btn, useDemo } from "./ui";
export function PreviewReadingTools({
  toolbar,
  summary,
  children,
  inline,
}: {
  toolbar: HTMLElement | null;
  summary: string;
  children: ReactNode;
  inline?: ReactNode;
}) {
  const { compact } = useDemo();
  const [open, setOpen] = useState(false);
  if (!toolbar) return null;
  const trigger = (
    <Btn
      variant="ghost"
      aria-label="阅读设置"
      title={summary}
      className="!h-11 !min-w-11 !max-w-36 !shrink !px-2"
    >
      <SlidersHorizontal size={16} className="shrink-0" />
      <span className="min-w-0 truncate text-xs">{summary}</span>
    </Btn>
  );
  const content = (
    <div className="min-h-0 space-y-4 overflow-auto p-4">{children}</div>
  );
  return createPortal(
    <div className="@container flex h-11 min-w-0 flex-1 items-center justify-end">
      <div className="hidden shrink-0 items-center @[400px]:flex">{inline}</div>
      {compact ? (
        <>
          <Btn
            variant="ghost"
            aria-label="阅读设置"
            className="!h-11 !min-w-11 !max-w-36 !shrink !px-2"
            onClick={() => setOpen(true)}
          >
            <SlidersHorizontal size={16} />
            <span className="truncate text-xs">{summary}</span>
          </Btn>
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogContent className="!gap-0 !p-0">
              <div className="p-4">
                <DialogTitle>阅读设置</DialogTitle>
                <DialogDescription>当前固定版本 · 只读演示</DialogDescription>
              </div>
              {content}
            </DialogContent>
          </Dialog>
        </>
      ) : (
        <Popover open={open} onOpenChange={setOpen} modal={false}>
          <PopoverTrigger render={trigger} />
          <PopoverContent
            align="end"
            className="!max-h-[min(680px,calc(100dvh-112px))] !w-[360px] !max-w-[calc(100vw-24px)] !gap-0 !overflow-hidden !p-0"
          >
            <div className="border-b p-3">
              <PopoverTitle>阅读设置</PopoverTitle>
              <PopoverDescription className="text-xs">
                当前固定版本 · 只读演示
              </PopoverDescription>
            </div>
            {content}
          </PopoverContent>
        </Popover>
      )}
    </div>,
    toolbar,
  );
}
