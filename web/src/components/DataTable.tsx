import { EmptyState } from "./EmptyState";
import { AsyncState } from "./AsyncState";
import { DataTableToolbar } from "./DataTableToolbar";
import {
  Children,
  isValidElement,
  useMemo,
  useState,
  useEffect,
  type ReactNode,
} from "react";
import {
  useTable,
  tableFeatures,
  rowSortingFeature,
  rowPaginationFeature,
  rowSelectionFeature,
  columnVisibilityFeature,
  columnFilteringFeature,
  globalFilteringFeature,
  createSortedRowModel,
  createFilteredRowModel,
  createPaginatedRowModel,
  type ColumnDef,
} from "@tanstack/react-table";
import { ArrowUpDown, Check, Copy, Columns3 } from "lucide-react";
import {
  Table,
  TableHeader,
  TableHead,
  TableBody,
  TableCell,
  TableRow,
} from "./ui/table";
import { Button } from "./ui/button";
import { Checkbox } from "./ui/checkbox";
import { Input } from "./ui/input";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuCheckboxItem,
} from "./ui/dropdown-menu";
import { SelectField } from "./SelectField";
import { Skeleton } from "./ui/skeleton";

const features = tableFeatures({
  rowSortingFeature,
  rowPaginationFeature,
  rowSelectionFeature,
  columnVisibilityFeature,
  columnFilteringFeature,
  globalFilteringFeature,
  sortedRowModel: createSortedRowModel(),
  filteredRowModel: createFilteredRowModel(),
  paginatedRowModel: createPaginatedRowModel(),
});
export type DataColumn<T> = {
  id: string;
  header: ReactNode;
  cell: (row: T) => ReactNode;
  value?: (row: T) => string | number;
  sortable?: boolean;
  align?: "left" | "right";
  width?: string;
};
export function nodeText(node: ReactNode): string {
  return Children.toArray(node)
    .map((child) =>
      isValidElement<{ children?: ReactNode }>(child)
        ? nodeText(child.props.children)
        : typeof child === "string" || typeof child === "number"
          ? String(child)
          : "",
    )
    .join(" ")
    .trim();
}
export function DataTable<T extends object>({
  data,
  columns,
  getRowId,
  loading = false,
  error,
  onRetry,
  label = "数据表",
  pageSize = 20,
  selectionActions,
  pagination = "internal",
  searchable = true,
}: {
  data: T[];
  columns: DataColumn<T>[];
  getRowId: (row: T) => string;
  loading?: boolean;
  error?: string;
  onRetry?: () => void;
  label?: string;
  pageSize?: number;
  pagination?: "internal" | "external";
  searchable?: boolean;
  selectionActions?: (rows: T[]) => ReactNode;
}) {
  const [notice, setNotice] = useState("");
  const defs = useMemo<ColumnDef<typeof features, T>[]>(
    () =>
      columns.map((c) => ({
        id: c.id,
        sortDescFirst: false,
        header: () => c.header,
        accessorFn: (r) => (c.value ? c.value(r) : nodeText(c.cell(r))),
        enableSorting: c.sortable !== false && nodeText(c.header) !== "操作",
        cell: (ctx) => c.cell(ctx.row.original),
      })),
    [columns],
  );
  const table = useTable({
    features,
    data,
    columns: defs,
    getRowId,
    autoResetPageIndex: false,
    manualPagination: pagination === "external",
    initialState: { pagination: { pageIndex: 0, pageSize } },
  });
  useEffect(() => {
    const last = Math.max(0, table.getPageCount() - 1);
    if (table.state.pagination.pageIndex > last) table.setPageIndex(last);
  }, [
    data,
    table.state.globalFilter,
    table.state.pagination.pageSize,
    table.state.pagination.pageIndex,
  ]);
  const rows = table.getRowModel().rows,
    selected = table.getFilteredSelectedRowModel().rows;
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(
        selected
          .map((r) =>
            columns
              .filter((c) => table.getColumn(c.id)!.getIsVisible())
              .map((c) => nodeText(c.cell(r.original)))
              .join("\t"),
          )
          .join("\n"),
      );
      setNotice("已复制所选记录");
    } catch {
      setNotice("无法使用剪贴板，请重试");
    }
  };
  return (
    <section
      className="min-w-0 max-w-full space-y-3 p-4"
      aria-label={label}
      aria-busy={loading}
    >
      <DataTableToolbar
        searchable={searchable}
        label={label}
        query={table.state.globalFilter ?? ""}
        onQueryChange={(value) => {
          table.setGlobalFilter(value);
          table.setPageIndex(0);
        }}
      >
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button
                type="button"
                variant="outline"
                aria-label={`${label}列设置`}
              >
                <Columns3 />
                列设置
              </Button>
            }
          />
          <DropdownMenuContent align="end">
            {columns.map((c) => (
              <DropdownMenuCheckboxItem
                key={c.id}
                checked={table.getColumn(c.id)!.getIsVisible()}
                disabled={
                  table.getVisibleLeafColumns().length === 1 &&
                  table.getColumn(c.id)!.getIsVisible()
                }
                onCheckedChange={(next) =>
                  table.getColumn(c.id)!.toggleVisibility(next)
                }
              >
                {c.header}
              </DropdownMenuCheckboxItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
        {selected.length > 0 && (
          <>
            <span className="text-sm text-muted-foreground">
              已选 {selected.length} 项
            </span>
            {selectionActions?.(selected.map((row) => row.original))}
            <Button type="button" variant="outline" onClick={() => void copy()}>
              <Copy />
              复制所选
            </Button>
            <Button
              type="button"
              variant="ghost"
              onClick={() => table.resetRowSelection()}
            >
              清空选择
            </Button>
          </>
        )}
      </DataTableToolbar>
      {notice && (
        <p role="status" className="text-sm text-muted-foreground">
          {notice}
        </p>
      )}
      {error ? (
        <AsyncState error={error} onRetry={onRetry}>
          {null}
        </AsyncState>
      ) : (
        <div className="overflow-hidden rounded-xl border border-border bg-card text-card-foreground">
          <Table className="w-full data-table" aria-label={label}>
            <TableHeader>
              <TableRow>
                <TableHead scope="col" role="columnheader" className="w-10">
                  <Checkbox
                    aria-label={`选择${label}当前页`}
                    checked={table.getIsAllPageRowsSelected()}
                    indeterminate={
                      !table.getIsAllPageRowsSelected() &&
                      table.getIsSomePageRowsSelected()
                    }
                    onCheckedChange={(next) =>
                      table.toggleAllPageRowsSelected(next)
                    }
                  />
                </TableHead>
                {table.getHeaderGroups()[0]?.headers.map((h) => (
                  <TableHead
                    scope="col"
                    role="columnheader"
                    key={h.id}
                    style={{ width: columns.find(c => c.id === h.id)?.width }}
                    className={columns.find(c => c.id === h.id)?.align === "right" || nodeText(columns.find(c => c.id === h.id)?.header) === "操作" ? "text-right" : undefined}
                    aria-sort={
                      h.column.getIsSorted() === "asc"
                        ? "ascending"
                        : h.column.getIsSorted() === "desc"
                          ? "descending"
                          : undefined
                    }
                  >
                    {h.column.getCanSort() ? (
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={() => h.column.toggleSorting()}
                      >
                        {columns.find((col) => col.id === h.column.id)?.header}
                        <ArrowUpDown className="size-3" />
                      </Button>
                    ) : (
                      columns.find((col) => col.id === h.column.id)?.header
                    )}
                  </TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                Array.from({ length: 5 }, (_, i) => (
                  <TableRow key={i}>
                    <TableCell colSpan={columns.length + 1}>
                      <Skeleton className="h-8 w-full" />
                    </TableCell>
                  </TableRow>
                ))
              ) : rows.length ? (
                rows.map((r) => (
                  <TableRow
                    key={r.id}
                    tabIndex={0}
                    data-list-item
                    data-id={r.id}
                    data-state={r.getIsSelected() ? "selected" : undefined}
                  >
                    <TableCell>
                      <Checkbox
                        aria-label={`选择记录 ${r.id}`}
                        checked={r.getIsSelected()}
                        onCheckedChange={(next) => r.toggleSelected(next)}
                      />
                    </TableCell>
                    {r.getVisibleCells().map((c) => (
                      <TableCell
                        key={c.id}
                        className={"max-w-72 whitespace-normal break-words align-middle " + (columns.find(col => col.id === c.column.id)?.align === "right" || nodeText(columns.find(col => col.id === c.column.id)?.header) === "操作" ? "text-right tabular-nums" : "")}
                        data-label={nodeText(
                          columns.find((col) => col.id === c.column.id)?.header,
                        )}
                      >
                        {columns
                          .find((col) => col.id === c.column.id)
                          ?.cell(r.original)}
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              ) : (
                <TableRow>
                  <TableCell
                    colSpan={columns.length + 1}
                    className="h-36 text-center text-muted-foreground"
                  >
                    <EmptyState
                      title={
                        table.state.globalFilter ? "没有匹配结果" : "暂无记录"
                      }
                      description={
                        table.state.globalFilter
                          ? "请调整搜索或筛选条件"
                          : "创建或接收内容后会显示在这里"
                      }
                    />
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
      )}
      <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
        <span>{pagination === "external" ? "本页 " : ""}{table.getFilteredRowModel().rows.length} 条记录</span>
        {pagination === "internal" && <div className="flex flex-wrap items-center gap-2">
          <SelectField
            aria-label={`${label}每页条数`}
            value={String(table.state.pagination.pageSize)}
            onValueChange={(v) => table.setPageSize(Number(v))}
            options={[20, 50, 100].map((n) => ({
              value: String(n),
              label: `${n} 条 / 页`,
            }))}
          />
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={!table.getCanPreviousPage()}
            onClick={() => table.previousPage()}
          >
            上一页
          </Button>
          <span>
            {table.state.pagination.pageIndex + 1} /{" "}
            {Math.max(1, table.getPageCount())}
          </span>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={!table.getCanNextPage()}
            onClick={() => table.nextPage()}
          >
            下一页
          </Button>
        </div>}
      </div>
    </section>
  );
}

export type GridRow = { id: string; cells: ReactNode[] };
export function GridTable({
  headers,
  rows,
  label = "数据表",
  pagination,
}: {
  headers: ReactNode[];
  rows: GridRow[];
  label?: string;
  pagination?: "internal" | "external";
}) {
  const columns = useMemo<DataColumn<GridRow>[]>(
    () =>
      headers.map((header, i) => ({
        id: `column-${i}`,
        header,
        cell: (row) => row.cells[i] ?? null,
      })),
    [headers],
  );
  return (
    <DataTable
      label={label}
      data={rows}
      pagination={pagination}
      columns={columns}
      getRowId={(row) => row.id}
    />
  );
}
