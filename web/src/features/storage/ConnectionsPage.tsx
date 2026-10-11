import { ArrowUpRight, Cloud, Plus, ShieldCheck } from "lucide-react";
import type { Connection } from "../../api";
import { backendOf, capabilityText } from "../../storage";
import { Button } from "../../components/ui/button";
import { WorkspacePageHeader } from "../../components/WorkspacePageHeader";

interface Props {
  connections: Connection[];
  onCheck: (connection: Connection) => void;
  onBrowse: (connection: Connection) => void;
  onUse: (connection: Connection) => void;
  onDisconnect: (connection: Connection) => void;
  onAdd: () => void;
}

// Presentation only: credentials, provider access and mutations stay server-owned.
export function ConnectionsPage(p: Props) {
  return <section aria-label="存储管理" className="min-w-0 space-y-4">
    <WorkspacePageHeader title="存储管理" description="管理已配置的连接。保存配置不代表当前连接可用，验证由你主动发起。" actions={<Button type="button" className="min-h-11 text-[13px] md:min-h-8" onClick={p.onAdd}><Plus size={16}/>连接存储</Button>}/>
    {p.connections.length ? <div className="divide-y divide-border rounded-lg border border-border bg-card">
      {p.connections.map(c => <article className="flex min-w-0 flex-wrap items-center gap-x-4 gap-y-3 p-3" key={c.id}>
        <Cloud className="size-5 shrink-0 text-muted-foreground" aria-hidden="true"/>
        <div className="min-w-0 flex-[1_1_16rem] space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="m-0 break-all text-sm font-semibold">{c.name}</h3>
            <span className="rounded bg-muted px-2 py-1 text-xs text-muted-foreground">{c.disabled ? "已断开" : c.status === "verified" ? "验证记录：通过" : c.status === "connected" || c.status === "configured" ? "已配置 · 未验证" : "验证状态未知"}</span>
          </div>
          <p className="m-0 text-xs leading-5 text-muted-foreground">{c.id === "platform" ? "管理员配置的平台存储（S3）" : `${backendOf(c.kind).label} · ${capabilityText(backendOf(c.kind))}`}</p>
        </div>
        <div className="flex flex-wrap gap-1">
          <Button variant="outline" className="min-h-11 text-[13px] md:min-h-8" type="button" disabled={c.disabled} onClick={() => p.onCheck(c)}>验证连接</Button>
          <Button variant="ghost" type="button" className="min-h-11 text-[13px] md:min-h-8" disabled={c.disabled} onClick={() => c.kind === "onedrive" ? p.onBrowse(c) : p.onUse(c)}>
            {c.kind === "onedrive" ? "浏览文件" : "用于空间"} <ArrowUpRight size={14} />
          </Button>
          {c.id !== "platform" && !c.disabled && <Button variant="ghost" type="button" className="min-h-11 text-[13px] md:min-h-8" onClick={() => p.onDisconnect(c)}>断开连接</Button>}
        </div>
      </article>)}
    </div> : <div className="space-y-3 border-y border-border py-6"><p className="text-sm text-muted-foreground">当前没有已配置的存储连接。</p><Button variant="outline" type="button" className="min-h-11 md:min-h-8" onClick={p.onAdd}><Plus size={16}/>连接你已有的存储</Button></div>}
    <div className="flex items-start gap-2 text-xs leading-5 text-muted-foreground"><ShieldCheck className="mt-0.5 size-4 shrink-0"/><p className="m-0">验证记录不代表此刻在线或所有读写已通过。连接凭据由服务端加密保管。断开连接会保留文件引用；已有导入副本不会随 OneDrive 原文件删除。</p></div>
  </section>;
}
