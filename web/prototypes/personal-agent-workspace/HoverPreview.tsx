import {
  createContext,
  useContext,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import type { ComponentProps, ReactNode, PointerEvent } from "react";
import { Popover } from "@base-ui/react/popover";
import { Btn, FileArt, useDemo } from "./ui";
import { previewUnavailable, refKey } from "./preview-model";
import type { PreviewRef } from "./preview-model";
import { versionSnapshot } from "./model";
import { createHoverController } from "./hover-model";
interface Candidate {
  reference: PreviewRef;
  anchor: HTMLElement;
  open: () => void;
}
interface HoverActions {
  enter: (c: Candidate) => void;
  leave: () => void;
  cancel: () => boolean;
  active: () => boolean;
}
const Hover = createContext<HoverActions | null>(null);
export const FileHoverBridge = Hover.Provider;
export const useFileHover = () => useContext(Hover);
export function HoverPreviewProvider({ children }: { children: ReactNode }) {
  const { state: s } = useDemo();
  const latest = useRef(s);
  latest.current = s;
  const [candidate, setCandidate] = useState<Candidate | null>(null);
  const controller = useMemo(
    () =>
      createHoverController<Candidate>(
        setCandidate,
        (c) =>
          c.anchor.isConnected &&
          c.anchor.getClientRects().length > 0 &&
          !previewUnavailable(latest.current, c.reference),
        (a, b) =>
          a.anchor === b.anchor && refKey(a.reference) === refKey(b.reference),
      ),
    [],
  );
  const error = candidate ? previewUnavailable(s, candidate.reference) : "";
  useLayoutEffect(() => {
    controller.cancel();
  }, [
    s.page,
    s.space,
    s.activeConnection,
    s.actor,
    s.viewState,
    s.browser.folder,
    JSON.stringify(s.browser.filter),
    s.browser.layout,
  ]);
  useEffect(() => {
    if (error) controller.cancel();
  }, [error]);
  useEffect(() => {
    const cancel = () => {
      controller.cancel();
    };
    const pointer = (e: Event) => {
      if (
        !(e.target instanceof Element) ||
        !e.target.closest("[data-hover-preview]")
      )
        cancel();
    };
    const keyboard = (e: KeyboardEvent) => {
      if (e.key === "Escape" && controller.active()) {
        e.preventDefault();
        e.stopImmediatePropagation();
        cancel();
      } else if (e.key !== "Escape") cancel();
    };
    window.addEventListener("keydown", keyboard, true);
    window.addEventListener("scroll", pointer, true);
    window.addEventListener("pointerdown", pointer, true);
    window.addEventListener("dragstart", cancel, true);
    window.addEventListener("resize", cancel);
    window.addEventListener("blur", cancel);
    return () => {
      cancel();
      window.removeEventListener("keydown", keyboard, true);
      window.removeEventListener("scroll", pointer, true);
      window.removeEventListener("pointerdown", pointer, true);
      window.removeEventListener("dragstart", cancel, true);
      window.removeEventListener("resize", cancel);
      window.removeEventListener("blur", cancel);
    };
  }, [controller]);
  const f = candidate
    ? s.files.find((f) => f.id === candidate.reference.id)
    : undefined;
  return (
    <Hover.Provider value={controller}>
      {children}
      <Popover.Root
        open={!!candidate && !error}
        modal={false}
        onOpenChange={(next) => {
          if (!next) controller.cancel();
        }}
      >
        {candidate && !error && (
          <Popover.Portal
            container={
              candidate?.anchor.closest<HTMLElement>(
                "[data-overlay-surface]",
              ) || undefined
            }
          >
            <Popover.Positioner
              anchor={candidate?.anchor}
              side="right"
              align="start"
              sideOffset={8}
              collisionPadding={12}
              positionMethod="fixed"
              className="z-[70]"
            >
              <Popover.Popup
                data-hover-preview
                initialFocus={false}
                finalFocus={false}
                onPointerEnter={() => controller.retain()}
                onPointerLeave={() => controller.leave()}
                className="w-80 max-w-[calc(100vw-24px)] overflow-hidden rounded-xl border border-border bg-card text-sm shadow-lg outline-none"
              >
                {candidate && f && !error && (
                  <>
                    <div
                      className="pointer-events-none max-h-44 overflow-hidden border-b border-border"
                      aria-hidden="true"
                    >
                      <FileArt file={f} version={candidate.reference.version} />
                    </div>
                    <div className="space-y-2 p-4">
                      <Popover.Title className="break-words font-semibold">
                        {f.name}
                      </Popover.Title>
                      <p className="text-xs text-muted-foreground">
                        {s.connections.find(
                          (c) =>
                            c.id ===
                            (candidate.reference.connectionId ||
                              f.connectionId ||
                              "drive"),
                        )?.name || "来源未知"}{" "}
                        ·{" "}
                        {candidate.reference.externalRevision ||
                          `v${candidate.reference.version}`}{" "}
                        · {f.size || "大小未知"}
                      </p>
                      {f.kind === "document" && (
                        <p className="line-clamp-3 text-xs leading-6">
                          {
                            versionSnapshot(f, candidate.reference.version)
                              .content
                          }
                        </p>
                      )}
                      <p className="text-xs text-muted-foreground">
                        {f.inbox ? "待验收 · " : ""}固定引用 · 演示预览
                        {["audio", "video"].includes(f.kind)
                          ? " · 不自动播放"
                          : ""}
                      </p>
                      <Btn
                        variant="outline"
                        className="w-full"
                        onClick={() => {
                          const next = candidate;
                          controller.cancel();
                          next.open();
                        }}
                      >
                        打开此版本
                      </Btn>
                    </div>
                  </>
                )}
              </Popover.Popup>
            </Popover.Positioner>
          </Popover.Portal>
        )}
      </Popover.Root>
    </Hover.Provider>
  );
}
export function FileButton({
  reference,
  onOpen,
  ...props
}: Omit<ComponentProps<typeof Btn>, "onClick"> & {
  reference?: PreviewRef;
  onOpen: () => void;
}) {
  const hover = useContext(Hover);
  return (
    <Btn
      {...props}
      data-file-preview-trigger={
        reference ? `${reference.id}@${reference.version}` : undefined
      }
      onPointerEnter={(e) => {
        props.onPointerEnter?.(e);
        if (
          reference &&
          e.pointerType === "mouse" &&
          e.buttons === 0 &&
          matchMedia("(hover:hover) and (pointer:fine)").matches &&
          !props.disabled &&
          !(
            e.target instanceof Element &&
            e.target.closest('[aria-label="文件标签"]')
          )
        )
          hover?.enter({
            reference: { ...reference },
            anchor: e.currentTarget,
            open: onOpen,
          });
      }}
      onPointerMove={(e) => {
        props.onPointerMove?.(e);
        if (
          e.target instanceof Element &&
          e.target.closest('[aria-label="文件标签"]')
        )
          hover?.cancel();
        else if (
          reference &&
          e.pointerType === "mouse" &&
          e.buttons === 0 &&
          !props.disabled &&
          matchMedia("(hover:hover) and (pointer:fine)").matches &&
          hover &&
          !hover.active()
        )
          hover.enter({
            reference: { ...reference },
            anchor: e.currentTarget,
            open: onOpen,
          });
      }}
      onPointerLeave={(e) => {
        props.onPointerLeave?.(e);
        hover?.leave();
      }}
      onClick={() => {
        hover?.cancel();
        onOpen();
      }}
    />
  );
}
