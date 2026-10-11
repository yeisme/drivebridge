import { auditActionLabel } from "./insights/audit-labels";

export function activityLabel(action: string) {
  return auditActionLabel(action);
}

export function activityOutcomeLabel(outcome: string) {
  return ({success:"已完成",completed:"已完成",intent:"操作请求",denied:"访问被拒绝",forbidden:"访问被拒绝",failed:"未完成",error:"未完成",cancelled:"已取消",interrupted:"已中断"} as Record<string,string>)[outcome] || "结果待确认";
}

export function agentState(expires: string, revoked?: boolean, archived?: boolean) {
  if (archived) return "已归档";
  if (revoked) return "已撤销";
  if (expires && new Date(expires) < new Date()) return "已到期";
  return "有效";
}

export function filterAgents<T extends { name: string; revoked: boolean; archived?: boolean; expires_at: string }>(
  agents: T[],
  query: string,
  filter: string,
) {
  const q = query.trim().toLowerCase();
  return agents.filter((a) => {
    const state = agentState(a.expires_at, a.revoked, a.archived);
    if (filter === "active" && state !== "有效") return false;
    if (filter !== "all" && filter !== "active" && state !== filter) return false;
    return `${a.name}`.toLowerCase().includes(q);
  });
}
