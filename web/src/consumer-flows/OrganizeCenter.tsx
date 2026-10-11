// Organize center (整理中心): risk-graded organization plans.
//
// The flow materializes an exact file query into a frozen snapshot
// collection first, so plan confirmation binds exact members and versions —
// a live search is never executed as if it were a fixed set. Preview shows
// per-item before/after, risk, rule authorization and impact digest;
// confirmation submits expected_revision. A plan that changed after preview
// is stale: the center demands a fresh preview and never re-confirms
// automatically. Partial execution keeps per-item results, and compensation
// is a new reverse plan, not an implicit undo.

import { useState } from "react";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { Table } from "../components/ui/table";
import { queryFiles } from "../info-workspace/client";
import { CapabilityGate, flowErrorText } from "./ConsumerFlows";
import {
  confirmOrganizationPlan,
  createCollectionSnapshot,
  createCompensationPlan,
  createFileCollection,
  createOrganizationPlan,
  executeOrganizationPlan,
  getOrganizationPlan,
  getOrganizationPolicy,
  type FileCollectionView,
  type FileSnapshotView,
  type FlowCapability,
  type OrganizationPolicyView,
} from "./client";
import {
  confirmDecision,
  partialOutcome,
  type OrganizationItem,
  type OrganizationPlanView,
} from "./model";

type Stage =
  | { kind: "source" }
  | { kind: "plan"; plan: OrganizationPlanView; items: OrganizationItem[] }
  | { kind: "executed"; plan: OrganizationPlanView; items: OrganizationItem[] };

export function OrganizeCenter({
  space,
  capability,
}: {
  space: string;
  capability: FlowCapability;
}) {
  const [stage, setStage] = useState<Stage>({ kind: "source" });
  const [prefix, setPrefix] = useState("");
  const [policy, setPolicy] = useState<OrganizationPolicyView | null>(null);
  const [policyLoaded, setPolicyLoaded] = useState(false);
  const [snapshot, setSnapshot] = useState<FileSnapshotView | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [stale, setStale] = useState(false);
  // The action kind the user chose for the current plan. A stale re-preview
  // must reuse it: silently relabeling a confirmed move as add_label would
  // change what the user is asked to re-approve.
  const [planKind, setPlanKind] = useState("add_label");
  const [confirmDigest, setConfirmDigest] = useState("");

  const loadPolicy = async () => {
    if (!space) return;
    try {
      // null = first use without a policy: a normal state, never an error.
      setPolicy(await getOrganizationPolicy(space));
    } catch (err) {
      setError(flowErrorText(err));
    } finally {
      setPolicyLoaded(true);
    }
  };

  // Materialize the exact prefix query into a frozen snapshot collection.
  // The plan then references the snapshot id, never a live query.
  const materialize = async () => {
    setBusy(true);
    setError("");
    try {
      const exact = await queryFiles({
        mode: "exact",
        space_ids: space ? [space] : [],
        filters: prefix ? [{ field: "prefix", op: "contains", value: prefix }] : [],
        limit: 200,
      });
      if (exact.items.length === 0) {
        setError("该前缀下没有可整理的文件，计划需要明确的固定集合。");
        return;
      }
      const key = crypto.randomUUID();
      const collection = await createFileCollection({ scope: "manual", space_id: space, members: exact.items.map((item) => item.file_id) }, key);
      const snap = await createCollectionSnapshot(collection.collection_id);
      setSnapshot(snap);
      await loadPolicy();
    } catch (err) {
      setError(flowErrorText(err));
    } finally {
      setBusy(false);
    }
  };

  const createPlan = async (kind: string) => {
    if (!snapshot) return;
    setBusy(true);
    setError("");
    setPlanKind(kind);
    try {
      const key = crypto.randomUUID();
      const plan = await createOrganizationPlan(
        {
          source_snapshot_id: snapshot.snapshot_id,
          actions: [kind === "move" ? { kind: "move", target_prefix: "/整理后/" } : { kind, namespace: "rule", value: "reference" }],
          policy_id: policy?.policy_id ?? "",
        },
        key,
      );
      const page = await getOrganizationPlan(plan.plan_id, { limit: 200 });
      setConfirmDigest(plan.impact_digest);
      setStage({ kind: "plan", plan: page.plan, items: page.items });
    } catch (err) {
      setError(flowErrorText(err));
    } finally {
      setBusy(false);
    }
  };

  const confirm = async () => {
    if (stage.kind !== "plan") return;
    const decision = confirmDecision(stage.plan, stage.plan.revision);
    if (decision.kind === "approval_required") {
      setError("该计划包含需要人工确认的动作（Agent 不能自批）；请在预览中逐项确认风险后再执行。");
      return;
    }
    if (decision.kind === "stale") {
      setStale(true);
      return;
    }
    setBusy(true);
    try {
      const confirmed = await confirmOrganizationPlan(stage.plan.plan_id, {
        expected_revision: stage.plan.revision,
        impact_digest: confirmDigest || stage.plan.impact_digest,
      });
      await runExecution(confirmed);
    } catch (err) {
      setError(flowErrorText(err));
    } finally {
      setBusy(false);
    }
  };

  const runExecution = async (plan: OrganizationPlanView) => {
    try {
      const key = crypto.randomUUID();
      const executed = await executeOrganizationPlan(plan.plan_id, { expected_revision: plan.revision }, key);
      const page = await getOrganizationPlan(executed.plan_id, { limit: 200 });
      setStage({ kind: "executed", plan: page.plan, items: page.items });
    } catch (err) {
      // plan_stale: the plan changed after confirmation — demand a fresh
      // preview instead of silently re-confirming the old approval.
      if (err instanceof Error && err.message === "plan_stale") setStale(true);
      else setError(flowErrorText(err));
    }
  };

  const compensate = async () => {
    if (stage.kind !== "executed") return;
    setBusy(true);
    try {
      const key = crypto.randomUUID();
      const reverse = await createCompensationPlan(stage.plan.plan_id, key);
      const page = await getOrganizationPlan(reverse.plan_id, { limit: 200 });
      setStage({ kind: "plan", plan: page.plan, items: page.items });
    } catch (err) {
      setError(flowErrorText(err));
    } finally {
      setBusy(false);
    }
  };

  const repreview = async () => {
    if (stage.kind === "source" || !snapshot) return;
    setStale(false);
    // Re-preview keeps the user's chosen action kind (see planKind).
    await createPlan(planKind);
  };

  return (
    <CapabilityGate capability={capability} action="organize_plan">
      <section className="cf-center" aria-label="整理中心">
        {error && (
          <div className="banner error" role="alert">
            {error}
          </div>
        )}
        {stale && (
          <div className="banner warn" role="alert" data-stale="plan">
            计划在预览后已发生变化（plan_stale）。请重新预览具体影响后再确认；已给出的批准不会自动重放。
          </div>
        )}

        {stage.kind === "source" && (
          <div className="cf-card" data-region="source">
            <h3>固定整理集合</h3>
            <p className="cf-hint">整理计划只能作用于固定的文件/版本集合。先按目录前缀精确查询，再物化为快照；语义候选不会作为全集执行。</p>
            <div className="cf-row">
              <Input type="text" value={prefix} onChange={(event) => setPrefix(event.target.value)} placeholder="目录前缀，例如 docs/" aria-label="目录前缀" />
              <Button variant="default" type="button" disabled={busy} onClick={() => void materialize()}>
                {busy ? "处理中…" : "物化快照"}
              </Button>
            </div>
            {snapshot && (
              <p role="status">
                快照已固定：<code>{snapshot.snapshot_id}</code>（{snapshot.item_count} 项）
                {policy && <> · 规则 <code>{policy.policy_id}</code>（revision {policy.revision}，{policy.enabled ? "已启用" : "未启用"}）</>}
              </p>
            )}
            {snapshot && policyLoaded && !policy && (
              <p className="cf-hint" role="status">
                整理策略未配置（首次使用属正常状态）：策略由本人在系统侧创建后，整理计划才能在规则授权内执行。
              </p>
            )}
            {snapshot && (
              <div className="cf-actions">
                <Button variant="outline" type="button" onClick={() => void createPlan("add_label")}>
                  生成标签整理计划
                </Button>
                <Button variant="outline" type="button" onClick={() => void createPlan("move")}>
                  生成移动计划
                </Button>
              </div>
            )}
          </div>
        )}

        {(stage.kind === "plan" || stage.kind === "executed") && (
          <div className="cf-card" data-region="plan">
            <h3>计划 {stage.plan.plan_id}</h3>
            <dl className="cf-facts">
              <div>
                <dt>阶段</dt>
                <dd>{stage.plan.phase}</dd>
              </div>
              <div>
                <dt>风险</dt>
                <dd>{stage.plan.risk}</dd>
              </div>
              <div>
                <dt>规则授权</dt>
                <dd>{stage.plan.approval_required ? "需要人工确认" : "规则范围内预授权"}</dd>
              </div>
              <div>
                <dt>影响摘要</dt>
                <dd>
                  <code>{stage.plan.impact_digest || "（未取得）"}</code>
                </dd>
              </div>
              <div>
                <dt>项数</dt>
                <dd>{stage.plan.items_total}</dd>
              </div>
            </dl>
            {stage.items.length > 0 && (
              <Table className="cf-table" aria-label="计划项">
                <thead>
                  <tr>
                    <th>文件</th>
                    <th>动作</th>
                    <th>before → after</th>
                    <th>状态</th>
                  </tr>
                </thead>
                <tbody>
                  {stage.items.map((item) => (
                    <tr key={item.item_key}>
                      <td>
                        <code>{item.file_id}</code>
                      </td>
                      <td>{item.action}</td>
                      <td>
                        {item.before} → {item.after}
                      </td>
                      <td>
                        {item.state}
                        {item.reason ? `（${item.reason}）` : ""}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </Table>
            )}
            {stage.kind === "executed" && (
              <p role="status">
                执行结果：成功 {partialOutcome(stage.items).succeeded} · 失败 {partialOutcome(stage.items).failed} · 待处理 {partialOutcome(stage.items).pending}。已成功的项不会重做；失败项需重新授权。
              </p>
            )}
            {stage.kind === "plan" && (
              <div className="cf-row">
                <label className="cf-label" htmlFor="cf-impact">
                  确认影响摘要
                </label>
                <Input id="cf-impact" type="text" value={confirmDigest} onChange={(event) => setConfirmDigest(event.target.value)} aria-label="确认影响摘要" />
              </div>
            )}
            <div className="cf-actions">
              {stage.kind === "plan" && (
                <Button variant="default" type="button" disabled={busy} onClick={() => void confirm()}>
                  确认并执行
                </Button>
              )}
              {stage.kind === "executed" && (
                <Button variant="outline" type="button" disabled={busy} onClick={() => void compensate()}>
                  生成补偿计划
                </Button>
              )}
              <Button variant="ghost" type="button" onClick={() => void repreview()}>
                重新预览
              </Button>
              <Button variant="ghost" type="button" onClick={() => setStage({ kind: "source" })}>
                返回集合选择
              </Button>
            </div>
          </div>
        )}
      </section>
    </CapabilityGate>
  );
}
