import { createContext, useContext } from "react";
import { Dialog as Primitive } from "@base-ui/react/dialog";
import type { ComponentProps } from "react";
import { X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
export {
  DialogTitle,
  DialogDescription,
  DialogHeader,
  DialogFooter,
  DialogClose,
  DialogTrigger,
} from "@/components/ui/dialog";
// Hidden origin pages retain their local state while their portal surfaces are suspended.
export const SurfaceVisible = createContext(true);
export function Dialog({
  open,
  ...props
}: ComponentProps<typeof Primitive.Root>) {
  const visible = useContext(SurfaceVisible);
  return <Primitive.Root {...props} open={!!open && visible} />;
}
export function DialogContent({
  className,
  children,
  showCloseButton = true,
  kind = "form",
  portalContainer,
  backdropClassName,
  ...props
}: ComponentProps<typeof Primitive.Popup> & {
  showCloseButton?: boolean;
  kind?: "confirm" | "form" | "complex" | "detail";
  portalContainer?: ComponentProps<typeof Primitive.Portal>["container"];
  backdropClassName?: string;
}) {
  return (
    <Primitive.Portal container={portalContainer}>
      <Primitive.Backdrop
        data-slot="dialog-overlay"
        className={cn(
          "fixed inset-0 z-50 bg-black/15 data-closed:pointer-events-none data-closed:opacity-0",
          kind === "detail" && "!bg-black/5",
          backdropClassName,
        )}
      />
      <Primitive.Popup
        data-slot="dialog-content"
        data-overlay-surface
        data-dialog-kind={kind}
        {...props}
        className={cn(
          "@container fixed left-1/2 top-1/2 z-50 flex w-[calc(100%-24px)] max-w-[520px] max-h-[90dvh] data-closed:hidden -translate-x-1/2 -translate-y-1/2 flex-col gap-3 overflow-y-auto rounded-lg border border-border bg-card p-4 text-sm shadow-lg outline-none",
          kind === "confirm" && "max-w-[400px]",
          kind === "complex" && "max-w-[640px]",
          kind === "detail" &&
            "!left-auto !right-0 !top-12 !bottom-0 !translate-x-0 !translate-y-0 !h-[calc(100dvh-48px)] !max-h-none !w-[min(600px,100vw)] !max-w-none !rounded-none",
          "[&>button:not([data-slot=dialog-close])]:self-end [&>[data-slot=dialog-title]]:pr-10",
          className,
        )}
      >
        {children}
        {showCloseButton && (
          <Primitive.Close
            render={
              <Button
                variant="ghost"
                size="icon"
                className="absolute right-2 top-2 !size-11"
                aria-label="关闭对话框"
              />
            }
          >
            <X size={17} />
          </Primitive.Close>
        )}
      </Primitive.Popup>
    </Primitive.Portal>
  );
}
