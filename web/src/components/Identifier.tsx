import { useState } from "react";
import { Check, Copy } from "lucide-react";
import { Button } from "./ui/button";

export function Identifier({ value }: { value: string }) {
  const [status, setStatus] = useState("");
  return <span className="inline-flex max-w-full items-center gap-1 align-middle text-xs text-muted-foreground">
    <code className="truncate" title={value}>{value.slice(0, 10)}{value.length > 10 ? "…" : ""}</code>
    <Button type="button" variant="ghost" size="icon" aria-label="复制标识" title="复制完整标识" onClick={async () => {
      try { await navigator.clipboard.writeText(value); setStatus("已复制标识"); }
      catch { setStatus("复制失败，请重试"); }
    }}>{status === "已复制标识" ? <Check /> : <Copy />}</Button>
    <span className="sr-only" role="status">{status}</span>
  </span>;
}
