import { useEffect, useRef, useState } from "react";
import { ChevronLeft, ChevronRight, RotateCcw } from "lucide-react";
import { Button } from "./components/ui/button";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "./components/ui/table";
import { SelectAdapter } from "./components/SelectAdapter";
import {
  MAX_COLUMNS,
  MAX_ROWS,
  PAGE_ROWS,
  SPREADSHEET_LIMIT,
  columnLabel,
  readSpreadsheetResponse,
  type Delimiter,
  type Encoding,
  type SpreadsheetData,
  type SpreadsheetFormat,
} from "./spreadsheet-source";
import "./spreadsheet-preview.css";

export default function SpreadsheetPreview({
  url,
  name,
  size,
  format,
}: {
  url: string;
  name: string;
  size: number;
  format: SpreadsheetFormat;
}) {
  const [sheetIndex, setSheetIndex] = useState(0);
  const [delimiter, setDelimiter] = useState<Delimiter>("auto");
  const [encoding, setEncoding] = useState<Encoding>("utf-8");
  const [page, setPage] = useState(0);
  const [retry, setRetry] = useState(0);
  const [data, setData] = useState<SpreadsheetData>();
  const [busy, setBusy] = useState(true);
  const [error, setError] = useState("");
  const cache = useRef<{ url: string; buffer: ArrayBuffer } | undefined>(
    undefined,
  );
  const viewport = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setBusy(true);
    setError("");
    setPage(0);
    if (size > SPREADSHEET_LIMIT) {
      setBusy(false);
      return;
    }
    const controller = new AbortController();
    let worker: Worker | undefined;
    let disposed = false;
    const timeout = setTimeout(() => {
      if (disposed) return;
      controller.abort();
      worker?.terminate();
      setError("表格读取超时，请重试或下载查看。");
      setBusy(false);
    }, 30000);
    async function load() {
      try {
        let buffer =
          cache.current?.url === url ? cache.current.buffer : undefined;
        if (!buffer) {
          buffer = await readSpreadsheetResponse(
            await fetch(url, {
              signal: controller.signal,
              credentials: "same-origin",
            }),
          );
          if (disposed || controller.signal.aborted) return;
          cache.current = { url, buffer };
        }
        if (disposed || controller.signal.aborted) return;
        worker = new Worker(
          new URL("./spreadsheet.worker.ts", import.meta.url),
          { type: "module" },
        );
        worker.onmessage = (
          event: MessageEvent<{ data?: SpreadsheetData; error?: string }>,
        ) => {
          if (disposed || controller.signal.aborted) return;
          clearTimeout(timeout);
          worker?.terminate();
          if (event.data.error) setError(event.data.error);
          else setData(event.data.data);
          setBusy(false);
        };
        worker.onerror = () => {
          if (disposed || controller.signal.aborted) return;
          clearTimeout(timeout);
          worker?.terminate();
          setError("表格解析失败，请重试或下载查看。");
          setBusy(false);
        };
        const copy = buffer.slice(0);
        worker.postMessage(
          {
            buffer: copy,
            options: { format, sheetIndex, delimiter, encoding },
          },
          [copy],
        );
      } catch (failure) {
        if (disposed || controller.signal.aborted) return;
        clearTimeout(timeout);
        setError(
          failure instanceof Error && /^(访问|文件|表格)/.test(failure.message)
            ? failure.message
            : "表格加载失败，请检查连接后重试。",
        );
        setBusy(false);
      }
    }
    void load();
    return () => {
      disposed = true;
      controller.abort();
      worker?.terminate();
      clearTimeout(timeout);
    };
  }, [url, size, format, sheetIndex, delimiter, encoding, retry]);

  useEffect(() => {
    viewport.current?.scrollTo?.({ top: 0, left: 0 });
  }, [page, data]);
  const shownRows = data?.rows.length || 0;
  const pages = Math.max(1, Math.ceil(shownRows / PAGE_ROWS));
  const start = page * PAGE_ROWS;
  const columns = Math.min(data?.columnCount || 0, MAX_COLUMNS);
  const label = data?.sheetNames[data.sheetIndex] || "数据";
  const reload = () => {
    cache.current = undefined;
    setRetry((value) => value + 1);
  };

  return (
    <div className="spreadsheet-reader">
      <div
        className="spreadsheet-toolbar"
        role="group"
        aria-label="表格阅读设置"
      >
        {format === "xlsx" ? (
          data &&
          data.sheetNames.length > 0 && (
            <label>
              工作表
              <SelectAdapter
                aria-label="工作表"
                value={sheetIndex}
                onChange={(event) => setSheetIndex(Number(event.target.value))}
                disabled={busy}
              >
                {data.sheetNames.map((sheet, index) => (
                  <option key={index} value={index}>
                    {sheet}
                  </option>
                ))}
              </SelectAdapter>
            </label>
          )
        ) : (
          <>
            <label>
              分隔符
              <SelectAdapter
                aria-label="分隔符"
                value={delimiter}
                onChange={(event) =>
                  setDelimiter(event.target.value as Delimiter)
                }
                disabled={busy}
              >
                <option value="auto">自动识别</option>
                <option value=",">逗号</option>
                <option value=";">分号</option>
                <option value={"\t"}>制表符</option>
                <option value="|">竖线</option>
              </SelectAdapter>
            </label>
            <label>
              编码
              <SelectAdapter
                aria-label="文本编码"
                value={encoding}
                onChange={(event) =>
                  setEncoding(event.target.value as Encoding)
                }
                disabled={busy}
              >
                <option value="utf-8">UTF-8 / Unicode</option>
                <option value="gb18030">GB18030 / GBK</option>
              </SelectAdapter>
            </label>
          </>
        )}
        <span className="spreadsheet-readonly">只读预览</span>
      </div>
      {size > SPREADSHEET_LIMIT ? (
        <p className="spreadsheet-message">表格超过 16 MiB，请下载查看。</p>
      ) : error ? (
        <div className="spreadsheet-message" role="alert">
          <p>{error}</p>
          <Button variant="outline" type="button" onClick={reload}>
            <RotateCcw size={16} />
            重新加载
          </Button>
        </div>
      ) : busy ? (
        <p className="spreadsheet-message" role="status">
          正在读取表格…
        </p>
      ) : !shownRows || !columns ? (
        <p className="spreadsheet-message" role="status">
          此工作表为空。
        </p>
      ) : (
        <div
          ref={viewport}
          className="spreadsheet-scroll [&>[data-slot=table-container]]:overflow-visible [&>[data-slot=table-container]]:w-max [&>[data-slot=table-container]]:min-w-full"
          tabIndex={0}
          role="region"
          aria-label={`${label}表格，可横向和纵向滚动`}
        >
          <Table
            className="spreadsheet-grid w-max min-w-full"
            aria-label={`${name} · ${label}`}
          >
            <TableHeader>
              <TableRow>
                <TableHead
                  scope="col"
                  className="spreadsheet-corner"
                  aria-label="行号"
                >
                  #
                </TableHead>
                {Array.from({ length: columns }, (_, col) => (
                  <TableHead scope="col" key={col}>
                    {columnLabel(col)}
                  </TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {data!.rows.slice(start, start + PAGE_ROWS).map((row, index) => (
                <TableRow key={start + index}>
                  <TableHead scope="row">{start + index + 1}</TableHead>
                  {Array.from({ length: columns }, (_, col) => (
                    <TableCell key={col}>
                      <div>{row[col] || ""}</div>
                    </TableCell>
                  ))}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
      {!busy && !error && data && size <= SPREADSHEET_LIMIT && (
        <div className="spreadsheet-footer">
          <div
            className="spreadsheet-pagination"
            role="group"
            aria-label="表格分页"
          >
            <span role="status">
              {data.rowCount} 行 · {data.columnCount} 列
              {shownRows > 0 &&
                ` · 显示 ${start + 1}–${Math.min(start + PAGE_ROWS, shownRows)} 行`}
            </span>
            {pages > 1 && (
              <>
                <Button
                  variant="ghost"
                  size="icon"
                  type="button"
                  aria-label="上一页表格"
                  disabled={page === 0}
                  onClick={() => setPage((value) => value - 1)}
                >
                  <ChevronLeft size={16} />
                </Button>
                <span>
                  {page + 1} / {pages}
                </span>
                <Button
                  variant="ghost"
                  size="icon"
                  type="button"
                  aria-label="下一页表格"
                  disabled={page + 1 >= pages}
                  onClick={() => setPage((value) => value + 1)}
                >
                  <ChevronRight size={16} />
                </Button>
              </>
            )}
          </div>
          {(data.rowCount > MAX_ROWS ||
            data.columnCount > MAX_COLUMNS ||
            data.clippedCells) && (
            <p>
              预览最多显示前 {MAX_ROWS} 行、{MAX_COLUMNS}{" "}
              列；过长单元格已截断。完整内容请下载查看。
            </p>
          )}
          {format === "xlsx" && (
            <p>
              显示已保存的单元格值；公式不重新计算，图表、图片与合并样式不展示。
            </p>
          )}
          {format !== "xlsx" && (
            <p>
              按原始文本显示
              {data.delimiter &&
                ` · 当前分隔符：${({ ",": "逗号", ";": "分号", "\t": "制表符", "|": "竖线" } as Record<string, string>)[data.delimiter]}`}
              ；乱码可切换编码。
            </p>
          )}
        </div>
      )}
    </div>
  );
}
