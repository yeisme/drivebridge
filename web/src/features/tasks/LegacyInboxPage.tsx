import { Inbox } from "lucide-react";
import { bytes, errorText, type Operation } from "../../api";
import { History } from "../../History";
import { MediaThumbnail } from "../../Media";
import { statusLabel } from "../../i18n";
import { handleListKey } from "../../page-list";
import { DataTable } from "../../components/DataTable";
import { Button } from "../../components/ui/button";

interface Props {
  operations: Operation[];
  scope: string;
  busy: boolean;
  onOpen: (operation: Operation) => void;
  onCopy: (operation: Operation) => void;
  onTransfers: () => void;
}

// Legacy operation results, not the workspace submission review model.
export function LegacyInboxPage(p: Props) {
  return <><div className="mb-3 flex justify-end"><Button variant="outline" type="button" className="min-h-11 md:min-h-8" onClick={p.onTransfers}>查看传输记录</Button></div><History ops={p.operations} scope={p.scope}>{operations => <div className="card-stack" role="list" aria-label="产物收件箱" onKeyDown={e => handleListKey(e.nativeEvent, e.currentTarget, { open(el) {
    const op = operations.find(o => o.id === el.dataset.id);
    if (op?.status === "completed") p.onOpen(op);
    else if (op) p.onTransfers();
  } })}>
    {!operations.length && <div className="empty"><Inbox size={36} /><h3>最近没有产物</h3><p>Agent 完成上传或导入后，结果会出现在这里。Enter 打开固定版本。</p></div>}
    <DataTable label="交付记录" data={operations} getRowId={op => String(op.id)} columns={[
      { id: "type", header: "类型", sortable: false, cell: op => <span className="file-type document">{op.file_id && op.version_id ? <MediaThumbnail file={{ id: op.file_id, version_id: op.version_id, path: op.path, mime_type: op.mime_type || "application/octet-stream", size_bytes: op.size_bytes, space_id: op.space_id, trashed: false, updated_at: op.updated_at || "" }} /> : <Inbox />}</span> },
      { id: "details", header: "内容与状态", cell: op => <div className="transfer-info"><strong>{op.path}</strong><small>{op.caller ? `来自 ${op.caller}` : "来源未声明"}{op.task_ref ? " · " + op.task_ref : ""}</small><div className="transfer-meta"><span>{op.status === "completed" ? "可归档" : statusLabel(op.status)}</span><span>{bytes(op.size_bytes)}</span></div>{op.error && <small className="error">{errorText(new Error(op.error))}</small>}</div> },
      { id: "actions", header: "操作", sortable: false, cell: op => <div className="flex flex-wrap gap-2">
        {op.status === "completed" && op.file_id && op.version_id && <div className="row-actions"><Button variant="outline" size="sm" type="button" disabled={p.busy} onClick={() => p.onOpen(op)}>{p.busy ? "正在打开…" : "预览并选版本"}</Button><Button variant="outline" size="sm" type="button" onClick={() => p.onCopy(op)}>复制引用</Button></div>}
        {op.status !== "completed" && <Button variant="outline" size="sm" type="button" onClick={p.onTransfers}>查看恢复</Button>}
      </div> },
    ]} />
  </div>}</History></>;
}
