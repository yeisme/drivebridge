import type { ReactNode, RefObject } from 'react';
import { X } from 'lucide-react';
import { Button } from '../../components/ui/button';
import { Sheet, SheetClose, SheetContent, SheetHeader, SheetTitle } from '../../components/ui/sheet';

// One directory subtree: inline on desktop, modal on mobile. The file listing
// stays mounted; opening the drawer must not restart file reads or transfers.
export function FinderDirectoryPanel({ mobile, open, onOpenChange, returnFocus, id, children }: {
  mobile: boolean;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  returnFocus: RefObject<HTMLButtonElement | null>;
  id: string;
  children: ReactNode;
}) {
  if (!mobile) return open ? <aside id={id} className="finder-tree" aria-label="目录树">{children}</aside> : null;
  return <Sheet open={open} onOpenChange={onOpenChange}>
    <SheetContent id={id} side="left" aria-label="目录树" aria-describedby={undefined} showCloseButton={false} finalFocus={returnFocus}
      className="data-[side=left]:w-[min(320px,100vw)] gap-0 motion-reduce:transition-none">
      <SheetHeader className="flex-row items-center gap-2 border-b border-border">
        <SheetTitle className="min-w-0 flex-1">目录树</SheetTitle>
        <SheetClose render={<Button type="button" size="icon" variant="ghost" className="size-11" aria-label="关闭目录树" />}><X /></SheetClose>
      </SheetHeader>
      <div className="finder-tree min-h-0 max-h-none flex-1 overflow-auto overscroll-contain border-0 p-3 [&_button]:min-h-11 [&_button]:min-w-11 [&_.tree-expander]:w-11">
        {children}
      </div>
    </SheetContent>
  </Sheet>;
}
