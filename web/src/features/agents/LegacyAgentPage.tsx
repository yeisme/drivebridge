import { Bot, Clock } from "lucide-react";
import type { Agent, Audit, Space } from "../../api";
import { activityLabel, activityOutcomeLabel, agentState, filterAgents } from "../../activity";
import { handleListKey } from "../../page-list";
import { SearchField } from "../../components/SearchField";
import { SelectAdapter } from "../../components/SelectAdapter";
import { Checkbox } from "../../components/ui/checkbox";
import { Button } from "../../components/ui/button";

interface Props {
  agents: Agent[];
  audits: Audit[];
  spaces: Space[];
  ownerLabel: string;
  query: string;
  filter: string;
  archived: boolean;
  onQuery: (value: string) => void;
  onFilter: (value: string) => void;
  onArchived: (value: boolean) => void;
  onRevoke: (agent: Agent) => void;
  onArchive: (agent: Agent) => void;
}

// Compatibility view only: mutations and confirmation remain with the owner.
export function LegacyAgentPage(p: Props) {
  const visible = filterAgents(p.agents.filter(a => p.archived || !a.archived), p.query, p.filter);
  return <>
    <div className="workspace-toolbar">
      <SearchField aria-label="搜索机器人" placeholder="搜索名称…" value={p.query} onChange={e => p.onQuery(e.target.value)} />
      <SelectAdapter aria-label="机器人状态" value={p.filter} onChange={e => p.onFilter(e.target.value)}>
        {[["all", "全部"], ["active", "有效"], ["已撤销", "已撤销"], ["已到期", "已到期"], ["已归档", "已归档"]].map(([id, label]) => <option key={id} value={id}>{label}</option>)}
      </SelectAdapter>
      <label className="checkbox-label"><Checkbox checked={p.archived} onCheckedChange={p.onArchived} />显示已归档</label>
    </div>
    <div className="card-stack" role="list" aria-label="机器人授权" onKeyDown={e => handleListKey(e.nativeEvent, e.currentTarget, { open(el) {
      const agent = visible.find(a => a.id === el.dataset.id);
      if (agent && !agent.revoked) p.onRevoke(agent);
    } })}>
      {!p.agents.length && <div className="empty"><Bot size={36} /><h3>还没有获准的机器人</h3><p>创建第一份授权，把文件交给你的助手。</p></div>}
      {visible.map(a => {
        const state = agentState(a.expires_at, a.revoked, a.archived);
        return <article className="agent-card" role="listitem" tabIndex={0} data-list-item data-id={a.id} key={a.id}>
          <span className="agent-avatar"><Bot size={22} /></span>
          <div>
            <h3>{a.name} <span className={"pill " + (state === "有效" ? "" : "muted")}>{state}</span></h3>
            <p>{a.prefix || "空间内全部目录"} · {a.scopes.split(",").map(s => ({ read: "读取", write: "上传", delete: "回收站", import: "导入", export: "回存" }[s] || s)).join(" / ")}</p>
            <small className="agent-binding">授权标识：{a.id.slice(0, 12)} · 归属：{p.ownerLabel} · 网盘：{p.spaces.find(s => s.id === a.space_id)?.name || a.space_id}</small>
            <small>{state === "有效" ? `剩余 ${Math.max(0, Math.ceil((new Date(a.expires_at).getTime() - Date.now()) / 86400000))} 天 · 至 ${new Date(a.expires_at).toLocaleDateString("zh-CN")}` : `有效期至 ${new Date(a.expires_at).toLocaleDateString("zh-CN")}`}</small>
          </div>
          {!a.archived && <Button variant="destructive" size="sm" type="button" onClick={() => p.onArchive(a)}>删除并归档</Button>}
          {!a.revoked && <Button variant="outline" size="sm" type="button" onClick={() => p.onRevoke(a)}>撤销授权</Button>}
        </article>;
      })}
    </div>
    <div className="section-line"><h3>最近活动</h3><span className="count">最近 {Math.min(p.audits.length, 12)} 条</span></div>
    <div className="activity-list [&>div]:text-sm [&_time]:text-xs" aria-label="最近活动">
      {p.audits.slice(0, 12).map(a => <div key={a.id}>
        {a.action.startsWith("agents.") ? <Bot size={16} className="shrink-0" /> : <Clock size={16} className="shrink-0" />}
        <span className="min-w-0 break-words" title={a.action}>{activityLabel(a.action)}</span>
        <span title={a.outcome} className={"shrink-0 text-xs " + (["failed", "error", "denied", "forbidden"].includes(a.outcome) ? "error" : "")}>{activityOutcomeLabel(a.outcome)}</span>
        <time className="shrink-0 tabular-nums" title={new Date(a.at).toLocaleString("zh-CN")}>{new Date(a.at).toLocaleTimeString("zh-CN")}</time>
      </div>)}
      {!p.audits.length && <p className="field-help">还没有可展示的活动。</p>}
    </div>
  </>;
}
