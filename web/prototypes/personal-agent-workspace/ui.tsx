import {
  Popover,
  PopoverTrigger,
  PopoverContent,
  PopoverTitle,
} from "@/components/ui/popover";
import React, { createContext, useContext } from "react";
import type { Dispatch, ReactNode, ComponentProps } from "react";
import {
  ArrowUpRight,
  FileText,
  Image,
  Video,
  Music2,
  Sheet,
  Check,
  Clock3,
  CircleAlert,
  Cloud,
  Laptop,
  Smartphone,
  Bot,
  ChevronRight,
  Info,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";
import type {
  Action,
  State,
  FileRecord,
  FileKind,
  Page,
  TaskStatus,
} from "./model";
import type { PreviewScope } from "./preview-model";
import { taskStatusText } from "./model";

export type Modal =
  | {
      type: "preview";
      id: string;
      version?: number;
      scope?: PreviewScope;
      single?: boolean;
      archiveFolder?: string;
      anchor?: string;
      edit?: boolean;
    }
  | {
      type: "confirm";
      taskId: string;
      revision: number;
      contextRevision: number;
    }
  | { type: "edit-plan" | "conflict"; taskId: string }
  | { type: "compare"; ids: string[]; version?: number }
  | {
      type:
        | "upload"
        | "consent"
        | "agent"
        | "rule"
        | "command"
        | "notifications"
        | "device"
        | "help"
        | "label"
        | "capacity"
        | "delivery"
        | "duplicates"
        | "organize"
        | "collection"
        | "new-folder"
        | "conversation";
      id?: string;
      version?: number;
      question?: string;
      ids?: string[];
      kind?: "organize" | "copy" | "rename";
      smart?: boolean;
    }
  | {
      type: "bookmark-groups";
      ids?: string[];
      refs?: import("./organization-model").EntryRef[];
    }
  | { type: "property-batch"; ids: string[] }
  | { type: "danger"; title: string; text: string; action: Action };
interface DemoContext {
  state: State;
  dispatch: Dispatch<Action>;
  compact: boolean;
  open: (modal: Modal) => void;
  close: () => void;
  go: (page: Page) => void;
  guardNavigation: (dirty: boolean) => void;
}
export const Demo = createContext<DemoContext>(null!);
export const useDemo = () => useContext(Demo);
export const muted = "text-muted-foreground";
export const panel = "rounded-lg border border-border/70 bg-card";
export function Btn({
  className,
  children,
  ...props
}: ComponentProps<typeof Button>) {
  return (
    <Button
      {...props}
      className={cn(
        "inline-flex items-center justify-center h-9 min-w-9 gap-2 px-3 text-sm group-data-[compact=true]/viewport:min-h-[44px] max-[767px]:min-h-[44px] max-[767px]:min-w-[44px] [@media(pointer:coarse)]:min-h-11 [@media(pointer:coarse)]:min-w-11",
        className,
      )}
    >
      {children}
    </Button>
  );
}
export function IconBtn({
  label,
  children,
  ...props
}: ComponentProps<typeof Button> & { label: string }) {
  return (
    <Btn
      variant="ghost"
      size="icon"
      aria-label={label}
      title={label}
      {...props}
    >
      {children}
    </Btn>
  );
}
export function Choice({
  label,
  value,
  options,
  onChange,
  className,
}: {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (v: string) => void;
  className?: string;
}) {
  return (
    <Select
      value={value}
      onValueChange={(v) => v !== null && onChange(String(v))}
      items={options}
    >
      <SelectTrigger
        aria-label={label}
        className={cn("min-h-11 min-w-0 max-w-full text-[13px]", className)}
      >
        <SelectValue />
      </SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>
        {options.map((o) => (
          <SelectItem key={o.value} value={o.value} className="min-h-11 px-3">
            {o.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
export function Pill({
  children,
  tone = "neutral",
}: {
  children: ReactNode;
  tone?: "neutral" | "green" | "amber" | "red";
}) {
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium",
        tone === "green"
          ? "bg-accent text-primary"
          : tone === "amber"
            ? "bg-[var(--warning-soft)] text-[var(--warning)]"
            : tone === "red"
              ? "bg-[var(--danger-soft)] text-destructive"
              : "bg-muted text-muted-foreground",
      )}
    >
      {children}
    </span>
  );
}
export function TaskPill({ status }: { status: TaskStatus }) {
  const Icon =
    status === "done"
      ? Check
      : ["partial", "stale", "blocked"].includes(status)
        ? CircleAlert
        : Clock3;
  return (
    <Pill
      tone={
        status === "done"
          ? "green"
          : ["approval", "partial", "stale", "unknown"].includes(status)
            ? "amber"
            : "neutral"
      }
    >
      <Icon size={12} />
      {taskStatusText[status]}
    </Pill>
  );
}
export const kindIcons = {
  document: FileText,
  image: Image,
  video: Video,
  audio: Music2,
  sheet: Sheet,
};
export const kindNames: Record<FileKind, string> = {
  document: "文档",
  image: "图片",
  video: "视频",
  audio: "音频",
  sheet: "表格",
};
export function FileIcon({
  file,
  small = false,
}: {
  file: Pick<FileRecord, "kind">;
  small?: boolean;
}) {
  const Icon = kindIcons[file.kind];
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center justify-center rounded-lg",
        small ? "size-9" : "size-11",
        file.kind === "document"
          ? "bg-[var(--info-soft)] text-[var(--info)]"
          : file.kind === "image"
            ? "bg-accent text-primary"
            : file.kind === "audio"
              ? "bg-[var(--warning-soft)] text-[var(--warning)]"
              : "bg-muted text-muted-foreground",
      )}
    >
      <Icon size={small ? 17 : 21} strokeWidth={1.6} />
    </span>
  );
}
export function Origin({ file }: { file: FileRecord }) {
  const Icon =
    file.origin === "local"
      ? Laptop
      : file.origin === "agent"
        ? Bot
        : file.origin === "phone"
          ? Smartphone
          : Cloud;
  return (
    <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
      <Icon size={12} />
      {file.origin === "local"
        ? "本机文件"
        : file.origin === "agent"
          ? "Agent 产物"
          : file.origin === "phone"
            ? "手机导入"
            : "云端文件"}
      {file.offlineVersion && " · 可离线"}
    </span>
  );
}
export function Heading({
  title,
  note,
  children,
}: {
  title: string;
  note?: string;
  children?: ReactNode;
}) {
  return (
    <header
      data-page-heading
      className="mb-3 flex h-11 shrink-0 items-center gap-2 border-b border-border/60"
    >
      <h1 className="min-w-0 truncate text-sm font-semibold">{title}</h1>
      {note && (
        <Popover>
          <PopoverTrigger
            render={
              <IconBtn
                label={`${title}说明`}
                className="!size-8 !min-h-0 !min-w-0 max-[767px]:!size-11"
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
      {children && (
        <div className="ml-auto flex min-w-0 shrink items-center gap-1 overflow-x-auto">
          {children}
        </div>
      )}
    </header>
  );
}
export function SectionTitle({
  title,
  action,
  onAction,
}: {
  title: string;
  action?: string;
  onAction?: () => void;
}) {
  return (
    <div className="mb-3 flex items-center justify-between gap-3">
      <h2 className="text-[15px] font-semibold">{title}</h2>
      {action && (
        <Btn
          variant="ghost"
          onClick={onAction}
          className="text-muted-foreground"
        >
          {action}
          <ChevronRight size={14} />
        </Btn>
      )}
    </div>
  );
}
export function Empty({
  title,
  text,
  action,
  onAction,
}: {
  title: string;
  text: string;
  action?: string;
  onAction?: () => void;
}) {
  return (
    <div className="flex min-h-36 flex-col items-center justify-center px-4 py-5 text-center">
      <span className="mb-2 flex size-8 items-center justify-center rounded-lg bg-accent text-primary">
        <FileText size={17} strokeWidth={1.3} />
      </span>
      <h2 className="font-semibold">{title}</h2>
      <p className="mt-2 max-w-sm text-sm leading-6 text-muted-foreground">
        {text}
      </p>
      {action && (
        <Btn className="mt-3" onClick={onAction}>
          {action}
          <ArrowUpRight size={14} />
        </Btn>
      )}
    </div>
  );
}
export function FileArt({
  file,
  large = false,
  version,
}: {
  file: FileRecord;
  large?: boolean;
  version?: number;
}) {
  const selectedVersion = version || file.version;
  const variant =
    (file.snapshots?.find((s) => s.version === selectedVersion)?.accent ||
      selectedVersion) + Number(file.id.replace(/\D/g, "") || 1);
  if (file.kind === "image" || file.kind === "video") {
    const sea = variant % 3 === 0;
    const dusk = variant % 3 === 1;
    return (
      <div
        className={cn(
          "@container relative isolate w-full overflow-hidden",
          large ? "aspect-[4/3]" : "aspect-[4/3]",
        )}
      >
        <svg
          viewBox="0 0 600 450"
          preserveAspectRatio="xMidYMid slice"
          className="h-full w-full"
          role="img"
          aria-label={`${file.name} · v${selectedVersion} 样例封面`}
        >
          <rect
            width="600"
            height="450"
            fill={
              sea
                ? "var(--cover-sea-sky)"
                : dusk
                  ? "var(--cover-sand-sky)"
                  : "var(--cover-pine-sky)"
            }
          />
          <circle
            cx={sea ? 450 : 135}
            cy={dusk ? 130 : 85}
            r={dusk ? 63 : 39}
            fill="var(--cover-sun)"
          />
          {sea ? (
            <>
              <path
                d="M0 208Q140 170 310 212T600 200V450H0Z"
                fill="var(--cover-sea)"
              />
              <path
                d="M0 304Q160 246 330 310T600 280V450H0Z"
                fill="var(--cover-sea-deep)"
              />
              <path
                d="M0 395Q120 265 256 338T600 388V450H0Z"
                fill="var(--cover-sand)"
              />
              <path
                d="M0 310Q130 260 290 308T600 300"
                fill="none"
                stroke="var(--cover-sun)"
                strokeWidth="3"
                opacity=".6"
              />
            </>
          ) : (
            <>
              <path
                d="M0 300L165 115L287 278L440 145L600 285V450H0Z"
                fill={dusk ? "var(--cover-clay)" : "var(--cover-ridge)"}
              />
              <path
                d="M0 380L105 268L255 362L379 237L600 363V450H0Z"
                fill={dusk ? "var(--cover-clay-deep)" : "var(--cover-forest)"}
              />
              <path
                d="M0 423Q130 335 270 411T600 383V450H0Z"
                fill="var(--cover-ink)"
              />
              {!dusk &&
                [32, 87, 506, 554].map((x, i) => (
                  <path
                    key={x}
                    d={`M${x} 160l-${24 + i * 2} 115h${48 + i * 4}zM${x} 247v120`}
                    fill="var(--cover-forest)"
                    stroke="var(--cover-forest)"
                    strokeWidth="5"
                  />
                ))}
            </>
          )}
        </svg>
        <span className="absolute left-3 top-3 rounded-md bg-card/90 px-2 py-1 text-xs font-medium text-foreground">
          {file.kind === "video" ? "影像" : "视觉参考"}
        </span>
        <span className="absolute bottom-3 left-3 hidden text-sm font-medium text-white @min-[180px]:block">
          {sea ? "潮汐与归途" : dusk ? "日落以前" : "山野之间"}
        </span>
        <span className="absolute bottom-3 right-3 rounded bg-black/35 px-1.5 py-0.5 text-xs text-white">
          {file.kind === "video" ? "00:42" : `v${selectedVersion}`}
        </span>
      </div>
    );
  }
  if (file.kind === "audio")
    return (
      <div className="flex aspect-[4/3] flex-col justify-center bg-[var(--cover-audio)] p-5">
        <div
          className="mb-6 flex items-end justify-center gap-1.5"
          aria-label="示例波形"
        >
          {Array.from({ length: 23 }, (_, i) => (
            <span
              key={i}
              className={cn(
                "w-1 rounded bg-[var(--cover-audio-ink)]",
                i % 4 === 0 ? "h-12" : i % 3 === 0 ? "h-6" : "h-9",
              )}
            />
          ))}
        </div>
        <p className="text-sm font-medium text-[var(--cover-audio-ink)]">
          山风与溪流
        </p>
        <p className="mt-1 text-xs text-[var(--cover-audio-ink)]">
          环境录音 · 00:42 · WAV
        </p>
      </div>
    );
  return (
    <div className="@container relative flex aspect-[4/3] items-center justify-center overflow-hidden bg-muted p-3">
      <div
        className={cn(
          "relative h-full w-4/5 -rotate-3 rounded-sm border border-border bg-card p-2 shadow-sm @min-[180px]:p-4",
          file.kind === "sheet" && "rotate-2",
        )}
      >
        <div className="flex items-center justify-between">
          <FileIcon file={file} small />
          <span className="hidden text-xs text-muted-foreground @min-[180px]:inline">
            {file.kind === "sheet" ? "XLSX" : "PDF / DOC"}
          </span>
        </div>
        <p className="mt-3 line-clamp-2 text-sm font-semibold leading-5">
          {file.name.split(" · ")[0]}
        </p>
        <div className="mt-3 space-y-2">
          {["w-full", "w-4/5", "w-2/3"].map((w) => (
            <div key={w} className={cn("h-1 rounded bg-border", w)} />
          ))}
        </div>
        {file.kind === "sheet" && (
          <div className="mt-3 grid grid-cols-4 gap-1">
            {Array.from({ length: 12 }, (_, i) => (
              <span key={i} className="h-2 bg-accent" />
            ))}
          </div>
        )}
      </div>
      <span className="absolute bottom-3 right-3 rounded-md bg-card px-2 py-1 text-xs text-muted-foreground">
        {file.metadata || "内容预览"}
      </span>
    </div>
  );
}
