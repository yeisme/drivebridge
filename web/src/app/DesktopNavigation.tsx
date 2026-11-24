import { useId, useState, type ReactNode } from "react";
import { AnimatePresence, motion } from "motion/react";
import {
  ChevronDown,
  FilePlus2,
  FolderPlus,
  HardDrive,
  Plus,
  Settings2,
  Star,
  Upload,
  UsersRound,
} from "lucide-react";
import { Button } from "../components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "../components/ui/dropdown-menu";
import type { Space } from "../api";
import { EASE_OUT_CRISP, SPRING_PILL } from "../lib/motion-presets";
import type { Page } from "./navigation-state";
import type { visibleNavGroups } from "../nav-groups";

export interface SidebarQuickCreate {
  readOnly?: boolean;
  onUpload: () => void;
  onNewFolder?: () => void;
  onNewNote?: () => void;
}

export interface SidebarDriveSwitcher {
  spaces: readonly Space[];
  activeSpaceId?: string;
  driveOpen: boolean;
  favorites?: readonly string[];
  activePrefix?: string;
  onOpenDrive: (space: Space) => void;
  onOpenAllDrives: () => void;
  onOpenFavorite?: (prefix: string) => void;
}

const CONSUMER_ITEM_TEXT: Partial<Record<Page, string>> = {
  overview: "最近使用",
  collections: "收藏与图集",
  knowledge: "知识笔记",
  inbox: "AI 投递箱",
  shares: "我的分享",
  members: "密钥与邀请",
};

interface Props {
  groups: ReturnType<typeof visibleNavGroups>;
  moreGroups?: ReturnType<typeof visibleNavGroups>;
  items: readonly [Page, string, ReactNode][];
  page: Page;
  title: string;
  pendingCount: number;
  inviteCount: number;
  inboxCount?: number;
  quickCreate?: SidebarQuickCreate;
  driveSwitcher?: SidebarDriveSwitcher;
  onToggle: (id: string, open: boolean) => void;
  onNavigate: (page: Page) => void;
}

// Render only the already-authorized groups supplied by the application.
export function DesktopNavigation(p: Props) {
  const navScopeId = useId();
  const [drivesExpanded, setDrivesExpanded] = useState(true);
  const [moreExpanded, setMoreExpanded] = useState(false);
  const singlePrimaryList = p.groups.length === 1 && !p.groups[0].items.includes("transfers");
  const primaryIds = new Set(p.groups.flatMap((g) => g.items));
  const extraItems = (p.moreGroups || [])
    .flatMap((g) => g.items)
    .filter((id) => !primaryIds.has(id) && id !== "settings");
  const activeInExtra = extraItems.includes(p.page);

  return (
    <nav
      aria-label="工作区导航"
      style={{ scrollbarWidth: "none" }}
      className="no-scrollbar db-pine-nav-menu mt-2 min-h-0 w-full flex-1 gap-0.5 overflow-y-auto overflow-x-hidden px-2 pb-2 group-data-[collapsed=true]/shell:px-1.5"
    >
      {p.quickCreate && (
        <div className="mb-3 px-0.5 group-data-[collapsed=true]/shell:px-0">
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button
                  type="button"
                  disabled={p.quickCreate.readOnly}
                  aria-label="新建或上传"
                  title="新建或上传"
                  className="tactile-cta h-8 w-full justify-center gap-1.5 rounded-lg text-[13px] font-medium group-data-[collapsed=true]/shell:size-9 group-data-[collapsed=true]/shell:px-0"
                >
                  <Plus className="size-4 shrink-0" strokeWidth={2} />
                  <span className="group-data-[collapsed=true]/shell:hidden">新建 / 上传</span>
                </Button>
              }
            />
            <DropdownMenuContent align="start" className="w-48 rounded-xl p-1.5 shadow-[var(--shadow-popover)]">
              <DropdownMenuItem disabled={p.quickCreate.readOnly} onClick={p.quickCreate.onUpload} className="rounded-md">
                <Upload className="size-4 text-muted-foreground" />
                上传文件
              </DropdownMenuItem>
              {p.quickCreate.onNewFolder && (
                <DropdownMenuItem disabled={p.quickCreate.readOnly} onClick={p.quickCreate.onNewFolder} className="rounded-md">
                  <FolderPlus className="size-4 text-muted-foreground" />
                  新建文件夹
                </DropdownMenuItem>
              )}
              {p.quickCreate.onNewNote && (
                <>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem onClick={p.quickCreate.onNewNote} className="rounded-md">
                    <FilePlus2 className="size-4 text-muted-foreground" />
                    打开知识笔记
                  </DropdownMenuItem>
                </>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      )}

      {p.groups.map((group) => (
        <div
          className="nav-group db-nav-group mb-2 flex flex-col gap-0.5"
          key={group.id}
          data-open={group.open ? "true" : "false"}
        >
          {!singlePrimaryList && (
            <Button
              layout="surface"
              variant="ghost"
              type="button"
              className="nav-group-toggle db-nav-group-toggle flex min-h-7.5 w-full items-center gap-1.5 rounded-md px-2 text-[11px] font-medium text-[var(--navigation-muted)] hover:bg-[var(--navigation-hover)] group-data-[collapsed=true]/shell:hidden"
              aria-expanded={group.open}
              aria-label={group.label}
              onClick={() => p.onToggle(group.id, !group.open)}
            >
              <span>{group.label}</span>
              {!group.open && group.items.includes("transfers") && p.pendingCount > 0 && (
                <span
                  className="nav-count"
                  title="当前已加载列表中的未结束任务"
                  aria-label={`当前已加载列表有 ${p.pendingCount} 个未结束任务`}
                >
                  {p.pendingCount}
                </span>
              )}
              {!group.open && group.items.includes(p.page) && (
                <span className="nav-group-current" title={p.title}>
                  {p.title}
                </span>
              )}
              <ChevronDown size={13} aria-hidden="true" />
            </Button>
          )}
          {group.items.map((id) => {
            const item = p.items.find(([target]) => target === id);
            if (!item) return null;
            const [target, name, icon] = item;
            const isFiles = id === "files";
            const isActive = p.page === id;
            const hasSwitcher =
              isFiles &&
              p.driveSwitcher &&
              p.driveSwitcher.driveOpen &&
              (p.driveSwitcher.spaces.length > 1 || (p.driveSwitcher.favorites?.length ?? 0) > 0);
            return (
              <div key={id} className="flex flex-col gap-0.5">
                <Button
                  layout="surface"
                  variant="ghost"
                  type="button"
                  className={
                    "nav-item db-nav-item relative isolate flex h-9 min-h-9 w-full shrink-0 items-center justify-start gap-2 rounded-lg px-2 text-left text-[13px] font-normal text-sidebar-foreground transition-colors hover:bg-[var(--navigation-hover)] aria-[current=page]:bg-[var(--navigation-selected)] aria-[current=page]:font-semibold aria-[current=page]:text-[var(--nav-strong)] group-data-[collapsed=true]/shell:justify-center group-data-[collapsed=true]/shell:px-0 group-data-[collapsed=true]/shell:[&>.nav-item-label]:hidden" +
                    (isActive ? " active" : "")
                  }
                  title={name}
                  aria-label={name}
                  aria-current={isActive ? "page" : undefined}
                  onClick={() => p.onNavigate(target)}
                >
                  {isActive && (
                    <motion.span
                      layoutId={`${navScopeId}-active-pill`}
                      transition={SPRING_PILL}
                      aria-hidden="true"
                      className="pointer-events-none absolute inset-0 -z-10 rounded-lg bg-[var(--navigation-selected)]"
                    />
                  )}
                  <span className="nav-glyph-chip" data-nav-id={target} aria-hidden="true">
                    {icon}
                  </span>
                  <span className="nav-item-label truncate">
                    {singlePrimaryList ? CONSUMER_ITEM_TEXT[target] || name : name}
                  </span>
                  {id === "transfers" && p.pendingCount > 0 && <i />}
                  {id === "inbox" && (p.inboxCount || 0) > 0 && (
                    <span className="nav-count" aria-label={`${p.inboxCount} 项待查收`}>
                      {p.inboxCount}
                    </span>
                  )}
                  {id === "teams" && p.inviteCount > 0 && <span className="nav-count">{p.inviteCount}</span>}
                </Button>

                {hasSwitcher && p.driveSwitcher && (
                  <div className="ml-3 flex flex-col gap-0.5 border-l border-[var(--navigation-edge)] pl-2 group-data-[collapsed=true]/shell:hidden">
                    {p.driveSwitcher.spaces.length > 1 && (
                      <>
                        <Button
                          layout="surface"
                          variant="ghost"
                          type="button"
                          aria-expanded={drivesExpanded}
                          onClick={() => setDrivesExpanded((v) => !v)}
                          className="flex h-7 items-center justify-between rounded-md px-2 text-[11px] font-medium text-[var(--navigation-muted)] hover:bg-[var(--navigation-hover)]"
                        >
                          <span>空间 ({p.driveSwitcher.spaces.length})</span>
                          <ChevronDown
                            size={12}
                            className={drivesExpanded ? "transition-transform duration-150" : "-rotate-90 transition-transform duration-150"}
                          />
                        </Button>
                        <AnimatePresence initial={false}>
                          {drivesExpanded && (
                            <motion.div
                              initial={{ opacity: 0, y: -4 }}
                              animate={{ opacity: 1, y: 0 }}
                              exit={{ opacity: 0, y: -4 }}
                              transition={EASE_OUT_CRISP}
                              className="flex flex-col gap-0.5"
                            >
                              {p.driveSwitcher.spaces.map((space) => {
                                const selected =
                                  p.page === "files" && p.driveSwitcher?.driveOpen && p.driveSwitcher.activeSpaceId === space.id;
                                return (
                                  <Button
                                    layout="surface"
                                    variant="ghost"
                                    type="button"
                                    key={space.id}
                                    aria-label={`打开网盘 ${space.name}`}
                                    aria-current={selected ? "location" : undefined}
                                    title={space.name}
                                    onClick={() => p.driveSwitcher?.onOpenDrive(space)}
                                    className={`flex h-7.5 min-h-7.5 w-full items-center justify-start gap-2 rounded-md px-2 text-left text-xs transition-colors hover:bg-[var(--navigation-hover)] ${
                                      selected
                                        ? "bg-[var(--navigation-selected)] font-semibold text-[var(--nav-strong)]"
                                        : "text-[var(--navigation-muted)]"
                                    }`}
                                  >
                                    {space.team_id ? (
                                      <UsersRound size={13} className="shrink-0" />
                                    ) : (
                                      <HardDrive size={13} className="shrink-0" />
                                    )}
                                    <span className="truncate">{space.name}</span>
                                  </Button>
                                );
                              })}
                            </motion.div>
                          )}
                        </AnimatePresence>
                        <Button
                          layout="surface"
                          variant="ghost"
                          type="button"
                          onClick={p.driveSwitcher.onOpenAllDrives}
                          className="flex h-7 items-center justify-start gap-1.5 rounded-md px-2 text-[11px] text-[var(--navigation-muted)] hover:bg-[var(--navigation-hover)] hover:text-foreground"
                        >
                          <Settings2 size={12} className="shrink-0" />
                          <span>全部网盘目录</span>
                        </Button>
                      </>
                    )}
                    {p.driveSwitcher.favorites && p.driveSwitcher.favorites.length > 0 && p.driveSwitcher.onOpenFavorite && (
                      <div className="mt-0.5 flex flex-col gap-0.5 pt-0.5">
                        <span className="px-2 text-[10px] font-medium tracking-wide text-[var(--navigation-muted)]">
                          收藏目录
                        </span>
                        {p.driveSwitcher.favorites.map((fav) => {
                          const activeFav = p.page === "files" && p.driveSwitcher?.activePrefix === fav;
                          return (
                            <Button
                              layout="surface"
                              variant="ghost"
                              type="button"
                              key={fav || "__root__"}
                              onClick={() => p.driveSwitcher?.onOpenFavorite?.(fav)}
                              title={fav || "全部文件"}
                              className={`flex h-7 min-h-7 w-full items-center justify-start gap-1.5 rounded-md px-2 text-left text-xs transition-colors hover:bg-[var(--navigation-hover)] ${
                                activeFav
                                  ? "bg-[var(--navigation-selected)] font-semibold text-[var(--nav-strong)]"
                                  : "text-[var(--navigation-muted)]"
                              }`}
                            >
                              <Star size={12} className="shrink-0" />
                              <span className="truncate">{fav ? fav.split("/").pop() : "全部文件"}</span>
                            </Button>
                          );
                        })}
                      </div>
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      ))}

      {extraItems.length > 0 && (
        <div className="mt-1 border-t border-[var(--navigation-edge)] pt-2">
          <Button
            layout="surface"
            variant="ghost"
            type="button"
            aria-expanded={moreExpanded || activeInExtra}
            aria-label="更多空间功能"
            onClick={() => setMoreExpanded((v) => !v)}
            className="flex h-8 min-h-8 w-full items-center justify-between rounded-md px-2 text-xs font-medium text-[var(--navigation-muted)] hover:bg-[var(--navigation-hover)] group-data-[collapsed=true]/shell:hidden"
          >
            <span>更多功能</span>
            <ChevronDown
              size={13}
              className={moreExpanded || activeInExtra ? "transition-transform duration-150" : "-rotate-90 transition-transform duration-150"}
            />
          </Button>
          <AnimatePresence initial={false}>
            {(moreExpanded || activeInExtra) && (
              <motion.div
                initial={{ opacity: 0, y: -4 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -4 }}
                transition={EASE_OUT_CRISP}
                className="mt-0.5 flex flex-col gap-0.5"
              >
                {extraItems.map((id) => {
                  const item = p.items.find(([target]) => target === id);
                  if (!item) return null;
                  const [target, name, icon] = item;
                  const isActive = p.page === id;
                  return (
                    <Button
                      layout="surface"
                      variant="ghost"
                      type="button"
                      key={id}
                      title={name}
                      aria-label={name}
                      aria-current={isActive ? "page" : undefined}
                      onClick={() => p.onNavigate(target)}
                      className={
                        "nav-item db-nav-item relative isolate flex h-9 min-h-9 w-full shrink-0 items-center justify-start gap-2 rounded-lg px-2 text-left text-[13px] font-normal text-sidebar-foreground transition-colors hover:bg-[var(--navigation-hover)] aria-[current=page]:bg-[var(--navigation-selected)] aria-[current=page]:font-semibold aria-[current=page]:text-[var(--nav-strong)] group-data-[collapsed=true]/shell:justify-center group-data-[collapsed=true]/shell:px-0 group-data-[collapsed=true]/shell:[&>.nav-item-label]:hidden" +
                        (isActive ? " active" : "")
                      }
                    >
                      {isActive && (
                        <motion.span
                          layoutId={`${navScopeId}-active-pill`}
                          transition={SPRING_PILL}
                          aria-hidden="true"
                          className="pointer-events-none absolute inset-0 -z-10 rounded-lg bg-[var(--navigation-selected)]"
                        />
                      )}
                      <span className="nav-glyph-chip" data-nav-id={target} aria-hidden="true">
                        {icon}
                      </span>
                      <span className="nav-item-label truncate">{name}</span>
                      {id === "transfers" && p.pendingCount > 0 && <i />}
                      {id === "teams" && p.inviteCount > 0 && <span className="nav-count">{p.inviteCount}</span>}
                    </Button>
                  );
                })}
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      )}
    </nav>
  );
}
