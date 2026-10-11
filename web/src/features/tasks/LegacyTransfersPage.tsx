import { ArrowUpDown, RotateCcw, X } from "lucide-react";
import { bytes, errorText, type Operation } from "../../api";
import { History } from "../../History";
import { MediaThumbnail } from "../../Media";
import { kindLabel, statusLabel } from "../../i18n";
import { handleListKey } from "../../page-list";
import { DataTable } from "../../components/DataTable";
import { Button } from "../../components/ui/button";
import { Progress } from "../../components/ui/progress";

interface Props {
  operations: Operation[];
  scope: string;
  busy: boolean;
  cancelId: string;
  onNavigate: (page: "transfers" | "inbox" | "shares" | "files") => void;
  onOpen: (operation: Operation) => void;
  onCopy: (operation: Operation) => void;
  onResume: (operation: Operation) => void;
  onCancelRequest: (id: string) => void;
  onCancelConfirm: (operation: Operation) => void;
}

// Read-only operation projection. Recovery and cancellation retain owner IDs.
export function LegacyTransfersPage(p: Props) {
  return <>
    <nav className="delivery-tabs" aria-label="交付中心视图">
      <Button variant="ghost" type="button" className="active" onClick={() => p.onNavigate("transfers")}>传输任务</Button>
      <Button variant="ghost" type="button" onClick={() => p.onNavigate("inbox")}>待处理产物</Button>
      <Button variant="ghost" type="button" onClick={() => p.onNavigate("shares")}>分享交付</Button>
    </nav>
    <History ops={p.operations} scope={p.scope}>{operations => <div className="card-stack" role="list" aria-label="传输记录" onKeyDown={e => handleListKey(e.nativeEvent, e.currentTarget, {
      open(el) { const op = operations.find(o => o.id === el.dataset.id); if (op?.status === "completed") p.onOpen(op); },
      cancel(el) { const op = operations.find(o => o.id === el.dataset.id); if (op && !["completed", "cancelled"].includes(op.status)) p.onCancelRequest(op.id); },
    })}>
      {!operations.length && <div className="empty"><ArrowUpDown size={36} /><h3>还没有传输记录</h3><p>上传、复制或导入文件后，进度会显示在这里。↑↓ 选择记录，Enter 打开结果。</p><Button type="button" onClick={() => p.onNavigate("files")}>去网盘上传</Button></div>}
      <DataTable label="交付记录" data={operations} getRowId={op => String(op.id)} columns={[
        { id: "type", header: "类型", sortable: false, cell: op => op.file_id && op.version_id ? <MediaThumbnail file={{ id: op.file_id, version_id: op.version_id, path: op.path, mime_type: op.mime_type || "application/octet-stream", size_bytes: op.size_bytes, space_id: op.space_id, trashed: false, updated_at: op.updated_at || "" }} /> : <span className="file-type document"><ArrowUpDown /></span> },
        { id: "details", header: "内容与状态", cell: op => <div className="transfer-info">
          <strong>{op.path}</strong>
          {op.caller && <small>来自 {op.caller}{op.task_ref ? " · " + op.task_ref : ""}</small>}
          <div className="transfer-meta"><span>{[kindLabel(op.kind), statusLabel(op.status)].filter(Boolean).join(" · ")}</span><span>{op.total_files ? `${op.progress_files || 0} / ${op.total_files} 个文件` : `${bytes(op.bytes_transferred)} / ${bytes(op.size_bytes)}`}</span></div>
          <Progress value={op.status === "completed" ? Math.max(1, op.total_files || op.size_bytes) : op.total_files ? op.progress_files || 0 : op.bytes_transferred} max={Math.max(1, op.total_files || op.size_bytes)} />
          {op.error && <small className="error">{errorText(new Error(op.error))}</small>}
          {p.cancelId === op.id && <div className="inline-confirm" role="group" aria-label="确认取消传输"><span>取消后可从原操作恢复，不会覆盖已有文件。</span><Button variant="destructive" size="sm" type="button" onClick={() => p.onCancelConfirm(op)}>确认取消</Button><Button variant="ghost" type="button" className="text-button" onClick={() => p.onCancelRequest("")}>返回</Button></div>}
        </div> },
        { id: "actions", header: "操作", sortable: false, cell: op => <div className="flex flex-wrap gap-2">
          {op.status === "completed" && op.file_id && op.version_id && <div className="row-actions"><Button variant="outline" size="sm" type="button" disabled={p.busy} onClick={() => p.onOpen(op)}>{p.busy ? "正在打开…" : "查看结果"}</Button><Button variant="outline" size="sm" type="button" onClick={() => p.onCopy(op)}>复制引用</Button></div>}
          {!["completed", "cancelled"].includes(op.status) && <div className="row-actions"><Button variant="outline" size="sm" type="button" onClick={() => p.onResume(op)}><RotateCcw size={15} />恢复</Button><Button variant="ghost" size="icon" type="button" className="icon" aria-label={"取消 " + op.path} onClick={() => p.onCancelRequest(op.id)}><X size={17} /></Button></div>}
        </div> },
      ]} />
    </div>}</History>
  </>;
}
