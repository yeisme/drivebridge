import type { ReactNode } from "react";
import { X } from "lucide-react";
import { Dialog, DialogContent, DialogTitle, DialogClose } from "./ui/dialog";
import { Button } from "./ui/button";
import "./app-dialog.css";
import { cn } from "../lib/utils";

export function AppDialog({ title, close, children, className, headerActions, headerClassName }: {
  title: string; close: () => void; children: ReactNode; className?: string; headerActions?: ReactNode; headerClassName?: string;
}) {
  return <Dialog open onOpenChange={open => { if (!open) close(); }}>
    <DialogContent aria-label={title} aria-describedby={undefined} showCloseButton={false} className={cn("app-dialog max-h-[90dvh] overflow-y-auto p-4 sm:p-5 sm:max-w-2xl [&_form]:min-w-0 [&_form]:space-y-4 [&_.dialog-actions]:flex [&_.dialog-actions]:flex-wrap [&_.dialog-actions]:justify-end [&_.dialog-actions]:gap-2 [&_.dialog-actions]:pt-6 [&_.dialog-actions]:mt-6", className)}>
      <header className={cn("flex items-center gap-3", headerClassName)}>
        <DialogTitle className="min-w-0 flex-1 truncate" title={title}>{title}</DialogTitle>
        {headerActions}
        <DialogClose render={<Button variant="ghost" size="icon" className="max-md:size-11" aria-label="关闭" />}><X /></DialogClose>
      </header>
      {children}
    </DialogContent>
  </Dialog>;
}
