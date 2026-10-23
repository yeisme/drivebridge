import { useEffect, useRef, useState } from "react";
import { getDataContext } from "../../api";
import { AppDialog } from "../../components/AppDialog";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { useRouteLeaveGuard } from "../../app/route-leave";
import { createNotebookDaily, createNotebookPage, lookupNotebookCreation, resumeNotebookCreation, type NotebookCreation } from "./clients";
import {
  claimNotebookRecovery,
  clearInvalidNotebookRecovery,
  clearNotebookRecovery,
  encodeNotebookRecovery,
  inspectNotebookRecovery,
  newNotebookKey,
  notebookPageSlot,
  notebookRecoveryMessage,
  notebookRecoveryReason,
  parseNotebookPageRecovery,
  type NotebookPageRecovery,
} from "./notebook-recovery";

type State = "loading" | "ready" | "pending" | "unknown" | "completed" | "denied" | "offline" | "blocked";
type BlockedReason = ReturnType<typeof notebookRecoveryReason>;

function isAbort(error: unknown) {
  return error && typeof error === "object" && "name" in error && error.name === "AbortError";
}

export function CreateNotebookPageDialog({ actor, space, directoryPath = "", parent, daily, close, onCreated, onDenied }: { actor: string; space: string; directoryPath?: string; parent?: string; daily: boolean; close: () => void; onCreated: (page?: NotebookCreation["page"]) => void; onDenied: () => void }) {
  const physicalPath = directoryPath;
  const context = getDataContext();
  const slot = notebookPageSlot(actor, context.tenant, space, physicalPath);
  const [title, setTitle] = useState("");
  const [date] = useState(() => { const now = new Date(); return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}-${String(now.getDate()).padStart(2, "0")}`; });
  const [state, setState] = useState<State>("loading");
  const [recovery, setRecovery] = useState<NotebookPageRecovery | null>(null);
  const [result, setResult] = useState<NotebookCreation | null>(null);
  const [message, setMessage] = useState("");
  const [blockedReason, setBlockedReason] = useState<BlockedReason | null>(null);
  const active = useRef(true), controller = useRef<AbortController | null>(null);
  const submitting = useRef(false);
  const original = useRef<{ title: string; parent: string } | null>(null);
  const ownsIntent = useRef(false);
  const ownKey = useRef("");
  useRouteLeaveGuard(!ownsIntent.current && !!title.trim(), "放弃未提交的新笔记标题并离开？");

  async function refreshAvailability() {
    if (!active.current) return;
    setState("loading"); setMessage(""); setBlockedReason(null);
    if (!navigator.onLine) { setState("offline"); return; }
    try {
      const saved = await inspectNotebookRecovery(slot, parseNotebookPageRecovery);
      if (!active.current) return;
      setRecovery(saved); setState(saved ? "unknown" : "ready");
    } catch (error) {
      if (!active.current) return;
      const reason = notebookRecoveryReason(error);
      setBlockedReason(reason); setState("blocked"); setMessage(notebookRecoveryMessage(reason));
    }
  }

  useEffect(() => {
    active.current = true;
    const invalidate = () => { active.current = false; controller.current?.abort(); original.current = null; setTitle(""); setRecovery(null); setResult(null); setState("denied"); };
    const offline = () => { controller.current?.abort(); setState("offline"); setResult(null); };
    window.addEventListener("drivebridge-context", invalidate); window.addEventListener("offline", offline);
    void refreshAvailability();
    return () => { active.current = false; controller.current?.abort(); window.removeEventListener("drivebridge-context", invalidate); window.removeEventListener("offline", offline); };
  }, [slot]);

  async function clearKnown(identity: NotebookPageRecovery) {
    try {
      await clearNotebookRecovery(slot, parseNotebookPageRecovery, identity.key, value => value.key);
      if (active.current) setRecovery(null);
      return true;
    } catch (error) {
      const reason = notebookRecoveryReason(error);
      if (active.current) { setBlockedReason(reason); setMessage(notebookRecoveryMessage(reason)); }
      return false;
    }
  }

  async function accept(value: NotebookCreation, identity: NotebookPageRecovery) {
    if (!active.current || controller.current?.signal.aborted) return;
    if (!value || typeof value.found !== "boolean" || typeof value.status !== "string") throw new Error("invalid_contract");
    if (value.status !== "completed") { setResult(value); setState("unknown"); setMessage(value.found ? "原创建尚未完成，可继续查询或恢复同一操作。" : "尚未找到原请求，不代表未执行；不会创建新请求。"); return; }
    if (!value.found || !value.operation_id || !value.page?.file_id || value.page.space_id !== space || !value.page.version_id || !value.page.path) throw new Error("invalid_contract");
    const cleared = await clearKnown(identity);
    if (!cleared && active.current) setMessage("已确认创建成功，但浏览器未能清理恢复记录；后续只会查询原请求，不会重复创建。");
    if (!active.current || controller.current?.signal.aborted) return;
    setResult(value);
    if (!ownsIntent.current && title.trim()) { setState("ready"); setMessage("另一笔原创建已确认；你的未提交标题仍保留，请重新确认。"); }
    else { setTitle(""); setState("completed"); onCreated(value.page); }
  }

  async function run(mode: "create" | "lookup" | "resume") {
    if (submitting.current || !active.current) return;
    if (!navigator.onLine) { setState("offline"); return; }
    submitting.current = true;
    const request = new AbortController(); controller.current = request; setState("pending"); setMessage("");
    try {
      let identity = recovery;
      if (mode === "create") {
        const next: NotebookPageRecovery = { key: newNotebookKey("notebook"), date: daily ? date : "", path: physicalPath };
        const claimed = await claimNotebookRecovery(slot, parseNotebookPageRecovery, next, encodeNotebookRecovery, request.signal);
        identity = claimed.record;
        setRecovery(identity);
        if (!claimed.claimed) { ownsIntent.current = false; setState("unknown"); setMessage("另一个标签页已有原请求；本标签页标题保留，先查询原请求。"); return; }
        ownsIntent.current = true; ownKey.current = identity.key; original.current = { title: daily ? date : title.trim(), parent: parent || "" };
      }
      if (!identity) throw new Error("missing_recovery");
      if (mode !== "create") {
        const current = await inspectNotebookRecovery(slot, parseNotebookPageRecovery);
        if (!current || current.key !== identity.key) throw new Error("recovery_changed");
      }
      const value = mode === "lookup" ? await lookupNotebookCreation(space, identity.key, identity.date, request.signal, ...(identity.path ? [identity.path] : []))
        : mode === "resume" ? await resumeNotebookCreation(space, identity.key, identity.date, request.signal, ...(identity.path ? [identity.path] : []))
        : identity.date ? await createNotebookDaily(space, identity.date, identity.key, request.signal, ...(identity.path ? [identity.path] : []))
        : await createNotebookPage(space, original.current!.title, original.current!.parent, "file", identity.key, request.signal, ...(identity.path ? [identity.path] : []));
      await accept(value, identity);
    } catch (error) {
      if (!active.current || request.signal.aborted || isAbort(error)) return;
      const status = error && typeof error === "object" && "status" in error ? error.status : undefined;
      const code = error instanceof Error ? error.message : "";
      const reason = notebookRecoveryReason(error);
      if (reason !== "unknown") {
        setBlockedReason(reason); setState(reason === "recovery_changed" ? "unknown" : "blocked"); setMessage(notebookRecoveryMessage(reason)); return;
      }
      if (mode === "create" && ownsIntent.current && ((status === 400 && code === "invalid_request") || (status === 409 && code === "name_conflict") || (status === 422 && code === "unsupported_action"))) {
        if (await clearKnown({ key: ownKey.current, date: daily ? date : "", path: physicalPath })) { ownsIntent.current = false; setRecovery(null); original.current = null; setState("ready"); setMessage(status === 409 ? "当前目录已存在同名笔记，请修改标题。" : "创建在受理前被拒绝，标题保留；请纠正输入或确认服务已启用后重试。"); }
        return;
      }
      if (status === 401 || status === 403) { setTitle(""); setRecovery(null); setResult(null); original.current = null; setState("denied"); onDenied(); }
      else {
        try { const saved = await inspectNotebookRecovery(slot, parseNotebookPageRecovery); setRecovery(saved); setState(saved ? "unknown" : "blocked"); }
        catch (inspectError) { const inspectReason = notebookRecoveryReason(inspectError); setBlockedReason(inspectReason); setState("blocked"); }
        setMessage("创建或回查未完成，请查询原请求；未替换幂等键，也未将失败当作创建成功。");
      }
    } finally { submitting.current = false; }
  }

  async function resetInvalidRecovery() {
    try {
      await clearInvalidNotebookRecovery(slot, parseNotebookPageRecovery);
      setRecovery(null); setBlockedReason(null); setMessage("损坏恢复记录已清除，可以重新创建。"); setState(navigator.onLine ? "ready" : "offline");
    } catch (error) {
      const reason = notebookRecoveryReason(error); setBlockedReason(reason); setMessage(notebookRecoveryMessage(reason)); setState("blocked");
    }
  }

  return <AppDialog title={daily ? "创建日记" : "新建笔记"} close={() => { if (state !== "pending" && (ownsIntent.current || !title.trim() || window.confirm("放弃未提交的新笔记标题？"))) close(); }}>
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">通过服务端在“{physicalPath || "网盘根目录"}”创建真实 Markdown 文件。{parent ? "兼容旧页面关系。" : ""}</p>
      {state === "ready" && <>{daily ? <p>日记日期：{date}</p> : <label className="grid gap-2 text-sm">笔记标题<Input autoFocus maxLength={120} value={title} onChange={event => setTitle(event.target.value)} className="min-h-11"/></label>}<Button className="min-h-11" disabled={!daily && !title.trim()} onClick={() => void run("create")}>确认创建</Button></>}
      {state === "loading" && <p role="status">检查浏览器恢复能力…</p>}
      {state === "pending" && <p role="status">正在提交或回查原创建，请勿重复提交…</p>}
      {state === "unknown" && <p role="alert">结果待回查。关闭或刷新不会丢失原请求身份；标题不写入浏览器存储。</p>}
      {state === "offline" && <p role="alert">当前离线，未创建新请求；联网后请点击“重新检查”或手动查询原创建。</p>}
      {state === "denied" && <p role="alert">身份或权限已失效，创建信息已清除，请重新进入。</p>}
      {state === "blocked" && <div className="space-y-2"><p role="alert">{message || notebookRecoveryMessage(blockedReason || "unknown")}</p><div className="flex flex-wrap gap-2"><Button variant="outline" className="min-h-11" onClick={() => void refreshAvailability()}>重新检查</Button>{blockedReason === "invalid_recovery" && <Button variant="outline" className="min-h-11" onClick={() => void resetInvalidRecovery()}>清除损坏恢复记录并重新开始</Button>}</div></div>}
      {message && state !== "blocked" && <p role="alert">{message}</p>}
      {result?.operation_id && <p className="break-all text-xs">原操作：{result.operation_id}</p>}
      {state === "completed" && <p role="status">已创建固定版本：{result?.page?.path}。关闭后可在知识库读取。</p>}
      {recovery && (state === "unknown" || state === "offline") && <div className="flex flex-wrap gap-2"><Button variant="outline" className="min-h-11" onClick={() => void run("lookup")}>查询原创建</Button>{result?.found && <Button variant="outline" className="min-h-11" onClick={() => void run("resume")}>恢复原创建</Button>}</div>}
    </div>
  </AppDialog>;
}
