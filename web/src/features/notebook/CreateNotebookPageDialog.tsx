import { useEffect, useRef, useState } from "react";
import { getDataContext } from "../../api";
import { AppDialog } from "../../components/AppDialog";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { useRouteLeaveGuard } from "../../app/route-leave";
import { createNotebookDaily, createNotebookPage, lookupNotebookCreation, resumeNotebookCreation, type NotebookCreation } from "./notebook-client";

interface Recovery { key: string; date: string }
function readRecovery(slot: string): Recovery | null {
  const raw = localStorage.getItem(slot);
  if (raw === null) return null;
  const value: unknown = JSON.parse(raw);
  if (!value || typeof value !== "object" || !("key" in value) || !("date" in value) || typeof value.key !== "string" || !/^notebook-[a-f0-9]{32}$/.test(value.key) || typeof value.date !== "string" || (value.date !== "" && !/^\d{4}-\d{2}-\d{2}$/.test(value.date))) throw new Error("invalid_recovery");
  return { key: value.key, date: value.date };
}

// Only original request identity/date survives reload, never title or document content.
export function CreateNotebookPageDialog({ actor, space, parent, daily, close, onCreated, onDenied }: { actor: string; space: string; parent?: string; daily: boolean; close: () => void; onCreated: () => void; onDenied: () => void }) {
  const slot = "drivebridge:notebook-create:" + JSON.stringify([actor, getDataContext().tenant, getDataContext().space, space]);
  const [title, setTitle] = useState("");
  const [date] = useState(() => { const now = new Date(); return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}-${String(now.getDate()).padStart(2, "0")}`; });
  const [state, setState] = useState<"loading" | "ready" | "pending" | "unknown" | "completed" | "denied" | "offline" | "blocked">("loading");
  const [recovery, setRecovery] = useState<Recovery | null>(null);
  const [result, setResult] = useState<NotebookCreation | null>(null);
  const [message, setMessage] = useState("");
  const active = useRef(true), controller = useRef<AbortController | null>(null);
  const submitting = useRef(false);
  const original = useRef<{ title: string; parent: string; daily: boolean } | null>(null);
  const ownsIntent = useRef(false);
  const ownKey = useRef("");
  useRouteLeaveGuard(!ownsIntent.current && !!title.trim(), "放弃未提交的新笔记标题并离开？");
  useEffect(() => {
    active.current = true;
    const invalidate = () => { active.current = false; controller.current?.abort(); original.current = null; setTitle(""); setRecovery(null); setResult(null); setState("denied"); };
    const offline = () => { controller.current?.abort(); setState("offline"); setResult(null); };
    window.addEventListener("drivebridge-context", invalidate); window.addEventListener("offline", offline);
    try { const saved = readRecovery(slot); setRecovery(saved); setState(!navigator.onLine ? "offline" : saved ? "unknown" : "ready"); } catch { setState("blocked"); }
    return () => { active.current = false; controller.current?.abort(); window.removeEventListener("drivebridge-context", invalidate); window.removeEventListener("offline", offline); };
  }, [slot]);
  async function accept(value: NotebookCreation, identity: Recovery) {
    if (!active.current || controller.current?.signal.aborted) return;
    if (!value || typeof value.found !== "boolean" || typeof value.status !== "string") throw new Error("invalid_contract");
    if (value.status === "completed") {
      if (!value.found || !value.operation_id || !value.page?.file_id || value.page.space_id !== space || !value.page.version_id || !value.page.path) throw new Error("invalid_contract");
      if (!navigator.locks?.request) throw new Error("recovery_lock_unavailable");
      await navigator.locks.request(slot, () => { if (readRecovery(slot)?.key === identity.key) localStorage.removeItem(slot); });
      if (!active.current || controller.current?.signal.aborted) return;
      setResult(value); setRecovery(null);
      if (!ownsIntent.current && title.trim()) { setState("ready"); setMessage("另一笔原创建已确认；你的未提交标题仍保留，请重新确认。"); }
      else { setTitle(""); setState("completed"); onCreated(); }
    } else { setResult(value); setState("unknown"); setMessage(value.found ? "原创建尚未完成，可继续查询或恢复同一操作。" : "尚未找到原请求，不代表未执行；不会创建新请求。"); }
  }
  async function run(mode: "create" | "lookup" | "resume") {
    if (submitting.current || !active.current) return;
    if (!navigator.onLine) { setState("offline"); return; }
    submitting.current = true;
    const request = new AbortController(); controller.current = request; setState("pending"); setMessage("");
    try {
      let identity = recovery;
      if (mode === "create") {
        if (!navigator.locks?.request) { setState("blocked"); return; }
        const claimed = await navigator.locks.request(slot, { signal: request.signal }, () => {
          const saved = readRecovery(slot); if (saved) return { identity: saved, claimed: false };
          const identity = { key: "notebook-" + Array.from(crypto.getRandomValues(new Uint8Array(16)), value => value.toString(16).padStart(2, "0")).join(""), date: daily ? date : "" };
          localStorage.setItem(slot, JSON.stringify(identity)); return { identity, claimed: true };
        });
        identity = claimed.identity; setRecovery(identity);
        if (!claimed.claimed) { ownsIntent.current = false; setState("unknown"); setMessage("另一个标签页已有原请求；本标签页标题保留，先查询原请求。"); return; }
        ownsIntent.current = true;
        ownKey.current = identity.key;
        original.current = { title: daily ? date : title.trim(), parent: parent || "", daily };
      }
      if (!identity) throw new Error("missing_recovery");
      const value = mode === "lookup" ? await lookupNotebookCreation(space, identity.key, identity.date, request.signal)
        : mode === "resume" ? await resumeNotebookCreation(space, identity.key, identity.date, request.signal)
        : identity.date ? await createNotebookDaily(space, identity.date, identity.key, request.signal)
        : await createNotebookPage(space, original.current!.title, original.current!.parent, "file", identity.key, request.signal);
      await accept(value, identity);
    } catch (error) {
      if (!active.current || request.signal.aborted) return;
      const status = error && typeof error === "object" && "status" in error ? error.status : undefined;
      const code = error instanceof Error ? error.message : "";
      if (mode === "create" && ownsIntent.current && ((status === 400 && code === "invalid_request") || (status === 422 && code === "unsupported_action"))) {
        await navigator.locks?.request(slot, () => { if (readRecovery(slot)?.key === ownKey.current) localStorage.removeItem(slot); });
        ownsIntent.current = false; setRecovery(null); original.current = null; setState("ready"); setMessage("创建在受理前被拒绝，标题保留；请纠正输入或确认服务已启用后重试。"); return;
      }
      if (status === 401 || status === 403) { setTitle(""); setRecovery(null); setResult(null); original.current = null; setState("denied"); onDenied(); }
      else {
        try { const saved = readRecovery(slot); setRecovery(saved); setState(saved ? "unknown" : "blocked"); } catch { setState("blocked"); }
        setMessage("创建或回查未完成，请查询原请求；未替换幂等键，也未将失败当作创建成功。");
      }
    } finally { submitting.current = false; }
  }
  return <AppDialog title={daily ? "创建日记" : "新建笔记"} close={() => { if (state !== "pending" && (ownsIntent.current || !title.trim() || window.confirm("放弃未提交的新笔记标题？"))) close(); }}>
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">通过服务端创建真实 Markdown 文件与页面关系。{parent ? "使用已选择的父页面。" : "创建顶层页面。"}</p>
      {state === "ready" && <>{daily ? <p>日记日期：{date}</p> : <label className="grid gap-2 text-sm">笔记标题<Input autoFocus maxLength={120} value={title} onChange={event => setTitle(event.target.value)} className="min-h-11"/></label>}<Button className="min-h-11" disabled={!daily && !title.trim()} onClick={() => void run("create")}>确认创建</Button></>}
      {state === "loading" && <p role="status">读取原创建身份…</p>}
      {state === "pending" && <p role="status">正在提交或回查原创建，请勿重复提交…</p>}
      {state === "unknown" && <p role="alert">结果待回查。关闭或刷新不会丢失原请求身份；标题不写入浏览器存储。</p>}
      {state === "offline" && <p role="alert">当前离线，未创建新请求；联网后请手动查询原创建。</p>}
      {state === "denied" && <p role="alert">身份或权限已失效，创建信息已清除，请重新进入。</p>}
      {state === "blocked" && <p role="alert">无法安全保存或锁定原请求身份，创建已停止；请检查浏览器存储与 Web Locks 支持。</p>}
      {message && <p role="alert">{message}</p>}
      {result?.operation_id && <p className="break-all text-xs">原操作：{result.operation_id}</p>}
      {state === "completed" && <p role="status">已创建固定版本：{result?.page?.path}。关闭后可在知识库读取。</p>}
      {recovery && (state === "unknown" || state === "offline") && <div className="flex flex-wrap gap-2"><Button variant="outline" className="min-h-11" onClick={() => void run("lookup")}>查询原创建</Button>{result?.found && <Button variant="outline" className="min-h-11" onClick={() => void run("resume")}>恢复原创建</Button>}</div>}
    </div>
  </AppDialog>;
}
