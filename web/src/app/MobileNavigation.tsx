import { useState } from "react";
import { FilePlus2, Files, House, Inbox, Plus, Upload, UserRound } from "lucide-react";
import { Button } from "../components/ui/button";
import { AppSheet } from "../components/AppSheet";
import type { Page } from "./navigation-state";
import styles from "./MobileNavigation.module.css";

interface MobileNavigationProps {
  page: Page;
  onNavigate: (page: Page) => void;
  allowed: readonly Page[];
  inboxCount?: number;
  readOnly?: boolean;
  onUpload?: () => void;
}

// Visibility is supplied by the owning shell; navigation never grants access.
export function MobileNavigation({
  page,
  onNavigate,
  allowed,
  inboxCount = 0,
  readOnly = false,
  onUpload,
}: MobileNavigationProps) {
  const [createOpen, setCreateOpen] = useState(false);

  const items = [
    { kind: "page" as const, page: "files" as Page, label: "文件", icon: Files },
    { kind: "page" as const, page: "overview" as Page, label: "最近", icon: House },
    { kind: "action" as const, label: "新建", icon: Plus },
    { kind: "page" as const, page: "inbox" as Page, label: "AI 投递", icon: Inbox },
    { kind: "page" as const, page: "settings" as Page, label: "我的", icon: UserRound },
  ];

  return (
    <>
      <nav className={styles.navigation} aria-label="手机主导航">
        {items.map((item) => {
          const Icon = item.icon;
          if (item.kind === "action") {
            return (
              <Button
                layout="surface"
                variant="ghost"
                key="create"
                type="button"
                aria-label="新建"
                disabled={readOnly && !allowed.includes("knowledge")}
                onClick={() => {
                  if (onUpload) setCreateOpen(true);
                  else if (allowed.includes("files")) onNavigate("files");
                }}
              >
                <span className={styles.createPill} aria-hidden="true">
                  <Icon size={20} strokeWidth={2.1} />
                </span>
                <span>{item.label}</span>
              </Button>
            );
          }
          const target = item.page;
          const enabled = allowed.includes(target);
          const active =
            page === target ||
            (target === "inbox" && (page === "agents" || page === "transfers"));
          return (
            <Button
              layout="surface"
              variant="ghost"
              key={target}
              type="button"
              disabled={!enabled}
              aria-current={active ? "page" : undefined}
              onClick={() => {
                if (enabled) onNavigate(target);
              }}
            >
              <span className={styles.iconWrap} aria-hidden="true">
                <Icon size={20} strokeWidth={1.8} />
                {target === "inbox" && inboxCount > 0 && (
                  <i className={styles.badge}>{inboxCount > 99 ? "99+" : inboxCount}</i>
                )}
              </span>
              <span>{item.label}</span>
            </Button>
          );
        })}
      </nav>

      {onUpload && (
        <AppSheet title="添加到云盘" open={createOpen} onOpenChange={setCreateOpen}>
          <div className="grid gap-2.5 p-1">
            <Button
              type="button"
              disabled={readOnly}
              className="min-h-12 justify-start gap-3 rounded-xl px-4 text-sm"
              onClick={() => {
                setCreateOpen(false);
                onUpload();
              }}
            >
              <Upload className="size-4" />
              上传照片或本地文件
            </Button>
            {allowed.includes("knowledge") && (
              <Button
                type="button"
                variant="outline"
                className="min-h-12 justify-start gap-3 rounded-xl px-4 text-sm"
                onClick={() => {
                  setCreateOpen(false);
                  onNavigate("knowledge");
                }}
              >
                <FilePlus2 className="size-4" />
                打开或新建知识笔记
              </Button>
            )}
          </div>
        </AppSheet>
      )}
    </>
  );
}
