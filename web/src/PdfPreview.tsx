import React, { useEffect, useRef, useState } from "react";
import {
  getDocument,
  GlobalWorkerOptions,
  PasswordResponses,
  TextLayer,
  type PDFDocumentProxy,
} from "pdfjs-dist";
import workerURL from "pdfjs-dist/build/pdf.worker.min.mjs?url";
import { Button } from "./components/ui/button";
import { Input } from "./components/ui/input";
import "./preview-reader.css";
GlobalWorkerOptions.workerSrc = workerURL;

function failure(error: unknown) {
  const e = error as { name?: string; status?: number; message?: string };
  if (e.status === 401 || e.status === 403)
    return "访问已失效或没有预览权限，请重新登录后再试。";
  if (e.status === 404) return "文件或版本已不存在。";
  if (e.name === "InvalidPDFException") return "PDF 文件损坏或格式无效。";
  return "PDF 加载失败，请检查连接后重试。";
}
export default function PdfPreview({
  url,
  name,
}: {
  url: string;
  name: string;
}) {
  const [doc, setDoc] = useState<PDFDocumentProxy>();
  const [page, setPage] = useState(1),
    [retry, setRetry] = useState(0);
  const [rendered, setRendered] = useState("");
  const [zoom, setZoom] = useState<number | "width" | "page">("page");
  const [error, setError] = useState(""),
    [busy, setBusy] = useState(true),
    [passwordReason, setPasswordReason] = useState(0);
  const [password, setPassword] = useState("");
  const passwordCallback = useRef<((value: string) => void) | undefined>(
    undefined,
  );
  const stage = useRef<HTMLDivElement>(null),
    canvas = useRef<HTMLCanvasElement>(null),
    layer = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState({ width: 600, height: 600 });
  const renderKey = `${url}:${page}:${zoom}:${size.width}:${size.height}:${retry}`;
  useEffect(() => {
    if (!stage.current) return;
    const observer = new ResizeObserver(([e]) =>
      setSize({ width: e.contentRect.width, height: e.contentRect.height }),
    );
    observer.observe(stage.current);
    return () => observer.disconnect();
  }, []);
  useEffect(() => {
    let cancelled = false;
    setDoc(undefined);
    setPage(1);
    setBusy(true);
    setError("");
    setPassword("");
    setPasswordReason(0);
    const task = getDocument({
      url,
      withCredentials: true,
      enableXfa: false,
      cMapUrl: "/pdfjs/cmaps/",
      cMapPacked: true,
      standardFontDataUrl: "/pdfjs/standard_fonts/",
      wasmUrl: "/pdfjs/wasm/",
    });
    const timer = setTimeout(() => {
      if (!cancelled) {
        setError("PDF 加载超时，请检查连接后重试。");
        setBusy(false);
        void task.destroy();
      }
    }, 30000);
    task.onPassword = (
      callback: (password: string) => void,
      reason: number,
    ) => {
      if (!cancelled) {
        clearTimeout(timer);
        passwordCallback.current = callback;
        setPasswordReason(reason);
        setBusy(false);
      }
    };
    task.promise
      .then((value) => {
        clearTimeout(timer);
        if (!cancelled) {
          setDoc(value);
          setPasswordReason(0);
          setBusy(false);
        }
      })
      .catch((e) => {
        clearTimeout(timer);
        if (!cancelled) {
          setError(failure(e));
          setBusy(false);
        }
      });
    return () => {
      cancelled = true;
      clearTimeout(timer);
      passwordCallback.current = undefined;
      void task.destroy();
    };
  }, [url, retry]);
  useEffect(() => {
    if (!doc || !canvas.current || !layer.current) return;
    let cancelled = false;
    let render:
      | ReturnType<Awaited<ReturnType<PDFDocumentProxy["getPage"]>>["render"]>
      | undefined;
    let text: TextLayer | undefined;
    setBusy(true);
    setError("");
    layer.current.replaceChildren();
    void doc
      .getPage(page)
      .then(async (p) => {
        if (cancelled) return;
        const natural = p.getViewport({ scale: 1 });
        const fitWidth = Math.max(0.1, (size.width - 32) / natural.width);
        const scale =
          typeof zoom === "number"
            ? zoom
            : zoom === "width"
              ? fitWidth
              : Math.min(
                  fitWidth,
                  Math.max(0.1, (size.height - 32) / natural.height),
                );
        const viewport = p.getViewport({ scale });
        const c = canvas.current!,
          l = layer.current!;
        const ratio = Math.min(devicePixelRatio || 1, 2);
        c.width = Math.ceil(viewport.width * ratio);
        c.height = Math.ceil(viewport.height * ratio);
        c.style.width = `${viewport.width}px`;
        c.style.height = `${viewport.height}px`;
        l.style.setProperty("--scale-factor", String(scale));
        l.style.setProperty("--total-scale-factor", String(scale));
        render = p.render({
          canvas: c,
          viewport,
          transform: [ratio, 0, 0, ratio, 0, 0],
        });
        await render.promise;
        if (cancelled) return;
        text = new TextLayer({
          textContentSource: await p.getTextContent(),
          container: l,
          viewport,
        });
        if (cancelled) return;
        await text.render();
        if (!cancelled) {
          setRendered(renderKey);
          setBusy(false);
        }
      })
      .catch((e) => {
        if (!cancelled) {
          setError(failure(e));
          setBusy(false);
        }
      });
    return () => {
      cancelled = true;
      render?.cancel();
      text?.cancel();
    };
  }, [doc, page, zoom, size]);
  return (
    <div className="pdf-reader" aria-label={`${name} PDF 阅读器`}>
      <div className="reader-toolbar">
        <Button
          size="sm"
          variant="ghost"
          disabled={!doc || page <= 1}
          onClick={() => setPage((p) => p - 1)}
        >
          上一页
        </Button>
        <Input
          aria-label="PDF 页码"
          type="number"
          min={1}
          max={doc?.numPages || 1}
          value={page}
          disabled={!doc}
          onChange={(e) => {
            const n = Number(e.target.value);
            if (doc && Number.isInteger(n) && n >= 1 && n <= doc.numPages)
              setPage(n);
          }}
        />
        <span>/ {doc?.numPages || "—"}</span>
        <Button
          size="sm"
          variant="ghost"
          disabled={!doc || page >= doc.numPages}
          onClick={() => setPage((p) => p + 1)}
        >
          下一页
        </Button>
        <Button size="sm" variant="ghost" onClick={() => setZoom("width")}>
          适应宽度
        </Button>
        <Button size="sm" variant="ghost" onClick={() => setZoom("page")}>
          适应整页
        </Button>
        <Button
          size="sm"
          variant="ghost"
          aria-label="缩小 PDF"
          onClick={() =>
            setZoom((z) =>
              Math.max(0.25, (typeof z === "number" ? z : 1) - 0.25),
            )
          }
        >
          −
        </Button>
        <Button
          size="sm"
          variant="ghost"
          aria-label="放大 PDF"
          onClick={() =>
            setZoom((z) => Math.min(4, (typeof z === "number" ? z : 1) + 0.25))
          }
        >
          ＋
        </Button>
      </div>
      <div className="pdf-scroll" ref={stage}>
        {passwordReason > 0 ? (
          <form
            className="reader-message"
            onSubmit={(e) => {
              e.preventDefault();
              passwordCallback.current?.(password);
              setPassword("");
              setPasswordReason(0);
              setBusy(true);
            }}
          >
            <p>
              {passwordReason === PasswordResponses.INCORRECT_PASSWORD
                ? "密码错误，请重试。"
                : "此 PDF 需要密码。"}
            </p>
            <Input
              type="password"
              aria-label="PDF 密码"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="off"
            />
            <Button type="submit" disabled={!password}>
              解锁 PDF
            </Button>
          </form>
        ) : error ? (
          <div className="reader-message" role="alert">
            <p>{error}</p>
            <Button onClick={() => setRetry((n) => n + 1)}>重新加载 PDF</Button>
          </div>
        ) : (
          <div className="pdf-page" hidden={!doc}>
            <canvas ref={canvas} aria-label={`第 ${page} 页`} />
            <div className="textLayer" ref={layer} />
          </div>
        )}
      </div>
      <div className="reader-status" role="status">
        {error
          ? "预览未完成"
          : passwordReason
            ? "等待密码"
            : busy || rendered !== renderKey
              ? "正在加载 PDF…"
              : doc
                ? `第 ${page} / ${doc.numPages} 页 · 可选择文字`
                : ""}
      </div>
    </div>
  );
}
