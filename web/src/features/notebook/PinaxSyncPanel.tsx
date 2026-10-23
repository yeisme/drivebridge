import { useEffect, useMemo, useState } from "react";
import { Button } from "../../components/ui/button";
import type { DesktopBridge, DesktopResult } from "../../generated/desktop-ipc";
import type { components as Pinax } from "../../generated/pinax-writer-sync";

type SyncProjection = Pinax["schemas"]["pinax.projection.v1"];
type SyncBridge = Pick<DesktopBridge, "syncStatus" | "syncDiff" | "listSyncConflicts">;

function resultError(result: DesktopResult<SyncProjection>, command: string): string | undefined {
  if ("error" in result) return result.error?.code || "sync_unavailable";
  if (!result.data || result.data.spec_version !== "1.0" || result.data.command !== command) return "sync_invalid_response";
  if (result.data.error || result.data.status !== "success") return result.data.error?.code || "sync_unavailable";
  return undefined;
}

export function PinaxSyncPanel({ bridge, sourceId }: { bridge: SyncBridge; sourceId: string }) {
  const { syncStatus, syncDiff, listSyncConflicts } = bridge;
  const binding = useMemo(() => ({ sourceId, syncStatus, syncDiff, listSyncConflicts }), [sourceId, syncStatus, syncDiff, listSyncConflicts]);
  const [attempt, setAttempt] = useState(0);
  const [snapshot, setSnapshot] = useState<{ binding: typeof binding; attempt: number; status?: SyncProjection; diff?: SyncProjection; conflicts?: SyncProjection; error?: string }>();
  useEffect(() => {
    let active = true;
    const failure = (error: string) => { if (active) setSnapshot({ binding, attempt, error }); };
    Promise.all([
      binding.syncStatus({ input: {}, source_id: binding.sourceId }),
      binding.syncDiff({ input: { dry_run: true, content_diff: false }, source_id: binding.sourceId }),
      binding.listSyncConflicts({ input: {}, source_id: binding.sourceId }),
    ]).then(([statusResult, diffResult, conflictsResult]) => {
      if (!active) return;
      const code = resultError(statusResult, "sync.status") || resultError(diffResult, "sync.diff") || resultError(conflictsResult, "sync.conflicts.list");
      if (code || !("data" in statusResult) || !("data" in diffResult) || !("data" in conflictsResult)) { failure(code || "sync_invalid_response"); return; }
      setSnapshot({ binding, attempt, status: statusResult.data, diff: diffResult.data, conflicts: conflictsResult.data });
    }).catch(() => { failure("sync_unavailable"); });
    return () => { active = false; };
  }, [attempt, binding]);
  const current = snapshot?.binding === binding && snapshot.attempt === attempt ? snapshot : undefined;
  const { status, diff, conflicts, error } = current || {};
  const loading = current === undefined;
  const facts = status?.facts || {};
  const diffFacts = diff?.facts || {};
  return <section aria-label="Pinax Cloud Sync" className="rounded-md border p-3">
    <div className="flex flex-wrap items-center gap-2"><h3 className="m-0 text-sm font-semibold">Cloud Sync</h3><Button type="button" variant="ghost" className="min-h-11 md:min-h-8" onClick={() => setAttempt(value => value + 1)} disabled={loading}>{loading ? "读取中…" : "刷新状态"}</Button><span className="ml-auto text-xs text-muted-foreground">DriveBridge 只显示 owner 事实</span></div>
    {error && <p role="alert" className="text-xs text-destructive">{error}</p>}
    {!error && <dl className="mt-2 grid gap-2 text-xs sm:grid-cols-2"><div><dt className="text-muted-foreground">状态</dt><dd className="m-0">{facts.sync_status || (loading ? "pending" : "未提供")}</dd></div><div><dt className="text-muted-foreground">上次同步 revision</dt><dd className="m-0 break-all">{facts.last_synced_revision || "未提供"}</dd></div><div><dt className="text-muted-foreground">远端 revision</dt><dd className="m-0 break-all">{facts.remote_revision || "未提供"}</dd></div><div><dt className="text-muted-foreground">最近运行</dt><dd className="m-0 break-all">{facts.run_id || "未提供"}</dd></div><div><dt className="text-muted-foreground">计划</dt><dd className="m-0">{diff?.summary || diffFacts.plan || "未生成"}</dd></div><div><dt className="text-muted-foreground">冲突</dt><dd className="m-0">{conflicts?.facts?.conflicts || facts.conflicts || diffFacts.conflicts || "未提供"}</dd></div><div><dt className="text-muted-foreground">远端 durable commit</dt><dd className="m-0">{facts.remote_write || "未确认"}</dd></div><div><dt className="text-muted-foreground">恢复动作</dt><dd className="m-0 break-all">{facts.recovery || "operation.show / operation.reconcile"}</dd></div></dl>}
    <div className="mt-3 flex flex-wrap gap-2"><Button type="button" disabled title="需要 Electron host 的 write admission">确认 Push</Button><Button type="button" variant="outline" disabled title="需要 Electron host 的 write admission">确认 Pull</Button><Button type="button" variant="outline" disabled title="需要单独的冲突输入合同">解决冲突</Button></div>
    <p className="m-0 mt-2 text-xs text-muted-foreground">执行流程：读取状态 → 生成计划 → 展示影响 → 用户确认 → 原 operation 回读。当前 host write admission 尚未开放。</p>
  </section>;
}
