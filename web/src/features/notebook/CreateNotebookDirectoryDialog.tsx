import { useEffect, useRef, useState } from "react";
import { getDataContext } from "../../api";
import { AppDialog } from "../../components/AppDialog";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { useRouteLeaveGuard } from "../../app/route-leave";
import { createNotebookDirectory, lookupNotebookDirectory, type DirectoryCreation } from "./clients";
import {
  claimNotebookRecovery,
  clearInvalidNotebookRecovery,
  clearNotebookRecovery,
  encodeNotebookRecovery,
  inspectNotebookRecovery,
  newNotebookKey,
  notebookDirectorySlot,
  notebookRecoveryMessage,
  notebookRecoveryReason,
  parseNotebookDirectoryRecovery,
  type NotebookDirectoryRecovery,
} from "./notebook-recovery";

type State = "loading" | "ready" | "pending" | "unknown" | "completed" | "denied" | "offline" | "blocked";
type BlockedReason = ReturnType<typeof notebookRecoveryReason>;

function isAbort(error: unknown) {
  return error && typeof error === "object" && "name" in error && error.name === "AbortError";
}

export function CreateNotebookDirectoryDialog({ actor, space, parentPath, close, onCreated, onDenied }: {
  actor: string;
  space: string;
  parentPath: string;
  close: () => void;
  onCreated: (path: string) => void;
  onDenied: () => void;
}) {
  const context = getDataContext();
  const slot = notebookDirectorySlot(actor, context.tenant, space, parentPath);
  const [name, setName] = useState("");
  const [state, setState] = useState<State>("loading");
  const [recovery, setRecovery] = useState<NotebookDirectoryRecovery | null>(null);
  const [result, setResult] = useState<DirectoryCreation | null>(null);
  const [message, setMessage] = useState("");
  const [blockedReason, setBlockedReason] = useState<BlockedReason | null>(null);
  const controller = useRef<AbortController | null>(null);
  const active = useRef(true);
  const submitting = useRef(false);
  const ownRecovery = useRef<NotebookDirectoryRecovery | null>(null);
  const nameError = name.includes("/") || name.includes("\\") ? "目录名称不能包含路径分隔符。" : "";

  useRouteLeaveGuard(state !== "completed" && Boolean(name.trim()), "放弃未提交的新目录名称并离开？");

  async function refreshAvailability() {
    if (!active.current) return;
    setState("loading"); setMessage(""); setBlockedReason(null);
    if (!navigator.onLine) { setState("offline"); return; }
    try {
      const saved = await inspectNotebookRecovery(slot, parseNotebookDirectoryRecovery);
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
    const invalidate = () => { active.current = false; controller.current?.abort(); ownRecovery.current = null; setState("denied"); setName(""); setRecovery(null); };
    const offline = () => { controller.current?.abort(); setState("offline"); };
    window.addEventListener("drivebridge-context", invalidate); window.addEventListener("offline", offline);
    void refreshAvailability();
    return () => { active.current = false; controller.current?.abort(); window.removeEventListener("drivebridge-context", invalidate); window.removeEventListener("offline", offline); };
  }, [slot]);

  async function clearKnown(identity: NotebookDirectoryRecovery) {
    try {
      await clearNotebookRecovery(slot, parseNotebookDirectoryRecovery, identity.key, value => value.key);
      if (active.current) { ownRecovery.current = null; setRecovery(null); }
      return true;
    } catch (error) {
      const reason = notebookRecoveryReason(error);
      if (active.current) { setBlockedReason(reason); setMessage(notebookRecoveryMessage(reason)); }
      return false;
    }
  }

  async function accept(value: DirectoryCreation, identity: NotebookDirectoryRecovery) {
    if (!active.current) return;
    if (!value || typeof value.status !== "string") throw new Error("invalid_contract");
    if (value.status !== "completed" || !value.directory?.path) { setResult(value); setState("unknown"); setMessage(value.found ? "原创建尚未完成，请继续查询。" : "尚未找到原请求，不代表未执行；不会创建新目录。"); return; }
    const cleared = await clearKnown(identity);
    if (!cleared && active.current) setMessage("已确认目录创建成功，但浏览器未能清理恢复记录；后续只会查询原请求，不会重复创建。");
    if (!active.current) return;
    setResult(value); setState("completed"); onCreated(value.directory.path);
  }

  async function run(mode: "create" | "lookup") {
    if (submitting.current || !active.current) return;
    if (!navigator.onLine) { setState("offline"); return; }
    submitting.current = true;
    const request = new AbortController(); controller.current = request; setState("pending"); setMessage("");
    try {
      let identity = recovery;
      if (mode === "create") {
        const next: NotebookDirectoryRecovery = { key: newNotebookKey("directory"), path: [parentPath, name.trim()].filter(Boolean).join("/") };
        const claimed = await claimNotebookRecovery(slot, parseNotebookDirectoryRecovery, next, encodeNotebookRecovery, request.signal);
        identity = claimed.record; setRecovery(identity);
        if (!claimed.claimed) { ownRecovery.current = null; setState("unknown"); setMessage("另一个标签页已有原请求，请先查询原创建。"); return; }
        ownRecovery.current = identity;
      }
      if (!identity) throw new Error("missing_recovery");
      if (mode === "lookup") {
        const current = await inspectNotebookRecovery(slot, parseNotebookDirectoryRecovery);
        if (!current || current.key !== identity.key) throw new Error("recovery_changed");
      }
      const value = mode === "create" ? await createNotebookDirectory(space, identity.path, identity.key, request.signal) : await lookupNotebookDirectory(space, identity.key, request.signal);
      await accept(value, identity);
    } catch (error) {
      if (!active.current || request.signal.aborted || isAbort(error)) return;
      const status = error && typeof error === "object" && "status" in error ? error.status : 0;
      const code = error instanceof Error ? error.message : "";
      const reason = notebookRecoveryReason(error);
      if (reason !== "unknown") { setBlockedReason(reason); setState(reason === "recovery_changed" ? "unknown" : "blocked"); setMessage(notebookRecoveryMessage(reason)); return; }
      if (mode === "create" && ((status === 409 && code === "name_conflict") || status === 400)) {
        const identity = ownRecovery.current || recovery;
        if (identity && await clearKnown(identity)) { setRecovery(null); setState("ready"); setMessage(status === 409 ? "当前目录已存在同名目录，请修改名称。" : "目录名称无效，请修改后重试。"); }
      } else if (status === 401 || status === 403) { ownRecovery.current = null; setRecovery(null); setState("denied"); onDenied(); }
      else {
        try { const saved = await inspectNotebookRecovery(slot, parseNotebookDirectoryRecovery); setRecovery(saved); setState(saved ? "unknown" : "blocked"); }
        catch (inspectError) { const inspectReason = notebookRecoveryReason(inspectError); setBlockedReason(inspectReason); setState("blocked"); }
        setMessage("创建或回查未完成，请查询原创建；未替换幂等键。");
      }
    } finally { submitting.current = false; }
  }

  async function resetInvalidRecovery() {
    try {
      await clearInvalidNotebookRecovery(slot, parseNotebookDirectoryRecovery);
      setRecovery(null); setBlockedReason(null); setMessage("损坏恢复记录已清除，可以重新创建。"); setState(navigator.onLine ? "ready" : "offline");
    } catch (error) {
      const reason = notebookRecoveryReason(error); setBlockedReason(reason); setMessage(notebookRecoveryMessage(reason)); setState("blocked");
    }
  }

  return <AppDialog title="新建目录" close={() => { if (state !== "pending" && (!name.trim() || window.confirm("放弃未提交的新目录名称？"))) close(); }}>
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">将在“{parentPath || "网盘根目录"}”中创建真实网盘目录。</p>
      {state === "ready" && <label className="grid gap-2 text-sm">目录名称<Input autoFocus maxLength={120} value={name} onChange={event => setName(event.target.value)} className="min-h-11"/>{nameError && <span className="text-xs text-destructive">{nameError}</span>}</label>}
      {state === "ready" && <Button className="min-h-11" disabled={!name.trim() || Boolean(nameError)} onClick={() => void run("create")}>确认创建</Button>}
      {state === "loading" && <p role="status">检查浏览器恢复能力…</p>}
      {state === "pending" && <p role="status">正在创建或回查原目录，请勿重复提交…</p>}
      {state === "unknown" && <p role="alert">结果待回查。原请求身份已保留。</p>}
      {state === "offline" && <p role="alert">当前离线，未创建新请求；联网后请点击“重新检查”或手动查询原创建。</p>}
      {state === "denied" && <p role="alert">身份或权限已失效，创建信息已清除。</p>}
      {state === "blocked" && <div className="space-y-2"><p role="alert">{message || notebookRecoveryMessage(blockedReason || "unknown")}</p><div className="flex flex-wrap gap-2"><Button variant="outline" className="min-h-11" onClick={() => void refreshAvailability()}>重新检查</Button>{blockedReason === "invalid_recovery" && <Button variant="outline" className="min-h-11" onClick={() => void resetInvalidRecovery()}>清除损坏恢复记录并重新开始</Button>}</div></div>}
      {message && state !== "blocked" && <p role="alert">{message}</p>}
      {state === "completed" && <p role="status">目录已创建：{result?.directory?.path}</p>}
      {recovery && (state === "unknown" || state === "offline") && <Button variant="outline" className="min-h-11" onClick={() => void run("lookup")}>查询原创建</Button>}
    </div>
  </AppDialog>;
}
