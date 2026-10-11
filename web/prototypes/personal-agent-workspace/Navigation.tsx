import type { FloatingNavigation } from "./useFloatingNavigation";
import { useState } from "react";
import {
  ChevronDown,
  Check,
  HardDrive,
  PanelLeftClose,
  PanelLeftOpen,
  Pin,
  Settings,
  Users,
  UserRound,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { BrandMark } from "@/BrandMark";
import { cn } from "@/lib/utils";
import type { Page, Space } from "./model";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "./Dialog";
import { Btn, IconBtn, useDemo } from "./ui";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
export function Navigation({
  width,
  pages,
  pending,
  inboxCount,
  onSpace,
  floatingNav,
}: {
  width: number;
  pages: { id: Page; title: string; icon: LucideIcon; group: string }[];
  pending: number;
  inboxCount: number;
  onSpace: (space: Space) => void;
  floatingNav: FloatingNavigation;
}) {
  const { state: s, dispatch, open, go } = useDemo();
  const [spaces, setSpaces] = useState(false);
  const { open: drawer, setOpen: setDrawer } = floatingNav;
  const expanded = width >= 1280 && !s.preferences.navCollapsed;
  const content = (wide: boolean, overlay = false) => (
    <>
      <div
        className={cn(
          "flex h-12 shrink-0 items-center gap-2",
          wide ? "px-4" : "justify-center",
        )}
      >
        {wide && (
          <>
            <BrandMark size={30} />
            <strong className="min-w-0 flex-1 text-base tracking-tight">
              DriveBridge
            </strong>
          </>
        )}
        <IconBtn
          label={wide ? "折叠主导航" : "展开主导航"}
          className="!size-11 shrink-0"
          onClick={() => {
            if (overlay) setDrawer(false);
            else if (width < 1280) setDrawer(true);
            else
              dispatch({ type: "preferences", patch: { navCollapsed: wide } });
          }}
        >
          {wide ? <PanelLeftClose size={17} /> : <PanelLeftOpen size={19} />}
        </IconBtn>
      </div>
      <DropdownMenu
        open={spaces}
        onOpenChange={(value) => {
          floatingNav.hold(value);
          setSpaces(value);
        }}
        modal={false}
      >
        <DropdownMenuTrigger
          render={
            <Btn
              variant="ghost"
              aria-label="切换空间"
              className={cn(
                "mx-2 mb-4 !h-14 !justify-start !gap-2 !px-2",
                wide ? "" : "!justify-center",
              )}
            >
              <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                {s.space === "team" ? (
                  <Users size={17} />
                ) : (
                  <UserRound size={17} />
                )}
              </span>
              {wide && (
                <>
                  <span className="min-w-0 flex-1 text-left">
                    <span className="block truncate text-xs font-semibold">
                      {s.space === "team"
                        ? "山野 · 团队空间"
                        : `${s.members.find((m) => m.id === s.actor)?.name || "Yeisme"} · 个人空间`}
                    </span>
                    <span className="mt-1 block text-[10px] text-muted-foreground">
                      {s.space === "team"
                        ? "共享资料与成员权限"
                        : "自己的文件与存储"}
                    </span>
                  </span>
                  <ChevronDown size={13} />
                </>
              )}
            </Btn>
          }
        />
        <DropdownMenuContent
          aria-label="切换空间"
          className="!w-[280px] !max-w-[calc(100vw-24px)] p-1.5"
          align="start"
          sideOffset={6}
        >
          <p className="px-3 py-2 text-[10px] text-muted-foreground">
            工作空间 · 权限与收藏分别管理
          </p>
          {(
            [
              {
                id: "personal",
                name: "个人空间",
                note: "自己的文件与已连接存储",
              },
              {
                id: "team",
                name: "山野 · 团队空间",
                note: "按成员权限访问共享资料",
              },
            ] as const
          ).map((x) => (
            <DropdownMenuItem
              key={x.id}
              className="min-h-16 gap-3 px-3"
              onClick={() => {
                floatingNav.hold(false);
                setSpaces(false);
                onSpace(x.id);
              }}
            >
              <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                {x.id === "team" ? (
                  <Users size={17} />
                ) : (
                  <UserRound size={17} />
                )}
              </span>
              <span className="min-w-0 flex-1">
                <strong className="block text-xs">{x.name}</strong>
                <small className="text-[10px] text-muted-foreground">
                  {x.note}
                </small>
              </span>
              {s.space === x.id && <Check aria-label="当前空间" size={16} />}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
      <nav
        className="min-h-0 flex-1 overflow-y-auto overscroll-contain px-2"
        aria-label={overlay ? "展开的导航" : "工作区导航"}
      >
        {["工作台", "协作", "管理"].map((group) => (
          <div className="mb-4" key={group}>
            {wide ? (
              <p className="mb-2 px-3 text-[10px] font-medium tracking-wide text-muted-foreground">
                {group}
              </p>
            ) : (
              <div className="mx-3 mb-2 border-t border-border" />
            )}
            {pages
              .filter((p) => p.group === group)
              .map((p) => {
                const count =
                  p.id === "tasks"
                    ? pending
                    : p.id === "inbox"
                      ? inboxCount
                      : 0;
                return (
                  <Btn
                    key={p.id}
                    variant="ghost"
                    className={cn(
                      "relative mb-1 !h-11 !w-full !gap-3 border-l-2 !px-3 text-xs",
                      wide ? "!justify-start" : "!justify-center",
                      s.page === p.id
                        ? "border-l-primary bg-accent font-semibold text-primary"
                        : "border-l-transparent text-muted-foreground",
                    )}
                    aria-label={p.title}
                    aria-current={s.page === p.id ? "page" : undefined}
                    title={count ? `${p.title} · ${count} 项待处理` : p.title}
                    onClick={() => {
                      setDrawer(false);
                      go(p.id);
                    }}
                  >
                    <p.icon size={18} strokeWidth={1.6} />
                    {wide && (
                      <span className="min-w-0 flex-1 truncate text-left">
                        {p.title}
                      </span>
                    )}
                    {count > 0 &&
                      (wide ? (
                        <span className="rounded bg-card px-1.5 py-0.5 text-[10px]">
                          {count}
                        </span>
                      ) : (
                        <span className="absolute right-1 top-1 rounded bg-accent px-1 text-[9px] text-primary">
                          {count}
                        </span>
                      ))}
                  </Btn>
                );
              })}
          </div>
        ))}
      </nav>
      <div className="shrink-0 border-t border-border p-2">
        {overlay && width >= 1280 && (
          <Btn
            variant="ghost"
            aria-label="固定主导航"
            className="!h-11 !w-full !justify-start"
            onClick={() => {
              setDrawer(false);
              dispatch({ type: "preferences", patch: { navCollapsed: false } });
            }}
          >
            <Pin size={16} />
            固定导航
          </Btn>
        )}
        {wide ? (
          <Btn
            layout="surface"
            className="mb-2 !h-auto !w-full !flex-col !items-stretch !gap-2 !px-3 !py-3 text-left"
            onClick={() => {
              setDrawer(false);
              open({ type: "capacity" });
            }}
          >
            <span className="flex justify-between text-[10px] text-muted-foreground">
              <span>空间用量</span>
              <span>24 / 100 GB</span>
            </span>
            <span className="h-1 overflow-hidden rounded-full bg-border">
              <span className="block h-full w-1/4 rounded-full bg-primary/65" />
            </span>
          </Btn>
        ) : (
          <IconBtn
            label="空间用量"
            className="!size-11"
            onClick={() => {
              setDrawer(false);
              open({ type: "capacity" });
            }}
          >
            <HardDrive size={18} />
          </IconBtn>
        )}
        <Btn
          variant="ghost"
          aria-label="个人设置"
          className={cn(
            "!h-12 !w-full !px-2",
            wide ? "!justify-start" : "!justify-center",
          )}
          onClick={() => {
            setDrawer(false);
            go("settings");
          }}
        >
          <span className="flex size-7 shrink-0 items-center justify-center rounded-full bg-primary text-xs text-primary-foreground">
            {s.members.find((m) => m.id === s.actor)?.name[0] || "Y"}
          </span>
          {wide && (
            <>
              <span className="flex-1 text-left text-xs">
                {s.members.find((m) => m.id === s.actor)?.name}
              </span>
              <Settings size={15} className="text-muted-foreground" />
            </>
          )}
        </Btn>
      </div>
    </>
  );
  return (
    <>
      {expanded && (
        <aside
          aria-label="主导航"
          data-nav-collapsed={!expanded}
          style={{ width: 224 }}
          className="flex min-h-0 shrink-0 flex-col border-r border-border bg-sidebar"
        >
          {content(true)}
        </aside>
      )}
      {!expanded && floatingNav.canHover && !drawer && (
        <div
          data-nav-hover-edge
          aria-hidden="true"
          className="absolute bottom-0 left-0 top-12 z-40 w-2"
          onPointerEnter={floatingNav.enter}
          onPointerLeave={floatingNav.leave}
          title="悬停显示导航"
        />
      )}
      {!expanded && drawer && floatingNav.floating && (
        <aside
          ref={floatingNav.panel}
          id="floating-main-navigation"
          aria-label="悬浮主导航"
          data-floating-navigation
          className="absolute inset-y-0 left-0 z-40 flex w-56 min-h-0 flex-col border-r border-border bg-sidebar shadow-xl"
          onPointerEnter={floatingNav.retain}
          onPointerLeave={floatingNav.leave}
          onFocusCapture={floatingNav.onFocus}
          onBlurCapture={(event) => floatingNav.onBlur(event.relatedTarget)}
        >
          {content(true, true)}
        </aside>
      )}
      <Dialog
        open={!expanded && drawer && !floatingNav.floating}
        onOpenChange={setDrawer}
      >
        <DialogContent
          showCloseButton={false}
          className="!left-0 !top-0 !h-[var(--prototype-height,100dvh)] !max-h-none !w-56 !max-w-56 !translate-x-0 !translate-y-0 !gap-0 !rounded-none !p-0"
        >
          <DialogTitle className="sr-only">工作区导航</DialogTitle>
          {content(true, true)}
        </DialogContent>
      </Dialog>
    </>
  );
}
