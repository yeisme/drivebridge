import { useLayoutEffect, useRef, useState } from "react";
import { ArrowLeft, ArrowRight, Copy, X, Eye } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Popover } from "@base-ui/react/popover";
import { Btn, Choice, Empty, IconBtn, useDemo } from "./ui";
import { PreviewReadingTools } from "./PreviewReadingTools";
import {
  defaultSheetReading,
  defaultSheetPage,
  sheetPage,
  sheetSamples,
  sheetColumn,
  sheetWidth,
} from "./sheet-preview-model";
import type { SheetReading, SheetPageView } from "./sheet-preview-model";
export function SheetPreview({
  version,
  reading,
  change,
  toolbar,
}: {
  version: number;
  reading?: SheetReading;
  change: (reading: SheetReading) => void;
  toolbar: HTMLElement | null;
}) {
  const { state: s, dispatch } = useDemo(),
    data = reading || defaultSheetReading(),
    samples = sheetSamples(version),
    sample = samples.find((x) => x.id === data.active) || samples[0],
    v = data.sheets[sample.id] || defaultSheetPage();
  const result = sheetPage(sample, v),
    reader = useRef<HTMLDivElement>(null),
    drag = useRef<{ column: number; start: number; width: number } | null>(
      null,
    ),
    latest = useRef({ data, v });
  latest.current = { data, v };
  const [full, setFull] = useState<{
      row: number;
      column: number;
      value: string;
    } | null>(null),
    [copied, setCopied] = useState("");
  const anchor = useRef<HTMLElement | null>(null);
  const restoreFocus = useRef(false);
  const copyGeneration = useRef(0);
  const disclosureFrame = useRef(0);
  useLayoutEffect(
    () => () => cancelAnimationFrame(disclosureFrame.current),
    [],
  );
  const close = (restore = false) => {
    cancelAnimationFrame(disclosureFrame.current);
    restoreFocus.current = restore;
    copyGeneration.current++;
    setFull(null);
  };
  const patch = (p: Partial<SheetPageView>) => {
    const { data, v } = latest.current;
    change({
      ...data,
      sheets: { ...data.sheets, [sample.id]: { ...v, ...p } },
    });
  };
  const restoration = useRef(false);
  useLayoutEffect(() => {
    close();
    restoration.current = true;
    if (reader.current) {
      reader.current.scrollTop = v.scrollTop;
      reader.current.scrollLeft = v.scrollLeft;
    }
    restoration.current = false;
  }, [sample.id, result.page, v.query, data.state]);
  const select = (row: number, column: number) =>
    patch({ cell: { row, column } });
  const show = (row: number, column: number) => {
    select(row, column);
    anchor.current =
      reader.current?.querySelector<HTMLElement>(
        `[data-row="${row}"][data-column="${column}"]`,
      ) || null;
    setCopied("");
    anchor.current?.scrollIntoView({ block: "nearest", inline: "nearest" });
    cancelAnimationFrame(disclosureFrame.current);
    disclosureFrame.current = requestAnimationFrame(() => {
      if (anchor.current?.isConnected)
        setFull({ row, column, value: String(sample.row(row)[column] ?? "") });
    });
  };
  const copy = async (row: number, column: number) => {
    const generation = ++copyGeneration.current;
    const value = String(sample.row(row)[column] ?? "");
    try {
      await navigator.clipboard.writeText(value);
      if (generation !== copyGeneration.current) return;
      setCopied("已复制");
      if (!full)
        dispatch({
          type: "notice",
          text: `已复制 ${sheetColumn(column)}${row + 1}`,
        });
    } catch {
      if (generation !== copyGeneration.current) return;
      if (!full) show(row, column);
      setCopied("剪贴板不可用，可选中下方文字手动复制。");
    }
  };
  const pageTo = (page: number) =>
    patch({ page, scrollTop: 0, cell: undefined });
  const columns = sample.columns.map((_, i) => v.widths[i] || sample.widths[i]);
  const pageControls = (
    <div className="flex items-center gap-1">
      <IconBtn
        label="表格上一页"
        disabled={result.page <= 1 || data.state !== "ready"}
        onClick={() => pageTo(result.page - 1)}
      >
        <ArrowLeft size={15} />
      </IconBtn>
      <span className="shrink-0 text-xs tabular-nums">
        {result.page} / {result.pages}
      </span>
      <IconBtn
        label="表格下一页"
        disabled={result.page >= result.pages || data.state !== "ready"}
        onClick={() => pageTo(result.page + 1)}
      >
        <ArrowRight size={15} />
      </IconBtn>
    </div>
  );
  return (
    <div
      className="flex h-full min-h-0 min-w-0 flex-col p-2"
      data-sheet-preview
    >
      <PreviewReadingTools
        toolbar={toolbar}
        summary={`${sample.name} · ${result.page}/${result.pages}`}
        inline={pageControls}
      >
        <label className="block space-y-1 text-xs">
          <span>工作表 · 模拟样本</span>
          <Choice
            label="工作表"
            className="w-full"
            value={sample.id}
            options={samples.map((x) => ({ value: x.id, label: x.name }))}
            onChange={(active) => change({ ...data, active, state: "ready" })}
          />
        </label>
        <label className="block space-y-1 text-xs">
          <span>查找此工作表</span>
          <Input
            aria-label="查找表格内容"
            placeholder="在当前工作表的样本行中查找"
            value={v.query}
            onChange={(e) =>
              patch({
                query: e.target.value,
                page: 1,
                scrollTop: 0,
                cell: undefined,
              })
            }
          />
        </label>
        <div className="flex items-center justify-between gap-2 text-xs">
          <span>
            {result.total} 行 · {sample.columns.length} 列 · 每页 50 行
          </span>
          {pageControls}
        </div>
        <label className="block space-y-1 text-xs">
          <span>跳转页码</span>
          <Input
            type="number"
            min={1}
            max={result.pages}
            aria-label="跳转表格页码"
            value={result.page}
            onChange={(e) =>
              pageTo(
                Math.max(
                  1,
                  Math.min(result.pages, Number(e.target.value) || 1),
                ),
              )
            }
          />
        </label>
        <Btn variant="outline" onClick={() => patch({ widths: {} })}>
          恢复默认列宽
        </Btn>
        <label className="block space-y-1 border-t pt-3 text-xs">
          <span>演示状态</span>
          <Choice
            label="表格演示状态"
            className="w-full"
            value={data.state}
            options={[
              { value: "ready", label: "正常" },
              { value: "loading", label: "加载中" },
              { value: "failed", label: "读取失败" },
            ]}
            onChange={(state) =>
              change({ ...data, state: state as SheetReading["state"] })
            }
          />
        </label>
        <p className="text-[11px] leading-5 text-muted-foreground">
          仅展示可重置样本，不解析真实工作簿，不执行宏、公式或脚本。
        </p>
      </PreviewReadingTools>
      {data.state !== "ready" ? (
        <Empty
          title={data.state === "loading" ? "正在读取表格…" : "表格读取失败"}
          text="演示状态；当前工作表和阅读位置保留。"
          action={data.state === "loading" ? "结束模拟加载" : "重试当前工作表"}
          onAction={() => change({ ...data, state: "ready" })}
        />
      ) : !sample.count ? (
        <Empty
          title="这个工作表是空的"
          text="0 行样本；可在阅读设置中选择其他工作表。"
        />
      ) : !result.rows.length ? (
        <Empty
          title="当前工作表没有匹配项"
          text="没有改变原表格内容。"
          action="清除表格查找"
          onAction={() => patch({ query: "", page: 1, cell: undefined })}
        />
      ) : (
        <div
          ref={reader}
          data-sheet-scroll
          role="region"
          aria-label="只读表格网格"
          className="min-h-0 min-w-0 flex-1 overflow-auto overscroll-contain outline-none"
          tabIndex={0}
          onScroll={(e) => {
            if (full) close();
            if (!restoration.current)
              patch({
                scrollTop: e.currentTarget.scrollTop,
                scrollLeft: e.currentTarget.scrollLeft,
              });
          }}
          onKeyDown={(e) => {
            if ((e.target as HTMLElement).closest('[role="separator"]')) return;
            const cell = (e.target as HTMLElement).closest<HTMLElement>(
              "[data-sheet-cell]",
            );
            if (!cell) return;
            const row = Number(cell.dataset.row),
              column = Number(cell.dataset.column);
            const ri = result.rows.findIndex((x) => x.index === row);
            if (
              (e.ctrlKey || e.metaKey) &&
              e.key.toLowerCase() === "c" &&
              !window.getSelection()?.toString() &&
              !(e.target as HTMLElement).closest(
                "input,textarea,[contenteditable]",
              )
            ) {
              e.preventDefault();
              void copy(row, column);
              return;
            }
            if (e.key === "Enter") {
              e.preventDefault();
              show(row, column);
              return;
            }
            const dr = e.key === "ArrowDown" ? 1 : e.key === "ArrowUp" ? -1 : 0,
              dc = e.key === "ArrowRight" ? 1 : e.key === "ArrowLeft" ? -1 : 0;
            if (dr || dc) {
              e.preventDefault();
              e.stopPropagation();
              const next =
                  result.rows[
                    Math.max(0, Math.min(result.rows.length - 1, ri + dr))
                  ].index,
                nc = Math.max(
                  0,
                  Math.min(sample.columns.length - 1, column + dc),
                );
              select(next, nc);
              reader.current
                ?.querySelector<HTMLElement>(
                  `[data-row="${next}"][data-column="${nc}"]`,
                )
                ?.focus();
            }
          }}
        >
          <table
            className="table-fixed border-separate border-spacing-0 text-xs"
            style={{ width: 36 + columns.reduce((sum, w) => sum + w, 0) }}
            aria-label={`${sample.name} · 只读演示`}
          >
            <caption className="sr-only">
              固定版本 v{version}，当前页 {result.rows.length} 行；不编辑内容。
            </caption>
            <colgroup>
              <col style={{ width: 36 }} />
              {columns.map((w, i) => (
                <col key={i} style={{ width: w }} />
              ))}
            </colgroup>
            <thead className="sticky top-0 z-10">
              <tr className="h-7 max-[767px]:h-11 [@media(pointer:coarse)]:h-11">
                <th className="sticky left-0 border-b border-r border-border/70 bg-muted text-[10px] font-normal text-muted-foreground">
                  #
                </th>
                {sample.columns.map((label, i) => (
                  <th
                    key={i}
                    scope="col"
                    className="relative border-b border-r border-border/70 bg-muted px-2 text-left font-medium"
                  >
                    <span className="block truncate">{label}</span>
                    <span
                      role="separator"
                      tabIndex={0}
                      aria-label={`调整 ${label} 列宽`}
                      aria-orientation="vertical"
                      aria-valuemin={72}
                      aria-valuemax={480}
                      aria-valuenow={columns[i]}
                      className="absolute bottom-0 right-0 top-0 w-2 cursor-col-resize touch-none border-r-2 border-transparent hover:border-primary focus:border-primary max-[767px]:w-11 [@media(pointer:coarse)]:w-11"
                      onDoubleClick={() =>
                        patch({
                          widths: { ...v.widths, [i]: sample.widths[i] },
                        })
                      }
                      onKeyDown={(e) => {
                        if (
                          ["ArrowLeft", "ArrowRight", "Home"].includes(e.key)
                        ) {
                          e.preventDefault();
                          e.stopPropagation();
                          patch({
                            widths: {
                              ...v.widths,
                              [i]:
                                e.key === "Home"
                                  ? sample.widths[i]
                                  : sheetWidth(
                                      columns[i] +
                                        (e.key === "ArrowRight" ? 16 : -16),
                                    ),
                            },
                          });
                        }
                      }}
                      onPointerDown={(e) => {
                        drag.current = {
                          column: i,
                          start: e.clientX,
                          width: columns[i],
                        };
                        e.currentTarget.setPointerCapture(e.pointerId);
                      }}
                      onPointerMove={(e) => {
                        if (drag.current)
                          patch({
                            widths: {
                              ...latest.current.v.widths,
                              [i]: sheetWidth(
                                drag.current.width +
                                  e.clientX -
                                  drag.current.start,
                              ),
                            },
                          });
                      }}
                      onPointerUp={() => (drag.current = null)}
                      onPointerCancel={() => (drag.current = null)}
                    />
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {result.rows.map((row) => (
                <tr
                  key={row.index}
                  data-sheet-row
                  className={`${s.preferences.density === "compact" ? "h-6" : "h-7"} max-[767px]:h-11 [@media(pointer:coarse)]:h-11`}
                >
                  <th
                    scope="row"
                    className="sticky left-0 border-b border-r border-border/50 bg-muted px-1 text-right text-[10px] font-normal tabular-nums text-muted-foreground"
                  >
                    {row.index + 1}
                  </th>
                  {row.cells.map((value, i) => {
                    const active =
                      v.cell?.row === row.index && v.cell.column === i;
                    return (
                      <td
                        key={i}
                        data-sheet-cell
                        data-row={row.index}
                        data-column={i}
                        tabIndex={
                          active ||
                          (!v.cell &&
                            row.index === result.rows[0].index &&
                            i === 0)
                            ? 0
                            : -1
                        }
                        aria-label={`${sheetColumn(i)}${row.index + 1} ${value}`}
                        aria-current={active ? "true" : undefined}
                        className={`relative cursor-default overflow-hidden border-b border-r border-border/50 px-2 py-0 outline-none focus:ring-1 focus:ring-inset focus:ring-primary ${active ? "bg-primary/10" : "bg-card hover:bg-muted/50"} ${typeof value === "number" ? "text-right tabular-nums" : "text-left"}`}
                        onFocus={() => {
                          if (!active) select(row.index, i);
                        }}
                        onClick={() => select(row.index, i)}
                        onDoubleClick={() => show(row.index, i)}
                        title={String(value)}
                      >
                        <span
                          className={`block truncate leading-5 ${active ? "max-[767px]:pr-9 [@media(pointer:coarse)]:pr-9" : ""}`}
                        >
                          {value}
                        </span>
                        {active && (
                          <button
                            aria-label="查看所选单元格内容"
                            className="absolute right-0 top-0 hidden h-full w-11 items-center justify-center bg-accent max-[767px]:flex [@media(pointer:coarse)]:flex"
                            onClick={(e) => {
                              e.stopPropagation();
                              show(row.index, i);
                            }}
                          >
                            <Eye size={14} />
                          </button>
                        )}
                      </td>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <Popover.Root
        open={!!full}
        modal={false}
        onOpenChange={(value, event) => {
          if (!value) close(event.reason === "escape-key");
        }}
      >
        <Popover.Portal>
          <Popover.Positioner
            anchor={anchor}
            side="bottom"
            align="start"
            sideOffset={6}
            collisionPadding={8}
            className="z-50"
          >
            <Popover.Popup
              aria-label="单元格内容"
              finalFocus={() => (restoreFocus.current ? anchor.current : false)}
              className="w-max min-w-40 max-w-[min(320px,calc(100vw-16px))] rounded-lg border bg-popover p-2.5 text-popover-foreground shadow-md outline-none"
              data-cell-popover
            >
              <div className="mb-2 flex items-center gap-2">
                <Popover.Title className="min-w-0 flex-1 truncate text-xs font-medium">
                  {full
                    ? `${sheetColumn(full.column)}${full.row + 1} · ${sample.columns[full.column]}`
                    : "单元格"}
                </Popover.Title>
                <IconBtn
                  label="复制单元格"
                  className="!size-8 !min-h-0 !min-w-0 max-[767px]:!size-11 [@media(pointer:coarse)]:!size-11"
                  onClick={() => full && void copy(full.row, full.column)}
                >
                  <Copy size={14} />
                </IconBtn>
                <IconBtn
                  label="关闭单元格内容"
                  className="!size-8 !min-h-0 !min-w-0 max-[767px]:!size-11 [@media(pointer:coarse)]:!size-11"
                  onClick={() => close(true)}
                >
                  <X size={14} />
                </IconBtn>
              </div>
              <div
                aria-label="单元格完整内容"
                tabIndex={0}
                className="max-h-60 overflow-auto whitespace-pre-wrap break-words px-1 text-[13px] leading-6 select-text"
              >
                {full?.value || "（空值）"}
              </div>
              {copied && (
                <p
                  role="status"
                  className="mt-2 border-t pt-2 text-[11px] text-muted-foreground"
                >
                  {copied}
                </p>
              )}
            </Popover.Popup>
          </Popover.Positioner>
        </Popover.Portal>
      </Popover.Root>
    </div>
  );
}
