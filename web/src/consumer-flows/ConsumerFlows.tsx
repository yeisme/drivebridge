// Consumer flows page (文件流程): upload center, cloud retrieval,
// organization center, and delivery & discussion.
//
// Each center is gated by a read-only capability probe against its own
// contract action; a deployment that has not enabled the surface shows an
// explicit unsupported state instead of a broken form. Switching tenant or
// space remounts the page (key includes the data context), which invalidates
// every in-flight observation so a late response from the old context can

import type { ReactNode } from "react";

import { useEffect, useMemo, useState } from "react";
import { getDataContext, type Space, type UserIdentity } from "../api";
import { localizedError } from "../i18n";
import { Button } from "../components/ui/button";
import { memoryRecoveryStore, openRecoveryStore, type RecoveryStore } from "./compat";
import { probeFlowCapability, type FlowCapability } from "./client";
import { UploadCenter } from "./UploadCenter";
import { SearchCenter } from "./SearchCenter";
import { OrganizeCenter } from "./OrganizeCenter";
import { DeliverCenter } from "./DeliverCenter";
import "./consumer-flows.css";

export type CenterId = "upload" | "search" | "organize" | "deliver";

export const CENTERS: { id: CenterId; label: string; action: string }[] = [
  { id: "upload", label: "上传中心", action: "upload_batch_create" },
  { id: "search", label: "云端检索", action: "content_query" },
  { id: "organize", label: "整理中心", action: "organize_plan" },
  { id: "deliver", label: "交付与讨论", action: "delivery_prepare" },
];

// The probe action for a center, shared by tests and the shell.
export function centerProbeAction(id: CenterId): string {
  return CENTERS.find((center) => center.id === id)?.action ?? "";
}

type Capabilities = Record<CenterId, FlowCapability>;

const PROBING: Capabilities = { upload: "checking", search: "checking", organize: "checking", deliver: "checking" };

export function ConsumerFlows({
  user,
  spaces,
  initialCenter = "upload",
  onCenterChange,
  store,
}: {
  user: UserIdentity;
  spaces: Space[];
  initialCenter?: CenterId;
  onCenterChange?: (center: CenterId) => void;
  store?: RecoveryStore;
}) {
  const [center, setCenter] = useState<CenterId>(initialCenter);
  useEffect(() => { setCenter(initialCenter); }, [initialCenter]);
  const [capabilities, setCapabilities] = useState<Capabilities>(PROBING);
  const [probeErrors, setProbeErrors] = useState<Partial<Record<CenterId, unknown>>>({});
  const [probeEpoch, setProbeEpoch] = useState(0);
  const space = getDataContext().space || spaces[0]?.id || "";
  useEffect(() => {
    const controller = new AbortController();
    setCapabilities(PROBING);
    setProbeErrors({});
    void Promise.all(CENTERS.map(async entry => {
      try {
        const capability = await probeFlowCapability(entry.action, controller.signal);
        if (!controller.signal.aborted) setCapabilities(value => ({ ...value, [entry.id]: capability }));
      } catch (error) {
        if (!controller.signal.aborted) setProbeErrors(value => ({ ...value, [entry.id]: error }));
      }
    }));
    // A tenant/space switch voids every in-flight probe (the shared api()
    // helper aborts them): re-probe under the new context instead of
    // showing a stale or half-loaded gate.
    const onContext = () => setProbeEpoch((epoch) => epoch + 1);
    window.addEventListener("drivebridge-context", onContext);
    return () => {
      controller.abort();
      window.removeEventListener("drivebridge-context", onContext);
    };
  }, [space, probeEpoch]);

  // Recovery metadata store: IndexedDB when the browser provides it, an
  // in-memory fallback otherwise. The instance scope is the origin.
  const activeStore = useMemo<RecoveryStore>(() => {
    if (store) return store;
    try {
      return openRecoveryStore(location.origin);
    } catch {
      return memoryRecoveryStore();
    }
  }, [store]);

  const switchCenter = (next: CenterId) => {
    setCenter(next);
    onCenterChange?.(next);
  };

  const probeError = probeErrors[center];
  return (
    <div className="cf-page" data-center={center}>
      <div className="cf-tabs" role="tablist" aria-label="文件流程视图">
        {CENTERS.map((entry) => (
          <Button
            key={entry.id}
            variant="ghost"
            type="button"
            role="tab"
            tabIndex={center === entry.id ? 0 : -1}
            aria-selected={center === entry.id}
            className={center === entry.id ? "active" : ""}
            onClick={() => switchCenter(entry.id)}
            onKeyDown={(event) => {
              const tabs = Array.from(event.currentTarget.parentElement?.querySelectorAll<HTMLButtonElement>('[role="tab"]') ?? []);
              const index = tabs.indexOf(event.currentTarget);
              const delta = event.key === "ArrowRight" ? 1 : event.key === "ArrowLeft" ? -1 : 0;
              if (delta && tabs[index + delta]) {
                event.preventDefault();
                tabs[index + delta].focus();
                switchCenter(CENTERS[index + delta].id);
              }
            }}
          >
            {entry.label}
          </Button>
        ))}
      </div>
      {center === "search" ? <div role="tabpanel" aria-label="云端检索">
        {probeError != null && <div role="alert"><p>无法确认内容发现能力；元数据查询仍会独立鉴权。</p><Button variant="outline" onClick={() => setProbeEpoch(value => value + 1)}>重试能力检查</Button></div>}
        <SearchCenter spaces={spaces} capability={capabilities.search}/>
      </div> : probeError ? (
        <section className="cf-state cf-error" role="alert">
          <h2>无法确认流程能力</h2>
          <p>{flowErrorText(probeError)}</p>
          <Button variant="outline" type="button" onClick={() => location.reload()}>
            重新加载
          </Button>
        </section>
      ) : (
        <div role="tabpanel" aria-label={CENTERS.find((entry) => entry.id === center)?.label}>
          {center === "upload" && (
            <UploadCenter user={user} space={space} capability={capabilities.upload} store={activeStore} />
          )}
          {center === "organize" && <OrganizeCenter space={space} capability={capabilities.organize} actor={user.id} />}
          {center === "deliver" && <DeliverCenter spaces={spaces} capability={capabilities.deliver} actor={user.id} />}
        </div>
      )}
    </div>
  );
}

// Local error rendering for the flow surfaces. New contract codes get
// flow-specific next steps; everything else falls back to the shared locale
// table so old codes keep their established meaning.
const FLOW_ERROR_LABELS: Record<string, string> = {
  idempotency_conflict: "同一幂等键已用于不同请求内容。请勿重复提交；如需新内容请重新发起明确的新任务。",
  batch_not_sealed: "批次尚未封存，请继续添加文件后封存批次。",
  source_reselection_required: "需要重新选择原文件后才能继续恢复。",
  transfer_session_expired: "传输会话已过期，请先核对原目标，再通过原任务人工恢复。",
  part_conflict: "同一分片序号出现不同内容，已验证的分片不会被替换。",
  plan_stale: "计划在确认后已发生变化，请重新预览具体影响后再确认。",
  policy_revoked: "整理规则已撤销，未执行的项已停止；已完成的项保持不变。",
  compensation_conflict: "文件此后又被修改，不能自动覆盖；冲突项保留待人工处理。",
  delivery_incomplete: "交付清单尚有缺项，不能发布不完整的集合。",
  retention_not_ready: "保留保护尚未就绪，发布前需要先建立真实的保留保护。",
  receiver_identity_required: "匿名下载不能登记为实名确认。",
  comment_write_forbidden: "当前身份可以阅读但不允许发言。",
  anchor_version_unavailable: "讨论锚定的原件版本已不可读；授权的讨论仍保留。",
  revision_conflict: "内容已被他人修改，草稿已保留；请重读最新内容后再提交。",
  content_index_stale: "内容索引已过期，返回的是旧版本标记；不会把旧证据当作新字节。",
  modality_unsupported: "缺少对应模态的处理能力，已保留元数据检索。",
  content_policy_required: "需要先选择已批准的内容策略。",
};

export function flowErrorText(error: unknown): string {
  const code = error instanceof Error ? error.message : "request_failed";
  return FLOW_ERROR_LABELS[code] ?? localizedError(new Error(code));
}

// Capability gate wrapper shared by all centers: while probing show a loading
// state, when absent show the honest unsupported message with the contract
// action name that was probed.
export function CapabilityGate({
  capability,
  action,
  children,
}: {
  capability: FlowCapability;
  action: string;
  children: ReactNode;
}) {
  if (capability === "checking")
    return (
      <section className="cf-state" role="status" data-state="loading">
        <p>正在确认此部署是否启用对应能力…</p>
      </section>
    );
  if (capability === "unsupported")
    return (
      <section className="cf-state" role="status" data-state="unsupported">
        <h2>此部署未启用对应能力</h2>
        <p>
          服务端尚未开放 <code>{action}</code> 流程（合同已冻结，功能分片由服务端各能力包启用）。既有上传、检索与分享入口不受影响。
        </p>
      </section>
    );
  return <>{children}</>;
}
