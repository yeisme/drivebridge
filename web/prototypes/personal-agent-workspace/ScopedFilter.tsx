import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
import { useState } from "react";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { SlidersHorizontal } from "lucide-react";
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
import { Btn, Choice, useDemo } from "./ui";
import { TagPicker } from "./TagWorkspace";
import { PropertyFilters } from "./UnifiedFind";
import { emptySearch } from "./cloud-model";
import type { SearchSpec } from "./cloud-model";
export const filterCount = (q: SearchSpec) =>
  (q.kind !== "all" ? 1 : 0) +
  (q.updated !== "all" ? 1 : 0) +
  q.tagIds.length +
  (q.documentTag ? 1 : 0) +
  (q.propertyConditions?.length || 0) +
  (q.onlyFavorites ? 1 : 0) +
  (q.favoriteGroup ? 1 : 0) +
  (q.favoriteDirectories ? 1 : 0) +
  (q.propertyIssue ? 1 : 0) +
  (q.minSize !== undefined ? 1 : 0) +
  (q.maxSize !== undefined ? 1 : 0);
export function ScopedFilter({
  value,
  onApply,
  label = "筛选条件",
  allowConnections = false,
}: {
  value: SearchSpec;
  onApply: (q: SearchSpec) => void;
  label?: string;
  allowConnections?: boolean;
}) {
  const { state: s, compact } = useDemo();
  const [show, setShow] = useState(false),
    [draft, setDraft] = useState(value);
  const toggle = (v: boolean) => {
    if (v) setDraft(structuredClone(value));
    setShow(v);
  };
  const patch = (q: Partial<SearchSpec>) => setDraft((d) => ({ ...d, ...q }));
  const connections = s.connections.filter(
    (c) => c.id === "drive" || c.space === s.space,
  );
  const content = (
    <>
      <div className="min-h-0 flex-1 space-y-4 overflow-auto p-4">
        <p className="text-xs text-muted-foreground">
          {value.connections
            .map((id) => s.connections.find((c) => c.id === id)?.name)
            .join("、")}{" "}
          /{" "}
          {s.storage.nodes.find((n) => n.id === value.directoryId)?.path ||
            "连接根目录"}
        </p>
        {allowConnections && (
          <section className="space-y-2">
            <h3 className="text-xs font-medium">连接范围 · 跨连接需显式选择</h3>
            <div className="grid grid-cols-2 gap-2">
              {connections.map((c) => (
                <label
                  key={c.id}
                  className="flex min-h-11 items-center gap-2 text-xs"
                >
                  <Checkbox
                    checked={draft.connections.includes(c.id)}
                    onCheckedChange={(value) =>
                      patch({
                        connections: value
                          ? [...draft.connections, c.id]
                          : draft.connections.filter((id) => id !== c.id),
                        directoryId: undefined,
                        folder: "全部文件",
                      })
                    }
                  />
                  {c.name}
                </label>
              ))}
            </div>
            <label className="block text-xs">限定目录</label>
            <Choice
              className="w-full"
              label="限定目录"
              value={draft.directoryId || "all"}
              onChange={(value) =>
                patch({ directoryId: value === "all" ? undefined : value })
              }
              options={[
                { value: "all", label: "所选连接的全部目录" },
                ...s.storage.nodes
                  .filter(
                    (n) =>
                      n.space === s.space &&
                      draft.connections.includes(n.connectionId),
                  )
                  .map((n) => ({
                    value: n.id,
                    label: `${connections.find((c) => c.id === n.connectionId)?.name} / ${n.path}`,
                  })),
              ]}
            />
          </section>
        )}
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <label className="space-y-1 text-xs">
            <span className="block text-muted-foreground">目录范围</span>
            <Choice
              label="目录范围"
              value={draft.recursive === false ? "level" : "recursive"}
              className="w-full"
              onChange={(v) => patch({ recursive: v === "recursive" })}
              options={[
                { value: "recursive", label: "当前目录及子目录" },
                { value: "level", label: "仅当前层" },
              ]}
            />
          </label>
          <label className="space-y-1 text-xs">
            <span className="block text-muted-foreground">文件类型</span>
            <Choice
              label="文件类型"
              value={draft.kind}
              className="w-full"
              onChange={(kind) => patch({ kind })}
              options={[
                ["all", "全部类型"],
                ["document", "文档"],
                ["pdf", "PDF"],
                ["image", "图片"],
                ["video", "视频"],
                ["audio", "音频"],
                ["sheet", "表格"],
              ].map(([value, label]) => ({ value, label }))}
            />
          </label>
          <label className="space-y-1 text-xs">
            <span className="block text-muted-foreground">更新时间</span>
            <Choice
              label="更新时间"
              value={draft.updated}
              className="w-full"
              onChange={(updated) => patch({ updated })}
              options={[
                { value: "all", label: "所有时间" },
                { value: "week", label: "最近七天" },
                { value: "last-month", label: "上个月" },
              ]}
            />
          </label>
          <label className="space-y-1 text-xs">
            <span className="block text-muted-foreground">标签组合</span>
            <Choice
              label="标签组合"
              value={draft.tagMode}
              className="w-full"
              onChange={(v) => patch({ tagMode: v as SearchSpec["tagMode"] })}
              options={[
                { value: "all", label: "包含全部" },
                { value: "any", label: "包含任意" },
                { value: "exclude", label: "排除" },
              ]}
            />
          </label>
        </div>
        <div className="grid grid-cols-2 gap-3">
          {["minSize", "maxSize"].map((key) => (
            <label key={key} className="space-y-1 text-xs">
              <span>{key === "minSize" ? "最小大小" : "最大大小"}（MB）</span>
              <Input
                aria-label={
                  key === "minSize" ? "最小大小（MB）" : "最大大小（MB）"
                }
                type="number"
                min={0}
                value={
                  draft[key as "minSize" | "maxSize"] === undefined
                    ? ""
                    : draft[key as "minSize" | "maxSize"]! / 1e6
                }
                onChange={(e) =>
                  patch({
                    [key]: e.target.value
                      ? Math.max(0, Number(e.target.value)) * 1e6
                      : undefined,
                  })
                }
              />
            </label>
          ))}
        </div>
        <section className="space-y-2">
          <h3 className="text-xs font-medium">平台标签</h3>
          <TagPicker
            ids={draft.tagIds}
            onChange={(tagIds) => patch({ tagIds })}
          />
        </section>
        <PropertyFilters query={draft} onChange={patch} />
        {draft.tab === "content" && (
          <label className="flex min-h-11 items-center gap-2 text-xs">
            <Checkbox
              checked={draft.history}
              onCheckedChange={(value) => patch({ history: !!value })}
            />
            包含已索引历史版本
          </label>
        )}
      </div>
      <footer className="flex h-14 shrink-0 items-center gap-2 border-t bg-popover px-4">
        <Btn
          variant="ghost"
          className="mr-auto"
          onClick={() =>
            setDraft({
              ...emptySearch(),
              connections: value.connections,
              directoryId: value.directoryId,
              query: value.query,
              recursive: value.recursive,
              tab: value.tab,
            })
          }
        >
          重置
        </Btn>
        <Btn variant="outline" onClick={() => toggle(false)}>
          取消
        </Btn>
        <Btn
          disabled={
            !draft.connections.length ||
            (draft.minSize !== undefined &&
              draft.maxSize !== undefined &&
              draft.minSize > draft.maxSize)
          }
          onClick={() => {
            onApply(draft);
            toggle(false);
          }}
        >
          {allowConnections ? "应用并检索" : "应用筛选"}
        </Btn>
      </footer>
    </>
  );
  const trigger = (
    <Btn
      variant="ghost"
      aria-label={label}
      className="!h-8 !min-w-8 !px-2 [@media(pointer:coarse)]:!size-11"
    >
      <SlidersHorizontal size={16} />
      {filterCount(value) > 0 && (
        <span className="text-[10px]">{filterCount(value)}</span>
      )}
    </Btn>
  );
  if (compact)
    return (
      <>
        {
          <Btn variant="ghost" aria-label={label} onClick={() => toggle(true)}>
            <SlidersHorizontal size={16} />
            {filterCount(value) || ""}
          </Btn>
        }
        <Dialog open={show} onOpenChange={toggle}>
          <DialogContent className="!bottom-0 !left-0 !top-auto !max-h-[88dvh] !w-full !max-w-none !translate-x-0 !translate-y-0 !gap-0 !rounded-b-none !p-0">
            <div className="p-4">
              <DialogTitle>筛选文件</DialogTitle>
              <DialogDescription>
                应用前保留当前结果，取消不改变条件。
              </DialogDescription>
            </div>
            {content}
          </DialogContent>
        </Dialog>
      </>
    );
  return (
    <Popover open={show} onOpenChange={toggle} modal={false}>
      <PopoverTrigger render={trigger} />
      <PopoverContent
        align="end"
        className="!w-[420px] !max-w-[calc(100vw-24px)] !max-h-[min(680px,calc(100dvh-100px))] !gap-0 !overflow-hidden !p-0"
      >
        <div className="border-b px-4 py-3">
          <PopoverTitle>筛选文件</PopoverTitle>
          <PopoverDescription className="text-xs">
            应用前保留当前结果，取消不改变条件。
          </PopoverDescription>
        </div>
        {content}
      </PopoverContent>
    </Popover>
  );
}

export function FilterConditions({
  value: q,
  onChange,
}: {
  value: SearchSpec;
  onChange: (q: SearchSpec) => void;
}) {
  const { state: s } = useDemo();
  const items: { label: string; clear: Partial<SearchSpec> }[] = [];
  if (q.kind !== "all")
    items.push({ label: `类型：${q.kind}`, clear: { kind: "all" } });
  if (q.updated !== "all")
    items.push({ label: `时间：${q.updated}`, clear: { updated: "all" } });
  if (q.documentTag)
    items.push({
      label: `#${q.documentTag}${q.exactTag ? " · 仅此标签" : " · 含子标签"}`,
      clear: { documentTag: undefined, exactTag: false },
    });
  q.tagIds.forEach((id) =>
    items.push({
      label: `标签：${s.tags.find((t) => t.id === id)?.name || id}`,
      clear: { tagIds: q.tagIds.filter((x) => x !== id) },
    }),
  );
  q.propertyConditions?.forEach((c, i) =>
    items.push({
      label: `${s.organization.definitions[s.space].find((d) => d.key === c.key)?.label || c.key} · ${c.op} · ${String(c.value ?? "")}`,
      clear: {
        propertyConditions: q.propertyConditions?.filter((_, j) => i !== j),
      },
    }),
  );
  if (q.propertyIssue)
    items.push({
      label: `属性状态：${{ type: "类型不一致", invalid: "解析失败", stale: "过期投影", unscanned: "未提取" }[q.propertyIssue.state]}`,
      clear: { propertyIssue: undefined },
    });
  if (q.onlyFavorites || q.favoriteGroup)
    items.push({
      label: "仅收藏",
      clear: { onlyFavorites: false, favoriteGroup: undefined },
    });
  if (q.favoriteDirectories)
    items.push({ label: "收藏目录内", clear: { favoriteDirectories: false } });
  if (q.minSize !== undefined || q.maxSize !== undefined)
    items.push({
      label: "大小范围",
      clear: { minSize: undefined, maxSize: undefined },
    });
  if (!items.length) return null;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Btn
            variant="ghost"
            aria-label="已生效条件"
            className="!h-8 !px-2 text-[10px] [@media(pointer:coarse)]:!h-11 max-[767px]:!h-11"
          >
            {items.length} 个条件
          </Btn>
        }
      />
      <DropdownMenuContent
        align="end"
        className="!w-72 !max-w-[calc(100vw-24px)]"
      >
        <p className="px-2 py-2 text-xs text-muted-foreground">点击移除条件</p>
        {items.map((item, i) => (
          <DropdownMenuItem
            key={i}
            className="min-h-11 justify-between gap-2"
            onClick={() => onChange({ ...q, ...item.clear })}
          >
            <span className="min-w-0 truncate">{item.label}</span>
            <span aria-hidden>×</span>
          </DropdownMenuItem>
        ))}
        {q.documentTag && (
          <DropdownMenuItem
            className="min-h-11 border-t"
            onClick={() => onChange({ ...q, exactTag: !q.exactTag })}
          >
            {q.exactTag ? "包含后代标签" : "仅此标签"}
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
