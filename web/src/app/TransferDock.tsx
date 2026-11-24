import { useEffect, useRef, useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import {
  AlertCircle,
  ArrowUpDown,
  CheckCircle2,
  ChevronDown,
  ChevronUp,
  ExternalLink,
  LoaderCircle,
  RotateCw,
  X,
} from "lucide-react";
import { bytes, type Operation } from "../api";
import { Button } from "../components/ui/button";
import { EASE_OUT_CRISP, SPRING_DOCK } from "../lib/motion-presets";

export function transferStatusMeta(op: Pick<Operation, "status" | "size_bytes" | "bytes_transferred">): {
  label: string;
  tone: "active" | "verifying" | "done" | "failed" | "cancelled";
  percent: number | null;
} {
  if (op.status === "completed") {
    return { label: "已完成", tone: "done", percent: 100 };
  }
  if (op.status === "cancelled") {
    return { label: "已取消", tone: "cancelled", percent: null };
  }
  if (op.status === "failed") {
    return { label: "中断，可续传", tone: "failed", percent: null };
  }
  if (op.status === "verifying" || op.status === "durable") {
    return { label: "云端校验保存中…", tone: "verifying", percent: 96 };
  }
  const pct =
    op.size_bytes > 0
      ? Math.min(95, Math.max(2, Math.round((op.bytes_transferred / op.size_bytes) * 100)))
      : null;
  if (op.status === "hashing" || op.status === "preparing") {
    return { label: "准备校验…", tone: "active", percent: pct };
  }
  return {
    label: pct !== null ? `上传中 ${pct}%` : "传输中…",
    tone: "active",
    percent: pct,
  };
}

export function TransferDock({
  operations,
  open,
  onOpenChange,
  onOpenFile,
  onResume,
  onCancel,
  onOpenHistory,
}: {
  operations: readonly Operation[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onOpenFile: (op: Operation) => void;
  onResume: (op: Operation) => void;
  onCancel: (op: Operation) => void;
  onOpenHistory: () => void;
}) {
  const [minimized, setMinimized] = useState(false);
  const prevActiveCount = useRef(0);
  const activeOps = operations.filter(
    (op) => !["completed", "cancelled", "failed"].includes(op.status),
  );
  const failedOps = operations.filter((op) => op.status === "failed");

  useEffect(() => {
    if (activeOps.length > prevActiveCount.current) {
      onOpenChange(true);
      setMinimized(false);
    }
    prevActiveCount.current = activeOps.length;
  }, [activeOps.length, onOpenChange]);

  useEffect(() => {
    if (!open || activeOps.length > 0 || failedOps.length > 0) return;
    const timer = setTimeout(() => onOpenChange(false), 3200);
    return () => clearTimeout(timer);
  }, [open, activeOps.length, failedOps.length, onOpenChange]);

  const visible = open || activeOps.length > 0;
  const recent = operations.slice(0, 8);

  return (
    <AnimatePresence>
      {visible && (
        <motion.aside
          key="transfer-dock"
          initial={{ opacity: 0, y: 12, scale: 0.98 }}
          animate={{ opacity: 1, y: 0, scale: 1 }}
          exit={{ opacity: 0, y: 10, scale: 0.98 }}
          transition={SPRING_DOCK}
          aria-label="传输列表"
          data-testid="transfer-dock-panel"
          className="fixed bottom-4 right-4 z-40 w-[min(336px,calc(100vw-24px))] overflow-hidden rounded-xl border border-border bg-popover text-popover-foreground shadow-[var(--shadow-float)] max-md:bottom-[68px] max-md:right-3"
        >
          <header className="flex min-h-10 items-center justify-between gap-2 border-b border-border bg-muted/40 px-3 py-1.5">
            <div className="flex min-w-0 items-center gap-2 text-xs font-semibold">
              {activeOps.length > 0 ? (
                <LoaderCircle className="size-4 shrink-0 animate-spin text-primary" />
              ) : failedOps.length > 0 ? (
                <AlertCircle className="size-4 shrink-0 text-[var(--warning)]" />
              ) : (
                <CheckCircle2 className="size-4 shrink-0 text-[var(--success)]" />
              )}
              <span className="truncate">
                {activeOps.length > 0
                  ? `正在传输 ${activeOps.length} 项`
                  : failedOps.length > 0
                    ? `${failedOps.length} 项待续传`
                    : `传输完成（最近 ${recent.length} 项）`}
              </span>
            </div>
            <div className="flex items-center gap-1">
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="size-8 max-md:size-11"
                aria-label={minimized ? "展开传输列表" : "收起传输列表"}
                onClick={() => setMinimized((v) => !v)}
              >
                {minimized ? <ChevronUp className="size-4" /> : <ChevronDown className="size-4" />}
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="size-8 max-md:size-11"
                aria-label="隐藏传输窗"
                onClick={() => onOpenChange(false)}
              >
                <X className="size-4" />
              </Button>
            </div>
          </header>

          <AnimatePresence initial={false}>
            {!minimized && (
              <motion.div
                key="dock-body"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={EASE_OUT_CRISP}
              >
                {recent.length === 0 ? (
                  <div className="flex flex-col items-center gap-1.5 px-4 py-6 text-center text-xs text-muted-foreground">
                    <ArrowUpDown className="size-5 opacity-60" />
                    <span>当前暂无上传或下载记录</span>
                  </div>
                ) : (
                  <ul className="m-0 max-h-64 list-none divide-y divide-border/70 overflow-y-auto p-0">
                    {recent.map((op) => {
                      const meta = transferStatusMeta(op);
                      const name = op.path.split("/").pop() || op.path || "云盘文件";
                      return (
                        <li key={op.id} className="grid gap-1.5 px-3.5 py-2.5 text-xs">
                          <div className="flex min-w-0 items-center justify-between gap-2">
                            <strong className="min-w-0 flex-1 truncate font-medium" title={op.path}>
                              {name}
                            </strong>
                            <span
                              className={`shrink-0 tabular-nums ${
                                meta.tone === "done"
                                  ? "text-[var(--success)]"
                                  : meta.tone === "failed"
                                    ? "text-destructive"
                                    : meta.tone === "verifying"
                                      ? "font-medium text-primary"
                                      : "text-muted-foreground"
                              }`}
                            >
                              {meta.label}
                            </span>
                          </div>
                          {meta.percent !== null && meta.tone !== "done" && (
                            <progress value={meta.percent} max={100} className="h-1.5 w-full" />
                          )}
                          <div className="flex items-center justify-between gap-2 text-[11px] text-muted-foreground">
                            <span className="tabular-nums">{bytes(op.size_bytes)}</span>
                            <div className="flex items-center gap-1">
                              {meta.tone === "done" && op.file_id && (
                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="sm"
                                  className="h-7 px-2 text-[11px]"
                                  onClick={() => onOpenFile(op)}
                                >
                                  打开文件
                                </Button>
                              )}
                              {meta.tone === "failed" && (
                                <Button
                                  type="button"
                                  variant="outline"
                                  size="sm"
                                  className="h-7 gap-1 px-2 text-[11px]"
                                  onClick={() => onResume(op)}
                                >
                                  <RotateCw className="size-3" />
                                  继续上传
                                </Button>
                              )}
                              {(meta.tone === "active" || meta.tone === "verifying") && (
                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="sm"
                                  className="h-7 px-2 text-[11px] text-muted-foreground hover:text-destructive"
                                  onClick={() => onCancel(op)}
                                >
                                  取消
                                </Button>
                              )}
                            </div>
                          </div>
                        </li>
                      );
                    })}
                  </ul>
                )}
                <footer className="flex items-center justify-between border-t border-border/70 bg-muted/30 px-3.5 py-2">
                  <span className="text-[11px] text-muted-foreground">校验通过后方才标记已完成</span>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="h-7 gap-1 px-2 text-xs text-primary"
                    onClick={onOpenHistory}
                  >
                    全部记录
                    <ExternalLink className="size-3" />
                  </Button>
                </footer>
              </motion.div>
            )}
          </AnimatePresence>
        </motion.aside>
      )}
    </AnimatePresence>
  );
}
